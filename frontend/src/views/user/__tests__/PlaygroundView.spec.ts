import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import PlaygroundView from '../PlaygroundView.vue'

const { listMock, listModelsMock, chatStreamMock, isSimpleModeRef } = vi.hoisted(() => ({
  listMock: vi.fn(),
  listModelsMock: vi.fn(),
  chatStreamMock: vi.fn(),
  isSimpleModeRef: { value: false }
}))

vi.mock('@/api/keys', () => ({ keysAPI: { list: listMock } }))
vi.mock('@/api/playground', () => ({
  playgroundAPI: { listModelsForKey: listModelsMock, chatStream: chatStreamMock, imageGenerate: vi.fn(), imageEdit: vi.fn() },
  // 类型 re-export 占位（组件仅 import type）
  chatStream: chatStreamMock,
  imageGenerate: vi.fn(),
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
})
