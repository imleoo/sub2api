//go:build integration

package repository

import (
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SmsCacheSuite struct {
	IntegrationRedisSuite
	cache service.SmsCache
}

func (s *SmsCacheSuite) SetupTest() {
	s.IntegrationRedisSuite.SetupTest()
	s.cache = NewSmsCache(s.rdb)
}

// TestIncrSmsVerifyAttempts_Concurrent 验证失败尝试计数的原子性：
// N 个并发自增后计数必须恰好为 N，证明不存在 TOCTOU 丢失更新（即旧实现的爆破绕过）。
func (s *SmsCacheSuite) TestIncrSmsVerifyAttempts_Concurrent() {
	const phone = "13800000001"
	const n = 200

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := s.cache.IncrSmsVerifyAttempts(s.ctx, phone, time.Minute)
			require.NoError(s.T(), err)
		}()
	}
	wg.Wait()

	// 再自增一次拿到最终值，断言等于 n+1（无丢失更新）。
	final, err := s.cache.IncrSmsVerifyAttempts(s.ctx, phone, time.Minute)
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(n+1), final, "concurrent increments must not lose updates")
}

// TestDeleteSmsVerifyCode_ClearsAttempts 验证删除验证码会一并清除尝试计数键。
func (s *SmsCacheSuite) TestDeleteSmsVerifyCode_ClearsAttempts() {
	const phone = "13800000002"

	n, err := s.cache.IncrSmsVerifyAttempts(s.ctx, phone, time.Minute)
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(1), n)

	require.NoError(s.T(), s.cache.DeleteSmsVerifyCode(s.ctx, phone))

	// 删除后再自增应从 1 重新开始，说明计数键已被清除。
	again, err := s.cache.IncrSmsVerifyAttempts(s.ctx, phone, time.Minute)
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(1), again, "attempts counter should reset after delete")
}

func TestSmsCacheSuite(t *testing.T) {
	suite.Run(t, new(SmsCacheSuite))
}
