package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/docker/docker/api/types/network"

	"github.com/tranthethang/docker-shared-services-ui/internal/catalog"
	"github.com/tranthethang/docker-shared-services-ui/internal/docker"
	"github.com/tranthethang/docker-shared-services-ui/internal/env"
	"github.com/tranthethang/docker-shared-services-ui/internal/exporter"
	"github.com/tranthethang/docker-shared-services-ui/internal/ops"
	"github.com/tranthethang/docker-shared-services-ui/internal/stack"
)

// AdminHandler serves env status, setup, validate, package export, and maintenance endpoints.
type AdminHandler struct {
	RepoPath string
	Catalog  *catalog.Loaded
	Identity stack.Identity
	Docker   docker.NetworkManager
	Runner   *ops.Runner
	AppCtx   context.Context
}

type networkStatusDTO struct {
	Name   string `json:"name"`
	Subnet string `json:"subnet"`
	Exists bool   `json:"exists"`
}

type envStatusResponse struct {
	StackName             string             `json:"stackName"`
	Files                 []env.FileStatus   `json:"files"`
	Problems              []env.Problem      `json:"problems"`
	Networks              []networkStatusDTO `json:"networks"`
	AvailableServices     []string           `json:"availableServices"`
	DefaultExportServices []string           `json:"defaultExportServices"`
}

type setupRequest struct {
	Password string `json:"password"`
	Force    bool   `json:"force"`
}

type mergeResponse struct {
	Added int              `json:"added"`
	Files []env.FileStatus `json:"files"`
}

type exportPreviewRequest struct {
	Services []string `json:"services"`
}

type exportRequest struct {
	Name     string   `json:"name"`
	Services []string `json:"services"`
}

type confirmRequest struct {
	ConfirmStackName string `json:"confirmStackName"`
}

// Register mounts all env, setup, export, and maintenance routes onto mux.
func (h *AdminHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/env/status", h.handleEnvStatus)
	mux.HandleFunc("POST /api/setup", h.handleSetup)
	mux.HandleFunc("POST /api/env/merge", h.handleEnvMerge)
	mux.HandleFunc("POST /api/validate", h.handleValidate)
	mux.HandleFunc("POST /api/export/preview", h.handleExportPreview)
	mux.HandleFunc("POST /api/export", h.handleExport)
	mux.HandleFunc("POST /api/maintenance/prune", h.handlePrune)
	mux.HandleFunc("POST /api/maintenance/teardown", h.handleTeardown)
}

func (h *AdminHandler) jobContext(reqCtx context.Context) context.Context {
	if h.AppCtx != nil {
		return h.AppCtx
	}
	return context.WithoutCancel(reqCtx)
}

func (h *AdminHandler) handleEnvStatus(w http.ResponseWriter, r *http.Request) {
	services := h.Catalog.ActiveNames
	files := env.CheckEnv(h.RepoPath, services)
	problems := env.PrerequisiteProblems(h.RepoPath, services)
	if problems == nil {
		problems = []env.Problem{}
	}

	nets := make([]networkStatusDTO, 0, len(h.Identity.Networks))
	for _, pair := range h.Identity.Networks {
		exists := false
		if h.Docker != nil {
			if _, err := h.Docker.NetworkInspect(r.Context(), pair.Name, network.InspectOptions{}); err == nil {
				exists = true
			}
		}
		nets = append(nets, networkStatusDTO{
			Name:   pair.Name,
			Subnet: pair.Subnet,
			Exists: exists,
		})
	}

	defaults := make([]string, 0, 4)
	for _, svc := range []string{"traefik", "postgres", "redis", "dozzle"} {
		if slices.Contains(services, svc) {
			defaults = append(defaults, svc)
		}
	}

	writeJSON(w, http.StatusOK, envStatusResponse{
		StackName:             ops.ExpectedConfirmName(h.Identity),
		Files:                 files,
		Problems:              problems,
		Networks:              nets,
		AvailableServices:     services,
		DefaultExportServices: defaults,
	})
}

func (h *AdminHandler) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req setupRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request body")
			return
		}
	}
	job, err := h.Runner.StartSetup(h.jobContext(r.Context()), req.Password, req.Force)
	if err != nil {
		if errors.Is(err, ops.ErrBusy) {
			writeError(w, http.StatusConflict, "ops_busy", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_setup", err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *AdminHandler) handleEnvMerge(w http.ResponseWriter, _ *http.Request) {
	added, err := env.MergeMissingEnv(h.RepoPath, h.Catalog.ActiveNames, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "merge_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mergeResponse{
		Added: added,
		Files: env.CheckEnv(h.RepoPath, h.Catalog.ActiveNames),
	})
}

func (h *AdminHandler) handleValidate(w http.ResponseWriter, r *http.Request) {
	job, err := h.Runner.StartValidate(h.jobContext(r.Context()))
	if err != nil {
		writeOpsError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *AdminHandler) handleExportPreview(w http.ResponseWriter, r *http.Request) {
	var req exportPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request body")
		return
	}
	res, err := exporter.Preview(h.RepoPath, req.Services, h.Catalog.ActiveNames)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_export", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *AdminHandler) handleExport(w http.ResponseWriter, r *http.Request) {
	var req exportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request body")
		return
	}
	res, err := exporter.Export(h.RepoPath, req.Name, req.Services, h.Catalog.ActiveNames)
	if err != nil {
		status := http.StatusBadRequest
		code := "invalid_export"
		if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
			code = "export_exists"
		}
		writeError(w, status, code, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (h *AdminHandler) handlePrune(w http.ResponseWriter, r *http.Request) {
	var req confirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request body")
		return
	}
	job, err := h.Runner.StartPrune(h.jobContext(r.Context()), req.ConfirmStackName)
	if err != nil {
		if errors.Is(err, ops.ErrConfirmMismatch) {
			writeError(w, http.StatusBadRequest, "confirm_mismatch", err.Error())
			return
		}
		writeOpsError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *AdminHandler) handleTeardown(w http.ResponseWriter, r *http.Request) {
	var req confirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request body")
		return
	}
	job, err := h.Runner.StartTeardown(h.jobContext(r.Context()), req.ConfirmStackName)
	if err != nil {
		if errors.Is(err, ops.ErrConfirmMismatch) {
			writeError(w, http.StatusBadRequest, "confirm_mismatch", err.Error())
			return
		}
		writeOpsError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}
