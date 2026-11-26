package api

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"

	"github.com/tranthethang/docker-shared-services-ui/internal/catalog"
	"github.com/tranthethang/docker-shared-services-ui/internal/docker"
	"github.com/tranthethang/docker-shared-services-ui/internal/ops"
	"github.com/tranthethang/docker-shared-services-ui/internal/server"
	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

type fakeOpsDocker struct {
	mu       sync.Mutex
	running  map[string]bool
	networks map[string]bool
	logBytes []byte
}

func (f *fakeOpsDocker) ContainerList(_ context.Context, _ container.ListOptions) ([]container.Summary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []container.Summary
	for svc, up := range f.running {
		if up {
			out = append(out, container.Summary{
				ID:     "id-" + svc,
				Labels: map[string]string{docker.ProjectLabel: svc},
			})
		}
	}
	return out, nil
}

func (f *fakeOpsDocker) ContainerInspect(_ context.Context, id string) (container.InspectResponse, error) {
	svc := strings.TrimPrefix(id, "id-")
	return container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{
			ID:   id,
			Name: "/" + svc,
			State: &container.State{
				Status: "running",
				Health: &container.Health{Status: "healthy"},
			},
			HostConfig: &container.HostConfig{
				RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
			},
		},
		Config: &container.Config{
			Labels: map[string]string{
				docker.ProjectLabel: svc,
				docker.ServiceLabel: svc,
			},
		},
	}, nil
}

func (f *fakeOpsDocker) NetworkInspect(_ context.Context, id string, _ network.InspectOptions) (network.Inspect, error) {
	if f.networks[id] {
		return network.Inspect{Name: id}, nil
	}
	return network.Inspect{}, errors.New("not found")
}

func (f *fakeOpsDocker) NetworkCreate(_ context.Context, name string, _ network.CreateOptions) (network.CreateResponse, error) {
	f.networks[name] = true
	return network.CreateResponse{ID: name}, nil
}

func (f *fakeOpsDocker) ContainerLogs(_ context.Context, _ string, _ container.LogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.logBytes)), nil
}

type fakeOpsCompose struct {
	blockCh chan struct{}
}

func (c *fakeOpsCompose) RunCompose(ctx context.Context, _, _, service string, _ []string, onLine func(string)) error {
	if c.blockCh != nil {
		select {
		case <-c.blockCh:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if onLine != nil {
		onLine("started " + service)
	}
	return nil
}

func muxLogFrame(payload string) []byte {
	var buf bytes.Buffer
	header := [8]byte{1}
	binary.BigEndian.PutUint32(header[4:], uint32(len(payload))) //nolint:gosec // test payload fits in uint32
	buf.Write(header[:])
	buf.WriteString(payload)
	return buf.Bytes()
}

func setupAPITestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		".env":       "GLOBAL_SECRET=top-secret-pw\n",
		"redis/.env": "REDIS_PASSWORD=redis-pw\n",
	}
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestOpsEndpointsBusyAndUnknownService(t *testing.T) {
	dir := setupAPITestRepo(t)
	fd := &fakeOpsDocker{
		running:  map[string]bool{"redis": true},
		networks: map[string]bool{"traefik_proxy": true},
	}
	block := make(chan struct{})
	runner := ops.NewRunner(ops.Config{
		RepoPath:       dir,
		Identity:       stack.Identity{Networks: []stack.NetworkPair{{Name: "traefik_proxy", Subnet: "10.0.0.0/16"}}},
		ActiveServices: []string{"redis", "postgres"},
		Docker:         fd,
		Compose:        &fakeOpsCompose{blockCh: block},
		ReadyTimeout:   500 * time.Millisecond,
		PollInterval:   5 * time.Millisecond,
	})
	opsHandler := &OpsHandler{Runner: runner, AppCtx: context.Background()}
	h := server.NewHandlerWithAPIs(nil, opsHandler.Register)

	// Unknown service -> 400
	req := httptest.NewRequest(http.MethodPost, "/api/services/nope/up", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown service status = %d, want 400", rec.Code)
	}

	// Start redis -> 202
	req = httptest.NewRequest(http.MethodPost, "/api/services/redis/up", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start redis status = %d, want 202: %s", rec.Code, rec.Body.String())
	}

	// Concurrent mutation -> 409
	req = httptest.NewRequest(http.MethodPost, "/api/ops/apply", strings.NewReader(`{"up":["redis"],"down":[]}`))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("concurrent apply status = %d, want 409", rec.Code)
	}

	close(block)

	// Status endpoint -> 200
	req = httptest.NewRequest(http.MethodGet, "/api/ops/status", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status endpoint = %d, want 200", rec.Code)
	}
	var snap ops.StatusSnapshot
	if err := json.NewDecoder(rec.Body).Decode(&snap); err != nil {
		t.Fatalf("decode status: %v", err)
	}
}

func TestLogsEndpointMasksSecrets(t *testing.T) {
	dir := setupAPITestRepo(t)
	fd := &fakeOpsDocker{
		running:  map[string]bool{"redis": true},
		logBytes: muxLogFrame("2026-11-26T00:00:00Z redis secret=top-secret-pw\n"),
	}
	logsHandler := &LogsHandler{
		RepoPath: dir,
		Catalog: &catalog.Loaded{
			ActiveNames: []string{"redis"},
			Entries:     map[string]catalog.Entry{"redis": {Name: "redis"}},
		},
		Docker: fd,
	}
	h := server.NewHandlerWithAPIs(nil, logsHandler.Register)

	req := httptest.NewRequest(http.MethodGet, "/api/services/unknown/logs", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown service logs status = %d, want 400", rec.Code)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req = httptest.NewRequest(http.MethodGet, "/api/services/redis/logs?tail=10", nil).WithContext(ctx)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logs status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "event: log") {
		t.Fatalf("expected SSE log event, got %q", body)
	}
	if strings.Contains(body, "top-secret-pw") {
		t.Fatalf("expected secret masked in logs SSE body, got %q", body)
	}
}
