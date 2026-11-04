package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

func newOpenAICodexTLSProfileTestService(enabled bool) *OpenAIGatewayService {
	cfg := &config.Config{}
	cfg.Gateway.OpenAICodexTLSFingerprint = config.GatewayOpenAICodexTLSFingerprintConfig{Enabled: enabled}
	return &OpenAIGatewayService{cfg: cfg}
}

func oauthAccountWithTLSExtra(extra map[string]any) *Account {
	return &Account{
		ID:          11,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"chatgpt_account_id": "acct-1"},
		Extra:       extra,
	}
}

func TestResolveOpenAICodexTLSProfile(t *testing.T) {
	// 全局开关关闭 → nil（旧行为，回滚通道）
	svc := newOpenAICodexTLSProfileTestService(false)
	require.Nil(t, svc.resolveOpenAICodexTLSProfile(oauthAccountWithTLSExtra(nil)))

	// 全局开关开启 + OAuth 账号 → 内置 OpenSSL(native-tls) 指纹
	svc = newOpenAICodexTLSProfileTestService(true)
	profile := svc.resolveOpenAICodexTLSProfile(oauthAccountWithTLSExtra(nil))
	require.NotNil(t, profile)
	require.Equal(t, tlsfingerprint.ProfileNameOpenSSLNative, profile.Name)
	// 与实测一致：真实 codex 不发 ALPN 扩展（HTTP/1.1）
	require.Empty(t, profile.ALPNProtocols)

	// 账号显式 opt-out → nil
	svc = newOpenAICodexTLSProfileTestService(true)
	require.Nil(t, svc.resolveOpenAICodexTLSProfile(oauthAccountWithTLSExtra(map[string]any{"disable_tls_fingerprint": true})))

	// API Key 账号（非 OAuth-like，走 api.openai.com）→ nil
	svc = newOpenAICodexTLSProfileTestService(true)
	apiKeyAccount := &Account{ID: 12, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	require.Nil(t, svc.resolveOpenAICodexTLSProfile(apiKeyAccount))

	// 绑定了不存在的模板 ID → 回退内置 OpenSSL 指纹
	svc = newOpenAICodexTLSProfileTestService(true)
	svc.SetTLSFingerprintProfileService(&TLSFingerprintProfileService{localCache: map[int64]*model.TLSFingerprintProfile{}})
	bound := oauthAccountWithTLSExtra(map[string]any{"tls_fingerprint_profile_id": int64(999)})
	profile = svc.resolveOpenAICodexTLSProfile(bound)
	require.NotNil(t, profile)
	require.Equal(t, tlsfingerprint.ProfileNameOpenSSLNative, profile.Name)
}
