import { apiClient } from '../client'

export interface ModelInfo {
  id: string
  provider: string
  mode: string
  input_cost_per_token: number
  output_cost_per_token: number
  supports_prompt_caching: boolean
  discount_rate: number
}

export interface ModelDiscountsData {
  discounts: Record<string, number>
  models: ModelInfo[]
}

export const getModelDiscounts = () =>
  apiClient.get<ModelDiscountsData>('/admin/settings/model-discounts')

export const updateModelDiscounts = (discounts: Record<string, number>) =>
  apiClient.put('/admin/settings/model-discounts', { discounts })
