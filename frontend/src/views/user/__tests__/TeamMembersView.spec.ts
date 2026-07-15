import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TeamMembersView from '@/views/user/TeamMembersView.vue'

const { teamState, authState, teamAPIMock } = vi.hoisted(() => ({
  teamState: {
    loaded: true,
    isTeamContext: false,
    currentTeam: null as { owner_user_id: number; owner_email: string; is_personal: boolean; role: string } | null,
    loadTeams: vi.fn()
  },
  authState: {
    user: { id: 1 }
  },
  teamAPIMock: {
    listMembers: vi.fn(),
    listInvitations: vi.fn(),
    inviteMember: vi.fn(),
    revokeInvitation: vi.fn(),
    resendInvitation: vi.fn(),
    removeMember: vi.fn()
  }
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/stores/team', () => ({
  useTeamStore: () => teamState
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState
}))

vi.mock('@/api/team', () => ({
  teamAPI: teamAPIMock
}))

function mountView() {
  return mount(TeamMembersView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true
      }
    }
  })
}

describe('TeamMembersView', () => {
  beforeEach(() => {
    teamState.loaded = true
    teamState.isTeamContext = false
    teamState.currentTeam = null
    Object.values(teamAPIMock).forEach((fn) => fn.mockReset())
    teamAPIMock.listMembers.mockResolvedValue([
      { user_id: 1, email: 'me@example.com', role: 'owner' },
      { user_id: 2, email: 'admin@example.com', role: 'admin', joined_at: '2026-01-01T00:00:00Z' }
    ])
    teamAPIMock.listInvitations.mockResolvedValue([])
  })

  it('shows the invite form and member list in personal (owner) context', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(teamAPIMock.listMembers).toHaveBeenCalled()
    expect(teamAPIMock.listInvitations).toHaveBeenCalled()
    expect(wrapper.find('input[type="email"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('me@example.com')
    expect(wrapper.text()).toContain('admin@example.com')
  })

  it('hides the invite form and only shows the owner-only hint when viewing a team as an admin', async () => {
    teamState.isTeamContext = true
    teamState.currentTeam = { owner_user_id: 10, owner_email: 'owner@example.com', is_personal: false, role: 'admin' }

    const wrapper = mountView()
    await flushPromises()

    expect(teamAPIMock.listInvitations).not.toHaveBeenCalled()
    expect(wrapper.find('input[type="email"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('team.members.ownerOnlyHint')
  })

  it('submits a new invitation and refreshes the pending invitation list', async () => {
    teamAPIMock.inviteMember.mockResolvedValue({
      id: 5,
      invited_email: 'new@example.com',
      status: 'pending',
      expires_at: '2026-02-01T00:00:00Z',
      created_at: '2026-01-25T00:00:00Z'
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('input[type="email"]').setValue('new@example.com')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(teamAPIMock.inviteMember).toHaveBeenCalledWith('new@example.com')
    expect(teamAPIMock.listInvitations).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('team.members.inviteSuccess')
  })

  it('keeps a failed invitation recoverable and can resend it', async () => {
    const pendingInvitation = {
      id: 7,
      invited_email: 'retry@example.com',
      status: 'pending',
      expires_at: '2026-02-01T00:00:00Z',
      created_at: '2026-01-25T00:00:00Z'
    }
    teamAPIMock.listInvitations
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([pendingInvitation])
    teamAPIMock.inviteMember.mockRejectedValue(new Error('mail unavailable'))
    teamAPIMock.resendInvitation.mockResolvedValue(undefined)

    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('input[type="email"]').setValue('retry@example.com')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(teamAPIMock.listInvitations).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('retry@example.com')

    const resendButton = wrapper.findAll('button').find((button) => button.text() === 'team.members.resend')
    expect(resendButton).toBeDefined()
    await resendButton!.trigger('click')
    await flushPromises()

    expect(teamAPIMock.resendInvitation).toHaveBeenCalledWith(7)
    expect(wrapper.text()).toContain('team.members.resendSuccess')
  })
})
