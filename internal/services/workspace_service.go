package services

import (
	"context"
	"fmt"

	"github.com/duckonomy/noture/internal/db"
	"github.com/duckonomy/noture/internal/domain"
	"github.com/duckonomy/noture/pkg/logger"
	"github.com/duckonomy/noture/pkg/pgconv"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type WorkspaceService struct {
	queries *db.Queries
	log     *logger.Logger
}

func NewWorkspaceService(queries *db.Queries) *WorkspaceService {
	return &WorkspaceService{
		queries: queries,
		log:     logger.New(),
	}
}

func (s *WorkspaceService) CreateWorkspace(ctx context.Context, req domain.CreateWorkspaceRequest, userID uuid.UUID, userTier domain.UserTier) (*domain.Workspace, error) {
	log := s.log.WithUser(userID.String(), "")
	log.Info("Creating new workspace", "name", req.Name, "user_tier", userTier)

	existingWorkspaces, err := s.queries.GetWorkspacesByUser(ctx, pgconv.UUIDToPg(userID))
	if err != nil {
		log.WithError(err).Error("Failed to get existing workspaces")
		return nil, fmt.Errorf("failed to get existing workspaces: %w", err)
	}

	maxWorkspaces := userTier.GetMaxWorkspaces()
	if maxWorkspaces > 0 && len(existingWorkspaces) >= maxWorkspaces {
		log.Warn("Workspace limit exceeded",
			"current_workspaces", len(existingWorkspaces),
			"max_workspaces", maxWorkspaces,
			"tier", userTier)
		return nil, fmt.Errorf("workspace limit reached for %s tier: %d/%d", userTier, len(existingWorkspaces), maxWorkspaces)
	}

	storageLimit := userTier.GetStorageLimit()

	workspace, err := s.queries.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		UserID:            pgconv.UUIDToPg(userID),
		Name:              req.Name,
		StorageLimitBytes: storageLimit,
	})
	if err != nil {
		log.WithError(err).Error("Failed to create workspace in database", "name", req.Name)
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	workspaceResult := &domain.Workspace{
		ID:                pgconv.PgToUUID(workspace.ID),
		UserID:            pgconv.PgToUUID(workspace.UserID),
		Name:              workspace.Name,
		StorageLimitBytes: workspace.StorageLimitBytes,
		StorageUsedBytes:  pgconv.PgToInt64(workspace.StorageUsedBytes),
		CreatedAt:         pgconv.PgToTime(workspace.CreatedAt),
		UpdatedAt:         pgconv.PgToTime(workspace.UpdatedAt),
	}

	log.LogWorkspaceOperation("create", workspaceResult.ID.String(), workspaceResult.Name)
	log.Info("Workspace created successfully",
		"workspace_id", workspaceResult.ID,
		"storage_limit", storageLimit)

	return workspaceResult, nil
}

func (s *WorkspaceService) GetWorkspacesByUser(ctx context.Context, userID uuid.UUID) ([]domain.Workspace, error) {
	log := s.log.WithUser(userID.String(), "")
	log.Debug("Fetching workspaces for user")

	dbWorkspaces, err := s.queries.GetWorkspacesByUser(ctx, pgconv.UUIDToPg(userID))
	if err != nil {
		log.WithError(err).Error("Failed to fetch workspaces from database")
		return nil, fmt.Errorf("failed to get workspaces: %w", err)
	}

	workspaces := make([]domain.Workspace, len(dbWorkspaces))
	for i, ws := range dbWorkspaces {
		workspaces[i] = domain.Workspace{
			ID:                pgconv.PgToUUID(ws.ID),
			UserID:            pgconv.PgToUUID(ws.UserID),
			Name:              ws.Name,
			StorageLimitBytes: ws.StorageLimitBytes,
			StorageUsedBytes:  pgconv.PgToInt64(ws.StorageUsedBytes),
			CreatedAt:         pgconv.PgToTime(ws.CreatedAt),
			UpdatedAt:         pgconv.PgToTime(ws.UpdatedAt),
		}
	}

	log.Info("Successfully retrieved workspaces", "count", len(workspaces))
	return workspaces, nil
}

func (s *WorkspaceService) GetWorkspaceByID(ctx context.Context, workspaceID uuid.UUID, userID uuid.UUID) (*domain.Workspace, error) {
	log := s.log.WithUser(userID.String(), "").WithWorkspace(workspaceID.String(), "")
	log.Debug("Fetching workspace by ID")

	workspace, err := s.queries.GetWorkspaceByID(ctx, pgconv.UUIDToPg(workspaceID))
	if err != nil {
		log.WithError(err).Error("Workspace not found", "workspace_id", workspaceID)
		return nil, fmt.Errorf("workspace not found: %w", err)
	}

	if pgconv.PgToUUID(workspace.UserID) != userID {
		log.Warn("Access denied: workspace belongs to different user",
			"workspace_owner", pgconv.PgToUUID(workspace.UserID),
			"requesting_user", userID)
		return nil, fmt.Errorf("access denied: workspace belongs to different user")
	}

	result := &domain.Workspace{
		ID:                pgconv.PgToUUID(workspace.ID),
		UserID:            pgconv.PgToUUID(workspace.UserID),
		Name:              workspace.Name,
		StorageLimitBytes: workspace.StorageLimitBytes,
		StorageUsedBytes:  pgconv.PgToInt64(workspace.StorageUsedBytes),
		CreatedAt:         pgconv.PgToTime(workspace.CreatedAt),
		UpdatedAt:         pgconv.PgToTime(workspace.UpdatedAt),
	}

	log.Debug("Successfully retrieved workspace", "workspace_name", result.Name)
	return result, nil
}

func (s *WorkspaceService) GetWorkspaceStorageInfo(ctx context.Context, workspaceID uuid.UUID, userID uuid.UUID) (*domain.WorkspaceStorageInfo, error) {
	log := s.log.WithUser(userID.String(), "").WithWorkspace(workspaceID.String(), "")
	log.Debug("Fetching workspace storage information")

	workspace, err := s.queries.GetWorkspaceByID(ctx, pgconv.UUIDToPg(workspaceID))
	if err != nil {
		log.WithError(err).Error("Workspace not found for storage info request")
		return nil, fmt.Errorf("workspace not found: %w", err)
	}

	if pgconv.PgToUUID(workspace.UserID) != userID {
		log.Warn("Access denied: storage info request for workspace belonging to different user",
			"workspace_owner", pgconv.PgToUUID(workspace.UserID))
		return nil, fmt.Errorf("access denied: workspace belongs to different user")
	}

	storageInfo, err := s.queries.GetWorkspaceStorageUsage(ctx, pgconv.UUIDToPg(workspaceID))
	if err != nil {
		log.WithError(err).Error("Failed to get storage usage from database")
		return nil, fmt.Errorf("failed to get storage usage: %w", err)
	}

	var actualUsed int64
	if numeric, ok := storageInfo.ActualStorageUsed.(pgtype.Numeric); ok && numeric.Valid {
		int64Val, _ := numeric.Int64Value()
		actualUsed = int64Val.Int64
	}

	result := &domain.WorkspaceStorageInfo{
		StorageLimitBytes: storageInfo.StorageLimitBytes,
		StorageUsedBytes:  pgconv.PgToInt64(storageInfo.StorageUsedBytes),
		FileCount:         storageInfo.FileCount,
		ActualStorageUsed: actualUsed,
	}

	log.Info("Retrieved workspace storage information",
		"storage_used", result.StorageUsedBytes,
		"storage_limit", result.StorageLimitBytes,
		"file_count", result.FileCount,
		"actual_used", result.ActualStorageUsed)

	return result, nil
}
