# Changelog (zhiguofan)

本文件记录 `zhiguofan` fork 分支相对上游 `Wei-Shaw/sub2api` 的同步与自定义改动。
版本号规则：上游 `0.x.y` → fork `1.x.y`（主号固定 1，次/修订跟随上游）。
逆向清理口径见 [`自定义开发功能列表.md`](自定义开发功能列表.md) 功能 35。

---

## 同步 - 2026-07-22 — 同步上游 0.1.162（fork 1.1.162）

三分支按序完成：`main` fast-forward 至 0.1.162 已推送；`zhiguofan` 合并共解决 **100 个冲突**（27 个 modify/delete 全部保持 fork 删除 + 72 个内容冲突手工归并 + logo.png 接受上游删除）；`feature/maas-refactor` 与 zhiguofan 已对齐（零差异）。

**采纳的上游主要变更**：客户端 IP 请求头可配置化（`trusted_proxies`/`forwarded_client_ip_headers`，config +346 行 + 设置链 + 安全设置 UI/测试）；step-up 2FA 开关化（安全开关默认关）；ops 入口拒绝日志子系统（迁移 183）；auth 缓存失效 outbox（迁移 184）；OpenAI reasoning effort policy（迁移 185，group schema 变更已 `go generate ./ent`）；优雅关停超时不再跳过 Cleanup；调度器修复（LastUsedAt 缓存隔离、配额元数据保留、模型级临时冷却、排除原因统计 `filterStats`）；计费修复（failover 同步缓存计费、hosted image token 并入 /responses、同账号重试防重复计费）；Anthropic 紧凑 SSE 兼容与 message_start null stop_reason；grok 官方 API 修复（count_tokens 本地 tiktoken 估算——引入 `tiktoken-go/tokenizer` 依赖、media 空结果按 failover）；`openai_images_responses.go` 流式管线重写（取上游版）；OpenAI apikey 账号上游计费倍率探测（创建弹窗自动 probe + 批量编辑入口）；账号批量编辑新增 codex_cli_only 系列；订阅到期时间显示到分钟；axios 1.18.1 / golang.org/x/text v0.39.0 安全升级；移动端/暗色/i18n 一批修复（fork 键经 `fork.ts` 深合并无冲突）。

**逆向订阅裁剪（功能 35 口径）**：27 个 fork 已删、上游又改的文件（antigravity/grok OAuth/codex import/quota 链）保持删除；上游新增文件现场裁剪——`ingress_reject.go` 仅保留通用 host/path 分类（合法保留点同 servertiming）、`gateway_model_availability.go` 去 antigravity 混排、`openai_gateway_count_tokens.go` 去 OAuth input_tokens 回退分支、`openai_alpha_search.go` 丢弃 chatgpt.com responses-web-search 回退簇（依赖已删 OAuth 符号且无调用点）、`openai_gateway_chat_completions.go` 拒绝 codex OAuth transform 块、`SettingsView.vue` 删上游 Codex 加固卡片（配套 script 未随行，后端 codex_cli_only 仍可 API 配置）、`EditAccountModal.vue` 拒 Grok OAuth UI、`types/index.ts` 拒 grok/antigravity 配额字段与 `require_oauth_only`/`require_privacy_set`；`golangci-lint unused` 全仓扫描删约 60 个孤儿符号（`openai_images_responses.go` 的 OAuth 图片转发簇 23 个、`gateway_claude_body.go` 伪装路径 session seed 及其自闭环测试、grok quota probe 孤儿等），`setting_parse.go` 顺带清空 if 分支（SA9003）。

**存量回归修复（本次合并顺带发现，均非上游引入）**：① `wire_gen.go` 的 5 处手改 setter（`SetEndpointRepository`×4 + `SetModelRoutingService`×1）在 2026-07-15 `f0ad756f4`（team-collaboration 合入重新 generate）时即已静默丢失——generic 渠道转发的 endpointRepo 自那时起为 nil，本次恢复并复核 4+1 达标；② 管理员「测试连接」的 `PlatformGrok` 分流与 `testGrokAccountConnection` 在 zhiguofan 上已丢失（grok 账号测试落 claude 兜底），按上游恢复分流、函数（quota probe 改内联 payload）及两个测试。

**其它适配**：fork 两个调度开关（`dual_bucket_enabled`/`protocol_bucket_enabled`）注册 viper 默认值（修上游新 env 可达性守卫测试）；`github_release_service.go` UA 恢复 `TokenPanel-Updater`；`docs/PAYMENT*.md` 取上游新费率/域名事实并重套 fork 品牌；迁移 **183 撞号**（上游 `183_ops_ingress_reject_aggregates` 与 fork `183_add_balance_snapshots`）按功能 43 先例共存、字典序均执行；`handler/wire.go` 的 `ProvideOpenAIGatewayHandler` 去 `GrokQuotaService` 参数（类型已删，`grokMediaEligibilityProber` 保持 nil 短路）；`api_contract_test.go` 契约补 `max_reasoning_effort`/`forwarded_client_ip_headers` 字段；上游改 rollback 请求超时/支付说明链接后对应 fork 测试同步适配（3 个 Agent Identity OAuth 向导测试删除——fork 无 OAuth 授权流 UI）。

**验证补充（集成层）**：`go test -tags=integration ./...` 全绿——首跑暴露两处测试侧问题并修复：① `TestClearRateLimitIfObservedProtectsRearmed429Generation` 上游新增的「OAuth 账号被管理员改回 APIKey 型时 stale recovery 不跨类型清除」尾段依赖已删的 `AccountTypeOAuth` 型语义，按 fork 口径裁掉该段（fork 实现无 `TypeEQ(OAuth)` 谓词属正确行为）；② `TestListWithFilters/filter_by_type` 的 fixture 系 zhiguofan 存量坏例（两个 apikey 账号却期望过滤后剩 1，集成套件久未跑未暴露），t2 改 `AccountTypeUpstream` 恢复过滤区分度。

**验证**：后端 `go build ./...`、`go vet -tags=unit`、`golangci-lint run`（0 issues）、全量 unit 测试全绿；前端 `pnpm typecheck`、`lint:check`、195 文件 / 1281 测试全绿；守护 grep 全过（lingjing 路由组、`getGroupInboundProtocol`、`ProtocolBucketEnabled`、`ApplyUpstreamCostSnapshot`、username 必填、sync-maas、statement/export）；逆向门禁核心关键词非测试代码零命中；workflows 全部仅 `workflow_dispatch:`；VERSION=1.1.162。

高风险复核：风险表 🔴/🟡 登记文件已逐个 diff——`setting_update.go` fork 字段块完整（上游仅插入 IP headers 归一化与 step-up 写入）、`config.go` 两个 fork 开关健在并补注册默认值、`gateway.go` 协议分流/lingjing 路由完整（count_tokens 分流并入上游 `countTokensHandler` 并保留 fork 的 openai 入站 404 门）、`wire_gen.go` 注入链逐行核对（新增 outbox/ingress/imageStorage 链，OAuth handler 链拒绝）、`billing_service.go`/`pricing_service.go`/`usage_log.go`/`usage_log_repo.go`/`admin_compliance.go` 本轮上游未触及、`BulkEditAccountModal.vue` `isMixedPlatform` 防护完整、`SubscriptionPlanCard.vue` 货币符号取上游但删 antigravity model scope 块。

---

## 测试 - 2026-07-22 — e2e 套件适配第三方渠道行为变化（推送门禁解锁）

推送前跑 `./script/e2e-test.sh` 发现三处渠道/上游行为变化导致的稳定失败（对照复跑确认非本批改动回归，转发/计费/限流等其余用例全绿）：① Claude 非流式——渠道强制注入 thinking 块且 `max_tokens=32` 被思维链耗尽，断言改为遍历 content 找 text 块 + 预算提至 1024；② OpenAI——渠道对 gpt-5.5 注入 5k token 系统提示后随机出现「reasoning_content 有内容、正文为空」（同一请求时过时不过），`gwOpenAIChat` 加 `reasoning_effort: low`（同 gwGemini 关 thinking 先例）+ 用例改 3 次重试取文本、连续全空且结构合法则按渠道行为 skip；③ Kiro vision 官方组子用例——上游 image 处理 90s 超时/502，改为上游超时/5xx 时 skip（reroute 机制由「兜底空组」强证明子用例独立验证，不受影响）。改后全量 e2e 顶层 PASS=21、0 FAIL。

---

## 修复 - 2026-07-21 — 清理上游同步带回的逆向订阅代码残留（antigravity），补测试覆盖率缺口

两条独立工作同批完成：

**测试覆盖率缺口补齐**：后端 billing/repository/handler/service 层（视频计费公式、model_pricing 定价 repo、lingjing 27 条模型映射表、gateway session seed/cache-control 裁剪、CSP 白名单解析、汇率/合规门控、手机短信、月结服务）与前端登录注册页/设置页短信联动/对账导出/MaaS 同步表单等 0% 覆盖点，新增约 25 个测试文件、200+ 用例，纯新增测试无生产逻辑变更。`codex review` 复审发现并修复一处真实数据竞争（`balance_snapshot_runonce_test.go` 轮询计数器改 `atomic.Int64`）。

**逆向订阅代码残留清理（三轮）**——根因均为功能 35 删除 Claude Code/Codex/Gemini CLI/Antigravity 全部 OAuth 订阅逆向后，一次上游同步局部带回/遗留的死代码：

- **第一轮（antigravity 专属残留）**：`credentialsBuilder.ts` 的 `applyAntigravityProjectID`/`ANTIGRAVITY_PROJECT_ID_CREDENTIAL_KEY`；`accounts.ts` 的 `getAntigravityDefaultModelMapping`（对应后端路由已不存在）；`setting_service_update_test.go` 从未实例化的孤儿桩 `settingAntigravityUARepoStub`；i18n `{zh,en}/admin/{accounts,overview,settings}.ts`、`dashboard.ts`、`landing.ts` 里几十条 Antigravity OAuth 向导/API Key 连接/GCP Project ID/UA 版本文案（渲染路径已随功能 35 删除）。
- **第二轮（同根因 OAuth 死代码，不含 antigravity 字样）**：`admin/accounts.ts` 整个 `oauth: {...}` 授权向导对象（Claude 根级 + openai/grok/gemini 嵌套子块，约 220 行/语言，零调用方）；"Re-Auth Modal" 整段标签组（`claudeCodeAccount`/`openaiAccount`/`geminiAccount`/`grokAccount`/`reAuthorizeAccount`/`inputMethod`/`reAuthorizedSuccess`）；顶层 `oauthType`/`setupToken` 与 `accounts.platforms.*`/`accounts.types.*`（徽标改用 `PlatformTypeBadge.vue` 硬编码 switch 后失去消费方，仅 `types.responsesApi` 仍在用予以保留）；`GroupsView.*.spec.ts` fixture 里的 `require_oauth_only`/`require_privacy_set`（migration 161 已删）/`mcp_xml_inject` 孤儿字段。
- **第三轮（`golangci-lint --enable-only unused` 全仓扫描补漏，人工 grep 抓不到）**：`gateway_upstream_request.go` 的 `applyClaudeCodeMimicHeaders` + `applyClaudeOAuthHeaderDefaults` + `mergeAnthropicBetaDropping`（连带清理失效的 `google/uuid` 导入）；`openai_gateway_count_tokens.go` 整条 OAuth 本地 tiktoken 估算兜底链（9 函数 + 3 常量共 264 行，`go mod tidy` 连带裁掉 `tiktoken-go/tokenizer` + `dlclark/regexp2/v2` 两个不再被引用的依赖）；`openai_oauth_passthrough_test.go` 的孤儿桩 `passthroughErrReadCloser`（该文件测试早已改名 `*_APIKeyPassthrough_*` 仍是活的，未误删）。

均逐条核实（git blame 溯源、grep 动态 key 拼接、读消费组件源码、golangci-lint unused 交叉验证）后删除。**合法保留点**（核实仍在用）：`servertiming` 通用 host 分类、`admin_account.go` 复制账号遗留字段丢弃名单、`admin.groups.platforms.antigravity`/`accounts.upstream.baseUrlHint` 兼容存量数据的 key。防复发：`script/pre_push_check.sh` 新增**检查 5**——扫描推送范围新增行命中 `PlatformAntigravity`/`AccountTypeOAuth`/`AccountTypeSetupToken`/裸词 `antigravity` 时要求交互确认。

高风险复核：本次改动的 fork 登记文件仅涉及测试新增（无生产逻辑变更）；i18n 大范围删除后 typecheck/lint:check/全量 vitest（186 文件/1235 用例）重跑无回归（含 zh/en key 对齐的 `localesMessageCompile.spec.ts`）；检查 5 已用合成 diff 验证匹配/排除逻辑；后端 build/vet/全量 unit 通过，`golangci-lint unused` 复扫 oauth/antigravity 清零；go.mod/go.sum 仅移除无引用间接依赖。

**codex review 复核发现并修复 2 处**：① `NormalizePhone`（`auth_phone.go`）只查长度与首字符、不查其余字符是否为数字，`"1380000000a"` 会被误判合法——已改 `^1\d{10}$` 正则校验并把对应用例改回断言拒绝；② 检查 5 的 grep 少 `-i`，全大写标识符（如 `ANTIGRAVITY_DEFAULT_URL`）会绕过门禁——已改 `grep -iE` 真正大小写不敏感。

---

## 修复 - 2026-07-19 — fork 设置空值防冲掉全面收紧（currency_mode 同类风险一次修完）

根因：`/admin/settings` 全量 PUT 下 handler 的 `*string` nil-preserve 只防字段缺失、挡不住显式空值（来源：部署窗口竞态或旧缓存前端 bundle）。逐字段审计三层防护后，同类可被空值冲掉的还有 `cny_rate`（0 值）、`sms_provider`、三家 SMS 全部非 secret 配置（共 11 字段）。修复：`buildSystemSettingsUpdates` 引入 `setIfNonEmpty`，所有枚举/凭证类 fork 字符串字段统一「空串=未设置=保留 DB 原值」（cny_rate 非正数同理），在 service 写库层唯一咽喉生效。语义变化：这些字段不再支持清空为空串（无业务意义）。回归测试重写为全集守护 `TestSettingService_UpdateSettings_EmptyForkFieldsDoNotWipe` + 非空落库反向用例。

高风险复核：`setting_update.go` 仅 fork 字段写入块改为 setIfNonEmpty 语义，bool 字段与互斥逻辑、上游字段写入均未动。

---

## 修复 - 2026-07-19 — 定时测试孤儿计划自愈（账号软删后不再每分钟 ERROR 刷屏）

账号软删（SoftDeleteMixin）使 `scheduled_test_plans.account_id` 的 ON DELETE CASCADE 永不触发，孤儿计划每分钟刷 "Account test error: Account not found" ERROR+堆栈。修复：`RunTestBackground` 先做账号存在性检查抛 `ErrAccountNotFound`；`ScheduledTestRunnerService.runOnePlan` 捕获后按 CASCADE 本意删除孤儿计划自愈，只留一条 removed 日志。新增 `scheduled_test_runner_orphan_test.go` 两用例。

高风险复核：`account_test_service.go` 仅入口新增存在性预检查；`scheduled_test_runner_service.go` 仅错误分支新增清理逻辑，正常测试路径未动。

---

## 修复 - 2026-07-19 — 货币模式被全量 PUT 空串冲掉 + 邀请成员不再硬依赖 frontend_url

① 客户端带 `currency_mode: ""` 的全量 PUT 会穿透 nil-preserve 把 DB 已配置模式冲成空串（本地 curl 复现）；系统无合法路径主动清空货币模式，`buildSystemSettingsUpdates` 现对空串跳过写入，回归测试 `TestSettingService_UpdateSettings_EmptyCurrencyModeDoesNotWipe`。② 邀请成员在 frontend_url 未配置时 500——`TeamHandler` 新增 `resolveFrontendBaseURL`：设置值优先，未配置回退请求来源（尊重 X-Forwarded-Proto/Host），`team_handler_frontend_url_test.go` 两用例。

高风险复核：`setting_update.go` 仅 currency_mode 改非空才写；`team_handler.go` 仅替换两处取值为新 helper，其余邀请逻辑未动。

---

## 新增 - 2026-07-18 — 团队邀请站内接受入口（已注册用户不再依赖邀请邮件）

SMTP 未配置时邀请邮件静默不发，已注册用户无法收到邀请。新增站内通道：`TeamInvitationRepository.ListPendingByInvitedEmail`、`TeamService.ListReceivedInvitations`（按当前登录邮箱返回 pending 未过期邀请，token 仅返回给邮箱匹配本人）、路由 `GET /api/v1/team/invitations/received`；接受复用既有 token 接口。前端 `TeamMembersView.vue` 顶部「我收到的邀请」卡片（有数据才显示）、`api/team.ts` 新增方法、fork.ts 新增 `received*` 文案。测试：service 2 用例 + 视图 1 用例，unit/typecheck/lint 全绿。

高风险复核：`team_service.go`/`team_handler.go`/`team_port.go`/`team_invitation_repo.go`/`routes/user.go`/`TeamMembersView.vue`/`api/team.ts`/`fork.ts` 均为纯追加，既有邀请/接受/成员管理逻辑未改动。

---

## 新增 - 2026-07-18 — 月度对账前端页面 + 管理端导出入口 + e2e（功能 45 PR3/3）

用户端 `StatementView.vue`（`/statement` 路由 + 侧边栏项，Simple 模式隐藏；月份下拉/汇总卡片/明细六种行/未封账徽标/恒等式差额脚注/Excel blob 导出，时区取浏览器 Intl）；管理端 `UsersView.vue`「更多」菜单「对账导出」（BaseDialog 选月默认上月，actions-count 7→8，`exportUserStatement`）。i18n：fork.ts `nav.statement` + `statement.*`，overview.ts `exportStatement*`。测试：`StatementView.spec.ts` 6 用例 + i18n 守护全绿；后端 `TestE2EFull_MonthlyStatement` 全链路断言（期初期末锚点/导出 MIME/-partial/真实消费后 utilisation 联动）。

高风险复核：`UsersView.vue` 仅新增菜单项/弹窗/handler（actions-count 7→8），既有操作与数据流未动；`router/index.ts`/`AppSidebar.vue`/`fork.ts`/`overview.ts` 均为纯追加。

---

## 新增 - 2026-07-18 — 月度对账后端 API + Excel 导出（功能 45 PR2/3）

`StatementService` 拼装六种行（Qty/等效单价/折扣率反算、滚动余额、恒等式 gap），已封账月快照优先、当月与缺失月实时反推（`source: snapshot|computed`）。新增 `pkg/statement` 共享 DTO 与 `pkg/xlsxreport`（首次引入 excelize v2.11）逐格复刻 Eonreach 模板（活公式 `E*F`/`G*(1-H)`/`C+I`/期末 SUM），未封账月文件名带 `-partial`。路由：用户端 `GET /api/v1/usage/statement{,/months,/export}`（静态段先于 `/:id`）；管理端 `GET /api/v1/admin/users/:id/statement/export`（`SetStatementService` setter 注入，导出方法在独立 fork 文件 `user_statement_handler.go`）。单测（service/xlsxreport/handler）+ 全量 unit 回归通过。

高风险复核：`wire_gen.go` 本次新增 statementService/statementHandler 构造与 `ProvideAdminHandlers`/`ProvideHandlers` 实参（`go generate` 生成）；`routes/user.go`/`routes/admin.go` 仅追加 statement 路由行。

---

## 新增 - 2026-07-18 — 月度对账（Vendor Report）数据层（功能 45 PR1/3）

方案见 `claudedocs/月度对账功能设计方案.md`。新增 `balance_snapshots` 月结快照表（ent schema + migration 183，UNIQUE(user_id,period)、无 users 外键台账语义）；`StatementRepository` 五表 raw SQL 聚合（充值/退款/赠送/兑换/企业划转/按日消耗 + 反推净变动），**Credit 统计排除充值链路兑换码防双算**（充值入账走「订单→兑换码→Redeem」链路）；`BalanceSnapshotService` 月结后台任务（每月 1 日 00:30 后补算上月缺口，leader lock，`RecomputeForUserMonth` 幂等 upsert），期末以 `users.balance` 为锚反推。零改动既有接口（零 stub 冲击）。单测（月界/反推/幂等/恒等式）+ 集成测试（五表口径/防双算/双视角/时区归日）全绿。

高风险复核：`wire_gen.go` 仅新增 statement/balance_snapshot repository、BalanceSnapshotService 构造与 provideCleanup 实参/形参/stop 条目，其余注入链未动。

---

## 修复 - 2026-07-17 — 平台费用悬浮层被表格 overflow 裁切

`PlatformUsageBreakdown.vue` 在 `DataTable` 单元格内悬浮时被祖先 `.table-wrapper`（overflow:auto）裁切。改为复用全站既有做法（同 `UsageTable.vue`）：弹层 `Teleport` 到 `body` + `position:fixed` + `getBoundingClientRect` 定位；`align` 语义保留。同修团队成员报表与后台用户管理两个引用点。

高风险复核：仅改 tooltip 定位机制（绝对定位→Teleport+fixed），props 契约、「其他」行聚合、文案均未变；两个引用点无需改动。

---

## 修复 - 2026-07-17 — 团队协作页移动端适配（迁移共享 DataTable）

`TeamMembersView.vue` 三个裸 `<table>` 在窄屏横向撑破页面，改为复用全站共享 `DataTable`（内置桌面表↔移动卡片切换）：新增 `memberColumns`/`transferColumns`/`reportColumns` 列定义 + `#cell-*` 插槽承接原单元格内容；Tab 栏加 `overflow-x-auto`。移动端 390px 实测三 Tab 均正常卡片化。

高风险复核：仅视图层重构（表格→DataTable + 列定义/插槽），业务逻辑/接口/数据流未改动；typecheck + lint + 相关 vitest（4+5+3）全绿。

---

## [1.1.160] - 2026-07-17 — 同步上游 0.1.160（OpenAI 兼容 prompt 审计）

同步 25 提交（0.1.158→0.1.160）。主体为新功能 **OpenAI 兼容 prompt 审计**（`internal/securityaudit/` + `frontend/src/features/prompt-audit/`，Qwen3Guard 异步复核/同步阻断），附带 grok media 修复、image_gen 显式意图检查（#4476）、backup S3 step-up TOTP。VERSION → 1.1.160。

15 个冲突文件的 fork 适配要点：`cmd/server/wire.go` 保留 fork `SQLDB` + 并入 `PromptAudit`；`handler/wire.go` 的 `ProvideGatewayHandler` 去 antigravity 参、补 fork `bridgeRegistry`，保留 Team/Enterprise 注入；`gateway_handler.go` 保留 `bridgeRegistry` + 并入 `securityAuditCoordinator`、不引入 `antigravityGatewayService`；`openai_gateway_handler.go` 采纳 #4476 `imageIntent`；`service/account.go` 保留 `GrokMediaEligibleExtraKey`、`GrokMediaGenerationEligibility` 按 fork 无-OAuth/billing 世界化简（仅 override）、弃 codex-PAT auth；`securityaudit/prompt_module.go` 补 `PromptAdminService` 显式 `wire.Bind`（供离线重生成）；`wire_gen.go` 用 `go run wire` 权威重生成；改/删冲突保持 fork 删除 `grok_quota_service.go`/`xai/billing.go` 及相关 OAuth/billing 测试；docker-compose 保留 tokenpanel 品牌。

高风险复核：逐个核对 `wire.go`/`wire_gen.go`/`handler/wire.go`/`gateway_handler.go`/`openai_gateway_handler.go`/`service/account.go`/`admin_account.go`/`routes/admin.go` 等登记文件合并结果，fork 注入链（lingjing/provider-pricing/model-pricing/team/enterprise/bridgeRegistry）与逆向清理均未被吞回。

验证：后端 build + 单测全过（含 securityaudit）；前端 typecheck + 58 测试 + lint + `--frozen-lockfile` 全过；浏览器 E2E 实操：提示词审计（DB·Redis ok、Qwen3Guard 策略）、模型折扣（90 条 + MaaS 同步 + lingjing）均正常。

---

## 工具 - 2026-07-17 — pre-push 门禁检查 3 扩到全部登记 fork 文件

检查 3 复核范围从「仅 🔴 高」扩大到风险表登记的**全部 fork 文件（🔴/🟡/🟢 三档）**。同时修复两处：(1) 文档 `{a,b,c}.go` brace 记法原匹配不展开会漏判——改 bash eval 展开（带安全字符集过滤）；(2) 逐文件×逐 pattern 嵌套循环在大同步会超时——改为正则集一次 `grep -Ef`，实测 1s 内。

---

## 工具 - 2026-07-17 — pre-push 门禁新增高风险文件复核 + e2e 强制

`script/pre_push_check.sh` 在原两项（CHANGELOG 必更、功能列表一致性）上新增：**检查 3** 高风险文件逐个复核（打印 diff + 「高风险复核：」书面留痕 + `/dev/tty` y/N 确认，GUI 客户端无 tty 命中即阻塞）；**检查 4** 有实质源码改动时内联跑 `./script/e2e-test.sh`，检查 1-3 失败则跳过（fail-fast）。绕过口径不变（`--no-verify`/`PREPUSH_SKIP=1`）。

---

## 同步上游 - 2026-07-17 — 合并 100 个上游 commit（版本 → 1.1.158）

合并 100 提交（41 个 merge PR），新增迁移 178~182。主要采纳的上游新功能：管理面操作审计日志（#4418，migration 180，append-only + 2FA 清空）；管理员 Step-up 二次验证（#4429，`middleware/step_up.go`/`session_binding.go`/`TotpStepUpDialog.vue`）；异步图片任务 + S3 对象存储（#4406，migration 179，`image_task_*`/`image_storage*`/`s3_client.go`，`usage_logs` 拆 `image_input_tokens`/`image_input_cost`）；图片输入 token 独立单价（#4396，migration 178 渠道级 + 182 目录级）；上游账号费率探测 / Key 账单信息（#4385/#4108/#4387）；分组/渠道一键复制（#4434/#4427，migration 181）；用户批量限额编辑（#4425，`BulkEditUserModal.vue`）；其余 Grok/Codex 兼容修复与零散 bugfix。

高风险复核：同步范围（`6be4b0cd6..HEAD`）命中的 **9 个 🔴 高文件逐个核对全部通过**——`billing_service.go`（无 applyDiscount/fallbackPrices 回归，tier_pricing 未触碰）、`pricing_service.go`（catalog/aliasIdx 在位）、`ent/schema/model_pricing.go`、`wire_gen.go`（手改 setter ×4/×1 在位、注入链完整）、`routes/admin.go`、`routes/gateway.go`、`config.go`（两 flag 在位）、`setting_update.go`（fork 字段块完整 + 回归测试在位）、`router/index.ts`。功能 35 无生产代码重引入（`IsOpenAIOAuth()` 恒 false，grok/openai oauth 文件未重现，仅剩 GroupsView spec 4 处 mock 惰性字段）。扩展核对：命中的 **13 个 🟡 中文件同样逐个通过**（`gateway_handler.go` 4 处删除是 `writeModelsList` 加 Grok 的签名重构、保留 generic 空列表语义；`openai_gateway_service.go` 16 处删除全为 gofmt 重对齐；`scheduler_snapshot_service.go` 平台列表被追加 `PlatformGrok` 但 `PlatformLingjing` 保留）。🟢 低 0 命中；新增 30 个源码文件全部来自 upstream/main（无漏登记）；未删除任何非测试源码文件。即触碰的全部 22 个登记文件无一被静默覆盖。

e2e：`./script/e2e-test.sh` 全套通过。顺带修复既存 e2e 脆弱性：`TestE2EFull_AdminAccountGroupCRUD` 改用 `search` 按唯一名精确过滤（原无过滤分页断言在本地累积分组超单页时漏判）。`TestE2EFull_KiroVisionReroute` 偶发 90s 超时属上游延迟抖动，重跑即过。

---

## 同步上游 - 2026-07-15 — 合并 214 个上游 commit（版本 → 1.1.156）

71 处内容冲突 + 44 处 delete/modify 冲突。按功能 35 口径重新剥离上游重引入的订阅逆向：删除 `grok_credential_failure.go`、`grok_quota_fetcher.go`、`openai_images_oauth_*`、`openai_codex_transform.go` 内 `applyCodexOAuthTransform`、`grok_import_probe.go` 等整块 OAuth 代码，残留调用点（`IsGrokOAuth`/`getRequestCredential`/`ShouldStopOpenAIOAuth429Failover` 等）钝化为恒定安全默认值；`pkg/xai/oauth.go` 裁剪为仅 apikey URL 构建。保留的合法上游新功能：账号复制（`DuplicateAccount`）、`SchedulerCache` P5-5 双桶比较统计、`long_context_billing_applied` 计费字段、`UpstreamFailoverError` 扩展字段。

顺带修复既存缺陷：`account_stats_pricing_test.go` 测试夹具未映射 `LongContextInputThreshold`/`*Multiplier` 导致长上下文倍率断言恒退化。

已知未解决（详见功能列表功能 44 后续跟进）：`TestSchedulerRebuildBatch*` 两个调度器批量查询去重测试失败（mixed/historical 模式查询计数与预期不符），初判生产逻辑问题，未定位根因。

---

## 新功能 - 2026-07-15 — 企业组织与额度分配（Team 协作 v2）

新增功能 44（完整描述见 `自定义开发功能列表.md` 功能 44、方案 `claudedocs/企业账号多管理员共享额度与Key方案.md`）：自助升级企业、按部门邀请、员工自管 Key、真实余额划转（allocated 手动 / shared 自动补给）+ `team_fund_transfers` 台账，三档角色 owner/admin/member。计费模型「真实划转余额」：消费扣员工自己余额，网关热路径/api_keys/usage_logs 零改动。新表 `enterprise_profiles`/`team_departments`/`team_fund_transfers` + `team_members`/`team_invitations` 加列；`TeamFundService` 原子划转（双行 FOR UPDATE 固定锁序）、`TeamAutoTopupService` 后台补给（leader lock）。同批**废弃回退 v1「共享控制面」**（未推送中间态）：删除 `middleware.TeamContext` 及 ~26 处 `GetResourceOwnerID` 接入点、前端 TeamSwitcher/请求头注入——共管同一批 Key 不符合企业管理要求。

---

## 清理 - 2026-07-14 — 删除 credentialsBuilder.ts 的 plan_type 孤儿函数

推翻 07-13 审计「保留待未来恢复」的决定：确认无生产调用即应删除。删 `buildPlanTypeOptions`/`applyPlanType`/`readPlanType`/`planTypeDisplayLabel` + `PlanTypeOption` 接口及对应 20 条单测；功能列表风险表移除相应行。typecheck/lint/vitest 通过。

---

## 文档审计 - 2026-07-13 — 0.1.130~0.1.153 全区间代码-文档一致性深度审计

对 141 个 fork 独有提交做函数级审计，修复 5 处缺口：新增功能 42（`admin_compliance.go` 合规门控禁用，`6170de3f7`，此前零记录）；新增功能 43（渠道级 `image_input_price`，`1a1e4c241` + migration 162，此前零记录）；风险表补 `setting_update.go`/`setting_public.go` 高危行（0.1.147 静默删除 fork 字段块事件，模式第 3 次出现，CLAUDE.md 同步补陷阱）；`model_pricing_handler.go` 行补 `for_whitelist` 说明（`42332c0fc`）；CLAUDE.md 批量改账号陷阱更新为已自动防护（`bd0bf3c44`）。审计聚焦「新功能是否被记录、已声明删除是否属实」，未逐行走查上游第三方 PR。

---

## [1.1.153] - 2026-07-13 — 同步上游 0.1.153（45 提交：Grok 官方 API 增强 + apicompat/性能修复）

**规模**：45 提交、101 文件（+4480/-221）；新迁移仅 `174_add_usage_logs_api_key_latest_ip_index_notx.sql`（与 0.1.152 的 174 重号，字典序常态）。

**采纳（fork 口径裁剪）**：Grok 第三方 base URL（`bc5d6ecb4`）、apikey 上游模型同步（`b0441ca5a`，仅 grok case）、video edits/extensions（`909b96edd`）、`GetGrokMediaBaseURL` 裁剪版；openai-ws 池上限 `min(并发, 8)`（`c8cfc9363`，fork 断言 20→8）；apicompat 三修复（max_tokens→incomplete、content_filter、Read 工具实时流式）；alpha search 前端 bypass（`b0fa2b352`）；静态资源 Cache-Control；IP 查询索引化；调度缓存异常时间修复（`fe184f8c3`）等。`.gitignore` 采纳 deploy/tests 白名单，**拒绝**上游忽略 `CLAUDE.md`/`.claude`。

**不引入（功能 35 口径）**：OpenAI OAuth plan_type 手动覆盖（`c56a64fab`，EditAccountModal planType 整链剥离）；Codex plan-gated 冷却 OAuth 分支（`5aeb03018`）及 3 个测试（`isOpenAICodexPlanGatedModelError` 纯函数保留）；Grok OAuth media 分流（`bb7341673`）与 `GetGrokAccessToken` 等 OAuth 符号及 15 个 OAuth 路径测试；failover Antigravity 测试 6 个、upstream_models antigravity 分支；README 上游 Grok OAuth 文档段。

**验证**：后端 build/vet/全量 unit 全过；前端 typecheck/lint/关键 vitest 全过；workflows dispatch-only；功能 35 门禁 rg 清零；VERSION 1.1.153。

---

## [1.1.152] - 2026-07-13 — 同步上游 0.1.152（40 提交：Grok xAI API key + alpha/search 按次计费）

**规模**：40 提交、120 文件（+6381/-393）。

**采纳（fork 口径裁剪）**：
- **Grok 官方 xAI API key 账号**（`d9e466ad3`）：创建/编辑弹窗 grok 入口（无 OAuth 选择区，直接 `accountCategory='apikey'` + 默认 `https://api.x.ai/v1`）；`DeriveUpstreamEndpoint` 的 grok 并入 OpenAI case；`UseKeyModal` Grok CLI/OpenCode tab（剔除 antigravity tab）。
- **grok 前端平台链补全**（fork 此前无 grok 前端入口，一并补齐）：类型加 `'grok'`、`platformColors.ts` 12 张映射表、`PlatformIcon` svg、`CreateAccountModal`/`GroupsView` 选项、后端 group dto `oneof` 补 `grok`（否则建组 400）、`UseKeyModal` 补回 `grokModels`。
- **Codex alpha/search 按次计费**（`7cbb36f27`）：`/v1/alpha/search` 路由 + `AlphaSearch` handler + `CalculateWebSearchCost`（组单价×倍率，默认 $0.01/次）+ 迁移 174 + GroupsView 配置；service 侧裁剪 OAuth 分支。
- **Grok 429→rate-limit 持久化**（`1dedb2097`）：按 Retry-After/配额窗口 reset 持久化限流，去掉 OAuth-only 守卫；429 测试改 `AccountTypeAPIKey` 保留。
- **Grok prompt cache identity**（`42f3c2283` 裁剪）：租户隔离 `prompt_cache_key` + `X-Grok-Conv-Id` 头路由；Free-tier 工具注入恒 `false`。
- **no-account 错误分类**（`8a22dc734`）：按平台区分 404/503，六处采纳；`QuotaPlatform` 记账字段；compact keepalive、gpt-5.6 对齐等非冲突改动整体采纳。

**继续删除的逆向链**：grok OAuth 整链（`grok_oauth_service`/`grok_quota_service`/`useGrokOAuth.ts` 及各自测试）、`openai_gateway_grok_chat_bridge`（共享常量 `grokChatRawEndpoint` 移入 `openai_gateway_chat_completions_raw.go`）；Codex 逆向不回流（`filterCodexInput` 等保持删除、`CodexModels` 路由不引入）；上游 OAuth 专属测试删除或改 apikey/upstream 类型；`TestGetModelPricing_GrokCatalogFallbacks` 删除（fork 无 fallbackPrices、fail-closed 是政策）；三个 grok 转发测试的工具注入断言改「不注入」。

**回归修复（0.1.151 遗留）**：接回 grok Responses 分流——`openai_gateway_forward.go` 的 `PlatformGrok → forwardGrokResponses` 在 `b181ba0c3` 被误删成死代码，本次恢复并加入口级守护测试 `TestForwardEntryRoutesGrokPlatformToXAIResponses`（反证验证：删分流→请求错落 api.openai.com→测试红）。

**ent 重新生成**：上游 group 新列 `web_search_price_per_call` + account `quota_dimension` 与 fork schema 合并后以 fork 生成码为基线重跑 `go generate ./ent`。

**验证**：E2E 19 过/2 失败——两失败（`ClaudeToolUse` 工具名被渠道改写、`ClaudeThinking` 渠道 400）经 merge 前代码复跑逐字复现，确认为 openclaw 渠道行为变化非本次回归；后端 build/vet/门禁单测全过；前端 typecheck/lint/关键 vitest 全过；fork 守护点全部完好；VERSION 1.1.152。

---

## [1.1.151] - 2026-07-11 — 同步上游 0.1.151（61 提交，无破坏性重构）

### 附带修复：fork 自定义设置持久化回归（0.1.147 合并遗留）

「货币向导每次进后台都弹、保存不生效」根因：上次 0.1.147 合并提交 `7c9e09d29` **静默删除**了 `buildSystemSettingsUpdates`（写入）与 `GetPublicSettings`（公开读取）尾部整段 fork 字段块（merge commit 内删除，`git log -S` 不可见），受影响 `currency_mode`/`cny_rate`/`ui_theme`/`show_overseas_models`/`phone_register_enabled`/`password_login_enabled` 及三家短信配置——只写内存缓存、重启即丢。修复：

- `setting_update.go` 补回 22 个 fork 字段写入 + phone/email 互斥（从 `7c9e09d29^` 恢复）；`setting_public.go` 补回 6 个公开字段；新增回归守护 `setting_fork_fields_persist_test.go`。
- **次生 bug**：`CurrencySetupModal.vue` 原来只发 `{currency_mode, cny_rate}` 到全量 PUT，而后端 **110 个值类型字段无 nil-check 回落**，部分更新会把站点/SMTP/OAuth 全套设置写空。系统筛查（触发/受害/读取三维度）确认全前端仅 `CurrencySetupModal` 与 `SetupWizardView` 两处部分提交，均改为「先 `getSettings()` 拉全量、仅覆盖目标字段后整体 PUT」；`api/admin/settings.ts` 的 `updateSettings` JSDoc 补强警告防再犯。

### 同步与冲突解决（21 个文本冲突）

上游内容集中在 OpenAI/Codex/apicompat bugfix、compact/SSE 加固（61 提交、123 文件）。要点：计费 SSOT 保留（删上游重引入的 `fallbackPrices`/`initFallbackPricing`/`matchOpenAIModel` 等静态兜底，仅采 `LongContext` 字段与 GPT-5.6 legacy 长上下文判定）；OAuth 逆向链保持删除（`token_refresher`、`ListOAuthRefreshCandidates`、全部 `AccountTypeOAuth` 分支剥离，生产代码残留 0）；保留上游用户级 Fast/Flex 策略、`stripOpenAIImageGenerationToolsFromRawPayload`、grok 被动配额快照、GPT-5.6 别名展示；spark 影子账号不引入（迁移 154 上游有 fork 无，编号重复为字典序常态）；OAuth 专属测试删除、通用测试改 `AccountTypeAPIKey`。

### 验证

全量后端 unit 全过；前端 typecheck/lint/关键 vitest 全过；fork 守护点完好；workflows dispatch-only；`go generate ./ent` 无 diff。

---

## [1.1.147] - 2026-07-10 — 同步上游 0.1.147（147 提交）+ Grok 官方 API 保留 + 逆向链再清理

**规模**：147 提交、410 文件；上游把 fork 重度改造的巨型文件做「纯移动拆分」（`usage_log_repo.go` 4701→212 拆 6 文件、`setting_handler.go` 3957→468 拆 5、`admin_service.go` 4409→642 拆 5，另拆 `gateway_service.go`/`openai_gateway_service.go`），fork 语义须逐函数重新落位。

**合并方法（可复用）**：以 merge-base 单体为 base、fork 单体为 ours、上游拆分文件为 theirs，逐顶层函数三方归并——上游纯移动直接采 fork 版，双方都改用 `git merge-file`。自动归并 64 函数、人工仲裁 3 处（`RecordUsage` 双段并存、两处 SSE `response.failed` 净化取上游）。

**决策：保留 Grok 官方 API、删除 Grok 订阅逆向**——保留 `api.x.ai` 官方链路（`openai_gateway_grok.go` 守卫改 `AccountTypeAPIKey`、`grok_media.go`、`pkg/xai` URL/模型/配额头解析、被动配额快照、WS→HTTP 桥、`isOpenAIAccount()` 纳入 grok、迁移 157/158/170/171/172）；删除 `grok_oauth_service/handler/client`、`grok_token_provider/refresher`、`grok_quota_service`、`pkg/xai/oauth.go`、前端 `useGrokOAuth.ts` 等。计费口径：grok 价格不内置（功能 26/34），未定价 fail-closed，上游 `TestGetModelPricing_Grok45OfficialFallback` 移除。

**逆向链再清理（功能 35）**：antigravity 平台（UA/fallback 设置、`DefaultAntigravityModelMapping`、调度/网关分支）、Claude Code 拟态（`gateway_claude_oauth_body.go` 按符号拆分——非逆向工具迁入新建 `gateway_claude_body.go`，OAuth 拟态整段丢弃）、codex CLI 限制策略（`CodexRestrictionPolicy`/manifest 分支）、spark 影子账号、`ListOAuthRefreshCandidates`。**运行时地雷修复**：上游 `ListCRSAccountIDs` SQL 带 `parent_account_id IS NULL` 谓词而 fork 库无此列（spark 迁移未引入）会直接 SQL 报错，已移除该谓词。

**合并后修复**：① `/v1/messages/count_tokens` 对 OpenAI 分组的桥接被漏接（上游新增桥接文件，但 `routes/gateway.go` 保留了 fork 旧「openai 一律 404」门，两个新文件成零引用死代码）——已接回：openai 平台走桥，grok/generic 仍 404；② `script/e2e-test.sh` 补 `-count=1`（外部 HTTP 服务测试会命中 go test 缓存假绿）。

**采纳的上游修复**：`response.failed` 错误透传（不再硬编码 502）、上下文超限不触发 failover、流式 usage 漏计费修复、鉴权绕过修复、site_name/logo/doc_url XSS sanitize、Go 1.26.5、批量生图、用户 Token 排行、SSE 扫描器封装（openai 侧不再复用 fork 64K buffer 池，可接受收敛；gateway_* 仍复用）。

**fork 语义补回**（合并期间发现）：`PublicSettingsInjectionPayload` 缺 6 个 fork 公开字段；`setting_parse.go` 丢失全部 fork 设置解析与品牌默认值；`imagesHandler` 漏 lingjing 分流（功能 12）；`rawChatCompletionsURL` 补 generic 分支（功能 25）；`openai_gateway_messages.go`/`openai_ws_http_bridge.go` grok 请求构建；`GroupsView.vue` `formatUsd` 委托 `formatUSD`、`AppHeader.vue` 补人民币口径（功能 5）。

**迁移**：上游 159-172 与 fork 159/160/161/162 数字重复但文件名不同，runner 按文件名主键排序无需重编号；恢复被误删的 `157_user_platform_quotas_add_grok.sql`（否则注册写 grok 配额违反 CHECK → 事务 abort）与 `158_enable_grok_media_generation_groups.sql`。

**i18n 结构迁移**：上游拆单体语言包为模块目录，fork 276/278 个自定义键提取到 `locales/{zh,en}/fork.ts` + `forkMerge.ts` 深合并覆盖（功能 41），后续同步不再冲突。

**验证**：build/vet 零错误；后端单测全 ok；前端 typecheck/lint 通过、vitest 148 文件 945 用例全绿；fork 守护（`SetEndpointRepository`=4、`SetModelRoutingService`=1、lingjing 路由、协议分流、两 flag、bill_request_id、sync-maas、双桶、Playground）均在；workflows dispatch-only；逆向门禁非测试代码 0。**待办**：本次未跑真实凭证 e2e；grok 定价需运营写入后方可计费。

---

## [未发布] - 2026-07-09 — 二轮排查：handler 层 generic 回退残留（功能 25）

三处补齐：① `gateway_handler.go::Models()` generic 分组无可路由模型时误回退 `claude.DefaultModels`（整页调不通纯误导）→ 返回空列表；② `gateway_service.go::GetAvailableModels` simple 路径 `ids, _ :=` 丢弃 `openEndpoint` 标志（空白名单=支持全部的账号一个模型都列不出）→ openEndpoint 时用已启用 catalog 兜底；③ `defaultModelIDsForPlatform` 对 generic 落 claude 默认 → 返回 nil（与 admin 侧同口径）。新增两测试；build + unit 全过。

---

## [未发布] - 2026-07-09 — 遗漏排查补漏（功能 25 · generic 模型来源一致性）

① `admin_service.go::GetGroupModelsListCandidates` 漏 generic endpoint `supported_models` 且错误 seed claude 默认 → 改走唯一口径 `genericEndpointModelIDs`，`defaultModelsListCandidateIDs` 对 generic 返回 nil；`NewAdminService` 新增 `endpointRepo` 构造参数（wire + api_contract_test 同步）。② 补 `GenericEndpointModelsField.spec.ts`（5 用例）满足 80% 覆盖率。排查确认其余路径（user `/api/v1/models`、gemini `/v1beta/models`、`toUserSupportedModels`）无遗漏。

---

## [未发布] - 2026-07-09 — 修复含 generic 账号分组的 /v1/models 漏算（功能 25）

**根因**：`GetAvailableModels` 只从 `credentials.model_mapping` 收集模型，generic 账号的模型在 endpoint `supported_models` 上被完全漏算 → 回退默认列表。**修复 + 防漂移收敛两层**：① 派生层——generic「暴露哪些模型」原在三处各写一份，抽出唯一口径 `genericEndpointModelIDs(ctx, repo, account) → (ids, openEndpoint)` 三处共用；② 口径层——`GetAvailableModels` 按运行模式分口径：标准模式委托 `ModelRoutingService.routableFromAccounts`（与广场收敛，只列已启用；依据：标准模式计费对未定价 fail-closed，列出未定价反误导），simple 模式 raw（计费关闭，便于定价前试模型）。为此 `GatewayService` 注入 `SetModelRoutingService` setter + wire。新增两测试；标准模式行为不变（79→79）。

---

## [未发布] - 2026-07-08 — generic 端点：模型勾选子集 + 别名映射（功能 25 增强）

① 新增共享组件 `GenericEndpointModelsField.vue`：端点 `supported_models` 从逗号 textarea 改为带搜索/全选/清空的复选清单（拉取结果 ∪ 已选）+ 折叠手动输入兜底，Create/Edit 两模态接入，旧孤儿代码清理。② 别名映射复用账号级 `model_mapping`（不改 schema）：`model_routing_service.go` generic 分支折入映射（别名进可路由集上广场）；修复 eligibility——配了 mapping 后 `IsModelSupported` 会误挡其余 supported_models 直连，新增纯增量 helper `genericEndpointSupportsModel`（仅 generic、OR 在其后）在 3 个准入点补回放行（openai 侧因 generic 早返回不受影响）；前端两模态加别名映射行编辑器。新增 `TestRoutableModelInfos_GenericFoldsSupportedModelsAndMapping`/`TestGenericEndpointSupportsModel_EligibilityFallback`。门禁全过。

---

## [未发布] - 2026-07-08 — 模型折扣：Provider 显式筛选时展示全部同步模型（功能 26）

新 provider（如 NVIDIA）同步后因未被账号路由，在广场口径下永不可见 → 无法补价+启用（鸡生蛋）。`model_pricing_handler.go::List` 在 `provider` 显式设置时跳过 `VisibleOnly` 过滤（默认视图口径不变，前端无改动）。验证：`provider=NVIDIA` 返回 121 含 unpriced；handler 单测通过。

---

## [未发布] - 2026-07-08 — 修复 generic 渠道 endpointRepo 未装配（功能 25 回归）

**根因**：0.1.146 同步跑 `go generate ./cmd/server` 把 4 处手动 setter `SetEndpointRepository(endpointRepository)` 全部丢失（wire 只生成构造器注入），`endpointRepo` 恒 nil——通用渠道账号测试报 `Endpoint repository is not configured`、generic 转发失效。按历史写法（cf1702b08）补回 4 处。**合并注意**：每次 generate 后必须 `grep -c '\.SetEndpointRepository(' cmd/server/wire_gen.go` 确认为 4——已记入功能列表功能 25 合并注意。

---

## [未发布] - 2026-07-08 — Playground（功能 38）bug 修复

修 3 个 bug：中文输入法回车误发送（`!e.isComposing`）；「停止」无法中止生图（`imageGenerate`/`imageEdit` 接入 `AbortSignal`，中断时移除空气泡）；上传预览/文件顺序竞态（改顺序异步读取成对追加）。可用性改进：「＋ 新对话」按钮、上限/拉取失败 toast、助手回复一键复制；回复仍纯文本 `<pre>` 渲染（防 XSS 取舍）。typecheck/lint 0 错误、playground vitest 25/25。

---

## [1.1.146] - 2026-07-07 — 同步上游 0.1.146

同步 47 提交。门禁：后端 build/vet/unit、前端 typecheck/lint/894 测试、fork 守护 ALL PASSED、E2E 21/21。

- **采纳**：apikey 账号请求头覆写（`account_header_override.go`，保留 `ApplyHeaderOverrides` 装配点、剥离 OAuth-header 分支，前端配置 UI 暂未移植）；入站端点归一化 + responses/compact 区分；Redis SCAN 优化；gpt-5.6 系列新模型；`ProvideAPIKeyService` 新增 `concurrencyService` 参（wire 重装保全 fork 注入链）。
- **逆向回炉**：保持删除 `grok_media.go`/`openai_gateway_count_tokens.go` 等；`billing_service.go`/`pricing_service.go` 拒绝上游重引入 `fallbackPrices`/`pricingData`，取 fork catalog 版；codex 版本门控 403 文案回退 fork 硬编码；测试 fixture 清理 antigravity/grok/OAuth；账号 3 模态与 `GroupsView.vue` 取 fork 版。
- **货币**：保住 1.1.145 的 `OrderTable`/`PaymentQRDialog` 口径；`PaymentStatusPanel.vue` 采纳上游支付重构。
- **测试**：新增 `TestE2EFull_BillRequestIDWriteback`（功能 27 显式断言三种回写行为）。

---

## [1.1.145] - 2026-07-06 — 同步上游 0.1.145 回炉

上游 0.1.145 原始合并处于「未清理·不编译」状态（199 处编译/类型错误），重新套用逆向清理口径恢复全门禁通过。

- **后端**：删除上游复活的纯逆向文件（`account_codex_import`/`antigravity_token_refresher`/`token_refresh_service` 含测试）；`model_rate_limit`/`account`/`ratelimit_service`/`setting_service`/`admin_service` 外科剥离 antigravity/grok/OAuth 死符号（保留 Fable 限流、OpenAI 高级调度器等上游新功能）；`account_usage_service` 删逆向主动抓取保留 Fable 被动路径；`openai_account_scheduler` 采纳加权调度器、剥离 shadow/OAuth 依赖；三个 handler 清理 antigravity/grok/Codex 符号。
- **前端**：`useModelWhitelist.ts` 删 antigravity/grok 预设、恢复被坏合并丢失的 lingjing 白名单；`api/admin/accounts.ts` 删 Codex 逆向 API；`AccountUsageCell.vue`/`EditAccountModal.vue` 恢复 fork 清理版。
- **货币修复**：`OrderTable.vue`/`PaymentQRDialog.vue` 修复共享订单表/支付弹窗口径（paid/pay_amount 按订单货币、credited 恒 USD，修 USD 订单误显 ¥）；`UsageView.spec.ts` CSV 期望对齐。
- **守护**：`script/check_fork12_guards.sh` ForcePlatform 规则预期 3→1。

---

## 附注

- 推送前门禁（5 项检查）、开发命令、架构说明见 [`CLAUDE.md`](CLAUDE.md)；fork 功能与高风险文件见 [`自定义开发功能列表.md`](自定义开发功能列表.md)。
- fork 已移除 OAuth 账号类型（`IsOpenAIOAuth` 等恒 false），上游每次同步都会重新引入 codex/grok/antigravity/oauth，合并后须按功能 35 口径剥离。
- E2E 用持久化 dev_local 库，`e2e-*` 测试数据会累积；分组数超 `page_size=100` 时 `AdminAccountGroupCRUD` 可能误报，清理 `e2e-*` 数据即可。
