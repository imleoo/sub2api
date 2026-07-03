import { describe, expect, it, vi, beforeEach } from 'vitest'
import {
  chatStream,
  imageGenerate,
  imageEdit,
  listModelsForKey,
  type ChatStreamCallbacks
} from '../playground'

// buildGatewayUrl 直接返回 path，便于断言端点
vi.mock('../url', () => ({
  buildGatewayUrl: (p: string) => p
}))

/** 用一组 SSE 字符串构造一个可读流 reader */
function makeReader(chunks: string[]) {
  const enc = new TextEncoder()
  let i = 0
  return {
    read: () =>
      Promise.resolve(
        i < chunks.length
          ? { value: enc.encode(chunks[i++]), done: false }
          : { value: undefined, done: true }
      )
  }
}

function streamResponse(chunks: string[]) {
  return { ok: true, body: { getReader: () => makeReader(chunks) } } as unknown as Response
}

function makeCallbacks() {
  const calls = {
    delta: '' as string,
    reasoning: '' as string,
    usage: null as Record<string, unknown> | null,
    error: null as { status: number; message: string } | null,
    done: false
  }
  const cb: ChatStreamCallbacks = {
    onDelta: (t) => (calls.delta += t),
    onReasoning: (t) => (calls.reasoning += t),
    onUsage: (u) => (calls.usage = u),
    onError: (e) => (calls.error = e),
    onDone: () => (calls.done = true)
  }
  return { cb, calls }
}

const noSignal = { aborted: false } as AbortSignal

describe('playground api - chatStream (OpenAI 兼容)', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('解析 delta / reasoning / usage，命中 /v1/chat/completions 且带 Authorization 与 stream', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      streamResponse([
        'data: {"choices":[{"delta":{"content":"He"}}]}\n',
        'data: {"choices":[{"delta":{"reasoning_content":"why"}}]}\n',
        'data: {"choices":[{"delta":{"content":"llo"}}]}\n',
        'data: {"usage":{"prompt_tokens":5,"completion_tokens":3}}\n',
        'data: [DONE]\n'
      ])
    )
    vi.stubGlobal('fetch', fetchMock)

    const { cb, calls } = makeCallbacks()
    await chatStream({
      apiKey: 'sk-test',
      platform: 'openai',
      model: 'gpt-x',
      messages: [{ role: 'user', content: 'hi' }],
      signal: noSignal,
      callbacks: cb
    })

    expect(calls.delta).toBe('Hello')
    expect(calls.reasoning).toBe('why')
    expect(calls.usage).toEqual({ prompt_tokens: 5, completion_tokens: 3 })
    expect(calls.done).toBe(true)

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/v1/chat/completions')
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer sk-test')
    expect(JSON.parse(init.body).stream).toBe(true)
  })

  it('非 2xx 响应映射为 onError（含 status）', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: false,
      status: 429,
      text: () => Promise.resolve('{"error":{"message":"rate"}}')
    })
    vi.stubGlobal('fetch', fetchMock)

    const { cb, calls } = makeCallbacks()
    await chatStream({
      apiKey: 'sk',
      platform: 'openai',
      model: 'm',
      messages: [{ role: 'user', content: 'x' }],
      signal: noSignal,
      callbacks: cb
    })
    expect(calls.error?.status).toBe(429)
    expect(calls.error?.message).toBe('rate')
  })
})

describe('playground api - chatStream (Claude 原生)', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('命中 /v1/messages，解析 content_block_delta 与 message_delta usage，system 单列', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      streamResponse([
        'data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hi"}}\n',
        'data: {"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"hmm"}}\n',
        'data: {"type":"message_delta","usage":{"output_tokens":2}}\n',
        'data: {"type":"message_stop"}\n'
      ])
    )
    vi.stubGlobal('fetch', fetchMock)

    const { cb, calls } = makeCallbacks()
    await chatStream({
      apiKey: 'sk',
      platform: 'anthropic',
      model: 'claude-x',
      messages: [
        { role: 'system', content: 'you are nice' },
        { role: 'user', content: 'hi' }
      ],
      params: { max_tokens: 100 },
      signal: noSignal,
      callbacks: cb
    })

    expect(calls.delta).toBe('Hi')
    expect(calls.reasoning).toBe('hmm')
    expect(calls.usage).toEqual({ output_tokens: 2 })
    expect(calls.done).toBe(true)

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/v1/messages')
    const body = JSON.parse(init.body)
    expect(body.system).toBe('you are nice')
    expect(body.max_tokens).toBe(100)
    // system 不应留在 messages 里
    expect(body.messages.every((m: { role: string }) => m.role !== 'system')).toBe(true)
  })
})

describe('playground api - 生图', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('imageGenerate 解析 b64_json 与 url', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({ data: [{ b64_json: 'abc' }, { url: 'http://x/y.png' }], usage: { n: 2 } })
    })
    vi.stubGlobal('fetch', fetchMock)

    const res = await imageGenerate({ apiKey: 'sk', model: 'gpt-image-1', prompt: 'cat', size: '1024x1024', n: 2 })
    expect(res.images).toEqual([{ b64: 'abc', url: undefined }, { b64: undefined, url: 'http://x/y.png' }])
    expect(fetchMock.mock.calls[0][0]).toBe('/v1/images/generations')
  })

  it('imageEdit 用 FormData 携带 image/mask/prompt/model，且不手动设置 Content-Type', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: [{ b64_json: 'zzz' }] })
    })
    vi.stubGlobal('fetch', fetchMock)

    const img = new File(['a'], 'a.png', { type: 'image/png' })
    const mask = new File(['m'], 'm.png', { type: 'image/png' })
    const res = await imageEdit({ apiKey: 'sk', model: 'gpt-image-1', prompt: 'edit it', size: '1024x1024', n: 1, images: [img], mask })

    expect(res.images).toEqual([{ b64: 'zzz', url: undefined }])
    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/v1/images/edits')
    expect((init.headers as Record<string, string>)['Content-Type']).toBeUndefined()
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer sk')
    const form = init.body as FormData
    expect(form.get('prompt')).toBe('edit it')
    expect(form.get('model')).toBe('gpt-image-1')
    expect(form.getAll('image')).toHaveLength(1)
    expect(form.get('mask')).toBeInstanceOf(File)
  })
})

describe('playground api - listModelsForKey', () => {
  beforeEach(() => vi.restoreAllMocks())

  it('从 data[].id 提取模型名', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: [{ id: 'gpt-4' }, { id: 'claude-3' }, { name: 'x' }] })
    })
    vi.stubGlobal('fetch', fetchMock)

    const models = await listModelsForKey('sk')
    expect(models).toEqual(['gpt-4', 'claude-3', 'x'])
    expect(fetchMock.mock.calls[0][0]).toBe('/v1/models')
  })
})
