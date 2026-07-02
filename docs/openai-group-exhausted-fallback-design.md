# OpenAI 分组账号耗尽兜底设计方案

> **状态**：设计方案（未实现，评审决策已落地，见 §11；若对 §11 决策有异议可随时改写）
> **作者**：zhiguofan
> **日期**：2026-07-01（2026-07-02 完成三轮修订：代码复核与完备性复核见 §8，Codex 独立审阅复核见 §9，第三方独立核查与评审决策见 §10-§11）
> **关联文档**：[`账号级协议适配-bedrock与kiro-设计方案.md`](../claudedocs/账号级协议适配-bedrock与kiro-设计方案.md)（功能37，`fallback_group_id_on_invalid_request` 的设计背景）

---

## 1. 背景与问题

### 1.1 触发场景

用户反馈：OpenAI 平台的分组在管理后台看不到"兜底分组"配置项，而 Anthropic 平台的分组有。

### 1.2 排查结论：不是 UI bug，是功能范围问题——而且现状比预想的更空

`GroupsView.vue` 里存在两个不同的"兜底分组"配置块，**都硬编码只在 `platform === 'anthropic'` 时显示**：

| 字段 | 代码位置 | 真实触发条件 | 覆盖场景 |
|---|---|---|---|
| `fallback_group_id` | [`GroupsView.vue:860-933`](../frontend/src/views/admin/GroupsView.vue#L860) | `claude_code_only=true` 且请求方不是 Claude Code 客户端 | 纯客户端类型限制，与账号健康度无关 |
| `fallback_group_id_on_invalid_request` | [`GroupsView.vue:1163-1182`](../frontend/src/views/admin/GroupsView.vue#L1163)（注释明确写"仅 anthropic 平台"） | 账号级协议适配器——[`account_adapter.go:70-82`](../backend/internal/service/account_adapter.go#L70) 的 `pickAdapter()` 只按账号 flag（`bedrock_compat`/`response_masking`）选出 adapter 实例，真正判定"当前账号的上游（Bedrock/Kiro）处理不了这个请求用到的能力"发生在被选中 adapter 的 `InspectRequest()` 方法里（调用点 [`gateway_service.go:3757`](../backend/internal/service/gateway_service.go#L3757)） | 纯协议能力不兼容，与账号是否耗尽无关 |

追查两个字段在后端的实际消费点（[`gateway_service.go:1915-1926`](../backend/internal/service/gateway_service.go#L1915) `invalidRequestFallbackGroupID()`、[`gateway_service.go:1954-1981`](../backend/internal/service/gateway_service.go#L1954) `resolveGatewayGroup()`），发现**两者都是窄场景专用，没有一个是"分组里所有账号都不可用（限流/余额不足/被禁用/网络故障）时重路由到另一个分组"的通用兜底**。这个最常见的失败场景——[`ErrNoAvailableAccounts`](../backend/internal/service/gateway_service.go#L296)——目前**任何平台**命中后都是直接返回 HTTP 503 "Service temporarily unavailable"，从未有过重路由的选项。

**结论**：用户想要的"OpenAI 兜底分组"，准确讲是一个全新能力（暂称"账号耗尽兜底"），不是把现有字段的平台限制去掉就行；而且这个能力目前对 Anthropic 同样缺失，只是 Anthropic 有另外两个语义不同、容易被误认为"兜底"的字段掩盖了这个事实。

---

## 2. 摸全：OpenAI 网关的账号选择入口清单

`OpenAIGatewayHandler` 上有 **6 个独立的顶层公开方法**，每个都有自己的账号选择重试循环，失败时各自独立处理，**没有像 Anthropic 那样收敛到一处**：

| # | 方法 | 文件:行 | 路由 | 选号函数 | 失败响应方式 |
|---|------|---------|------|---------|-------------|
| 1 | `ChatCompletions` | [`openai_chat_completions.go:23`](../backend/internal/handler/openai_chat_completions.go#L23) | `POST /v1/chat/completions` | `SelectAccountWithSchedulerForCapability` | HTTP JSON（`handleStreamingAwareError`） |
| 2 | `Responses` | [`openai_gateway_handler.go:137`](../backend/internal/handler/openai_gateway_handler.go#L137) | `POST /v1/responses` | `SelectAccountWithSchedulerForCapability` | HTTP JSON |
| 3 | `Messages` | [`openai_gateway_handler.go:612`](../backend/internal/handler/openai_gateway_handler.go#L612) | `POST /v1/messages`（OpenAI 账号走 Anthropic 协议兼容，功能25 相关） | `SelectAccountWithSchedulerForCapability` | HTTP JSON（`anthropicStreamingAwareError`，Anthropic 错误体格式） |
| 4 | `ResponsesWebSocket` | [`openai_gateway_handler.go:1130`](../backend/internal/handler/openai_gateway_handler.go#L1130) | `WS /v1/responses`（流式 v2） | `SelectAccountWithSchedulerForCapability` | **WS 关闭帧**（`closeOpenAIClientWS`），不是 HTTP 响应 |
| 5 | `Embeddings` | [`openai_embeddings.go:23`](../backend/internal/handler/openai_embeddings.go#L23) | `POST /v1/embeddings` | `SelectAccountWithSchedulerForCapability` | HTTP JSON |
| 6 | `Images` | [`openai_images.go:23`](../backend/internal/handler/openai_images.go#L23) | `POST /v1/images/generations` | `SelectAccountWithSchedulerForImages`（**不同的选号函数**） | HTTP JSON |

**路由别名补全**（2026-07-02 复核补充，实现时测试必须覆盖，不能只测主路径）：上表只列了主路由，[`routes/gateway.go`](../backend/internal/server/routes/gateway.go) 里同一批 handler 还挂了多个别名入口——`POST /responses/*subpath`（:77）、`POST /images/edits`（:119，与 `generations` 共用 `Images` handler）、根路径直连的 `POST /responses` + `GET /responses`（:157-159），以及 `/backend-api/codex/responses` 路由组（:160-165，Codex 客户端直连）。这些别名共享同一个 handler 方法，改造本身天然覆盖，但 e2e 验证时要确认每个别名入口的兜底行为一致。

底层选号函数已经是集中的：前 5 个（除 Images）都调用同一个入口 [`openai_account_scheduler.go:1210`](../backend/internal/service/openai_account_scheduler.go#L1210) `SelectAccountWithSchedulerForCapability`，其内部再调 `selectAccountWithLoadAwareness`/`selectAccountForModelWithExclusions`（[`openai_gateway_service.go:1651-2164`](../backend/internal/service/openai_gateway_service.go#L1651)）。这意味着"判断是否耗尽"这件事本身是集中的，**但"耗尽后要不要重路由、怎么重路由"这个决策必须在 6 个调用方各自处理**，因为只有调用方知道怎么把失败写回给客户端（HTTP JSON 还是 WS 关闭帧）。

### 2.1 架构差异：为什么不能照抄 Anthropic 的实现

Anthropic 的 `RequestRerouteError` 之所以好接，是因为账号选择+重试收敛在 [`gateway_handler.go:762-780`](../backend/internal/handler/gateway_handler.go#L762) **一个循环**里，一处 `catch` 就够。OpenAI 没有这个结构，6 个入口分散在 4 个文件里，每处都要单独接入。

### 2.2 依赖缺口：`OpenAIGatewayService` 拿不到 Group 的兜底字段

`GatewayService`（Anthropic）持有 `groupRepo GroupRepository`（[`gateway_service.go:525`](../backend/internal/service/gateway_service.go#L525)），并导出了 `ResolveGroupByID()`（[`gateway_service.go:1911`](../backend/internal/service/gateway_service.go#L1911)）。`OpenAIGatewayService` 的字段列表（[`openai_gateway_service.go:350-380`](../backend/internal/service/openai_gateway_service.go#L350)）里**没有 `groupRepo`**，只有 `channelService *ChannelService`（用于计费/渠道映射，没有读取分组兜底字段的方法）。`OpenAIGatewayHandler` 结构体（[`openai_gateway_handler.go:30-42`](../backend/internal/handler/openai_gateway_handler.go#L30)）也只持有 `*service.OpenAIGatewayService`，不持有 `*service.GatewayService`。

好消息：单跳兜底不需要额外查库——handler 里已经有 `apiKey.Group *Group` 在内存中（[`openai_gateway_handler.go:45`](../backend/internal/handler/openai_gateway_handler.go#L45) `apiKey.Group.ResolveMessagesDispatchModel(...)` 已经在用），可以直接读 `apiKey.Group.FallbackGroupIDOnExhausted`（新字段）而无需查库。但如果要支持**多跳兜底链**（A 耗尽 → B 也耗尽 → C），第二跳开始就需要重新查询 B 分组的对象才能读到它自己的兜底字段，这就需要 `groupRepo` 访问能力——见 §4.3。

---

## 3. 设计目标

| 目标 | 说明 |
|------|------|
| **分组级容灾** | 分组内所有账号不可用时，能自动重路由到运营预先配置的另一个分组，而不是直接 503 |
| **平台无关** | 触发条件（账号耗尽）与协议、平台无关，理论上 OpenAI/Anthropic/Gemini 都适用；本方案优先实现 OpenAI，但字段/UI 设计上不排斥后续对齐 Anthropic |
| **防环** | 支持链式兜底（A→B→C），但必须检测循环配置（A→B→A） |
| **不影响现有 failover 语义** | 与"同分组内账号级 failover"（`failedAccountIDs` 排除重试）是两层不同机制，不能混淆。触发条件以 §4.2 修正后的口径为准：HTTP 入口看 `!streamStarted`（未向客户端写过响应字节即可换组），且失败原因必须是容量/健康类耗尽（见 §4.2"策略性拒绝必须排除"小节）；~~初版"只有 `len(failedAccountIDs) == 0` 才算耗尽"的条件已废弃~~ |
| **零回归** | 未配置兜底组的分组，行为与现在完全一致 |

---

## 4. 方案设计

### 4.1 数据模型

新增字段 `fallback_group_id_on_exhausted`（与现有两个字段同构，`Optional().Nillable() int64`）：

```go
// ent/schema/group.go
field.Int64("fallback_group_id_on_exhausted").
    Optional().
    Nillable().
    Comment("账号耗尽兜底使用的分组 ID（分组内无可用账号时重路由目标）"),
```

- 新增迁移 `migrations/163_group_fallback_group_id_on_exhausted.sql`
- 不复用现有 `fallback_group_id_on_invalid_request`：语义不同（耗尽 vs 能力不兼容），复用会让字段名和实际行为对不上，未来排障容易误判为"这个请求触发了协议能力不兼容"

**字段流转清单**（2026-07-02 复核补充——只加 schema + migration 是不够的，字段进了库也不会被 API 创建/更新/返回，以下每一环都要补，参照现有两个 fallback 字段的对应位置逐一对齐）：

| 环节 | 位置 | 参照 |
|---|---|---|
| Ent schema + 生成代码 | `ent/schema/group.go` + `go generate ./ent` | [`group.go:112-116`](../backend/ent/schema/group.go#L112) 现有两字段 |
| service 层 `Group` 结构体 | [`service/group.go:40`](../backend/internal/service/group.go#L40) 附近 | `FallbackGroupIDOnInvalidRequest` 字段 |
| admin 输入结构（Create/Update Input） | [`admin_service.go:188`](../backend/internal/service/admin_service.go#L188) 附近 | 同上 |
| repository create/update 落库 | [`group_repo.go:59`](../backend/internal/repository/group_repo.go#L59) / [:174](../backend/internal/repository/group_repo.go#L174) | 同上 |
| handler DTO 类型 | [`dto/types.go:110-112`](../backend/internal/handler/dto/types.go#L110) | `fallback_group_id_on_invalid_request` json tag |
| DTO mapper | [`dto/mappers.go:188-189`](../backend/internal/handler/dto/mappers.go#L188) | 同上 |
| 前端类型定义 + create/edit form + payload | `GroupsView.vue`（form 定义约 :2921、payload 组装约 :3230）+ api 类型 | 同上 |

### 4.2 触发条件调研：429 等错误是否也会落到兜底组里

**结论先行：会，而且比最初设想的覆盖面更广——初版方案里"只在首次选号就失败时触发"的条件过窄，站不住脚，已修正。**

#### 调研过程

用 LSP `findReferences`/`goToDefinition` 追了一遍 `Account.IsSchedulable()`（[`account.go:120-141`](../backend/internal/service/account.go#L120)，OpenAI 选号链路里 `isOpenAIAccountEligibleForRequest` → `IsSchedulableForModelWithContext` 最终都会走到这个方法）的调度排除条件：

```go
func (a *Account) IsSchedulable() bool {
    if !a.IsActive() || !a.Schedulable { return false }
    if a.AutoPauseOnExpired && ... { return false }
    if a.OverloadUntil != nil && now.Before(*a.OverloadUntil) { return false }      // 529 过载冷却
    if a.RateLimitResetAt != nil && now.Before(*a.RateLimitResetAt) { return false } // 429 限流冷却
    if a.TempUnschedulableUntil != nil && ... { return false }
    if a.IsAPIKeyOrBedrock() && a.IsQuotaExceeded() { return false }
    return true
}
```

再追账号级失败处理入口 [`openai_account_runtime_block_fastpath.go:26`](../backend/internal/service/openai_account_runtime_block_fastpath.go#L26) `handleOpenAIAccountUpstreamError()` → `RateLimitService.HandleUpstreamError()`（[`ratelimit_service.go:164`](../backend/internal/service/ratelimit_service.go#L164)）的状态码分支：

| 上游状态码 | 处理函数 | 是否持久化排除状态 | 排除方式 |
|---|---|---|---|
| 429 | `handle429()`（[:788](../backend/internal/service/ratelimit_service.go#L788)） | ✅ 是 | `accountRepo.SetRateLimited()` → `RateLimitResetAt` |
| 529 | `handle529()`（[:1332](../backend/internal/service/ratelimit_service.go#L1332)） | ✅ 是 | 过载冷却 → `OverloadUntil` |
| 401 | 内联分支（[:227](../backend/internal/service/ratelimit_service.go#L227)） | ✅ 是（永久） | `handleAuthError` → 账号标记 error，`shouldDisable=true` |
| 402 | 内联分支（[:256](../backend/internal/service/ratelimit_service.go#L256)） | ✅ 是（永久） | 同上，余额不足视为永久性问题 |
| 403 | `handle403()`（[:716](../backend/internal/service/ratelimit_service.go#L716)） | 视具体错误内容而定 | — |
| 5xx（未启用自定义错误码） | `default` 分支（[:301](../backend/internal/service/ratelimit_service.go#L301)） | ❌ 否，仅记录日志 | 只影响当次请求内的 failover 重试，不影响未来请求 |

而 [`shouldFailoverUpstreamError()`](../backend/internal/service/openai_gateway_service.go#L2364)（决定"要不要在同分组内换账号重试"的判定函数）覆盖的状态码正好是 `401, 402, 403, 429, 529` 以及所有 `>= 500`——**与上表高度重合**。换句话说：**能进入 failover 重试循环的状态码，本身就已经被过滤成"容量/鉴权/账号健康类问题"，不包含真正的请求格式错误**（400 类错误不在 `shouldFailoverUpstreamError` 列表里，会直接透传给客户端，从不进入选号重试循环，也就不可能导致"耗尽"）。

#### 两条耗尽路径，都该触发兜底

1. **预先耗尽**：分组内账号被*其它*并发请求的 429/401/402/403/529 提前打上排除标记（`RateLimitResetAt`/`OverloadUntil`/disable），这次新请求的**首次**选号（`len(failedAccountIDs)==0`）就直接找不到账号——这正是初版方案设计的触发条件，成立。
2. **当次请求内耗尽**：这次请求自己在 failover 循环里依次把分组内账号试了一遍，每次都命中 429/其它 failover 状态码，账号被逐个排除、`failedAccountIDs` 最终覆盖全部账号，选号最终失败——初版方案把这种情况排除在外（理由是"问题可能出在请求本身"），但上面的调研证明**这个理由不成立**：会进入 failover 循环的状态码本来就已经排除了请求格式类错误，所以路径 2 和路径 1 在本质上是同一类问题（分组容量/健康度问题），没有理由区别对待。

#### 修正后的触发条件

不再用 `len(failedAccountIDs) == 0` 判断，改用是否已经向客户端发送过响应字节（`streamStarted`，6 个入口的选号循环里已经存在这个变量，用于 `handleStreamingAwareError`/`handleFailoverExhausted` 判断能不能安全地整个换一种方式重新应答）：

- **`!streamStarted`**：选号最终失败（不管是首次失败还是 failover 耗尽后失败）→ 尝试兜底组重路由，找不到兜底组才落回现有的 503 / `handleFailoverExhausted`
- **`streamStarted == true`**：已经有响应字节发给客户端了，不能静默换组重试（客户端可能已经收到部分内容），维持现状，不触发兜底

这个条件天然覆盖了 429 等场景，且逻辑上更站得住脚：判断依据从"重试了几次"换成了"现在换组重试是否还安全"。

**注意**：`streamStarted` 只在 5 个 HTTP 入口存在（`openai_chat_completions.go:24`、`openai_gateway_handler.go:139` 等），**`ResponsesWebSocket` 的选号循环里没有这个变量**（[`openai_gateway_handler.go:1320-1340`](../backend/internal/handler/openai_gateway_handler.go#L1320)，失败路径直接 `closeOpenAIClientWS`）——初版"6 个入口的选号循环里已经存在这个变量"的说法对 WS 不成立，WS 的触发条件需要单独定义，见 §4.5。

#### 策略性拒绝必须排除：`ErrNoAvailableAccounts` 不全是"耗尽"（2026-07-02 复核新增，高风险）

上面的调研有一个漏洞：**`ErrNoAvailableAccounts` 这个错误哨兵并不只在"账号真的耗尽"时返回**。用 grep 追 `ErrNoAvailableAccounts` 的所有产生点后发现，至少两类"策略性拒绝"也被包装成同一个错误：

1. **渠道定价限制**：`checkChannelPricingRestriction` 命中时（[`openai_gateway_service.go:1652`](../backend/internal/service/openai_gateway_service.go#L1652)、[:1862](../backend/internal/service/openai_gateway_service.go#L1862)），直接返回 `fmt.Errorf("%w supporting model: %s (channel pricing restriction)", ErrNoAvailableAccounts, requestedModel)`——这是"该分组的渠道策略**禁止**这个模型"，不是"账号不够用了"。
2. **compact 能力缺失**：`ErrNoAvailableCompactAccounts`（[`openai_gateway_service.go:345-347`](../backend/internal/service/openai_gateway_service.go#L345)，产生点 [:1324](../backend/internal/service/openai_gateway_service.go#L1324)、[:2162](../backend/internal/service/openai_gateway_service.go#L2162)）——"分组内没有账号支持 `/responses/compact` 能力"，属于能力配置问题，不是容量问题。

如果 §4.4 的 handler 改造按"任意 `err != nil` 且 `!streamStarted`"触发兜底，会把这两类请求也重路由到兜底组：**运营在原分组明确禁止的模型，会通过兜底路径在另一个分组被放行**——这等于给用户开了一条绕过分组级模型/渠道限制的后门，比"多试一个分组"的容量兜底完全是两种性质的行为。

**修正后的触发条件（最终版）**：`!streamStarted` **且** 失败原因属于容量/健康类耗尽。落地方式二选一：

- **方案 A（推荐）**：为容量类耗尽引入独立的错误哨兵（如 `ErrGroupCapacityExhausted`），在 `selectAccountForModelWithExclusions`/`selectAccountWithLoadAwareness` 里区分"策略拒绝"（保持现有 `ErrNoAvailableAccounts` 包装）和"扫完所有账号无一可调度"（改用新哨兵），handler 只对新哨兵触发兜底。改动集中在 service 层，语义最干净。
- **方案 B**：handler 层用 `errors.Is` + 错误消息特征排除 channel pricing restriction / `ErrNoAvailableCompactAccounts`。改动小但依赖错误消息文本，脆弱，不推荐。

对应地，§4.4 伪代码里的触发判断 `if (err != nil || selection == nil || ...) && !streamStarted` 需要改为对具体错误类型的甄别，不能照现在的写法实现。

### 4.3 服务层：新增 helper + 群组解析能力

在 `OpenAIGatewayService` 新增字段 `groupRepo GroupRepository`。**helper 的返回值设计（2026-07-02 复核修正）：不要只返回目标 ID，要返回加载并校验过的目标分组对象**——初版 helper（只 `return group.FallbackGroupIDOnExhausted`）没有加载目标分组，无法校验目标"是否启用/是否同平台"（下文的禁用绕过风险），而这些校验必须收敛在 helper 内部，不能指望 6 个调用方各自记得处理：

```go
// openai_gateway_service.go
// ResolveExhaustedFallbackGroup 解析当前分组的耗尽兜底目标。
// 返回 nil 表示：未配置、目标不存在/已删除、目标已禁用、目标平台不兼容——调用方一律视同"没有兜底"。
func (s *OpenAIGatewayService) ResolveExhaustedFallbackGroup(ctx context.Context, groupID *int64) *Group {
    if groupID == nil || s.groupRepo == nil {
        return nil
    }
    group, err := s.groupRepo.GetByIDLite(ctx, *groupID)
    if err != nil || group == nil || group.FallbackGroupIDOnExhausted == nil {
        return nil
    }
    target, err := s.groupRepo.GetByIDLite(ctx, *group.FallbackGroupIDOnExhausted)
    if err != nil || target == nil {
        return nil // 目标被删除（SoftDeleteMixin 天然 ErrNotFound）
    }
    if !target.IsActive() {
        return nil // 目标被禁用：不能绕过运营的禁用开关，见下文
    }
    // 平台校验策略见 §5 风险 2（保存时校验为主，此处兜底复查）
    return target
}
```

注意：返回 `*Group` 对象还有一个连带好处——§4.4 里"重路由后重新计算订阅资格/RPM/渠道映射"需要的正是兜底组的完整对象，handler 拿到它就不用再查一次库。

**保存时校验器（2026-07-02 复核新增，必须做）**：现有两个 fallback 字段在后端保存时都有校验器，新字段不能没有——否则运营可以直接通过 API 配出循环链/跨平台目标，运行时 `visitedGroups` 只能保证不死循环，拦不住错误配置本身：

- [`admin_service.go:1916`](../backend/internal/service/admin_service.go#L1916) `validateFallbackGroup`：**支持链式**，用 `visited` map 沿链走到底做完整环检测，还校验目标存在性和 `claude_code_only` 死循环条件
- [`admin_service.go:1955`](../backend/internal/service/admin_service.go#L1955) `validateFallbackGroupOnInvalidRequest`：**强制单跳**（目标分组不得再配置兜底）+ 平台一致性 + 排除自身 + 订阅类型限制

两种范式后端都有现成先例。新增 `validateFallbackGroupOnExhausted` 时按 §4.7 的链式/单跳决策二选一直接参照对应实现：选链式抄 `validateFallbackGroup` 的环遍历，选单跳抄 `validateFallbackGroupOnInvalidRequest` 的"目标不得再配兜底"约束；两种都要加上"目标必须同平台（或至少提示）+ 目标必须启用"的校验。

**`wire_gen.go` 改动点已核实，改动量很小**：`NewOpenAIGatewayService`（[`openai_gateway_service.go:410-432`](../backend/internal/service/openai_gateway_service.go#L410)）目前是 21 个参数，不含 `groupRepo`；但 `groupRepository` 这个变量在 [`cmd/server/wire_gen.go:53`](../backend/cmd/server/wire_gen.go#L53)（`repository.NewGroupRepository(client, db)`）已经构造好，只是没有传给 `NewOpenAIGatewayService`（[`wire_gen.go:146`](../backend/cmd/server/wire_gen.go#L146)）。改动包括：constructor 签名 + struct 字段 + `wire_gen.go` 调用处追加实参。优先跑 `go generate ./cmd/server` 重新生成，仅在 Wire 工具因网络问题不可用时才按 CLAUDE.md 惯例手动同步 `wire_gen.go`。

**关于查库开销，用 LSP 追查后需要修正上一版的表述**：`GetByIDLite`（[`group_repo.go:104`](../backend/internal/repository/group_repo.go#L104)）本身**没有缓存层**，是直接的 Ent 查询（`r.client.Group.Query().Where(group.IDEQ(id)).Only(ctx)`）。`GatewayService.resolveGroupByID`（[`gateway_service.go:1900`](../backend/internal/service/gateway_service.go#L1900)）之所以看起来"轻量"，是因为它先查 `groupFromContext`（[`gateway_service.go:1893`](../backend/internal/service/gateway_service.go#L1893)）——但那只是读**当前请求 ctx 里中间件已解析好的原分组对象**，条件是 `group.ID == groupID`。这意味着：
- **首跳**（查询原分组自己的 `FallbackGroupIDOnExhausted`）：可以直接读 `apiKey.Group`（handler 已有的内存对象），零查库
- **重路由后的每一跳**（查询兜底组 B 自己的兜底字段，用于支持链式兜底）：`groupID` 已经不是 ctx 里的原分组，`groupFromContext` 必然 miss，**会真实触发一次数据库查询**

所以多跳链的每一跳都有真实的数据库往返开销，不是"可忽略"。阶段一验证时应把"多跳兜底链的延迟"作为专项测试点（对应 §5 风险 4）。

**目标分组被禁用（非删除）时的兜底绕过风险**：`Group` 启用了 `SoftDeleteMixin`（[`ent/schema/group.go:30`](../backend/ent/schema/group.go#L30)），所以 `GetByIDLite` 对已删除的目标分组天然返回 `ErrNotFound`，`exhaustedFallbackGroupID` 的 `err != nil → nil` 分支已经兜住了"兜底目标被删除"这一种悬空引用。但目标分组被**禁用**（`Status`/`IsActive()` 为 false，记录仍在）时完全没有防护——`group.IsActive()` 目前只在鉴权中间件（[`api_key_auth.go:330`](../backend/internal/server/middleware/api_key_auth.go#L330)）里针对 apiKey 自身绑定的分组检查一次，`exhaustedFallbackGroupID` 和后续的 `SelectAccountWithSchedulerForCapability` 都不会复查重路由目标分组的启用状态。运营在后台禁用一个分组，本意是阻断该分组的所有流量，但如果有其它分组把它配成"耗尽兜底目标"，这个被禁用的分组依然会通过重路由路径继续悄悄接收请求，绕开了"禁用分组"这个运营侧的硬开关。**这个校验必须补在 `exhaustedFallbackGroupID` helper 内部**（查到目标分组后再判一次 `IsActive()`，非启用则视同未配置兜底），不能指望每个调用方各自记得处理。

### 4.4 Handler 改造模式（HTTP 类，5 个入口通用）

以 `ChatCompletions` 为例，改造点集中在选号失败分支，新增一层"重路由 or 503"判断，仿照 Anthropic 的换组重试写法：

```go
currentGroupID := apiKey.GroupID
visitedGroups := map[int64]struct{}{}
if currentGroupID != nil {
    visitedGroups[*currentGroupID] = struct{}{}
}

for {
    selection, scheduleDecision, err := h.gatewayService.SelectAccountWithSchedulerForCapability(
        ctx, currentGroupID, previousResponseID, sessionHash, reqModel,
        failedAccountIDs, service.OpenAIUpstreamTransportAny,
        service.OpenAIEndpointCapabilityChatCompletions, requireCompact,
    )
    // 触发条件见 §4.2 调研结论：不看 len(failedAccountIDs)，看 streamStarted——
    // 只要还没给客户端写过响应字节，不管是首次选号失败还是 failover 耗尽后失败，都可以安全地
    // 整个换一个分组重试。
    // ⚠️ 2026-07-02 复核修正：err != nil 不能直接当"耗尽"——channel pricing restriction /
    // ErrNoAvailableCompactAccounts 等策略性拒绝也包装成 ErrNoAvailableAccounts（见 §4.2
    // "策略性拒绝必须排除"小节），此处必须换成对容量类耗尽哨兵（方案 A 的
    // ErrGroupCapacityExhausted）的 errors.Is 判断，下面的写法仅示意流程结构。
    if isCapacityExhausted(err, selection) && !streamStarted {
        if fb := h.gatewayService.ResolveExhaustedFallbackGroup(ctx, currentGroupID); fb != nil {
            if _, seen := visitedGroups[fb.ID]; seen {
                reqLog.Warn("openai.exhausted_fallback_cycle_detected", zap.Int64("group_id", fb.ID))
                // 落入下方原有 503 / failover exhausted 分支
            } else {
                visitedGroups[fb.ID] = struct{}{}
                currentGroupID = &fb.ID
                currentGroup = fb // 供换组后重算订阅资格/RPM/渠道映射使用（见下方连带影响）
                failedAccountIDs = make(map[int64]struct{}) // 换组后重置排除集，新组账号未必和旧组重叠
                lastFailoverErr = nil
                continue // 重新进入循环，用新 groupID 重选
            }
        }
    }
    // ...原有的错误分支（503 / failover exhausted）保持不变
}
```

**需要注意的连带影响**（`ChatCompletions`/`Responses`/`Messages`/`Embeddings` 四处都要过一遍）：
- 计费/日志字段里的 `group_id` 要用重路由后的 `currentGroupID`，不能停留在最初的 `apiKey.GroupID`，否则账单和实际扣费账号所属分组对不上
- **粘性会话绑定与并发槽的 `groupID` 传参当前是硬编码 `apiKey.GroupID`**（2026-07-02 复核新增）：`acquireResponsesAccountSlot(c, apiKey.GroupID, ...)`（[`openai_chat_completions.go:177`](../backend/internal/handler/openai_chat_completions.go#L177)、[`openai_gateway_handler.go:379`](../backend/internal/handler/openai_gateway_handler.go#L379)）和 WS 的 `BindStickySession(ctx, apiKey.GroupID, ...)`（[`openai_gateway_handler.go:1379`](../backend/internal/handler/openai_gateway_handler.go#L1379)）。由于粘性缓存 key 按 `groupID` 隔离（见下文），重路由后若不改传 `currentGroupID`，兜底账号会被绑回**原分组**的粘性 key——下次请求在原分组恢复后按粘性命中一个不属于原分组的账号。所有选号后使用 `groupID` 的调用点（槽位、绑定、usage log、ops 日志）必须统一切换到 `currentGroupID`
- `markOpsRoutingCapacityLimited(c)` 之类的运维指标标记，要能区分"最终 503"和"重路由后又失败"，避免误报
- **换组后必须重置 `failedAccountIDs`**（旧组账号 ID 在新组里没有意义，若不重置，新组恰好有 ID 相同的账号会被误排除；若新组账号 ID 集合与旧组不重叠则无影响，但清空是唯一在两种情况下都正确的做法）
- 因为路径 2（当次请求 failover 耗尽后触发）现在也会重路由，`lastFailoverErr` 之类记录"最后一次 failover 失败原因"的变量在换组后也要重置，避免新组的错误日志/客户端错误体里混进旧组的失败原因

#### RPM 限流：只在原分组门口查了一次，重路由后新分组的限流被绕过

追了 `CheckBillingEligibility`（[`billing_cache_service.go:708`](../backend/internal/service/billing_cache_service.go#L708)）的调用点——[`openai_chat_completions.go:116`](../backend/internal/handler/openai_chat_completions.go#L116)，用 `apiKey.Group`（也就是**最初**的分组）在**选号循环开始之前**调用一次，不是账号级"槽位"占用（不像 `ConcurrencyHelper` 那种 acquire/release 语义），而是 Redis 计数器语义（`checkRPM`，[`billing_cache_service.go:761`](../backend/internal/service/billing_cache_service.go#L761)）：先 `IncrementUserGroupRPM(ctx, user.ID, group.ID)` 判断是否超过 `group.RPMLimit`，**若超限则直接 `return ErrGroupRPMExceeded`，短路退出**（[:788-813](../backend/internal/service/billing_cache_service.go#L788)）；只有分组级判断为"未超限"或"未配置限额"时，才会继续执行 `IncrementUserRPM(ctx, user.ID)` 判断是否超过 `user.RPMLimit`（用户级全局硬上限，与分组无关）。也就是说这不是"两段无条件都执行"，而是短路逻辑——但在**正常放行路径**下（分组未超限，请求才能真正进入选号循环），用户级计数必然会被执行一次，这正是下面两个问题成立的前提。

这引出两个需要产品决策的具体问题：
1. **重路由后要不要对新分组重新跑一次 `checkRPM`？**
   - **不跑**：新分组自己配置的 `RPMLimit` 对这次通过兜底路由过来的请求完全不生效，运营配的"这个分组最多 N RPM"的预期被绕过。
   - **跑**：`IncrementUserRPM(ctx, user.ID)` 这一步没有分组维度，会被调用两次（原分组一次、兜底分组一次），导致该用户的**全局 RPM 计数被虚增 1**——高频用户在触发兜底重路由的时段里，会比正常情况更快撞到 `user.RPMLimit`，属于反直觉的"重路由惩罚"。
2. 目前没有第三个选项能两全，需要产品明确取舍（或者接受"新分组 RPM 检查缺失"作为已知限制写进上线说明，后续再补 `checkRPM` 支持"只查 group 维度、跳过 user 维度重复计数"的变体）。

#### 换组重试并非零副作用：渠道映射、订阅资格、计费费率均锚定在最初分组

除 RPM 外，至少还有三处逻辑在选号循环**之前**用最初的 `apiKey.Group` 算好，重路由后从未重新计算：

- **渠道模型映射**：`ResolveChannelMappingAndRestrict(ctx, apiKey.GroupID, reqModel)`（[`openai_chat_completions.go:97`](../backend/internal/handler/openai_chat_completions.go#L97)）在选号循环（135 行）之前执行，用的是原始 `apiKey.GroupID`；转发时（186-188 行）用这份映射改写请求 model。重路由到兜底组后，兜底组账号可能会收到一份不属于它自己的模型映射/限制，轻则模型名对不上导致上游 400，重则绕开兜底组自己配置的模型白名单。
- **订阅资格与限额**：`CheckBillingEligibility(..., apiKey.Group, ...)`（[`openai_chat_completions.go:116`](../backend/internal/handler/openai_chat_completions.go#L116)）内部 `checkSubscriptionEligibility`（[`billing_cache_service.go:873-908`](../backend/internal/service/billing_cache_service.go#L873)）按最初分组的 `group.ID` 查订阅状态和日/周/月限额，重路由后从未对兜底组重新校验——兜底组自己配置的限额形同虚设，用户理论上也可能被路由进其无权访问的订阅分组。
- **计费费率**：`RecordUsage`（[`openai_gateway_service.go:5741-5747`](../backend/internal/service/openai_gateway_service.go#L5741)）取的费率倍数来自 `apiKey.Group.RateMultiplier`（对象引用），即使按下文 §4.4 建议把日志 `group_id` 换成 `currentGroupID`，实际计费费率仍然锚定最初分组——这意味着 §5 风险 3（计费口径该按谁的价格算）不只是一个"需要产品决策的口径问题"，还牵涉要不要改 `OpenAIRecordUsageInput` 之类的结构体，是一项实打实的代码改动，不是配置开关能解决的。**问题比初判更深**（2026-07-02 复核补充）：不只是费率倍数，连定价维度的分组归属也锚定原分组——`usageLog.GroupID = apiKey.GroupID`（[`openai_gateway_service.go:5901`](../backend/internal/service/openai_gateway_service.go#L5901) 附近）和成本计算里直接取 `apiKey.Group.ID` 作为 `CalculateCostUnified` 的 `GroupID` 入参（[:6022](../backend/internal/service/openai_gateway_service.go#L6022) 附近）都是同样的模式，修复时这几处要一起改，不能只改一处费率倍数。

**这不是本方案独有的疏漏**：Anthropic 现有的 `RequestRerouteError` 重路由机制（[`gateway_handler.go:167`](../backend/internal/handler/gateway_handler.go#L167) 循环外算 `channelMapping`，[:780-783](../backend/internal/handler/gateway_handler.go#L780) reroute 时只换 `currentGroupID`）本身就有同样的问题。下文 §4.4 建议"仿照 Anthropic 的换组重试写法"，如果原样照抄，等于把这个既有缺陷也一并带入 OpenAI 侧，而不是复用一个已经验证过的成熟范式。建议实现阶段把"重路由后重新计算 channelMapping/订阅资格"作为阶段一的显式验收项，而不是默认沿用 Anthropic 的现状。

#### 粘性会话缓存已按 groupID 隔离，重路由无跨组命中风险（复核修正）

初版方案曾主张：`getStickySessionAccountID(ctx, groupID, sessionHash)`（[`openai_sticky_compat.go:122`](../backend/internal/service/openai_sticky_compat.go#L122)）内部构造缓存 key 的 `openAISessionCacheKey(sessionHash)`（[`openai_sticky_compat.go:91`](../backend/internal/service/openai_sticky_compat.go#L91)）只拼了 `"openai:" + sessionHash`，不含 `groupID`，据此推断重路由后可能"跨组命中"原分组写入的账号 ID。

**这个结论经进一步追查后不成立，已作废**：`openAISessionCacheKey` 产出的只是传给仓储层的 `primaryKey` 参数，真正落盘的 Redis key 由 `buildSessionKey(groupID, sessionHash)`（[`gateway_cache.go:27`](../backend/internal/repository/gateway_cache.go#L27)）构造，格式为 `"sticky_session:{groupID}:{primaryKey}"`——**已经按 `groupID` 完整隔离**。重路由到兜底组后，即便复用同一个 `sessionHash` 查询粘性缓存，也只会因为 key 里的 `groupID` 不同而缓存未命中（cache miss），自然落回正常的优先级+LRU 选号流程，不存在"跨组命中错误账号"的风险，**无需为此做任何额外改造**（原方案建议的"重路由后跳过粘性会话读取"不再需要）。

**复核时顺带发现一个与重路由无关、但同样会在本方案验证阶段暴露的既有副作用**：`acquireResponsesAccountSlot`（[`openai_gateway_handler.go:1079`](../backend/internal/handler/openai_gateway_handler.go#L1079) 附近）在请求**转发成功前**就会写入 `sessionHash → accountID` 粘性缓存。如果账号随后失败、且分组账号耗尽后未配置兜底组（或兜底也失败），缓存会残留指向一个已失败账号，供下次同会话请求命中后再次触发同样的失败-重试。这是网关既有逻辑的独立问题，不由本方案引入，但阶段一验证耗尽场景时会自然暴露，建议一并记录、评估是否需要在耗尽最终失败时清理粘性缓存。

### 4.5 `ResponsesWebSocket` 的特殊性

WS 连接在选号循环执行时已经完成升级（[`openai_gateway_handler.go:1300-1305`](../backend/internal/handler/openai_gateway_handler.go#L1300) 此时已用 `wsConn` 做billing校验），但账号选择本身只是决定"用哪个分组的哪个账号"，不涉及重新握手——重路由只需要把循环里的 `groupID` 变量换掉、`wsConn` 对象不用动，改造方式和 HTTP 类一致，只是失败分支换成 `closeOpenAIClientWS` 而非 JSON 响应。风险点：如果客户端在选号阶段已经发送了首条消息帧（`firstMessage`），重路由后这条消息要保证仍然会被转发到新选中的账号，需要确认 `firstMessage` 变量在循环外层，不会因为重试而丢失（现有 failover 重试逻辑应该已经处理了这一点，需要复用同一套，不要另起一套）。

**触发条件需要单独定义（2026-07-02 复核新增）**：§4.2 的 `!streamStarted` 判断依赖 HTTP 入口的 `streamStarted` 变量，但该变量**不存在于 WS 选号循环**——实际代码（[`openai_gateway_handler.go:1320-1340`](../backend/internal/handler/openai_gateway_handler.go#L1320)）选号失败时直接进入 `closeOpenAIClientWS`/`closeOpenAIWSFailoverExhausted` 分支，没有任何"是否已写响应"的中间状态判断。WS 场景下"是否还能安全换组重试"的等价判断应该是：**是否已经把上游返回的任何消息帧转发给客户端**（对应 WS 连接升级后、账号选定前 vs 账号选定后已开始双向转发这两个阶段），需要引入一个类似 `streamStarted` 语义的独立标志，不能直接照抄 HTTP 入口的变量名字对付过去。

### 4.6 `Images` 的特殊性（已用 LSP 查证，结论：无特殊性，可安全套用同一套改造）

`SelectAccountWithSchedulerForImages`（[`openai_account_scheduler.go:1224`](../backend/internal/service/openai_account_scheduler.go#L1224)）和 `SelectAccountWithSchedulerForCapability`（[:1210](../backend/internal/service/openai_account_scheduler.go#L1210)）最终都调用同一个私有实现 `selectAccountWithScheduler`（[:1243](../backend/internal/service/openai_account_scheduler.go#L1243)），只是传入的 capability 参数类型不同（`OpenAIImagesCapability` vs `OpenAIEndpointCapability`）。错误类型（`ErrNoAvailableAccounts`）、粘性会话读写（同一个 `sessionHash` 参数）、选号失败判断逻辑完全共用一套底层代码，没有 Images 专属的失败语义。

唯一的额外分支：`SelectAccountWithSchedulerForImages` 内部有一层"如果要求 native 能力但没有可用 APIKey 账号，自动降级到 basic capability（OAuth 账号）重试一次"（[:1236-1239](../backend/internal/service/openai_account_scheduler.go#L1236)）。这层降级发生在函数**内部**、对 handler 透明，属于同分组内的能力降级重试，不影响兜底组重路由设计——handler 层只需要照旧判断"函数返回后 `selection` 是否为 nil"即可，无需为 Images 单独处理。

### 4.7 前端

`GroupsView.vue` 新增一个独立配置块（**不是**把 §1.2 表格里现有的 `v-if="['anthropic'].includes(...)"` 从 anthropic 改成 `['anthropic','openai']`——语义不同，文案也得区分）：

- i18n key 新增 `admin.groups.exhaustedFallback.{title,hint,noFallback}`（参照 `invalidRequestFallback` 的现有三个 key 命名）
- `v-if` 条件：**"不限平台展示"与"阶段一只实现 OpenAI"存在矛盾，需要重新决策**（2026-07-02 复核修正）——初版建议"由于触发条件本身平台无关，直接不限平台展示"，但 §7 验证计划阶段一到阶段三只覆盖 OpenAI 的 6 个入口，Anthropic/Gemini/Generic/Lingjing 分组若能配置这个字段却不会按同语义生效，运营会误判"已经配置了兜底"而实际没有任何效果。**建议改为**：`v-if` 只对当前后端已实现的平台展示（阶段一到阶段三只有 `openai`），文案明确标注"仅 OpenAI 网关入口生效"；后续对齐 Anthropic（见 §5 风险 11）时再放开对应平台的展示
- 下拉选项 `exhaustedFallbackOptions`，仿照 `invalidRequestFallbackOptions` 的现有实现（排除自身、按平台过滤候选目标分组——**这里要决定：兜底目标分组是否要求同平台**，见 §5 风险项）
- 前端与后端 DTO/表单/payload 的完整改动清单见 §4.1"字段流转清单"

**与 §3 设计目标的潜在冲突**：现有 `invalidRequestFallbackOptions`（[`GroupsView.vue:2776`](../frontend/src/views/admin/GroupsView.vue#L2776) 附近）的候选过滤方式不是"检测循环并提示"，而是直接把候选范围收窄为"自身尚未配置 `fallback_group_id_on_invalid_request`"的分组——从产品交互层面直接禁止多跳链。而 §3 把"支持链式兜底 A→B→C"列为设计目标之一。二者若不对齐，会出现两种后果：

- 若 `exhaustedFallbackOptions` 复用同样的"候选必须未配置自身兜底"过滤逻辑，则 §3 目标 3（链式兜底）在前端层面根本无法配置出来，后端 §4.4 的 `visitedGroups` 循环检测代码永远不会被触发，属于无效设计；
- 若不复用、允许自由选择任意分组（包括已配置兜底的分组）作为候选，则必须实现真正的图遍历循环检测（不只是排除自身候选），比文档当前描述的前端工作量更大。

需要在评审阶段明确二选一：要么明确前端不做链式限制、补图遍历检测；要么承认本方案实际只支持单跳兜底，同步把 §3 目标 3 改为"仅支持单跳兜底，链式作为后续迭代"。

**后端两种范式都有现成先例，直接对应这个二选一**（2026-07-02 复核补充）：`admin_service.go` 里已经分别实现了两种校验器——[`validateFallbackGroup`](../backend/internal/service/admin_service.go#L1916)（**支持链式**，用 `visited` map 沿链走到底做完整环检测）和 [`validateFallbackGroupOnInvalidRequest`](../backend/internal/service/admin_service.go#L1955)（**强制单跳**，校验目标分组不得再配置自己的兜底）。选链式就照抄前者的环遍历逻辑，选单跳就照抄后者"目标不得再配兜底"的约束，不需要从零设计校验算法。

---

## 5. 风险与边界（需要产品/工程决策的点）

1. **触发条件必须排除策略性拒绝，否则会绕过分组模型限制**（2026-07-02 复核新增，**最高优先级**，详见 §4.2"策略性拒绝必须排除"小节）：`ErrNoAvailableAccounts` 同时被"账号真的耗尽"和"渠道定价策略禁止该模型"（`checkChannelPricingRestriction`）、"分组没有支持 compact 能力的账号"（`ErrNoAvailableCompactAccounts`）三种场景复用。如果 §4.4 按"任意 `err != nil` 且 `!streamStarted`"触发兜底，会把运营在原分组明确禁止的模型请求重路由到兜底组放行——这不是"多试一个分组"的容量兜底，而是给用户开了一条绕过分组级模型/渠道限制的后门。必须先落地 §4.2 提出的独立错误哨兵（或等价的错误类型甄别），再实现 handler 改造，否则整个方案的触发条件都是错的。
2. **兜底目标分组是否要求同平台？** 如果 OpenAI 分组的账号全耗尽，重路由到一个 Anthropic 分组，客户端发的是 OpenAI 协议请求，Anthropic 分组的账号处理不了——这种跨协议重路由没有意义，必须在保存配置时校验兜底目标分组的 `platform` 与当前分组一致（或至少提示风险）。
3. **计费口径**：重路由后实际扣费该按最初分组的费率还是兜底分组的费率？两个分组的 `rate_multiplier`/折扣配置可能不同，需要产品明确"到底按谁的价格算"，这直接影响 `usage_log` 该记哪个 `group_id`。**这不只是口径决策，还牵涉具体代码改动**：详见 §4.4"换组重试并非零副作用"小节，`RecordUsage` 的费率倍数、`usageLog.GroupID`、`CalculateCostUnified` 的定价 `GroupID` 均当前锚定最初分组的对象引用，需要修改 `OpenAIRecordUsageInput` 等结构体才能切换到兜底组费率。
4. **多跳链的深度限制**：即使做了循环检测，是否要限制最大跳数（比如最多 3 跳），避免运营配置了一条很长的链导致单次请求延迟暴涨（每跳都要过一次完整的账号扫描）。这个决策还依赖风险 9 的前端链式配置取舍——如果前端最终只支持单跳，本项可直接作废。
5. **RPM 限流被重路由绕过 vs 用户级 RPM 重复计数**（详见 §4.4"RPM 限流"小节，已用代码调用链核实）：`checkRPM` 只在选号循环**之前**、用最初分组查了一次，且是短路逻辑——分组级超限直接返回不再执行用户级判断；只有分组级放行时才会继续判断 `user.RPMLimit`。不跑第二次则兜底组的 `RPMLimit` 形同虚设；跑第二次则在正常放行路径下 `user.RPMLimit`（全局硬上限，无分组维度）会被重复计数，产生"重路由惩罚"。两者必须二选一或设计新变体，无法自然两全。
6. **换组重试未重新计算渠道映射与订阅资格**（新增，详见 §4.4"换组重试并非零副作用"小节）：重路由后 `channelMapping`（渠道模型映射/限制）、`checkSubscriptionEligibility`（订阅资格与日/周/月限额）均未针对兜底组重新计算，可能导致模型映射错位、上游 400，或绕开兜底组自己的订阅限额。且 Anthropic 现有 `RequestRerouteError` 机制本身就有同样的缺陷，"仿照 Anthropic 写法"不能直接视为已验证安全的范式，需要在本方案里显式修复而不是照搬。
7. **目标分组被禁用时的绕过风险**（新增，详见 §4.3）：现有软删除机制能兜住"兜底目标分组被删除"，但对"目标分组被禁用（`IsActive()=false`）但记录仍在"没有任何防护——运营禁用分组本意是阻断流量，但作为兜底目标被间接路由进来的流量不受影响。`ResolveExhaustedFallbackGroup` helper 必须显式校验目标分组的启用状态（见 §4.3 已修正的 helper 设计）。
8. **并发耗尽下的流量雪崩**（新增）：分组耗尽通常是瞬时性、批量性事件（一批账号同时触发限流/欠费），命中同一分组的所有并发请求会在同一时刻把负载转移到同一个兜底组。仓库内未发现针对"分组耗尽状态"的熔断或背压机制（`singleflight` 目前只用于配置/余额加载去重，不覆盖这个场景），需要评估兜底组能否承受这种瞬时流量转移，是否需要加限流、排队或渐进式切流。
9. **前端链式配置与循环检测的产品决策**（新增，详见 §4.7）：现有 `invalidRequestFallbackOptions` 的候选过滤方式是直接禁止多跳（候选只能是"自身未配置兜底"的分组），与 §3"支持链式兜底"目标冲突，需要在评审阶段明确二选一：允许链式则复用 `validateFallbackGroup` 的环遍历校验；不允许则复用 `validateFallbackGroupOnInvalidRequest` 的单跳约束，同步把 §3 目标 3 改为"仅支持单跳兜底"。
10. **可观测性**：重路由发生时必须有明确的日志/指标（结构化字段区分"耗尽重路由"和"能力不兼容重路由"和"failover"），否则线上出现"请求偶发变慢"时无法定位是不是兜底链导致的。
11. **范围决策**：这次只做 OpenAI，还是顺带给 Anthropic 也补上"账号耗尽兜底"（目前 Anthropic 同样没有）？如果只做 OpenAI，将来大概率会被问"为什么 Anthropic 没有"；如果两个都做，工作量会更大（Anthropic 那边虽然选号收敛在一处、改造点少，但要重新梳理它和已有 `RequestRerouteError`/`FallbackGroupIDOnInvalidRequest` 机制的优先级关系——同一个分组如果同时命中"能力不兼容"和"账号耗尽"该走哪个兜底）。
12. **触发频率评估**（§4.2 修正后新增）：由于触发条件从"仅首次选号失败"放宽为"`!streamStarted` 即可"，401/402（永久禁用类错误）在一个账号被误配置/欠费但还没人发现之前，会让**这个分组接下来的每一次请求**都经历"选号失败 → 判定重路由 → 查询兜底字段"的额外开销（即使最终因为没配兜底组而 503）。这层额外查询**没有缓存**（见 §4.3 的 `GetByIDLite` 调研修正），是真实的数据库往返，需要评估账号大面积失效时的整体影响，建议阶段一验证时专门测一下这个场景的延迟。
13. **与"当次请求内耗尽"重路由的用户可见延迟**：路径 2（先在原分组走完整个 failover 序列、试了 N 个账号都失败，才重路由到新分组）比路径 1（首次选号直接失败即重路由）多了 N 次账号级请求往返的延迟。如果 N 较大（分组账号数多），单次请求的尾延迟可能明显变长，需要考虑是否要给"同分组内 failover 尝试次数"设置上限，超过后即使还有未试账号也直接触发重路由（用"及时换分组"换"更快的失败/成功"，这是一个体验 vs 穷举的取舍，需要产品定夺）。

> **已复核并撤回的风险**：初版曾列出"粘性会话缓存跨组命中"作为风险项，经追查仓储层 `buildSessionKey(groupID, sessionHash)`（[`gateway_cache.go:27`](../backend/internal/repository/gateway_cache.go#L27)）后确认 Redis key 已按 `groupID` 完整隔离，重路由不会导致跨组命中，详见 §4.4"粘性会话缓存已按 groupID 隔离"小节，不再单独列为风险项。

---

## 6. 备选方案（暂不推荐）

**方案 B：只做单一入口（比如只支持 `ChatCompletions`），其余 5 个入口维持现状。**
理由暂不推荐：用户预期是"OpenAI 分组"整体有兜底能力，只做一个入口会造成"为什么 `/v1/responses` 没有但 `/v1/chat/completions` 有"的新困惑，属于把问题从"平台维度不一致"换成"接口维度不一致"，没有本质解决。如果要分阶段交付，建议按 §7 的验证计划分阶段，而不是砍掉入口覆盖面。

---

## 7. 验证计划 / 分阶段建议

**阶段一（最小可用）**：数据模型 + service 层 helper + `ChatCompletions`（使用量最大的入口）接入 + 前端配置 UI。**触发条件必须先落地 §5 风险 1（策略性拒绝甄别）再开始 handler 改造**，否则阶段一交付的就是一个会绕过分组模型限制的功能。测试拆成两层，不要混在一起——`ResolveExhaustedFallbackGroup` helper 本身只查两次库，不具备跨分组状态感知能力，循环检测（`visitedGroups`）实际写在 handler 层的选号循环里，两者的验证目标不同：
- **`ResolveExhaustedFallbackGroup` helper 单测**（service 层）：覆盖空值（`groupID` 为 nil / `groupRepo` 为 nil / 未配置兜底）、查到（返回正确的目标 `*Group`）、查不到（目标不存在）、目标已禁用（对应 §5 风险 7）四种情况。
- **handler 层集成测试**（循环检测与触发条件甄别的真正落点）：至少覆盖"首次选号失败 + 配置了兜底组，断言重新用新 group_id 发起了一次选号"、"A→B→A 循环配置，断言检测到重复后落回原有 503/failover-exhausted 分支而不是死循环"、"**channel pricing restriction 导致的选号失败不触发兜底**（对应 §5 风险 1）"三种场景，需要 mock 至少两个分组的账号选择结果。
- **保存时校验单测**（admin service 层，对应 §5 风险 9 的决策结果）：新增的 `validateFallbackGroupOnExhausted`（或等价校验器）需要覆盖自引用、环检测、目标不存在、目标平台不一致这几种非法配置。

**阶段二**：`Responses`/`Messages`/`Embeddings`/`Images` 四个 HTTP 入口补齐，复用阶段一验证过的 helper，逐个加集成测试。

**阶段三（风险最高，单独验证）**：`ResponsesWebSocket`。需要真实 WS 连接的集成测试，覆盖"选号阶段耗尽触发重路由后，首条消息仍正确转发到新账号"这个场景——这是本方案里最容易出隐蔽 bug 的地方。

**每阶段完成后**：跑一次真实上游的 e2e（仿照本次 Gemini 生图计费修复的验证方式）——故意把某个分组的账号全部设为 `is_enabled=false` 或耗尽速率限制，配置兜底组指向一个真实可用分组，验证请求最终成功且账单记录的 `group_id` 符合 §5 风险 3 的口径决策；同时补一条"分组因渠道限制拒绝模型"的 e2e，验证**不会**被误判为耗尽兜底（对应 §5 风险 1）。

---

## 8. 修订记录（2026-07-02）

基于对 §1-§7 全部代码引用的逐条复核（5 路并行验证：前端字段与 6 个入口方法、服务层依赖缺口与 wire_gen 改动量、触发条件调研的状态码逻辑、RPM 与粘性会话细节、方案完备性），本次修订：

**修正的错误**：
- §1.2：`pickAdapter()` 职责描述修正——能力判定实际发生在被选中 adapter 的 `InspectRequest()`，`pickAdapter()` 本身只负责按账号 flag 选出 adapter 实例
- §4.4：RPM 限流描述修正为短路逻辑（分组级超限直接 `return`，不会执行到用户级判断），原"不管前面判断结果如何都再判断"的表述不准确
- §4.4：**撤回**"粘性会话缓存跨组命中"风险——追查到仓储层 `buildSessionKey(groupID, sessionHash)` 后确认 Redis key 已按 `groupID` 隔离，原风险判断的代码追溯不彻底，结论不成立

**新增的风险与设计缺口**（完备性复核发现，均已用代码核实）：
- 换组重试未重新计算渠道映射（channelMapping）、订阅资格、计费费率——三者均锚定最初分组的对象引用，且 Anthropic 现有 `RequestRerouteError` 机制本身就有相同缺陷，"仿照 Anthropic 写法"不等于安全
- 目标分组被禁用（非删除）时的兜底绕过风险，现有软删除机制未覆盖这种场景
- 并发耗尽场景下的流量雪崩，仓库内未找到应对瞬时流量转移的熔断/背压机制
- 前端现有候选过滤逻辑（`invalidRequestFallbackOptions`）直接禁止多跳链，与 §3"支持链式兜底"目标冲突，需要评审阶段二选一
- §7 测试计划中"循环检测"应作为 handler 层集成测试，不应算作 service 层 helper 单测的覆盖项
- 附带发现一个与本方案无关的既有问题：`acquireResponsesAccountSlot` 在转发成功前写入粘性缓存，分组彻底耗尽时可能残留指向失败账号，建议一并记录评估

**未变更**：其余行号级别的偏差（2-9 行）经核实均不影响结论有效性，未逐一修正；§4.2 触发条件调研的核心逻辑链（`shouldFailoverUpstreamError` 覆盖范围、两条耗尽路径都应触发兜底）经复核成立，未做改动。

---

## 9. 修订记录（2026-07-02，Codex 独立审阅补充）

在 §8 修订完成后，另行委托 Codex 对全文做了一次不预设结论的独立审阅，随后逐条用 grep/sed 直接核对代码证据，确认全部发现真实存在（无虚构行号或臆测结论）。本次补充修订：

**新增最高优先级风险**：
- §4.2/§5 风险 1（新增，最高优先级）：`ErrNoAvailableAccounts` 同时被"账号耗尽"和"渠道定价策略拒绝"（`checkChannelPricingRestriction`，[`openai_gateway_service.go:1652`](../backend/internal/service/openai_gateway_service.go#L1652)/[:1862](../backend/internal/service/openai_gateway_service.go#L1862)）、"compact 能力缺失"（`ErrNoAvailableCompactAccounts`）三种场景复用。初版 §4.4 伪代码按"任意 `err != nil`"触发兜底会误吞后两种策略性拒绝，导致运营在原分组禁止的模型被兜底路由放行——绕过分组级模型/渠道限制。这是本方案迄今发现的最高优先级设计缺陷，必须先解决触发条件甄别，再实现 handler 改造。

**修正/补全的设计细节**：
- §3：修正设计目标表格与 §4.2 触发条件描述的内部矛盾（`len(failedAccountIDs)==0` vs `!streamStarted`）
- §4.1：新增"字段流转清单"——初版只写了 Ent schema + migration，遗漏了 service Group 结构体、admin input、repository、DTO、前端类型这条完整链路
- §4.3：helper 设计修正为返回目标 `*Group` 对象（而非只返回 ID），使其能够加载并校验目标分组的启用状态；新增"保存时校验器"要求，指出 `admin_service.go` 已有链式（`validateFallbackGroup`）和单跳（`validateFallbackGroupOnInvalidRequest`）两种现成范式可直接复用；`NewOpenAIGatewayService` 参数数量精确修正为 21 个（原"约20个"）
- §4.4：计费费率问题的证据补充（`usageLog.GroupID`、`CalculateCostUnified` 的定价 `GroupID` 同样锚定原分组，不只是费率倍数）；新增粘性会话/并发槽传参当前硬编码 `apiKey.GroupID`、重路由后需统一改为 `currentGroupID` 的具体证据
- §4.5：新增 `ResponsesWebSocket` 缺少 `streamStarted` 等价变量的说明，WS 触发条件需要单独定义
- §4.7：新增前端"不限平台展示"与"阶段一只实现 OpenAI"的矛盾说明及修正建议；补充后端两种校验器范式可直接对应链式/单跳的产品决策
- §2：补全路由别名清单（`/responses/*subpath`、`/images/edits`、`/backend-api/codex/responses` 等）
- §7：测试计划新增策略性拒绝不触发兜底的用例、保存时校验单测

**风险编号调整**：因新增 §5 风险 1，原风险 1-12 依次顺延为 2-13，文中所有交叉引用已同步更新。

---

## 10. 第三方独立核查记录（2026-07-02）

对 §1-§9 全部代码引用（约 50 条）做了 5 路并行、与作者本人无关的独立复核，逐条重新读码验证，未预设信任文档已有的"复核"结论。核查结果：**全部代码引用与核心论证成立**，仅发现两处此前复核未察觉的瑕疵：

1. **§4.2 论证略有夸大**：文档主张"当次请求内 failover 耗尽"与"预先耗尽"是同一类问题、不应区别对待。独立核查确认这个方向是对的，但论证过头——触发 failover 重试的 5xx 错误**不会**持久化排除账号（`ratelimit_service.go` 默认分支 `shouldDisable=false`，不写库，只影响当次请求内重试），而 429/401/402/529 会持久化排除（`RateLimitResetAt`/`OverloadUntil`/禁用）。也就是说，如果一次请求纯粹因为连续撞上 5xx 而耗尽兜底触发条件，分组账号在这次请求结束后的下一秒其实仍然可调度——这和"账号被真正标记为不可用"在运行时状态上不完全等价。不影响"该不该触发兜底"的最终结论，但实现时若有依赖"耗尽后账号状态"做后续判断的逻辑（例如是否需要额外冷却期），需注意这个差异。
2. **§4.7 参照行号轻微失焦**：文档称 payload 组装在 `GroupsView.vue` 约 3230 行附近，实际该行是 `editForm` 字段声明，真正的 `fallback_group_id_on_invalid_request` payload 赋值在约 3690-3696 行。不影响结论，但实现时按此文参照定位会先找错地方，建议直接以 3690-3696 为准。

## 11. 评审决策（2026-07-02，用户未即时响应，按推荐项落地，可随时改写本节）

对 §5 中需要产品/工程拍板、且代码本身给不出唯一答案的关键决策点，评审阶段给出以下结论：

| 决策点 | 结论 | 理由 |
|---|---|---|
| §5风险9/§4.7 链式 vs 单跳 | **仅单跳** | 复用现有 `validateFallbackGroupOnInvalidRequest` 的单跳约束和前端候选过滤逻辑，改动小、无多跳查库延迟风险，符合 §7 阶段一"最小可用"原则。§3 目标 3（链式兜底）改为后续迭代项，非本次范围。§5风险4（多跳深度限制）随之作废。 |
| §5风险3 计费口径 | **按兜底分组费率结算** | 语义上更自洽：实际消耗的是兜底分组账号的上游资源，成本应记在资源实际发生方。需要修改 `RecordUsage` 费率倍数、`usageLog.GroupID`、`CalculateCostUnified` 的 `GroupID` 入参，统一改用重路由后的 `currentGroupID`/对应 `Group` 对象。 |
| §5风险5 RPM 限流 | **阶段一跳过新分组 RPM 检查，写入已知限制** | 不新增 `checkRPM` 变体，接受"兜底分组自己的 RPMLimit 对重路由请求暂不生效"作为阶段一已知限制，写进上线说明；避免用户级 RPM 重复计数的"重路由惩罚"问题。后续如有需求再补"只查 group 维度、跳过 user 维度重复计数"的变体。 |
| §5风险11 范围 | **仅 OpenAI** | 与 §7 验证计划的阶段划分一致。Anthropic 已有 `RequestRerouteError`/`FallbackGroupIDOnInvalidRequest` 两套机制，同步实现需要额外理清三种兜底机制的优先级关系，工作量与本次范围不匹配，列为后续迭代。 |

**随决策同步的既有风险项处理**（不需要额外拍板，工程上直接采纳）：
- §5风险1（策略性拒绝甄别）：采纳方案 A，引入独立错误哨兵 `ErrGroupCapacityExhausted`。
- §5风险2（同平台校验）：保存时强制要求兜底目标与当前分组同平台。
- §5风险6（渠道映射/订阅资格未重算）、§5风险7（目标分组禁用校验）：均为阶段一必须修复项，非可选。
- §5风险8（流量雪崩熔断）、§5风险13（failover 尝试次数上限）：列为后续迭代项，不纳入阶段一。
- §5风险10（可观测性）、§5风险12（触发频率延迟评估）：纳入阶段一验证清单常规要求。

**对本节决策的连带修订**：
- §3 设计目标表格"防环"一行改为脚注：本次仅支持单跳兜底，不存在链式循环，因此 §3 原表述的"防环"目标在阶段一不适用；如后续升级为链式兜底需重新引入 `visitedGroups` 环检测。
- §4.7 `exhaustedFallbackOptions` 直接复用 `invalidRequestFallbackOptions` 的候选过滤逻辑（排除自身、按平台过滤、候选须未配置自身兜底），不需要图遍历循环检测。
- §4.4 伪代码中的 `visitedGroups` 多跳循环结构可简化为一次性单跳判断（`if !streamStarted && 目标未配置自己的兜底 && 目标平台一致 && 目标已启用`），无需 `for` 循环。
