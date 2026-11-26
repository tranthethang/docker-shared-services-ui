package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"

	"github.com/tranthethang/docker-shared-services-ui/internal/docker"
	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

type fakeDocker struct {
	mu       sync.Mutex
	running  map[string]bool
	networks map[string]bool
}

func newFakeDocker() *fakeDocker {
	return &fakeDocker{
		running:  make(map[string]bool),
		networks: map[string]bool{"traefik_proxy": true, "backend_net": true, "data_net": true},
	}
}

func (f *fakeDocker) ContainerList(_ context.Context, _ container.ListOptions) ([]container.Summary, error) {
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

func (f *fakeDocker) ContainerInspect(_ context.Context, id string) (container.InspectResponse, error) {
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

func (f *fakeDocker) NetworkInspect(_ context.Context, id string, _ network.InspectOptions) (network.Inspect, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.networks[id] {
		return network.Inspect{Name: id}, nil
	}
	return network.Inspect{}, errors.New("not found")
}

func (f *fakeDocker) NetworkCreate(_ context.Context, name string, _ network.CreateOptions) (network.CreateResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.networks[name] = true
	return network.CreateResponse{ID: name}, nil
}

type fakeCompose struct {
	mu      sync.Mutex
	calls   []string
	docker  *fakeDocker
	blockCh chan struct{}
}

func (c *fakeCompose) RunCompose(ctx context.Context, _ string, _ string, service string, extra []string, onLine func(string)) error {
	if c.blockCh != nil {
		select {
		case <-c.blockCh:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.mu.Lock()
	c.calls = append(c.calls, service+":"+strings.Join(extra, " "))
	c.mu.Unlock()
	if onLine != nil {
		onLine("running " + service + " secret=super-secret-token")
	}
	if c.docker != nil && len(extra) > 0 && extra[0] == "up" {
		c.docker.mu.Lock()
		c.docker.running[service] = true
		c.docker.mu.Unlock()
	}
	return nil
}

func setupRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestRunnerSingleAndBusy(t *testing.T) {
	dir := setupRepo(t, map[string]string{
		".env":          "API_SECRET=super-secret-token\n",
		"postgres/.env": "POSTGRES16_PASSWORD=Str0ng!PostgresPW\n",
		"zitadel/.env":  "ZITADEL_MASTERKEY=0123456789abcdef0123456789abcdef\nZITADEL_ADMIN_PASSWORD=Str0ng!ZitadelPW\n",
	})

	fd := newFakeDocker()
	block := make(chan struct{})
	fc := &fakeCompose{docker: fd, blockCh: block}
	r := NewRunner(Config{
		RepoPath:       dir,
		Identity:       stack.Identity{Networks: []stack.NetworkPair{{Name: "traefik_proxy", Subnet: "10.0.0.0/16"}}},
		ActiveServices: []string{"postgres", "zitadel", "redis"},
		Docker:         fd,
		Compose:        fc,
		ReadyTimeout:   500 * time.Millisecond,
		PollInterval:   5 * time.Millisecond,
	})

	subID, ch, _ := r.Subscribe()
	defer r.Unsubscribe(subID)

	job, err := r.StartSingle(context.Background(), "zitadel", "up")
	if err != nil {
		t.Fatalf("StartSingle(zitadel, up) error = %v", err)
	}

	if _, busyErr := r.StartSingle(context.Background(), "redis", "down"); !errors.Is(busyErr, ErrBusy) {
		t.Fatalf("expected ErrBusy while running, got %v", busyErr)
	}

	close(block)

	var doneJob *Job
	var seenLines []string
	timeout := time.After(2 * time.Second)
	for doneJob == nil {
		select {
		case ev := <-ch:
			switch ev.Type {
			case "line":
				seenLines = append(seenLines, ev.Line)
			case "done":
				doneJob = ev.Job
			}
		case <-timeout:
			t.Fatal("timed out waiting for job completion")
		}
	}

	if doneJob.ID != job.ID || doneJob.Status != StatusSucceeded {
		t.Fatalf("unexpected done job: %+v", doneJob)
	}
	wantCalls := []string{"postgres:up -d", "zitadel:up -d"}
	if !reflect.DeepEqual(fc.calls, wantCalls) {
		t.Fatalf("compose calls = %v, want %v", fc.calls, wantCalls)
	}
	for _, l := range seenLines {
		if strings.Contains(l, "super-secret-token") {
			t.Fatalf("unmasked secret in progress line: %q", l)
		}
	}
}

func TestRunnerValidationErrors(t *testing.T) {
	dir := setupRepo(t, map[string]string{
		".env": "DOMAIN=sandbox.localhost\n",
	})
	r := NewRunner(Config{
		RepoPath:       dir,
		ActiveServices: []string{"oauth2-proxy", "redis"},
		Docker:         newFakeDocker(),
		Compose:        &fakeCompose{},
	})

	if _, err := r.StartSingle(context.Background(), "unknown", "up"); !errors.Is(err, ErrUnknownService) {
		t.Fatalf("got %v, want ErrUnknownService", err)
	}
	if _, err := r.StartSingle(context.Background(), "redis", "invalid"); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("got %v, want ErrInvalidAction", err)
	}
	if _, err := r.StartSingle(context.Background(), "oauth2-proxy", "up"); !errors.Is(err, ErrServiceLocked) {
		t.Fatalf("got %v, want ErrServiceLocked", err)
	}
	if _, err := r.StartSingle(context.Background(), "redis", "up"); !errors.Is(err, ErrPrereqFailed) {
		t.Fatalf("got %v, want ErrPrereqFailed", err)
	}
}
