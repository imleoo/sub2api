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
                <th class="px-4 py-3 text-center text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ t('models.availability') }}
                </th>
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ t('models.inputPrice') }}/M
                </th>
                <th class="px-4 py-3 text-right text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">
                  {{ t('models.outputPrice') }}/M
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

                <!-- Availability Status -->
                <td class="px-4 py-3.5 text-center">
                  <div class="flex flex-col items-center gap-1">
                    <!-- Availability Badge -->
                    <span
                      v-if="model.is_available"
                      class="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2.5 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                    >
                      <svg class="h-3 w-3" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                      </svg>
                      {{ t('models.available') }}
                    </span>
                    <span
                      v-else
                      class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-500 dark:bg-gray-800 dark:text-gray-400"
                    >
                      <svg class="h-3 w-3" fill="currentColor" viewBox="0 0 20 20">
                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
                      </svg>
                      {{ t('models.unavailable') }}
                    </span>
                    <!-- Test Status Badge (only if available and has test result) -->
                    <span
                      v-if="model.is_available && model.test_status"
                      :class="[
                        'inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium',
                        model.test_status.status === 'success'
                          ? 'bg-green-50 text-green-600 dark:bg-green-900/20 dark:text-green-400'
                          : model.test_status.status === 'failed'
                            ? 'bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-400'
                            : 'bg-gray-50 text-gray-500 dark:bg-gray-800 dark:text-gray-400'
                      ]"
                    >
                      <span
                        class="h-1.5 w-1.5 rounded-full"
                        :class="[
                          model.test_status.status === 'success'
                            ? 'bg-green-500'
                            : model.test_status.status === 'failed'
                              ? 'bg-red-500'
                              : 'bg-gray-400'
                        ]"
                      ></span>
                      {{ model.test_status.status === 'success' ? t('models.testPassed') : model.test_status.status === 'failed' ? t('models.testFailed') : model.test_status.status }}
                      <span v-if="model.test_status.latency_ms" class="opacity-70">{{ model.test_status.latency_ms }}ms</span>
                    </span>
                  </div>
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
                      v-if="isImageModel(model)"
                      class="inline-flex items-center rounded-full bg-purple-50 px-2 py-0.5 text-[10px] font-medium text-purple-700 dark:bg-purple-900/30 dark:text-purple-400"
                    >
                      {{ t('models.imageMode') }}
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
  { value: 'chat', label: t('models.chatMode') },
  { value: 'image', label: t('models.imageMode') },
])

const filteredModels = computed(() => {
  let result = allModels.value

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
    // Filter models based on version requirements
    allModels.value = (res.models ?? []).filter(shouldShowModel)
  } catch (err) {
    console.error('[ModelsView] Failed to load models:', err)
    allModels.value = []
  } finally {
    loading.value = false
  }
}

/**
 * Determine if a model should be shown to users based on version requirements:
 * - Claude: 4.5+
 * - GPT: 5.2+
 * - Gemini: 3+
 * - GLM: 5+
 * - Others: hidden
 * - Unavailable models: hidden
 */
function shouldShowModel(model: ModelInfo): boolean {
  // Hide unavailable models
  if (!model.is_available) {
    return false
  }

  if (isImageModel(model)) {
    return true
  }

  const id = model.id.toLowerCase()
  const provider = model.provider?.toLowerCase() || ''

  // Claude models (anthropic provider or model id starts with claude)
  if (provider === 'anthropic' || id.startsWith('claude-')) {
    return isClaudeVersionAtLeast(id, 4.5)
  }

  // GPT models (openai provider or model id starts with gpt-/o1/o3/o4)
  if (provider === 'openai' || id.startsWith('gpt-') || id.match(/^o[1-4]/)) {
    return isGPTVersionAtLeast(id, 5.2)
  }

  // Gemini models (google provider or model id starts with gemini)
  if (provider === 'google' || id.startsWith('gemini-')) {
    return isGeminiVersionAtLeast(id, 3)
  }

  // GLM models (zhipu provider or model id starts with glm)
  if (provider === 'zhipu' || id.startsWith('glm-')) {
    return isGLMVersionAtLeast(id, 5)
  }

  // Other providers: hidden
  return false
}

/**
 * Parse Claude model version from id
 * Returns version number (e.g., 4.5, 4.6, 3.5) or 0 if cannot parse
 */
function parseClaudeVersion(id: string): number {
  // claude-sonnet-4-5-20250929 -> 4.5
  // claude-sonnet-4-6 -> 4.6
  // claude-opus-4-6 -> 4.6
  // claude-3-5-sonnet-20241022 -> 3.5
  // claude-3-opus-20240229 -> 3.0

  // Match pattern: claude-{major}-{minor}-{...} or claude-{family}-{major}-{minor}
  const match = id.match(/claude-(?:\w+-)?(\d+)(?:[-.](\d+))?/)
  if (match) {
    const major = parseInt(match[1], 10)
    const minor = match[2] ? parseInt(match[2], 10) : 0
    return major + minor / 10
  }
  return 0
}

function isClaudeVersionAtLeast(id: string, minVersion: number): boolean {
  return parseClaudeVersion(id) >= minVersion
}

/**
 * Parse GPT model version from id
 * Returns version number (e.g., 5.2, 5.4) or 0 if cannot parse
 */
function parseGPTVersion(id: string): number {
  // gpt-5.2 -> 5.2
  // gpt-5.4 -> 5.4
  // gpt-5 -> 5.0
  // gpt-4o -> 4.0 (o series counted as 4.x)
  // o1, o3, o4-mini -> treat as legacy/o-series, return 0

  // Match gpt-{major}.{minor} or gpt-{major}
  const gptMatch = id.match(/gpt-(\d+)(?:\.(\d+))?/)
  if (gptMatch) {
    const major = parseInt(gptMatch[1], 10)
    const minor = gptMatch[2] ? parseInt(gptMatch[2], 10) : 0
    return major + minor / 10
  }

  // o-series (o1, o3, o4-mini) are below 5.2 threshold
  if (id.match(/^o[1-4]/)) {
    return 0
  }

  return 0
}

function isGPTVersionAtLeast(id: string, minVersion: number): boolean {
  return parseGPTVersion(id) >= minVersion
}

/**
 * Parse Gemini model version from id
 * Returns version number (e.g., 3.0, 3.1, 2.5) or 0 if cannot parse
 */
function parseGeminiVersion(id: string): number {
  // gemini-3-flash -> 3.0
  // gemini-3.1-flash-image -> 3.1
  // gemini-2.5-flash -> 2.5
  // gemini-3-pro-preview -> 3.0

  const match = id.match(/gemini-(\d+)(?:\.(\d+))?/)
  if (match) {
    const major = parseInt(match[1], 10)
    const minor = match[2] ? parseInt(match[2], 10) : 0
    return major + minor / 10
  }
  return 0
}

function isGeminiVersionAtLeast(id: string, minVersion: number): boolean {
  return parseGeminiVersion(id) >= minVersion
}

/**
 * Parse GLM model version from id
 * Returns version number (e.g., 5.0, 4.5) or 0 if cannot parse
 */
function parseGLMVersion(id: string): number {
  // glm-4 -> 4.0
  // glm-4.5 -> 4.5
  // glm-4.6 -> 4.6
  // glm-5 -> 5.0

  const match = id.match(/glm-(\d+)(?:\.(\d+))?/)
  if (match) {
    const major = parseInt(match[1], 10)
    const minor = match[2] ? parseInt(match[2], 10) : 0
    return major + minor / 10
  }
  return 0
}

function isGLMVersionAtLeast(id: string, minVersion: number): boolean {
  return parseGLMVersion(id) >= minVersion
}

// ─── Formatters ──────────────────────────
function formatPrice(costPerToken: number): string {
  if (!costPerToken || costPerToken <= 0) return '--'
  
  // costPerToken is the price per single token
  // Convert to price per 1M tokens for display
  const per1m = costPerToken * 1_000_000
  
  if (per1m >= 1) {
    // >= $1/1M tokens: show as $X.XX
    return `$${per1m.toFixed(2)}`
  } else if (per1m >= 0.1) {
    // >= $0.1/1M tokens: show as $0.XX
    return `$${per1m.toFixed(2)}`
  } else {
    // < $0.1/1M tokens: show more precision
    return `$${per1m.toFixed(3)}`
  }
}

function formatNumber(n: number | undefined): string {
  if (!n) return '--'
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`
  return String(n)
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
