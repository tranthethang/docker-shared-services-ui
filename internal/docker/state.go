package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
)

const (
	// ProjectLabel is the Compose project label on containers.
	ProjectLabel = "com.docker.compose.project"
	// ServiceLabel is the Compose service label on containers.
	ServiceLabel = "com.docker.compose.service"

	// StateRunning indicates all non-completed containers for a service are running.
	StateRunning = "running"
	// StatePartial indicates some but not all non-completed containers are running.
	StatePartial = "partial"
	// StateStopped indicates containers exist but none are running.
	StateStopped = "stopped"
	// StateAbsent indicates no non-completed containers exist for the service.
	StateAbsent = "absent"

	// HealthUnhealthy indicates at least one active container is unhealthy.
	HealthUnhealthy = "unhealthy"
	// HealthStarting indicates at least one active container is starting (and none unhealthy).
	HealthStarting = "starting"
	// HealthHealthy indicates all healthchecked active containers are healthy.
	HealthHealthy = "healthy"
	// HealthNone indicates running container(s) have no healthcheck configured.
	HealthNone = "none"
	// HealthDash indicates no active container is running or has health information.
	HealthDash = "-"
)

// ContainerInfo represents a single inspected container belonging to a Compose service.
type ContainerInfo struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Status         string `json:"status"`
	Health         string `json:"health"`
	Image          string `json:"image"`
	ComposeService string `json:"composeService"`
	Completed      bool   `json:"completed"`
	Project        string `json:"project"`
	RestartPolicy  string `json:"restartPolicy"`
}

// ServiceStatus represents the aggregated runtime state and health of a service.
type ServiceStatus struct {
	Name           string          `json:"name"`
	State          string          `json:"state"`
	Health         string          `json:"health"`
	ContainerCount int             `json:"containerCount"`
	RunningCount   int             `json:"runningCount"`
	Containers     []ContainerInfo `json:"containers"`
}

// InspectToContainerInfo converts a Docker ContainerInspect response into ContainerInfo.
func InspectToContainerInfo(resp container.InspectResponse) ContainerInfo {
	name := strings.TrimPrefix(resp.Name, "/")

	var status string
	var exitCode int
	health := HealthNone
	if resp.State != nil {
		status = resp.State.Status
		exitCode = resp.State.ExitCode
		if resp.State.Health != nil && resp.State.Health.Status != "" {
			health = resp.State.Health.Status
		}
	}
	if health == HealthNone && status != StateRunning {
		health = HealthDash
	}

	var image, project, composeService string
	if resp.Config != nil {
		image = resp.Config.Image
		project = resp.Config.Labels[ProjectLabel]
		composeService = resp.Config.Labels[ServiceLabel]
	}
	if composeService == "" {
		composeService = name
	}

	policy := "no"
	if resp.HostConfig != nil && resp.HostConfig.RestartPolicy.Name != "" {
		policy = string(resp.HostConfig.RestartPolicy.Name)
	}

	completed := status == "exited" && exitCode == 0 && (policy == "" || policy == "no")

	return ContainerInfo{
		ID:             resp.ID,
		Name:           name,
		Status:         status,
		Health:         health,
		Image:          image,
		ComposeService: composeService,
		Completed:      completed,
		Project:        project,
		RestartPolicy:  policy,
	}
}

func stateLabel(running, total int) string {
	switch {
	case total == 0:
		return StateAbsent
	case running == 0:
		return StateStopped
	case running < total:
		return StatePartial
	default:
		return StateRunning
	}
}

func aggregateHealth(containers []ContainerInfo) string {
	has := func(target string) bool {
		for _, c := range containers {
			if c.Health == target {
				return true
			}
		}
		return false
	}
	switch {
	case has(HealthUnhealthy):
		return HealthUnhealthy
	case has(HealthStarting):
		return HealthStarting
	case has(HealthHealthy):
		return HealthHealthy
	case has(HealthNone):
		return HealthNone
	default:
		return HealthDash
	}
}

// BuildStatuses aggregates inspected containers into one ServiceStatus per configured service.
func BuildStatuses(services []string, containers []ContainerInfo, prefix string) []ServiceStatus {
	statuses := make([]ServiceStatus, 0, len(services))
	for _, svc := range services {
		own := make([]ContainerInfo, 0)
		active := make([]ContainerInfo, 0)
		running := 0

		for _, c := range containers {
			if prefix != "" && !strings.HasPrefix(c.Project, prefix) {
				continue
			}
			if strings.TrimPrefix(c.Project, prefix) != svc {
				continue
			}
			own = append(own, c)
			if !c.Completed {
				active = append(active, c)
				if c.Status == StateRunning {
					running++
				}
			}
		}

		statuses = append(statuses, ServiceStatus{
			Name:           svc,
			State:          stateLabel(running, len(active)),
			Health:         aggregateHealth(active),
			ContainerCount: len(active),
			RunningCount:   running,
			Containers:     own,
		})
	}
	return statuses
}

// ListServiceStatuses queries Docker for all stack containers and aggregates their statuses.
func ListServiceStatuses(ctx context.Context, cli ContainerInspector, services []string, prefix string) ([]ServiceStatus, error) {
	summaries, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	serviceSet := make(map[string]struct{}, len(services))
	for _, svc := range services {
		serviceSet[svc] = struct{}{}
	}

	infos := make([]ContainerInfo, 0, len(summaries))
	for _, s := range summaries {
		project := s.Labels[ProjectLabel]
		if prefix != "" && !strings.HasPrefix(project, prefix) {
			continue
		}
		if _, ok := serviceSet[strings.TrimPrefix(project, prefix)]; !ok {
			continue
		}
		resp, err := cli.ContainerInspect(ctx, s.ID)
		if err != nil {
			return nil, fmt.Errorf("inspect container %s: %w", s.ID, err)
		}
		infos = append(infos, InspectToContainerInfo(resp))
	}

	return BuildStatuses(services, infos, prefix), nil
}
