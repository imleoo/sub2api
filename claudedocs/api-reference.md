# We2AI API 接口文档

> Base URL: `https://api.we2ai.com`
>
> 本文档说明如何通过 API 调用各类 AI 模型，包括 Claude、Gemini、GPT 文本模型及图像生成模型。

---

## 目录

1. [认证方式](#1-认证方式)
2. [Claude 模型](#2-claude-模型)
3. [GPT 模型](#3-gpt-模型)
4. [GPT 图像生成](#4-gpt-图像生成)
5. [Gemini 模型](#5-gemini-模型)
6. [Gemini 图像生成](#6-gemini-图像生成)
7. [通用说明](#7-通用说明)
8. [错误处理](#8-错误处理)

---

## 1. 认证方式

所有 API 请求均需在 HTTP Header 中携带 API Key。

### 推荐方式（Bearer Token）

```http
Authorization: Bearer sk-your-api-key
```

### 备用方式

```http
x-api-key: sk-your-api-key
```

### Gemini SDK 兼容方式

```http
x-goog-api-key: your-api-key
```

> **注意**：API Key 在后台管理面板中创建，请勿在公开代码中明文保存。

---

## 2. Claude 模型

使用 Anthropic 原生 Messages API 格式调用 Claude 系列模型。

### 端点

```
POST https://api.we2ai.com/v1/messages
```

### 支持的模型

| 模型名称 | 说明 |
|----------|------|
| `claude-opus-4-5` | Claude Opus 4.5（最强能力） |
| `claude-opus-4-6` | Claude Opus 4.6 |
| `claude-opus-4-7` | Claude Opus 4.7 |
| `claude-sonnet-4` | Claude Sonnet 4（推荐，性价比高） |
| `claude-3-5-sonnet-20241022` | Claude 3.5 Sonnet |
| `claude-3-5-haiku-20241022` | Claude 3.5 Haiku（速度快） |
| `claude-3-opus-20240229` | Claude 3 Opus |
| `claude-3-haiku-20240307` | Claude 3 Haiku |

### 请求示例

#### 基础对话

```bash
curl https://api.we2ai.com/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: sk-your-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4",
    "max_tokens": 1024,
    "messages": [
      {
        "role": "user",
        "content": "你好，请介绍一下自己。"
      }
    ]
  }'
```

#### 多轮对话

```bash
curl https://api.we2ai.com/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: sk-your-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4",
    "max_tokens": 2048,
    "system": "你是一个专业的代码审查助手，请用简洁的语言给出建议。",
    "messages": [
      {
        "role": "user",
        "content": "请帮我审查这段 Python 代码：\ndef add(a, b):\n  return a+b"
      },
      {
        "role": "assistant",
        "content": "这段代码功能正确，但建议添加类型注解..."
      },
      {
        "role": "user",
        "content": "如何添加单元测试？"
      }
    ]
  }'
```

#### 流式输出（推荐）

```bash
curl https://api.we2ai.com/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: sk-your-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4",
    "max_tokens": 1024,
    "stream": true,
    "messages": [
      {
        "role": "user",
        "content": "写一首关于春天的诗。"
      }
    ]
  }'
```

流式响应为 Server-Sent Events 格式：

```
event: message_start
data: {"type":"message_start","message":{"id":"msg_xxx","type":"message","role":"assistant","content":[],"model":"claude-sonnet-4","stop_reason":null}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"春风"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"轻抚柳梢..."}}

event: message_stop
data: {"type":"message_stop"}
```

#### 带图片的多模态请求

```bash
curl https://api.we2ai.com/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: sk-your-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4",
    "max_tokens": 1024,
    "messages": [
      {
        "role": "user",
        "content": [
          {
            "type": "image",
            "source": {
              "type": "base64",
              "media_type": "image/jpeg",
              "data": "/9j/4AAQ..."
            }
          },
          {
            "type": "text",
            "text": "请描述这张图片的内容。"
          }
        ]
      }
    ]
  }'
```

### 响应格式

```json
{
  "id": "msg_01XFDUDYJgAACzvnptvVoYEL",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "你好！我是 Claude，由 Anthropic 开发的 AI 助手..."
    }
  ],
  "model": "claude-sonnet-4",
  "stop_reason": "end_turn",
  "stop_sequence": null,
  "usage": {
    "input_tokens": 12,
    "output_tokens": 48
  }
}
```

### Token 计数

在不实际发送消息的情况下预估 Token 消耗：

```bash
curl https://api.we2ai.com/v1/messages/count_tokens \
  -H "Content-Type: application/json" \
  -H "x-api-key: sk-your-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4",
    "messages": [
      {
        "role": "user",
        "content": "你好！"
      }
    ]
  }'
```

响应：`{"input_tokens": 8}`

### Python SDK 示例

```python
import anthropic

client = anthropic.Anthropic(
    api_key="sk-your-api-key",
    base_url="https://api.we2ai.com"
)

message = client.messages.create(
    model="claude-sonnet-4",
    max_tokens=1024,
    messages=[
        {"role": "user", "content": "你好，Claude！"}
    ]
)
print(message.content[0].text)
```

### Node.js SDK 示例

```javascript
import Anthropic from "@anthropic-ai/sdk";

const client = new Anthropic({
  apiKey: "sk-your-api-key",
  baseURL: "https://api.we2ai.com",
});

const message = await client.messages.create({
  model: "claude-sonnet-4",
  max_tokens: 1024,
  messages: [{ role: "user", content: "你好，Claude！" }],
});

console.log(message.content[0].text);
```

---

## 3. GPT 模型

使用 OpenAI 兼容格式调用 GPT 系列模型。

### 端点

```
POST https://api.we2ai.com/v1/chat/completions
```

### 支持的模型

| 模型名称 | 说明 |
|----------|------|
| `gpt-5.4` | GPT 5.4（旗舰版） |
| `gpt-5.5` | GPT 5.5 |
| `gpt-5.4-mini` | GPT 5.4 Mini（快速版） |
| `gpt-5.2` | GPT 5.2 |
| `gpt-5.3-codex` | GPT Codex（代码专用） |
| `gpt-5.3-codex-spark` | GPT Codex Spark（轻量版，不支持图像） |

### 请求示例

#### 基础对话

```bash
curl https://api.we2ai.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-api-key" \
  -d '{
    "model": "gpt-5.4",
    "messages": [
      {
        "role": "system",
        "content": "你是一个有帮助的助手。"
      },
      {
        "role": "user",
        "content": "请用 Python 写一个快速排序算法。"
      }
    ]
  }'
```

#### 流式输出

```bash
curl https://api.we2ai.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-api-key" \
  -d '{
    "model": "gpt-5.4",
    "stream": true,
    "messages": [
      {
        "role": "user",
        "content": "请解释量子纠缠的原理。"
      }
    ]
  }'
```

流式响应格式（SSE）：

```
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":"量子"},"index":0}]}
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","choices":[{"delta":{"content":"纠缠"},"index":0}]}
data: [DONE]
```

#### 带参数的请求

```bash
curl https://api.we2ai.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-api-key" \
  -d '{
    "model": "gpt-5.4",
    "temperature": 0.7,
    "max_tokens": 2048,
    "top_p": 1,
    "messages": [
      {
        "role": "user",
        "content": "写一篇500字的产品发布公告。"
      }
    ]
  }'
```

### 响应格式

```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1677858242,
  "model": "gpt-5.4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "这是一个快速排序的 Python 实现：..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 28,
    "completion_tokens": 256,
    "total_tokens": 284
  }
}
```

### Python SDK 示例

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-your-api-key",
    base_url="https://api.we2ai.com/v1"
)

response = client.chat.completions.create(
    model="gpt-5.4",
    messages=[
        {"role": "system", "content": "你是一个有帮助的助手。"},
        {"role": "user", "content": "你好！"}
    ]
)
print(response.choices[0].message.content)
```

### Node.js SDK 示例

```javascript
import OpenAI from "openai";

const client = new OpenAI({
  apiKey: "sk-your-api-key",
  baseURL: "https://api.we2ai.com/v1",
});

const response = await client.chat.completions.create({
  model: "gpt-5.4",
  messages: [{ role: "user", content: "你好！" }],
});

console.log(response.choices[0].message.content);
```

### OpenAI Responses API（高级）

支持 OpenAI 最新的 Responses API，可用于 Codex 等场景：

```
POST https://api.we2ai.com/v1/responses
```

---

## 4. GPT 图像生成

### 端点

```
POST https://api.we2ai.com/v1/images/generations   # 文生图
POST https://api.we2ai.com/v1/images/edits         # 图片编辑
```

> **注意**：图像生成 API 仅对 OpenAI 平台分组的 API Key 可用。

### 支持的模型

| 模型名称 | 说明 |
|----------|------|
| `gpt-image-2` | GPT Image 2（推荐，最新图像模型） |
| `dall-e-3` | DALL-E 3 |
| `dall-e-2` | DALL-E 2 |

### 4.1 文生图

#### 请求示例

```bash
curl https://api.we2ai.com/v1/images/generations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-api-key" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一只可爱的橘猫坐在樱花树下，日式水彩画风格",
    "n": 1,
    "size": "1024x1024"
  }'
```

#### 请求参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `model` | string | 模型名称 |
| `prompt` | string | 图像描述（英文效果更好） |
| `n` | integer | 生成数量（1-4） |
| `size` | string | 尺寸：`256x256` / `512x512` / `1024x1024` / `1792x1024` / `1024x1792` |
| `quality` | string | 质量：`standard` / `hd`（仅 dall-e-3） |
| `style` | string | 风格：`vivid` / `natural`（仅 dall-e-3） |
| `response_format` | string | `url`（默认）或 `b64_json` |

#### 响应格式

```json
{
  "created": 1589478378,
  "data": [
    {
      "url": "https://..."
    }
  ]
}
```

### 4.2 图片编辑（局部重绘）

```bash
curl https://api.we2ai.com/v1/images/edits \
  -H "Authorization: Bearer sk-your-api-key" \
  -F "model=gpt-image-2" \
  -F "image=@original.png" \
  -F "mask=@mask.png" \
  -F "prompt=将猫的颜色改为白色" \
  -F "n=1" \
  -F "size=1024x1024"
```

> 图片编辑使用 `multipart/form-data` 格式，需要提供原图和蒙版图（透明区域为待编辑区域）。

### Python SDK 示例

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-your-api-key",
    base_url="https://api.we2ai.com/v1"
)

# 文生图
response = client.images.generate(
    model="gpt-image-2",
    prompt="一只可爱的橘猫坐在樱花树下，日式水彩画风格",
    n=1,
    size="1024x1024"
)
print(response.data[0].url)

# 图片编辑
with open("original.png", "rb") as image_file:
    with open("mask.png", "rb") as mask_file:
        response = client.images.edit(
            model="gpt-image-2",
            image=image_file,
            mask=mask_file,
            prompt="将背景改为夜晚星空",
            n=1,
            size="1024x1024"
        )
print(response.data[0].url)
```

---

## 5. Gemini 模型

支持两种调用方式：

- **方式一**：通过 Gemini 原生 `/v1beta` API（完全兼容 Google AI SDK）
- **方式二**：通过 OpenAI 兼容格式 `/v1/chat/completions`（需要 Gemini 分组 API Key）

### 端点（原生 Gemini API）

```
POST https://api.we2ai.com/v1beta/models/{model}:generateContent
POST https://api.we2ai.com/v1beta/models/{model}:streamGenerateContent
GET  https://api.we2ai.com/v1beta/models
GET  https://api.we2ai.com/v1beta/models/{model}
```

### 支持的模型

| 模型名称 | 说明 |
|----------|------|
| `gemini-2.5-pro` | Gemini 2.5 Pro（旗舰版） |
| `gemini-2.5-flash` | Gemini 2.5 Flash（快速版） |
| `gemini-3.1-pro` | Gemini 3.1 Pro |
| `gemini-3.1-flash` | Gemini 3.1 Flash |

### 5.1 原生 Gemini API 格式

#### 基础对话

```bash
curl "https://api.we2ai.com/v1beta/models/gemini-2.5-pro:generateContent" \
  -H "Content-Type: application/json" \
  -H "x-goog-api-key: your-api-key" \
  -d '{
    "contents": [
      {
        "parts": [
          {
            "text": "请介绍一下机器学习的基本概念。"
          }
        ]
      }
    ]
  }'
```

#### 多轮对话

```bash
curl "https://api.we2ai.com/v1beta/models/gemini-2.5-pro:generateContent" \
  -H "Content-Type: application/json" \
  -H "x-goog-api-key: your-api-key" \
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [{"text": "什么是神经网络？"}]
      },
      {
        "role": "model",
        "parts": [{"text": "神经网络是一种受人类大脑启发的机器学习模型..."}]
      },
      {
        "role": "user",
        "parts": [{"text": "它和深度学习有什么关系？"}]
      }
    ]
  }'
```

#### 流式输出

```bash
curl "https://api.we2ai.com/v1beta/models/gemini-2.5-pro:streamGenerateContent" \
  -H "Content-Type: application/json" \
  -H "x-goog-api-key: your-api-key" \
  -d '{
    "contents": [
      {
        "parts": [{"text": "写一篇关于人工智能未来的短文。"}]
      }
    ]
  }'
```

#### 带图片的多模态请求

```bash
curl "https://api.we2ai.com/v1beta/models/gemini-2.5-pro:generateContent" \
  -H "Content-Type: application/json" \
  -H "x-goog-api-key: your-api-key" \
  -d '{
    "contents": [
      {
        "parts": [
          {
            "inline_data": {
              "mime_type": "image/jpeg",
              "data": "/9j/4AAQ..."
            }
          },
          {
            "text": "请描述这张图片中的内容。"
          }
        ]
      }
    ]
  }'
```

#### 系统指令

```bash
curl "https://api.we2ai.com/v1beta/models/gemini-2.5-pro:generateContent" \
  -H "Content-Type: application/json" \
  -H "x-goog-api-key: your-api-key" \
  -d '{
    "system_instruction": {
      "parts": [{"text": "你是一个专业的医学助手，请用通俗易懂的语言解释医学问题。"}]
    },
    "contents": [
      {
        "parts": [{"text": "什么是高血压？"}]
      }
    ],
    "generationConfig": {
      "temperature": 0.4,
      "maxOutputTokens": 1024
    }
  }'
```

### 响应格式

```json
{
  "candidates": [
    {
      "content": {
        "parts": [
          {
            "text": "机器学习是人工智能的一个子领域..."
          }
        ],
        "role": "model"
      },
      "finishReason": "STOP",
      "index": 0
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 12,
    "candidatesTokenCount": 256,
    "totalTokenCount": 268
  }
}
```

### 5.2 OpenAI 兼容格式（适合已有 OpenAI 代码迁移）

使用 Gemini 分组的 API Key，通过 `/v1/chat/completions` 调用 Gemini 模型：

```bash
curl https://api.we2ai.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk-your-gemini-group-api-key" \
  -d '{
    "model": "gemini-2.5-pro",
    "messages": [
      {
        "role": "user",
        "content": "你好！"
      }
    ]
  }'
```

### Python SDK 示例（原生 Google SDK）

```python
import google.generativeai as genai

genai.configure(
    api_key="your-api-key",
    transport="rest",
    client_options={"api_endpoint": "https://api.we2ai.com"}
)

model = genai.GenerativeModel("gemini-2.5-pro")
response = model.generate_content("请介绍一下机器学习的基本概念。")
print(response.text)
```

### Python 示例（使用 requests 库）

```python
import requests

API_KEY = "your-api-key"
BASE_URL = "https://api.we2ai.com"

def chat(model: str, prompt: str) -> str:
    url = f"{BASE_URL}/v1beta/models/{model}:generateContent"
    headers = {
        "Content-Type": "application/json",
        "x-goog-api-key": API_KEY,
    }
    payload = {
        "contents": [
            {"parts": [{"text": prompt}]}
        ]
    }
    response = requests.post(url, json=payload, headers=headers)
    response.raise_for_status()
    return response.json()["candidates"][0]["content"]["parts"][0]["text"]

print(chat("gemini-2.5-pro", "你好！"))
```

### 列出可用模型

```bash
curl "https://api.we2ai.com/v1beta/models" \
  -H "x-goog-api-key: your-api-key"
```

---

## 6. Gemini 图像生成

Gemini 图像生成模型使用 Anthropic Messages API 格式（Claude 接口规范），通过 Antigravity 路由调用。

### 端点

```
POST https://api.we2ai.com/v1/messages
```

### 支持的图像模型

| 模型名称 | 说明 |
|----------|------|
| `gemini-2.5-flash-image` | Gemini 2.5 Flash 图像生成 |
| `gemini-2.5-flash-image-preview` | Gemini 2.5 Flash 图像生成（预览版） |
| `gemini-3.1-flash-image` | Gemini 3.1 Flash 图像生成 |
| `gemini-3.1-flash-image-preview` | Gemini 3.1 Flash 图像生成（预览版） |
| `gemini-3-pro-image` | Gemini 3 Pro 图像生成 |
| `gemini-3-pro-image-preview` | Gemini 3 Pro 图像生成（预览版） |

### 请求示例

#### 文生图

```bash
curl https://api.we2ai.com/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: sk-your-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gemini-2.5-flash-image",
    "max_tokens": 1024,
    "messages": [
      {
        "role": "user",
        "content": "请生成一张图片：一只可爱的小猫在樱花树下玩耍，日系动漫风格，色彩明亮。"
      }
    ]
  }'
```

#### 图文混合（理解图片后生图）

```bash
curl https://api.we2ai.com/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: sk-your-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gemini-3.1-flash-image",
    "max_tokens": 2048,
    "messages": [
      {
        "role": "user",
        "content": [
          {
            "type": "image",
            "source": {
              "type": "base64",
              "media_type": "image/jpeg",
              "data": "/9j/4AAQ..."
            }
          },
          {
            "type": "text",
            "text": "请参考这张图片的风格，生成一张类似的夜景图。"
          }
        ]
      }
    ]
  }'
```

### 响应格式

图像生成模型返回包含图片的 content 块：

```json
{
  "id": "msg_xxx",
  "type": "message",
  "role": "assistant",
  "content": [
    {
      "type": "text",
      "text": "我已为您生成了一张图片："
    },
    {
      "type": "image",
      "source": {
        "type": "base64",
        "media_type": "image/png",
        "data": "iVBORw0KGgo..."
      }
    }
  ],
  "model": "gemini-2.5-flash-image",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 24,
    "output_tokens": 512
  }
}
```

### Python 示例

```python
import anthropic
import base64
from pathlib import Path

client = anthropic.Anthropic(
    api_key="sk-your-api-key",
    base_url="https://api.we2ai.com"
)

# 文生图
message = client.messages.create(
    model="gemini-2.5-flash-image",
    max_tokens=1024,
    messages=[
        {
            "role": "user",
            "content": "生成一张赛博朋克风格的城市夜景图，霓虹灯光，雨夜，高清写实。"
        }
    ]
)

# 提取并保存图片
for block in message.content:
    if block.type == "image":
        image_data = base64.b64decode(block.source.data)
        Path("output.png").write_bytes(image_data)
        print("图片已保存到 output.png")
    elif block.type == "text":
        print(block.text)
```

---

## 7. 通用说明

### 7.1 查询可用模型

```bash
# Claude/GPT 分组
curl https://api.we2ai.com/v1/models \
  -H "x-api-key: sk-your-api-key"

# Gemini 分组
curl "https://api.we2ai.com/v1beta/models" \
  -H "x-goog-api-key: your-api-key"
```

### 7.2 查询配额使用情况

```bash
curl https://api.we2ai.com/v1/usage \
  -H "x-api-key: sk-your-api-key"
```

响应示例：

```json
{
  "quota_used": 1234567,
  "quota_total": 10000000,
  "quota_remaining": 8765433,
  "reset_at": "2026-05-01T00:00:00Z"
}
```

### 7.3 请求头说明

| Header | 必填 | 说明 |
|--------|------|------|
| `Authorization: Bearer {key}` | 是（三选一） | Bearer Token 认证 |
| `x-api-key: {key}` | 是（三选一） | Key 直接传入 |
| `x-goog-api-key: {key}` | 是（三选一） | Gemini SDK 兼容 |
| `Content-Type: application/json` | 是 | 请求格式声明 |
| `anthropic-version: 2023-06-01` | 推荐 | Claude API 版本声明 |
| `x-request-id` | 否 | 自定义请求 ID，用于追踪 |

### 7.4 API Key 分组说明

不同的 API Key 绑定到不同分组，分组决定可调用的模型类型：

| 分组平台 | 可用接口 |
|----------|----------|
| `anthropic` | `/v1/messages`（Claude 模型） |
| `openai` | `/v1/chat/completions`, `/v1/responses`, `/v1/images/*`（GPT 模型） |
| `gemini` | `/v1beta/models/*`（Gemini 原生格式） |

如需同时使用多种模型，需要申请对应分组的 API Key。

### 7.5 流式输出最佳实践

- 生产环境建议始终使用流式输出（`"stream": true`），可显著降低用户感知延迟
- 流式响应超时默认较长（通常 5-10 分钟），适合长文本生成
- 网络不稳定时，流式输出中断后不支持断点续传，需重新发起请求

---

## 8. 错误处理

### 8.1 HTTP 状态码

| 状态码 | 含义 |
|--------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未认证（API Key 无效或缺失） |
| 403 | 无权限（Key 所在分组不支持该 API） |
| 429 | 请求频率超限（RPM 超出） |
| 503 | 服务暂时不可用（上游账号池耗尽） |
| 529 | 系统过载（稍后重试） |

### 8.2 Claude 错误格式

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "max_tokens: 1000000 > maximum allowed value of 64000"
  }
}
```

### 8.3 OpenAI 错误格式

```json
{
  "error": {
    "type": "authentication_error",
    "code": "invalid_api_key",
    "message": "Invalid API key provided."
  }
}
```

### 8.4 Google 错误格式

```json
{
  "error": {
    "code": 401,
    "message": "Invalid API key.",
    "status": "UNAUTHENTICATED"
  }
}
```

### 8.5 限流处理建议

遇到 `429` 错误时，响应头中包含 `Retry-After` 字段：

```python
import time
import requests

def chat_with_retry(payload, max_retries=3):
    for attempt in range(max_retries):
        resp = requests.post(
            "https://api.we2ai.com/v1/messages",
            json=payload,
            headers={"x-api-key": "sk-your-api-key"}
        )
        if resp.status_code == 429:
            retry_after = int(resp.headers.get("Retry-After", 5))
            print(f"限流，{retry_after}秒后重试...")
            time.sleep(retry_after)
            continue
        resp.raise_for_status()
        return resp.json()
    raise Exception("超过最大重试次数")
```

---

## 快速上手示例

### 30 秒接入 Claude

```python
import anthropic

client = anthropic.Anthropic(
    api_key="sk-your-api-key",        # 替换为你的 API Key
    base_url="https://api.we2ai.com"   # 指向 We2AI 网关
)

# 发送请求
message = client.messages.create(
    model="claude-sonnet-4",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Hello!"}]
)
print(message.content[0].text)
```

### 30 秒接入 GPT

```python
from openai import OpenAI

client = OpenAI(
    api_key="sk-your-api-key",           # 替换为你的 API Key
    base_url="https://api.we2ai.com/v1"  # 指向 We2AI 网关
)

response = client.chat.completions.create(
    model="gpt-5.4",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)
```

### 30 秒接入 Gemini

```python
import requests

response = requests.post(
    "https://api.we2ai.com/v1beta/models/gemini-2.5-pro:generateContent",
    headers={"x-goog-api-key": "your-api-key"},
    json={"contents": [{"parts": [{"text": "Hello!"}]}]}
)
print(response.json()["candidates"][0]["content"]["parts"][0]["text"])
```
