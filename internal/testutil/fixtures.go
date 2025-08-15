package testutil

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/duckonomy/noture/internal/db"
	"github.com/duckonomy/noture/internal/domain"
	"github.com/duckonomy/noture/pkg/pgconv"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type TestFixtures struct {
	testDB *TestDB

	FreeUser      db.User
	PremiumUser   db.User
	EnterpriseUser db.User

	FreeUserToken      db.ApiToken
	PremiumUserToken   db.ApiToken
	EnterpriseUserToken db.ApiToken

	FreeWorkspace      db.Workspace
	PremiumWorkspace   db.Workspace
	EnterpriseWorkspace db.Workspace
}

func NewTestFixtures(t *testing.T, testDB *TestDB) *TestFixtures {
	t.Helper()

	ctx := context.Background()
	queries := testDB.Queries()

	fixtures := &TestFixtures{testDB: testDB}

	freeUser, err := queries.CreateUser(ctx, db.CreateUserParams{
		Email:        "free@example.com",
		PasswordHash: "hashed_password",
		Tier:         "free",
	})
	require.NoError(t, err)
	fixtures.FreeUser = freeUser

	premiumUser, err := queries.CreateUser(ctx, db.CreateUserParams{
		Email:        "premium@example.com",
		PasswordHash: "hashed_password",
		Tier:         "premium",
	})
	require.NoError(t, err)
	fixtures.PremiumUser = premiumUser

	enterpriseUser, err := queries.CreateUser(ctx, db.CreateUserParams{
		Email:        "enterprise@example.com",
		PasswordHash: "hashed_password",
		Tier:         "enterprise",
	})
	require.NoError(t, err)
	fixtures.EnterpriseUser = enterpriseUser

	freeToken := "free-token-123"
	freeHash := hashToken(freeToken)
	freeUserToken, err := queries.CreateAPIToken(ctx, db.CreateAPITokenParams{
		UserID:    freeUser.ID,
		TokenHash: freeHash,
		Name:      "free-token",
		ExpiresAt: pgconv.TimePtrToPg(nil), // Never expires
	})
	require.NoError(t, err)
	fixtures.FreeUserToken = freeUserToken

	premiumToken := "premium-token-123"
	premiumHash := hashToken(premiumToken)
	premiumUserToken, err := queries.CreateAPIToken(ctx, db.CreateAPITokenParams{
		UserID:    premiumUser.ID,
		TokenHash: premiumHash,
		Name:      "premium-token",
		ExpiresAt: pgconv.TimePtrToPg(nil),
	})
	require.NoError(t, err)
	fixtures.PremiumUserToken = premiumUserToken

	enterpriseToken := "enterprise-token-123"
	enterpriseHash := hashToken(enterpriseToken)
	enterpriseUserToken, err := queries.CreateAPIToken(ctx, db.CreateAPITokenParams{
		UserID:    enterpriseUser.ID,
		TokenHash: enterpriseHash,
		Name:      "enterprise-token",
		ExpiresAt: pgconv.TimePtrToPg(nil),
	})
	require.NoError(t, err)
	fixtures.EnterpriseUserToken = enterpriseUserToken

	freeWorkspace, err := queries.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		UserID:            freeUser.ID,
		Name:              "free-workspace",
		StorageLimitBytes: domain.TierFree.GetStorageLimit(),
	})
	require.NoError(t, err)
	fixtures.FreeWorkspace = freeWorkspace

	premiumWorkspace, err := queries.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		UserID:            premiumUser.ID,
		Name:              "premium-workspace",
		StorageLimitBytes: domain.TierPremium.GetStorageLimit(),
	})
	require.NoError(t, err)
	fixtures.PremiumWorkspace = premiumWorkspace

	enterpriseWorkspace, err := queries.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		UserID:            enterpriseUser.ID,
		Name:              "enterprise-workspace",
		StorageLimitBytes: domain.TierEnterprise.GetStorageLimit(),
	})
	require.NoError(t, err)
	fixtures.EnterpriseWorkspace = enterpriseWorkspace

	return fixtures
}

func (f *TestFixtures) GetTokenString(tokenType string) string {
	switch tokenType {
	case "free":
		return "free-token-123"
	case "premium":
		return "premium-token-123"
	case "enterprise":
		return "enterprise-token-123"
	default:
		return ""
	}
}

func (f *TestFixtures) CreateTestFile(t *testing.T, workspaceID uuid.UUID, filePath string, content []byte) db.File {
	t.Helper()

	ctx := context.Background()
	queries := f.testDB.Queries()

	hash := sha256.Sum256(content)
	contentHash := fmt.Sprintf("%x", hash)

	file, err := queries.UpsertFile(ctx, db.UpsertFileParams{
		WorkspaceID:  pgconv.UUIDToPg(workspaceID),
		FilePath:     filePath,
		ContentHash:  contentHash,
		Content:      content,
		SizeBytes:    int64(len(content)),
		MimeType:     pgconv.StringToPg("text/plain"),
		LastModified: pgconv.TimeToPg(time.Now()),
	})
	require.NoError(t, err)

	return file
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", hash)
}
