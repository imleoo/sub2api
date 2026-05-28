# 模型定价 SSOT 重构完成报告

> 状态：**已完成（PR-1 ～ PR-8）** · 完成日期：2026-05-28
> 分支：`feature/maas-refactor`
> 关联设计文档：[`model-pricing-ssot-design.md`](model-pricing-ssot-design.md)

---

## 1. 背景

重构前模型数据被分散在三套内存 map（`pricingData` / `discounts` / `customPrices`）、`model_pricings` DB 表、64 条硬编码 `fallbackPrices` 以及 `backend/data/model_pricing.json` 文件中。写入路径多且彼此覆盖，直接导致"折扣表 60+ 模型，模型广场只有 5 个"的体感 bug。

重构目标：**`model_pricings` DB 表为唯一真源（SSOT）**，内存层降级为定时刷新的只读 catalog + alias 索引。

---

## 2. 八个 PR 完成的工作

### PR-1 · Schema 扩展（migration 145）

新增 14 列：

| 列名 | 类型 | 用途 |
|-----|------|------|
| `long_context_input_token_threshold` | int64 nullable | 长上下文触发阈值 |
| `long_context_input_cost_multiplier` | decimal nullable | 长上下文 input 倍率 |
| `long_context_output_cost_multiplier` | decimal nullable | 长上下文 output 倍率 |
| `input_cost_per_token_priority` | decimal nullable | priority tier input 单价 |
| `output_cost_per_token_priority` | decimal nullable | priority tier output 单价 |
| `cache_read_input_token_cost_priority` | decimal nullable | priority tier cache read 单价 |
| `source` | varchar(20) | `litellm` / `upstream_sync` / `manual` / `bootstrap` / `lingjing` |
| `pricing_status` | varchar(20) | `priced` / `unpriced` |
| `output_cost_per_image` | decimal nullable | 灵境图片生成单价 |
| `image_output_price_per_token` | decimal nullable | 视频生成 token 单价 |
| `source_provider` | varchar(100) | 触发入库的 provider name（审计） |
| `source_account_id` | bigint nullable | 触发入库的账号 id（审计） |
| `supports_cache_breakdown` | bool | 是否支持 5m/1h 分档 cache pricing |
| `cache_creation_5m_token_cost` / `cache_creation_1h_token_cost` | decimal nullable | 5m/1h cache creation 单价 |

迁移文件：`backend/migrations/145_model_pricings_ssot_columns.sql`（全部 `ADD COLUMN IF NOT EXISTS`，回滚安全）

### PR-2 · 黄金值回归测试基线

新建两个测试文件固化所有计费路径：

- `backend/internal/service/billing_service_golden_test.go`：14 模型 × 7 场景 × 6 token 边界，精确到 1e-12
- `backend/internal/service/pricing_service_match_test.go`：固化 `matchByModelFamily` 对 10 个家族锚点的命中

### PR-3 · Bootstrap Seed + 覆盖验证工具

- `backend/internal/service/model_catalog_seed.go`：`BootstrapPricingSeeds()` 返回 21 条 seed（16 条 billing fallback + 5 条灵境），启动时调 `SeedIfNotExists` 写入 DB
- `backend/internal/service/model_catalog_seed_test.go`：内容回归测试，种子价格与 fallback 字面值 1:1 校验
- `backend/scripts/verify_pricing_coverage/main.go`：CLI 工具，列出 fallback ∪ 家族锚点 ∪ 最近 30 天 usage_log 模型 ID，逐项检查 DB，miss 退出码 1

### PR-4 · Catalog + AliasIndex（影子模式）

- `PricingService` 新增 `catalog map[string]*DBModelPricing` + `aliasIdx map[string]string`
- `buildCatalogFromDB(ctx)` 构建：DB → catalog → aliasIdx（三路合并：codex 别名、家族正则、model_id 归一化）
- 复用现有 `startUpdateScheduler` ticker，tick 时原子替换 catalog/aliasIdx
- `openai_codex_transform.go` 新增 `CodexAliasPairs()` 导出
- 此阶段计费仍走老路径，catalog 仅并行构建

### PR-5 · 影子比对（V1/V2 双跑）

- `BillingService.GetModelPricing` 内双跑 V1/V2，不一致写 `pricing_drift_log` + warn 日志
- 观测 72h 零漂移后进 PR-6
- 配置开关：`cfg.Pricing.ShadowCompare`（默认 true，PR-8 已删除）

### PR-6 · 主路径切 catalog（保留 fallbackPrices 二级兜底）

- `BillingService.GetModelPricing` 重写：`catalog → aliasIdx → matchByModelFamily(catalog) → if LegacyFallbackEnabled: fallbackPrices → unpriced 5xx`
- `matchByModelFamily` 改为在 `catalog` 上查找（原在 `pricingData`）
- long-context 字段直接从 catalog 行读取
- 灰度开关：`cfg.Pricing.UseCatalogPath`（默认 true）、`cfg.Pricing.LegacyFallbackEnabled`（过渡期 true）

### PR-7 · 写路径统一（ModelCatalogService）

- `backend/internal/service/model_catalog_service.go`：`UpsertModels(ctx, batch, source)` 按 source 决定覆盖规则

  | source | 覆盖规则 |
  |--------|---------|
  | `litellm` | 覆盖价格字段，不覆盖 `display_name`/`discount_rate`/`is_enabled` |
  | `upstream_sync` | 仅当行不存在时插入（pricing_status=unpriced） |
  | `manual` | 覆盖全部字段 |
  | `bootstrap` / `lingjing` | SeedIfNotExists，仅当行不存在时插入 |

- 所有写路径（syncPricingToDB、seedCustomModels、injectCustomPricingModels、account_handler、model_pricing_handler）改调 ModelCatalogService
- 任意写入成功后触发 `ReloadFromDB` → catalog/aliasIdx 重建

### PR-8 · 清理（完成于 2026-05-28）

**删除内容：**
- `billing_service.go`：`fallbackPrices` 字段及 64 条静态价格、`getFallbackPricing()` 函数
- `pricing_service.go`：`pricingData` / `discounts` / `customPrices` 三个内存 map 及所有专用方法（`syncPricingToDB`、`loadDiscounts`、`migrateOldDiscounts`、`injectCustomPricingModels` 等）
- `downloadPricingData` 文件写入路径（保留 JSON 读路径作冷启动兜底）
- `cfg.Pricing.ShadowCompare` 开关与漂移日志写入
- `cfg.Pricing.LegacyFallbackEnabled` 设为永久 false（fail-closed 语义）

**修复测试：**
- `gateway_record_usage_test.go`：改用 `newTestBillingServiceWithConfig(cfg)` 构建有效 catalog
- `openai_gateway_record_usage_test.go`：新增 `newGatewayTestBillingService` helper，补充 `gpt-5.1` test-only 定价
- `api_contract_test.go`：补充三个新 setting 字段快照（`api_key_acl_trust_forwarded_ip`、`show_overseas_models`、`subscription_expiry_notify_enabled`）；补充 `BatchUpdate` stub、`NewAdminService` 第 19 参数、`NewUsageHandler` 第 7 参数

---

## 3. 现行架构（重构后）

```
写入路径
  LiteLLM 远端 JSON ──────────────────────┐
  上游 /v1/models (账号拉取) ──────────────┤──► ModelCatalogService.UpsertModels
  管理员手动新增/修改 ─────────────────────┤    (按 source 决定覆盖规则)
  Bootstrap Seed (启动时) ─────────────────┘         │
                                                      ▼
                                              model_pricings (DB SSOT)
                                                      │
读取路径                                               ▼
  BillingService.GetModelPricing(model)   ◄── PricingService (定时刷新)
         │                                     catalog map[modelID]*DBModelPricing
         ├─ catalog[model]                     aliasIdx map[normalizedID]modelID
         ├─ aliasIdx[normalize(model)] → catalog[...]
         ├─ matchByModelFamily(catalog)
         └─ pricing_status=unpriced → 5xx (fail-closed)
```

---

## 4. 关键不变项

- UsageLog 快照字段（`actual_cost` / `input_cost` / `output_cost` / `cache_*_cost` / `cost_finalized_at`）路径完全未动
- 计费单位仍为 USD，CNY 仅展示层换算
- `openai_codex_transform.go` 别名字典保留（aliasIdx 依赖）
- `backend/data/model_pricing.json` 保留只读冷启动兜底（仅去除写路径）

---

## 5. 运维注意

### 回滚方式
- PR-8 回滚：revert 该 commit，即回到 PR-7 双轨状态，DB 数据完全不受影响
- PR-6 回滚：`cfg.Pricing.UseCatalogPath=false`

### 上游合并风险
以下文件在 SSOT 重构后与上游差异最大，合并前必须重点检查：

| 文件 | 检查要点 |
|-----|---------|
| `billing_service.go` | 不得重新引入 `fallbackPrices`；`GetModelPricing` 必须走 catalog 路径 |
| `pricing_service.go` | 不得重新引入 `pricingData`/`discounts`/`customPrices`；`catalog`/`aliasIdx` 字段必须保留 |
| `model_catalog_service.go` | 上游无此文件，合并时确认未被意外删除 |
| `model_catalog_seed.go` | 上游无此文件，合并时确认未被意外删除 |
| `migrations/145_*.sql` | 上游无此文件 |

### Bootstrap Seed 21 条模型 ID

```
anthropic: claude-opus-4.5/4.6/4.7, claude-sonnet-4, claude-3-5-sonnet,
           claude-3-5-haiku, claude-3-opus, claude-3-haiku
google:    gemini-3.1-pro
openai:    gpt-5.4, gpt-5.5, gpt-5.4-mini, gpt-5.4-nano, gpt-5.2,
           gpt-5.3-codex, gpt-5.3-codex-spark
lingjing:  doubao-seedream-4-0-250828, doubao-seedream-4-5-251128,
           Doubao-Seedream-5.0-lite, doubao-seedance-1.5-pro-5s,
           doubao-seedance-1.5-pro-10s
```
