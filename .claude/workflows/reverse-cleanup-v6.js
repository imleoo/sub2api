export const meta = {
  name: 'reverse-cleanup-v6',
  description: '执行逆向代码清理 v6 方案 P2-P7+前端（顺序·编译门禁·逐阶段提交·失败即停）',
  phases: [
    { title: 'P2', detail: '删后端逆向网关路由与账号 OAuth 入口' },
    { title: 'P3', detail: '删 OpenAI/Codex 逆向链' },
    { title: 'P4', detail: '删 Gemini OAuth/CodeAssist 逆向链' },
    { title: 'P5', detail: '删 Claude OAuth + 共享 token 刷新设施 + constants.go 按符号取舍' },
    { title: 'P6', detail: '拆分 pkg/antigravity 共享类型并删 antigravity 整平台' },
    { title: 'P7', detail: '收敛账号类型/Wire/Ent schema + require_oauth_only/mcp_xml_inject + 迁移' },
    { title: 'PF', detail: '前端清理 antigravity/OAuth/Codex 残留 UI 与类型' },
    { title: 'Report', detail: '写执行报告供验收' },
  ],
}

const ROOT = '/Users/leoobai/jiwu-project/SubPanel'
const BE = ROOT + '/backend'
const FE = ROOT + '/frontend'
const DOC = ROOT + '/claudedocs/逆向代码清理执行方案_v6.md'

const RESULT_SCHEMA = {
  type: 'object',
  additionalProperties: false,
  properties: {
    status: { type: 'string', enum: ['green', 'failed'] },
    phase: { type: 'string' },
    commit: { type: 'string', description: 'git 短哈希；未提交则空' },
    summary: { type: 'string', description: '本阶段做了什么（中文，简明）' },
    filesDeleted: { type: 'array', items: { type: 'string' } },
    filesEdited: { type: 'array', items: { type: 'string' } },
    gateResults: { type: 'string', description: 'build/vet/test/typecheck 结果摘要' },
    remainingErrors: { type: 'string', description: '若 failed，剩余报错原文摘要' },
    deviations: { type: 'string', description: '与方案的偏离及理由（无则写无）' },
  },
  required: ['status', 'phase', 'summary'],
}

const GLOBAL = `你是资深 Go + Vue 工程师，正在执行「逆向代码清理 v6」方案的一个阶段。

# 权威规范（必须先读）
完整方案在：${DOC}
开工前先 Read 该文档全文（约 470 行，分两次 Read 即可），它是逐符号 KEEP/DELETE 的权威来源。本阶段提示只是导航，细节以文档为准。

# 三类绝对不能误删（§0.3）
1. 用户登录 OAuth：handler/auth_*_oauth.go、auth_oauth_pending_flow.go、service 层 auth_oauth_email_flow.go / auth_email_oauth_auto.go / auth_oauth_first_bind.go、routes/auth.go、前端 api/auth.ts、*CallbackView.vue、types/index.ts 用户登录 OAuth 块。
2. 面板自有订阅/配额：subscription_service.go、user_subscription_repo.go、handler/subscription_handler.go、前端 payment/*、SubscriptionsView.vue。
3. 正规云接入：Vertex（claude_token_provider.go、vertex_service_account.go、Gemini service_account 分支）、Bedrock、Anthropic/OpenAI/Gemini apikey 直通、GeminiTokenCache、RefreshTokenCache、Lingjing(灵境，默认保留)。

# 核心原则
- 先解耦/迁移共享符号 → 改混合文件去掉对逆向符号的引用 → 再删纯逆向文件 → 最后清 Wire/路由/枚举。
- 禁止照行号盲删：文档里的行号是历史命中点，会随改动漂移。必须打开文件、按符号名核对再改。
- 删除产生的孤儿 import / 变量 / 函数 / stub 字段一并清除。
- 删逆向 provider/接口方法后，所有 test stub 与 _test.go 同步改（grep 'type.*Stub.*struct\\|type.*Mock.*struct' backend/internal）。删的是逆向功能断言，不是为过测试而跳过正规路径覆盖。
- 禁止用「加回已删逆向代码」或「注释/跳过测试」来强行变绿。

# 环境
- 后端 go 命令在：${BE}（如 cd ${BE} && go build ./...）
- 前端 pnpm 命令在：${FE}
- git 命令在仓库根：${ROOT}
- macOS 没有 timeout 命令，不要用它。go build 整模块较慢属正常。

# 完成标准（gate）
必须自己循环 编辑→构建→修复，直到本阶段所有 gate 命令通过。后端每阶段强制：cd ${BE} && go build ./... 与 go vet ./... 全绿。其余 gate 见各阶段说明。

# 收尾
- 达绿后在 ${ROOT} 执行 git add -A && git commit，commit message 用本阶段给定标题，正文末尾加一行：
Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
- 然后用 StructuredOutput 报告：status(green/failed)、commit 短哈希、summary、filesDeleted、filesEdited、gateResults、deviations。
- 若多轮仍无法达绿：不要提交，status=failed，把剩余报错原文摘要填进 remainingErrors，如实报告。`

const PHASES = [
  {
    id: 'P2',
    commit: 'refactor(P2): 删除后端逆向网关路由与账号 OAuth 入口',
    scope: `# Phase P2：关闭后端逆向入口（仅后端，不动前端）
依据 §1(P2 行)、§4.2 路由表、§3.2 account_codex_import。
要做：
- routes/gateway.go：删 antigravity 路由组（约 232-263）；保留 Lingjing(220-230)、/v1、Gemini 正规路由。
- routes/admin.go：删 Codex import 路由(330)、账号 apply-oauth/privacy(337-339)、:id/refresh 路由（其 handler refreshSingleAccount 作为 OAuth 残留一并删）、Gemini/OpenAI/Antigravity OAuth 专用 routes（用 rg -n "oauth|antigravity|setup-token" routes/admin.go 确认）。保留普通账号 CRUD/test/stats。
- routes/auth.go：**完全不动**。
- 对应 handler 方法此刻可暂时无路由引用（Go 仍能编译），不强制本阶段删 handler 函数本体。
注意：本阶段不要动任何 frontend/ 文件。
Gate：cd ${BE} && go build ./... && go vet ./...；go test -tags=unit ./internal/server ./internal/handler/...`,
  },
  {
    id: 'P3',
    commit: 'refactor(P3): 删除 OpenAI/Codex 逆向链',
    scope: `# Phase P3：OpenAI/Codex 逆向链（仅后端）
依据 §3.1(account.go/account_usage_service.go 的 OpenAI 部分)、§3.2(openai_gateway_service.go/openai_ws_forwarder.go/openai_ws_pool.go/openai_codex_transform.go/account_codex_import.go/openai_codex_instructions_template.go/codex_image_generation_bridge.go-保留)、§3.3(openai_gateway_messages.go/openai_gateway_chat_completions.go/upstream_models.go-openai 部分/token_refresher.go-整删)、§3.4(OpenAI OAuth 429 failover 局部切除)、§4.1(wire openai)、附录 A2、§7、§9.1。
关键：
- 整删文件：service/openai_oauth_service.go、service/openai_token_provider.go(P1 已改构造器签名)、service/openai_privacy_service.go(先改 admin_service.go/token_refresh 的 PrivacyClientFactory 签名)、repository/openai_oauth_service.go、pkg/openai/oauth.go(仅此文件，pkg/openai 其它协议转换保留)、handler/admin/openai_oauth_handler.go、service/openai_codex_instructions_template.go(孤儿)、service/openai_codex_transform.go(按 §3.2 切，正规 Responses alias 若仍被 apikey 用则保留)、handler/admin/account_codex_import.go(整删)、service/token_refresher.go(整删)。
- codex_image_generation_bridge.go：**保留勿删**（apikey 图片路径）。
- §3.4：openai_account_runtime_block_fastpath.go 仅局部切除 OAuth 429 逻辑（含同步删唯一调用块 :45-47），保留通用 Block/Clear/Check 与 handleOpenAIAccountUpstreamError 主体；同名 _test.go 改写不整删。
- account.go：SupportsOpenAIImageCapability 不整删，改为 return Type==AccountTypeAPIKey。
- handler 层 openai_gateway_handler.go/openai_chat_completions.go/openai_images.go + service/openai_images.go 删 OAuth 429/AccountTypeOAuth 分支。
- §9.1 此阶段先不动 pkg/claude/constants.go（留 P5）。但 OpenAI 相关 DefaultBetaHeader 等若仅 OAuth 分支引用，分支删除即可，常量本体留 P5 统一处理。
- wire：§4.1 删 ProvideOpenAIOAuthService/ProvideOpenAITokenProvider 等；优先 go generate ./cmd/server，失败手改 wire_gen.go。
- 测试：§7 service/*_test.go OpenAI/Codex OAuth 用例删除；gateway_oauth_metadata_test.go 随对应函数删。
Gate：cd ${BE} && go build ./... && go vet ./...；go test -tags=unit ./internal/service ./internal/handler/...`,
  },
  {
    id: 'P4',
    commit: 'refactor(P4): 删除 Gemini OAuth/CodeAssist 逆向链',
    scope: `# Phase P4：Gemini OAuth/CodeAssist 链（仅后端）
依据 §3.1(gemini_token_provider.go-P1已大部完成,核对剩余)、§3.2(gemini_messages_compat_service.go/gemini_chat_completions_compat_service.go/gemini_v1beta_handler.go/gateway_handler.go 的 gemini 部分)、§3.3、§4.1(wire gemini + repository/wire.go)、附录 A3。
关键：
- 整删文件：service/gemini_oauth.go、service/gemini_oauth_service.go、service/gemini_token_refresher.go、service/geminicli_codeassist.go、repository/gemini_oauth_client.go、repository/geminicli_codeassist_client.go、repository/gemini_drive_client.go、handler/admin/gemini_oauth_handler.go。
- **必保**：service/gemini_token_cache.go、repository/gemini_token_cache.go（Vertex+service_account 复用 GeminiTokenCache）。
- pkg/geminicli/：**不整删·局部拆分**——保留/迁移 AIStudioBaseURL / DefaultModels / DefaultTestModel（被 gemini compat / gateway_handler / account_test 用），删 OAuth/CodeAssist/CLI UA/cloudcode-pa 内容。
- 混合文件按 §3.2 切除 OAuth preference/token flow，保留 AI Studio/apikey base URL 分支。
- gemini_v1beta_handler.go::304 var geminiReq 与 :405/:413 CleanGeminiNativeThoughtSignatures 是正规逻辑，**保留**（antigravity 类型迁移在 P6，本阶段 antigravity.GeminiRequest 仍可引用旧包）。
- wire：repository/wire.go 删 NewGeminiOAuthClient/NewGeminiCliCodeAssistClient/NewGeminiDriveClient/NewClaudeUsageFetcher；保留 NewGeminiTokenCache/NewRefreshTokenCache。service/wire.go 删 NewGeminiOAuthService。优先 go generate，失败手改 wire_gen.go。
- 测试：§7 Gemini OAuth/CodeAssist 用例删除，保留 apikey/service_account/Vertex 覆盖。
Gate：cd ${BE} && go build ./... && go vet ./...；go test -tags=unit ./internal/service ./internal/repository ./internal/handler/...`,
  },
  {
    id: 'P5',
    commit: 'refactor(P5): 删除 Claude OAuth 与共享 token 刷新设施',
    scope: `# Phase P5：Claude OAuth + 共享刷新链 + constants.go 按符号取舍（仅后端）
依据 §3.1(claude_token_provider.go/pkg/claude/constants.go/crs_sync_service.go)、§3.3(handler/dto/mappers.go 的 5h 窗口映射 + token_cache_invalidator.go)、§4.1(wire + cmd/server/wire.go cleanup)、§2.3(pkg/oauth)、附录 A1/A5、§9.1(CRITICAL-1)、§8.4。
关键：
- 整删文件：repository/claude_oauth_service.go、repository/claude_usage_service.go、service/oauth_service.go、service/token_refresh_service.go、service/oauth_refresh_api.go、service/token_cache_key.go。
- **必保**：claude_token_provider.go(删 OAuth refresh 残留，保留 ClaudeTokenCache 与 service_account token)、repository/refresh_token_cache.go、service/refresh_token_cache.go（用户登录 Refresh Token）。
- pkg/oauth：**不整删**。只删 Claude 逆向专属类型（SessionStore/OAuthSession/TokenResponse/URL 构造），保留 PKCE 工具（pkce.go 已抽出，P1 完成）。先 rg 确认用户登录 auth_*_oauth.go 仍能引用 GenerateState/GenerateCodeVerifier/GenerateCodeChallenge。
- §9.1 CRITICAL-1 pkg/claude/constants.go：**严禁按行号区间删，逐符号取舍**。删：BetaOAuth、BetaTokenCounting(确认)、BetaContext1M/FastMode/PromptCachingScope/Effort/RedactThinking/ContextManagement/ExtendedCacheTTL(逐个 rg 确认仅 mimic 用)、DefaultBetaHeader、MessageBetaHeaderNoTools/WithTools、CountTokensBetaHeader、HaikuBetaHeader、FullClaudeCodeMimicryBetas。**🔴必保**：BetaClaudeCode、BetaInterleavedThinking、BetaFineGrainedToolStreaming、APIKeyBetaHeader、APIKeyHaikuBetaHeader、DefaultHeaders、CLICurrentVersion、ModelIDOverrides/ReverseOverrides/NormalizeModelID/DenormalizeModelID(181-215整段)、模型结构列表。每删一个 Beta* 前 rg -n "claude.<符号>" backend/internal --glob '!**/*_test.go' 确认全落在 OAuth/mimic 分支。
- handler/dto/mappers.go：删 IsAnthropicOAuthOrSetupToken 调用(:241)与 5h 窗口/会话映射(WindowCostLimit/WindowCostStickyReserve/MaxSessions)。注意 account.go 的 IsAnthropicOAuthOrSetupToken 整删前先清 15 个外部调用点(§3.1 列出)。
- §8.4 token_cache_invalidator.go：采用**缩小为正规缓存失效器**方案（推荐，注入契约改动面小）。先把通用函数 CheckTokenVersion(:68-114) 迁到独立文件 token_version.go（被 gemini_token_provider/openai_token_provider 调用），再删 AccountTypeOAuth/OpenAI/Claude/Antigravity key 分支，仅保留 Gemini/service_account 需要的失效逻辑。
- cmd/server/wire.go：provideCleanup 删 tokenRefresh *service.TokenRefreshService 入参与 "TokenRefreshService" Stop 步骤。wire_gen.go 同步。
- 测试：删 Claude OAuth/setup-token 用例。
Gate：cd ${BE} && go build ./... && go vet ./...；go test -tags=unit ./internal/service ./internal/repository ./internal/handler/...`,
  },
  {
    id: 'P6',
    commit: 'refactor(P6): 拆分 pkg/antigravity 共享类型并删除 antigravity 整平台',
    scope: `# Phase P6：Antigravity 整平台（仅后端，引用面最大）
依据 §9.2(CRITICAL-2 类型迁移)、§3.5(强类型依赖)、§3.1/§3.2/§3.3(antigravity 各分支)、附录 A4、§7。
**严格顺序**：
1. §9.2 先迁移共享符号（删 pkg/antigravity 前必做）：
   - 新建 internal/pkg/gemini/native_types.go (package gemini)，把 pkg/antigravity/gemini_types.go 里 23 个 Gemini* 协议类型整簇剪过去；V1InternalRequest 留在 antigravity(随删)。
   - DummyThoughtSignature 常量迁到 pkg/gemini(如 pkg/gemini/constants.go)。
   - 改三个正规文件 import：service/gateway_request.go:1116、service/gemini_native_signature_cleaner.go:55、service/gemini_session.go:24，把 antigravity.DummyThoughtSignature→gemini.DummyThoughtSignature、*antigravity.GeminiRequest→*gemini.GeminiRequest；gemini_v1beta_handler.go 同步。这三个文件**仅改 import/类型引用，逻辑保留**。
   - 验证 pkg/gemini 无同名冲突。
2. §3.5 改强类型依赖构造器（删 service/antigravity_*.go 前必做）：ops_service.go、gemini_messages_compat_service.go、gateway_handler.go、gemini_v1beta_handler.go、account_test_service.go、account_usage_service.go、upstream_models.go 去掉 *AntigravityGatewayService/*AntigravityQuotaFetcher 字段与构造器参数及转发分支；service/wire.go + wire_gen.go 同步移除 NewAntigravityGatewayService/NewAntigravityQuotaFetcher/ProvideAntigravityTokenProvider 及 service/wire.go:449 的 antigravity.SetUserAgentVersionResolver 调用。
3. 删 service/antigravity_*.go(10 文件)、handler/admin/antigravity_oauth_handler.go、pkg/antigravity 其余逆向内容(oauth/transformer/client/stream/schema_cleaner/response_transformer/claude_types；gemini_types.go 迁移后剩余随删)。
4. 清 ~94 处 PlatformAntigravity 非 test 引用(§3 各表 + §3.3：admin_service/account_service/account_test_service/scheduler_*/model_rate_limit/endpoint/ops_error_logger/failover_loop/channel_handler/setting_service/domain_constants/scheduler_snapshot/ratelimit_service 等)。domain/constants.go 的 PlatformAntigravity 常量本体可留到 P7(若仍被引用)；本阶段目标是清完所有使用点。
5. 测试：§7 antigravity 用例。⚠️**只删用例不整删** antigravity_*_test.go：stubAntigravityAccountRepo(antigravity_rate_limit_test.go:85) 被 error_policy_test.go:393 复用；tempUnschedCall 被 openai_upstream_transport_error_handle_test.go 复用——先迁出/保留共享 stub 再清理。handler/admin 测试文件(§7 v7 补)同步收敛。
Gate：cd ${BE} && go build ./... && go vet ./...；go test -tags=unit ./internal/service ./internal/repository ./internal/server ./internal/handler/...；rg -n "AntigravityGatewayService|AntigravityQuotaFetcher|DefaultAntigravityModelMapping|antigravity\\." backend/internal backend/cmd （除迁移到 pkg/gemini 的项外应无逆向残留；PlatformAntigravity 常量本体若留 P7 需说明）`,
  },
  {
    id: 'P7',
    commit: 'refactor(P7): 收敛账号类型/Wire/Ent schema 与 require_oauth_only 等死功能',
    scope: `# Phase P7：枚举/Wire/Ent/迁移收敛（后端最后一步）
依据 §3.1(ent/schema account.go/group.go/user_platform_quota.go/error_passthrough_rule.go)、§8.5(require_oauth_only 整删)、§8.6(DROP COLUMN 迁移=方案A)、§8.补(mcp_xml_inject 死 flag)、§4.1(枚举/wire)、domain/constants.go、§7(集成/e2e 测试桶)。
要做：
- domain/constants.go：删 PlatformAntigravity、AccountTypeOAuth、AccountTypeSetupToken、DefaultAntigravityModelMapping(确认 map 结束行)；保留 PlatformLingjing 与正规 account types。
- domain/protocol.go、model/error_passthrough_rule.go、repository/simple_mode_default_groups.go：删 antigravity enum/分组。
- ent/schema/account.go：仅删注释中 oauth/setup-token/antigravity 提及(对生成代码零变更)。
- ent/schema/group.go：删 mcp_xml_inject(:133) 与 require_oauth_only(:152)；ent/schema/user_platform_quota.go::44 平台 switch 去 antigravity；error_passthrough_rule.go 注释。
- §8.5 require_oauth_only 整功能删除：service/group.go、admin_service.go、account_service.go、repository/group_repo.go(63,65,139,141)、handler/admin/group_handler.go、handler/dto/{types,mappers}.go。
- §8.补 mcp_xml_inject 死 flag 删除：含 service/api_key_auth_cache.go:83 MCPXMLInject 字段(json tag 进 Redis 缓存)与赋值点 api_key_auth_cache_impl.go:272,345。
- go generate ./ent（必跑，牵动 group_create/update/mutation/where/runtime/migrate 全套）；go generate ./cmd/server（失败则手改 wire_gen.go）。
- §8.6 方案A：新增前向迁移 backend/migrations/（取最大序号+1）XXX_drop_group_oauth_only_and_mcp_xml_inject.sql：ALTER TABLE groups DROP COLUMN IF EXISTS require_oauth_only, DROP COLUMN IF EXISTS mcp_xml_inject;
- 测试(§7)：先改 repository/fixtures_integration_test.go:173 a.Type = service.AccountTypeOAuth → apikey/service_account，再删常量；删 api_contract_test/protocol_test/endpoint_test 的 antigravity/require_oauth_only 断言；auth_service_platform_quota_test.go 删 antigravity 键、!=4 改 3。
Gate：cd ${BE} && go generate ./ent && go build ./... && go vet ./...；go test -tags=unit ./internal/service ./internal/repository ./internal/server ./internal/handler/...；集成/e2e 编译验证：go vet -tags=integration ./... && go test -tags=integration -run xxx_NONEXISTENT ./internal/repository ./internal/integration && go test -tags=e2e -run xxx_NONEXISTENT ./internal/integration；rg -n "AccountTypeOAuth|AccountTypeSetupToken|require_oauth_only|mcp_xml_inject|setup-token|setupToken|antigravity|PlatformAntigravity" backend/internal --glob '!**/auth_*_oauth.go' （应零命中）`,
  },
  {
    id: 'PF',
    commit: 'refactor(PF): 前端清理 antigravity/OAuth/Codex 残留 UI 与类型',
    scope: `# Phase PF：前端全部清理（独立于后端，集中一阶段以保证 typecheck 始终绿）
依据 §5 前端改动表全部行、§7(frontend *.spec.ts)。
整删：composables/useAccountOAuth.ts、useGeminiOAuth.ts、useAntigravityOAuth.ts、useOpenAIOAuth.ts、components/account/OAuthAuthorizationFlow.vue、api/admin/antigravity.ts。
ReAuthAccountModal.vue 两份(components/account/ + components/admin/account/)整删候选——若仅 apikey refresh 价值则按 §5 处理。
局部改(按 §5 各行 keep/delete 边界)：api/admin/index.ts(去 antigravity barrel/export，gemini 若只剩 OAuth 则整删并同步 barrel)、api/admin/accounts.ts、api/admin/gemini.ts、api/admin/settings.ts、api/admin/users.ts、CreateAccountModal.vue、EditAccountModal.vue、AccountTableFilters.vue、AccountUsageCell.vue(~33 处逆向 UI，含 antigravity_quota computed；同步删 types/index.ts:988 AccountUsageInfo.antigravity_quota 与 AntigravityModelQuota)、UserDashboardStats.vue、AccountsView.vue、SettingsView.vue(保留用户登录 OAuth 块)、types/index.ts(保留用户登录 OAuth 220-229；删 GroupPlatform/AccountPlatform antigravity、AccountType oauth/setup-token、Codex import types、require_oauth_only 字段)、BulkEditAccountModal.vue、ModelWhitelistSelector.vue + composables/useModelWhitelist.ts、utils/ccswitchImport.ts、utils/platformColors.ts + PlatformIcon.vue、components/admin/channel/types.ts + views/admin/groupsSupportedModelScopes.ts、ErrorPassthroughRulesModal.vue/GroupOptionItem.vue/PlatformTypeBadge.vue/UserPlatformQuotaModal.vue/UserPlatformQuotaCell.vue/PlatformUsageBreakdown.vue、UsersView.vue(usage_antigravity 列等)、SubscriptionsView.vue(admin+user)/SubscriptionPlanCard.vue(仅删 antigravity 枚举，保留面板订阅支付)、UseKeyModal.vue、OpsDashboardHeader.vue、GroupsView.vue(require_oauth_only UI 删)、i18n/locales/en.ts + zh.ts(§5 按键路径白名单逐键删：删 account.oauth.*/oauth.cookieAutoAuth*/startAutoAuth/openai.oauthPassthrough*/chatgptOauth/OPENAI_OAUTH_PROXY_REQUIRED/引导教程 OAuth块；**保留** oauthFlow.*社交登录 / googleDrive.*authorize* / auth.callback* / 社交登录键。不要用宽正则误伤社交登录与 Drive 备份)。
测试(§7)：同步更新 *.spec.ts(含 components/account/__tests__/AccountUsageCell.spec.ts)平台 union/列配置/用量组件/模型白名单/支付订阅/ccswitch import 断言。删的是逆向断言，不是跳过正规覆盖。
Gate：cd ${FE} && pnpm run typecheck && pnpm exec vitest run && pnpm run lint:check；rg -n "antigravity|setup-token|setupToken|require_oauth_only|mcp_xml_inject|importCodexSession|OAuthAuthorizationFlow|useOpenAIOAuth|useGeminiOAuth|useAntigravityOAuth|useAccountOAuth" frontend/src （命中逐条核对：用户登录 OAuth/Drive 白名单可留，账号逆向残留不可留）`,
  },
]

const results = []
let stopped = null

for (let i = 0; i < PHASES.length; i++) {
  const p = PHASES[i]
  phase(p.id)
  log(`▶ 开始 ${p.id}：${p.commit}`)

  const execPrompt = `${GLOBAL}

================ 本阶段 ================
阶段标题(commit message 第一行)：${p.commit}

${p.scope}`

  let r = await agent(execPrompt, { schema: RESULT_SCHEMA, label: `${p.id}-exec`, phase: p.id })

  // 容错：执行 agent 未达绿，派全新上下文修复 agent 继续推到绿
  if (!r || r.status !== 'green') {
    log(`⚠ ${p.id} 执行未达绿，启动修复 agent`)
    const repairPrompt = `${GLOBAL}

================ 修复任务（${p.id}）================
上一个 agent 已对本阶段做了大量逆向清理编辑，但未达到编译/测试绿。当前工作区有未提交改动（请勿丢弃）。
上一个 agent 的报告摘要：${r ? JSON.stringify({ summary: r.summary, remainingErrors: r.remainingErrors, gateResults: r.gateResults }) : '（无返回，可能中途中断）'}

你的任务：先读权威文档与本阶段范围，运行 gate 命令，读取所有报错，**继续完成清理**——删除残留逆向引用、补全/删除 stub、清孤儿 import、按 keep/delete 表收敛——直到本阶段所有 gate 全绿。
禁止：加回已删除的逆向代码；注释或跳过测试；删除 §0.3 三类必保代码。
本阶段范围与 gate：
${p.scope}

达绿后 git add -A && git commit（message 用「${p.commit}」+ Co-Authored-By 行），报告 status=green 与 commit。仍无法达绿则 status=failed 并填 remainingErrors。`
    r = await agent(repairPrompt, { schema: RESULT_SCHEMA, label: `${p.id}-repair`, phase: p.id })
  }

  results.push({ phase: p.id, result: r })

  if (!r || r.status !== 'green') {
    stopped = p.id
    log(`⛔ ${p.id} 仍未达绿，停止后续阶段。上一个绿色提交即为还原点。`)
    break
  }
  log(`✅ ${p.id} 完成，commit=${r.commit || '(未报告哈希)'}`)
}

// 最终报告（无论完成或中途停止都写）
phase('Report')
const reportPrompt = `你是技术写作者。把「逆向代码清理 v6」本次自动执行的结果写成一份验收报告，保存到 ${ROOT}/claudedocs/逆向清理_执行报告_v6.md。

执行结构信息（JSON）：
${JSON.stringify({ stoppedAt: stopped, phases: results }, null, 2)}

报告须包含：
1. 总览：完成到哪个阶段、是否中途停止、当前 git HEAD（你可 cd ${ROOT} && git log --oneline -12 查看）。
2. 每阶段：状态、commit、删了哪些文件、改了哪些文件、gate 结果、偏离与理由。
3. 已知偏离说明：本次把所有前端改动集中到 PF 阶段（前端 typecheck 与后端编译独立，集中可避免逐阶段半破坏 import），§8 待拍板项按方案默认/推荐执行（Lingjing 保留、面板用量保留、token_cache_invalidator 缩小为正规失效器、require_oauth_only 删除、mcp_xml_inject 删除、DROP COLUMN 用方案A）。
4. 若中途停止：列出失败阶段的剩余报错与建议的人工接手步骤。
5. 验收建议命令：cd ${ROOT}/backend && go build ./... && go vet ./... && go test -tags=unit ./...；cd ${ROOT}/frontend && pnpm run typecheck && pnpm exec vitest run && pnpm run lint:check。
写完用 StructuredOutput 返回一句话 summary。`
await agent(reportPrompt, { label: 'report', phase: 'Report', schema: { type: 'object', additionalProperties: false, properties: { summary: { type: 'string' } }, required: ['summary'] } })

return { stoppedAt: stopped, completed: stopped === null, phases: results.map(x => ({ phase: x.phase, status: x.result?.status, commit: x.result?.commit })) }
