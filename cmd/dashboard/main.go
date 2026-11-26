// Package main runs the docker-shared-services-ui local dashboard server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tranthethang/docker-shared-services-ui/internal/server"
	"github.com/tranthethang/docker-shared-services-ui/web"
)

const (
	defaultAddr     = "127.0.0.1:8787"
	shutdownTimeout = 5 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", defaultAddr, "HTTP listen address")
	repo := flag.String("repo", "", "Path to docker-shared-services repository")
	noOpen := flag.Bool("no-open", false, "Do not open browser on startup")
	flag.Parse()

	_ = repo
	_ = noOpen

	srv := server.New(server.Config{
		Addr:     *addr,
		StaticFS: web.DistFS(),
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		fmt.Printf("dashboard listening on http://%s\n", *addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

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
