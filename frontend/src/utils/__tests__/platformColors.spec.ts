import { describe, expect, it } from 'vitest'
import {
  platformBadgeClass,
  platformBadgeLightClass,
  platformBorderClass,
  platformAccentBarClass,
  platformTextClass,
  platformIconClass,
  platformButtonClass,
  platformDiscountClass,
  platformGradientClass,
  platformGradientTextClass,
  platformGradientSubtextClass,
  platformLabel,
} from '../platformColors'

// isPlatform() 本身未导出（仅内部使用），通过所有导出函数间接验证其
// 'lingjing' 命中分支与未知平台 fallback 分支。

describe('platformColors', () => {
  describe('lingjing 命中分支', () => {
    it('platformBadgeClass 返回红色 badge 样式', () => {
      expect(platformBadgeClass('lingjing')).toBe(
        'bg-red-500/10 text-red-600 border-red-500/30 dark:text-red-400'
      )
    })

    it('platformBadgeLightClass 返回红色轻量 badge 样式', () => {
      expect(platformBadgeLightClass('lingjing')).toBe(
        'bg-red-500/10 text-red-600 dark:bg-red-500/10 dark:text-red-300'
      )
    })

    it('platformBorderClass 返回红色边框样式', () => {
      expect(platformBorderClass('lingjing')).toBe('border-red-500/20 dark:border-red-500/20')
    })

    it('platformAccentBarClass 返回红色渐变条样式', () => {
      expect(platformAccentBarClass('lingjing')).toBe('bg-gradient-to-r from-red-400 to-red-500')
    })

    it('platformTextClass 返回红色文字样式', () => {
      expect(platformTextClass('lingjing')).toBe('text-red-600 dark:text-red-400')
    })

    it('platformIconClass 返回红色图标样式', () => {
      expect(platformIconClass('lingjing')).toBe('text-red-500 dark:text-red-400')
    })

    it('platformButtonClass 返回红色按钮样式', () => {
      expect(platformButtonClass('lingjing')).toBe(
        'bg-red-500 text-white hover:bg-red-600 active:bg-red-700 dark:bg-red-500/80 dark:hover:bg-red-500'
      )
    })

    it('platformDiscountClass 返回红色折扣标样式', () => {
      expect(platformDiscountClass('lingjing')).toBe(
        'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
      )
    })

    it('platformGradientClass 返回红色渐变样式', () => {
      expect(platformGradientClass('lingjing')).toBe('from-red-500 to-red-600')
    })

    it('platformGradientTextClass 返回红色渐变文字样式', () => {
      expect(platformGradientTextClass('lingjing')).toBe('text-red-100')
    })

    it('platformGradientSubtextClass 返回红色渐变副文字样式', () => {
      expect(platformGradientSubtextClass('lingjing')).toBe('text-red-200')
    })

    it('platformLabel 返回中文标签「灵境」', () => {
      expect(platformLabel('lingjing')).toBe('灵境')
    })
  })

  describe('已知平台的 fallback 边界（anthropic/openai/gemini/grok 均不落入 DEFAULT）', () => {
    it.each(['anthropic', 'openai', 'gemini', 'grok'])(
      '%s 不使用 DEFAULT badge 样式（不含 slate）',
      (platform) => {
        expect(platformBadgeClass(platform)).not.toContain('slate')
      }
    )

    it.each(['anthropic', 'openai', 'gemini', 'grok'])('%s 有对应的 platformLabel', (platform) => {
      expect(platformLabel(platform)).not.toBe(platform)
      expect(platformLabel(platform).length).toBeGreaterThan(0)
    })
  })

  describe('未知平台的 fallback 行为', () => {
    it('未知字符串（如已删除的 antigravity）落入各 DEFAULT 样式', () => {
      expect(platformBadgeClass('antigravity')).toBe(
        'bg-slate-500/10 text-slate-600 border-slate-500/30 dark:text-slate-400'
      )
      expect(platformBorderClass('antigravity')).toBe('border-gray-200 dark:border-dark-700')
      expect(platformAccentBarClass('antigravity')).toBe(
        'bg-gradient-to-r from-primary-400 to-primary-500'
      )
      expect(platformTextClass('antigravity')).toBe('text-primary-600 dark:text-primary-400')
      expect(platformIconClass('antigravity')).toBe('text-primary-500 dark:text-primary-400')
      expect(platformButtonClass('antigravity')).toBe(
        'bg-primary-500 text-white hover:bg-primary-600 dark:bg-primary-600 dark:hover:bg-primary-500'
      )
      expect(platformDiscountClass('antigravity')).toBe(
        'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
      )
      expect(platformGradientClass('antigravity')).toBe('from-primary-500 to-primary-600')
      expect(platformGradientTextClass('antigravity')).toBe('text-primary-100')
      expect(platformGradientSubtextClass('antigravity')).toBe('text-primary-200')
    })

    it('空字符串 fallback 到 DEFAULT 样式，platformLabel 回退为 "API"', () => {
      expect(platformBadgeClass('')).toBe(
        'bg-slate-500/10 text-slate-600 border-slate-500/30 dark:text-slate-400'
      )
      expect(platformLabel('')).toBe('API')
    })

    it('未知平台字符串 platformLabel 原样返回输入值', () => {
      expect(platformLabel('some-unknown-platform')).toBe('some-unknown-platform')
    })
  })
})
