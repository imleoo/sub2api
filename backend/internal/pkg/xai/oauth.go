package xai

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

// fork：仅官方 xAI apikey 接入，OAuth 订阅代理授权流程（PKCE/device flow/token
// 刷新等）已随功能 35 删除。本文件只保留 apikey 转发路径需要的 base_url 校验与
// 端点拼接工具。

const (
	DefaultBaseURL = "https://api.x.ai/v1"

	EnvBaseURL                 = "XAI_BASE_URL"
	EnvAllowUnsafeURLOverrides = "XAI_ALLOW_UNSAFE_URL_OVERRIDES"
)

// *.api.x.ai 覆盖 xAI 区域端点（us-east-1/us-west-2/eu-west-1 等），
// 运营方可在端点间手动切换以规避单点不可用。
var baseURLAllowedHosts = []string{"api.x.ai", "*.api.x.ai", "cli-chat-proxy.grok.com"}

func EffectiveBaseURL(override string) string {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return strings.TrimRight(trimmed, "/")
	}
	return strings.TrimRight(envOrDefault(EnvBaseURL, DefaultBaseURL), "/")
}

func ValidatedBaseURL(override string) (string, error) {
	return ValidateBaseURL(EffectiveBaseURL(override))
}

// BaseURLValidator applies the caller's outbound URL trust policy before xAI
// endpoint paths are appended. The service layer uses this for API-key
// accounts so the global security.url_allowlist policy remains the single
// source of truth.
type BaseURLValidator func(string) (string, error)

func validatedBaseURLWithValidator(override string, validator BaseURLValidator) (string, error) {
	if validator == nil {
		return ValidatedBaseURL(override)
	}
	raw := EffectiveBaseURL(override)
	validated, err := validator(raw)
	if err != nil {
		return "", err
	}
	return normalizeKnownBaseURLPath(validated)
}

func ValidateBaseURL(raw string) (string, error) {
	if AllowUnsafeURLOverrides() {
		return urlvalidator.ValidateURLFormat(raw, true)
	}
	normalized, err := urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{
		AllowPrivate: false,
	})
	if err != nil {
		return "", err
	}
	return normalizeKnownBaseURLPath(normalized)
}

// ValidateTrustedBaseURL 校验 raw 必须命中官方主机白名单（baseURLAllowedHosts），
// 用于官方主机专属的转发路径（不接受任意自定义 base_url）。
func ValidateTrustedBaseURL(raw string) (string, error) {
	if AllowUnsafeURLOverrides() {
		return urlvalidator.ValidateURLFormat(raw, true)
	}
	normalized, err := urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{
		AllowedHosts:     baseURLAllowedHosts,
		RequireAllowlist: true,
		AllowPrivate:     false,
	})
	if err != nil {
		return "", err
	}
	return normalizeKnownBaseURLPath(normalized)
}

// normalizeKnownBaseURLPath 规范化 base URL 的 path 部分：
//   - 官方主机固定使用 /v1 前缀（空 path 自动补齐，其余 path 拒绝）；
//   - 其他主机保留管理员配置的任意 path 前缀（第三方转发地址常见
//     /xxx/v1 之类的路由前缀），空 path 仍按惯例补 /v1。
//
// 所有主机统一禁止 userinfo/query/fragment，并去除尾部斜杠。
func normalizeKnownBaseURLPath(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("invalid base URL")
	}
	if parsed.User != nil {
		return "", errors.New("base URL must not include userinfo")
	}
	if parsed.ForceQuery || parsed.RawQuery != "" {
		return "", errors.New("base URL must not include a query")
	}
	if parsed.Fragment != "" {
		return "", errors.New("base URL must not include a fragment")
	}
	path := strings.TrimRight(parsed.Path, "/")
	if path == "" {
		parsed.Path = "/v1"
		parsed.RawPath = ""
		return strings.TrimRight(parsed.String(), "/"), nil
	}
	if path != "/v1" && IsOfficialBaseURLHost(parsed.Hostname()) {
		return "", fmt.Errorf("base URL path must be /v1")
	}
	parsed.Path = path
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

// IsOfficialBaseURLHost 报告 host 是否属于官方 API / 区域 API / CLI 网关主机。
func IsOfficialBaseURLHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	for _, allowed := range baseURLAllowedHosts {
		if strings.HasPrefix(allowed, "*.") {
			suffix := strings.TrimPrefix(allowed, "*.")
			if host == suffix || strings.HasSuffix(host, "."+suffix) {
				return true
			}
			continue
		}
		if host == allowed {
			return true
		}
	}
	return false
}

// IsParseableBaseURL 报告 raw 是否能解析出 host。
// 供读取路径判定存量脏数据：无法解析的值应回落默认端点，而不是把流量发往未定义目标。
func IsParseableBaseURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	parsed, err := url.Parse(trimmed)
	return err == nil && parsed.Host != ""
}

// IsOfficialBaseURL 报告 raw 是否指向官方主机（api.x.ai / *.api.x.ai 区域端点 / CLI 网关），
// 容忍存量凭证中的历史变体（大小写、显式 443 端口、百分号编码 path 等）。
// 无法解析的值一并视为官方，调用方据此回落默认端点而不是把流量发往未定义目标。
func IsOfficialBaseURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return true
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return true
	}
	return IsOfficialBaseURLHost(parsed.Hostname())
}

func AllowUnsafeURLOverrides() bool {
	return envBool(EnvAllowUnsafeURLOverrides)
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func BuildResponsesURL(baseURL string) (string, error) {
	return BuildResponsesURLWithValidator(baseURL, nil)
}

func BuildResponsesURLWithValidator(baseURL string, validator BaseURLValidator) (string, error) {
	validatedBaseURL, err := validatedBaseURLWithValidator(baseURL, validator)
	if err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	return validatedBaseURL + "/responses", nil
}

func BuildChatCompletionsURL(baseURL string) (string, error) {
	return BuildChatCompletionsURLWithValidator(baseURL, nil)
}

func BuildChatCompletionsURLWithValidator(baseURL string, validator BaseURLValidator) (string, error) {
	validatedBaseURL, err := validatedBaseURLWithValidator(baseURL, validator)
	if err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	return validatedBaseURL + "/chat/completions", nil
}

func BuildImagesGenerationsURL(baseURL string) (string, error) {
	return BuildImagesGenerationsURLWithValidator(baseURL, nil)
}

func BuildImagesGenerationsURLWithValidator(baseURL string, validator BaseURLValidator) (string, error) {
	validatedBaseURL, err := validatedBaseURLWithValidator(baseURL, validator)
	if err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	return validatedBaseURL + "/images/generations", nil
}

func BuildImagesEditsURL(baseURL string) (string, error) {
	return BuildImagesEditsURLWithValidator(baseURL, nil)
}

func BuildImagesEditsURLWithValidator(baseURL string, validator BaseURLValidator) (string, error) {
	validatedBaseURL, err := validatedBaseURLWithValidator(baseURL, validator)
	if err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	return validatedBaseURL + "/images/edits", nil
}

func BuildVideosGenerationsURL(baseURL string) (string, error) {
	return BuildVideosGenerationsURLWithValidator(baseURL, nil)
}

func BuildVideosGenerationsURLWithValidator(baseURL string, validator BaseURLValidator) (string, error) {
	validatedBaseURL, err := validatedBaseURLWithValidator(baseURL, validator)
	if err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	return validatedBaseURL + "/videos/generations", nil
}

func BuildVideosEditsURL(baseURL string) (string, error) {
	return BuildVideosEditsURLWithValidator(baseURL, nil)
}

func BuildVideosEditsURLWithValidator(baseURL string, validator BaseURLValidator) (string, error) {
	validatedBaseURL, err := validatedBaseURLWithValidator(baseURL, validator)
	if err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	return validatedBaseURL + "/videos/edits", nil
}

func BuildVideosExtensionsURL(baseURL string) (string, error) {
	return BuildVideosExtensionsURLWithValidator(baseURL, nil)
}

func BuildVideosExtensionsURLWithValidator(baseURL string, validator BaseURLValidator) (string, error) {
	validatedBaseURL, err := validatedBaseURLWithValidator(baseURL, validator)
	if err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	return validatedBaseURL + "/videos/extensions", nil
}

func BuildVideoURL(baseURL, requestID string) (string, error) {
	return BuildVideoURLWithValidator(baseURL, requestID, nil)
}

func BuildVideoURLWithValidator(baseURL, requestID string, validator BaseURLValidator) (string, error) {
	validatedBaseURL, err := validatedBaseURLWithValidator(baseURL, validator)
	if err != nil {
		return "", fmt.Errorf("invalid base url: %w", err)
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return "", fmt.Errorf("request id is required")
	}
	return validatedBaseURL + "/videos/" + url.PathEscape(requestID), nil
}
