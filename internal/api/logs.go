package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/tranthethang/docker-shared-services-ui/internal/catalog"
	"github.com/tranthethang/docker-shared-services-ui/internal/docker"
	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

// LogsHandler serves masked container log SSE streams for active services.
type LogsHandler struct {
	RepoPath string
	Identity stack.Identity
	Catalog  *catalog.Loaded
	Docker   docker.LogStreamer
}

// Register mounts the container log streaming route onto mux.
func (h *LogsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/services/{name}/logs", h.handleServiceLogs)
}

func (h *LogsHandler) handleServiceLogs(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if h.Catalog == nil {
		writeError(w, http.StatusInternalServerError, "catalog_unavailable", "catalog is not loaded")
		return
	}
	if _, ok := h.Catalog.Entries[name]; !ok {
		writeError(w, http.StatusBadRequest, "unknown_service", fmt.Sprintf("unknown service: %s", name))
		return
	}

	tail := r.URL.Query().Get("tail")
	if tail != "" && tail != "all" {
		n, err := strconv.Atoi(tail)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid_tail", "tail must be 'all' or a non-negative integer")
			return
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported", "streaming is not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	outCh, errCh := docker.StreamServiceLogs(r.Context(), h.Docker, docker.LogStreamOptions{
		RepoPath:  h.RepoPath,
		Prefix:    h.Identity.Prefix,
		Service:   name,
		Container: r.URL.Query().Get("container"),
		Tail:      tail,
		Follow:    true,
	})

	ctx := r.Context()
	for outCh != nil || errCh != nil {
		select {
		case <-ctx.Done():
			return
		case line, open := <-outCh:
			if !open {
				outCh = nil
				continue
			}
			if err := writeSSEEvent(w, flusher, "log", line); err != nil {
				return
			}
		case err, open := <-errCh:
			if !open {
				errCh = nil
				continue
			}
			if err != nil {
				_ = writeSSEEvent(w, flusher, "error", errorResponse{
					Code:  "log_stream_error",
					Error: err.Error(),
				})
				return
			}
		}
	}
}
