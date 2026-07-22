//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestExtractOriginFromURL_VariousInputs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"simple https", "https://example.com", "https://example.com"},
		{"with port", "https://example.com:8443/some/path", "https://example.com:8443"},
		{"with path and query", "http://foo.bar/a/b?x=1", "http://foo.bar"},
		{"non http scheme rejected", "ftp://files.example.com", ""},
		{"javascript scheme rejected", "javascript:alert(1)", ""},
		{"malformed url", "http://[::1", ""},
		{"whitespace padded", "  https://trimmed.example.com  ", "https://trimmed.example.com"},
		{"missing host", "https:///no-host", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, extractOriginFromURL(tc.in))
		})
	}
}

func TestFilterUserVisibleMenuItems(t *testing.T) {
	t.Run("empty raw returns empty array", func(t *testing.T) {
		require.JSONEq(t, `[]`, string(filterUserVisibleMenuItems("")))
	})

	t.Run("literal empty array returns empty array", func(t *testing.T) {
		require.JSONEq(t, `[]`, string(filterUserVisibleMenuItems("[]")))
	})

	t.Run("invalid json returns empty array", func(t *testing.T) {
		require.JSONEq(t, `[]`, string(filterUserVisibleMenuItems("not-json")))
	})

	t.Run("filters out admin-only items", func(t *testing.T) {
		raw := `[{"title":"Public","visibility":"user","url":"https://a.example.com"},{"title":"Admin Only","visibility":"admin","url":"https://b.example.com"}]`
		got := filterUserVisibleMenuItems(raw)
		require.JSONEq(t, `[{"title":"Public","visibility":"user","url":"https://a.example.com"}]`, string(got))
	})

	t.Run("all admin items yields empty array", func(t *testing.T) {
		raw := `[{"title":"Admin Only","visibility":"admin","url":"https://b.example.com"}]`
		require.JSONEq(t, `[]`, string(filterUserVisibleMenuItems(raw)))
	})

	t.Run("missing visibility field treated as visible", func(t *testing.T) {
		raw := `[{"title":"No Visibility Field","url":"https://c.example.com"}]`
		got := filterUserVisibleMenuItems(raw)
		require.JSONEq(t, raw, string(got))
	})
}

func TestParseCustomMenuItemURLs(t *testing.T) {
	t.Run("empty raw returns nil", func(t *testing.T) {
		require.Nil(t, parseCustomMenuItemURLs(""))
	})

	t.Run("literal empty array returns nil", func(t *testing.T) {
		require.Nil(t, parseCustomMenuItemURLs("[]"))
	})

	t.Run("invalid json returns nil", func(t *testing.T) {
		require.Nil(t, parseCustomMenuItemURLs("{not valid"))
	})

	t.Run("extracts urls and skips empty ones", func(t *testing.T) {
		raw := `[{"title":"A","url":"https://a.example.com"},{"title":"NoURL"},{"title":"B","url":"https://b.example.com"}]`
		got := parseCustomMenuItemURLs(raw)
		require.Equal(t, []string{"https://a.example.com", "https://b.example.com"}, got)
	})
}

func TestSettingService_GetFrameSrcOrigins_DeduplicatesAndFiltersInvalid(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyHomeContent:                 "https://embed.example.com/home",
			SettingKeyPurchaseSubscriptionEnabled: "true",
			SettingKeyPurchaseSubscriptionURL:     "https://embed.example.com/buy", // same origin as home content
			SettingKeyCustomMenuItems:             `[{"title":"A","url":"https://menu-a.example.com/x"},{"title":"B","url":"not a url"},{"title":"C","url":"https://menu-a.example.com/y"}]`,
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	origins, err := svc.GetFrameSrcOrigins(context.Background())
	require.NoError(t, err)
	// home content + purchase subscription share an origin (deduped), plus one distinct menu-item origin
	// (menu-a appears twice with different paths but same origin, also deduped).
	require.Equal(t, []string{"https://embed.example.com", "https://menu-a.example.com"}, origins)
}

func TestSettingService_GetFrameSrcOrigins_IgnoresDisabledPurchaseSubscriptionURL(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyPurchaseSubscriptionEnabled: "false",
			SettingKeyPurchaseSubscriptionURL:     "https://should-not-appear.example.com",
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	origins, err := svc.GetFrameSrcOrigins(context.Background())
	require.NoError(t, err)
	require.Empty(t, origins)
}

func TestSettingService_GetPublicSettingsForInjection_MapsForkFieldsAndFiltersMenu(t *testing.T) {
	repo := &settingPublicRepoStub{
		values: map[string]string{
			SettingKeyCurrencyMode:         "cny",
			SettingKeyCNYRate:              "7.25",
			SettingKeyShowOverseasModels:   "false",
			SettingKeyPhoneRegisterEnabled: "true",
			SettingKeyPasswordLoginEnabled: "false",
			SettingKeyUITheme:              "midnight",
			SettingKeyCustomMenuItems:      `[{"title":"Public","visibility":"user","url":"https://a.example.com"},{"title":"Admin Only","visibility":"admin","url":"https://b.example.com"}]`,
		},
	}
	svc := NewSettingService(repo, &config.Config{})

	payload, err := svc.GetPublicSettingsForInjection(context.Background())
	require.NoError(t, err)

	injected, ok := payload.(*PublicSettingsInjectionPayload)
	require.True(t, ok, "expected *PublicSettingsInjectionPayload, got %T", payload)

	require.Equal(t, "cny", injected.CurrencyMode)
	require.InDelta(t, 7.25, injected.CNYRate, 0.0001)
	require.False(t, injected.ShowOverseasModels)
	require.True(t, injected.PhoneRegisterEnabled)
	require.False(t, injected.PasswordLoginEnabled)
	require.Equal(t, "midnight", injected.UITheme)
	require.JSONEq(t, `[{"title":"Public","visibility":"user","url":"https://a.example.com"}]`, string(injected.CustomMenuItems))
}
