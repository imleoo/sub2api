# 账号管理接入方式精简方案

## 目标

移除 OAuth、Setup Token、Upstream 等接入方式，**保留 `apikey`、`bedrock`、`service_account`**，同时移除 Antigravity 平台。

---

## 现状梳理

### 当前支持的接入类型（`AccountType`）

| 类型 | 说明 | 平台 | 处理 |
|------|------|------|------|
| `apikey` | 直接填写 API Key | 全平台 | **保留** |
| `bedrock` | AWS Bedrock SigV4 / API Key | Anthropic | **保留** |
| `service_account` | Google Vertex AI Service Account JSON | Gemini / Anthropic | **保留** |
| `oauth` | OAuth 授权流程 | Anthropic / OpenAI / Gemini / Antigravity | 移除 |
| `setup-token` | Claude Code 专用 setup token | Anthropic | 移除 |
| `upstream` | 自定义上游代理 | Antigravity | 移除（随平台一起移除） |

### 平台处理

| 平台 | 处理 |
|------|------|
| Anthropic | **保留**（apikey + bedrock） |
| OpenAI | **保留**（apikey） |
| Gemini | **保留**（apikey + service_account） |
| Antigravity | **移除**（整个平台去掉） |

---

## 需要删除/修改的文件清单

### 后端

#### 完整删除（文件整体移除）

| 文件 | 行数 | 说明 |
|------|------|------|
| `internal/service/oauth_service.go` | 309 | Anthropic OAuth 通用服务 |
| `internal/service/openai_oauth_service.go` | 379 | OpenAI OAuth |
| `internal/service/gemini_oauth_service.go` | 1099 | Gemini OAuth |
| `internal/service/antigravity_oauth_service.go` | 485 | Antigravity 全部（含 upstream） |
| `internal/handler/admin/openai_oauth_handler.go` | 264 | OpenAI OAuth handler |
| `internal/handler/admin/gemini_oauth_handler.go` | 146 | Gemini OAuth handler |
| `internal/handler/admin/antigravity_oauth_handler.go` | 91 | Antigravity OAuth handler |

#### 修改（局部删除）

**`internal/handler/admin/account_handler.go`**（2226 行）
- 删除 `GenerateAuthURL`、`GenerateSetupTokenURL`、`ExchangeCode`、`ExchangeSetupTokenCode`、`CookieAuth`、`SetupTokenCookieAuth` 方法
- 删除 `oauthService *service.OAuthService` 字段及注入
- `CreateAccountRequest.Type` binding 校验改为 `oneof=apikey bedrock service_account`
- 删除 `CreateAccount` 中 `upstream` 凭证构建分支，保留 `bedrock`/`service_account` 分支
- 删除 Antigravity 平台相关的账号处理逻辑

**`internal/server/routes/admin.go`**
- 删除以下路由注册：
  ```
  accounts.POST("/generate-auth-url", ...)
  accounts.POST("/generate-setup-token-url", ...)
  accounts.POST("/exchange-code", ...)
  accounts.POST("/exchange-setup-token-code", ...)
  accounts.POST("/cookie-auth", ...)
  accounts.POST("/setup-token-cookie-auth", ...)
  openai.POST("/generate-auth-url", ...)
  openai.POST("/exchange-code", ...)
  gemini.POST("/oauth/exchange-code", ...)
  antigravity.POST("/oauth/exchange-code", ...)
  ```
- 删除 `antigravity` 路由组整体

**`cmd/server/wire_gen.go` / Wire providers**
- 移除所有 OAuth service/handler 和 Antigravity service/handler 的注入

**`ent/schema/account.go`**
- `platform` 枚举移除 `antigravity`（非必须，保留兼容性也可）
- `type` 枚举移除 `oauth`、`setup-token`、`upstream`

### 前端

#### 完整删除

| 文件 | 说明 |
|------|------|
| `src/components/account/OAuthAuthorizationFlow.vue` | OAuth 授权流程组件 |
| `src/components/account/ReAuthAccountModal.vue` | 重新授权弹窗 |

#### 修改（局部删除）

**`src/components/account/CreateAccountModal.vue`**（5361 行 → 预计精简至 ~1500 行）

删除：
- Antigravity 平台选项卡及其所有表单区块（`form.platform === 'antigravity'` 分支）
- Anthropic 平台的 `oauth-based`、`setup-token` 选项卡，保留 `apikey` 和 `bedrock`
- OpenAI 平台的 `oauth-based` 选项，保留 `apikey`
- Gemini 平台的 `oauth-based` 选项，保留 `apikey` 和 `service_account`
- Step 2 的整个 OAuth 授权流程区块（`<OAuthAuthorizationFlow>`，第 2786 行附近）
- Upstream 配置区块（第 777–800 行）
- 所有 `isOAuthFlow`、`oauthFlowRef`、`antigravityAccountType`、`upstreamXxx` 相关 ref/computed/watch

保留：
- 平台选择（Anthropic / OpenAI / Gemini，去掉 Antigravity）
- `apikey` 分支的 API Key 输入表单
- `bedrock` 分支的 AWS 凭证表单（`bedrockXxx` refs 保留）
- `service_account` 分支的 Vertex AI JSON 上传（`vertexXxx` refs 保留）
- 通用配置（并发、优先级、代理、分组、模型映射、过期时间等）

**`src/components/account/EditAccountModal.vue`**（164KB）
- 同上，删除 OAuth/Upstream/Antigravity 相关区块，保留 Bedrock/Service Account

**`src/types/index.ts`**
```typescript
// 修改前
type AccountType = 'oauth' | 'setup-token' | 'apikey' | 'upstream' | 'bedrock' | 'service_account'
type AccountPlatform = 'anthropic' | 'openai' | 'gemini' | 'antigravity'

// 修改后
type AccountType = 'apikey' | 'bedrock' | 'service_account'
type AccountPlatform = 'anthropic' | 'openai' | 'gemini'
```

**`src/api/admin/accounts.ts`**
- 删除 `generateAuthUrl`、`exchangeCode`、`cookieAuth` 等 OAuth 相关 API 函数
- 删除 Antigravity 相关 API 函数

**`src/constants/account.ts`**
- 保留 `VERTEX_LOCATION_OPTIONS`（Vertex AI 保留）
- 删除 Antigravity 相关常量

---

## 精简后的添加账号流程

```
选择平台（Anthropic / OpenAI / Gemini）
    ↓
选择接入类型
  Anthropic: API Key | AWS Bedrock
  OpenAI:    API Key
  Gemini:    API Key | Vertex AI Service Account
    ↓
填写对应凭证
    ↓
配置通用参数（并发、优先级、代理、分组、模型映射）
    ↓
提交
```

---

## 实施建议

1. **先删后端**：Wire DI 移除 OAuth/Antigravity 注入 → 删除对应 service/handler 文件 → 修改 `account_handler.go` → 删除路由 → 编译验证
2. **再删前端**：先删 `OAuthAuthorizationFlow.vue` 和 `ReAuthAccountModal.vue` → 精简 `CreateAccountModal.vue` 和 `EditAccountModal.vue` → 更新类型定义
3. **验证**：`go test -tags=unit ./internal/handler/...` + `pnpm run typecheck`

---

## 风险提示

- 数据库中现有 `oauth`/`upstream` 类型或 `antigravity` 平台的账号记录，删除后前端展示需要 fallback 处理（建议后端 `ListAccounts` 过滤，或前端对未知类型显示"已停用"）
- `require_oauth_only` 分组限制逻辑（`account_service.go:176` 和 `292`）需一并评估是否保留或删除
