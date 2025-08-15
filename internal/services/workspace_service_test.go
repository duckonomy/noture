package services

import (
	"context"
	"testing"

	"github.com/duckonomy/noture/internal/domain"
	"github.com/duckonomy/noture/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceService_GetWorkspacesByUser_Simple(t *testing.T) {
	testDB := testutil.NewIsolatedTestDB(t)
	testData := testutil.CreateSimpleTestData(t, testDB.Queries())

	service := NewWorkspaceService(testDB.Queries())
	ctx := context.Background()

	t.Run("get workspaces for user", func(t *testing.T) {
		workspaces, err := service.GetWorkspacesByUser(ctx, testData.FreeUserID)

		require.NoError(t, err)
		assert.Len(t, workspaces, 1)
		assert.Equal(t, "test-workspace", workspaces[0].Name)
		assert.Equal(t, testData.FreeUserID, workspaces[0].UserID)
	})
}

func TestWorkspaceService_GetWorkspaceByID_Simple(t *testing.T) {
	testDB := testutil.NewIsolatedTestDB(t)
	testData := testutil.CreateSimpleTestData(t, testDB.Queries())

	service := NewWorkspaceService(testDB.Queries())
	ctx := context.Background()

	t.Run("get existing workspace", func(t *testing.T) {
		workspace, err := service.GetWorkspaceByID(ctx, testData.FreeWorkspaceID, testData.FreeUserID)

		require.NoError(t, err)
		assert.Equal(t, testData.FreeWorkspaceID, workspace.ID)
		assert.Equal(t, "test-workspace", workspace.Name)
	})

	t.Run("access denied for different user", func(t *testing.T) {
		_, err := service.GetWorkspaceByID(ctx, testData.FreeWorkspaceID, testData.PremiumUserID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})
}

func TestWorkspaceService_CreateWorkspace_Simple(t *testing.T) {
	testDB := testutil.NewIsolatedTestDB(t)
	testData := testutil.CreateSimpleTestData(t, testDB.Queries())

	service := NewWorkspaceService(testDB.Queries())
	ctx := context.Background()

	t.Run("create workspace successfully", func(t *testing.T) {
		req := domain.CreateWorkspaceRequest{
			Name: "New Test Workspace",
		}

		workspace, err := service.CreateWorkspace(ctx, req, testData.PremiumUserID, domain.TierPremium)

		require.NoError(t, err)
		assert.Equal(t, "New Test Workspace", workspace.Name)
		assert.Equal(t, testData.PremiumUserID, workspace.UserID)
		assert.True(t, workspace.StorageLimitBytes > 0)
	})

	t.Run("create workspace with duplicate name for same user", func(t *testing.T) {
		req1 := domain.CreateWorkspaceRequest{
			Name: "Duplicate Workspace",
		}

		_, err := service.CreateWorkspace(ctx, req1, testData.PremiumUserID, domain.TierPremium)
		require.NoError(t, err)

		req2 := domain.CreateWorkspaceRequest{
			Name: "Duplicate Workspace",
		}

		_, err = service.CreateWorkspace(ctx, req2, testData.PremiumUserID, domain.TierPremium)
		require.NoError(t, err)
	})

	t.Run("create workspace exceeding tier limits", func(t *testing.T) {
		req := domain.CreateWorkspaceRequest{
			Name: "Extra Workspace",
		}

		_, err := service.CreateWorkspace(ctx, req, testData.FreeUserID, domain.TierFree)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "workspace limit reached")
	})
}

func TestWorkspaceService_GetWorkspaceStorageInfo_Simple(t *testing.T) {
	testDB := testutil.NewIsolatedTestDB(t)
	testData := testutil.CreateSimpleTestData(t, testDB.Queries())

	service := NewWorkspaceService(testDB.Queries())
	ctx := context.Background()

	t.Run("get storage info for empty workspace", func(t *testing.T) {
		storageInfo, err := service.GetWorkspaceStorageInfo(ctx, testData.FreeWorkspaceID, testData.FreeUserID)

		require.NoError(t, err)
		assert.Equal(t, int64(0), storageInfo.StorageUsedBytes)
		assert.True(t, storageInfo.StorageLimitBytes > 0)
		assert.Equal(t, int64(0), storageInfo.FileCount)
	})

	t.Run("access denied for different user", func(t *testing.T) {
		_, err := service.GetWorkspaceStorageInfo(ctx, testData.FreeWorkspaceID, testData.PremiumUserID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})
}
