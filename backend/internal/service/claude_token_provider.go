package service

import (
	"context"
	"errors"
)

// ClaudeTokenCache token cache interface.
type ClaudeTokenCache = GeminiTokenCache

// ClaudeTokenProvider manages access_token for Claude OAuth and Vertex service account accounts.
type ClaudeTokenProvider struct {
	accountRepo   AccountRepository
	tokenCache    ClaudeTokenCache
	refreshAPI    *OAuthRefreshAPI
	executor      OAuthRefreshExecutor
	refreshPolicy ProviderRefreshPolicy
}

func NewClaudeTokenProvider(
	accountRepo AccountRepository,
	tokenCache ClaudeTokenCache,
) *ClaudeTokenProvider {
	return &ClaudeTokenProvider{
		accountRepo:   accountRepo,
		tokenCache:    tokenCache,
		refreshPolicy: ClaudeProviderRefreshPolicy(),
	}
}

// SetRefreshAPI injects unified OAuth refresh API and executor.
func (p *ClaudeTokenProvider) SetRefreshAPI(api *OAuthRefreshAPI, executor OAuthRefreshExecutor) {
	p.refreshAPI = api
	p.executor = executor
}

// SetRefreshPolicy injects caller-side refresh policy.
func (p *ClaudeTokenProvider) SetRefreshPolicy(policy ProviderRefreshPolicy) {
	p.refreshPolicy = policy
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
