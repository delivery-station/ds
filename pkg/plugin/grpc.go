package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/delivery-station/ds/pkg/types"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Handshake is a common handshake config for all plugins
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "DS_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "delivery-station-plugin",
}

// PluginMap is the map of plugins we can dispense.
var PluginMap = map[string]plugin.Plugin{
	"ds-plugin": &DSPlugin{},
}

// DSPlugin is the implementation of plugin.Plugin so we can serve/consume this
type DSPlugin struct {
	plugin.Plugin
	// Impl is the interface implementation
	Impl types.PluginProtocol
}

func (p *DSPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	RegisterDSPluginServer(s, &GRPCServer{Impl: p.Impl, broker: broker})
	return nil
}

func (p *DSPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{client: NewDSPluginClient(c), broker: broker}, nil
}

// GRPCClient is an implementation of PluginProtocol that talks over RPC
type GRPCClient struct {
	client DSPluginClient
	broker *plugin.GRPCBroker
}

func (m *GRPCClient) GetManifest(ctx context.Context) (*types.PluginManifest, error) {
	resp, err := m.client.GetManifest(ctx, &GetManifestRequest{})
	if err != nil {
		return nil, err
	}

	manifest := &types.PluginManifest{
		Name:        resp.GetName(),
		Version:     resp.GetVersion(),
		Description: resp.GetDescription(),
	}

	if resp.GetPlatform() != nil {
		manifest.Platform = types.PluginPlatform{
			OS:   append([]string{}, resp.GetPlatform().GetOs()...),
			Arch: append([]string{}, resp.GetPlatform().GetArch()...),
		}
	}

	if len(resp.GetAnnotations()) > 0 {
		manifest.Annotations = make(map[string]string, len(resp.GetAnnotations()))
		for k, v := range resp.GetAnnotations() {
			manifest.Annotations[k] = v
		}
	}

	if len(resp.GetCommands()) > 0 {
		manifest.Commands = make([]types.PluginCommand, 0, len(resp.GetCommands()))
		for _, cmd := range resp.GetCommands() {
			if cmd == nil {
				continue
			}
			manifest.Commands = append(manifest.Commands, types.PluginCommand{
				Name:        cmd.GetName(),
				Description: cmd.GetDescription(),
			})
		}
	}

	return manifest, nil
}

func (m *GRPCClient) Execute(ctx context.Context, operation string, args []string, env map[string]string) (*types.ExecutionResult, error) {
	var brokerID uint32

	if m.broker != nil {
		if cfg, ok := types.HostConfigPayloadFromContext(ctx); ok && cfg != nil {
			brokerID = m.broker.NextId()

			server := &hostConfigServer{config: cfg}
			go m.broker.AcceptAndServe(brokerID, func(opts []grpc.ServerOption) *grpc.Server {
				grpcServer := grpc.NewServer(opts...)
				RegisterHostConfigServer(grpcServer, server)
				return grpcServer
			})

			ctx = types.WithHostConfigBrokerID(ctx, brokerID)
		}
	}

	if brokerID == 0 {
		if id, ok := types.HostConfigBrokerIDFromContext(ctx); ok {
			brokerID = id
		}
	}

	resp, err := m.client.Execute(ctx, &ExecuteRequest{
		Operation:          operation,
		Args:               args,
		Env:                env,
		HostConfigBrokerId: brokerID,
	})
	if err != nil {
		return nil, err
	}

	finalizers := make([]types.FinalizerRequest, 0, len(resp.Finalizers))
	for _, f := range resp.Finalizers {
		if f == nil {
			continue
		}
		finalizer := types.FinalizerRequest{
			Name:      f.GetName(),
			Operation: f.GetOperation(),
			Args:      append([]string{}, f.GetArgs()...),
		}
		finalizers = append(finalizers, finalizer)
	}

	return &types.ExecutionResult{
		Stdout:     resp.Stdout,
		Stderr:     resp.Stderr,
		ExitCode:   int(resp.ExitCode),
		Error:      resp.Error,
		Finalizers: finalizers,
	}, nil
}

func (m *GRPCClient) ValidateConfig(ctx context.Context, config map[string]interface{}) error {
	// Convert map[string]interface{} to map[string]string for proto
	strConfig := make(map[string]string)
	for k, v := range config {
		strConfig[k] = fmt.Sprintf("%v", v)
	}

	resp, err := m.client.ValidateConfig(ctx, &ValidateConfigRequest{
		Config: strConfig,
	})
	if err != nil {
		return err
	}
	if !resp.Valid {
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (m *GRPCClient) GetSchema(ctx context.Context) (*types.PluginSchema, error) {
	resp, err := m.client.GetSchema(ctx, &GetSchemaRequest{})
	if err != nil {
		return nil, err
	}

	props := make(map[string]types.SchemaProperty)
	for k, v := range resp.Properties {
		props[k] = types.SchemaProperty{
			Type:        v.Type,
			Description: v.Description,
			Required:    v.Required,
			Default:     v.Default,
		}
	}

	return &types.PluginSchema{
		Version:    resp.Version,
		Properties: props,
	}, nil
}

// GRPCServer is the gRPC server that GRPCClient talks to
type GRPCServer struct {
	UnimplementedDSPluginServer
	Impl   types.PluginProtocol
	broker *plugin.GRPCBroker
}

func (m *GRPCServer) GetManifest(ctx context.Context, req *GetManifestRequest) (*GetManifestResponse, error) {
	provider, ok := m.Impl.(types.PluginManifestProvider)
	if !ok {
		return nil, status.Errorf(codes.Unimplemented, "manifest RPC not implemented")
	}

	manifest, err := provider.GetManifest(ctx)
	if err != nil {
		return nil, err
	}

	if manifest == nil {
		return &GetManifestResponse{}, nil
	}

	resp := &GetManifestResponse{
		Name:        manifest.Name,
		Version:     manifest.Version,
		Description: manifest.Description,
	}

	if len(manifest.Commands) > 0 {
		resp.Commands = make([]*PluginCommand, 0, len(manifest.Commands))
		for _, cmd := range manifest.Commands {
			resp.Commands = append(resp.Commands, &PluginCommand{
				Name:        cmd.Name,
				Description: cmd.Description,
			})
		}

	}

	if manifest.Annotations != nil {
		resp.Annotations = make(map[string]string, len(manifest.Annotations))
		for k, v := range manifest.Annotations {
			resp.Annotations[k] = v
		}
	}

	if len(manifest.Platform.OS) > 0 || len(manifest.Platform.Arch) > 0 {
		resp.Platform = &Platform{
			Os:   append([]string{}, manifest.Platform.OS...),
			Arch: append([]string{}, manifest.Platform.Arch...),
		}
	}

	return resp, nil
}

func (m *GRPCServer) Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResponse, error) {
	if req.HostConfigBrokerId != 0 && m.broker != nil {
		provider := newHostConfigClient(m.broker, req.HostConfigBrokerId)
		ctx = types.WithHostConfigProvider(ctx, provider)
	}

	res, err := m.Impl.Execute(ctx, req.Operation, req.Args, req.Env)
	if err != nil {
		return nil, err
	}

	finalizers := make([]*FinalizerRequest, 0, len(res.Finalizers))
	for _, f := range res.Finalizers {
		finalizers = append(finalizers, &FinalizerRequest{
			Name:      f.Name,
			Operation: f.Operation,
			Args:      append([]string{}, f.Args...),
		})
	}
	return &ExecuteResponse{
		Stdout:     res.Stdout,
		Stderr:     res.Stderr,
		ExitCode:   int32(res.ExitCode),
		Error:      res.Error,
		Finalizers: finalizers,
	}, nil
}

func (m *GRPCServer) ValidateConfig(ctx context.Context, req *ValidateConfigRequest) (*ValidateConfigResponse, error) {
	// Convert back
	config := make(map[string]interface{})
	for k, v := range req.Config {
		config[k] = v
	}
	err := m.Impl.ValidateConfig(ctx, config)
	if err != nil {
		return &ValidateConfigResponse{Valid: false, Error: err.Error()}, nil
	}
	return &ValidateConfigResponse{Valid: true}, nil
}

func (m *GRPCServer) GetSchema(ctx context.Context, req *GetSchemaRequest) (*GetSchemaResponse, error) {
	schema, err := m.Impl.GetSchema(ctx)
	if err != nil {
		return nil, err
	}

	props := make(map[string]*SchemaProperty)
	for k, v := range schema.Properties {
		props[k] = &SchemaProperty{
			Type:        v.Type,
			Description: v.Description,
			Required:    v.Required,
			Default:     v.Default,
		}
	}

	return &GetSchemaResponse{
		Version:    schema.Version,
		Properties: props,
	}, nil
}

type brokerHostConfigClient struct {
	broker *plugin.GRPCBroker
	id     uint32

	mu     sync.Mutex
	client HostConfigClient
}

func newHostConfigClient(broker *plugin.GRPCBroker, id uint32) *brokerHostConfigClient {
	return &brokerHostConfigClient{broker: broker, id: id}
}

func (c *brokerHostConfigClient) ensureClient() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client != nil {
		return nil
	}

	conn, err := c.broker.Dial(c.id)
	if err != nil {
		return fmt.Errorf("failed to dial host config broker: %w", err)
	}

	c.client = NewHostConfigClient(conn)
	return nil
}

func (c *brokerHostConfigClient) GetEffectiveConfig(ctx context.Context) (*types.Config, error) {
	if err := c.ensureClient(); err != nil {
		return nil, err
	}

	resp, err := c.client.GetEffectiveConfig(ctx, &GetEffectiveConfigRequest{})
	if err != nil {
		return nil, fmt.Errorf("host config request failed: %w", err)
	}

	if len(resp.ConfigJson) == 0 {
		return nil, fmt.Errorf("host returned empty config payload")
	}

	var cfg types.Config
	if err := json.Unmarshal(resp.ConfigJson, &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode host config: %w", err)
	}

	return &cfg, nil
}

var _ types.HostConfigProvider = (*brokerHostConfigClient)(nil)

type hostConfigServer struct {
	UnimplementedHostConfigServer
	config *types.Config
}

func (s *hostConfigServer) GetEffectiveConfig(ctx context.Context, _ *GetEffectiveConfigRequest) (*GetEffectiveConfigResponse, error) {
	if s.config == nil {
		return nil, fmt.Errorf("host configuration not available")
	}

	payload, err := json.Marshal(s.config)
	if err != nil {
		return nil, fmt.Errorf("failed to encode host configuration: %w", err)
	}

	return &GetEffectiveConfigResponse{ConfigJson: payload}, nil
}
