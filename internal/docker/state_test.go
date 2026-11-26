package docker

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types/container"
)

type mockInspector struct {
	summaries []container.Summary
	inspects  map[string]container.InspectResponse
	listErr   error
}

func (m *mockInspector) ContainerList(_ context.Context, _ container.ListOptions) ([]container.Summary, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.summaries, nil
}

func (m *mockInspector) ContainerInspect(_ context.Context, id string) (container.InspectResponse, error) {
	resp, ok := m.inspects[id]
	if !ok {
		return container.InspectResponse{}, errors.New("not found")
	}
	return resp, nil
}

func makeInspect(id, name, status, health, project, svc, policy string, exitCode int) container.InspectResponse {
	var h *container.Health
	if health != "" {
		h = &container.Health{Status: health}
	}
	return container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:   id,
			Name: "/" + name,
			State: &container.State{
				Status:   status,
				ExitCode: exitCode,
				Health:   h,
			},
			HostConfig: &container.HostConfig{
				RestartPolicy: container.RestartPolicy{
					Name: container.RestartPolicyMode(policy),
				},
			},
		},
		Config: &container.Config{
			Image: "example:latest",
			Labels: map[string]string{
				ProjectLabel: project,
				ServiceLabel: svc,
			},
		},
	}
}

func TestListServiceStatuses(t *testing.T) {
	mock := &mockInspector{
		summaries: []container.Summary{
			{ID: "c1", Labels: map[string]string{ProjectLabel: "dev-crawl4ai"}},
			{ID: "c2", Labels: map[string]string{ProjectLabel: "dev-crawl4ai"}},
			{ID: "c3", Labels: map[string]string{ProjectLabel: "dev-postgres"}},
			{ID: "c4", Labels: map[string]string{ProjectLabel: "dev-redis"}},
			{ID: "c5", Labels: map[string]string{ProjectLabel: "dev-redis"}},
			{ID: "c6", Labels: map[string]string{ProjectLabel: "other-project"}},
		},
		inspects: map[string]container.InspectResponse{
			"c1": makeInspect("c1", "crawl4ai-volume-init", "exited", "", "dev-crawl4ai", "crawl4ai-volume-init", "no", 0),
			"c2": makeInspect("c2", "crawl4ai", "running", "healthy", "dev-crawl4ai", "crawl4ai", "unless-stopped", 0),
			"c3": makeInspect("c3", "postgres", "exited", "", "dev-postgres", "postgres", "unless-stopped", 1),
			"c4": makeInspect("c4", "redis-1", "running", "healthy", "dev-redis", "redis", "unless-stopped", 0),
			"c5": makeInspect("c5", "redis-2", "running", "unhealthy", "dev-redis", "redis", "unless-stopped", 0),
		},
	}

	services := []string{"crawl4ai", "postgres", "redis", "traefik"}
	statuses, err := ListServiceStatuses(context.Background(), mock, services, "dev-")
	if err != nil {
		t.Fatalf("ListServiceStatuses() error = %v", err)
	}

	byName := make(map[string]ServiceStatus, len(statuses))
	for _, st := range statuses {
		byName[st.Name] = st
	}

	// crawl4ai: 1 finished init container + 1 running healthy container => running, healthy, count 1
	c := byName["crawl4ai"]
	if c.State != StateRunning || c.Health != HealthHealthy || c.ContainerCount != 1 || c.RunningCount != 1 {
		t.Fatalf("unexpected crawl4ai status: %+v", c)
	}
	if len(c.Containers) != 2 || !c.Containers[0].Completed {
		t.Fatalf("expected completed init container preserved in Containers: %+v", c.Containers)
	}

	// postgres: exited with restart policy unless-stopped => stopped, "-"
	p := byName["postgres"]
	if p.State != StateStopped || p.Health != HealthDash {
		t.Fatalf("unexpected postgres status: %+v", p)
	}

	// redis: unhealthy overrides healthy
	r := byName["redis"]
	if r.State != StateRunning || r.Health != HealthUnhealthy || r.ContainerCount != 2 {
		t.Fatalf("unexpected redis status: %+v", r)
	}

	// traefik: no containers => absent, "-"
	tr := byName["traefik"]
	if tr.State != StateAbsent || tr.Health != HealthDash || tr.ContainerCount != 0 {
		t.Fatalf("unexpected traefik status: %+v", tr)
	}
}

func TestBuildStatusesHealthPrecedenceAndPartial(t *testing.T) {
	containers := []ContainerInfo{
		{Name: "a", Status: StateRunning, Health: HealthHealthy, Project: "app"},
		{Name: "b", Status: StateRunning, Health: HealthStarting, Project: "app"},
		{Name: "c", Status: "exited", Health: HealthDash, Project: "app"},
		{Name: "d", Status: StateRunning, Health: HealthNone, Project: "plain"},
	}

	statuses := BuildStatuses([]string{"app", "plain"}, containers, "")
	if statuses[0].State != StatePartial || statuses[0].Health != HealthStarting {
		t.Fatalf("unexpected app status: %+v", statuses[0])
	}
	if statuses[1].State != StateRunning || statuses[1].Health != HealthNone {
		t.Fatalf("unexpected plain status: %+v", statuses[1])
	}
}
