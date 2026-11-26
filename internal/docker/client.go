// Package docker wraps the Docker Engine SDK for container state, telemetry, and events.
package docker

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
)

// ContainerInspector is the subset of Docker SDK methods needed for service state aggregation.
type ContainerInspector interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
}

// StatsCollector is the subset of Docker SDK methods needed for stats, capacity, and disk usage.
type StatsCollector interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerStatsOneShot(ctx context.Context, containerID string) (container.StatsResponseReader, error)
	Info(ctx context.Context) (system.Info, error)
	DiskUsage(ctx context.Context, options types.DiskUsageOptions) (types.DiskUsage, error)
}

// EventSubscriber is the subset of Docker SDK methods needed to stream container events.
type EventSubscriber interface {
	Events(ctx context.Context, options events.ListOptions) (<-chan events.Message, <-chan error)
}

// Engine combines all Docker Engine SDK operations used by the read-only dashboard backend.
type Engine interface {
	ContainerInspector
	StatsCollector
	EventSubscriber
	Close() error
}

// NewClient constructs a Docker Engine SDK client using environment settings and API negotiation.
func NewClient() (*client.Client, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}
