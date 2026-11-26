package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/network"

	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

// NetworkManager is the subset of Docker SDK methods needed to inspect and create shared networks.
type NetworkManager interface {
	NetworkInspect(ctx context.Context, networkID string, options network.InspectOptions) (network.Inspect, error)
	NetworkCreate(ctx context.Context, name string, options network.CreateOptions) (network.CreateResponse, error)
}

// EnsureNetworks creates any missing shared networks, falling back to an auto-assigned subnet on pool overlap.
func EnsureNetworks(ctx context.Context, cli NetworkManager, pairs []stack.NetworkPair, say func(string)) error {
	for _, pair := range pairs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := cli.NetworkInspect(ctx, pair.Name, network.InspectOptions{}); err == nil {
			continue
		} else if ctx.Err() != nil {
			return ctx.Err()
		}

		emit(say, fmt.Sprintf("Creating network %s...", pair.Name))
		opts := network.CreateOptions{
			Driver: "bridge",
			IPAM: &network.IPAM{
				Config: []network.IPAMConfig{{Subnet: pair.Subnet}},
			},
		}
		if _, err := cli.NetworkCreate(ctx, pair.Name, opts); err == nil {
			emit(say, fmt.Sprintf("Network %s created (%s)", pair.Name, pair.Subnet))
			continue
		} else if !isPoolOverlapError(err) {
			return fmt.Errorf("create network %s: %w", pair.Name, err)
		}

		emit(say, fmt.Sprintf("Subnet %s for %s overlaps another Docker address pool; using an auto-assigned subnet.", pair.Subnet, pair.Name))
		if _, err := cli.NetworkCreate(ctx, pair.Name, network.CreateOptions{Driver: "bridge"}); err != nil {
			return fmt.Errorf("create network %s (auto subnet): %w", pair.Name, err)
		}
		emit(say, fmt.Sprintf("Network %s created (auto subnet)", pair.Name))
	}
	return nil
}

func isPoolOverlapError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "overlap") || strings.Contains(lower, "pool")
}

func emit(say func(string), msg string) {
	if say != nil {
		say(msg)
	}
}
