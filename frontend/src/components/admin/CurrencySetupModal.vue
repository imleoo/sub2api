<template>
  <Teleport to="body">
    <div v-if="show" class="fixed inset-0 z-[9999] flex items-center justify-center bg-black/60">
      <div class="w-full max-w-md rounded-2xl bg-white p-8 shadow-2xl dark:bg-dark-800">
        <h2 class="mb-2 text-xl font-semibold text-gray-900 dark:text-white">
          {{ t('admin.currency.setup.title') }}
        </h2>
        <p class="mb-6 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.currency.setup.description') }}
        </p>
        <div class="space-y-3">
          <button
            v-for="opt in options"
            :key="opt.mode"
            @click="selected = opt.mode"
            :class="[
              'w-full rounded-xl border-2 p-4 text-left transition-all',
              selected === opt.mode
                ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20'
                : 'border-gray-200 hover:border-gray-300 dark:border-dark-600'
            ]"
          >
            <div class="font-semibold text-gray-900 dark:text-white">{{ opt.label }}</div>
            <div class="text-sm text-gray-500 dark:text-gray-400">{{ opt.desc }}</div>
          </button>
        </div>
        <div v-if="selected === 'cny'" class="mt-4">
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.currency.cnyRate') }}
          </label>
          <input
            v-model.number="cnyRate"
            type="number"
            min="1"
            step="0.1"
            class="input w-full"
          />
        </div>
        <button
          @click="confirm"
          :disabled="!selected || saving"
          class="btn btn-primary mt-6 w-full"
        >
          {{ saving ? t('common.saving') : t('admin.currency.setup.confirm') }}
        </button>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { updateSettings, getSettings } from '@/api/admin/settings'

defineProps<{ show: boolean }>()
const emit = defineEmits<{ done: [] }>()

const { t } = useI18n()
const appStore = useAppStore()

const selected = ref<'usd' | 'cny' | ''>('')
const cnyRate = ref(7.2)
const saving = ref(false)

const options = computed(() => [
  { mode: 'usd' as const, label: t('setup.currency.usdMode'), desc: t('setup.currency.usdModeDesc') },
  { mode: 'cny' as const, label: t('setup.currency.cnyMode'), desc: t('setup.currency.cnyModeDesc') },
])

async function confirm() {
  if (!selected.value) return
  saving.value = true
  try {
    // 全量提交：/admin/settings 是全量 PUT，后端 site_name 等值类型字段无 nil-check 回落，
    // 若只发 currency 两个字段会把这些字段写空、破坏站点设置。故先拉完整当前设置，仅覆盖 currency 后整体提交。
    const current = await getSettings()
    await updateSettings({ ...current, currency_mode: selected.value, cny_rate: cnyRate.value })
    await appStore.fetchPublicSettings(true)
    emit('done')
  } catch (err) {
    console.error('[CurrencySetupModal] confirm failed:', err)
    appStore.showError(String((err as { message?: string })?.message ?? err))
  } finally {
    saving.value = false
  }
}
</script>
