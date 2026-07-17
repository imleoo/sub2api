<template>
  <div
    ref="triggerRef"
    class="relative text-sm"
    @mouseenter="showTooltip"
    @mouseleave="hideTooltip"
  >
    <div class="flex items-center gap-1.5">
      <span class="text-gray-500 dark:text-gray-400">{{ t('admin.users.today') }}:</span>
      <span class="font-medium text-gray-900 dark:text-white">${{ today.toFixed(4) }}</span>
      <Icon
        v-if="hasBreakdown"
        name="infoCircle"
        size="xs"
        class="text-gray-400 dark:text-gray-500"
      />
    </div>
    <div class="mt-0.5 flex items-center gap-1.5">
      <span class="text-gray-500 dark:text-gray-400">{{ t('admin.users.total') }}:</span>
      <span class="font-medium text-gray-900 dark:text-white">${{ total.toFixed(4) }}</span>
    </div>

    <!-- Teleport 到 body + fixed 定位，避免被 DataTable 的 .table-wrapper(overflow:auto) 裁剪 -->
    <Teleport to="body">
      <div
        v-if="show && hasBreakdown"
        class="pointer-events-none fixed z-[9999] min-w-[220px] whitespace-nowrap rounded-md bg-gray-900 px-3 py-2 text-xs text-white shadow-xl dark:bg-dark-600"
        :class="align === 'left' ? '-translate-x-full -translate-y-1/2' : '-translate-y-1/2'"
        :style="{ left: tooltipPos.x + 'px', top: tooltipPos.y + 'px' }"
      >
        <div class="mb-1.5 flex items-center justify-between gap-3 border-b border-white/10 pb-1 text-[11px] opacity-80">
          <span>{{ t('admin.users.platformBreakdown') }}</span>
          <span class="font-mono">{{ t('admin.users.today') }} / {{ t('admin.users.total') }}</span>
        </div>
        <div
          v-for="item in sortedBreakdown"
          :key="item.platform"
          class="flex items-center justify-between gap-3 py-0.5"
          :class="{ 'opacity-70 italic': item.isOther }"
        >
          <span class="capitalize">
            {{ item.isOther ? t('admin.users.platformOther') : platformLabel(item.platform) }}
          </span>
          <span class="font-mono">
            ${{ item.today_actual_cost.toFixed(4) }}
            <span class="opacity-50">/</span>
            ${{ item.total_actual_cost.toFixed(4) }}
          </span>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { PlatformUsage } from '@/api/admin/dashboard'

const props = withDefaults(defineProps<{
  today: number
  total: number
  byPlatform?: PlatformUsage[]
  /** tooltip 弹出方向；靠近视口右边缘的用法（如撑满宽度的表格最后一列）传 'left'。 */
  align?: 'left' | 'right'
}>(), {
  align: 'right'
})

const { t } = useI18n()

const triggerRef = ref<HTMLElement | null>(null)
const show = ref(false)
const tooltipPos = ref({ x: 0, y: 0 })

function showTooltip() {
  const el = triggerRef.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  // align='left'：贴单元格左侧向左展开(配合 -translate-x-full)；否则贴右侧向右展开
  tooltipPos.value = {
    x: props.align === 'left' ? rect.left - 8 : rect.right + 8,
    y: rect.top + rect.height / 2
  }
  show.value = true
}

function hideTooltip() {
  show.value = false
}

// 与 UserDashboardStats 保持一致：把"总值 - 各平台之和"的差作为"其他"行展示，
// 避免 tooltip 内各平台费用加总与列首总值对不上。
const OTHER_THRESHOLD = 0.0001

interface BreakdownRow {
  platform: string
  today_actual_cost: number
  total_actual_cost: number
  isOther?: boolean
}

const sortedBreakdown = computed<BreakdownRow[]>(() => {
  const list = props.byPlatform ?? []
  const rows: BreakdownRow[] = [...list]
    .sort((a, b) => b.total_actual_cost - a.total_actual_cost)
    .map((p) => ({ ...p }))

  const sumTotal = rows.reduce((s, r) => s + r.total_actual_cost, 0)
  const sumToday = rows.reduce((s, r) => s + r.today_actual_cost, 0)
  const diffTotal = Math.max(0, props.total - sumTotal)
  const diffToday = Math.max(0, props.today - sumToday)
  if (diffTotal > OTHER_THRESHOLD || diffToday > OTHER_THRESHOLD) {
    rows.push({
      platform: '__other__',
      today_actual_cost: diffToday,
      total_actual_cost: diffTotal,
      isOther: true
    })
  }
  return rows
})

const hasBreakdown = computed(() => sortedBreakdown.value.length > 0)

const PLATFORM_LABELS: Record<string, string> = {
  anthropic: 'Claude',
  openai: 'OpenAI',
  gemini: 'Gemini'
}

function platformLabel(platform: string): string {
  return PLATFORM_LABELS[platform] ?? platform
}
</script>
