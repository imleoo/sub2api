import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ModelsView from '../ModelsView.vue'
import { getModels, type ModelInfo } from '@/api/models'

const messages: Record<string, string> = {
  'common.loading': 'Loading',
  'models.title': 'Supported Models',
  'models.description': 'Available models',
  'models.refresh': 'Refresh',
  'models.modelsAvailable': 'models available',
  'models.searchPlaceholder': 'Search',
  'models.allProviders': 'All Providers',
  'models.allModes': 'All Modes',
  'models.chatMode': 'Chat',
  'models.imageMode': 'Image',
  'models.noResults': 'No results',
  'models.showing': 'Showing models',
  'models.modelName': 'Model Name',
  'models.provider': 'Provider',
  'models.availability': 'Availability',
  'models.available': 'Available',
  'models.unavailable': 'Unavailable',
  'models.testPassed': 'Test Passed',
  'models.testFailed': 'Test Failed',
  'models.inputPrice': 'Input Price',
  'models.outputPrice': 'Output Price',
  'models.contextWindow': 'Context Window',
  'models.features': 'Features',
  'models.promptCaching': 'Cache',
}

vi.mock('vue-i18n', async importOriginal => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    t: (key: string) => messages[key] ?? key,
  }),
}))

vi.mock('@/api/models', () => ({
  getModels: vi.fn(),
}))

const mockedGetModels = vi.mocked(getModels)

function model(overrides: Partial<ModelInfo> = {}): ModelInfo {
  return {
    id: 'gpt-5.2',
    provider: 'openai',
    mode: 'chat',
    input_cost_per_token: 0.000001,
    output_cost_per_token: 0.000002,
    supports_prompt_caching: false,
    is_available: true,
    ...overrides,
  }
}

function mountView() {
  return mount(ModelsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        ModelIcon: { template: '<span />' },
      },
    },
  })
}

describe('ModelsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows whitelisted image models returned with image_generation mode', async () => {
    mockedGetModels.mockResolvedValue({
      total: 4,
      available_platforms: ['openai', 'google'],
      models: [
        model({ id: 'gpt-image-1', mode: 'image_generation' }),
        model({ id: 'gemini-2.5-flash-image', provider: 'google', mode: 'image_generation' }),
        model({ id: 'gpt-5.2' }),
        model({ id: 'gpt-4o' }),
      ],
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('gpt-image-1')
    expect(wrapper.text()).toContain('gemini-2.5-flash-image')
    expect(wrapper.text()).toContain('gpt-5.2')
    expect(wrapper.text()).not.toContain('gpt-4o')

    await wrapper.findAll('button').find(button => button.text() === 'Image')?.trigger('click')

    expect(wrapper.text()).toContain('gpt-image-1')
    expect(wrapper.text()).toContain('gemini-2.5-flash-image')
    expect(wrapper.text()).not.toContain('gpt-5.2')
  })
})
