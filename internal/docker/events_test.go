package docker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types/events"
)

type mockEventSubscriber struct {
	calls atomic.Int32
	send  func(attempt int32, msgCh chan<- events.Message, errCh chan<- error)
}

func (m *mockEventSubscriber) Events(ctx context.Context, _ events.ListOptions) (<-chan events.Message, <-chan error) {
	attempt := m.calls.Add(1)
	msgCh := make(chan events.Message, 8)
	errCh := make(chan error, 1)
	go func() {
		defer close(msgCh)
		defer close(errCh)
		if m.send != nil {
			m.send(attempt, msgCh, errCh)
		}
		<-ctx.Done()
	}()
	return msgCh, errCh
}

func TestWatchEventsDebouncesBurst(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := &mockEventSubscriber{
		send: func(_ int32, msgCh chan<- events.Message, _ chan<- error) {
			attrs := map[string]string{ProjectLabel: "dev-redis"}
			msgCh <- events.Message{Action: "exec_create", Actor: events.Actor{Attributes: attrs}} // ignored
			msgCh <- events.Message{Action: "start", Actor: events.Actor{Attributes: attrs}}
			msgCh <- events.Message{Action: "health_status: healthy", Actor: events.Actor{Attributes: attrs}}
			msgCh <- events.Message{Action: "die", Actor: events.Actor{Attributes: attrs}}
		},
	}

	out := WatchEvents(ctx, sub, EventWatcherOptions{
		Debounce:         25 * time.Millisecond,
		ReconnectBackoff: 10 * time.Millisecond,
	})

	select {
	case sig := <-out:
		if sig.Count != 3 {
			t.Fatalf("sig.Count = %d, want 3", sig.Count)
		}
		if sig.LastAction != "die" || sig.Project != "dev-redis" {
			t.Fatalf("unexpected signal: %+v", sig)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for debounced signal")
	}

	cancel()
	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("expected out channel to close on cancel")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for WatchEvents shutdown")
	}
}

func TestWatchEventsReconnectsAfterError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := &mockEventSubscriber{
		send: func(attempt int32, msgCh chan<- events.Message, errCh chan<- error) {
			if attempt == 1 {
				errCh <- errors.New("transient stream failure")
				return
			}
			msgCh <- events.Message{
				Action: "stop",
				Actor:  events.Actor{Attributes: map[string]string{ProjectLabel: "postgres"}},
			}
		},
	}

	out := WatchEvents(ctx, sub, EventWatcherOptions{
		Debounce:         20 * time.Millisecond,
		ReconnectBackoff: 15 * time.Millisecond,
	})

	select {
	case sig := <-out:
		if sig.LastAction != "stop" || sig.Project != "postgres" {
			t.Fatalf("unexpected signal after reconnect: %+v", sig)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for signal after reconnect")
	}

	if got := sub.calls.Load(); got < 2 {
		t.Fatalf("expected reconnect (>=2 calls), got %d", got)
	}
}
