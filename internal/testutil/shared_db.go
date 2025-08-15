package testutil

import (
	"context"
	"sync"
	"testing"

	"github.com/duckonomy/noture/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

var (
	sharedDB   *TestDB
	dbMutex    sync.Mutex
	dbInitOnce sync.Once
)

func GetSharedTestDB(t *testing.T) *TestDB {
	t.Helper()

	dbInitOnce.Do(func() {
		sharedDB = NewTestDB(t)
	})

	return sharedDB
}

func CleanupSharedDB() {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	if sharedDB != nil {
		if sharedDB.conn != nil {
			sharedDB.conn.Close(context.Background())
		}
		sharedDB = nil
		dbInitOnce = sync.Once{}
	}
}

type TxTestDB struct {
	tx      pgx.Tx
	queries *db.Queries
	conn    *pgx.Conn
}

func NewTxTestDB(t *testing.T) *TxTestDB {
	t.Helper()

	sharedDB := GetSharedTestDB(t)

	tx, err := sharedDB.conn.Begin(context.Background())
	require.NoError(t, err)

	queries := sharedDB.queries.WithTx(tx)

	testDB := &TxTestDB{
		tx:      tx,
		queries: queries,
		conn:    sharedDB.conn,
	}

	t.Cleanup(func() {
		testDB.tx.Rollback(context.Background())
	})

	return testDB
}

func (tdb *TxTestDB) Conn() *pgx.Conn {
	return tdb.conn
}

func (tdb *TxTestDB) Queries() *db.Queries {
	return tdb.queries
}

func (tdb *TxTestDB) Tx() pgx.Tx {
	return tdb.tx
}
