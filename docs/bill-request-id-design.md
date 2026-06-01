# Bill-Request-ID 下游对账标识设计方案

> **状态**：设计方案（未实现，待评审）
> **作者**：zhiguofan
> **日期**：2026-06-01
> **关联文档**：[`RECONCILIATION_API_CN.md`](RECONCILIATION_API_CN.md)

---

## 1. 背景与问题

### 1.1 现状：响应头 `X-Request-Id` 不可控

网关对外（下游 = 调用本平台的客户/应用）返回的 `X-Request-Id` **不是下游自己发的那个**，而是被上游 AI 服务商的 `x-request-id` 覆盖。链路如下：

1. 入口中间件 [`RequestLogger()`](../backend/internal/server/middleware/request_logger.go) 会读下游请求里的 `X-Request-ID`（没有则生成 UUID），并回写到响应头 —— 这一步本来符合下游预期。
2. 但网关转发接口（`/v1/messages`、`/v1/chat/completions` 等）在拿到上游响应后，又用上游 `x-request-id` 覆盖：

   ```go
   // backend/internal/service/gateway_service.go:5462
   if v := resp.Header.Get("x-request-id"); v != "" {
       c.Header("x-request-id", v)   // 底层 Set，覆盖中间件写的下游 ID
   }
   ```

   这种写法在 openai / claude / gemini / antigravity / bedrock 各平台共有数十处。HTTP 头大小写不敏感，`X-Request-ID` 与 `x-request-id` 规范化为同一 canonical key `X-Request-Id`，**后写覆盖先写**。

3. 此外白名单透传 [`responseheaders.go:25`](../backend/internal/util/responseheaders/responseheaders.go#L25) 也会把上游 `x-request-id` 用 `Add` 追加，可能导致响应里出现两个 `X-Request-Id`。

**结论**：下游无法依赖 `X-Request-Id` 做端到端追踪和对账，因为它的值由上游决定、对下游不稳定也不唯一可控。

### 1.2 现状：账单记录的 request_id 也是上游 ID

`usage_log.request_id`（[schema](../backend/ent/schema/usage_log.go#L38)，`MaxLen 64, NotEmpty`）落库的同样是**上游** `x-request-id`。例如 [`antigravity_gateway_service.go:1714`](../backend/internal/service/antigravity_gateway_service.go#L1714) 取上游头 → :1744 写入 `UsageLog.RequestID`。

因此现有对账接口 `/api/v1/usage`（见 RECONCILIATION_API_CN.md §5）即便支持按 request_id 查询，下游持有的也只能是上游 ID，无法用**自己生成或本平台返回的稳定 ID** 反查账单。

### 1.3 已确认不存在的字段

代码中 **不存在** `X-Oneapi-Request-Id` / `oneapi-request` / `one-api` 等任何 OneAPI 风格的请求头（已全仓搜索确认）。当前写回下游的自定义头仅有：`x-request-id`、`X-Idempotency-Replayed`、`X-Idempotency-Degraded`、`X-Snapshot-Cache`、`X-Accel-Buffering` 等。

### 1.4 已有但未对外的资产：`client_request_id`

中间件 [`ClientRequestID()`](../backend/internal/server/middleware/client_request_id.go) 为每个请求生成一个 UUID，存入 `ctxkey.ClientRequestID`，贯穿日志与 Ops 监控模块（`ops_repo_request_details` 等）做端到端关联。但它：

- 只通过 `X-Client-Request-ID` 头返回，**语义是 Ops 排障**，不是对账；
- **未落库到 `usage_log`**，因此无法直接作为账单对账键。

---

## 2. 设计目标

引入一个专用于**网关 ↔ 下游对账**的响应头 `Bill-Request-ID`，满足：

| 目标 | 说明 |
|------|------|
| **双方共持（核心）** | 由下游在请求头上传，网关**原样回写**并落库 —— 同一个 ID 下游与平台都持有，对账闭环 |
| **缺省兜底** | 下游未上传（或值非法）时，平台生成兜底 ID 并回写，保证字段始终有值、链路不断 |
| **绝不被上游覆盖** | 使用独立头名，与网关对 `x-request-id` 的写入隔离 |
| **可反查账单** | 落库到 `usage_log`，下游凭此 ID 调对账 API 查到对应计费明细 |
| **职责分离** | 与 `X-Request-Id`（上游覆盖）、`X-Client-Request-ID`（Ops 排障）解耦，互不干扰 |

### 2.1 三个 ID 的职责对照（目标态）

| 响应头 | 值来源 | 唯一性/稳定性 | 用途 | 是否落 usage_log |
|--------|--------|---------------|------|------------------|
| `X-Request-Id` | 下游传入，缺省时上游覆盖 | 不稳定（被上游覆盖） | 兼容上游调试 | 是（存上游值，现状不变） |
| `X-Client-Request-ID` | 网关生成 UUID | 唯一 | Ops 端到端排障 | 否 |
| **`Bill-Request-ID`（新增）** | **下游上传**，缺省时网关兜底生成；**原样回写** | 双方共持、稳定 | **下游对账** | **是（新增字段）** |

> 注：HTTP 头规范化后 `Bill-Request-ID` 的 canonical key 为 `Bill-Request-Id`，下游读取时应大小写不敏感。

---

## 3. 方案设计

### 3.1 值定义：下游上传优先，缺省兜底

`Bill-Request-ID` 采用"**上传—回写**"语义，与 `RequestLogger` 处理 `X-Request-ID` 的模式一致，但用独立头名并落库：

1. **下游上传**：下游在请求头携带 `Bill-Request-ID`（如其内部订单号/调用流水号）。网关校验清洗后**原样回写**到响应头，并落库。这样下游与平台持有**同一个**对账键，双向可核对。
2. **缺省兜底**：下游未传或值非法时，网关回退使用 `client_request_id`（兜底 UUID）作为该请求的 `Bill-Request-ID`，回写并落库 —— 保证字段始终非空、对账记录不断链。此种情况下游虽未预先记录该 ID，但仍能从响应头取得。

**校验/清洗规则**（防止脏数据与越界）：

- `strings.TrimSpace`；
- 长度上限 64（与 `usage_log.bill_request_id` 字段 `MaxLen 64` 对齐），超长则视为非法 → 走兜底（不建议静默截断，截断后的值与下游持有的不一致，反而破坏对账）；
- 空字符串 → 走兜底。
- 字符集：HTTP 头值本身已排除控制字符，落库 `VARCHAR(64)` 无注入风险，**不额外做白名单**（YAGNI）。

> 唯一性由下游负责。平台不保证下游上传值的全局唯一（不同下游可能撞值），因此该列**不加 UNIQUE 约束**，仅靠索引点查 —— 详见 §5。

### 3.2 响应头写回：在中间件统一处理，规避覆盖

在 `ClientRequestID()` 中间件中，读取并回写一个独立的 `Bill-Request-ID` 头，同时把最终生效值存入新的 ctx key：

```go
// backend/internal/server/middleware/client_request_id.go（示意）
const billRequestIDHeader = "Bill-Request-ID"

c.Header(clientRequestIDHeader, id)              // X-Client-Request-ID（现有）

billID := strings.TrimSpace(c.GetHeader(billRequestIDHeader))
if billID == "" || len(billID) > 64 {
    billID = id                                  // 兜底：复用 client_request_id
}
c.Header(billRequestIDHeader, billID)            // 原样回写下游上传值 / 兜底值
ctx = context.WithValue(ctx, ctxkey.BillRequestID, billID)
```

要点：

- 头名独立（`Bill-Request-Id` ≠ `X-Request-Id`），网关后续对 `x-request-id` 的 `Set`/`Add` **不会触及**它，从根本上避免 §1.1 的覆盖问题，**无需改动数十处网关写头代码**。
- 新增 `ctxkey.BillRequestID`：因为生效值可能是**下游上传值**（≠ client_request_id），不能直接复用 `ctxkey.ClientRequestID`，必须单独存放供记账读取。

> 注意：`ClientRequestID()` 当前仅挂在网关路由组（[`routes/gateway.go:29`](../backend/internal/server/routes/gateway.go#L29)）。读取下游请求头同样只在网关路由生效；对账场景只关心计费请求，**维持仅网关路由即可**。

### 3.3 落库：`usage_log` 新增 `bill_request_id`

为支持反查，需把 `Bill-Request-ID` 持久化到账单记录。

**Ent schema 变更**（[`ent/schema/usage_log.go`](../backend/ent/schema/usage_log.go)）新增字段：

```go
field.String("bill_request_id").
    MaxLen(64).
    Optional().
    Comment("下游对账标识：下游上传值，缺省时回退 client_request_id"),
```

- 设为 `Optional`：历史数据无此值，保证向后兼容。
- 改完执行 `go generate ./ent` 并提交生成代码。
- 新增迁移文件 `backend/migrations/NNN_usage_log_bill_request_id.sql`：`ALTER TABLE usage_logs ADD COLUMN bill_request_id VARCHAR(64);` + 在该列建索引（对账按此列点查，需索引）。

**写入链路**：在记账（`RecordUsage` / `UsageLog` 组装）处，从 ctx 取 `ctxkey.BillRequestID`（§3.2 中间件写入的最终生效值）填入 `UsageLog.BillRequestID`。`UsageLog` 结构体（[`service/usage_log.go:99`](../backend/internal/service/usage_log.go#L99) 附近）需新增 `BillRequestID string` 字段，并在 repository 写库处（[`usage_log_repo.go`](../backend/internal/repository/usage_log_repo.go) 的 `SetRequestID` 同侧）补 `SetNillableBillRequestID`。

### 3.4 对账 API 扩展

在 `/api/v1/usage`（RECONCILIATION_API_CN.md §5）查询参数中新增可选过滤：

| 参数 | 类型 | 说明 |
|------|------|------|
| `bill_request_id` | string | 按下游对账标识精确查询单条计费记录 |

并在响应 item 中回吐 `bill_request_id` 字段，使下游能把"我发起的某次调用"与"本平台某条账单"精确对齐。

> 实现上需在 usage 查询的 repository where 条件、DTO mapper（[`handler/dto/mappers.go`](../backend/internal/handler/dto/mappers.go)）补该字段。

---

## 4. 改动点清单

| # | 文件 | 改动 | 类型 |
|---|------|------|------|
| 1 | `backend/internal/pkg/ctxkey/*.go` | 新增 `BillRequestID` ctx key | 必须 |
| 2 | `backend/internal/server/middleware/client_request_id.go` | 读下游 `Bill-Request-ID` 请求头 → 校验/兜底 → 回写响应头 + 存 ctx | 必须 |
| 3 | `backend/ent/schema/usage_log.go` | 新增 `bill_request_id` 字段，`go generate ./ent` | 必须 |
| 4 | `backend/migrations/NNN_usage_log_bill_request_id.sql` | 加列 + 建索引 | 必须 |
| 5 | `backend/internal/service/usage_log.go` | `UsageLog` 结构体新增 `BillRequestID` | 必须 |
| 6 | 记账组装处（各 gateway service 的 `RecordUsage` 调用） | 从 `ctxkey.BillRequestID` 取值填入 | 必须 |
| 7 | `backend/internal/repository/usage_log_repo.go` | 写库 `SetNillableBillRequestID`、读库回填、查询 where | 必须 |
| 8 | `backend/internal/handler/dto/mappers.go` + usage handler | `/api/v1/usage` 新增 `bill_request_id` 过滤与回吐 | 必须 |
| 9 | `docs/RECONCILIATION_API_CN.md` | 文档补充新参数与响应字段 | 必须 |
| 10 | 各 test stub / 单测 | 接口/结构体变更后补齐断言 | 必须 |

> ⚠️ 改动 #6/#7 涉及多个 gateway service 的记账分支，需逐一覆盖（claude/openai/gemini/antigravity/bedrock/images/embeddings），避免某些路径漏填 `bill_request_id` 导致对账缺口。

---

## 5. 兼容性与边界

- **向后兼容**：`bill_request_id` 为 `Optional`，历史 `usage_log` 行为 `NULL`，对账接口对历史数据返回空值，不报错。
- **不影响现有 `X-Request-Id` 行为**：本方案不改动网关对 `x-request-id` 的写法，上游调试链路保持原样。
- **下游读取**：HTTP 头大小写不敏感，下游须以 `bill-request-id` 不区分大小写读写。
- **缺失场景**：非网关接口（不挂 `ClientRequestID()` 中间件）不读取/返回该头；这是预期，对账只覆盖计费请求。
- **唯一性由下游负责**：下游上传值可能撞值（不同下游、或下游 bug 复用），加之一次下游请求可能因重试/fallback/分段计费在内部产生多条 usage_log，因此 `bill_request_id` 列**不加 `UNIQUE` 约束**，允许同值多行，靠 `(bill_request_id)` 普通索引点查。对账时若一个 ID 命中多行，应聚合返回（属正常场景，需在对账文档中说明）。
- **兜底值的对账语义**：下游未上传时回写的兜底 UUID，下游事前并未记录，只能"事后从响应头取得"。如要求严格双向对账，应推动下游**总是上传** `Bill-Request-ID`。

## 6. 备选：平台生成而非下游上传（暂不采用）

最初设想由平台统一生成 `bill_request_id`（复用 client_request_id）。优点是值规整、平台完全可控；缺点是**下游事前不持有该 ID**，无法做到真正的双向对账，只能事后从响应头取。本方案采纳"下游上传 + 兜底"正是为了补齐这一闭环。若某些下游无法改造上传，则对其退化为平台兜底生成，与备选方案等价。

## 7. 验证计划

实现后至少覆盖：

1. **上传—回写一致**：下游请求带 `Bill-Request-ID: abc123`，断言响应头 `Bill-Request-Id` 原样为 `abc123`，且与上游 `x-request-id` 互不影响。
2. **缺省兜底**：下游不带该头，断言响应头 `Bill-Request-Id` 非空（等于 `X-Client-Request-ID` 兜底值）。
3. **非法值兜底**：下游带超长（>64）或空白值，断言走兜底、不写入脏值。
4. **不被上游覆盖**：构造网关返回上游 `x-request-id` 的场景，断言 `X-Request-Id` 为上游值、`Bill-Request-Id` 保持下游上传/兜底值。
5. **落库一致**：一次计费请求后，`usage_log.bill_request_id` 等于响应头返回值。
6. **对账 API**：`GET /api/v1/usage?bill_request_id=<id>` 能查到对应记录（含一个 ID 命中多行的聚合场景）。

> 以上为设计阶段计划，尚未实现与运行，结论待编码后以测试验证。
