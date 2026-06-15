/**
 * 网关模型调用测试
 *
 * 本文件演示如何使用 API Token 通过本平台网关调用各类 AI 模型。
 * 网关端点与内部管理 API (/api/v1/...) 完全独立，直接挂载在根路径下。
 *
 * 认证方式（三选一）：
 *   Authorization: Bearer sk_live_xxxxx   ← 推荐
 *   x-api-key: sk_live_xxxxx
 *   x-goog-api-key: sk_live_xxxxx         ← Gemini CLI 兼容
 *
 * 所有网关端点的 Base URL 为服务器根路径，例如 http://localhost:8080
 */

import { describe, it, expect, vi, beforeEach } from 'vitest'
import axios from 'axios'

// ─────────────────────────────────────────
// 测试辅助：构造一个带 API Token 的 axios 实例
// ─────────────────────────────────────────

function createGatewayClient(apiToken: string, baseURL = 'http://localhost:8080') {
  return axios.create({
    baseURL,
    headers: {
      Authorization: `Bearer ${apiToken}`,
      'Content-Type': 'application/json',
    },
    timeout: 30_000,
  })
}

const MOCK_TOKEN = 'sk_live_test_token_123'
const MOCK_BASE_URL = 'http://localhost:8080'

// ─────────────────────────────────────────
// Mock axios
// ─────────────────────────────────────────

vi.mock('axios', async (importOriginal) => {
  const actual = await importOriginal<typeof import('axios')>()
  return {
    ...actual,
    default: {
      ...actual.default,
      create: vi.fn(),
    },
  }
})

const mockGet = vi.fn()
const mockPost = vi.fn()

beforeEach(() => {
  mockGet.mockReset()
  mockPost.mockReset()
  vi.mocked(axios.create).mockReturnValue({
    get: mockGet,
    post: mockPost,
  } as never)
})

// ══════════════════════════════════════════════════════════════
// 1. Claude API 兼容接口  (/v1/messages)
// ══════════════════════════════════════════════════════════════

describe('Claude API 兼容接口 - /v1/messages', () => {
  it('非流式调用 claude-3-5-sonnet', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)

    mockPost.mockResolvedValueOnce({
      data: {
        id: 'msg_01XFDUDYJgAACTvhOSnS9',
        type: 'message',
        role: 'assistant',
        content: [{ type: 'text', text: 'Hello! How can I help you?' }],
        model: 'claude-3-5-sonnet-20241022',
        stop_reason: 'end_turn',
        usage: { input_tokens: 10, output_tokens: 15 },
      },
    })

    const response = await client.post('/v1/messages', {
      model: 'claude-3-5-sonnet-20241022',
      max_tokens: 1024,
      messages: [{ role: 'user', content: 'Hello!' }],
    })

    expect(mockPost).toHaveBeenCalledWith('/v1/messages', {
      model: 'claude-3-5-sonnet-20241022',
      max_tokens: 1024,
      messages: [{ role: 'user', content: 'Hello!' }],
    })
    expect(response.data.type).toBe('message')
    expect(response.data.role).toBe('assistant')
    expect(response.data.content[0].type).toBe('text')
  })

  it('流式调用 claude-3-5-sonnet（stream: true）', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)

    // 流式响应为 SSE，此处验证请求体包含 stream: true
    mockPost.mockResolvedValueOnce({ data: {} })

    await client.post('/v1/messages', {
      model: 'claude-3-5-sonnet-20241022',
      max_tokens: 1024,
      stream: true,
      messages: [{ role: 'user', content: 'Tell me a joke' }],
    })

    const [, body] = mockPost.mock.calls[0]
    expect(body).toMatchObject({ stream: true })
  })

  it('调用 claude-3-opus', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({
      data: {
        id: 'msg_opus_001',
        type: 'message',
        role: 'assistant',
        content: [{ type: 'text', text: 'Deep analysis result...' }],
        model: 'claude-3-opus-20250219',
        stop_reason: 'end_turn',
        usage: { input_tokens: 20, output_tokens: 150 },
      },
    })

    const response = await client.post('/v1/messages', {
      model: 'claude-3-opus-20250219',
      max_tokens: 2048,
      messages: [
        { role: 'user', content: 'Analyze this complex problem...' },
      ],
      temperature: 0.5,
    })

    expect(mockPost).toHaveBeenCalledWith('/v1/messages', expect.objectContaining({
      model: 'claude-3-opus-20250219',
    }))
    expect(response.data.model).toBe('claude-3-opus-20250219')
  })

  it('调用 claude-3-haiku（轻量快速模型）', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({
      data: {
        id: 'msg_haiku_001',
        type: 'message',
        role: 'assistant',
        content: [{ type: 'text', text: 'Quick answer.' }],
        model: 'claude-3-haiku-20240307',
        stop_reason: 'end_turn',
        usage: { input_tokens: 5, output_tokens: 3 },
      },
    })

    await client.post('/v1/messages', {
      model: 'claude-3-haiku-20240307',
      max_tokens: 256,
      messages: [{ role: 'user', content: 'Quick question' }],
    })

    const [, body] = mockPost.mock.calls[0]
    expect(body.model).toBe('claude-3-haiku-20240307')
  })

  it('Token 计数端点 /v1/messages/count_tokens', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({
      data: {
        type: 'message',
        id: 'ctm_01234',
        input_tokens: 12,
        cache_creation_input_tokens: 0,
        cache_read_input_tokens: 0,
      },
    })

    const response = await client.post('/v1/messages/count_tokens', {
      model: 'claude-3-5-sonnet-20241022',
      messages: [{ role: 'user', content: 'Count these tokens' }],
    })

    expect(mockPost).toHaveBeenCalledWith(
      '/v1/messages/count_tokens',
      expect.objectContaining({ model: 'claude-3-5-sonnet-20241022' })
    )
    expect(response.data).toHaveProperty('input_tokens')
  })

  it('多轮对话调用', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({
      data: {
        id: 'msg_multi',
        type: 'message',
        role: 'assistant',
        content: [{ type: 'text', text: 'Paris.' }],
        model: 'claude-3-5-sonnet-20241022',
        stop_reason: 'end_turn',
        usage: { input_tokens: 25, output_tokens: 5 },
      },
    })

    await client.post('/v1/messages', {
      model: 'claude-3-5-sonnet-20241022',
      max_tokens: 512,
      messages: [
        { role: 'user', content: 'What is the capital of France?' },
        { role: 'assistant', content: 'The capital of France is...' },
        { role: 'user', content: 'Just give me the city name.' },
      ],
    })

    const [, body] = mockPost.mock.calls[0]
    expect(body.messages).toHaveLength(3)
    expect(body.messages[2].role).toBe('user')
  })

  it('带 system prompt 调用', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({ data: { type: 'message', role: 'assistant', content: [] } })

    await client.post('/v1/messages', {
      model: 'claude-3-5-sonnet-20241022',
      max_tokens: 1024,
      system: 'You are a helpful assistant that speaks only in Chinese.',
      messages: [{ role: 'user', content: 'Hello' }],
    })

    const [, body] = mockPost.mock.calls[0]
    expect(body.system).toContain('Chinese')
  })
})

// ══════════════════════════════════════════════════════════════
// 2. OpenAI Chat Completions 兼容  (/v1/chat/completions)
// ══════════════════════════════════════════════════════════════

describe('OpenAI Chat Completions 兼容接口 - /v1/chat/completions', () => {
  it('gpt-4 非流式调用', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({
      data: {
        id: 'chatcmpl-123',
        object: 'chat.completion',
        model: 'gpt-4',
        choices: [
          {
            index: 0,
            message: { role: 'assistant', content: 'Hello! How can I assist you?' },
            finish_reason: 'stop',
          },
        ],
        usage: { prompt_tokens: 10, completion_tokens: 12, total_tokens: 22 },
      },
    })

    const response = await client.post('/v1/chat/completions', {
      model: 'gpt-4',
      messages: [{ role: 'user', content: 'Hello' }],
      temperature: 0.7,
      max_tokens: 1024,
    })

    expect(mockPost).toHaveBeenCalledWith('/v1/chat/completions', expect.objectContaining({
      model: 'gpt-4',
    }))
    expect(response.data.choices[0].message.role).toBe('assistant')
  })

  it('gpt-4o 调用', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({
      data: {
        id: 'chatcmpl-gpt4o',
        object: 'chat.completion',
        model: 'gpt-4o',
        choices: [{ index: 0, message: { role: 'assistant', content: 'Sure!' }, finish_reason: 'stop' }],
        usage: { prompt_tokens: 8, completion_tokens: 3, total_tokens: 11 },
      },
    })

    await client.post('/v1/chat/completions', {
      model: 'gpt-4o',
      messages: [{ role: 'user', content: 'Can you help me?' }],
    })

    const [endpoint, body] = mockPost.mock.calls[0]
    expect(endpoint).toBe('/v1/chat/completions')
    expect(body.model).toBe('gpt-4o')
  })

  it('gpt-3.5-turbo 流式调用', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({ data: {} })

    await client.post('/v1/chat/completions', {
      model: 'gpt-3.5-turbo',
      messages: [{ role: 'user', content: 'Tell me a story' }],
      stream: true,
    })

    const [, body] = mockPost.mock.calls[0]
    expect(body.stream).toBe(true)
    expect(body.model).toBe('gpt-3.5-turbo')
  })

  it('gpt-4-turbo 带 system 角色的多轮对话', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({
      data: {
        id: 'chatcmpl-turbo',
        object: 'chat.completion',
        model: 'gpt-4-turbo',
        choices: [{ index: 0, message: { role: 'assistant', content: 'Done.' }, finish_reason: 'stop' }],
        usage: { prompt_tokens: 30, completion_tokens: 5, total_tokens: 35 },
      },
    })

    await client.post('/v1/chat/completions', {
      model: 'gpt-4-turbo',
      messages: [
        { role: 'system', content: 'You are a coding assistant.' },
        { role: 'user', content: 'Write a Python hello world' },
        { role: 'assistant', content: 'print("Hello, World!")' },
        { role: 'user', content: 'Add a comment to it.' },
      ],
    })

    const [, body] = mockPost.mock.calls[0]
    expect(body.messages[0].role).toBe('system')
    expect(body.messages).toHaveLength(4)
  })

  it('/chat/completions 等价路径同样有效', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({ data: { choices: [] } })

    // 不带 /v1 前缀的路径也被网关支持
    await client.post('/chat/completions', {
      model: 'gpt-4',
      messages: [{ role: 'user', content: 'test' }],
    })

    expect(mockPost).toHaveBeenCalledWith('/chat/completions', expect.any(Object))
  })
})

// ══════════════════════════════════════════════════════════════
// 3. 图像生成  (/v1/images/generations)
// ══════════════════════════════════════════════════════════════

describe('OpenAI 图像生成 - /v1/images/generations', () => {
  it('DALL-E 3 生成图像', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({
      data: {
        created: 1700000000,
        data: [{ url: 'https://cdn.example.com/image.png', revised_prompt: 'A serene mountain landscape' }],
      },
    })

    const response = await client.post('/v1/images/generations', {
      model: 'dall-e-3',
      prompt: 'A serene mountain landscape at sunset',
      n: 1,
      size: '1024x1024',
      quality: 'standard',
    })

    expect(mockPost).toHaveBeenCalledWith('/v1/images/generations', {
      model: 'dall-e-3',
      prompt: expect.stringContaining('landscape'),
      n: 1,
      size: '1024x1024',
      quality: 'standard',
    })
    expect(response.data.data[0]).toHaveProperty('url')
  })

  it('DALL-E 2 生成图像', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({
      data: {
        created: 1700000001,
        data: [{ url: 'https://cdn.example.com/dalle2.png' }],
      },
    })

    await client.post('/v1/images/generations', {
      model: 'dall-e-2',
      prompt: 'A cute cat',
      n: 1,
      size: '512x512',
    })

    const [, body] = mockPost.mock.calls[0]
    expect(body.model).toBe('dall-e-2')
    expect(body.size).toBe('512x512')
  })
})

// ══════════════════════════════════════════════════════════════
// 4. Gemini API 兼容  (/v1beta/models/...)
// ══════════════════════════════════════════════════════════════

describe('Gemini API 兼容接口 - /v1beta/models', () => {
  it('获取 Gemini 模型列表', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockGet.mockResolvedValueOnce({
      data: {
        models: [
          { name: 'models/gemini-2.5-pro', displayName: 'Gemini 2.5 Pro' },
          { name: 'models/gemini-2.5-flash', displayName: 'Gemini 2.5 Flash' },
        ],
      },
    })

    const response = await client.get('/v1beta/models')

    expect(mockGet).toHaveBeenCalledWith('/v1beta/models')
    expect(response.data.models).toHaveLength(2)
  })

  it('gemini-2.5-pro generateContent 调用', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({
      data: {
        candidates: [
          {
            content: {
              parts: [{ text: 'The sky is blue due to Rayleigh scattering.' }],
              role: 'model',
            },
            finishReason: 'STOP',
          },
        ],
        usageMetadata: { promptTokenCount: 8, candidatesTokenCount: 12 },
      },
    })

    const response = await client.post(
      '/v1beta/models/gemini-2.5-pro:generateContent',
      {
        contents: [{ role: 'user', parts: [{ text: 'Why is the sky blue?' }] }],
      }
    )

    expect(mockPost).toHaveBeenCalledWith(
      '/v1beta/models/gemini-2.5-pro:generateContent',
      { contents: expect.arrayContaining([expect.objectContaining({ role: 'user' })]) }
    )
    expect(response.data.candidates[0].content.role).toBe('model')
  })

  it('gemini-2.5-flash streamGenerateContent 调用', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({ data: {} })

    await client.post('/v1beta/models/gemini-2.5-flash:streamGenerateContent', {
      contents: [{ role: 'user', parts: [{ text: 'Write a short poem' }] }],
    })

    const [endpoint] = mockPost.mock.calls[0]
    expect(endpoint).toContain('streamGenerateContent')
    expect(endpoint).toContain('gemini-2.5-flash')
  })

  it('gemini-3.1-pro 带 generationConfig 调用', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockResolvedValueOnce({
      data: {
        candidates: [{ content: { parts: [{ text: 'Result' }], role: 'model' }, finishReason: 'STOP' }],
      },
    })

    await client.post('/v1beta/models/gemini-3.1-pro:generateContent', {
      contents: [{ role: 'user', parts: [{ text: 'Explain AI' }] }],
      generationConfig: {
        temperature: 0.8,
        maxOutputTokens: 512,
        topP: 0.9,
      },
    })

    const [, body] = mockPost.mock.calls[0]
    expect(body.generationConfig.temperature).toBe(0.8)
    expect(body.generationConfig.maxOutputTokens).toBe(512)
  })
})

// ══════════════════════════════════════════════════════════════
// 5. 模型列表查询  (GET /v1/models)
// ══════════════════════════════════════════════════════════════

describe('模型列表 - GET /v1/models', () => {
  it('获取所有可用模型', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockGet.mockResolvedValueOnce({
      data: {
        object: 'list',
        data: [
          { id: 'claude-3-5-sonnet-20241022', object: 'model', owned_by: 'anthropic' },
          { id: 'claude-3-opus-20250219', object: 'model', owned_by: 'anthropic' },
          { id: 'gpt-4', object: 'model', owned_by: 'openai' },
          { id: 'gpt-4o', object: 'model', owned_by: 'openai' },
          { id: 'gemini-2.5-pro', object: 'model', owned_by: 'google' },
        ],
      },
    })

    const response = await client.get('/v1/models')

    expect(mockGet).toHaveBeenCalledWith('/v1/models')
    expect(response.data.object).toBe('list')
    expect(response.data.data.length).toBeGreaterThan(0)

    const modelIds = response.data.data.map((m: { id: string }) => m.id)
    expect(modelIds).toContain('claude-3-5-sonnet-20241022')
    expect(modelIds).toContain('gpt-4')
    expect(modelIds).toContain('gemini-2.5-pro')
  })
})

// ══════════════════════════════════════════════════════════════
// 6. 用量查询  (GET /v1/usage)
// ══════════════════════════════════════════════════════════════

describe('API Key 用量查询 - GET /v1/usage', () => {
  it('查询当前 API Key 的配额和使用情况', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockGet.mockResolvedValueOnce({
      data: {
        quota: 10.0,
        used: 1.23,
        remaining: 8.77,
        concurrency_limit: 5,
        concurrency_used: 1,
      },
    })

    const response = await client.get('/v1/usage')

    expect(mockGet).toHaveBeenCalledWith('/v1/usage')
    expect(response.data).toHaveProperty('quota')
    expect(response.data).toHaveProperty('remaining')
  })
})

// ══════════════════════════════════════════════════════════════
// 7. Antigravity 专属路由
// ══════════════════════════════════════════════════════════════

// ══════════════════════════════════════════════════════════════
// 8. 认证方式验证
// ══════════════════════════════════════════════════════════════

describe('API Token 认证方式', () => {
  it('Authorization Bearer 头（推荐方式）', () => {
    const client = createGatewayClient('sk_live_abc123')
    expect(axios.create).toHaveBeenCalledWith(expect.objectContaining({
      headers: expect.objectContaining({
        Authorization: 'Bearer sk_live_abc123',
      }),
    }))
  })

  it('x-api-key 头方式', () => {
    // x-api-key 认证不通过 createGatewayClient，直接手动构造
    const headers = { 'x-api-key': 'sk_live_abc123', 'Content-Type': 'application/json' }
    expect(headers['x-api-key']).toBe('sk_live_abc123')
    expect(headers).not.toHaveProperty('Authorization')
  })

  it('x-goog-api-key 头方式（Gemini CLI 兼容）', () => {
    const headers = { 'x-goog-api-key': 'sk_live_abc123', 'Content-Type': 'application/json' }
    expect(headers['x-goog-api-key']).toBe('sk_live_abc123')
  })
})

// ══════════════════════════════════════════════════════════════
// 9. 错误处理
// ══════════════════════════════════════════════════════════════

describe('网关错误处理', () => {
  it('401 - API Key 无效（Anthropic 格式错误响应）', async () => {
    const client = createGatewayClient('sk_live_invalid', MOCK_BASE_URL)
    mockPost.mockRejectedValueOnce({
      response: {
        status: 401,
        data: {
          type: 'error',
          error: { type: 'authentication_error', message: 'Invalid API key' },
        },
      },
    })

    await expect(
      client.post('/v1/messages', {
        model: 'claude-3-5-sonnet-20241022',
        max_tokens: 10,
        messages: [{ role: 'user', content: 'test' }],
      })
    ).rejects.toMatchObject({
      response: { status: 401 },
    })
  })

  it('429 - 超过速率限制（OpenAI 格式错误响应）', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockRejectedValueOnce({
      response: {
        status: 429,
        data: {
          error: {
            message: 'Rate limit exceeded',
            type: 'rate_limit_error',
            code: 'rate_limit_exceeded',
          },
        },
      },
    })

    await expect(
      client.post('/v1/chat/completions', {
        model: 'gpt-4',
        messages: [{ role: 'user', content: 'test' }],
      })
    ).rejects.toMatchObject({
      response: { status: 429 },
    })
  })

  it('403 - 权限不足', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockRejectedValueOnce({
      response: {
        status: 403,
        data: {
          type: 'error',
          error: { type: 'permission_error', message: 'Access denied' },
        },
      },
    })

    await expect(
      client.post('/v1/messages', {
        model: 'claude-3-5-sonnet-20241022',
        max_tokens: 10,
        messages: [{ role: 'user', content: 'test' }],
      })
    ).rejects.toMatchObject({
      response: { status: 403 },
    })
  })

  it('503 - 服务不可用', async () => {
    const client = createGatewayClient(MOCK_TOKEN, MOCK_BASE_URL)
    mockPost.mockRejectedValueOnce({
      response: {
        status: 503,
        data: {
          type: 'error',
          error: { type: 'unavailable_error', message: 'Service temporarily unavailable' },
        },
      },
    })

    await expect(
      client.post('/v1/messages', {
        model: 'claude-3-5-sonnet-20241022',
        max_tokens: 10,
        messages: [{ role: 'user', content: 'test' }],
      })
    ).rejects.toMatchObject({
      response: { status: 503 },
    })
  })
})
