import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TeamMembersView from '@/views/user/TeamMembersView.vue'

const { teamState, authState, teamAPIMock } = vi.hoisted(() => ({
  teamState: {
    loaded: true,
    joinedTeams: [] as { owner_user_id: number; owner_email: string; is_personal: boolean; role: string }[],
    manageableJoinedTeams: [] as { owner_user_id: number; owner_email: string; is_personal: boolean; role: string }[],
    loadTeams: vi.fn()
  },
  authState: {
    user: { id: 1 }
  },
  teamAPIMock: {
    listMyTeams: vi.fn(),
    inviteMember: vi.fn(),
    listInvitations: vi.fn(),
    listReceivedInvitations: vi.fn(),
    acceptInvitation: vi.fn(),
    revokeInvitation: vi.fn(),
    resendInvitation: vi.fn(),
    listMembers: vi.fn(),
    removeMember: vi.fn(),
    setMemberDepartment: vi.fn(),
    setMemberQuotaSettings: vi.fn(),
    setMemberRole: vi.fn(),
    grantToMember: vi.fn(),
    reclaimFromMember: vi.fn(),
    listTransfers: vi.fn(),
    getReport: vi.fn(),
    getMemberUsageStats: vi.fn(),
    listDepartments: vi.fn(),
    createDepartment: vi.fn(),
    updateDepartment: vi.fn(),
    deleteDepartment: vi.fn()
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
        BaseDialog: true,
        UserStatsModal: true,
        PlatformUsageBreakdown: true,
        Icon: true
      }
    }
  })
}

function findTab(wrapper: ReturnType<typeof mountView>, label: string) {
  const tab = wrapper.findAll('button').find((button) => button.text() === label)
  expect(tab).toBeDefined()
  return tab!
}

describe('TeamMembersView', () => {
  beforeEach(() => {
    teamState.loaded = true
    teamState.joinedTeams = []
    teamState.manageableJoinedTeams = []
    Object.values(teamAPIMock).forEach((fn) => fn.mockReset())
    teamAPIMock.listMembers.mockResolvedValue([
      { user_id: 1, email: 'me@example.com', role: 'owner', granted_net_usd: 0 },
      {
        user_id: 2,
        email: 'admin@example.com',
        role: 'admin',
        granted_net_usd: 0,
        joined_at: '2026-01-01T00:00:00Z'
      }
    ])
    teamAPIMock.listInvitations.mockResolvedValue([])
    teamAPIMock.listReceivedInvitations.mockResolvedValue([])
    teamAPIMock.listDepartments.mockResolvedValue([])
    teamAPIMock.listTransfers.mockResolvedValue({ items: [], total: 0 })
    teamAPIMock.getReport.mockResolvedValue([])
  })

  it('loads members, invitations, departments, transfers and report on mount', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(teamAPIMock.listMembers).toHaveBeenCalledWith(undefined)
    expect(teamAPIMock.listInvitations).toHaveBeenCalledWith(undefined)
    expect(teamAPIMock.listDepartments).toHaveBeenCalledWith(undefined)
    expect(teamAPIMock.listTransfers).toHaveBeenCalled()
    expect(teamAPIMock.getReport).toHaveBeenCalledWith(undefined)
    expect(wrapper.text()).toContain('me@example.com')
    expect(wrapper.text()).toContain('admin@example.com')
  })

  it('shows received invitations and accepts one in-app via its token', async () => {
    teamAPIMock.listReceivedInvitations.mockResolvedValue([
      {
        id: 9,
        owner_user_id: 5,
        owner_email: 'boss@example.com',
        role: 'member',
        quota_mode: 'allocated',
        token: 'tok-abc',
        expires_at: '2099-01-01T00:00:00Z',
        created_at: '2026-01-01T00:00:00Z'
      }
    ])
    teamAPIMock.acceptInvitation.mockResolvedValue({ owner_user_id: 5, role: 'member' })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('team.members.receivedFrom')
    const acceptButton = wrapper.findAll('button').find((b) => b.text() === 'team.members.receivedAccept')
    expect(acceptButton).toBeDefined()
    await acceptButton!.trigger('click')
    await flushPromises()

    expect(teamAPIMock.acceptInvitation).toHaveBeenCalledWith('tok-abc')
    expect(teamState.loadTeams).toHaveBeenCalled()
  })

  it('submits a new invitation from the invitations tab and refreshes the pending list', async () => {
    teamAPIMock.inviteMember.mockResolvedValue({
      id: 5,
      invited_email: 'new@example.com',
      status: 'pending',
      role: 'member',
      quota_mode: 'allocated',
      expires_at: '2026-02-01T00:00:00Z',
      created_at: '2026-01-25T00:00:00Z'
    })

    const wrapper = mountView()
    await flushPromises()

    await findTab(wrapper, 'team.members.tabInvitations').trigger('click')
    await wrapper.find('input[type="email"]').setValue('new@example.com')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(teamAPIMock.inviteMember).toHaveBeenCalledWith(
      {
        email: 'new@example.com',
        department_id: null,
        role: 'member',
        quota_mode: 'allocated',
        initial_grant_usd: null
      },
      undefined
    )
    expect(teamAPIMock.listInvitations).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('team.members.inviteSuccess')
  })

  it('creates a new department from the departments tab', async () => {
    teamAPIMock.createDepartment.mockResolvedValue({ id: 1, name: 'Engineering', display_order: 0 })

    const wrapper = mountView()
    await flushPromises()

    await findTab(wrapper, 'team.members.tabDepartments').trigger('click')
    await wrapper.find('input').setValue('Engineering')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(teamAPIMock.createDepartment).toHaveBeenCalledWith('Engineering', 0, undefined)
    expect(teamAPIMock.listDepartments).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('team.departments.createSuccess')
  })

  it('resends a pending invitation', async () => {
    teamAPIMock.listInvitations.mockResolvedValue([
      {
        id: 7,
        invited_email: 'retry@example.com',
        status: 'pending',
        role: 'member',
        quota_mode: 'allocated',
        expires_at: '2026-02-01T00:00:00Z',
        created_at: '2026-01-25T00:00:00Z'
      }
    ])
    teamAPIMock.resendInvitation.mockResolvedValue({ resent: true })

    const wrapper = mountView()
    await flushPromises()

    await findTab(wrapper, 'team.members.tabInvitations').trigger('click')
    expect(wrapper.text()).toContain('retry@example.com')

    const resendButton = wrapper.findAll('button').find((button) => button.text() === 'team.members.resend')
    expect(resendButton).toBeDefined()
    await resendButton!.trigger('click')
    await flushPromises()

    expect(teamAPIMock.resendInvitation).toHaveBeenCalledWith(7, undefined)
    expect(wrapper.text()).toContain('team.members.resendSuccess')
  })
})
