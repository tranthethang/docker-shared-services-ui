package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/tranthethang/docker-shared-services-ui/internal/docker"
)

func (h *ReadOnlyHandler) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported", "streaming is not supported")
		return
	}

	interval := h.TelemetryInterval
	if interval <= 0 {
		interval = defaultTelemetryInterval
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ctx := r.Context()
	eventsCh := docker.WatchEvents(ctx, h.Docker, h.EventOptions)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case sig, open := <-eventsCh:
			if !open {
				return
			}
			if err := writeSSEEvent(w, flusher, "refresh", sig); err != nil {
				return
			}
		case <-ticker.C:
			tel, err := docker.CollectTelemetry(ctx, h.Docker)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				continue
			}
			if err := writeSSEEvent(w, flusher, "telemetry", tel); err != nil {
				return
			}
		}
	}
}

func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
