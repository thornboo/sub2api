import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import SupportedModelChip from '../SupportedModelChip.vue'

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: vi.fn(),
    showError: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => params?.path ? `${key}:${params.path}` : key
    })
  }
})

describe('SupportedModelChip', () => {
  it('gives endpoint copy buttons an accessible name and keyboard focus style', () => {
    const wrapper = mount(SupportedModelChip, {
      props: {
        model: {
          name: 'MiniMax-M3',
          platform: 'openai',
          pricing: null,
          supported_endpoints: [
            {
              protocol: 'anthropic_messages',
              path: '/v1/messages',
              group_ids: [1]
            }
          ]
        }
      },
      global: {
        stubs: {
          PlatformIcon: true,
          PricingRow: true,
          Teleport: true
        }
      }
    })

    const button = wrapper.get('button')
    expect(button.attributes('aria-label')).toBe('availableChannels.endpoints.copyHint:/v1/messages')
    expect(button.classes()).toContain('focus-visible:ring-2')
    expect(wrapper.classes()).toContain('max-w-full')
    expect(wrapper.get('[data-testid="supported-model-name"]').classes()).toContain('truncate')
    expect(button.classes()).toContain('shrink-0')
  })
})
