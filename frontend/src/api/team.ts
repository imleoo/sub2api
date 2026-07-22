/**
 * Enterprise team endpoints (Team 协作 v2)
 * 企业组织与额度分配（zhiguofan fork-only）
 *
 * v2 不再有 v1 那种全局 X-Team-Id 请求头切换身份（会连带影响 Key/支付/用量归属）。
 * 员工自管 Key，但企业允许设置多个 admin：一个 admin 可能同时管理自己创建的企业
 * 和加入的别人的企业，因此这组接口显式支持可选的 ownerUserId 参数（转成
 * ?owner_user_id= 查询参数），不传时后端默认操作调用者自己创建的企业。
 */

import { apiClient } from './client'
import type { PlatformUsage } from './admin/dashboard'
import type { AccountUsageStatsResponse } from '@/types'

export interface TeamSummary {
  owner_user_id: number
  owner_email: string
  is_personal: boolean
  role: string
  balance: number
  frozen_balance: number
}

export interface TeamInvitation {
  id: number
  invited_email: string
  status: string
  department_id?: number | null
  role: string
  quota_mode: string
  initial_grant_usd?: number | null
  expires_at: string
  created_at: string
}

export interface TeamReceivedInvitation {
  id: number
  owner_user_id: number
  owner_email: string
  role: string
  quota_mode: string
  initial_grant_usd?: number | null
  token: string
  expires_at: string
  created_at: string
}

export interface InviteMemberRequest {
  email: string
  department_id?: number | null
  role?: string
  quota_mode?: string
  initial_grant_usd?: number | null
}

export interface TeamMember {
  user_id: number
  email: string
  role: string
  department_id?: number | null
  quota_mode?: string
  granted_net_usd: number
  joined_at?: string
}

export interface TeamDepartment {
  id: number
  name: string
  display_order: number
}

export interface TeamFundTransfer {
  id: number
  member_user_id: number
  member_email?: string
  direction: 'grant' | 'reclaim' | 'auto_topup'
  amount: number
  operator_user_id: number
  operator_email?: string
  note: string
  created_at: string
}

export interface TeamFundTransferPage {
  items: TeamFundTransfer[]
  total: number
}

export interface TeamReportRow {
  user_id: number
  email: string
  role: string
  department_id?: number | null
  quota_mode?: string
  balance: number
  granted_net_usd: number
  cost_30d: number
  today_cost: number
  by_platform?: PlatformUsage[]
}

export interface SetQuotaSettingsRequest {
  quota_mode: string
  auto_topup_threshold_usd?: number | null
  auto_topup_target_usd?: number | null
}

function ownerParams(ownerUserId?: number) {
  return ownerUserId ? { owner_user_id: ownerUserId } : undefined
}

/** List accounts the current user can operate: personal + enterprises joined as a member. */
export async function listMyTeams(): Promise<TeamSummary[]> {
  const { data } = await apiClient.get<TeamSummary[]>('/teams')
  return data
}

/** Invite an employee by email (owner/admin). Works even if the email isn't registered yet. */
export async function inviteMember(req: InviteMemberRequest, ownerUserId?: number): Promise<TeamInvitation> {
  const { data } = await apiClient.post<TeamInvitation>('/team/invitations', req, { params: ownerParams(ownerUserId) })
  return data
}

/** List pending invitations for the current enterprise (owner/admin). */
export async function listInvitations(ownerUserId?: number): Promise<TeamInvitation[]> {
  const { data } = await apiClient.get<TeamInvitation[]>('/team/invitations', { params: ownerParams(ownerUserId) })
  return data
}

/**
 * List pending invitations sent to the current user's email (invitee view).
 * Lets a registered user see and accept invitations in-app without relying
 * on the invitation email being delivered.
 */
export async function listReceivedInvitations(): Promise<TeamReceivedInvitation[]> {
  const { data } = await apiClient.get<TeamReceivedInvitation[]>('/team/invitations/received')
  return data
}

/** Revoke a pending invitation (owner/admin). */
export async function revokeInvitation(id: number, ownerUserId?: number): Promise<{ revoked: boolean }> {
  const { data } = await apiClient.delete<{ revoked: boolean }>(`/team/invitations/${id}`, { params: ownerParams(ownerUserId) })
  return data
}

export async function resendInvitation(id: number, ownerUserId?: number): Promise<{ resent: boolean }> {
  const { data } = await apiClient.post<{ resent: boolean }>(`/team/invitations/${id}/resend`, undefined, { params: ownerParams(ownerUserId) })
  return data
}

/** Accept an invitation by token (the invited email must match the current login). */
export async function acceptInvitation(token: string): Promise<{ owner_user_id: number; role: string }> {
  const { data } = await apiClient.post<{ owner_user_id: number; role: string }>(
    `/team/invitations/accept/${encodeURIComponent(token)}`
  )
  return data
}

/** List members of the current enterprise (owner + active members). */
export async function listMembers(ownerUserId?: number): Promise<TeamMember[]> {
  const { data } = await apiClient.get<TeamMember[]>('/team/members', { params: ownerParams(ownerUserId) })
  return data
}

/** Remove an employee (owner/admin; admin cannot remove another admin). */
export async function removeMember(memberUserId: number, ownerUserId?: number): Promise<{ removed: boolean }> {
  const { data } = await apiClient.delete<{ removed: boolean }>(`/team/members/${memberUserId}`, { params: ownerParams(ownerUserId) })
  return data
}

export async function setMemberDepartment(memberUserId: number, departmentId: number | null, ownerUserId?: number): Promise<void> {
  await apiClient.put(`/team/members/${memberUserId}/department`, { department_id: departmentId }, { params: ownerParams(ownerUserId) })
}

export async function setMemberQuotaSettings(memberUserId: number, req: SetQuotaSettingsRequest, ownerUserId?: number): Promise<void> {
  await apiClient.put(`/team/members/${memberUserId}/quota-settings`, req, { params: ownerParams(ownerUserId) })
}

/** Promote/demote a member's role (owner-only). */
export async function setMemberRole(memberUserId: number, role: string, ownerUserId?: number): Promise<void> {
  await apiClient.put(`/team/members/${memberUserId}/role`, { role }, { params: ownerParams(ownerUserId) })
}

export async function grantToMember(memberUserId: number, amount: number, note?: string, ownerUserId?: number): Promise<TeamFundTransfer> {
  const { data } = await apiClient.post<TeamFundTransfer>(`/team/members/${memberUserId}/grant`, { amount, note }, { params: ownerParams(ownerUserId) })
  return data
}

export async function reclaimFromMember(memberUserId: number, amount: number, note?: string, ownerUserId?: number): Promise<TeamFundTransfer> {
  const { data } = await apiClient.post<TeamFundTransfer>(`/team/members/${memberUserId}/reclaim`, { amount, note }, { params: ownerParams(ownerUserId) })
  return data
}

export async function listTransfers(page = 1, pageSize = 20, ownerUserId?: number): Promise<TeamFundTransferPage> {
  const { data } = await apiClient.get<TeamFundTransferPage>('/team/transfers', {
    params: { page, page_size: pageSize, ...ownerParams(ownerUserId) }
  })
  return data
}

export async function getReport(ownerUserId?: number): Promise<TeamReportRow[]> {
  const { data } = await apiClient.get<TeamReportRow[]>('/team/report', { params: ownerParams(ownerUserId) })
  return data
}

/**
 * Detailed usage statistics for one member — same underlying data as the admin
 * "user management" usage stats popup, scoped by team permission instead of
 * platform-admin permission (owner/admin can view the enterprise's own members).
 */
export async function getMemberUsageStats(memberUserId: number, days = 30, ownerUserId?: number): Promise<AccountUsageStatsResponse> {
  const { data } = await apiClient.get<AccountUsageStatsResponse>(`/team/members/${memberUserId}/usage-stats`, {
    params: { days, ...ownerParams(ownerUserId) }
  })
  return data
}

export async function listDepartments(ownerUserId?: number): Promise<TeamDepartment[]> {
  const { data } = await apiClient.get<TeamDepartment[]>('/team/departments', { params: ownerParams(ownerUserId) })
  return data
}

export async function createDepartment(name: string, displayOrder = 0, ownerUserId?: number): Promise<TeamDepartment> {
  const { data } = await apiClient.post<TeamDepartment>('/team/departments', { name, display_order: displayOrder }, { params: ownerParams(ownerUserId) })
  return data
}

export async function updateDepartment(id: number, name: string, displayOrder = 0, ownerUserId?: number): Promise<void> {
  await apiClient.put(`/team/departments/${id}`, { name, display_order: displayOrder }, { params: ownerParams(ownerUserId) })
}

export async function deleteDepartment(id: number, ownerUserId?: number): Promise<void> {
  await apiClient.delete(`/team/departments/${id}`, { params: ownerParams(ownerUserId) })
}

export const teamAPI = {
  listMyTeams,
  inviteMember,
  listInvitations,
  listReceivedInvitations,
  revokeInvitation,
  resendInvitation,
  acceptInvitation,
  listMembers,
  removeMember,
  setMemberDepartment,
  setMemberQuotaSettings,
  setMemberRole,
  grantToMember,
  reclaimFromMember,
  listTransfers,
  getReport,
  getMemberUsageStats,
  listDepartments,
  createDepartment,
  updateDepartment,
  deleteDepartment
}

export default teamAPI
