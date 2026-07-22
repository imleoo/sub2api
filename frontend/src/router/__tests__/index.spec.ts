import { describe, expect, it } from 'vitest'

import router from '../index'

// zhiguofan fork-only: 功能 34 删除了独立的模型折扣页，`/admin/model-discounts` 仅保留为
// 到 `/admin/model-pricings` 的重定向兜底。风险表标注 router/index.ts 为🔴高风险文件
// （historically 曾发生 workflow 触发器被上游合并覆盖的同类事故），锁定这条重定向防止被静默删除。
describe('router redirects', () => {
  it('redirects /admin/model-discounts to /admin/model-pricings', () => {
    const resolved = router.resolve('/admin/model-discounts')
    const discountsRecord = resolved.matched.find((record) => record.path === '/admin/model-discounts')

    expect(discountsRecord).toBeTruthy()
    expect(discountsRecord!.redirect).toBe('/admin/model-pricings')
  })
})
