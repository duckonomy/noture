-- +goose Up
CREATE TYPE user_tier AS ENUM ('free', 'premium', 'enterprise');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    tier user_tier NOT NULL DEFAULT 'free',
    storage_used_bytes BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE api_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    last_used_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    storage_limit_bytes BIGINT NOT NULL,
    storage_used_bytes BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    file_path VARCHAR(1000) NOT NULL, -- relative path within workspace
    content_hash VARCHAR(64) NOT NULL, -- SHA-256 of content
    content BYTEA NOT NULL, -- raw file content
    size_bytes BIGINT NOT NULL,
    mime_type VARCHAR(100) DEFAULT 'text/plain',
    last_modified TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(workspace_id, file_path)
);

-- cache layer
CREATE TABLE file_metadata (
    file_id UUID PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
    format VARCHAR(20) NOT NULL, -- 'markdown', 'orgmode', 'plaintext'
    parsed_blocks JSONB, -- cached block structure
    properties JSONB, -- extracted properties (tags, dates, etc)
    word_count INTEGER,
    last_parsed TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE sync_operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    file_id UUID REFERENCES files(id) ON DELETE SET NULL,
    operation_type VARCHAR(20) NOT NULL, -- 'upload', 'download', 'delete', 'conflict'
    client_id VARCHAR(100), -- identify different clients
    status VARCHAR(20) NOT NULL, -- 'pending', 'success', 'failed'
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE file_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    content BYTEA NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(file_id, version_number)
);

CREATE INDEX idx_users_email ON users(email);

-- CREATE UNIQUE INDEX idx_users_email ON users(email);

CREATE INDEX idx_api_tokens_user_id ON api_tokens(user_id);
CREATE INDEX idx_api_tokens_hash ON api_tokens(token_hash);
CREATE INDEX idx_workspaces_user_id ON workspaces(user_id);
CREATE INDEX idx_files_workspace_id ON files(workspace_id);

-- NOTE: Redundant
CREATE INDEX idx_files_path ON files(workspace_id, file_path);
CREATE INDEX idx_files_hash ON files(content_hash);
CREATE INDEX idx_sync_operations_workspace_id ON sync_operations(workspace_id);
CREATE INDEX idx_sync_operations_created_at ON sync_operations(created_at);
CREATE INDEX idx_file_versions_file_id ON file_versions(file_id, version_number DESC);

-- +goose Down
DROP TABLE IF EXISTS file_versions;
DROP TABLE IF EXISTS sync_operations;
DROP TABLE IF EXISTS file_content;
DROP TABLE IF EXISTS synced_files;
DROP TABLE IF EXISTS file_metadata;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS user_tier;
