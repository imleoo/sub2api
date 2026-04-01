/**
 * Admin Prompt Analytics API endpoints
 * Fetches keyword statistics for the word cloud feature.
 */

import { apiClient } from '../client'

export interface KeywordCount {
  keyword: string
  count: number
}

export interface TopKeywordsResponse {
  period: string
  user_id?: number
  keywords: KeywordCount[]
}

/**
 * Get top keywords for the word cloud
 * @param options.userId  - (optional) filter by user ID; omit for global
 * @param options.period  - (optional) year-month string, e.g. "2024-01"
 * @param options.limit   - (optional) max keywords to return (default 50)
 */
export async function getTopKeywords(options?: {
  userId?: number
  period?: string
  limit?: number
}): Promise<TopKeywordsResponse> {
  const params: Record<string, any> = {}
  if (options?.userId) params.user_id = options.userId
  if (options?.period) params.period = options.period
  if (options?.limit) params.limit = options.limit

  const { data } = await apiClient.get<TopKeywordsResponse>(
    '/admin/prompt-analytics/top-keywords',
    { params }
  )
  return data
}

export const promptAnalyticsAPI = {
  getTopKeywords
}

export default promptAnalyticsAPI
