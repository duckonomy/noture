package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/duckonomy/noture/pkg/logger"
)

type GitHubOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Log          *logger.Logger
}

type GitHubUserInfo struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type GitHubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

const (
	GitHubAuthURL     = "https://github.com/login/oauth/authorize"
	GitHubTokenURL    = "https://github.com/login/oauth/access_token"
	GitHubUserURL     = "https://api.github.com/user"
	GitHubEmailURL    = "https://api.github.com/user/emails"
	GitHubScopes      = "user:email"
)

func NewGitHubOAuthConfig(clientID, clientSecret, redirectURL string, log *logger.Logger) *GitHubOAuthConfig {
	return &GitHubOAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Log:          log,
	}
}

func (g *GitHubOAuthConfig) GetAuthURL(state string) string {
	params := url.Values{
		"client_id":    {g.ClientID},
		"redirect_uri": {g.RedirectURL},
		"scope":        {GitHubScopes},
		"state":        {state},
	}

	return GitHubAuthURL + "?" + params.Encode()
}

func (g *GitHubOAuthConfig) ExchangeCodeForToken(ctx context.Context, code string) (*TokenResponse, error) {
	g.Log.Info("Exchanging GitHub authorization code for token")

	data := url.Values{
		"client_id":     {g.ClientID},
		"client_secret": {g.ClientSecret},
		"code":          {code},
		"redirect_uri":  {g.RedirectURL},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", GitHubTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		g.Log.WithError(err).Error("Failed to create GitHub token exchange request")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		g.Log.WithError(err).Error("Failed to exchange code for GitHub token")
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		g.Log.WithError(err).Error("Failed to read GitHub token response")
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		g.Log.Error("GitHub token exchange returned non-200 status",
			"status_code", resp.StatusCode,
			"response", string(body))
		return nil, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, body)
	}

	var tokenResponse TokenResponse
	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		g.Log.WithError(err).Error("Failed to parse GitHub token response")
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	g.Log.Info("Successfully exchanged code for GitHub token", "token_type", tokenResponse.TokenType)
	return &tokenResponse, nil
}

func (g *GitHubOAuthConfig) GetUserInfo(ctx context.Context, accessToken string) (*GitHubUserInfo, error) {
	g.Log.Info("Fetching user information from GitHub")

	userInfo, err := g.fetchUserInfo(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	if userInfo.Email == "" {
		email, err := g.fetchPrimaryEmail(ctx, accessToken)
		if err != nil {
			g.Log.WithError(err).Warn("Failed to fetch user email, proceeding without it")
		} else {
			userInfo.Email = email
		}
	}

	g.Log.Info("Successfully retrieved GitHub user info",
		"user_id", userInfo.ID,
		"login", userInfo.Login,
		"email", userInfo.Email)

	return userInfo, nil
}

func (g *GitHubOAuthConfig) fetchUserInfo(ctx context.Context, accessToken string) (*GitHubUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", GitHubUserURL, nil)
	if err != nil {
		g.Log.WithError(err).Error("Failed to create GitHub user info request")
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		g.Log.WithError(err).Error("Failed to fetch GitHub user info")
		return nil, fmt.Errorf("user info request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		g.Log.WithError(err).Error("Failed to read GitHub user info response")
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		g.Log.Error("GitHub user info request returned non-200 status",
			"status_code", resp.StatusCode,
			"response", string(body))
		return nil, fmt.Errorf("user info request failed with status %d: %s", resp.StatusCode, body)
	}

	var userInfo GitHubUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		g.Log.WithError(err).Error("Failed to parse GitHub user info response")
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return &userInfo, nil
}

func (g *GitHubOAuthConfig) fetchPrimaryEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", GitHubEmailURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("email request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("email request failed with status %d: %s", resp.StatusCode, body)
	}

	var emails []GitHubEmail
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", fmt.Errorf("failed to parse emails: %w", err)
	}

	for _, email := range emails {
		if email.Primary && email.Verified {
			return email.Email, nil
		}
	}

	for _, email := range emails {
		if email.Verified {
			return email.Email, nil
		}
	}

	return "", fmt.Errorf("no verified email found")
}
