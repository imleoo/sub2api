/**
 * Models API - 用户端模型列表接口
 */

import { apiClient } from './client'

export interface ModelInfo {
  id: string
  provider: string
  mode: string
  input_cost_per_token: number
  output_cost_per_token: number
  supports_prompt_caching: boolean
  long_context_input_token_threshold?: number
}

export interface ModelsResponse {
  models: ModelInfo[]
  total: number
}

/**
 * 获取当前用户可见的全部模型列表（含定价信息）
 * GET /api/v1/models
 */
export async function getModels(): Promise<ModelsResponse> {
  const { data } = await apiClient.get<ModelsResponse>('/models')
  return data ?? { models: [], total: 0 }
}

export const modelsAPI = { getModels }
export default modelsAPI
