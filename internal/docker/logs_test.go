package docker

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
)

type fakeLogStreamer struct {
	mockInspector
	logsFn func(ctx context.Context, id string, opts container.LogsOptions) (io.ReadCloser, error)
}

func (f *fakeLogStreamer) ContainerLogs(ctx context.Context, id string, opts container.LogsOptions) (io.ReadCloser, error) {
	return f.logsFn(ctx, id, opts)
}

func muxFrame(streamType byte, payload string) []byte {
	var buf bytes.Buffer
	header := [8]byte{streamType}
	binary.BigEndian.PutUint32(header[4:], uint32(len(payload))) //nolint:gosec // test payload length fits in uint32
	buf.Write(header[:])
	buf.WriteString(payload)
	return buf.Bytes()
}

type blockingReadCloser struct {
	pr     *io.PipeReader
	closed atomic.Bool
}

func (b *blockingReadCloser) Read(p []byte) (int, error) {
	return b.pr.Read(p)
}

func (b *blockingReadCloser) Close() error {
	b.closed.Store(true)
	return b.pr.Close()
}

func TestStreamServiceLogsMasksSecretsAndDemuxes(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, ".env"), "GLOBAL_SECRET=super-secret-value\n")
	mustWriteFile(t, filepath.Join(dir, "redis", ".env"), "REDIS_PASSWORD=redis-pw\n")

	var payload bytes.Buffer
	payload.Write(muxFrame(1, "2026-11-26T00:00:00Z connecting with super-secret-value\n"))
	payload.Write(muxFrame(2, "2026-11-26T00:00:01Z auth failed for redis-pw\n"))

	streamer := &fakeLogStreamer{
		mockInspector: mockInspector{
			summaries: []container.Summary{
				{ID: "c1", Labels: map[string]string{ProjectLabel: "dev-redis"}},
			},
			inspects: map[string]container.InspectResponse{
				"c1": makeInspect("c1", "redis", "running", "healthy", "dev-redis", "redis", "unless-stopped", 0),
			},
		},
		logsFn: func(_ context.Context, _ string, _ container.LogsOptions) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(payload.Bytes())), nil
		},
	}

	outCh, errCh := StreamServiceLogs(context.Background(), streamer, LogStreamOptions{
		RepoPath: dir,
		Prefix:   "dev-",
		Service:  "redis",
		Follow:   false,
	})

	var got []LogLine
	for line := range outCh {
		got = append(got, line)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2: %+v", len(got), got)
	}
	for _, l := range got {
		if l.Container != "redis" {
			t.Errorf("unexpected container name %q", l.Container)
		}
		if bytes.Contains([]byte(l.Line), []byte("super-secret-value")) || bytes.Contains([]byte(l.Line), []byte("redis-pw")) {
			t.Errorf("unmasked secret in line: %q", l.Line)
		}
	}
}

func TestStreamServiceLogsClosesOnCancel(t *testing.T) {
	pr, pw := io.Pipe()
	defer func() { _ = pw.Close() }()
	rc := &blockingReadCloser{pr: pr}

	streamer := &fakeLogStreamer{
		mockInspector: mockInspector{
			summaries: []container.Summary{
				{ID: "c1", Labels: map[string]string{ProjectLabel: "dev-redis"}},
			},
			inspects: map[string]container.InspectResponse{
				"c1": makeInspect("c1", "redis", "running", "healthy", "dev-redis", "redis", "unless-stopped", 0),
			},
		},
		logsFn: func(_ context.Context, _ string, _ container.LogsOptions) (io.ReadCloser, error) {
			return rc, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	outCh, errCh := StreamServiceLogs(ctx, streamer, LogStreamOptions{
		Prefix:  "dev-",
		Service: "redis",
		Follow:  true,
	})

	cancel()
	select {
	case _, ok := <-outCh:
		if ok {
			t.Fatal("expected outCh to close on cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for log stream to close")
	}
	<-errCh
	if !rc.closed.Load() {
		t.Fatal("expected underlying ReadCloser to be closed on cancellation")
	}
}

func TestStreamServiceLogsNoMatchingContainer(t *testing.T) {
	streamer := &fakeLogStreamer{
		mockInspector: mockInspector{
			summaries: []container.Summary{},
		},
	}
	outCh, errCh := StreamServiceLogs(context.Background(), streamer, LogStreamOptions{
		Prefix:  "dev-",
		Service: "redis",
	})
	for range outCh {
	}
	err := <-errCh
	if !errors.Is(err, ErrNoMatchingContainers) {
		t.Fatalf("got err=%v, want ErrNoMatchingContainers", err)
	}
}
