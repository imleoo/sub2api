<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Header -->
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ t('models.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('models.description') }}
            <span v-if="total > 0" class="ml-2 inline-flex items-center rounded-full bg-primary-50 px-2.5 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
              {{ total }} {{ t('models.modelsAvailable') }}
            </span>
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
              d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182m0-4.991v4.991"
            />
          </svg>
          {{ t('models.refresh') }}
        </button>
      </div>

      <!-- Filters -->
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-4">
          <!-- Search -->
          <div class="relative min-w-[200px] flex-1 sm:min-w-[280px]">
            <svg
              class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z"
              />
            </svg>
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('models.searchPlaceholder')"
              class="input w-full pl-10"
            />
          </div>

          <!-- Provider Filter Tabs -->
          <div class="flex overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-600">
            <button
              v-for="p in providerOptions"
              :key="p.value"
              class="whitespace-nowrap px-3 py-1.5 text-sm transition-colors"
              :class="
                selectedProvider === p.value
                  ? 'bg-primary-600 text-white'
                  : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700'
              "
              @click="selectProvider(p.value)"
            >
              {{ p.label }}
              <span
                v-if="p.count !== undefined && selectedProvider === p.value"
                class="ml-1.5 text-xs opacity-80"
              >({{ p.count }})</span>
            </button>
          </div>

          <!-- Mode Filter -->
          <div class="flex overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
            <button
              v-for="m in modeOptions"
              :key="m.value"
              class="px-3 py-1.5 text-sm transition-colors"
              :class="
                selectedMode === m.value
                  ? 'bg-primary-600 text-white'
                  : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700'
              "
              @click="selectMode(m.value)"
            >
              {{ m.label }}
            </button>
          </div>
        </div>
      </div>

      <!-- Loading State -->
      <div v-if="loading" class="card p-12">
        <div class="flex flex-col items-center justify-center gap-4">
          <div class="h-10 w-10 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
          <span class="text-sm text-gray-400">{{ t('common.loading') }}</span>
        </div>
      </div>

      <!-- Empty State -->
      <div
        v-else-if="filteredModels.length === 0"
        class="card flex flex-col items-center justify-center p-16"
      >
        <svg
          class="mb-4 h-16 w-16 text-gray-300 dark:text-gray-600"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="1"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125M3.75 6.375c0 2.278 3.694 4.125 8.25 4.125m0 0c4.614 0 8.308-1.852 8.308-4.125m0 0V9c0 2.278-3.694 4.125-8.25 4.125M12 10.5v6.375m0 0c4.614 0 8.308-1.852 8.308-4.125m0 0V12c0-2.278-3.694-4.125-8.25-4.125"
          />
        </svg>
        <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('models.noResults') }}</p>
      </div>

      <!-- Model Card Grid -->
      <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        <div
          v-for="model in filteredModels"
          :key="model.id"
          class="card relative overflow-hidden p-4 transition-shadow hover:shadow-md"
        >
          <!-- Discount badge (top-right corner) -->
          <div
            v-if="model.discount_rate && model.discount_rate < 1"
            class="absolute right-0 top-0 rounded-bl-lg bg-red-500 px-2 py-0.5 text-xs font-bold text-white"
          >
            {{ formatDiscountLabel(model.discount_rate) }}
          </div>

          <!-- Model name + copy -->
          <div class="mb-3 flex items-start gap-2 pr-10">
            <span class="flex-1 break-all font-mono text-sm font-semibold text-gray-900 dark:text-white" :title="model.id">
              {{ model.id }}
            </span>
            <button
              type="button"
              class="mt-0.5 flex-shrink-0 text-gray-400 transition-colors hover:text-gray-700 dark:hover:text-gray-200"
              :title="copiedModelId === model.id ? t('models.copied') : t('models.copyModelName')"
              @click="copyModelName(model.id)"
            >
              <svg v-if="copiedModelId === model.id" class="h-4 w-4 text-emerald-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
              </svg>
              <svg v-else class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8">
                <path stroke-linecap="round" stroke-linejoin="round" d="M8 8.25V6a2.25 2.25 0 012.25-2.25h7.5A2.25 2.25 0 0120 6v7.5a2.25 2.25 0 01-2.25 2.25H15.5" />
                <path stroke-linecap="round" stroke-linejoin="round" d="M4 10.5A2.5 2.5 0 016.5 8h6A2.5 2.5 0 0115 10.5v6A2.5 2.5 0 0112.5 19h-6A2.5 2.5 0 014 16.5v-6z" />
              </svg>
            </button>
          </div>

          <!-- Tags row -->
          <div class="mb-3 flex flex-wrap gap-1.5">
            <span class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium" :class="providerBadgeClass(model.provider)">
              {{ providerLabel(model.provider) }}
            </span>
            <span v-if="isImageModel(model)" class="inline-flex items-center rounded-full bg-purple-50 px-2 py-0.5 text-[11px] font-medium text-purple-700 dark:bg-purple-900/30 dark:text-purple-400">
              {{ t('models.imageMode') }}
            </span>
            <span v-if="model.supports_prompt_caching" class="inline-flex items-center rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400">
              {{ t('models.promptCaching') }}
            </span>
            <span v-if="isVisionModel(model.id)" class="inline-flex items-center rounded-full bg-blue-50 px-2 py-0.5 text-[11px] font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
              Vision
            </span>
          </div>

          <!-- Price section -->
          <div class="border-t border-gray-100 pt-3 dark:border-dark-700">
            <div v-if="model.pricing_unit === 'second'" class="text-xs">
              <div class="mb-0.5 text-gray-400">{{ t('models.pricing.secondPrice') }}</div>
              <div v-if="appStore.currencyMode !== 'cny'" class="font-mono font-medium text-gray-800 dark:text-gray-200">
                {{ formatSecondUsdPrice(model) }}
              </div>
              <div class="text-gray-400">{{ formatSecondCnyPrice(model) }}</div>
            </div>
            <div v-else-if="model.pricing_unit === 'image_generation' || model.pricing_unit === 'video_generation'" class="text-xs">
              <div class="mb-0.5 text-gray-400">{{ visualPriceLabel(model) }}</div>
              <div v-if="appStore.currencyMode !== 'cny'" class="font-mono font-medium text-gray-800 dark:text-gray-200">
                {{ formatVisualUsdPrice(model) }}
              </div>
              <div class="text-gray-400">{{ formatVisualCnyPrice(model) }}</div>
            </div>
            <div v-else class="grid grid-cols-2 gap-2 text-xs">
              <div>
                <div class="mb-0.5 text-gray-400">{{ t('models.inputPrice') }}</div>
                <div v-if="appStore.currencyMode !== 'cny'" class="font-mono font-medium text-gray-800 dark:text-gray-200">
                  {{ formatPrice(model.input_cost_per_token * (model.discount_rate ?? 1)) }}
                </div>
                <div class="text-gray-400">¥{{ (model.input_cost_per_token * (model.discount_rate ?? 1) * 1_000_000 * appStore.cnyRate).toFixed(2) }}</div>
              </div>
              <div>
                <div class="mb-0.5 text-gray-400">{{ t('models.outputPrice') }}</div>
                <div v-if="appStore.currencyMode !== 'cny'" class="font-mono font-medium text-gray-800 dark:text-gray-200">
                  {{ formatPrice(model.output_cost_per_token * (model.discount_rate ?? 1)) }}
                </div>
                <div class="text-gray-400">¥{{ (model.output_cost_per_token * (model.discount_rate ?? 1) * 1_000_000 * appStore.cnyRate).toFixed(2) }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer count -->
      <p v-if="filteredModels.length > 0" class="text-xs text-gray-400">
        {{ t('models.showing', { count: filteredModels.length, total: total }) }}
      </p>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getModels, type ModelInfo } from '@/api/models'

const { t } = useI18n()
const appStore = useAppStore()

// 海外开关 + 版本下限过滤已下沉到后端 /api/v1/models（service.FilterVisibleModels），
// 前端不再重复过滤，getModels 返回的即为用户应见集合。

// ─── State ────────────────────────────────
const loading = ref(false)
const allModels = ref<ModelInfo[]>([])
const cnyRate = ref(7)
const searchQuery = ref('')
const selectedProvider = ref<string>('all')
const selectedMode = ref<string>('all')
const copiedModelId = ref<string | null>(null)

// ─── Computed ─────────────────────────────
// 可见性已由后端过滤，baseModels 直接取全部返回模型。
const baseModels = computed(() => allModels.value)

const total = computed(() => baseModels.value.length)

// Provider options derived from data
const providerOptions = computed(() => {
  const counts = new Map<string, number>()

  for (const m of baseModels.value) {
    const key = m.provider || 'other'
    counts.set(key, (counts.get(key) || 0) + 1)
  }

  // Sort by count descending, then alphabetically
  const sorted = Array.from(counts.entries())
    .sort((a, b) => b[1] - a[1])
    .slice(0, 8)
    .map(([value, count]) => ({ value, label: providerLabel(value), count }))

  return [
    { value: 'all', label: t('models.allProviders'), count: baseModels.value.length },
    ...sorted,
  ]
})

const modeOptions = computed(() => [
  { value: 'all', label: t('models.allModes') },
  { value: 'chat', label: t('models.chatMode') },
  { value: 'image', label: t('models.imageMode') },
])

const filteredModels = computed(() => {
  let result = baseModels.value

  if (selectedProvider.value !== 'all') {
    result = result.filter(m => (m.provider || 'other') === selectedProvider.value)
  }

  if (selectedMode.value !== 'all') {
    result = result.filter(m => normalizeModelMode(m) === selectedMode.value)
  }

  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    result = result.filter(m => m.id.toLowerCase().includes(q))
  }

  // Sort: by input price descending (most expensive/prominent first)
  return [...result].sort((a, b) => b.input_cost_per_token - a.input_cost_per_token)
})

// ─── Actions ─────────────────────────────
function selectProvider(v: string) {
  selectedProvider.value = v
}

function selectMode(v: string) {
  selectedMode.value = v
}

async function loadData() {
  loading.value = true
  try {
    const res = await getModels()
    cnyRate.value = res.cny_rate ?? 7.2
    // 可见性（海外开关 + 版本下限）已由后端过滤，前端直接采用返回集合。
    allModels.value = res.models ?? []
  } catch (err) {
    console.error('[ModelsView] Failed to load models:', err)
    allModels.value = []
  } finally {
    loading.value = false
  }
}

async function copyModelName(modelId: string) {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(modelId)
    } else {
      copyTextFallback(modelId)
    }
    copiedModelId.value = modelId
    window.setTimeout(() => {
      if (copiedModelId.value === modelId) {
        copiedModelId.value = null
      }
    }, 1500)
  } catch (err) {
    console.error('[ModelsView] Failed to copy model name:', err)
  }
}

function copyTextFallback(text: string) {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  document.execCommand('copy')
  document.body.removeChild(textarea)
}

// ─── Formatters ──────────────────────────
// discount_rate ∈ (0,1)：0.85 → 8.5 折，0.01 → 0.1 折。
// 保留 1 位小数避免出现 "0 折"/"9 折" 这类被四舍五入吞掉的折扣。
function formatDiscountLabel(rate: number): string {
  const tenths = Math.round(rate * 1000) / 100
  return Number.isInteger(tenths) ? `${tenths}折` : `${tenths.toFixed(1)}折`
}

function formatPrice(costPerToken: number): string {
  if (!costPerToken || costPerToken <= 0) return '--'

  // costPerToken is the price per single token
  // Convert to price per 1M tokens for display
  const per1m = costPerToken * 1_000_000

  if (per1m >= 0.1) {
    // ≥ $0.1/1M tokens: 2 位小数足够
    return `$${per1m.toFixed(2)}`
  }
  if (per1m >= 0.01) {
    // 0.01 ~ 0.1: 给 3 位小数避免被 toFixed(2) 吞掉
    return `$${per1m.toFixed(3)}`
  }
  // < 0.01：保留至少 2 位有效数字，避免 0.1 折后 $0.0018 被截成 0
  return `$${per1m.toPrecision(2)}`
}

function formatSecondUsdPrice(model: ModelInfo): string {
  const price = model.input_cost_per_token * (model.discount_rate ?? 1)
  if (!price || price <= 0) return '--'
  const display = price.toFixed(6).replace(/0+$/, '').replace(/\.$/, '')
  return `$${display}${t('models.pricing.unitPerSecond')}`
}

function formatSecondCnyPrice(model: ModelInfo): string {
  const price = model.input_cost_per_token * (model.discount_rate ?? 1) * appStore.cnyRate
  if (!price || price <= 0) return '--'
  return `¥${price.toFixed(4)}${t('models.pricing.unitPerSecond')}`
}

// 图片/视频：分列价（按次 output_cost_per_image / 按 token output_cost_per_image_token），含折扣。
function visualUnitUsd(model: ModelInfo): number {
  const d = model.discount_rate ?? 1
  if (model.output_cost_per_image != null && model.output_cost_per_image > 0) return model.output_cost_per_image * d
  if (model.output_cost_per_image_token != null && model.output_cost_per_image_token > 0) return model.output_cost_per_image_token * d
  return 0
}
function visualUnitSuffix(model: ModelInfo): string {
  if (model.output_cost_per_image != null && model.output_cost_per_image > 0) {
    return model.pricing_unit === 'video_generation' ? t('models.pricing.unitPerSecond') : t('models.pricing.unitPerImage')
  }
  return t('models.pricing.unitPerToken')
}
function visualPriceLabel(model: ModelInfo): string {
  return model.pricing_unit === 'video_generation' ? t('models.pricing.videoPrice') : t('models.pricing.imagePrice')
}
function formatVisualUsdPrice(model: ModelInfo): string {
  const v = visualUnitUsd(model)
  if (!v) return '--'
  const display = v.toFixed(6).replace(/0+$/, '').replace(/\.$/, '')
  return `$${display}${visualUnitSuffix(model)}`
}
function formatVisualCnyPrice(model: ModelInfo): string {
  const v = visualUnitUsd(model)
  if (!v) return '--'
  return `¥${(v * appStore.cnyRate).toFixed(4)}${visualUnitSuffix(model)}`
}

// ─── Helpers ────────────────────────────
function normalizeModelMode(model: ModelInfo): string {
  const mode = (model.mode || '').toLowerCase()
  if (mode === 'image' || mode === 'image_generation') {
    return 'image'
  }
  return mode || 'chat'
}

function isImageModel(model: ModelInfo): boolean {
  const id = model.id.toLowerCase()
  return (
    normalizeModelMode(model) === 'image' ||
    id.startsWith('gpt-image-') ||
    id.startsWith('dall-e') ||
    id.startsWith('imagen-') ||
    (id.startsWith('gemini-') && id.includes('-image')) ||
    (id.startsWith('grok-') && id.includes('image'))
  )
}

function isVisionModel(modelId: string): boolean {
  const lower = modelId.toLowerCase()
  return (
    lower.includes('vision') ||
    lower.includes('gpt-4o') ||
    lower.includes('gpt-4.1') ||
    lower.includes('gemini-2.5-flash-image') ||
    lower.includes('gemini-3.1-flash-image') ||
    lower.includes('glm-4v')
  )
}

function providerLabel(provider: string): string {
  const map: Record<string, string> = {
    anthropic: 'Anthropic',
    openai: 'OpenAI',
    google: 'Google',
    deepseek: 'DeepSeek',
    mistral: 'Mistral',
    meta: 'Meta',
    xai: 'xAI',
    cohere: 'Cohere',
    moonshot: 'Moonshot',
    qwen: 'Qwen',
    zhipu: 'Zhipu',
    doubao: 'Doubao',
    minimax: 'MiniMax',
  }
  return map[provider] || provider.charAt(0).toUpperCase() + provider.slice(1)
}

function providerBadgeClass(provider: string): string {
  const map: Record<string, string> = {
    anthropic: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
    openai: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400',
    google: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400',
    deepseek: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400',
    mistral: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400',
    meta: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400',
    xai: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
  }
  return map[provider] || 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
}

// ─── Init ────────────────────────────────
onMounted(() => {
  loadData()
})
</script>
