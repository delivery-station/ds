package plugin

import (
	"context"
	"fmt"

	"github.com/delivery-station/ds/pkg/types"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
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
	RegisterDSPluginServer(s, &GRPCServer{Impl: p.Impl})
	return nil
}

func (p *DSPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return &GRPCClient{client: NewDSPluginClient(c)}, nil
}

// GRPCClient is an implementation of PluginProtocol that talks over RPC
type GRPCClient struct {
	client DSPluginClient
}

func (m *GRPCClient) GetMetadata(ctx context.Context) (*types.PluginMetadata, error) {
	resp, err := m.client.GetMetadata(ctx, &GetMetadataRequest{})
	if err != nil {
		return nil, err
	}

	return &types.PluginMetadata{
		Name:        resp.Name,
		Version:     resp.Version,
		Description: resp.Description,
		Operations:  resp.Operations,
		Platform: types.PluginPlatform{
			OS:   resp.Platform.Os,
			Arch: resp.Platform.Arch,
		},
		Config: resp.Config,
	}, nil
}

func (m *GRPCClient) Execute(ctx context.Context, operation string, args []string, env map[string]string) (*types.ExecutionResult, error) {
	resp, err := m.client.Execute(ctx, &ExecuteRequest{
		Operation: operation,
		Args:      args,
		Env:       env,
	})
	if err != nil {
		return nil, err
	}

	return &types.ExecutionResult{
		Stdout:   resp.Stdout,
		Stderr:   resp.Stderr,
		ExitCode: int(resp.ExitCode),
		Error:    resp.Error,
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
	Impl types.PluginProtocol
}

func (m *GRPCServer) GetMetadata(ctx context.Context, req *GetMetadataRequest) (*GetMetadataResponse, error) {
	meta, err := m.Impl.GetMetadata(ctx)
	if err != nil {
		return nil, err
	}
	return &GetMetadataResponse{
		Name:        meta.Name,
		Version:     meta.Version,
		Description: meta.Description,
		Operations:  meta.Operations,
		Platform: &Platform{
			Os:   meta.Platform.OS,
			Arch: meta.Platform.Arch,
		},
		Config: meta.Config,
	}, nil
}

func (m *GRPCServer) Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResponse, error) {
	res, err := m.Impl.Execute(ctx, req.Operation, req.Args, req.Env)
	if err != nil {
		return nil, err
	}
	return &ExecuteResponse{
		Stdout:   res.Stdout,
		Stderr:   res.Stderr,
		ExitCode: int32(res.ExitCode),
		Error:    res.Error,
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
