package routes

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Phase 2 P2-6 fork 12 项功能 Go 单测守护。
//
// 与 script/check_fork12_guards.sh 双轨守护：脚本在 CI 早期跑（fail fast），
// Go 单测随 `go test ./...` 跑（与代码同步审计）。
//
// 守护规则（docs/sprint-plan.md P2-6 + glossary.md §5 fork 锚点）：
//   - routes/gateway.go 必须保留 /lingjing/v1/video 路由组（fork 12）
//   - routes/gateway.go 必须保留 ForcePlatform middleware 至少 3 处

func loadGatewayRouteSource(t *testing.T) string {
	t.Helper()
	bytes, err := os.ReadFile("gateway.go")
	require.NoError(t, err, "routes/gateway.go must be readable")
	return string(bytes)
}

// TestGatewayRoutes_LingjingVideoRouteGroup 守护 fork 12 /lingjing/v1/video 路由组完整。
func TestGatewayRoutes_LingjingVideoRouteGroup(t *testing.T) {
	src := loadGatewayRouteSource(t)
	require.Contains(t, src, "/lingjing/v1",
		"fork 12 /lingjing/v1 route group must be preserved")
	require.Contains(t, src, "video/submit",
		"fork 12 /lingjing/v1/video/submit endpoint must be preserved")
	require.Contains(t, src, "video/:taskId",
		"fork 12 /lingjing/v1/video/:taskId polling endpoint must be preserved")
}

// TestGatewayRoutes_ForcePlatformMiddleware 守护 ForcePlatform middleware 至少 3 处。
//
// 期望：lingjing × 1 + antigravity v1 × 1 + antigravity v1beta × 1 = 3 处。
// 若 Phase 5 引入更多专用路由，守护应放宽。
func TestGatewayRoutes_ForcePlatformMiddleware(t *testing.T) {
	src := loadGatewayRouteSource(t)
	count := strings.Count(src, "ForcePlatform(")
	require.GreaterOrEqual(t, count, 3,
		"ForcePlatform middleware (lingjing + antigravity x2) must be preserved (found %d, expected >= 3)", count)
}
