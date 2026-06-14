package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// GeminiTokenProvider manages access_token for Vertex/Gemini service account accounts.
type GeminiTokenProvider struct {
	accountRepo AccountRepository
	tokenCache  GeminiTokenCache
}

func NewGeminiTokenProvider(
	accountRepo AccountRepository,
	tokenCache GeminiTokenCache,
) *GeminiTokenProvider {
	return &GeminiTokenProvider{
		accountRepo: accountRepo,
		tokenCache:  tokenCache,
	}
}

func (p *GeminiTokenProvider) GetAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is nil")
	}
	if account.Platform != PlatformGemini || account.Type != AccountTypeServiceAccount {
		return "", errors.New("not a gemini service account")
	}
	return p.getServiceAccountAccessToken(ctx, account)
}

func (p *GeminiTokenProvider) getServiceAccountAccessToken(ctx context.Context, account *Account) (string, error) {
	return getVertexServiceAccountAccessToken(ctx, p.tokenCache, account)
}

func GeminiTokenCacheKey(account *Account) string {
	if account != nil && account.Type == AccountTypeServiceAccount {
		if key, err := parseVertexServiceAccountKey(account); err == nil {
			return vertexServiceAccountCacheKey(account, key)
		}
	}
	projectID := strings.TrimSpace(account.GetCredential("project_id"))
	if projectID != "" {
		return "gemini:" + projectID
	}
	return "gemini:account:" + strconv.FormatInt(account.ID, 10)
}
