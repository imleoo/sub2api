# MAAS 重构 — 术语与事实表

> 状态：v1 | 日期：2026-05-15 | 关联：`relay-architecture-design.md`、`generic-channel-design.md`、`upstream-cost-snapshot.md`、`sprint-plan.md`

> **维护规约**：本文是 4 份设计文档的**单一权威源**。所有原子事实（字段名、Phase 编号、工期、protocol 取值、provider 别名表、代码定位锚点）只在本文定义一次；设计文档与 sprint-plan 引用本文，不重复定义。
>
> 新增字段 / 术语 / Phase 时**先改 glossary，再改其他文档**。

---

## 1. 协议与厂商术语

### 1.1 protocol 取值（唯一权威）

- **取值集**：`anthropic_messages` | `openai_chat` | `openai_responses` | `gemini_v1beta`
- **字段位置**：`Group.inbound_protocol` / `Account.outbound_protocol`（Phase 2 引入，详见 §3.3）
- **决议（消除审阅 #7）**：废弃所有短名写法（`"protocol": "openai"` / `"protocol": "anthropic"` / `"protocol": "gemini"`）。设计文档中所有 endpoint 示例、Bridge ID、能力矩阵列名均使用长名。
- **Bridge ID 命名**：`<inbound>-><outbound>`，如 `anthropic_messages->openai_responses`。

### 1.2 platform / provider / vendor 三层概念

| 术语 | 含义 | 取值 |
|---|---|---|
| `platform` | 路由 / 调度 / UI 分类字段（账号、分组通用） | `anthropic` / `openai` / `gemini` / `antigravity` / `lingjing`（Phase 5 可能扩 `generic`） |

> **fork 第 12 项备注**：`lingjing` 已落地 `backend/internal/domain/constants.go:25`，为京东云灵境 Doubao Seedream/Seedance 异步任务平台；**不参与 Phase 5 generic 多 endpoint 改造**（详见 `generic-channel-design.md` §10 排除条款）。
| `vendor` / `provider` | 上游厂商或聚合渠道来源 | `deepseek` / `doubao` / `siliconflow` / `wanjie` 等（开放集合） |
| `provider_key` | provider 经 `normalize_provider` 规范化后的稳定主键 | 同上，但保证小写、去空白、合并别名 |

`platform != protocol != provider`：调度按 platform 走，协议转换按 protocol 走，统计按 provider_key 聚合。

### 1.3 normalize_provider 别名表（唯一权威）

规则：`normalize_provider(s) = strings.ToLower(strings.TrimSpace(s))` + 别名合并。

| 输入（不区分大小写、可带空白） | `provider_key` |
|---|---|
| `DeepSeek` / `deep-seek` / `deepseek-official` | `deepseek` |
| `SiliconFlow` / `silicon-flow` / `硅基流动` | `siliconflow` |
| `Doubao` / `dou-bao` / `豆包` | `doubao` |
| `Kimi` / `moonshot` | `kimi` |
| `Qwen` / `qwen-plus` | `qwen` |
| `Wanjie` / `wan-jie` / `万界方舟` | `wanjie` |
| `Anthropic`（官方） | `anthropic` |
| `OpenAI`（官方） | `openai` |
| `Gemini` / `Google` | `gemini` |

新增 provider 必须先 PR 扩本表，再上线写入路径。

---

## 2. Phase 编号与工期（唯一权威）

- **总 Phase 数**：6 个（Phase 0 + Phase 1–5）
- **总工期**：**15 周**（不含 Phase 5 观察期） / **23 周**（含 Phase 5 的 4 周双桶并存观察）—— 包含 fork 12 项适配 +5 人天（P0-7 lingjing 异步计费 +2、P2-3 修订 +2、P2-6 fork 回归 +1）
- **执行视图**：详细 PR 拆分见 `sprint-plan.md`，本文仅给口径锚点。

### Phase 简表

| Phase | 名称 | 开发周（Sprint 编排） | PR 数 | 关键交付 |
|---|---|---|---|---|
| 0 | 计费基础设施先行 | W1–W2 | **7** | UsageLog 上游成本快照 + `provider_pricing` 表 + **lingjing 异步计费接入（P0-7，fork 12）** |
| 1 | 轻量 Generic + provider_key 规范化 | W3–W6 | 5 | `extra.provider` 写入 + 跨 group 聚合视图 |
| 2 | InboundProtocol / OutboundProtocol 双写 | W5–W8 | **6** | `group.inbound_protocol` + `account.outbound_protocol` + **fork 12 项回归（P2-6）** |
| 3 | Bridge Registry + N×M 矩阵收口 | W9–W12 | 6 | ProtocolBridge 抽象 + 新桥拼装 |
| 4 | Gemini 入站桥 | W13–W14 | 3 | `gemini_v1beta` 入站接入桥矩阵 |
| 5 | Generic 多 Endpoint + Scheduler 重构 | W15–W23 | 6 | `endpoint` 实体 + 双桶并存验证（**排除 lingjing**） |

> **总 PR 数**：33（原 31 + P0-7 + P2-6）。

> 顺序约束：Phase 0/1 schema 正交可并行；Phase 2 起严格串行。

---

## 3. 数据 schema 字段清单（唯一权威）

> 此处仅列字段名与归属 PR；类型细节、迁移脚本、回填策略见各子系统文档。

### 3.1 Phase 0 — UsageLog 新增 7 列（全部可空）

| 字段 | 类型 | 落地 PR | 说明 |
|---|---|---|---|
| `upstream_unit_price_input` | `decimal(20,10)` | P0-2 | 上游 input 单价快照 |
| `upstream_unit_price_output` | `decimal(20,10)` | P0-2 | 上游 output 单价快照 |
| `upstream_unit_price_cache_creation` | `decimal(20,10)` | P0-2 | 上游 cache_creation 单价快照 |
| `upstream_unit_price_cache_read` | `decimal(20,10)` | P0-2 | 上游 cache_read 单价快照 |
| `upstream_total_cost` | `decimal(20,10)` | P0-2 | 上游真实成本快照，NULL = 无上游成本快照 |
| `provider` | `varchar(50)` | P0-2 | 规范化 `provider_key`（见 §1.3） |
| `pricing_source` | `varchar(20)` | P0-2 | `provider_table` / NULL |
| `async_task_id` | `varchar(64)` | P0-7 | 异步任务 gen_task_id（fork 12 lingjing）；同步请求 NULL |
| `cost_finalized_at` | `timestamptz` | P0-7 | 成本最终确定时刻；同步=request_end，异步=poll_runner 触发计费时刻 |

**一致性约束（消除审阅 #4）**：`upstream_total_cost` 与 `pricing_source` 必须**同时 NULL 或同时非空**，由 CHECK 约束或单测断言保证。

**短路约束**（fork 第 8 项 Response Masking）：`gateway_response_masking.go` 命中身份问题时未真正打上游，`upstream_total_cost` 与 `pricing_source` 必须**同时 NULL**，不允许写 0；该行 `actual_cost > 0` 仍记客户售价（masking 假回包按正常请求计费）。

**异步约束**（fork 第 12 项 lingjing）：HTTP 202 返回时该行 `upstream_total_cost=NULL` + `async_task_id!=NULL` + `cost_finalized_at=NULL`；poll_runner 完成后回填 `upstream_total_cost` 非空 + `cost_finalized_at` 非空；轮询超时（240 次后 timeout）保持 NULL，归入"待结算超时"维度。

### 3.2 Phase 1 — UsageLog 索引与快照补充

| 变更 | 落地 PR |
|---|---|
| `platform` 快照字段（如未与 P0-2 合并落地） | P1-x |
| 复合索引 `(provider, created_at)` | P1-x |
| 复合索引 `(account_id, created_at)` | P1-x |

> Phase 1 的 `Account.extra.provider` 写入规范、跨 group 聚合 API 设计见 `generic-channel-design.md`。

### 3.3 Phase 2 — 协议字段双写

| 字段 | 实体 | 落地 PR |
|---|---|---|
| `inbound_protocol` | `Group` | P2-1 |
| `outbound_protocol` | `Account` | P2-1 |

兼容迁移：旧行 `platform` 保留，新字段从 `platform` 推导默认值；写入时双字段必须互恰。

### 3.4 Phase 5 — 多 Endpoint 与统计字段

| 变更 | 实体 | 落地 PR |
|---|---|---|
| `endpoint` 实体（新表） | — | P5-1 |
| `Account` 1→N `Endpoint` 边 | `Account` / `Endpoint` | P5-1 |
| `endpoint_id` 快照 | `UsageLog` | P5-1 |
| `endpoint_protocol` 快照 | `UsageLog` | P5-1 |

---

## 4. Phase 5 多 Endpoint 数据结构（唯一权威）

**决议（消除审阅 #8）**：Phase 5 endpoint **采用数组结构**，便于 ent 1→N 关系与稳定 `endpoint_id`。废弃 map 结构（`"endpoints": { "openai": {...} }`）。

```jsonc
{
  "platform": "generic",
  "type": "apikey",
  "credentials": {
    "api_key": "sk-xxx",
    "provider": "wanjie",
    "endpoints": [
      {
        "id": "wanjie-openai",                              // 稳定 ID,用于 UsageLog 快照
        "protocol": "openai_chat",                          // 取值见 §1.1
        "base_url": "https://maas.example.com/openai/v1",
        "auth": { "header": "Authorization", "scheme": "Bearer" },
        "models": { "source": "remote", "path": "/models" }
      },
      {
        "id": "wanjie-anthropic",
        "protocol": "anthropic_messages",                   // 取值见 §1.1
        "base_url": "https://maas.example.com/anthropic",
        "auth": { "header": "x-api-key", "scheme": "" },
        "models": { "source": "manual", "items": ["claude-sonnet-4-6"] }
      }
    ]
  }
}
```

每个 endpoint 必填：`id`（稳定主键） / `protocol`（§1.1 取值） / `base_url` / `auth` / `models`。

---

## 5. 关键代码定位（锚点）

> 设计文档引用 `file:line` 时统一指向本表；代码移动后只需更新此处。

| 引用名 | 文件:行 | 用途 |
|---|---|---|
| `ForwardAsAnthropic` | `backend/internal/service/openai_gateway_messages.go:28` | Anthropic Messages 入站 → OpenAI Responses 上游桥（Phase 3 P3-1 注册） |
| `ForwardAsResponses` | `backend/internal/service/gateway_forward_as_responses.go:31` | OpenAI Responses 入站 → Anthropic Messages 上游桥（Phase 3 P3-1 注册） |
| `ForwardUpstream` | `backend/internal/service/antigravity_gateway_service.go:4207` | Antigravity Anthropic 透传 |
| `bucketFor` | `backend/internal/service/scheduler_snapshot_service.go:678` | Scheduler 分桶函数（Phase 5 P5-2 双桶并存改造点） |
| `writeUsageLogBestEffort` | `backend/internal/service/gateway_service.go:8260` | UsageLog 写入封装（**不构造字段**，Phase 0 不在此函数改） |
| UsageLog builder（gateway） | `backend/internal/service/gateway_service.go:8641` | UsageLog 字段装配点（Phase 0 P0-5 改造点之一） |
| UsageLog builder（openai） | `backend/internal/service/openai_gateway_service.go:5309` | UsageLog 字段装配点（Phase 0 P0-5 改造点之二） |
| UsageLog builder（usage） | `backend/internal/service/usage_service.go:94` | UsageLog 字段装配点（Phase 0 P0-5 改造点之三） |
| UsageLog builder（lingjing 异步） | `backend/internal/service/lingjing_poll_runner.go` | UsageLog **第 4 个装配点**（fork 12，Phase 0 P0-7 改造点；异步路径不走 `USAGE_UPSTREAM_COST_ENABLED` flag） |
| `apicompat/` | `backend/internal/pkg/apicompat/`（6 实现 + 3 测试 = 9 文件） | Bridge 翻译层（Anthropic↔Responses、ChatCompletions↔Responses、Responses↔Anthropic） |
| `upstream_capability.go` | `backend/internal/pkg/openai_compat/upstream_capability.go` | `ResolveResponsesSupport` 三态模板，Phase 3 P3-6 复用 |
| `account_stats_pricing.go` | `backend/internal/service/account_stats_pricing.go` | 客户售价四级回退链，**Phase 0 不改** |
| `GetAccountStatsAggregated` | `backend/internal/repository/usage_log_repo.go:1791` | Phase 1 跨 group 聚合 API 已有后端,Phase 1 接前端 |
| `routes/gateway.go` 分流判断 | `backend/internal/server/routes/gateway.go` 行 52 / 60 / 76 / 83 / 92 / 142 / 160（共 **7 处**） | Phase 2 P2-3 改造点（**消除审阅 #6**：早期文档误标 8 处含 helper 行 234，实际是 7 处分流 + 1 处 helper） |

### fork 自定义功能锚点（zhiguofan 分支，唯一权威源）

> 详见 `claudedocs/自定义开发功能列表.md`。本表只列出与本期 MAAS 重构 Phase 0/1/2/5 直接相关的 fork 锚点。

| 引用名 | 文件:行 | 用途 |
|---|---|---|
| `PlatformLingjing` | `backend/internal/domain/constants.go:25` | fork 12 第 5 个 platform 常量 |
| `applyDiscount` | `backend/internal/service/billing_service.go` | fork 5 客户售价折扣链，Phase 0 **不动**，与 `upstream_total_cost` 正交 |
| `loadDiscounts` / `GetDiscount` / `GetCNYRate` / `ListAllModels` | `backend/internal/service/pricing_service.go` | fork 5 折扣/汇率/全量模型表 |
| `IsResponseMaskingEnabled` | `backend/internal/service/account.go` | fork 8 masking 开关 |
| `isIdentityQuestion` | `backend/internal/service/gateway_response_masking.go` | fork 8 短路计费触发点 |
| `LingjingPollRunner` | `backend/internal/service/lingjing_poll_runner.go` | fork 12 异步计费 Runner（5s ticker / 10 workers / max 240 次） |
| `lingjingGatewayService.SubmitVideoTask` | `backend/internal/service/lingjing_gateway_service.go` | fork 12 视频任务计费起点 |
| `promptAnalytics middleware` | `backend/internal/plugin/promptanalytics/middleware.go` + `routes/gateway.go:31,45,129,148-179,214,230`（共 **7 处挂载**） | fork 4 中间件，Phase 2 P2-3 **必须保留** |
| `getEntityUsageStats` | `backend/internal/repository/usage_log_repo.go`（私有方法） | fork 3 跨实体聚合范式，Phase 1 P1-2 **必须复用** |
| `/lingjing/v1/video/*` 路由组 | `backend/internal/server/routes/gateway.go:193-203` | fork 12 lingjing 异步任务专用路由（含 ForcePlatform middleware L198，**第 4 处 ForcePlatform**） |

---

## 6. 平台常量扩散面（Phase 2 规模估算）

`Platform{Anthropic,OpenAI,Gemini,Antigravity,Lingjing}` 常量当前命中：

- 非测试 Go 文件：约 48 个（不含 fork 12 lingjing 新增的 5 个 service 文件 + 1 个 handler + 1 个 repository + 1 个 schema）
- 含测试：约 166 个

> 数字会随提交波动，以 `grep -rl 'Platform\(Anthropic\|OpenAI\|Gemini\|Antigravity\|Lingjing\)' backend/internal/ backend/cmd/` 实测为准。
>
> **当前 5 个 platform**（`anthropic/openai/gemini/antigravity/lingjing`，fork 12 引入），Phase 2 改 `routes/gateway.go` 时 `getGroupPlatform()` 下游条件实际 **~10-11 处**：
> - 7 处 platform 分流（行 52/60/76/83/92/142/160）
> - L168 `/v1/images/generations` 的 lingjing 平台分支（`!= PlatformOpenAI && != PlatformLingjing`，fork 12）
> - 4 处 ForcePlatform middleware（L198/216/232/256，其中 L198 是 fork 12 lingjing 第 4 处）
>
> 此处仅用于 Phase 2 改造规模评估。

---

## 7. 文档关系

```
docs/
├── README.md                          导航
├── glossary.md                        ← 本文，单一权威源
├── relay-architecture-design.md       顶层架构（设计原理 + 桥矩阵）
├── generic-channel-design.md          Generic Channel 子系统设计
├── upstream-cost-snapshot.md          上游成本快照子系统设计
└── sprint-plan.md                     31 PR 颗粒度执行视图
```

设计文档负责"为什么这么设计"；本文负责"事实是什么"；sprint-plan 负责"PR 怎么拆"。三者职责正交。
