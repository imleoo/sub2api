# MAAS 重构 — 文档导航

## 按角色查阅

| 我是 | 先读 | 然后读 |
|---|---|---|
| **架构师 / Tech Lead** | [`relay-architecture-design.md`](./relay-architecture-design.md) — 三层分离、Bridge 矩阵、调度规则 | [`glossary.md`](./glossary.md) §1–§4 拿术语和字段口径 |
| **Phase 0 计费 owner** | [`upstream-cost-snapshot.md`](./upstream-cost-snapshot.md) — 上游成本两态语义、Shadow 期策略 | [`glossary.md`](./glossary.md) §3.1 拿 7 个字段清单；[`sprint-plan.md`](./sprint-plan.md) 拿 P0-1～P0-6 PR 列表 |
| **Phase 1 / 5 Generic owner** | [`generic-channel-design.md`](./generic-channel-design.md) — Phase 1 轻量接入 + Phase 5 多 endpoint | [`glossary.md`](./glossary.md) §1.3 拿 provider 别名表；§4 拿 endpoint 结构 |
| **PM / Sprint 排期** | [`sprint-plan.md`](./sprint-plan.md) — 31 PR + Sprint 编排 | [`glossary.md`](./glossary.md) §2 拿总工期与 Phase 简表 |
| **改字段名 / Phase 编号 / 术语取值** | [`glossary.md`](./glossary.md) — **只在这里改** | 改完后同步设计文档与 sprint-plan 的引用 |

## 维护规约

1. **原子事实只在 `glossary.md` 改一次**——字段名、Phase 编号、工期、protocol 取值、provider 别名表、代码定位锚点。设计文档与 sprint-plan 引用 glossary，不重复定义。
2. **新增字段先登记**——加新 UsageLog 列、新 protocol 取值、新 provider 别名前，先 PR 改 glossary，再 PR 改其他文档。
3. **代码定位锚点统一在 `glossary.md` §5**——设计文档需要引用 `file:line` 时也指向 §5，确保代码移动后单点更新。

## 文档结构

```
docs/
├── README.md                          ← 本文,角色导航
├── glossary.md                        单一权威源(术语 / 字段 / Phase / 锚点)
├── relay-architecture-design.md       顶层架构设计
├── generic-channel-design.md          Generic Channel 子系统设计
├── upstream-cost-snapshot.md          上游成本快照子系统设计
└── sprint-plan.md                     31 PR 执行视图
```

## 相关文档（不属于本次重构范围）

- 支付集成：`PAYMENT.md` / `PAYMENT_CN.md` / `ADMIN_PAYMENT_INTEGRATION_API.md`
- 对账 API：`RECONCILIATION_API_CN.md`
- 业务背景与历史评估：`../claudedocs/`
