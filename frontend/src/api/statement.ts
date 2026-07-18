/**
 * Monthly statement (Vendor Report) API endpoints
 * zhiguofan fork-only: 月度对账（功能 45）
 */

import { apiClient } from './client'

export type StatementNature =
  | 'opening'
  | 'deposit'
  | 'withdraw'
  | 'credit'
  | 'utilisation'
  | 'closing'

export interface StatementRow {
  date: string
  nature: StatementNature
  amount?: number
  note?: string
  qty?: number
  unit_price?: number
  cost_before?: number
  discount_rate?: number
  cost_after?: number
  running_total: number
}

export interface StatementTotals {
  deposit: number
  withdraw: number
  credit: number
  utilisation_before: number
  utilisation_after: number
  identity_gap: number
}

export interface StatementResponse {
  period: string
  timezone: string
  source: 'snapshot' | 'computed'
  closed: boolean
  opening_balance: number
  closing_balance: number
  rows: StatementRow[]
  totals: StatementTotals
}

/** 获取指定月份对账单（month: YYYY-MM） */
export async function getStatement(month: string, timezone?: string): Promise<StatementResponse> {
  const { data } = await apiClient.get<StatementResponse>('/usage/statement', {
    params: { month, timezone }
  })
  return data
}

/** 获取可查询月份列表（倒序，最近在前） */
export async function getStatementMonths(): Promise<string[]> {
  const { data } = await apiClient.get<string[]>('/usage/statement/months')
  return data
}

/** 导出对账单 Excel（blob） */
export async function exportStatement(month: string, timezone?: string): Promise<Blob> {
  const response = await apiClient.get('/usage/statement/export', {
    params: { month, timezone },
    responseType: 'blob'
  })
  return response.data
}

export const statementAPI = {
  getStatement,
  getStatementMonths,
  exportStatement
}

export default statementAPI
