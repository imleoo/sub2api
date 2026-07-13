import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(
  resolve(process.cwd(), 'src/components/account/CreateAccountModal.vue'),
  'utf8'
)

describe('CreateAccountModal Grok account types', () => {
  // fork：Grok 仅支持官方 xAI API Key（OAuth 订阅逆向已删除，功能 35），
  // 无 OAuth/APIKey 选择区；选中 grok 平台时 accountCategory 自动置为 apikey。
  it('grok is API-key only with the official xAI default base URL', () => {
    expect(source).not.toContain('data-testid="grok-account-type-api-key"')
    expect(source).toContain("newPlatform === 'grok'")
    expect(source).toContain("? 'https://api.x.ai/v1'")
    expect(source).toContain("form.platform === 'grok'")
    expect(source).toContain("? 'xai-...'")
  })

  it('switching to grok forces the apikey category', () => {
    const grokSwitch = source.match(
      /if \(newPlatform === 'grok'\) \{[\s\S]*?\}/
    )
    expect(grokSwitch).toBeTruthy()
    expect(grokSwitch![0]).toContain("accountCategory.value = 'apikey'")
  })
})
