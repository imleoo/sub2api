import { afterEach, describe, expect, it } from 'vitest'

import { getPersistedPageSize, setPersistedPageSize } from '@/composables/usePersistedPageSize'

describe('usePersistedPageSize', () => {
  afterEach(() => {
    localStorage.clear()
    delete window.__APP_CONFIG__
  })

  it('uses persisted localStorage state when available', () => {
    window.__APP_CONFIG__ = {
      table_default_page_size: 1000,
      table_page_size_options: [20, 50, 1000]
    } as any
    localStorage.setItem('table-page-size', '50')

    expect(getPersistedPageSize()).toBe(50)
  })

  it('uses the system table default when no localStorage state exists', () => {
    window.__APP_CONFIG__ = {
      table_default_page_size: 1000,
      table_page_size_options: [20, 50, 1000]
    } as any

    expect(getPersistedPageSize()).toBe(1000)
  })

  it('persists normalized page size changes', () => {
    window.__APP_CONFIG__ = {
      table_default_page_size: 20,
      table_page_size_options: [20, 50, 100]
    } as any

    setPersistedPageSize(50)

    expect(localStorage.getItem('table-page-size')).toBe('50')
    expect(getPersistedPageSize()).toBe(50)
  })
})
