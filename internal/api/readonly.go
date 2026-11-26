// Package api implements the HTTP REST and SSE handlers for the dashboard.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/tranthethang/docker-shared-services-ui/internal/catalog"
	"github.com/tranthethang/docker-shared-services-ui/internal/docker"
	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

const defaultTelemetryInterval = 5 * time.Second

// ReadOnlyHandler serves read-only catalog, stack, Docker status, telemetry, and SSE endpoints.
type ReadOnlyHandler struct {
	Catalog           *catalog.Loaded
	Identity          stack.Identity
	Docker            docker.Engine
	TelemetryInterval time.Duration
	EventOptions      docker.EventWatcherOptions
}

// ServiceDTO merges catalog metadata with live Docker container status.
type ServiceDTO struct {
	Name               string                 `json:"name"`
	Category           string                 `json:"category"`
	Description        string                 `json:"description"`
	ExpectedContainers []string               `json:"expectedContainers"`
	WebUI              []string               `json:"webUi"`
	Info               []string               `json:"info"`
	State              string                 `json:"state"`
	Health             string                 `json:"health"`
	ContainerCount     int                    `json:"containerCount"`
	RunningCount       int                    `json:"runningCount"`
	Containers         []docker.ContainerInfo `json:"containers"`
}

type servicesResponse struct {
	Services   []ServiceDTO `json:"services"`
	Categories []string     `json:"categories"`
}

type presetsResponse struct {
	Presets []catalog.Preset `json:"presets"`
}

type infoResponse struct {
	Domain       string              `json:"domain"`
	StackName    string              `json:"stackName"`
	Prefix       string              `json:"prefix"`
	SubnetBase   int                 `json:"subnetBase"`
	Networks     []stack.NetworkPair `json:"networks"`
	ServiceCount int                 `json:"serviceCount"`
}

type errorResponse struct {
	Code  string `json:"code"`
	Error string `json:"error"`
}

// Register mounts all read-only API and SSE routes onto mux.
func (h *ReadOnlyHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/services", h.handleServices)
	mux.HandleFunc("GET /api/services/{name}", h.handleServiceDetail)
	mux.HandleFunc("GET /api/presets", h.handlePresets)
	mux.HandleFunc("GET /api/telemetry", h.handleTelemetry)
	mux.HandleFunc("GET /api/info", h.handleInfo)
	mux.HandleFunc("GET /api/events", h.handleEvents)
}

func (h *ReadOnlyHandler) handleServices(w http.ResponseWriter, r *http.Request) {
	statuses, err := docker.ListServiceStatuses(r.Context(), h.Docker, h.Catalog.ActiveNames, h.Identity.Prefix)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "docker_unavailable", err.Error())
		return
	}

	dtos := make([]ServiceDTO, 0, len(statuses))
	for _, st := range statuses {
		entry := h.Catalog.Entries[st.Name]
		dtos = append(dtos, mergeServiceDTO(entry, st))
	}

	writeJSON(w, http.StatusOK, servicesResponse{
		Services:   dtos,
		Categories: h.Catalog.Categories,
	})
}

func (h *ReadOnlyHandler) handleServiceDetail(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	entry, ok := h.Catalog.Entries[name]
	if !ok {
		writeError(w, http.StatusNotFound, "service_not_found", "service not found")
		return
	}

	statuses, err := docker.ListServiceStatuses(r.Context(), h.Docker, []string{name}, h.Identity.Prefix)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "docker_unavailable", err.Error())
		return
	}
	if len(statuses) == 0 {
		writeError(w, http.StatusNotFound, "service_not_found", "service not found")
		return
	}

	writeJSON(w, http.StatusOK, mergeServiceDTO(entry, statuses[0]))
}

func (h *ReadOnlyHandler) handlePresets(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, presetsResponse{
		Presets: catalog.FilterPresets(h.Catalog.ActiveNames),
	})
}

func (h *ReadOnlyHandler) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	telemetry, err := docker.CollectTelemetry(r.Context(), h.Docker)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "docker_unavailable", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, telemetry)
}

func (h *ReadOnlyHandler) handleInfo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, infoResponse{
		Domain:       h.Catalog.Domain,
		StackName:    h.Identity.Name,
		Prefix:       h.Identity.Prefix,
		SubnetBase:   h.Identity.SubnetBase,
		Networks:     h.Identity.Networks,
		ServiceCount: len(h.Catalog.ActiveNames),
	})
}

func mergeServiceDTO(entry catalog.Entry, st docker.ServiceStatus) ServiceDTO {
	return ServiceDTO{
		Name:               st.Name,
		Category:           entry.Category,
		Description:        entry.Description,
		ExpectedContainers: entry.Containers,
		WebUI:              entry.WebUI,
		Info:               entry.Info,
		State:              st.State,
		Health:             st.Health,
		ContainerCount:     st.ContainerCount,
		RunningCount:       st.RunningCount,
		Containers:         st.Containers,
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, errorResponse{
		Code:  code,
		Error: msg,
	})
}
