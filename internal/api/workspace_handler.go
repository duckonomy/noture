package api

import (
	"encoding/json"
	"net/http"

	"github.com/duckonomy/noture/internal/domain"
	"github.com/duckonomy/noture/internal/services"
	"github.com/duckonomy/noture/pkg/logger"
	"github.com/google/uuid"
)

type WorkspaceHandler struct {
	workspaceService *services.WorkspaceService
	log              *logger.Logger
}

func NewWorkspaceHandler(workspaceService *services.WorkspaceService) *WorkspaceHandler {
	return &WorkspaceHandler{
		workspaceService: workspaceService,
		log:              logger.New(),
	}
}

func (h *WorkspaceHandler) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	authCtx := r.Context().Value("auth").(*domain.AuthContext)

	var req domain.CreateWorkspaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Missing required field: name", http.StatusBadRequest)
		return
	}

	workspace, err := h.workspaceService.CreateWorkspace(r.Context(), req, authCtx.UserID, authCtx.UserTier)
	if err != nil {
		if err.Error() == "workspace limit reached" {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(workspace)
}

func (h *WorkspaceHandler) GetWorkspaces(w http.ResponseWriter, r *http.Request) {
	authCtx := r.Context().Value("auth").(*domain.AuthContext)

	workspaces, err := h.workspaceService.GetWorkspacesByUser(r.Context(), authCtx.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workspaces": workspaces,
		"count":      len(workspaces),
	})
}

func (h *WorkspaceHandler) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	authCtx := r.Context().Value("auth").(*domain.AuthContext)

	workspaceIDStr := r.PathValue("id")
	if workspaceIDStr == "" {
		http.Error(w, "Missing workspace ID", http.StatusBadRequest)
		return
	}

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		http.Error(w, "Invalid workspace ID format", http.StatusBadRequest)
		return
	}

	workspace, err := h.workspaceService.GetWorkspaceByID(r.Context(), workspaceID, authCtx.UserID)
	if err != nil {
		if err.Error() == "workspace not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if err.Error() == "access denied: workspace belongs to different user" {
			http.Error(w, "Workspace not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workspace)
}

func (h *WorkspaceHandler) GetWorkspaceStorage(w http.ResponseWriter, r *http.Request) {
	authCtx := r.Context().Value("auth").(*domain.AuthContext)

	workspaceIDStr := r.PathValue("id")
	if workspaceIDStr == "" {
		http.Error(w, "Missing workspace ID", http.StatusBadRequest)
		return
	}

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		http.Error(w, "Invalid workspace ID format", http.StatusBadRequest)
		return
	}

	storageInfo, err := h.workspaceService.GetWorkspaceStorageInfo(r.Context(), workspaceID, authCtx.UserID)
	if err != nil {
		if err.Error() == "workspace not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if err.Error() == "access denied: workspace belongs to different user" {
			http.Error(w, "Workspace not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(storageInfo)
}

func (h *WorkspaceHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/workspaces", h.CreateWorkspace)
	mux.HandleFunc("GET /api/workspaces", h.GetWorkspaces)
	mux.HandleFunc("GET /api/workspaces/{id}", h.GetWorkspace)
	mux.HandleFunc("GET /api/workspaces/{id}/storage", h.GetWorkspaceStorage)
}
