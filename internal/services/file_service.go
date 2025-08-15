package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"mime"
	"path/filepath"
	"strings"

	"github.com/duckonomy/noture/internal/db"
	"github.com/duckonomy/noture/internal/domain"
	"github.com/duckonomy/noture/pkg/logger"
	"github.com/duckonomy/noture/pkg/pgconv"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type FileService struct {
	queries                     *db.Queries
	conn                        *pgx.Conn
	disableAsyncMetadataParsing bool
	log                         *logger.Logger
}

func NewFileService(queries *db.Queries, conn *pgx.Conn) *FileService {
	return &FileService{
		queries:                     queries,
		conn:                        conn,
		disableAsyncMetadataParsing: false,
		log:                         logger.New(),
	}
}

func NewFileServiceForTesting(queries *db.Queries, conn *pgx.Conn) *FileService {
	return &FileService{
		queries:                     queries,
		conn:                        conn,
		disableAsyncMetadataParsing: true,
		log:                         logger.New(),
	}
}

func (s *FileService) UploadFile(ctx context.Context, req domain.FileUploadRequest, userID uuid.UUID) (*domain.FileInfo, error) {
	log := s.log.WithUser(userID.String(), "").WithWorkspace(req.WorkspaceID.String(), "")
	log.Info("Starting file upload", "file_path", req.FilePath, "size_bytes", len(req.Content))

	workspace, err := s.queries.GetWorkspaceByID(ctx, pgconv.UUIDToPg(req.WorkspaceID))
	if err != nil {
		log.WithError(err).Error("Workspace not found", "workspace_id", req.WorkspaceID)
		return nil, fmt.Errorf("workspace not found: %w", err)
	}

	if pgconv.PgToUUID(workspace.UserID) != userID {
		log.Warn("Access denied: workspace belongs to different user",
			"workspace_owner", pgconv.PgToUUID(workspace.UserID),
			"requesting_user", userID)
		return nil, fmt.Errorf("access denied: workspace belongs to different user")
	}

	hash := sha256.Sum256(req.Content)
	contentHash := fmt.Sprintf("%x", hash)

	storageInfo, err := s.queries.GetWorkspaceStorageUsage(ctx, pgconv.UUIDToPg(req.WorkspaceID))
	if err != nil {
		return nil, fmt.Errorf("failed to get storage usage: %w", err)
	}

	var currentFileSize int64
	existingFile, err := s.queries.GetFile(ctx, db.GetFileParams{
		WorkspaceID: pgconv.UUIDToPg(req.WorkspaceID),
		FilePath:    req.FilePath,
	})
	if err == nil {
		currentFileSize = existingFile.SizeBytes
	}

	newStorageUsage := pgconv.PgToInt64(storageInfo.StorageUsedBytes) - currentFileSize + int64(len(req.Content))
	if newStorageUsage > storageInfo.StorageLimitBytes {
		log.Warn("Storage limit exceeded",
			"current_usage", pgconv.PgToInt64(storageInfo.StorageUsedBytes),
			"needed_usage", newStorageUsage,
			"limit", storageInfo.StorageLimitBytes)
		return nil, fmt.Errorf("storage limit exceeded: need %d bytes, limit %d bytes",
			newStorageUsage, storageInfo.StorageLimitBytes)
	}

	mimeType := s.detectMimeType(req.FilePath, req.Content)

	syncOp, err := s.queries.CreateSyncOperation(ctx, db.CreateSyncOperationParams{
		WorkspaceID:   pgconv.UUIDToPg(req.WorkspaceID),
		OperationType: "upload",
		ClientID:      pgconv.StringToPg(req.ClientID),
		Status:        "pending",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create sync operation: %w", err)
	}

	tx, err := s.conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	file, err := qtx.UpsertFile(ctx, db.UpsertFileParams{
		WorkspaceID:  pgconv.UUIDToPg(req.WorkspaceID),
		FilePath:     req.FilePath,
		ContentHash:  contentHash,
		Content:      req.Content,
		SizeBytes:    int64(len(req.Content)),
		MimeType:     pgconv.StringToPg(mimeType),
		LastModified: pgconv.TimeToPg(req.LastModified),
	})
	if err != nil {
		errStr := err.Error()
		s.queries.UpdateSyncOperationStatus(ctx, db.UpdateSyncOperationStatusParams{
			ID:           syncOp.ID,
			Status:       "failed",
			ErrorMessage: pgconv.StringPtrToPg(&errStr),
		})
		return nil, fmt.Errorf("failed to upsert file: %w", err)
	}

	err = qtx.UpdateWorkspaceStorageUsed(ctx, db.UpdateWorkspaceStorageUsedParams{
		ID:               pgconv.UUIDToPg(req.WorkspaceID),
		StorageUsedBytes: pgconv.Int64ToPg(newStorageUsage),
	})
	if err != nil {
		errStr := err.Error()
		s.queries.UpdateSyncOperationStatus(ctx, db.UpdateSyncOperationStatusParams{
			ID:           syncOp.ID,
			Status:       "failed",
			ErrorMessage: pgconv.StringPtrToPg(&errStr),
		})
		return nil, fmt.Errorf("failed to update storage usage: %w", err)
	}

	err = qtx.CreateFileVersion(ctx, db.CreateFileVersionParams{
		FileID:        file.ID,
		VersionNumber: 1, // TODO: implement proper versioning
		ContentHash:   contentHash,
		Content:       req.Content,
	})
	if err != nil {
		// Don't fail the entire operation for versioning issues
		// TODO: log this error
	}

	if err = tx.Commit(ctx); err != nil {
		errStr := err.Error()
		s.queries.UpdateSyncOperationStatus(ctx, db.UpdateSyncOperationStatusParams{
			ID:           syncOp.ID,
			Status:       "failed",
			ErrorMessage: pgconv.StringPtrToPg(&errStr),
		})
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	err = s.queries.UpdateSyncOperationStatus(ctx, db.UpdateSyncOperationStatusParams{
		ID:     syncOp.ID,
		Status: "success",
	})
	if err != nil {
		// Don't fail the entire operation for sync log issues
		// TODO: log this error
	}

	if !s.disableAsyncMetadataParsing {
		go s.parseFileMetadata(context.Background(), file)
	}

	fileInfo := &domain.FileInfo{
		ID:           pgconv.PgToUUID(file.ID),
		WorkspaceID:  pgconv.PgToUUID(file.WorkspaceID),
		FilePath:     file.FilePath,
		ContentHash:  file.ContentHash,
		SizeBytes:    file.SizeBytes,
		MimeType:     pgconv.PgToString(file.MimeType),
		LastModified: pgconv.PgToTime(file.LastModified),
		UpdatedAt:    pgconv.PgToTime(file.UpdatedAt),
	}

	log.LogFileOperation("upload", req.FilePath, file.SizeBytes)
	log.Info("File upload completed successfully", "file_id", fileInfo.ID)

	return fileInfo, nil
}

func (s *FileService) GetFile(ctx context.Context, workspaceID uuid.UUID, filePath string, userID uuid.UUID) (*domain.FileInfo, error) {
	workspace, err := s.queries.GetWorkspaceByID(ctx, pgconv.UUIDToPg(workspaceID))
	if err != nil {
		return nil, fmt.Errorf("workspace not found: %w", err)
	}

	if pgconv.PgToUUID(workspace.UserID) != userID {
		return nil, fmt.Errorf("access denied: workspace belongs to different user")
	}

	file, err := s.queries.GetFile(ctx, db.GetFileParams{
		WorkspaceID: pgconv.UUIDToPg(workspaceID),
		FilePath:    filePath,
	})
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	return &domain.FileInfo{
		ID:           pgconv.PgToUUID(file.ID),
		WorkspaceID:  pgconv.PgToUUID(file.WorkspaceID),
		FilePath:     file.FilePath,
		ContentHash:  file.ContentHash,
		SizeBytes:    file.SizeBytes,
		MimeType:     pgconv.PgToString(file.MimeType),
		LastModified: pgconv.PgToTime(file.LastModified),
		UpdatedAt:    pgconv.PgToTime(file.UpdatedAt),
	}, nil
}

func (s *FileService) GetFileContent(ctx context.Context, workspaceID uuid.UUID, filePath string, userID uuid.UUID) (*domain.FileWithContent, error) {
	workspace, err := s.queries.GetWorkspaceByID(ctx, pgconv.UUIDToPg(workspaceID))
	if err != nil {
		return nil, fmt.Errorf("workspace not found: %w", err)
	}

	if pgconv.PgToUUID(workspace.UserID) != userID {
		return nil, fmt.Errorf("access denied: workspace belongs to different user")
	}

	file, err := s.queries.GetFile(ctx, db.GetFileParams{
		WorkspaceID: pgconv.UUIDToPg(workspaceID),
		FilePath:    filePath,
	})
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}

	return &domain.FileWithContent{
		FileInfo: domain.FileInfo{
			ID:           pgconv.PgToUUID(file.ID),
			WorkspaceID:  pgconv.PgToUUID(file.WorkspaceID),
			FilePath:     file.FilePath,
			ContentHash:  file.ContentHash,
			SizeBytes:    file.SizeBytes,
			MimeType:     pgconv.PgToString(file.MimeType),
			LastModified: pgconv.PgToTime(file.LastModified),
			UpdatedAt:    pgconv.PgToTime(file.UpdatedAt),
		},
		Content: file.Content,
	}, nil
}

func (s *FileService) ListFiles(ctx context.Context, workspaceID uuid.UUID, userID uuid.UUID) ([]domain.FileInfo, error) {
	workspace, err := s.queries.GetWorkspaceByID(ctx, pgconv.UUIDToPg(workspaceID))
	if err != nil {
		return nil, fmt.Errorf("workspace not found: %w", err)
	}

	if pgconv.PgToUUID(workspace.UserID) != userID {
		return nil, fmt.Errorf("access denied: workspace belongs to different user")
	}

	files, err := s.queries.ListFiles(ctx, pgconv.UUIDToPg(workspaceID))
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	result := make([]domain.FileInfo, len(files))
	for i, file := range files {
		result[i] = domain.FileInfo{
			ID:           pgconv.PgToUUID(file.ID),
			WorkspaceID:  pgconv.PgToUUID(file.WorkspaceID),
			FilePath:     file.FilePath,
			ContentHash:  file.ContentHash,
			SizeBytes:    file.SizeBytes,
			MimeType:     pgconv.PgToString(file.MimeType),
			LastModified: pgconv.PgToTime(file.LastModified),
			UpdatedAt:    pgconv.PgToTime(file.UpdatedAt),
		}
	}

	return result, nil
}

func (s *FileService) DeleteFile(ctx context.Context, workspaceID uuid.UUID, filePath string, userID uuid.UUID) error {
	workspace, err := s.queries.GetWorkspaceByID(ctx, pgconv.UUIDToPg(workspaceID))
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}

	if pgconv.PgToUUID(workspace.UserID) != userID {
		return fmt.Errorf("access denied: workspace belongs to different user")
	}

	file, err := s.queries.GetFile(ctx, db.GetFileParams{
		WorkspaceID: pgconv.UUIDToPg(workspaceID),
		FilePath:    filePath,
	})
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	tx, err := s.conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	err = qtx.DeleteFile(ctx, db.DeleteFileParams{
		WorkspaceID: pgconv.UUIDToPg(workspaceID),
		FilePath:    filePath,
	})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	newUsage := pgconv.PgToInt64(workspace.StorageUsedBytes) - file.SizeBytes
	err = qtx.UpdateWorkspaceStorageUsed(ctx, db.UpdateWorkspaceStorageUsedParams{
		ID:               pgconv.UUIDToPg(workspaceID),
		StorageUsedBytes: pgconv.Int64ToPg(newUsage),
	})
	if err != nil {
		return fmt.Errorf("failed to update storage usage: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *FileService) detectMimeType(filePath string, content []byte) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".md", ".markdown":
		return "text/markdown"
	case ".org":
		return "text/org"
	case ".txt":
		return "text/plain"
	default:
		mimeType := mime.TypeByExtension(ext)
		if mimeType != "" {
			return mimeType
		}
		return "text/plain"
	}
}

func (s *FileService) parseFileMetadata(ctx context.Context, file db.File) {
	format := s.DetectFileFormat(file.FilePath, file.Content)

	// TODO: Implement actual parsing logic for different formats
	var parsedBlocks []byte
	var properties []byte
	wordCount := len(strings.Fields(string(file.Content)))

	err := s.queries.UpsertFileMetadata(ctx, db.UpsertFileMetadataParams{
		FileID:       file.ID,
		Format:       string(format),
		ParsedBlocks: parsedBlocks,
		Properties:   properties,
		WordCount:    pgconv.Int32ToPg(int32(wordCount)),
	})

	if err != nil {
		// TODO: log this error properly
		fmt.Printf("Failed to store file metadata for %s: %v\n", file.FilePath, err)
	}
}

func (s *FileService) DetectFileFormat(filePath string, content []byte) domain.FileFormat {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".md", ".markdown":
		return domain.FormatMarkdown
	case ".org":
		return domain.FormatOrgMode
	default:
		return domain.FormatPlainText
	}
}
