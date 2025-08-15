package testutil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/duckonomy/noture/internal/domain"
	"github.com/stretchr/testify/require"
)

func CreateAuthContext(user domain.User, token domain.APIToken) *domain.AuthContext {
	return &domain.AuthContext{
		User:      user,
		Token:     token,
		UserID:    user.ID,
		UserEmail: user.Email,
		UserTier:  user.Tier,
	}
}

func AuthenticatedRequest(t *testing.T, method, url string, authCtx *domain.AuthContext) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, url, nil)
	ctx := context.WithValue(req.Context(), "auth", authCtx)

	return req.WithContext(ctx)
}

func AssertJSONResponse(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int, target interface{}) {
	t.Helper()

	require.Equal(t, expectedStatus, recorder.Code)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	if target != nil {
		err := json.NewDecoder(recorder.Body).Decode(target)
		require.NoError(t, err)
	}
}

func AssertErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, expectedStatus int, expectedError string) {
	t.Helper()

	require.Equal(t, expectedStatus, recorder.Code)
	require.Contains(t, recorder.Body.String(), expectedError)
}
