import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import AccountDistributionChart from '../AccountDistributionChart.vue'
import type { Account } from '@/types'

const messages: Record<string, string> = {
  'admin.dashboard.accountDistribution': 'Account Distribution',
  'admin.dashboard.accountsEmpty': 'No accounts',
  'admin.dashboard.account': 'Account',
  'admin.dashboard.coveredGroups': 'Covered groups',
  'admin.dashboard.coveredGroupsHint': 'Account is attached to {n} groups',
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

const mockGetStatsCrossGroup = vi.fn()
vi.mock('@/api/admin/accounts', () => ({
  getStatsCrossGroup: (...args: unknown[]) => mockGetStatsCrossGroup(...args),
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

const fakeAccount = (id: number, name: string, groupIDs: number[]): Account =>
  ({
    id,
    name,
    group_ids: groupIDs,
  }) as unknown as Account

describe('AccountDistributionChart', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockGetStatsCrossGroup.mockReset()
  })

  it('shows covered groups count from account.group_ids (P1-4 验收 §5.4 第 10 条)', async () => {
    mockGetStatsCrossGroup.mockResolvedValueOnce(makeStats(15, 1500, 0.75))

    const accountWithTwoGroups = fakeAccount(42, 'deepseek-key-1', [101, 202])
    const wrapper = mount(AccountDistributionChart, {
      props: { accounts: [accountWithTwoGroups], days: 30 },
    })

    await flushPromises()

    const bodyRow = wrapper.find('tbody tr')
    expect(bodyRow.text()).toContain('deepseek-key-1')
    // 覆盖分组数 = 2（绿色 badge）
    expect(bodyRow.text()).toContain('2')

    // 调用了 cross-group API（不是普通 getStats）
    expect(mockGetStatsCrossGroup).toHaveBeenCalledWith(42, 30)
  })

  it('handles account with zero groups gracefully', async () => {
    mockGetStatsCrossGroup.mockResolvedValueOnce(makeStats(0, 0, 0))

    const orphanAccount = fakeAccount(99, 'no-group-account', [])
    const wrapper = mount(AccountDistributionChart, {
      props: { accounts: [orphanAccount], days: 30 },
    })

    await flushPromises()
    const bodyRow = wrapper.find('tbody tr')
    // 0 groups 显示为 "0"，不应崩溃
    expect(bodyRow.text()).toContain('0')
  })

  it('shows empty state when no accounts provided', () => {
    const wrapper = mount(AccountDistributionChart, {
      props: { accounts: [], days: 30 },
    })
    expect(wrapper.text()).toContain('No accounts')
  })

  it('aggregates total requests across accounts', async () => {
    mockGetStatsCrossGroup
      .mockResolvedValueOnce(makeStats(10, 1000, 0.5))
      .mockResolvedValueOnce(makeStats(20, 2000, 1.5))

    const wrapper = mount(AccountDistributionChart, {
      props: {
        accounts: [fakeAccount(1, 'a', [1]), fakeAccount(2, 'b', [2])],
        days: 30,
      },
    })

    await flushPromises()
    const footRow = wrapper.find('tfoot tr')
    expect(footRow.exists()).toBe(true)
    // 30 requests = 10 + 20
    expect(footRow.text()).toContain('30')
  })
})
