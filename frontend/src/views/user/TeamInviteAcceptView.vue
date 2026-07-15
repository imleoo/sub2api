<template>
  <AppLayout v-if="authStore.isAuthenticated">
    <div class="mx-auto max-w-md">
      <div class="card p-6 text-center">
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('team.invite.title') }}</h1>

        <div v-if="accepting" class="mt-6 text-sm text-gray-500 dark:text-dark-400">
          {{ t('team.invite.accepting') }}
        </div>

        <div v-else-if="success" class="mt-6 space-y-4">
          <p class="text-sm text-green-600 dark:text-green-400">{{ t('team.invite.success') }}</p>
          <RouterLink to="/team/members" class="btn btn-primary inline-flex">
            {{ t('team.invite.goToTeam') }}
          </RouterLink>
        </div>

        <div v-else-if="errorMessage" class="mt-6">
          <p class="text-sm text-red-600 dark:text-red-400">{{ errorMessage }}</p>
        </div>
      </div>
    </div>
  </AppLayout>

  <div v-else class="flex min-h-screen items-center justify-center bg-gray-50 dark:bg-dark-950">
    <div class="card w-full max-w-sm p-6 text-center">
      <p class="text-sm text-gray-600 dark:text-dark-300">{{ t('team.invite.loginRequired') }}</p>
      <RouterLink :to="loginTarget" class="btn btn-primary mt-4 inline-flex">
        {{ t('team.invite.goToLogin') }}
      </RouterLink>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, RouterLink } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAuthStore } from '@/stores/auth'
import { teamAPI } from '@/api/team'

const { t } = useI18n()
const route = useRoute()
const authStore = useAuthStore()

const accepting = ref(false)
const success = ref(false)
const errorMessage = ref('')

const token = computed(() => String(route.query.token || ''))
const loginTarget = computed(() => ({ path: '/login', query: { redirect: route.fullPath } }))

async function acceptInvitation() {
  if (!token.value) {
    errorMessage.value = t('common.error')
    return
  }
  accepting.value = true
  errorMessage.value = ''
  try {
    await teamAPI.acceptInvitation(token.value)
    success.value = true
  } catch (err) {
    errorMessage.value = (err as { message?: string })?.message || t('common.error')
  } finally {
    accepting.value = false
  }
}

onMounted(() => {
  if (authStore.isAuthenticated) {
    acceptInvitation()
  }
})
</script>
