import { apiClient } from '../client'

export interface DBModelPricing {
  id: number
  model_id: string
  display_name: string | null
  description: string | null
  provider: string
  mode: string
  input_cost_per_token: number | null
  output_cost_per_token: number | null
  cache_creation_input_token_cost: number | null
  cache_read_input_token_cost: number | null
  output_cost_per_image: number | null
  output_cost_per_image_token: number | null
  supports_prompt_caching: boolean
  custom_input_cost: number | null
  custom_output_cost: number | null
  discount_rate: number | null
  is_custom: boolean
  is_enabled: boolean
  last_synced_at: string | null
  created_at: string
  updated_at: string
}

export interface ModelPricingListFilter {
  q?: string
  provider?: string
  is_custom?: boolean
  is_enabled?: boolean
  page?: number
  page_size?: number
}

export interface ModelPricingListResponse {
  items: DBModelPricing[]
  total: number
  page: number
  page_size: number
}

export interface CreateModelPricingRequest {
  model_id: string
  display_name?: string | null
  description?: string | null
  provider: string
  mode: string
  input_cost_per_token?: number | null
  output_cost_per_token?: number | null
  cache_creation_input_token_cost?: number | null
  cache_read_input_token_cost?: number | null
  output_cost_per_image?: number | null
  output_cost_per_image_token?: number | null
  supports_prompt_caching?: boolean
  custom_input_cost?: number | null
  custom_output_cost?: number | null
  discount_rate?: number | null
  is_enabled?: boolean
}

export const listModelPricings = (filter?: ModelPricingListFilter) =>
  apiClient.get<ModelPricingListResponse>('/admin/model-pricings', { params: filter })

export const createModelPricing = (data: CreateModelPricingRequest) =>
  apiClient.post<DBModelPricing>('/admin/model-pricings', data)

export const updateModelPricing = (id: number, data: Partial<CreateModelPricingRequest>) =>
  apiClient.put<DBModelPricing>(`/admin/model-pricings/${id}`, data)

export const deleteModelPricing = (id: number) =>
  apiClient.delete(`/admin/model-pricings/${id}`)

export const triggerModelPricingSync = () =>
  apiClient.post('/admin/model-pricings/sync')

export interface SyncFromUpstreamRequest {
  base_url: string
  api_key: string
  auth_header?: string
  auth_scheme?: string
  provider?: string
  mode?: string
}

export interface SyncFromUpstreamResponse {
  models: string[]
  fetched: number
}

export const syncModelPricingsFromUpstream = (data: SyncFromUpstreamRequest) =>
  apiClient.post<SyncFromUpstreamResponse>('/admin/model-pricings/sync-from-upstream', data)

export interface SyncFromWanjieRequest {
  url?: string
  access_token?: string
  save_credentials?: boolean
  json_data?: string
}

export interface SyncFromWanjieResponse {
  message: string
  total: number
  source: string
}

export const syncModelPricingsFromWanjie = (data: SyncFromWanjieRequest) =>
  apiClient.post<SyncFromWanjieResponse>('/admin/model-pricings/sync-from-wanjie', data)

export const clearAllModelPricingDiscounts = () =>
  apiClient.post('/admin/model-pricings/clear-discounts')
