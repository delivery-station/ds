package types

import "time"

// Artifact represents an OCI artifact
type Artifact struct {
	Reference string            `json:"reference"`
	Digest    string            `json:"digest"`
	Size      int64             `json:"size"`
	MediaType string            `json:"mediaType"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Layers    []Layer           `json:"layers,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
}

// Layer represents a layer in an artifact
type Layer struct {
	Digest    string            `json:"digest"`
	Size      int64             `json:"size"`
	MediaType string            `json:"mediaType"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// PluginMetadata represents metadata about a plugin
type PluginMetadata struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author,omitempty"`
	Homepage    string            `json:"homepage,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ServiceInfo represents information about a registered service
type ServiceInfo struct {
	Name     string            `json:"name"`
	Address  string            `json:"address"`
	Protocol string            `json:"protocol,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Health   ServiceHealth     `json:"health,omitempty"`
}

// ServiceHealth represents the health status of a service
type ServiceHealth struct {
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
