import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useTeamStore } from '@/stores/team'

const mockListMyTeams = vi.fn()

vi.mock('@/api/team', () => ({
  teamAPI: {
    listMyTeams: (...args: any[]) => mockListMyTeams(...args)
  }
}))

const personalTeam = { owner_user_id: 1, owner_email: 'me@example.com', is_personal: true, role: 'owner' }
const joinedTeam = { owner_user_id: 2, owner_email: 'owner@example.com', is_personal: false, role: 'admin' }
const joinedTeamAsMember = { owner_user_id: 3, owner_email: 'other-owner@example.com', is_personal: false, role: 'member' }

describe('useTeamStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockListMyTeams.mockReset()
  })

  it('loadTeams populates the team list and marks loaded', async () => {
    mockListMyTeams.mockResolvedValue([personalTeam, joinedTeam])
    const store = useTeamStore()

    await store.loadTeams()

    expect(store.teams).toEqual([personalTeam, joinedTeam])
    expect(store.loaded).toBe(true)
  })

  it('marks loaded even when loadTeams fails, and keeps teams empty', async () => {
    mockListMyTeams.mockRejectedValue(new Error('network'))
    const store = useTeamStore()

    await expect(store.loadTeams()).rejects.toThrow('network')

    expect(store.teams).toEqual([])
    expect(store.loaded).toBe(true)
  })

  it('joinedTeams excludes the personal account row', async () => {
    mockListMyTeams.mockResolvedValue([personalTeam, joinedTeam])
    const store = useTeamStore()

    await store.loadTeams()

    expect(store.joinedTeams).toEqual([joinedTeam])
  })

  it('manageableJoinedTeams only includes enterprises where I am an admin', async () => {
    mockListMyTeams.mockResolvedValue([personalTeam, joinedTeam, joinedTeamAsMember])
    const store = useTeamStore()

    await store.loadTeams()

    expect(store.manageableJoinedTeams).toEqual([joinedTeam])
  })

  it('reset clears teams and the loaded flag', async () => {
    mockListMyTeams.mockResolvedValue([personalTeam, joinedTeam])
    const store = useTeamStore()
    await store.loadTeams()

    store.reset()

    expect(store.teams).toEqual([])
    expect(store.loaded).toBe(false)
  })
})
