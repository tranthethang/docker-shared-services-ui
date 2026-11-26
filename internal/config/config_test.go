package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolve(t *testing.T) {
	repoDir := t.TempDir()

	t.Run("flag overrides env and uses default addr", func(t *testing.T) {
		t.Setenv(EnvRepoPath, "/nonexistent/path")
		cfg, err := Resolve(repoDir, "")
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if cfg.RepoPath != repoDir {
			t.Fatalf("RepoPath = %q, want %q", cfg.RepoPath, repoDir)
		}
		if cfg.Addr != DefaultAddr {
			t.Fatalf("Addr = %q, want %q", cfg.Addr, DefaultAddr)
		}
	})

	t.Run("falls back to env and custom addr", func(t *testing.T) {
		t.Setenv(EnvRepoPath, repoDir)
		cfg, err := Resolve("", "127.0.0.1:9090")
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if cfg.RepoPath != repoDir {
			t.Fatalf("RepoPath = %q, want %q", cfg.RepoPath, repoDir)
		}
		if cfg.Addr != "127.0.0.1:9090" {
			t.Fatalf("Addr = %q, want 127.0.0.1:9090", cfg.Addr)
		}
	})

	t.Run("errors when missing repo", func(t *testing.T) {
		t.Setenv(EnvRepoPath, "")
		if _, err := Resolve("", ""); err == nil {
			t.Fatal("expected error when repo is empty")
		}
	})

	t.Run("errors when repo does not exist", func(t *testing.T) {
		if _, err := Resolve(filepath.Join(repoDir, "missing"), ""); err == nil {
			t.Fatal("expected error for missing directory")
		}
	})

	t.Run("errors when repo path is a file", func(t *testing.T) {
		filePath := filepath.Join(repoDir, "file.txt")
		if err := os.WriteFile(filePath, []byte("hi"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Resolve(filePath, ""); err == nil {
			t.Fatal("expected error when repo path is a file")
		}
	})
}
