package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestShouldFlattenOpenAIResponsesNamespaces 命名空间摊平原本只对 Codex OAuth
// 账号生效；OAuth 账号类型已随订阅逆向清理移除（功能 35），
// shouldFlattenOpenAIResponsesNamespaces 现在对任意入参恒返回 false，
// 行为与上游 apikey 账号一致。
func TestShouldFlattenOpenAIResponsesNamespaces(t *testing.T) {
	apiKey := &Account{Type: AccountTypeAPIKey}

	tests := []struct {
		name               string
		account            *Account
		transport          OpenAIUpstreamTransport
		passthroughEnabled bool
	}{
		{name: "apikey_http", account: apiKey, transport: OpenAIUpstreamTransportHTTPSSE},
		{name: "apikey_http_passthrough", account: apiKey, transport: OpenAIUpstreamTransportHTTPSSE, passthroughEnabled: true},
		{name: "apikey_wsv2", account: apiKey, transport: OpenAIUpstreamTransportResponsesWebsocketV2},
		{name: "nil_account", account: nil, transport: OpenAIUpstreamTransportHTTPSSE},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.False(t, shouldFlattenOpenAIResponsesNamespaces(tt.account, tt.transport, tt.passthroughEnabled))
		})
	}
}
