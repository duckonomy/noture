package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/duckonomy/noture/pkg/logger"
)

type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Log          *logger.Logger
}

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	VerifiedEmail bool   `json:"verified_email"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
	IDToken      string `json:"id_token,omitempty"`
}

const (
	GoogleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL    = "https://oauth2.googleapis.com/token"
	GoogleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
	GoogleScopes      = "openid email profile"
)

func NewGoogleOAuthConfig(clientID, clientSecret, redirectURL string) *GoogleOAuthConfig {
	return &GoogleOAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Log:          logger.New(),
	}
}

func (g *GoogleOAuthConfig) GetAuthURL(state string) string {
	params := url.Values{
		"client_id":     {g.ClientID},
		"redirect_uri":  {g.RedirectURL},
		"response_type": {"code"},
		"scope":         {GoogleScopes},
		"state":         {state},
		"access_type":   {"offline"},
		"prompt":        {"consent"},
	}

	return GoogleAuthURL + "?" + params.Encode()
}

func (g *GoogleOAuthConfig) ExchangeCodeForToken(ctx context.Context, code string) (*TokenResponse, error) {
	g.Log.Info("Exchanging authorization code for token")

	data := url.Values{
		"client_id":     {g.ClientID},
		"client_secret": {g.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {g.RedirectURL},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", GoogleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		g.Log.WithError(err).Error("Failed to create token exchange request")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		g.Log.WithError(err).Error("Failed to exchange code for token")
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		g.Log.WithError(err).Error("Failed to read token response")
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		g.Log.Error("Token exchange returned non-200 status",
			"status_code", resp.StatusCode,
			"response", string(body))
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, body)
	}

	var tokenResponse TokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		g.Log.WithError(err).Error("Failed to parse token response")
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	g.Log.Info("Successfully exchanged code for token", "token_type", tokenResponse.TokenType)
	return &tokenResponse, nil
}

func (g *GoogleOAuthConfig) GetUserInfo(ctx context.Context, accessToken string) (*GoogleUserInfo, error) {
	g.Log.Info("Fetching user information from Google")

	req, err := http.NewRequestWithContext(ctx, "GET", GoogleUserInfoURL, nil)
	if err != nil {
		g.Log.WithError(err).Error("Failed to create user info request")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		g.Log.WithError(err).Error("Failed to fetch user info")
		return nil, fmt.Errorf("user info request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		g.Log.WithError(err).Error("Failed to read user info response")
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		g.Log.Error("User info request returned non-200 status",
			"status_code", resp.StatusCode,
			"response", string(body))
		return nil, fmt.Errorf("user info request failed with status %d: %s", resp.StatusCode, body)
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		g.Log.WithError(err).Error("Failed to parse user info response")
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	g.Log.Info("Successfully retrieved user info",
		"user_id", userInfo.ID,
		"email", userInfo.Email,
		"verified_email", userInfo.VerifiedEmail)

	return &userInfo, nil
}

func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random state: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
