package exporter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

func TestValidateStackNameRejectsTraversal(t *testing.T) {
	for _, bad := range []string{"", "../evil", "a/b", "a\\b", "Upper", "1bad", "a..b"} {
		if err := ValidateStackName(bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
	for _, good := range []string{"demo", "my-stack_1", "core"} {
		if err := ValidateStackName(good); err != nil {
			t.Fatalf("expected %q to be valid, got %v", good, err)
		}
	}
}

func TestExportExpandsZitadelDependenciesAndWritesManifest(t *testing.T) {
	dir := t.TempDir()
	for _, svc := range []string{"traefik", "postgres", "zitadel"} {
		svcDir := filepath.Join(dir, svc)
		if err := os.MkdirAll(svcDir, 0o750); err != nil {
			t.Fatal(err)
		}
		compose := "services: {}\n"
		if svc == "zitadel" {
			compose = "services:\n  zitadel:\n    labels:\n      - traefik.enable=true\n"
		}
		if err := os.WriteFile(filepath.Join(svcDir, "docker-compose.yml"), []byte(compose), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(svcDir, ".env"), []byte("SECRET=do-not-copy\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.shared.yml"), []byte("networks: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env.example"), []byte("DOMAIN=sandbox.localhost\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := Export(dir, "auth-stack", []string{"zitadel"}, []string{"traefik", "postgres", "zitadel", "redis"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(res.Services, "postgres") || !slices.Contains(res.Services, "traefik") || !slices.Contains(res.Services, "zitadel") {
		t.Fatalf("expected postgres, traefik, zitadel in resolved services, got %v", res.Services)
	}
	if _, err := os.Stat(filepath.Join(res.OutputPath, "zitadel", ".env")); !os.IsNotExist(err) {
		t.Fatal("expected .env to be excluded from export")
	}

	raw, err := os.ReadFile(filepath.Join(res.OutputPath, stack.ManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Services []string `json:"services"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(manifest.Services, res.Services) {
		t.Fatalf("manifest mismatch: %v vs %v", manifest.Services, res.Services)
	}
}
