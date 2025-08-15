package domain

import (
	"time"

	"github.com/google/uuid"
)

type FileFormat string

const (
	FormatPlainText FileFormat = "plaintext"
	FormatMarkdown  FileFormat = "markdown"
	FormatOrgMode   FileFormat = "orgmode"
)

type FileInfo struct {
	ID           uuid.UUID `json:"id"`
	WorkspaceID  uuid.UUID `json:"workspace_id"`
	FilePath     string    `json:"file_path"`
	ContentHash  string    `json:"content_hash"`
	SizeBytes    int64     `json:"size_bytes"`
	MimeType     string    `json:"mime_type"`
	LastModified time.Time `json:"last_modified"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type FileWithContent struct {
	FileInfo
	Content []byte `json:"content"`
}

type FileUploadRequest struct {
	WorkspaceID  uuid.UUID `json:"workspace_id"`
	FilePath     string    `json:"file_path"`
	Content      []byte    `json:"content"`
	LastModified time.Time `json:"last_modified"`
	ClientID     string    `json:"client_id,omitempty"`
}

type FileMetadata struct {
	FileID       uuid.UUID              `json:"file_id"`
	Format       FileFormat             `json:"format"`
	ParsedBlocks map[string]interface{} `json:"parsed_blocks,omitempty"`
	Properties   map[string]interface{} `json:"properties,omitempty"`
	WordCount    int                    `json:"word_count"`
	LastParsed   time.Time              `json:"last_parsed"`
}

type SyncOperation struct {
	ID            uuid.UUID `json:"id"`
	WorkspaceID   uuid.UUID `json:"workspace_id"`
	FileID        *uuid.UUID `json:"file_id,omitempty"`
	OperationType string    `json:"operation_type"`
	ClientID      *string   `json:"client_id,omitempty"`
	Status        string    `json:"status"`
	ErrorMessage  *string   `json:"error_message,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type WorkspaceStorageInfo struct {
	StorageLimitBytes   int64 `json:"storage_limit_bytes"`
	StorageUsedBytes    int64 `json:"storage_used_bytes"`
	FileCount           int64 `json:"file_count"`
	ActualStorageUsed   int64 `json:"actual_storage_used"`
}
