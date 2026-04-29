# OpenClaw 对账 API 接口文档

> **服务器地址**：`https://openclaw.zhiguo.fan`  
> **测试账号**：`jp-test1@b517.com`  
> **文档生成时间**：2026-04-20（基于真实接口测试）

---

## 目录

1. [认证：登录获取 Token](#1-认证登录获取-token)
2. [认证：刷新 Token](#2-认证刷新-token)
3. [账户信息：余额查询](#3-账户信息余额查询)
4. [支付订单：查询我的订单](#4-支付订单查询我的订单)
5. [用量明细：查询用量记录](#5-用量明细查询用量记录)
6. [用量统计：汇总数据](#6-用量统计汇总数据)
7. [用量统计：仪表盘总览](#7-用量统计仪表盘总览)
8. [用量统计：按天趋势](#8-用量统计按天趋势)
9. [用量统计：按模型分组](#9-用量统计按模型分组)
10. [附录：典型对账场景](#附录典型对账场景)

---

## 通用说明

### 认证头

所有需要登录的接口均需在请求头中携带 JWT Token：

```
Authorization: Bearer <access_token>
```

### 统一响应格式

```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

| 字段 | 说明 |
|------|------|
| `code` | `0` = 成功，其他 = 错误 |
| `message` | 状态描述 |
| `data` | 业务数据 |

---

## 1. 认证：登录获取 Token

### 接口信息

| 项目 | 内容 |
|------|------|
| **方法** | `POST` |
| **路径** | `/api/v1/auth/login` |
| **认证** | 无需 |

### 请求体

```json
{
  "email": "jp-test1@b517.com",
  "password": "jp20260408"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `email` | string | ✅ | 用户邮箱 |
| `password` | string | ✅ | 登录密码 |
| `turnstile_token` | string | ❌ | Cloudflare 人机验证 token（站点未开启时可省略） |

### curl 示例

```bash
curl -X POST "https://openclaw.zhiguo.fan/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "jp-test1@b517.com",
    "password": "jp20260408"
  }'
```

### 真实响应

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo3LCJlbWFpbCI6ImpwLXRlc3QxQGI1MTcuY29tIiwicm9sZSI6InVzZXIiLCJ0b2tlbl92ZXJzaW9uIjowLCJleHAiOjE3NzY3NTgxNDYsIm5iZiI6MTc3NjY3MTc0NiwiaWF0IjoxNzc2NjcxNzQ2fQ.6g7m-AI88lduAAxfpgW5Nx72Q7r8SCd_g8pYPNmpXR4",
        "refresh_token": "rt_8ea11c96b4bf17516eb80570b4495a9b2d4f9efe3c82b6c3b4dbb1101140307c",
        "expires_in": 86400,
        "token_type": "Bearer",
        "user": {
            "id": 7,
            "email": "jp-test1@b517.com",
            "username": "jptest1",
            "role": "user",
            "balance": 190.72021231,
            "concurrency": 200,
            "status": "active",
            "created_at": "2026-04-08T22:03:25.97024+08:00",
            "total_recharged": 0
        }
    }
}
```

### 响应字段说明

| 字段 | 说明 |
|------|------|
| `access_token` | **主 Token**，用于所有接口认证，有效期 `expires_in` 秒 |
| `refresh_token` | 刷新 Token，access_token 过期后用于换新 Token |
| `expires_in` | access_token 有效期（秒），此处为 `86400`（24小时） |
| `token_type` | 固定为 `Bearer` |
| `user.balance` | 当前余额（USD） |
| `user.total_recharged` | 累计充值总额 |

---

## 2. 认证：刷新 Token

当 `access_token` 过期时，用 `refresh_token` 换取新 Token，**无需重新登录**。

### 接口信息

| 项目 | 内容 |
|------|------|
| **方法** | `POST` |
| **路径** | `/api/v1/auth/refresh` |
| **认证** | 无需 |

### 请求体

```json
{
  "refresh_token": "rt_8ea11c96b4bf17516eb80570b4495a9b2d4f9efe3c82b6c3b4dbb1101140307c"
}
```

### curl 示例

```bash
curl -X POST "https://openclaw.zhiguo.fan/api/v1/auth/refresh" \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "rt_8ea11c96b4bf17516eb80570b4495a9b2d4f9efe3c82b6c3b4dbb1101140307c"
  }'
```

### 真实响应

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo3...",
        "refresh_token": "rt_ac73f155a271f3aac2d15a8ef7cc7ee7055212b4a0e830e14c2d161c5fdcad54",
        "expires_in": 86400,
        "token_type": "Bearer"
    }
}
```

> **注意**：每次刷新都会返回一个新的 `refresh_token`，旧的立即失效，请务必保存最新的。

---

## 3. 账户信息：余额查询

### 接口信息

| 项目 | 内容 |
|------|------|
| **方法** | `GET` |
| **路径** | `/api/v1/auth/me` |
| **认证** | ✅ 需要 Bearer Token |

### curl 示例

```bash
curl -X GET "https://openclaw.zhiguo.fan/api/v1/auth/me" \
  -H "Authorization: Bearer <access_token>"
```

### 真实响应

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "id": 7,
        "email": "jp-test1@b517.com",
        "username": "jptest1",
        "role": "user",
        "balance": 190.72021231,
        "concurrency": 200,
        "status": "active",
        "allowed_groups": null,
        "created_at": "2026-04-08T22:03:25.97024+08:00",
        "updated_at": "2026-04-20T15:47:21.923944+08:00",
        "balance_notify_enabled": true,
        "balance_notify_threshold_type": "fixed",
        "balance_notify_threshold": null,
        "balance_notify_extra_emails": null,
        "total_recharged": 0,
        "run_mode": "standard"
    }
}
```

### 对账关键字段

| 字段 | 说明 |
|------|------|
| `balance` | 当前可用余额（USD），示例值：`190.72021231` |
| `total_recharged` | 历史累计充值总额 |
| `concurrency` | 最大并发请求数 |
| `status` | 账号状态：`active` 正常 |

---

## 4. 支付订单：查询我的订单

### 接口信息

| 项目 | 内容 |
|------|------|
| **方法** | `GET` |
| **路径** | `/api/v1/payment/orders/my` |
| **认证** | ✅ 需要 Bearer Token |

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 `1` |
| `page_size` | int | ❌ | 每页数量，默认 `20` |
| `status` | string | ❌ | 订单状态过滤（见下表） |
| `order_type` | string | ❌ | `balance`（余额充值）/ `plan`（套餐订阅） |
| `payment_type` | string | ❌ | `alipay` / `wxpay` / `stripe` 等 |

#### 订单状态（status）说明

| 值 | 含义 |
|----|------|
| `PENDING` | 待支付，等待用户完成支付 |
| `PAID` | 已支付，等待系统到账 |
| `COMPLETED` | ✅ 已完成，余额已到账 |
| `EXPIRED` | 已过期，超时未支付 |
| `CANCELLED` | 已取消 |
| `FAILED` | 充值失败 |
| `REFUND_REQUESTED` | 已申请退款 |
| `REFUNDING` | 退款处理中 |
| `REFUNDED` | 已退款 |

### curl 示例

```bash
# 查询所有订单（分页）
curl -X GET "https://openclaw.zhiguo.fan/api/v1/payment/orders/my?page=1&page_size=10" \
  -H "Authorization: Bearer <access_token>"

# 仅查已完成的充值订单
curl -X GET "https://openclaw.zhiguo.fan/api/v1/payment/orders/my?status=COMPLETED&order_type=balance" \
  -H "Authorization: Bearer <access_token>"
```

### 真实响应

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "items": [],
        "total": 0,
        "page": 1,
        "page_size": 5,
        "pages": 1
    }
}
```

### items 内单条订单字段说明

| 字段 | 说明 |
|------|------|
| `id` | 系统订单 ID |
| `out_trade_no` | 平台订单号 |
| `amount` | 订单金额 |
| `pay_amount` | 实际支付金额 |
| `payment_type` | 支付方式 |
| `status` | 订单状态 |
| `created_at` | 下单时间 |

---

## 5. 用量明细：查询用量记录

### 接口信息

| 项目 | 内容 |
|------|------|
| **方法** | `GET` |
| **路径** | `/api/v1/usage` |
| **认证** | ✅ 需要 Bearer Token |

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 `1` |
| `page_size` | int | ❌ | 每页数量 |
| `start_date` | string | ❌ | 开始日期，格式 `YYYY-MM-DD` |
| `end_date` | string | ❌ | 结束日期，格式 `YYYY-MM-DD` |
| `timezone` | string | ❌ | 用户时区，如 `Asia/Shanghai` |
| `model` | string | ❌ | 按模型名过滤，如 `claude-opus-4-6` |
| `api_key_id` | int64 | ❌ | 按指定 API Key ID 过滤 |
| `sort_by` | string | ❌ | 排序字段，默认 `created_at` |
| `sort_order` | string | ❌ | `asc` / `desc`（默认 `desc`） |

### curl 示例

```bash
# 查最新 3 条记录
curl -X GET "https://openclaw.zhiguo.fan/api/v1/usage?page=1&page_size=3&sort_order=desc&timezone=Asia/Shanghai" \
  -H "Authorization: Bearer <access_token>"

# 查本月 claude-opus-4-6 的所有用量
curl -X GET "https://openclaw.zhiguo.fan/api/v1/usage?start_date=2026-04-01&end_date=2026-04-30&model=claude-opus-4-6&timezone=Asia/Shanghai" \
  -H "Authorization: Bearer <access_token>"
```

### 真实响应（单条记录示例）

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "items": [
            {
                "id": 30623,
                "user_id": 7,
                "api_key_id": 18,
                "model": "claude-sonnet-4-6",
                "reasoning_effort": "medium",
                "inbound_endpoint": "/v1/messages",
                "group_id": 8,
                "input_tokens": 13372,
                "output_tokens": 1533,
                "cache_creation_tokens": 0,
                "cache_read_tokens": 5990,
                "input_cost": 0.040116,
                "output_cost": 0.022995,
                "cache_read_cost": 0.001797,
                "total_cost": 0.064908,
                "actual_cost": 0.064908,
                "rate_multiplier": 1,
                "billing_type": 0,
                "request_type": "stream",
                "stream": true,
                "duration_ms": 34196,
                "first_token_ms": 1278,
                "billing_mode": "token",
                "created_at": "2026-04-20T15:25:17.838568+08:00",
                "api_key": {
                    "id": 18,
                    "name": "tokeneasy_prod",
                    "key": "sk-f9e6c6d766a69a42fef4..."
                },
                "group": {
                    "id": 8,
                    "name": "aws",
                    "platform": "anthropic"
                }
            }
        ],
        "total": 2061,
        "page": 1,
        "page_size": 3,
        "pages": 687
    }
}
```

### 单条记录关键字段

| 字段 | 说明 |
|------|------|
| `model` | 使用的模型名 |
| `input_tokens` | 输入 Token 数 |
| `output_tokens` | 输出 Token 数 |
| `cache_read_tokens` | 缓存命中 Token 数 |
| `total_cost` | 本次请求费用（USD） |
| `actual_cost` | 实际扣费（含汇率倍率后） |
| `rate_multiplier` | 费率倍率（`1` = 原价） |
| `duration_ms` | 总请求耗时（毫秒） |
| `first_token_ms` | 首 Token 耗时（毫秒） |
| `billing_mode` | 计费模式：`token`（按 token）|

---

## 6. 用量统计：汇总数据

### 接口信息

| 项目 | 内容 |
|------|------|
| **方法** | `GET` |
| **路径** | `/api/v1/usage/stats` |
| **认证** | ✅ 需要 Bearer Token |

### 查询参数

| 参数 | 说明 |
|------|------|
| `period` | 快捷时段：`today` / `week` / `month`（与日期参数互斥） |
| `start_date` | 自定义起始日期（`YYYY-MM-DD`） |
| `end_date` | 自定义结束日期（`YYYY-MM-DD`） |
| `timezone` | 用户时区 |
| `api_key_id` | 按 API Key 过滤 |

### curl 示例

```bash
# 本月汇总
curl -X GET "https://openclaw.zhiguo.fan/api/v1/usage/stats?period=month&timezone=Asia/Shanghai" \
  -H "Authorization: Bearer <access_token>"

# 自定义日期范围
curl -X GET "https://openclaw.zhiguo.fan/api/v1/usage/stats?start_date=2026-04-01&end_date=2026-04-20&timezone=Asia/Shanghai" \
  -H "Authorization: Bearer <access_token>"
```

### 真实响应

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "total_requests": 2062,
        "total_input_tokens": 7582108,
        "total_output_tokens": 589105,
        "total_cache_tokens": 7831277,
        "total_tokens": 16002490,
        "total_cost": 39.33417765,
        "total_actual_cost": 39.33417765,
        "average_duration_ms": 11512.311833171678
    }
}
```

### 字段说明

| 字段 | 说明 |
|------|------|
| `total_requests` | 总请求次数：`2,062` 次 |
| `total_input_tokens` | 总输入 Token：`7,582,108` |
| `total_output_tokens` | 总输出 Token：`589,105` |
| `total_cache_tokens` | 缓存命中 Token：`7,831,277` |
| `total_tokens` | 全量 Token（含缓存）：`16,002,490` |
| `total_cost` | 总费用：`$39.33` |
| `total_actual_cost` | 实际扣费（同上，含倍率） |
| `average_duration_ms` | 平均响应时长：`11,512ms（约11.5秒）` |

---

## 7. 用量统计：仪表盘总览

一次性返回账号全局统计 + 今日统计，适合概览页。

### 接口信息

| 项目 | 内容 |
|------|------|
| **方法** | `GET` |
| **路径** | `/api/v1/usage/dashboard/stats` |
| **认证** | ✅ 需要 Bearer Token |

### curl 示例

```bash
curl -X GET "https://openclaw.zhiguo.fan/api/v1/usage/dashboard/stats" \
  -H "Authorization: Bearer <access_token>"
```

### 真实响应

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "total_api_keys": 5,
        "active_api_keys": 5,
        "total_requests": 2061,
        "total_input_tokens": 7577065,
        "total_output_tokens": 589099,
        "total_cache_creation_tokens": 2327562,
        "total_cache_read_tokens": 5499071,
        "total_tokens": 15992797,
        "total_cost": 39.27978765,
        "total_actual_cost": 39.27978765,
        "today_requests": 181,
        "today_input_tokens": 851328,
        "today_output_tokens": 25512,
        "today_cache_creation_tokens": 322013,
        "today_cache_read_tokens": 1301835,
        "today_tokens": 2500688,
        "today_cost": 5.1862672,
        "today_actual_cost": 5.1862672,
        "average_duration_ms": 11515.512857836002,
        "rpm": 0,
        "tpm": 0
    }
}
```

### 字段说明

| 字段 | 说明 |
|------|------|
| `total_api_keys` / `active_api_keys` | API Key 总数 / 活跃数：`5 / 5` |
| `today_requests` | 今日请求数：`181` |
| `today_cost` | 今日消耗：`$5.19` |
| `total_cost` | 历史总消耗：`$39.28` |
| `rpm` / `tpm` | 实时每分钟请求数 / Token 数 |

---

## 8. 用量统计：按天趋势

### 接口信息

| 项目 | 内容 |
|------|------|
| **方法** | `GET` |
| **路径** | `/api/v1/usage/dashboard/trend` |
| **认证** | ✅ 需要 Bearer Token |

### 查询参数

| 参数 | 说明 |
|------|------|
| `granularity` | `day`（按天）/ `hour`（按小时） |
| `start_date` | 起始日期（`YYYY-MM-DD`） |
| `end_date` | 结束日期（`YYYY-MM-DD`） |
| `timezone` | 用户时区 |

### curl 示例

```bash
curl -X GET "https://openclaw.zhiguo.fan/api/v1/usage/dashboard/trend?granularity=day&start_date=2026-04-14&end_date=2026-04-20&timezone=Asia/Shanghai" \
  -H "Authorization: Bearer <access_token>"
```

### 真实响应（节选）

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "trend": [
            {
                "date": "2026-04-17",
                "requests": 613,
                "input_tokens": 2072720,
                "output_tokens": 101578,
                "cache_creation_tokens": 1100295,
                "cache_read_tokens": 2257767,
                "total_tokens": 5532360,
                "cost": 14.35987425,
                "actual_cost": 14.35987425
            },
            {
                "date": "2026-04-18",
                "requests": 49,
                "input_tokens": 133453,
                "output_tokens": 405,
                "cache_creation_tokens": 160452,
                "cache_read_tokens": 193579,
                "total_tokens": 487889,
                "cost": 1.7625245,
                "actual_cost": 1.7625245
            },
            {
                "date": "2026-04-20",
                "requests": 181,
                "input_tokens": 851328,
                "output_tokens": 25512,
                "cache_creation_tokens": 322013,
                "cache_read_tokens": 1301835,
                "total_tokens": 2500688,
                "cost": 5.1862672,
                "actual_cost": 5.1862672
            }
        ],
        "start_date": "2026-04-14",
        "end_date": "2026-04-20",
        "granularity": "day"
    }
}
```

---

## 9. 用量统计：按模型分组

### 接口信息

| 项目 | 内容 |
|------|------|
| **方法** | `GET` |
| **路径** | `/api/v1/usage/dashboard/models` |
| **认证** | ✅ 需要 Bearer Token |

### curl 示例

```bash
curl -X GET "https://openclaw.zhiguo.fan/api/v1/usage/dashboard/models?start_date=2026-04-01&end_date=2026-04-20&timezone=Asia/Shanghai" \
  -H "Authorization: Bearer <access_token>"
```

### 真实响应（节选，按费用排序）

```json
{
    "code": 0,
    "message": "success",
    "data": {
        "start_date": "2026-04-01",
        "end_date": "2026-04-20",
        "models": [
            {
                "model": "claude-opus-4-6",
                "requests": 983,
                "input_tokens": 1779693,
                "output_tokens": 222636,
                "cache_creation_tokens": 1066054,
                "cache_read_tokens": 1592120,
                "total_tokens": 4660503,
                "cost": 21.9232625,
                "actual_cost": 21.9232625,
                "account_cost": 21.9232625
            },
            {
                "model": "claude-sonnet-4-6",
                "requests": 526,
                "input_tokens": 1270132,
                "output_tokens": 171525,
                "total_tokens": 3982139,
                "cost": 9.6951864,
                "actual_cost": 9.6951864,
                "account_cost": 9.6951864
            },
            {
                "model": "gpt-5.1-codex-max",
                "requests": 88,
                "input_tokens": 3256993,
                "output_tokens": 49253,
                "total_tokens": 4770862,
                "cost": 4.7155145,
                "actual_cost": 4.7155145,
                "account_cost": 4.7155145
            },
            {
                "model": "claude-haiku-4-5-20251001",
                "requests": 348,
                "input_tokens": 971164,
                "output_tokens": 117763,
                "total_tokens": 1902750,
                "cost": 1.9333762,
                "actual_cost": 1.9333762,
                "account_cost": 1.9333762
            }
        ]
    }
}
```

### 字段说明

| 字段 | 说明 |
|------|------|
| `model` | 模型名 |
| `requests` | 该模型调用次数 |
| `cost` | 该模型总费用（USD） |
| `account_cost` | 账号实际扣费（同 `actual_cost`） |

---

## 附录：典型对账场景

### 场景一：用户自助核查本月消耗

```bash
# Step 1: 登录获取 Token
TOKEN=$(curl -s -X POST "https://openclaw.zhiguo.fan/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"jp-test1@b517.com","password":"jp20260408"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['access_token'])")

# Step 2: 查当前余额
curl -s "https://openclaw.zhiguo.fan/api/v1/auth/me" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

# Step 3: 查本月消耗统计
curl -s "https://openclaw.zhiguo.fan/api/v1/usage/stats?period=month&timezone=Asia/Shanghai" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

# Step 4: 查充值记录
curl -s "https://openclaw.zhiguo.fan/api/v1/payment/orders/my?status=COMPLETED" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
```

### 场景二：按模型核查费用构成

```bash
# 查本月各模型消费占比
curl -s "https://openclaw.zhiguo.fan/api/v1/usage/dashboard/models?start_date=2026-04-01&end_date=2026-04-30&timezone=Asia/Shanghai" \
  -H "Authorization: Bearer $TOKEN"
```
