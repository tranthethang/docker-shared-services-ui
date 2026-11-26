// Package server provides the HTTP server and SPA static asset handler for the dashboard.
package server

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

const (
	defaultReadHeaderTimeout = 5 * time.Second
	indexFileName            = "index.html"
)

// Config holds configuration for the dashboard HTTP server.
type Config struct {
	Addr        string
	StaticFS    fs.FS
	RegisterAPI func(mux *http.ServeMux)
	RegisterOps func(mux *http.ServeMux)
}

// New creates an *http.Server configured with the dashboard routes and timeouts.
func New(cfg Config) *http.Server {
	return &http.Server{
		Addr:              cfg.Addr,
		Handler:           NewHandlerWithAPIs(cfg.StaticFS, cfg.RegisterAPI, cfg.RegisterOps),
		ReadHeaderTimeout: defaultReadHeaderTimeout,
	}
}

// NewHandler constructs the HTTP handler with /api/health and SPA fallback routes.
func NewHandler(staticFS fs.FS) http.Handler {
	return NewHandlerWithAPIs(staticFS)
}

// NewHandlerWithAPI constructs the HTTP handler with /api/health, optional API routes, and SPA fallback.
func NewHandlerWithAPI(staticFS fs.FS, registerAPI func(mux *http.ServeMux)) http.Handler {
	return NewHandlerWithAPIs(staticFS, registerAPI)
}

// NewHandlerWithAPIs constructs the HTTP handler with /api/health, multiple route registrars, and SPA fallback.
func NewHandlerWithAPIs(staticFS fs.FS, registrars ...func(mux *http.ServeMux)) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	for _, register := range registrars {
		if register != nil {
			register(mux)
		}
	}
	mux.HandleFunc("GET /", spaHandler(staticFS))
	return mux
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func spaHandler(staticFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/api" {
			http.NotFound(w, r)
			return
		}
		if staticFS == nil {
			http.NotFound(w, r)
			return
		}

		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if cleanPath == "" || cleanPath == "." {
			serveFile(w, r, staticFS, indexFileName)
			return
		}

		if info, err := fs.Stat(staticFS, cleanPath); err == nil && !info.IsDir() {
			serveFile(w, r, staticFS, cleanPath)
			return
		}

		serveFile(w, r, staticFS, indexFileName)
	}
}

func serveFile(w http.ResponseWriter, r *http.Request, staticFS fs.FS, name string) {
	f, err := staticFS.Open(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() {
		_ = f.Close()
	}()

	info, err := f.Stat()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	rs, ok := f.(io.ReadSeeker)
	if !ok {
		http.ServeContent(w, r, name, info.ModTime(), mustReadAllSeeker(f))
		return
	}
	http.ServeContent(w, r, name, info.ModTime(), rs)
}

func mustReadAllSeeker(r io.Reader) io.ReadSeeker {
	data, err := io.ReadAll(r)
	if err != nil {
		return strings.NewReader("")
	}
	return strings.NewReader(string(data))
}
