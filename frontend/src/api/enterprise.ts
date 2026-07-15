/**
 * Enterprise self-service upgrade endpoints (Team 协作 v2)
 * 企业组织与额度分配（zhiguofan fork-only）
 */

import { apiClient } from './client'

export interface EnterpriseProfile {
  company_name: string
  contact_name: string
  contact_phone: string
  industry: string
  created_at: string
}

export interface UpgradeEnterpriseRequest {
  company_name: string
  contact_name?: string
  contact_phone?: string
  industry?: string
}

/** Self-service upgrade to an enterprise account; takes effect immediately. */
export async function upgrade(req: UpgradeEnterpriseRequest): Promise<EnterpriseProfile> {
  const { data } = await apiClient.post<EnterpriseProfile>('/enterprise/profile', req)
  return data
}

/** Get the current user's enterprise profile; null when not an enterprise account. */
export async function getProfile(): Promise<EnterpriseProfile | null> {
  const { data } = await apiClient.get<EnterpriseProfile | null>('/enterprise/profile')
  return data
}

export const enterpriseAPI = {
  upgrade,
  getProfile
}

export default enterpriseAPI
