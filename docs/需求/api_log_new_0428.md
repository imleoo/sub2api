# AI 调用日志查询接口文档

> 本文档已对齐系统现有实现：使用**用户登录账号（JWT）**查询该账号名下所有模型调用明细。
> 直接复用现有接口 `GET /api/v1/usage`，无需新增后端接口。

## 一、基础信息
- 接口地址：`https://XXX/api/v1/usage`
- 请求方式：**GET**（查询参数走 query string，非 POST body）
- 接口描述：查询当前登录用户账号名下的所有模型调用明细（不限单个 API Key）
- 鉴权方式：用户登录态 JWT（**不是** AI 调用用的 `sk-` API Key）
- 响应格式：application/json

## 二、如何获取 Token（JWT）

先用账号密码登录拿 `access_token`：

```bash
curl "https://XXX/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -X POST \
  -d '{"email": "user@example.com", "password": "******"}'
```

响应（节选）：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "..."
  }
}
```

后续请求用 `data.access_token` 作为 Bearer Token。

## 三、请求头（Headers）

| Header | 必填 | 说明 |
|--------|------|------|
| Authorization | 是 | `Bearer {access_token}`，登录返回的 JWT |
| Accept | 否 | application/json |

> **说明**：该接口走用户登录鉴权，服务端从 JWT 解析用户身份，**强制只返回当前用户自己的数据**，无法越权查询他人记录。

## 四、查询参数（Query）

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| start_date | String | 否 | 开始日期，格式 `YYYY-MM-DD`（按 timezone 解析） |
| end_date | String | 否 | 结束日期，格式 `YYYY-MM-DD`，半开区间 `[start_date, end_date+1天)` |
| timezone | String | 否 | 解析日期用的时区，如 `Asia/Shanghai`，缺省用服务端默认 |
| model | String | 否 | 模型名称精确过滤 |
| api_key_id | Integer | 否 | 按指定 API Key 的**数字 ID** 过滤；**不传则返回账号下所有 Key 的调用**。仅允许查询本人名下的 Key，否则返回 403 |
| request_type | String | 否 | 请求类型过滤 |
| stream | Boolean | 否 | 是否流式（`true`/`false`） |
| billing_type | Integer | 否 | 计费类型过滤 |
| page | Integer | 否 | 页码，默认 1 |
| page_size | Integer | 否 | 每页条数，默认 20，最大 1000（也可用 `limit`） |
| sort_by | String | 否 | 排序字段，默认 `created_at` |
| sort_order | String | 否 | 排序方向，默认 `desc` |

> **注意**：原需求草稿用的是 `beginTime/endTime`（秒级时间戳）；现有接口用 `start_date/end_date`（按天的日期字符串）。如确需秒级时间戳过滤，需要另开方案 2（新增 `POST /api/v1/logs` 薄封装）。

## 五、响应数据结构

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [],
    "total": 0,
    "page": 1,
    "page_size": 20,
    "pages": 1
  }
}
```

> **注意**：成功时 `code` 为 **0**（不是 200）；分页字段为 `items/total/page/page_size/pages`。

### items 元素字段说明（节选，对照原需求）

| 原需求字段 | 实际返回字段 | 类型 | 说明 |
|------------|--------------|------|------|
| request_id | `request_id` | string | 调用唯一请求 ID |
| request_time | `created_at` | string | 请求时间，RFC3339（如 `2026-04-28T10:00:00Z`），**非秒级时间戳** |
| model | `model` | string | 调用模型名称 |
| input_token | `input_tokens` | number | 输入 token 数 |
| output_token | `output_tokens` | number | 输出 token 数 |
| cache_write_token | `cache_creation_tokens` | number | 缓存写入 token 数 |
| cache_read_token | `cache_read_tokens` | number | 缓存读取 token 数 |
| amount | `actual_cost` | number | 本次实际消耗费用（美元） |
| api_key | `api_key_id` | number | 本次调用使用的 API Key 的数字 ID（**当前返回 ID，非脱敏 `sk-***` 字符串**） |

> 其余可用字段还包括：`total_cost`、`input_cost`/`output_cost`/`cache_*_cost`、`rate_multiplier`、`request_type`、`stream`、`duration_ms`、`group_id`、`subscription_id` 等。
> `thought_token`（推理思考 token）系统不记录，无此字段。

## 六、错误码

| 错误码（HTTP / 业务 code） | 描述 |
|--------|------|
| 200 / code=0 | 成功 |
| 400 | 请求参数错误（如日期格式非法、api_key_id 非法） |
| 401 | 认证失败（JWT 缺失或无效） |
| 403 | 越权（api_key_id 不属于当前用户） |
| 500 | 服务器内部错误 |

## 七、示例 CURL 请求

```bash
# 查询本账号 2026-04-28 全天、claude 模型、第一页的调用明细
curl "https://XXX/api/v1/usage?start_date=2026-04-28&end_date=2026-04-28&model=claude-3-5-sonnet&page=1&page_size=20&timezone=Asia/Shanghai" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..." \
  -H "Accept: application/json"
```
