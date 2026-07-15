import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TeamInviteAcceptView from '@/views/user/TeamInviteAcceptView.vue'

const routeState = vi.hoisted(() => ({
  query: {} as Record<string, unknown>,
  fullPath: '/team/invite/accept'
}))

const { authState, acceptInvitation } = vi.hoisted(() => ({
  authState: { isAuthenticated: false },
  acceptInvitation: vi.fn()
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => routeState
  }
})

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState
}))

vi.mock('@/api/team', () => ({
  teamAPI: {
    acceptInvitation: (...args: any[]) => acceptInvitation(...args)
  }
}))

function mountView() {
  return mount(TeamInviteAcceptView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        RouterLink: { template: '<a :data-to="JSON.stringify(to)"><slot /></a>', props: ['to'] },
        Icon: true
      }
    }
  })
}

describe('TeamInviteAcceptView', () => {
  beforeEach(() => {
    routeState.query = {}
    authState.isAuthenticated = false
    acceptInvitation.mockReset()
  })

  it('prompts login when the user is not authenticated, without calling acceptInvitation', async () => {
    routeState.query = { token: 'tok-123' }
    routeState.fullPath = '/team/invite/accept?token=tok-123'
    authState.isAuthenticated = false

    const wrapper = mountView()
    await flushPromises()

    expect(acceptInvitation).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('team.invite.loginRequired')
    const loginLink = wrapper.find('a')
    expect(JSON.parse(loginLink.attributes('data-to'))).toEqual({
      path: '/login',
      query: { redirect: '/team/invite/accept?token=tok-123' }
    })
  })

  it('accepts the invitation automatically on mount when authenticated', async () => {
    routeState.query = { token: 'tok-123' }
    authState.isAuthenticated = true
    acceptInvitation.mockResolvedValue({ owner_user_id: 10, role: 'admin' })

    const wrapper = mountView()
    await flushPromises()

    expect(acceptInvitation).toHaveBeenCalledWith('tok-123')
    expect(wrapper.text()).toContain('team.invite.success')
  })

  it('shows the error message when accepting fails', async () => {
    routeState.query = { token: 'tok-expired' }
    authState.isAuthenticated = true
    acceptInvitation.mockRejectedValue({ message: 'invitation has expired' })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('invitation has expired')
    expect(wrapper.text()).not.toContain('team.invite.success')
  })
})
