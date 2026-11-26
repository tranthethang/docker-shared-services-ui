package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

const sampleCatalogJSON = `{
  "adminer": {
    "category": "Admin UI",
    "description": "Web database admin.",
    "containers": ["adminer"],
    "web_ui": ["https://adminer.{domain}"],
    "info": ["Adminer - https://adminer.{domain}"]
  },
  "postgres": {
    "category": "Database",
    "description": "PostgreSQL database.",
    "containers": ["postgres"],
    "web_ui": [],
    "info": ["Host: postgres.{domain}"]
  },
  "traefik": {
    "category": "Auth & Gateway",
    "description": "Edge proxy.",
    "containers": ["traefik"],
    "web_ui": ["https://traefik.{domain}"],
    "info": []
  }
}`

func TestLoadAndDomainSubstitution(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "cli"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, CatalogRelPath), []byte(sampleCatalogJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, stack.RootEnvFile), []byte("DOMAIN=example.test\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Domain != "example.test" {
		t.Fatalf("Domain = %q, want example.test", loaded.Domain)
	}
	if len(loaded.Categories) != 10 || loaded.Categories[0] != "Database" || loaded.Categories[9] != "Dev Tools" {
		t.Fatalf("unexpected categories: %v", loaded.Categories)
	}

	adminer := loaded.Entries["adminer"]
	if adminer.Name != "adminer" {
		t.Fatalf("adminer.Name = %q, want adminer", adminer.Name)
	}
	if !reflect.DeepEqual(adminer.WebUI, []string{"https://adminer.example.test"}) {
		t.Fatalf("adminer.WebUI = %v", adminer.WebUI)
	}
	for name, entry := range loaded.Entries {
		for _, item := range append(append([]string{}, entry.WebUI...), entry.Info...) {
			if strings.Contains(item, "{domain}") {
				t.Fatalf("%s still contains {domain}: %q", name, item)
			}
		}
	}
}

func TestLoadDomainFallbackAndManifestFiltering(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "cli"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, CatalogRelPath), []byte(sampleCatalogJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, stack.ManifestFile), []byte(`{"services":["traefik","postgres"]}`), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Domain != DefaultDomain {
		t.Fatalf("Domain = %q, want %q", loaded.Domain, DefaultDomain)
	}
	if !reflect.DeepEqual(loaded.ActiveNames, []string{"postgres", "traefik"}) {
		t.Fatalf("ActiveNames = %v", loaded.ActiveNames)
	}
	if _, ok := loaded.Entries["adminer"]; ok {
		t.Fatal("expected adminer to be filtered out by dss-package.json")
	}

	presets := FilterPresets(loaded.ActiveNames)
	if len(presets) != 4 {
		t.Fatalf("len(presets) = %d, want 4", len(presets))
	}
	if presets[0].Name != "Core Infra" {
		t.Fatalf("presets[0].Name = %q, want Core Infra", presets[0].Name)
	}
	if !reflect.DeepEqual(presets[0].Services, []string{"traefik", "postgres"}) {
		t.Fatalf("Core Infra services = %v, want [traefik postgres]", presets[0].Services)
	}
}
