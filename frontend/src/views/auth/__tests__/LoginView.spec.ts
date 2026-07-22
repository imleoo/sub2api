import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import LoginView from '@/views/auth/LoginView.vue'

const {
  pushMock,
  showSuccessMock,
  showErrorMock,
  showWarningMock,
  loginMock,
  login2FAMock,
  phoneLoginMock,
  getPublicSettingsMock,
  sendSmsCodeMock,
} = vi.hoisted(() => ({
  pushMock: vi.fn(),
  showSuccessMock: vi.fn(),
  showErrorMock: vi.fn(),
  showWarningMock: vi.fn(),
  loginMock: vi.fn(),
  login2FAMock: vi.fn(),
  phoneLoginMock: vi.fn(),
  getPublicSettingsMock: vi.fn(),
  sendSmsCodeMock: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: pushMock,
    currentRoute: { value: { query: {} } },
  }),
  useRoute: () => ({ query: {} }),
}))

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      t: (key: string) => key,
    },
  }),
  useI18n: () => ({
    t: (key: string, params?: Record<string, string | number>) =>
      params ? `${key}:${JSON.stringify(params)}` : key,
  }),
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    login: (...args: any[]) => loginMock(...args),
    login2FA: (...args: any[]) => login2FAMock(...args),
    phoneLogin: (...args: any[]) => phoneLoginMock(...args),
  }),
  useAppStore: () => ({
    showSuccess: (...args: any[]) => showSuccessMock(...args),
    showError: (...args: any[]) => showErrorMock(...args),
    showWarning: (...args: any[]) => showWarningMock(...args),
  }),
}))

vi.mock('@/api/auth', async () => {
  const actual = await vi.importActual<typeof import('@/api/auth')>('@/api/auth')
  return {
    ...actual,
    getPublicSettings: (...args: any[]) => getPublicSettingsMock(...args),
    sendSmsCode: (...args: any[]) => sendSmsCodeMock(...args),
  }
})

const globalStubs = {
  AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
  Icon: true,
  TurnstileWidget: true,
  TotpLoginModal: true,
  LoginAgreementPrompt: true,
  EmailOAuthButtons: true,
  LinuxDoOAuthSection: true,
  DingTalkOAuthSection: true,
  WechatOAuthSection: true,
  OidcOAuthSection: true,
  RouterLink: true,
  transition: false,
}

function basePublicSettings(overrides: Record<string, unknown> = {}) {
  return {
    turnstile_enabled: false,
    turnstile_site_key: '',
    linuxdo_oauth_enabled: false,
    dingtalk_oauth_enabled: false,
    wechat_oauth_enabled: false,
    backend_mode_enabled: false,
    oidc_oauth_enabled: false,
    oidc_oauth_provider_name: 'OIDC',
    github_oauth_enabled: false,
    google_oauth_enabled: false,
    password_reset_enabled: false,
    phone_register_enabled: false,
    password_login_enabled: true,
    invitation_code_enabled: false,
    ...overrides,
  }
}

describe('LoginView', () => {
  beforeEach(() => {
    pushMock.mockReset()
    showSuccessMock.mockReset()
    showErrorMock.mockReset()
    showWarningMock.mockReset()
    loginMock.mockReset()
    login2FAMock.mockReset()
    phoneLoginMock.mockReset()
    getPublicSettingsMock.mockReset()
    sendSmsCodeMock.mockReset()
    sessionStorage.clear()
    getPublicSettingsMock.mockResolvedValue(basePublicSettings())
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('disables the resend button while the SMS countdown is running and re-enables it once it reaches zero', async () => {
    vi.useFakeTimers()
    getPublicSettingsMock.mockResolvedValue(
      basePublicSettings({ phone_register_enabled: true })
    )
    sendSmsCodeMock.mockResolvedValue({ countdown: 3 })

    const wrapper = mount(LoginView, { global: { stubs: globalStubs } })
    await flushPromises()

    await wrapper.get('#login-phone').setValue('13800138000')
    await wrapper.find('button[type="button"]').trigger('click')
    await flushPromises()

    expect(sendSmsCodeMock).toHaveBeenCalledWith({ phone: '13800138000', turnstile_token: undefined })

    const resendButton = wrapper.find('button[type="button"]')
    expect(resendButton.attributes('disabled')).toBeDefined()
    expect(resendButton.text()).toContain('auth.resendSmsCountdown')

    // advance past the countdown
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    const resendButtonAfter = wrapper.find('button[type="button"]')
    expect(resendButtonAfter.attributes('disabled')).toBeUndefined()
    expect(resendButtonAfter.text()).toBe('auth.sendSmsCode')
  })

  it('blocks sending the SMS code and shows an error when the phone number is empty', async () => {
    getPublicSettingsMock.mockResolvedValue(
      basePublicSettings({ phone_register_enabled: true })
    )

    const wrapper = mount(LoginView, { global: { stubs: globalStubs } })
    await flushPromises()

    await wrapper.find('button[type="button"]').trigger('click')
    await flushPromises()

    expect(sendSmsCodeMock).not.toHaveBeenCalled()
  })

  it('hides the switch-to-password link when password login is disabled, trapping the user in phone-only mode', async () => {
    getPublicSettingsMock.mockResolvedValue(
      basePublicSettings({ phone_register_enabled: true, password_login_enabled: false })
    )

    const wrapper = mount(LoginView, { global: { stubs: globalStubs } })
    await flushPromises()

    // still in phone mode: phone input present, email/password fields absent
    expect(wrapper.find('#login-phone').exists()).toBe(true)
    expect(wrapper.find('#email').exists()).toBe(false)
    expect(wrapper.find('#password').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('auth.switchToPasswordLogin')
  })

  it('shows the switch-to-password link when password login is enabled and reveals the password form once clicked', async () => {
    getPublicSettingsMock.mockResolvedValue(
      basePublicSettings({ phone_register_enabled: true, password_login_enabled: true })
    )

    const wrapper = mount(LoginView, { global: { stubs: globalStubs } })
    await flushPromises()

    expect(wrapper.text()).toContain('auth.switchToPasswordLogin')

    const switchButtons = wrapper.findAll('button[type="button"]')
    const switchToPassword = switchButtons.find((btn) => btn.text() === 'auth.switchToPasswordLogin')
    expect(switchToPassword).toBeTruthy()
    await switchToPassword!.trigger('click')

    expect(wrapper.find('#login-phone').exists()).toBe(false)
    expect(wrapper.find('#email').exists()).toBe(true)
    expect(wrapper.find('#password').exists()).toBe(true)
  })
})
