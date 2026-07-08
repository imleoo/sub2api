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
import { getModels } from '@/api/models'
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
// 生图专用模型（从该 key 的图像模型中选，默认取第一个；不写死具体模型名）
const imageModel = ref('')
// 模型 id → mode 映射（来自 /api/v1/models，权威区分 chat / image_generation / …）
const modelModes = ref<Record<string, string>>({})

// ─── Computed ─────────────────────────────
const activeKeys = computed(() => keys.value.filter((k) => k.status === 'active'))
const selectedKey = computed(() => keys.value.find((k) => k.id === selectedKeyId.value) ?? null)
const isSimpleMode = computed(() => authStore.isSimpleMode)
const showRiskBanner = computed(() => !riskAcked.value && !isSimpleMode.value && activeKeys.value.length > 0)

// 数据驱动判定：mode 为 image_generation 即图像模型（未知 mode 视为对话，避免误藏对话模型）
function isImageModel(id: string): boolean {
  return modelModes.value[id] === 'image_generation'
}
// 对话可用模型（剔除图像模型，它们只能走生图端点）
const chatModels = computed(() => models.value.filter((m) => !isImageModel(m)))
// 该 key 可用的图像模型
const imageModels = computed(() => models.value.filter((m) => isImageModel(m)))

const imageCapable = computed(() => {
  const g = selectedKey.value?.group
  if (!g) return false
  return (g.platform === 'openai' || g.platform === 'lingjing') && g.allow_image_generation === true
})

const canSend = computed(() => {
  if (streaming.value || !selectedKey.value) return false
  if (mode.value === 'chat') return inputText.value.trim().length > 0 && !!selectedModel.value
  const hasImageModel = imageModel.value.trim().length > 0
  if (mode.value === 'edit')
    return uploadFiles.value.length > 0 && inputText.value.trim().length > 0 && hasImageModel
  return inputText.value.trim().length > 0 && hasImageModel // image
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

// 加载模型 id → mode 映射（一次即可，缓存）
async function ensureModelModes() {
  if (Object.keys(modelModes.value).length > 0) return
  try {
    const res = await getModels()
    const map: Record<string, string> = {}
    for (const m of res.models ?? []) map[m.id] = m.mode
    modelModes.value = map
  } catch {
    /* 拿不到 mode 时，全部按对话处理（不误藏对话模型） */
  }
}

async function loadModels() {
  const key = selectedKey.value
  models.value = []
  selectedModel.value = ''
  if (!key) return
  await ensureModelModes()
  try {
    const list = await playgroundAPI.listModelsForKey(key.key)
    models.value = list
    // 对话默认选第一个非图像模型；图像默认选第一个图像模型（均由数据决定，不写死）
    selectedModel.value = list.find((m) => !isImageModel(m)) ?? ''
    const firstImage = list.find((m) => isImageModel(m))
    if (firstImage) imageModel.value = firstImage
    // 纯图像分组：自动切到生图意图，避免用户在对话模式里困惑
    if (!selectedModel.value && firstImage && imageCapable.value) {
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

onMounted(() => {
  ensureModelModes()
  loadKeys()
})

// ─── Actions ──────────────────────────────
function ackRisk() {
  riskAcked.value = true
  localStorage.setItem(RISK_ACK_KEY, '1')
}

function setMode(next: Mode) {
  if ((next === 'image' || next === 'edit') && !imageCapable.value) return
  mode.value = next
}

function readDataURL(f: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(f)
  })
}

async function onFilePick(e: Event) {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = '' // 立即清空，允许再次选同名文件；已拿到 files 快照
  for (const f of files) {
    if (uploadFiles.value.length >= MAX_IMAGES) break
    if (f.size > MAX_IMAGE_MB * 1024 * 1024) {
      pushErrorToast(t('playground.imageTooLarge', { name: f.name, size: MAX_IMAGE_MB }))
      continue
    }
    try {
      // 顺序读取，文件与预览成对追加，保证 uploadFiles[i] 与 attachmentPreviews[i] 对应
      const dataUrl = await readDataURL(f)
      uploadFiles.value.push(f)
      attachmentPreviews.value.push(dataUrl)
    } catch {
      pushErrorToast(t('playground.errors.generic'))
    }
  }
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

  // 生图也走 abortController，让「停止」能真正中断请求（否则照样计费）
  const controller = new AbortController()
  abortController = controller
  try {
    // 生图模型来自该 key 的图像模型选择（canSend 已保证非空）
    const model = imageModel.value.trim()
    const result = isEdit
      ? await playgroundAPI.imageEdit({
          apiKey: key.key,
          model,
          prompt,
          size: imageSize.value,
          n: imageCount.value,
          images: files,
          signal: controller.signal
        })
      : await playgroundAPI.imageGenerate({
          apiKey: key.key,
          model,
          prompt,
          size: imageSize.value,
          n: imageCount.value,
          signal: controller.signal
        })
    assistant.images = result.images
    assistant.usage = result.usage
  } catch (e) {
    if (controller.signal.aborted) {
      // 用户主动停止：移除空的助手气泡（生图无部分结果可留）
      const idx = messages.value.indexOf(assistant)
      if (idx !== -1) messages.value.splice(idx, 1)
    } else {
      assistant.error = mapError(e as PlaygroundError)
    }
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
          // !e.isComposing：中文/日文等输入法回车确认候选词时不触发发送
          if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
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
            // 图像模型：有可选列表用下拉；分组允许生图但未列出图像模型时回退可编辑输入
            imageModels.value.length
              ? h(
                  'select',
                  { class: 'max-w-[10rem] rounded bg-gray-100 px-1 py-0.5 dark:bg-dark-600 dark:text-gray-200', value: imageModel.value, title: t('playground.model'), onChange: (e: Event) => (imageModel.value = (e.target as HTMLSelectElement).value) },
                  imageModels.value.map((m) => h('option', { value: m }, m))
                )
              : h('input', { class: 'w-32 rounded bg-gray-100 px-1.5 py-0.5 dark:bg-dark-600 dark:text-gray-200', value: imageModel.value, placeholder: t('playground.imageModelPlaceholder'), title: t('playground.model'), onInput: (e: Event) => (imageModel.value = (e.target as HTMLInputElement).value) }),
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
