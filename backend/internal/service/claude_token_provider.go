package service

import (
	"context"
	"errors"
)

// ClaudeTokenCache token cache interface.
type ClaudeTokenCache = GeminiTokenCache

// ClaudeTokenProvider manages access_token for Vertex service account accounts.
type ClaudeTokenProvider struct {
	accountRepo AccountRepository
	tokenCache  ClaudeTokenCache
}

func NewClaudeTokenProvider(
	accountRepo AccountRepository,
	tokenCache ClaudeTokenCache,
) *ClaudeTokenProvider {
	return &ClaudeTokenProvider{
		accountRepo: accountRepo,
		tokenCache:  tokenCache,
	}
}

// GetAccessToken returns a valid access_token for service_account type.
func (p *ClaudeTokenProvider) GetAccessToken(ctx context.Context, account *Account) (string, error) {
	if account == nil {
		return "", errors.New("account is nil")
	}
	if account.Platform != PlatformAnthropic || account.Type != AccountTypeServiceAccount {
		return "", errors.New("not an anthropic service account")
	}
	return p.getServiceAccountAccessToken(ctx, account)
}

func (p *ClaudeTokenProvider) getServiceAccountAccessToken(ctx context.Context, account *Account) (string, error) {
	return getVertexServiceAccountAccessToken(ctx, p.tokenCache, account)
}
