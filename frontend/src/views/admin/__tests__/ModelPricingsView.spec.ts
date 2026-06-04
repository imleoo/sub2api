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
  syncModelPricingsFromWanjie: vi.fn(),
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

  it('loads visible models by default and refetches all models when toggled off', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(mockedListModelPricings).toHaveBeenLastCalledWith(expect.objectContaining({
      visible_only: true,
      page: 1,
      page_size: 20,
    }))

    const visibleOnly = wrapper.find('input[type="checkbox"]')
    await visibleOnly.setValue(false)
    await flushPromises()

    expect(mockedListModelPricings).toHaveBeenLastCalledWith(expect.objectContaining({
      visible_only: false,
      page: 1,
      page_size: 20,
    }))
  })
})
