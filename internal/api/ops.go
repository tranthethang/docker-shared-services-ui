package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/tranthethang/docker-shared-services-ui/internal/ops"
)

// OpsHandler serves lifecycle mutation, status, and progress SSE endpoints.
type OpsHandler struct {
	Runner *ops.Runner
	AppCtx context.Context
}

type applyRequest struct {
	Up   []string `json:"up"`
	Down []string `json:"down"`
}

// Register mounts all lifecycle operation routes onto mux.
func (h *OpsHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/services/{name}/{action}", h.handleServiceAction)
	mux.HandleFunc("POST /api/ops/apply", h.handleApply)
	mux.HandleFunc("GET /api/ops/status", h.handleStatus)
	mux.HandleFunc("GET /api/ops/stream", h.handleStream)
}

func (h *OpsHandler) jobContext(reqCtx context.Context) context.Context {
	if h.AppCtx != nil {
		return h.AppCtx
	}
	return context.WithoutCancel(reqCtx)
}

func (h *OpsHandler) handleServiceAction(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	action := r.PathValue("action")

	job, err := h.Runner.StartSingle(h.jobContext(r.Context()), name, action)
	if err != nil {
		writeOpsError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *OpsHandler) handleApply(w http.ResponseWriter, r *http.Request) {
	var req applyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request body")
		return
	}

	job, err := h.Runner.StartBatch(h.jobContext(r.Context()), req.Up, req.Down)
	if err != nil {
		writeOpsError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *OpsHandler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.Runner.Status())
}

func (h *OpsHandler) handleStream(w http.ResponseWriter, r *http.Request) {
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

	subID, ch, snap := h.Runner.Subscribe()
	defer h.Runner.Unsubscribe(subID)

	if err := writeSSEEvent(w, flusher, "status", snap); err != nil {
		return
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, open := <-ch:
			if !open {
				return
			}
			if err := writeSSEEvent(w, flusher, ev.Type, ev); err != nil {
				return
			}
		}
	}
}

func writeOpsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ops.ErrBusy):
		writeError(w, http.StatusConflict, "ops_busy", err.Error())
	case errors.Is(err, ops.ErrUnknownService):
		writeError(w, http.StatusBadRequest, "unknown_service", err.Error())
	case errors.Is(err, ops.ErrInvalidAction):
		writeError(w, http.StatusBadRequest, "invalid_action", err.Error())
	case errors.Is(err, ops.ErrServiceLocked):
		writeError(w, http.StatusUnprocessableEntity, "service_locked", err.Error())
	case errors.Is(err, ops.ErrPrereqFailed):
		writeError(w, http.StatusUnprocessableEntity, "prereq_failed", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "ops_error", err.Error())
	}
}
