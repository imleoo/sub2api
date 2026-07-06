<template>
  <AppLayout>
    <div class="flex h-[calc(100vh-8rem)] flex-col">
      <!-- 风险确认横幅（首次进入，Simple 模式隐藏） -->
      <div
        v-if="showRiskBanner"
        class="mb-3 flex flex-col gap-2 rounded-xl border border-amber-300 bg-amber-50 p-3 text-sm text-amber-800 dark:border-amber-700/50 dark:bg-amber-900/20 dark:text-amber-200 sm:flex-row sm:items-center sm:justify-between"
      >
        <div>
          <p class="font-semibold">⚠ {{ t('playground.risk.bannerTitle') }}</p>
          <p class="mt-0.5 text-xs opacity-90">{{ t('playground.risk.bannerBody') }}</p>
        </div>
        <button class="btn btn-primary shrink-0 self-start sm:self-auto" @click="ackRisk">
          {{ t('playground.risk.ack') }}
        </button>
      </div>

      <!-- 无可用 Key -->
      <div v-if="!loading && activeKeys.length === 0" class="flex flex-1 flex-col items-center justify-center gap-3 text-center">
        <p class="text-lg font-medium text-gray-900 dark:text-white">{{ t('playground.noKey') }}</p>
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('playground.noKeyHint') }}</p>
        <router-link to="/keys" class="btn btn-primary">{{ t('playground.goCreateKey') }}</router-link>
      </div>

      <template v-else>
        <!-- 空态：居中欢迎语 -->
        <div v-if="messages.length === 0" class="flex flex-1 flex-col items-center justify-center">
          <h1 class="mb-8 text-2xl font-semibold text-gray-900 dark:text-white">
            {{ t('playground.greeting') }}
          </h1>
          <div class="w-full max-w-2xl px-4">
            <component :is="composer" />
          </div>
        </div>

        <!-- 对话态：消息列表 + 底部输入 -->
        <template v-else>
          <div ref="listEl" class="flex-1 space-y-4 overflow-y-auto px-1 py-2">
            <MessageBubble
              v-for="m in messages"
              :key="m.id"
              :message="m"
              @use-as-edit-input="useAsEditInput"
            />
          </div>
          <div class="mt-2">
            <component :is="composer" />
          </div>
        </template>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch, h } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import MessageBubble, { type UiMessage } from '@/components/playground/MessageBubble.vue'
import { keysAPI } from '@/api/keys'
import { playgroundAPI, type PlaygroundImage, type PlaygroundMessage, type PlaygroundError } from '@/api/playground'
import { useAuthStore } from '@/stores'
import type { ApiKey } from '@/types'

const { t } = useI18n()
const authStore = useAuthStore()

type Mode = 'chat' | 'image' | 'edit'

// ─── State ────────────────────────────────
const loading = ref(false)
const keys = ref<ApiKey[]>([])
const selectedKeyId = ref<number | null>(null)
const models = ref<string[]>([])
const selectedModel = ref<string>('')
const mode = ref<Mode>('chat')
const messages = ref<UiMessage[]>([])
const inputText = ref('')
const uploadFiles = ref<File[]>([])
const attachmentPreviews = ref<string[]>([])
const streaming = ref(false)
const showParams = ref(false)
const listEl = ref<HTMLElement | null>(null)
let abortController: AbortController | null = null
let msgSeq = 0

const RISK_ACK_KEY = 'playground_risk_ack'
const riskAcked = ref(localStorage.getItem(RISK_ACK_KEY) === '1')

const MAX_IMAGES = 4
const MAX_IMAGE_MB = 20
const IMAGE_SIZES = ['1024x1024', '1536x1024', '1024x1536']

const params = ref({
  temperature: { enabled: false, value: 1 },
  max_tokens: { enabled: false, value: 4096 },
  top_p: { enabled: false, value: 1 },
  system: { enabled: false, value: '' }
})
const imageSize = ref(IMAGE_SIZES[0])
const imageCount = ref(1)
// 生图专用模型（须 gpt-image-* 前缀，与聊天模型选择器解耦；默认与后端一致）
const imageModel = ref('gpt-image-2')

// ─── Computed ─────────────────────────────
const activeKeys = computed(() => keys.value.filter((k) => k.status === 'active'))
const selectedKey = computed(() => keys.value.find((k) => k.id === selectedKeyId.value) ?? null)
const isSimpleMode = computed(() => authStore.isSimpleMode)
const showRiskBanner = computed(() => !riskAcked.value && !isSimpleMode.value && activeKeys.value.length > 0)

// 对话可用模型（剔除图像模型，它们只能走生图端点）
const chatModels = computed(() => models.value.filter((m) => !isImageModelName(m)))

const imageCapable = computed(() => {
  const g = selectedKey.value?.group
  if (!g) return false
  return (g.platform === 'openai' || g.platform === 'lingjing') && g.allow_image_generation === true
})

const canSend = computed(() => {
  if (streaming.value || !selectedKey.value) return false
  if (mode.value === 'chat') return inputText.value.trim().length > 0 && !!selectedModel.value
  if (mode.value === 'edit') return uploadFiles.value.length > 0 && inputText.value.trim().length > 0
  return inputText.value.trim().length > 0 // image
})

// ─── Load ─────────────────────────────────
async function loadKeys() {
  loading.value = true
  try {
    const res = await keysAPI.list(1, 100)
    keys.value = res.items ?? []
    const first = activeKeys.value[0]
    if (first) selectedKeyId.value = first.id
  } finally {
    loading.value = false
  }
}

// 图像模型不能用于对话端点（后端只在 images 端点接受 gpt-image-*）
function isImageModelName(id: string): boolean {
  const m = id.toLowerCase()
  return m.startsWith('gpt-image-') || m.includes('dall-e')
}

async function loadModels() {
  const key = selectedKey.value
  models.value = []
  selectedModel.value = ''
  if (!key) return
  try {
    const list = await playgroundAPI.listModelsForKey(key.key)
    models.value = list
    // 对话默认选第一个「非图像」模型；纯图像分组则留空（对话不可用）
    selectedModel.value = list.find((m) => !isImageModelName(m)) ?? ''
    // 纯图像分组：自动切到生图意图，避免用户在对话模式里困惑
    if (!selectedModel.value && list.length > 0 && imageCapable.value) {
      mode.value = 'image'
    }
  } catch {
    models.value = []
  }
}

watch(selectedKeyId, () => {
  loadModels()
  // 切 key 后若不支持生图但当前是生图意图，回退对话
  if (!imageCapable.value && mode.value !== 'chat') mode.value = 'chat'
})

onMounted(loadKeys)

// ─── Actions ──────────────────────────────
function ackRisk() {
  riskAcked.value = true
  localStorage.setItem(RISK_ACK_KEY, '1')
}

function setMode(next: Mode) {
  if ((next === 'image' || next === 'edit') && !imageCapable.value) return
  mode.value = next
}

function onFilePick(e: Event) {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  for (const f of files) {
    if (uploadFiles.value.length >= MAX_IMAGES) break
    if (f.size > MAX_IMAGE_MB * 1024 * 1024) {
      pushErrorToast(t('playground.imageTooLarge', { name: f.name, size: MAX_IMAGE_MB }))
      continue
    }
    uploadFiles.value.push(f)
    const reader = new FileReader()
    reader.onload = () => attachmentPreviews.value.push(String(reader.result))
    reader.readAsDataURL(f)
  }
  input.value = ''
  if (uploadFiles.value.length > 0) mode.value = 'edit'
}

function removeUpload(idx: number) {
  uploadFiles.value.splice(idx, 1)
  attachmentPreviews.value.splice(idx, 1)
  if (uploadFiles.value.length === 0 && mode.value === 'edit') mode.value = 'chat'
}

async function useAsEditInput(img: PlaygroundImage) {
  if (!imageCapable.value) return
  const file = await imageToFile(img)
  if (!file) return
  if (uploadFiles.value.length >= MAX_IMAGES) return
  uploadFiles.value.push(file)
  attachmentPreviews.value.push(img.url ?? `data:image/png;base64,${img.b64}`)
  mode.value = 'edit'
}

async function imageToFile(img: PlaygroundImage): Promise<File | null> {
  try {
    if (img.b64) {
      const bin = atob(img.b64)
      const bytes = new Uint8Array(bin.length)
      for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
      return new File([bytes], 'input.png', { type: 'image/png' })
    }
    if (img.url) {
      const res = await fetch(img.url)
      const blob = await res.blob()
      return new File([blob], 'input.png', { type: blob.type || 'image/png' })
    }
  } catch {
    return null
  }
  return null
}

const toast = ref<string>('')
function pushErrorToast(msg: string) {
  toast.value = msg
  window.setTimeout(() => (toast.value = ''), 4000)
}

function mapError(err: PlaygroundError): string {
  if (err.status === 402 || err.status === 403) return t('playground.errors.insufficient')
  if (err.status === 429) return t('playground.errors.rateLimited')
  if (err.status === 404) return t('playground.errors.imageNotAllowed')
  return err.message || t('playground.errors.generic')
}

function buildParamsPayload(): Record<string, unknown> {
  const p: Record<string, unknown> = {}
  if (params.value.temperature.enabled) p.temperature = params.value.temperature.value
  if (params.value.max_tokens.enabled) p.max_tokens = params.value.max_tokens.value
  if (params.value.top_p.enabled) p.top_p = params.value.top_p.value
  return p
}

function scrollToBottom() {
  nextTick(() => {
    if (listEl.value) listEl.value.scrollTop = listEl.value.scrollHeight
  })
}

async function send() {
  if (!canSend.value || !selectedKey.value) return
  if (mode.value === 'chat') await sendChat()
  else await sendImage()
}

async function sendChat() {
  const key = selectedKey.value!
  const text = inputText.value.trim()

  // 组装请求消息（历史文本 + 本次输入）
  const reqMessages: PlaygroundMessage[] = []
  if (params.value.system.enabled && params.value.system.value.trim()) {
    reqMessages.push({ role: 'system', content: params.value.system.value.trim() })
  }
  for (const m of messages.value) {
    if (m.kind === 'text' && !m.error && m.content) {
      reqMessages.push({ role: m.role, content: m.content })
    }
  }
  reqMessages.push({ role: 'user', content: text })

  messages.value.push({ id: ++msgSeq, role: 'user', kind: 'text', content: text })
  messages.value.push({ id: ++msgSeq, role: 'assistant', kind: 'text', content: '', streaming: true })
  // 取回数组中的响应式代理再改，直接改原始对象不会触发重渲染
  const assistant = messages.value[messages.value.length - 1]
  inputText.value = ''
  streaming.value = true
  scrollToBottom()

  abortController = new AbortController()
  await playgroundAPI.chatStream({
    apiKey: key.key,
    platform: key.group?.platform,
    model: selectedModel.value,
    messages: reqMessages,
    params: buildParamsPayload(),
    signal: abortController.signal,
    callbacks: {
      onDelta: (txt) => {
        assistant.content += txt
        scrollToBottom()
      },
      onReasoning: (txt) => {
        assistant.reasoning = (assistant.reasoning ?? '') + txt
      },
      onUsage: (u) => {
        assistant.usage = u
      },
      onError: (err) => {
        assistant.error = mapError(err)
      },
      onDone: () => {
        assistant.streaming = false
        streaming.value = false
        scrollToBottom()
      }
    }
  })
  // 兜底：若 stream 因异常提前返回未触发 onDone
  assistant.streaming = false
  streaming.value = false
}

async function sendImage() {
  const key = selectedKey.value!
  const prompt = inputText.value.trim()
  const isEdit = mode.value === 'edit'
  const files = [...uploadFiles.value]
  const previews = [...attachmentPreviews.value]

  messages.value.push({
    id: ++msgSeq,
    role: 'user',
    kind: 'text',
    content: prompt,
    attachments: isEdit ? previews : undefined
  })
  messages.value.push({ id: ++msgSeq, role: 'assistant', kind: 'image', content: '', images: [], streaming: true })
  // 取回响应式代理再改（见 sendChat 同注）
  const assistant = messages.value[messages.value.length - 1]
  inputText.value = ''
  if (isEdit) {
    uploadFiles.value = []
    attachmentPreviews.value = []
  }
  streaming.value = true
  scrollToBottom()

  try {
    // 生图必须用 gpt-image-* 模型，不能复用聊天模型选择器（否则后端 400）
    const model = imageModel.value.trim() || 'gpt-image-2'
    const result = isEdit
      ? await playgroundAPI.imageEdit({
          apiKey: key.key,
          model,
          prompt,
          size: imageSize.value,
          n: imageCount.value,
          images: files
        })
      : await playgroundAPI.imageGenerate({
          apiKey: key.key,
          model,
          prompt,
          size: imageSize.value,
          n: imageCount.value
        })
    assistant.images = result.images
    assistant.usage = result.usage
  } catch (e) {
    assistant.error = mapError(e as PlaygroundError)
  } finally {
    assistant.streaming = false
    streaming.value = false
    if (isEdit) mode.value = 'chat'
    scrollToBottom()
  }
}

function stop() {
  abortController?.abort()
  streaming.value = false
}

// ─── Composer（内联渲染函数，避免 props 透传样板） ───
const composer = () => {
  const key = selectedKey.value
  const placeholder =
    mode.value === 'image'
      ? t('playground.imagePrompt')
      : mode.value === 'edit'
        ? t('playground.editPrompt')
        : t('playground.inputPlaceholder')

  return h('div', { class: 'space-y-2' }, [
    // 上传缩略图预览
    attachmentPreviews.value.length
      ? h(
          'div',
          { class: 'flex flex-wrap gap-2' },
          attachmentPreviews.value.map((src, i) =>
            h('div', { key: i, class: 'relative' }, [
              h('img', { src, class: 'h-14 w-14 rounded-lg object-cover' }),
              h(
                'button',
                {
                  class: 'absolute -right-1 -top-1 flex h-5 w-5 items-center justify-center rounded-full bg-gray-700 text-xs text-white',
                  onClick: () => removeUpload(i)
                },
                '×'
              )
            ])
          )
        )
      : null,

    // 胶囊输入框
    h('div', { class: 'flex items-center gap-2 rounded-3xl border border-gray-200 bg-white px-3 py-2 shadow-sm dark:border-dark-600 dark:bg-dark-700' }, [
      // + 上传
      imageCapable.value
        ? h('label', { class: 'flex h-8 w-8 shrink-0 cursor-pointer items-center justify-center rounded-full text-gray-500 hover:bg-gray-100 dark:hover:bg-dark-600', title: t('playground.upload') }, [
            h('span', { class: 'text-xl leading-none' }, '＋'),
            h('input', { type: 'file', accept: 'image/*', multiple: true, class: 'hidden', onChange: onFilePick })
          ])
        : null,
      // 文本输入
      h('input', {
        class: 'min-w-0 flex-1 bg-transparent text-sm text-gray-900 outline-none placeholder:text-gray-400 dark:text-white',
        value: inputText.value,
        placeholder,
        disabled: streaming.value,
        onInput: (e: Event) => (inputText.value = (e.target as HTMLInputElement).value),
        onKeydown: (e: KeyboardEvent) => {
          if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault()
            send()
          }
        }
      }),
      // 模型选择器（仅对话模式，且仅列对话模型；生图模型在下方独立设置）
      mode.value === 'chat' && chatModels.value.length
        ? h(
            'select',
            {
              class: 'max-w-[9rem] shrink-0 rounded-full bg-gray-100 px-2 py-1 text-xs text-gray-700 outline-none dark:bg-dark-600 dark:text-gray-200',
              value: selectedModel.value,
              onChange: (e: Event) => (selectedModel.value = (e.target as HTMLSelectElement).value)
            },
            chatModels.value.map((m) => h('option', { value: m }, m))
          )
        : null,
      // 参数面板开关
      mode.value === 'chat'
        ? h('button', { class: 'shrink-0 rounded-full px-2 py-1 text-xs text-gray-500 hover:bg-gray-100 dark:hover:bg-dark-600', onClick: () => (showParams.value = !showParams.value) }, '⚙')
        : null,
      // 发送 / 停止
      streaming.value
        ? h('button', { class: 'flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-gray-800 text-white', title: t('playground.stop'), onClick: stop }, '■')
        : h('button', {
            class: 'flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-black text-white disabled:opacity-40 dark:bg-white dark:text-black',
            title: isSimpleMode.value ? t('playground.send') : t('playground.risk.sendTooltip'),
            disabled: !canSend.value,
            onClick: send
          }, '↑')
    ]),

    // 参数面板
    showParams.value && mode.value === 'chat' ? renderParamPanel() : null,

    // 底部：Key 选择器 + 意图 chip + 风险提示
    h('div', { class: 'flex flex-wrap items-center gap-2' }, [
      // Key 选择器
      activeKeys.value.length
        ? h(
            'select',
            {
              class: 'rounded-full border border-gray-200 bg-white px-3 py-1 text-xs text-gray-700 outline-none dark:border-dark-600 dark:bg-dark-700 dark:text-gray-200',
              value: selectedKeyId.value ?? '',
              onChange: (e: Event) => (selectedKeyId.value = Number((e.target as HTMLSelectElement).value))
            },
            activeKeys.value.map((k) =>
              h('option', { value: k.id }, `${k.name} · ${k.group?.name ?? '—'}`)
            )
          )
        : null,
      // 生成图片 chip
      h('button', {
        class: chipClass('image'),
        disabled: !imageCapable.value,
        title: imageCapable.value ? '' : t('playground.imageNotAvailable'),
        onClick: () => setMode(mode.value === 'image' ? 'chat' : 'image')
      }, '🖼 ' + t('playground.chips.generateImage')),
      // 图生图 chip
      h('button', {
        class: chipClass('edit'),
        disabled: !imageCapable.value,
        title: imageCapable.value ? '' : t('playground.imageNotAvailable'),
        onClick: () => setMode(mode.value === 'edit' ? 'chat' : 'edit')
      }, '🎨 ' + t('playground.chips.editImage')),
      // 生图参数（模型/尺寸/数量）
      mode.value !== 'chat'
        ? h('span', { class: 'flex items-center gap-1 text-xs text-gray-500' }, [
            h('input', { class: 'w-28 rounded bg-gray-100 px-1.5 py-0.5 dark:bg-dark-600 dark:text-gray-200', value: imageModel.value, placeholder: 'gpt-image-2', title: t('playground.model'), onInput: (e: Event) => (imageModel.value = (e.target as HTMLInputElement).value) }),
            h('select', { class: 'rounded bg-gray-100 px-1 py-0.5 dark:bg-dark-600', value: imageSize.value, onChange: (e: Event) => (imageSize.value = (e.target as HTMLSelectElement).value) }, IMAGE_SIZES.map((s) => h('option', { value: s }, s))),
            h('select', { class: 'rounded bg-gray-100 px-1 py-0.5 dark:bg-dark-600', value: String(imageCount.value), onChange: (e: Event) => (imageCount.value = Number((e.target as HTMLSelectElement).value)) }, [1, 2, 3, 4].map((n) => h('option', { value: n }, `×${n}`)))
          ])
        : null,
      // 风险常驻提示
      !isSimpleMode.value
        ? h('span', { class: 'ml-auto text-[11px] text-amber-600 dark:text-amber-400', title: t('playground.risk.bannerBody') }, '⚠ ' + t('playground.risk.inlineHint'))
        : null
    ]),

    // 轻量 toast
    toast.value ? h('p', { class: 'text-xs text-red-500' }, toast.value) : null,
    // 对话模式下当前 key 无「对话」模型的提示
    key && chatModels.value.length === 0 && mode.value === 'chat'
      ? h(
          'p',
          { class: 'text-xs text-amber-600 dark:text-amber-400' },
          imageCapable.value ? t('playground.imageOnlyKey') : t('playground.noModel')
        )
      : null
  ])
}

function chipClass(m: Mode): string {
  const active = mode.value === m
  const base = 'rounded-full border px-3 py-1 text-xs transition disabled:cursor-not-allowed disabled:opacity-40 '
  return base + (active
    ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
    : 'border-gray-200 bg-white text-gray-600 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-300')
}

function renderParamPanel() {
  const numRow = (labelKey: string, obj: { enabled: boolean; value: number }, step: number, min: number, max: number) =>
    h('div', { class: 'flex items-center gap-2' }, [
      h('label', { class: 'flex w-32 items-center gap-1 text-xs text-gray-600 dark:text-gray-300' }, [
        h('input', { type: 'checkbox', checked: obj.enabled, onChange: (e: Event) => (obj.enabled = (e.target as HTMLInputElement).checked) }),
        t(labelKey)
      ]),
      h('input', { type: 'number', step, min, max, value: obj.value, disabled: !obj.enabled, class: 'w-24 rounded border border-gray-200 bg-white px-2 py-0.5 text-xs disabled:opacity-40 dark:border-dark-600 dark:bg-dark-700', onInput: (e: Event) => (obj.value = Number((e.target as HTMLInputElement).value)) })
    ])

  return h('div', { class: 'space-y-2 rounded-xl border border-gray-200 p-3 dark:border-dark-600' }, [
    h('p', { class: 'text-xs font-medium text-gray-700 dark:text-gray-200' }, t('playground.params.title')),
    numRow('playground.params.temperature', params.value.temperature, 0.1, 0, 2),
    numRow('playground.params.maxTokens', params.value.max_tokens, 256, 1, 200000),
    numRow('playground.params.topP', params.value.top_p, 0.05, 0, 1),
    h('div', { class: 'flex items-start gap-2' }, [
      h('label', { class: 'flex w-32 items-center gap-1 text-xs text-gray-600 dark:text-gray-300' }, [
        h('input', { type: 'checkbox', checked: params.value.system.enabled, onChange: (e: Event) => (params.value.system.enabled = (e.target as HTMLInputElement).checked) }),
        t('playground.params.systemPrompt')
      ]),
      h('textarea', { rows: 2, value: params.value.system.value, disabled: !params.value.system.enabled, class: 'flex-1 rounded border border-gray-200 bg-white px-2 py-1 text-xs disabled:opacity-40 dark:border-dark-600 dark:bg-dark-700', onInput: (e: Event) => (params.value.system.value = (e.target as HTMLTextAreaElement).value) })
    ])
  ])
}
</script>
