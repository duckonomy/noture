package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/duckonomy/noture/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type TestDB struct {
	conn    *pgx.Conn
	queries *db.Queries
	dbName  string
}

func NewTestDB(t *testing.T) *TestDB {
	t.Helper()

	testDBName := fmt.Sprintf("noture_test_%s", uuid.New().String()[:8])

	postgresURL := getTestDatabaseURL("postgres")
	setupConn, err := pgx.Connect(context.Background(), postgresURL)
	require.NoError(t, err)
	defer setupConn.Close(context.Background())

	_, err = setupConn.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", testDBName))
	require.NoError(t, err)

	testURL := getTestDatabaseURL(testDBName)
	conn, err := pgx.Connect(context.Background(), testURL)
	require.NoError(t, err)

	err = runMigrations(conn)
	require.NoError(t, err)

	queries := db.New(conn)

	testDB := &TestDB{
		conn:    conn,
		queries: queries,
		dbName:  testDBName,
	}

	t.Cleanup(func() {
		testDB.Close(t)
	})

	return testDB
}

func (tdb *TestDB) Conn() *pgx.Conn {
	return tdb.conn
}

func (tdb *TestDB) Queries() *db.Queries {
	return tdb.queries
}

func (tdb *TestDB) Close(t *testing.T) {
	t.Helper()

	if tdb.conn != nil {
		tdb.conn.Close(context.Background())
		time.Sleep(10 * time.Millisecond)
	}

	postgresURL := getTestDatabaseURL("postgres")
	setupConn, err := pgx.Connect(context.Background(), postgresURL)
	if err != nil {
		t.Logf("Failed to connect to postgres for cleanup: %v", err)
		return
	}
	defer setupConn.Close(context.Background())

	_, err = setupConn.Exec(context.Background(),
		`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid != pg_backend_pid()`,
		tdb.dbName)
	if err != nil {
		t.Logf("Failed to terminate connections: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	_, err = setupConn.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s", tdb.dbName))
	if err != nil {
		t.Logf("Failed to drop test database %s: %v", tdb.dbName, err)
	}
}

func getTestDatabaseURL(dbName string) string {
	testURL := os.Getenv("TEST_DATABASE_URL")
	if testURL != "" {
		return fmt.Sprintf("%s/%s?sslmode=disable", testURL, dbName)
	}
	return fmt.Sprintf("postgres://postgres:password@localhost:5432/%s?sslmode=disable", dbName)
}

func runMigrations(conn *pgx.Conn) error {
	migrationSQL := `
-- User tiers for service limits
CREATE TYPE user_tier AS ENUM ('free', 'premium', 'enterprise');

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    tier user_tier NOT NULL DEFAULT 'free',
    storage_used_bytes BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- API tokens for authentication
CREATE TABLE api_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    last_used_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Workspaces (sync directories)
CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    storage_limit_bytes BIGINT NOT NULL,
    storage_used_bytes BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Files - the source of truth (raw content)
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

-- File metadata and parsed structure (cache layer)
CREATE TABLE file_metadata (
    file_id UUID PRIMARY KEY REFERENCES files(id) ON DELETE CASCADE,
    format VARCHAR(20) NOT NULL, -- 'markdown', 'orgmode', 'plaintext'
    parsed_blocks JSONB, -- cached block structure
    properties JSONB, -- extracted properties (tags, dates, etc)
    word_count INTEGER,
    last_parsed TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Sync operations log for debugging and conflict resolution
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

-- File versions for conflict resolution (keep last N versions)
CREATE TABLE file_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id UUID NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    content BYTEA NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(file_id, version_number)
);

-- Performance indexes
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_api_tokens_user_id ON api_tokens(user_id);
CREATE INDEX idx_api_tokens_hash ON api_tokens(token_hash);
CREATE INDEX idx_workspaces_user_id ON workspaces(user_id);
CREATE INDEX idx_files_workspace_id ON files(workspace_id);
CREATE INDEX idx_files_path ON files(workspace_id, file_path);
CREATE INDEX idx_files_hash ON files(content_hash);
CREATE INDEX idx_sync_operations_workspace_id ON sync_operations(workspace_id);
CREATE INDEX idx_sync_operations_created_at ON sync_operations(created_at);
CREATE INDEX idx_file_versions_file_id ON file_versions(file_id, version_number DESC);
`

	_, err := conn.Exec(context.Background(), migrationSQL)
	return err
}
