import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import EndpointDistributionChart from '../EndpointDistributionChart.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('vue-chartjs', () => ({
  Doughnut: {
    props: ['data'],
    template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>',
  },
}))

vi.mock('@/api/admin/dashboard', () => ({
  getUserBreakdown: vi.fn(),
}))

describe('EndpointDistributionChart', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  const endpointStats = [
    {
      endpoint: '/v1/messages',
      requests: 4,
      total_tokens: 1200,
      cost: 1,
      actual_cost: 0.5,
    },
  ]

  it('formats actual cost in CNY mode', () => {
    const appStore = useAppStore()
    appStore.currencyMode = 'cny'
    appStore.cnyRate = 7.2

    const wrapper = mount(EndpointDistributionChart, {
      props: {
        endpointStats,
        metric: 'actual_cost',
      },
      global: {
        stubs: {
          LoadingSpinner: true,
          UserBreakdownSubTable: true,
        },
      },
    })

    expect(wrapper.text()).toContain('¥3.60')

    const options = (wrapper.vm as any).$?.setupState.doughnutOptions
    const label = options.plugins.tooltip.callbacks.label({
      label: '/v1/messages',
      raw: 0.5,
      dataset: { data: [0.5] },
    })
    expect(label).toBe('/v1/messages: ¥3.60 (100.0%)')
  })
})
