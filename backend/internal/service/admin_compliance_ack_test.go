//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// adminComplianceAckRepoStub lets tests force an arbitrary lookup error,
// which adminComplianceRepoStub (in admin_compliance_test.go) cannot do since
// it only ever returns ErrSettingNotFound for missing keys.
type adminComplianceAckRepoStub struct {
	values   map[string]string
	forceErr error
}

func (r *adminComplianceAckRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	if r.forceErr != nil {
		return nil, r.forceErr
	}
	if value, ok := r.values[key]; ok {
		return &Setting{Key: key, Value: value}, nil
	}
	return nil, ErrSettingNotFound
}

func (r *adminComplianceAckRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	setting, err := r.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (r *adminComplianceAckRepoStub) Set(ctx context.Context, key, value string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}

func (r *adminComplianceAckRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *adminComplianceAckRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	return nil
}

func (r *adminComplianceAckRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *adminComplianceAckRepoStub) Delete(ctx context.Context, key string) error {
	delete(r.values, key)
	return nil
}

// fork 定制回归断言：合规门控恒禁用，IsAdminComplianceAcknowledged 在"从未确认"
// 与"已确认历史记录"两条分支下都必须返回 true（因为 Required 恒为 false），
// 不能因为上游改动 GetAdminComplianceStatus 的字段/校验路径而重新要求确认。
func TestIsAdminComplianceAcknowledged_NoHistoryRecord_ReturnsTrue(t *testing.T) {
	svc := NewSettingService(&adminComplianceAckRepoStub{}, &config.Config{})

	acknowledged, err := svc.IsAdminComplianceAcknowledged(context.Background(), 7)
	require.NoError(t, err)
	require.True(t, acknowledged)
}

func TestIsAdminComplianceAcknowledged_WithPriorAcceptance_ReturnsTrue(t *testing.T) {
	repo := &adminComplianceAckRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	_, err := svc.AcceptAdminCompliance(context.Background(), AdminComplianceAcceptInput{
		AdminUserID: 99,
		Language:    "zh",
		Phrase:      AdminComplianceAckPhraseZH,
	})
	require.NoError(t, err)

	acknowledged, err := svc.IsAdminComplianceAcknowledged(context.Background(), 99)
	require.NoError(t, err)
	require.True(t, acknowledged, "Required must stay hard-disabled regardless of acknowledgement history")
}

func TestIsAdminComplianceAcknowledged_StaleAcknowledgementVersion_StillReturnsTrue(t *testing.T) {
	repo := &adminComplianceAckRepoStub{
		values: map[string]string{
			adminComplianceAcknowledgementKey(5): `{"version":"v2000.01.01","admin_user_id":5}`,
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	acknowledged, err := svc.IsAdminComplianceAcknowledged(context.Background(), 5)
	require.NoError(t, err)
	require.True(t, acknowledged, "hard-disabled gate must not require re-acknowledgement even for a stale version")
}

func TestIsAdminComplianceAcknowledged_PropagatesUnexpectedRepoError(t *testing.T) {
	repo := &adminComplianceAckRepoStub{forceErr: errors.New("db unavailable")}
	svc := NewSettingService(repo, &config.Config{})

	_, err := svc.IsAdminComplianceAcknowledged(context.Background(), 1)
	require.Error(t, err)
}
