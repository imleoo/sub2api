# MAAS 重构 — Sprint 拆分计划

> 状态：v1 | 日期：2026-05-14 | 关联：`glossary.md`（事实表）、`relay-architecture-design.md`、`upstream-cost-snapshot.md`、`generic-channel-design.md`

把 6 个 Phase 的总体路线拆解为 **33 个独立可合并、独立可回滚的 PR**（原 31 PR + P0-7 lingjing 异步计费 + P2-6 fork 12 项回归）。本文档面向 sprint planning 和 PR 排期，不重复设计细节（设计见上述三份关联文档）；Phase 编号、总工期、字段清单、protocol 取值等原子事实统一引用 `glossary.md`。

**总工期与 Phase 简表**：见 `glossary.md` §2（**单一权威源**）。摘要：**15 周**（不含 Phase 5 观察期） / **23 周**（含 Phase 5 的 4 周双桶并存观察）——比初版 14/22 周 +1 周，吸收 P0-7（lingjing 异步计费，+2 人天）与 P2-6（fork 12 项回归，+1 人天）+ P2-3 修订（+2 人天）共计 +5 人天的 fork 适配开销。

---

## 拆分原则

1. **每 PR 人天上限**：
   - ≤3 天为目标
   - 3–5 天可接受，但 PR 描述必须说明为什么不可再切（强内聚 / 测试整体 / 观察期）
   - **>5 天必须再切**
   - "观察期 / 双桶并存 / 4 周对账"类 wall-clock 长但开发量小的 PR，按"观察期 PR"单独标注，不计入开发上限
2. **每 PR 自带回滚开关**：feature flag 或配置项，回滚不靠 revert。
3. **schema/接口契约对齐**：PR 之间不靠口头协调，靠合同。
4. **Phase 0/1 schema 正交可并行**；**Phase 2 起严格串行**（动路由/调度热路径）。
5. **每个 PR 必须自带验收用例**（单测/集成/端到端任一）。

---

## Phase 0：计费基础设施（7 PR / ~12 人天）

**目标**：UsageLog 行内同时见上游成本与下游售价，毛利率可查。

| PR | 内容 | 关键文件 | 人天 | 依赖 | 验收 |
|---|---|---|---|---|---|
| **P0-1** | 新增 `provider_pricing` ent schema + 迁移 | `backend/ent/schema/provider_pricing.go`（新）；`go generate ./ent` | 1 | — | `provider_pricing` 表创建成功；可手动 INSERT/SELECT |
| **P0-2** | `UsageLog` 9 个新字段（全部可空）：`upstream_unit_price_input` / `upstream_unit_price_output` / `upstream_unit_price_cache_creation` / `upstream_unit_price_cache_read` / `upstream_total_cost` / `provider` / `pricing_source` / `async_task_id` / `cost_finalized_at`（详见 `upstream-cost-snapshot.md` §2.1；async 字段服务 fork 12 lingjing 异步计费） | `backend/ent/schema/usage_log.go` + ent 生成 | 1 | — | schema migration 通过；新字段写入读取空值不报错；`upstream_total_cost` 与 `pricing_source` 一致性约束写入 CHECK 或单测断言；async 字段一致性见 `glossary.md §3.1` 异步约束 |
| **P0-3** | `ProviderPricingRepository` CRUD + `FindEffective(provider, model, now)` | `backend/internal/repository/provider_pricing_repo.go`（新）+ 单测 | 1.5 | P0-1 | 单测覆盖：精确匹配 / 通配符 / 生效区间命中 |
| **P0-4** | 新增独立 `resolveUpstreamCost()`（**不修改 `account_stats_pricing.go`**） + 单测 | 新增 `backend/internal/service/upstream_cost.go`；详见 `upstream-cost-snapshot.md` §3.2 | 2 | P0-2, P0-3 | 单测覆盖：命中 `provider_pricing` 返回 `(cost, "provider_table")`；未命中返回 `(nil, "")`；**不与 LiteLLM/fallback 估算交互**；`account_stats_pricing.go` 文件 0 diff |
| **P0-5** | UsageLog 装配点双轨写入 + feature flag | 抽 `applyUpstreamCostSnapshot()` helper，由 3 处构造点共调：`gateway_service.go:8641`（builder）、`openai_gateway_service.go:5309`、`usage_service.go:94`；配置项 `USAGE_UPSTREAM_COST_ENABLED`（**`writeUsageLogBestEffort` 函数本身不改**） | 4 | P0-4 | 集成测试：未命中行 `upstream_total_cost` 与 `pricing_source` **同时 NULL**；命中行同时非空；flag=false 时回到旧行为；3 个构造点行为等价 |
| **P0-6** | 对账 job（`provider` 历史回填**延后到 Phase 1**，本 PR 不做） | `script/reconcile_upstream_cost.sh`（新，仅对账） | 1 | P0-5 | 对账 job 每日产出 provider/account 维度差异报告；旧行 `provider` 保持 NULL（避免按 `account.platform` 粗粒度回填污染 provider_key 桶，详见 `upstream-cost-snapshot.md` §2.3） |
| **P0-7** | lingjing 异步计费接入上游成本快照（**fork 第 12 项**） | `backend/internal/service/lingjing_poll_runner.go` 增加 `applyUpstreamCostSnapshot()` 调用（UsageLog 第 4 个装配点）；UsageLog 新增 `async_task_id` / `cost_finalized_at` 两列（P0-2 已合入）；feature flag **强制 ON**（异步路径不走 `USAGE_UPSTREAM_COST_ENABLED`） | 2 | P0-2, P0-4, P0-5 | poll_runner 完成时 UsageLog 行 `upstream_total_cost` 与 `cost_finalized_at` 同时非空；超时行（240 次后 timeout）保持 NULL 并归"待结算超时"维度；与 `lingjing_task` 表对账每日差异 ≤0.1%；详见 `upstream-cost-snapshot.md` §6 用例 8 |

**Sprint 编排建议**：

- Sprint 1（5 天）：P0-1 + P0-2 + P0-3 + P0-4（共 5.5 人天，留 buffer）
- Sprint 2（5 天）：P0-5 + P0-6 + P0-7 + 启动 Phase 1

---

## Phase 1：轻量 Generic + provider_key 规范化（5 PR / ~12 人天）

**目标**：DeepSeek/豆包/硅基流动等用 `extra.provider` 元信息表达；新增 Account/Provider 跨 group 聚合视图。

| PR | 内容 | 关键文件 | 人天 | 依赖 | 验收 |
|---|---|---|---|---|---|
| **P1-1** | `normalize_provider` 别名表 + `extra.provider` 写入规范 + 细颗粒历史回填 job | `backend/internal/service/provider_normalize.go`（新）+ `backend/cmd/migrate_provider/`（新，按 `account.extra.provider` 推 UsageLog `provider`，**不读 `account.platform`**）+ 单测 | 2.5 | — | 单测覆盖：`DeepSeek` / `deep-seek` / `deepseek-official` → `deepseek` 单桶；回填 job 仅处理 `extra.provider` 非空且账号存活的行，已删账号或未写 `extra.provider` 行保持 NULL |
| **P1-2** | 跨 group 聚合 Repository API | `backend/internal/repository/usage_log_repo.go` 新增 `GetStatsByProvider` / `GetAccountStatsCrossGroup` + 单测 | 2.5 | P0-2 | 单测验证：账号挂多 group 时汇总 = 各 group 行级和；查询调用栈无 `group_id` 强过滤；**必须复用 fork 已有的 `getEntityUsageStats` 私有方法**（fork 第 3 项引入，见 `glossary.md §5` fork 锚点），不写第二套跨实体聚合范式 |
| **P1-3** | 前端表单 Provider 预设 | `frontend/src/types/index.ts` 类型枚举 fork **已有** `antigravity` / `lingjing`（fork 12 引入，无需补）；`frontend/src/components/account/CreateAccountModal.vue` OpenAI APIKey 模式加 Provider 下拉 + 自定义 base_url；**不复活 OAuth UI**（fork 第 11 项约束，仅暴露 API Key / Setup Token） | 3 | — | 类型检查通过；表单可选择预设并保存；旧账号兼容；不出现可选但无法路由的 `generic` 选项；OAuth 入口在 UI 中保持已删除状态 |
| **P1-4** | `ProviderDistributionChart.vue` + `AccountDistributionChart.vue` | `frontend/src/components/charts/` 新增两个组件 + 接 P1-2 API | 3 | P1-2 | 统计页三视图并列；Account 卡片显示"覆盖分组数 = N" |
| **P1-5** | i18n locale + 端到端 case | `frontend/src/i18n/locales/{en,zh}.ts` 补 `admin.accounts.providers.*`、`admin.dashboard.providers.*` | 2 | P1-3, P1-4 | 中英双语切换无 `missing translation` 警告；e2e：创建 DeepSeek 账号 → 发请求 → Provider 视图可见 |

**并行性**：P1-1 / P1-3 与 P1-2 可同时启动；P1-4/P1-5 末位串行。

---

## Phase 2：InboundProtocol / OutboundProtocol 双写（6 PR / ~18 人天）

**目标**：新增协议字段，旧字段保留作 alias，路由分流先看新字段。

| PR | 内容 | 关键文件 | 人天 | 依赖 | 验收 |
|---|---|---|---|---|---|
| **P2-1** | schema 新增 + 迁移回填 | `backend/ent/schema/group.go:55` 加 `inbound_protocol`；`account.go:64` 加 `outbound_protocol`；迁移脚本从 `platform` 推导默认值 | 2 | — | 升级后旧 group/account 双字段一致；新建支持只填新字段 |
| **P2-2** | `domain/protocol.go` 常量集中 + alias + deprecation linter | 新文件 + `service.PlatformOpenAI` 等改 alias；staticcheck 自定义检查 | 3 | P2-1 | `go build` 通过；旧引用编译期 warning；CI 报告但不阻断 |
| **P2-3** | `routes/gateway.go` **~10-11 处条件改造**"先新字段后回退" | 7 处 platform 分流（行号见 `glossary.md` §5）+ L168 `/v1/images/generations` 的 lingjing 分支（fork 12）+ 4 处 ForcePlatform middleware（L198/216/232/256，其中 L198 是 fork 12 lingjing 第 4 处）；**必须保留** promptAnalytics 中间件（fork 4，7 处挂载，见 `glossary.md §5` fork 锚点）和 `/lingjing/v1/video/*` 路由组（fork 12） | 6 | P2-2 | 旧 group（仅 `platform`）所有路径正常；新 group（只填 `inbound_protocol`）所有路径正常；5 个 platform 路由全通；promptAnalytics 中间件采集不丢失；lingjing 路由组完整 |
| **P2-4** | 启动健康检查 + 监控告警 | 新增 startup check 扫描所有 group/account 双字段一致性 + 不一致行计数 metric | 2 | P2-3 | 启动时若发现不一致打 ERROR 日志；监控面板可见不一致行数 |
| **P2-5** | 6 入口路径端到端回归集 | 6 入口路径 × **5 平台**（含 lingjing） × 流式/非流式 测试用例（强内聚不可再切：同一套 fixture 与断言库） | 4 | P2-4 | 全集通过；新增任意桥时复用该回归集 |
| **P2-6** | fork 12 项功能回归测试集（**合并守护**） | 5 个 platform 路由 × promptAnalytics 中间件采集 × masking 短路 × lingjing 视频任务 202+poll × 折扣 + 人民币换算 × 模型广场白名单 | 1 | P2-5 | 全集通过；CI 增加 `grep -q 'promptAnalytics\|/lingjing/v1/video' backend/internal/server/routes/gateway.go` 守护检查；fork 12 项功能列表（`claudedocs/自定义开发功能列表.md`）逐项扫描 |

**回滚**：drop 新字段 + 还原 routes/gateway.go 即可（但需手动恢复 fork 12 项功能的 lingjing 路由组——通过 P2-6 守护检查）。

> **更新（2026-06-08）**：fork 4 `promptAnalytics` 词云中间件已整体移除，本节及下方风险表中「必须保留 promptAnalytics 挂载」相关守护项均已撤销（`check_fork12_guards.sh` 与 `gateway_fork12_guard_test.go` 已删除该规则）。以上 P2-3/P2-6 计划原文保留作历史记录，lingjing 路由组守护仍然有效。

---

## Phase 3：Bridge Registry + N×M 矩阵收口（6 PR / ~18 人天）

**目标**：把 `ForwardAsAnthropic` / `ForwardAsResponses` 抽到 Registry；补齐缺失桥；建立流式回归测试集。

| PR | 内容 | 关键文件 | 人天 | 依赖 | 验收 |
|---|---|---|---|---|---|
| **P3-1** | `protocol_bridge_registry.go` 注册表（map，**不抽 interface**） | `backend/internal/service/protocol_bridge_registry.go`（新）；注册现有两条 Forward | 2 | P2-3 | 路由命中后从 Registry 取桥执行；与旧路径行为等价（差异化埋点） |
| **P3-2** | 流式 SSE 测试基础设施 | `script/record_upstream_sse.sh`（录制工具）；`backend/internal/pkg/apicompat/sse_replay.go`（重放框架）；`backend/testdata/sse/` 目录约定 | 3 | — | 每协议录 ≥5 fixture；replay 测试可断言事件序列 |
| **P3-3** | 新桥 `anthropic→openai_chat_completions` | `backend/internal/pkg/apicompat/anthropic_to_chatcompletions.go`（含流式）+ Registry 注册 | 3 | P3-1, P3-2 | 流式与非流式 fixture 全过；end-to-end DeepSeek 账号承接 Anthropic 入站 |
| **P3-4** | 抽出 `ProtocolBridge` interface（桥数已 ≥3） | Registry 重构为 interface 持有；旧 func 包装实现 | 1 | P3-3 | 所有现存桥实现新接口；行为不变 |
| **P3-5** | 新桥 `chatcompletions→anthropic` + `responses→chatcompletions` | apicompat 两个新文件 + Registry 注册 | 4 | P3-4 | 全部 4 桥的 4×N 矩阵流式回归通过 |
| **P3-6** | 能力探测 + RequestFeatures 嗅探 + 调度诊断 | `accounts.extra.bridge_capabilities` JSON；`RequestFeatures` 嗅探入 context；选账号/排除日志 | 5 | P3-5 | sticky/普通候选/failover 三处统一使用 features；诊断日志可追溯排除原因 |

**Phase 3 关键**：**P3-1 必须先做，P3-4 在桥数 ≥3 时才抽 interface（YAGNI）**。

---

## Phase 4：Gemini 入站桥（3 PR / ~10 人天）

| PR | 内容 | 关键文件 | 人天 | 依赖 | 验收 |
|---|---|---|---|---|---|
| **P4-1** | `gemini_to_openai.go` + `gemini_to_anthropic.go` 请求/非流式响应 | `backend/internal/pkg/apicompat/`（新两文件） | 4 | P3-5 | 非流式 `generateContent` 经 OpenAI/Anthropic 账号返回 |
| **P4-2** | Gemini streaming `streamGenerateContent` → SSE + function calling fuzz | apicompat streaming + 模糊测试集 | 4 | P4-1 | 流式 fixture 全过；function calling schema 映射零丢失 |
| **P4-3** | `gemini_v1beta_handler.go` 接 Bridge Registry | `backend/internal/handler/gemini_v1beta_handler.go` 接 Registry；保留 native 路径 | 2 | P4-2 | `/v1beta/*` 路由可路由到 anthropic/openai 账号；UsageLog `inbound_protocol='gemini_v1beta'` |

---

## Phase 5：Generic 多 Endpoint + Scheduler 重构（6 PR / ~22 人天，最硬骨头）

**目标**：单账号挂多 endpoint，scheduler 按 endpoint 分桶；双桶并存 4 周验证。

| PR | 内容 | 关键文件 | 人天 | 依赖 | 验收 |
|---|---|---|---|---|---|
| **P5-1** | `endpoint` ent schema + account 1→N + 迁移 | `backend/ent/schema/endpoint.go`（新）；现有 generic 账号迁出多 endpoint；迁移脚本在 `WHERE platform != 'lingjing'` 范围内执行 | 3 | P4-3 | 老账号兼容（单 endpoint 自动派生）；新建多 endpoint 账号可用；**lingjing 账号不参与 endpoint 实体迁移**（详见 `generic-channel-design.md` §10），保持单 endpoint 派生 |
| **P5-2** | scheduler `bucketFor` 双桶并存 + 埋点对比 | `backend/internal/service/scheduler_snapshot_service.go:678` 双写；新增对比 metric | 5 | P5-1 | 双桶产出每次分配结果对比埋点；离线 job 算每日一致率 |
| **P5-3** | sticky_session key 加 endpoint_id 维度 | sticky 服务全链路 key 升级；旧 key 平滑迁移 | 3 | P5-2 | 多 endpoint 账号 sticky 黏在同一 endpoint 1h |
| **P5-4** | `platform=generic` 常量 + 前端 generic 表单 | `domain/constants.go` 加 `PlatformGeneric`；前端 endpoint 列表表单 | 4 | P5-1 | 前端可创建/编辑多 endpoint 账号；测试连接按 endpoint 分发 |
| **P5-5** | 双桶并存 4 周观察 + chaos 测试 | 监控 dashboard + 报警规则 ≥1% 差异；chaos 注入失败切换 | 5 | P5-3, P5-4 | 4 周内 ≥99% 一致率；任意 endpoint 失败可切换无客户端可见错误 |
| **P5-6** | 切单桶 + 旧逻辑下线 | 删除旧 platform 桶代码路径；保留兼容兜底 | 2 | P5-5 | 切换后旧桶代码无任何调用；保留 1 季度回滚开关 |

---

## 全局 Sprint 编排

Phase 编号与总工期口径见 `glossary.md` §2。按 1–2 人全栈,6 个 Phase 约 8 个 Sprint:

| Sprint | 起止时间 | PR | 重点 |
|---|---|---|---|
| 1 | W1-W2 | P0-1 → P0-4 + P1-1 启动 | Phase 0 schema 落地，启动 Phase 1 |
| 2 | W3-W4 | P0-5/6/7 + P1-1～P1-3 | Phase 0 写入路径上线（shadow）含 **fork 12 lingjing 异步计费**；Phase 1 前后端并行 |
| 3 | W5-W6 | P1-4/5 + P2-1/2 | Phase 1 收尾；启动 Phase 2 schema |
| 4 | W7-W8 | P2-3/4/5/6 | Phase 2 路由分流改造（~10-11 处含 fork 12 lingjing）+ 健康检查 + **fork 12 项回归（P2-6）**+ 全集回归 |
| 5 | W9-W10 | P3-1/2/3 | Phase 3 Registry + 流式测试 + 第一条新桥 |
| 6 | W11-W12 | P3-4/5/6 | Phase 3 收口 + 能力探测 |
| 7 | W13-W14 | P4-1/2/3 | Phase 4 Gemini 桥 |
| 8 | W15-W23 | P5-1～P5-6 | Phase 5 多 endpoint + scheduler 重构（含 4 周观察，**排除 lingjing**） |

**总工期**：见 `glossary.md` §2（单一权威源）。

---

## 状态追踪

每个 PR 在合并时更新本文档对应行的"状态"列（pending/in-progress/merged/blocked）。可用前置脚本：

```bash
# 查看当前 sprint 进度
grep -E "^\| \*\*P[0-9]-[0-9]\*\*" docs/sprint-plan.md | grep -c "merged"
```

未来若拆分调整（PR 合并、拆分、删除），同步更新本表与对应 Phase 章节。

---

## 风险与依赖矩阵

| 风险 | 命中阶段 | 缓解所在 PR |
|---|---|---|
| ent codegen 冲突 | P0-1/2, P2-1, P5-1 | 每个 schema PR 单独发布，避免合并冲突 |
| 流式 SSE 时序 bug | Phase 3/4 | P3-2 提前建测试集 |
| Platform 常量扩散面 | Phase 2 | P2-2 alias + linter 控制爆炸 |
| Scheduler 热路径 | Phase 5 | P5-2 双桶 + P5-5 4 周观察 |
| 上游单价维护脱节 | Phase 0 起 | P0-6 对账 job + LiteLLM 同步 |
| 前端类型滞后 | Phase 1 | P1-3 强制与后端同 sprint |
| **platform 漏算 lingjing 导致 Phase 2 / Phase 5 改造遗漏**（fork 12） | Phase 2/5 | `glossary.md §1.2` platform 表 + `generic-channel-design.md §10` 排除条款 + P5-1 验收 `WHERE platform != 'lingjing'` 过滤三重守护 |
| **Phase 2 改 routes/gateway.go 时不小心移除 lingjing 路由组**（fork 12）<br>（注：fork 4 promptAnalytics 已于 2026-06-08 移除，对应守护项撤销） | Phase 2 P2-3 | P2-6 fork 12 项回归 PR + CI `grep -q '/lingjing/v1/video'` 守护检查 |
| **`setting_handler.go` 同时被 fork 5（pricingService/adminService）和 Phase 0 改签名导致 wire_gen 双向 merge 冲突** | Phase 0 P0-3 | P0-3 注入 `ProviderPricingRepo` 时新建独立 `admin/provider_pricing_handler.go`，**不挤进** 已经膨胀的 SettingHandler |
| **Lingjing 240×5s=20 分钟超时窗口内 UsageLog 行级查询缓存不一致**（fork 12） | Phase 0 P0-7 | 统计查询加 `WHERE cost_finalized_at IS NOT NULL` 过滤实时毛利视图；BI 按 `cost_finalized_at` 而非 `created_at` 聚合 |
| **Masking 短路计费被误归为"无快照异常"**（fork 8） | Phase 0 起 | `upstream-cost-snapshot.md §6` 验收用例 9 + 运营页"无快照异常"列表 SQL 加 `AND async_task_id IS NULL` 以区分四态 |

---

## 与文档关系

- 设计细节：`docs/relay-architecture-design.md` / `docs/generic-channel-design.md` / `docs/upstream-cost-snapshot.md`
- 评估报告（含决策依据）：`/Users/leoobai/.claude/plans/tender-squishing-ullman.md`
- 本文档：**执行视图**，PR 颗粒，sprint planning 使用
