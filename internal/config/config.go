// Package config resolves and validates dashboard runtime configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultAddr is the default localhost listen address for the dashboard server.
	DefaultAddr = "127.0.0.1:8787"
	// EnvRepoPath is the environment variable used when --repo is not set.
	EnvRepoPath = "DSS_REPO_PATH"
)

// Config holds resolved runtime configuration for the dashboard.
type Config struct {
	RepoPath string
	Addr     string
}

// Resolve builds and validates Config from CLI flags and environment variables.
func Resolve(repoFlag, addrFlag string) (Config, error) {
	repo := repoFlag
	if repo == "" {
		repo = os.Getenv(EnvRepoPath)
	}
	if repo == "" {
		return Config{}, errors.New("repository path is required (--repo or DSS_REPO_PATH)")
	}

	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return Config{}, fmt.Errorf("resolve repository path %q: %w", repo, err)
	}

	info, err := os.Stat(absRepo) //nolint:gosec // absRepo is an explicit CLI/env repository root path
	if err != nil {
		return Config{}, fmt.Errorf("stat repository path %q: %w", absRepo, err)
	}
	if !info.IsDir() {
		return Config{}, fmt.Errorf("repository path %q is not a directory", absRepo)
	}

	addr := addrFlag
	if addr == "" {
		addr = DefaultAddr
	}

	return Config{
		RepoPath: absRepo,
		Addr:     addr,
	}, nil
}
