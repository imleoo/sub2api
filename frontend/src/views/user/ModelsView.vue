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

      <!-- Model Grid / Table -->
      <div v-else class="card overflow-hidden">
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead class="sticky top-0 z-10 bg-gray-50 dark:bg-dark-800">
              <tr class="border-b border-gray-100 dark:border-dark-700">
                <th class="px-6 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ t('models.modelName') }}
                </th>
                <th class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ t('models.provider') }}
                </th>
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ t('models.inputPrice') }}
                </th>
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ t('models.outputPrice') }}
                </th>
                <th class="px-4 py-3 text-center text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ t('models.contextWindow') }}
                </th>
                <th class="px-6 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ t('models.features') }}
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-50 dark:divide-dark-700/50">
              <tr
                v-for="(model, index) in filteredModels"
                :key="model.id"
                class="transition-colors hover:bg-gray-50/70 dark:hover:bg-dark-800/50"
                :style="{ animationDelay: `${Math.min(index * 15, 300)}ms` }"
              >
                <!-- Model Name + Icon -->
                <td class="px-6 py-3.5">
                  <div class="flex items-center gap-3">
                    <ModelIcon :model="model.id" size="22" />
                    <div class="min-w-0 flex-1">
                      <span class="block truncate font-mono text-sm font-medium text-gray-900 dark:text-white" :title="model.id">
                        {{ model.id }}
                      </span>
                    </div>
                  </div>
                </td>

                <!-- Provider Badge -->
                <td class="px-4 py-3.5">
                  <span
                    class="inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium"
                    :class="providerBadgeClass(model.provider)"
                  >
                    {{ providerLabel(model.provider) }}
                  </span>
                </td>

                <!-- Input Price -->
                <td class="px-4 py-3.5 text-right">
                  <span class="font-mono text-gray-700 dark:text-gray-300">
                    {{ formatPrice(model.input_cost_per_token) }}
                  </span>
                </td>

                <!-- Output Price -->
                <td class="px-4 py-3.5 text-right">
                  <span class="font-mono text-gray-700 dark:text-gray-300">
                    {{ formatPrice(model.output_cost_per_token) }}
                  </span>
                </td>

                <!-- Context Window -->
                <td class="px-4 py-3.5 text-center">
                  <span v-if="model.long_context_input_token_threshold" class="font-mono text-xs text-gray-600 dark:text-gray-400">
                    {{ formatNumber(model.long_context_input_token_threshold) }}
                  </span>
                  <span v-else class="text-xs text-gray-400">--</span>
                </td>

                <!-- Features -->
                <td class="px-6 py-3.5">
                  <div class="flex flex-wrap items-center gap-1.5">
                    <span
                      v-if="model.supports_prompt_caching"
                      class="inline-flex items-center rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                    >
                      {{ t('models.promptCaching') }}
                    </span>
                    <span
                      v-if="model.mode === 'image'"
                      class="inline-flex items-center rounded-full bg-purple-50 px-2 py-0.5 text-[10px] font-medium text-purple-700 dark:bg-purple-900/30 dark:text-purple-400"
                    >
                      Image
                    </span>
                    <span
                      v-if="isVisionModel(model.id)"
                      class="inline-flex items-center rounded-full bg-blue-50 px-2 py-0.5 text-[10px] font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-400"
                    >
                      Vision
                    </span>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Footer count -->
        <div class="border-t border-gray-100 px-6 py-3 dark:border-dark-700">
          <p class="text-xs text-gray-400">
            {{ t('models.showing', { count: filteredModels.length, total: total }) }}
          </p>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'
import { getModels, type ModelInfo } from '@/api/models'

const { t } = useI18n()

// ─── State ────────────────────────────────
const loading = ref(false)
const allModels = ref<ModelInfo[]>([])
const searchQuery = ref('')
const selectedProvider = ref<string>('all')
const selectedMode = ref<string>('all')

// ─── Computed ─────────────────────────────
const total = computed(() => allModels.value.length)

// Provider options derived from data
const providerOptions = computed(() => {
  const counts = new Map<string, number>()

  for (const m of allModels.value) {
    const key = m.provider || 'other'
    counts.set(key, (counts.get(key) || 0) + 1)
  }

  // Sort by count descending, then alphabetically
  const sorted = Array.from(counts.entries())
    .sort((a, b) => b[1] - a[1])
    .slice(0, 8)
    .map(([value, count]) => ({ value, label: providerLabel(value), count }))

  return [
    { value: 'all', label: t('models.allProviders'), count: allModels.value.length },
    ...sorted,
  ]
})

const modeOptions = computed(() => [
  { value: 'all', label: t('models.allModes') },
  { value: 'chat', label: 'Chat' },
  { value: 'image', label: 'Image' },
])

const filteredModels = computed(() => {
  let result = allModels.value

  if (selectedProvider.value !== 'all') {
    result = result.filter(m => (m.provider || 'other') === selectedProvider.value)
  }

  if (selectedMode.value !== 'all') {
    result = result.filter(m => m.mode === selectedMode.value)
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
    allModels.value = res.models ?? []
  } catch (err) {
    console.error('[ModelsView] Failed to load models:', err)
    allModels.value = []
  } finally {
    loading.value = false
  }
}

// ─── Formatters ──────────────────────────
function formatPrice(costPerToken: number): string {
  if (!costPerToken || costPerToken <= 0) return '--'
  const per1k = costPerToken * 1000
  if (per1k >= 0.01) return `$${per1k.toFixed(2)}`
  if (per1k >= 0.0001) return `$${(per1k * 1000).toFixed(2)}`
  return `$${(per1k * 1000000).toFixed(0)} / MTok`
}

function formatNumber(n: number | undefined): string {
  if (!n) return '--'
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`
  return String(n)
}

// ─── Helpers ────────────────────────────
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
