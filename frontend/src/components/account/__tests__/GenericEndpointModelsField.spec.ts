import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import GenericEndpointModelsField from '../GenericEndpointModelsField.vue'

const { fetchEndpointModelsMock, showSuccessMock, showErrorMock } = vi.hoisted(() => ({
  fetchEndpointModelsMock: vi.fn(),
  showSuccessMock: vi.fn(),
  showErrorMock: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { accounts: { fetchEndpointModels: fetchEndpointModelsMock } }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: showSuccessMock, showError: showErrorMock })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (k: string) => k }) }
})

function mountField(props: Record<string, unknown> = {}) {
  return mount(GenericEndpointModelsField, {
    props: { modelValue: [], baseUrl: 'https://x.test', ...props }
  })
}

// 按 i18n key 文案定位按钮（mock 下 t 返回 key 本身）。
function buttonByKey(wrapper: ReturnType<typeof mountField>, key: string) {
  return wrapper.findAll('button').find((b) => b.text().includes(key))!
}

describe('GenericEndpointModelsField', () => {
  beforeEach(() => {
    fetchEndpointModelsMock.mockReset()
    showSuccessMock.mockReset()
    showErrorMock.mockReset()
  })

  it('拉取后填充可勾选清单，勾选后 emit update:modelValue', async () => {
    fetchEndpointModelsMock.mockResolvedValue({ models: ['m-a', 'm-b'], fetched: 2, pricing_added: 0 })
    const wrapper = mountField({ accountId: 7 })

    await buttonByKey(wrapper, 'fetchModels').trigger('click')
    await flushPromises()

    expect(fetchEndpointModelsMock).toHaveBeenCalledWith(
      expect.objectContaining({ base_url: 'https://x.test', account_id: 7 })
    )
    expect(showSuccessMock).toHaveBeenCalled()

    const boxes = wrapper.findAll('input[type="checkbox"]')
    expect(boxes.length).toBe(2)

    await boxes[0].trigger('change')
    const emitted = wrapper.emitted('update:modelValue')
    expect(emitted).toBeTruthy()
    expect(emitted!.at(-1)![0]).toContain('m-a')
  })

  it('已选模型即使不在拉取结果也显示且为勾中态', () => {
    const wrapper = mountField({ modelValue: ['existing-model'] })
    const boxes = wrapper.findAll('input[type="checkbox"]')
    expect(boxes.length).toBe(1)
    expect((boxes[0].element as HTMLInputElement).checked).toBe(true)
  })

  it('全选 emit 全部拉取结果，清空 emit 去除', async () => {
    fetchEndpointModelsMock.mockResolvedValue({ models: ['m-a', 'm-b'], fetched: 2, pricing_added: 0 })
    const wrapper = mountField()
    await buttonByKey(wrapper, 'fetchModels').trigger('click')
    await flushPromises()

    await buttonByKey(wrapper, 'modelPickerSelectAll').trigger('click')
    let emitted = wrapper.emitted('update:modelValue')!
    expect(emitted.at(-1)![0]).toEqual(expect.arrayContaining(['m-a', 'm-b']))

    await buttonByKey(wrapper, 'modelPickerClear').trigger('click')
    emitted = wrapper.emitted('update:modelValue')!
    expect(emitted.at(-1)![0]).toEqual([])
  })

  it('手动输入按逗号/空白拆分去重后 emit', async () => {
    const wrapper = mountField()
    await buttonByKey(wrapper, 'modelPickerManual').trigger('click')
    const textarea = wrapper.find('textarea')
    expect(textarea.exists()).toBe(true)
    await textarea.setValue('a, b  b\nc')
    const emitted = wrapper.emitted('update:modelValue')!
    expect(emitted.at(-1)![0]).toEqual(['a', 'b', 'c'])
  })

  it('base_url 为空时报错、不发请求', async () => {
    const wrapper = mountField({ baseUrl: '' })
    // 按钮 disabled，直接调用组件内 fetch 逻辑通过点击（trigger 绕过 disabled）
    await buttonByKey(wrapper, 'fetchModels').trigger('click')
    await flushPromises()
    expect(fetchEndpointModelsMock).not.toHaveBeenCalled()
  })
})
