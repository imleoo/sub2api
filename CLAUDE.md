# CLAUDE.md

**每次交互必须叫爸爸，探索处理代码必须用 LSP**
This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

**tokenpanel**（fork: imleoo/tokenpanel，上游: Wei-Shaw/sub2api，分支: zhiguofan）是一个 AI API 网关平台：从上游 AI 供应商（OpenAI、Anthropic/Claude、Gemini 及 generic/lingjing 等正规 API 接入）生成 API Key 分发给用户，支持计费、负载均衡和请求转发。注：上游的「订阅逆向」能力（antigravity/OAuth/Codex）本分支已整链删除，见功能 35。

**技术栈**：Go 1.26（小版本以 `backend/go.mod` 为准）+ Gin + Ent ORM；Vue 3.4+（Vite 5 + TailwindCSS + Pinia）+ PostgreSQL 18 + Redis 8

### 延伸阅读（优先于全局搜索）

动手前如需更深背景，先看这里再决定要不要 grep：

- [`claudedocs/系统架构设计文档.md`](claudedocs/系统架构设计文档.md) — 整体架构（分层、网关、后台服务）
- [`claudedocs/repo-wiki.md`](claudedocs/repo-wiki.md) — 仓库结构 wiki
- [`claudedocs/api-reference.md`](claudedocs/api-reference.md) / [`API_DOCUMENTATION.md`](API_DOCUMENTATION.md) — REST API 契约
- [`claudedocs/admin-manual.md`](claudedocs/admin-manual.md) — 管理后台行为与字段含义
- [`自定义开发功能列表.md`](自定义开发功能列表.md) — fork 自定义功能、高风险文件、合并清单（**唯一源**）
- [`CHANGELOG.md`](CHANGELOG.md) — 同步/改动详录；[`DEV_GUIDE.md`](DEV_GUIDE.md) — 仓库自带开发指南

> **多 AI 助手规则文件**：`CLAUDE.md` 是 Claude Code 唯一规则源；`AGENTS.md` 是指向它的 symlink（勿复制成两份，恢复：`rm AGENTS.md && ln -s CLAUDE.md AGENTS.md`）；`CODEBUDDY.md` 是 CodeBuddy 独立规则文件，内容偏旧，非 CodeBuddy 任务视为只读、不要顺手同步。

## zhiguofan 分支差异化开发

fork 功能列表、高风险文件、合并检查清单详见 **[`自定义开发功能列表.md`](自定义开发功能列表.md)**（唯一维护源，本节不重复）。

### 版本与同步策略

- `main` 对齐上游（版本 `0.x.y`）；`zhiguofan` 用 fork 版本线：`backend/cmd/server/VERSION` 主号固定 `1`，次/修订跟随上游（实际版本以该文件为准，勿在文档/代码中硬编码）
- 同步上游用 `./script/sync_upstream_to_zhiguofan.sh`（`upstream/main → main → origin/main → zhiguofan → origin/zhiguofan`，自动改版本主号）；手动 merge 必须手动检查 `VERSION`
- `frontend/package-lock.json` 是历史遗留，开发以 pnpm 为准（维护 `pnpm-lock.yaml`）
- 根目录 `.playwright-mcp/` 是 MCP Playwright 运行产物（已 gitignore），不要在其中编辑源文件，可整目录删除

### 合并后最低验证

```bash
git diff --check
cd backend && go test -tags=unit ./internal/service ./internal/repository ./internal/server ./internal/handler/...
cd frontend && pnpm exec vitest run src/views/admin/__tests__/SettingsView.spec.ts src/composables/__tests__/usePersistedPageSize.spec.ts
cd frontend && pnpm exec vitest run src/views/user/__tests__/ModelsView.spec.ts src/i18n/__tests__/usageServiceTierLocales.spec.ts
cd frontend && pnpm run lint:check
```

### 推送前门禁（pre-push hook）

每次 `git push` 前由 `.git/hooks/pre-push` → `script/pre_push_check.sh` 检查**本次推送范围**（5 项，细节见脚本头注释）：

1. **CHANGELOG 必更**：范围内有实质源码改动（`.go/.ts/.vue`，排除测试）时 `CHANGELOG.md` 必须同步更新。
2. **功能列表一致性**：范围内新增的 fork 独有源码文件必须已在 `自定义开发功能列表.md` 出现。
3. **登记 fork 文件逐个复核**：改动了功能列表风险表登记文件（🔴/🟡/🟢 三档全部）时，钩子逐个打印 diff，需双重留痕放行：(a) `CHANGELOG.md` 或功能列表新增一行「高风险复核：」开头的结论；(b) 交互终端 y/N 确认（读 `/dev/tty`——GUI git 客户端会被阻塞，改用命令行 push 或 `--no-verify`）。
4. **e2e 全量测试**：有实质源码改动时内联跑 `./script/e2e-test.sh`（需本地 `script/e2e.env` 真实凭证，耗时数分钟；检查 1-3 失败则跳过 fail-fast）。
5. **逆向订阅关键词扫描**：推送范围新增行命中 `PlatformAntigravity`/`AccountTypeOAuth`/`AccountTypeSetupToken`/裸词 `antigravity` 时要求人工确认（防上游残留静默落地，见已知陷阱）。

- 安装：`./script/install_git_hooks.sh`（一次）；绕过：`git push --no-verify` 或 `PREPUSH_SKIP=1`；改检查逻辑改 `script/pre_push_check.sh`，勿手改 `.git/hooks/pre-push`
- 因此**新增 fork 功能/文件时，同一批提交内就要更新 `CHANGELOG.md` 与 `自定义开发功能列表.md`**；同步上游后推送务必留出跑 e2e + 逐个核对高风险 diff 的时间。

## 常用命令

> 根目录 `Makefile` 是空占位文件，无可用 target；命令直接进 `backend/`、`frontend/` 或用 `script/dev_local.sh`。

### 后端（Go，均在 `backend/` 下执行）

```bash
go run ./cmd/server/                        # 运行服务器
go build -tags embed -o tokenpanel ./cmd/server   # 构建（含前端嵌入）
go test -tags=unit ./...                    # 单元测试（过滤 integration/e2e tag）
go test -tags=unit -run TestFunctionName ./internal/handler/...
go test -tags=integration ./...             # 集成测试（testcontainers 提供 PG+Redis）
go test -tags=e2e -v -timeout=300s ./internal/integration/...   # 历史黑盒 e2e（需自起服务+env）
./script/e2e-test.sh                        # 全功能自包含 e2e（根目录执行；首次先 cp script/e2e.env.example script/e2e.env 填真实 key，已 gitignore）
golangci-lint run ./...                     # Lint
go generate ./ent                           # Ent schema 变更后
go generate ./cmd/server                    # Wire DI 变更后
```

### 前端（Vue，必须用 pnpm，禁止 npm）

```bash
cd frontend
pnpm install / pnpm dev / pnpm build        # 安装 / 热重载 / 类型检查+构建
pnpm run typecheck / pnpm run lint:check    # 仅类型检查 / Lint
pnpm test:run / pnpm test:coverage          # 测试 / 覆盖率
```

### 本地调试脚本

```bash
./script/dev_local.sh up|down|status|logs   # 连本机 Docker PG+Redis，后端 :8082 前端 :3002
# 端口冲突：BACKEND_PORT=18082 FRONTEND_PORT=13002 ./script/dev_local.sh up
./script/sync_upstream_to_zhiguofan.sh      # 同步上游（需 GitHub 网络）
```

## 架构

### 关键设计模式

1. **严格分层**：Handler → Service → Repository → Ent ORM（golangci-lint 强制：Service 不能导入 Repository，Handler 不能直接导入 Repository）
2. **依赖注入**：Google Wire，生成文件 `cmd/server/wire_gen.go`；Application 内嵌 `*http.Server` + `Cleanup` 优雅关闭
3. **API 响应契约**：`{ code, message, data }`，前端 Axios 拦截器自动解包
4. **API 路由**（`internal/server/routes/`）：公开 `/api/v1/auth/*` 等；用户 `/api/v1/dashboard` 等；管理员 `/api/v1/admin/*`；网关 `/v1/messages`、`/v1/chat/completions`；健康检查 `/health`
5. **数据库**：Ent + PostgreSQL；`ent/schema/` 变更后必须 `go generate ./ent`；迁移运行器 `repository/migrations_runner.go`（新表必须手写 SQL 迁移，Ent 不自动建表）
6. **后台服务生命周期**：大量后台 goroutine（定时报告/清理/告警/轮询），Wire cleanup 优雅关闭时并行停止
7. **前端嵌入**：`go build -tags embed` 单文件部署
8. **前端架构**：Vue 3 Composition API + Pinia + 懒加载路由（守卫检查 `requiresAuth`/`requiresAdmin`）；Axios 401 自动刷新重试队列；vue-i18n（后端用 `Accept-Language`）；API 调用统一走 `frontend/src/api/`
9. **Simple 模式**：`RUN_MODE=simple` + `SIMPLE_MODE_CONFIRM=true` 禁用计费/配额
10. **自动初始化**：首次运行 Web 安装向导；Docker 用 `AUTO_SETUP=true` 静默初始化

### 网关代理架构（`internal/pkg/` + `internal/service/`）

平台协议适配包：`pkg/claude/`（请求格式化 + cache-control，含 `conversecompat`/`kirocompat`）、`pkg/openai/`、`pkg/gemini/`、`pkg/apicompat/`（跨协议转换桥）。网关服务：`gateway_service.go`(Claude 主代理)、`openai_gateway_service.go`、`lingjing_gateway_service.go`(fork)，generic 账号经 `generic_gateway.go` 复用三者。关键特性：Claude Sticky Session（1h TTL）、Singleflight 限速缓存、H2C、账号选择等待队列。

### 其它

- **插件目录 `internal/plugin/`**：fork 独有可插拔约定（自包含 + 独立 wire ProviderSet），当前为空（词云插件已移除）
- **支付 `internal/payment/`**：工厂模式多提供商（Alipay/WxPay/Stripe/EasyPay），费用计算与支付商解耦
- **前端测试覆盖率**：Vitest 要求语句/分支/函数/行 80%（`frontend/vitest.config.ts`）；测试与实现同目录（`*.spec.ts`）

## 关键开发规则

- **前端必须 pnpm**（禁止 npm），改依赖必须提交 `pnpm-lock.yaml`
- **Go 版本以 `backend/go.mod` 为准**，CI 用 `go-version-file` 自动读取，勿在文档/CI 硬编码小版本
- **Ent schema 变更**：`go generate ./ent` 并提交生成代码
- **Wire DI 变更**：`go generate ./cmd/server`；若因无法联网拉取 wire 工具失败，可手动同步 `cmd/server/wire_gen.go`（fork 注入链均同此惯例）。注意 generate 会丢失手改 setter（`SetEndpointRepository` ×4、`SetModelRoutingService` ×1），见功能列表功能 25 合并注意
- **接口变更**：Go interface 新增方法后补全**所有** test stub（`grep -r "type.*Stub.*struct\|type.*Mock.*struct" internal/`）
- **golangci-lint v2.9**：推送前确保通过
- **GitHub Workflows**：所有 `.github/workflows/*.yml` 的 `on:` 仅保留 `workflow_dispatch:`（见已知陷阱）

## 已知陷阱

### 批量修改账号导致模型映射丢失（已自动防护）
混选多平台账号批量修改曾导致模型白名单/映射被跨平台覆盖（API 报 `Service temporarily unavailable`）。`bd0bf3c44` 起 `BulkEditAccountModal.vue` 的 `isMixedPlatform` 已自动禁用模型限制写入，配回归测试 `TestE2EFull_BatchEditModelMappingRepro`。仅当该组件重构导致 `isMixedPlatform` 失效才会复发。

### fork 专属代码块被静默丢弃（已出现 5 次，触发者不止上游合并）
编译/测试全过，但 fork 独有字段/分支运行时悄悄失效（`git log -S` 也难发现）。案例：① workflows 触发器被覆盖；② 批量改账号丢映射；③ 0.1.147 静默删掉 `setting_update.go::buildSystemSettingsUpdates` 尾部 fork 字段写入（`currency_mode` 等重启即丢，`07af185e3` 修复 + 回归测试 `setting_fork_fields_persist_test.go`）；④ **本地功能分支合入也会触发**——2026-07-15 team-collaboration 合入时重跑 wire generate，`wire_gen.go` 的 5 处手改 setter（`SetEndpointRepository`×4 + `SetModelRoutingService`×1）全部丢失，generic 转发的 endpointRepo 恒 nil **带病运行一周**，直到 1.1.162 同步做 grep 复核才发现；⑤ 管理员「测试连接」的 `PlatformGrok` 分流在某次同步中丢失（grok 账号测试落 claude 兜底），1.1.162 按上游恢复。
**应对**：同步上游后除跑测试外，对功能列表风险表 🔴 高文件逐个人工 diff，不要只信编译和测试全绿（pre-push 检查 3 会强制这一步）；**任何触碰 `wire_gen.go` 的提交**（不限于同步）后必须 `grep -c '\.SetEndpointRepository('` = 4、`grep -c '\.SetModelRoutingService('` = 1；条目 ④⑤ 证明这类丢失可长期潜伏，值得为关键注入点补「装配存在性」单测而非只靠 grep 纪律。

### 上游合并静默带回「逆向订阅代码」残留（2026-07-21 实测）
与上一条相反方向：上游同步把功能 35 已删的 antigravity/OAuth 逆向代码**局部带回**（孤立辅助代码，编译/lint/测试全无告警）。2026-07-21 清理了三轮（antigravity 专属残留 → 同根因 OAuth 死代码 → `golangci-lint --enable-only unused` 补漏的后端孤儿函数簇），完整清单见 `CHANGELOG.md` 2026-07-21 段。
**应对**：`script/pre_push_check.sh` 检查 5 已自动扫描推送新增行的逆向关键词并要求人工确认。**合法保留点**（门禁不拦）：`servertiming` host 分类、`admin_account.go` 遗留字段丢弃表、`admin.groups.platforms.antigravity`/`accounts.upstream.baseUrlHint` 兼容存量数据的 key。此外命中即大概率是再次带回，按 `claudedocs/逆向清理_执行报告_v6.md` 重新删除。

### unused 报告 ≠ 死代码：删除前先判断是「孤儿」还是「断线」（2026-07-22 实测）
`golangci-lint unused` 报告的函数有两种截然不同的成因：**孤儿**（逆向清理后的真死代码，应删）与**断线**（功能是活的、只是调用点被静默丢弃，应恢复调用点——见上一条案例 ⑤：`testGrokAccountConnection` 被报 unused，真相是 `TestAccountConnection` 里的 grok 分流丢了）。机械批量删除会把断线误杀成永久失能。另一个盲区：**「函数 + 测试自闭环」**——死函数被自己的专属测试引用着，旧版 unused 不报、人工 grep 会误以为"有测试在用"（案例：`buildStableSessionSeed` 伪装路径 seed 及其 `_session_test.go` 互相掩护潜伏多轮清理）。
**应对**：每个 unused 符号删除前先问「这函数在 fork 语义下**该不该有**生产调用点」——该有 → 对照上游找回调用点；不该有 → 连同其专属测试一起删（只删函数留测试会编译失败，只删测试留函数会再次潜伏）。

### 集成测试（-tags=integration）长期不跑会积累坏例
合并最低验证清单与推送门禁都不含 `go test -tags=integration ./...`，坏测试可长期潜伏：1.1.162 补跑时暴露 zhiguofan 存量坏 fixture（`filter_by_type` 两个 apikey 账号却期望过滤后剩 1——当年剥离 OAuth 时机械替换 fixture 类型所致）。
**应对**：每次同步上游后至少补跑一次全量 integration（需本机 Docker，testcontainers 起 PG+Redis，约 3-5 分钟）。注意**管道退出码假象**：`go test ... | tail`/`| grep` 的退出码是管道末端命令的，会把 FAIL 掩盖成 exit 0——要么 `set -o pipefail`，要么直接看输出里的 `FAIL` 行。

### pnpm-lock.yaml 未同步
CI 用 `--frozen-lockfile`，lock 不同步即失败。解决：`cd frontend && pnpm install && git add pnpm-lock.yaml`

### node_modules 冲突
npm 装过再用 pnpm 报 `EPERM`。解决：`rm -rf frontend/node_modules && pnpm install`

### 上游同步会覆盖 GitHub Workflows 触发器
`git merge upstream/main` 会把 `on:` 覆盖为 push/pull_request/schedule，消耗配额。每次同步后逐文件检查 `.github/workflows/*.yml`，恢复为仅 `workflow_dispatch:` 再 commit。

## CI/CD

所有 workflows 在 zhiguofan 分支仅 `workflow_dispatch:` 手动触发：`backend-ci.yml`（单测+集成+lint）、`security-scan.yml`（govulncheck+gosec+pnpm audit）、`docker-push.yml`（镜像）、`release.yml`（需 tag 参数）、`sync-upstream.yml`（同步上游）。

## Fork 信息

- 上游：`Wei-Shaw/sub2api`（remote `upstream`）；Fork：`imleoo/tokenpanel`（remote `origin`；镜像 `imleoo/sub2api` = remote `mirror`）
- 当前分支：`zhiguofan`；同步：`git fetch upstream && git merge upstream/main`（优先用脚本）
