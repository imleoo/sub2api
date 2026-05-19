<script setup lang="ts">
/**
 * AccountDistributionChart (Phase 1 P1-4)
 *
 * Displays usage statistics per account, cross-group. Each account card shows
 * "covered groups = N" to make the m2m relation visible — a key UI promise from
 * docs/generic-channel-design.md §5.4 第 10 条.
 *
 * Lightweight by design: row-per-account table. Uses GetStatsCrossGroup so the
 * aggregation never gets split by group_id.
 */
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { getStatsCrossGroup } from '@/api/admin/accounts'
import type { Account, AccountUsageStatsResponse } from '@/types'

interface Props {
  /** Account list to display (already loaded by the parent view). */
  accounts: Account[]
  /** Window in days (default 30, max 90 enforced by backend). */
  days?: number
}

const props = withDefaults(defineProps<Props>(), { days: 30 })
const { t } = useI18n()

interface AccountRow {
  account: Account
  loading: boolean
  error: string | null
  stats: AccountUsageStatsResponse | null
}

const rows = ref<AccountRow[]>([])

async function fetchAll() {
  rows.value = props.accounts.map((acc) => ({ account: acc, loading: true, error: null, stats: null }))
  await Promise.all(
    rows.value.map(async (row, idx) => {
      try {
        const stats = await getStatsCrossGroup(row.account.id, props.days)
        rows.value[idx] = { ...row, loading: false, stats, error: null }
      } catch (err) {
        rows.value[idx] = {
          ...row,
          loading: false,
          stats: null,
          error: err instanceof Error ? err.message : String(err)
        }
      }
    })
  )
}

watch(
  () => [props.accounts.map((a) => a.id), props.days],
  () => {
    void fetchAll()
  },
  { immediate: true, deep: true }
)

const totalRequests = computed(() =>
  rows.value.reduce((sum, r) => sum + (r.stats?.summary.total_requests ?? 0), 0)
)
const totalUserCost = computed(() =>
  rows.value.reduce((sum, r) => sum + (r.stats?.summary.total_user_cost ?? 0), 0)
)

function coveredGroups(acc: Account): number {
  return acc.group_ids?.length ?? 0
}

function formatNumber(n: number | undefined | null): string {
  if (n == null) return '-'
  return n.toLocaleString()
}

function formatCost(n: number | undefined | null): string {
  if (n == null) return '-'
  return `$${n.toFixed(4)}`
}
</script>

<template>
  <div class="card p-4">
    <div class="mb-4 flex items-center justify-between gap-3">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.dashboard.accountDistribution') }}
      </h3>
      <div class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.dashboard.lastNDays', { n: days }) }}
      </div>
    </div>

    <table class="w-full text-xs">
      <thead>
        <tr class="text-gray-500 dark:text-gray-400">
          <th class="pb-2 text-left">{{ t('admin.dashboard.account') }}</th>
          <th class="pb-2 text-right">{{ t('admin.dashboard.coveredGroups') }}</th>
          <th class="pb-2 text-right">{{ t('admin.dashboard.requests') }}</th>
          <th class="pb-2 text-right">{{ t('admin.dashboard.tokens') }}</th>
          <th class="pb-2 text-right">{{ t('admin.dashboard.actual') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in rows"
          :key="row.account.id"
          class="border-t border-gray-100 dark:border-gray-700"
        >
          <td class="py-1.5 font-medium text-gray-900 dark:text-white">
            {{ row.account.name }}
          </td>
          <td class="py-1.5 text-right">
            <span
              class="inline-flex items-center rounded-md bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-300"
              :title="t('admin.dashboard.coveredGroupsHint', { n: coveredGroups(row.account) })"
            >
              {{ coveredGroups(row.account) }}
            </span>
          </td>
          <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">
            <span v-if="row.loading">…</span>
            <span v-else-if="row.error" class="text-red-500" :title="row.error">!</span>
            <span v-else>{{ formatNumber(row.stats?.summary.total_requests) }}</span>
          </td>
          <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">
            {{ row.loading ? '…' : formatNumber(row.stats?.summary.total_tokens) }}
          </td>
          <td class="py-1.5 text-right text-green-600 dark:text-green-400">
            {{ row.loading ? '…' : formatCost(row.stats?.summary.total_user_cost) }}
          </td>
        </tr>
      </tbody>
      <tfoot v-if="rows.length > 0">
        <tr class="border-t-2 border-gray-200 dark:border-gray-600 font-semibold">
          <td class="py-1.5 text-gray-900 dark:text-white" colspan="2">
            {{ t('admin.dashboard.total') }}
          </td>
          <td class="py-1.5 text-right">{{ formatNumber(totalRequests) }}</td>
          <td class="py-1.5 text-right">-</td>
          <td class="py-1.5 text-right text-green-600 dark:text-green-400">
            {{ formatCost(totalUserCost) }}
          </td>
        </tr>
      </tfoot>
    </table>

    <p v-if="rows.length === 0" class="mt-4 text-center text-xs text-gray-500 dark:text-gray-400">
      {{ t('admin.dashboard.accountsEmpty') }}
    </p>
  </div>
</template>
