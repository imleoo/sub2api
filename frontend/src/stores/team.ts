/**
 * Enterprise / Team Store
 * 企业组织与额度分配（Team 协作 v2，zhiguofan fork-only）
 *
 * v2 语义：不再有"上下文切换"（X-Team-Id 已废弃）。本 store 只承载：
 * - 我的企业归属信息（我是 owner 的企业 / 我作为成员加入的企业）
 * - 企业管理页所需的成员/邀请等数据由页面组件按需经 teamAPI 拉取
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { teamAPI, type TeamSummary } from '@/api/team'

export const useTeamStore = defineStore('team', () => {
  const teams = ref<TeamSummary[]>([])
  const loaded = ref(false)

  /** 我作为成员加入的企业（不含自己的个人账户行） */
  const joinedTeams = computed(() => teams.value.filter((t) => !t.is_personal))

  /** 我作为 admin 加入、可管理的别人的企业（owner 需另外通过企业资料判断，见 EnterpriseUpgradeCard）。 */
  const manageableJoinedTeams = computed(() => joinedTeams.value.filter((t) => t.role === 'admin'))

  async function loadTeams(): Promise<void> {
    try {
      teams.value = await teamAPI.listMyTeams()
    } finally {
      loaded.value = true
    }
  }

  function reset(): void {
    teams.value = []
    loaded.value = false
  }

  return {
    teams,
    joinedTeams,
    manageableJoinedTeams,
    loaded,
    loadTeams,
    reset
  }
})
