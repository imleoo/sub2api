// fork（zhiguofan 分支）i18n 深合并工具：把 fork.ts 的自定义键覆盖到上游模块组装结果上。
// 上游同步时本文件与 fork.ts 均为 fork 独有，不与上游模块冲突。
type LocaleTree = Record<string, unknown>

export function deepMergeLocale<T extends LocaleTree>(base: T, patch: LocaleTree): T {
  const out: LocaleTree = { ...base }
  for (const [key, value] of Object.entries(patch)) {
    const prev = out[key]
    if (
      value &&
      typeof value === 'object' &&
      !Array.isArray(value) &&
      prev &&
      typeof prev === 'object' &&
      !Array.isArray(prev)
    ) {
      out[key] = deepMergeLocale(prev as LocaleTree, value as LocaleTree)
    } else {
      out[key] = value
    }
  }
  return out as T
}
