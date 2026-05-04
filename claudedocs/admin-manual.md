# We2AI 后台管理员操作手册

> 版本：v1.2 | 更新日期：2026-05-04

---

## 目录

1. [登录与权限](#1-登录与权限)
2. [仪表盘](#2-仪表盘)
3. [用户管理](#3-用户管理)
4. [分组管理](#4-分组管理)
5. [上游账号管理](#5-上游账号管理)
6. [渠道管理](#6-渠道管理)
7. [模型折扣配置](#7-模型折扣配置)
8. [套餐订阅管理](#8-套餐订阅管理)
8. [卡密管理](#8-卡密管理)
9. [优惠码管理](#9-优惠码管理)
10. [公告管理](#10-公告管理)
11. [代理管理](#11-代理管理)
12. [使用记录](#12-使用记录)
13. [运维监控（Ops）](#13-运维监控ops)
14. [提示词分析](#14-提示词分析)
15. [订单与支付](#15-订单与支付)
16. [数据备份](#16-数据备份)
17. [系统设置](#17-系统设置)
18. [系统更新与维护](#18-系统更新与维护)

---

## 1. 登录与权限

### 1.1 访问后台

后台管理地址：`https://你的域名/admin`

### 1.2 管理员账号

- 第一次部署时，通过 Web 安装向导或环境变量 `AUTO_SETUP=true` 创建初始管理员账号
- 管理员账号需要 `is_admin = true` 标记，普通用户无法访问 `/admin` 路由
- 登录方式与普通用户相同（邮箱 + 密码，或配置的第三方 OAuth）

### 1.3 后台模式（Backend Mode）

当系统配置 `RUN_MODE=backend` 时，只有管理员可以登录，普通用户的登录和 Token 刷新均被拒绝，适用于纯后台运维场景。

---

## 2. 仪表盘

路径：`/admin/dashboard`

### 2.1 概览快照

- **今日数据快照**：总请求数、Token 消耗、收入、活跃用户数
- **实时指标**：当前并发数、QPS、延迟分布
- **模型使用统计**：各模型调用量占比
- **分组统计**：各用户分组的使用情况

### 2.2 趋势分析

| 图表 | 说明 |
|------|------|
| 吞吐量趋势 | 按时间段展示请求量变化 |
| 延迟直方图 | P50/P95/P99 延迟分布 |
| 错误率趋势 | 4xx/5xx 错误变化 |
| 错误分布 | 按错误类型分类 |
| Token 统计 | 输入/输出 Token 消耗对比 |

### 2.3 排行榜

- 用户消费排行（按余额消耗）
- API Key 使用排行
- 批量查询用户/Key 使用详情

---

## 3. 用户管理

路径：`/admin/users`

### 3.1 用户列表

支持按邮箱、用户名、注册时间筛选，分页展示所有用户。

### 3.2 创建用户

1. 点击「新建用户」
2. 填写邮箱、用户名、密码
3. 选择所属分组
4. 设置初始余额（可选）
5. 是否设为管理员

### 3.3 编辑用户

可修改：
- 基本信息（邮箱、用户名）
- 所属分组（会影响可用模型和费率）
- 余额（支持增减操作，附备注）
- 管理员权限开关

### 3.4 余额管理

- **充值**：POST `/api/v1/admin/users/:id/balance`，填写金额和备注
- **查看余额历史**：GET `/api/v1/admin/users/:id/balance-history`

### 3.5 API Key 管理

- 查看用户的所有 API Key：`/api/v1/admin/users/:id/api-keys`
- 可变更 Key 所属分组

### 3.6 用户属性

管理员可为用户添加自定义属性（如 VIP 等级、备注等）：
- 路径：`/admin/users/:id` → 用户详情 → 属性标签页
- 属性定义在「系统设置 → 用户属性定义」中维护

### 3.7 RPM 状态

查看用户当前每分钟请求数（RPM）使用情况，用于诊断限流问题。

### 3.8 绑定第三方身份

管理员可手动为用户绑定 OAuth 身份（LinuxDo、OIDC、微信等）：
`POST /api/v1/admin/users/:id/auth-identities`

### 3.9 删除用户

删除前建议先检查用户的订阅和余额情况。删除操作不可恢复。

---

## 4. 分组管理

路径：`/admin/groups`

分组是权限和计费的核心单元，每个用户和 API Key 都属于某个分组。

### 4.1 分组属性

| 属性 | 说明 |
|------|------|
| 名称 | 分组唯一标识 |
| 平台 | `anthropic`（Claude）/ `openai`（GPT）/ `gemini` |
| 费率倍数 | 在基础价格上的乘数（如 1.2 = 120% 原价） |
| RPM 限制 | 每分钟最大请求数 |
| 默认映射模型 | 未指定模型时的默认模型 |
| 并发数限制 | 单用户最大并发请求数 |

### 4.2 创建/编辑分组

1. 路径：`/admin/groups` → 「新建分组」
2. 选择平台（决定可用的 API 类型）
3. 配置费率倍数和 RPM 限制
4. 保存

### 4.3 费率倍数配置

- 全局费率：直接在分组上设置，对所有模型生效
- 模型级费率：`PUT /api/v1/admin/groups/:id/rate-multipliers` 为特定模型配置不同倍率
- 清除模型级费率：`DELETE /api/v1/admin/groups/:id/rate-multipliers`

### 4.4 RPM 覆盖

为特定模型设置不同的 RPM 限制：
- `PUT /api/v1/admin/groups/:id/rpm-overrides`
- `DELETE /api/v1/admin/groups/:id/rpm-overrides`（清除，恢复使用全局配置）

### 4.5 模型调度映射

路径：「分组详情 → 消息调度」

可为特定模型设置分发规则，例如：
- 请求 `claude-sonnet-4` → 实际调度到 `claude-3-5-sonnet-20241022`
- 支持按权重随机分发到多个目标模型

### 4.6 查看分组订阅

`GET /api/v1/admin/groups/:id/subscriptions`

---

## 5. 上游账号管理

路径：`/admin/accounts`

上游账号是实际调用 AI API 的账号池。系统支持四种平台：

| 平台 | 说明 |
|------|------|
| Anthropic | Claude 系列模型原生账号 |
| OpenAI | GPT/DALL-E/Codex 账号 |
| Google Gemini | Gemini 原生账号（AI Studio OAuth） |
| Antigravity | 第三方中转账号，支持 Claude 和 Gemini |

### 5.1 创建账号

**方法一：手动创建**
1. 点击「新建账号」
2. 选择平台类型
3. 填写 API Key 或 Cookie/Token
4. 分配代理（可选）
5. 保存并测试

**方法二：OAuth 流程（Anthropic/OpenAI/Gemini/Antigravity）**
1. 「账号管理」→「OAuth 授权」
2. 生成授权 URL → 在浏览器中完成授权
3. 粘贴回调 Code → 系统自动交换 Token 并创建账号

**方法三：批量导入**
`POST /api/v1/admin/accounts/data` 上传 CSV/JSON 文件批量创建

### 5.2 账号状态管理

| 操作 | 说明 |
|------|------|
| 测试账号 | `POST /accounts/:id/test` 发起实时连接测试 |
| 恢复状态 | `POST /accounts/:id/recover-state` 将错误状态重置为正常 |
| 刷新 Token | `POST /accounts/:id/refresh` 强制刷新 OAuth Token |
| 清除错误 | `POST /accounts/:id/clear-error` 清除账号错误标记 |
| 清除限流 | `POST /accounts/:id/clear-rate-limit` 解除 Rate Limit 状态 |
| 重置配额 | `POST /accounts/:id/reset-quota` 重置使用配额计数 |

### 5.3 调度控制

- **设置可调度**：`POST /accounts/:id/schedulable` 手动启用/禁用账号参与调度
- **临时不可调度**：查看/清除账号的临时排除状态（`GET/DELETE /accounts/:id/temp-unschedulable`）
- **查看今日统计**：`GET /accounts/:id/today-stats` 当日请求量、成功率、延迟

### 5.4 批量操作

| 操作 | 说明 |
|------|------|
| 批量创建 | `POST /accounts/batch` |
| 批量刷新 Token | `POST /accounts/batch-refresh` |
| 批量刷新 Tier | `POST /accounts/batch-refresh-tier` |
| 批量清除错误 | `POST /accounts/batch-clear-error` |
| 批量更新凭据 | `POST /accounts/batch-update-credentials` |
| 批量更新属性 | `POST /accounts/bulk-update` |

### 5.5 账号数据导出/导入

- 导出：`GET /accounts/data` 下载所有账号配置（不含密钥明文）
- 导入：`POST /accounts/data` 批量恢复账号

### 5.6 定时测试计划

为账号配置自动定时测试，及时发现失效账号：
1. 进入「账号详情 → 定时测试」
2. 创建计划：设置 cron 表达式和测试模型
3. 查看历史测试结果

### 5.7 Antigravity 默认模型映射

`GET /api/v1/admin/accounts/antigravity/default-model-mapping`

查看 Antigravity 平台的模型映射配置。

---

## 6. 渠道管理

路径：`/admin/channels/pricing`

渠道是对模型计费规则的自定义封装，可覆盖默认价格。

### 6.1 渠道定价

- 查看各模型默认定价：`GET /admin/channels/model-pricing`
- 创建自定义渠道：设置模型名称匹配规则和价格（输入/输出 Token 单价）
- 支持按模型前缀批量匹配

### 6.2 渠道监控

路径：`/admin/channels/monitor`

**监控模板**：预定义的检测规则集，包含检测 URL、期望响应、超时配置：
1. 「渠道监控 → 模板」→ 新建模板
2. 配置检测请求和成功判断条件
3. 将模板应用到具体监控项

**监控项**：
- 创建：选择模板和监控频率
- 手动触发：`POST /monitors/:id/run`
- 查看历史：`GET /monitors/:id/history`

---

## 7. 模型折扣配置

路径：`/admin/model-discounts`

在后台 UI 界面为每个模型设置折扣率，保存后立即生效，无需重启服务。

### 7.1 操作步骤

1. 进入「模型折扣」页面
2. 在模型列表中找到需要设置折扣的模型（可搜索过滤）
3. 在「折扣率」列输入折扣率（范围 0.1~1.0，如 `0.8` = 8折）
4. 点击「保存配置」

### 7.2 折扣率说明

| 折扣率 | 含义 | 用户实际扣费 |
|--------|------|-------------|
| 1.0（不填）| 原价 | 100% |
| 0.9 | 9折 | 90% |
| 0.8 | 8折 | 80% |
| 0.5 | 5折 | 50% |

### 7.3 人民币汇率

默认汇率为 `7`（1 USD = 7 CNY），可在服务器配置文件中通过 `pricing.cny_rate` 修改（修改后需重启）：

```yaml
pricing:
  cny_rate: 7.2
```

### 7.4 前端展示效果

用户模型列表页（`/models`）会显示：
- 折后美元价格
- 折扣标签（如 `8折`）
- 人民币换算价格（¥X.XX/M tokens）

### 7.5 注意事项

- 折扣同时影响**展示价格**和**实际计费**，两者保持一致
- 折扣与分组费率倍数（`rate_multiplier`）**叠加生效**：最终价格 = 原价 × 折扣率 × 费率倍数
- 模型列表来源于账号白名单中已配置的模型

---

## 8. 套餐订阅管理

路径：`/admin/subscriptions`

订阅是对用户的使用配额授权，绑定到分组。

### 7.1 分配订阅

1. 「订阅管理」→「分配订阅」
2. 选择用户和分组
3. 设置有效期和配额上限（Token 数或请求数）
4. 支持单个或批量分配（`POST /subscriptions/bulk-assign`）

### 7.2 管理已有订阅

| 操作 | 说明 |
|------|------|
| 延期 | `POST /subscriptions/:id/extend` 延长有效期 |
| 重置配额 | `POST /subscriptions/:id/reset-quota` 清零使用量 |
| 撤销 | `DELETE /subscriptions/:id` 立即失效 |
| 查看进度 | `GET /subscriptions/:id/progress` 已用配额占比 |

### 7.3 查看方式

- 按分组查看：`GET /admin/groups/:id/subscriptions`
- 按用户查看：`GET /admin/users/:id/subscriptions`

---

## 8. 卡密管理

路径：`/admin/redeem`

卡密用于用户自助兑换余额或订阅。

### 8.1 生成卡密

1. 路径：「卡密管理」→「批量生成」
2. 设置面值（余额金额或订阅时长）
3. 设置数量和有效期
4. 下载导出

### 8.2 单张卡密操作

- 创建并立即兑换：`POST /codes/create-and-redeem`
- 使卡密过期：`POST /codes/:id/expire`
- 删除：`DELETE /codes/:id`
- 批量删除：`POST /codes/batch-delete`

### 8.3 统计信息

`GET /api/v1/admin/redeem-codes/stats`：已生成/已使用/已过期数量

### 8.4 导出

`GET /api/v1/admin/redeem-codes/export` 导出所有卡密记录

---

## 9. 优惠码管理

路径：`/admin/promo-codes`

优惠码用于折扣购买，与订单系统结合使用。

### 9.1 创建优惠码

- 设置折扣类型（百分比 / 固定金额）
- 设置使用次数上限和有效期
- 支持全场或指定套餐可用

### 9.2 查看使用记录

`GET /promo-codes/:id/usages`

---

## 10. 公告管理

路径：`/admin/announcements`

向所有用户或特定分组发布系统公告。

### 10.1 创建公告

1. 路径：「公告管理」→「新建公告」
2. 填写标题和内容（支持 Markdown）
3. 设置显示类型（弹窗/横幅）
4. 设置有效期

### 10.2 查看已读状态

`GET /announcements/:id/read-status` 查看哪些用户已读

---

## 11. 代理管理

路径：`/admin/proxies`

配置 HTTP/SOCKS5 代理，分配给特定账号使用，适用于需要网络访问限制的场景。

### 11.1 创建代理

1. 路径：「代理管理」→「新建代理」
2. 填写代理地址、端口、用户名密码
3. 保存后可立即测试

### 11.2 代理操作

| 操作 | 说明 |
|------|------|
| 测试连接 | `POST /proxies/:id/test` |
| 质量检测 | `POST /proxies/:id/quality-check` 检测延迟和稳定性 |
| 查看关联账号 | `GET /proxies/:id/accounts` |
| 查看统计 | `GET /proxies/:id/stats` |

### 11.3 批量操作

- 批量创建：`POST /proxies/batch`
- 批量删除：`POST /proxies/batch-delete`
- 导入/导出：支持 CSV 格式

---

## 12. 使用记录

路径：`/admin/usage`

### 12.1 查看调用记录

筛选条件：
- 时间范围
- 用户 / API Key
- 模型
- 请求结果（成功/失败）

### 12.2 统计汇总

`GET /usage/stats`：按维度汇总 Token 消耗、请求量、费用

### 12.3 数据清理

定期清理历史使用记录，释放数据库空间：
1. 「使用记录」→「清理任务」
2. 设置清理范围（时间段和数据类型）
3. 创建任务：`POST /usage/cleanup-tasks`
4. 取消进行中的任务：`POST /cleanup-tasks/:id/cancel`

---

## 13. 运维监控（Ops）

路径：`/admin/ops`

### 13.1 实时流量

- **并发监控**：`GET /ops/concurrency` 当前全局并发数
- **用户并发**：`GET /ops/user-concurrency` 按用户维度
- **账号可用性**：`GET /ops/account-availability`
- **实时流量摘要**：`GET /ops/realtime-traffic`
- **实时 QPS（WebSocket）**：`WS /ops/ws/qps` 推送实时每秒请求数

### 13.2 告警规则

支持基于指标阈值触发告警：
1. 「运维 → 告警规则」→「新建规则」
2. 选择监控指标（错误率、延迟、并发数等）
3. 设置触发阈值和持续时间
4. 配置通知方式（邮件等）

**告警事件处理**：
- 查看告警事件列表：`GET /ops/alert-events`
- 更新事件状态（确认/解决）：`PUT /ops/alert-events/:id/status`
- 创建静默期（抑制告警）：`POST /ops/alert-silences`

**指标阈值全局配置**：`GET/PUT /ops/settings/metric-thresholds`

### 13.3 错误日志

三级错误体系：

| 类型 | 路径 | 说明 |
|------|------|------|
| 请求错误 | `/ops/request-errors` | 用户侧请求失败 |
| 上游错误 | `/ops/upstream-errors` | 上游 API 返回错误 |
| 系统错误 | `/ops/errors` | 内部系统异常 |

每类错误均支持：
- 详情查看
- 手动重试
- 标记解决

### 13.4 系统日志

- 查看日志流：`GET /ops/system-logs`
- 日志清理：`POST /ops/system-logs/cleanup`
- 日志摄入健康：`GET /ops/system-logs/health`

### 13.5 运行时配置

无需重启即可动态调整：
- 告警运行时设置：`GET/PUT /ops/runtime/alert`
- 日志级别：`GET/PUT /ops/runtime/logging`（`POST /reset` 恢复默认）

### 13.6 高级设置

`GET/PUT /ops/advanced-settings`

包含：并发控制参数、流超时、请求整流器（Rectifier）配置、Beta 功能开关等。

### 13.7 邮件通知配置

`GET/PUT /ops/email-notification/config` 设置告警邮件的发件配置（独立于系统 SMTP 设置）。

---

## 14. 提示词分析

路径：`/admin/prompt-analytics`

展示用户提示词的关键词词云，用于分析用户使用场景：

- 显示 Top 关键词（CJK 分词 + 英文词提取）
- 按时间段和分组筛选

---

## 15. 订单与支付

路径：`/admin/orders`

### 15.1 订单管理

- 查看所有订单：`/admin/orders`
- 订单详情：按状态（待支付/已支付/已退款）筛选
- 支付方式：微信支付、支付宝（需在系统设置中配置）

### 15.2 支付套餐

路径：`/admin/orders/plans`

- 创建套餐：设置套餐名称、价格、包含的订阅配额
- 编辑/下架套餐

### 15.3 支付仪表盘

路径：`/admin/orders/dashboard`

- 今日收入
- 订单转化率
- 支付方式分布

---

## 16. 数据备份

路径：`/admin` → 系统设置 → 备份

### 16.1 手动备份

1. 「备份」→「立即备份」
2. 选择备份内容（数据库/配置）
3. 等待完成后下载

### 16.2 自动备份计划

`GET/PUT /api/v1/admin/backup/schedule`

配置 cron 表达式，设置自动备份频率（建议每日一次）。

### 16.3 备份存储（S3）

支持对接兼容 S3 的对象存储：
1. 「备份 → S3 配置」填写 Endpoint、AccessKey、Bucket
2. `POST /backup/s3-config/test` 测试连接
3. 保存后备份文件自动上传到 S3

### 16.4 恢复备份

`POST /backup/:id/restore` 从指定备份点恢复（**高危操作**，会覆盖当前数据）。

### 16.5 数据管理（高级）

路径：`/api/v1/admin/data-management`

- 配置多数据源（PostgreSQL 主从等）
- 数据源切换和健康检查

---

## 17. 系统设置

路径：`/admin/settings`

### 17.1 基本设置

- 站点名称、Logo
- 注册方式（开放注册 / 邀请制 / 关闭）
- 默认分组

### 17.2 SMTP 邮件配置

| 字段 | 说明 |
|------|------|
| SMTP Host | 邮件服务器地址 |
| SMTP Port | 端口（通常 465/587） |
| 用户名 | 发件邮箱 |
| 密码 | 授权码 |
| 发件人名称 | 显示名称 |
| TLS | 是否启用 TLS |

- `POST /settings/test-smtp` 测试 SMTP 连接
- `POST /settings/send-test-email` 发送测试邮件

### 17.3 支付配置

微信支付：
- AppID、MCH_ID、API 密钥
- 支付回调地址配置

支付宝：
- AppID、私钥、公钥
- 沙箱/正式环境切换

### 17.4 Admin API Key

用于通过 API 方式直接调用后台管理接口：
- 查看当前 Key：`GET /settings/admin-api-key`
- 重新生成：`POST /settings/admin-api-key/regenerate`
- 删除：`DELETE /settings/admin-api-key`

### 17.5 高级功能设置

**过载冷却（Overload Cooldown）**：
- 当账号频繁报错时，自动进入冷却期暂停调度
- 配置冷却时长和触发阈值

**流超时（Stream Timeout）**：
- 配置 SSE 流式响应的最大时长
- 超时后自动断开并记录错误

**请求整流器（Rectifier）**：
- 限制单账号的并发请求数
- 防止单个账号因并发过高被上游限流

**Beta 功能策略**：
- 控制实验性功能的启用范围

### 17.6 Web 搜索仿真

- 配置联网搜索工具的模拟行为
- 测试搜索结果质量
- 重置使用量统计

### 17.7 用户属性定义

`/api/v1/admin/user-attribute-definitions`

定义用户自定义属性的结构：
- 创建属性定义（名称、类型、默认值）
- 调整属性显示顺序
- 删除不再使用的属性

### 17.8 错误透传规则

`/api/v1/admin/error-passthrough-rules`

配置哪些上游错误原样返回给客户端，而不是被系统统一处理：
- 按错误码匹配
- 按账号平台匹配

### 17.9 TLS 指纹模板

`/api/v1/admin/tls-fingerprint-profiles`

配置请求上游时使用的 TLS 指纹，用于绕过部分 AI 平台的风控检测。

---

## 18. 系统更新与维护

路径：`/api/v1/admin/system`

### 18.1 检查更新

`GET /system/check-updates` 检查是否有新版本可用

### 18.2 执行更新

`POST /system/update` 在线更新到最新版本（需要确认，更新过程中服务短暂中断）

### 18.3 回滚

`POST /system/rollback` 回滚到上一个版本

### 18.4 重启服务

`POST /system/restart` 优雅重启（等待当前请求处理完成后重启）

### 18.5 查看版本

`GET /system/version` 查看当前运行版本号

---

## 附录：常见运维操作流程

### A. 新增 Claude 账号（OAuth 流程）

```
1. /admin/accounts → 点击「OAuth 授权」→ 选择「Anthropic」
2. 生成授权 URL → 在浏览器中登录 Claude.ai 完成授权
3. 复制回调 URL 中的 code 参数 → 粘贴到「交换 Code」
4. 系统自动创建账号 → 点击「测试」验证
5. 确认账号状态为「正常」后生效
```

### B. 用户余额充值

```
1. /admin/users → 搜索用户
2. 点击用户名进入详情
3. 「余额」标签页 → 「调整余额」
4. 输入金额（正数充值，负数扣减）和备注
5. 确认提交
```

### C. 处理账号异常

```
1. /admin/accounts → 筛选「异常」状态
2. 查看账号错误信息
3. 根据错误类型：
   - 限流错误：「清除限流」
   - Token 过期：「刷新 Token」
   - 其他错误：先「清除错误」再「测试」
4. 若仍异常：检查代理配置或考虑删除
```

### D. 监控告警响应

```
1. /admin/ops → 「告警事件」查看活跃告警
2. 点击告警详情查看触发指标
3. 根据指标类型排查：
   - 错误率高：查看 /ops/request-errors 找规律
   - 延迟高：查看 /ops/concurrency 是否过载
   - 账号不可用：检查 /ops/account-availability
4. 处理完成后将告警状态设为「已解决」
```
