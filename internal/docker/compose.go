package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

const (
	// TraefikPublishFile is the optional compose override enabling direct HTTP/HTTPS port publishing for Traefik.
	TraefikPublishFile = "traefik/docker-compose.publish.yml"
	sharedComposeFile  = "docker-compose.shared.yml"
	scanInitBufSize    = 64 * 1024
	scanMaxBufSize     = 1024 * 1024
)

// ComposeRunner executes docker compose commands for a service and streams output lines.
type ComposeRunner interface {
	RunCompose(ctx context.Context, repoPath, project, service string, extra []string, onLine func(string)) error
}

// ExecComposeRunner runs docker compose via os/exec.CommandContext.
type ExecComposeRunner struct {
	CommandName string
}

// BuildComposeArgs builds the `docker compose` argument slice for a service.
func BuildComposeArgs(repoPath, project, service string, extra ...string) []string {
	args := []string{"compose"}
	if project != "" {
		args = append(args, "-p", project)
	}
	args = append(args, "--project-directory", service)

	for _, rel := range envFileArgs(repoPath, service) {
		args = append(args, "--env-file", rel)
	}

	args = append(args, "-f", sharedComposeFile, "-f", filepath.ToSlash(filepath.Join(service, "docker-compose.yml")))
	if service == "traefik" && hasTraefikPublishPort(repoPath) {
		args = append(args, "-f", TraefikPublishFile)
	}
	args = append(args, extra...)
	return args
}

func envFileArgs(repoPath, service string) []string {
	candidates := []string{
		stack.RootEnvFile,
		filepath.ToSlash(filepath.Join(service, ".env")),
	}
	out := make([]string, 0, len(candidates))
	for _, rel := range candidates {
		info, err := os.Stat(filepath.Join(repoPath, rel))
		if err == nil && !info.IsDir() {
			out = append(out, rel)
		}
	}
	return out
}

func hasTraefikPublishPort(repoPath string) bool {
	val, ok := stack.ReadEnvValue(filepath.Join(repoPath, "traefik", ".env"), "TRAEFIK_HTTP_PORT")
	return ok && val != ""
}

// RunCompose executes `docker compose` in repoPath and streams combined stdout/stderr lines to onLine.
func (r ExecComposeRunner) RunCompose(ctx context.Context, repoPath, project, service string, extra []string, onLine func(string)) error {
	bin := r.CommandName
	if bin == "" {
		bin = "docker"
	}
	args := BuildComposeArgs(repoPath, project, service, extra...)
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // bin and args are constructed from validated config and service names
	cmd.Dir = repoPath

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s compose: %w", bin, err)
	}

	var mu sync.Mutex
	safeLine := func(line string) {
		if onLine == nil {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		onLine(line)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go scanPipe(stdout, safeLine, &wg)
	go scanPipe(stderr, safeLine, &wg)
	wg.Wait()

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("docker compose %s: %w", service, err)
	}
	return nil
}

func scanPipe(r io.Reader, onLine func(string), wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, scanInitBufSize), scanMaxBufSize)
	for scanner.Scan() {
		onLine(scanner.Text())
	}
}
