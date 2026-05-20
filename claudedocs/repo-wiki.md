# TokenPanel（SubPanel）项目 Repo Wiki

> **版本**：fork `1.1.126`（上游 `Wei-Shaw/tokenpanel` `0.1.126`），分支 `zhiguofan`  
> **最后更新**：2026-05-18

本文档是项目的完整知识库，按"从整体到细节、从概念到实现"的顺序组织，建议顺序阅读。

---

## 目录

1. [项目概述](#1-项目概述)
2. [技术栈总览](#2-技术栈总览)
3. [仓库结构](#3-仓库结构)
4. [zhiguofan 分支差异](#4-zhiguofan-分支差异)
5. [后端架构](#5-后端架构)
   - 5.1 [分层设计](#51-分层设计)
   - 5.2 [依赖注入（Wire）](#52-依赖注入wire)
   - 5.3 [数据模型（Ent Schema）](#53-数据模型ent-schema)
   - 5.4 [API 路由地图](#54-api-路由地图)
   - 5.5 [中间件栈](#55-中间件栈)
6. [核心功能模块](#6-核心功能模块)
   - 6.1 [AI 网关代理](#61-ai-网关代理)
   - 6.2 [认证与身份](#62-认证与身份)
   - 6.3 [计费与定价](#63-计费与定价)
   - 6.4 [支付集成](#64-支付集成)
   - 6.5 [订阅与配额](#65-订阅与配额)
   - 6.6 [运维监控（Ops）](#66-运维监控ops)
   - 6.7 [插件系统](#67-插件系统)
7. [前端架构](#7-前端架构)
   - 7.1 [路由与布局](#71-路由与布局)
   - 7.2 [状态管理（Pinia）](#72-状态管理pinia)
   - 7.3 [API 调用层](#73-api-调用层)
   - 7.4 [组件体系](#74-组件体系)
   - 7.5 [国际化（i18n）](#75-国际化i18n)
8. [数据库与缓存](#8-数据库与缓存)
9. [本地开发](#9-本地开发)
10. [测试策略](#10-测试策略)
11. [CI/CD](#11-cicd)
12. [部署](#12-部署)
13. [上游同步策略](#13-上游同步策略)
14. [已知陷阱与注意事项](#14-已知陷阱与注意事项)

---

## 1. 项目概述

**TokenPanel**（本仓库前端面板称 SubPanel）是一个 **AI API 聚合网关平台**，核心职责是：

1. 管理多个上游 AI 订阅账号（OpenAI、Anthropic/Claude、Google Gemini、AWS Bedrock、京东云灵境等）
2. 为用户生成 API Key，对外提供 OpenAI 兼容 / Anthropic 兼容的统一接口
3. 对每次请求实时计费，支持余额充值、订阅计划、促销码、兑换码
4. 提供负载均衡、账号健康检测、渠道监控、运维 Dashboard

**面向人群**：  
- **管理员**：配置上游账号、设置分组、管理用户、查看运营数据  
- **用户**：获取 API Key、查看用量、充值余额、管理订阅

**上游 Fork 关系**：
```
Wei-Shaw/tokenpanel (上游 main)
    └── bayma888/tokenpanel-bmai (fork)
            └── zhiguofan (本开发分支，含自定义功能)
```

---

## 2. 技术栈总览

| 层次 | 技术 | 版本/说明 |
|------|------|---------|
| **后端语言** | Go | 1.26.3（以 `backend/go.mod` 为准） |
| **Web 框架** | Gin | v1.9.1 |
| **ORM** | Ent | v0.14.5，代码生成，强类型 |
| **数据库** | PostgreSQL | 18，主存储 |
| **缓存** | Redis | 8，会话/限速/计费缓存 |
| **依赖注入** | Google Wire | 编译期生成，无反射 |
| **前端框架** | Vue 3 | v3.4.0，Composition API |
| **构建工具** | Vite | v5.0.10 |
| **前端状态** | Pinia | v2.1.7 |
| **前端路由** | Vue Router | v4.2.5 |
| **HTTP 客户端** | Axios | v1.16.0 |
| **样式** | TailwindCSS | v3.4.0 |
| **国际化** | Vue I18n | v9.14.5（中/英） |
| **包管理** | pnpm | v9.15.9（禁用 npm） |
| **测试（前）** | Vitest | v2.1.9 |
| **测试（后）** | Go test + testcontainers | 单元/集成/E2E |
| **Lint（后）** | golangci-lint | v2.9 |
| **Lint（前）** | ESLint + vue-eslint-parser | — |
| **容器** | Docker / Docker Compose | 多环境配置 |
| **发布** | GoReleaser | 多平台二进制 |

---

## 3. 仓库结构

```
SubPanel/
├── backend/                  # Go 后端（主体）
│   ├── cmd/server/           # 主程序入口、Wire DI、版本文件
│   ├── ent/                  # Ent ORM Schema 及生成代码
│   ├── internal/             # 业务代码（不对外暴露）
│   │   ├── config/           # 配置加载（Viper）
│   │   ├── domain/           # 常量、枚举
│   │   ├── handler/          # HTTP 处理器（请求解析/响应）
│   │   ├── service/          # 业务逻辑层
│   │   ├── repository/       # 数据访问层（Ent + Redis）
│   │   ├── server/           # HTTP 服务器、路由、中间件
│   │   ├── payment/          # 支付提供商抽象层
│   │   ├── plugin/           # 可插拔功能（promptanalytics）
│   │   ├── pkg/              # 通用工具库（各平台 SDK 适配）
│   │   ├── setup/            # 首次安装向导
│   │   ├── integration/      # E2E 集成测试
│   │   └── testutil/         # 测试工具
│   ├── migrations/           # SQL 迁移文件（20+ 个）
│   ├── data/                 # 静态数据（模型定价 JSON、折扣 JSON）
│   ├── resources/            # 其他资源（model-pricing 数据）
│   ├── go.mod, go.sum        # Go 依赖
│   ├── .golangci.yml         # Lint 规则
│   └── Makefile              # 后端构建脚本
│
├── frontend/                 # Vue 3 前端
│   ├── src/
│   │   ├── api/              # Axios 封装（用户 + 管理员 API）
│   │   ├── components/       # 可复用 Vue 组件
│   │   ├── composables/      # 组合式函数（useXxx）
│   │   ├── constants/        # 前端常量
│   │   ├── i18n/             # 国际化（zh/en）
│   │   ├── router/           # Vue Router 路由配置
│   │   ├── stores/           # Pinia 状态管理
│   │   ├── styles/           # 全局样式
│   │   ├── types/            # TypeScript 类型定义
│   │   ├── utils/            # 工具函数
│   │   └── views/            # 页面组件
│   ├── public/               # 静态资源
│   ├── package.json          # 依赖（pnpm）
│   ├── tailwind.config.js    # TailwindCSS 配置
│   ├── tsconfig.json         # TypeScript 配置
│   └── vitest.config.ts      # Vitest 配置（覆盖率 80%）
│
├── script/                   # 运维脚本
│   ├── dev_local.sh          # 本地开发环境启停
│   ├── sync_upstream_to_zhiguofan.sh  # 上游同步
│   └── push_zhiguofan_to_internal_git.sh  # 推送内网 Git
│
├── deploy/                   # Docker Compose 部署配置
│   ├── docker-compose.yml    # 主配置
│   ├── docker-compose.production.yml
│   ├── docker-compose.standalone.yml
│   ├── .env.example          # 环境变量模板
│   ├── config.example.yaml   # 应用配置模板
│   └── Caddyfile             # Caddy 反向代理配置
│
├── .github/workflows/        # CI/CD（均为 workflow_dispatch 手动触发）
├── claudedocs/               # 项目内部文档（本文件所在目录）
├── docs/                     # 需求文档
├── statics/                  # 静态文档页（we2ai.com、api.cxm.icu）
├── tools/                    # 工具脚本（Python 快速测试）
├── test/                     # API 一致性测试
├── CLAUDE.md                 # Claude Code 项目规则（唯一维护源）
├── AGENTS.md                 # → symlink 到 CLAUDE.md
└── Dockerfile                # 根目录镜像（GoReleaser 用）
```

### 关键规则

- **前端必须使用 pnpm**，禁止 npm（`frontend/package-lock.json` 是历史遗留，不维护）
- **Ent Schema 变更后**必须运行 `go generate ./ent`，提交生成代码
- **Wire DI 变更后**必须运行 `go generate ./cmd/server`
- **版本文件**：`backend/cmd/server/VERSION`，主号固定为 `1`（fork 策略）

---

## 4. zhiguofan 分支差异

本分支相对上游（`Wei-Shaw/tokenpanel`）的全部自定义功能，按风险等级排列。

### 4.1 功能清单

| 功能 | 描述 | 核心文件 |
|------|------|---------|
| **① 用户端模型广场** | 用户查看白名单内可用模型、折后价、CNY 价格 | `views/user/ModelsView.vue`、`handler/usage_handler.go` |
| **② 管理员用户使用统计** | 管理员查看单用户汇总/日历史/模型/端点分布 | `admin/UserStatsModal.vue`、`admin_service.go` |
| **③ 词云分析（Prompt Analytics）** | 网关实时提取关键词，生成月度词云和 Top-N 排行 | `plugin/promptanalytics/`、`ent/schema/keyword_stat.go` |
| **④ 模型折扣与 CNY 定价** | 管理员设置折扣率，计费自动应用，用户端展示人民币价 | `service/pricing_service.go`、`service/billing_service.go` |
| **⑤ UI 主题扩展** | 新增 violet、orange 两套主题 | `style.css`、`stores/app.ts` |
| **⑥ 语言切换 Cookie 持久化** | 刷新后保持用户语言偏好 | `main.ts` |
| **⑦ 网关响应遮蔽（Masking）** | 拦截"你是什么模型"类问题，返回固定身份回答 | `service/gateway_response_masking.go` |
| **⑧ 移除 OAuth 账号创建 UI** | 账号创建弹窗只保留 API Key / Setup Token | `CreateAccountModal.vue`、`EditAccountModal.vue` |
| **⑨ 京东云灵境接入** | Doubao Seedream 生图（同步）+ Seedance 视频（异步 + 后台 Runner） | `service/lingjing_*.go`、`handler/lingjing_handler.go` |
| **⑩ GitHub Actions 禁用自动触发** | 所有 workflow 改为 `workflow_dispatch:` 手动触发 | `.github/workflows/*.yml` |

### 4.2 版本历史

| fork 版本 | 上游版本 | 主要新增 |
|----------|---------|---------|
| 1.1.108 | 0.1.108 | 模型广场、用户使用统计、词云 |
| 1.1.121 | 0.1.121 | 模型折扣、CNY 定价、UI 主题、语言 Cookie |
| 1.1.123 | 0.1.123 | 响应遮蔽、货币模式设置 |
| 1.1.123+ | 0.1.123 | 移除 OAuth UI |
| 1.1.125 | 0.1.125 | 同步上游：Airwallex 多币种、ccswitch codex 导入、Vertex token 代理 |
| 1.1.126 | 0.1.126 | 同步上游：cache_control 开关、Antigravity UA 可配置、unpriced 零成本计费 |
| 1.1.126+ | 0.1.126 | 京东云灵境接入（Seedream + Seedance） |

### 4.3 高风险文件（上游同步时必查）

| 风险 | 文件 | 检查要点 |
|------|------|---------|
| 🔴 高 | `backend/cmd/server/wire_gen.go` | promptAnalytics、lingjing 初始化链、NewSettingHandler 参数数量 |
| 🔴 高 | `backend/internal/server/routes/admin.go` | model-discounts、prompt-analytics 路由 |
| 🔴 高 | `backend/internal/server/routes/gateway.go` | promptAnalytics 中间件、masking 逻辑、lingjing 路由组 |
| 🔴 高 | `frontend/src/router/index.ts` | `/models`、`/admin/prompt-analytics`、`/admin/model-discounts` |
| 🔴 高 | `backend/internal/service/billing_service.go` | `applyDiscount()` 调用 |
| 🔴 高 | `.github/workflows/*.yml` | `on:` 必须为 `workflow_dispatch:` |
| 🟡 中 | `backend/internal/service/scheduler_snapshot_service.go` | 平台列表含 `PlatformLingjing` |
| 🟡 中 | `frontend/src/views/user/ModelsView.vue` | 白名单过滤、折扣展示 |
| 🟡 中 | `frontend/src/i18n/locales/zh.ts` & `en.ts` | `models.*`、`admin.promptAnalytics.*` 键 |

---

## 5. 后端架构

### 5.1 分层设计

```
┌─────────────────────────────────────────────┐
│  HTTP Layer：Routes（gin.Engine）+ Middleware │
└──────────────────┬──────────────────────────┘
                   ↓
┌─────────────────────────────────────────────┐
│  Handler 层：解析请求、校验参数、调用 Service    │
│  internal/handler/ + internal/handler/admin/ │
└──────────────────┬──────────────────────────┘
                   ↓
┌─────────────────────────────────────────────┐
│  Service 层：业务逻辑、编排、计算               │
│  internal/service/                           │
└──────────────────┬──────────────────────────┘
                   ↓
┌─────────────────────────────────────────────┐
│  Repository 层：数据访问（Ent ORM + Redis）    │
│  internal/repository/                        │
└──────────────────┬──────────────────────────┘
                   ↓
┌──────────────┐  ┌──────────────┐
│  PostgreSQL  │  │    Redis     │
└──────────────┘  └──────────────┘
```

**golangci-lint 强制分层**：Service 不能导入 Repository 包，Handler 不能直接导入 Repository 包。

**API 响应契约**：所有响应统一格式

```json
{ "code": 0, "message": "ok", "data": <T> }
```

前端 Axios 拦截器自动解包 `data` 字段。

### 5.2 依赖注入（Wire）

项目使用 [Google Wire](https://github.com/google/wire) 进行编译期依赖注入，**无运行时反射开销**。

**文件结构**

```
backend/cmd/server/
├── wire.go         # Provider 声明（手动维护）
└── wire_gen.go     # Wire 自动生成（禁止手动编辑，除 lingjing 等 fork 注入点）
```

**各模块 ProviderSet**

| 文件 | ProviderSet |
|------|------------|
| `internal/config/wire.go` | `ConfigSet` |
| `internal/repository/wire.go` | `RepositorySet` |
| `internal/service/wire.go` | `ServiceSet` |
| `internal/handler/wire.go` | `HandlerSet` |
| `internal/server/middleware/wire.go` | `MiddlewareSet` |
| `internal/payment/wire.go` | `PaymentSet` |
| `internal/plugin/promptanalytics/wire.go` | `PromptAnalyticsSet` |

**变更流程**：修改 `wire.go` 的 Provider 声明 → 运行 `go generate ./cmd/server` → 提交 `wire_gen.go`。

**fork 注入点**（wire_gen.go 中不能丢失的手动片段）：

```go
// 词云插件
promptAnalyticsPlugin := promptanalytics.NewPlugin(cfg.PromptAnalytics)
promptAnalyticsHandler := promptanalytics.NewHandler(promptAnalyticsRepo)

// 灵境（Lingjing）完整初始化链
lingjingClient := service.NewLingjingClient()
lingjingTaskRepository := repository.NewLingjingTaskRepository(client)
lingjingGatewayService := service.NewLingjingGatewayService(...)
openAIGatewayService.SetLingjingService(lingjingGatewayService)
lingjingHandler := handler.NewLingjingHandler(...)
lingjingPollRunner := service.ProvideLingjingPollRunner(...)
```

### 5.3 数据模型（Ent Schema）

所有 Schema 文件位于 `backend/ent/schema/`，变更后运行 `go generate ./ent` 重新生成。

#### 用户与认证

| Entity | 文件 | 核心字段 |
|--------|------|---------|
| **User** | `user.go` | email, password_hash, role(user/admin), balance(decimal), totp_secret |
| **AuthIdentity** | `auth_identity.go` | provider(email/google/github/wechat/linuxdo/oidc), provider_user_id |
| **AuthIdentityChannel** | `auth_identity_channel.go` | identity_id, channel(web/mobile) |
| **PendingAuthSession** | `pending_auth_session.go` | OAuth 待完成会话，TTL 短暂 |
| **UserAttributeDefinition** | `user_attribute_definition.go` | 自定义用户字段定义（键名、类型） |
| **UserAttributeValue** | `user_attribute_value.go` | 自定义字段值 |
| **UserAllowedGroup** | `user_allowed_group.go` | 用户可访问的分组 |

#### 账号与分组

| Entity | 文件 | 核心字段 |
|--------|------|---------|
| **Account** | `account.go` | platform, credentials(JSON), status, group_ids |
| **AccountGroup** | `account_group.go` | 账号与分组的多对多关联 |
| **Group** | `group.go` | name, platform, model_mapping(JSON), rate_limit |
| **APIKey** | `api_key.go` | key_hash, user_id, group_id, quota, rpm_limit |

#### 计费与支付

| Entity | 文件 | 核心字段 |
|--------|------|---------|
| **UsageLog** | `usage_log.go` | user_id, model, input_tokens, output_tokens, cost, endpoint |
| **UsageCleanupTask** | `usage_cleanup_task.go` | 分批清理任务状态 |
| **ModelPricing** | `model_pricing.go` | model_id, input_price, output_price, currency |
| **PaymentOrder** | `payment_order.go` | user_id, amount, provider, status, tx_id |
| **PaymentProviderInstance** | `payment_provider_instance.go` | provider_type, config(JSON), weight |
| **PaymentAuditLog** | `payment_audit_log.go` | 支付操作审计，不可修改 |
| **SubscriptionPlan** | `subscription_plan.go` | name, price, quota, reset_period |
| **UserSubscription** | `user_subscription.go` | user_id, plan_id, remaining_quota, reset_at |
| **PromoCode** | `promo_code.go` | code, discount_type, discount_value, max_uses |
| **PromoCodeUsage** | `promo_code_usage.go` | 使用记录（防重复使用） |
| **RedeemCode** | `redeem_code.go` | code, amount, status(unused/used) |

#### 监控与安全

| Entity | 文件 | 核心字段 |
|--------|------|---------|
| **ChannelMonitor** | `channel_monitor.go` | 渠道监控配置、告警阈值 |
| **ChannelMonitorDailyRollup** | `channel_monitor_daily_rollup.go` | 每日聚合统计 |
| **ChannelMonitorHistory** | `channel_monitor_history.go` | 监控历史快照 |
| **ChannelMonitorRequestTemplate** | `channel_monitor_request_template.go` | 自定义监控请求体模板 |
| **ErrorPassthroughRule** | `error_passthrough_rule.go` | 上游错误码透传规则 |
| **TLSFingerprintProfile** | `tls_fingerprint_profile.go` | utls 指纹配置（模拟浏览器 TLS） |
| **SecuritySecret** | `security_secret.go` | 内部服务密钥 |

#### 其他

| Entity | 文件 | 核心字段 |
|--------|------|---------|
| **Proxy** | `proxy.go` | url, type(http/socks5), 账号关联 |
| **Announcement** | `announcement.go` | title, content(markdown), level |
| **AnnouncementRead** | `announcement_read.go` | user_id, announcement_id，已读状态 |
| **Setting** | `setting.go` | key, value(JSON)，系统全局配置 KV 表 |
| **IdempotencyRecord** | `idempotency_record.go` | request_hash, TTL，防重 |
| **IdentityAdoptionDecision** | `identity_adoption_decision.go` | OAuth 身份合并决策 |
| **KeywordStat** | `keyword_stat.go` | keyword, period(YYYY-MM), count，词云数据 |
| **LingjingTask** | `lingjing_task.go` | gen_task_id, status, result_url，视频异步任务 |

**Schema Mixin**（所有 Entity 均使用）：
- `TimeMixin`：自动管理 `created_at`、`updated_at`
- `SoftDeleteMixin`：软删除支持（`deleted_at`，大多数 Entity 启用）

### 5.4 API 路由地图

路由定义在 `backend/internal/server/routes/`，前缀统一为 `/api/v1`。

#### 公开路由（无需认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/auth/email/login` | 邮箱登录 |
| POST | `/auth/email/register` | 邮箱注册 |
| GET | `/auth/{provider}/oauth` | OAuth 跳转（google/github/wechat/linuxdo/oidc） |
| POST | `/auth/callback` | OAuth 回调处理 |
| POST | `/auth/totp/verify` | TOTP 验证 |
| POST | `/auth/password/forgot` | 发送重置邮件 |
| POST | `/auth/password/reset` | 重置密码 |
| GET | `/auth/user` | 获取当前用户信息 |
| GET | `/settings/public` | 获取公开配置（OAuth 开关等） |
| GET | `/announcements` | 获取公告列表 |
| GET | `/pages/{pageId}` | 获取自定义页面内容 |

#### 用户路由（需 JWT 认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/user/dashboard` | 用户仪表盘（余额、用量概览） |
| GET/POST | `/user/api-keys` | 列出 / 创建 API Key |
| DELETE | `/user/api-keys/{id}` | 删除 API Key |
| GET | `/user/models` | 白名单内可用模型列表（fork 功能） |
| GET | `/user/usage` | 使用记录（分页、模型过滤） |
| GET | `/user/usage/billing` | 计费汇总 |
| GET/PUT | `/user/subscriptions` | 订阅信息 |
| POST | `/user/payment/create-order` | 创建充值订单 |
| GET | `/user/payment/orders` | 充值订单历史 |
| POST | `/user/redeem-codes/redeem` | 兑换码充值 |
| GET/PUT | `/user/profile` | 用户资料 |

#### 管理员路由（需管理员权限）

| 分组 | 代表路径 | 说明 |
|------|---------|------|
| 仪表盘 | `GET /admin/dashboard` | 全局统计快照 |
| 用户管理 | `GET/PUT/DELETE /admin/users` | CRUD + 搜索 + 批量更新 |
| 用户统计 | `GET /admin/users/:id/usage` | 单用户使用明细（fork 功能） |
| 账号管理 | `GET/POST/PUT /admin/accounts` | AI 账号 CRUD + 批量导入 |
| 分组管理 | `GET/POST/PUT /admin/groups` | 分组 CRUD |
| 渠道管理 | `GET/POST/PUT /admin/channels` | 渠道监控配置 |
| 代理管理 | `GET/POST/PUT /admin/proxies` | HTTP/SOCKS5 代理 |
| 卡密管理 | `GET/POST /admin/redeem-codes` | 生成 + 导出兑换码 |
| 促销码 | `GET/POST/PUT /admin/promo-codes` | 折扣促销码管理 |
| 公告 | `GET/POST/PUT /admin/announcements` | 系统公告管理 |
| 系统设置 | `GET/PUT /admin/settings` | 全局配置（OAuth、邮件、支付等） |
| 模型折扣 | `GET/PUT /admin/settings/model-discounts` | 折扣率配置（fork 功能） |
| 使用统计 | `GET /admin/usage` | 全局使用分析 + 导出 |
| 订阅管理 | `GET/POST/PUT /admin/subscriptions/plans` | 订阅套餐管理 |
| 支付管理 | `GET/PUT /admin/payment/orders` | 支付订单管理 |
| 支付提供商 | `GET/POST /admin/payment/providers` | 提供商实例配置 |
| 数据备份 | `GET/POST /admin/backup` | PostgreSQL/S3 备份 |
| 数据管理 | `POST /admin/data-management/reset-user-usage` | 重置用户用量 |
| Ops 监控 | `GET /admin/ops/dashboard` | 运维仪表盘 |
| Ops 实时 | `WS /admin/ops/ws` | 实时监控 WebSocket |
| 词云分析 | `GET /admin/prompt-analytics/top-keywords` | 关键词 Top-N（fork 功能） |
| 联盟管理 | `GET /admin/affiliates/*` | 邀请返佣系统 |
| TLS 指纹 | `GET/POST /admin/tls-fingerprint-profiles` | 自定义 TLS 指纹 |
| 错误透传 | `GET/POST /admin/error-passthrough-rules` | 上游错误码透传规则 |

#### 网关路由（用 API Key 认证）

| 方法 | 路径 | 协议 |
|------|------|------|
| POST | `/v1/chat/completions` | OpenAI Chat Completions |
| POST | `/v1/images/generations` | OpenAI 图像生成（含灵境分流） |
| GET | `/v1/models` | 模型列表 |
| POST | `/anthropic/v1/messages` | Anthropic Messages API |
| POST | `/gemini/v1beta/*` | Google Gemini API |
| POST | `/bedrock/*` | AWS Bedrock 兼容 |
| WS | `/v1/chat/completions/stream` | OpenAI WebSocket 流式 |
| POST | `/lingjing/v1/video/submit` | 灵境视频提交（fork 功能） |
| GET | `/lingjing/v1/video/:taskId` | 灵境视频查询（fork 功能） |

#### 支付 Webhook 路由（无需认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/payment/webhook/{provider}` | 支付宝/微信/Stripe/Airwallex 回调 |

#### 健康检查

| 方法 | 路径 |
|------|------|
| GET | `/health` |

### 5.5 中间件栈

中间件定义在 `backend/internal/server/middleware/`。

| 中间件文件 | 作用 | 挂载位置 |
|----------|------|---------|
| `cors.go` | CORS 跨域 | 全局 |
| `recovery.go` | panic 恢复，返回 500 | 全局 |
| `security_headers.go` | 安全响应头（CSP、HSTS 等） | 全局 |
| `logger.go` | 请求日志（zap） | 全局 |
| `request_logger.go` | 详细请求/响应日志（调试） | 可选 |
| `request_body_limit.go` | 请求体大小限制 | 全局 |
| `client_request_id.go` | 注入/透传 X-Request-ID | 全局 |
| `jwt_auth.go` | JWT 验证，写入 context | 用户路由 |
| `admin_auth.go` | 管理员角色验证 | 管理员路由 |
| `admin_only.go` | 拒绝非管理员请求 | 管理员路由 |
| `api_key_auth.go` | API Key 验证（查缓存/DB） | 网关路由 |
| `api_key_auth_google.go` | Google API Key 验证 | Gemini 路由 |
| `auth_subject.go` | 从 JWT 或 API Key 提取用户主体 | 混合路由 |
| `backend_mode_guard.go` | Simple 模式守卫（禁用计费） | 网关路由 |
| `rate_limiter.go` | 基于 Redis 的 RPM 限速 | 网关路由 |

---

## 6. 核心功能模块

### 6.1 AI 网关代理

网关是整个系统的核心，负责将用户请求转发到上游 AI 平台。

#### 支持平台

| 平台 | 平台常量 | 协议 | 核心 Service |
|------|---------|------|-------------|
| Anthropic/Claude | `claude` | Native Messages API | `gateway_service.go` |
| OpenAI（含第三方兼容） | `openai` | Chat Completions | `openai_gateway_service.go` |
| Google Gemini | `gemini` | Gemini v1beta / Chat Completions 兼容 | `gemini_messages_compat_service.go` |
| AWS Bedrock | `bedrock` | Bedrock API | `openai_gateway_service.go`（兼容层） |
| Antigravity | `antigravity` | 私有协议 | `antigravity_gateway_service.go` |
| 京东云灵境 | `lingjing` | JDCloud model API | `lingjing_gateway_service.go` |

#### 请求流转（Claude 为例）

```
用户请求 POST /anthropic/v1/messages
    ↓
api_key_auth 中间件（验证 API Key，加载用户/分组信息）
    ↓
promptAnalytics 中间件（异步提取关键词，非阻塞）
    ↓
GatewayHandler.HandleMessages()
    ↓
GatewayService.ForwardMessages()
    ├── SelectAccount()（从分组内选健康账号，带等待队列）
    ├── IsResponseMaskingEnabled() → 若命中身份问题，直接返回假响应
    ├── 构造上游请求（pkg/claude/）
    ├── 发送 HTTP/2 请求（httpclient 连接池）
    ├── 流式响应透传（SSE）
    └── UsageLog 写入（异步，含 token 计数、费用计算）
```

#### Claude Sticky Session

保证同一用户的对话会话路由到同一账号（1 小时 TTL），避免 Claude 的上下文 ID 跨账号失效。

```
用户 API Key → singleflight 分组 → Redis TTL(3600s) → 固定账号 ID
```

#### OpenAI WebSocket 支持

`openai_ws_client.go` + `openai_ws_pool.go`：为需要 WebSocket 流式的 OpenAI 兼容场景维护长连接池。

#### TLS 指纹伪装

通过 `pkg/tlsfingerprint/`（基于 `refraction-networking/utls`）让出站请求模拟真实浏览器 TLS 握手，绕过某些平台的反爬检测。配置见 `TLSFingerprintProfile` Entity。

#### 灵境异步视频任务（fork 功能）

```
POST /lingjing/v1/video/submit → 返回 202 + taskId
                                          ↓
                              LingjingPollRunner（后台 goroutine）
                              alitto/pond v2，5s ticker，10 workers
                              最多轮询 240 次（20 分钟）
                                          ↓
                              任务完成 → 触发 billingService 扣费
```

### 6.2 认证与身份

#### 支持的登录方式

| 方式 | Provider 常量 | 实现文件 |
|------|------------|---------|
| 邮箱/密码 | `email` | `auth_service.go` |
| Google OAuth | `google` | `auth_email_oauth.go` |
| GitHub OAuth | `github` | `auth_email_oauth.go` |
| 微信扫码 | `wechat` | `auth_wechat_oauth.go` |
| LinuxDo | `linuxdo` | `auth_linuxdo_oauth.go` |
| OIDC 通用 | `oidc` | `auth_oidc_oauth.go` |

#### JWT 认证流程

1. 登录成功 → 签发 Access Token（短 TTL）+ Refresh Token（长 TTL）
2. 前端 Axios 拦截器：401 时自动用 Refresh Token 换新 Token（排队队列，防并发）
3. Token 存储：`localStorage`（Access）+ HttpOnly Cookie（Refresh，可选）

#### TOTP 两步认证

- 用户可在设置中绑定 TOTP（`pquerna/otp`）
- 登录时若已绑定，需额外提交 6 位验证码
- 前端：`TotpLoginModal.vue`、`totp.ts` API

#### OAuth 身份合并

多个 OAuth 账号可合并到同一用户（`IdentityAdoptionDecision` Entity），通过 `auth_pending_identity_service.go` 处理合并决策流程。

### 6.3 计费与定价

#### 定价数据来源

1. **`backend/data/model_pricing.json`**：基础定价（SHA256 校验完整性），上游各平台官方价格
2. **`ModelPricing` Entity（DB）**：管理员可在后台覆盖或新增定价
3. **`backend/data/model_discounts.json`**（fork）：折扣配置，可被 DB 热覆盖

#### 计费流程

```
请求结束 → token 计数
    ↓
pricing_service.GetModelPricing()
    ↓
billing_service.applyDiscount()  ← fork 新增，读折扣配置
    ↓
billing_service.CalculateCost()
    ↓
余额扣减（原子操作，Redis 锁）
    ↓
UsageLog 写入（异步）
```

#### CNY 汇率（fork 功能）

- `SettingKeyCNYRate`：管理员可设置 USD→CNY 汇率，默认 7
- `SettingKeyCurrencyMode`：切换展示货币（USD / CNY）
- 前端 `stores/app.ts` 中维护 `currencyMode`、`cnyRate`，ModelsView 据此展示价格

#### Simple 模式

`RUN_MODE=simple`（+ `SIMPLE_MODE_CONFIRM=true`）：禁用余额检查和计费逻辑，适合个人开发者使用。

### 6.4 支付集成

#### 架构设计

```
PaymentService
    └── LoadBalancer（按 weight 选提供商）
            ├── Stripe Provider（stripe.go）
            ├── 支付宝 Provider（alipay.go）
            ├── 微信支付 Provider（wxpay.go）
            ├── Airwallex Provider（airwallex.go）
            └── EasyPay Provider（easypay.go）
```

提供商通过 `PaymentProviderInstance` Entity 配置（可多实例），`LoadBalancer` 按 weight 字段分流。

#### 订单状态流转

```
created → pending → paid → fulfilled
              ↓
           failed / timeout
```

`payment_fulfillment.go` 负责 paid → fulfilled：扣减用量、增加余额、发邮件通知。

#### Webhook 处理

`payment_webhook_handler.go`：验证签名 → 更新订单状态 → 触发 fulfillment（幂等保护，`IdempotencyRecord`）。

#### 促销码 vs 兑换码

- **促销码（PromoCode）**：充值时输入，折扣比例/固定减免
- **兑换码（RedeemCode）**：类似充值卡，一码一值，兑换即到账

### 6.5 订阅与配额

用户可购买**订阅套餐**（`SubscriptionPlan`），套餐含周期性配额（如每月 100 万 token）。

- `UserSubscription`：记录用户当前套餐、剩余配额、下次重置时间
- `subscription_expiry_service.go`：定时检测过期订阅，自动降级或续费
- 配额 vs 余额：配额先消耗，耗尽后用余额计费（可配置策略）

### 6.6 运维监控（Ops）

Ops Dashboard 位于管理员面板 `/admin/ops`，提供：

| 功能 | 路径 | 说明 |
|------|------|------|
| 实时概览 | `GET /admin/ops/dashboard` | 当前 RPM、成功率、延迟、活跃账号 |
| 实时推送 | `WS /admin/ops/ws` | WebSocket 10s 推送实时数据 |
| 告警 | `GET/POST /admin/ops/alerts` | 阈值告警配置，`ops_alert_evaluator_service.go` 定时评估 |
| 系统日志 | `GET /admin/ops/system-logs` | 结构化日志查询（错误/警告） |
| 渠道监控 | `ChannelMonitor` Entity | 定时探测上游账号健康状态 |

前端页面：`frontend/src/views/admin/ops/OpsDashboard.vue`（含 20+ 子组件）。

### 6.7 插件系统

插件目录：`backend/internal/plugin/`，每个插件自包含 handler/repository/middleware/wire，不侵入主业务代码。

#### Prompt Analytics 插件（fork 功能）

```
网关请求经过 promptAnalytics 中间件
    ↓（异步，不阻塞请求）
keyword_extractor.go（正则/分词提取关键词）
    ↓
repository.go（KeywordStat 按 YYYY-MM 聚合计数）
    ↓
handler.go 暴露 GET /admin/prompt-analytics/top-keywords
    ↓
前端 PromptAnalyticsView.vue（词云可视化）
```

---

## 7. 前端架构

### 7.1 路由与布局

路由定义在 `frontend/src/router/index.ts`，使用 Vue Router 4 懒加载。

#### 路由分类

| 类别 | 路由前缀 | 布局组件 | 守卫 |
|------|---------|---------|------|
| 初始化 | `/setup` | 无 | 检测 isSetup |
| 公开页面 | `/home`、`/login`、`/register` | `AuthLayout` | 无 |
| OAuth 回调 | `/auth/*` | 无 | 无 |
| 用户面板 | `/dashboard`、`/keys`、`/models`... | `AppLayout` | `requiresAuth` |
| 管理面板 | `/admin/*` | `AppLayout` | `requiresAuth + requiresAdmin` |
| 支付页面 | `/payment`、`/stripe-payment`... | 独立布局 | `requiresAuth` |

#### 布局组件

```
AppLayout.vue
├── AppHeader.vue（顶部导航：用户信息、公告铃铛、语言切换）
├── AppSidebar.vue（侧边栏：管理员/用户菜单项）
└── <RouterView>（页面内容）
```

#### 路由守卫逻辑（`router/index.ts`）

```typescript
router.beforeEach(async (to, from, next) => {
    // 1. 检测 setup 是否完成
    // 2. requiresAuth → 检查 authStore.isLoggedIn
    // 3. requiresAdmin → 检查 authStore.isAdmin
    // 4. 重定向 /login（携带 redirect 参数）
})
```

### 7.2 状态管理（Pinia）

| Store 文件 | 状态职责 |
|-----------|---------|
| `stores/auth.ts` | 用户信息（id/email/role）、JWT tokens、登录/登出 action |
| `stores/app.ts` | 全局配置（OAuth 开关、主题、货币模式、CNY 汇率） |
| `stores/adminSettings.ts` | 管理员缓存的系统设置 |
| `stores/payment.ts` | 当前支付流程状态（订单 ID、提供商、金额） |
| `stores/subscriptions.ts` | 用户订阅信息缓存 |
| `stores/announcements.ts` | 未读公告列表 |
| `stores/onboarding.ts` | 用户引导（Driver.js）完成状态 |

### 7.3 API 调用层

所有 HTTP 调用通过 `frontend/src/api/` 封装，禁止直接使用 `axios`。

#### Axios 实例配置（`api/client.ts`）

- `baseURL`：`/api/v1`
- **请求拦截器**：自动附加 `Authorization: Bearer <token>`
- **响应拦截器**：
  1. 自动解包 `{ code, message, data }` 格式，返回 `data`
  2. 401 → 触发 Token 刷新，排队等待（防并发重复刷新）
  3. 刷新失败 → 跳转 `/login`

#### 目录结构

```
api/
├── client.ts          # Axios 实例（上述逻辑）
├── auth.ts            # 认证（登录/注册/登出/刷新）
├── user.ts            # 用户信息
├── keys.ts            # API Key CRUD
├── usage.ts           # 使用记录
├── payment.ts         # 支付流程
├── models.ts          # 模型列表（含折扣/CNY 价格）
├── channels.ts        # 渠道查询
├── subscriptions.ts   # 订阅
├── announcements.ts   # 公告
├── redeem.ts          # 兑换码
├── totp.ts            # TOTP 绑定/验证
├── channelMonitor.ts  # 渠道监控
├── setup.ts           # 初始化设置
└── admin/             # 管理员 API（31 个文件）
    ├── accounts.ts    # 账号 CRUD
    ├── users.ts       # 用户管理（含 usage stats）
    ├── groups.ts      # 分组管理
    ├── settings.ts    # 系统设置（含 model-discounts）
    ├── ops.ts         # 运维数据
    ├── payment.ts     # 支付管理
    ├── promptAnalytics.ts  # 词云分析（fork）
    └── ...（共 31 个文件）
```

### 7.4 组件体系

#### 通用组件（`components/common/`，50+ 个）

| 组件 | 用途 |
|------|------|
| `DataTable.vue` | 带排序/分页/选择的通用数据表格 |
| `BaseDialog.vue` / `ConfirmDialog.vue` | 弹窗基类 + 确认弹窗 |
| `Pagination.vue` | 分页控件 |
| `DateRangePicker.vue` | 日期范围选择 |
| `StatCard.vue` | 统计卡片（数值 + 趋势） |
| `StatusBadge.vue` / `PlatformTypeBadge.vue` | 状态/平台徽章 |
| `AnnouncementBell.vue` | 顶部公告铃铛（未读红点） |
| `LocaleSwitcher.vue` | 中/英语言切换 |
| `Toast.vue` | 全局 Toast 消息 |
| `SearchInput.vue` | 防抖搜索输入框 |
| `AutoRefreshButton.vue` | 自动刷新按钮（可设间隔） |
| `ImageUpload.vue` | 图片上传（Logo 等） |

#### 业务组件分组

```
components/
├── account/     # 账号创建/编辑弹窗（CreateAccountModal、EditAccountModal）
├── auth/        # OAuth 按钮组、TOTP 弹窗
├── charts/      # Token 趋势图、模型分布图、端点分布图
├── keys/        # API Key 创建/管理相关
├── layout/      # AppLayout、AppHeader、AppSidebar、TablePageLayout
├── payment/     # 支付提供商选择、二维码、Stripe 内嵌
├── admin/       # 管理员专用组件（UserStatsModal 等）
└── user/        # 用户专用组件
```

#### 组合式函数（`composables/`，15+）

| 函数 | 职责 |
|------|------|
| `useTableLoader` | 封装表格分页/排序/加载状态 |
| `useTableSelection` | 表格行多选（含全选） |
| `useAutoRefresh` | 自动轮询刷新（配置间隔） |
| `usePersistedPageSize` | 记忆每页条数（localStorage） |
| `useKeyedDebouncedSearch` | 防抖搜索（带 key 区分多搜索框） |
| `useClipboard` | 剪贴板复制（带 fallback） |
| `useOnboardingTour` | Driver.js 引导流程 |
| `useNavigationLoading` | 路由跳转加载进度条 |
| `useRoutePrefetch` | 路由组件预加载 |
| `useModelWhitelist` | 模型白名单过滤逻辑 |
| `useChannelMonitorFormat` | 监控数据格式化 |
| `useAccountOAuth` / `useOpenAIOAuth` 等 | 各平台 OAuth 账号绑定流程 |

### 7.5 国际化（i18n）

- 框架：Vue I18n 9.x
- 语言：中文（`locales/zh.ts`）、英文（`locales/en.ts`）
- 默认语言：系统设置 `default_language`，用户可通过 Cookie 覆盖（fork 功能）
- fork 新增键：`models.*`（模型广场）、`admin.promptAnalytics.*`（词云）、`nav.models` 等

---

## 8. 数据库与缓存

### PostgreSQL 数据库

- **连接管理**：`repository/db_pool.go`，连接池参数可通过环境变量配置
- **迁移**：`repository/migrations_runner.go`，启动时自动按序执行 `backend/migrations/` 下的 SQL 文件（20+ 个）
- **ORM**：Ent，Schema 在 `ent/schema/`，生成代码在 `ent/`（提交到 Git）

### Redis 缓存分类

| 缓存类型 | 文件 | 用途 |
|---------|------|------|
| API Key 缓存 | `api_key_cache.go` | 减少 API Key 验证的 DB 查询 |
| 计费缓存 | `billing_cache.go` | 用户余额/配额的热缓存（原子操作） |
| 仪表盘缓存 | `dashboard_cache.go` | 统计快照，TTL 短 |
| 网关缓存 | `gateway_cache.go` | Sticky Session、Singleflight 缓存 |
| 身份缓存 | `identity_cache.go` | JWT 用户信息缓存 |
| RPM 缓存 | `rpm_cache.go` | 每分钟请求计数（滑动窗口） |
| 兑换缓存 | `redeem_cache.go` | 防重复兑换 |
| 调度缓存 | `scheduler_cache.go` | 账号调度状态 |

### 幂等性保护

关键操作（支付 Webhook、兑换码）通过 `IdempotencyRecord` Entity 实现幂等，`idempotency_helper.go` 提供通用辅助。

---

## 9. 本地开发

### 前提条件

- Go 1.26.x（以 `backend/go.mod` 为准）
- Node.js 18+（pnpm v9.15.9）
- Docker（用于 PostgreSQL + Redis）

### 快速启动

```bash
# 启动后端（:8082）+ 前端（:3002）+ Docker 中的 DB/Redis
./script/dev_local.sh up

# 查看状态
./script/dev_local.sh status

# 查看日志
./script/dev_local.sh logs

# 停止
./script/dev_local.sh down

# 端口冲突时覆盖
BACKEND_PORT=18082 FRONTEND_PORT=13002 ./script/dev_local.sh up
```

### 单独启动

```bash
# 后端
cd backend && go run ./cmd/server/

# 前端
cd frontend && pnpm dev
```

### 常用开发命令

```bash
# 后端单元测试
cd backend && go test -tags=unit ./...

# 后端集成测试（需要 Docker）
cd backend && go test -tags=integration ./...

# 后端 E2E 测试
cd backend && go test -tags=e2e -v -timeout=300s ./internal/integration/...

# 后端 Lint
cd backend && golangci-lint run ./...

# 重新生成 Ent ORM（修改 schema 后）
cd backend && go generate ./ent

# 重新生成 Wire DI（修改 providers 后）
cd backend && go generate ./cmd/server

# 前端测试
cd frontend && pnpm test:run

# 前端覆盖率
cd frontend && pnpm test:coverage

# 前端 Lint
cd frontend && pnpm run lint:check

# 前端 TypeScript 检查
cd frontend && pnpm run typecheck
```

### 自动初始化

首次启动时系统自动检测是否已初始化：
- 未初始化 → 前端显示 `/setup` 向导
- Docker 部署：设置 `AUTO_SETUP=true` 从环境变量静默初始化（`internal/setup/`）

---

## 10. 测试策略

### 后端测试分层

| 类型 | 标签 | 位置 | 说明 |
|------|------|------|------|
| 单元测试 | `-tags=unit` | 各 `*_test.go` | 纯函数，无外部依赖 |
| 集成测试 | `-tags=integration` | `repository/` | 真实 DB（testcontainers 启动） |
| E2E 测试 | `-tags=e2e` | `internal/integration/` | 真实 HTTP 请求，全流程 |

### 前端测试

- 框架：Vitest + @vue/test-utils + jsdom
- 配置：`frontend/vitest.config.ts`
- **覆盖率要求**：语句/分支/函数/行均 ≥ 80%
- 关键测试文件：
  - `views/admin/__tests__/SettingsView.spec.ts`
  - `composables/__tests__/usePersistedPageSize.spec.ts`
  - `views/user/__tests__/ModelsView.spec.ts`（fork 功能）
  - `i18n/__tests__/usageServiceTierLocales.spec.ts`

### 合并后最低验证命令

```bash
# 后端编译 + 单元测试
cd backend && go build ./cmd/server/ && go test -tags=unit \
    ./internal/service ./internal/repository ./internal/handler/...

# 前端关键测试
cd frontend && pnpm exec vitest run \
    src/views/admin/__tests__/SettingsView.spec.ts \
    src/composables/__tests__/usePersistedPageSize.spec.ts \
    src/views/user/__tests__/ModelsView.spec.ts \
    src/i18n/__tests__/usageServiceTierLocales.spec.ts

# 前端 Lint
cd frontend && pnpm run lint:check
```

---

## 11. CI/CD

所有 Workflow 在 `zhiguofan` 分支均**仅通过 `workflow_dispatch:` 手动触发**（节省 GitHub Actions 额度），上游同步后必须检查并还原。

### Workflow 列表

| 文件 | 功能 | 原上游触发条件 |
|------|------|-------------|
| `backend-ci.yml` | 单元测试 + 集成测试 + golangci-lint v2.9 | push / pull_request |
| `security-scan.yml` | govulncheck + gosec + pnpm audit | push / pull_request / 每周一 |
| `docker-push.yml` | 构建并推送 Docker 镜像 | push zhiguofan 分支 |
| `release.yml` | GoReleaser 多平台构建发布（需输入 tag） | push `v*` tag |
| `sync-upstream.yml` | 同步上游 main 到 fork | 每日定时 |
| `cla.yml` | CLA 检查 | issue_comment / pull_request_target |

### Docker 镜像构建

- **生产镜像**：`go build -tags embed` 将前端嵌入二进制，单文件部署
- **GoReleaser**：`.goreleaser.yaml`（多平台）和 `.goreleaser.simple.yaml`（简化版）
- **Dockerfile**：根目录用于 GoReleaser，`backend/Dockerfile` 用于独立后端镜像

---

## 12. 部署

### Docker Compose 方案

```bash
# 进入部署目录
cd deploy/

# 复制并填写环境变量
cp .env.example .env

# 复制并修改应用配置
cp config.example.yaml config.yaml

# 启动（生产配置）
docker compose -f docker-compose.yml -f docker-compose.production.yml up -d
```

### 环境变量（关键配置）

| 变量 | 说明 | 示例 |
|------|------|------|
| `DATABASE_URL` | PostgreSQL 连接串 | `postgres://user:pass@host:5432/db` |
| `REDIS_URL` | Redis 连接串 | `redis://host:6379` |
| `JWT_SECRET` | JWT 签名密钥 | 随机 32 字节 |
| `RUN_MODE` | 运行模式 | `production` / `simple` |
| `AUTO_SETUP` | Docker 自动初始化 | `true` |
| `SIMPLE_MODE_CONFIRM` | 确认 Simple 模式 | `true` |

### 内网 Git 推送

```bash
# 目标：ssh://git_prod_backend@192.168.1.10/home/git_prod_backend/wmtoken_platform.git
./script/push_zhiguofan_to_internal_git.sh
```

### Caddy 反向代理（`deploy/Caddyfile`）

前后端通过 Caddy 统一暴露，静态文件由 Caddy 直接服务，API 请求代理到 Go 服务。

---

## 13. 上游同步策略

### 版本策略

- `main` 分支：对齐上游，版本 `0.x.y`
- `zhiguofan` 分支：fork 版本线，主号固定 `1`（上游 `0.x.y` → fork `1.x.y`）

### 标准同步流程

```bash
./script/sync_upstream_to_zhiguofan.sh
# 脚本执行：upstream/main → main → origin/main → zhiguofan → origin/zhiguofan
# 同时自动将 VERSION 主号改为 1
```

### 手动 Merge 时必检项

```bash
# 1. 版本号主号是否为 1
cat backend/cmd/server/VERSION

# 2. 高风险路由是否存在
grep -r "model-discounts\|prompt-analytics\|/models" \
    backend/internal/server/routes/

# 3. wire_gen.go 关键注入点
grep -A2 "promptAnalyticsPlugin\|promptAnalyticsHandler\|lingjingGatewayService\|lingjingPollRunner" \
    backend/cmd/server/wire_gen.go

# 4. billing_service 折扣调用
grep "applyDiscount" backend/internal/service/billing_service.go

# 5. 前端路由完整性
grep -E "'/models'|prompt-analytics|model-discounts" \
    frontend/src/router/index.ts

# 6. i18n 键完整性
grep -E "nav\.models|promptAnalytics|models\." \
    frontend/src/i18n/locales/zh.ts | head -5

# 7. Workflows 触发条件（必须是 workflow_dispatch）
grep -A3 "^on:" .github/workflows/*.yml
```

---

## 14. 已知陷阱与注意事项

### 批量修改账号导致模型映射丢失

**现象**：前端测试正常，但 API 调用返回 `Service temporarily unavailable`。  
**根因**：同时选中不同平台账号（如 OpenAI + Antigravity）批量修改时，模型白名单/映射被跨平台策略覆盖，导致 OpenAI 账号关键模型映射丢失。  
**修复**：批量修改前按平台分组，不要混选不同平台账号。

### pnpm-lock.yaml 未同步

`package.json` 新增依赖后，CI 使用 `--frozen-lockfile`，lock 不同步会导致 CI 失败。  
**修复**：`cd frontend && pnpm install && git add pnpm-lock.yaml`

### node_modules 冲突

之前用 npm 装过后再用 pnpm 会报 `EPERM` 错误。  
**修复**：`rm -rf frontend/node_modules && pnpm install`

### Ent Schema 变更未重新生成

修改 `ent/schema/*.go` 后忘记运行 `go generate ./ent`，导致运行时 ORM 代码与 Schema 不一致。  
**规则**：每次 Schema 变更必须提交生成代码。

### Wire 接口新增方法未同步 Stub

给 Go interface 新增方法后，所有 test stub 必须同步补全。  
**查找命令**：`grep -r "type.*Stub.*struct\|type.*Mock.*struct" internal/`

### Workflow 自动触发被上游还原

同步上游后，`.github/workflows/*.yml` 中的 `on:` 段会被上游配置覆盖，导致 CI 被意外自动触发。  
**规则**：每次同步后必须检查所有 workflow 文件的触发条件。

### Wire DI 变更后忘记重新生成

修改 Wire providers 后忘记运行 `go generate ./cmd/server`，导致 `wire_gen.go` 与 `wire.go` 不一致，编译失败。

### fork 注入点被覆盖

`wire_gen.go` 中 lingjing 和 promptAnalytics 的初始化片段（约 10 行）在上游同步时可能丢失，因为上游会重新生成该文件。  
**规则**：同步后手动核对 `wire_gen.go` 中的 fork 注入点（见第 4 章速查表）。

---

*文档由 Claude Code 根据代码库自动整理，最后更新：2026-05-18*
