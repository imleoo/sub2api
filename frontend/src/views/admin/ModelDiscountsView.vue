<template>
  <AppLayout>
    <div class="p-6">
      <div class="mb-6 flex items-center justify-between">
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
          {{ t('nav.modelDiscounts') }}
        </h1>
        <button class="btn btn-primary" :disabled="saving" @click="save">
          {{ saving ? t('common.saving', 'Saving...') : t('common.save', 'Save') }}
        </button>
      </div>

      <div class="mb-4">
        <input
          v-model="search"
          type="text"
          :placeholder="t('common.search', 'Search...')"
          class="input w-full sm:w-64"
        />
      </div>

      <div v-if="loading" class="py-12 text-center text-gray-500">{{ t('common.loading', 'Loading...') }}</div>

      <div v-else class="overflow-x-auto rounded-lg border border-gray-200 dark:border-gray-700">
        <table class="w-full text-sm">
          <thead class="bg-gray-50 dark:bg-gray-800">
            <tr>
              <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">{{ t('common.model', 'Model') }}</th>
              <th class="px-4 py-3 text-left font-medium text-gray-600 dark:text-gray-300">{{ t('common.provider', 'Provider') }}</th>
              <th class="px-4 py-3 text-right font-medium text-gray-600 dark:text-gray-300">{{ t('models.inputPrice', 'Input') }}</th>
              <th class="px-4 py-3 text-right font-medium text-gray-600 dark:text-gray-300">{{ t('models.outputPrice', 'Output') }}</th>
              <th class="px-4 py-3 text-center font-medium text-gray-600 dark:text-gray-300" style="width:140px">
                {{ t('nav.modelDiscounts') }} (0~1)
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-gray-700">
            <tr
              v-for="m in filtered"
              :key="m.id"
              class="bg-white hover:bg-gray-50 dark:bg-gray-900 dark:hover:bg-gray-800"
            >
              <td class="px-4 py-3 font-mono text-xs text-gray-800 dark:text-gray-200">{{ m.id }}</td>
              <td class="px-4 py-3 text-gray-600 dark:text-gray-400">{{ m.provider }}</td>
              <td class="px-4 py-3 text-right text-gray-600 dark:text-gray-400">
                {{ formatPrice(m.input_cost_per_token) }}
              </td>
              <td class="px-4 py-3 text-right text-gray-600 dark:text-gray-400">
                {{ formatPrice(m.output_cost_per_token) }}
              </td>
              <td class="px-4 py-3 text-center">
                <input
                  v-model.number="discounts[m.id]"
                  type="number"
                  min="0.01"
                  max="1"
                  step="0.05"
                  :placeholder="'1.0'"
                  class="input w-24 text-center text-sm"
                />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getModelDiscounts, updateModelDiscounts } from '@/api/admin/modelDiscounts'
import type { ModelInfo } from '@/api/admin/modelDiscounts'
import { useAppStore } from '@/stores/app'
import { formatUSD } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

// 已知海外 AI 服务商黑名单（与 ModelsView 保持同步）
// 使用黑名单而非白名单，确保用户自定义添加的国内供应商不被误过滤
const OVERSEAS_PROVIDERS = new Set([
  'anthropic', 'openai', 'google', 'gemini',
  'vertex_ai', 'vertex_ai-language-models', 'vertex_ai-vision-models', 'vertex_ai-embedding-models',
  'bedrock', 'text-completion-openai',
  'mistral', 'meta', 'cohere', 'xai', 'perplexity',
])

const showOverseasModels = computed(
  () => appStore.cachedPublicSettings?.show_overseas_models !== false
)

const loading = ref(true)
const saving = ref(false)
const search = ref('')
const models = ref<ModelInfo[]>([])
const discounts = ref<Record<string, number>>({})

const filtered = computed(() => {
  let result = models.value

  if (!showOverseasModels.value) {
    result = result.filter(m => !OVERSEAS_PROVIDERS.has((m.provider || '').toLowerCase()))
  }

  const q = search.value.toLowerCase()
  return q ? result.filter(m => m.id.toLowerCase().includes(q) || m.provider.toLowerCase().includes(q)) : result
})

function formatPrice(v: number) {
  if (!v) return '-'
  return `${formatUSD(v * 1_000_000, 4)}/MTok`
}

async function load() {
  loading.value = true
  try {
    const res = await getModelDiscounts()
    models.value = res.data.models ?? []
    discounts.value = { ...res.data.discounts }
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    const payload: Record<string, number> = {}
    for (const [k, v] of Object.entries(discounts.value)) {
      if (v && v > 0 && v < 1) payload[k] = v
    }
    await updateModelDiscounts(payload)
    appStore.showSuccess(t('common.saveSuccess', 'Saved'))
  } catch {
    appStore.showError(t('common.saveFailed', 'Save failed'))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
