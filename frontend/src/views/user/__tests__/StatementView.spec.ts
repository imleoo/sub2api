import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import StatementView from '../StatementView.vue'
import { getStatement, getStatementMonths, exportStatement } from '@/api/statement'
import type { StatementResponse } from '@/api/statement'

vi.mock('vue-i18n', async importOriginal => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key}:${JSON.stringify(params)}` : key,
  }),
}))

// statementAPI 命名空间在组件中使用，命名导出与命名空间指向同一组 mock 函数
vi.mock('@/api/statement', async () => {
  const getStatement = vi.fn()
  const getStatementMonths = vi.fn()
  const exportStatement = vi.fn()
  return {
    getStatement,
    getStatementMonths,
    exportStatement,
    statementAPI: { getStatement, getStatementMonths, exportStatement },
  }
})

const mockedGetStatement = vi.mocked(getStatement)
const mockedGetMonths = vi.mocked(getStatementMonths)
const mockedExport = vi.mocked(exportStatement)

function sampleStatement(overrides: Partial<StatementResponse> = {}): StatementResponse {
  return {
    period: '2026-06',
    timezone: 'UTC',
    source: 'snapshot',
    closed: true,
    opening_balance: 1000,
    closing_balance: 4410,
    rows: [
      { date: '2026-06-01', nature: 'opening', running_total: 1000 },
      { date: '2026-06-01', nature: 'deposit', amount: 5000, note: 'order A', running_total: 6000 },
      {
        date: '2026-06-02',
        nature: 'utilisation',
        qty: -20000,
        unit_price: 0.05,
        cost_before: 1000,
        discount_rate: 0.4,
        cost_after: 600,
        running_total: 5400,
      },
      { date: '2026-06-04', nature: 'withdraw', amount: 1000, note: 'refund', running_total: 4400 },
      { date: '2026-06-06', nature: 'credit', amount: 10, note: 'redeem', running_total: 4410 },
      { date: '2026-06-30', nature: 'closing', running_total: 4410 },
    ],
    totals: {
      deposit: 5000,
      withdraw: 1000,
      credit: 10,
      utilisation_before: 1000,
      utilisation_after: 600,
      identity_gap: 0,
    },
    ...overrides,
  }
}

async function mountView() {
  const wrapper = mount(StatementView, {
    global: {
      stubs: {
        DataTable: {
          template: '<div data-testid="data-table"><slot v-for="row in data" name="cell-nature" :row="row" /></div>',
          props: ['columns', 'data', 'loading', 'rowKey', 'virtualizeThreshold'],
        },
        Icon: true,
      },
    },
  })
  await flushPromises()
  return wrapper
}

describe('StatementView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    mockedGetMonths.mockResolvedValue(['2026-06', '2026-05'])
    mockedGetStatement.mockResolvedValue(sampleStatement())
  })

  it('加载月份列表并自动选中最近月、拉取对账单', async () => {
    const wrapper = await mountView()
    expect(mockedGetMonths).toHaveBeenCalledOnce()
    expect(mockedGetStatement).toHaveBeenCalledWith('2026-06', expect.any(String))
    const select = wrapper.find('[data-testid="statement-month-select"]')
    expect((select.element as HTMLSelectElement).value).toBe('2026-06')
  })

  it('渲染汇总卡片（期初/期末/入账/支出）', async () => {
    const wrapper = await mountView()
    const text = wrapper.text()
    expect(text).toContain('1,000.00') // opening
    expect(text).toContain('4,410.00') // closing
    expect(text).toContain('5,010.00') // in = 5000 + 10
    expect(text).toContain('1,600.00') // out = 1000 + 600
  })

  it('未封账月显示徽标', async () => {
    mockedGetStatement.mockResolvedValue(sampleStatement({ closed: false, source: 'computed' }))
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('statement.notClosed')
    expect(wrapper.text()).toContain('statement.computed')
  })

  it('恒等式差额非 0 时提示', async () => {
    mockedGetStatement.mockResolvedValue(
      sampleStatement({
        totals: {
          deposit: 0,
          withdraw: 0,
          credit: 0,
          utilisation_before: 0,
          utilisation_after: 0,
          identity_gap: 12.5,
        },
      })
    )
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('statement.identityGapNote')
  })

  it('点击导出触发 blob 下载', async () => {
    mockedExport.mockResolvedValue(new Blob(['x']))
    const createObjectURL = vi.fn(() => 'blob:url')
    const revokeObjectURL = vi.fn()
    vi.stubGlobal('URL', { createObjectURL, revokeObjectURL })

    const wrapper = await mountView()
    await wrapper.find('[data-testid="statement-export-btn"]').trigger('click')
    await flushPromises()

    expect(mockedExport).toHaveBeenCalledWith('2026-06', expect.any(String))
    expect(createObjectURL).toHaveBeenCalledOnce()
    expect(revokeObjectURL).toHaveBeenCalledOnce()
    vi.unstubAllGlobals()
  })

  it('切换月份重新拉取', async () => {
    const wrapper = await mountView()
    mockedGetStatement.mockClear()
    await wrapper.find('[data-testid="statement-month-select"]').setValue('2026-05')
    await flushPromises()
    expect(mockedGetStatement).toHaveBeenCalledWith('2026-05', expect.any(String))
  })
})
