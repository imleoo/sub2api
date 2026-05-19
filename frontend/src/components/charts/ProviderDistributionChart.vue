<script setup lang="ts">
/**
 * ProviderDistributionChart (Phase 1 P1-4)
 *
 * Displays usage statistics aggregated by `provider_key` (normalized — see
 * backend `service.NormalizeProvider`). The component focuses on the
 * "cross-group" promise of P1-2 / P1-4: aggregations are NOT split by group_id.
 *
 * Lightweight by design: a card grid with key metrics (requests / tokens / cost).
 * Intended to live next to GroupDistributionChart / AccountDistributionChart on
 * the admin dashboard so all three perspectives can be compared at a glance.
 */
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { getStatsByProvider } from '@/api/admin/accounts'
import type { AccountUsageStatsResponse } from '@/types'

interface Props {
  /** Normalized provider_keys to display (e.g. ["anthropic", "openai", "deepseek"]). */
  providers: string[]
  /** Window in days (default 30, max 90 enforced by backend). */
  days?: number
}

const props = withDefaults(defineProps<Props>(), { days: 30 })
const { t } = useI18n()

interface ProviderRow {
  provider: string
  loading: boolean
  error: string | null
  stats: AccountUsageStatsResponse | null
}

const rows = ref<ProviderRow[]>([])

async function fetchAll() {
  rows.value = props.providers.map((p) => ({ provider: p, loading: true, error: null, stats: null }))
  await Promise.all(
    rows.value.map(async (row, idx) => {
      try {
        const stats = await getStatsByProvider(row.provider, props.days)
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
  () => [props.providers, props.days],
  () => {
    void fetchAll()
  },
  { immediate: true, deep: true }
)

const totalRequests = computed(() =>
  rows.value.reduce((sum, r) => sum + (r.stats?.summary.total_requests ?? 0), 0)
)
const totalTokens = computed(() =>
  rows.value.reduce((sum, r) => sum + (r.stats?.summary.total_tokens ?? 0), 0)
)
const totalUserCost = computed(() =>
  rows.value.reduce((sum, r) => sum + (r.stats?.summary.total_user_cost ?? 0), 0)
)

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
        {{ t('admin.dashboard.providerDistribution') }}
      </h3>
      <div class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.dashboard.lastNDays', { n: days }) }}
      </div>
    </div>

    <table class="w-full text-xs">
      <thead>
        <tr class="text-gray-500 dark:text-gray-400">
          <th class="pb-2 text-left">{{ t('admin.dashboard.provider') }}</th>
          <th class="pb-2 text-right">{{ t('admin.dashboard.requests') }}</th>
          <th class="pb-2 text-right">{{ t('admin.dashboard.tokens') }}</th>
          <th class="pb-2 text-right">{{ t('admin.dashboard.actual') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="row in rows"
          :key="row.provider"
          class="border-t border-gray-100 dark:border-gray-700"
        >
          <td class="py-1.5 font-medium text-gray-900 dark:text-white">
            {{ row.provider }}
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
          <td class="py-1.5 text-gray-900 dark:text-white">
            {{ t('admin.dashboard.total') }}
          </td>
          <td class="py-1.5 text-right">{{ formatNumber(totalRequests) }}</td>
          <td class="py-1.5 text-right">{{ formatNumber(totalTokens) }}</td>
          <td class="py-1.5 text-right text-green-600 dark:text-green-400">
            {{ formatCost(totalUserCost) }}
          </td>
        </tr>
      </tfoot>
    </table>

    <p v-if="rows.length === 0" class="mt-4 text-center text-xs text-gray-500 dark:text-gray-400">
      {{ t('admin.dashboard.providersEmpty') }}
    </p>
  </div>
</template>
