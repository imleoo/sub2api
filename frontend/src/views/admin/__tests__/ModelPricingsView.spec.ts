import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ModelPricingsView from '../ModelPricingsView.vue'
import { listModelPricings, listModelPricingProviders } from '@/api/admin/modelPricings'

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
