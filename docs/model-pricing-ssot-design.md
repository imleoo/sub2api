# 模型数据「DB 为唯一真源」重构方案

> 状态：草案 · 作者：assistant · 日期：2026-05-27
> 关联：[`generic-channel-design.md`](generic-channel-design.md)、[`relay-architecture-design.md`](relay-architecture-design.md)、[`upstream-cost-snapshot.md`](upstream-cost-snapshot.md)、[`claudedocs/模型定价与折扣管理升级方案.md`](../claudedocs/模型定价与折扣管理升级方案.md)

## 1. 问题陈述

当前模型数据被分散管理在 **3 套内存映射 + 2 张表 + 1 个 JSON 文件** 里，写入路径多、读取出口不一致，造成体感 bug：

> "模型折扣里有 60+ 模型，但模型广场只有 5 个。"

根本原因：模型广场的"可见模型"由 `endpoints.supported_models` 过滤，而该字段只在前端"拉取并保存账号"时落库；折扣表查 `model_pricings` 表，两者数据源不同。

## 2. 现状梳理（代码索引）

### 2.1 数据源

| 来源 | 触发 | 落点 | 备注 |
|---|---|---|---|
| LiteLLM 远端 JSON | 启动 + 定时 hash check + `POST /admin/model-pricings/sync` | 内存 `pricingData` + 文件 + `model_pricings` 表（`is_custom=false`） | `pricing_service.go:225 syncPricingToDB` |
| 上游 `/v1/models`（添加账号时拉取） | `POST /admin/accounts/endpoints/fetch-models` | `model_pricings` 表（`is_custom=true`，SeedIfNotExists 不覆盖）+ 前端 merge 到 `endpoints.supported_models` | `account_handler.go:2118` |
| 上游 `/v1/models`（模型定价页同步） | `POST /admin/model-pricings/sync-from-upstream` | `model_pricings` 表（SeedIfNotExists） | `model_pricing_handler.go:417` |
| 灵境静态价格 | 启动 seedCustomModels | `model_pricings` 表 + `injectCustomPricingModels` 注入到 `pricingData` | `pricing_service.go:280, 866` |
| 旧 settings.model_discounts | 启动 migrate | `model_pricings.discount_rate` | `pricing_service.go:335` |
| 管理员手动新增/修改 | `Create` / `Update` | `model_pricings` 表（`is_custom=true`）+ `ReloadFromDB` | `model_pricing_handler.go:225, 309` |

### 2.2 内存层（PricingService）

```go
type PricingService struct {
    pricingData  map[string]*LiteLLMModelPricing // 主目录：LiteLLM JSON + DB 独有模型注入
    discounts    map[string]float64              // 折扣率
    customPrices map[string]*DBModelPricing      // 自定义 cost + 折扣 DB 副本
}
```

`pricingData` 是早期从 JSON 直接反序列化得到的「主目录」。`loadPricingFromDB` 把 DB 行回填进 `pricingData`（最近修复），但**这只是补丁**：每次新增模型都依赖手动 `ReloadFromDB`，并且 `s.pricingData = data` 这种整体替换的代码路径（`downloadPricingData`、`loadPricingData`）会把 DB 注入的条目刷掉，要再次 `loadPricingFromDB` 才能恢复。

### 2.3 读取出口

| 功能 | 调用路径 | 实际数据源 |
|---|---|---|
| **模型计费** `BillingService.GetModelPricing(model)` | `pricingService.GetModelPricing` → `pricingData` + `GetDBModelPricing` → `customPrices` + `GetDiscount` → `discounts` | 三套内存 map |
| **模型广场** `GET /v1/usage/models` | `pricingService.ListAllModels()` → `pricingData`，再按 `endpoints.supported_models` + `account.credentials.model_mapping` 过滤 | `pricingData` + endpoints 表 |
| **模型折扣表** `GET /admin/model-pricings` | `repo.List` → `model_pricings` 表（支持 `ExcludeOverseasModels`） | DB 直读 |
| **模型详情/搜索** `pricingService.GetModelPricing(name)` | 模糊匹配 `pricingData`，再 fallback 系列/家族正则 | 内存 |
| **provider 模型列表** `pricingService.ListModelNamesByProvider` | `pricingData` | 内存 |

### 2.4 问题清单（按影响度排序）

1. **真源分裂**：DB 与内存 `pricingData` 并存，`downloadPricingData` 整体替换会丢失 DB 独有条目；`syncPricingToDB` 又把内存反向同步到 DB，互相覆盖。
2. **模型广场过滤错位**：`endpoints.supported_models` 由用户在前端表单维护，拉取模型只 merge 到内存、需用户再点账号"保存"才落库——这是 60+ 在折扣表、5 个在广场的直接原因。
3. **去重粒度模糊**：远端同步用 `UpsertBatch` 跳过 `is_custom=true`；账号拉取用 `SeedIfNotExists`（任何 `is_custom` 都跳过）；管理员 Create 强制 `is_custom=true`。三种路径，`is_custom` 语义其实是「不被远端 JSON 覆盖」的标记，跟"手动添加"已经不等价（账号拉取的也被标 `is_custom=true`）。
4. **JSON 文件副本**：`backend/data/model_pricing.json` + `.sha256` 与 DB 重复。文件存在的唯一作用是断网时回退首次加载——其实 DB 已经能承担这个角色。
5. **discount 双轨**：`discounts` map + `model_pricings.discount_rate` 同时存在，启动迁移一次但不持续同步。
6. **匹配逻辑只在内存**：`GetModelPricing` 的模糊匹配、家族 fallback、OpenAI/Claude 系列识别都依赖完整 `pricingData`，迁移到 DB 后这部分需要重新实现或保留为「DB → 内存索引」。

## 3. 设计目标

1. **DB 为唯一真源（SSOT）**：所有"模型存在/价格/折扣/启用"事实都从 `model_pricings` 表读取。
2. **内存层降级为只读索引**：仅作为热缓存与模糊匹配索引，刷新时间 ≤ N 秒（或写后立即失效），不再持有 DB 不知道的信息。
3. **去重入库自动化**：LiteLLM JSON、上游 `/v1/models`、管理员手动新增 → 同一张表 `model_pricings`，按 `model_id` 唯一键去重。
4. **同步的定价保留，其余手动**：LiteLLM 里有价的模型保留远端价格；上游 `/v1/models` 只拿到模型 ID 时入库 `cost=NULL`，靠管理员手动定价。
5. **模型广场来源切换**：从"按账号 supported_models 过滤" → "按 `model_pricings.is_enabled` + 账号 platform 可用性"。`supported_models` 退化为可选的"额外限制白名单"。

## 4. 数据模型变更

### 4.1 model_pricings 表（向后兼容）

新增 / 调整字段：

| 字段 | 类型 | 用途 | 默认 |
|---|---|---|---|
| `source` | varchar(20) | `litellm` / `upstream_sync` / `manual` / `lingjing` | `manual` |
| `source_provider` | varchar(100) | 当 `source=upstream_sync` 时记录账号 name，便于"从该 provider 同步过来的模型"列表 | `''` |
| `source_account_id` | bigint nullable | 触发入库的账号 id（审计） | NULL |
| `pricing_status` | varchar(20) | `priced` / `unpriced` / `disabled` | `unpriced` |
| ~~`is_custom`~~ | 保留兼容 | 计算值：`source != 'litellm'`，老代码读它仍能工作 | — |

迁移：
- 新加列默认值兜底。
- `source` 回填规则：`is_custom=false` → `litellm`；`is_custom=true AND provider IN (...lingjing...)` → `lingjing`；其余 `is_custom=true` → `manual`（上游同步过来的暂归 manual，后续按 `last_synced_at` + provider 名再回填 `upstream_sync`）。
- `pricing_status` 回填：`input_cost_per_token IS NOT NULL OR custom_input_cost IS NOT NULL` → `priced`；否则 `unpriced`。

### 4.2 endpoints.supported_models 语义调整

- 改名为「可选转发限制」：**为空 = 该 endpoint 允许所有 `model_pricings.is_enabled=true` 模型**。
- 非空 = 严格白名单（保留原语义，便于个别 endpoint 限制）。

### 4.3 文件副本退役（可选）

- 保留 `backend/data/model_pricing.json` 作为"启动时 DB 为空且远端不可达"的最后兜底。
- 不再写文件；下载完直接 upsert 到 DB；hash 比对仍可用（hash 字段挪到 `settings` 表）。

## 5. 写入路径统一

```
┌─────────────────────┐
│ LiteLLM hash poller │──┐
└─────────────────────┘  │
┌─────────────────────┐  │   ┌──────────────────────────┐
│ /admin/sync-upstream│──┼──▶│ ModelCatalogService      │   ┌──────────────┐
└─────────────────────┘  │   │ ─ Upsert(model_id, …)    │──▶│ model_pricings│
┌─────────────────────┐  │   │ ─ 去重：ON CONFLICT model_id│   └──────────────┘
│ accounts/fetch-models│─┤   │ ─ 来源策略：覆盖规则见 §5.2│
└─────────────────────┘  │   └──────────────────────────┘
┌─────────────────────┐  │
│ /admin/model-pricings│─┘
│  (Create/Update)     │
└─────────────────────┘
```

### 5.1 新增服务 `ModelCatalogService`

把所有写路径收归到这个 service，提供统一的 `UpsertModels(ctx, batch, source)` 方法，废弃以下分散方法：
- `PricingService.syncPricingToDB`
- `PricingService.seedCustomModels`
- `ModelPricingRepository.SeedIfNotExists`（保留底层接口）
- `account_handler.go` 直接调用 repo 的逻辑

### 5.2 覆盖规则（按字段细粒度）

写入时按 `source` 决定哪些字段可覆盖：

| 入库源 | 可覆盖字段 | 不覆盖字段 |
|---|---|---|
| `litellm`（定时同步） | `input_cost_per_token` / `output_cost_per_token` / `cache_*` / `supports_prompt_caching` / `mode` / `last_synced_at` | `display_name` / `description` / `discount_rate` / `custom_*_cost` / `is_enabled` / `source` |
| `upstream_sync`（账号拉取） | 仅当行不存在时插入 `model_id` + `provider` + `mode='chat'` + `pricing_status='unpriced'` | 任何已存在字段都不动 |
| `manual`（管理员 UI） | 全部字段 | — |
| `lingjing`（seed） | 仅当行不存在时插入 | — |

实现要点：用 SQL `ON CONFLICT (model_id) DO UPDATE SET … WHERE model_pricings.source != 'manual'`，避免把手动维护的价格冲掉。

### 5.3 拉取模型自动落库 + 自动入广场

- 账号"从上游拉取"成功后：
  1. `UpsertModels(source=upstream_sync, source_account_id=...)` → DB；
  2. **可选**：把 `endpoints.supported_models` 直接由后端写满（接口接受 `endpoint_id`），不依赖用户再点账号"保存"。前端按钮文案改成"同步并入广场"。
- LiteLLM 同步：保持现有 hash check 节奏，落 DB 后触发 `ReloadFromDB`。
- 折扣/价格：仅 `source=manual` 行能保留 `custom_*_cost` 与 `discount_rate`，定时同步不会覆盖。

## 6. 读取路径统一

### 6.1 内存层降级

```go
// 仅保留两个 map，并由 DB 单向构建
type PricingService struct {
    catalog   map[string]*DBModelPricing // model_id -> 全字段（包括 cost/discount）
    aliasIdx  map[string]string          // 小写别名/家族 -> model_id（用于模糊匹配）
}
```

- `Initialize()`：`loadFromDB()`（首屏）→ 启动定时 puller 写 DB → `loadFromDB()`（刷新）。
- 不再有 `pricingData` 与 `discounts` 的双轨。`GetModelPricing` / `GetDiscount` 全部基于 `catalog`。
- 模糊匹配（家族 fallback、OpenAI 别名）在 `loadFromDB` 时一次性预生成 `aliasIdx`。

#### 内存刷新策略（关键）

计费路径每次都从内存取价格（不能同步查 DB，否则会卡热路径）。`catalog` 必须有明确的刷新节奏：

1. **定时刷新**：与远端 LiteLLM hash check 同节奏，复用现有 `cfg.Pricing.HashCheckIntervalMinutes`（默认 10 分钟，最小 1 分钟）。
   - 单个 goroutine、单一 ticker，避免多个定时器互相覆盖锁。
   - 调度顺序：`fetchRemoteHash → if changed: downloadAndUpsert → loadFromDB`。即使远端没变，每个 tick 仍执行一次 `loadFromDB`，保证多副本部署下 A 实例的管理员改价能在 ≤ 1 个 tick 后被 B 实例看到。
2. **写后立即刷新**（已实现的 `ReloadFromDB`）：管理员 Create/Update/Delete、账号拉取入库、`sync-from-upstream` 完成后同步调用，让当前实例立刻看到新数据。
3. **首屏**：`Initialize` 内串行执行 `loadFromDB → 远端 hash check（若需要则下载入库）→ loadFromDB`。先加载本地 DB 保证不阻塞启动，再尝试远端，远端失败不影响服务。
4. **失败兜底**：`loadFromDB` 失败保留旧 `catalog`，仅记 warn 日志；连续 N 次失败触发告警（运营信号）。

#### 一致性约束

- `loadFromDB` 必须原子替换 `catalog`，不能边读边写。实现用 `tmp := build(); s.mu.Lock(); s.catalog = tmp; s.mu.Unlock()`。
- 计费路径的 `RLock` 持有时间必须短，禁止在锁内做 IO。
- 多实例部署下短暂不一致可接受（≤ 1 个刷新周期），因为：
  - 写后实例自身已立即刷新；
  - 其他实例最多滞后一个 tick；
  - 折扣/价格变更不是高频操作。
- 配置可见：`PricingService.GetStatus()` 暴露 `last_loaded_at` / `last_remote_hash_check_at` / `model_count` 三个字段供运维巡检。

### 6.2 模型广场

`UsageHandler.getModels` 调整为：

```go
// 1. 用户可访问账号 → platforms
// 2. 平台 → 模型筛选规则：
//    - generic: 取每个 endpoint.supported_models；为空时全部 enabled 模型
//    - 其他平台: 取 account.credentials.model_mapping 或平台默认白名单
// 3. 与 DB 中 is_enabled=true 的模型求交
```

落地：所有模型来源 `pricingService.ListEnabledModels()`（新方法，直接走 `catalog`），不再依赖账号 supported_models 决定"模型存在不存在"，只决定"模型对该用户是否可见"。

### 6.3 模型折扣页 / 模型定价页

- 继续走 `repo.List`，但 `pricing_status` 暴露给前端，让"未定价"模型在列表里有视觉标记，引导管理员补价。
- 列表里多一个过滤条件 `source=upstream_sync` → 一键看"刚同步进来还没定价"的模型。

### 6.4 模型计费

- `BillingService.GetModelPricing` 改为：`pricingService.GetModelPricing(model)` 直接返回 `DBModelPricing`，按 `custom_*_cost ?? input/output_cost_per_token` 计算最终价。
- 当 `pricing_status='unpriced'` 时计费返回 0 + 日志 warn，触发"未定价模型被调用"告警（运营信号）。

## 7. UI 变更

| 页面 | 现在 | 改后 |
|---|---|---|
| 模型广场 `ModelsView` | 只显示账号 supported_models ∩ pricingData | 显示用户可访问平台的所有 `is_enabled` 模型 |
| 模型折扣 `ModelPricingsView` | 已经 DB 直读 | 增加 `pricing_status` 标记 + `source` 过滤 |
| 添加/编辑账号（generic） | "拉取"后只更新前端，再保存账号才落库 | "拉取"成功 = 落 `model_pricings` + 自动写 `endpoints.supported_models` |
| 模型定价同步入口 `/admin/model-pricings/sync-from-upstream` | 已经实现，需要在 UI 暴露独立入口 | 在折扣页加"从 URL 同步"按钮，与账号管理解耦 |

## 8. 迁移步骤（按 PR 拆分）

> 每个 PR 自包含、独立可回滚。Wire 改动随对应 PR 同步生成。

1. **PR-1 Schema 扩展**：加 `source / source_provider / source_account_id / pricing_status` 列，回填默认值；不动代码逻辑。
2. **PR-2 ModelCatalogService 抽象**：新建 service，把现有 4 个写路径迁过去；保留旧函数为薄包装一段时间。
3. **PR-3 内存层重构**：`PricingService` 改用 `catalog + aliasIdx`，删除 `pricingData / discounts` 双轨；BillingService 透传不变。
4. **PR-4 模型广场切源**：`UsageHandler.getModels` 改为基于 `is_enabled` + 平台白名单；`endpoints.supported_models` 退化为可选限制。
5. **PR-5 账号拉取自动落广场**：`FetchEndpointModels` 接 endpoint_id，后端写 `endpoints.supported_models`，前端去掉手动 merge。
6. **PR-6 文件副本退役（可选）**：移除 JSON/SHA 写文件；保留只读 fallback。
7. **PR-7 前端**：折扣页加 `pricing_status` 标记、过滤 `source`；模型定价同步按钮独立入口。

## 9. 兼容与回滚

- `is_custom` 字段保留，新代码以 `source` 为准，旧调用方仍能读 `is_custom`。
- 内存层重构后保留 `pricingService.GetModelPricing / GetDiscount / GetDBModelPricing` 公开 API 签名不变，BillingService 不需要改动。
- 每个 PR 都可单独 revert；schema 变更使用幂等迁移（`ADD COLUMN IF NOT EXISTS`）。
- 文件副本仅在 PR-6 退役，前序 PR 不动文件路径，保证回滚时不丢数据。

## 10. 风险点

| 风险 | 缓解 |
|---|---|
| LiteLLM 同步覆盖已被管理员手动改价的行 | `ON CONFLICT DO UPDATE SET … WHERE source != 'manual'`，并在 UpsertBatch 测试用例里固化该不变量 |
| 模糊匹配性能（每次 GetModelPricing 都从 DB 取） | 计费路径只读内存 `catalog + aliasIdx`，绝不查 DB；DB 仅由"首屏 + 定时刷新（同 hash check 节奏） + 写后刷新"三个入口注入内存 |
| 多实例部署下管理员改价滞后 | 定时刷新即使远端无变化也强制 `loadFromDB` 一次，保证 ≤ 1 个 tick 内对齐；写实例自身立刻可见 |
| 定时刷新卡住计费热路径 | `loadFromDB` 构建 `tmp` 后原子替换，`RLock` 持有时间限定在 map 查找；任何 IO 都在 Lock 外 |
| 未定价模型被计费 → 收费 0 | `pricing_status` warn 日志 + 后台告警面板 + 列表筛选 |
| 模型广场突然多出 60+ 模型造成用户疑惑 | 模型广场上线前先在折扣页面用 `source=upstream_sync` 一键清理无用模型；提供"批量禁用"按钮 |
| `endpoints.supported_models` 语义变更影响存量行为 | 兼容期：旧行为开关 `gateway.scheduling.legacy_supported_models_strict`，默认关闭新语义；灰度后强行切换 |

## 11. 验收用例

- [ ] LiteLLM 同步后，手动改价的模型 `custom_input_cost` 不被覆盖。
- [ ] 上游 `/v1/models` 拉取 60+ 模型 → 全部落 `model_pricings` (source=upstream_sync, pricing_status=unpriced)。
- [ ] 同样这 60+ 模型，未在折扣页定价时，模型广场可见且标记"未定价"。
- [ ] 管理员在折扣页定价后，再次 LiteLLM 同步不覆盖自定义价格。
- [ ] 删除自定义模型后，下次同步如远端有同 model_id 才会回归 `source=litellm`；否则保持已删除。
- [ ] 国内/海外过滤（`OverseasModelIDPrefixes`）在模型广场和折扣页表现一致。
- [ ] `BillingService` 对 `unpriced` 模型返回 0 价 + warn 日志。
- [ ] 多实例部署下：A 实例改价 → B 实例 ≤ `HashCheckIntervalMinutes` 内 `GetModelPricing` 返回新价。
- [ ] `loadFromDB` 失败时 `catalog` 保持上次成功状态，计费不中断。
- [ ] 定时 ticker 持续运行 24h+ 不漏 tick、不重叠（log 抽样 + `last_loaded_at` 单调递增）。

## 12. 不做的事（明确边界）

- **不做** GraphQL / 复杂查询 DSL。模型集合不大（< 1k），SQL `Where + In` 已经够用。
- **不做** 多租户的 model_pricings 分表。所有租户共享同一张目录表。
- **不做** 价格历史版本表（已有 `upstream-cost-snapshot.md` 单独议题）。
- **不做** LiteLLM 替代品评估。本方案不依赖具体上游源，只是数据流重组。
