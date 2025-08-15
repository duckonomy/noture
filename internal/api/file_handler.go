package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/duckonomy/noture/internal/domain"
	"github.com/duckonomy/noture/internal/services"
	"github.com/google/uuid"
)

type FileHandler struct {
	fileService *services.FileService
}

func NewFileHandler(fileService *services.FileService) *FileHandler {
	return &FileHandler{
		fileService: fileService,
	}
}

func (h *FileHandler) UploadFile(w http.ResponseWriter, r *http.Request) {
	authCtx := r.Context().Value("auth").(*domain.AuthContext)

	err := r.ParseMultipartForm(32 << 20) // 32MB limit
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	workspaceIDStr := r.FormValue("workspace_id")
	filePath := r.FormValue("file_path")
	lastModifiedStr := r.FormValue("last_modified")
	clientID := r.FormValue("client_id")

	if workspaceIDStr == "" || filePath == "" {
		http.Error(w, "Missing required fields: workspace_id, file_path", http.StatusBadRequest)
		return
	}

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		http.Error(w, "Invalid workspace_id format", http.StatusBadRequest)
		return
	}

	var lastModified time.Time
	if lastModifiedStr != "" {
		lastModified, err = time.Parse(time.RFC3339, lastModifiedStr)
		if err != nil {
			http.Error(w, "Invalid last_modified format (use RFC3339)", http.StatusBadRequest)
			return
		}
	} else {
		lastModified = time.Now()
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file in form data", http.StatusBadRequest)
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read file content", http.StatusBadRequest)
		return
	}

	req := domain.FileUploadRequest{
		WorkspaceID:  workspaceID,
		FilePath:     filePath,
		Content:      content,
		LastModified: lastModified,
		ClientID:     clientID,
	}

	fileInfo, err := h.fileService.UploadFile(r.Context(), req, authCtx.UserID)
	if err != nil {
		if err.Error() == "storage limit exceeded" {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(fileInfo)
}

func (h *FileHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	authCtx := r.Context().Value("auth").(*domain.AuthContext)

	workspaceIDStr := r.PathValue("workspace_id")
	filePath := r.PathValue("file_path")

	if workspaceIDStr == "" || filePath == "" {
		http.Error(w, "Missing workspace_id or file_path", http.StatusBadRequest)
		return
	}

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		http.Error(w, "Invalid workspace_id format", http.StatusBadRequest)
		return
	}

	includeContent := r.URL.Query().Get("content") == "true"
	isDownload := r.URL.Query().Get("download") == "true"

	if isDownload {
		fileWithContent, err := h.fileService.GetFileContent(r.Context(), workspaceID, filePath, authCtx.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", fileWithContent.MimeType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filePath))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileWithContent.Content)))
		w.Header().Set("Last-Modified", fileWithContent.LastModified.Format(http.TimeFormat))

		w.Write(fileWithContent.Content)
	} else if includeContent {
		fileWithContent, err := h.fileService.GetFileContent(r.Context(), workspaceID, filePath, authCtx.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fileWithContent)
	} else {
		fileInfo, err := h.fileService.GetFile(r.Context(), workspaceID, filePath, authCtx.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fileInfo)
	}
}

func (h *FileHandler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	authCtx := r.Context().Value("auth").(*domain.AuthContext)

	workspaceIDStr := r.PathValue("workspace_id")
	filePath := r.PathValue("file_path")

	if workspaceIDStr == "" || filePath == "" {
		http.Error(w, "Missing workspace_id or file_path", http.StatusBadRequest)
		return
	}

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		http.Error(w, "Invalid workspace_id format", http.StatusBadRequest)
		return
	}

	fileWithContent, err := h.fileService.GetFileContent(r.Context(), workspaceID, filePath, authCtx.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", fileWithContent.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filePath))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileWithContent.Content)))
	w.Header().Set("Last-Modified", fileWithContent.LastModified.Format(http.TimeFormat))

	w.Write(fileWithContent.Content)
}

func (h *FileHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
	authCtx := r.Context().Value("auth").(*domain.AuthContext)

	workspaceIDStr := r.PathValue("workspace_id")
	if workspaceIDStr == "" {
		http.Error(w, "Missing workspace_id", http.StatusBadRequest)
		return
	}

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		http.Error(w, "Invalid workspace_id format", http.StatusBadRequest)
		return
	}

	files, err := h.fileService.ListFiles(r.Context(), workspaceID, authCtx.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"files": files,
		"count": len(files),
	})
}

func (h *FileHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	authCtx := r.Context().Value("auth").(*domain.AuthContext)

	workspaceIDStr := r.PathValue("workspace_id")
	filePath := r.PathValue("file_path")

	if workspaceIDStr == "" || filePath == "" {
		http.Error(w, "Missing workspace_id or file_path", http.StatusBadRequest)
		return
	}

	workspaceID, err := uuid.Parse(workspaceIDStr)
	if err != nil {
		http.Error(w, "Invalid workspace_id format", http.StatusBadRequest)
		return
	}

	err = h.fileService.DeleteFile(r.Context(), workspaceID, filePath, authCtx.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *FileHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/files/upload", h.UploadFile)
	mux.HandleFunc("GET /api/files/{workspace_id}/{file_path...}", h.GetFile)
	mux.HandleFunc("GET /api/workspaces/{workspace_id}/files", h.ListFiles)
	mux.HandleFunc("DELETE /api/files/{workspace_id}/{file_path...}", h.DeleteFile)
}
