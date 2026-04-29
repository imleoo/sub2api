# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

**sub2api**（fork: bayma888/sub2api-bmai，分支: zhiguofan）是一个 AI API 网关平台，用于订阅配额分发。从上游 AI 订阅（OpenAI、Anthropic/Claude、Gemini、Antigravity 等）生成 API Key，分发给用户，支持计费、负载均衡和请求转发。

**技术栈**：Go 1.26.1（Gin + Ent ORM）+ Vue 3.4+（Vite 5 + TailwindCSS + Pinia）+ PostgreSQL 18 + Redis 8

## 常用命令

### 根目录（Makefile）
```bash
make build        # 构建后端 + 前端
make test         # 运行所有测试（后端单元测试 + 前端 lint/typecheck）
make generate     # 重新生成 Ent ORM + Wire DI（go generate ./ent && go generate ./cmd/server）
make dev-up       # 启动本地开发环境（Docker Compose）
make dev-down     # 停止本地开发环境
make dev-status   # 检查开发环境状态
make dev-logs     # 查看开发环境日志
make secret-scan  # 通过 tools/secret_scan.py 扫描密钥/凭据
```

### 后端（Go）
```bash
cd backend

# 运行服务器
go run ./cmd/server/

# 构建（含前端嵌入）
go build -tags embed -o sub2api ./cmd/server

# 单元测试
go test -tags=unit ./...

# 运行指定测试
go test -tags=unit -run TestFunctionName ./internal/handler/...

# 集成测试（需要 PostgreSQL + Redis，通过 testcontainers 提供）
go test -tags=integration ./...

# E2E 测试
go test -tags=e2e -v -timeout=300s ./internal/integration/...

# Lint 检查
golangci-lint run ./...

# Schema 变更后重新生成 Ent ORM
go generate ./ent

# Wire DI 变更后重新生成
go generate ./cmd/server
```

### 前端（Vue，必须用 pnpm）
```bash
cd frontend

pnpm install        # 安装依赖（禁止使用 npm）
pnpm dev            # 启动热重载开发服务器
pnpm build          # TypeScript 类型检查 + 构建
pnpm run lint:check # Lint 检查
pnpm test:run       # 运行测试
pnpm test:coverage  # 测试覆盖率
```

### 本地调试脚本
```bash
# 连接本机 Docker 中的 PostgreSQL + Redis，同时启动后端和前端
./script/dev_local.sh up       # 启动（后端 :8082，前端 :3002）
./script/dev_local.sh down     # 停止
./script/dev_local.sh status   # 查看状态
./script/dev_local.sh logs     # 查看日志
# 端口冲突时覆盖：BACKEND_PORT=18082 FRONTEND_PORT=13002 ./script/dev_local.sh up

# 同步上游到 zhiguofan（需要 GitHub 网络）
# 流程：upstream/main → main → origin/main → zhiguofan → origin/zhiguofan
./script/sync_upstream_to_zhiguofan.sh

# 推送 zhiguofan 到内网 Git（切换到内网后执行）
# 目标：ssh://git_prod_backend@192.168.1.10/home/git_prod_backend/wmtoken_platform.git
./script/push_zhiguofan_to_internal_git.sh
```

## 架构

### 关键设计模式

1. **严格分层架构**：Handler → Service → Repository → Ent ORM
   - golangci-lint 强制执行：Service 不能导入 Repository，Handler 不能直接导入 Repository
   - Handler 负责解析请求、调用 Service、返回响应
   - Service 包含业务逻辑
   - Repository 通过 Ent 处理数据库查询，带 Redis 缓存和分页

2. **依赖注入**：Google Wire，生成文件位于 `cmd/server/wire_gen.go`
   - ProviderSets：config、repository、service、middleware、handler、server
   - Application struct 内嵌 `*http.Server` 和 `Cleanup` 函数用于优雅关闭

3. **API 响应契约**：所有响应格式为 `{ code: number, message: string, data: T }`
   - 前端 Axios 拦截器自动解包

4. **API 路由**（定义在 `internal/server/routes/`）：
   - 公开：`/api/v1/auth/*`、`/api/v1/keys/*`、`/api/v1/usage/*`、`/api/v1/subscriptions/*`
   - 用户（需认证）：`/api/v1/dashboard`、`/api/v1/usage` 等
   - 管理员（需管理员权限）：`/api/v1/admin/*`
   - 网关（转发到上游 AI API）：`/v1/messages`、`/v1/chat/completions`
   - 健康检查：`/health`

5. **数据库**：Ent ORM + PostgreSQL。`ent/schema/` 下任何 Schema 变更后必须运行 `go generate ./ent`。迁移运行器在 `repository/migrations_runner.go`。

6. **后台服务生命周期**：许多服务以后台 goroutine 运行（定时报告、清理、告警评估、Token 刷新、OAuth）。Wire 生成的 cleanup 函数在优雅关闭时并行停止所有后台服务。

7. **前端嵌入**：生产构建使用 `go build -tags embed` 将前端嵌入二进制，实现单文件部署。

8. **前端架构**：
   - Vue 3 Composition API + Pinia 状态管理 + Vue Router（懒加载路由）
   - `router/index.ts` 中的路由守卫检查 `requiresAuth` 和 `requiresAdmin` 元字段
   - Axios 实例含 Token 自动刷新（401 重试队列）
   - i18n via vue-i18n；后端使用 `Accept-Language` 头
   - 所有 API 调用通过 `frontend/src/api/` 封装的 Axios

9. **Simple 模式**：`RUN_MODE=simple`（配合 `SIMPLE_MODE_CONFIRM=true`）禁用计费和配额检查，适合个人开发者使用。

10. **自动初始化**：首次运行检测 + Web 安装向导。Docker 部署使用 `AUTO_SETUP=true` 从环境变量静默初始化。

### 网关代理架构（`internal/pkg/` + `internal/service/`）

每个 AI 平台都有独立的 pkg 包处理协议适配：
- `pkg/claude/`：Claude 专用请求格式化和 cache-control 管理
- `pkg/openai/`、`pkg/gemini/`、`pkg/antigravity/`：各平台 API 兼容层
- `pkg/apicompat/`：跨平台请求/响应转换工具

网关服务层有三个专用实现：`gateway_service.go`（Claude 主代理）、`openai_gateway_service.go`、`antigravity_gateway_service.go`。

**关键网关特性**：
- Claude Sticky Session：1 小时 TTL，保证对话连续性
- Singleflight 用户组限速缓存，防止缓存击穿
- H2C（HTTP/2 Cleartext）支持，可通过环境变量调节帧大小和 buffer
- 账号选择带等待队列，避免并发争用

### 支付集成（`internal/payment/`）

工厂模式多支付提供商：Alipay、WxPay、Stripe、EasyPay，带负载均衡。费用计算与支付商逻辑解耦。

### 前端测试覆盖率

Vitest 配置要求语句/分支/函数/行均达到 80% 覆盖率（`frontend/vitest.config.ts`）。测试与实现代码放在同目录（`*.spec.ts`）。

## 关键开发规则

- **前端必须使用 pnpm**（禁止 npm）。每次修改依赖都要提交 `pnpm-lock.yaml`
- **Go 版本必须是 1.26.1**（由 `backend/go.mod` 指定，CI 通过 `go-version-file` 自动读取）
- **Ent schema 变更**：修改 `ent/schema/*.go` 后必须运行 `go generate ./ent`，并提交生成的代码
- **Wire DI 变更**：修改 Wire providers 后运行 `go generate ./cmd/server`
- **接口变更**：给 Go interface 新增方法后，**所有**实现该接口的 test stub 都必须补全。查找方式：`grep -r "type.*Stub.*struct\|type.*Mock.*struct" internal/`
- **golangci-lint v2.9**：CI 自动运行 lint，推送前确保代码通过检查

## 已知陷阱

### 批量修改账号导致模型映射丢失
**现象**：前端测试看起来正常，但通过 API 调用时返回 `Service temporarily unavailable`。
**根因**：同时选中不同平台账号（如 OpenAI + Antigravity/Gemini）批量修改时，模型白名单/映射可能被跨平台策略覆盖，导致 OpenAI 账号的关键模型映射丢失。
**修复**：在批量修改中补回正确的透传映射（如 `gpt-5.3-codex -> gpt-5.3-codex-spark`）。批量操作前按平台分组，不要混选不同平台账号。

### pnpm-lock.yaml 未同步
`package.json` 新增依赖后，CI 使用 `pnpm install --frozen-lockfile`，lock 文件不同步会导致 CI 失败。解决：`cd frontend && pnpm install && git add pnpm-lock.yaml`

### node_modules 冲突
之前用 npm 装过 `node_modules` 后再用 pnpm 会报 `EPERM` 错误。解决：`rm -rf frontend/node_modules && pnpm install`

## 分支策略

- **禁止**推送 `fix/openai-connection-test` 分支到 `main`
- 只能将 `main` merge 到 `fix/openai-connection-test`

## CI/CD

| Workflow | 触发条件 | 检查内容 |
|----------|----------|----------|
| **backend-ci.yml** | push, PR | 单元测试 + 集成测试 + golangci-lint |
| **security-scan.yml** | push, PR, 每周一 | govulncheck + gosec + pnpm audit |
| **release.yml** | tag `v*` | 构建发布（PR 不触发） |

## Fork 信息

- 上游仓库：`Wei-Shaw/sub2api`
- Fork 仓库：`bayma888/sub2api-bmai`
- 当前分支：`zhiguofan`
- 同步上游：`git fetch upstream && git merge upstream/main`
