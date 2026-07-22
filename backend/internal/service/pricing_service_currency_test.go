//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// pricingCurrencyRepoStub is a minimal SettingRepository backed by a map, with
// GetValue wired directly (unlike settingPublicRepoStub, which only supports
// GetMultiple) since GetCNYRate/GetCurrencyMode/GetShowOverseasModels call
// GetValue directly.
type pricingCurrencyRepoStub struct {
	values map[string]string
}

func (r *pricingCurrencyRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	if value, ok := r.values[key]; ok {
		return &Setting{Key: key, Value: value}, nil
	}
	return nil, ErrSettingNotFound
}

func (r *pricingCurrencyRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (r *pricingCurrencyRepoStub) Set(ctx context.Context, key, value string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}

func (r *pricingCurrencyRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		if v, ok := r.values[k]; ok {
			out[k] = v
		}
	}
	return out, nil
}

func (r *pricingCurrencyRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	return nil
}

func (r *pricingCurrencyRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	return r.values, nil
}

func (r *pricingCurrencyRepoStub) Delete(ctx context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestPricingService_GetCNYRate_PrefersDBValue(t *testing.T) {
	repo := &pricingCurrencyRepoStub{values: map[string]string{SettingKeyCNYRate: "7.31"}}
	svc := NewPricingService(&config.Config{}, nil, repo)

	require.InDelta(t, 7.31, svc.GetCNYRate(), 0.0001)
}

func TestPricingService_GetCNYRate_FallsBackToConfigWhenDBMissing(t *testing.T) {
	cfg := &config.Config{}
	cfg.Pricing.CNYRate = 7.1
	svc := NewPricingService(cfg, nil, &pricingCurrencyRepoStub{})

	require.InDelta(t, 7.1, svc.GetCNYRate(), 0.0001)
}

func TestPricingService_GetCNYRate_FallsBackToHardcodedDefaultWhenNothingConfigured(t *testing.T) {
	svc := NewPricingService(&config.Config{}, nil, &pricingCurrencyRepoStub{})

	require.InDelta(t, 6.8, svc.GetCNYRate(), 0.0001)
}

func TestPricingService_GetCNYRate_IgnoresNonPositiveDBValue(t *testing.T) {
	repo := &pricingCurrencyRepoStub{values: map[string]string{SettingKeyCNYRate: "-1"}}
	cfg := &config.Config{}
	cfg.Pricing.CNYRate = 7.5
	svc := NewPricingService(cfg, nil, repo)

	require.InDelta(t, 7.5, svc.GetCNYRate(), 0.0001, "non-positive DB value must fall back to config")
}

func TestPricingService_GetCNYRate_NilSettingRepoFallsBackToConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Pricing.CNYRate = 6.9
	svc := NewPricingService(cfg, nil, nil)

	require.InDelta(t, 6.9, svc.GetCNYRate(), 0.0001)
}

func TestPricingService_GetCurrencyMode(t *testing.T) {
	cases := []struct {
		name  string
		dbVal string
		want  string
	}{
		{"usd from db", "usd", "usd"},
		{"cny from db", "cny", "cny"},
		{"unrecognized value falls back to usd", "eur", "usd"},
		{"missing key falls back to usd", "", "usd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &pricingCurrencyRepoStub{}
			if tc.dbVal != "" {
				repo.values = map[string]string{SettingKeyCurrencyMode: tc.dbVal}
			}
			svc := NewPricingService(&config.Config{}, nil, repo)
			require.Equal(t, tc.want, svc.GetCurrencyMode())
		})
	}
}

func TestPricingService_GetCurrencyMode_NilSettingRepoFallsBackToUSD(t *testing.T) {
	svc := NewPricingService(&config.Config{}, nil, nil)
	require.Equal(t, "usd", svc.GetCurrencyMode())
}

func TestPricingService_GetShowOverseasModels(t *testing.T) {
	cases := []struct {
		name   string
		dbVal  string
		hasKey bool
		want   bool
	}{
		{"explicit false hides overseas models", "false", true, false},
		{"explicit true shows overseas models", "true", true, true},
		{"missing key defaults to true", "", false, true},
		{"unexpected value defaults to true (only exact \"false\" hides)", "no", true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &pricingCurrencyRepoStub{}
			if tc.hasKey {
				repo.values = map[string]string{SettingKeyShowOverseasModels: tc.dbVal}
			}
			svc := NewPricingService(&config.Config{}, nil, repo)
			require.Equal(t, tc.want, svc.GetShowOverseasModels())
		})
	}
}

func TestPricingService_GetShowOverseasModels_NilSettingRepoDefaultsToTrue(t *testing.T) {
	svc := NewPricingService(&config.Config{}, nil, nil)
	require.True(t, svc.GetShowOverseasModels())
}
