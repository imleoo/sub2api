export const meta = {
  name: 'reverse-cleanup-v6-p7pf',
  description: '逆向清理 v6 续跑 P7(后端枚举/Ent/Wire/残留收敛)+PF(前端)，顺序·门禁·逐阶段提交',
  phases: [
    { title: 'P7', detail: '账号类型枚举/Wire/Ent schema + require_oauth_only/mcp_xml_inject + 迁移 + 残留收敛' },
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
    commit: { type: 'string' },
    summary: { type: 'string' },
    filesDeleted: { type: 'array', items: { type: 'string' } },
    filesEdited: { type: 'array', items: { type: 'string' } },
    gateResults: { type: 'string' },
    remainingErrors: { type: 'string' },
    deviations: { type: 'string' },
  },
  required: ['status', 'phase', 'summary'],
}

const GLOBAL = `你是资深 Go + Vue 工程师，执行「逆向代码清理 v6」方案。P0-P6 已完成并提交（HEAD 为 P6）。

# 权威规范
完整方案：${DOC}（开工先 Read，逐符号 KEEP/DELETE 的权威来源）。

# 三类绝对不能误删（§0.3）
1. 用户登录 OAuth：handler/auth_*_oauth.go、auth_oauth_pending_flow.go、service 层 auth_oauth_email_flow.go/auth_email_oauth_auto.go/auth_oauth_first_bind.go、routes/auth.go、前端 api/auth.ts、*CallbackView.vue、types/index.ts 用户登录 OAuth 块、i18n 的 oauthFlow.*/googleDrive.*authorize*/auth.callback*。
2. 面板自有订阅/配额：subscription_service.go、user_subscription_repo.go、handler/subscription_handler.go、前端 payment/*、SubscriptionsView.vue。
3. 正规云接入：Vertex/Bedrock/apikey 直通、GeminiTokenCache、RefreshTokenCache、PlatformLingjing(灵境，保留)。

# 核心原则
- 禁止照行号盲删：行号会漂移，按符号名打开文件核对再改。
- 每个 if account.Type==AccountTypeOAuth / ==AccountTypeSetupToken 分支：删除 OAuth 分支，**保留**其 apikey/正规分支（这些分支运行时已不可达，但删时仍要确认不误伤同块的正规逻辑）。
- 删方法/常量前，先清其所有调用点。删除产生的孤儿 import/字段/stub 一并清。
- 改 Go interface 后，所有 test stub 同步改（grep 'type.*Stub.*struct\\|type.*Mock.*struct' backend/internal）。
- 禁止：加回已删逆向代码；注释/跳过测试；删正规路径覆盖。

# 环境
后端 go 命令在 ${BE}；前端 pnpm 在 ${FE}；git 在 ${ROOT}。macOS 无 timeout 命令。

# 完成标准
循环 编辑→构建→修复 直到本阶段所有 gate 全绿，再提交。

# 收尾
达绿后在 ${ROOT}：git add -A && git commit（message 用本阶段标题 + 末行 Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>）。
用 StructuredOutput 报告 status/commit/summary/filesDeleted/filesEdited/gateResults/deviations。多轮仍不绿则不提交，status=failed 填 remainingErrors。`

const P7_SCOPE = `# Phase P7：后端枚举/Ent/Wire/残留收敛（后端最后一步）
依据 §3.1(ent schema)、§4.1、§8.5(require_oauth_only 整删)、§8.6(DROP COLUMN=方案A)、§8.补(mcp_xml_inject 死flag)、§7(集成/e2e 测试桶)。

## 实测残留面（已 grep，非测试代码）——必须全部清零
- AccountTypeOAuth 46 处：gateway_service.go(10)、crs_sync_service.go(10)、openai_ws_forwarder.go(7)、gemini_messages_compat_service.go(6)、account.go(6)、ratelimit_service.go/openai_ws_pool.go/openai_codex_transform.go/domain_constants.go/account_usage_service.go/handler/admin/account_data.go(各1)、domain/constants.go(常量声明)。
- AccountTypeSetupToken 8 处：account.go(2)、gateway_service.go/domain_constants.go/crs_sync_service.go/account_usage_service.go/handler/admin/account_data.go(各1)、domain/constants.go(常量声明)。
- account.go 的 OAuth 专属方法及其调用点要整体清除/收敛：IsOAuth(:158)、GeminiOAuthType(:183)、IsGeminiCodeAssist(:199)、CanGetUsage(:210)、:1012(IsOpenAI&&OAuth)、IsAnthropicOAuthOrSetupToken(:1531，P5 漏删，连同其所有外部调用点一起删，见 §3.1)。删方法前先 grep 全部调用点改掉。
- require_oauth_only / RequireOAuthOnly 26 处(§8.5 整删)：service/group.go、admin_service.go、account_service.go、repository/group_repo.go、handler/admin/group_handler.go、handler/dto/{types,mappers}.go、ent/schema/group.go:152 字段。
- mcp_xml_inject / MCPXMLInject 25 处(§8.补 死flag 整删)：ent/schema/group.go:133 字段、service/api_key_auth_cache.go:83 字段(json tag 进 Redis)、api_key_auth_cache_impl.go:272/345 赋值、其余引用点。
- PlatformAntigravity 2 处：domain/constants.go:24 常量 + service/domain_constants.go 再导出。先确认无消费者(P6 已清)再删。
- ClearAntigravityQuotaScopes(P6 遗留残留)：account_service.go:75 接口方法、repository/account_repo.go:1270 impl、调用点 ratelimit_service.go:1399 与 admin_service.go:2962、以及 7 个 test stub(api_contract_test/gateway_multiplatform_test/gemini_multiplatform_test/ratelimit_session_window_test/account_service_delete_test/admin_service_clear_error_test/ratelimit_service_clear_test) 及其 clearAntigravityCalls/clearAntigravityErr 字段与断言、专属测试 TestRateLimitService_ClearRateLimit_ClearAntigravityFailed。整体删除(antigravity 平台已无，该清理无意义)。

## 枚举/协议/默认分组
- domain/constants.go：删 PlatformAntigravity、AccountTypeOAuth、AccountTypeSetupToken；保留 PlatformLingjing 与正规 account types。service/domain_constants.go 同步删再导出。
- domain/protocol.go：删 antigravity 协议派生。model/error_passthrough_rule.go：删 antigravity enum。repository/simple_mode_default_groups.go：删 antigravity 默认分组。

## Ent schema + 生成 + 迁移
- ent/schema/account.go：仅删注释中 oauth/setup-token/antigravity 提及(对生成代码零变更)。
- ent/schema/group.go：删 mcp_xml_inject 与 require_oauth_only 字段。ent/schema/user_platform_quota.go::44 平台 switch 去 antigravity。error_passthrough_rule.go 注释。
- 跑 go generate ./ent（本地 entgo，应可离线）。go generate ./cmd/server 若离线失败则手改 cmd/server/wire_gen.go。
- §8.6 方案A：新增前向迁移 backend/migrations/(取现有最大序号+1)_drop_group_oauth_only_and_mcp_xml_inject.sql：ALTER TABLE groups DROP COLUMN IF EXISTS require_oauth_only, DROP COLUMN IF EXISTS mcp_xml_inject;（先 ls backend/migrations 看命名规范与最大序号）

## 测试契约(§7)
- 先改 repository/fixtures_integration_test.go 的 a.Type = service.AccountTypeOAuth → apikey/service_account，再删常量。
- 删 api_contract_test/protocol_test/endpoint_test 的 antigravity/require_oauth_only/oauth/setup-token 断言。
- auth_service_platform_quota_test.go 删 antigravity 键、!=4 改 3。
- handler/admin 层测试(account_data_handler_test/admin_service_stub_test/account_handler_available_models_test 等)同步收敛 AccountTypeOAuth/PlatformAntigravity。
- 所有引用上述被删常量/方法的 _test.go 同步改。

## Gate（全绿才提交）
cd ${BE} && go generate ./ent && go build ./... && go vet ./...
go test -tags=unit ./internal/service ./internal/repository ./internal/server ./internal/handler/...
集成/e2e 仅编译：go vet -tags=integration ./... ; go test -tags=integration -run xxx_NONE ./internal/repository ./internal/integration ; go test -tags=e2e -run xxx_NONE ./internal/integration
残留扫描(应零命中)：rg -n "AccountTypeOAuth|AccountTypeSetupToken|PlatformAntigravity|require_oauth_only|RequireOAuthOnly|mcp_xml_inject|MCPXMLInject|ClearAntigravityQuotaScopes|antigravity" backend/internal backend/cmd --glob '!**/auth_*oauth*.go'
commit message: refactor(P7): 收敛账号类型/Wire/Ent schema 与 require_oauth_only/mcp_xml_inject 等死功能`

const PF_SCOPE = `# Phase PF：前端全部清理（独立于后端，集中一阶段保证 typecheck 始终绿）
依据 §5 前端改动表全部行 + §7(frontend *.spec.ts)。
整删：composables/useAccountOAuth.ts、useGeminiOAuth.ts、useAntigravityOAuth.ts、useOpenAIOAuth.ts、components/account/OAuthAuthorizationFlow.vue、api/admin/antigravity.ts；ReAuthAccountModal.vue 两份(components/account/+components/admin/account/)整删候选(若仅 apikey refresh 价值按 §5 处理)。
局部改(§5 各行 keep/delete 边界)：api/admin/index.ts(去 antigravity barrel/export；gemini 若只剩 OAuth 则整删并同步 barrel)、api/admin/accounts.ts、api/admin/gemini.ts、api/admin/settings.ts(去 fallback_model_antigravity/antigravity_user_agent_version 类型)、api/admin/users.ts、CreateAccountModal.vue、EditAccountModal.vue、AccountTableFilters.vue、AccountUsageCell.vue(~33 处逆向 UI+antigravity_quota；同步删 types/index.ts AccountUsageInfo.antigravity_quota 与 AntigravityModelQuota)、UserDashboardStats.vue、AccountsView.vue、SettingsView.vue(保留用户登录 OAuth 块)、types/index.ts(保留用户登录 OAuth；删 GroupPlatform/AccountPlatform antigravity、AccountType oauth/setup-token、Codex import types、require_oauth_only 字段)、BulkEditAccountModal.vue、ModelWhitelistSelector.vue+composables/useModelWhitelist.ts、utils/ccswitchImport.ts、utils/platformColors.ts+PlatformIcon.vue、components/admin/channel/types.ts+views/admin/groupsSupportedModelScopes.ts、ErrorPassthroughRulesModal.vue/GroupOptionItem.vue/PlatformTypeBadge.vue/UserPlatformQuotaModal.vue/UserPlatformQuotaCell.vue/PlatformUsageBreakdown.vue、UsersView.vue、SubscriptionsView.vue(admin+user)/SubscriptionPlanCard.vue(仅删 antigravity 枚举，保留面板订阅支付)、UseKeyModal.vue、OpsDashboardHeader.vue、GroupsView.vue(require_oauth_only UI)、i18n/locales/en.ts+zh.ts(§5 按键路径白名单逐键删：account.oauth.*/oauth.cookieAutoAuth*/startAutoAuth/openai.oauthPassthrough*/chatgptOauth/OPENAI_OAUTH_PROXY_REQUIRED/引导教程 OAuth 块；保留 oauthFlow.*/googleDrive.*authorize*/auth.callback*/社交登录键，勿用宽正则误伤)。
测试(§7)：同步更新 *.spec.ts(含 AccountUsageCell.spec.ts)平台 union/列配置/用量组件/模型白名单/支付订阅/ccswitch import 断言。
Gate(全绿才提交)：cd ${FE} && pnpm run typecheck && pnpm exec vitest run && pnpm run lint:check
残留扫描：rg -n "antigravity|setup-token|setupToken|require_oauth_only|mcp_xml_inject|importCodexSession|OAuthAuthorizationFlow|useOpenAIOAuth|useGeminiOAuth|useAntigravityOAuth|useAccountOAuth" frontend/src （命中逐条核对：用户登录 OAuth/Drive 白名单可留）
commit message: refactor(PF): 前端清理 antigravity/OAuth/Codex 残留 UI 与类型`

const PHASES = [
  { id: 'P7', scope: P7_SCOPE, commit: 'refactor(P7): 收敛账号类型/Wire/Ent schema 与 require_oauth_only/mcp_xml_inject 等死功能' },
  { id: 'PF', scope: PF_SCOPE, commit: 'refactor(PF): 前端清理 antigravity/OAuth/Codex 残留 UI 与类型' },
]

const results = []
let stopped = null

for (let i = 0; i < PHASES.length; i++) {
  const p = PHASES[i]
  phase(p.id)
  log(`▶ 开始 ${p.id}`)

  let r = await agent(`${GLOBAL}\n\n================ 本阶段 ================\n${p.scope}`, { schema: RESULT_SCHEMA, label: `${p.id}-exec`, phase: p.id })

  if (!r || r.status !== 'green') {
    log(`⚠ ${p.id} 执行未达绿，启动修复 agent`)
    const repairPrompt = `${GLOBAL}\n\n================ 修复任务（${p.id}）================\n上一个 agent 已做了大量编辑但未达绿，工作区有未提交改动(勿丢弃)。上次报告：${r ? JSON.stringify({ summary: r.summary, remainingErrors: r.remainingErrors }) : '(无返回，可能中断)'}\n运行 gate，读报错，继续完成清理直到全绿。禁止加回已删代码/跳过测试。达绿后 git commit(message「${p.commit}」+Co-Authored-By)并报告。\n本阶段范围与 gate：\n${p.scope}`
    r = await agent(repairPrompt, { schema: RESULT_SCHEMA, label: `${p.id}-repair`, phase: p.id })
  }

  results.push({ phase: p.id, result: r })
  if (!r || r.status !== 'green') {
    stopped = p.id
    log(`⛔ ${p.id} 仍未达绿，停止。上一个绿色提交即还原点。`)
    break
  }
  log(`✅ ${p.id} 完成 commit=${r.commit || '?'}`)
}

phase('Report')
await agent(`你是技术写作者。把「逆向清理 v6」P7+PF 续跑结果追加写入 ${ROOT}/claudedocs/逆向清理_执行报告_v6.md（若已存在则在末尾追加「## 续跑 P7+PF」小节，不要覆盖原内容）。\n执行结构(JSON)：${JSON.stringify({ stoppedAt: stopped, phases: results }, null, 2)}\n须含：每阶段状态/commit/删改文件/gate 结果/偏离；若中途停止列出剩余报错与人工接手步骤；当前 git HEAD(cd ${ROOT} && git log --oneline -15)。\n用 StructuredOutput 返回一句话 summary。`, { label: 'report', phase: 'Report', schema: { type: 'object', additionalProperties: false, properties: { summary: { type: 'string' } }, required: ['summary'] } })

return { stoppedAt: stopped, completed: stopped === null, phases: results.map(x => ({ phase: x.phase, status: x.result?.status, commit: x.result?.commit })) }
