package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildPassiveUsageWindow(t *testing.T) {
	future := time.Now().Add(48 * time.Hour).Unix()

	t.Run("utilization and reset", func(t *testing.T) {
		window := buildPassiveUsageWindow(map[string]any{
			"passive_usage_7d_oi_utilization": 0.87,
			"passive_usage_7d_oi_reset":       float64(future),
		}, "passive_usage_7d_oi_utilization", "passive_usage_7d_oi_reset")
		require.NotNil(t, window)
		require.InDelta(t, 87.0, window.Utilization, 1e-9)
		require.NotNil(t, window.ResetsAt)
		require.Equal(t, future, window.ResetsAt.Unix())
		require.Greater(t, window.RemainingSeconds, 0)
	})

	t.Run("no data returns nil", func(t *testing.T) {
		require.Nil(t, buildPassiveUsageWindow(nil, "u", "r"))
		require.Nil(t, buildPassiveUsageWindow(map[string]any{}, "u", "r"))
	})

	t.Run("expired reset clamps remaining to zero", func(t *testing.T) {
		past := time.Now().Add(-time.Hour).Unix()
		window := buildPassiveUsageWindow(map[string]any{
			"u": 0.5,
			"r": float64(past),
		}, "u", "r")
		require.NotNil(t, window)
		require.Equal(t, 0, window.RemainingSeconds)
	})

	t.Run("utilization only", func(t *testing.T) {
		window := buildPassiveUsageWindow(map[string]any{"u": 0.25}, "u", "r")
		require.NotNil(t, window)
		require.InDelta(t, 25.0, window.Utilization, 1e-9)
		require.Nil(t, window.ResetsAt)
	})
}

func TestSyncActiveToPassive_WritesFableExtras(t *testing.T) {
	repo := &accountUsageCodexProbeRepo{updateExtraCh: make(chan map[string]any, 1)}
	svc := &AccountUsageService{accountRepo: repo}

	resetAt := time.Now().Add(72 * time.Hour).Truncate(time.Second)
	usage := &UsageInfo{
		SevenDayFable: &UsageProgress{
			Utilization: 87,
			ResetsAt:    &resetAt,
		},
	}

	svc.syncActiveToPassive(t.Context(), 1, usage)

	select {
	case updates := <-repo.updateExtraCh:
		require.InDelta(t, 0.87, updates["passive_usage_7d_oi_utilization"], 1e-9)
		require.Equal(t, resetAt.Unix(), updates["passive_usage_7d_oi_reset"])
		require.Contains(t, updates, "passive_usage_sampled_at")
	default:
		t.Fatal("expected UpdateExtra to be called with fable extras")
	}
}
