# 上游成本快照设计方案

> 状态：草稿 v1 | 日期：2026-05-14 | 依据：当前 `zhiguofan` 分支源码 | 关联：`docs/relay-architecture-design.md` §7 Phase 0、`docs/generic-channel-design.md` §5.4

---

## 1. 背景与问题

当前 UsageLog 只记录用户侧售价（`actual_cost`），不记录上游真实成本。这导致：

- 无法在请求维度回答"这次调用我们赚了多少？亏了多少？"
- 无法做账号/Provider 维度的毛利率分析。
- 无法识别"被滥用的低价模型映射"或"账号倍率配置错误导致的赔本"。
- 运营调价（涨价/降价）只能依赖经验，无数据支撑。

**MAAS 中转商商业模式的核心字段是"上游成本"**。它必须与 `actual_cost` 在同一行 UsageLog 中可见、可对账、可聚合。

### 现状基线

- `backend/ent/schema/usage_log.go:32-146` 已有字段：`total_cost`（客户侧未应用倍率前）、`actual_cost`（已应用倍率）、`account_rate_multiplier`、`rate_multiplier`、`requested_model`、`upstream_model`、`model_mapping_chain`、`billing_mode`、`channel_id`、`group_id`。
- `backend/internal/service/account_stats_pricing.go` 已有"账号侧成本估算"四级回退链：自定义规则 / 客户计费 / LiteLLM 模型表 / `total_cost × account_rate_multiplier`。**但这是估算，不是上游真实账单快照**。
- 已有 `account_rate_multiplier` 字段可作为"上游倍率系数"的折算手段；但语义模糊，不能替代显式"上游单价 × token 数 = 上游成本"。

### 设计原则

- 上游成本与售价**双轨快照**，写入同一行，事后不可被账号配置或价表变更回溯改写。
- **不破坏现有字段**：`total_cost` / `actual_cost` / `account_rate_multiplier` 保留原语义；现有 `account_stats_pricing.resolveAccountStatsCost()` 链路是**客户/账号统计估算**，本期不动。
- **`upstream_total_cost` 是严格的真实成本快照，不是估算**：只有从 `provider_pricing` 表精确命中时才写入；LiteLLM、自定义规则、默认公式等其他来源**不写入** `upstream_total_cost`，避免精度不可信的数据流入运营对账。
- **`pricing_source` 字段语义跟随 `upstream_total_cost`**：
  - 命中 `provider_pricing` 表 → `upstream_total_cost` 非空 + `pricing_source = "provider_table"`
  - 未命中 → `upstream_total_cost = NULL` 且 `pricing_source = NULL`（统计页据此打"无上游成本快照"标签）
- **历史数据兼容**：旧行两者都为 NULL 是合法状态。

---

## 2. 数据模型变更

### 2.1 UsageLog 新增字段（全部可空）

修改 `backend/ent/schema/usage_log.go`，在现有"成本字段"块下方新增：

```go
// 上游真实成本快照（Phase 0 引入）
// NULL 表示无上游成本快照（provider_pricing 未命中），统计页须显式标注"无上游成本快照"
// 并从毛利聚合中排除；**不允许混入 LiteLLM/fallback 估算或显示为"估算"标签**。
field.Float("upstream_unit_price_input").
    Optional().Nillable().
    SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
field.Float("upstream_unit_price_output").
    Optional().Nillable().
    SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
field.Float("upstream_unit_price_cache_creation").
    Optional().Nillable().
    SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
field.Float("upstream_unit_price_cache_read").
    Optional().Nillable().
    SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
field.Float("upstream_total_cost").
    Optional().Nillable().
    SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}).
    Comment("上游真实成本快照；NULL 表示无上游单价"),

// Provider 规范化键快照（Phase 0 引入）
// 与 `generic-channel-design.md` §4.4 normalize_provider 规则一致
field.String("provider").
    MaxLen(50).Optional().Nillable().
    Comment("规范化的 provider_key，如 anthropic/openai/deepseek/siliconflow"),

// 计价来源标签（Phase 0 引入；与 upstream_total_cost 严格同步：要么同时非空、要么同时 NULL）
field.String("pricing_source").
    MaxLen(20).Optional().Nillable().
    Comment("provider_table（命中 provider_pricing 表）；预留 upstream_billing（未来接上游账单 API）；NULL 表示无上游成本快照，不允许写入 litellm/fallback 等估算来源"),
```

**不新增索引**：写入热表加索引代价高，等 Phase 1 跨 group 聚合视图上线后视查询模式再加。

### 2.2 新增 `provider_pricing` 实体

新建 `backend/ent/schema/provider_pricing.go`：

```go
type ProviderPricing struct {
    ent.Schema
}

func (ProviderPricing) Fields() []ent.Field {
    return []ent.Field{
        field.String("provider").MaxLen(50).NotEmpty().
            Comment("规范化 provider_key"),
        field.String("model").MaxLen(100).NotEmpty().
            Comment("上游模型 ID（精确名或带 * 通配符）"),
        field.String("billing_mode").MaxLen(20).Default("token").
            Comment("token | per_request | image"),

        // 单价（每 token 价格；per_request 计价时只看 input_price）
        field.Float("input_price").Default(0).
            SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
        field.Float("output_price").Default(0).
            SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
        field.Float("cache_creation_price").Default(0).
            SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
        field.Float("cache_read_price").Default(0).
            SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),

        field.String("currency").MaxLen(8).Default("USD"),

        // 价格生效区间（支持调价历史追溯）
        field.Time("effective_from").Default(time.Now).
            SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
        field.Time("effective_to").Optional().Nillable().
            SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

        field.String("source").MaxLen(50).Default("manual").
            Comment("manual | litellm_sync | upstream_billing_api"),

        field.Time("created_at").Default(time.Now).Immutable().
            SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
        field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).
            SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
    }
}

func (ProviderPricing) Indexes() []ent.Index {
    return []ent.Index{
        // 命中查询：给定 (provider, model, now()) 找当前生效价格
        index.Fields("provider", "model", "effective_from"),
    }
}
```

### 2.3 历史数据迁移

由后台离线 job 完成（不阻塞发布）：

1. 旧行 `provider` 字段：根据 `account_id` 反向 join `accounts.platform`，按 `normalize_provider` 规则规范化后填入。
2. 旧行 `upstream_total_cost` 与 `pricing_source`：**两者均保留 NULL**，不补算。统计页对 `upstream_total_cost = NULL` 的行明确打"无上游成本快照"标签并从毛利聚合中排除（不再混入 `account_stats_pricing` 的估算到上游成本字段）。
3. 分批回填脚本：单批 ≤10000 行，分批间 sleep 500ms，避免锁表。

---

## 3. 上游成本计算与回退策略（两态）

### 3.1 严格的两态语义

`upstream_total_cost` 与 `pricing_source` 是**两态**字段，不是估算回退链：

| `provider_pricing` 是否命中 | `upstream_total_cost` | `pricing_source` | 含义 |
|---|---|---|---|
| 命中（精确或通配，且 `effective_from ≤ now < effective_to`） | 非空（按命中单价计算） | `"provider_table"` | 真实上游成本快照 |
| 未命中 | NULL | NULL | 无上游成本快照；统计页打"无快照"标签并从毛利聚合排除 |

**绝不把 LiteLLM、自定义规则、默认公式的估算写入 `upstream_total_cost`**。LiteLLM 公开价表对 DeepSeek/豆包等第三方渠道未必准确；自定义规则与默认公式是为"账号统计估算"设计，不是上游真实成本。让运营看到"看似真实但实际可能错"的成本，会比看到 NULL 更糟。

### 3.2 与现有 `account_stats_pricing.go` 的关系

现有 `resolveAccountStatsCost()` 四级链**保持不变**，它是给 `actual_cost`（客户售价）与运营内部账号统计用的回退估算链：

- 自定义规则（`channel.AccountStatsPricingRules`）
- 渠道开关 `ApplyPricingToAccountStats`（直接用客户计费）
- LiteLLM 模型表
- 默认公式 `total_cost × account_rate_multiplier`

Phase 0 **不改这条链**，只在它**外侧**新增一个独立的 `resolveUpstreamCost()`，专门负责 `upstream_total_cost` 与 `pricing_source` 写入：

```go
// resolveUpstreamCost 严格的两态：命中返回 (cost, "provider_table")，未命中返回 (nil, "")。
// 不做任何形式的回退估算。
func resolveUpstreamCost(
    ctx context.Context,
    providerPricingRepo *ProviderPricingRepository,
    provider, upstreamModel string,
    tokens UsageTokens,
) (cost *float64, source string) {
    if provider == "" || upstreamModel == "" {
        return nil, ""
    }
    pricing, err := providerPricingRepo.FindEffective(ctx, provider, upstreamModel, time.Now())
    if err != nil || pricing == nil {
        return nil, ""
    }
    c := float64(tokens.InputTokens)*pricing.InputPrice +
        float64(tokens.OutputTokens)*pricing.OutputPrice +
        float64(tokens.CacheCreationTokens)*pricing.CacheCreationPrice +
        float64(tokens.CacheReadTokens)*pricing.CacheReadPrice
    if c <= 0 {
        return nil, ""
    }
    return &c, "provider_table"
}
```

### 3.3 未来扩展（**不在 Phase 0 范围**）

如果将来要把"上游真实账单 API"（如 Anthropic Console Usage、OpenAI Usage API）接入，可作为 `pricing_source = "upstream_billing"` 的第二来源加入两态体系——它依然是"真实成本"，不是"估算"。这是 Phase 0 之后的独立工程，本文档不展开。

---

## 4. 写入路径

### 4.1 双轨写入

`backend/internal/service/gateway_service.go:8260-8283` 的 `writeUsageLogBestEffort` 在构造 UsageLog 时，同时计算：

- **售价侧（不变）**：现有客户计费链（`resolveAccountStatsCost` 四级回退）产出 `total_cost` 与 `actual_cost`。
- **成本侧（新增，两态）**：调用 `resolveUpstreamCost(ctx, provider, upstreamModel, tokens)`：
  - 命中：写入 `upstream_total_cost` + 四个 `upstream_unit_price_*` 单价字段 + `pricing_source = "provider_table"`
  - 未命中：以上字段全部 NULL（不回退到 LiteLLM/估算）

两轨**独立赋值**，不互相干扰。任一计算失败仅影响该轨字段为 NULL。

### 4.2 Shadow Write 阶段

Phase 0 发布的第一周走 shadow write：

- 字段已上线、写入路径已激活，但运营页不读这些字段（仍用旧估算展示）。
- 后台对账 job 每天对比：仅对 `upstream_total_cost` 非空的行，对比它与 `resolveAccountStatsCost()` 估算的差异，按 provider/account/model 维度产出报告。
- 差异 ≥10% 的 provider 单独标注，运营审核单价表后再切展示侧。
- `upstream_total_cost = NULL` 的行**不进入差异对账**，因为缺少快照值不是误差而是"无快照"。

### 4.3 切换展示

Shadow 验证 24h 后，运营页改读 `upstream_total_cost`：

- 非空行：用快照值参与毛利计算
- NULL 行：UI 明确标注"无上游成本快照"，从毛利聚合中排除，**不混用 `account_stats_pricing` 估算到上游成本字段**（避免精度不可信的数据流入对账）

---

## 5. 统计与视图

### 5.1 毛利率字段（视图层计算，不入库）

```sql
margin       = actual_cost - upstream_total_cost
margin_ratio = margin / NULLIF(upstream_total_cost, 0)
```

`upstream_total_cost = NULL` 的行**跳过毛利聚合**，运营页 UI 必须显式标注"无上游成本快照"行的数量与占比，避免运营误读为"零成本"。

### 5.2 新增 Repository 查询

`backend/internal/repository/usage_log_repo.go` 新增：

- `GetMarginByProvider(ctx, timeRange)`：按 provider 聚合 actual_cost / upstream_total_cost / margin
- `GetMarginByAccount(ctx, timeRange)`：按 account 聚合（跨 group，**禁止 group_id 强过滤**）
- `GetMarginByModel(ctx, timeRange)`：按上游模型聚合，识别赔本模型

### 5.3 前端

Phase 1 同步推进 `MarginDashboardView`，与 `GroupDistributionChart` 同级：

- 一级维度：Provider / Account / Model 三选一
- 时间范围：日 / 周 / 月 / 自定义
- 关键指标：售价、上游成本、毛利、毛利率、"无上游成本快照行占比"

---

## 6. 验收用例

Phase 0 验收：

1. 调 Anthropic Claude 4.7 一次（带 cache_read），UsageLog 行内：
   - `upstream_unit_price_input` / `upstream_unit_price_output` / `upstream_unit_price_cache_read` 三列均非空（cache_creation 视请求是否触发缓存写）
   - `upstream_total_cost` 等于上述单价乘以对应 token 数之和
   - `provider = "anthropic"`，`pricing_source = "provider_table"`
   - `actual_cost ≥ upstream_total_cost`
2. 后台导入一份 DeepSeek 单价（`provider="deepseek"`），DeepSeek 账号的下一次请求 `pricing_source = "provider_table"` 且金额吻合。
3. 未导入单价的 provider（如刚接入的新渠道），请求行 `upstream_total_cost = NULL` **且** `pricing_source = NULL`（**两者必须同时 NULL**，不允许出现"NULL 成本 + 非 NULL source"或反向组合）；运营页对该行明确显示"无上游成本快照"，不混用 `account_stats_pricing` 估算回填。
4. 历史回填：升级前的 UsageLog 行，离线 job 跑完后 `provider` 字段非空（从 `account.platform` 推导）；`upstream_total_cost` 与 `pricing_source` 均保持 NULL。
5. 同一 provider 在不同时间段调价：调价后写入的行 `upstream_unit_price_*` 反映新价；调价前的历史行保持旧价快照，不被回写。
6. 毛利视图：按 provider 聚合，`margin = sum(actual_cost) - sum(upstream_total_cost)`，仅对 `upstream_total_cost` 非空的行汇总；视图上"无上游成本快照"行数与占比明示。
7. 字段一致性约束（单元/集成测试）：扫描全表，**不应存在** `(upstream_total_cost IS NULL AND pricing_source IS NOT NULL)` 或 `(upstream_total_cost IS NOT NULL AND pricing_source IS NULL)` 的行。

---

## 7. 风险与回滚

| 风险 | 概率 | 影响 | 缓解 |
|---|---|---|---|
| `provider_pricing` 单价维护脱节，导致大量行落入 NULL 快照、毛利视图覆盖率下降 | 中 | 中 | 运营页 dashboard 显示"无上游成本快照"行占比并设阈值告警；后台跑 LiteLLM 同步作为**新模型上线提醒**（仅提示，不自动写入 `provider_pricing`，单价值由运营审核后落库） |
| 上游 Provider 单位（per-million-token / per-thousand-token）混乱 | 中 | 高 | schema 固定 `decimal(20,10)` 为"每 token 价格"；导入界面强制单位换算 + 单测 |
| 双轨写入计算耗时增加 | 低 | 低 | 异步链路；任一轨失败不阻塞另一轨 |
| 历史回填 job 卡死大表 | 低 | 中 | 分批 ≤10000 行 + sleep 500ms + 可中断 |
| Shadow write 期间字段写入与展示不一致引起客服困惑 | 中 | 低 | 内部仅运营看，shadow 周期 ≤7 天，差异 ≥10% 单独审核 |
| 删除 provider_pricing 行导致历史快照悬挂 | 低 | 中 | UsageLog 自带单价快照，删 provider_pricing 不影响历史 |
| 字段过多影响 UsageLog 写入吞吐 | 低 | 中 | 新增字段全部可空 + 不加索引；监控 P99 写入延时 |

**回滚策略**：

- 字段层：纯新增列，drop 即回滚；写入路径有 feature flag `USAGE_UPSTREAM_COST_ENABLED`（默认 true，可热切 false）。
- 价表层：`provider_pricing` 是独立表，drop 不影响其他实体。
- 展示层：前端 `MarginDashboardView` 独立路由，下线只删 router 项。

---

## 8. 与其他文档关系

- 协议层重构、Phase 路线全局编号见 `docs/relay-architecture-design.md` §7。
- Provider 规范化（`normalize_provider`）与别名表见 `docs/generic-channel-design.md` §4.4。
- 跨 group 聚合视图（Account / Provider 维度）见 `docs/generic-channel-design.md` §5.4 与本文 §5。
- 计费现状基线见 `backend/internal/service/account_stats_pricing.go` 与 `backend/internal/service/usage_service.go`。
