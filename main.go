package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/duckonomy/noture/internal/api"
	"github.com/duckonomy/noture/internal/db"
	"github.com/duckonomy/noture/internal/services"
	"github.com/duckonomy/noture/pkg/auth"
	"github.com/duckonomy/noture/pkg/logger"
	"github.com/jackc/pgx/v5"
)

func main() {
	log := logger.New()
	log.Info("Starting Noture server", "version", "dev")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:password@localhost:5432/noture?sslmode=disable"
		log.Debug("Using default database URL")
	}

	log.Info("Connecting to database")
	conn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		log.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())
	log.Info("Database connection established")

	queries := db.New(conn)

	log.Info("Initializing services")
	fileService := services.NewFileService(queries, conn)
	workspaceService := services.NewWorkspaceService(queries)

	authMiddleware := auth.NewAuthMiddleware(queries)

	fileHandler := api.NewFileHandler(fileService)
	workspaceHandler := api.NewWorkspaceHandler(workspaceService)
	oauthHandler := api.NewOAuthHandler(queries)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status": "OK",
			"service": "Noture Server",
			"version": "dev",
			"oauth": map[string]bool{
				"google_configured": os.Getenv("GOOGLE_CLIENT_ID") != "",
				"github_configured": os.Getenv("GITHUB_CLIENT_ID") != "",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	fileHandler.RegisterRoutes(mux)
	workspaceHandler.RegisterRoutes(mux)

	oauthHandler.RegisterRoutes(mux)

	authMux := http.NewServeMux()
	authMux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status": "OK",
			"service": "Noture Server",
			"version": "dev",
			"oauth": map[string]bool{
				"google_configured": os.Getenv("GOOGLE_CLIENT_ID") != "",
				"github_configured": os.Getenv("GITHUB_CLIENT_ID") != "",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	oauthHandler.RegisterRoutes(authMux)

	authMux.HandleFunc("POST /api/files/upload", authMiddleware.RequireAuth(fileHandler.UploadFile))
	authMux.HandleFunc("GET /api/files/{workspace_id}/{file_path...}", authMiddleware.RequireAuth(fileHandler.GetFile))
	authMux.HandleFunc("GET /api/workspaces/{workspace_id}/files", authMiddleware.RequireAuth(fileHandler.ListFiles))
	authMux.HandleFunc("DELETE /api/files/{workspace_id}/{file_path...}", authMiddleware.RequireAuth(fileHandler.DeleteFile))

	authMux.HandleFunc("POST /api/workspaces", authMiddleware.RequireAuth(workspaceHandler.CreateWorkspace))
	authMux.HandleFunc("GET /api/workspaces", authMiddleware.RequireAuth(workspaceHandler.GetWorkspaces))
	authMux.HandleFunc("GET /api/workspaces/{id}", authMiddleware.RequireAuth(workspaceHandler.GetWorkspace))
	authMux.HandleFunc("GET /api/workspaces/{id}/storage", authMiddleware.RequireAuth(workspaceHandler.GetWorkspaceStorage))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	log.Info("Server starting", "port", port, "environment", os.Getenv("ENVIRONMENT"))

	handler := loggingMiddleware(log, authMux)

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}

func loggingMiddleware(log *logger.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(ww, r)

		duration := time.Since(start)
		log.LogRequest(r.Method, r.URL.Path, ww.statusCode, duration.String())
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}
