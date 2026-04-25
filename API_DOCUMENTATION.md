# SubPanel API 文档与管理系统功能汇总

## 目录
1. [后台管理系统功能](#后台管理系统功能)
2. [API 网关与客户端接口](#api-网关与客户端接口)

---

## 后台管理系统功能

### 管理员认证与访问

#### 认证方式
- **认证中间件**: `AdminAuthMiddleware`
- **保护方式**: 所有管理路由都使用 `/api/v1/admin` 前缀，并受 admin 认证中间件保护
- **路由前缀**: `/api/v1/admin`

### 管理功能模块

#### 1. 仪表盘 (Dashboard)
**路由前缀**: `/api/v1/admin/dashboard`

主要端点:
- `GET /snapshot-v2` - 获取仪表盘快照 V2 (关键指标概览)
- `GET /stats` - 获取统计数据
- `GET /realtime` - 获取实时指标
- `GET /trend` - 获取使用趋势
- `GET /models` - 获取模型统计
- `GET /groups` - 获取分组统计
- `GET /api-keys-trend` - 获取 API Key 使用趋势
- `GET /users-trend` - 获取用户使用趋势
- `GET /users-ranking` - 获取用户消费排行
- `POST /users-usage` - 批量获取用户使用情况
- `POST /api-keys-usage` - 批量获取 API Key 使用情况
- `GET /user-breakdown` - 获取用户分类统计
- `POST /aggregation/backfill` - 数据回填

#### 2. 用户管理
**路由前缀**: `/api/v1/admin/users`

主要端点:
- `GET` - 获取用户列表
- `GET /:id` - 获取单个用户
- `POST` - 创建用户
- `PUT /:id` - 更新用户
- `DELETE /:id` - 删除用户
- `POST /:id/balance` - 更新用户余额
  - 操作类型: `set`, `add`, `subtract`
- `GET /:id/api-keys` - 获取用户的 API Keys
- `GET /:id/usage` - 获取用户使用记录
- `GET /:id/balance-history` - 获取余额历史
- `POST /:id/replace-group` - 替换用户分组
- `GET /:id/rpm-status` - 获取用户 RPM 状态
- `GET /:id/attributes` - 获取用户自定义属性
- `PUT /:id/attributes` - 更新用户自定义属性
- `POST /:id/auth-identities` - 绑定身份认证

创建用户请求体:
```json
{
  "email": "user@example.com",
  "password": "securepassword",
  "username": "optional_username",
  "notes": "optional_notes",
  "balance": 0,
  "concurrency": 5,
  "rpm_limit": 60,
  "allowed_groups": [1, 2, 3]
}
```

#### 3. 分组管理 (Groups)
**路由前缀**: `/api/v1/admin/groups`

主要端点:
- `GET` - 获取分组列表
- `GET /all` - 获取所有分组
- `GET /usage-summary` - 获取分组使用汇总
- `GET /capacity-summary` - 获取容量汇总
- `PUT /sort-order` - 更新排序顺序
- `GET /:id` - 获取分组详情
- `POST` - 创建分组
- `PUT /:id` - 更新分组
- `DELETE /:id` - 删除分组
- `GET /:id/stats` - 获取分组统计
- `GET /:id/rate-multipliers` - 获取分组费率倍数
- `PUT /:id/rate-multipliers` - 批量设置费率倍数
- `DELETE /:id/rate-multipliers` - 清除费率倍数
- `PUT /:id/rpm-overrides` - 批量设置 RPM 覆盖
- `DELETE /:id/rpm-overrides` - 清除 RPM 覆盖
- `GET /:id/api-keys` - 获取分组的 API Keys

#### 4. 账号管理 (Accounts - 上游账号)
**路由前缀**: `/api/v1/admin/accounts`

主要功能:
- 管理来自 OpenAI、Anthropic、Google Gemini、Antigravity 等平台的上游账号
- 支持账号池隔离和负载均衡调度

端点:
- `GET` - 获取账号列表
- `GET /:id` - 获取账号详情
- `POST` - 创建账号
- `POST /check-mixed-channel` - 检查混合渠道
- `POST /sync/crs` - 从 CRS 同步账号
- `POST /sync/crs/preview` - 预览 CRS 同步结果
- `PUT /:id` - 更新账号
- `DELETE /:id` - 删除账号
- `POST /:id/test` - 测试账号连接
- `POST /:id/recover-state` - 恢复账号状态
- `POST /:id/refresh` - 刷新账号 (如 OAuth token)
- `POST /:id/set-privacy` - 设置隐私级别
- `POST /:id/refresh-tier` - 刷新用户等级 (OpenAI)
- `GET /:id/stats` - 获取账号统计
- `POST /:id/clear-error` - 清除账号错误
- `GET /:id/usage` - 获取账号使用情况
- `GET /:id/today-stats` - 获取今日统计
- `POST /today-stats/batch` - 批量获取今日统计
- `POST /:id/clear-rate-limit` - 清除速率限制
- `POST /:id/reset-quota` - 重置配额
- `GET /:id/temp-unschedulable` - 获取临时不可调度信息
- `DELETE /:id/temp-unschedulable` - 清除临时不可调度状态
- `POST /:id/schedulable` - 设置可调度
- `GET /:id/models` - 获取账号支持的模型
- `POST /batch` - 批量创建账号
- `GET /data` - 导出账号数据
- `POST /data` - 导入账号数据
- `POST /batch-update-credentials` - 批量更新凭据
- `POST /batch-refresh-tier` - 批量刷新等级
- `POST /bulk-update` - 批量更新
- `POST /batch-clear-error` - 批量清除错误
- `POST /batch-refresh` - 批量刷新
- `GET /antigravity/default-model-mapping` - 获取 Antigravity 默认模型映射

#### 5. OAuth 账号创建与管理
**路由前缀**: `/api/v1/admin/accounts` (OAuth 相关)

OAuth 流程端点:
- `POST /generate-auth-url` - 生成 OAuth 授权 URL
- `POST /generate-setup-token-url` - 生成 Setup Token URL
- `POST /exchange-code` - 用授权码交换 token
- `POST /exchange-setup-token-code` - 用 Setup Token 交换
- `POST /cookie-auth` - Cookie 认证
- `POST /setup-token-cookie-auth` - Setup Token Cookie 认证

OpenAI OAuth:
- `POST /openai/generate-auth-url`
- `POST /openai/exchange-code`
- `POST /openai/refresh-token`
- `POST /openai/accounts/:id/refresh`
- `POST /openai/create-from-oauth`

Gemini OAuth:
- `POST /gemini/oauth/auth-url`
- `POST /gemini/oauth/exchange-code`
- `GET /gemini/oauth/capabilities`

Antigravity OAuth:
- `POST /antigravity/oauth/auth-url`
- `POST /antigravity/oauth/exchange-code`
- `POST /antigravity/oauth/refresh-token`

#### 6. 公告管理 (Announcements)
**路由前缀**: `/api/v1/admin/announcements`

端点:
- `GET` - 获取公告列表
- `POST` - 创建公告
- `GET /:id` - 获取单个公告
- `PUT /:id` - 更新公告
- `DELETE /:id` - 删除公告
- `GET /:id/read-status` - 获取阅读状态

#### 7. 代理管理 (Proxies)
**路由前缀**: `/api/v1/admin/proxies`

端点:
- `GET` - 获取代理列表
- `GET /all` - 获取所有代理
- `GET /data` - 导出代理数据
- `POST /data` - 导入代理数据
- `GET /:id` - 获取代理详情
- `POST` - 创建代理
- `PUT /:id` - 更新代理
- `DELETE /:id` - 删除代理
- `POST /:id/test` - 测试代理
- `POST /:id/quality-check` - 检查代理质量
- `GET /:id/stats` - 获取代理统计
- `GET /:id/accounts` - 获取代理下的账号
- `POST /batch-delete` - 批量删除
- `POST /batch` - 批量创建

#### 8. 卡密/兑换码管理 (Redeem Codes)
**路由前缀**: `/api/v1/admin/redeem-codes`

端点:
- `GET` - 获取卡密列表
- `GET /stats` - 获取卡密统计
- `GET /export` - 导出卡密
- `GET /:id` - 获取单个卡密
- `POST /create-and-redeem` - 创建并立即兑换
- `POST /generate` - 生成卡密
- `DELETE /:id` - 删除卡密
- `POST /batch-delete` - 批量删除
- `POST /:id/expire` - 过期卡密

#### 9. 优惠码管理 (Promo Codes)
**路由前缀**: `/api/v1/admin/promo-codes`

端点:
- `GET` - 获取优惠码列表
- `GET /:id` - 获取单个优惠码
- `POST` - 创建优惠码
- `PUT /:id` - 更新优惠码
- `DELETE /:id` - 删除优惠码
- `GET /:id/usages` - 获取优惠码使用记录

#### 10. 订阅管理 (Subscriptions)
**路由前缀**: `/api/v1/admin/subscriptions`

端点:
- `GET` - 获取订阅列表
- `GET /:id` - 获取订阅详情
- `GET /:id/progress` - 获取订阅进度
- `POST /assign` - 分配订阅
- `POST /bulk-assign` - 批量分配
- `POST /:id/extend` - 延期订阅
- `POST /:id/reset-quota` - 重置配额
- `DELETE /:id` - 撤销订阅
- `GET /groups/:id/subscriptions` - 获取分组订阅
- `GET /users/:id/subscriptions` - 获取用户订阅

#### 11. 使用记录管理 (Usage)
**路由前缀**: `/api/v1/admin/usage`

端点:
- `GET` - 获取使用记录列表
- `GET /stats` - 获取使用统计
- `GET /search-users` - 搜索用户使用情况
- `GET /search-api-keys` - 搜索 API Key 使用情况
- `GET /cleanup-tasks` - 获取清理任务列表
- `POST /cleanup-tasks` - 创建清理任务
- `POST /cleanup-tasks/:id/cancel` - 取消清理任务

#### 12. 系统设置 (Settings)
**路由前缀**: `/api/v1/admin/settings`

主要配置项:
- `GET` - 获取所有设置
- `PUT` - 更新设置
- `POST /test-smtp` - 测试 SMTP 连接
- `POST /send-test-email` - 发送测试邮件

SMTP/邮件配置:
- `smtp_host` - SMTP 服务器地址
- `smtp_port` - SMTP 端口
- `smtp_username` - SMTP 用户名
- `smtp_password` - SMTP 密码
- `smtp_from` - 发件人邮箱
- `smtp_tls` - 是否使用 TLS

支付配置:
- `payment_enabled` - 是否启用支付
- 各支付网关的配置 (Stripe, WeChat, 等)

Admin API Key 管理:
- `GET /admin-api-key` - 获取 Admin API Key
- `POST /admin-api-key/regenerate` - 重新生成
- `DELETE /admin-api-key` - 删除

高级设置:
- `GET /overload-cooldown` - 获取过载冷却设置
- `PUT /overload-cooldown` - 更新过载冷却设置
- `GET /stream-timeout` - 获取流超时设置
- `PUT /stream-timeout` - 更新流超时设置
- `GET /rectifier` - 获取请求整流器设置
- `PUT /rectifier` - 更新请求整流器设置
- `GET /beta-policy` - 获取 Beta 策略设置
- `PUT /beta-policy` - 更新 Beta 策略设置
- `GET /web-search-emulation` - 获取网页搜索模拟设置
- `PUT /web-search-emulation` - 更新网页搜索模拟设置
- `POST /web-search-emulation/test` - 测试网页搜索模拟
- `POST /web-search-emulation/reset-usage` - 重置使用配额

#### 13. 运维监控 (Ops)
**路由前缀**: `/api/v1/admin/ops`

实时监控:
- `GET /concurrency` - 获取并发统计
- `GET /user-concurrency` - 获取用户并发统计
- `GET /account-availability` - 获取账号可用性
- `GET /realtime-traffic` - 获取实时流量

告警规则与事件:
- `GET /alert-rules` - 获取告警规则列表
- `POST /alert-rules` - 创建告警规则
- `PUT /alert-rules/:id` - 更新告警规则
- `DELETE /alert-rules/:id` - 删除告警规则
- `GET /alert-events` - 获取告警事件
- `GET /alert-events/:id` - 获取单个事件
- `PUT /alert-events/:id/status` - 更新事件状态
- `POST /alert-silences` - 创建告警静默

邮件通知配置:
- `GET /email-notification/config` - 获取邮件配置
- `PUT /email-notification/config` - 更新邮件配置

运时设置:
- `GET /runtime/alert` - 获取告警运时设置
- `PUT /runtime/alert` - 更新告警运时设置
- `GET /runtime/logging` - 获取日志配置
- `PUT /runtime/logging` - 更新日志配置
- `POST /runtime/logging/reset` - 重置日志配置

系统日志:
- `GET /system-logs` - 获取系统日志
- `POST /system-logs/cleanup` - 清理系统日志
- `GET /system-logs/health` - 获取日志摄入健康状态

仪表盘:
- `GET /dashboard/snapshot-v2` - 仪表盘快照
- `GET /dashboard/overview` - 概览
- `GET /dashboard/throughput-trend` - 吞吐量趋势
- `GET /dashboard/latency-histogram` - 延迟直方图
- `GET /dashboard/error-trend` - 错误趋势
- `GET /dashboard/error-distribution` - 错误分布
- `GET /dashboard/openai-token-stats` - OpenAI Token 统计

WebSocket:
- `GET /ops/ws/qps` - 实时 QPS 监控 (WebSocket)

#### 14. 数据管理与备份
**路由前缀**: `/api/v1/admin/data-management` 和 `/api/v1/admin/backups`

数据源管理:
- `GET /agent/health` - 数据代理健康检查
- `GET /config` - 获取配置
- `PUT /config` - 更新配置
- `GET /sources/:source_type/profiles` - 获取数据源配置
- `POST /sources/:source_type/profiles` - 创建数据源
- `PUT /sources/:source_type/profiles/:profile_id` - 更新数据源
- `DELETE /sources/:source_type/profiles/:profile_id` - 删除数据源
- `POST /sources/:source_type/profiles/:profile_id/activate` - 激活数据源

S3 存储:
- `GET /s3/profiles` - 获取 S3 配置列表
- `POST /s3/profiles` - 创建 S3 配置
- `PUT /s3/profiles/:profile_id` - 更新 S3 配置
- `DELETE /s3/profiles/:profile_id` - 删除 S3 配置
- `POST /s3/profiles/:profile_id/activate` - 激活 S3 配置
- `POST /s3/test` - 测试 S3 连接

备份操作:
- `GET /backups` - 获取备份列表
- `POST /backups` - 创建备份
- `GET /backups/:id` - 获取备份详情
- `DELETE /backups/:id` - 删除备份
- `GET /backups/:id/download-url` - 获取下载 URL
- `POST /backups/:id/restore` - 恢复备份

定时备份:
- `GET /schedule` - 获取备份计划
- `PUT /schedule` - 更新备份计划

#### 15. 渠道管理 (Channels)
**路由前缀**: `/api/v1/admin/channels`

端点:
- `GET` - 获取渠道列表
- `GET /model-pricing` - 获取模型默认定价
- `GET /:id` - 获取渠道详情
- `POST` - 创建渠道
- `PUT /:id` - 更新渠道
- `DELETE /:id` - 删除渠道

#### 16. 渠道监控 (Channel Monitor)
**路由前缀**: `/api/v1/admin/channel-monitors` 和 `/api/v1/admin/channel-monitor-templates`

监控管理:
- `GET` - 获取监控列表
- `POST` - 创建监控
- `GET /:id` - 获取监控详情
- `PUT /:id` - 更新监控
- `DELETE /:id` - 删除监控
- `POST /:id/run` - 运行监控
- `GET /:id/history` - 获取监控历史

模板管理:
- `GET` - 获取模板列表
- `POST` - 创建模板
- `GET /:id` - 获取模板详情
- `PUT /:id` - 更新模板
- `DELETE /:id` - 删除模板
- `GET /:id/monitors` - 获取关联的监控
- `POST /:id/apply` - 应用模板

#### 17. 用户属性 (User Attributes)
**路由前缀**: `/api/v1/admin/user-attributes`

端点:
- `GET` - 获取属性定义列表
- `POST` - 创建属性定义
- `POST /batch` - 批量获取用户属性值
- `PUT /reorder` - 重新排序
- `PUT /:id` - 更新属性定义
- `DELETE /:id` - 删除属性定义

#### 18. 错误透传规则 (Error Passthrough)
**路由前缀**: `/api/v1/admin/error-passthrough-rules`

端点:
- `GET` - 获取规则列表
- `GET /:id` - 获取单个规则
- `POST` - 创建规则
- `PUT /:id` - 更新规则
- `DELETE /:id` - 删除规则

#### 19. TLS 指纹模板 (TLS Fingerprint)
**路由前缀**: `/api/v1/admin/tls-fingerprint-profiles`

端点:
- `GET` - 获取模板列表
- `GET /:id` - 获取模板详情
- `POST` - 创建模板
- `PUT /:id` - 更新模板
- `DELETE /:id` - 删除模板

#### 20. 定时测试计划 (Scheduled Tests)
**路由前缀**: `/api/v1/admin/scheduled-test-plans`

端点:
- `POST` - 创建测试计划
- `PUT /:id` - 更新计划
- `DELETE /:id` - 删除计划
- `GET /:id/results` - 获取测试结果
- `GET /accounts/:id/scheduled-test-plans` - 获取账号的测试计划

#### 21. 提示词分析 (Prompt Analytics)
**路由前缀**: `/api/v1/admin/prompt-analytics`

端点:
- `GET /top-keywords` - 获取热门关键词 (词云)

#### 22. 支付管理 (Payment Admin)
**路由前缀**: `/api/v1/admin/orders`

端点:
- `GET /dashboard` - 支付仪表盘
- `GET` - 订单列表
- `GET /plans` - 套餐列表

#### 23. API Key 管理 (Admin API Keys)
**路由前缀**: `/api/v1/admin/api-keys`

端点:
- `PUT /:id` - 更新 API Key 分组

---

## API 网关与客户端接口

### 认证方式

#### API Key 认证
客户端向 Gateway API 端点发起请求时，需要在请求头中提供 API Key。支持三种方式:

1. **Authorization Header (推荐)**
```
Authorization: Bearer sk_live_xxxxx
```

2. **x-api-key Header**
```
x-api-key: sk_live_xxxxx
```

3. **x-goog-api-key Header** (Gemini CLI 兼容)
```
x-goog-api-key: sk_live_xxxxx
```

API Key 格式通常为 `sk_live_` 或 `sk_test_` 开头。

### API 端点

#### Claude API 兼容接口

**基础路由** `/v1`

##### Messages 端点
```
POST /v1/messages
```

用于 Claude API 兼容的消息调用。根据 API Key 关联的分组平台自动路由:
- **Anthropic 平台**: 直接调用 Anthropic API
- **OpenAI 平台**: 转换为 OpenAI Responses API 格式

请求体示例 (Anthropic 格式):
```json
{
  "model": "claude-3-5-sonnet-20241022",
  "max_tokens": 1024,
  "messages": [
    {
      "role": "user",
      "content": "Hello, Claude!"
    }
  ],
  "temperature": 1.0,
  "top_p": 1.0,
  "top_k": 0
}
```

响应格式:
```json
{
  "id": "msg_1234567890",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "Hello! I'm Claude, an AI assistant..."
    }
  ],
  "model": "claude-3-5-sonnet-20241022",
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 10,
    "output_tokens": 20,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 0
  }
}
```

**流式调用**:
在请求体中添加 `"stream": true`:
```json
{
  "model": "claude-3-5-sonnet-20241022",
  "max_tokens": 1024,
  "stream": true,
  "messages": [...]
}
```

响应为 Server-Sent Events (SSE) 流:
```
event: content_block_start
data: {"type": "content_block_start", "index": 0, "content_block": {"type": "text"}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Hello"}}

event: message_stop
data: {"type": "message_stop"}
```

##### Token 计数端点
```
POST /v1/messages/count_tokens
```

计算请求的 Token 数量。

请求体:
```json
{
  "model": "claude-3-5-sonnet-20241022",
  "messages": [
    {
      "role": "user",
      "content": "Count these tokens"
    }
  ]
}
```

响应:
```json
{
  "type": "message",
  "id": "ctm_1234567890",
  "input_tokens": 5,
  "cache_creation_input_tokens": 0,
  "cache_read_input_tokens": 0
}
```

##### 模型列表
```
GET /v1/models
```

获取支持的模型列表。

响应格式:
```json
{
  "object": "list",
  "data": [
    {
      "id": "claude-3-5-sonnet-20241022",
      "object": "model",
      "created": 1609459200,
      "owned_by": "anthropic"
    },
    ...
  ]
}
```

支持的模型示例:
- `claude-3-5-sonnet-20241022`
- `claude-3-opus-20250219`
- `claude-3-haiku-20240307`
- `gpt-4`
- `gpt-4-turbo`
- `gpt-3.5-turbo`

##### Usage 端点 (用户可用性查询)
```
GET /v1/usage
```

获取当前 API Key 的配额和使用情况。

#### Responses API (OpenAI 兼容)

**基础路由** `/v1` 或 `/responses`

##### Responses 端点
```
POST /v1/responses
POST /v1/responses/compact
POST /responses
POST /backend-api/codex/responses
```

用于 OpenAI Responses API 调用。仅支持 OpenAI 平台的分组。

请求体示例:
```json
{
  "model": "gpt-4",
  "prompt": "Explain quantum computing",
  "max_tokens": 1024,
  "temperature": 0.7
}
```

##### 图像生成 (Images)
```
POST /v1/images/generations
POST /images/generations
```

用于 DALL-E 图像生成。仅支持 OpenAI 平台。

请求体:
```json
{
  "model": "dall-e-3",
  "prompt": "A serene landscape",
  "n": 1,
  "size": "1024x1024",
  "quality": "standard"
}
```

##### Chat Completions 端点
```
POST /v1/chat/completions
POST /chat/completions
```

用于 OpenAI Chat Completions API 调用。

请求体:
```json
{
  "model": "gpt-4",
  "messages": [
    {
      "role": "user",
      "content": "Hello, how are you?"
    }
  ],
  "temperature": 0.7,
  "max_tokens": 1024,
  "stream": false
}
```

#### Gemini API 兼容

**基础路由** `/v1beta` 或 `/antigravity/v1beta`

##### 模型列表
```
GET /v1beta/models
GET /v1beta/models/:model
```

获取 Gemini 模型列表。

##### 调用端点
```
POST /v1beta/models/*modelAction
```

例如:
- `POST /v1beta/models/gemini-3.1-pro:generateContent`
- `POST /v1beta/models/gemini-3.1-pro:streamGenerateContent`

#### Antigravity 专属路由

**基础路由** `/antigravity/v1` 或 `/antigravity/v1beta`

仅使用 Antigravity 账号，不进行跨平台混合调度。

##### Messages
```
POST /antigravity/v1/messages
POST /antigravity/v1/messages/count_tokens
GET /antigravity/v1/models
GET /antigravity/v1/usage
```

##### Gemini (v1beta)
```
GET /antigravity/v1beta/models
POST /antigravity/v1beta/models/*modelAction
```

##### 模型列表
```
GET /antigravity/models
```

### 支持的模型

#### Anthropic Claude Models
- `claude-3-5-sonnet-20241022` (最新)
- `claude-3-opus-20250219`
- `claude-3-haiku-20240307`
- `claude-3-5-haiku-20241022`

#### OpenAI Models
- `gpt-4` / `gpt-4-turbo`
- `gpt-4o` / `gpt-4-omni`
- `gpt-3.5-turbo`
- `dall-e-2` / `dall-e-3` (图像生成)

#### Google Gemini Models
- `gemini-3.1-pro`
- `gemini-3.1-flash`
- `gemini-3.1-flash-8b`
- `gemini-3-pro`
- `gemini-3-flash`
- `gemini-2.5-pro`
- `gemini-2.5-flash`

#### Antigravity Models
根据 Antigravity 平台的模型支持情况，通常包括多个 Claude 和 Gemini 模型版本。

### 请求头

#### 必需头
- `Authorization: Bearer <API_KEY>` 或 `x-api-key: <API_KEY>`
- `Content-Type: application/json`

#### 可选头
- `X-Request-ID` - 自定义请求 ID，用于追踪
- `User-Agent` - 客户端标识

### 错误处理

#### 通用错误响应格式

Anthropic/Claude API 兼容格式:
```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid API key"
  }
}
```

OpenAI API 兼容格式:
```json
{
  "error": {
    "message": "Invalid API key",
    "type": "invalid_request_error",
    "param": null,
    "code": "invalid_api_key"
  }
}
```

#### 常见错误码

| HTTP 状态 | 错误类型 | 说明 |
|----------|--------|------|
| 401 | authentication_error / INVALID_API_KEY | API Key 无效或过期 |
| 401 | API_KEY_REQUIRED | 缺少 API Key |
| 403 | permission_error / ACCESS_DENIED | 权限不足 |
| 400 | invalid_request_error | 请求参数无效 |
| 429 | rate_limit_error | 超过速率限制 (RPM/RPS) |
| 500 | api_error | 服务器内部错误 |
| 503 | unavailable_error | 服务不可用 |

### 流式处理

支持流式响应的端点会在请求中包含 `"stream": true`。

SSE 流格式:
```
event: content_block_start
data: {"type": "content_block_start", ...}

event: content_block_delta  
data: {"type": "content_block_delta", "delta": {"type": "text_delta", "text": "..."}}

event: message_delta
data: {"type": "message_delta", "delta": {"stop_reason": "end_turn"}}

event: message_stop
data: {"type": "message_stop"}
```

### 计费与配额

#### 使用统计
- 每个请求都会根据输入/输出 token 数计费
- 支持缓存 Token (`cache_creation_input_tokens`, `cache_read_input_tokens`)
- 用户/分组级别的配额限制
- 支持每分钟请求限制 (RPM) 和每分钟 Token 数限制

#### 查询用量
```
GET /v1/usage
Authorization: Bearer <API_KEY>
```

返回当前 API Key 的配额和使用情况。

---

## 前端管理界面路由

### 管理员页面
- `/admin/dashboard` - 仪表盘
- `/admin/ops` - 运维监控
- `/admin/users` - 用户管理
- `/admin/groups` - 分组管理
- `/admin/channels/pricing` - 渠道管理
- `/admin/channels/monitor` - 渠道监控
- `/admin/accounts` - 上游账号管理
- `/admin/subscriptions` - 订阅管理
- `/admin/announcements` - 公告管理
- `/admin/proxies` - 代理管理
- `/admin/redeem` - 卡密管理
- `/admin/promo-codes` - 优惠码管理
- `/admin/settings` - 系统设置
- `/admin/usage` - 使用记录
- `/admin/prompt-analytics` - 提示词分析
- `/admin/orders/dashboard` - 支付仪表盘
- `/admin/orders` - 订单管理
- `/admin/orders/plans` - 套餐管理

### 用户页面
- `/dashboard` - 用户仪表盘
- `/keys` - API Keys 管理
- `/usage` - 使用记录
- `/redeem` - 兑换卡密
- `/affiliate` - 邀请返利
- `/available-channels` - 可用渠道
- `/profile` - 个人资料
- `/subscriptions` - 我的订阅
- `/purchase` - 购买订阅
- `/orders` - 我的订单
- `/models` - 支持的模型

