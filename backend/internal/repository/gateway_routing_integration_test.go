//go:build integration

package repository

import (
	"context"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/suite"
)

// GatewayRoutingSuite 测试网关路由相关的数据库查询
// 验证账户选择和分流逻辑在真实数据库环境下的行为
type GatewayRoutingSuite struct {
	suite.Suite
	ctx         context.Context
	client      *dbent.Client
	accountRepo *accountRepository
}

func (s *GatewayRoutingSuite) SetupTest() {
	s.ctx = context.Background()
	tx := testEntTx(s.T())
	s.client = tx.Client()
	s.accountRepo = newAccountRepositoryWithSQL(s.client, tx, nil)
}

func TestGatewayRoutingSuite(t *testing.T) {
	suite.Run(t, new(GatewayRoutingSuite))
}

// TestListSchedulableByPlatforms_GeminiAndOpenAI 验证多平台账户查询
func (s *GatewayRoutingSuite) TestListSchedulableByPlatforms_GeminiAndOpenAI() {
	// 创建各平台账户
	geminiAcc := mustCreateAccount(s.T(), s.client, &service.Account{
		Name:        "gemini-apikey",
		Platform:    service.PlatformGemini,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Priority:    1,
	})

	openaiAcc := mustCreateAccount(s.T(), s.client, &service.Account{
		Name:        "openai-apikey",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Priority:    2,
		Credentials: map[string]any{
			"api_key": "test-key",
		},
	})

	// 创建不应被选中的 anthropic 账户
	mustCreateAccount(s.T(), s.client, &service.Account{
		Name:        "anthropic-apikey",
		Platform:    service.PlatformAnthropic,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Priority:    0,
	})

	// 查询 gemini + openai 平台
	accounts, err := s.accountRepo.ListSchedulableByPlatforms(s.ctx, []string{
		service.PlatformGemini,
		service.PlatformOpenAI,
	})

	s.Require().NoError(err)
	s.Require().Len(accounts, 2, "应返回 gemini 和 openai 两个账户")

	// 验证返回的账户平台
	platforms := make(map[string]bool)
	for _, acc := range accounts {
		platforms[acc.Platform] = true
	}
	s.Require().True(platforms[service.PlatformGemini], "应包含 gemini 账户")
	s.Require().True(platforms[service.PlatformOpenAI], "应包含 openai 账户")
	s.Require().False(platforms[service.PlatformAnthropic], "不应包含 anthropic 账户")

	// 验证账户 ID 匹配
	ids := make(map[int64]bool)
	for _, acc := range accounts {
		ids[acc.ID] = true
	}
	s.Require().True(ids[geminiAcc.ID])
	s.Require().True(ids[openaiAcc.ID])
}

// TestListSchedulableByGroupIDAndPlatforms_WithGroupBinding 验证按分组过滤
func (s *GatewayRoutingSuite) TestListSchedulableByGroupIDAndPlatforms_WithGroupBinding() {
	// 创建 gemini 分组
	group := mustCreateGroup(s.T(), s.client, &service.Group{
		Name:     "gemini-group",
		Platform: service.PlatformGemini,
		Status:   service.StatusActive,
	})

	// 创建账户
	boundAcc := mustCreateAccount(s.T(), s.client, &service.Account{
		Name:        "bound-gemini",
		Platform:    service.PlatformGemini,
		Status:      service.StatusActive,
		Schedulable: true,
	})
	unboundAcc := mustCreateAccount(s.T(), s.client, &service.Account{
		Name:        "unbound-gemini",
		Platform:    service.PlatformGemini,
		Status:      service.StatusActive,
		Schedulable: true,
	})

	// 只绑定一个账户到分组
	mustBindAccountToGroup(s.T(), s.client, boundAcc.ID, group.ID, 1)

	// 查询分组内的账户
	accounts, err := s.accountRepo.ListSchedulableByGroupIDAndPlatforms(s.ctx, group.ID, []string{
		service.PlatformGemini,
		service.PlatformOpenAI,
	})

	s.Require().NoError(err)
	s.Require().Len(accounts, 1, "应只返回绑定到分组的账户")
	s.Require().Equal(boundAcc.ID, accounts[0].ID)

	// 确认未绑定的账户不在结果中
	for _, acc := range accounts {
		s.Require().NotEqual(unboundAcc.ID, acc.ID, "不应包含未绑定的账户")
	}
}

// TestListSchedulableByPlatform_SinglePlatform 验证单平台查询
func (s *GatewayRoutingSuite) TestListSchedulableByPlatform_SinglePlatform() {
	// 创建多种平台账户
	mustCreateAccount(s.T(), s.client, &service.Account{
		Name:        "gemini-1",
		Platform:    service.PlatformGemini,
		Status:      service.StatusActive,
		Schedulable: true,
	})

	openaiAcc := mustCreateAccount(s.T(), s.client, &service.Account{
		Name:        "openai-1",
		Platform:    service.PlatformOpenAI,
		Status:      service.StatusActive,
		Schedulable: true,
	})

	// 只查询 openai 平台
	accounts, err := s.accountRepo.ListSchedulableByPlatform(s.ctx, service.PlatformOpenAI)

	s.Require().NoError(err)
	s.Require().Len(accounts, 1)
	s.Require().Equal(openaiAcc.ID, accounts[0].ID)
	s.Require().Equal(service.PlatformOpenAI, accounts[0].Platform)
}

// TestSchedulableFilter_ExcludesInactive 验证不可调度账户被过滤
func (s *GatewayRoutingSuite) TestSchedulableFilter_ExcludesInactive() {
	// 创建可调度账户
	activeAcc := mustCreateAccount(s.T(), s.client, &service.Account{
		Name:        "active-openai",
		Platform:    service.PlatformOpenAI,
		Status:      service.StatusActive,
		Schedulable: true,
	})

	// 创建不可调度账户（需要先创建再更新，因为 fixture 默认设置 Schedulable=true）
	inactiveAcc := mustCreateAccount(s.T(), s.client, &service.Account{
		Name:     "inactive-openai",
		Platform: service.PlatformOpenAI,
		Status:   service.StatusActive,
	})
	s.Require().NoError(s.client.Account.UpdateOneID(inactiveAcc.ID).SetSchedulable(false).Exec(s.ctx))

	// 创建错误状态账户
	mustCreateAccount(s.T(), s.client, &service.Account{
		Name:        "error-openai",
		Platform:    service.PlatformOpenAI,
		Status:      service.StatusError,
		Schedulable: true,
	})

	accounts, err := s.accountRepo.ListSchedulableByPlatform(s.ctx, service.PlatformOpenAI)

	s.Require().NoError(err)
	s.Require().Len(accounts, 1, "应只返回可调度的 active 账户")
	s.Require().Equal(activeAcc.ID, accounts[0].ID)
}
