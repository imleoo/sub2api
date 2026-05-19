# 通用渠道设计方案

> 状态：草稿 v2 | 日期：2026-05-14 | 依据：当前 `zhiguofan` 分支源码
>
> 修订：本文 §4–§5 的 Phase 1 与 `docs/relay-architecture-design.md` Phase 1 对齐；§6 原 "Phase 2 多 endpoint 账号" **重新定位为全局 Phase 5**，因其依赖 scheduler snapshot 桶维度重构、双桶并存验证，与协议矩阵推进顺序正交。Phase 编号全局以 `relay-architecture-design.md` §7 为准。

---

## 1. 目标与边界

通用渠道用于承载 Anthropic、OpenAI、Gemini、Antigravity 内置平台之外的上游来源，包括：

- **原厂兼容接口**：DeepSeek、豆包、Kimi、Qwen 等有独立厂商身份，但对外提供 OpenAI-compatible 或其他兼容协议。
- **非原厂聚合平台**：硅基流动、万界方舟等，一个账号可能代理多个厂商、多个模型或多个协议。

本设计的核心边界是：

```text
vendor/provider != protocol
```

`deepseek`、`doubao`、`siliconflow`、`wanjie` 是厂商或渠道来源；`openai`、`anthropic`、`gemini`、`openai_responses` 才是协议能力。调度和协议转换必须按协议能力执行，不能只按厂商名称执行。

---

## 2. 当前源码事实

### 2.1 当前没有 `generic` 平台常量

后端平台常量当前只有：

```go
PlatformAnthropic   = "anthropic"
PlatformOpenAI      = "openai"
PlatformGemini      = "gemini"
PlatformAntigravity = "antigravity"
PlatformLingjing    = "lingjing"  // fork 第 12 项，京东云灵境 Doubao Seedream/Seedance
```

> `PlatformLingjing` 是**专有异步任务平台**（Doubao Seedream 图同步、Seedance 视频异步），**不属于 generic 范畴**；详见本文 §10 排除条款。

前端类型 fork 已枚举 `'lingjing'` 与 `'antigravity'`（fork 12 引入 `frontend/src/types/index.ts`），账号创建表单已有 lingjing 入口，但**没有通用渠道入口**。直接写入 `platform=generic` 虽然数据库能保存，但路由、调度、模型同步和前端展示不会自动可用。

### 2.2 数据库存储不是主要阻塞点

`account.platform` 和 `group.platform` 都是字符串字段，不是数据库 enum。账号还有 `credentials` 和 `extra` JSON 字段，可以先承载厂商元信息。

主要阻塞在业务链路：

- 路由层仍按 `Group.Platform` 分流 handler。
- OpenAI handler 调度固定筛 `PlatformOpenAI` 账号。
- Anthropic/Gemini 主链路按 group/platform 和混合策略筛账号。
- scheduler snapshot 也按固定平台桶构建。
- `/v1/responses` 能力探测只针对 `platform=openai && type=apikey`。

因此，第一阶段不能把 DeepSeek、豆包、硅基流动这类 OpenAI-compatible 渠道改成 `platform=generic`，否则会绕开现有可用链路。

### 2.3 已有 OpenAI-compatible 能力

当前代码已有 `openai_compat` 能力判断，用于处理第三方 OpenAI-compatible 上游：

- 探测 `/v1/responses` 是否可用。
- 如果不支持 Responses，则直接走 Chat Completions。
- 根据 `credentials.base_url` 构造上游地址。

这说明 DeepSeek、豆包、Kimi、Qwen、硅基流动这类提供 OpenAI-compatible API 的渠道，Phase 1 应优先复用 `platform=openai`。

---

## 3. 术语定义

`platform` / `protocol` / `vendor` / `provider_key` 的定义与取值见 `glossary.md` §1.1–§1.3，本文档不重复。仅本节特有的两个术语：

| 名称 | 含义 |
|---|---|
| `endpoint` | 一个具体可请求的上游入口（`base_url + path + auth`），Phase 5 数据结构见 `glossary.md` §4 |
| `generic channel` | 内置平台之外的厂商/聚合渠道抽象；**不是第四种入站协议** |

**Account ↔ Group 关系约束**：

`Account` 与 `Group` 在 `backend/ent/schema/account_group.go` 定义为多对多（含 `priority` 字段）。**一个上游渠道账号被多个分组复用是常态，而非异常**。因此本文档涉及的所有计费、用量、统计视图必须满足：按账号或按 `provider_key` 的汇总不被 `group_id` 切分；账号维度与厂商维度的总额等于该账号/厂商在所有分组下行级 UsageLog 的逐行求和。

---

## 4. Phase 1：不新增 `platform=generic`

### 4.1 实施原则

Phase 1 的目标是让通用渠道先可用、可测试、可统计，不改造整个网关调度体系。

```text
OpenAI-compatible 通用渠道
  -> platform=openai
  -> type=apikey
  -> credentials.base_url 指向厂商或聚合平台
  -> extra/credentials 记录 vendor/provider
```

这条路线复用现有能力：

- OpenAI handler。
- OpenAI APIKey 调度。
- `/v1/responses` 探测。
- Chat Completions fallback。
- 现有模型白名单、模型映射、usage 记录。

### 4.2 推荐账号表示

> `protocol` 取值见 `glossary.md` §1.1；`provider` 取值见 `glossary.md` §1.3 别名表。

```jsonc
{
  "platform": "openai",
  "type": "apikey",
  "credentials": {
    "api_key": "sk-xxx",
    "base_url": "https://api.deepseek.com",
    "provider": "deepseek"
  },
  "extra": {
    "provider": "deepseek",
    "provider_type": "official_compatible",
    "protocol": "openai_chat",
    "models_source": "remote"
  }
}
```

聚合平台的 OpenAI-compatible endpoint 也按同样方式表示：

```jsonc
{
  "platform": "openai",
  "type": "apikey",
  "credentials": {
    "api_key": "sk-xxx",
    "base_url": "https://api.siliconflow.cn/v1",
    "provider": "siliconflow"
  },
  "extra": {
    "provider": "siliconflow",
    "provider_type": "aggregator",
    "protocol": "openai_chat",
    "models_source": "remote"
  }
}
```

### 4.3 多协议聚合平台的 MVP 表示

如果一个聚合平台同时暴露 OpenAI、Anthropic、Gemini endpoint，Phase 1 不做“一个账号多协议”。应拆成多个协议账号：

| 账号名称 | platform | base_url | provider |
|---|---|---|---|
| 硅基流动 OpenAI | `openai` | OpenAI-compatible endpoint | `siliconflow` |
| 某聚合 Anthropic | `anthropic` | Anthropic-compatible endpoint | `vendor_x` |
| 某聚合 Gemini | `gemini` | Gemini-compatible endpoint | `vendor_x` |

这样可以复用现有 handler 和调度器，避免 Generic 账号绕过平台桶后无法被选中。

### 4.4 provider 字段规范化

`normalize_provider` 函数与完整别名表见 `glossary.md` §1.3（**单一权威源**）。

本节仅强调 Phase 1 实施约束：

- 前端预设的 provider 标签为权威值；自定义 base_url 允许填新 provider，但保存前后端均执行规范化。
- 所有按 provider 的聚合 API 仅接受规范化后的 `provider_key`，不接受未规范化字符串。
- 别名表由后端维护并持久化为常量，**新增 provider 必须先 PR 扩 `glossary.md` §1.3，再上线写入路径**。

---

## 5. Phase 1 改造清单

### 5.1 后端

1. **UsageLog schema 快照**：`provider` 字段（规范化后的 `provider_key`）由 **Phase 0 落地**（详见 `docs/upstream-cost-snapshot.md` §2.1 与 `docs/sprint-plan.md` P0-2），本 Phase **不再重复新增 `provider` 列**，避免 ent codegen 与迁移脚本双写冲突。本 Phase 仅在 `backend/ent/schema/usage_log.go` 补：(a) `platform` 快照字段（如未与 P0-2 合并落地）；(b) `(provider, created_at)`、`(account_id, created_at)` 两个复合索引，避免账号/厂商维度查询退化为 `(group_id, created_at)` 索引的次选项。写入路径仍遵循"快照同行、不 join account 反推"的原则，与 P0-2 的 `account_rate_multiplier` 快照模式一致。
2. **provider 元信息读写规范**：前端入参写 `extra.provider`，后端写 UsageLog 前调用 §4.4 的 `normalize_provider`；`credentials.provider` 仅作历史兼容读路径，新写入不再使用。账号创建、测试连接、模型同步、统计展示均透出 `provider_key`。
3. **新增聚合 API**：
   - `GetStatsByProvider(ctx, timeRange, filters)`：按 `provider_key` 聚合 token/cost/请求数。
   - `GetAccountStatsCrossGroup(ctx, account_id, timeRange)`：按账号跨 group 全量汇总。
   - 上述两个 API **不接受 `group_id` 为强制过滤条件**，可选用作辅助筛选。
   - `GetAccountStatsAggregated`（`backend/internal/repository/usage_log_repo.go:1791-1823`）后端已有但前端未接，本期接入。
4. **返回结构透明化 m2m 关系**：按账号/按厂商的统计接口，返回结构显式包含 `group_ids: []string` 字段，让管理员一眼看出该账号/该厂商被多少分组复用。
5. **平台与协议约束（不变更项）**：OpenAI-compatible provider 继续走 `PlatformOpenAI`；不新增 `PlatformGeneric` 常量；不新增 `Group.InboundProtocol` 或 `Account.OutboundProtocol` 作为 Phase 1 强依赖。

### 5.2 前端

在 OpenAI APIKey 账号创建表单中增加“渠道预设”：

| 预设 | platform | 默认 base_url | 说明 |
|---|---|---|---|
| OpenAI | `openai` | `https://api.openai.com` | 官方 OpenAI |
| DeepSeek | `openai` | `https://api.deepseek.com` | OpenAI-compatible |
| 豆包 | `openai` | 火山/豆包兼容 endpoint | OpenAI-compatible |
| 硅基流动 | `openai` | `https://api.siliconflow.cn/v1` | 聚合平台 |
| 自定义 | `openai` | 用户填写 | 其他兼容渠道 |

表单保存时写入：

- `credentials.base_url`
- `credentials.api_key`
- `extra.provider`（保存前后端均执行 §4.4 规范化）
- `extra.provider_type`
- `extra.protocol = "openai"`

**不复活 OAuth UI**（fork 第 11 项约束）：fork 已从 `CreateAccountModal.vue` / `EditAccountModal.vue` 删除所有 OAuth 相关 handler（`handleOpenAIExchange` / `handleAnthropicExchange` / `handleGeminiExchange` / `OPENAI_MOBILE_RT_CLIENT_ID` 等）。Phase 1 P1-3 加 Provider 预设下拉时，仅暴露 **API Key / Setup Token** 两种入口；后端 `ForwardAsAnthropic` 内部的 OAuth codex 伪装链（`applyCodexOAuthTransformWithOptions`）照常工作，与前端 UI 入口解耦。

统计页面必须在 `frontend/src/components/charts/` 下增加两个与 `GroupDistributionChart` 同级的视图：

| 视图 | 聚合维度 | 绑定 API | 关键 UI 元素 |
|---|---|---|---|
| `ProviderDistributionChart` | `provider_key` | `GetStatsByProvider` | 厂商卡片显示 token/cost/请求数，下钻为该厂商下的账号列表 |
| `AccountDistributionChart` | `account_id` | `GetAccountStatsCrossGroup` / `GetAccountStatsAggregated` | 账号卡片必须显示"覆盖分组数 = N"，提示 m2m 复用关系 |

三视图并列切换，**不得把 Account / Provider 视图嵌在某个 Group 详情页之下**，避免视觉上把渠道账单"挂在分组之下"。

### 5.3 模型同步

模型同步策略按 provider 决定默认行为：

- `remote`：调用 OpenAI-compatible `/models`。
- `manual`：前端手动填写模型。
- `static_preset`：使用内置推荐模型列表。

不要假设所有 provider 都支持 `/v1/models`，也不要假设聚合平台返回的模型名一定等于最终请求模型名。

### 5.4 统计与诊断（Phase 1 硬要求）

**统计口径承诺**：

1. **UsageLog 必须快照**：`provider`（规范化后的 `provider_key`）、`account_id`、`upstream_model`、`mapped_model`、`platform`，不依赖运行时 join account 反推。`account_rate_multiplier`（`backend/ent/schema/usage_log.go:105-108`）已是快照模式，本期 provider/platform 保持同一原则。
2. **三个一级聚合维度并存**：Group / Account / Provider。Account 与 Provider 维度的聚合查询**不得以 `group_id` 为必选过滤条件**——账号通过 `account_groups` 中间表归属多个 Group 时，账号总账与厂商总账是跨 group 的全量加总，不是各 group 的局部和。
3. **账号汇总等式**：当账号被多个 group 复用，按 Account 维度返回的 token/cost 必须等于该账号在所有 group 下行级 UsageLog 之和；前后端对账测试为 Phase 1 必跑项。
4. **厂商汇总等式**：同一 `provider_key` 下若挂多个 Account（如同一管理员开了两把 DeepSeek key），其总账等于这些 Account 总账之和，与 Account 所属 group 数量无关。
5. **历史快照独立性**：删除或停用 Account、修改 `extra.provider` 不会擦除或回写历史 UsageLog 的 `provider`/`account_id`/`platform`，保证历史账单可追溯。
6. **异步任务汇总等式**（fork 第 12 项 lingjing）：lingjing 视频任务的 token/cost 总账 = `lingjing_task` 表中所有已 finalized 任务的 `actual_cost` 之和 = UsageLog 中 `async_task_id IS NOT NULL` 且 `upstream_total_cost IS NOT NULL` 行的求和。两路对账每日 ≤0.1% 差异；超出阈值触发告警。lingjing 异步任务**不参与同步路径的 7 天 shadow 对账**（详见 `upstream-cost-snapshot.md` §4.2）。

---

## 6. Phase 5：真正 Generic 多 endpoint 账号

> 原 "Phase 2"。重新编号为 Phase 5（对齐 `relay-architecture-design.md` §7），因为此阶段必须在 Phase 0（计费快照）/ Phase 1（轻量 Generic）/ Phase 2（协议字段双写）/ Phase 3（Bridge Registry）/ Phase 4（Gemini 入站桥）全部完成后才能稳定推进；scheduler 双桶并存验证是关键前置。

只有当业务明确需要"一个账号内管理多个协议 endpoint"时，再新增 `platform=generic`。

### 6.1 目标结构

完整数据结构定义见 `glossary.md` §4（**单一权威源**）。本节仅记录 Phase 5 的设计意图：

- 一个 `platform=generic` 账号挂多个 endpoint，每个 endpoint 有稳定 `id` 用于 UsageLog 快照。
- 每个 endpoint 独立持有 `protocol`（取值见 `glossary.md` §1.1）、`base_url`、`auth`、`models` 配置。
- 同一 `provider`（如万界方舟）下的多协议 endpoint 在一个账号内表达，避免拆账号管理多个 base_url。

### 6.2 必须同步改造的链路

如果引入 `PlatformGeneric`，必须同时完成：

- domain 常量和前端类型增加 `generic`。
- 账号创建表单支持 endpoint 列表、鉴权头策略、模型来源。
- 账号测试连接按 endpoint protocol 分发到对应测试器。
- 调度器从“按 `account.platform` 筛选”改为“按 endpoint protocol + feature + model 筛选”。
- scheduler snapshot 增加 generic endpoint bucket，或从平台桶重构为协议桶。
- sticky 命中后重新校验 endpoint protocol、模型和请求特性。
- failover 重选时保留同一份 `RequestFeatures`，避免选到有损 endpoint。
- usage、error、availability、cost 统计能定位 provider、endpoint、protocol。

只新增 `PlatformGeneric` 而不改这些链路，会得到一个能保存但无法稳定调度的账号类型。

### 6.3 多 endpoint 计费一致性

UsageLog 在 Phase 5 必须增加 `endpoint_id` 与 `endpoint_protocol` 快照字段，并约束汇总规则：

- **Account 维度汇总** = 该 account 下所有 endpoint 的逐行求和。
- **Provider 维度汇总** = 该 `provider_key` 下所有 account × endpoint 的逐行求和。
- **Endpoint 维度独立可查**：必须额外支持 `GetStatsByEndpoint(account_id, endpoint_id, timeRange)`，作为账号维度的下钻视图。
- 三类汇总均独立于请求所在 group，复用 §5.4 第 2 条的"不得以 `group_id` 为必选过滤"约束。
- 修改 endpoint `base_url` 或删除 endpoint 不影响历史 UsageLog 的 `endpoint_id`/`endpoint_protocol` 快照，与 §5.4 第 5 条保持同一原则。

---

## 7. 与协议桥的关系

Generic Channel 不是 Bridge 的替代品。它只描述“有哪些可用上游 endpoint”；Bridge 负责“当入站协议和出站协议不一致时如何转换”。

Phase 1 规则：

```text
inbound protocol == account platform/protocol -> 直接走现有 handler
inbound protocol != account platform/protocol -> 不自动跨协议
```

后续支持 Anthropic 入站转 OpenAI-compatible 上游时，应复用现有 `ForwardAsAnthropic` 方向：

```text
Anthropic Messages 入站
  -> OpenAI Responses 上游
  -> Anthropic Messages 响应
```

不能把它误写成 OpenAI 入站转 Anthropic 出站。

---

## 8. 验收用例

Phase 1 验收：

1. DeepSeek APIKey 可作为 OpenAI-compatible 账号创建，保存 provider 元信息。
2. DeepSeek 账号可完成连接测试或模型同步，不要求支持 `/v1/responses` 时能 fallback 到 Chat Completions。
3. 硅基流动 OpenAI-compatible endpoint 可作为 OpenAI 账号创建，并能按 provider 展示。
4. 自定义 OpenAI-compatible base_url 不会被误保存为 `platform=generic`。
5. 原有 OpenAI 官方账号不受 provider preset 影响。
6. **跨分组账号汇总一致性**：创建 DeepSeek 账号 `A` 通过 `account_groups` 同时挂到 `group_X` 与 `group_Y`，两组各发起 N 次和 M 次请求后，按 Account 维度查询返回的 token/cost 等于 N+M 次请求的逐行求和；查询调用栈中**不出现以 `group_id` 为必选过滤的语句**。
7. **跨分组 provider 汇总**：在用例 6 基础上再加一个 provider 同为 `deepseek` 的第二把 key `A'`（挂到 `group_Z`），按 `provider_key=deepseek` 查询的总额等于 `A + A'` 全部行级汇总。
8. **provider 规范化**：分别以 `DeepSeek`、`deep-seek`、`deepseek` 创建/编辑账号，UsageLog 与统计 API 返回的 `provider_key` 均为 `deepseek` 单一桶，不出现三个并列分组。
9. **历史快照独立性**：用例 6 完成后，把账号 `A` 的 `extra.provider` 改成 `doubao`，再删除账号 `A`，历史 UsageLog 与按 Account / Provider 查询返回的口径不变。
10. **前端入口验证**：统计页同级展示 Group / Account / Provider 三个视图；Account 卡片显示"覆盖分组数 = 2"；Account / Provider 视图不嵌套在任何 Group 详情之下。

Phase 5 验收：

1. `platform=generic` 账号的每个 endpoint 都能单独测试连接。
2. 调度器只选择与入站协议兼容的 endpoint。
3. sticky 命中不兼容 endpoint 时会跳过并重新选路。
4. failover 不会把带 `document`、`computer_use`、`citations` 的请求切到不兼容 endpoint。
5. usage 记录能定位 provider、endpoint、protocol。
6. Scheduler 双桶并存 4 周期间，新旧分配结果一致率 ≥99%（埋点对比报警）。

---

## 9. 决策

本仓库不恢复旧版 `generic-channel-design.md`。当前文件作为新的通用渠道设计基线：

- Phase 1（全局阶段编号）：不新增 `platform=generic`，先用 provider 元信息增强现有 OpenAI-compatible 接入；新增 Account/Provider 跨 group 聚合视图。
- Phase 5（全局阶段编号）：在 Phase 0 计费快照、Phase 2 协议字段双写、Phase 3 Bridge Registry、Phase 4 Gemini 桥就绪后，引入真正 Generic 多 endpoint 账号，并完成 scheduler snapshot 双桶并存验证。
- 详细阶段路线与依赖见 `docs/relay-architecture-design.md` §7。

---

## 10. lingjing 平台与 Generic 边界（fork 第 12 项排除条款）

`PlatformLingjing`（fork 第 12 项）虽然是内置 4 平台之外的"第 5 平台"，但**不归入 generic 范畴**。理由：

1. **异步任务制**：Seedance 视频任务通过 HTTP 202 + taskId + 后台 poll_runner（5s ticker / 10 workers / max 240 次轮询）完成计费，与 generic 设计的"同步 endpoint 选择 + Bridge 转换"模型不兼容。
2. **专用路由组**：`/lingjing/v1/video/*` 是独立路由（`backend/internal/server/routes/gateway.go:193-203`），不复用 `/v1/messages` / `/v1/chat/completions` / `/v1/responses` 入口；自带第 4 处 ForcePlatform middleware（L198）。
3. **专用 service 链**：`lingjing_client.go` / `lingjing_gateway_service.go` / `lingjing_images.go` / `lingjing_poll_runner.go` / `lingjing_task_port.go` 共 5 个独立 service 文件，不走通用 gateway service。
4. **独立 ent 实体**：`backend/ent/schema/lingjing_task.go` 持久化任务状态（gen_task_id、status、result_url、billing 字段等），generic 不需要这类持久化。
5. **混合调度**：`/v1/images/generations` 的 Seedream 同步生图复用 OpenAI 路由（`routes/gateway.go:168` 加 `lingjing` 平台分支）；Seedance 视频走专用路由。这种"复用+专用"混合模式不应被 generic 多 endpoint 抽象吞掉。

### 10.1 Phase 5 实施约束

引入 `platform=generic` 多 endpoint 账号时：

- **P5-1 schema 迁移**：endpoint 实体迁移脚本在 `WHERE platform != 'lingjing'` 范围内执行，lingjing 账号保持单 endpoint 派生即可。
- **P5-2 scheduler 双桶并存**：lingjing 不进入双桶对比（异步任务无 endpoint 概念）。
- **P5-3 sticky_session key**：lingjing 不需要 sticky（异步任务通过 `gen_task_id` 关联，与 endpoint 无关）。
- **P5-4 前端表单**：lingjing 表单保持现状（fork 第 12 项已实现 `CreateAccountModal.vue` 的 lingjing 入口），不复用 generic 多 endpoint 列表表单。

### 10.2 与上游成本快照的协调

虽然 lingjing 不归 generic，但 **Phase 0 P0-7 仍负责** lingjing 异步计费接入 `upstream_total_cost`（见 `upstream-cost-snapshot.md` §4.1）。这是计费侧的统一接入，与"是否属于 generic"无关：

- 计费层：lingjing **进入** `upstream_total_cost` / `provider_pricing` / 毛利视图统一体系（fork 12 第 12 项 + P0-7）。
- 调度层：lingjing **不进入** generic 多 endpoint / Bridge Registry / scheduler 协议桶（本节 §10.1）。

两个维度正交，互不依赖。


