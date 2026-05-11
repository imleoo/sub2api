package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// AntigravityTokenCache token cache interface.
type AntigravityTokenCache = GeminiTokenCache

// AntigravityTokenProvider manages access_token for antigravity accounts.
type AntigravityTokenProvider struct {
	accountRepo AccountRepository
	tokenCache  AntigravityTokenCache
}

func NewAntigravityTokenProvider(
	accountRepo AccountRepository,
	tokenCache AntigravityTokenCache,
) *AntigravityTokenProvider {
	return &AntigravityTokenProvider{
		accountRepo: accountRepo,
		tokenCache:  tokenCache,
	}
}

// GetAccessToken returns a valid access_token.
func (p *AntigravityTokenProvider) GetAccessToken(_ context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is nil")
	}
	if account.Platform != PlatformAntigravity {
		return "", errors.New("not an antigravity account")
	}
	return "", errors.New("antigravity platform is no longer supported")
}

func AntigravityTokenCacheKey(account *Account) string {
	projectID := strings.TrimSpace(account.GetCredential("project_id"))
	if projectID != "" {
		return "ag:" + projectID
	}
	return "ag:account:" + strconv.FormatInt(account.ID, 10)
}
