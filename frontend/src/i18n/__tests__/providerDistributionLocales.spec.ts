import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

// Phase 1 P1-5：验证 P1-3 / P1-4 引入的 i18n key 覆盖完整、中英双语对齐。
//
// 验收（docs/sprint-plan.md P1-5）：
//   - 中英双语切换无 missing translation
//   - admin.accounts.providers.* 完整（P1-3 表单 Provider 预设）
//   - admin.dashboard.providerDistribution / accountDistribution 完整（P1-4 图表）

describe('P1-3 Provider preset locale keys', () => {
  const expectedKeys = ['label', 'hint', 'openai', 'deepseek', 'doubao', 'siliconflow', 'custom']

  it('zh.admin.accounts.providers has all keys', () => {
    expect(zh.admin.accounts.providers).toBeDefined()
    for (const k of expectedKeys) {
      expect((zh.admin.accounts.providers as Record<string, unknown>)[k]).toBeTypeOf('string')
    }
  })

  it('en.admin.accounts.providers has all keys', () => {
    expect(en.admin.accounts.providers).toBeDefined()
    for (const k of expectedKeys) {
      expect((en.admin.accounts.providers as Record<string, unknown>)[k]).toBeTypeOf('string')
    }
  })

  it('zh and en providers keys are aligned', () => {
    expect(Object.keys(zh.admin.accounts.providers).sort()).toEqual(
      Object.keys(en.admin.accounts.providers).sort()
    )
  })
})

describe('P1-4 dashboard distribution locale keys', () => {
  const expectedKeys = [
    'providerDistribution',
    'accountDistribution',
    'lastNDays',
    'total',
    'provider',
    'providersEmpty',
    'account',
    'coveredGroups',
    'coveredGroupsHint',
    'accountsEmpty',
  ]

  it('zh.admin.dashboard has P1-4 keys', () => {
    for (const k of expectedKeys) {
      expect((zh.admin.dashboard as Record<string, unknown>)[k]).toBeTypeOf('string')
    }
  })

  it('en.admin.dashboard has P1-4 keys', () => {
    for (const k of expectedKeys) {
      expect((en.admin.dashboard as Record<string, unknown>)[k]).toBeTypeOf('string')
    }
  })

  it('lastNDays template includes {n} placeholder in both locales', () => {
    expect(zh.admin.dashboard.lastNDays).toContain('{n}')
    expect(en.admin.dashboard.lastNDays).toContain('{n}')
  })

  it('coveredGroupsHint template includes {n} placeholder in both locales', () => {
    expect(zh.admin.dashboard.coveredGroupsHint).toContain('{n}')
    expect(en.admin.dashboard.coveredGroupsHint).toContain('{n}')
  })
})
