package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func degradedSwitchTestConfig(enabled bool) *config.Config {
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1.0
	cfg.Gateway.OpenAIDegradedSwitch.Enabled = enabled
	cfg.Gateway.OpenAIDegradedSwitch.CooldownMinutes = 30
	return cfg
}

func degradedSwitchAccount() *Account {
	return &Account{
		ID:       7,
		Name:     "baizisu",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}
}

func degradedSwitchResult(effort string, outputTokens, reasoningTokens int) *OpenAIForwardResult {
	effortPtr := effort
	return &OpenAIForwardResult{
		RequestID:       "req-degraded-1",
		Model:           "gpt-6.1-sol",
		UpstreamModel:   "gpt-6.1-sol",
		ReasoningEffort: &effortPtr,
		Usage: OpenAIUsage{
			InputTokens:     150,
			OutputTokens:    outputTokens,
			ReasoningTokens: reasoningTokens,
		},
		Stream: true,
	}
}

func TestOpenAIUsageFromGJSON_ReasoningTokens(t *testing.T) {
	usage, ok := openAIUsageFromGJSON(gjson.Parse(
		`{"input_tokens":100,"output_tokens":300,"output_tokens_details":{"reasoning_tokens":256}}`,
	))
	require.True(t, ok)
	require.Equal(t, 256, usage.ReasoningTokens)

	usage, ok = openAIUsageFromGJSON(gjson.Parse(
		`{"input_tokens":100,"output_tokens":300,"completion_tokens_details":{"reasoning_tokens":64}}`,
	))
	require.True(t, ok)
	require.Equal(t, 64, usage.ReasoningTokens, "兼容 completion_tokens_details 回退")

	usage, ok = openAIUsageFromGJSON(gjson.Parse(`{"input_tokens":100,"output_tokens":300}`))
	require.True(t, ok)
	require.Equal(t, 0, usage.ReasoningTokens, "缺省时为 0（降智检测的关键口径）")
}

func TestIsOpenAIDegradedReasoningTurn(t *testing.T) {
	cfg := degradedSwitchTestConfig(true)
	acct := degradedSwitchAccount()

	tests := []struct {
		name    string
		cfg     *config.Config
		account *Account
		result  *OpenAIForwardResult
		want    bool
	}{
		{"高 effort 零思考", cfg, acct, degradedSwitchResult("high", 300, 0), true},
		{"xhigh 零思考", cfg, acct, degradedSwitchResult("xhigh", 300, 0), true},
		{"max 零思考", cfg, acct, degradedSwitchResult("max", 300, 0), true},
		{"effort 大小写不敏感", cfg, acct, degradedSwitchResult("XHigh", 300, 0), true},
		{"正常思考", cfg, acct, degradedSwitchResult("high", 300, 256), false},
		{"effort low", cfg, acct, degradedSwitchResult("low", 300, 0), false},
		{"effort 空", cfg, acct, degradedSwitchResult("", 300, 0), false},
		{"零输出跳过", cfg, acct, degradedSwitchResult("high", 0, 0), false},
		{"开关关闭", degradedSwitchTestConfig(false), acct, degradedSwitchResult("high", 300, 0), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isOpenAIDegradedReasoningTurn(tt.cfg, tt.account, tt.result))
		})
	}

	t.Run("apikey 账号跳过", func(t *testing.T) {
		apikeyAcct := &Account{ID: 8, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
		require.Equal(t, AccountTypeAPIKey, apikeyAcct.Type)
		require.False(t, isOpenAIDegradedReasoningTurn(cfg, apikeyAcct, degradedSwitchResult("high", 300, 0)))
	})
	t.Run("shadow 账号跳过", func(t *testing.T) {
		shadow := degradedSwitchAccount()
		shadow.ParentAccountID = i64p(1)
		require.False(t, isOpenAIDegradedReasoningTurn(cfg, shadow, degradedSwitchResult("high", 300, 0)))
	})
	t.Run("非 openai 平台跳过", func(t *testing.T) {
		grok := degradedSwitchAccount()
		grok.Platform = PlatformGrok
		require.False(t, isOpenAIDegradedReasoningTurn(cfg, grok, degradedSwitchResult("high", 300, 0)))
	})
}

func TestDegradedSwitchMarkerRequestID(t *testing.T) {
	require.Equal(t, "req-1"+degradedSwitchRequestIDSuffix, degradedSwitchMarkerRequestID("req-1"))
	require.Empty(t, degradedSwitchMarkerRequestID("  "))
	long := strings.Repeat("a", 100)
	marker := degradedSwitchMarkerRequestID(long)
	require.Len(t, marker, degradedSwitchRequestIDMaxLen)
	require.True(t, strings.HasSuffix(marker, degradedSwitchRequestIDSuffix))
}

func TestRequestTypeDegradedSwitch_ParseAndString(t *testing.T) {
	require.Equal(t, "degraded_switch", RequestTypeDegradedSwitch.String())
	require.True(t, RequestTypeDegradedSwitch.IsValid())
	parsed, err := ParseUsageRequestType("degraded_switch")
	require.NoError(t, err)
	require.Equal(t, RequestTypeDegradedSwitch, parsed)
}

type degradedSwitchAccountRepoStub struct {
	AccountRepository
	tempUnschedCalls int
	tempUnschedID    int64
	tempUnschedUntil time.Time
	tempUnschedRev   string
}

func (s *degradedSwitchAccountRepoStub) SetTempUnschedulable(_ context.Context, id int64, until time.Time, reason string) error {
	s.tempUnschedCalls++
	s.tempUnschedID = id
	s.tempUnschedUntil = until
	s.tempUnschedRev = reason
	return nil
}

func TestHandleOpenAIDegradedSwitch_BlocksSchedulingAndWritesMarker(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	acctRepo := &degradedSwitchAccountRepoStub{}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.accountRepo = acctRepo
	svc.cfg = degradedSwitchTestConfig(true)

	input := &OpenAIRecordUsageInput{
		Result:  degradedSwitchResult("xhigh", 300, 0),
		APIKey:  &APIKey{ID: 2, User: &User{ID: 1}, GroupID: i64p(3)},
		User:    &User{ID: 1},
		Account: degradedSwitchAccount(),
	}
	svc.handleOpenAIDegradedSwitch(context.Background(), input)

	// 熔断：内存 + 持久化
	require.Equal(t, 1, acctRepo.tempUnschedCalls)
	require.Equal(t, int64(7), acctRepo.tempUnschedID)
	require.Equal(t, openAIDegradedSwitchReason, acctRepo.tempUnschedRev)
	require.True(t, acctRepo.tempUnschedUntil.After(time.Now().Add(25*time.Minute)), "冷却约 30 分钟")
	require.True(t, svc.isOpenAIAccountRuntimeBlocked(input.Account), "内存熔断立即生效")

	// marker 行
	require.Equal(t, 1, usageRepo.calls)
	require.NotNil(t, usageRepo.lastLog)
	marker := usageRepo.lastLog
	require.Equal(t, RequestTypeDegradedSwitch, marker.RequestType)
	require.Equal(t, degradedSwitchMarkerRequestID("req-degraded-1"), marker.RequestID)
	require.Equal(t, 0, marker.InputTokens)
	require.Equal(t, 0, marker.OutputTokens)
	require.Zero(t, marker.TotalCost)
	require.Zero(t, marker.ActualCost)
	require.NotNil(t, marker.ReasoningEffort)
	require.Equal(t, "xhigh", *marker.ReasoningEffort)
}

func TestHandleOpenAIDegradedSwitch_DisabledNoop(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	acctRepo := &degradedSwitchAccountRepoStub{}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.accountRepo = acctRepo
	svc.cfg = degradedSwitchTestConfig(false)

	input := &OpenAIRecordUsageInput{
		Result:  degradedSwitchResult("high", 300, 0),
		APIKey:  &APIKey{ID: 2, User: &User{ID: 1}},
		User:    &User{ID: 1},
		Account: degradedSwitchAccount(),
	}
	svc.handleOpenAIDegradedSwitch(context.Background(), input)

	require.Equal(t, 0, usageRepo.calls, "开关关闭不写 marker 行")
	require.Equal(t, 0, acctRepo.tempUnschedCalls, "开关关闭不熔断")
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(input.Account))
}

func TestBuildDegradedSwitchOpsErrorInput_999(t *testing.T) {
	input := &OpenAIRecordUsageInput{
		Result:           degradedSwitchResult("high", 300, 0),
		APIKey:           &APIKey{ID: 2, User: &User{ID: 1}, GroupID: i64p(3)},
		User:             &User{ID: 1},
		Account:          degradedSwitchAccount(),
		InboundEndpoint:  "/v1/responses",
		UpstreamEndpoint: "/v1/responses",
		UserAgent:        "codex_cli_rs/0.146.0",
		IPAddress:        "1.2.3.4",
	}
	entry := buildDegradedSwitchOpsErrorInput(input, "req-degraded-1", "high")

	require.Equal(t, degradedSwitchErrorCode, entry.StatusCode)
	require.Equal(t, 999, entry.StatusCode)
	require.Equal(t, openAIDegradedSwitchReason, entry.ErrorType)
	require.Equal(t, "upstream", entry.ErrorPhase)
	require.Equal(t, "warning", entry.Severity)
	require.NotNil(t, entry.UpstreamStatusCode)
	require.Equal(t, 200, *entry.UpstreamStatusCode)
	require.Equal(t, "req-degraded-1", entry.RequestID)
	require.Equal(t, int64(7), *entry.AccountID)
	require.Equal(t, int64(1), *entry.UserID)
	require.Equal(t, int64(3), *entry.GroupID)
	require.Equal(t, PlatformOpenAI, entry.Platform)
	require.Equal(t, "gpt-6.1-sol", entry.Model)
	require.Contains(t, entry.ErrorMessage, "reasoning_tokens=0")
}

func TestRecordUsage_DegradedSwitch_WritesMarkerAlongsideNormalRow(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	acctRepo := &degradedSwitchAccountRepoStub{}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.accountRepo = acctRepo
	svc.resolver = NewModelPricingResolver(nil, svc.billingService)
	svc.cfg = degradedSwitchTestConfig(true)

	input := &OpenAIRecordUsageInput{
		Result:           degradedSwitchResult("high", 300, 0),
		APIKey:           openAIRecordUsageAPIKeyWithGroup(svc, 2, false),
		User:             &User{ID: 1},
		Account:          degradedSwitchAccount(),
		InboundEndpoint:  "/v1/responses",
		UpstreamEndpoint: "/v1/responses",
	}
	input.APIKey.User = &User{ID: 1}
	require.NoError(t, svc.RecordUsage(context.Background(), input))

	require.Equal(t, 2, usageRepo.calls, "marker 行 + 正常计费行各一条")
	first, second := usageRepo.logs[0], usageRepo.logs[1]
	require.Equal(t, RequestTypeDegradedSwitch, first.RequestType, "第一行是降智 marker")
	require.Equal(t, 0, first.OutputTokens)
	require.Zero(t, first.TotalCost)
	require.NotEqual(t, RequestTypeDegradedSwitch, second.RequestType, "第二行是正常用量行")
	require.Equal(t, 300, second.OutputTokens)
	require.Greater(t, second.TotalCost, 0.0, "正常行照常计费")
	require.Equal(t, 1, acctRepo.tempUnschedCalls)
}
