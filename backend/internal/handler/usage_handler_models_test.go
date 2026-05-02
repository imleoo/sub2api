package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type accountRepoForModelList struct {
	service.AccountRepository
	accounts []*service.Account
}

func (r accountRepoForModelList) GetByIDs(context.Context, []int64) ([]*service.Account, error) {
	return r.accounts, nil
}

func TestUsageHandlerCollectWhitelistedModelsForAccounts_HidesNonWhitelistedModels(t *testing.T) {
	handler := &UsageHandler{
		accountRepo: accountRepoForModelList{
			accounts: []*service.Account{
				{
					ID:       1,
					Status:   service.StatusActive,
					Platform: service.PlatformOpenAI,
					Credentials: map[string]any{
						"model_mapping": map[string]any{
							"gpt-5.4": "gpt-5.4-openai",
						},
					},
				},
				{
					ID:          2,
					Status:      service.StatusActive,
					Platform:    service.PlatformAnthropic,
					Credentials: map[string]any{},
				},
			},
		},
	}
	modelsByID := map[string]service.ModelInfo{
		"gpt-5.4":          {ID: "gpt-5.4", LiteLLMProvider: "openai"},
		"claude-sonnet-4":  {ID: "claude-sonnet-4", LiteLLMProvider: "anthropic"},
		"gemini-3-pro":     {ID: "gemini-3-pro", LiteLLMProvider: "google"},
		"gpt-5.4-openai":   {ID: "gpt-5.4-openai", LiteLLMProvider: "openai"},
		"gpt-5.4-mini":     {ID: "gpt-5.4-mini", LiteLLMProvider: "openai"},
		"gpt-5.4-mini-alt": {ID: "gpt-5.4-mini-alt", LiteLLMProvider: "openai"},
	}

	got := handler.collectWhitelistedModelsForAccounts(context.Background(), []int64{1, 2}, modelsByID)

	require.Equal(t, []service.ModelInfo{{ID: "gpt-5.4", LiteLLMProvider: "openai"}}, got)
}

func TestUsageHandlerCollectWhitelistedModelsForAccounts_ExpandsWildcardAndMappedAlias(t *testing.T) {
	handler := &UsageHandler{
		accountRepo: accountRepoForModelList{
			accounts: []*service.Account{
				{
					ID:       1,
					Status:   service.StatusActive,
					Platform: service.PlatformOpenAI,
					Credentials: map[string]any{
						"model_mapping": map[string]any{
							"gpt-5.4-mini*":     "gpt-5.4-mini*",
							"gpt-5.3-codex":     "gpt-5.3-codex-spark",
							"not-priced-custom": "not-priced-upstream",
						},
					},
				},
			},
		},
	}
	modelsByID := map[string]service.ModelInfo{
		"gpt-5.4-mini":        {ID: "gpt-5.4-mini", LiteLLMProvider: "openai"},
		"gpt-5.4-mini-alt":    {ID: "gpt-5.4-mini-alt", LiteLLMProvider: "openai"},
		"gpt-5.3-codex-spark": {ID: "gpt-5.3-codex-spark", LiteLLMProvider: "openai"},
		"gpt-5.4":             {ID: "gpt-5.4", LiteLLMProvider: "openai"},
	}

	got := handler.collectWhitelistedModelsForAccounts(context.Background(), []int64{1}, modelsByID)

	require.Equal(t, []service.ModelInfo{
		{ID: "gpt-5.3-codex", LiteLLMProvider: "openai"},
		{ID: "gpt-5.4-mini", LiteLLMProvider: "openai"},
		{ID: "gpt-5.4-mini-alt", LiteLLMProvider: "openai"},
	}, got)
}
