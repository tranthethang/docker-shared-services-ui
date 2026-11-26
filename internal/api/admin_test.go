package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"

	"github.com/tranthethang/docker-shared-services-ui/internal/catalog"
	"github.com/tranthethang/docker-shared-services-ui/internal/ops"
	"github.com/tranthethang/docker-shared-services-ui/internal/server"
	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

type fakeAdminDocker struct{}

func (fakeAdminDocker) ContainerList(context.Context, container.ListOptions) ([]container.Summary, error) {
	return nil, nil
}
func (fakeAdminDocker) ContainerInspect(context.Context, string) (container.InspectResponse, error) {
	return container.InspectResponse{}, nil
}
func (fakeAdminDocker) NetworkInspect(context.Context, string, network.InspectOptions) (network.Inspect, error) {
	return network.Inspect{}, nil
}
func (fakeAdminDocker) NetworkCreate(context.Context, string, network.CreateOptions) (network.CreateResponse, error) {
	return network.CreateResponse{}, nil
}
func (fakeAdminDocker) ContainersPrune(context.Context, filters.Args) (container.PruneReport, error) {
	return container.PruneReport{}, nil
}
func (fakeAdminDocker) ImagesPrune(context.Context, filters.Args) (image.PruneReport, error) {
	return image.PruneReport{}, nil
}
func (fakeAdminDocker) NetworksPrune(context.Context, filters.Args) (network.PruneReport, error) {
	return network.PruneReport{}, nil
}
func (fakeAdminDocker) VolumesPrune(context.Context, filters.Args) (volume.PruneReport, error) {
	return volume.PruneReport{}, nil
}

type fakeAdminCompose struct{}

func (fakeAdminCompose) RunCompose(context.Context, string, string, string, []string, func(string)) error {
	return nil
}

func newAdminTestServer(t *testing.T, dir string) http.Handler {
	t.Helper()
	id := stack.Identity{
		Name:     "mystack",
		Prefix:   "mystack-",
		Networks: []stack.NetworkPair{{Name: "mystack_traefik_proxy", Subnet: "10.0.0.0/16"}},
	}
	cat := &catalog.Loaded{
		Domain:      "sandbox.localhost",
		ActiveNames: []string{"traefik", "postgres", "zitadel"},
	}
	runner := ops.NewRunner(ops.Config{
		RepoPath:       dir,
		Identity:       id,
		ActiveServices: cat.ActiveNames,
		Docker:         fakeAdminDocker{},
		Compose:        fakeAdminCompose{},
	})
	h := &AdminHandler{
		RepoPath: dir,
		Catalog:  cat,
		Identity: id,
		Docker:   fakeAdminDocker{},
		Runner:   runner,
	}
	return server.NewHandlerWithAPIs(nil, h.Register)
}

func TestEnvStatusNeverExposesSecrets(t *testing.T) {
	dir := t.TempDir()
	secretVal := "SuperSecretPasswordValue999!"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DSS_SHARED_PASSWORD="+secretVal+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	pgDir := filepath.Join(dir, "postgres")
	if err := os.MkdirAll(pgDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pgDir, ".env"), []byte("POSTGRES16_PASSWORD="+secretVal+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	srv := newAdminTestServer(t, dir)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/env/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secretVal) {
		t.Fatal("raw secret value leaked in /api/env/status response")
	}
}

func TestDestructiveEndpointsRequireConfirmation(t *testing.T) {
	dir := t.TempDir()
	srv := newAdminTestServer(t, dir)

	for _, path := range []string{"/api/maintenance/prune", "/api/maintenance/teardown"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{"confirmStackName":"wrong"}`))
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400 on mismatch, got %d", path, rec.Code)
		}

		rec = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(`{}`))
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400 on missing confirm, got %d", path, rec.Code)
		}
	}
}

func TestExportPreviewAndExportEndpoints(t *testing.T) {
	dir := t.TempDir()
	for _, svc := range []string{"traefik", "postgres", "zitadel"} {
		svcDir := filepath.Join(dir, svc)
		if err := os.MkdirAll(svcDir, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(svcDir, "docker-compose.yml"), []byte("services: {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	srv := newAdminTestServer(t, dir)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/export/preview", bytes.NewBufferString(`{"services":["zitadel"]}`))
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var preview struct {
		Added []string `json:"added"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Added) != 1 || preview.Added[0] != "postgres" {
		t.Fatalf("expected postgres auto-added, got %v", preview.Added)
	}
}
