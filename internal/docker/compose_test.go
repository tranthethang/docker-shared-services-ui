package docker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/network"

	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

type fakeNetworkManager struct {
	existing map[string]bool
	created  []networkCreateCall
	errFn    func(name string, opts network.CreateOptions) error
}

type networkCreateCall struct {
	name   string
	subnet string
}

func (f *fakeNetworkManager) NetworkInspect(_ context.Context, networkID string, _ network.InspectOptions) (network.Inspect, error) {
	if f.existing[networkID] {
		return network.Inspect{Name: networkID}, nil
	}
	return network.Inspect{}, errors.New("not found")
}

func (f *fakeNetworkManager) NetworkCreate(_ context.Context, name string, opts network.CreateOptions) (network.CreateResponse, error) {
	if f.errFn != nil {
		if err := f.errFn(name, opts); err != nil {
			return network.CreateResponse{}, err
		}
	}
	subnet := ""
	if opts.IPAM != nil && len(opts.IPAM.Config) > 0 {
		subnet = opts.IPAM.Config[0].Subnet
	}
	f.created = append(f.created, networkCreateCall{name: name, subnet: subnet})
	if f.existing == nil {
		f.existing = make(map[string]bool)
	}
	f.existing[name] = true
	return network.CreateResponse{ID: name}, nil
}

func TestEnsureNetworks(t *testing.T) {
	pairs := []stack.NetworkPair{
		{Name: "dev_traefik_proxy", Subnet: "10.10.0.0/16"},
		{Name: "dev_backend_net", Subnet: "10.11.0.0/16"},
		{Name: "dev_data_net", Subnet: "10.12.0.0/16"},
	}

	t.Run("creates missing and falls back on overlap", func(t *testing.T) {
		mgr := &fakeNetworkManager{
			existing: map[string]bool{"dev_traefik_proxy": true},
			errFn: func(name string, opts network.CreateOptions) error {
				if name == "dev_backend_net" && opts.IPAM != nil {
					return errors.New("Pool overlaps with other one on this address space")
				}
				return nil
			},
		}

		var logs []string
		err := EnsureNetworks(context.Background(), mgr, pairs, func(msg string) {
			logs = append(logs, msg)
		})
		if err != nil {
			t.Fatalf("EnsureNetworks() error = %v", err)
		}

		want := []networkCreateCall{
			{name: "dev_backend_net", subnet: ""},
			{name: "dev_data_net", subnet: "10.12.0.0/16"},
		}
		if !reflect.DeepEqual(mgr.created, want) {
			t.Fatalf("created = %+v, want %+v", mgr.created, want)
		}
		if len(logs) == 0 {
			t.Fatal("expected progress log lines")
		}
	})

	t.Run("returns non-overlap error", func(t *testing.T) {
		mgr := &fakeNetworkManager{
			errFn: func(_ string, _ network.CreateOptions) error {
				return errors.New("permission denied")
			},
		}
		if err := EnsureNetworks(context.Background(), mgr, pairs[:1], nil); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestBuildComposeArgs(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, ".env"), "DOMAIN=sandbox.localhost\n")
	mustWriteFile(t, filepath.Join(dir, "traefik", ".env"), "TRAEFIK_HTTP_PORT=80\n")

	args := BuildComposeArgs(dir, "dev-traefik", "traefik", "up", "-d")
	want := []string{
		"compose",
		"-p", "dev-traefik",
		"--project-directory", "traefik",
		"--env-file", ".env",
		"--env-file", "traefik/.env",
		"-f", "docker-compose.shared.yml",
		"-f", "traefik/docker-compose.yml",
		"-f", "traefik/docker-compose.publish.yml",
		"up", "-d",
	}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("BuildComposeArgs() = %v, want %v", args, want)
	}
}

func TestExecComposeRunnerStreamsAndCancels(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-docker.sh")
	mustWriteFile(t, script, "#!/bin/sh\npwd\necho \"out:$*\"\necho \"err:warn\" >&2\n")
	if err := os.Chmod(script, 0o755); err != nil { //nolint:gosec // test helper script must be executable
		t.Fatal(err)
	}

	runner := ExecComposeRunner{CommandName: script}
	var lines []string
	if err := runner.RunCompose(context.Background(), dir, "dev-redis", "redis", []string{"up", "-d"}, func(line string) {
		lines = append(lines, line)
	}); err != nil {
		t.Fatalf("RunCompose() error = %v", err)
	}

	if !slices.Contains(lines, "err:warn") {
		t.Fatalf("expected stderr line in %v", lines)
	}
	foundOut := false
	for _, l := range lines {
		if strings.HasPrefix(l, "out:compose -p dev-redis") {
			foundOut = true
		}
	}
	if !foundOut {
		t.Fatalf("expected compose args output in %v", lines)
	}

	slowScript := filepath.Join(dir, "slow-docker.sh")
	mustWriteFile(t, slowScript, "#!/bin/sh\nsleep 30\n")
	if err := os.Chmod(slowScript, 0o755); err != nil { //nolint:gosec // test helper script must be executable
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := (ExecComposeRunner{CommandName: slowScript}).RunCompose(ctx, dir, "", "redis", []string{"up", "-d"}, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
