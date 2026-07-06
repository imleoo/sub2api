import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import PlaygroundView from '../PlaygroundView.vue'

const { listMock, listModelsMock, chatStreamMock, imageGenerateMock, getModelsMock, isSimpleModeRef } = vi.hoisted(() => ({
  listMock: vi.fn(),
  listModelsMock: vi.fn(),
  chatStreamMock: vi.fn(),
  imageGenerateMock: vi.fn(),
  getModelsMock: vi.fn(),
  isSimpleModeRef: { value: false }
}))

vi.mock('@/api/keys', () => ({ keysAPI: { list: listMock } }))
vi.mock('@/api/models', () => ({ getModels: getModelsMock }))
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
    // 权威 mode 映射：区分 chat / image_generation
    getModelsMock.mockResolvedValue({
      models: [
        { id: 'gpt-4o', mode: 'chat' },
        { id: 'gpt-5.4', mode: 'chat' },
        { id: 'gpt-image-1', mode: 'image_generation' },
        { id: 'gpt-image-2', mode: 'image_generation' },
        { id: 'kling-v2', mode: 'image_generation' }
      ]
    })
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

  // 回归：纯图像分组的 key（按 mode 判定）→ 自动进入生图模式，对话模式下无对话模型
  it('纯图像 Key 自动进入生图模式，对话模式下提示仅支持生图', async () => {
    isSimpleModeRef.value = true
    listMock.mockResolvedValue({ items: [makeKey()], total: 1, page: 1, page_size: 100, pages: 1 })
    listModelsMock.mockResolvedValue(['gpt-image-1', 'gpt-image-2']) // 仅图像模型
    const w = mountView()
    await flushPromises()
    // 自动生图模式：图像模型下拉列出该 key 的图像模型
    const optTexts = w.findAll('option').map((o) => o.text())
    expect(optTexts).toContain('gpt-image-1')
    expect(optTexts).toContain('gpt-image-2')
    // 切回对话模式（再次点击生成图片 chip 取消）→ 无对话模型，提示仅支持生图
    const genChip = w.findAll('button').find((b) => b.text().includes('generateImage'))!
    await genChip.trigger('click')
    expect(w.text()).toContain('playground.imageOnlyKey')
  })

  // 回归：生图用该 Key 的图像模型（数据驱动，非硬编码 gpt-image），不复用聊天 selectedModel
  it('文生图使用该 Key 的图像模型且不等于聊天模型', async () => {
    isSimpleModeRef.value = true
    listMock.mockResolvedValue({ items: [makeKey()], total: 1, page: 1, page_size: 100, pages: 1 })
    listModelsMock.mockResolvedValue(['gpt-5.4', 'gpt-image-2']) // 对话 + 图像各一
    imageGenerateMock.mockResolvedValue({ images: [{ b64: 'AAA' }], usage: {} })
    const w = mountView()
    await flushPromises()

    // 切到生图意图
    const genChip = w.findAll('button').find((b) => b.text().includes('generateImage'))!
    await genChip.trigger('click')
    const textInput = w.findAll('input').find((i) => i.attributes('type') !== 'file')!
    await textInput.setValue('画只猫')
    const sendBtn = w.findAll('button').find((b) => b.text() === '↑')!
    await sendBtn.trigger('click')
    await flushPromises()

    expect(imageGenerateMock).toHaveBeenCalledTimes(1)
    // 模型来自该 key 的图像模型列表（gpt-image-2），而非聊天模型 gpt-5.4
    expect(imageGenerateMock.mock.calls[0][0].model).toBe('gpt-image-2')
    expect(imageGenerateMock.mock.calls[0][0].model).not.toBe('gpt-5.4')
  })

  // 新模型即插即用：只要 /api/v1/models 标注 mode=image_generation，就当图像模型（如 kling）
  it('非 gpt-image 的图像模型(kling)也被识别为图像模型', async () => {
    isSimpleModeRef.value = true
    listMock.mockResolvedValue({ items: [makeKey()], total: 1, page: 1, page_size: 100, pages: 1 })
    listModelsMock.mockResolvedValue(['kling-v2']) // 仅一个非 gpt-image 的图像模型
    imageGenerateMock.mockResolvedValue({ images: [{ url: 'http://x' }], usage: {} })
    const w = mountView()
    await flushPromises()
    // 应自动进生图模式，且 kling-v2 作为图像模型可选
    const optTexts = w.findAll('option').map((o) => o.text())
    expect(optTexts).toContain('kling-v2')
    const textInput = w.findAll('input').find((i) => i.attributes('type') !== 'file')!
    await textInput.setValue('a cat')
    const sendBtn = w.findAll('button').find((b) => b.text() === '↑')!
    await sendBtn.trigger('click')
    await flushPromises()
    expect(imageGenerateMock.mock.calls[0][0].model).toBe('kling-v2')
  })
})
