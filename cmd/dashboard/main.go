// Package main runs the docker-shared-services-ui local dashboard server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tranthethang/docker-shared-services-ui/internal/api"
	"github.com/tranthethang/docker-shared-services-ui/internal/catalog"
	"github.com/tranthethang/docker-shared-services-ui/internal/config"
	"github.com/tranthethang/docker-shared-services-ui/internal/docker"
	"github.com/tranthethang/docker-shared-services-ui/internal/ops"
	"github.com/tranthethang/docker-shared-services-ui/internal/server"
	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
	"github.com/tranthethang/docker-shared-services-ui/web"
)

const shutdownTimeout = 5 * time.Second

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	addrFlag := flag.String("addr", config.DefaultAddr, "HTTP listen address")
	repoFlag := flag.String("repo", "", "Path to docker-shared-services repository")
	noOpen := flag.Bool("no-open", false, "Do not open browser on startup")
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("dashboard %s (commit %s, built %s)\n", version, commit, buildDate)
		return nil
	}

	cfg, err := config.Resolve(*repoFlag, *addrFlag)
	if err != nil {
		return err
	}

	identity, err := stack.ParseIdentity(cfg.RepoPath)
	if err != nil {
		return err
	}

	loadedCatalog, err := catalog.Load(cfg.RepoPath)
	if err != nil {
		return err
	}

	dockerClient, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	defer func() {
		_ = dockerClient.Close()
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runner := ops.NewRunner(ops.Config{
		RepoPath:       cfg.RepoPath,
		Identity:       identity,
		ActiveServices: loadedCatalog.ActiveNames,
		Docker:         dockerClient,
	})

	roHandler := &api.ReadOnlyHandler{
		Catalog:  loadedCatalog,
		Identity: identity,
		Docker:   dockerClient,
	}
	opsHandler := &api.OpsHandler{
		Runner: runner,
		AppCtx: ctx,
	}
	adminHandler := &api.AdminHandler{
		RepoPath: cfg.RepoPath,
		Catalog:  loadedCatalog,
		Identity: identity,
		Docker:   dockerClient,
		Runner:   runner,
		AppCtx:   ctx,
	}

	srv := server.New(server.Config{
		Addr:          cfg.Addr,
		StaticFS:      web.DistFS(),
		RegisterAPI:   roHandler.Register,
		RegisterOps:   opsHandler.Register,
		RegisterAdmin: adminHandler.Register,
	})

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.Addr, err)
	}

	url := config.BrowserURL(ln.Addr().String())
	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("dashboard listening on %s\n", url)
		if serveErr := srv.Serve(ln); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
		}
		close(errCh)
	}()

	if !*noOpen {
		go func() {
			if openErr := config.OpenBrowser(ctx, url); openErr != nil {
				fmt.Fprintf(os.Stderr, "warning: open browser: %v\n", openErr)
			}
		}()
	}

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown error: %w", err)
		}
		return nil
	}
}
