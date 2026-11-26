package docker

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
)

func TestDependencyAndShutdownQueues(t *testing.T) {
	if got := DependencyQueue("zitadel", nil); !reflect.DeepEqual(got, []string{"postgres"}) {
		t.Fatalf("DependencyQueue(zitadel) = %v, want [postgres]", got)
	}
	if got := DependencyQueue("hindsight", nil); !reflect.DeepEqual(got, []string{"pgvector"}) {
		t.Fatalf("DependencyQueue(hindsight) = %v, want [pgvector]", got)
	}

	up := StartupQueue([]string{"zitadel", "postgres", "traefik", "redis"})
	wantUp := []string{"traefik", "postgres", "zitadel", "redis"}
	if !reflect.DeepEqual(up, wantUp) {
		t.Fatalf("StartupQueue() = %v, want %v", up, wantUp)
	}

	down := ShutdownQueue([]string{"traefik", "postgres", "zitadel"})
	if down[len(down)-1] != "traefik" {
		t.Fatalf("expected traefik last in ShutdownQueue, got %v", down)
	}
}

type scriptedInspector struct {
	mu       sync.Mutex
	sequence []container.InspectResponse
	idx      int
	prefix   string
	svc      string
}

func (s *scriptedInspector) ContainerList(_ context.Context, _ container.ListOptions) ([]container.Summary, error) {
	return []container.Summary{
		{ID: "c1", Labels: map[string]string{ProjectLabel: s.prefix + s.svc}},
	}, nil
}

func (s *scriptedInspector) ContainerInspect(_ context.Context, _ string) (container.InspectResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	resp := s.sequence[s.idx]
	if s.idx+1 < len(s.sequence) {
		s.idx++
	}
	return resp, nil
}

func TestWaitUntilReady(t *testing.T) {
	t.Run("becomes healthy after polling", func(t *testing.T) {
		insp := &scriptedInspector{
			prefix: "dev-",
			svc:    "postgres",
			sequence: []container.InspectResponse{
				makeInspect("c1", "postgres", "running", "starting", "dev-postgres", "postgres", "unless-stopped", 0),
				makeInspect("c1", "postgres", "running", "healthy", "dev-postgres", "postgres", "unless-stopped", 0),
			},
		}
		err := WaitUntilReady(context.Background(), insp, "dev-", "postgres", 500*time.Millisecond, 5*time.Millisecond, nil)
		if err != nil {
			t.Fatalf("WaitUntilReady() error = %v", err)
		}
	})

	t.Run("fails on unhealthy timeout", func(t *testing.T) {
		insp := &scriptedInspector{
			prefix: "dev-",
			svc:    "postgres",
			sequence: []container.InspectResponse{
				makeInspect("c1", "postgres", "running", "unhealthy", "dev-postgres", "postgres", "unless-stopped", 0),
			},
		}
		err := WaitUntilReady(context.Background(), insp, "dev-", "postgres", 30*time.Millisecond, 5*time.Millisecond, nil)
		if err == nil {
			t.Fatal("expected timeout error for unhealthy container")
		}
	})
}

func TestWaitUntilOneshotsDone(t *testing.T) {
	t.Run("succeeds when one-shot exits 0", func(t *testing.T) {
		insp := &scriptedInspector{
			prefix: "dev-",
			svc:    "crawl4ai",
			sequence: []container.InspectResponse{
				makeInspect("c1", "init", "running", "", "dev-crawl4ai", "init", "no", 0),
				makeInspect("c1", "init", "exited", "", "dev-crawl4ai", "init", "no", 0),
			},
		}
		err := WaitUntilOneshotsDone(context.Background(), insp, "dev-", "crawl4ai", 500*time.Millisecond, 5*time.Millisecond, nil)
		if err != nil {
			t.Fatalf("WaitUntilOneshotsDone() error = %v", err)
		}
	})

	t.Run("fails when one-shot exits non-zero", func(t *testing.T) {
		insp := &scriptedInspector{
			prefix: "dev-",
			svc:    "crawl4ai",
			sequence: []container.InspectResponse{
				makeInspect("c1", "init", "exited", "", "dev-crawl4ai", "init", "no", 2),
			},
		}
		err := WaitUntilOneshotsDone(context.Background(), insp, "dev-", "crawl4ai", 500*time.Millisecond, 5*time.Millisecond, nil)
		if err == nil {
			t.Fatal("expected error for failed one-shot")
		}
	})
}
