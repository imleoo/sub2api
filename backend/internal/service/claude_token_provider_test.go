//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClaudeTokenProvider_NilAccount(t *testing.T) {
	provider := NewClaudeTokenProvider(nil, nil)

	token, err := provider.GetAccessToken(context.Background(), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "account is nil")
	require.Empty(t, token)
}

func TestClaudeTokenProvider_WrongPlatform(t *testing.T) {
	provider := NewClaudeTokenProvider(nil, nil)
	account := &Account{
		ID:       104,
		Platform: PlatformOpenAI,
		Type:     AccountTypeServiceAccount,
	}

	token, err := provider.GetAccessToken(context.Background(), account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not an anthropic service account")
	require.Empty(t, token)
}

func TestClaudeTokenProvider_WrongAccountType(t *testing.T) {
	provider := NewClaudeTokenProvider(nil, nil)
	account := &Account{
		ID:       105,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
	}

	token, err := provider.GetAccessToken(context.Background(), account)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not an anthropic service account")
	require.Empty(t, token)
}
