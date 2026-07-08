<template>
  <div>
    <div class="flex items-center justify-between">
      <label class="input-label mb-0">{{ t('admin.accounts.generic.supportedModels') }}</label>
      <button
        type="button"
        :disabled="loading || !baseUrl?.trim() || (requireApiKey && !apiKey?.trim())"
        @click="fetchModels"
        class="rounded-md bg-purple-50 px-2 py-1 text-xs font-medium text-purple-700 hover:bg-purple-100 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-purple-900/20 dark:text-purple-400 dark:hover:bg-purple-900/30"
      >
        {{ loading ? t('admin.accounts.generic.fetchModelsLoading') : t('admin.accounts.generic.fetchModels') }}
      </button>
    </div>

    <!-- 可搜索复选清单：拉取结果 ∪ 已选，用户勾选要暴露的子集 -->
    <div v-if="displayModels.length > 0" class="mt-1 rounded-lg border border-gray-200 dark:border-dark-600">
      <div class="flex items-center gap-2 border-b border-gray-200 p-2 dark:border-dark-600">
        <input
          v-model="filter"
          type="text"
          class="input h-8 flex-1 text-xs"
          :placeholder="t('admin.accounts.generic.modelPickerSearch')"
        />
        <button type="button" class="whitespace-nowrap text-xs text-primary-600 hover:underline dark:text-primary-400" @click="selectAllFiltered">
          {{ t('admin.accounts.generic.modelPickerSelectAll') }}
        </button>
        <button type="button" class="whitespace-nowrap text-xs text-gray-500 hover:underline dark:text-gray-400" @click="clearFiltered">
          {{ t('admin.accounts.generic.modelPickerClear') }}
        </button>
      </div>
      <div class="max-h-52 overflow-y-auto p-1">
        <label
          v-for="m in filteredModels"
          :key="m"
          class="flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-xs hover:bg-gray-50 dark:hover:bg-dark-700"
        >
          <input type="checkbox" :checked="selectedSet.has(m)" class="h-3.5 w-3.5" @change="toggle(m)" />
          <span class="font-mono">{{ m }}</span>
        </label>
        <p v-if="filteredModels.length === 0" class="px-2 py-2 text-xs text-gray-400">
          {{ t('admin.accounts.generic.modelPickerNoMatch') }}
        </p>
      </div>
      <div class="border-t border-gray-200 px-2 py-1 text-[11px] text-gray-500 dark:border-dark-600 dark:text-gray-400">
        {{ t('admin.accounts.generic.modelPickerSelected', { n: modelValue.length, total: displayModels.length }) }}
      </div>
    </div>

    <!-- 空态提示 -->
    <p v-else class="input-hint mt-1">{{ t('admin.accounts.generic.modelPickerEmpty') }}</p>

    <!-- 手动输入 / 高级（兜底：加清单外的模型 ID） -->
    <button
      type="button"
      class="mt-1 text-[11px] text-gray-500 hover:underline dark:text-gray-400"
      @click="manualOpen = !manualOpen"
    >
      {{ manualOpen ? '▾' : '▸' }} {{ t('admin.accounts.generic.modelPickerManual') }}
    </button>
    <textarea
      v-if="manualOpen"
      :value="modelValue.join(', ')"
      @input="onManualInput(($event.target as HTMLTextAreaElement).value)"
      rows="2"
      class="input mt-1 font-mono"
      :placeholder="t('admin.accounts.generic.supportedModelsPlaceholder')"
    />
    <p v-if="manualOpen" class="input-hint">{{ t('admin.accounts.generic.supportedModelsHint') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'

const props = defineProps<{
  modelValue: string[]
  baseUrl: string
  apiKey?: string
  accountId?: number
  requireApiKey?: boolean
}>()

const emit = defineEmits<{ (e: 'update:modelValue', value: string[]): void }>()

const { t } = useI18n()
const appStore = useAppStore()

const fetched = ref<string[]>([])
const filter = ref('')
const loading = ref(false)
const manualOpen = ref(false)

const selectedSet = computed(() => new Set(props.modelValue))

// 清单 = 拉取结果 ∪ 已选（已选但不在拉取结果里的也要显示，避免勾选丢失），去重排序。
const displayModels = computed(() => {
  const set = new Set<string>([...fetched.value, ...props.modelValue])
  return Array.from(set).sort((a, b) => a.localeCompare(b))
})

const filteredModels = computed(() => {
  const kw = filter.value.trim().toLowerCase()
  if (!kw) return displayModels.value
  return displayModels.value.filter((m) => m.toLowerCase().includes(kw))
})

function emitUnique(list: string[]) {
  emit('update:modelValue', Array.from(new Set(list.map((s) => s.trim()).filter(Boolean))))
}

function toggle(model: string) {
  if (selectedSet.value.has(model)) {
    emitUnique(props.modelValue.filter((m) => m !== model))
  } else {
    emitUnique([...props.modelValue, model])
  }
}

function selectAllFiltered() {
  emitUnique([...props.modelValue, ...filteredModels.value])
}

function clearFiltered() {
  const drop = new Set(filteredModels.value)
  emitUnique(props.modelValue.filter((m) => !drop.has(m)))
}

// parseGenericSupportedModels 同款：逗号/换行/空格分隔 → 去重数组。
function onManualInput(raw: string) {
  emitUnique(raw.split(/[\s,]+/))
}

async function fetchModels() {
  if (!props.baseUrl?.trim()) {
    appStore.showError(t('admin.accounts.generic.fetchModelsNeedBaseUrl'))
    return
  }
  const key = props.apiKey?.trim()
  if (props.requireApiKey && !key) {
    appStore.showError(t('admin.accounts.generic.fetchModelsNeedApiKey'))
    return
  }
  loading.value = true
  try {
    const res = await adminAPI.accounts.fetchEndpointModels({
      base_url: props.baseUrl.trim(),
      ...(key ? { api_key: key } : props.accountId != null ? { account_id: props.accountId } : {})
    })
    fetched.value = Array.from(new Set([...fetched.value, ...res.models]))
    appStore.showSuccess(t('admin.accounts.generic.fetchModelsSuccessPick', { count: res.fetched }))
  } catch {
    appStore.showError(t('admin.accounts.generic.fetchModelsFailed'))
  } finally {
    loading.value = false
  }
}
</script>
