//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

// --- invalidRequestFallbackGroupID ---

func TestInvalidRequestFallbackGroupID_NilGroupID(t *testing.T) {
	t.Parallel()
	svc := &GatewayService{}
	require.Nil(t, svc.invalidRequestFallbackGroupID(context.Background(), nil))
}

func TestInvalidRequestFallbackGroupID_NoFallbackConfigured(t *testing.T) {
	t.Parallel()
	svc := &GatewayService{}
	gid := int64(10)
	group := &Group{ID: gid, Hydrated: true, Platform: PlatformAnthropic, Status: StatusActive}
	ctx := context.WithValue(context.Background(), ctxkey.Group, group)
	require.Nil(t, svc.invalidRequestFallbackGroupID(ctx, &gid))
}

func TestInvalidRequestFallbackGroupID_FallbackConfigured(t *testing.T) {
	t.Parallel()
	svc := &GatewayService{}
	gid := int64(10)
	fallbackID := int64(20)
	group := &Group{
		ID:                              gid,
		Hydrated:                        true,
		Platform:                        PlatformAnthropic,
		Status:                          StatusActive,
		FallbackGroupIDOnInvalidRequest: &fallbackID,
	}
	ctx := context.WithValue(context.Background(), ctxkey.Group, group)
	got := svc.invalidRequestFallbackGroupID(ctx, &gid)
	require.NotNil(t, got)
	require.Equal(t, fallbackID, *got)
}

func TestInvalidRequestFallbackGroupID_GroupLookupFailed(t *testing.T) {
	t.Parallel()
	// 未注入 context，groupRepo 查不到该 group：resolveGroupByID 返回 err，
	// invalidRequestFallbackGroupID 必须吞掉错误返回 nil（不阻塞主流程）。
	svc := &GatewayService{groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{}}}
	gid := int64(999)
	require.Nil(t, svc.invalidRequestFallbackGroupID(context.Background(), &gid))
}

// --- calculateVideoCost ---

func TestCalculateVideoCost_NoChannelPricing_UsesPerSecondPath(t *testing.T) {
	t.Parallel()
	price := 0.5
	ps := &PricingService{
		catalog: map[string]*DBModelPricing{
			"wanjie-video": {ModelID: "wanjie-video", OutputCostPerImage: &price},
		},
	}
	bs := NewBillingService(&config.Config{}, ps)
	// resolver 为 nil ⇒ resolveChannelPricing 直接返回 nil，走 CalculatePerSecondVideoCost。
	svc := &GatewayService{billingService: bs}

	apiKey := &APIKey{Group: &Group{ID: 1}}
	result := &ForwardResult{VideoSeconds: 10}

	got := svc.calculateVideoCost(context.Background(), result, apiKey, "wanjie-video", 1.0)
	require.NotNil(t, got)
	require.InDelta(t, 5.0, got.TotalCost, 1e-9) // 0.5 * 10s
	require.InDelta(t, 5.0, got.ActualCost, 1e-9)
}

func TestCalculateVideoCost_NoPricing_ReturnsZeroCost(t *testing.T) {
	t.Parallel()
	ps := &PricingService{catalog: map[string]*DBModelPricing{}}
	bs := NewBillingService(&config.Config{}, ps)
	svc := &GatewayService{billingService: bs}

	apiKey := &APIKey{Group: &Group{ID: 1}}
	result := &ForwardResult{VideoSeconds: 8}

	got := svc.calculateVideoCost(context.Background(), result, apiKey, "unknown-video-model", 1.0)
	require.NotNil(t, got)
	require.Zero(t, got.ActualCost)
}

func TestCalculateVideoCost_ZeroSeconds_ReturnsEmptyBreakdown(t *testing.T) {
	t.Parallel()
	ps := &PricingService{catalog: map[string]*DBModelPricing{}}
	bs := NewBillingService(&config.Config{}, ps)
	svc := &GatewayService{billingService: bs}

	apiKey := &APIKey{Group: &Group{ID: 1}}
	result := &ForwardResult{VideoSeconds: 0}

	got := svc.calculateVideoCost(context.Background(), result, apiKey, "wanjie-video", 1.0)
	require.NotNil(t, got)
	require.Zero(t, got.ActualCost)
	require.Zero(t, got.TotalCost)
}

func TestCalculateVideoCost_DifferentDurations_ScaleLinearly(t *testing.T) {
	t.Parallel()
	price := 0.2
	ps := &PricingService{
		catalog: map[string]*DBModelPricing{
			"wanjie-video": {ModelID: "wanjie-video", OutputCostPerImage: &price},
		},
	}
	bs := NewBillingService(&config.Config{}, ps)
	svc := &GatewayService{billingService: bs}
	apiKey := &APIKey{Group: &Group{ID: 1}}

	five := svc.calculateVideoCost(context.Background(), &ForwardResult{VideoSeconds: 5}, apiKey, "wanjie-video", 1.0)
	ten := svc.calculateVideoCost(context.Background(), &ForwardResult{VideoSeconds: 10}, apiKey, "wanjie-video", 1.0)
	require.InDelta(t, five.ActualCost*2, ten.ActualCost, 1e-9)
}

