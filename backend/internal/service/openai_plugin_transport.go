package service

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

func (s *OpenAIGatewayService) SetPluginManager(manager *PluginManager) {
	s.pluginManager = manager
}

// SetTLSFingerprintProfileService 注入 TLS 指纹模板服务（构造后装配，nil 容错）。
// 用于账号显式绑定了 TLS 指纹模板（tls_fingerprint_profile_id）时复用模板。
func (s *OpenAIGatewayService) SetTLSFingerprintProfileService(svc *TLSFingerprintProfileService) {
	if s != nil {
		s.tlsFPProfileService = svc
	}
}

// resolveOpenAICodexTLSProfile 解析 OpenAI OAuth-like 出站 TLS 指纹：
//  1. 全局开关关闭（gateway.openai_codex_tls_fingerprint.enabled=false）→ nil，
//     DoWithTLS 收到 nil 自动落回 Do（完全旧行为，秒级回滚通道）
//  2. 账号非 OAuth-like（API Key 走 api.openai.com，无固定客户端形态）或显式
//     opt-out（Extra["disable_tls_fingerprint"]）→ nil
//  3. 账号显式绑定 DB 模板（tls_fingerprint_profile_id >0 / -1 随机）→ 复用模板
//  4. 默认 → 内置 OpenSSL(native-tls) 指纹（实测复现官方 codex CLI 的
//     ClientHello，与 UA 声称的 codex_cli_rs 客户端一致；无 ALPN → HTTP/1.1）
func (s *OpenAIGatewayService) resolveOpenAICodexTLSProfile(account *Account) *tlsfingerprint.Profile {
	if s == nil || s.cfg == nil || !s.cfg.Gateway.OpenAICodexTLSFingerprint.Enabled {
		return nil
	}
	if account == nil || !account.IsOpenAIOAuthLike() || account.IsOpenAITLSFingerprintDisabled() {
		return nil
	}
	if svc := s.tlsFPProfileService; svc != nil {
		if id := account.GetTLSFingerprintProfileID(); id > 0 {
			if p := svc.GetProfileByID(id); p != nil {
				return p
			}
		} else if id == -1 {
			if p := svc.getRandomProfile(); p != nil {
				return p
			}
		}
	}
	return tlsfingerprint.OpenSSLNativeProfile()
}

// doOpenAIUpstream 只在 OpenAI OAuth 能力绑定已启用时把真实请求交给插件。
// 插件返回标准 http.Response，响应解析、错误映射、SSE 和计费仍由现有核心链处理。
// 无插件时走带 TLS 指纹的出站通道：用 utls 模拟 rustls/reqwest 指纹，
// 消除 Go 默认 crypto/tls 指纹与 UA 声称的 codex_cli_rs 客户端不匹配的风控破绽。
func (s *OpenAIGatewayService) doOpenAIUpstream(request *http.Request, proxyURL string, account *Account) (*http.Response, error) {
	if s.pluginManager != nil {
		response, handled, err := s.pluginManager.RoundTripOpenAIOAuth(request.Context(), request, proxyURL, account)
		if handled {
			return response, err
		}
	}
	return s.httpUpstream.DoWithTLS(request, proxyURL, account.ID, account.Concurrency, s.resolveOpenAICodexTLSProfile(account))
}

// doOpenAIAccountTestUpstream 让 OpenAI OAuth 账号测试与真实转发使用同一插件路径。
// API Key 和未命中插件的账号保持各自原有的 HTTPUpstream 行为。
func (s *AccountTestService) doOpenAIAccountTestUpstream(
	request *http.Request,
	proxyURL string,
	account *Account,
	useTLSFallback bool,
) (*http.Response, error) {
	if s.pluginManager != nil {
		response, handled, err := s.pluginManager.RoundTripOpenAIOAuth(request.Context(), request, proxyURL, account)
		if handled {
			return response, err
		}
	}
	if useTLSFallback {
		return s.httpUpstream.DoWithTLS(
			request,
			proxyURL,
			account.ID,
			account.Concurrency,
			s.tlsFPProfileService.ResolveTLSProfile(account),
		)
	}
	return s.httpUpstream.Do(request, proxyURL, account.ID, account.Concurrency)
}