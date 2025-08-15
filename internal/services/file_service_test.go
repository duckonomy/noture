package services

import (
	"context"
	"testing"
	"time"

	"github.com/duckonomy/noture/internal/domain"
	"github.com/duckonomy/noture/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileService_UploadFile(t *testing.T) {
	testDB := testutil.NewIsolatedTestDB(t)
	testData := testutil.CreateSimpleTestData(t, testDB.Queries())

	service := NewFileServiceForTesting(testDB.Queries(), testDB.Conn())
	ctx := context.Background()

	t.Run("successful file upload", func(t *testing.T) {
		content := []byte("# Test Note\n\nThis is a test markdown file.")
		req := domain.FileUploadRequest{
			WorkspaceID:  testData.FreeWorkspaceID,
			FilePath:     "test.md",
			Content:      content,
			LastModified: time.Now(),
			ClientID:     "test-client",
		}

		fileInfo, err := service.UploadFile(ctx, req, testData.FreeUserID)

		require.NoError(t, err)
		assert.Equal(t, req.WorkspaceID, fileInfo.WorkspaceID)
		assert.Equal(t, req.FilePath, fileInfo.FilePath)
		assert.Equal(t, int64(len(content)), fileInfo.SizeBytes)
		assert.Equal(t, "text/markdown", fileInfo.MimeType)
	})
}

func TestFileService_ListFiles_Simple(t *testing.T) {
	testDB := testutil.NewIsolatedTestDB(t)
	testData := testutil.CreateSimpleTestData(t, testDB.Queries())

	service := NewFileServiceForTesting(testDB.Queries(), testDB.Conn())
	ctx := context.Background()

	t.Run("list files in empty workspace", func(t *testing.T) {
		files, err := service.ListFiles(ctx, testData.FreeWorkspaceID, testData.FreeUserID)

		require.NoError(t, err)
		assert.Len(t, files, 0)
	})

	t.Run("list files after upload", func(t *testing.T) {
		content := []byte("test content")
		req := domain.FileUploadRequest{
			WorkspaceID:  testData.FreeWorkspaceID,
			FilePath:     "test.txt",
			Content:      content,
			LastModified: time.Now(),
			ClientID:     "test-client",
		}

		_, err := service.UploadFile(ctx, req, testData.FreeUserID)
		require.NoError(t, err)

		files, err := service.ListFiles(ctx, testData.FreeWorkspaceID, testData.FreeUserID)
		require.NoError(t, err)
		assert.Len(t, files, 1)
		assert.Equal(t, "test.txt", files[0].FilePath)
	})
}

func TestFileService_GetFile_Simple(t *testing.T) {
	testDB := testutil.NewIsolatedTestDB(t)
	testData := testutil.CreateSimpleTestData(t, testDB.Queries())

	service := NewFileServiceForTesting(testDB.Queries(), testDB.Conn())
	ctx := context.Background()

	t.Run("get non-existent file", func(t *testing.T) {
		_, err := service.GetFile(ctx, testData.FreeWorkspaceID, "nonexistent.txt", testData.FreeUserID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("get existing file", func(t *testing.T) {
		content := []byte("test file content")
		req := domain.FileUploadRequest{
			WorkspaceID:  testData.FreeWorkspaceID,
			FilePath:     "existing.txt",
			Content:      content,
			LastModified: time.Now(),
			ClientID:     "test-client",
		}

		_, err := service.UploadFile(ctx, req, testData.FreeUserID)
		require.NoError(t, err)

		fileInfo, err := service.GetFile(ctx, testData.FreeWorkspaceID, "existing.txt", testData.FreeUserID)

		require.NoError(t, err)
		assert.Equal(t, testData.FreeWorkspaceID, fileInfo.WorkspaceID)
		assert.Equal(t, "existing.txt", fileInfo.FilePath)
		assert.Equal(t, int64(len(content)), fileInfo.SizeBytes)
	})

	t.Run("access denied for different user", func(t *testing.T) {
		content := []byte("test file content")
		req := domain.FileUploadRequest{
			WorkspaceID:  testData.FreeWorkspaceID,
			FilePath:     "restricted.txt",
			Content:      content,
			LastModified: time.Now(),
			ClientID:     "test-client",
		}

		_, err := service.UploadFile(ctx, req, testData.FreeUserID)
		require.NoError(t, err)

		_, err = service.GetFile(ctx, testData.FreeWorkspaceID, "restricted.txt", testData.PremiumUserID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})
}

func TestFileService_GetFileContent_Simple(t *testing.T) {
	testDB := testutil.NewIsolatedTestDB(t)
	testData := testutil.CreateSimpleTestData(t, testDB.Queries())

	service := NewFileServiceForTesting(testDB.Queries(), testDB.Conn())
	ctx := context.Background()

	t.Run("get non-existent file content", func(t *testing.T) {
		_, err := service.GetFileContent(ctx, testData.FreeWorkspaceID, "nonexistent.txt", testData.FreeUserID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("get existing file content", func(t *testing.T) {
		content := []byte("# Test Markdown\n\nThis is test content with some **bold** text.")
		req := domain.FileUploadRequest{
			WorkspaceID:  testData.FreeWorkspaceID,
			FilePath:     "content-test.md",
			Content:      content,
			LastModified: time.Now(),
			ClientID:     "test-client",
		}

		_, err := service.UploadFile(ctx, req, testData.FreeUserID)
		require.NoError(t, err)

		fileContent, err := service.GetFileContent(ctx, testData.FreeWorkspaceID, "content-test.md", testData.FreeUserID)

		require.NoError(t, err)
		assert.Equal(t, content, fileContent.Content)
		assert.Equal(t, testData.FreeWorkspaceID, fileContent.WorkspaceID)
		assert.Equal(t, "content-test.md", fileContent.FilePath)
	})

	t.Run("access denied for different user", func(t *testing.T) {
		content := []byte("secret content")
		req := domain.FileUploadRequest{
			WorkspaceID:  testData.FreeWorkspaceID,
			FilePath:     "secret.txt",
			Content:      content,
			LastModified: time.Now(),
			ClientID:     "test-client",
		}

		_, err := service.UploadFile(ctx, req, testData.FreeUserID)
		require.NoError(t, err)

		_, err = service.GetFileContent(ctx, testData.FreeWorkspaceID, "secret.txt", testData.PremiumUserID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")
	})
}

func TestFileService_DeleteFile_Simple(t *testing.T) {
	testDB := testutil.NewIsolatedTestDB(t)
	testData := testutil.CreateSimpleTestData(t, testDB.Queries())

	service := NewFileServiceForTesting(testDB.Queries(), testDB.Conn())
	ctx := context.Background()

	t.Run("delete non-existent file", func(t *testing.T) {
		err := service.DeleteFile(ctx, testData.FreeWorkspaceID, "nonexistent.txt", testData.FreeUserID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("delete existing file", func(t *testing.T) {
		content := []byte("file to be deleted")
		req := domain.FileUploadRequest{
			WorkspaceID:  testData.FreeWorkspaceID,
			FilePath:     "delete-me.txt",
			Content:      content,
			LastModified: time.Now(),
			ClientID:     "test-client",
		}

		_, err := service.UploadFile(ctx, req, testData.FreeUserID)
		require.NoError(t, err)

		_, err = service.GetFile(ctx, testData.FreeWorkspaceID, "delete-me.txt", testData.FreeUserID)
		require.NoError(t, err)

		err = service.DeleteFile(ctx, testData.FreeWorkspaceID, "delete-me.txt", testData.FreeUserID)
		require.NoError(t, err)

		_, err = service.GetFile(ctx, testData.FreeWorkspaceID, "delete-me.txt", testData.FreeUserID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file not found")
	})

	t.Run("access denied for different user", func(t *testing.T) {
		content := []byte("protected file")
		req := domain.FileUploadRequest{
			WorkspaceID:  testData.FreeWorkspaceID,
			FilePath:     "protected.txt",
			Content:      content,
			LastModified: time.Now(),
			ClientID:     "test-client",
		}

		_, err := service.UploadFile(ctx, req, testData.FreeUserID)
		require.NoError(t, err)

		err = service.DeleteFile(ctx, testData.FreeWorkspaceID, "protected.txt", testData.PremiumUserID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "access denied")

		_, err = service.GetFile(ctx, testData.FreeWorkspaceID, "protected.txt", testData.FreeUserID)
		assert.NoError(t, err)
	})
}

func TestFileService_DetectFileFormat_Simple(t *testing.T) {
	testDB := testutil.NewIsolatedTestDB(t)
	service := NewFileServiceForTesting(testDB.Queries(), testDB.Conn())

	testCases := []struct {
		name     string
		filePath string
		content  []byte
		expected domain.FileFormat
	}{
		{
			name:     "markdown file with .md extension",
			filePath: "test.md",
			content:  []byte("# Test\n\nThis is markdown."),
			expected: domain.FormatMarkdown,
		},
		{
			name:     "markdown file with .markdown extension",
			filePath: "document.markdown",
			content:  []byte("## Header\n\n*italic*"),
			expected: domain.FormatMarkdown,
		},
		{
			name:     "org-mode file",
			filePath: "notes.org",
			content:  []byte("* TODO Task\n** Subtask"),
			expected: domain.FormatOrgMode,
		},
		{
			name:     "plain text file",
			filePath: "readme.txt",
			content:  []byte("Just plain text"),
			expected: domain.FormatPlainText,
		},
		{
			name:     "file without extension",
			filePath: "README",
			content:  []byte("File without extension"),
			expected: domain.FormatPlainText,
		},
		{
			name:     "unknown extension",
			filePath: "data.json",
			content:  []byte(`{"key": "value"}`),
			expected: domain.FormatPlainText,
		},
		{
			name:     "case insensitive markdown",
			filePath: "TEST.MD",
			content:  []byte("# Uppercase extension"),
			expected: domain.FormatMarkdown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			format := service.DetectFileFormat(tc.filePath, tc.content)
			assert.Equal(t, tc.expected, format)
		})
	}
}
