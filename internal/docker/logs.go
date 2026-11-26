package docker

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/tranthethang/docker-shared-services-ui/internal/env"
)

const defaultLogTail = "200"

// ErrNoMatchingContainers is returned when a service has no containers matching the log filter.
var ErrNoMatchingContainers = errors.New("no matching containers for service")

// LogStreamer is the subset of Docker SDK methods needed to inspect and stream container logs.
type LogStreamer interface {
	ContainerInspector
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
}

// LogLine represents a single masked log line from a service container.
type LogLine struct {
	Container string `json:"container"`
	Stream    string `json:"stream"`
	Line      string `json:"line"`
}

// LogStreamOptions configures container log streaming for a service.
type LogStreamOptions struct {
	RepoPath  string
	Prefix    string
	Service   string
	Container string
	Tail      string
	Follow    bool
}

// StreamServiceLogs streams masked logs from all (or one filtered) containers of a service.
func StreamServiceLogs(ctx context.Context, cli LogStreamer, opts LogStreamOptions) (<-chan LogLine, <-chan error) {
	outCh := make(chan LogLine, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(outCh)
		defer close(errCh)

		targets, err := resolveLogTargets(ctx, cli, opts)
		if err != nil {
			sendLogErr(ctx, errCh, err)
			return
		}

		tail := opts.Tail
		if tail == "" {
			tail = defaultLogTail
		}
		maskValues := env.ValuesToMask(opts.RepoPath, []string{opts.Service})

		var wg sync.WaitGroup
		var firstErrMu sync.Mutex
		var firstErr error
		setErr := func(e error) {
			if e == nil || errors.Is(e, context.Canceled) {
				return
			}
			firstErrMu.Lock()
			if firstErr == nil {
				firstErr = e
			}
			firstErrMu.Unlock()
		}

		for _, target := range targets {
			wg.Add(1)
			go func(t logTarget) {
				defer wg.Done()
				if err := streamSingleContainer(ctx, cli, t, tail, opts.Follow, maskValues, outCh); err != nil {
					setErr(err)
				}
			}(target)
		}
		wg.Wait()

		if firstErr != nil {
			sendLogErr(ctx, errCh, firstErr)
		}
	}()

	return outCh, errCh
}

type logTarget struct {
	id   string
	name string
	tty  bool
}

func resolveLogTargets(ctx context.Context, cli LogStreamer, opts LogStreamOptions) ([]logTarget, error) {
	statuses, err := ListServiceStatuses(ctx, cli, []string{opts.Service}, opts.Prefix)
	if err != nil {
		return nil, err
	}
	if len(statuses) == 0 || len(statuses[0].Containers) == 0 {
		return nil, ErrNoMatchingContainers
	}

	var targets []logTarget
	for _, c := range statuses[0].Containers {
		if opts.Container != "" && c.Name != opts.Container && c.ID != opts.Container && !strings.HasPrefix(c.ID, opts.Container) {
			continue
		}
		tty := false
		if insp, err := cli.ContainerInspect(ctx, c.ID); err == nil && insp.Config != nil {
			tty = insp.Config.Tty
		}
		targets = append(targets, logTarget{id: c.ID, name: c.Name, tty: tty})
	}
	if len(targets) == 0 {
		return nil, ErrNoMatchingContainers
	}
	return targets, nil
}

func streamSingleContainer(
	ctx context.Context,
	cli LogStreamer,
	target logTarget,
	tail string,
	follow bool,
	maskValues []string,
	outCh chan<- LogLine,
) error {
	rc, err := cli.ContainerLogs(ctx, target.id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Tail:       tail,
		Timestamps: true,
	})
	if err != nil {
		return fmt.Errorf("container logs %s: %w", target.name, err)
	}

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = rc.Close()
		case <-done:
			_ = rc.Close()
		}
	}()

	if target.tty {
		return scanLogReader(ctx, rc, target.name, "stdout", maskValues, outCh)
	}

	outR, outW := io.Pipe()
	errR, errW := io.Pipe()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = scanLogReader(ctx, outR, target.name, "stdout", maskValues, outCh)
	}()
	go func() {
		defer wg.Done()
		_ = scanLogReader(ctx, errR, target.name, "stderr", maskValues, outCh)
	}()

	_, copyErr := stdcopy.StdCopy(outW, errW, rc)
	_ = outW.Close()
	_ = errW.Close()
	wg.Wait()

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if copyErr != nil && !errors.Is(copyErr, io.EOF) {
		return fmt.Errorf("demux logs %s: %w", target.name, copyErr)
	}
	return nil
}

func scanLogReader(
	ctx context.Context,
	r io.Reader,
	containerName, streamName string,
	maskValues []string,
	outCh chan<- LogLine,
) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, scanInitBufSize), scanMaxBufSize)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		masked := env.MaskKnownValues(scanner.Text(), maskValues)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case outCh <- LogLine{
			Container: containerName,
			Stream:    streamName,
			Line:      masked,
		}:
		}
	}
	return scanner.Err()
}

func sendLogErr(ctx context.Context, errCh chan<- error, err error) {
	select {
	case <-ctx.Done():
	case errCh <- err:
	}
}
