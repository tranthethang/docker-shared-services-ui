package ops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"

	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

type fakeMaintDocker struct{}

func (fakeMaintDocker) ContainerList(context.Context, container.ListOptions) ([]container.Summary, error) {
	return nil, nil
}
func (fakeMaintDocker) ContainerInspect(context.Context, string) (container.InspectResponse, error) {
	return container.InspectResponse{}, nil
}
func (fakeMaintDocker) NetworkInspect(context.Context, string, network.InspectOptions) (network.Inspect, error) {
	return network.Inspect{}, nil
}
func (fakeMaintDocker) NetworkCreate(context.Context, string, network.CreateOptions) (network.CreateResponse, error) {
	return network.CreateResponse{}, nil
}
func (fakeMaintDocker) ContainersPrune(context.Context, filters.Args) (container.PruneReport, error) {
	return container.PruneReport{ContainersDeleted: []string{"c1"}, SpaceReclaimed: 100}, nil
}
func (fakeMaintDocker) ImagesPrune(context.Context, filters.Args) (image.PruneReport, error) {
	return image.PruneReport{SpaceReclaimed: 200}, nil
}
func (fakeMaintDocker) NetworksPrune(context.Context, filters.Args) (network.PruneReport, error) {
	return network.PruneReport{}, nil
}
func (fakeMaintDocker) VolumesPrune(context.Context, filters.Args) (volume.PruneReport, error) {
	return volume.PruneReport{SpaceReclaimed: 50}, nil
}

type fakeMaintCompose struct {
	mu    sync.Mutex
	calls [][]string
}

func (f *fakeMaintCompose) RunCompose(_ context.Context, _, _, service string, extra []string, _ func(string)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string{service}, extra...))
	return nil
}

func waitJobDone(t *testing.T, r *Runner) Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snap := r.Status()
		if !snap.Busy && len(snap.History) > 0 {
			return snap.History[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for job")
	return Job{}
}

func TestTeardownConfirmAndReverseOrder(t *testing.T) {
	dir := t.TempDir()
	for _, svc := range []string{"traefik", "postgres", "redis"} {
		svcDir := filepath.Join(dir, svc)
		if err := os.MkdirAll(svcDir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(svcDir, "docker-compose.yml"), []byte("services: {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	compose := &fakeMaintCompose{}
	r := NewRunner(Config{
		RepoPath:       dir,
		Identity:       stack.Identity{Name: "mystack", Prefix: "mystack-"},
		ActiveServices: []string{"traefik", "postgres", "redis"},
		Docker:         fakeMaintDocker{},
		Compose:        compose,
	})

	if _, err := r.StartTeardown(context.Background(), "wrong"); !errors.Is(err, ErrConfirmMismatch) {
		t.Fatalf("expected ErrConfirmMismatch, got %v", err)
	}
	if _, err := r.StartPrune(context.Background(), ""); !errors.Is(err, ErrConfirmMismatch) {
		t.Fatalf("expected ErrConfirmMismatch for prune, got %v", err)
	}

	job, err := r.StartTeardown(context.Background(), "mystack")
	if err != nil {
		t.Fatal(err)
	}
	if job.Kind != "teardown" {
		t.Fatalf("unexpected job kind: %s", job.Kind)
	}
	done := waitJobDone(t, r)
	if done.Status != StatusSucceeded {
		t.Fatalf("teardown failed: %s", done.Message)
	}

	if len(compose.calls) != 3 {
		t.Fatalf("expected 3 compose calls, got %d", len(compose.calls))
	}
	if compose.calls[len(compose.calls)-1][0] != "traefik" {
		t.Fatalf("expected traefik stopped last, got %v", compose.calls)
	}
}

func TestValidateAndPrune(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "redis"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "redis", "docker-compose.yml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	compose := &fakeMaintCompose{}
	r := NewRunner(Config{
		RepoPath:       dir,
		Identity:       stack.Identity{Name: "demo"},
		ActiveServices: []string{"redis"},
		Docker:         fakeMaintDocker{},
		Compose:        compose,
	})

	if _, err := r.StartValidate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if done := waitJobDone(t, r); done.Status != StatusSucceeded {
		t.Fatalf("validate failed: %s", done.Message)
	}
	if !slices.Equal(compose.calls[0], []string{"redis", "config", "-q"}) {
		t.Fatalf("unexpected validate args: %v", compose.calls[0])
	}

	if _, err := r.StartPrune(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if done := waitJobDone(t, r); done.Status != StatusSucceeded {
		t.Fatalf("prune failed: %s", done.Message)
	}
}
