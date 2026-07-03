/**
 * Playground (Model Experience) API
 *
 * 前端直连网关 /v1 端点，携带用户自己的网关 API Key（sk-...）。
 * 注意：这里用裸 fetch（因 EventSource 不支持 POST 流式），不走 axios 拦截器，
 * 所以 Authorization 需手动携带；用的是网关 Key（非 JWT，不过期），无需刷新。
 */

import { buildGatewayUrl } from './url'

/** 一条对话消息 */
export interface PlaygroundMessage {
  role: 'system' | 'user' | 'assistant'
  content: string
}

/** 归一化后的网关错误 */
export interface PlaygroundError {
  status: number
  code?: string | number
  message: string
}

/** 生图结果项 */
export interface PlaygroundImage {
  b64?: string
  url?: string
}

/** 生图返回（含可选 usage） */
export interface PlaygroundImageResult {
  images: PlaygroundImage[]
  usage?: Record<string, unknown>
}

/** 对话流式回调 */
export interface ChatStreamCallbacks {
  onDelta: (text: string) => void
  onReasoning?: (text: string) => void
  onUsage?: (usage: Record<string, unknown>) => void
  onError: (err: PlaygroundError) => void
  onDone: () => void
}

/** 平台 → 是否走 Claude 原生 /v1/messages */
function isClaudePlatform(platform: string | undefined): boolean {
  return platform === 'anthropic'
}

/** 从响应体尽力解析出结构化错误 */
async function toPlaygroundError(res: Response): Promise<PlaygroundError> {
  let message = `HTTP ${res.status}`
  let code: string | number | undefined
  try {
    const text = await res.text()
    if (text) {
      try {
        const j = JSON.parse(text)
        const e = j?.error ?? j
        message = e?.message ?? e?.reason ?? message
        code = e?.code ?? e?.type
      } catch {
        message = text.slice(0, 500)
      }
    }
  } catch {
    /* ignore body read errors */
  }
  return { status: res.status, code, message }
}

/** 把 SSE 一行 data: 载荷解析并分发（兼容 OpenAI 与 Claude 两种事件格式） */
function dispatchStreamChunk(
  payload: string,
  claude: boolean,
  cb: ChatStreamCallbacks
): boolean {
  // 返回 true 表示流已结束（[DONE] / message_stop）
  if (payload === '[DONE]') return true
  let j: any
  try {
    j = JSON.parse(payload)
  } catch {
    return false // 心跳/非 JSON 行，忽略
  }

  if (claude) {
    // Claude 原生事件：content_block_delta / message_delta(usage) / message_stop
    const type = j?.type
    if (type === 'content_block_delta') {
      const d = j?.delta
      if (d?.type === 'thinking_delta' && d?.thinking) cb.onReasoning?.(d.thinking)
      else if (d?.text) cb.onDelta(d.text)
    } else if (type === 'message_delta' && j?.usage) {
      cb.onUsage?.(j.usage)
    } else if (type === 'message_start' && j?.message?.usage) {
      cb.onUsage?.(j.message.usage)
    } else if (type === 'error') {
      cb.onError({ status: 0, message: j?.error?.message ?? 'stream error' })
    } else if (type === 'message_stop') {
      return true
    }
    return false
  }

  // OpenAI 兼容：choices[].delta / usage
  const delta = j?.choices?.[0]?.delta
  if (delta?.reasoning_content) cb.onReasoning?.(delta.reasoning_content)
  if (delta?.content) cb.onDelta(delta.content)
  if (j?.usage) cb.onUsage?.(j.usage)
  if (j?.error) cb.onError({ status: 0, message: j.error?.message ?? 'stream error' })
  return false
}

/**
 * 流式对话。按 group 平台选择端点：
 * - anthropic → POST /v1/messages（Claude 原生事件）
 * - 其它      → POST /v1/chat/completions（OpenAI 兼容）
 */
export async function chatStream(opts: {
  apiKey: string
  platform?: string
  model: string
  messages: PlaygroundMessage[]
  params?: Record<string, unknown>
  signal: AbortSignal
  callbacks: ChatStreamCallbacks
}): Promise<void> {
  const claude = isClaudePlatform(opts.platform)
  const cb = opts.callbacks
  const path = claude ? '/v1/messages' : '/v1/chat/completions'

  let body: Record<string, unknown>
  if (claude) {
    // Claude 端点：system 单列，max_tokens 必填
    const system = opts.messages.find((m) => m.role === 'system')?.content
    const turns = opts.messages.filter((m) => m.role !== 'system')
    body = {
      model: opts.model,
      messages: turns,
      stream: true,
      max_tokens: (opts.params?.max_tokens as number) ?? 4096,
      ...(system ? { system } : {}),
      ...opts.params
    }
  } else {
    body = { model: opts.model, messages: opts.messages, stream: true, ...opts.params }
  }

  let res: Response
  try {
    res = await fetch(buildGatewayUrl(path), {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${opts.apiKey}`
      },
      body: JSON.stringify(body),
      signal: opts.signal
    })
  } catch (e) {
    if (opts.signal.aborted) return // 主动中断，不算错误
    cb.onError({ status: 0, message: (e as Error)?.message ?? 'network error' })
    return
  }

  if (!res.ok || !res.body) {
    cb.onError(await toPlaygroundError(res))
    return
  }

  // 兜底：若后端未按 SSE 返回（如某些模型/错误回退成普通 JSON），按整体 JSON 处理，
  // 避免解析器找不到 data: 行而气泡空转、streaming 卡死。
  const contentType = res.headers?.get?.('content-type') ?? ''
  if (contentType && !contentType.includes('text/event-stream')) {
    try {
      const text = await new Response(res.body).text()
      handleNonStreamBody(text, claude, cb)
    } catch (e) {
      cb.onError({ status: 0, message: (e as Error)?.message ?? 'parse error' })
    }
    cb.onDone()
    return
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buf = ''
  const processLine = (line: string): boolean => {
    const s = line.trim()
    if (!s.startsWith('data:')) return false
    return dispatchStreamChunk(s.slice(5).trim(), claude, cb)
  }
  try {
    for (;;) {
      const { value, done } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const lines = buf.split('\n')
      buf = lines.pop() ?? ''
      for (const line of lines) {
        if (processLine(line)) {
          cb.onDone()
          return
        }
      }
    }
    // flush 末尾残留行（流结束时无 trailing newline 的情况）
    if (buf.trim()) processLine(buf)
  } catch (e) {
    if (opts.signal.aborted) return
    cb.onError({ status: 0, message: (e as Error)?.message ?? 'stream read error' })
    return
  }
  cb.onDone()
}

/** 非 SSE 响应体：尽力从整体 JSON 提取正文或错误 */
function handleNonStreamBody(text: string, claude: boolean, cb: ChatStreamCallbacks): void {
  let j: any
  try {
    j = JSON.parse(text)
  } catch {
    if (text.trim()) cb.onDelta(text.slice(0, 2000))
    return
  }
  if (j?.error) {
    cb.onError({ status: 0, message: j.error?.message ?? 'error', code: j.error?.code ?? j.error?.type })
    return
  }
  if (claude) {
    // Anthropic 非流式：content 是块数组
    const blocks = j?.content
    if (Array.isArray(blocks)) {
      for (const b of blocks) {
        if (b?.type === 'thinking' && b?.thinking) cb.onReasoning?.(b.thinking)
        else if (b?.text) cb.onDelta(b.text)
      }
    }
    if (j?.usage) cb.onUsage?.(j.usage)
    return
  }
  // OpenAI 非流式：choices[0].message.content
  const msg = j?.choices?.[0]?.message
  if (msg?.reasoning_content) cb.onReasoning?.(msg.reasoning_content)
  if (msg?.content) cb.onDelta(msg.content)
  if (j?.usage) cb.onUsage?.(j.usage)
}

/** 文生图：POST /v1/images/generations（JSON） */
export async function imageGenerate(opts: {
  apiKey: string
  model: string
  prompt: string
  size: string
  n: number
}): Promise<PlaygroundImageResult> {
  const res = await fetch(buildGatewayUrl('/v1/images/generations'), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${opts.apiKey}`
    },
    body: JSON.stringify({
      model: opts.model,
      prompt: opts.prompt,
      size: opts.size,
      n: opts.n
    })
  })
  if (!res.ok) throw await toPlaygroundError(res)
  const j = await res.json()
  return {
    images: (j?.data ?? []).map((it: any) => ({ b64: it.b64_json, url: it.url })),
    usage: j?.usage
  }
}

/**
 * 图生图 / 图片编辑：POST /v1/images/edits（multipart/form-data）
 * 注意：绝不手动设置 Content-Type，让浏览器自动带上含 boundary 的头，
 * 否则后端会因缺 boundary 解析失败。
 */
export async function imageEdit(opts: {
  apiKey: string
  model: string
  prompt: string
  size: string
  n: number
  images: File[]
  mask?: File
}): Promise<PlaygroundImageResult> {
  const form = new FormData()
  form.append('model', opts.model)
  form.append('prompt', opts.prompt)
  form.append('size', opts.size)
  form.append('n', String(opts.n))
  // 后端接受 image 或 image[]（多图用重复 image 字段）
  opts.images.forEach((f) => form.append('image', f))
  if (opts.mask) form.append('mask', opts.mask)

  const res = await fetch(buildGatewayUrl('/v1/images/edits'), {
    method: 'POST',
    headers: { Authorization: `Bearer ${opts.apiKey}` },
    body: form
  })
  if (!res.ok) throw await toPlaygroundError(res)
  const j = await res.json()
  return {
    images: (j?.data ?? []).map((it: any) => ({ b64: it.b64_json, url: it.url })),
    usage: j?.usage
  }
}

/** 拉取某个 key 在网关侧可用的模型（GET /v1/models，带该 key） */
export async function listModelsForKey(apiKey: string): Promise<string[]> {
  const res = await fetch(buildGatewayUrl('/v1/models'), {
    method: 'GET',
    headers: { Authorization: `Bearer ${apiKey}` }
  })
  if (!res.ok) throw await toPlaygroundError(res)
  const j = await res.json()
  const data = j?.data ?? j?.models ?? []
  return data
    .map((it: any) => (typeof it === 'string' ? it : it?.id ?? it?.name))
    .filter((x: unknown): x is string => typeof x === 'string' && x.length > 0)
}

export const playgroundAPI = {
  chatStream,
  imageGenerate,
  imageEdit,
  listModelsForKey
}

export default playgroundAPI
