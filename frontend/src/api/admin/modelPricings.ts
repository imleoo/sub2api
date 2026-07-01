import { apiClient } from '../client'

export interface DBModelPricing {
  id: number
  model_id: string
  display_name: string | null
  description: string | null
  provider: string
  mode: string
  pricing_unit: 'token' | 'second' | 'image_generation' | 'video_generation'
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
  pricing_health?: PricingHealth
  last_synced_at: string | null
  created_at: string
  updated_at: string
}

// pricing_health 派生状态（后端 mode-aware 计算），ok 以外置顶高亮。
export type PricingHealth =
  | 'ok'
  | 'missing_pricing'
  | 'unit_contamination'
  | 'orphan_no_active_account'

export interface ModelPricingListFilter {
  q?: string
  provider?: string
  is_custom?: boolean
  is_enabled?: boolean
  visible_only?: boolean
  // 跳过后端默认的「广场可路由集」交集过滤，用于账号编辑弹窗的模型白名单选择器——
  // 该场景要展示的正是「已同步定价数据、但还没被任何账号引用过」的模型，套用广场口径
  // 会导致新模型永远搜不到（循环依赖）。仅供 ModelWhitelistSelector.vue 使用。
  for_whitelist?: boolean
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
  pricing_unit?: 'token' | 'second' | 'image_generation' | 'video_generation'
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

// 通用 MaaS 定价同步：价格完全由运营管理，不内置任何厂商价。
// source 任意标识（wanjie/doubao/lingjing/minimax/glm…），凭证按 source 维度保存。
// 二选一：上传 json_data（万界 API 响应格式），或填 url+access_token 在线拉取。
export interface SyncMaasRequest {
  source: string
  url?: string
  access_token?: string
  save_credentials?: boolean
  json_data?: string
}

export interface SyncMaasResponse {
  message: string
  total: number
  source: string
  mode: string
}

export const syncModelPricingsFromMaas = (data: SyncMaasRequest) =>
  apiClient.post<SyncMaasResponse>('/admin/model-pricings/sync-maas', data)

export const clearAllModelPricingDiscounts = () =>
  apiClient.post('/admin/model-pricings/clear-discounts')

export interface ListProvidersResponse {
  providers: string[]
}

export const listModelPricingProviders = () =>
  apiClient.get<ListProvidersResponse>('/admin/model-pricings/providers')
