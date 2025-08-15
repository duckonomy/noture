package testutil

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/duckonomy/noture/internal/db"
	"github.com/duckonomy/noture/internal/domain"
	"github.com/duckonomy/noture/pkg/pgconv"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type SimpleTestData struct {
	FreeUserID      uuid.UUID
	PremiumUserID   uuid.UUID
	FreeWorkspaceID uuid.UUID
	FreeUserToken   string
}

func CreateSimpleTestData(t *testing.T, queries *db.Queries) *SimpleTestData {
	t.Helper()
	ctx := context.Background()

	freeUser, err := queries.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("free-%s@example.com", uuid.New().String()[:8]),
		PasswordHash: "hashed_password",
		Tier:         "free",
	})
	require.NoError(t, err)

	premiumUser, err := queries.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("premium-%s@example.com", uuid.New().String()[:8]),
		PasswordHash: "hashed_password",
		Tier:         "premium",
	})
	require.NoError(t, err)

	freeWorkspace, err := queries.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		UserID:            freeUser.ID,
		Name:              "test-workspace",
		StorageLimitBytes: domain.TierFree.GetStorageLimit(),
	})
	require.NoError(t, err)

	tokenString := fmt.Sprintf("test-token-%s", uuid.New().String()[:8])
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(tokenString)))

	_, err = queries.CreateAPIToken(ctx, db.CreateAPITokenParams{
		UserID:    freeUser.ID,
		TokenHash: tokenHash,
		Name:      "test-token",
		ExpiresAt: pgconv.TimePtrToPg(nil),
	})
	require.NoError(t, err)

	return &SimpleTestData{
		FreeUserID:      pgconv.PgToUUID(freeUser.ID),
		PremiumUserID:   pgconv.PgToUUID(premiumUser.ID),
		FreeWorkspaceID: pgconv.PgToUUID(freeWorkspace.ID),
		FreeUserToken:   tokenString,
	}
}
