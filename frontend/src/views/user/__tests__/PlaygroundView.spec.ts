import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import PlaygroundView from '../PlaygroundView.vue'

const { listMock, listModelsMock, chatStreamMock, imageGenerateMock, isSimpleModeRef } = vi.hoisted(() => ({
  listMock: vi.fn(),
  listModelsMock: vi.fn(),
  chatStreamMock: vi.fn(),
  imageGenerateMock: vi.fn(),
  isSimpleModeRef: { value: false }
}))

vi.mock('@/api/keys', () => ({ keysAPI: { list: listMock } }))
vi.mock('@/api/playground', () => ({
  playgroundAPI: { listModelsForKey: listModelsMock, chatStream: chatStreamMock, imageGenerate: imageGenerateMock, imageEdit: vi.fn() },
  // 类型 re-export 占位（组件仅 import type）
  chatStream: chatStreamMock,
  imageGenerate: imageGenerateMock,
  imageEdit: vi.fn(),
  listModelsForKey: listModelsMock
}))
vi.mock('@/stores', () => ({ useAuthStore: () => ({ get isSimpleMode() { return isSimpleModeRef.value } }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const AppLayoutStub = defineComponent({ name: 'AppLayout', template: '<div><slot /></div>' })
const RouterLinkStub = defineComponent({ name: 'RouterLink', props: ['to'], template: '<a><slot /></a>' })

function makeKey(over: Record<string, unknown> = {}) {
  return {
    id: 1,
    name: 'my-key',
    key: 'sk-abc',
    status: 'active',
    group: { id: 1, name: 'G', platform: 'openai', allow_image_generation: true },
    ...over
  }
}

function mountView() {
  return mount(PlaygroundView, {
    global: {
      stubs: { AppLayout: AppLayoutStub, RouterLink: RouterLinkStub, 'router-link': RouterLinkStub }
    }
  })
}

describe('PlaygroundView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    isSimpleModeRef.value = false
    localStorage.clear()
    listModelsMock.mockResolvedValue(['gpt-4o'])
  })

  it('无可用 key 时展示引导创建', async () => {
    listMock.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 100, pages: 0 })
    const w = mountView()
    await flushPromises()
    expect(w.text()).toContain('playground.noKey')
  })

  it('OpenAI 且开启生图的 key → 生图 chip 可用', async () => {
    listMock.mockResolvedValue({ items: [makeKey()], total: 1, page: 1, page_size: 100, pages: 1 })
    const w = mountView()
    await flushPromises()
    const genChip = w.findAll('button').find((b) => b.text().includes('generateImage'))
    expect(genChip).toBeTruthy()
    expect(genChip!.attributes('disabled')).toBeUndefined()
  })

  it('anthropic 分组的 key → 生图 chip 灰化(disabled)', async () => {
    listMock.mockResolvedValue({
      items: [makeKey({ group: { id: 2, name: 'C', platform: 'anthropic', allow_image_generation: false } })],
      total: 1, page: 1, page_size: 100, pages: 1
    })
    const w = mountView()
    await flushPromises()
    const genChip = w.findAll('button').find((b) => b.text().includes('generateImage'))
    expect(genChip!.attributes('disabled')).toBeDefined()
  })

  it('非 Simple 模式且未确认 → 展示风险横幅，点击后写入 localStorage', async () => {
    listMock.mockResolvedValue({ items: [makeKey()], total: 1, page: 1, page_size: 100, pages: 1 })
    const w = mountView()
    await flushPromises()
    expect(w.text()).toContain('playground.risk.bannerTitle')
    const ackBtn = w.findAll('button').find((b) => b.text().includes('playground.risk.ack'))
    await ackBtn!.trigger('click')
    expect(localStorage.getItem('playground_risk_ack')).toBe('1')
    expect(w.text()).not.toContain('playground.risk.bannerTitle')
  })

  it('Simple 模式隐藏风险横幅', async () => {
    isSimpleModeRef.value = true
    listMock.mockResolvedValue({ items: [makeKey()], total: 1, page: 1, page_size: 100, pages: 1 })
    const w = mountView()
    await flushPromises()
    expect(w.text()).not.toContain('playground.risk.bannerTitle')
  })

  // 回归：push 进响应式数组后必须改代理，直接改原始对象不会重渲染
  it('流式 onDelta 会更新气泡内容并结束 streaming', async () => {
    isSimpleModeRef.value = true // 跳过风险横幅
    listMock.mockResolvedValue({ items: [makeKey()], total: 1, page: 1, page_size: 100, pages: 1 })
    chatStreamMock.mockImplementation(async (opts: { callbacks: Record<string, (...a: unknown[]) => void> }) => {
      opts.callbacks.onDelta('Hello')
      opts.callbacks.onDelta(' world')
      opts.callbacks.onDone()
    })
    const w = mountView()
    await flushPromises()

    const input = w.find('input[type="file"]').exists()
      ? w.findAll('input').find((i) => i.attributes('type') !== 'file')!
      : w.find('input')
    await input.setValue('你好')
    const sendBtn = w.findAll('button').find((b) => b.text() === '↑')
    expect(sendBtn).toBeTruthy()
    await sendBtn!.trigger('click')
    await flushPromises()

    expect(w.text()).toContain('Hello world')
    // streaming 结束：出现发送按钮而非停止按钮
    expect(w.findAll('button').some((b) => b.text() === '↑')).toBe(true)
  })

  // 回归：纯图像分组的 key，对话模型被过滤空 → 自动进入生图模式
  it('纯图像 Key 自动进入生图模式且不把图像模型当对话模型', async () => {
    isSimpleModeRef.value = true
    listMock.mockResolvedValue({ items: [makeKey()], total: 1, page: 1, page_size: 100, pages: 1 })
    listModelsMock.mockResolvedValue(['gpt-image-1', 'gpt-image-2']) // 仅图像模型
    const w = mountView()
    await flushPromises()
    // 处于生图模式：出现图像模型输入框(placeholder gpt-image-2)
    const imgModelInput = w.findAll('input').find((i) => i.attributes('placeholder') === 'gpt-image-2')
    expect(imgModelInput).toBeTruthy()
    // 对话模型下拉不应把 gpt-image-* 列为可选项
    const chatSelectHasImage = w
      .findAll('option')
      .some((o) => o.text().startsWith('gpt-image-'))
    expect(chatSelectHasImage).toBe(false)
  })

  // 回归：生图必须用 gpt-image-* 模型，不能复用聊天模型选择器
  it('文生图使用独立的 gpt-image 模型而非聊天 selectedModel', async () => {
    isSimpleModeRef.value = true
    listMock.mockResolvedValue({ items: [makeKey()], total: 1, page: 1, page_size: 100, pages: 1 })
    listModelsMock.mockResolvedValue(['gpt-5.4']) // 聊天模型，绝不能被生图使用
    imageGenerateMock.mockResolvedValue({ images: [{ b64: 'AAA' }], usage: {} })
    const w = mountView()
    await flushPromises()

    // 切到生图意图
    const genChip = w.findAll('button').find((b) => b.text().includes('generateImage'))!
    await genChip.trigger('click')
    const textInput = w.findAll('input').find((i) => i.attributes('type') !== 'file' && i.attributes('placeholder') !== 'gpt-image-1')!
    await textInput.setValue('画只猫')
    const sendBtn = w.findAll('button').find((b) => b.text() === '↑')!
    await sendBtn.trigger('click')
    await flushPromises()

    expect(imageGenerateMock).toHaveBeenCalledTimes(1)
    expect(imageGenerateMock.mock.calls[0][0].model).toBe('gpt-image-2')
    expect(imageGenerateMock.mock.calls[0][0].model).not.toBe('gpt-5.4')
  })
})
