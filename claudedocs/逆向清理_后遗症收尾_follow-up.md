# 逆向清理后遗症收尾 — 剩余项 Follow-up 清单

> 生成日期：2026-06-15 ｜ 分支：feature/reverse-cleanup-v6
> 背景：逆向清理（删 antigravity/OAuth/Codex）采取了大量「就地阉割」（让守卫恒返回常量、stub 方法）而非「删分支」，留下成片不可达死代码。本轮已收尾大部分（见下「已完成」），本文件列出**剩余三类**，交给一个**带 integration/e2e + `golangci-lint unused`** 的专项一次性清除。

## 已完成（本轮，已提交、全程 build+vet+unit 绿）

| commit | 内容 |
|---|---|
| `1fcc6c06` | usage_logs 批量插入 CTE 列错位 bug |
| `a981d3d9` | 孤儿 handler/路由/binding、Internal500、thinking plumbing、token_version、i18n 孤儿、误导注释、前端 refresh/privacy UI |
| `5df6d30b` | **`account.IsOAuth()` 彻底消除** + 全部死分支 |
| `7525688f` | 删 `applyClaudeCodeOAuthMimicryToBody` + `buildOAuthMetadataUserIDFromBody` 孤儿 + 死指纹块 |
| `8e79a45c` | gemini compat 的 `isOAuth` 死分支链（const/`unwrapIfNeeded`/参数+分支） |

---

## 剩余 1：网关 `tokenType == "oauth"` 死分支 + constants.go OAuth 符号（🔴 热路径，必须带 integration/e2e）

**为何留到专项**：`GetAccessToken` 只返回 `apikey/service_account/bedrock/upstream`，**永不返回 `"oauth"`**，故所有 `tokenType == "oauth"` 分支不可达。但 `tokenType`/`mimicClaudeCode` 串在 `buildUpstreamRequest`/`buildCountTokensRequest`/`computeFinalAnthropicBeta`/`computeFinalCountTokensAnthropicBeta` 参数链上，还参与**实际鉴权头拼装（Bearer vs x-api-key）**。unit 测试覆盖不到转发行为正确性 —— 必须 `-tags=e2e ./internal/integration` + 真实 apikey/vertex 转发验证。

**范围（实测）**：
- `internal/service/gateway_service.go`：**10 处** `tokenType == "oauth"`（含 `computeFinalAnthropicBeta` ~6320、`computeFinalCountTokensAnthropicBeta`、`buildUpstreamRequest` 鉴权头/mimic 头分支、`buildCountTokensRequest`）。
- 删除这些 `if tokenType == "oauth" {…}` 死分支后，`tokenType`/`mimicClaudeCode` 形参在多个函数里变为未用 → 需逐层去参数并改调用点（调用点现已全部传 `false`/apikey）。
- 解开后 `internal/pkg/claude/constants.go` 的以下符号将无引用，可一并删除（KEEP 符号见 v6 方案 §9.1，**勿误删** `APIKeyBetaHeader`/`APIKeyHaikuBetaHeader`/`BetaClaudeCode`/`BetaInterleavedThinking`/`BetaFineGrainedToolStreaming`/`DefaultHeaders`/`CLICurrentVersion`/`ModelIDOverrides`/`NormalizeModelID`/`DenormalizeModelID`）：
  - `BetaOAuth`（当前 3 文件引用，均在 oauth 死分支）
  - `FullClaudeCodeMimicryBetas`（2 文件）
  - `DefaultBetaHeader`（2 文件）
  - `HaikuBetaHeader`（1 文件）
  - `CountTokensBetaHeader`（2 文件）

**执行顺序建议**：先删 `computeFinalAnthropicBeta`/`computeFinalCountTokensAnthropicBeta` 的 `tokenType=="oauth"` 分支 → 去 `tokenType`/`mimicClaudeCode` 形参 → 改调用点 → 删 `buildUpstreamRequest`/`buildCountTokensRequest` 内的 oauth 鉴权头/mimic 头分支 → 删 constants.go 5 符号。每步 `go build ./... && go vet ./...`；最后 `go test -tags=e2e -run xxx_NONEXISTENT ./internal/integration`（仅编译）+ 真实转发冒烟。

**门禁**：`rg -n 'tokenType == "oauth"|claude\.BetaOAuth|FullClaudeCodeMimicryBetas' internal` 应清零（除 KEEP 符号）。

---

## 剩余 2：`RequirePrivacySet` / `IsPrivacySet` 死功能（🟡 涉及 Ent schema + 迁移）

**现状**：`Account.IsPrivacySet()`（`internal/service/account.go`）恒 `return true`（注释："当前所有平台均无 privacy 概念"），其 5 处调用 `schedGroup.RequirePrivacySet && !acc.IsPrivacySet()` 恒为 false → privacy 调度门控整体失效（inert）。前端 setPrivacy UI 已在本轮删除。

**为何留到专项**：`RequirePrivacySet` 是 `groups` 表的 Ent schema 字段，彻底删除需：
1. 删 `IsPrivacySet()` 方法 + 5 处调用点（`gateway_service.go:3182/3291/3433/3539`、`openai_account_scheduler.go:923` 行号会漂移）。
2. 删 `ent/schema/group.go` 的 `RequirePrivacySet` 字段 → `go generate ./ent`（牵动 group_create/update/mutation/where/runtime/migrate 全套）。
3. 补前向迁移 `XXX_drop_group_require_privacy_set.sql`：`ALTER TABLE groups DROP COLUMN IF EXISTS require_privacy_set;`（参照 v6 方案 §8.6 列删惯例）。
4. 前端若有 `RequirePrivacySet` 分组开关 UI 一并清。

**门禁**：`rg -n 'RequirePrivacySet|IsPrivacySet' backend frontend` 清零；`go generate ./ent && go build ./...`。

---

## 剩余 3：openai_*.go 的 unused 孤儿函数海（🟢 低风险，建议 `golangci-lint unused` 一次性扫）

**为何留到专项**：用 `golangci-lint run`（项目已配 v2.9）或 staticcheck 的 `unused` linter 能一次性列出**全部**死函数，比逐个 IDE 诊断更全。逐个手删易遗漏级联新产生的孤儿。

**已实测确认零调用方（含测试，可直接删）**：
| 文件 | 函数 |
|---|---|
| `openai_images_responses.go` | `handleOpenAIImagesOAuthNonStreamingResponse`、`handleOpenAIImagesOAuthStreamingResponse`、`openAIImagesStreamPrefix` |
| `openai_gateway_service.go` | `resolveOpenAIUpstreamOriginator`、`resolveOpenAICompactSessionID` |
| `openai_messages_bridge.go` | `isOpenAICompatMessagesBridgeRequestBody`、`isOpenAICompatMessagesBridgeContext` |
| `openai_messages_continuation.go` | `getOpenAICompatSessionTurnState`、`bindOpenAICompatSessionTurnState` |
| `openai_messages_todo_guard.go` | `appendOpenAICompatClaudeCodeTodoGuardToRequestBody` |
| `openai_codex_transform.go` | `applyCodexSparkImageUnsupportedInstructions`、`extractTextFromContent` |
| `account_credentials_persistence.go` | `persistAccountCredentials` |

**附带（cosmetic，linter `nil != nil`）**：`gateway_service.go` count_tokens 路径里恒 nil 的 `ctFingerprint`/`ctEnableFP` 死块（约 9594-9646 区域，行号会漂移）。

**执行**：`golangci-lint run ./...` 收集全部 `unused`/`unparam` → 逐文件删 → `go build && go vet && go test -tags=unit ./...`。删一批后重跑 linter 捕捉级联新孤儿，直至干净。

---

## 验收总标准

```
go build ./... && go vet ./...            # 全绿
go test -tags=unit ./internal/...         # 通过
go test -tags=e2e -run x ./internal/integration   # 至少可编译；剩余1必须真实转发冒烟
golangci-lint run ./...                   # unused 清零
cd frontend && pnpm run typecheck && pnpm run lint:check && pnpm test:run
rg -n 'tokenType == "oauth"|RequirePrivacySet|IsPrivacySet' backend/internal   # 清零
```
