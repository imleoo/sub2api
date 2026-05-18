<template>
  <div>
    <!-- Multi-select Dropdown -->
    <div class="relative mb-3">
      <div
        @click="toggleDropdown"
        class="cursor-pointer rounded-lg border border-gray-300 bg-white px-3 py-2 dark:border-dark-500 dark:bg-dark-700"
      >
        <div class="grid grid-cols-2 gap-1.5">
          <span
            v-for="model in modelValue"
            :key="model"
            class="inline-flex items-center justify-between gap-1 rounded bg-gray-100 px-2 py-1 text-xs text-gray-700 dark:bg-dark-600 dark:text-gray-300"
          >
            <span class="flex items-center gap-1 truncate">
              <ModelIcon :model="model" size="14px" />
              <span class="truncate">{{ model }}</span>
            </span>
            <button
              type="button"
              @click.stop="removeModel(model)"
              class="shrink-0 rounded-full hover:bg-gray-200 dark:hover:bg-dark-500"
            >
              <Icon name="x" size="xs" class="h-3.5 w-3.5" :stroke-width="2" />
            </button>
          </span>
        </div>
        <div class="mt-2 flex items-center justify-between border-t border-gray-200 pt-2 dark:border-dark-600">
          <span class="text-xs text-gray-400">{{ t('admin.accounts.modelCount', { count: modelValue.length }) }}</span>
          <svg class="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
          </svg>
        </div>
      </div>
      <!-- Dropdown List -->
      <div
        v-if="showDropdown"
        class="absolute left-0 right-0 top-full z-50 mt-1 rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-600 dark:bg-dark-700"
      >
        <div class="sticky top-0 border-b border-gray-200 bg-white p-2 dark:border-dark-600 dark:bg-dark-700">
          <input
            v-model="searchQuery"
            type="text"
            class="input w-full text-sm"
            :placeholder="t('admin.accounts.searchModels')"
            @click.stop
          />
        </div>
        <div class="max-h-52 overflow-auto">
          <button
            v-for="model in filteredModels"
            :key="model.model_id"
            type="button"
            @click="toggleModel(model.model_id)"
            class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-dark-600"
          >
            <span
              :class="[
                'flex h-4 w-4 shrink-0 items-center justify-center rounded border',
                modelValue.includes(model.model_id)
                  ? 'border-primary-500 bg-primary-500 text-white'
                  : 'border-gray-300 dark:border-dark-500'
              ]"
            >
              <svg v-if="modelValue.includes(model.model_id)" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7" />
              </svg>
            </span>
            <ModelIcon :model="model.model_id" size="18px" />
            <span class="truncate font-mono text-xs text-gray-900 dark:text-white">{{ model.model_id }}</span>
            <span class="ml-auto shrink-0 rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500 dark:bg-dark-600 dark:text-gray-400">{{ model.provider }}</span>
            <span class="shrink-0 rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500 dark:bg-dark-600 dark:text-gray-400">{{ model.mode }}</span>
          </button>
          <div v-if="searchLoading" class="px-3 py-4 text-center text-sm text-gray-500">
            加载中...
          </div>
          <div v-else-if="filteredModels.length === 0" class="px-3 py-4 text-center text-sm text-gray-500">
            {{ t('admin.accounts.noMatchingModels') }}
          </div>
        </div>
      </div>
    </div>

    <!-- Quick Actions -->
    <div class="mb-4 flex flex-wrap gap-2">
      <button
        type="button"
        @click="fillRelated"
        class="rounded-lg border border-blue-200 px-3 py-1.5 text-sm text-blue-600 hover:bg-blue-50 dark:border-blue-800 dark:text-blue-400 dark:hover:bg-blue-900/30"
      >
        {{ t('admin.accounts.fillRelatedModels') }}
      </button>
      <button
        type="button"
        @click="clearAll"
        class="rounded-lg border border-red-200 px-3 py-1.5 text-sm text-red-600 hover:bg-red-50 dark:border-red-800 dark:text-red-400 dark:hover:bg-red-900/30"
      >
        {{ t('admin.accounts.clearAllModels') }}
      </button>
    </div>

    <!-- Custom Model Input -->
    <div class="mb-3">
      <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.accounts.customModelName') }}</label>
      <div class="flex gap-2">
        <input
          v-model="customModel"
          type="text"
          class="input flex-1"
          :placeholder="t('admin.accounts.enterCustomModelName')"
          @keydown.enter.prevent="handleEnter"
          @compositionstart="isComposing = true"
          @compositionend="isComposing = false"
        />
        <button
          type="button"
          @click="addCustom"
          class="rounded-lg bg-primary-50 px-4 py-2 text-sm font-medium text-primary-600 hover:bg-primary-100 dark:bg-primary-900/30 dark:text-primary-400 dark:hover:bg-primary-900/50"
        >
          {{ t('admin.accounts.addModel') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import ModelIcon from '@/components/common/ModelIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import { listModelPricings, type DBModelPricing } from '@/api/admin/modelPricings'

const { t } = useI18n()

const props = defineProps<{
  modelValue: string[]
  platform?: string
  platforms?: string[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const appStore = useAppStore()

const showDropdown = ref(false)
const searchQuery = ref('')
const customModel = ref('')
const isComposing = ref(false)
const searchLoading = ref(false)
const searchResults = ref<DBModelPricing[]>([])

let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null

// Build provider filter from platform/platforms props (best-effort mapping)
const getProviderParam = (): string | undefined => {
  const rawPlatforms =
    props.platforms && props.platforms.length > 0
      ? props.platforms
      : props.platform
        ? [props.platform]
        : []
  if (rawPlatforms.length === 0) return undefined
  // Return the first platform as provider hint (lowercase)
  return rawPlatforms[0]?.toLowerCase()
}

const fetchModels = async (q: string) => {
  searchLoading.value = true
  try {
    const params: Record<string, unknown> = {
      is_enabled: true,
      page: 1,
      page_size: 50,
    }
    if (q) params.q = q
    const provider = getProviderParam()
    if (provider) params.provider = provider

    const { data } = await listModelPricings(params)
    searchResults.value = data.items ?? []
  } catch {
    searchResults.value = []
  } finally {
    searchLoading.value = false
  }
}

const filteredModels = ref<DBModelPricing[]>([])

watch(searchQuery, (q) => {
  if (searchDebounceTimer) clearTimeout(searchDebounceTimer)
  searchDebounceTimer = setTimeout(() => {
    fetchModels(q)
  }, 300)
})

watch(searchResults, (results) => {
  filteredModels.value = results
})

const toggleDropdown = () => {
  showDropdown.value = !showDropdown.value
  if (showDropdown.value) {
    // Initial fetch when opening
    fetchModels(searchQuery.value)
  } else {
    searchQuery.value = ''
  }
}

const removeModel = (model: string) => {
  emit('update:modelValue', props.modelValue.filter(m => m !== model))
}

const toggleModel = (modelId: string) => {
  if (props.modelValue.includes(modelId)) {
    removeModel(modelId)
  } else {
    emit('update:modelValue', [...props.modelValue, modelId])
  }
}

const addCustom = () => {
  const model = customModel.value.trim()
  if (!model) return
  if (props.modelValue.includes(model)) {
    appStore.showInfo(t('admin.accounts.modelExists'))
    return
  }
  emit('update:modelValue', [...props.modelValue, model])
  customModel.value = ''
}

const handleEnter = () => {
  if (!isComposing.value) addCustom()
}

const fillRelated = async () => {
  // Fetch all enabled models for the platform and add any not already selected
  searchLoading.value = true
  try {
    const params: Record<string, unknown> = { is_enabled: true, page: 1, page_size: 200 }
    const provider = getProviderParam()
    if (provider) params.provider = provider
    const { data } = await listModelPricings(params)
    const newModels = [...props.modelValue]
    for (const m of data.items ?? []) {
      if (!newModels.includes(m.model_id)) {
        newModels.push(m.model_id)
      }
    }
    emit('update:modelValue', newModels)
  } catch {
    appStore.showError('获取模型列表失败')
  } finally {
    searchLoading.value = false
  }
}

const clearAll = () => {
  emit('update:modelValue', [])
}
</script>
