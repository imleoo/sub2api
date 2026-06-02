<template>
  <ProfileIdentityBindingsSection
    :user="user"
    :phone-register-enabled="phoneRegisterEnabled"
    :linuxdo-enabled="linuxdoEnabled"
    :dingtalk-enabled="dingtalkEnabled"
    :oidc-enabled="oidcEnabled"
    :oidc-provider-name="oidcProviderName"
    :wechat-enabled="wechatEnabled"
    :wechat-open-enabled="wechatOpenEnabled"
    :wechat-mp-enabled="wechatMpEnabled"
  />
</template>

<script setup lang="ts">
import ProfileIdentityBindingsSection from '@/components/user/profile/ProfileIdentityBindingsSection.vue'
import type { User } from '@/types'

withDefaults(
  defineProps<{
    user: User | null
    phoneRegisterEnabled?: boolean
    linuxdoEnabled?: boolean
    dingtalkEnabled?: boolean
    oidcEnabled?: boolean
    oidcProviderName?: string
    wechatEnabled?: boolean
    wechatOpenEnabled?: boolean
    wechatMpEnabled?: boolean
  }>(),
  {
    phoneRegisterEnabled: false,
    linuxdoEnabled: false,
    dingtalkEnabled: false,
    oidcEnabled: false,
    oidcProviderName: 'OIDC',
    wechatEnabled: false,
    wechatOpenEnabled: undefined,
    wechatMpEnabled: undefined
  }
)
</script>
