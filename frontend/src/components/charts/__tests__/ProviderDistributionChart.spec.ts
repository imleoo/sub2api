import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import ProviderDistributionChart from '../ProviderDistributionChart.vue'

const messages: Record<string, string> = {
  'admin.dashboard.providerDistribution': 'Provider Distribution',
  'admin.dashboard.provider': 'Provider',
  'admin.dashboard.providersEmpty': 'No provider data',
  'admin.dashboard.lastNDays': 'Last {n} days',
  'admin.dashboard.total': 'Total',
  'admin.dashboard.requests': 'Requests',
  'admin.dashboard.tokens': 'Tokens',
  'admin.dashboard.actual': 'Actual',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        let raw = messages[key] ?? key
        if (params) {
          for (const [k, v] of Object.entries(params)) {
            raw = raw.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v))
          }
        }
        return raw
      },
    }),
  }
})

// Mock the api module called by ProviderDistributionChart
const mockGetStatsByProvider = vi.fn()
vi.mock('@/api/admin/accounts', () => ({
  getStatsByProvider: (...args: unknown[]) => mockGetStatsByProvider(...args),
}))

function makeStats(requests: number, tokens: number, userCost: number) {
  return {
    history: [],
    summary: {
      days: 30,
      actual_days_used: 1,
      total_cost: userCost,
      total_user_cost: userCost,
      total_standard_cost: userCost,
      total_requests: requests,
      total_tokens: tokens,
      avg_daily_cost: 0,
      avg_daily_user_cost: 0,
      avg_daily_requests: 0,
      avg_daily_tokens: 0,
      avg_duration_ms: 0,
    },
    models: [],
    endpoints: [],
    upstream_endpoints: [],
  }
}

describe('ProviderDistributionChart', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockGetStatsByProvider.mockReset()
  })

  it('aggregates per-provider rows and totals after fetch (P1-4 验收)', async () => {
    mockGetStatsByProvider
      .mockResolvedValueOnce(makeStats(10, 1000, 0.5)) // anthropic
      .mockResolvedValueOnce(makeStats(20, 2000, 1.5)) // deepseek

    const wrapper = mount(ProviderDistributionChart, {
      props: {
        providers: ['anthropic', 'deepseek'],
        days: 30,
      },
    })

    await flushPromises()

    // 2 行 + 1 行 tfoot
    const bodyRows = wrapper.findAll('tbody tr')
    expect(bodyRows).toHaveLength(2)
    expect(bodyRows[0].text()).toContain('anthropic')
    expect(bodyRows[1].text()).toContain('deepseek')

    // 合计行
    const footRow = wrapper.find('tfoot tr')
    expect(footRow.exists()).toBe(true)
    expect(footRow.text()).toContain('Total')
    // 30 requests = 10 + 20
    expect(footRow.text()).toContain('30')
    // 3000 tokens = 1000 + 2000
    expect(footRow.text()).toContain('3,000')

    // API 必须被规范化 provider_key 调用（验收）
    expect(mockGetStatsByProvider).toHaveBeenCalledWith('anthropic', 30)
    expect(mockGetStatsByProvider).toHaveBeenCalledWith('deepseek', 30)
  })

  it('shows empty state when no providers configured', () => {
    const wrapper = mount(ProviderDistributionChart, {
      props: { providers: [], days: 30 },
    })
    expect(wrapper.text()).toContain('No provider data')
  })

  it('shows error indicator on API failure', async () => {
    mockGetStatsByProvider.mockRejectedValueOnce(new Error('boom'))

    const wrapper = mount(ProviderDistributionChart, {
      props: { providers: ['anthropic'], days: 30 },
    })

    await flushPromises()
    const bodyRow = wrapper.find('tbody tr')
    expect(bodyRow.text()).toContain('!')
  })
})
