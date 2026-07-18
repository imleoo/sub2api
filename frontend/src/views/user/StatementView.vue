<template>
  <div class="space-y-6">
    <!-- 工具栏：月份选择 + 导出 -->
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-3">
        <select
          v-model="selectedMonth"
          class="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-primary-500 focus:outline-none dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100"
          data-testid="statement-month-select"
        >
          <option v-for="m in months" :key="m" :value="m">{{ m }}</option>
        </select>
        <span
          v-if="statement && !statement.closed"
          class="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-900/40 dark:text-amber-300"
        >
          {{ t('statement.notClosed') }}
        </span>
        <span
          v-if="statement && statement.source === 'computed'"
          class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-400"
          :title="t('statement.computedNote')"
        >
          {{ t('statement.computed') }}
        </span>
      </div>
      <button
        :disabled="exporting || !selectedMonth"
        class="inline-flex items-center gap-2 rounded-lg bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-700 disabled:opacity-50"
        data-testid="statement-export-btn"
        @click="handleExport"
      >
        <Icon name="download" size="sm" :stroke-width="2" />
        {{ exporting ? t('statement.exporting') : t('statement.export') }}
      </button>
    </div>

    <!-- 汇总卡片 -->
    <div v-if="statement" class="grid grid-cols-2 gap-4 md:grid-cols-4">
      <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('statement.openingBalance') }}</div>
        <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">${{ fmt(statement.opening_balance) }}</div>
      </div>
      <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('statement.totalIn') }}</div>
        <div class="mt-1 text-lg font-semibold text-emerald-600 dark:text-emerald-400">
          +${{ fmt(statement.totals.deposit + statement.totals.credit) }}
        </div>
      </div>
      <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('statement.totalOut') }}</div>
        <div class="mt-1 text-lg font-semibold text-red-600 dark:text-red-400">
          -${{ fmt(statement.totals.withdraw + statement.totals.utilisation_after) }}
        </div>
      </div>
      <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="text-xs text-gray-500 dark:text-gray-400">{{ t('statement.closingBalance') }}</div>
        <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-gray-100">${{ fmt(statement.closing_balance) }}</div>
      </div>
    </div>

    <!-- 对账明细表 -->
    <DataTable
      :columns="columns"
      :data="statement?.rows ?? []"
      :loading="loading"
      :row-key="rowKey"
      :virtualize-threshold="200"
    >
      <template #cell-date="{ row }">
        <span class="text-sm text-gray-900 dark:text-gray-100">{{ row.date }}</span>
      </template>
      <template #cell-nature="{ row }">
        <span :class="natureClass(row.nature)">{{ t(`statement.nature.${row.nature}`) }}</span>
      </template>
      <template #cell-amount="{ row }">
        <span v-if="row.nature === 'deposit' || row.nature === 'credit'" class="text-sm text-emerald-600 dark:text-emerald-400">
          +${{ fmt(row.amount ?? 0) }}
        </span>
        <span v-else-if="row.nature === 'withdraw'" class="text-sm text-red-600 dark:text-red-400">
          -${{ fmt(row.amount ?? 0) }}
        </span>
        <span v-else-if="row.nature === 'utilisation'" class="text-sm text-red-600 dark:text-red-400">
          -${{ fmt(row.cost_after ?? 0) }}
        </span>
        <span v-else class="text-sm text-gray-400">—</span>
      </template>
      <template #cell-detail="{ row }">
        <template v-if="row.nature === 'utilisation'">
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('statement.qty') }} {{ (row.qty ?? 0).toLocaleString() }} ·
            {{ t('statement.costBefore') }} ${{ fmt(row.cost_before ?? 0) }} ·
            {{ t('statement.discount') }} {{ ((row.discount_rate ?? 0) * 100).toFixed(1) }}%
          </span>
        </template>
        <span v-else class="text-xs text-gray-500 dark:text-gray-400">{{ row.note || '—' }}</span>
      </template>
      <template #cell-running_total="{ row }">
        <span class="text-sm font-medium text-gray-900 dark:text-gray-100">${{ fmt(row.running_total) }}</span>
      </template>
    </DataTable>

    <!-- 恒等式差额提示 -->
    <p
      v-if="statement && Math.abs(statement.totals.identity_gap) > 1e-6"
      class="text-xs text-amber-600 dark:text-amber-400"
    >
      {{ t('statement.identityGapNote', { gap: fmt(statement.totals.identity_gap) }) }}
    </p>
  </div>
</template>

<script setup lang="ts">
// zhiguofan fork-only: 月度对账（Vendor Report，功能 45）
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'
import { statementAPI, type StatementResponse, type StatementRow } from '@/api/statement'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

const months = ref<string[]>([])
const selectedMonth = ref('')
const statement = ref<StatementResponse | null>(null)
const loading = ref(false)
const exporting = ref(false)

const columns = computed(() => [
  { key: 'date', label: t('statement.columns.date') },
  { key: 'nature', label: t('statement.columns.nature') },
  { key: 'amount', label: t('statement.columns.amount') },
  { key: 'detail', label: t('statement.columns.detail') },
  { key: 'running_total', label: t('statement.columns.runningTotal') }
])

function rowKey(row: StatementRow): string {
  return `${row.date}-${row.nature}-${row.note ?? ''}`
}

function fmt(v: number): string {
  return v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

function natureClass(nature: string): string {
  const base = 'inline-flex rounded-full px-2 py-0.5 text-xs font-medium '
  switch (nature) {
    case 'deposit':
      return base + 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'credit':
      return base + 'bg-sky-100 text-sky-800 dark:bg-sky-900/40 dark:text-sky-300'
    case 'withdraw':
      return base + 'bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300'
    case 'utilisation':
      return base + 'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300'
    default:
      return base + 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
  }
}

const browserTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone

async function loadStatement() {
  if (!selectedMonth.value) return
  loading.value = true
  try {
    statement.value = await statementAPI.getStatement(selectedMonth.value, browserTimezone)
  } catch {
    appStore.showError(t('statement.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function handleExport() {
  if (!selectedMonth.value) return
  exporting.value = true
  try {
    const blob = await statementAPI.exportStatement(selectedMonth.value, browserTimezone)
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `statement-${selectedMonth.value}.xlsx`
    link.click()
    window.URL.revokeObjectURL(url)
  } catch {
    appStore.showError(t('statement.exportFailed'))
  } finally {
    exporting.value = false
  }
}

watch(selectedMonth, loadStatement)

onMounted(async () => {
  try {
    months.value = await statementAPI.getStatementMonths()
    if (months.value.length > 0) {
      selectedMonth.value = months.value[0]
    }
  } catch {
    appStore.showError(t('statement.loadFailed'))
  }
})
</script>
