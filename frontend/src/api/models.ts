/**
 * Models API - 用户端模型列表接口
 */

import { apiClient } from './client'

export interface ModelTestStatus {
  model_id: string
  status: string
  latency_ms: number
  finished_at: string | null
}

export interface ModelInfo {
  id: string
  provider: string
  mode: string
  input_cost_per_token: number
  output_cost_per_token: number
  supports_prompt_caching: boolean
  long_context_input_token_threshold?: number
  is_available: boolean
  test_status?: ModelTestStatus
  discount_rate?: number
}

export interface ModelsResponse {
  models: ModelInfo[]
  total: number
  available_platforms: string[]
  cny_rate?: number
  currency_mode?: string
}

/**
 * 获取当前用户可见的全部模型列表（含定价信息和可用性）
 * GET /api/v1/models
 */
export async function getModels(): Promise<ModelsResponse> {
  const { data } = await apiClient.get<ModelsResponse>('/models')
  return data ?? { models: [], total: 0, available_platforms: [] }
}

export const modelsAPI = { getModels }
export default modelsAPI
