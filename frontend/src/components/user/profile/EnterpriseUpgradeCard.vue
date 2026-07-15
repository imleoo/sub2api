<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-medium text-gray-900 dark:text-white">
        {{ profile ? t('enterprise.profile.title') : t('enterprise.upgrade.title') }}
      </h2>
      <p v-if="!profile" class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('enterprise.upgrade.description') }}
      </p>
    </div>
    <div class="px-6 py-6">
      <div v-if="loading" class="text-sm text-gray-400">{{ t('common.loading') }}</div>

      <!-- Already an enterprise account: show profile + management link -->
      <div v-else-if="profile" class="space-y-2 text-sm">
        <div class="flex justify-between">
          <span class="text-gray-500 dark:text-gray-400">{{ t('enterprise.profile.companyName') }}</span>
          <span class="text-gray-900 dark:text-white">{{ profile.company_name }}</span>
        </div>
        <div v-if="profile.contact_name" class="flex justify-between">
          <span class="text-gray-500 dark:text-gray-400">{{ t('enterprise.profile.contactName') }}</span>
          <span class="text-gray-900 dark:text-white">{{ profile.contact_name }}</span>
        </div>
        <div v-if="profile.contact_phone" class="flex justify-between">
          <span class="text-gray-500 dark:text-gray-400">{{ t('enterprise.profile.contactPhone') }}</span>
          <span class="text-gray-900 dark:text-white">{{ profile.contact_phone }}</span>
        </div>
        <div v-if="profile.industry" class="flex justify-between">
          <span class="text-gray-500 dark:text-gray-400">{{ t('enterprise.profile.industry') }}</span>
          <span class="text-gray-900 dark:text-white">{{ profile.industry }}</span>
        </div>
        <router-link to="/team/members" class="btn btn-secondary mt-4 inline-block">
          {{ t('enterprise.upgrade.manageLink') }}
        </router-link>
      </div>

      <!-- Not yet an enterprise account: upgrade form -->
      <form v-else @submit.prevent="handleUpgrade" class="space-y-4">
        <div>
          <label class="input-label">{{ t('enterprise.upgrade.companyNameLabel') }}</label>
          <input v-model="form.companyName" required :placeholder="t('enterprise.upgrade.companyNamePlaceholder')" class="input w-full" />
        </div>
        <div>
          <label class="input-label">{{ t('enterprise.upgrade.contactNameLabel') }}</label>
          <input v-model="form.contactName" class="input w-full" />
        </div>
        <div>
          <label class="input-label">{{ t('enterprise.upgrade.contactPhoneLabel') }}</label>
          <input v-model="form.contactPhone" class="input w-full" />
        </div>
        <div>
          <label class="input-label">{{ t('enterprise.upgrade.industryLabel') }}</label>
          <input v-model="form.industry" class="input w-full" />
        </div>
        <button type="submit" class="btn btn-primary" :disabled="submitting">
          {{ t('enterprise.upgrade.submit') }}
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { enterpriseAPI, type EnterpriseProfile } from '@/api/enterprise'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(true)
const submitting = ref(false)
const profile = ref<EnterpriseProfile | null>(null)
const form = ref({ companyName: '', contactName: '', contactPhone: '', industry: '' })

async function loadProfile() {
  loading.value = true
  try {
    profile.value = await enterpriseAPI.getProfile()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    loading.value = false
  }
}

async function handleUpgrade() {
  if (!form.value.companyName.trim()) return
  submitting.value = true
  try {
    profile.value = await enterpriseAPI.upgrade({
      company_name: form.value.companyName.trim(),
      contact_name: form.value.contactName.trim(),
      contact_phone: form.value.contactPhone.trim(),
      industry: form.value.industry.trim()
    })
    appStore.showSuccess(t('enterprise.upgrade.success'))
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    submitting.value = false
  }
}

onMounted(loadProfile)
</script>
