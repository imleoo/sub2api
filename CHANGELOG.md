# Changelog (zhiguofan)

本文件记录 `zhiguofan` fork 分支相对上游 `Wei-Shaw/sub2api` 的同步与自定义改动。
版本号规则：上游 `0.x.y` → fork `1.x.y`（主号固定 1，次/修订跟随上游）。
逆向清理口径见 [`自定义开发功能列表.md`](自定义开发功能列表.md) 功能 35。

---

## [1.1.151] - 2026-07-11 — 同步上游 0.1.151（61 提交，无破坏性重构）

### 附带修复：fork 自定义设置持久化回归（历史 0.1.147 合并遗留）

同步过程中发现「货币显示模式向导每次进后台都弹、保存不生效」。根因经 git 定位为**上一次**
0.1.147 同步的合并提交 `7c9e09d29` 静默删除了 `buildSystemSettingsUpdates`（写入）与
`GetPublicSettings`（公开读取）尾部的整段 fork 自定义字段块（merge commit 内删除，`git log -S`
不可见）。受影响字段：`currency_mode`/`cny_rate`/`ui_theme`/`show_overseas_models`/
`phone_register_enabled`/`password_login_enabled` 及三家短信（火山/腾讯/阿里）配置——这些设置
此前只写进内存缓存、重启即丢。`currency_mode` 因无 seed 默认值 + 有强制向导弹窗，成为首个暴露点。

- `backend/internal/service/setting_update.go`：`buildSystemSettingsUpdates` return 前逐字补回
  22 个 fork 字段的 `updates[SettingKey*]` 写入 + phone/email 互斥逻辑（从 `7c9e09d29^` 恢复）。
- `backend/internal/service/setting_public.go`：`GetPublicSettings` keys 白名单 + 返回字面量补回
  6 个 fork 公开字段（含 `cny_rate` 默认 6.8 解析）。
- 新增回归守护测试 `setting_fork_fields_persist_test.go`：断言 `UpdateSettings` 落库含全部 fork 字段，防下次上游合并再次覆盖。

**规模**：上游 61 提交、123 文件（+6819/-606）。上游内容集中在 OpenAI/Codex/apicompat
bugfix（tool_search、namespace 摊平撞名拒绝、Codex MCP 工具桥、GPT-5.6 计费/缓存计价、
setup-token 后台刷新）、compact/SSE 加固、usage/i18n 修正。无结构性重构。

### 冲突解决（21 个文本冲突 + 若干 auto-merge 语义冲突）
- **计费 SSOT 保留**：`pricing_service.go`/`billing_service.go` 删除上游重新引入的
  `fallbackPrices`/`initFallbackPricing`/`matchOpenAIModel`/`openAIGPT5*FallbackPricing`
  静态兜底（功能 26/34 计费走 catalog，不得内置厂商价）；仅采上游 `LongContext` 字段处理
  与 GPT-5.6 长上下文 legacy 判定（`usesOpenAILegacyLongContextPricing`，归一同用
  `normalizeKnownOpenAICodexModel`，无语义回退）。
- **OAuth 逆向链保持删除**（功能 35）：`token_refresher.go`/`_test.go`（modify/delete 保删）、
  `account_repo.go` 的 `ListOAuthRefreshCandidates`、`openai_gateway_{forward,passthrough,
  messages}.go`/`openai_ws_forwarder_payload.go`/`account_{test,usage}_service.go` 中所有
  `AccountTypeOAuth` 分支（`enforceCodexIdentityHeaders`/`overrideBrowserUserAgent`/
  `applyCodexOAuthTransform` 等 fork 无符号）全部剥离。生产代码 `AccountTypeOAuth` 残留 0。
- **保留上游新功能**：用户级 Fast/Flex 策略（`user_ids`，前端 `settings.ts` scope 去 `oauth`
  留 `user_ids`）、`stripOpenAIImageGenerationToolsFromRawPayload`（补 `encoding/json` import）、
  grok 被动配额快照、GPT-5.6 别名/max 变体展示。
- **spark 影子账号功能不引入**（fork 从未有）：删除 `TestMigration154*`（`154_account_spark_shadow.sql`
  上游有、fork 无）；migration 编号重复为 fork 历史常态（runner 按文件名字典序），非本次问题。
- **测试收敛**：删除 OAuth 专属测试（compact 降级 / responses effort / oauth 模型路由等）；
  账号仅作 setup 的通用测试改 `AccountTypeAPIKey`；`pricing_service_test.go` 恢复 fork 版本
  （上游新增全是 `pricingData` 字段/fallback 用例）；`UseKeyModal` 删 antigravity fable 用例。

### 验证
全量后端 `go test -tags=unit ./...` 全过；前端 typecheck + lint + 关键 vitest 全过；
fork 守护点（lingjing 路由、协议分流、`ProtocolBucketEnabled`、上游成本快照、provider-pricings、
`/models`）完好；`.github/workflows/*.yml` 触发器均为 `workflow_dispatch`；`go generate ./ent` 无 diff。

---

## [1.1.147] - 2026-07-10 — 同步上游 0.1.147（147 提交）+ Grok 官方 API 保留 + 逆向链再清理

**规模**：上游 147 提交、410 文件；上游把 fork 重度改造的三个巨型文件做了「纯移动拆分」
（`usage_log_repo.go` 4701→212 行拆 6 文件、`setting_handler.go` 3957→468 拆 5 文件、
`admin_service.go` 4409→642 拆 5 文件，另拆 `gateway_service.go`/`openai_gateway_service.go`），
fork 语义须逐函数重新落位。

### 合并方法（可复用）
以 `merge-base` 单体为 base、fork 单体为 ours、上游拆分文件为 theirs，逐个顶层函数做三方归并：
上游纯移动 → 直接采 fork 版；双方都改 → `git merge-file` 三方合并。共自动归并 64 个函数、
人工仲裁 3 处（`RecordUsage` 双段并存、两处 SSE `response.failed` 净化取上游）。

### 决策：保留 Grok 官方 API、删除 Grok 订阅逆向
- **保留**：`api.x.ai` 官方链路 —— `openai_gateway_grok.go`（守卫由 `AccountTypeOAuth` 改为
  `AccountTypeAPIKey`）、`grok_media.go` 生图/生视频、`pkg/xai` 的 URL/模型/配额头解析、
  被动配额快照（`grok_quota_snapshot` Extra 键，apikey 账号同样写入）、WS→HTTP 桥、
  `isOpenAIAccount()` 纳入 grok（复用 openai 运行时封锁/冷却）、迁移 157/158/170/171/172。
- **删除**：`grok_oauth_service/handler/client`、`grok_token_provider/refresher`、
  `grok_quota_service`（主动探测）、`pkg/xai/oauth.go`、前端 `useGrokOAuth.ts`/`api/admin/grok.ts`/
  `GrokQuotaProbeCell.vue`。
- **计费口径**：grok 价格不内置（遵循功能 26 SSOT + 功能 34「绝对不内置厂商价」），
  须由运营写入 `model_pricings`（`sync-maas` 或手工）；未定价时 fail-closed 而非按 0 计费。
  上游的 `TestGetModelPricing_Grok45OfficialFallback` 因此移除。

### 逆向链再清理（功能 35）
上游重新引入的整套订阅逆向已再次删除：antigravity 平台（`setting_*` 的 UA/fallback 设置、
`DefaultAntigravityModelMapping`、调度/网关分支）、Claude Code 拟态（`gateway_claude_oauth_body.go`
按符号拆分——非逆向工具迁入新建的 `gateway_claude_body.go`，OAuth 拟态整段丢弃）、
codex CLI 限制策略（`CodexRestrictionPolicy`/白名单/指纹信号/`/v1/models` 的 codex manifest 分支）、
spark 影子账号（`ListShadowsByParent`/`parentHealthyForShadow` 守卫、迁移 154/154a 未引入）、
`ListOAuthRefreshCandidates`（含 `type='oauth'` 死查询）。

**运行时地雷修复**：上游新代码 `ListCRSAccountIDs` 的 SQL 带 `parent_account_id IS NULL` 谓词，
而 fork 库无此列（spark 迁移未引入）→ 会直接 SQL 报错。已移除该谓词。

### 合并后修复（回答「本系统是否支持 count_tokens」时发现）
- **`/v1/messages/count_tokens` 对 OpenAI 分组的桥接被漏接**：上游 0.1.147 新增了
  `handler/openai_gateway_count_tokens.go` + `service/openai_gateway_count_tokens.go`
  （桥到官方 `POST /v1/responses/input_tokens`），但合并时 `routes/gateway.go` 保留了 fork 旧的
  「openai 入站一律 404」门 → 两个新文件成为零引用死代码（编译器不报错）。已接回：
  `platform == openai` 走桥接，其它 openai 协议入站（grok/generic）仍 404，anthropic 入站直转上游。
- **`script/e2e-test.sh` 缺 `-count=1`**：e2e 打的是外部 HTTP 服务，改服务端代码不会让 `go test`
  缓存失效，重跑会拿上一轮旧结果（假绿）。已补 `-count=1`。

### 采纳的上游修复
- `/v1/messages` 与 OpenAI 流式的 `response.failed` 错误透传规则（不再硬编码 502）
- **上下文超限不再触发 failover**（换账号无用，直接回写客户端错误）
- 流式 usage 漏计费修复（客户端断开后继续合并 usage）
- 鉴权绕过修复、`site_name`/`site_logo`/`doc_url` 的 XSS sanitize、Go 工具链 1.26.5（stdlib 漏洞）
- 批量生图（batch image）整功能、用户 Token 排行、管理员用户角色、版本徽章在线回退
- SSE 扫描器封装 `newUpstreamSSEScanner`（openai 侧不再复用 fork 的 64K buffer 池，属可接受收敛；
  gateway_* 路径仍复用池）

### fork 语义修复（合并期间发现并补回）
- `PublicSettingsInjectionPayload` 缺 6 个 fork 公开字段（`ui_theme`/`currency_mode`/`cny_rate`/
  `show_overseas_models`/`phone_register_enabled`/`password_login_enabled`）
- `setting_parse.go` 丢失全部 fork 设置解析（UI 主题、货币/汇率、手机号注册、密码登录开关、
  短信三家凭证）与品牌默认值（`site_name`/`site_subtitle` 被上游 `Sub2API` 覆盖）
- `imagesHandler` 漏了 lingjing 生图分流（功能 12）
- `rawChatCompletionsURL` 补 generic 端点分支（功能 25）
- `openai_gateway_messages.go` / `openai_ws_http_bridge.go` 的 grok 请求构建分支
- `GroupsView.vue` 的 `formatUsd` 改为委托 `formatUSD`（保住人民币模式，功能 5）
- `AppHeader.vue` 的 `formatHeaderMoney` 补人民币口径

### 迁移
上游 `159-172` 与 fork `159/160/161/162` 数字前缀重复但**文件名不同**；迁移运行器以 filename 为主键
且按文件名排序，上游自身也存在同号文件，故**无需重编号**。恢复了被误删的 `157_user_platform_quotas_add_grok.sql`
（不恢复会导致自助注册写 grok 默认配额时违反 CHECK → 事务 abort）与 `158_enable_grok_media_generation_groups.sql`。

### i18n 结构迁移
上游把单体 `locales/{zh,en}.ts` 拆成模块目录。fork 的 276/278 个自定义键提取到
`locales/{zh,en}/fork.ts`，由新增的 `locales/forkMerge.ts` 深合并覆盖上游模块，
后续上游同步不再与语言包冲突。

### 验证
- `go build ./...` / `go vet -tags=unit ./internal/...` 零错误
- 后端单测：service / repository / server / handler(+admin,dto,quotaview) 全部 `ok`
- 前端：`typecheck` / `lint:check` 通过，`vitest` 148 文件 945 用例全绿
- fork 守护：`SetEndpointRepository`=4、`SetModelRoutingService`=1、lingjing 视频路由、
  协议分流、`ProtocolBucketEnabled`、`GenericRuntimeEnabled`、`bill_request_id` 全链路、
  `sync-maas`、双桶看板、Playground 路由均在；workflows 全部仅 `workflow_dispatch:`
- 逆向门禁：`PlatformAntigravity` / `AccountTypeOAuth` / `AccountTypeSetupToken` / `IsOAuth()` /
  `CodexModels` 在非测试代码中均为 0

> **待办**：E2E（`./script/e2e-test.sh`）需真实上游凭证，尚未在本次合并中执行；
> grok 模型定价需运营写入 `model_pricings` 后方可计费。

## [未发布] - 2026-07-09 — 二轮排查：handler 层 generic 回退残留（功能 25）

第一轮把 generic 模型口径在 service 层收敛后，handler 层还剩三处 generic 回退残留，本次补齐：

- **`/v1/models` 默认回退对 generic 返回 claude 列表**（`gateway_handler.go` `Models()`）：generic 分组无可路由模型时（如标准模式下 supported_models 全未定价），回退分支落进最后的 else 返回 `claude.DefaultModels`——整页模型全调不通，纯误导。修复：`PlatformGeneric` 返回空列表。
- **raw 口径丢弃 `openEndpoint` 标志**（`gateway_service.go` `GetAvailableModels` simple 路径）：`ids, _ :=` 忽略「空白名单 endpoint = 支持全部」语义，这类账号在 `/v1/models` 一个模型都列不出来（再叠加上一条回退 claude 列表）。修复：openEndpoint 时用已启用 catalog 兜底（与 `routableFromAccounts` 同语义；catalog 不可用时维持原状）。
- **`defaultModelIDsForPlatform` 对 generic 落 claude 默认**（`gateway_handler.go`，自定义模型列表分支的 fallback 源）：与 admin 侧 `defaultModelsListCandidateIDs` 刚改的「generic 无内置默认」口径相反。修复：`PlatformGeneric` 返回 nil（实际输出行为不变——generic 勾选模型与 claude 默认交集本就为空）。

新增测试：`TestGetAvailableModels_SimpleModeOpenEndpointFallsBackToEnabledCatalog`（service）、`TestGatewayModels_GenericGroupFallsBackToEmptyList`（handler）。

门禁：后端 build + `-tags=unit`（service/handler/server）全通过。

---

## [未发布] - 2026-07-09 — 遗漏排查补漏（功能 25 · generic 模型来源一致性）

对 generic 模型链做全面排查，补两处遗漏：

- **分组自定义模型列表候选漏 generic**（`admin_service.go` `GetGroupModelsListCandidates`，`GET /groups/:id/models-list-candidates`）：此前只读 `acc.GetModelMapping()`、且对 generic 分组错误 seed claude 默认模型。修复：走唯一口径 `genericEndpointModelIDs` 补上 endpoint `supported_models`；`defaultModelsListCandidateIDs` 对 `PlatformGeneric` 返回 nil（无内置默认）。为此 `NewAdminService` 新增 `endpointRepo` 构造参数（wire + api_contract_test 同步）。
- **新组件 `GenericEndpointModelsField.vue` 无测试**：补 `GenericEndpointModelsField.spec.ts`（拉取填充清单 / 勾选 emit / 全选清空 / 手动输入拆分去重 / base_url 空校验，5 用例），满足前端 80% 覆盖率门槛。

排查确认**无遗漏**的路径：user `/api/v1/models`（`usage_handler.ListModels` 走 `ModelRoutingService.RoutableModelInfos`，已统一口径）、gemini `/v1beta/models`（直接代理上游）、`toUserSupportedModels`（渠道定价，非本链）。

门禁：后端 build+vet+`-tags=unit`（service/server/handler）、前端 typecheck+lint+组件 vitest、fork 守护全通过。

---

## [未发布] - 2026-07-09 — 修复含 generic 账号分组的 /v1/models 漏算（功能 25）

- **现象**：模型体验广场选含通用渠道账号的分组 key 时，可选模型不是该分组能路由的列表（回退到默认/自定义列表）。
- **根因**：网关 `/v1/models` 走 `GatewayService.GetAvailableModels`，它只从 `acc.GetModelMapping()`（credentials.model_mapping）收集模型；generic 账号的模型在 endpoint `supported_models` 上、不在 model_mapping，被完全漏算 → 无账号有 mapping 时返回 nil → handler 回退默认列表。这是独立于 `ModelRoutingService`（广场/后台已统一）的旧实现残留漂移。
- **修复**：`GetAvailableModels` 收集环节对 generic 账号补上 endpoint `supported_models`（复用 `s.endpointRepo`），与 `routableFromAccounts` 同口径；账号级别名映射（model_mapping）继续并入。
- **防漂移收敛（两层）**：
  1. **派生层**：generic 账号「暴露哪些模型」的逻辑此前在 `routableFromAccounts`、`GetAvailableModels`、`genericEndpointSupportsModel` 三处各写一份（本 bug 正是其中一处漏写）。抽出**唯一口径** `genericEndpointModelIDs(ctx, repo, account) → (ids, openEndpoint)`，三处共用。
  2. **口径层（模式感知）**：`GetAvailableModels` 按运行模式分口径——**标准模式**委托 `ModelRoutingService.routableFromAccounts`，与模型广场**收敛**（含 catalog 交集，只列已启用=真能调的模型）；**simple 模式**用 raw 口径（计费关闭，未定价 supported_models 也可调，便于定价前在 Playground 试模型）。依据：标准模式下计费对未定价模型 fail-closed（`billing_service.go` `ErrModelUnpriced`），列出未定价模型反而误导。为此给 `GatewayService` 注入 `ModelRoutingService`（`SetModelRoutingService` setter + wire）。
- **验证**：新增 `TestGetAvailableModels_GenericIncludesEndpointSupportedModels`（raw 口径）、`TestGetAvailableModels_ModeAwareCatalogIntersection`（标准→catalog 交集只留已启用 / simple→raw 全出）；端到端标准模式分组（账号全启用）`/v1/models` 行为不变（79→79），未定价模型在标准模式被滤（单测覆盖）。

---

## [未发布] - 2026-07-08 — generic 端点：模型勾选子集 + 别名映射（功能 25 增强）

补齐通用渠道「拉回模型列表后没得选、也没有别名映射」的缺口。

### ① 拉取后可勾选子集（前端）
- 新增共享组件 `GenericEndpointModelsField.vue`：端点 `supported_models` 由「逗号分隔 textarea」改为**带搜索 + 全选/清空的复选清单**（拉取结果 ∪ 已选），保留折叠的手动输入兜底。`CreateAccountModal`/`EditAccountModal` 均接入，旧的 `fetchGenericEndpointModels`/`parseGenericSupportedModels` 孤儿代码已清。
- 「拉取模型」不再自动全并入列表，改为填充可勾选清单，由运营 curate 暴露子集。

### ② 模型别名映射（复用账号级 `model_mapping`，不改 schema）
- **路由**：`model_routing_service.go` 的 generic 分支折入 `model_mapping`，别名（目标命中已启用 catalog）以别名 ID 进入可路由集、上广场，与 `supported_models` 并存。
- **eligibility 修复**：generic 账号一旦配置 `model_mapping`，`Account.IsModelSupported` 会转为「仅认映射内模型」，从而误挡其余 `supported_models` 的直连请求。新增纯增量 helper `genericEndpointSupportsModel`（仅 generic、OR 在 `IsModelSupported` 之后），在 3 个准入点（`openai_account_scheduler`、`gateway_service.isModelSupportedByAccountWithContext`、`gemini_messages_compat_service`）补回 `supported_models` 直连放行。openai 网关侧 `isOpenAIAccountEligibleForRequest` 因 generic 早返回不受影响，无需改。网关转发时 `account.GetMappedModel` 已把别名改回上游名。
- **前端**：`CreateAccountModal`/`EditAccountModal` 的 generic 段新增别名映射编辑器（`别名 → 上游模型` 行编辑），保存写入 `credentials.model_mapping`、加载回填。
- **测试**：新增 `TestRoutableModelInfos_GenericFoldsSupportedModelsAndMapping`、`TestGenericEndpointSupportsModel_EligibilityFallback`（覆盖路由折入 + eligibility 单调放宽 + 非 generic 不受影响）。

门禁：后端 build+vet+`-tags=unit`（service+handler）通过、fork 12 守护 ALL PASSED；前端 typecheck+lint+账号模态/i18n vitest 93 通过。

---

## [未发布] - 2026-07-08 — 模型折扣：Provider 显式筛选时展示全部同步模型（功能 26）

- **问题**：折扣页用「从上游同步」拉进新 provider（如 NVIDIA）的模型后，这些 `unpriced`/未启用记录已落库，但折扣列表默认按「广场可路由」口径过滤，未被账号路由的新模型永远不显示 → 无法补价+启用（与账号白名单选择器同一鸡生蛋）。
- **修复**：`model_pricing_handler.go` List 在 `provider` 显式设置时跳过广场 `VisibleOnly` 过滤，展示该 provider 下全部同步模型（含未定价）。**默认视图（无 provider 筛选）仍保持广场口径不变**。前端无改动（Provider 下拉本就传 `provider` 参数）。
- **验证**：`provider=NVIDIA` 返回 121（含 unpriced）；无 provider 默认视图 total 不受影响；handler 单测通过。

---

## [未发布] - 2026-07-08 — 修复 generic 渠道 endpointRepo 未装配（功能 25 回归）

- **根因**：0.1.146 同步时 `go generate ./cmd/server` 重生成 `wire_gen.go`，把 4 处手动 setter 注入 `SetEndpointRepository(endpointRepository)` 全部丢失（wire 只生成构造器注入，setter 是 fork 手改点）。导致 `gatewayService`/`openAIGatewayService`/`geminiMessagesCompatService`/`accountTestService` 的 `endpointRepo` 恒为 nil。
- **现象**：测试通用渠道账号报 `Endpoint repository is not configured`（`account_test_service.go:212`）；generic 运行时转发同样失效。
- **修复**：按历史写法（cf1702b08）在 `wire_gen.go` 无条件补回 4 处 `SetEndpointRepository(endpointRepository)`。
- **验证**：generic 账号测试 SSE 已跑通端点匹配 + 上游探测（返回上游 HTTP 状态而非装配错误）；后端 build+vet 通过、fork 12 守护 ALL PASSED。
- **合并注意**：每次上游同步跑完 `go generate ./cmd/server` 后，必须 `grep -c '\.SetEndpointRepository(' cmd/server/wire_gen.go` 确认为 **4**。已记入 [`自定义开发功能列表.md`](自定义开发功能列表.md) 功能 25 合并注意。

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
