<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Header -->
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ t('admin.promptAnalytics.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.promptAnalytics.description') }}
          </p>
        </div>
        <button
          class="btn btn-primary flex items-center gap-2 self-start sm:self-auto"
          :disabled="loading"
          @click="loadData"
        >
          <svg
            class="h-4 w-4"
            :class="{ 'animate-spin': loading }"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="1.5"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182m0-4.991v4.99"
            />
          </svg>
          {{ t('admin.promptAnalytics.refresh') }}
        </button>
      </div>

      <!-- Filters Card -->
      <div class="card p-4">
        <div class="flex flex-wrap items-end gap-4">
          <!-- View Mode Toggle -->
          <div class="flex flex-col gap-1">
            <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.promptAnalytics.period') }}
            </label>
            <div class="flex overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
              <button
                v-for="opt in periodOptions"
                :key="opt.value"
                class="px-3 py-1.5 text-sm transition-colors"
                :class="
                  selectedPeriod === opt.value
                    ? 'bg-primary-600 text-white'
                    : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700'
                "
                @click="selectPeriod(opt.value)"
              >
                {{ opt.label }}
              </button>
            </div>
          </div>

          <!-- User ID Filter -->
          <div class="flex flex-col gap-1">
            <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.promptAnalytics.userId') }}
            </label>
            <div class="flex items-center gap-2">
              <input
                v-model="userIdInput"
                type="number"
                min="0"
                class="input w-44"
                :placeholder="t('admin.promptAnalytics.userIdPlaceholder')"
                @keydown.enter="loadData"
              />
              <button
                v-if="userIdInput"
                class="flex h-9 w-9 items-center justify-center rounded-lg text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-300"
                :title="t('admin.promptAnalytics.globalView')"
                @click="clearUserId"
              >
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
          </div>

          <!-- Limit -->
          <div class="flex flex-col gap-1">
            <label class="text-xs font-medium text-gray-500 dark:text-gray-400">
              {{ t('admin.promptAnalytics.limit') }}
            </label>
            <div class="flex overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
              <button
                v-for="l in limitOptions"
                :key="l"
                class="px-3 py-1.5 text-sm transition-colors"
                :class="
                  selectedLimit === l
                    ? 'bg-primary-600 text-white'
                    : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700'
                "
                @click="selectLimit(l)"
              >
                {{ l }}
              </button>
            </div>
          </div>

          <!-- View badge -->
          <div class="ml-auto">
            <span
              class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium"
              :class="
                userIdInput
                  ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
                  : 'bg-green-50 text-green-700 dark:bg-green-900/30 dark:text-green-300'
              "
            >
              <span
                class="inline-block h-1.5 w-1.5 rounded-full"
                :class="userIdInput ? 'bg-blue-500' : 'bg-green-500'"
              ></span>
              {{ userIdInput ? t('admin.promptAnalytics.userView') : t('admin.promptAnalytics.globalView') }}
            </span>
          </div>
        </div>
      </div>

      <!-- Content: Word Cloud + Table side by side on large screens -->
      <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <!-- Word Cloud Card -->
        <div class="card p-6">
          <h2 class="mb-4 text-base font-semibold text-gray-800 dark:text-gray-200">
            {{ t('admin.promptAnalytics.wordCloud') }}
          </h2>

          <!-- Loading skeleton -->
          <div v-if="loading" class="flex h-56 items-center justify-center">
            <div class="flex flex-col items-center gap-3">
              <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
              <span class="text-sm text-gray-400">{{ t('common.loading') }}</span>
            </div>
          </div>

          <!-- No data -->
          <div
            v-else-if="keywords.length === 0"
            class="flex h-56 flex-col items-center justify-center gap-3 text-center"
          >
            <svg
              class="h-12 w-12 text-gray-300 dark:text-gray-600"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="1"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M12 20.25c4.97 0 9-3.694 9-8.25s-4.03-8.25-9-8.25S3 7.444 3 12c0 2.104.859 4.023 2.273 5.48.432.447.74 1.04.586 1.641a4.483 4.483 0 01-.923 1.785A5.969 5.969 0 006 21c1.282 0 2.47-.402 3.445-1.087.81.22 1.668.337 2.555.337z"
              />
            </svg>
            <p class="max-w-xs text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.promptAnalytics.noData') }}
            </p>
          </div>

          <!-- Word cloud canvas -->
          <div v-else ref="wordCloudEl" class="h-64 w-full select-none overflow-hidden rounded-lg bg-gray-50 dark:bg-dark-900"></div>
        </div>

        <!-- Top Keywords Table -->
        <div class="card overflow-hidden">
          <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-800 dark:text-gray-200">
              {{ t('admin.promptAnalytics.topKeywords') }}
            </h2>
          </div>

          <!-- Loading skeleton rows -->
          <div v-if="loading" class="divide-y divide-gray-100 dark:divide-dark-700">
            <div
              v-for="i in 8"
              :key="i"
              class="flex items-center gap-4 px-6 py-3"
            >
              <div class="h-5 w-6 animate-pulse rounded bg-gray-200 dark:bg-dark-600"></div>
              <div class="h-4 flex-1 animate-pulse rounded bg-gray-200 dark:bg-dark-600"></div>
              <div class="h-4 w-12 animate-pulse rounded bg-gray-200 dark:bg-dark-600"></div>
            </div>
          </div>

          <!-- Empty -->
          <div
            v-else-if="keywords.length === 0"
            class="flex h-40 items-center justify-center text-sm text-gray-400 dark:text-gray-500"
          >
            {{ t('admin.promptAnalytics.noData') }}
          </div>

          <!-- Table -->
          <div v-else class="max-h-80 overflow-y-auto">
            <table class="w-full text-sm">
              <thead class="sticky top-0 bg-gray-50 dark:bg-dark-800">
                <tr class="text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  <th class="px-6 py-3 w-12">{{ t('admin.promptAnalytics.rank') }}</th>
                  <th class="px-4 py-3">{{ t('admin.promptAnalytics.keyword') }}</th>
                  <th class="px-6 py-3 text-right">{{ t('admin.promptAnalytics.count') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                <tr
                  v-for="(item, index) in keywords"
                  :key="item.keyword"
                  class="group transition-colors hover:bg-gray-50 dark:hover:bg-dark-800/50"
                >
                  <td class="px-6 py-3">
                    <span
                      class="flex h-6 w-6 items-center justify-center rounded-full text-xs font-bold"
                      :class="rankClass(index)"
                    >
                      {{ index + 1 }}
                    </span>
                  </td>
                  <td class="px-4 py-3">
                    <span
                      class="inline-block max-w-[200px] truncate rounded-full px-2.5 py-0.5 text-xs font-medium"
                      :style="keywordBadgeStyle(index, item.count)"
                    >
                      {{ item.keyword }}
                    </span>
                  </td>
                  <td class="px-6 py-3 text-right">
                    <div class="flex items-center justify-end gap-2">
                      <!-- Progress bar -->
                      <div class="hidden w-20 sm:block">
                        <div class="h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                          <div
                            class="h-full rounded-full bg-primary-500 transition-all duration-500"
                            :style="{ width: barWidth(item.count) }"
                          ></div>
                        </div>
                      </div>
                      <span class="font-mono text-gray-700 dark:text-gray-300">{{ item.count.toLocaleString() }}</span>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- Period info footer -->
      <div v-if="responsePeriod" class="text-right text-xs text-gray-400 dark:text-gray-500">
        {{ t('admin.promptAnalytics.period') }}: {{ responsePeriod }}
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { promptAnalyticsAPI, type KeywordCount } from '@/api/admin/promptAnalytics'

const { t } = useI18n()

// ─── State ───────────────────────────────────────────────────────────────────
const loading = ref(false)
const keywords = ref<KeywordCount[]>([])
const responsePeriod = ref('')

const userIdInput = ref<string>('')
const selectedPeriod = ref<string>('')
const selectedLimit = ref<number>(50)

const wordCloudEl = ref<HTMLElement | null>(null)

// ─── Options ─────────────────────────────────────────────────────────────────
const periodOptions = computed(() => [
  { value: currentYearMonth(-1), label: currentYearMonth(-1) },
  { value: currentYearMonth(0),  label: t('admin.promptAnalytics.currentPeriod') },
])

const limitOptions = [20, 50, 100]

// ─── Helpers ─────────────────────────────────────────────────────────────────
function currentYearMonth(offset = 0): string {
  const d = new Date()
  d.setMonth(d.getMonth() + offset)
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  return `${y}-${m}`
}

function selectPeriod(v: string) {
  selectedPeriod.value = v
  loadData()
}

function selectLimit(v: number) {
  selectedLimit.value = v
  loadData()
}

function clearUserId() {
  userIdInput.value = ''
  loadData()
}

const maxCount = computed(() => keywords.value[0]?.count ?? 1)

function barWidth(count: number): string {
  return `${Math.round((count / maxCount.value) * 100)}%`
}

function rankClass(index: number): string {
  if (index === 0) return 'bg-yellow-400 text-white'
  if (index === 1) return 'bg-gray-300 text-gray-700 dark:bg-gray-500 dark:text-white'
  if (index === 2) return 'bg-amber-600 text-white'
  return 'bg-gray-100 text-gray-500 dark:bg-dark-600 dark:text-gray-400'
}

// Colour palette cycling for keyword badges
const PALETTE = [
  { bg: 'rgba(99,102,241,0.12)', color: 'rgb(99,102,241)' },
  { bg: 'rgba(16,185,129,0.12)', color: 'rgb(16,185,129)' },
  { bg: 'rgba(245,158,11,0.12)', color: 'rgb(245,158,11)' },
  { bg: 'rgba(239,68,68,0.12)',  color: 'rgb(239,68,68)' },
  { bg: 'rgba(59,130,246,0.12)', color: 'rgb(59,130,246)' },
  { bg: 'rgba(168,85,247,0.12)', color: 'rgb(168,85,247)' },
]

function keywordBadgeStyle(index: number, count: number) {
  const p = PALETTE[index % PALETTE.length]
  // Slightly larger font for high-count keywords
  const size = Math.min(1 + (count / maxCount.value) * 0.3, 1.3)
  return {
    background: p.bg,
    color: p.color,
    fontSize: `${size}em`,
  }
}

// ─── Word Cloud (pure CSS/DOM — no external library) ─────────────────────────
function renderWordCloud() {
  const el = wordCloudEl.value
  if (!el || keywords.value.length === 0) return

  el.innerHTML = ''
  const words = keywords.value.slice(0, selectedLimit.value)
  const max = words[0]?.count ?? 1

  words.forEach((item, i) => {
    const span = document.createElement('span')
    span.textContent = item.keyword
    const ratio = item.count / max
    const fontSize = 12 + ratio * 28  // 12px – 40px
    const p = PALETTE[i % PALETTE.length]
    span.style.cssText = `
      display: inline-block;
      font-size: ${fontSize}px;
      font-weight: ${ratio > 0.6 ? 700 : ratio > 0.3 ? 600 : 400};
      color: ${p.color};
      margin: ${4 + Math.random() * 6}px ${3 + Math.random() * 8}px;
      cursor: default;
      transition: transform 0.15s;
      line-height: 1.3;
    `
    span.title = `${item.keyword}: ${item.count}`
    span.addEventListener('mouseenter', () => { span.style.transform = 'scale(1.15)' })
    span.addEventListener('mouseleave', () => { span.style.transform = 'scale(1)' })
    el.appendChild(span)
  })
}

// ─── Data loading ─────────────────────────────────────────────────────────────
async function loadData() {
  loading.value = true
  try {
    const opts: { userId?: number; period?: string; limit?: number } = {
      limit: selectedLimit.value,
    }
    if (userIdInput.value && Number(userIdInput.value) > 0) {
      opts.userId = Number(userIdInput.value)
    }
    if (selectedPeriod.value) {
      opts.period = selectedPeriod.value
    }

    const res = await promptAnalyticsAPI.getTopKeywords(opts)
    keywords.value = res.keywords ?? []
    responsePeriod.value = res.period ?? ''

    await nextTick()
    renderWordCloud()
  } catch (err) {
    console.error('Failed to load prompt analytics:', err)
    keywords.value = []
  } finally {
    loading.value = false
  }
}

// ─── Init ─────────────────────────────────────────────────────────────────────
onMounted(() => {
  selectedPeriod.value = currentYearMonth(0)
  loadData()
})

// Re-render word cloud if keyword list changes externally
watch(keywords, async () => {
  await nextTick()
  renderWordCloud()
})
</script>
