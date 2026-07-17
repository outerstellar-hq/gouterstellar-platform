package security

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/outerstellar-hq/gouterstellar-platform/internal/model"
	"github.com/outerstellar-hq/gouterstellar-platform/internal/persistence"
)

const (
	apiKeyRandomBytes = 16
	apiKeyPrefixLen   = 8
)

type ApiKeyService struct {
	apiKeyRepo  persistence.ApiKeyRepository
	userRepo    persistence.UserRepository
	tokenHasher TokenHasher
}

func NewApiKeyService(apiKeyRepo persistence.ApiKeyRepository, userRepo persistence.UserRepository, tokenPepper string) *ApiKeyService {
	return &ApiKeyService{
		apiKeyRepo:  apiKeyRepo,
		userRepo:    userRepo,
		tokenHasher: NewTokenHasher(tokenPepper),
	}
}

func (s *ApiKeyService) CreateApiKey(ctx context.Context, userID uuid.UUID, name string) (*model.CreateApiKeyResponse, error) {
	if strings.TrimSpace(name) == "" {
		return nil, &model.ValidationError{Errors: []string{"API key name is required"}}
	}

	rawBytes := make([]byte, apiKeyRandomBytes)
	if _, err := rand.Read(rawBytes); err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}
	rawKey := "osk_" + hex.EncodeToString(rawBytes)

	keyHash := s.tokenHasher.Hash(rawKey)
	keyPrefix := rawKey[:apiKeyPrefixLen]

	_, err := s.apiKeyRepo.CreateApiKey(ctx, userID, keyHash, keyPrefix, name)
	if err != nil {
		return nil, fmt.Errorf("failed to store API key: %w", err)
	}

	return &model.CreateApiKeyResponse{
		Key:       rawKey,
		Name:      name,
		KeyPrefix: keyPrefix,
	}, nil
}

func (s *ApiKeyService) AuthenticateApiKey(ctx context.Context, rawKey string) (*model.User, error) {
	keyHash := s.tokenHasher.Hash(rawKey)

	apiKey, err := s.apiKeyRepo.FindByKeyHash(ctx, keyHash)
	if err != nil {
		slog.Warn("API key not found or invalid", "error", err)
		return nil, fmt.Errorf("invalid API key")
	}

	if err := s.apiKeyRepo.UpdateLastUsed(ctx, apiKey.ID); err != nil {
		slog.Warn("Failed to update API key last used", "keyId", apiKey.ID, "error", err)
	}

	pltUser, err := s.userRepo.FindByID(ctx, apiKey.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found for API key: %w", err)
	}

	user := PltUserToModel(pltUser)
	if !user.Enabled {
		return nil, fmt.Errorf("invalid API key")
	}
	return user, nil
}

func (s *ApiKeyService) ListApiKeys(ctx context.Context, userID uuid.UUID) ([]model.ApiKeySummary, error) {
	keys, err := s.apiKeyRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}

	summaries := make([]model.ApiKeySummary, len(keys))
	for i, k := range keys {
		summaries[i] = pltApiKeySummaryToModel(k)
	}
	return summaries, nil
}

func (s *ApiKeyService) DeleteApiKey(ctx context.Context, userID uuid.UUID, keyID int64) error {
	_, err := s.apiKeyRepo.DeleteApiKey(ctx, keyID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}
	return nil
}
