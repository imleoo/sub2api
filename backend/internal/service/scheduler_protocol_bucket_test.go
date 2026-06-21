//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// ── P5-6: protocol 桶切换单测 ────────────────────────────────────────────

// protocolBucketAccountRepoSpy 记录哪个查询方法被调用。
type protocolBucketAccountRepoSpy struct {
	accountRepoStub
	calledProtocol   string
	calledPlatform   string
	returnedAccounts []Account
}

func (s *protocolBucketAccountRepoSpy) ListSchedulableByGroupIDAndOutboundProtocol(
	_ context.Context, _ int64, protocol string,
) ([]Account, error) {
	s.calledProtocol = "group:" + protocol
	return s.returnedAccounts, nil
}

func (s *protocolBucketAccountRepoSpy) ListSchedulableByOutboundProtocol(
	_ context.Context, protocol string,
) ([]Account, error) {
	s.calledProtocol = protocol
	return s.returnedAccounts, nil
}

func (s *protocolBucketAccountRepoSpy) ListSchedulableUngroupedByOutboundProtocol(
	_ context.Context, protocol string,
) ([]Account, error) {
	s.calledProtocol = "ungrouped:" + protocol
	return s.returnedAccounts, nil
}

func (s *protocolBucketAccountRepoSpy) ListSchedulableByPlatform(
	_ context.Context, platform string,
) ([]Account, error) {
	s.calledPlatform = platform
	return s.returnedAccounts, nil
}

func (s *protocolBucketAccountRepoSpy) ListSchedulableByGroupIDAndPlatform(
	_ context.Context, _ int64, platform string,
) ([]Account, error) {
	s.calledPlatform = "group:" + platform
	return s.returnedAccounts, nil
}

func (s *protocolBucketAccountRepoSpy) ListSchedulableUngroupedByPlatform(
	_ context.Context, platform string,
) ([]Account, error) {
	s.calledPlatform = "ungrouped:" + platform
	return s.returnedAccounts, nil
}

// TestProtocolBucketEnabled_UsesProtocolQuery ProtocolBucketEnabled=true 时走协议维度查询。
func TestProtocolBucketEnabled_UsesProtocolQuery(t *testing.T) {
	spy := &protocolBucketAccountRepoSpy{
		returnedAccounts: []Account{{ID: 1}},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.ProtocolBucketEnabled = true

	svc := &SchedulerSnapshotService{
		accountRepo: spy,
		cfg:         cfg,
	}

	bucket := SchedulerBucket{GroupID: 0, Platform: PlatformAnthropic, Mode: SchedulerModeSingle}
	accounts, err := svc.loadAccountsFromDB(context.Background(), bucket, false)

	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, "ungrouped:anthropic_messages", spy.calledProtocol, "应调用协议维度查询（ungrouped 路径）")
	require.Empty(t, spy.calledPlatform, "不应调用平台维度查询")
}

// TestProtocolBucketDisabled_UsesPlatformQuery ProtocolBucketEnabled=false（默认）时走平台维度查询。
func TestProtocolBucketDisabled_UsesPlatformQuery(t *testing.T) {
	spy := &protocolBucketAccountRepoSpy{
		returnedAccounts: []Account{{ID: 2}},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.ProtocolBucketEnabled = false

	svc := &SchedulerSnapshotService{
		accountRepo: spy,
		cfg:         cfg,
	}

	bucket := SchedulerBucket{GroupID: 0, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}
	accounts, err := svc.loadAccountsFromDB(context.Background(), bucket, false)

	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Empty(t, spy.calledProtocol, "不应调用协议维度查询")
	require.Equal(t, "ungrouped:"+PlatformOpenAI, spy.calledPlatform, "应调用平台维度查询（ungrouped 路径）")
}

// TestProtocolBucketEnabled_GenericFallsBackToPlatform
// generic 平台 protocol="" 时即使开启 ProtocolBucket 也降级到 platform 查询。
func TestProtocolBucketEnabled_GenericFallsBackToPlatform(t *testing.T) {
	spy := &protocolBucketAccountRepoSpy{
		returnedAccounts: []Account{{ID: 3}},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.ProtocolBucketEnabled = true

	svc := &SchedulerSnapshotService{
		accountRepo: spy,
		cfg:         cfg,
	}

	bucket := SchedulerBucket{GroupID: 0, Platform: PlatformGeneric, Mode: SchedulerModeSingle}
	_, err := svc.loadAccountsFromDB(context.Background(), bucket, false)

	require.NoError(t, err)
	// generic 的 platform 查询路径被调用（platform="generic"）
	require.Empty(t, spy.calledProtocol, "generic 不走协议维度")
	require.Equal(t, "ungrouped:"+PlatformGeneric, spy.calledPlatform, "generic 降级到 platform 查询")
}

// TestProtocolBucketEnabled_LingjingFallsBackToPlatform lingjing 同上。
func TestProtocolBucketEnabled_LingjingFallsBackToPlatform(t *testing.T) {
	spy := &protocolBucketAccountRepoSpy{
		returnedAccounts: []Account{{ID: 4}},
	}
	cfg := &config.Config{}
	cfg.Gateway.Scheduling.ProtocolBucketEnabled = true

	svc := &SchedulerSnapshotService{
		accountRepo: spy,
		cfg:         cfg,
	}

	bucket := SchedulerBucket{GroupID: 0, Platform: PlatformLingjing, Mode: SchedulerModeSingle}
	_, err := svc.loadAccountsFromDB(context.Background(), bucket, false)

	require.NoError(t, err)
	require.Empty(t, spy.calledProtocol, "lingjing 不走协议维度")
	require.Equal(t, "ungrouped:"+PlatformLingjing, spy.calledPlatform)
}
