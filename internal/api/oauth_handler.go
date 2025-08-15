package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/duckonomy/noture/internal/db"
	"github.com/duckonomy/noture/internal/domain"
	"github.com/duckonomy/noture/pkg/logger"
	"github.com/duckonomy/noture/pkg/oauth"
	"github.com/duckonomy/noture/pkg/pgconv"
	"github.com/google/uuid"
)

type OAuthHandler struct {
	queries      *db.Queries
	googleConfig *oauth.GoogleOAuthConfig
	githubConfig *oauth.GitHubOAuthConfig
	log          *logger.Logger
	// TODO: Redis
	pendingAuth map[string]*PendingAuthSession
}

type PendingAuthSession struct {
	State      string
	DeviceCode string
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

type DeviceAuthRequest struct {
	DeviceName string `json:"device_name,omitempty"`
}

type DeviceAuthResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

type AuthCallbackResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	RedirectURL string `json:"redirect_url,omitempty"`
}

func NewOAuthHandler(queries *db.Queries) *OAuthHandler {
	log := logger.New()

	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if googleClientID == "" || googleClientSecret == "" {
		log.Warn("Google OAuth credentials not configured",
			"client_id_set", googleClientID != "",
			"client_secret_set", googleClientSecret != "")
	}

	githubClientID := os.Getenv("GITHUB_CLIENT_ID")
	githubClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")

	if githubClientID == "" || githubClientSecret == "" {
		log.Warn("GitHub OAuth credentials not configured",
			"client_id_set", githubClientID != "",
			"client_secret_set", githubClientSecret != "")
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8090"
	}

	googleRedirectURL := baseURL + "/auth/google/callback"
	githubRedirectURL := baseURL + "/auth/github/callback"

	return &OAuthHandler{
		queries:      queries,
		googleConfig: oauth.NewGoogleOAuthConfig(googleClientID, googleClientSecret, googleRedirectURL),
		githubConfig: oauth.NewGitHubOAuthConfig(githubClientID, githubClientSecret, githubRedirectURL, log),
		log:          log,
		pendingAuth:  make(map[string]*PendingAuthSession),
	}
}

func (h *OAuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/device", h.StartDeviceAuth)
	mux.HandleFunc("GET /auth/device/poll", h.PollDeviceAuth)

	mux.HandleFunc("GET /auth/google/login", h.GoogleLogin)
	mux.HandleFunc("GET /auth/google/callback", h.GoogleCallback)

	mux.HandleFunc("GET /auth/github/login", h.GitHubLogin)
	mux.HandleFunc("GET /auth/github/callback", h.GitHubCallback)
}

func (h *OAuthHandler) StartDeviceAuth(w http.ResponseWriter, r *http.Request) {
	h.log.Info("Starting device authentication flow")

	var req DeviceAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.WithError(err).Error("Failed to decode device auth request")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	deviceCode, err := generateRandomCode(32)
	if err != nil {
		h.log.WithError(err).Error("Failed to generate device code")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	userCode, err := generateUserCode()
	if err != nil {
		h.log.WithError(err).Error("Failed to generate user code")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	expiresAt := time.Now().Add(10 * time.Minute)
	h.pendingAuth[deviceCode] = &PendingAuthSession{
		DeviceCode: deviceCode,
		CreatedAt:  time.Now(),
		ExpiresAt:  expiresAt,
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8090"
	}
	verificationURL := fmt.Sprintf("%s/auth/verify?code=%s", baseURL, userCode)

	response := DeviceAuthResponse{
		DeviceCode:      deviceCode,
		UserCode:        userCode,
		VerificationURL: verificationURL,
		ExpiresIn:       600,
		Interval:        5,
	}

	h.log.Info("Device auth session created",
		"device_code", deviceCode,
		"user_code", userCode,
		"device_name", req.DeviceName)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *OAuthHandler) PollDeviceAuth(w http.ResponseWriter, r *http.Request) {
	deviceCode := r.URL.Query().Get("device_code")
	if deviceCode == "" {
		http.Error(w, "device_code is required", http.StatusBadRequest)
		return
	}

	session, exists := h.pendingAuth[deviceCode]
	if !exists {
		http.Error(w, "Invalid device code", http.StatusBadRequest)
		return
	}

	if time.Now().After(session.ExpiresAt) {
		delete(h.pendingAuth, deviceCode)
		http.Error(w, "Device code expired", http.StatusBadRequest)
		return
	}

	// TODO: check if the user has completed OAuth
	response := map[string]interface{}{
		"status": "pending",
		"message": "Waiting for user to complete authentication",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *OAuthHandler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	h.log.Info("Initiating Google OAuth flow")

	state, err := oauth.GenerateState()
	if err != nil {
		h.log.WithError(err).Error("Failed to generate OAuth state")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// TODO: Proper session-store
	authURL := h.googleConfig.GetAuthURL(state)
	h.log.Info("Redirecting to Google OAuth", "auth_url", authURL)

	response := map[string]string{
		"auth_url": authURL,
		"state":    state,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *OAuthHandler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	h.log.Info("Handling Google OAuth callback")

	code := r.URL.Query().Get("code")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		h.log.Error("OAuth error returned from Google", "error", errorParam)
		h.sendCallbackResponse(w, false, fmt.Sprintf("OAuth error: %s", errorParam), "")
		return
	}

	if code == "" {
		h.log.Error("No authorization code received")
		h.sendCallbackResponse(w, false, "No authorization code received", "")
		return
	}

	tokenResponse, err := h.googleConfig.ExchangeCodeForToken(r.Context(), code)
	if err != nil {
		h.log.WithError(err).Error("Failed to exchange code for token")
		h.sendCallbackResponse(w, false, "Failed to exchange authorization code", "")
		return
	}

	userInfo, err := h.googleConfig.GetUserInfo(r.Context(), tokenResponse.AccessToken)
	if err != nil {
		h.log.WithError(err).Error("Failed to get user info from Google")
		h.sendCallbackResponse(w, false, "Failed to retrieve user information", "")
		return
	}

	if !userInfo.VerifiedEmail {
		h.log.Warn("User email not verified", "email", userInfo.Email)
		h.sendCallbackResponse(w, false, "Email address must be verified", "")
		return
	}

	user, err := h.createOrGetUser(r.Context(), userInfo)
	if err != nil {
		h.log.WithError(err).Error("Failed to create or get user", "email", userInfo.Email)
		h.sendCallbackResponse(w, false, "Failed to process user account", "")
		return
	}

	token, err := h.generateAPIToken(r.Context(), user.ID)
	if err != nil {
		h.log.WithError(err).Error("Failed to generate API token", "user_id", user.ID)
		h.sendCallbackResponse(w, false, "Failed to generate authentication token", "")
		return
	}

	h.log.LogAuthEvent("oauth_success", user.ID.String(), "google")

	// TODO: handle device flow completion
	response := map[string]interface{}{
		"success": true,
		"message": "Authentication successful",
		"token":   token,
		"user": map[string]interface{}{
			"id":    user.ID,
			"email": user.Email,
			"tier":  user.Tier,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *OAuthHandler) createOrGetUser(ctx context.Context, userInfo *oauth.GoogleUserInfo) (*domain.User, error) {
	existingUser, err := h.queries.GetUserByEmail(ctx, userInfo.Email)
	if err == nil {
		return &domain.User{
			ID:               pgconv.PgToUUID(existingUser.ID),
			Email:            existingUser.Email,
			Tier:             domain.UserTier(existingUser.Tier),
			StorageUsedBytes: pgconv.PgToInt64(existingUser.StorageUsedBytes),
			CreatedAt:        pgconv.PgToTime(existingUser.CreatedAt),
			UpdatedAt:        pgconv.PgToTime(existingUser.UpdatedAt),
		}, nil
	}

	h.log.Info("Creating new user", "email", userInfo.Email)

	newUser, err := h.queries.CreateUser(ctx, db.CreateUserParams{
		Email:        userInfo.Email,
		PasswordHash: "",
		Tier:         db.UserTierFree,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &domain.User{
		ID:               pgconv.PgToUUID(newUser.ID),
		Email:            newUser.Email,
		Tier:             domain.UserTier(newUser.Tier),
		StorageUsedBytes: pgconv.PgToInt64(newUser.StorageUsedBytes),
		CreatedAt:        pgconv.PgToTime(newUser.CreatedAt),
		UpdatedAt:        pgconv.PgToTime(newUser.UpdatedAt),
	}, nil
}

func (h *OAuthHandler) generateAPIToken(ctx context.Context, userID uuid.UUID) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	tokenString := hex.EncodeToString(tokenBytes)

	hasher := func(data string) string {
		// TODO: use proper crypto
		return fmt.Sprintf("%x", data)
	}
	tokenHash := hasher(tokenString)

	_, err := h.queries.CreateAPIToken(ctx, db.CreateAPITokenParams{
		UserID:    pgconv.UUIDToPg(userID),
		TokenHash: tokenHash,
		Name:      "OAuth Token",
		// TODO: set expiration
		ExpiresAt: pgconv.TimePtrToPg(nil),
	})
	if err != nil {
		return "", fmt.Errorf("failed to store token: %w", err)
	}

	return tokenString, nil
}

func (h *OAuthHandler) sendCallbackResponse(w http.ResponseWriter, success bool, message, redirectURL string) {
	response := AuthCallbackResponse{
		Success:     success,
		Message:     message,
		RedirectURL: redirectURL,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func generateRandomCode(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func generateUserCode() (string, error) {
	code, err := generateRandomCode(8)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(code[:4] + "-" + code[4:8]), nil
}

func (h *OAuthHandler) GitHubLogin(w http.ResponseWriter, r *http.Request) {
	h.log.Info("Initiating GitHub OAuth flow")

	state, err := oauth.GenerateState()
	if err != nil {
		h.log.WithError(err).Error("Failed to generate OAuth state")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	authURL := h.githubConfig.GetAuthURL(state)
	h.log.Info("Redirecting to GitHub OAuth", "auth_url", authURL)

	response := map[string]string{
		"auth_url": authURL,
		"state":    state,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *OAuthHandler) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	h.log.Info("Handling GitHub OAuth callback")

	code := r.URL.Query().Get("code")
	errorParam := r.URL.Query().Get("error")

	if errorParam != "" {
		h.log.Error("OAuth error returned from GitHub", "error", errorParam)
		h.sendCallbackResponse(w, false, fmt.Sprintf("OAuth error: %s", errorParam), "")
		return
	}

	if code == "" {
		h.log.Error("No authorization code received")
		h.sendCallbackResponse(w, false, "No authorization code received", "")
		return
	}

	tokenResponse, err := h.githubConfig.ExchangeCodeForToken(r.Context(), code)
	if err != nil {
		h.log.WithError(err).Error("Failed to exchange code for token")
		h.sendCallbackResponse(w, false, "Failed to exchange authorization code", "")
		return
	}

	userInfo, err := h.githubConfig.GetUserInfo(r.Context(), tokenResponse.AccessToken)
	if err != nil {
		h.log.WithError(err).Error("Failed to get user info from GitHub")
		h.sendCallbackResponse(w, false, "Failed to retrieve user information", "")
		return
	}

	if userInfo.Email == "" {
		h.log.Warn("No email address found for GitHub user", "login", userInfo.Login)
		h.sendCallbackResponse(w, false, "Email address is required for authentication", "")
		return
	}

	googleUserInfo := &oauth.GoogleUserInfo{
		Email:         userInfo.Email,
		Name:          userInfo.Name,
		VerifiedEmail: true,
	}

	user, err := h.createOrGetUser(r.Context(), googleUserInfo)
	if err != nil {
		h.log.WithError(err).Error("Failed to create or get user", "email", userInfo.Email)
		h.sendCallbackResponse(w, false, "Failed to process user account", "")
		return
	}

	token, err := h.generateAPIToken(r.Context(), user.ID)
	if err != nil {
		h.log.WithError(err).Error("Failed to generate API token", "user_id", user.ID)
		h.sendCallbackResponse(w, false, "Failed to generate authentication token", "")
		return
	}

	h.log.LogAuthEvent("oauth_success", user.ID.String(), "github")

	response := map[string]interface{}{
		"success": true,
		"message": "Authentication successful",
		"token":   token,
		"user": map[string]interface{}{
			"id":    user.ID,
			"email": user.Email,
			"tier":  user.Tier,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
