import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ModelPricingsView from '../ModelPricingsView.vue'
import { createModelPricing, listModelPricings, listModelPricingProviders, syncModelPricingsFromMaas } from '@/api/admin/modelPricings'

vi.mock('vue-i18n', async importOriginal => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/api/admin/modelPricings', () => ({
  listModelPricings: vi.fn(),
  createModelPricing: vi.fn(),
  updateModelPricing: vi.fn(),
  deleteModelPricing: vi.fn(),
  triggerModelPricingSync: vi.fn(),
  syncModelPricingsFromUpstream: vi.fn(),
  syncModelPricingsFromMaas: vi.fn(),
  clearAllModelPricingDiscounts: vi.fn(),
  listModelPricingProviders: vi.fn(),
}))

const mockedListModelPricings = vi.mocked(listModelPricings)
const mockedListModelPricingProviders = vi.mocked(listModelPricingProviders)
const mockedCreateModelPricing = vi.mocked(createModelPricing)
const mockedSyncModelPricingsFromMaas = vi.mocked(syncModelPricingsFromMaas)

function mountView() {
  return mount(ModelPricingsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
        Pagination: true,
        BaseDialog: true,
        ConfirmDialog: true,
        Icon: true,
        Select: {
          props: ['modelValue', 'options'],
          emits: ['change', 'update:modelValue'],
          template: '<select :value="modelValue" @change="$emit(\'change\')"><option v-for="o in options" :key="o.value" :value="o.value">{{ o.label }}</option></select>',
        },
      },
    },
  })
}

// BaseDialog 在 mountView() 里被整体 stub 掉（不渲染插槽内容），无法用来交互表单。
// 这里单独渲染一个「show 时渲染默认插槽 + footer 插槽」的轻量 stub，供表单交互类用例使用。
const BaseDialogStub = defineComponent({
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

function mountViewWithDialogs() {
  return mount(ModelPricingsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
        Pagination: true,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        Icon: true,
        Select: {
          props: ['modelValue', 'options'],
          emits: ['change', 'update:modelValue'],
          template: '<select :value="modelValue" @change="$emit(\'change\')"><option v-for="o in options" :key="o.value" :value="o.value">{{ o.label }}</option></select>',
        },
      },
    },
  })
}

describe('ModelPricingsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockedListModelPricings.mockResolvedValue({
      data: {
        items: [],
        total: 0,
        page: 1,
        page_size: 20,
      },
    })
    mockedListModelPricingProviders.mockResolvedValue({
      data: { providers: [] },
    })
  })

  it('default load follows the model-square scope server-side (no visible_only param, no toggle)', async () => {
    const wrapper = mountView()
    await flushPromises()

    // 去掉「只显示可见模型」开关：后台默认即广场口径（后端按海外开关+版本下限+可路由过滤），
    // 前端不再传 visible_only。
    const params = mockedListModelPricings.mock.calls.at(-1)?.[0] as Record<string, unknown>
    expect(params).toMatchObject({ page: 1, page_size: 20 })
    expect(params).not.toHaveProperty('visible_only')
    expect(wrapper.text()).not.toContain('只显示可见模型')
  })
})

  it('sorts abnormal pricing_health rows to top and shows a red badge', async () => {
    const base = {
      provider: 'x', mode: 'chat', pricing_unit: 'token',
      input_cost_per_token: 1e-6, output_cost_per_token: 2e-6,
      cache_creation_input_token_cost: null, cache_read_input_token_cost: null,
      output_cost_per_image: null, output_cost_per_image_token: null,
      supports_prompt_caching: false, custom_input_cost: null, custom_output_cost: null,
      discount_rate: null, is_custom: false, is_enabled: true,
      display_name: null, description: null,
      last_synced_at: null, created_at: '', updated_at: '',
    }
    mockedListModelPricings.mockResolvedValue({
      data: {
        items: [
          { ...base, id: 1, model_id: 'normal-model', pricing_health: 'ok' },
          { ...base, id: 2, model_id: 'orphan-model', pricing_health: 'orphan_no_active_account' },
        ],
        total: 2, page: 1, page_size: 20,
      },
    } as never)

    const wrapper = mountView()
    await flushPromises()

    const html = wrapper.html()
    // 异常徽标文案出现
    expect(html).toContain('无活跃账号')
    // 异常行（orphan-model）排在正常行（normal-model）之前
    expect(html.indexOf('orphan-model')).toBeLessThan(html.indexOf('normal-model'))
  })

it('create form: pricing_unit=image_generation shows output_cost_per_image field, auto-links mode, and blanks token-unit fields on submit', async () => {
  mockedListModelPricings.mockResolvedValue({
    data: { items: [], total: 0, page: 1, page_size: 20 },
  } as never)
  mockedListModelPricingProviders.mockResolvedValue({ data: { providers: [] } } as never)
  mockedCreateModelPricing.mockResolvedValue({ data: {} } as never)

  const wrapper = mountViewWithDialogs()
  await flushPromises()

  const openCreateBtn = wrapper.findAll('button').find(b => b.text().includes('添加自定义模型'))
  expect(openCreateBtn).toBeTruthy()
  await openCreateBtn!.trigger('click')

  // token 计价单位是默认值，此时按张计价字段不应渲染
  expect(wrapper.find('input[placeholder="例: 0.3"]').exists()).toBe(false)

  await wrapper.find('input[placeholder="例: my-custom-model"]').setValue('my-image-model')

  const imageUnitRadio = wrapper.find('input[type="radio"][value="image_generation"]')
  expect(imageUnitRadio.exists()).toBe(true)
  await imageUnitRadio.setValue()

  // 切到「图片（按张）」后：字段联动展示 + mode 自动切为 image_generation
  const priceInput = wrapper.find('input[placeholder="例: 0.3"]')
  expect(priceInput.exists()).toBe(true)
  await priceInput.setValue('0.5')

  await wrapper.find('form#create-pricing-form').trigger('submit')
  await flushPromises()

  expect(mockedCreateModelPricing).toHaveBeenCalledTimes(1)
  const payload = mockedCreateModelPricing.mock.calls[0][0]
  expect(payload).toMatchObject({
    model_id: 'my-image-model',
    pricing_unit: 'image_generation',
    mode: 'image_generation',
    output_cost_per_image: 0.5,
    // 按张计价不应带出上游 per-token 占位字段，避免计费口径混入 token 价格
    input_cost_per_token: null,
    custom_input_cost: null,
    output_cost_per_token: null,
  })
})

it('MaaS sync modal posts to the unified sync-maas endpoint (not the legacy per-provider endpoints)', async () => {
  mockedListModelPricings.mockResolvedValue({
    data: { items: [], total: 0, page: 1, page_size: 20 },
  } as never)
  mockedListModelPricingProviders.mockResolvedValue({ data: { providers: [] } } as never)
  mockedSyncModelPricingsFromMaas.mockResolvedValue({
    data: { message: 'ok', total: 3, source: 'wanjie', mode: 'live' },
  } as never)

  const wrapper = mountViewWithDialogs()
  await flushPromises()

  const openMaasBtn = wrapper.findAll('button').find(b => b.text().includes('同步 MaaS 定价'))
  expect(openMaasBtn).toBeTruthy()
  await openMaasBtn!.trigger('click')

  const sourceInput = wrapper.find('input[placeholder*="wanjie / doubao"]')
  expect(sourceInput.exists()).toBe(true)
  await sourceInput.setValue('wanjie')

  const urlInput = wrapper.find('input[type="url"]')
  await urlInput.setValue('https://fangzhou.wanjiedata.com/maas/model/myModelList')
  const tokenInput = wrapper.find('input[type="password"]')
  await tokenInput.setValue('test-token')

  await wrapper.find('form#maas-sync-form').trigger('submit')
  await flushPromises()

  // 走统一的 sync-maas 接口，且不触发「从上游导入」等其它同步入口
  expect(mockedSyncModelPricingsFromMaas).toHaveBeenCalledTimes(1)
  expect(mockedSyncModelPricingsFromMaas).toHaveBeenCalledWith(
    expect.objectContaining({
      source: 'wanjie',
      url: 'https://fangzhou.wanjiedata.com/maas/model/myModelList',
      access_token: 'test-token',
    }),
  )
})
