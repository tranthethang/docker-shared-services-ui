package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tranthethang/docker-shared-services-ui/internal/server"
)

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()

	handler := server.NewHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json content-type, got %q", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %q", body["status"])
	}
}

func TestSPAFallbackAndStaticAssets(t *testing.T) {
	t.Parallel()

	staticFS := fstest.MapFS{
		"index.html":        {Data: []byte("<!doctype html><html><body>dashboard</body></html>")},
		"assets/app.js":     {Data: []byte("console.log('ok');")},
		"assets/styles.css": {Data: []byte("body { margin: 0; }")},
	}
	handler := server.NewHandler(staticFS)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "root serves index.html",
			path:       "/",
			wantStatus: http.StatusOK,
			wantBody:   "dashboard",
		},
		{
			name:       "existing asset serves asset",
			path:       "/assets/app.js",
			wantStatus: http.StatusOK,
			wantBody:   "console.log('ok');",
		},
		{
			name:       "unknown route falls back to index.html",
			path:       "/services/postgres",
			wantStatus: http.StatusOK,
			wantBody:   "dashboard",
		},
		{
			name:       "unknown api route returns 404",
			path:       "/api/unknown",
			wantStatus: http.StatusNotFound,
			wantBody:   "404",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("path %s: expected status %d, got %d", tc.path, tc.wantStatus, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("path %s: expected body to contain %q, got %q", tc.path, tc.wantBody, rec.Body.String())
			}
		})
	}
}
