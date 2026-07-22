import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { AccountUsageStatsResponse } from '@/types'

const apiMocks = vi.hoisted(() => ({
  getUserUsageStats: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      getUserUsageStats: apiMocks.getUserUsageStats,
    },
  },
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

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    name: 'BaseDialog',
    props: ['show', 'title', 'width'],
    template: '<div v-if="show"><slot /><slot name="footer" /></div>',
  },
}))

// UserStatsModal 内部渲染 chart.js 图表，jsdom 没有 canvas 2d context，
// stub 掉图表相关子组件，只验证 fetchStats prop 的调用行为。
vi.mock('vue-chartjs', () => ({
  Line: { name: 'LineStub', template: '<div />' },
}))

vi.mock('@/components/charts/ModelDistributionChart.vue', () => ({
  default: { name: 'ModelDistributionChartStub', props: ['modelStats', 'loading'], template: '<div />' },
}))

vi.mock('@/components/charts/EndpointDistributionChart.vue', () => ({
  default: {
    name: 'EndpointDistributionChartStub',
    props: ['endpointStats', 'loading', 'title'],
    template: '<div />',
  },
}))

import UserStatsModal from '../UserStatsModal.vue'

function makeStats(): AccountUsageStatsResponse {
  return {
    history: [],
    summary: {
      days: 30,
      actual_days_used: 1,
      total_cost: 1,
      total_user_cost: 1,
      total_standard_cost: 1,
      total_requests: 1,
      total_tokens: 1,
      avg_daily_cost: 1,
      avg_daily_user_cost: 1,
      avg_daily_requests: 1,
      avg_daily_tokens: 1,
      avg_duration_ms: 1,
      today: { date: '2026-07-21', cost: 0, user_cost: 0, requests: 0, tokens: 0 },
      highest_cost_day: null,
      highest_request_day: null,
    } as AccountUsageStatsResponse['summary'],
    models: [],
    endpoints: [],
    upstream_endpoints: [],
  }
}

const user = { id: 42, email: 'u@example.com', status: 'active' }

describe('UserStatsModal fetchStats prop', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    apiMocks.getUserUsageStats.mockResolvedValue(makeStats())
  })

  it('不传 fetchStats 时默认调用 adminAPI.users.getUserUsageStats(userId, 30)', async () => {
    const w = mount(UserStatsModal, {
      props: { show: false, user },
    })
    await w.setProps({ show: true })
    await flushPromises()

    expect(apiMocks.getUserUsageStats).toHaveBeenCalledTimes(1)
    expect(apiMocks.getUserUsageStats).toHaveBeenCalledWith(42, 30)
  })

  it('传入自定义 fetchStats 时优先调用自定义函数并透传正确入参，不再调用默认 API', async () => {
    const customFetch = vi.fn().mockResolvedValue(makeStats())
    const w = mount(UserStatsModal, {
      props: { show: false, user, fetchStats: customFetch },
    })
    await w.setProps({ show: true })
    await flushPromises()

    expect(customFetch).toHaveBeenCalledTimes(1)
    expect(customFetch).toHaveBeenCalledWith(42, 30)
    expect(apiMocks.getUserUsageStats).not.toHaveBeenCalled()
  })

  it('关闭弹窗（show=false）不触发任何 fetchStats 调用', async () => {
    mount(UserStatsModal, {
      props: { show: false, user },
    })
    await flushPromises()

    expect(apiMocks.getUserUsageStats).not.toHaveBeenCalled()
  })
})
