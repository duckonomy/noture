package auth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"

	"github.com/duckonomy/noture/internal/db"
	"github.com/duckonomy/noture/internal/domain"
	"github.com/duckonomy/noture/pkg/pgconv"
)

type AuthMiddleware struct {
	queries *db.Queries
}

func NewAuthMiddleware(queries *db.Queries) *AuthMiddleware {
	return &AuthMiddleware{
		queries: queries,
	}
}

func (a *AuthMiddleware) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			http.Error(w, "Missing token", http.StatusUnauthorized)
			return
		}

		hash := sha256.Sum256([]byte(token))
		tokenHash := fmt.Sprintf("%x", hash)

		tokenInfo, err := a.queries.GetTokenByHash(r.Context(), tokenHash)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		err = a.queries.UpdateTokenLastUsed(r.Context(), tokenInfo.ID)
		if err != nil {
			// Don't fail the request for this, just log it
			// TODO: add proper logging
		}

		authCtx := &domain.AuthContext{
			User: domain.User{
				ID:               pgconv.PgToUUID(tokenInfo.UserID),
				Email:            tokenInfo.Email,
				Tier:             domain.UserTier(tokenInfo.Tier),
				StorageUsedBytes: 0, // TODO: get from user table if needed
			},
			Token: domain.APIToken{
				ID:         pgconv.PgToUUID(tokenInfo.ID),
				UserID:     pgconv.PgToUUID(tokenInfo.UserID),
				Name:       tokenInfo.Name,
				LastUsedAt: pgconv.PgToTimePtr(tokenInfo.LastUsedAt),
				ExpiresAt:  pgconv.PgToTimePtr(tokenInfo.ExpiresAt),
				CreatedAt:  pgconv.PgToTime(tokenInfo.CreatedAt),
			},
			UserID:    pgconv.PgToUUID(tokenInfo.UserID),
			UserEmail: tokenInfo.Email,
			UserTier:  domain.UserTier(tokenInfo.Tier),
		}

		ctx := context.WithValue(r.Context(), "auth", authCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func (a *AuthMiddleware) OptionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			next.ServeHTTP(w, r)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		hash := sha256.Sum256([]byte(token))
		tokenHash := fmt.Sprintf("%x", hash)

		tokenInfo, err := a.queries.GetTokenByHash(r.Context(), tokenHash)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		a.queries.UpdateTokenLastUsed(r.Context(), tokenInfo.ID)

		authCtx := &domain.AuthContext{
			User: domain.User{
				ID:    pgconv.PgToUUID(tokenInfo.UserID),
				Email: tokenInfo.Email,
				Tier:  domain.UserTier(tokenInfo.Tier),
			},
			UserID:    pgconv.PgToUUID(tokenInfo.UserID),
			UserEmail: tokenInfo.Email,
			UserTier:  domain.UserTier(tokenInfo.Tier),
		}

		ctx := context.WithValue(r.Context(), "auth", authCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func (a *AuthMiddleware) RequireTier(tier domain.UserTier) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			authCtx := r.Context().Value("auth")
			if authCtx == nil {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			auth := authCtx.(*domain.AuthContext)

			userTierLevel := getTierLevel(auth.UserTier)
			requiredTierLevel := getTierLevel(tier)

			if userTierLevel < requiredTierLevel {
				http.Error(w, "Insufficient tier level", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		}
	}
}

func getTierLevel(tier domain.UserTier) int {
	switch tier {
	case domain.TierFree:
		return 1
	case domain.TierPremium:
		return 2
	case domain.TierEnterprise:
		return 3
	default:
		return 0
	}
}
