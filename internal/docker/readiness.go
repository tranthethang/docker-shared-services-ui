package docker

import (
	"context"
	"fmt"
	"time"
)

const (
	// DefaultReadyTimeout is the default maximum wait for a dependency or one-shot container.
	DefaultReadyTimeout = 300 * time.Second
	// DefaultPollInterval is the default polling interval for readiness checks.
	DefaultPollInterval = 2 * time.Second
)

// IsContainerReady reports whether a single container is running & healthy (or completed one-shot).
func IsContainerReady(c ContainerInfo) bool {
	if c.Status == StateRunning {
		return c.Health == HealthNone || c.Health == HealthHealthy
	}
	return c.Completed
}

// ServiceContainersReady reports whether a service has at least one container and all are ready.
func ServiceContainersReady(ctx context.Context, inspector ContainerInspector, prefix, service string) (hasContainers, ready bool, err error) {
	statuses, err := ListServiceStatuses(ctx, inspector, []string{service}, prefix)
	if err != nil {
		return false, false, err
	}
	if len(statuses) == 0 || len(statuses[0].Containers) == 0 {
		return false, false, nil
	}
	for _, c := range statuses[0].Containers {
		if !IsContainerReady(c) {
			return true, false, nil
		}
	}
	return true, true, nil
}

// WaitUntilReady blocks until every container of service is ready, ctx is cancelled, or timeout elapses.
func WaitUntilReady(
	ctx context.Context,
	inspector ContainerInspector,
	prefix, service string,
	timeout, poll time.Duration,
	say func(string),
) error {
	has, ready, err := ServiceContainersReady(ctx, inspector, prefix, service)
	if err != nil {
		return err
	}
	if !has {
		emit(say, fmt.Sprintf("%s has no containers", service))
		return fmt.Errorf("%s has no containers", service)
	}
	if ready {
		return nil
	}

	emit(say, fmt.Sprintf("Waiting until %s is ready...", service))
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := sleepWithContext(ctx, poll); err != nil {
			return err
		}
		_, ready, err = ServiceContainersReady(ctx, inspector, prefix, service)
		if err != nil {
			return err
		}
		if ready {
			emit(say, fmt.Sprintf("%s is ready", service))
			return nil
		}
	}

	emit(say, fmt.Sprintf("%s was not ready within %s", service, timeout.Round(time.Second)))
	return fmt.Errorf("%s was not ready within %s", service, timeout.Round(time.Second))
}

type oneshotState int

const (
	oneshotDone oneshotState = iota
	oneshotPending
	oneshotFailed
)

func classifyServiceOneshots(ctx context.Context, inspector ContainerInspector, prefix, service string) (oneshotState, error) {
	statuses, err := ListServiceStatuses(ctx, inspector, []string{service}, prefix)
	if err != nil {
		return oneshotPending, err
	}
	if len(statuses) == 0 {
		return oneshotPending, nil
	}
	pending := false
	for _, c := range statuses[0].Containers {
		if c.RestartPolicy != "" && c.RestartPolicy != "no" {
			continue
		}
		if c.Status == "exited" {
			if !c.Completed {
				return oneshotFailed, nil
			}
			continue
		}
		pending = true
	}
	if pending {
		return oneshotPending, nil
	}
	return oneshotDone, nil
}

// WaitUntilOneshotsDone blocks until all restart:no containers for service exit 0, fail, or timeout.
func WaitUntilOneshotsDone(
	ctx context.Context,
	inspector ContainerInspector,
	prefix, service string,
	timeout, poll time.Duration,
	say func(string),
) error {
	deadline := time.Now().Add(timeout)
	for {
		state, err := classifyServiceOneshots(ctx, inspector, prefix, service)
		if err != nil {
			return err
		}
		switch state {
		case oneshotDone:
			return nil
		case oneshotFailed:
			emit(say, fmt.Sprintf("%s one-shot exited with an error", service))
			return fmt.Errorf("%s one-shot exited with an error", service)
		case oneshotPending:
			if !time.Now().Before(deadline) {
				emit(say, fmt.Sprintf("%s one-shot did not exit 0 within %s", service, timeout.Round(time.Second)))
				return fmt.Errorf("%s one-shot did not exit 0 within %s", service, timeout.Round(time.Second))
			}
			if err := sleepWithContext(ctx, poll); err != nil {
				return err
			}
		}
	}
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
