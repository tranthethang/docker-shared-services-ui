package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/system"

	"github.com/tranthethang/docker-shared-services-ui/internal/catalog"
	"github.com/tranthethang/docker-shared-services-ui/internal/docker"
	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

type fakeEngine struct {
	summaries []container.Summary
	inspects  map[string]container.InspectResponse
	listErr   error
	eventMsgs []events.Message
}

func (f *fakeEngine) ContainerList(_ context.Context, opts container.ListOptions) ([]container.Summary, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	if opts.All {
		return f.summaries, nil
	}
	var running []container.Summary
	for _, s := range f.summaries {
		if s.State == docker.StateRunning {
			running = append(running, s)
		}
	}
	return running, nil
}

func (f *fakeEngine) ContainerInspect(_ context.Context, id string) (container.InspectResponse, error) {
	resp, ok := f.inspects[id]
	if !ok {
		return container.InspectResponse{}, errors.New("not found")
	}
	return resp, nil
}

func (f *fakeEngine) ContainerStatsOneShot(_ context.Context, _ string) (container.StatsResponseReader, error) {
	payload, _ := json.Marshal(container.StatsResponse{
		Name: "/redis",
		CPUStats: container.CPUStats{
			CPUUsage:    container.CPUUsage{TotalUsage: 200},
			SystemUsage: 1000,
			OnlineCPUs:  1,
		},
		PreCPUStats: container.CPUStats{
			CPUUsage:    container.CPUUsage{TotalUsage: 100},
			SystemUsage: 500,
		},
		MemoryStats: container.MemoryStats{Usage: 512, Limit: 1024},
	})
	return container.StatsResponseReader{Body: io.NopCloser(strings.NewReader(string(payload)))}, nil
}

func (f *fakeEngine) Info(_ context.Context) (system.Info, error) {
	return system.Info{NCPU: 4, MemTotal: 8 * 1024 * 1024 * 1024}, nil
}

func (f *fakeEngine) DiskUsage(_ context.Context, _ types.DiskUsageOptions) (types.DiskUsage, error) {
	return types.DiskUsage{}, nil
}

func (f *fakeEngine) Events(ctx context.Context, _ events.ListOptions) (<-chan events.Message, <-chan error) {
	msgCh := make(chan events.Message, len(f.eventMsgs))
	errCh := make(chan error, 1)
	go func() {
		defer close(msgCh)
		defer close(errCh)
		for _, m := range f.eventMsgs {
			msgCh <- m
		}
		<-ctx.Done()
	}()
	return msgCh, errCh
}

func (f *fakeEngine) Close() error { return nil }

func newTestHandler(eng docker.Engine) *ReadOnlyHandler {
	return &ReadOnlyHandler{
		Catalog: &catalog.Loaded{
			Domain:      "dev.localhost",
			Categories:  catalog.Categories(),
			ActiveNames: []string{"redis", "traefik"},
			Entries: map[string]catalog.Entry{
				"redis": {
					Name:        "redis",
					Category:    "Cache & Queue",
					Description: "In-memory store.",
					Containers:  []string{"redis"},
					WebUI:       []string{},
					Info:        []string{"redis://redis.dev.localhost:6379"},
				},
				"traefik": {
					Name:        "traefik",
					Category:    "Auth & Gateway",
					Description: "Edge router.",
					Containers:  []string{"traefik"},
					WebUI:       []string{"https://traefik.dev.localhost"},
					Info:        []string{},
				},
			},
		},
		Identity: stack.Identity{
			Name:       "dev",
			SubnetBase: 10,
			Prefix:     "dev-",
			Networks: []stack.NetworkPair{
				{Name: "dev_traefik_proxy", Subnet: "10.10.0.0/16", Pair: "dev_traefik_proxy:10.10.0.0/16"},
			},
		},
		Docker:            eng,
		TelemetryInterval: 30 * time.Millisecond,
		EventOptions: docker.EventWatcherOptions{
			Debounce:         15 * time.Millisecond,
			ReconnectBackoff: 10 * time.Millisecond,
		},
	}
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{
		summaries: []container.Summary{
			{
				ID:     "c1",
				Names:  []string{"/redis"},
				State:  docker.StateRunning,
				Labels: map[string]string{docker.ProjectLabel: "dev-redis"},
			},
		},
		inspects: map[string]container.InspectResponse{
			"c1": {
				ContainerJSONBase: &container.ContainerJSONBase{
					ID:    "c1",
					Name:  "/redis",
					State: &container.State{Status: docker.StateRunning, Health: &container.Health{Status: docker.HealthHealthy}},
				},
				Config: &container.Config{
					Image:  "redis:7",
					Labels: map[string]string{docker.ProjectLabel: "dev-redis", docker.ServiceLabel: "redis"},
				},
			},
		},
		eventMsgs: []events.Message{
			{Action: "start", Actor: events.Actor{Attributes: map[string]string{docker.ProjectLabel: "dev-redis"}}},
		},
	}
}

func TestReadOnlyRESTEndpoints(t *testing.T) {
	h := newTestHandler(newFakeEngine())
	mux := http.NewServeMux()
	h.Register(mux)

	t.Run("GET /api/services", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/services", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body servicesResponse
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Services) != 2 || body.Services[0].State != docker.StateRunning || body.Services[1].State != docker.StateAbsent {
			t.Fatalf("unexpected services response: %+v", body.Services)
		}
	})

	t.Run("GET /api/services/{name}", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/services/redis", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		rec404 := httptest.NewRecorder()
		mux.ServeHTTP(rec404, httptest.NewRequest(http.MethodGet, "/api/services/missing", nil))
		if rec404.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rec404.Code)
		}
	})

	t.Run("GET /api/presets", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/presets", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body presetsResponse
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body.Presets) != 4 || len(body.Presets[0].Services) != 2 {
			t.Fatalf("unexpected presets: %+v", body.Presets)
		}
	})

	t.Run("GET /api/telemetry", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/telemetry", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body docker.Telemetry
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Capacity.NCPU != 4 || len(body.Containers) != 1 {
			t.Fatalf("unexpected telemetry: %+v", body)
		}
	})

	t.Run("GET /api/info", func(t *testing.T) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/info", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var body infoResponse
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Domain != "dev.localhost" || body.StackName != "dev" || body.ServiceCount != 2 {
			t.Fatalf("unexpected info: %+v", body)
		}
	})
}

func TestSSEEventsStream(t *testing.T) {
	h := newTestHandler(newFakeEngine())
	mux := http.NewServeMux()
	h.Register(mux)

	ts := httptest.NewServer(mux)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}

	reader := bufio.NewReader(resp.Body)
	var sawRefresh, sawTelemetry bool
	for !sawRefresh || !sawTelemetry {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read SSE line error: %v (sawRefresh=%v sawTelemetry=%v)", err, sawRefresh, sawTelemetry)
		}
		if strings.HasPrefix(line, "event: refresh") {
			sawRefresh = true
		}
		if strings.HasPrefix(line, "event: telemetry") {
			sawTelemetry = true
		}
	}
	cancel()
}
