# E2E 测试覆盖设计（openclaw 真实测试边界内的"全系统闭环"）

> 状态：设计中（brainstorm 收敛产物） · 最后更新：2026-06-24 · 分支：zhiguofan
> 本文是 e2e 覆盖讨论的唯一汇总源。代码引用均为撰写时实况，落地前需复核。

## 1. 目标与已锁定决策

**衡量基准**：全系统端到端闭环，但**限定在 openclaw 真实站点可达的范围内**。

| 决策项 | 结论 |
|--------|------|
| 测试床 | **不 mock**，全部打真实 openclaw（`https://openclaw.zhiguo.fan`，API-key 鉴权） |
| 平台范围 | 仅 **Claude / OpenAI / Gemini**（openclaw 供给的三平台） |
| 故障转移触发 | **坏账号制造真实失败**（错 key / quota=0 / 错 model / 错 base_url）→ 真实上游 4xx → 验证排除+重试。**主路径，已认可** |
| 触发方式 | 维持 `workflow_dispatch` 手动触发（真实扣费 + flaky，不上 push） |

**明确排除**（openclaw 够不到或已决定跳过）：
- **Antigravity**：已移除（无 pkg / 无 service / 不在平台枚举 / 无路由），仅剩 `account_repo.go:666` 一句死 SQL 字符串。无可测路径。
- **支付**（Alipay/Wx/Stripe/EasyPay）：跳过 e2e。
- **上游账号 OAuth token 刷新**：openclaw 用 API-key，不走此路；需另接 OAuth 上游，超范围。
- **Lingjing / Generic**（fork 自定义平台）：不纳入跨平台对称性，靠现有单元测试。

**Token 刷新的语义澄清**：本设计只覆盖**我方 JWT refresh-token 轮转**（`refresh_token_cache.go`：access/refresh 轮转 + 防重放 + 改密失效），不覆盖上游 OAuth 刷新。

## 2. 现状：e2e 覆盖实况

目录 `backend/internal/integration/`，全部带 `//go:build e2e`。两套件：

**① 自包含套件 `TestE2EFull_*`**（`./script/e2e-test.sh` 入口，admin API 自动 seed）
已覆盖：Claude 转发（非流/流/工具/thinking）、OpenAI 转发（**仅非流**）、Gemini 转发、计费扣减、配额拦截、余额不足拒绝、限流、API Key 生命周期、admin 账号·分组 CRUD、批量改模型映射回归（已知陷阱#1）。

**② 黑盒套件**（需预置网关 key）：偏 Claude 协议细节（复杂工具、thinking+tools、no-signature、models 列表）、用户注册登录。

## 3. 缺口地图（按回归风险排序）

### 3.1 网关韧性层（原 P0~P1，零 e2e）
| 优先级 | 用例 | 触发/断言 |
|--------|------|----------|
| P0 | **多账号选择链**（优先级→`PreferSoonestReset`→负载→LRU） | seed N 账号设定状态 → usage_log.account_id 断言命中。**含近期改动代码** |
| P0 | **故障转移 / 账号排除重试** | 坏账号→真实 4xx→断言切好账号成功 + 坏账号 `temp_unschedulable_until` 置位 |
| P1 | **Sticky session 连续性** | 同 session_id 连发 → N 条 usage_log 的 account_id 全相同 |
| P1 | **OpenAI 流式 + 工具调用** | 补与 Claude 的深度不对称 |

### 3.2 功能/计费盲区（完整性批判补充，全部 0 覆盖，带钱/正确性风险）
| 优先级 | 盲点 | 说明 |
|--------|------|------|
| **P0（升级）** | **并发计费竞态 / 余额 double-spend** | 钱的正确性，并发扣余额 race 是真 bug |
| P0 | **多模态计费**（图片/视频） | usage_log 有 `image_count`/`video_seconds`/`image_size_*`，整条零覆盖 |
| P1 | **缓存 token 计费** | `cache_creation_5m/1h`/`cache_read` 维度未断言 |
| P1 | **订阅消耗路径** | `subscription_id`：订阅扣减 vs 余额扣减是两条计费路径，只测了余额 |
| P2 | **幂等性** | 网关有 `X-Idempotency-Replayed` 重放保护，零验证 |
| P2 | **鉴权边界** | 非 admin→403、撤销 key→拒绝、用户间用量隔离 |
| P2 | **JWT refresh-token 轮转** | 自包含可闭环：登录→refresh 换新→旧 token 作废+改密失效 |
| P3 | **Simple 模式** | 起 `RUN_MODE=simple` 实例断言跳计费/配额 |

## 3.3 端点 × 计费模式矩阵（完整性扫描新增，**最大遗漏**）

之前覆盖图把"转发"笼统当一条，实际网关暴露 **~9 条转发端点、≥4 种计费模式**（`routes/gateway.go`）。e2e 只覆盖 token 模式的三条，其余全是盲区——且**不同计费模式各有独立钱风险**。

| 端点 | 计费模式 | e2e 覆盖 | 备注 |
|------|---------|---------|------|
| `POST /v1/messages` | token | ✅ | Claude |
| `POST /v1/chat/completions` | token | ✅ | OpenAI |
| `/v1beta/models/*:generateContent` | token | ✅ | Gemini |
| `POST /v1/messages/count_tokens` | 不计费 | ⚠️ skip | 上游不支持即 skip |
| `POST /v1/responses` + `/responses/*` | token | 🔴 **零** | OpenAI Responses API / Codex direct |
| `GET /v1/responses`（**WebSocket**） | **WS 有状态** | 🔴 **零** | `ResponsesWebSocket`/`openai_ws_v2`，usage 在 WS 流中解析（有 `UsageParseFailureTotal` 指标→会失败），**最高风险** |
| `POST /v1/embeddings` | token（仅 input） | 🔴 **零** | |
| `POST /v1/images/generations` | **image（按张）** | 🔴 **零** | `image billing_mode`，对应 usage_log `image_count`/`image_size_*` |
| `POST /v1/images/edits` | **image（按张）** | 🔴 **零** | |

**结论**：3.2 里"多模态计费"只是冰山一角。真正盲区是**整个 image billing_mode（按张）+ WebSocket 有状态计费 + responses/embeddings 协议**，每条都有独立的成本计算路径未验证。

**openclaw 可达性（已实证探针 2026-06-24，直连 `openclaw.zhiguo.fan/v1`）**：
| 端点 | 探针结果 | 真实 e2e 可行性 |
|------|---------|----------------|
| `/responses` POST | HTTP 200，真实 `resp_...` completed | ✅ **可达可测** |
| `/images/generations` | HTTP 400「requires an image model」（非 404） | ✅ **端点已路由**，需有效 image model（真生成=真付费） |
| `/embeddings` | HTTP 503「temporarily unavailable」 | 🔶 已路由但上游抖动 → skip-on-503 模式可测 |
| `GET /responses` WebSocket | **真 WS 客户端握手 101**（curl 的 426 是 curl 做不了 WS upgrade 的假象） | ✅ **可达可测**，需 `OpenAI-Beta: realtime=v1` 头 + 真 WS 客户端（`coder/websocket`） |

→ **四条全部真实 e2e 可落地**：responses(POST)/images/WebSocket 确认可达，embeddings 需容忍 503。WS 用 `coder/websocket`（项目已依赖）+ realtime beta 头。无悬而未决项。

## 3.4 完整性扫描·其余发现

| 盲点 | 现状 | openclaw 可测? | 优先级 |
|------|------|---------------|--------|
| **账号并发等待队列** | `concurrency_service.go`：账号级并发上限 + 等待队列 + 超时（CLAUDE.md 点名特性），零 e2e | ✅ 设低并发上限、并发打、断言排队/超时 | P1（与并发计费配套） |
| **模型映射 / wildcard 解析** | 仅 batch-edit repro 覆盖一例（`gpt-5.3-codex→gpt-5.4-mini`），通用映射/通配未系统 e2e | ✅ | P1 |
| **beta-header / body 透传** | 本次改动相关：`context_management` 字段 + `anthropic-beta: context-management-2025-06-27` 透传（断错会废 Claude Code CLI 客户端），仅单元测试 | ✅ 若上游校验该字段 | P2 |
| **web-search-emulation** | 账号/渠道级特性（`GetWebSearchEmulationMode`），计费相邻，零 e2e | 🔶 视实现 | P3 |
| **Vertex（Claude on GCP）** | `vertex_service_account.go` 独立网关路径，GCP service-account 鉴权，零 e2e | ⛔ **不走 openclaw**（GCP 鉴权）→ 同 OAuth 归"超 openclaw 范围" | 排除 |

> Vertex 加入第 1 节"明确排除"的同类（openclaw 够不到）。其余四条纳入对应优先级。

## 4. 命门：账号观测机制

P0 选号/sticky/转移的全部断言，依赖"观测请求落到哪个账号"。**已确认可行**：

- `usage_log` 行带 **`account_id` + `request_id`**；网关响应回 **`x-request-id`** 头
- admin `GET /admin/usage` 支持按 **`account_id`/`api_key_id`** 过滤（无 request_id 过滤，响应体含每行 request_id，客户端匹配）
- **观测步骤**：专属 API Key → 发请求抓 `x-request-id` → 轮询 `GET /admin/usage?api_key_id=<id>&sort_order=desc` → 按 request_id 匹配 → 读 `account_id`
- **故障转移双信号**：①正向 usage_log 最终账号=好账号；②排除证明 = 坏账号 `temp_unschedulable_until`/`error_message`/`status` 置位（账号 schema 有这些字段）

### 三个假设的深挖结论（已读码实证，2026-06-24）
1. **`usage_log` 写入时机 = 异步 post-response**：经 `finalizePostUsageBilling`（gateway_service.go:7722）**异步 goroutine** 写库（`postUsageBillingTimeout=15s` 为上限非典型值）。但 `usage_service.Create` 把 **usage_log 行 + 扣费放同一 DB 事务**，故 `account_id`+成本一旦出现即可靠。→ **观测须轮询 `GET /admin/usage`，超时给足（建议 ≤15s）**；沿用现有"等待计费落账"模式。
2. **流式**：在 stream 结束才结算，之后走**同一异步路径**，机制一致、只是相对首字节更晚。低风险，落地时 spot-check 即可。
3. **`temp_unschedulable` 不是通用故障转移信号（结论修正）**：
   - 通用故障转移 = `SelectAccountForModelWithExclusions(excludedIDs)` + **请求内存级排除 + 重试循环**（`maxRetryAttempts=5` / `maxRetryElapsed=10s`，gateway_service.go:3890）。失败账号被加入排除集后重选下一个，**失败尝试不留任何持久痕迹**。
   - `temp_unschedulable` 是**另一套**：受账号 Credentials 门控（`temp_unschedulable_enabled=true`+`temp_unschedulable_rules`，account.go:272），且触发器很窄（Google config error / 空响应等特定类型），同步写。属可选二级覆盖，非主信号。

### 故障转移测试法（修正后，更干净）
- **不依赖持久痕迹**。用**优先级排序**做逻辑闭环证明：把**坏账号设为更高优先级**（会被先选），好账号低优先级。若请求仍 200 且 usage_log 服务账号=好账号，则唯一可能路径 = 坏账号先被选→失败→排除→好账号成功。逻辑上无懈可击，无需逐尝试观测。
- 注意 `maxRetryAttempts=5`：坏账号数别超过重试预算。

### 账号状态可控性（P0 选号用例的落地前提，已实证）
| 选号维度 | 可控性 | 结论 |
|---------|--------|------|
| `priority` | ✅ admin 建/改账号直接设（默认 100） | 优先级选号 + 故障转移优先级trick 完全可测 |
| `load_factor` | ✅ admin 直接设 | 负载维度可测 |
| LRU | ✅ 由请求时序天然产生 | 可测 |
| **`PreferSoonestReset` / `session_window_end`** | 🔴 **两道坎** | **恰是近期改动代码，最难测** |

**🔴 关键发现：改动的那段代码（`filterBySoonestReset`）几乎无法在"真实 openclaw + 纯 admin API"下确定性 e2e：**
1. `PreferSoonestReset` 是**全局配置开关、默认关**（`config.go:1084` `prefer_soonest_reset`）→ 必须专门起一个开了该 flag 的实例。
2. 它读的 `session_window_end` **只由上游响应头驱动**（`rateLimitService.UpdateSessionWindow(ctx, account, resp.Header)`），**admin API 无字段、无路由可设**。无法 seed 出"账号 A 比 B 更早重置"的确定状态。
3. 更糟：openclaw 是中转站，**未必透传 Anthropic 的窗口重置头** → 该 flag 在此环境下可能根本拿不到 window 值，`filterBySoonestReset` 退化为 no-op，**改动代码在 e2e 里等于休眠、无法被真实触发**。

**出路（已定 = (a)，2026-06-24）**：测试起一个 `prefer_soonest_reset=true` 的实例，建两账号后**直连 DB seed 不同 `session_window_end`**，发请求断言命中"最早重置"者。属 seed 状态（非 mock 上游），不违背"不 mock"原则，是唯一能确定性触发 `filterBySoonestReset` 改动代码的路径。
> 备选 (b) 加 test-only/admin setter（污染生产面，否决）、(c) 仅单元测试（改动代码 e2e 裸奔，否决）。

### 约束
- `usage_log` **无 status/error 字段** → 失败尝试不记账 → 故障转移靠上述"优先级排序+最终账号"证明，不靠失败行。
- **配额/限流计数另有 ~2s 批量 flusher**（`user_platform_quota_flusher.go`，默认 `UserPlatformQuotaFlushIntervalMs=2000`）→ 配额/限流断言滞后 ~2s；可压低该 env 提升确定性，或轮询。
- 每用例用 `runNonce()` 隔离新账号/分组。
- 响应内容非确定 → 结构化断言；**精确计费 delta 不测**，只断言方向/比例（已踩 Gemini thinking、count_tokens 两坑）。

## 4.5 并发计费架构（深挖实证，2026-06-24）

读码结论：扣费本身原子，但**架构是后付费 + 前置闸**，并发透支是设计内属性，非 bug。

- **余额扣减**（`usage_billing_repo.go:176`）：裸 SQL `UPDATE users SET balance = balance - $1 ... RETURNING balance`，**原子递减但无 `WHERE balance >= $1` 守卫** → 会扣成负。
- **配额递增**（`:194`）：`quota_used + $1` 原子 + CASE **原子翻转 exhausted** + RETURNING 判越界，设计精良。
- **前置闸**（`billing_cache_service.go:837 checkBalanceEligibility`）：只查**缓存余额 > 0**（`:850 if balance <= 0`），非 DB、非 >=本次成本。
- 三项（订阅/余额/配额）在**同一 tx 内**对单请求原子；race 发生在**并发请求之间**。

**双层 race**：①缓存余额滞后 ②后付费扣减在请求完成后。→ N 个并发请求都见"缓存余额>0"→都转发→都后扣 → **余额可透支为负，透支上界 ≈ 在途并发数**。

**对 P0「并发计费竞态」测试目标的修正**：不是断言"绝不超扣"（超扣是设计内可能），而是**刻画/约束透支边界**——如"余额仅够 1 次 + 10 并发，最终负到多少"。把实际行为钉死，回归（如透支变无界）才抓得到。配额同理（可超发，有界）。

## 4.6 Sticky session（深挖实证，完全可测）

机制：sessionHash **不是客户端传的 session_id**，而是**从请求内容派生**——`BuildAnthropicDigestChain(parsed)`（anthropic_session.go:28）生成 digest 链 `s:<hash(system)>-u:<hash(msg1)>-a:<hash(msg2)>-...`（system + 每条消息规范化 JSON 哈希），按**前缀**匹配。绑定存 (groupID, sessionHash)→accountID，TTL=1h（`stickySessionTTL`），接口 Get/Set/Refresh/Delete。各平台独立派生（anthropic/gemini/openai）。

**测试设计（真实 openclaw 即可，无需特殊配置）**：
1. 分组内放 **≥2 个账号**（只有 1 个时 sticky 恒成立、测了等于没测）
2. 请求1 = `[system S, user "hi"]` → usage_log 记下服务账号 A
3. 请求2 = `[system S, user "hi", assistant "<上轮回复>", user "继续"]`（**延续同一前缀**）→ 断言 usage_log 服务账号仍是 A
4. 即"多账号可选时，同会话延续仍黏在同一账号"——这才证明 sticky 生效

**关键点**：造"同会话"靠**延续对话前缀**（相同 system + 相同历史消息），不是传任何 session 字段。

## 4.7 幂等性 & JWT 轮转（深挖实证，均完全可测）

**幂等性**（客户端 `Idempotency-Key` 头驱动，`idempotency_helper.go:45`）：
- 同 key 重放命中 → 返回缓存响应 + `X-Idempotency-Replayed: true`。配置 `IdempotencyConfig`（`ObserveOnly`/`DefaultTTLSeconds`/`MaxStoredResponseLen` 等）。
- **测试**：同 `Idempotency-Key` 发两次 → 断言第二次 `X-Idempotency-Replayed:true` + 响应一致 + **不二次扣费**（usage_log 仍 1 行）。**用非流式请求**（`MaxStoredResponseLen` 限制大流式存储/重放）。

**JWT refresh 轮转**（自包含，无需上游）：
- `POST /refresh` → `RefreshTokenPair` 轮转发**新** access+refresh；旧 refresh 二次用 → `ErrRefreshTokenReused`（防重放，疑似攻击吊销整个 token family）；改密 bump `TokenVersion` 使所有 JWT 失效。
- **测试**：①登录得 refresh1 → refresh 换 (access2,refresh2)，断言 refresh2≠refresh1；②旧 refresh1 再用 → 断言 401 `REFRESH_TOKEN_REUSED`；③改密后旧 access 被拒（TokenVersion）。

## 5. 元层风险：测试床本身不可信（最致命，须最先解决）

1. **假绿（已证实 + 修法已设计）**：`buildProvision()`（provision_test.go:91）把三种本质不同的错误一律 `t.Skipf` → exit 0 全绿。本次调试开头即被骗（12 用例全 SKIP 却 exit 0）。

   **根因分类**（provision_test.go）：
   - anthropic key 未配（:103）→ **合法 skip**（故意没配）
   - admin 登录失败（:107）→ **应 FAIL**（key 已配=意图跑，却登录崩=真问题，正是我们踩的坑）
   - seed 平台失败（:124）→ **应 FAIL**

   **修法（两级错误分类，最小改动）**：
   ```go
   // buildProvision: 仅"未配 key"返回哨兵错误
   var errE2ENotConfigured = errors.New("E2E 未配置上游凭证")
   if os.Getenv("E2E_ANTHROPIC_UPSTREAM_KEY") == "" {
       return nil, fmt.Errorf("%w", errE2ENotConfigured)
   }
   // requireProvision: 配置已就绪却失败 → Fatal，不再 Skip
   if provErr != nil {
       if errors.Is(provErr, errE2ENotConfigured) { t.Skipf("...未启用：%v", provErr) }
       t.Fatalf("E2E provision 失败（配置就绪却初始化失败）：%v", provErr)
   }
   ```
   - 平台级 `requirePlatform` 的 Skip（:86）**保持**（只配 anthropic、跳 openai/gemini 合法）。
   - **Runner 层加固**（`e2e-test.sh`）：跑完断言"非全 SKIP"——`E2E_ANTHROPIC_UPSTREAM_KEY` 已设时，输出若含"未启用"或 `PASS` 计数为 0 则 runner 退非零。双保险防止未来新增用例又静默 skip。
2. **CI 凭证注入未设计 + e2e 根本没进 CI（已实证）**：`backend-ci.yml` 仅 `workflow_dispatch`，**不注入任何 E2E_* / openclaw 凭证、不跑 e2e**。`e2e.env` 又是 gitignored。
   **修法**：openclaw key 存 GitHub repo secrets（`E2E_ANTHROPIC_UPSTREAM_KEY` 等）→ 新增一个 `workflow_dispatch` job，从 secrets 生成 `script/e2e.env` 或直接注入 env → 跑 `e2e-test.sh`。手动触发，成本/flaky 受控。

3. **dev_local 端口冲突竞态（已实证根因）**：`dev_local.sh:88-91` 启动时若端口被占会 **`kill_port` 直接杀掉占用进程**。故 `e2e-test.sh` auto-boot **杀掉用户已运行服务**→kill→restart 空窗期撞 Go 测试→`connection refused`，叠加假绿 Skip→exit 0。本次开头即此坑。
   **修法**：`e2e-test.sh` 在 `dev_local up` 前**先探目标端口健康**——已有健康服务则**自动切复用模式**（`BASE_URL=已运行实例`、`BOOTED_SERVER=false`，不 boot 不 kill）。既消除 kill→restart race，又免去"忘了设 `E2E_BASE_URL`"的脚枪。

## 6. 推荐路线图

1. **元层先行**：① runner fail-loud（前置失败即 FAIL + 断言执行条数）；② 实测命门三假设（最小探针）；③ 修 dev_local 端口冲突 / 明确"复用已运行服务"为默认。
2. **P0（功能正确性）**：多账号选择链 → 故障转移 → 并发计费竞态 → 多模态计费。
3. **P1**：sticky session → OpenAI 流式/工具 → 缓存计费 → 订阅消耗。
4. **P2+**：幂等 → 鉴权边界 → JWT 轮转 → simple 模式。
5. **CI 收口**：凭证注入 + workflow_dispatch 串起确定性可跑部分。

## 7. 关键引用（撰写时实况，落地复核）

- 网关响应头：`gateway_service.go` / `openai_gateway_service.go` 仅设 `x-request-id`、`X-Accel-Buffering`（不暴露账号）
- usage_log schema：`ent/schema/usage_log.go`（含 account_id/request_id/image_*/video_seconds/cache_*/subscription_id/cost_finalized_at/async_task_id）
- account 健康字段：`ent/schema/account.go`（status/error_message/schedulable/temp_unschedulable_until/temp_unschedulable_reason/session_window_status）
- usage 查询：`internal/handler/admin/usage_handler.go`（List 过滤参数）、路由 `internal/server/routes/admin.go:545` 起
- 平台枚举：`internal/service/domain_constants.go`（Anthropic/Gemini/Lingjing/Generic）
- 假绿点：`internal/integration/e2e_full_provision_test.go:76,86`（t.Skipf）
- 账号选择改动：`gateway_service.go` `SelectAccountWithLoadAwareness` + `filterBySoonestReset`
