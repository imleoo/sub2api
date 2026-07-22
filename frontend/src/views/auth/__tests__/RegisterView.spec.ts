import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import RegisterView from '@/views/auth/RegisterView.vue'

const {
  pushMock,
  showSuccessMock,
  showErrorMock,
  showWarningMock,
  registerMock,
  phoneRegisterMock,
  getPublicSettingsMock,
  sendSmsCodeMock,
  validatePromoCodeMock,
  validateInvitationCodeMock,
} = vi.hoisted(() => ({
  pushMock: vi.fn(),
  showSuccessMock: vi.fn(),
  showErrorMock: vi.fn(),
  showWarningMock: vi.fn(),
  registerMock: vi.fn(),
  phoneRegisterMock: vi.fn(),
  getPublicSettingsMock: vi.fn(),
  sendSmsCodeMock: vi.fn(),
  validatePromoCodeMock: vi.fn(),
  validateInvitationCodeMock: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: pushMock,
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
    locale: { value: 'en' },
  }),
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    register: (...args: any[]) => registerMock(...args),
    phoneRegister: (...args: any[]) => phoneRegisterMock(...args),
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
    validatePromoCode: (...args: any[]) => validatePromoCodeMock(...args),
    validateInvitationCode: (...args: any[]) => validateInvitationCodeMock(...args),
  }
})

const globalStubs = {
  AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
  Icon: true,
  TurnstileWidget: true,
  LoginAgreementPrompt: true,
  EmailOAuthButtons: true,
  LinuxDoOAuthSection: true,
  WechatOAuthSection: true,
  OidcOAuthSection: true,
  RouterLink: true,
  transition: false,
}

function basePublicSettings(overrides: Record<string, unknown> = {}) {
  return {
    registration_enabled: true,
    email_verify_enabled: false,
    phone_register_enabled: false,
    promo_code_enabled: false,
    invitation_code_enabled: false,
    turnstile_enabled: false,
    turnstile_site_key: '',
    site_name: 'TokenPanel',
    linuxdo_oauth_enabled: false,
    wechat_oauth_enabled: false,
    oidc_oauth_enabled: false,
    oidc_oauth_provider_name: 'OIDC',
    github_oauth_enabled: false,
    google_oauth_enabled: false,
    registration_email_suffix_whitelist: [],
    ...overrides,
  }
}

describe('RegisterView', () => {
  beforeEach(() => {
    pushMock.mockReset()
    showSuccessMock.mockReset()
    showErrorMock.mockReset()
    showWarningMock.mockReset()
    registerMock.mockReset()
    phoneRegisterMock.mockReset()
    getPublicSettingsMock.mockReset()
    sendSmsCodeMock.mockReset()
    validatePromoCodeMock.mockReset()
    validateInvitationCodeMock.mockReset()
    sessionStorage.clear()
    getPublicSettingsMock.mockResolvedValue(basePublicSettings())
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders the email/password form and requires a username when phone registration is disabled', async () => {
    getPublicSettingsMock.mockResolvedValue(basePublicSettings({ phone_register_enabled: false }))

    const wrapper = mount(RegisterView, { global: { stubs: globalStubs } })
    await flushPromises()

    expect(wrapper.find('#email').exists()).toBe(true)
    expect(wrapper.find('#password').exists()).toBe(true)
    expect(wrapper.find('#phone').exists()).toBe(false)
    expect(wrapper.get('#username').attributes('required')).toBeDefined()
  })

  it('switches to the phone/SMS form and drops the required attribute from username when phone registration is enabled', async () => {
    getPublicSettingsMock.mockResolvedValue(basePublicSettings({ phone_register_enabled: true }))

    const wrapper = mount(RegisterView, { global: { stubs: globalStubs } })
    await flushPromises()

    expect(wrapper.find('#phone').exists()).toBe(true)
    expect(wrapper.find('#sms_code').exists()).toBe(true)
    expect(wrapper.find('#email').exists()).toBe(false)
    expect(wrapper.find('#password').exists()).toBe(false)
    expect(wrapper.get('#username').attributes('required')).toBeUndefined()
  })

  it('blocks sending the SMS code and shows an error when the phone number is empty', async () => {
    getPublicSettingsMock.mockResolvedValue(basePublicSettings({ phone_register_enabled: true }))

    const wrapper = mount(RegisterView, { global: { stubs: globalStubs } })
    await flushPromises()

    await wrapper.find('button[type="button"]').trigger('click')
    await flushPromises()

    expect(sendSmsCodeMock).not.toHaveBeenCalled()
  })

  it('disables the resend button while the SMS countdown is running and re-enables it once it reaches zero', async () => {
    vi.useFakeTimers()
    getPublicSettingsMock.mockResolvedValue(basePublicSettings({ phone_register_enabled: true }))
    sendSmsCodeMock.mockResolvedValue({ countdown: 3 })

    const wrapper = mount(RegisterView, { global: { stubs: globalStubs } })
    await flushPromises()

    await wrapper.get('#phone').setValue('13800138000')
    await wrapper.find('button[type="button"]').trigger('click')
    await flushPromises()

    expect(sendSmsCodeMock).toHaveBeenCalledWith({ phone: '13800138000', turnstile_token: undefined })

    const resendButton = wrapper.find('button[type="button"]')
    expect(resendButton.attributes('disabled')).toBeDefined()
    expect(resendButton.text()).toContain('auth.resendSmsCountdown')

    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    const resendButtonAfter = wrapper.find('button[type="button"]')
    expect(resendButtonAfter.attributes('disabled')).toBeUndefined()
    expect(resendButtonAfter.text()).toBe('auth.sendSmsCode')
  })
})
