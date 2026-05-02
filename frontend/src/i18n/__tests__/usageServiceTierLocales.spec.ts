import { describe, expect, it } from 'vitest'

import en from '../locales/en'
import zh from '../locales/zh'

describe('usage service tier locale keys', () => {
  it('contains zh labels for service tier tooltip', () => {
    expect(zh.usage.serviceTier).toBe('服务档位')
    expect(zh.usage.serviceTierPriority).toBe('Fast')
    expect(zh.usage.serviceTierFlex).toBe('Flex')
    expect(zh.usage.serviceTierStandard).toBe('Standard')
  })

  it('contains en labels for service tier tooltip', () => {
    expect(en.usage.serviceTier).toBe('Service tier')
    expect(en.usage.serviceTierPriority).toBe('Fast')
    expect(en.usage.serviceTierFlex).toBe('Flex')
    expect(en.usage.serviceTierStandard).toBe('Standard')
  })
})

describe('models page locale keys', () => {
  it('contains zh labels for the user models page', () => {
    expect(zh.models.title).toBe('支持的模型')
    expect(zh.models.description).toContain('当前账号可用')
    expect(zh.models.allProviders).toBe('全部提供商')
    expect(zh.models.modelName).toBe('模型名称')
  })

  it('keeps the user models page keys aligned between locales', () => {
    expect(Object.keys(zh.models).sort()).toEqual(Object.keys(en.models).sort())
  })
})
