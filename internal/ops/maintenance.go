package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"

	"github.com/tranthethang/docker-shared-services-ui/internal/docker"
	"github.com/tranthethang/docker-shared-services-ui/internal/env"
	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

// ErrConfirmMismatch is returned when a destructive operation lacks a matching stack confirmation name.
var ErrConfirmMismatch = errors.New("confirmation stack name does not match")

// DockerPruner is the optional subset of Docker SDK methods used to prune unused resources.
type DockerPruner interface {
	ContainersPrune(ctx context.Context, pruneFilters filters.Args) (container.PruneReport, error)
	ImagesPrune(ctx context.Context, pruneFilters filters.Args) (image.PruneReport, error)
	NetworksPrune(ctx context.Context, pruneFilters filters.Args) (network.PruneReport, error)
	VolumesPrune(ctx context.Context, pruneFilters filters.Args) (volume.PruneReport, error)
}

// ExpectedConfirmName returns the stack name string required to confirm destructive operations.
func ExpectedConfirmName(id stack.Identity) string {
	if id.Name != "" {
		return id.Name
	}
	return "default"
}

// StartSetup runs network creation, env file creation/merge, password/secret fill, Dozzle users, and compose validation.
func (r *Runner) StartSetup(ctx context.Context, password string, force bool) (Job, error) {
	if trimmed := strings.TrimSpace(password); trimmed != "" {
		if trimmed == env.LegacyPlaceholderPassword {
			return Job{}, fmt.Errorf("%s is an insecure placeholder", env.LegacyPlaceholderPassword)
		}
		if reason := env.ValidatePasswordStrength(trimmed); reason != "" {
			return Job{}, errors.New(reason)
		}
	}
	return r.launchJob(Job{Kind: "setup", Action: "setup"}, r.cfg.ActiveServices, "Setup completed successfully", func(say func(string)) error {
		if err := docker.EnsureNetworks(ctx, r.cfg.Docker, r.cfg.Identity.Networks, say); err != nil {
			return err
		}
		if _, err := env.CreateMissingEnv(r.cfg.RepoPath, r.cfg.ActiveServices, say); err != nil {
			return err
		}
		if _, err := env.MergeMissingEnv(r.cfg.RepoPath, r.cfg.ActiveServices, say); err != nil {
			return err
		}
		if _, err := env.FillSharedPassword(r.cfg.RepoPath, password, force, say); err != nil {
			return err
		}
		if _, err := env.FillSecrets(r.cfg.RepoPath, r.cfg.ActiveServices, say); err != nil {
			return err
		}
		if err := env.EnsureDozzleUsers(ctx, r.cfg.RepoPath, say); err != nil {
			return err
		}
		if failed := docker.ValidateComposeAll(ctx, r.cfg.Compose, r.cfg.RepoPath, r.cfg.Identity, r.cfg.ActiveServices, say); len(failed) > 0 {
			return fmt.Errorf("validation failed for: %s", strings.Join(failed, ", "))
		}
		return nil
	})
}

// StartValidate runs `docker compose config -q` across all active services.
func (r *Runner) StartValidate(ctx context.Context) (Job, error) {
	return r.launchJob(Job{Kind: "validate", Action: "validate"}, r.cfg.ActiveServices, "All Compose configurations are valid", func(say func(string)) error {
		failed := docker.ValidateComposeAll(ctx, r.cfg.Compose, r.cfg.RepoPath, r.cfg.Identity, r.cfg.ActiveServices, say)
		if len(failed) > 0 {
			return fmt.Errorf("validation failed for: %s", strings.Join(failed, ", "))
		}
		return nil
	})
}

// StartPrune prunes unused Docker containers, images, networks, and volumes after verifying confirmStackName.
func (r *Runner) StartPrune(ctx context.Context, confirmStackName string) (Job, error) {
	if confirmStackName != ExpectedConfirmName(r.cfg.Identity) {
		return Job{}, ErrConfirmMismatch
	}
	pruner, ok := r.cfg.Docker.(DockerPruner)
	if !ok {
		return Job{}, errors.New("docker client does not support prune")
	}
	return r.launchJob(Job{Kind: "prune", Action: "prune"}, nil, "Docker resources pruned", func(say func(string)) error {
		return runPrune(ctx, pruner, say)
	})
}

func runPrune(ctx context.Context, pruner DockerPruner, say func(string)) error {
	var reclaimed uint64
	cRep, err := pruner.ContainersPrune(ctx, filters.Args{})
	if err != nil {
		return fmt.Errorf("prune containers: %w", err)
	}
	reclaimed += cRep.SpaceReclaimed
	say(fmt.Sprintf("Pruned %d container(s)", len(cRep.ContainersDeleted)))

	iRep, err := pruner.ImagesPrune(ctx, filters.Args{})
	if err != nil {
		return fmt.Errorf("prune images: %w", err)
	}
	reclaimed += iRep.SpaceReclaimed
	say(fmt.Sprintf("Pruned %d image(s)", len(iRep.ImagesDeleted)))

	nRep, err := pruner.NetworksPrune(ctx, filters.Args{})
	if err != nil {
		return fmt.Errorf("prune networks: %w", err)
	}
	say(fmt.Sprintf("Pruned %d network(s)", len(nRep.NetworksDeleted)))

	vRep, err := pruner.VolumesPrune(ctx, filters.Args{})
	if err != nil {
		return fmt.Errorf("prune volumes: %w", err)
	}
	reclaimed += vRep.SpaceReclaimed
	say(fmt.Sprintf("Pruned %d volume(s); total space reclaimed: %d bytes", len(vRep.VolumesDeleted), reclaimed))
	return nil
}

// StartTeardown runs `docker compose down -v` in reverse startup order after verifying confirmStackName.
func (r *Runner) StartTeardown(ctx context.Context, confirmStackName string) (Job, error) {
	if confirmStackName != ExpectedConfirmName(r.cfg.Identity) {
		return Job{}, ErrConfirmMismatch
	}
	var targets []string
	for _, svc := range r.cfg.ActiveServices {
		info, err := os.Stat(filepath.Join(r.cfg.RepoPath, svc, "docker-compose.yml"))
		if err == nil && !info.IsDir() {
			targets = append(targets, svc)
		}
	}
	ordered := docker.ShutdownQueue(targets)
	return r.launchJob(Job{Kind: "teardown", Action: "teardown", Down: ordered}, ordered, "All containers and volumes removed", func(say func(string)) error {
		var failed []string
		for _, svc := range ordered {
			say(fmt.Sprintf("Removing %s (down -v)...", svc))
			if err := r.cfg.Compose.RunCompose(ctx, r.cfg.RepoPath, r.cfg.Identity.ProjectName(svc), svc, []string{"down", "-v"}, say); err != nil {
				failed = append(failed, svc)
			}
		}
		if len(failed) > 0 {
			return fmt.Errorf("teardown failed for: %s", strings.Join(failed, ", "))
		}
		return nil
	})
}
