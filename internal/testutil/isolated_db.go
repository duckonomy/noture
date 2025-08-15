package testutil

import (
	"context"
	"fmt"
	"testing"

	"github.com/duckonomy/noture/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type IsolatedTestDB struct {
	conn    *pgx.Conn
	queries *db.Queries
	dbName  string
}

func NewIsolatedTestDB(t *testing.T) *IsolatedTestDB {
	t.Helper()

	dbName := fmt.Sprintf("noture_test_%s", uuid.New().String()[:8])

	postgresURL := getTestDatabaseURL("postgres")
	adminConn, err := pgx.Connect(context.Background(), postgresURL)
	require.NoError(t, err)

	_, err = adminConn.Exec(context.Background(), fmt.Sprintf("CREATE DATABASE %s", dbName))
	require.NoError(t, err)
	adminConn.Close(context.Background())

	testURL := getTestDatabaseURL(dbName)
	conn, err := pgx.Connect(context.Background(), testURL)
	require.NoError(t, err)

	err = runMigrations(conn)
	require.NoError(t, err)

	queries := db.New(conn)

	testDB := &IsolatedTestDB{
		conn:    conn,
		queries: queries,
		dbName:  dbName,
	}

	t.Cleanup(func() {
		testDB.Cleanup()
	})

	return testDB
}

func (idb *IsolatedTestDB) Conn() *pgx.Conn {
	return idb.conn
}

func (idb *IsolatedTestDB) Queries() *db.Queries {
	return idb.queries
}

func (idb *IsolatedTestDB) Cleanup() {
	if idb.conn != nil {
		idb.conn.Close(context.Background())
	}

	postgresURL := getTestDatabaseURL("postgres")
	adminConn, err := pgx.Connect(context.Background(), postgresURL)
	if err == nil {
		adminConn.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s", idb.dbName))
		adminConn.Close(context.Background())
	}
}
