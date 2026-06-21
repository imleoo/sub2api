# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

**tokenpanel**（fork: bayma888/tokenpanel-bmai，分支: zhiguofan）是一个 AI API 网关平台，用于订阅配额分发。从上游 AI 订阅（OpenAI、Anthropic/Claude、Gemini、Antigravity 等）生成 API Key，分发给用户，支持计费、负载均衡和请求转发。

**技术栈**：Go 1.26（具体小版本以 `backend/go.mod` 为准）+ Gin + Ent ORM；Vue 3.4+（Vite 5 + TailwindCSS + Pinia）+ PostgreSQL 18 + Redis 8

### 延伸阅读（优先于全局搜索）

在动手前如果需要更深背景，先看这里再决定要不要 grep：

- [`claudedocs/系统架构设计文档.md`](claudedocs/系统架构设计文档.md) — 整体架构（分层、网关、后台服务）
- [`claudedocs/repo-wiki.md`](claudedocs/repo-wiki.md) — 仓库结构 wiki
- [`claudedocs/api-reference.md`](claudedocs/api-reference.md) / [`API_DOCUMENTATION.md`](API_DOCUMENTATION.md) — REST API 契约
- [`claudedocs/admin-manual.md`](claudedocs/admin-manual.md) — 管理后台行为与字段含义
- [`claudedocs/自定义开发功能列表.md`](claudedocs/自定义开发功能列表.md) — fork 自定义功能、高风险文件、合并清单（**唯一源**）
- [`DEV_GUIDE.md`](DEV_GUIDE.md) — 仓库自带的开发指南

> **多 AI 助手规则文件的关系**：
> - `CLAUDE.md`（本文件）—— Claude Code 的唯一规则源
> - `AGENTS.md` —— 指向 `CLAUDE.md` 的 symlink（其它 agent 框架的默认入口）
> - `CODEBUDDY.md` —— CodeBuddy Code 的独立规则文件，**不与 CLAUDE.md 自动同步**，内容偏旧（如硬编码 Go 小版本）。修改本文件时不要顺手改 CODEBUDDY.md；除非明确接到 CodeBuddy 相关任务，否则视为只读

## zhiguofan 分支差异化开发

fork 功能列表、高风险文件、合并检查清单详见 **[`claudedocs/自定义开发功能列表.md`](claudedocs/自定义开发功能列表.md)**，该文档为唯一维护源，本节不再重复。

### 版本与同步策略

- `main` 对齐上游，版本保持上游 `0.x.y`
- `zhiguofan` 使用 fork 版本线，`backend/cmd/server/VERSION` 主号固定为 `1`，次/修订号跟随上游（上游 `0.x.y` → 本分支 `1.x.y`）。**实际版本必须以仓库内 `backend/cmd/server/VERSION` 文件为准**（截至本次更新为 `1.1.136`，仅作分支新鲜度参考，不要在其他文档/代码中硬编码该值）
- 同步上游优先使用 `./script/sync_upstream_to_zhiguofan.sh`，该脚本负责 `upstream/main → main → origin/main → zhiguofan → origin/zhiguofan`
- 脚本同步到 `zhiguofan` 后会把版本主号改为 `1`；如果手动 merge，必须手动检查 `backend/cmd/server/VERSION`
- `AGENTS.md` 是指向 `CLAUDE.md` 的 symlink，保留单一规则源，不要复制成两份。修改前用 `test -L AGENTS.md && readlink AGENTS.md`（应输出 `CLAUDE.md`）确认；若发现它变成了普通文件，恢复方式：`rm AGENTS.md && ln -s CLAUDE.md AGENTS.md`
- `frontend/package-lock.json` 是历史遗留文件；本项目开发仍以 pnpm 为准，新增依赖时优先维护 `pnpm-lock.yaml`
- 根目录 `.playwright-mcp/` 是 MCP Playwright 浏览器自动化的运行产物（截图、trace、临时会话等），**已在 `.gitignore` 中忽略**，不属于业务代码。不要在此目录下编辑或新增源文件；需要清理时可整目录删除

### 合并后最低验证

```bash
git diff --check
cd backend && go test -tags=unit ./internal/service ./internal/repository ./internal/server ./internal/handler/...
cd frontend && pnpm exec vitest run src/views/admin/__tests__/SettingsView.spec.ts src/composables/__tests__/usePersistedPageSize.spec.ts
cd frontend && pnpm exec vitest run src/views/user/__tests__/ModelsView.spec.ts src/i18n/__tests__/usageServiceTierLocales.spec.ts
cd frontend && pnpm run lint:check
```

## 常用命令

### 根目录

> 当前 `Makefile` 是空文件（1 字节占位），没有可用 target。所有构建/测试/生成/开发环境命令请直接进入 `backend/`、`frontend/` 或调用 `script/dev_local.sh`（见下方）。
>
> 如未来要新增 Makefile target，**先与 `script/dev_local.sh`、`script/sync_upstream_to_zhiguofan.sh` 比对**，避免与脚本入口重复或行为不一致。

### 后端（Go）

> 以下命令全部在 `backend/` 目录下执行（包含 `go generate ./ent` 与 `go generate ./cmd/server`）。

```bash
cd backend

# 运行服务器
go run ./cmd/server/

# 构建（含前端嵌入）
go build -tags embed -o tokenpanel ./cmd/server

# 单元测试（-tags=unit 会过滤掉 integration/e2e build tag 文件）
go test -tags=unit ./...

# 运行指定测试
go test -tags=unit -run TestFunctionName ./internal/handler/...

# 集成测试（需要 PostgreSQL + Redis，通过 testcontainers 提供）
go test -tags=integration ./...

# E2E 测试
# - 历史黑盒套件（需自行起服务 + 预置网关 key，BASE_URL/CLAUDE_API_KEY 等 env）：
go test -tags=e2e -v -timeout=300s ./internal/integration/...
# - 全功能自包含套件（自动起服务 + admin API seed + 计费/配额/限流/CRUD 断言）：
#   上游凭证写入本地 script/e2e.env（已 gitignore，勿提交；模板 script/e2e.env.example）
cp script/e2e.env.example script/e2e.env   # 然后编辑填入真实 key（仅首次）
./script/e2e-test.sh                        # 自动加载 e2e.env；或 cd backend && make test-e2e

# Lint 检查
golangci-lint run ./...

# Schema 变更后重新生成 Ent ORM（在 backend/ 下执行）
go generate ./ent

# Wire DI 变更后重新生成（在 backend/ 下执行）
go generate ./cmd/server
```

### 前端（Vue，必须用 pnpm）
```bash
cd frontend

pnpm install        # 安装依赖（禁止使用 npm）
pnpm dev            # 启动热重载开发服务器
pnpm build          # TypeScript 类型检查 + 构建
pnpm run typecheck  # 仅运行 TypeScript 类型检查（不构建）
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

### 插件架构（`internal/plugin/`）

zhiguofan 分支独有的可插拔功能目录，每个插件自包含 handler/repository/middleware/wire。当前该目录为空（原 `promptanalytics` 词云插件已于 2026-06-08 整体移除，见迁移 `159_drop_keyword_stats.sql`）；新增 fork 插件时沿用「自包含 + 独立 wire ProviderSet」约定。

### 支付集成（`internal/payment/`）

工厂模式多支付提供商：Alipay、WxPay、Stripe、EasyPay，带负载均衡。费用计算与支付商逻辑解耦。

### 前端测试覆盖率

Vitest 配置要求语句/分支/函数/行均达到 80% 覆盖率（`frontend/vitest.config.ts`）。测试与实现代码放在同目录（`*.spec.ts`）。

## 关键开发规则

- **前端必须使用 pnpm**（禁止 npm）。每次修改依赖都要提交 `pnpm-lock.yaml`
- **Go 版本以 `backend/go.mod` 为准**（当前 `go 1.26.3`），CI 通过 `go-version-file` 自动读取，不要在文档/CI 中硬编码具体小版本
- **Ent schema 变更**：修改 `ent/schema/*.go` 后必须运行 `go generate ./ent`，并提交生成的代码
- **Wire DI 变更**：修改 Wire providers 后运行 `go generate ./cmd/server`。若该命令因无法联网拉取 `google/wire` 工具依赖而失败，可手动同步 `cmd/server/wire_gen.go`（例如给某个 `NewXxxHandler(...)` 补实参），其它 fork 注入链（lingjing、provider-pricing、model-pricing）均同此惯例
- **接口变更**：给 Go interface 新增方法后，**所有**实现该接口的 test stub 都必须补全。查找方式：`grep -r "type.*Stub.*struct\|type.*Mock.*struct" internal/`
- **golangci-lint v2.9**：CI 自动运行 lint，推送前确保代码通过检查
- **GitHub Workflows 触发器**：zhiguofan 分支所有 `.github/workflows/*.yml` 的 `on:` 必须仅保留 `workflow_dispatch:`（详见"已知陷阱"中的踩坑场景）

## 已知陷阱

### 批量修改账号导致模型映射丢失
**现象**：前端测试看起来正常，但通过 API 调用时返回 `Service temporarily unavailable`。
**根因**：同时选中不同平台账号（如 OpenAI + Antigravity/Gemini）批量修改时，模型白名单/映射可能被跨平台策略覆盖，导致 OpenAI 账号的关键模型映射丢失。
**修复**：在批量修改中补回正确的透传映射（如 `gpt-5.3-codex -> gpt-5.3-codex-spark`）。批量操作前按平台分组，不要混选不同平台账号。

### pnpm-lock.yaml 未同步
`package.json` 新增依赖后，CI 使用 `pnpm install --frozen-lockfile`，lock 文件不同步会导致 CI 失败。解决：`cd frontend && pnpm install && git add pnpm-lock.yaml`

### node_modules 冲突
之前用 npm 装过 `node_modules` 后再用 pnpm 会报 `EPERM` 错误。解决：`rm -rf frontend/node_modules && pnpm install`

### 上游同步会覆盖 GitHub Workflows 触发器
**现象**：合并完上游后，CI 突然在 `push` / `pull_request` / `schedule` 上自动触发，消耗云端配额或泄露非预期构建。
**根因**：zhiguofan 分支约定所有 workflow 仅保留 `workflow_dispatch:`，但上游 `.github/workflows/*.yml` 携带其它触发器，`git merge upstream/main` 会覆盖本地 `on:` 配置。
**修复**：每次跑完 `./script/sync_upstream_to_zhiguofan.sh` 或手动 merge 上游后，逐文件检查 `.github/workflows/*.yml`，把 `on:` 恢复为仅 `workflow_dispatch:`，再 commit。

## CI/CD

所有 workflows 在 zhiguofan 分支均仅通过 `workflow_dispatch:` 手动触发（不自动触发）。

| Workflow | 检查内容 |
|----------|----------|
| **backend-ci.yml** | 单元测试 + 集成测试 + golangci-lint v2.9 |
| **security-scan.yml** | govulncheck + gosec + pnpm audit |
| **docker-push.yml** | 构建并推送 Docker 镜像 |
| **release.yml** | 构建发布（需输入 tag 参数） |
| **sync-upstream.yml** | 同步上游 main 到本 fork |

## Fork 信息

- 上游仓库：`Wei-Shaw/tokenpanel`
- Fork 仓库：`bayma888/tokenpanel-bmai`
- 当前分支：`zhiguofan`
- 同步上游：`git fetch upstream && git merge upstream/main`
