<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="card p-6">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('team.members.title') }}</h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('team.members.description') }}</p>
          </div>
          <div v-if="managementTargets.length > 1" class="flex items-center gap-2">
            <label class="text-sm text-gray-500 dark:text-dark-400">{{ t('team.orgSwitcher.label') }}</label>
            <select v-model.number="selectedOwnerId" class="input" @change="loadAll">
              <option v-for="opt in managementTargets" :key="opt.ownerUserId" :value="opt.ownerUserId">
                {{ opt.label }}
              </option>
            </select>
          </div>
        </div>
      </div>

      <div v-if="errorMessage" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-600 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400">
        {{ errorMessage }}
      </div>
      <div v-if="successMessage" class="rounded-lg border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-600 dark:border-green-800 dark:bg-green-900/20 dark:text-green-400">
        {{ successMessage }}
      </div>

      <!-- 我收到的邀请：已注册用户不依赖邮件即可站内接受 -->
      <div v-if="receivedInvitations.length > 0" class="card p-6">
        <h2 class="text-base font-medium text-gray-900 dark:text-white">{{ t('team.members.receivedTitle') }}</h2>
        <ul class="mt-4 divide-y divide-gray-100 dark:divide-dark-700">
          <li v-for="inv in receivedInvitations" :key="inv.id" class="flex flex-wrap items-center justify-between gap-3 py-3">
            <div>
              <p class="text-sm font-medium text-gray-900 dark:text-white">
                {{ t('team.members.receivedFrom', { email: inv.owner_email }) }}
              </p>
              <p class="text-xs text-gray-400">
                {{ roleLabel(inv.role) }} · {{ t('team.members.expiresAt') }}: {{ formatDate(inv.expires_at) }}
              </p>
            </div>
            <button class="btn btn-primary btn-sm" :disabled="acceptingReceivedId === inv.id" @click="handleAcceptReceived(inv)">
              {{ t('team.members.receivedAccept') }}
            </button>
          </li>
        </ul>
      </div>

      <div class="card overflow-hidden">
        <div class="flex overflow-x-auto border-b border-gray-100 dark:border-dark-700">
          <button
            v-for="tab in tabs"
            :key="tab.key"
            class="shrink-0 whitespace-nowrap px-4 py-3 text-sm font-medium transition-colors"
            :class="activeTab === tab.key ? 'border-b-2 border-primary-600 text-primary-600 dark:text-primary-400' : 'text-gray-500 hover:text-gray-700 dark:text-dark-400'"
            @click="activeTab = tab.key"
          >
            {{ tab.label }}
          </button>
        </div>

        <!-- Members tab -->
        <div v-if="activeTab === 'members'" class="p-6">
          <DataTable :columns="memberColumns" :data="members" :loading="loadingMembers" row-key="user_id" flat>
            <template #cell-member="{ row }">
              <p class="font-medium text-gray-900 dark:text-white">{{ row.email }}</p>
              <p class="text-xs text-gray-400">{{ roleLabel(row.role) }}</p>
            </template>
            <template #cell-department="{ row }">
              <span class="text-gray-500 dark:text-dark-300">{{ departmentName(row.department_id) }}</span>
            </template>
            <template #cell-quota_mode="{ row }">
              <span class="text-gray-500 dark:text-dark-300">{{ row.role === 'owner' ? '—' : quotaModeLabel(row.quota_mode) }}</span>
            </template>
            <template #cell-granted="{ row }">
              <span class="text-gray-500 dark:text-dark-300">{{ row.role === 'owner' ? '—' : row.granted_net_usd.toFixed(2) }}</span>
            </template>
            <template #cell-usage="{ row }">
              <button class="btn btn-secondary btn-sm" @click.stop="openUsageStats(row)">{{ t('team.members.viewUsage') }}</button>
            </template>
            <template #cell-actions="{ row }">
              <div v-if="row.role !== 'owner'" class="flex flex-wrap justify-end gap-2">
                <button class="btn btn-secondary btn-sm" @click.stop="openTransferDialog(row, 'grant')">{{ t('team.members.grant') }}</button>
                <button class="btn btn-secondary btn-sm" @click.stop="openTransferDialog(row, 'reclaim')">{{ t('team.members.reclaim') }}</button>
                <button class="btn btn-secondary btn-sm" @click.stop="openQuotaDialog(row)">{{ t('team.members.setQuota') }}</button>
                <button class="btn btn-secondary btn-sm" @click.stop="openDepartmentDialog(row)">{{ t('team.members.setDepartment') }}</button>
                <button
                  v-if="isOwnerOfSelected"
                  class="btn btn-secondary btn-sm"
                  @click.stop="handleToggleRole(row)"
                >
                  {{ row.role === 'admin' ? t('team.members.member') : t('team.members.admin') }}
                </button>
                <button class="btn btn-secondary btn-sm text-red-600 dark:text-red-400" @click.stop="handleRemove(row)">
                  {{ t('team.members.remove') }}
                </button>
              </div>
            </template>
          </DataTable>
        </div>

        <!-- Departments tab -->
        <div v-if="activeTab === 'departments'" class="p-6">
          <h2 class="text-base font-medium text-gray-900 dark:text-white">{{ t('team.departments.title') }}</h2>
          <form @submit.prevent="handleCreateDepartment" class="mt-4 flex gap-3">
            <input v-model="newDepartmentName" :placeholder="t('team.departments.nameLabel')" class="input flex-1" :disabled="creatingDepartment" />
            <button type="submit" class="btn btn-primary shrink-0" :disabled="creatingDepartment">
              {{ t('team.departments.createSubmit') }}
            </button>
          </form>
          <div v-if="loadingDepartments" class="mt-4 text-sm text-gray-400">{{ t('common.loading') }}</div>
          <div v-else-if="departments.length === 0" class="mt-4 text-sm text-gray-400">{{ t('team.departments.empty') }}</div>
          <ul v-else class="mt-4 divide-y divide-gray-100 dark:divide-dark-700">
            <li v-for="d in departments" :key="d.id" class="flex items-center justify-between py-3">
              <span class="text-sm text-gray-900 dark:text-white">{{ d.name }}</span>
              <button class="btn btn-secondary btn-sm text-red-600 dark:text-red-400" @click="handleDeleteDepartment(d)">
                {{ t('team.departments.delete') }}
              </button>
            </li>
          </ul>
        </div>

        <!-- Invitations tab -->
        <div v-if="activeTab === 'invitations'" class="p-6 space-y-6">
          <div>
            <h2 class="text-base font-medium text-gray-900 dark:text-white">{{ t('team.members.inviteTitle') }}</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('team.members.inviteHint') }}</p>
            <form @submit.prevent="handleInvite" class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
              <input
                v-model="inviteForm.email"
                type="email"
                required
                :placeholder="t('team.members.inviteEmailPlaceholder')"
                class="input"
                :disabled="inviting"
              />
              <select v-model="inviteForm.departmentId" class="input">
                <option :value="null">{{ t('team.members.inviteDepartmentNone') }}</option>
                <option v-for="d in departments" :key="d.id" :value="d.id">{{ d.name }}</option>
              </select>
              <select v-if="isOwnerOfSelected" v-model="inviteForm.role" class="input">
                <option value="member">{{ t('team.members.inviteRoleMember') }}</option>
                <option value="admin">{{ t('team.members.inviteRoleAdmin') }}</option>
              </select>
              <select v-model="inviteForm.quotaMode" class="input">
                <option value="allocated">{{ t('team.members.inviteQuotaModeAllocated') }}</option>
                <option value="shared">{{ t('team.members.inviteQuotaModeShared') }}</option>
              </select>
              <input
                v-model.number="inviteForm.initialGrant"
                type="number"
                min="0"
                step="0.01"
                :placeholder="t('team.members.inviteInitialGrant')"
                class="input"
              />
              <button type="submit" class="btn btn-primary" :disabled="inviting">
                {{ t('team.members.inviteSubmit') }}
              </button>
            </form>
          </div>

          <div>
            <h2 class="text-base font-medium text-gray-900 dark:text-white">{{ t('team.members.pendingInvitations') }}</h2>
            <div v-if="loadingInvitations" class="mt-4 text-sm text-gray-400">{{ t('common.loading') }}</div>
            <div v-else-if="invitations.length === 0" class="mt-4 text-sm text-gray-400">
              {{ t('team.members.noPendingInvitations') }}
            </div>
            <ul v-else class="mt-4 divide-y divide-gray-100 dark:divide-dark-700">
              <li v-for="inv in invitations" :key="inv.id" class="flex items-center justify-between py-3">
                <div>
                  <p class="text-sm font-medium text-gray-900 dark:text-white">{{ inv.invited_email }}</p>
                  <p class="text-xs text-gray-400">{{ t('team.members.expiresAt') }}: {{ formatDate(inv.expires_at) }}</p>
                </div>
                <div class="flex gap-2">
                  <button @click="handleResend(inv.id)" :disabled="resendingId === inv.id" class="btn btn-secondary btn-sm">
                    {{ t('team.members.resend') }}
                  </button>
                  <button @click="handleRevoke(inv.id)" :disabled="revokingId === inv.id" class="btn btn-secondary btn-sm text-red-600 dark:text-red-400">
                    {{ t('team.members.revoke') }}
                  </button>
                </div>
              </li>
            </ul>
          </div>
        </div>

        <!-- Transfers tab -->
        <div v-if="activeTab === 'transfers'" class="p-6">
          <h2 class="text-base font-medium text-gray-900 dark:text-white">{{ t('team.transfers.title') }}</h2>
          <div class="mt-4">
            <DataTable :columns="transferColumns" :data="transfers" :loading="loadingTransfers" row-key="id" flat>
              <template #cell-direction="{ row }">{{ directionLabel(row.direction) }}</template>
              <template #cell-amount="{ row }">{{ row.amount.toFixed(2) }}</template>
              <template #cell-time="{ row }"><span class="text-gray-400">{{ formatDate(row.created_at) }}</span></template>
              <template #cell-note="{ row }"><span class="text-gray-400">{{ row.note }}</span></template>
              <template #empty>
                <div class="p-6 text-center text-sm text-gray-400">{{ t('team.transfers.empty') }}</div>
              </template>
            </DataTable>
          </div>
        </div>

        <!-- Report tab -->
        <div v-if="activeTab === 'report'" class="p-6">
          <h2 class="text-base font-medium text-gray-900 dark:text-white">{{ t('team.report.title') }}</h2>
          <div class="mt-4">
            <DataTable :columns="reportColumns" :data="report" :loading="loadingReport" row-key="user_id" flat>
              <template #cell-member="{ row }"><span class="text-gray-900 dark:text-white">{{ row.email }}</span></template>
              <template #cell-balance="{ row }">{{ row.balance.toFixed(2) }}</template>
              <template #cell-granted="{ row }">{{ row.granted_net_usd.toFixed(2) }}</template>
              <template #cell-usage="{ row }">
                <PlatformUsageBreakdown :today="row.today_cost" :total="row.cost_30d" :by-platform="row.by_platform" align="left" />
              </template>
            </DataTable>
          </div>
        </div>
      </div>
    </div>

    <!-- Transfer dialog -->
    <BaseDialog
      :show="!!transferDialog.member"
      :title="transferDialog.member ? t(transferDialog.direction === 'grant' ? 'team.members.grantTitle' : 'team.members.reclaimTitle', { email: transferDialog.member.email }) : ''"
      width="narrow"
      @close="transferDialog.member = null"
    >
      <div class="space-y-3">
        <input v-model.number="transferDialog.amount" type="number" min="0" step="0.01" :placeholder="t('team.members.amountLabel')" class="input w-full" />
        <input v-model="transferDialog.note" :placeholder="t('team.members.noteLabel')" class="input w-full" />
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="transferDialog.member = null">{{ t('team.members.cancel') }}</button>
          <button class="btn btn-primary" :disabled="transferSubmitting" @click="handleTransferSubmit">{{ t('team.members.confirm') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- Quota settings dialog -->
    <BaseDialog
      :show="!!quotaDialog.member"
      :title="t('team.members.setQuota')"
      width="narrow"
      @close="quotaDialog.member = null"
    >
      <div class="space-y-3">
        <select v-model="quotaDialog.mode" class="input w-full">
          <option value="allocated">{{ t('team.members.inviteQuotaModeAllocated') }}</option>
          <option value="shared">{{ t('team.members.inviteQuotaModeShared') }}</option>
        </select>
        <template v-if="quotaDialog.mode === 'shared'">
          <p class="text-xs text-gray-400">{{ t('team.members.quotaSharedHint') }}</p>
          <input v-model.number="quotaDialog.threshold" type="number" min="0" step="0.01" :placeholder="t('team.members.quotaThreshold')" class="input w-full" />
          <input v-model.number="quotaDialog.target" type="number" min="0" step="0.01" :placeholder="t('team.members.quotaTarget')" class="input w-full" />
        </template>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="quotaDialog.member = null">{{ t('team.members.cancel') }}</button>
          <button class="btn btn-primary" :disabled="quotaSubmitting" @click="handleQuotaSubmit">{{ t('team.members.confirm') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- Department assignment dialog -->
    <BaseDialog
      :show="!!departmentDialog.member"
      :title="t('team.members.setDepartment')"
      width="narrow"
      @close="departmentDialog.member = null"
    >
      <select v-model="departmentDialog.departmentId" class="input w-full">
        <option :value="null">{{ t('team.members.noDepartment') }}</option>
        <option v-for="d in departments" :key="d.id" :value="d.id">{{ d.name }}</option>
      </select>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="departmentDialog.member = null">{{ t('team.members.cancel') }}</button>
          <button class="btn btn-primary" :disabled="departmentSubmitting" @click="handleDepartmentSubmit">{{ t('team.members.confirm') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- Usage stats modal (same UI/data as admin user management) -->
    <UserStatsModal
      :show="!!usageStatsMember"
      :user="usageStatsMember"
      :fetch-stats="fetchMemberUsageStats"
      @close="usageStatsMember = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PlatformUsageBreakdown from '@/components/user/PlatformUsageBreakdown.vue'
import UserStatsModal from '@/components/admin/user/UserStatsModal.vue'
import DataTable from '@/components/common/DataTable.vue'
import type { Column } from '@/components/common/types'
import { useTeamStore } from '@/stores/team'
import { useAuthStore } from '@/stores/auth'
import {
  teamAPI,
  type TeamInvitation,
  type TeamReceivedInvitation,
  type TeamMember,
  type TeamDepartment,
  type TeamFundTransfer,
  type TeamReportRow
} from '@/api/team'

const { t } = useI18n()
const teamStore = useTeamStore()
const authStore = useAuthStore()

type TabKey = 'members' | 'departments' | 'invitations' | 'transfers' | 'report'
const activeTab = ref<TabKey>('members')
const tabs = computed(() => [
  { key: 'members' as TabKey, label: t('team.members.tabMembers') },
  { key: 'departments' as TabKey, label: t('team.members.tabDepartments') },
  { key: 'invitations' as TabKey, label: t('team.members.tabInvitations') },
  { key: 'transfers' as TabKey, label: t('team.members.tabTransfers') },
  { key: 'report' as TabKey, label: t('team.members.tabReport') }
])

// DataTable 列定义：桌面渲染为表格，移动端由 DataTable 自动降级为 label:value 卡片
const memberColumns = computed<Column[]>(() => [
  { key: 'member', label: t('team.members.memberList') },
  { key: 'department', label: t('team.members.department') },
  { key: 'quota_mode', label: t('team.members.quotaMode') },
  { key: 'granted', label: t('team.members.grantedNet') },
  { key: 'usage', label: t('team.report.usage') },
  { key: 'actions', label: '' }
])
const transferColumns = computed<Column[]>(() => [
  { key: 'direction', label: t('team.transfers.direction') },
  { key: 'amount', label: t('team.transfers.amount') },
  { key: 'time', label: t('team.transfers.time') },
  { key: 'note', label: t('team.transfers.note') }
])
const reportColumns = computed<Column[]>(() => [
  { key: 'member', label: t('team.members.memberList') },
  { key: 'balance', label: t('team.report.balance') },
  { key: 'granted', label: t('team.report.grantedNet') },
  { key: 'usage', label: t('team.report.usage') }
])

// 可管理的企业列表：自己创建的企业（默认，ownerUserId=undefined 让后端按调用者本人解析）
// + 作为 admin 加入的其他企业（显式带 owner_user_id）。
const managementTargets = computed(() => {
  const opts: { ownerUserId: number | undefined; label: string }[] = [
    { ownerUserId: authStore.user?.id, label: t('team.orgSwitcher.own') }
  ]
  for (const jt of teamStore.manageableJoinedTeams) {
    opts.push({ ownerUserId: jt.owner_user_id, label: jt.owner_email })
  }
  return opts
})
const selectedOwnerId = ref<number | undefined>(authStore.user?.id)
const currentOwnerParam = computed<number | undefined>(() =>
  selectedOwnerId.value === authStore.user?.id ? undefined : selectedOwnerId.value
)
const isOwnerOfSelected = computed(() => selectedOwnerId.value === authStore.user?.id)

const invitations = ref<TeamInvitation[]>([])
const members = ref<TeamMember[]>([])
const departments = ref<TeamDepartment[]>([])
const transfers = ref<TeamFundTransfer[]>([])
const report = ref<TeamReportRow[]>([])

const loadingMembers = ref(false)
const loadingInvitations = ref(false)
const loadingDepartments = ref(false)
const loadingTransfers = ref(false)
const loadingReport = ref(false)

const errorMessage = ref('')
const successMessage = ref('')

function notifyError(err: unknown) {
  errorMessage.value = (err as { message?: string })?.message || t('common.error')
}
function notifySuccess(msg: string) {
  successMessage.value = msg
  errorMessage.value = ''
}

function formatDate(value: string): string {
  try {
    return new Date(value).toLocaleString()
  } catch {
    return value
  }
}
function roleLabel(role: string): string {
  if (role === 'owner') return t('team.members.owner')
  if (role === 'admin') return t('team.members.admin')
  return t('team.members.member')
}
function quotaModeLabel(mode?: string): string {
  return mode === 'shared' ? t('team.members.quotaModeShared') : t('team.members.quotaModeAllocated')
}
function directionLabel(direction: string): string {
  if (direction === 'grant') return t('team.transfers.directionGrant')
  if (direction === 'reclaim') return t('team.transfers.directionReclaim')
  return t('team.transfers.directionAutoTopup')
}
function departmentName(id?: number | null): string {
  if (!id) return t('team.members.noDepartment')
  return departments.value.find((d) => d.id === id)?.name ?? t('team.members.noDepartment')
}

async function loadMembers() {
  loadingMembers.value = true
  try {
    members.value = await teamAPI.listMembers(currentOwnerParam.value)
  } catch (err) {
    notifyError(err)
  } finally {
    loadingMembers.value = false
  }
}

async function loadInvitations() {
  loadingInvitations.value = true
  try {
    invitations.value = await teamAPI.listInvitations(currentOwnerParam.value)
  } catch (err) {
    notifyError(err)
  } finally {
    loadingInvitations.value = false
  }
}

async function loadDepartments() {
  loadingDepartments.value = true
  try {
    departments.value = await teamAPI.listDepartments(currentOwnerParam.value)
  } catch (err) {
    notifyError(err)
  } finally {
    loadingDepartments.value = false
  }
}

async function loadTransfers() {
  loadingTransfers.value = true
  try {
    const page = await teamAPI.listTransfers(1, 50, currentOwnerParam.value)
    transfers.value = page.items
  } catch (err) {
    notifyError(err)
  } finally {
    loadingTransfers.value = false
  }
}

async function loadReport() {
  loadingReport.value = true
  try {
    report.value = await teamAPI.getReport(currentOwnerParam.value)
  } catch (err) {
    notifyError(err)
  } finally {
    loadingReport.value = false
  }
}

function loadAll() {
  errorMessage.value = ''
  successMessage.value = ''
  loadMembers()
  loadInvitations()
  loadDepartments()
  loadTransfers()
  loadReport()
  loadReceivedInvitations()
}

// --- 我收到的邀请（被邀请人视角，站内接受，不依赖邮件送达） ---
const receivedInvitations = ref<TeamReceivedInvitation[]>([])
const acceptingReceivedId = ref<number | null>(null)

async function loadReceivedInvitations() {
  try {
    receivedInvitations.value = await teamAPI.listReceivedInvitations()
  } catch {
    // 静默失败：该卡片是辅助入口，不阻塞主页面
    receivedInvitations.value = []
  }
}

async function handleAcceptReceived(inv: TeamReceivedInvitation) {
  acceptingReceivedId.value = inv.id
  try {
    await teamAPI.acceptInvitation(inv.token)
    notifySuccess(t('team.members.receivedAcceptSuccess'))
    await teamStore.loadTeams()
    loadAll()
  } catch (err) {
    notifyError(err)
    await loadReceivedInvitations()
  } finally {
    acceptingReceivedId.value = null
  }
}

// --- Invite ---
const inviteForm = ref<{ email: string; departmentId: number | null; role: string; quotaMode: string; initialGrant: number | null }>({
  email: '',
  departmentId: null,
  role: 'member',
  quotaMode: 'allocated',
  initialGrant: null
})
const inviting = ref(false)

async function handleInvite() {
  if (!inviteForm.value.email.trim()) return
  inviting.value = true
  try {
    await teamAPI.inviteMember(
      {
        email: inviteForm.value.email.trim(),
        department_id: inviteForm.value.departmentId,
        role: isOwnerOfSelected.value ? inviteForm.value.role : 'member',
        quota_mode: inviteForm.value.quotaMode,
        initial_grant_usd: inviteForm.value.initialGrant || null
      },
      currentOwnerParam.value
    )
    notifySuccess(t('team.members.inviteSuccess'))
    inviteForm.value.email = ''
    inviteForm.value.initialGrant = null
    await loadInvitations()
  } catch (err) {
    notifyError(err)
    await loadInvitations()
  } finally {
    inviting.value = false
  }
}

const revokingId = ref<number | null>(null)
const resendingId = ref<number | null>(null)

async function handleRevoke(id: number) {
  if (!confirm(t('team.members.revokeConfirm'))) return
  revokingId.value = id
  try {
    await teamAPI.revokeInvitation(id, currentOwnerParam.value)
    await loadInvitations()
  } catch (err) {
    notifyError(err)
  } finally {
    revokingId.value = null
  }
}

async function handleResend(id: number) {
  resendingId.value = id
  try {
    await teamAPI.resendInvitation(id, currentOwnerParam.value)
    notifySuccess(t('team.members.resendSuccess'))
  } catch (err) {
    notifyError(err)
  } finally {
    resendingId.value = null
  }
}

async function handleRemove(m: TeamMember) {
  if (!confirm(t('team.members.removeConfirm'))) return
  try {
    await teamAPI.removeMember(m.user_id, currentOwnerParam.value)
    await loadMembers()
  } catch (err) {
    notifyError(err)
  }
}

async function handleToggleRole(m: TeamMember) {
  const nextRole = m.role === 'admin' ? 'member' : 'admin'
  try {
    await teamAPI.setMemberRole(m.user_id, nextRole, currentOwnerParam.value)
    notifySuccess(t('team.members.updateSuccess'))
    await loadMembers()
  } catch (err) {
    notifyError(err)
  }
}

// --- Departments ---
const newDepartmentName = ref('')
const creatingDepartment = ref(false)

async function handleCreateDepartment() {
  if (!newDepartmentName.value.trim()) return
  creatingDepartment.value = true
  try {
    await teamAPI.createDepartment(newDepartmentName.value.trim(), 0, currentOwnerParam.value)
    newDepartmentName.value = ''
    notifySuccess(t('team.departments.createSuccess'))
    await loadDepartments()
  } catch (err) {
    notifyError(err)
  } finally {
    creatingDepartment.value = false
  }
}

async function handleDeleteDepartment(d: TeamDepartment) {
  if (!confirm(t('team.departments.deleteConfirm'))) return
  try {
    await teamAPI.deleteDepartment(d.id, currentOwnerParam.value)
    notifySuccess(t('team.departments.deleteSuccess'))
    await loadDepartments()
    await loadMembers()
  } catch (err) {
    notifyError(err)
  }
}

// --- Transfer dialog ---
const transferDialog = ref<{ member: TeamMember | null; direction: 'grant' | 'reclaim'; amount: number | null; note: string }>({
  member: null,
  direction: 'grant',
  amount: null,
  note: ''
})
const transferSubmitting = ref(false)

function openTransferDialog(m: TeamMember, direction: 'grant' | 'reclaim') {
  transferDialog.value = { member: m, direction, amount: null, note: '' }
}

async function handleTransferSubmit() {
  const { member, direction, amount, note } = transferDialog.value
  if (!member || !amount || amount <= 0) return
  transferSubmitting.value = true
  try {
    if (direction === 'grant') {
      await teamAPI.grantToMember(member.user_id, amount, note, currentOwnerParam.value)
      notifySuccess(t('team.members.grantSuccess'))
    } else {
      await teamAPI.reclaimFromMember(member.user_id, amount, note, currentOwnerParam.value)
      notifySuccess(t('team.members.reclaimSuccess'))
    }
    transferDialog.value.member = null
    await Promise.all([loadMembers(), loadTransfers()])
  } catch (err) {
    notifyError(err)
  } finally {
    transferSubmitting.value = false
  }
}

// --- Quota dialog ---
const quotaDialog = ref<{ member: TeamMember | null; mode: string; threshold: number | null; target: number | null }>({
  member: null,
  mode: 'allocated',
  threshold: null,
  target: null
})
const quotaSubmitting = ref(false)

function openQuotaDialog(m: TeamMember) {
  quotaDialog.value = {
    member: m,
    mode: m.quota_mode || 'allocated',
    threshold: m.quota_mode === 'shared' ? null : null,
    target: null
  }
}

async function handleQuotaSubmit() {
  const { member, mode, threshold, target } = quotaDialog.value
  if (!member) return
  quotaSubmitting.value = true
  try {
    await teamAPI.setMemberQuotaSettings(
      member.user_id,
      {
        quota_mode: mode,
        auto_topup_threshold_usd: mode === 'shared' ? threshold : null,
        auto_topup_target_usd: mode === 'shared' ? target : null
      },
      currentOwnerParam.value
    )
    notifySuccess(t('team.members.updateSuccess'))
    quotaDialog.value.member = null
    await loadMembers()
  } catch (err) {
    notifyError(err)
  } finally {
    quotaSubmitting.value = false
  }
}

// --- Department assignment dialog ---
const departmentDialog = ref<{ member: TeamMember | null; departmentId: number | null }>({ member: null, departmentId: null })
const departmentSubmitting = ref(false)

function openDepartmentDialog(m: TeamMember) {
  departmentDialog.value = { member: m, departmentId: m.department_id ?? null }
}

async function handleDepartmentSubmit() {
  const { member, departmentId } = departmentDialog.value
  if (!member) return
  departmentSubmitting.value = true
  try {
    await teamAPI.setMemberDepartment(member.user_id, departmentId, currentOwnerParam.value)
    notifySuccess(t('team.members.updateSuccess'))
    departmentDialog.value.member = null
    await loadMembers()
  } catch (err) {
    notifyError(err)
  } finally {
    departmentSubmitting.value = false
  }
}

// --- Usage stats modal (same UI/data as admin user management) ---
const usageStatsMember = ref<{ id: number; email: string; status?: string } | null>(null)

function openUsageStats(m: TeamMember) {
  usageStatsMember.value = { id: m.user_id, email: m.email, status: 'active' }
}

function fetchMemberUsageStats(userId: number, days: number) {
  return teamAPI.getMemberUsageStats(userId, days, currentOwnerParam.value)
}

onMounted(() => {
  selectedOwnerId.value = authStore.user?.id
  if (!teamStore.loaded) {
    teamStore.loadTeams().then(loadAll)
  } else {
    loadAll()
  }
})

watch(() => authStore.user?.id, (id) => {
  selectedOwnerId.value = id
  if (teamStore.loaded) loadAll()
})
</script>
