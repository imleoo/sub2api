# 模型体验（Playground）实现方案

> 目标：在用户端新增「模型体验」功能（对标 new-api Playground）。用户用**自己已创建的网关 API Key** 进行**对话**与**生图**，全程复用现有网关的鉴权 / 计费 / 限流 / 负载均衡链路。
>
> 状态：设计方案（未开工）｜分支：zhiguofan｜作者：Claude Code 调研
>
> 关键前提已核验：后端对话与生图端点**均已存在且完整**，本功能**后端几乎零改动**，主体是前端工程。

---

## 1. 需求与范围

| 项 | 结论 |
|----|------|
| 谁用 | 登录用户（`requiresAuth: true`，非管理员） |
| 用什么凭证 | 用户在 `KeysView` 里创建的**网关 Key**（`api_keys.key`，形如 `sk-...`） |
| 能力 | ① 多轮文本对话（流式 SSE）② **文生图**（`/v1/images/generations`）③ **图生图 / 图片编辑**（`/v1/images/edits`，上传图 + prompt） |
| UI 风格 | **对标 ChatGPT**：统一对话入口（居中欢迎语 + 胶囊输入框 + 底部快捷 chip），意图路由而非 tab 切换 |
| 鉴权架构 | **前端直连 `/v1`**，裸 `fetch` 携带 `Authorization: Bearer <用户key>`（已与需求方确认） |
| 计费 | 走现有网关链路，**真实扣费/扣配额**（与正式调用一致，非 mock）——**必须显著风险提示**（见 §5.4） |

**明确不做（首版）**：视频（灵境 `/lingjing/v1/video/*`）、多 Key 并发对比、对话历史落库、语音输入。

---

## 2. 调研结论（带证据）

### 2.1 两套 Key 概念（务必区分）

| 概念 | 表 | 谁建 | 存什么 | Playground 用哪个 |
|------|-----|------|--------|:---:|
| **网关 Key** | `api_keys` | 普通用户 | 本网关签发的 `sk-xxx` | ✅ **用这个** |
| **上游账号 Key** | `accounts` | 管理员 | 上游平台真实凭证（JSONB `credentials`） | ❌ 用户无权访问 |

- 网关 Key Schema：`backend/ent/schema/api_key.go:34`（表 `api_keys`），关键字段 `key`(`:37`) / `group_id`(`:44`) / `status`(`:47`) / 配额 `quota`,`quota_used`(`:63`,`:68`) / 限速 `rate_limit_5h/1d/7d`(`:80`起)。
- 上游账号 Schema：`backend/ent/schema/account.go:51`，凭证在 `credentials` JSONB(`:87`)。
- 二者通过 **group** 间接连接：用户 Key 绑 group → group 经 `account_groups` 关联一组上游 account。

### 2.2 关键命门：列表接口返回明文 Key ✅

`GET /api/v1/keys` → `dto.APIKeyFromService` **直接输出明文 `key` 字段**：
- `backend/internal/handler/dto/mappers.go:85`：`Key: k.Key`
- `backend/internal/handler/dto/types.go:55`：`Key string \`json:"key"\``
- 且同时返回 `Group`（含 `platform`、`allow_image_generation` 等，`mappers.go:107`）。

> ∴ 前端调现成的 `keysAPI.list()` 即可拿到「明文 key + 它的 group 能力」，无需任何后端新增接口。这是「前端直连方案」成立的基础。

### 2.3 网关端点全部现成

路由定义 `backend/internal/server/routes/gateway.go`，`/v1` 组中间件链（`:26-42`）：
`RequestBodyLimit → ClientRequestID → OpsErrorLogger → InboundEndpoint → APIKeyAuth（鉴权+计费准入）→ RequireGroupAssignment`。

Playground 会用到的端点（均 `Authorization: Bearer <key>` 鉴权）：

| 用途 | 方法 & 路径 | gateway.go | 说明 |
|------|-------------|:---:|------|
| Claude 对话 | `POST /v1/messages` | `:45` | 按入站协议分流 Claude/OpenAI handler |
| OpenAI 对话 | `POST /v1/chat/completions` | `:86` | 同上 |
| **文生图** | `POST /v1/images/generations` | `:106` | **仅 OpenAI 入站或 lingjing 平台**，否则 404(`:107-118`) |
| **图生图/编辑** | `POST /v1/images/edits` | `:119` | `multipart/form-data`，同样受生图能力门约束 |
| 该 key 可用模型 | `GET /v1/models` | `:67` | 返回**该 key 绑定 group** 能访问的模型 |

图生图（`/v1/images/edits`）后端已支持，multipart 字段（解析于 `internal/service/openai_images.go:356-388`，标准 OpenAI 契约）：

| 字段 | 类型 | 说明 |
|------|------|------|
| `image` / `image[]` | 文件 | 待编辑图（支持多图，`:356`） |
| `mask` | 文件（可选） | 遮罩，透明处为编辑区（`:343`） |
| `prompt` | 文本 | 编辑指令 |
| `model` / `size` / `n` / `response_format` | 文本 | 同文生图 |

- 对话调用链：`gateway_handler.go:116 Messages` → 账号选择 `SelectAccountWithLoadAwareness`(`gateway_service.go:1188`) → 转发 `Forward`(`gateway_service.go:3814`) → 异步记账 `RecordUsage`(`gateway_service.go:8267`)。流式为 `Content-Type: text/event-stream`。
- 生图调用链：`openai_images.go:23 Images` → 权限门 `GroupAllowsImageGeneration`(`:85`) → 生图并发槽 `acquireImageGenerationSlot`(`:93`) → 账号选择 `SelectAccountWithSchedulerForImages` → 转发 `ForwardImages`(`:538`)，响应解析 `b64_json`/`image_url`。

### 2.4 生图能力门（重要约束）

`/v1/images/generations` 对**非 OpenAI 入站且非 lingjing** 的请求返回 **404**（`gateway.go:107-118`），并且分组需 `AllowImageGeneration=true`（`backend/ent/schema/group.go:101`，权限门 `image_generation_intent.go:21`）。

> ∴ 只有当**用户选中的 key 所属 group** 满足「platform=openai（或 lingjing）且 allow_image_generation=true」时，生图 tab 才可用。前端需据此**探测并禁用**不支持的能力。

### 2.5 new-api 对照（借鉴与差异）

- new-api Playground 用**登录态 + 内存临时 token 复用 relay 管线**（`controller/playground.go`，路由 `/pg/chat/completions`），用户**无需先建 key**。
- 但 new-api Playground **不支持生图**（Issue #4503 未实现），我们的需求（用用户自己的 key + 生图）比它更完整。
- 值得抄的设计点：① 参数面板每个参数**独立启用开关**（禁用的不进 payload，避免给不支持的模型硬塞报错）② 分组下拉展示倍率做**计费透明化** ③ 流式对 `reasoning_content`/`<think>` 折叠处理 ④ Debug 面板（Request/Response/原始 SSE）+ 调用代码片段。

### 2.6 前端可复用件

- **SSE 流式范例**：`frontend/src/components/account/AccountTestModal.vue:407-483` —— 因 `EventSource` 不支持 POST，用 `fetch(POST) + response.body.getReader() + TextDecoder` 逐块解析 `data:` 行，`AbortController` 中断。**Playground 直接照搬此模式**。
- **注意**：裸 `fetch` **不走 axios 拦截器**，`Authorization` 需手动加，且不会自动触发 token 刷新（见 §5.3 坑）。
- 目前**没有**任何聊天 UI（消息气泡 / markdown 流式渲染 / 生图展示），需新建。

---

## 3. 架构决策

**采用「前端直连 `/v1`」（需求方已确认）。**

```
┌─────────────┐   ① keysAPI.list()      ┌──────────────────────┐
│ Playground  │ ─────────────────────▶  │ /api/v1/keys (JWT)   │  → 明文 key + group 能力
│   (Vue)     │                          └──────────────────────┘
│             │   ② Bearer <用户key>     ┌──────────────────────┐
│  裸 fetch   │ ─────────────────────▶  │ /v1/messages         │  对话(SSE)
│             │                          │ /v1/chat/completions │
│             │                          │ /v1/images/generations│ 生图
│             │                          └──────────┬───────────┘
└─────────────┘         已有中间件：鉴权│计费│限流│负载 ▼  上游 AI
```

**权衡（诚实说明）**：

| | 前端直连（采用） | 后端代理端点（未采用） |
|--|--|--|
| 后端工作量 | 近乎零 | 新增 handler/service/路由 |
| Key 暴露 | 出现在前端请求头（但**本就是用户自己的 key**，`KeysView` 里可见可复制，无新增泄露面） | key 不出后端 |
| 拦截器 | 裸 fetch 不走 axios，需手动加 token、手动处理 401 | 复用 axios |
| 专属审计/限流 | 无（与正式调用同源） | 可加体验专属策略 |

> 结论：因 key 明文本就对用户可见、且后端零改动即可上线，前端直连是投入产出比最优解。后续若要「体验免费额度 / 专属限流」，再演进到后端代理端点即可（本方案预留该演进路径）。

---

## 4. 详细设计（前端为主）

### 4.1 文件清单

| 动作 | 文件 | 说明 |
|------|------|------|
| 新建 | `frontend/src/views/user/PlaygroundView.vue` | 页面主体 |
| 新建 | `frontend/src/api/playground.ts` | 封装带 key 的 fetch（对话 SSE + 生图） |
| 新建 | `frontend/src/components/playground/*` | 消息气泡、参数面板、生图结果卡等子组件 |
| 改 | `frontend/src/router/index.ts` | 用户段加 `/playground` 路由 |
| 改 | `frontend/src/api/index.ts` | re-export `playgroundAPI` |
| 改 | `frontend/src/components/layout/AppSidebar.vue` | `buildSelfNavItems` 内 `/models` 后加菜单项 |
| 改 | `frontend/src/i18n/locales/en.ts` + `zh.ts` | `nav.playground` + `playground.*` 文案（**必须两文件对齐**，否则 i18n 一致性测试失败） |
| 新建 | `frontend/src/views/user/__tests__/PlaygroundView.spec.ts` | 单测（Vitest 覆盖率门槛 80%） |

### 4.2 路由（router/index.ts，用户段）

```ts
{
  path: '/playground',
  name: 'Playground',
  component: () => import('@/views/user/PlaygroundView.vue'),
  meta: {
    requiresAuth: true,
    requiresAdmin: false,
    titleKey: 'playground.title',
    descriptionKey: 'playground.description',
    hideInSimpleMode: false // Simple 模式下也可用（无计费）
  }
}
```

### 4.3 侧边栏（AppSidebar.vue，`buildSelfNavItems`，约 `:677`）

```ts
{ path: '/models', label: t('nav.models'), icon: ModelGridIcon, hideInSimpleMode: true },
// 新增（复用或新增图标）：
{ path: '/playground', label: t('nav.playground'), icon: PlaygroundIcon, hideInSimpleMode: false },
```

`userNavItems` 与管理员 `personalNavItems` 都调用 `buildSelfNavItems`，加一行两处生效。

### 4.4 UI 设计：ChatGPT 风格统一入口（对标参考图）

**核心理念**：不做「对话 tab / 生图 tab」切换，而是**一个统一对话入口**——用户在同一个输入框里，通过「底部快捷 chip」或「+ 上传图片」表达意图，前端把不同意图路由到不同 `/v1` 端点。所有结果（文字、生成的图）都以**消息气泡**形式落在同一条对话流里。

#### 空态（欢迎屏，居中）

```
                         今天有什么计划?                 ← 大标题(t('playground.greeting'))

     ╭──────────────────────────────────────────────────╮
     │  ⊕   有问题，尽管问                    [模型 ⌄]  ▶ │   ← 胶囊输入框(圆角~28px)
     ╰──────────────────────────────────────────────────╯
       └ +上传图                         └ Key/模型选择器  └ 发送

        ┌ 🖼 生成图片 ┐  ┌ 🎨 图生图 ┐              ← 底部快捷 chip(点击=切换意图)
        └───────────┘  └──────────┘
```

- 布局照参考图：垂直居中的欢迎语 + 大圆角胶囊输入框，下方一排 chip。
- **去掉**参考图里的语音波形/麦克风/查找资料（不做）；**保留并复用**「模型选择器」位置放我们的 **Key + 模型选择器**。
- chip：`生成图片`（文生图意图）、`图生图`（提示点 ⊕ 上传，或直接触发上传）。

#### 对话态（发送首条后）

```
┌───────────────────────────────────────────────┐
│  [顶部条] Key: my-key(OpenAI/默认组)  模型:gpt… │  ← 固定，可切换
├───────────────────────────────────────────────┤
│                              你: 画一只赛博猫 ▷ │  ← 用户气泡(右)
│  ◀ 助手: 好的，这是生成结果：                    │  ← 助手气泡(左)
│     ┌────────┐                                  │
│     │ [生成图] │  ← 生图结果直接嵌在气泡里         │
│     └────────┘   [下载] [作为图生图输入]          │
│  ◀ 助手: 流式文字逐字输出…▍                       │  ← 对话流式
├───────────────────────────────────────────────┤
│  ╭─────────────────────────────────────────╮   │
│  │ ⊕  继续输入…              [模型⌄]     ▶ │   │  ← 输入框固定底部
│  ╰─────────────────────────────────────────╯   │
│    [🖼生成图片] [🎨图生图]     ⚠ 消耗真实额度    │  ← chip + 风险提示常驻
└───────────────────────────────────────────────┘
```

#### 意图路由（同一输入框 → 不同端点）

| 用户动作 | 意图 | 端点 | 载荷 |
|----------|------|------|------|
| 直接输入文字发送 | 对话 | `/v1/chat/completions`（OpenAI 组）或 `/v1/messages`（Claude 组） | JSON, `stream:true` |
| 点「🖼 生成图片」chip 后输入 | 文生图 | `/v1/images/generations` | JSON |
| 点 ⊕ 上传图片 + 输入（或点「🎨 图生图」） | 图生图 | `/v1/images/edits` | `multipart/form-data` |

> 意图用一个 `mode: 'chat' | 'image' | 'edit'` 状态标识，chip 高亮当前意图；上传图片自动切到 `edit`。生成的图气泡上提供「作为图生图输入」按钮，一键把该图回填为 `edit` 的输入图，形成迭代闭环。

#### 组件拆分建议

- `PlaygroundView.vue`：状态编排（消息数组、当前 key、mode、参数）。
- `components/playground/ChatComposer.vue`：胶囊输入框（⊕ 上传、文本、Key/模型选择器、发送）+ 底部 chip。
- `components/playground/MessageList.vue` + `MessageBubble.vue`：气泡（文本 markdown 流式、reasoning 折叠、图片网格、错误态）。
- `components/playground/ImageResult.vue`：生图结果卡（b64/url 渲染、下载、作为图生图输入）。
- `components/playground/ParamPanel.vue`：参数面板（temperature/max_tokens/system…，每项独立启用开关）。

### 4.5 能力探测规则

| 能力 | 判定 | 数据来源 | 不满足时的 UI |
|------|------|----------|----------------|
| 对话可用模型 | `GET /v1/models`（带选中 key）返回列表 | 网关端点，最准确 | 模型下拉为空 → 提示该 key 无可用模型 |
| 文生图 / 图生图 | `key.group.platform ∈ {openai, lingjing}` **且** `key.group.allow_image_generation === true` | `keysAPI.list()` 的 `group` | **灰化**「生成图片 / 图生图」chip，悬浮提示原因 |
| 无可用 key | 列表为空或全 inactive | `keysAPI.list()` | 空态引导「先去创建 API Key」跳 `/keys` |

> 生图不可用时禁用相关 chip 并提示：「当前 Key 所属分组未开启生图，请切换到 OpenAI 类且已开启生图的分组」。**不要**让用户发出注定 404 的请求。

### 4.6 对话请求（playground.ts，SSE 核心，照搬 AccountTestModal 模式）

```ts
import { buildGatewayUrl } from './url' // 指向 /v1 而非 /api/v1

export async function chatStream(opts: {
  apiKey: string
  model: string
  messages: Array<{ role: string; content: string }>
  params: Record<string, unknown>   // 仅含「已启用」的参数
  signal: AbortSignal
  onDelta: (text: string) => void
  onReasoning?: (text: string) => void
  onError: (e: unknown) => void
  onDone: () => void
}) {
  const res = await fetch(buildGatewayUrl('/v1/chat/completions'), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${opts.apiKey}`, // 手动加，裸 fetch 不走 axios 拦截器
    },
    body: JSON.stringify({ model: opts.model, messages: opts.messages, stream: true, ...opts.params }),
    signal: opts.signal,
  })
  if (!res.ok || !res.body) { opts.onError(await safeErr(res)); return }
  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buf = ''
  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buf += decoder.decode(value, { stream: true })
    const lines = buf.split('\n'); buf = lines.pop() ?? ''
    for (const line of lines) {
      const s = line.trim()
      if (!s.startsWith('data:')) continue
      const payload = s.slice(5).trim()
      if (payload === '[DONE]') { opts.onDone(); return }
      try {
        const j = JSON.parse(payload)
        const d = j.choices?.[0]?.delta
        if (d?.reasoning_content) opts.onReasoning?.(d.reasoning_content)
        if (d?.content) opts.onDelta(d.content)
      } catch { /* 忽略非 JSON 心跳/ping 行 */ }
    }
  }
  opts.onDone()
}
```

> 需同时兼容 Claude 入站（`/v1/messages`，事件为 `content_block_delta` 等）与 OpenAI 入站（`choices[].delta`）。建议按选中 key 的 group.platform 决定走哪个端点与哪套解析。首版可优先 OpenAI 兼容格式（`/v1/chat/completions`），Claude 分组走 `/v1/messages`。

### 4.7 生图请求（非流式，OpenAI Images 格式）

```ts
export async function imageGenerate(opts: {
  apiKey: string; model: string; prompt: string; size: string; n: number
}): Promise<Array<{ b64?: string; url?: string }>> {
  const res = await fetch(buildGatewayUrl('/v1/images/generations'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${opts.apiKey}` },
    body: JSON.stringify({ model: opts.model, prompt: opts.prompt, size: opts.size, n: opts.n }),
  })
  if (!res.ok) throw await safeErr(res)
  const j = await res.json()
  return (j.data ?? []).map((it: any) => ({ b64: it.b64_json, url: it.url }))
}
```

渲染：`b64_json` → `<img :src="'data:image/png;base64,' + b64">`；`url` → 直接 `<img :src="url">`。支持下载。

### 4.7b 图生图请求（multipart，`/v1/images/edits`）

```ts
export async function imageEdit(opts: {
  apiKey: string; model: string; prompt: string; size: string; n: number
  images: File[]            // ⊕ 上传的图片（支持多张）
  mask?: File               // 可选遮罩
}): Promise<Array<{ b64?: string; url?: string }>> {
  const form = new FormData()
  form.append('model', opts.model)
  form.append('prompt', opts.prompt)
  form.append('size', opts.size)
  form.append('n', String(opts.n))
  // 后端接受 image 或 image[]（openai_images.go:356）；多图用重复 image 字段
  opts.images.forEach((f) => form.append('image', f))
  if (opts.mask) form.append('mask', opts.mask)

  const res = await fetch(buildGatewayUrl('/v1/images/edits'), {
    method: 'POST',
    headers: { Authorization: `Bearer ${opts.apiKey}` }, // 注意：multipart 不要手动设 Content-Type，让浏览器带 boundary
    body: form,
  })
  if (!res.ok) throw await safeErr(res)
  const j = await res.json()
  return (j.data ?? []).map((it: any) => ({ b64: it.b64_json, url: it.url }))
}
```

> **坑**：`multipart/form-data` **不要**手动设 `Content-Type`——浏览器会自动带上含 `boundary` 的头，手动设会导致后端解析 boundary 失败（`openai_images.go:316` 会报 boundary 缺失）。上传图注意前端做大小校验（后端单 part 上限 20MB，`openai_images.go:41`）。

### 4.8 i18n

- `nav:` 块加 `playground: 'Playground' / '模型体验'`。
- 新增顶层块 `playground: { title, description, selectKey, noKey, chat, image, send, stop, imageNotAvailable, ... }`。
- **en.ts 与 zh.ts 键结构必须一一对应**，参照现有 `models:` 块写法。

---

## 5. 后端改动与坑

### 5.1 后端改动：无（默认）

对话、生图、模型列表端点全部现成，鉴权/计费/限流/负载均衡自动生效。**首版不需要任何后端 Go 改动**。

### 5.2 可能需要的后端微调（按需，非必须）

- **CORS**：确认本地/生产部署下 `/v1/*` 允许前端来源携带 `Authorization` 头。生产为前端嵌入同源（`go build -tags embed`）通常无碍；本地 `dev_local.sh`（后端 :8082 / 前端 :3002）**跨端口**，需确认网关组 CORS 允许 `Authorization`。→ 落地前用 `dev_local.sh` 实测一次预检请求。
- 若未来要「体验专属免费额度」，才需要后端代理端点（本方案不含）。

### 5.3 前端必须处理的坑

1. **裸 fetch 不走 axios 拦截器**：① `Authorization` 手动加；② token 过期不会自动刷新——`/v1` 用的是**网关 key**（不是 JWT），本身不过期，但拉 key 列表 `keysAPI.list()` 走 axios（JWT）会自动刷新，二者分离，无冲突。
2. **真实计费**：体验会真实扣用户余额/配额/限速窗口。UI 需明确提示「模型体验会消耗真实额度」，并友好展示 429（限速/配额）、403（订阅/余额）错误。
3. **生图 404**：非 OpenAI/lingjing 分组打生图端点返回 404 —— 必须靠 §4.5 探测**提前禁用**，不要让用户发出注定 404 的请求。
4. **流式错误**：区分「正常 `[DONE]` 结束」与「中途 abort/网络断」，避免正常结束被误报错（参考 AccountTestModal 的 `isStreamComplete` 判断）。
5. **SSE 已开始后的错误**：网关在流已开始时把错误作为 SSE 事件写出（`gateway_handler.go:1495`），前端需解析流内错误事件并渲染到当前气泡。
6. **Simple 模式**：`RUN_MODE=simple` 下无计费，鉴权中间件提前返回，Playground 照常可用（去掉计费提示即可）。

### 5.4 风险提示设计（明确、多处、不可忽略）

体验会**真实扣费 / 扣配额 / 占用限速窗口**，与正式 API 调用完全一致。风险提示分四层落地：

**① 进入页面：一次性确认横幅（localStorage 记住）**

```
┌────────────────────────────────────────────────────────────┐
│ ⚠ 模型体验将使用你选定的 API Key 发起真实请求，会消耗真实额度  │
│   （余额 / 配额 / 限速窗口），与正式调用一致，不可退款。        │
│                                    [我已知晓，继续]           │
└────────────────────────────────────────────────────────────┘
```
- 首次进入必须点「我已知晓」才可用；记 `localStorage['playground_risk_ack']`，后续进入折叠为顶部细条。
- Simple 模式（`RUN_MODE=simple`）下**隐藏**此提示（无计费）。

**② 输入区常驻提示（每次发送前可见）**
- 输入框下方常驻一行浅色小字：`⚠ 消耗真实额度`；hover 展开完整说明。
- 发送按钮 tooltip 附带「本次将真实计费」。

**③ 结果区实时用量回显（发送后）**
- 每条助手气泡底部展示本次 **实际用量**：从上游返回的 `usage`（tokens / 图片数）+ 估算成本（若能拿到分组倍率）。
- 生图气泡明确标注「本次生成 N 张图，已计费」。

**④ 错误态友好化（区分计费类错误）**

| HTTP | 含义 | 提示文案 |
|------|------|----------|
| 402/403 | 余额不足 / 订阅超限 | 「余额或订阅额度不足，请充值或更换 Key」 + 跳转充值 |
| 429 | 限速 / 配额窗口耗尽 | 「触发限速（5h/1d/7d 窗口）或配额已用尽，请稍后再试」 |
| 404（生图） | 分组不支持生图 | 「当前 Key 分组未开启生图」（本应被 §4.5 探测拦截，此为兜底） |

> 设计原则：**花钱之前充分知情**（①②），**花钱之后清楚花了多少**（③），**出错时知道为什么、怎么办**（④）。

---

## 6. 实施步骤（建议顺序）

1. **P1 骨架 + ChatGPT 空态**：路由 + 菜单 + i18n 键 + 居中欢迎语 + 胶囊输入框 + 底部 chip（静态） → 页面可进入、样式对齐参考图。
2. **P2 Key 选择器 + 风险横幅**：`keysAPI.list()` 拉取，输入框内 Key/模型选择器；§5.4 ① 一次性风险确认横幅。
3. **P3 对话（OpenAI 兼容）**：`playground.ts chatStream` + 消息气泡 + 流式渲染 + 参数面板（独立开关）+ 中断；对话态布局切换。
4. **P4 对话（Claude 分组）**：`/v1/messages` 端点与 `content_block_delta` 事件解析适配。
5. **P5 文生图**：能力探测（灰化 chip）+「生成图片」意图 + `imageGenerate` + 结果气泡（b64/url）+ 下载。
6. **P6 图生图**：⊕ 上传（多图/大小校验）+「图生图」意图 + `imageEdit`（multipart）+「生成图作为图生图输入」迭代闭环。
7. **P7 风险回显 + 打磨**：§5.4 ②③④（常驻提示、用量回显、错误态友好化）、reasoning 折叠、Debug 面板（可选）。
8. **P8 测试**：Vitest 单测达 80% 覆盖；本地 `dev_local.sh` 端到端手测 CORS + 真实对话/文生图/图生图。

粗估：前端约 P1–P8 集中在 `views/user` + `api` + `components/playground`，**不触碰后端与 Ent/Wire**，风险与合并冲突面小。

---

## 7. 测试计划

| 层 | 内容 |
|----|------|
| 单测（Vitest） | Key 选择器渲染/筛选、能力探测（生图 chip 灰化）、意图路由（chat/image/edit → 端点映射）、SSE 解析（delta/reasoning/[DONE]/错误行）、生图结果渲染（b64 vs url）、图生图 FormData 构造（image/mask 字段、不手动设 Content-Type）、风险横幅 ack 逻辑 |
| 手测（dev_local） | 真实 key 对话（Claude + OpenAI 分组各一）、文生图（OpenAI 分组）、图生图（上传图 + prompt）、「生成图作为图生图输入」闭环、生图禁用（Claude 分组灰化）、429/403/404 错误展示、中断、风险横幅、用量回显、CORS 预检（含 multipart 上传） |
| 回归 | 不改后端 → 后端测试无需跑；前端跑 `pnpm run lint:check` + `pnpm run typecheck` + 相关 `vitest run` |

---

## 8. 风险与后续演进

| 风险 | 缓解 |
|------|------|
| 用户误以为体验免费，产生真实扣费投诉 | UI 显著提示「消耗真实额度」，展示本次预估/实际用量 |
| 本地跨端口 CORS 拦截 Authorization | 落地前实测预检；必要时后端网关组补 CORS 允许头 |
| Claude vs OpenAI 两套流式格式维护成本 | 首版按 group.platform 分派；抽象一个 `parseStreamChunk(platform)` |
| 未来要「免费体验额度」 | 演进到后端代理端点 `/api/v1/playground/*`（JWT + 内存临时策略），本方案已预留路径 |

---

## 附：关键文件索引

- 路由：`backend/internal/server/routes/gateway.go`（`/v1` 组 `:36`）、`user.go:59`（`/keys`）
- 鉴权：`backend/internal/server/middleware/api_key_auth.go:29`
- 对话：`backend/internal/handler/gateway_handler.go:116`、`internal/service/gateway_service.go:3814`
- 生图：`backend/internal/handler/openai_images.go:23`、`internal/service/openai_images.go`
- Key DTO（明文）：`backend/internal/handler/dto/mappers.go:85`、`types.go:55`
- 前端 SSE 范例：`frontend/src/components/account/AccountTestModal.vue:407`
- 前端 Key API：`frontend/src/api/keys.ts`、路由 `frontend/src/router/index.ts`、侧栏 `frontend/src/components/layout/AppSidebar.vue:677`
