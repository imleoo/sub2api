//go:build unit

package handler

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// teamSettingRepoStub 仅为 GetFrontendURL 提供可控返回值。
type teamSettingRepoStub struct {
	frontendURL string
}

func (s *teamSettingRepoStub) Get(ctx context.Context, key string) (*service.Setting, error) {
	return nil, errors.New("not found")
}

func (s *teamSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if key == service.SettingKeyFrontendURL && s.frontendURL != "" {
		return s.frontendURL, nil
	}
	return "", errors.New("not found")
}

func (s *teamSettingRepoStub) Set(ctx context.Context, key, value string) error { return nil }

func (s *teamSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (s *teamSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	return nil
}

func (s *teamSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (s *teamSettingRepoStub) Delete(ctx context.Context, key string) error { return nil }

func newTeamHandlerWithFrontendURL(t *testing.T, frontendURL string) *TeamHandler {
	t.Helper()
	settingSvc := service.NewSettingService(&teamSettingRepoStub{frontendURL: frontendURL}, &config.Config{})
	return &TeamHandler{settingService: settingSvc}
}

// TestTeamHandler_ResolveFrontendBaseURL_FallsBackToRequestOrigin 回归守护。
//
// frontend_url 未配置时，邀请成员/重发邀请曾直接 500 "frontend URL is not configured"，
// 导致整个邀请功能不可用（线上 2026-07-19 实际发生）。未配置时应回退为当前请求的
// 来源（系统所在 URL），并尊重反代的 X-Forwarded-Proto / X-Forwarded-Host。
func TestTeamHandler_ResolveFrontendBaseURL_FallsBackToRequestOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTeamHandlerWithFrontendURL(t, "")

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/team/invitations", nil)
	c.Request.Host = "panel.example.com"
	require.Equal(t, "http://panel.example.com", h.resolveFrontendBaseURL(c))

	// 反代场景：X-Forwarded-Proto/Host 优先
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = httptest.NewRequest("POST", "/api/v1/team/invitations", nil)
	c2.Request.Host = "127.0.0.1:8080"
	c2.Request.Header.Set("X-Forwarded-Proto", "https")
	c2.Request.Header.Set("X-Forwarded-Host", "panel.example.com")
	require.Equal(t, "https://panel.example.com", h.resolveFrontendBaseURL(c2))
}

// TestTeamHandler_ResolveFrontendBaseURL_PrefersConfiguredSetting 已配置 frontend_url 时优先生效。
func TestTeamHandler_ResolveFrontendBaseURL_PrefersConfiguredSetting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTeamHandlerWithFrontendURL(t, "https://configured.example.com")

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/v1/team/invitations", nil)
	c.Request.Host = "panel.example.com"
	require.Equal(t, "https://configured.example.com", h.resolveFrontendBaseURL(c))
}
