import { describe, expect, it } from 'vitest'

import { supportsModelProtocolCapabilities } from '@/utils/modelProtocolCapabilities'

describe('supportsModelProtocolCapabilities', () => {
  it('matches the backend native routing scope exactly', () => {
    expect(supportsModelProtocolCapabilities({ platform: 'openai', type: 'apikey' })).toBe(true)
    expect(supportsModelProtocolCapabilities({ platform: 'openai', type: 'oauth' })).toBe(false)
    expect(supportsModelProtocolCapabilities({ platform: 'anthropic', type: 'apikey' })).toBe(false)
    expect(supportsModelProtocolCapabilities(null)).toBe(false)
  })
})
