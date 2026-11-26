package docker

import (
	"context"
	"strings"
	"time"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

const (
	// DefaultEventDebounce is the default coalescing window for burst container events.
	DefaultEventDebounce = 300 * time.Millisecond
	// DefaultReconnectBackoff is the default delay before re-subscribing after a stream error.
	DefaultReconnectBackoff = 1 * time.Second
)

// RefreshSignal represents a debounced notification that container state may have changed.
type RefreshSignal struct {
	Timestamp  time.Time `json:"timestamp"`
	LastAction string    `json:"lastAction"`
	Project    string    `json:"project,omitempty"`
	Count      int       `json:"count"`
}

// EventWatcherOptions configures debouncing and reconnect behavior for WatchEvents.
type EventWatcherOptions struct {
	Debounce         time.Duration
	ReconnectBackoff time.Duration
}

// WatchEvents subscribes to Docker container lifecycle events and emits debounced RefreshSignals.
// The returned channel is closed when ctx is cancelled.
func WatchEvents(ctx context.Context, sub EventSubscriber, opts EventWatcherOptions) <-chan RefreshSignal {
	debounce := opts.Debounce
	if debounce <= 0 {
		debounce = DefaultEventDebounce
	}
	backoff := opts.ReconnectBackoff
	if backoff <= 0 {
		backoff = DefaultReconnectBackoff
	}

	out := make(chan RefreshSignal, 1)
	go func() {
		defer close(out)
		runEventLoop(ctx, sub, debounce, backoff, out)
	}()
	return out
}

func runEventLoop(
	ctx context.Context,
	sub EventSubscriber,
	debounce time.Duration,
	backoff time.Duration,
	out chan<- RefreshSignal,
) {
	listOpts := events.ListOptions{
		Filters: filters.NewArgs(filters.Arg("type", string(events.ContainerEventType))),
	}

	timer := time.NewTimer(debounce)
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	defer timer.Stop()

	var (
		timerActive bool
		pending     int
		lastAction  string
		lastProject string
	)

	emit := func() {
		if pending == 0 {
			return
		}
		sig := RefreshSignal{
			Timestamp:  time.Now().UTC(),
			LastAction: lastAction,
			Project:    lastProject,
			Count:      pending,
		}
		pending = 0
		lastAction = ""
		lastProject = ""
		select {
		case out <- sig:
		case <-ctx.Done():
		}
	}

	for {
		if ctx.Err() != nil {
			return
		}

		msgCh, errCh := sub.Events(ctx, listOpts)
		streamOpen := true
		for streamOpen {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				timerActive = false
				emit()
			case msg, ok := <-msgCh:
				if !ok {
					streamOpen = false
					break
				}
				if !isRelevantAction(string(msg.Action)) {
					continue
				}
				pending++
				lastAction = string(msg.Action)
				lastProject = msg.Actor.Attributes[ProjectLabel]
				resetTimer(timer, debounce, &timerActive)
			case err, ok := <-errCh:
				if !ok || err != nil {
					streamOpen = false
				}
			}
		}

		if ctx.Err() != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			timerActive = false
			emit()
		case <-time.After(backoff):
		}
	}
}

func resetTimer(timer *time.Timer, d time.Duration, active *bool) {
	if *active && !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(d)
	*active = true
}

func isRelevantAction(action string) bool {
	switch action {
	case "start", "stop", "die", "destroy", "health_status":
		return true
	default:
		return strings.HasPrefix(action, "health_status:")
	}
}
