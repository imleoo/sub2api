# Changelog (zhiguofan)

本文件记录 `zhiguofan` fork 分支相对上游 `Wei-Shaw/sub2api` 的同步与自定义改动。
版本号规则：上游 `0.x.y` → fork `1.x.y`（主号固定 1，次/修订跟随上游）。
逆向清理口径见 [`自定义开发功能列表.md`](自定义开发功能列表.md) 功能 35。

---

## [未发布] - 2026-07-08 — Playground（功能 38）bug 修复

代码 review 后修复 3 个 bug：
- **中文输入法回车误发送**：`PlaygroundView.vue` 输入框 `onKeydown` 增加 `!e.isComposing` 判断，输入法确认候选词的 Enter 不再触发发送。
- **「停止」无法中止生图**：`imageGenerate`/`imageEdit` 增加 `AbortSignal` 参数，`sendImage` 接入 `abortController`；停止时真正中断请求（避免继续计费），中断时移除空的助手气泡而非报错。
- **上传预览/文件顺序竞态**：`onFilePick` 改为顺序异步读取，文件与预览成对追加，保证 `uploadFiles[i]` 与 `attachmentPreviews[i]` 对应（此前多张大图并发读取可能错位）。

补充的可用性改进：
- **新对话入口**：对话态右上角加「＋ 新对话」按钮（`clearConversation`，流式中先中断再清空），此前只能刷新页面重置。
- **静默失败反馈**：`useAsEditInput`/`onFilePick` 达 4 张上限、`loadModels` 拉取失败均改为 toast 提示（此前静默）。
- **助手回复一键复制**：`MessageBubble` 流式结束后显示「复制」按钮（`navigator.clipboard`，不可用时静默）。
- 说明：回复仍以纯文本 `<pre>` 渲染（安全取舍，避免 Markdown 富文本的 XSS 面），未改。

门禁：前端 typecheck 0 错误、lint 0、playground 相关 vitest 25/25 通过。

---

## [1.1.146] - 2026-07-07 — 同步上游 0.1.146

同步上游 `0.1.145 → 0.1.146`（47 提交），冲突与逆向残留已按 fork 口径回炉。门禁：后端 build+vet+`-tags=unit`、前端 typecheck+lint+894 测试、fork 守护 ALL PASSED、E2E 21/21。

### 采纳的上游合法新功能
- **apikey 账号请求头覆写**（`account_header_override.go`）：保留 `ApplyHeaderOverrides` 装配点（网关转发 + 账号测试），剥离其 OAuth-header 分支。后端支持，前端配置 UI 暂未移植（fork 账号模态保持简化版）。
- 入站端点归一化 + `responses/compact` 端点区分（`endpoint.go`）。
- Redis SCAN 清理架构优化。
- 新增 OpenAI 模型 `gpt-5.6-sol/terra/luna`、非 -v1 OpenAI 模型 URL 支持。
- `ProvideAPIKeyService` 新增 `concurrencyService` 参数（`wire_gen.go` 经 `go generate` 重装，保全全部 fork 注入链）。

### 逆向回炉（保持 fork 已删状态）
- 保持删除 `grok_media.go`、`openai_gateway_count_tokens.go`、`openai_client_restriction_detector_test.go`。
- `billing_service.go` 拒绝上游重引入的 `fallbackPrices`、`pricing_service.go` 拒绝 `pricingData`/`matchOpenAIModel` —— 取 fork catalog(SSOT) 版。
- `openai_gateway_service.go` 的 codex 版本门控 403 文案回退为 fork 硬编码（上游 `CodexClientRestrictionMessage` 依赖已删的 min/max codex 版本门控）。
- 测试清理：`endpoint_test`/`account_header_override_test`/`openai_gateway_record_usage_test`/`openai_gateway_service_codex_cli_only_test` 中 antigravity/grok/OAuth/版本门控 的 fixture 与用例。
- 账号 3 模态（Create/Edit/Bulk）与 `GroupsView.vue` 取 fork 版，剥离 antigravity/OAuth UI。

### 货币
- 保住 1.1.145 修复的 `OrderTable`/`PaymentQRDialog` 货币口径；`PaymentStatusPanel.vue` 采纳上游支付重构。

### 测试
- **新增 `TestE2EFull_BillRequestIDWriteback`**（功能 27 显式断言）：真实网关请求验证 `Bill-Request-ID` 三种回写行为（下游合法值原样回写 / 缺失回退 X-Client-Request-ID / 超 64 字符回退），新增 `apiCallH` 助手返回响应头。

---

## [1.1.145] - 2026-07-06 — 同步上游 0.1.145 回炉

上游 `0.1.145` 的原始合并处于「未清理·不编译」状态（199 处编译/类型错误）。本次对 merge 带回的上游逆向代码重新套用逆向清理口径，恢复到可编译 + 全门禁通过。

### 后端逆向回炉
- 删除上游复活的纯逆向文件：`account_codex_import`、`antigravity_token_refresher`、`token_refresh_service`（含各自测试）。
- 三方合并文件外科清理：`model_rate_limit`/`account`/`ratelimit_service`/`setting_service`/`admin_service` 剥离 antigravity/grok/OAuth 死符号，保留 Fable 限流、OpenAI 高级调度器设置等上游新功能。
- `account_usage_service`：删逆向主动抓取（tls scraping / Antigravity / Grok / ClaudeUsageResponse），保留 Fable 被动用量路径。
- `openai_account_scheduler`：采纳上游加权高级调度器，剥离 compatible-platform/privacy/shadow/OAuth订阅 依赖。
- `gateway_handler`/`openai_gateway_handler`/`setting_handler` 清理 antigravity/grok/Codex/ClaudeOAuthSystemPrompt。

### 前端逆向回炉
- `useModelWhitelist.ts`：删 antigravity/grok 模型与预设，恢复被坏合并丢失的 fork 12 lingjing(灵境) 白名单定义。
- `api/admin/accounts.ts`：删 Codex 逆向 API（importCodexSession/createOpenAICodexPAT）。
- `AccountUsageCell.vue`/`EditAccountModal.vue`：恢复 fork 清理版（连同 `*.spec.ts`）。

### 货币修复（payment）
- `OrderTable.vue`/`PaymentQRDialog.vue`：修复共享订单表/支付弹窗货币口径——paid/pay_amount 按订单货币、credited(入账余额)恒 USD（对齐 `AdminOrderTable` 正确实现，修复 USD 订单被误显示为 ¥）。
- `UsageView.spec.ts`：CSV 导出测试期望对齐 fork 组件的货币标注口径。

### 守护
- `script/check_fork12_guards.sh`：ForcePlatform 规则预期 3→1（antigravity 路由已于逆向清理 P2 移除）。

---

## 开发流程

- **推送前门禁（pre-push hook）**：`script/pre_push_check.sh` 在每次 `git push` 前检查本次推送范围——①有实质源码改动时 `CHANGELOG.md` 必须已更新；②新增的 fork 独有源码文件（`.go/.ts/.vue`，排除测试/生成/上游已有）必须已在 `自定义开发功能列表.md` 记录。不满足则阻塞推送。安装：`./script/install_git_hooks.sh`（克隆后运行一次）；绕过：`git push --no-verify` 或 `PREPUSH_SKIP=1 git push`。

## 架构说明

- fork 移除了 OAuth 账号类型（`AccountTypeOAuth`/`AccountTypeSetupToken`），`IsOpenAIOAuth`/`IsAnthropicOAuthOrSetupToken` 恒为 `false`。上游每次同步都会重新引入 codex/grok/antigravity/oauth，合并后须按逆向清理口径剥离。
- 同步流程与高风险文件清单见 [`自定义开发功能列表.md`](自定义开发功能列表.md)；合并后验证跑 `script/check_fork12_guards.sh` + `script/e2e-test.sh`。
- E2E 用持久化 dev_local 库，`e2e-*` 命名的测试数据会累积；当分组数超过 `page_size=100` 时 `AdminAccountGroupCRUD` 会误报，清理 `e2e-*` 测试数据即可。
