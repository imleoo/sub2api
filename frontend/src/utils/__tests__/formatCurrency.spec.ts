import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import {
  currencyLabel,
  currencySymbol,
  formatUSD,
  formatUSDCompact,
  fromDisplayCurrencyAmount,
  toDisplayCurrencyAmount,
} from '../format'

describe('currency formatting', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('formats USD by default', () => {
    expect(currencyLabel()).toBe('USD')
    expect(currencySymbol()).toBe('$')
    expect(formatUSD(1.234, 2)).toBe('$1.23')
    expect(formatUSDCompact(0.009)).toBe('$0.0090')
    expect(formatUSDCompact(1200)).toBe('$1.20K')
    expect(toDisplayCurrencyAmount(10)).toBe(10)
    expect(fromDisplayCurrencyAmount(10)).toBe(10)
  })

  it('formats USD stored values as CNY when currency mode is cny', () => {
    const appStore = useAppStore()
    appStore.currencyMode = 'cny'
    appStore.cnyRate = 7.2

    expect(currencyLabel()).toBe('CNY')
    expect(currencySymbol()).toBe('¥')
    expect(formatUSD(1.25, 2)).toBe('¥9.00')
    expect(formatUSDCompact(0.01)).toBe('¥0.072')
    expect(formatUSDCompact(200)).toBe('¥1.44K')
    expect(toDisplayCurrencyAmount(10)).toBe(72)
    expect(fromDisplayCurrencyAmount(72)).toBe(10)
  })
})
