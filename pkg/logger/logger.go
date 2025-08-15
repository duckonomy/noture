package logger

import (
	"log/slog"
	"os"
)

type Logger struct {
	*slog.Logger
}

func New() *Logger {
	var handler slog.Handler

	env := os.Getenv("ENVIRONMENT")
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	return &Logger{
		Logger: slog.New(handler),
	}
}

func (l *Logger) WithUser(userID, userEmail string) *Logger {
	return &Logger{
		Logger: l.Logger.With(
			"user_id", userID,
			"user_email", userEmail,
		),
	}
}

func (l *Logger) WithWorkspace(workspaceID, workspaceName string) *Logger {
	return &Logger{
		Logger: l.Logger.With(
			"workspace_id", workspaceID,
			"workspace_name", workspaceName,
		),
	}
}

func (l *Logger) WithRequest(method, path, userAgent string) *Logger {
	return &Logger{
		Logger: l.Logger.With(
			"method", method,
			"path", path,
			"user_agent", userAgent,
		),
	}
}

func (l *Logger) WithError(err error) *Logger {
	return &Logger{
		Logger: l.Logger.With("error", err.Error()),
	}
}

func (l *Logger) LogRequest(method, path string, statusCode int, duration string) {
	l.Info("HTTP request",
		"method", method,
		"path", path,
		"status_code", statusCode,
		"duration", duration,
	)
}

func (l *Logger) LogFileOperation(operation, filePath string, sizeBytes int64) {
	l.Info("File operation",
		"operation", operation,
		"file_path", filePath,
		"size_bytes", sizeBytes,
	)
}

func (l *Logger) LogWorkspaceOperation(operation, workspaceID, workspaceName string) {
	l.Info("Workspace operation",
		"operation", operation,
		"workspace_id", workspaceID,
		"workspace_name", workspaceName,
	)
}

func (l *Logger) LogAuthEvent(event, userID, method string) {
	l.Info("Authentication event",
		"event", event,
		"user_id", userID,
		"auth_method", method,
	)
}
