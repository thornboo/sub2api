import { beforeEach, describe, expect, it, vi } from 'vitest'

const { put } = vi.hoisted(() => ({
  put: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { put }
}))

import { updateModelProtocolCapabilityOverrides } from '@/api/admin/accounts'

describe('admin account model protocol capability API', () => {
  beforeEach(() => {
    put.mockReset()
    put.mockResolvedValue({ data: { account_id: 7, items: [], warnings: [] } })
  })

  it('sends only override fields even when a caller passes capability metadata', async () => {
    await updateModelProtocolCapabilityOverrides(7, [
      {
        upstream_model: 'MiniMax-M3',
        protocol: 'anthropic_messages',
        state: 'supported',
        observed_state: 'unsupported',
        observed_source: 'untrusted-caller',
        effective_state: 'unsupported'
      } as any
    ])

    expect(put).toHaveBeenCalledWith(
      '/admin/accounts/7/model-protocol-capabilities/overrides',
      {
        items: [
          {
            upstream_model: 'MiniMax-M3',
            protocol: 'anthropic_messages',
            state: 'supported'
          }
        ]
      }
    )
  })
})
