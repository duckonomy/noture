-- name: CreateUser :one
INSERT INTO users (email, password_hash, tier)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UpdateUserStorageUsed :exec
UPDATE users SET storage_used_bytes = $2, updated_at = NOW() WHERE id = $1;

-- NOTE: Atomic updates
-- -- name: UpdateUserStorageUsed :exec
-- UPDATE users SET storage_used_bytes = storage_used_bytes + $2, updated_at = NOW() WHERE id = $1;

-- name: CreateAPIToken :one
INSERT INTO api_tokens (user_id, token_hash, name, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetTokenByHash :one
SELECT t.*, u.id as user_id, u.email, u.tier
FROM api_tokens t
JOIN users u ON t.user_id = u.id
WHERE t.token_hash = $1 AND (t.expires_at IS NULL OR t.expires_at > NOW());

-- name: UpdateTokenLastUsed :exec
UPDATE api_tokens SET last_used_at = NOW() WHERE id = $1;

-- name: DeleteAPIToken :exec
DELETE FROM api_tokens WHERE id = $1 AND user_id = $2;

-- name: CreateWorkspace :one
INSERT INTO workspaces (user_id, name, storage_limit_bytes)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetWorkspacesByUser :many
SELECT * FROM workspaces WHERE user_id = $1 ORDER BY created_at DESC;

-- name: GetWorkspaceByID :one
SELECT * FROM workspaces WHERE id = $1;

-- name: UpdateWorkspaceStorageUsed :exec
UPDATE workspaces SET storage_used_bytes = $2, updated_at = NOW() WHERE id = $1;

-- NOTE: Atomic updates
-- -- name: UpdateUserStorageUsed :exec
-- UPDATE workspaces SET storage_used_bytes = storage_used_bytes + $2, updated_at = NOW() WHERE id = $1;

-- name: UpsertFile :one
INSERT INTO files (workspace_id, file_path, content_hash, content, size_bytes, mime_type, last_modified)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (workspace_id, file_path)
DO UPDATE SET
    content_hash = EXCLUDED.content_hash,
    content = EXCLUDED.content,
    size_bytes = EXCLUDED.size_bytes,
    mime_type = EXCLUDED.mime_type,
    last_modified = EXCLUDED.last_modified,
    updated_at = NOW()
RETURNING *;

-- name: GetFile :one
SELECT * FROM files WHERE workspace_id = $1 AND file_path = $2;

-- name: GetFileByID :one
SELECT * FROM files WHERE id = $1;

-- name: ListFiles :many
SELECT id, workspace_id, file_path, content_hash, size_bytes, mime_type, last_modified, updated_at
FROM files
WHERE workspace_id = $1
ORDER BY file_path;

-- name: DeleteFile :exec
DELETE FROM files WHERE workspace_id = $1 AND file_path = $2;

-- name: GetFileContent :one
SELECT content FROM files WHERE workspace_id = $1 AND file_path = $2;

-- name: UpsertFileMetadata :exec
INSERT INTO file_metadata (file_id, format, parsed_blocks, properties, word_count)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (file_id)
DO UPDATE SET
    format = EXCLUDED.format,
    parsed_blocks = EXCLUDED.parsed_blocks,
    properties = EXCLUDED.properties,
    word_count = EXCLUDED.word_count,
    last_parsed = NOW();

-- name: GetFileMetadata :one
SELECT * FROM file_metadata WHERE file_id = $1;

-- name: CreateSyncOperation :one
INSERT INTO sync_operations (workspace_id, file_id, operation_type, client_id, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateSyncOperationStatus :exec
UPDATE sync_operations
SET status = $2, error_message = $3
WHERE id = $1;

-- name: GetSyncOperations :many
SELECT * FROM sync_operations
WHERE workspace_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: CreateFileVersion :exec
INSERT INTO file_versions (file_id, version_number, content_hash, content)
VALUES ($1, $2, $3, $4);

-- name: GetFileVersions :many
SELECT * FROM file_versions
WHERE file_id = $1
ORDER BY version_number DESC
LIMIT $2;

-- name: GetWorkspaceStorageUsage :one
SELECT
    w.storage_limit_bytes,
    w.storage_used_bytes,
    COUNT(f.id) as file_count,
    COALESCE(SUM(f.size_bytes), 0) as actual_storage_used
FROM workspaces w
LEFT JOIN files f ON w.id = f.workspace_id
WHERE w.id = $1
GROUP BY w.id, w.storage_limit_bytes, w.storage_used_bytes;
