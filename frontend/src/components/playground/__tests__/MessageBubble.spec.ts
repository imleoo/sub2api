import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import MessageBubble, { type UiMessage } from '../MessageBubble.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function mountMsg(message: UiMessage) {
  return mount(MessageBubble, { props: { message } })
}

describe('MessageBubble', () => {
  it('用户文本消息右对齐并展示内容', () => {
    const w = mountMsg({ id: 1, role: 'user', kind: 'text', content: '你好' })
    expect(w.text()).toContain('你好')
    expect(w.find('.justify-end').exists()).toBe(true)
  })

  it('助手生图消息渲染 b64 图片为 data URL', () => {
    const w = mountMsg({ id: 2, role: 'assistant', kind: 'image', content: '', images: [{ b64: 'AAA' }] })
    const img = w.find('img')
    expect(img.exists()).toBe(true)
    expect(img.attributes('src')).toBe('data:image/png;base64,AAA')
  })

  it('渲染 url 图片直接使用 url', () => {
    const w = mountMsg({ id: 3, role: 'assistant', kind: 'image', content: '', images: [{ url: 'http://x/y.png' }] })
    expect(w.find('img').attributes('src')).toBe('http://x/y.png')
  })

  it('错误态展示错误文本', () => {
    const w = mountMsg({ id: 4, role: 'assistant', kind: 'text', content: '', error: '余额不足' })
    expect(w.text()).toContain('余额不足')
  })

  it('有 usage 时展示用量回显', () => {
    const w = mountMsg({ id: 5, role: 'assistant', kind: 'text', content: 'hi', usage: { input_tokens: 10, output_tokens: 5 } })
    expect(w.text()).toContain('playground.usage.tokens')
  })

  it('推理块可折叠展开', async () => {
    const w = mountMsg({ id: 6, role: 'assistant', kind: 'text', content: 'ans', reasoning: '推理内容' })
    // 默认收起
    expect(w.text()).not.toContain('推理内容')
    await w.find('button').trigger('click')
    expect(w.text()).toContain('推理内容')
  })

  it('点击「作为图生图输入」触发 emit', async () => {
    const w = mountMsg({ id: 7, role: 'assistant', kind: 'image', content: '', images: [{ b64: 'AAA' }] })
    const btns = w.findAll('button')
    const useBtn = btns.find((b) => b.text().includes('useAsEditInput'))
    expect(useBtn).toBeTruthy()
    await useBtn!.trigger('click')
    expect(w.emitted('use-as-edit-input')).toBeTruthy()
    expect(w.emitted('use-as-edit-input')![0][0]).toEqual({ b64: 'AAA' })
  })
})
