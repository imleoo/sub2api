package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const openAIPlatformAlphaSearchURL = "https://api.openai.com/v1/alpha/search"

// ForwardAlphaSearch proxies Codex standalone web search without binding the
// evolving alpha request or response schema.
//
// 返回值约定：仅当上游返回 2xx（一次真实成功的搜索）时返回非 nil 的
// *OpenAIForwardResult（WebSearchCalls=1，供按次计费）；上游错误被原样透传
// 给客户端时返回 (nil, nil)，不产生计费。
func (s *OpenAIGatewayService) ForwardAlphaSearch(ctx context.Context, c *gin.Context, account *Account, body []byte) (*OpenAIForwardResult, error) {
	if s == nil || c == nil || account == nil {
		return nil, fmt.Errorf("service, context, and account are required")
	}
	modelResult := gjson.GetBytes(body, "model")
	requestedModel := strings.TrimSpace(modelResult.String())
	if modelResult.Type != gjson.String || requestedModel == "" {
		return nil, fmt.Errorf("model is required")
	}

	upstreamModel := normalizeOpenAIModelForUpstream(account, account.GetMappedModel(requestedModel))
	if upstreamModel != "" && upstreamModel != requestedModel {
		body = ReplaceModelInBody(body, upstreamModel)
	}
	sanitizedBody, err := sanitizeOpenAIAlphaSearchBody(body)
	if err != nil {
		return nil, fmt.Errorf("sanitize alpha search request body: %w", err)
	}
	body = sanitizedBody

	token, _, err := s.GetAccessToken(ctx, account)
	if err != nil {
		return nil, err
	}

	req, err := s.buildOpenAIAlphaSearchRequest(ctx, c, account, body, token)
	if err != nil {
		return nil, err
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	upstreamStart := time.Now()
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, true)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, fmt.Errorf("read alpha search response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		upstreamMessage := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(respBody)))
		if s.shouldFailoverOpenAIUpstreamResponse(resp.StatusCode, upstreamMessage, respBody) ||
			isOpenAIAlphaSearchEndpointUnsupported(account, resp.StatusCode) {
			resp.Body = io.NopCloser(bytes.NewReader(respBody))
			// alpha/search 是独立的工具端点，单次 401 不能证明账号的模型调用
			// 凭据全局失效。若沿用通用 401 逻辑，PAT 会因没有 refresh_token
			// 被永久标记为 error；历史导入且缺少 auth_mode 标记的 at- token 也会
			// 漏过 PAT 类型判断。这里仍允许本次请求换号，但不修改任何账号状态；
			// 真正的凭据失效由普通 Responses 请求或 whoami 校验判定。
			shouldDisable := false
			if shouldApplyOpenAIAlphaSearchAccountErrorSideEffects(resp.StatusCode) {
				shouldDisable = s.handleFailoverSideEffects(ctx, resp, account, respBody, upstreamModel)
			}
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: !shouldDisable && account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
			}
		}
	}

	if !account.IsShadow() {
		s.UpdateCodexUsageSnapshotFromHeaders(ctx, account.ID, resp.Header)
	}
	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), resp.Header, s.responseHeaderFilter)
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, respBody)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		// 非 2xx（错误/重定向）已原样透传给客户端：不是一次成功的搜索，不计费。
		return nil, nil
	}
	return &OpenAIForwardResult{
		RequestID:      strings.TrimSpace(resp.Header.Get("x-request-id")),
		Model:          requestedModel,
		UpstreamModel:  upstreamModel,
		Duration:       time.Since(upstreamStart),
		WebSearchCalls: 1,
	}, nil
}

func (s *OpenAIGatewayService) buildOpenAIAlphaSearchRequest(ctx context.Context, c *gin.Context, account *Account, body []byte, token string) (*http.Request, error) {
	clientBeta := ""
	if c != nil {
		clientBeta = c.GetHeader("OpenAI-Beta")
	}
	req, err := s.buildUpstreamRequestOpenAIPassthrough(ctx, c, account, body, token)
	if err != nil {
		return nil, err
	}

	targetURL, err := s.openAIAlphaSearchURL(account)
	if err != nil {
		return nil, err
	}
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("parse alpha search URL: %w", err)
	}
	if c != nil && c.Request != nil && c.Request.URL != nil {
		query := parsedURL.Query()
		for key, values := range c.Request.URL.Query() {
			for _, value := range values {
				query.Add(key, value)
			}
		}
		parsedURL.RawQuery = query.Encode()
	}
	req.URL = parsedURL
	req.Header.Set("Accept", "application/json")
	if clientBeta == "" {
		req.Header.Del("OpenAI-Beta")
	}
	// fork：OAuth 账号类型已删除（功能 35），仅透传客户端 Version 头。
	if version := strings.TrimSpace(c.GetHeader("Version")); version != "" {
		req.Header.Set("Version", version)
	}
	return req, nil
}

var openAIAlphaSearchUnsupportedBodyFields = [...]string{
	// Codex alpha/search 是 SearchRequest 独立协议，不是 /responses 子请求。
	// 新版 Codex/第三方代理可能把 Responses 公共字段误带到搜索请求里；上游
	// alpha/search 会对这些字段返回 Unknown parameter（例如 prompt_cache_key）。
	"prompt_cache_key",
	"prompt_cache_retention",
}

func sanitizeOpenAIAlphaSearchBody(body []byte) ([]byte, error) {
	if len(body) == 0 {
		return body, nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil || obj == nil {
		return body, nil
	}
	changed := false
	for _, field := range openAIAlphaSearchUnsupportedBodyFields {
		if _, ok := obj[field]; ok {
			delete(obj, field)
			changed = true
		}
	}
	if !changed {
		return body, nil
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// isOpenAIAlphaSearchEndpointUnsupported 识别「API key 上游没有实现
// /v1/alpha/search 端点」的响应。404/405 不在通用 failover 状态集里（模型
// 调用中的 404 通常是用户请求问题），但对这个独立工具端点而言，它几乎只
// 意味着所选上游（官方平台或第三方中转）不提供该端点——应换号重试，而
// 不是把 404 透传给客户端。
func isOpenAIAlphaSearchEndpointUnsupported(account *Account, statusCode int) bool {
	if account == nil || account.Type != AccountTypeAPIKey {
		return false
	}
	return statusCode == http.StatusNotFound || statusCode == http.StatusMethodNotAllowed
}

func shouldApplyOpenAIAlphaSearchAccountErrorSideEffects(statusCode int) bool {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusNotFound, http.StatusMethodNotAllowed:
		// 401：工具端点的 access enforcement 不代表凭据全局失效；
		// 404/405：端点不存在只说明该上游不支持独立搜索，账号本身健康。
		// 两类都只换号，不写账号错误状态。
		return false
	default:
		return true
	}
}

func (s *OpenAIGatewayService) openAIAlphaSearchURL(account *Account) (string, error) {
	if account == nil {
		return "", fmt.Errorf("account is required")
	}
	// fork：OAuth 账号类型已删除（功能 35），仅支持官方 APIKey 账号。
	switch account.Type {
	case AccountTypeAPIKey:
		baseURL := account.GetOpenAIBaseURL()
		if baseURL == "" {
			return openAIPlatformAlphaSearchURL, nil
		}
		validatedURL, err := s.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return "", err
		}
		return buildOpenAIEndpointURL(validatedURL, "/v1/alpha/search"), nil
	default:
		return "", fmt.Errorf("unsupported OpenAI account type: %s", account.Type)
	}
}
