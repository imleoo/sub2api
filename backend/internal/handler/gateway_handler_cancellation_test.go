//go:build unit

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// fakeSchedulerCache 是一个最小的内存 SchedulerCache 桩，仅用于本文件的取消/预取
// 测试；不含 Antigravity 预热拦截专用逻辑（该功能 fork 不支持）。
type fakeSchedulerCache struct {
	accounts []*service.Account
}

func (f *fakeSchedulerCache) GetSnapshot(_ context.Context, _ service.SchedulerBucket) ([]*service.Account, bool, error) {
	return f.accounts, true, nil
}
func (f *fakeSchedulerCache) CaptureBucketWriteToken(_ context.Context, bucket service.SchedulerBucket) (service.SchedulerBucketWriteToken, error) {
	return service.SchedulerBucketWriteToken{Bucket: bucket, Epoch: 1}, nil
}
func (f *fakeSchedulerCache) SetSnapshot(_ context.Context, _ service.SchedulerBucket, _ service.SchedulerBucketWriteToken, _ []service.Account) error {
	return nil
}
func (f *fakeSchedulerCache) RetireBucket(_ context.Context, _ service.SchedulerBucket) error {
	return nil
}
func (f *fakeSchedulerCache) ReopenBucket(_ context.Context, bucket service.SchedulerBucket) (service.SchedulerBucketWriteToken, error) {
	return service.SchedulerBucketWriteToken{Bucket: bucket, Epoch: 1}, nil
}
func (f *fakeSchedulerCache) TryAcquireGroupLifecycleLease(_ context.Context, _ int64, _ time.Duration) (service.SchedulerGroupLifecycleLease, bool, error) {
	return service.SchedulerGroupLifecycleLease{}, false, nil
}
func (f *fakeSchedulerCache) ReleaseGroupLifecycleLease(_ context.Context, _ service.SchedulerGroupLifecycleLease) error {
	return nil
}
func (f *fakeSchedulerCache) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	for _, account := range f.accounts {
		if account != nil && account.ID == id {
			return account, nil
		}
	}
	return nil, nil
}
func (f *fakeSchedulerCache) SetAccount(_ context.Context, _ *service.Account) error { return nil }
func (f *fakeSchedulerCache) DeleteAccount(_ context.Context, _ int64) error         { return nil }
func (f *fakeSchedulerCache) UpdateLastUsed(_ context.Context, _ map[int64]time.Time) error {
	return nil
}
func (f *fakeSchedulerCache) TryLockBucket(_ context.Context, _ service.SchedulerBucket, _ time.Duration) (bool, error) {
	return true, nil
}
func (f *fakeSchedulerCache) UnlockBucket(_ context.Context, _ service.SchedulerBucket) error {
	return nil
}
func (f *fakeSchedulerCache) ListBuckets(_ context.Context) ([]service.SchedulerBucket, error) {
	return nil, nil
}
func (f *fakeSchedulerCache) GetOutboxWatermark(_ context.Context) (int64, error) { return 0, nil }
func (f *fakeSchedulerCache) SetOutboxWatermark(_ context.Context, _ int64) error { return nil }
func (f *fakeSchedulerCache) IncrDualBucketTotal(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (f *fakeSchedulerCache) IncrDualBucketDiverged(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (f *fakeSchedulerCache) GetDualBucketStats(_ context.Context, _ string, _ int) ([]service.DualBucketDayStats, error) {
	return nil, nil
}

type fakeGroupRepo struct {
	group *service.Group
}

func (f *fakeGroupRepo) Create(context.Context, *service.Group) error { return nil }
func (f *fakeGroupRepo) GetByID(context.Context, int64) (*service.Group, error) {
	return f.group, nil
}
func (f *fakeGroupRepo) GetByIDLite(context.Context, int64) (*service.Group, error) {
	return f.group, nil
}
func (f *fakeGroupRepo) Update(context.Context, *service.Group) error          { return nil }
func (f *fakeGroupRepo) Delete(context.Context, int64) error                   { return nil }
func (f *fakeGroupRepo) DeleteCascade(context.Context, int64) ([]int64, error) { return nil, nil }
func (f *fakeGroupRepo) List(context.Context, pagination.PaginationParams) ([]service.Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (f *fakeGroupRepo) ListWithFilters(context.Context, pagination.PaginationParams, string, string, string, *bool) ([]service.Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (f *fakeGroupRepo) ListActive(context.Context) ([]service.Group, error) { return nil, nil }
func (f *fakeGroupRepo) ListActiveByPlatform(context.Context, string) ([]service.Group, error) {
	return nil, nil
}
func (f *fakeGroupRepo) ExistsByName(context.Context, string) (bool, error) { return false, nil }
func (f *fakeGroupRepo) GetAccountCount(context.Context, int64) (int64, int64, error) {
	return 0, 0, nil
}
func (f *fakeGroupRepo) DeleteAccountGroupsByGroupID(context.Context, int64) (int64, error) {
	return 0, nil
}
func (f *fakeGroupRepo) GetAccountIDsByGroupIDs(context.Context, []int64) ([]int64, error) {
	return nil, nil
}
func (f *fakeGroupRepo) BindAccountsToGroup(context.Context, int64, []int64) error { return nil }
func (f *fakeGroupRepo) UpdateSortOrders(context.Context, []service.GroupSortOrderUpdate) error {
	return nil
}

type countingGatewaySchedulerCache struct {
	*fakeSchedulerCache
	snapshotCalls atomic.Int64
}

func (c *countingGatewaySchedulerCache) GetSnapshot(ctx context.Context, bucket service.SchedulerBucket) ([]*service.Account, bool, error) {
	c.snapshotCalls.Add(1)
	return c.fakeSchedulerCache.GetSnapshot(ctx, bucket)
}

func TestGatewayHandlerPreCancelledCompatibleRequestsDoNotSelectAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(9100)
	group := &service.Group{ID: groupID, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive}
	account := &service.Account{
		ID: 9101, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 1,
		AccountGroups: []service.AccountGroup{{AccountID: 9101, GroupID: groupID}},
	}
	schedulerCache := &countingGatewaySchedulerCache{fakeSchedulerCache: &fakeSchedulerCache{accounts: []*service.Account{account}}}
	schedulerSnapshot := service.NewSchedulerSnapshotService(schedulerCache, nil, nil, nil, nil)
	gatewayService := service.NewGatewayService(
		nil, &fakeGroupRepo{group: group}, nil, nil, nil, nil, nil, nil, nil,
		schedulerSnapshot, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	cfg := &config.Config{RunMode: config.RunModeSimple}
	billingCacheService := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCacheService.Stop)
	h := &GatewayHandler{
		gatewayService:      gatewayService,
		billingCacheService: billingCacheService,
		concurrencyHelper:   NewConcurrencyHelper(service.NewConcurrencyService(&concurrencyCacheMock{}), SSEPingFormatClaude, 0),
		maxAccountSwitches:  1,
		cfg:                 cfg,
	}
	apiKey := &service.APIKey{
		ID: 9102, UserID: 9103, GroupID: &groupID, Group: group, Status: service.StatusActive,
		User: &service.User{ID: 9103, Concurrency: 10, Balance: 100},
	}

	tests := []struct {
		name string
		path string
		body string
		call func(*gin.Context)
	}{
		{
			name: "responses", path: "/v1/responses", body: `{"model":"claude-test","input":"hello","stream":false}`,
			call: h.Responses,
		},
		{
			name: "chat completions", path: "/v1/chat/completions", body: `{"model":"claude-test","messages":[{"role":"user","content":"hello"}],"stream":false}`,
			call: h.ChatCompletions,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedulerCache.snapshotCalls.Store(0)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			ctx = context.WithValue(ctx, ctxkey.Group, group)
			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString(tt.body)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			c.Request = req
			c.Set(string(middleware.ContextKeyAPIKey), apiKey)
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})

			tt.call(c)

			require.Zero(t, schedulerCache.snapshotCalls.Load(), "a cancelled request must stop before the account selector")
			_, selected := c.Get(opsAccountIDKey)
			require.False(t, selected)
		})
	}
}
