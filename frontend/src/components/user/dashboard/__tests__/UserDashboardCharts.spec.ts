import { describe, expect, it, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import UserDashboardCharts from '../UserDashboardCharts.vue'

vi.mock('vue-chartjs', () => ({
  Doughnut: { template: '<div />' },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('UserDashboardCharts currency display', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('formats model distribution costs as CNY when currency mode is cny', () => {
    const appStore = useAppStore()
    appStore.currencyMode = 'cny'
    appStore.cnyRate = 7.2

    const wrapper = mount(UserDashboardCharts, {
      props: {
        loading: false,
        startDate: '2026-05-01',
        endDate: '2026-05-10',
        granularity: 'day',
        trend: [],
        models: [
          {
            model: 'gpt-test',
            requests: 10,
            input_tokens: 100,
            output_tokens: 50,
            total_tokens: 150,
            cost: 2,
            actual_cost: 1.5,
          },
        ],
      },
      global: {
        stubs: {
          DateRangePicker: true,
          Select: true,
          LoadingSpinner: true,
          TokenUsageTrend: true,
        },
      },
    })

    const text = wrapper.text()
    expect(text).toContain('¥10.8000')
    expect(text).toContain('¥14.4000')
    expect(text).not.toContain('$1.5000')
    expect(text).not.toContain('$2.0000')
  })
})
