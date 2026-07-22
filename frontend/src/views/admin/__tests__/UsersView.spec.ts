import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminUser } from '@/types'
import UsersView from '../UsersView.vue'

const {
  listUsers,
  getAllGroups,
  getBatchUsersUsage,
  listEnabledDefinitions,
  getBatchUserAttributes,
  exportUserStatement
} = vi.hoisted(() => ({
  listUsers: vi.fn(),
  getAllGroups: vi.fn(),
  getBatchUsersUsage: vi.fn(),
  listEnabledDefinitions: vi.fn(),
  getBatchUserAttributes: vi.fn(),
  exportUserStatement: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      list: listUsers,
      toggleStatus: vi.fn(),
      delete: vi.fn(),
      exportUserStatement
    },
    groups: {
      getAll: getAllGroups
    },
    dashboard: {
      getBatchUsersUsage
    },
    userAttributes: {
      listEnabledDefinitions,
      getBatchUserAttributes
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const createAdminUser = (overrides: Partial<AdminUser> = {}): AdminUser => ({
  id: 42,
  username: 'scoped-user',
  email: 'scoped@example.com',
  role: 'user',
  balance: 0,
  concurrency: 1,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-04-17T00:00:00Z',
  updated_at: '2026-04-17T00:00:00Z',
  notes: '',
  last_active_at: '2026-04-16T02:00:00Z',
  last_used_at: '2026-04-17T02:00:00Z',
  current_concurrency: 0,
  ...overrides
})

const DataTableStub = {
  props: ['columns', 'data', 'selectedKeys'],
  emits: ['sort', 'update:selectedKeys'],
  template: `
    <div>
      <div data-test="columns">{{ columns.map(col => col.key).join(',') }}</div>
      <div data-test="row-order">{{ data.map(row => row.email).join(',') }}</div>
      <div data-test="selected-keys">{{ (selectedKeys || []).join(',') }}</div>
      <button data-test="sort-last-used" @click="$emit('sort', 'last_used_at', 'desc')">sort</button>
      <button
        v-for="row in data"
        :key="'select-' + row.id"
        :data-test="'select-' + row.id"
        @click="$emit('update:selectedKeys', Array.from(new Set([...(selectedKeys || []), row.id])))"
      >
        select
      </button>
      <template v-for="col in columns" :key="col.key">
        <slot :name="'header-' + col.key" :column="col" />
      </template>
      <div v-for="row in data" :key="row.id">
        <slot name="cell-last_used_at" :value="row.last_used_at" :row="row" />
      </div>
    </div>
  `
}

const PaginationStub = {
  emits: ['update:page'],
  template: '<button data-test="next-page" @click="$emit(\'update:page\', 2)">next</button>'
}

const ActionsDataTableStub = {
  props: ['columns', 'data', 'selectedKeys', 'actionsCount'],
  emits: ['sort', 'update:selectedKeys'],
  template: `
    <div>
      <div data-test="actions-count">{{ actionsCount }}</div>
      <div v-for="row in data" :key="row.id">
        <slot name="cell-actions" :row="row" />
      </div>
    </div>
  `
}

const BulkEditUserModalStub = {
  props: ['show', 'selectedIds'],
  emits: ['close', 'success'],
  template: `
    <div v-if="show" data-test="bulk-modal">
      <span data-test="bulk-modal-ids">{{ selectedIds.join(',') }}</span>
      <button data-test="bulk-success" @click="$emit('success', selectedIds.length)">success</button>
    </div>
  `
}

describe('admin UsersView', () => {
  beforeEach(() => {
    vi.useRealTimers()
    localStorage.clear()

    listUsers.mockReset()
    getAllGroups.mockReset()
    getBatchUsersUsage.mockReset()
    listEnabledDefinitions.mockReset()
    getBatchUserAttributes.mockReset()
    exportUserStatement.mockReset()

    listUsers.mockResolvedValue({
      items: [createAdminUser()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getAllGroups.mockResolvedValue([])
    getBatchUsersUsage.mockResolvedValue({ stats: {} })
    listEnabledDefinitions.mockResolvedValue([])
    getBatchUserAttributes.mockResolvedValue({ values: {} })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows active, used, and created activity columns in order and requests last_used_at sort', async () => {
    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          BulkEditUserModal: BulkEditUserModalStub,
          UserPlatformQuotaModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    const columns = wrapper.get('[data-test="columns"]').text()
    const visibleColumns = columns.split(',')
    expect(visibleColumns.slice(-4, -1)).toEqual(['last_active_at', 'last_used_at', 'created_at'])
    expect(visibleColumns).not.toContain('last_login_at')

    await wrapper.get('[data-test="sort-last-used"]').trigger('click')
    await flushPromises()

    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'last_used_at',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('clears usage current-page sort when switching to last_used_at server sort', async () => {
    vi.useFakeTimers()
    localStorage.setItem('user-column-settings-version', '3')
    localStorage.setItem(
      'user-hidden-columns',
      JSON.stringify([
        'notes',
        'groups',
        'subscriptions',
        'concurrency',
        'usage_anthropic',
        'usage_openai',
        'usage_gemini',
        'usage_antigravity',
        'balance_platform_quota'
      ])
    )

    listUsers.mockResolvedValue({
      items: [
        createAdminUser({ id: 1, email: 'last-used-first@example.com' }),
        createAdminUser({ id: 2, email: 'usage-first@example.com' })
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getBatchUsersUsage.mockResolvedValue({
      stats: {
        1: { user_id: 1, today_actual_cost: 1, total_actual_cost: 1, by_platform: [] },
        2: { user_id: 2, today_actual_cost: 9, total_actual_cost: 9, by_platform: [] }
      }
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          BulkEditUserModal: BulkEditUserModalStub,
          UserPlatformQuotaModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(50)
    await flushPromises()

    expect(wrapper.get('[data-test="row-order"]').text()).toBe('last-used-first@example.com,usage-first@example.com')

    await wrapper.get('[data-test="usage-sort-trigger-usage"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="usage-sort-usage-today"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="row-order"]').text()).toBe('usage-first@example.com,last-used-first@example.com')
    expect(localStorage.getItem('admin-users-usage-sort')).toContain('"key":"usage"')

    await wrapper.get('[data-test="sort-last-used"]').trigger('click')
    await flushPromises()

    expect(localStorage.getItem('admin-users-usage-sort')).toBeNull()
    expect(wrapper.get('[data-test="row-order"]').text()).toBe('last-used-first@example.com,usage-first@example.com')
    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'last_used_at',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('keeps selected user IDs across pages and clears them after a successful bulk update', async () => {
    let refreshed = false
    listUsers.mockImplementation(async (page: number) => {
      const user = page === 2
        ? createAdminUser({
            id: 43,
            email: refreshed ? 'refreshed-page-two@example.com' : 'page-two@example.com'
          })
        : createAdminUser({ id: 42, email: 'page-one@example.com' })
      return {
        items: [user],
        total: 2,
        page,
        page_size: 20,
        pages: 2
      }
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: PaginationStub,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          BulkEditUserModal: BulkEditUserModalStub,
          UserPlatformQuotaModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.find('[data-test="bulk-edit-limits"]').exists()).toBe(false)
    await wrapper.get('[data-test="select-42"]').trigger('click')
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('42')
    expect(wrapper.find('[data-test="bulk-edit-limits"]').exists()).toBe(true)

    await wrapper.get('[data-test="next-page"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('42')

    await wrapper.get('[data-test="select-43"]').trigger('click')
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('42,43')

    await wrapper.get('[data-test="bulk-edit-limits"]').trigger('click')
    expect(wrapper.get('[data-test="bulk-modal-ids"]').text()).toBe('42,43')

    const callsBeforeSuccess = listUsers.mock.calls.length
    refreshed = true
    await wrapper.get('[data-test="bulk-success"]').trigger('click')
    await flushPromises()

    expect(listUsers.mock.calls.length).toBeGreaterThan(callsBeforeSuccess)
    expect(wrapper.get('[data-test="row-order"]').text()).toBe('refreshed-page-two@example.com')
    expect(wrapper.find('[data-test="bulk-edit-limits"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="selected-keys"]').text()).toBe('')
  })

  // zhiguofan fork-only: 月度对账导出（功能 45），风险表标注 handleExportStatement/confirmExportStatement +
  // actions-count="8" 为上游合并事故高发点，须锁定"更多"菜单项数量与导出参数。
  it('keeps 8 row actions and exports the selected month via the statement dialog', async () => {
    window.URL.createObjectURL = vi.fn(() => 'blob:mock-url')
    window.URL.revokeObjectURL = vi.fn()
    const anchorClickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    exportUserStatement.mockResolvedValue(new Blob(['ok']))

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: ActionsDataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          BulkEditUserModal: BulkEditUserModalStub,
          UserPlatformQuotaModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true
          // 注意：这里刻意不 stub Teleport，因为对账导出菜单/弹窗都依赖真实 Teleport 到 document.body
        }
      }
    })

    await flushPromises()

    expect(wrapper.get('[data-test="actions-count"]').text()).toBe('8')

    await wrapper.get('.action-menu-trigger').trigger('click')
    await flushPromises()

    const exportButton = Array.from(document.body.querySelectorAll('button')).find(
      (btn) => btn.textContent?.trim() === 'admin.users.exportStatement'
    )
    expect(exportButton).toBeTruthy()
    exportButton!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()

    const monthSelect = document.body.querySelector('[data-testid="statement-export-month"]') as HTMLSelectElement | null
    expect(monthSelect).toBeTruthy()
    expect(monthSelect!.value).toMatch(/^\d{4}-\d{2}$/)

    const confirmButton = document.body.querySelector(
      '[data-testid="statement-export-confirm"]'
    ) as HTMLButtonElement | null
    expect(confirmButton).toBeTruthy()
    confirmButton!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flushPromises()
    await flushPromises()

    expect(exportUserStatement).toHaveBeenCalledWith(42, monthSelect!.value, expect.any(String))
    expect(anchorClickSpy).toHaveBeenCalled()

    anchorClickSpy.mockRestore()
  })
})
