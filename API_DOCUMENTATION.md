# API 调用文档

本文档面向使用平台 API Key 调用 AI 模型的开发者。

- **Base URL**：`https://openclaw.zhiguo.fan`
- **认证**：请求头携带 `Authorization: Bearer <your-api-key>`

---

## 目录

1. [认证方式](#认证方式)
2. [Claude API（Anthropic 兼容）](#claude-apianthropic-兼容)
3. [OpenAI Chat Completions](#openai-chat-completions)
4. [OpenAI 推理模型（o 系列）](#openai-推理模型o-系列)
5. [图像生成](#图像生成)
6. [Gemini API 兼容](#gemini-api-兼容)
7. [通用端点](#通用端点)
8. [流式响应](#流式响应)
9. [支持的模型](#支持的模型)
10. [错误处理](#错误处理)

---

## 认证方式

所有请求必须携带 API Key，支持以下三种方式（三选一）：

```http
Authorization: Bearer sk_live_xxxxx
```
```http
x-api-key: sk_live_xxxxx
```
```http
x-goog-api-key: sk_live_xxxxx
```

> `Authorization: Bearer` 为推荐方式，与主流 SDK 兼容性最好。

---

## Claude API（Anthropic 兼容）

### 发送消息

```
POST /v1/messages
```

**curl 示例：**

```bash
curl https://openclaw.zhiguo.fan/v1/messages \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "max_tokens": 1024,
    "messages": [
      { "role": "user", "content": "你好，介绍一下你自己。" }
    ]
  }'
```

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `model` | string | ✅ | 模型 ID，见[支持的模型](#claude-模型) |
| `max_tokens` | integer | ✅ | 最大输出 token 数 |
| `messages` | array | ✅ | 对话消息列表 |
| `system` | string | ❌ | System prompt |
| `temperature` | float | ❌ | 随机性，范围 0~1，默认 1.0 |
| `top_p` | float | ❌ | Top-p 采样 |
| `top_k` | integer | ❌ | Top-k 采样 |
| `stream` | boolean | ❌ | 是否流式响应，默认 false |

**响应示例：**

```json
{
  "id": "resp_0e218dc6bbfca132...",
  "type": "message",
  "role": "assistant",
  "content": [
    { "type": "text", "text": "你好！我是 Claude，一个由 Anthropic 开发的 AI 助手..." }
  ],
  "model": "claude-sonnet-4-5",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 18,
    "output_tokens": 42,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 0
  }
}
```

**多轮对话示例：**

```bash
curl https://openclaw.zhiguo.fan/v1/messages \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "max_tokens": 1024,
    "messages": [
      { "role": "user", "content": "法国的首都是哪里？" },
      { "role": "assistant", "content": "法国的首都是巴黎。" },
      { "role": "user", "content": "它有多少人口？" }
    ]
  }'
```

**带 System Prompt 示例：**

```bash
curl https://openclaw.zhiguo.fan/v1/messages \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-opus-4-7",
    "max_tokens": 2048,
    "system": "你是一个专业的代码审查助手，只使用中文回复，并给出具体的改进建议。",
    "messages": [
      { "role": "user", "content": "帮我审查这段 Python 代码：\ndef add(a,b):\n  return a+b" }
    ]
  }'
```

**流式调用示例：**

```bash
curl https://openclaw.zhiguo.fan/v1/messages \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "max_tokens": 1024,
    "stream": true,
    "messages": [
      { "role": "user", "content": "写一首关于秋天的诗。" }
    ]
  }'
```

---

### 计算 Token 数

```
POST /v1/messages/count_tokens
```

提前估算 token 消耗，不生成实际内容。

```bash
curl https://openclaw.zhiguo.fan/v1/messages/count_tokens \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "messages": [
      { "role": "user", "content": "请帮我写一篇关于人工智能的文章" }
    ]
  }'
```

**响应：**

```json
{
  "type": "message",
  "id": "ctm_01234",
  "input_tokens": 20,
  "cache_creation_input_tokens": 0,
  "cache_read_input_tokens": 0
}
```

---

## OpenAI Chat Completions

```
POST /v1/chat/completions
POST /chat/completions
```

**curl 示例：**

```bash
curl https://openclaw.zhiguo.fan/v1/chat/completions \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4.1",
    "messages": [
      { "role": "user", "content": "你好！" }
    ],
    "max_tokens": 1024
  }'
```

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `model` | string | ✅ | 模型 ID，见[支持的模型](#openai-模型) |
| `messages` | array | ✅ | 支持 `system` / `user` / `assistant` 角色 |
| `max_tokens` | integer | ❌ | 最大输出 token 数 |
| `temperature` | float | ❌ | 温度，默认 1.0 |
| `top_p` | float | ❌ | Top-p 采样 |
| `stream` | boolean | ❌ | 是否流式响应，默认 false |
| `n` | integer | ❌ | 生成候选数，默认 1 |

**响应示例：**

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "model": "gpt-4.1",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "你好！有什么我可以帮助你的？"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 14,
    "total_tokens": 24
  }
}
```

**带 System 角色的多轮对话：**

```bash
curl https://openclaw.zhiguo.fan/v1/chat/completions \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [
      { "role": "system", "content": "你是一个专业的 Python 编程助手。" },
      { "role": "user", "content": "如何用列表推导式生成 1 到 10 的平方列表？" }
    ]
  }'
```

**流式调用：**

```bash
curl https://openclaw.zhiguo.fan/v1/chat/completions \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4.1-mini",
    "stream": true,
    "messages": [
      { "role": "user", "content": "讲一个简短的笑话。" }
    ]
  }'
```

---

## OpenAI 推理模型（o 系列）

o 系列为推理增强模型，接口与 Chat Completions 完全相同，适合复杂逻辑、数学、代码任务。

```bash
curl https://openclaw.zhiguo.fan/v1/chat/completions \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "o3-mini",
    "messages": [
      { "role": "user", "content": "证明根号2是无理数。" }
    ]
  }'
```

```bash
curl https://openclaw.zhiguo.fan/v1/chat/completions \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "o4-mini",
    "messages": [
      { "role": "user", "content": "用 Python 实现一个 LRU Cache。" }
    ]
  }'
```

> o 系列模型内部自动进行多步推理，响应时间比普通模型长，请适当增加超时时间。

---

## 图像生成

```
POST /v1/images/generations
POST /images/generations
```

仅支持 OpenAI 平台分组。返回格式为 **base64 编码的图片数据**（`b64_json` 字段）。

**gpt-image-2 生成图片：**

```bash
curl https://openclaw.zhiguo.fan/v1/images/generations \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一片宁静的山间湖泊，夕阳西下，金色光芒映照在水面上",
    "n": 1,
    "size": "1024x1024"
  }'
```

**生成并保存为图片文件（bash + python3）：**

```bash
curl -s https://openclaw.zhiguo.fan/v1/images/generations \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一只可爱的橘猫坐在窗台上看雨",
    "n": 1,
    "size": "1024x1024"
  }' | python3 -c "
import sys, json, base64
data = json.load(sys.stdin)
img = base64.b64decode(data['data'][0]['b64_json'])
with open('output.png', 'wb') as f:
    f.write(img)
print(f'已保存 output.png ({len(img)//1024} KB)')
"
```

**gpt-image-1（经典版）：**

```bash
curl https://openclaw.zhiguo.fan/v1/images/generations \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-1",
    "prompt": "未来城市的夜景，霓虹灯与飞行汽车",
    "n": 1,
    "size": "1024x1024"
  }'
```

**请求参数：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `model` | string | ✅ | `gpt-image-1` / `gpt-image-1.5` / `gpt-image-2` |
| `prompt` | string | ✅ | 图片描述，支持中英文 |
| `n` | integer | ❌ | 生成张数，默认 1 |
| `size` | string | ❌ | `1024x1024` / `1024x1792` / `1792x1024` |

**响应结构：**

```json
{
  "created": 1777282564,
  "data": [
    {
      "b64_json": "iVBORw0KGgoAAAANS..."
    }
  ],
  "usage": {
    "input_tokens": 89,
    "output_tokens": 1756,
    "total_tokens": 1845
  }
}
```

---

## Gemini API 兼容

Gemini 模型通过 `/antigravity/v1beta` 路由访问，使用普通 API Key 即可调用。

### 生成内容

```
POST /antigravity/v1beta/models/{model}:generateContent
POST /antigravity/v1beta/models/{model}:streamGenerateContent
```

```bash
curl https://openclaw.zhiguo.fan/antigravity/v1beta/models/gemini-2.5-flash:generateContent \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [{ "text": "为什么天空是蓝色的？" }]
      }
    ],
    "generationConfig": {
      "temperature": 0.8,
      "maxOutputTokens": 1024
    }
  }'
```

**多轮对话：**

```bash
curl https://openclaw.zhiguo.fan/antigravity/v1beta/models/gemini-3.1-pro-high:generateContent \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      { "role": "user", "parts": [{ "text": "介绍一下机器学习" }] },
      { "role": "model", "parts": [{ "text": "机器学习是一种 AI 技术..." }] },
      { "role": "user", "parts": [{ "text": "能举个实际应用的例子吗？" }] }
    ]
  }'
```

**流式调用：**

```bash
curl https://openclaw.zhiguo.fan/antigravity/v1beta/models/gemini-3-flash:streamGenerateContent \
  -H "Authorization: Bearer sk-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      { "role": "user", "parts": [{ "text": "写一首关于春天的短诗。" }] }
    ]
  }'
```

**响应示例：**

```json
{
  "candidates": [
    {
      "content": {
        "parts": [{ "text": "天空呈蓝色是由于瑞利散射..." }],
        "role": "model"
      },
      "finishReason": "STOP"
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 8,
    "candidatesTokenCount": 42,
    "totalTokenCount": 50
  }
}
```

### 获取模型列表

```bash
curl https://openclaw.zhiguo.fan/antigravity/v1beta/models \
  -H "Authorization: Bearer sk-your-api-key"
```

---

## 通用端点

### 查询可用模型列表

```
GET /v1/models
```

```bash
curl https://openclaw.zhiguo.fan/v1/models \
  -H "Authorization: Bearer sk-your-api-key"
```

**响应：**

```json
{
  "object": "list",
  "data": [
    { "id": "gpt-4.1", "object": "model", "owned_by": "openai" },
    { "id": "gpt-4o", "object": "model", "owned_by": "openai" },
    { "id": "o3-mini", "object": "model", "owned_by": "openai" }
  ]
}
```

### 查询 API Key 用量

```
GET /v1/usage
```

```bash
curl https://openclaw.zhiguo.fan/v1/usage \
  -H "Authorization: Bearer sk-your-api-key"
```

**响应：**

```json
{
  "quota": 10.0,
  "used": 1.23,
  "remaining": 8.77,
  "concurrency_limit": 5,
  "concurrency_used": 1
}
```

---

## 流式响应

在 Claude / OpenAI 请求体中加 `"stream": true`，Gemini 使用 `:streamGenerateContent` 端点。

**Claude SSE 格式：**

```
event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"你好"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"！"}}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}
```

**OpenAI SSE 格式：**

```
data: {"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"delta":{"content":"你好"},"index":0}]}

data: {"id":"chatcmpl-123","object":"chat.completion.chunk","choices":[{"delta":{"content":"！"},"index":0}]}

data: [DONE]
```

---

## 支持的模型

### Claude 模型

| 模型 ID | 说明 |
|---------|------|
| `claude-opus-4-7` | 最新旗舰版，最强推理能力 |
| `claude-opus-4-5` | Opus 4 稳定版 |
| `claude-sonnet-4-5` | 均衡性能，推荐日常使用 |
| `claude-haiku-4-5` | 轻量快速版 |
| `claude-3-5-sonnet-20241022` | Claude 3.5 Sonnet |
| `claude-3-5-haiku-20241022` | Claude 3.5 Haiku |
| `claude-3-opus-20240229` | Claude 3 Opus |
| `claude-3-haiku-20240307` | Claude 3 Haiku，速度最快、成本最低 |

### OpenAI 模型

| 模型 ID | 说明 |
|---------|------|
| `gpt-4.1` | GPT-4.1 旗舰版 |
| `gpt-4.1-mini` | GPT-4.1 轻量版 |
| `gpt-4.1-nano` | GPT-4.1 极速版 |
| `gpt-4o` | GPT-4o 多模态版 |
| `gpt-4o-mini` | GPT-4o 轻量版 |
| `gpt-4o-2024-11-20` | GPT-4o 指定版本 |
| `gpt-4-turbo` | GPT-4 Turbo |
| `gpt-4` | GPT-4 标准版 |
| `gpt-3.5-turbo` | 经济快速版 |
| `chatgpt-4o-latest` | ChatGPT 最新 4o 版 |
| `gpt-5.2` | GPT-5.2 |
| `gpt-5.2-pro` | GPT-5.2 Pro |
| `gpt-5.3-codex` | GPT-5.3 代码专项版 |
| `gpt-5.4` | GPT-5.4 |
| `gpt-5.4-mini` | GPT-5.4 轻量版 |
| `gpt-5.5` | GPT-5.5 |

### OpenAI 推理模型（o 系列）

| 模型 ID | 说明 |
|---------|------|
| `o4-mini` | 最新推理模型，轻量 |
| `o3` | o3 旗舰推理版 |
| `o3-mini` | o3 轻量推理版 |
| `o3-pro` | o3 专业版 |
| `o1` | o1 推理版 |
| `o1-mini` | o1 轻量版 |
| `o1-pro` | o1 专业版 |
| `o1-preview` | o1 预览版 |

### 图像生成模型

| 模型 ID | 说明 |
|---------|------|
| `gpt-image-2` | 最新图像生成模型（推荐） |
| `gpt-image-1.5` | 图像生成中间版本 |
| `gpt-image-1` | 图像生成经典版 |

### Gemini 模型（通过 `/antigravity/v1beta` 路由调用）

| 模型 ID | 说明 |
|---------|------|
| `gemini-2.5-flash` | Gemini 2.5 Flash，快速均衡 |
| `gemini-2.5-flash-lite` | Gemini 2.5 Flash 极速轻量版 |
| `gemini-2.5-flash-thinking` | Gemini 2.5 Flash 推理增强版 |
| `gemini-2.5-flash-image` | Gemini 2.5 Flash 图像理解版 |
| `gemini-2.5-flash-image-preview` | Gemini 2.5 Flash 图像预览版 |
| `gemini-3-flash` | Gemini 3 Flash |
| `gemini-3-pro-low` | Gemini 3 Pro 低配版 |
| `gemini-3-pro-high` | Gemini 3 Pro 高配版 |
| `gemini-3-pro-preview` | Gemini 3 Pro 预览版 |
| `gemini-3-pro-image` | Gemini 3 Pro 图像版 |
| `gemini-3.1-pro-low` | Gemini 3.1 Pro 低配版 |
| `gemini-3.1-pro-high` | Gemini 3.1 Pro 高配版（推荐） |
| `gemini-3.1-flash-image` | Gemini 3.1 Flash 图像版 |
| `gemini-3.1-flash-image-preview` | Gemini 3.1 Flash 图像预览版 |

> 模型 ID 来源于 `GET /antigravity/v1beta/models` 真实接口。Gemini 调用依赖服务端 Gemini 账号资源，若返回 `No available Gemini accounts` 表示当前账号资源暂时耗尽，请稍后重试或联系管理员。

---

## 错误处理

### Claude / Anthropic 格式

```json
{
  "type": "error",
  "error": {
    "type": "authentication_error",
    "message": "Invalid API key"
  }
}
```

### OpenAI 格式

```json
{
  "error": {
    "message": "Rate limit exceeded",
    "type": "rate_limit_error",
    "param": null,
    "code": "rate_limit_exceeded"
  }
}
```

### 错误码一览

| HTTP 状态码 | 错误类型 | 原因 |
|-------------|----------|------|
| `400` | `invalid_request_error` | 请求参数无效（缺少必填字段、格式错误等） |
| `401` | `authentication_error` | API Key 无效或已过期 |
| `401` | `API_KEY_REQUIRED` | 请求未携带 API Key |
| `403` | `permission_error` | 该 Key 无权访问此模型或功能 |
| `429` | `rate_limit_error` | 超过速率限制（RPM/TPM） |
| `500` | `api_error` | 服务端内部错误 |
| `503` | `unavailable_error` | 上游服务暂时不可用，请稍后重试 |
