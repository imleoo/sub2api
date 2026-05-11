package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

const (
	GeminiTierGoogleOneFree    = "google_one_free"
	GeminiTierGoogleAIPro      = "google_ai_pro"
	GeminiTierGoogleAIUltra    = "google_ai_ultra"
	GeminiTierGCPStandard      = "gcp_standard"
	GeminiTierGCPEnterprise    = "gcp_enterprise"
	GeminiTierAIStudioFree     = "aistudio_free"
	GeminiTierAIStudioPaid     = "aistudio_paid"
	GeminiTierGoogleOneUnknown = "google_one_unknown"
)

func canonicalGeminiTierID(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	switch lower {
	case GeminiTierGoogleOneFree, GeminiTierGoogleAIPro, GeminiTierGoogleAIUltra,
		GeminiTierGCPStandard, GeminiTierGCPEnterprise,
		GeminiTierAIStudioFree, GeminiTierAIStudioPaid,
		GeminiTierGoogleOneUnknown:
		return lower
	case "ai_premium":
		return GeminiTierGoogleAIPro
	case "google_one_unlimited":
		return GeminiTierGoogleAIUltra
	case "free", "google_one_basic", "google_one_standard":
		return GeminiTierGoogleOneFree
	case "standard", "pro", "legacy":
		return GeminiTierGCPStandard
	case "enterprise", "ultra":
		return GeminiTierGCPEnterprise
	default:
		return lower
	}
}

func canonicalGeminiTierIDForOAuthType(oauthType, tierID string) string {
	oauthType = strings.ToLower(strings.TrimSpace(oauthType))
	canonical := canonicalGeminiTierID(tierID)
	if canonical == "" {
		return ""
	}
	switch oauthType {
	case "google_one":
		switch canonical {
		case GeminiTierGoogleOneFree, GeminiTierGoogleAIPro, GeminiTierGoogleAIUltra:
			return canonical
		default:
			return ""
		}
	case "code_assist":
		switch canonical {
		case GeminiTierGCPStandard, GeminiTierGCPEnterprise:
			return canonical
		default:
			return ""
		}
	case "ai_studio":
		switch canonical {
		case GeminiTierAIStudioFree, GeminiTierAIStudioPaid:
			return canonical
		default:
			return ""
		}
	default:
		return canonical
	}
}

// GeminiTokenProvider manages access_token for Gemini Vertex service account accounts.
type GeminiTokenProvider struct {
	accountRepo   AccountRepository
	tokenCache    GeminiTokenCache
	refreshAPI    *OAuthRefreshAPI
	executor      OAuthRefreshExecutor
	refreshPolicy ProviderRefreshPolicy
}

func NewGeminiTokenProvider(
	accountRepo AccountRepository,
	tokenCache GeminiTokenCache,
) *GeminiTokenProvider {
	return &GeminiTokenProvider{
		accountRepo:   accountRepo,
		tokenCache:    tokenCache,
		refreshPolicy: GeminiProviderRefreshPolicy(),
	}
}

// SetRefreshAPI injects unified OAuth refresh API and executor.
func (p *GeminiTokenProvider) SetRefreshAPI(api *OAuthRefreshAPI, executor OAuthRefreshExecutor) {
	p.refreshAPI = api
	p.executor = executor
}

// SetRefreshPolicy injects caller-side refresh policy.
func (p *GeminiTokenProvider) SetRefreshPolicy(policy ProviderRefreshPolicy) {
	p.refreshPolicy = policy
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
