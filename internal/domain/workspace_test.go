package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserTier_GetStorageLimit(t *testing.T) {
	tests := []struct {
		name     string
		tier     UserTier
		expected int64
	}{
		{
			name:     "free tier storage limit",
			tier:     TierFree,
			expected: 100 * 1024 * 1024,
		},
		{
			name:     "premium tier storage limit",
			tier:     TierPremium,
			expected: 10 * 1024 * 1024 * 1024,
		},
		{
			name:     "enterprise tier storage limit",
			tier:     TierEnterprise,
			expected: 100 * 1024 * 1024 * 1024,
		},
		{
			name:     "invalid tier defaults to free",
			tier:     UserTier("invalid"),
			expected: 100 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tier.GetStorageLimit()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUserTier_GetMaxWorkspaces(t *testing.T) {
	tests := []struct {
		name     string
		tier     UserTier
		expected int
	}{
		{
			name:     "free tier workspace limit",
			tier:     TierFree,
			expected: 1,
		},
		{
			name:     "premium tier workspace limit",
			tier:     TierPremium,
			expected: 10,
		},
		{
			name:     "enterprise tier workspace limit",
			tier:     TierEnterprise,
			expected: -1, // Unlimited
		},
		{
			name:     "invalid tier defaults to free",
			tier:     UserTier("invalid"),
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tier.GetMaxWorkspaces()
			assert.Equal(t, tt.expected, result)
		})
	}
}
