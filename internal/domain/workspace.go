package domain

import (
	"time"

	"github.com/google/uuid"
)

type UserTier string

const (
	TierFree       UserTier = "free"
	TierPremium    UserTier = "premium"
	TierEnterprise UserTier = "enterprise"
)

func (t UserTier) GetStorageLimit() int64 {
	switch t {
	case TierFree:
		return 100 * 1024 * 1024
	case TierPremium:
		return 10 * 1024 * 1024 * 1024
	case TierEnterprise:
		return 100 * 1024 * 1024 * 1024
	default:
		return 100 * 1024 * 1024
	}
}

func (t UserTier) GetMaxWorkspaces() int {
	switch t {
	case TierFree:
		return 1
	case TierPremium:
		return 10
	case TierEnterprise:
		return -1 // Unlimited
	default:
		return 1
	}
}

type User struct {
	ID               uuid.UUID `json:"id"`
	Email            string    `json:"email"`
	Tier             UserTier  `json:"tier"`
	StorageUsedBytes int64     `json:"storage_used_bytes"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Workspace struct {
	ID                uuid.UUID `json:"id"`
	UserID            uuid.UUID `json:"user_id"`
	Name              string    `json:"name"`
	StorageLimitBytes int64     `json:"storage_limit_bytes"`
	StorageUsedBytes  int64     `json:"storage_used_bytes"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type CreateWorkspaceRequest struct {
	Name string `json:"name" validate:"required,min=1,max=255"`
}

type APIToken struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	TokenHash   string     `json:"-"` // Never expose in JSON
	Name        string     `json:"name"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type CreateTokenRequest struct {
	Name      string     `json:"name" validate:"required,min=1,max=100"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type AuthContext struct {
	User      User      `json:"user"`
	Token     APIToken  `json:"token"`
	UserID    uuid.UUID `json:"user_id"`
	UserEmail string    `json:"user_email"`
	UserTier  UserTier  `json:"user_tier"`
}
