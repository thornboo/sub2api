import { beforeEach, describe, expect, it, vi } from 'vitest'

const { patch, post } = vi.hoisted(() => ({ patch: vi.fn(), post: vi.fn() }))

vi.mock('@/api/client', () => ({
  apiClient: { patch, post }
}))

import { batchAdjustUsage, batchUpdate, type EnterpriseMember } from '@/api/enterpriseMembers'

const members = [
  { id: 11, version: 3 },
  { id: 22, version: 7 }
] as EnterpriseMember[]

describe('enterprise member batch API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('sends only explicit policy fields with optimistic versions', async () => {
    patch.mockResolvedValue({ data: { updated_count: 2 } })

    await batchUpdate(members, { monthly_limit_usd: 100, group_mode: 'keep' }, 'stable-policy-key')

    expect(patch).toHaveBeenCalledWith('/enterprise/members/batch', {
      members: [
        { id: 11, expected_version: 3 },
        { id: 22, expected_version: 7 }
      ],
      monthly_limit_usd: 100,
      group_mode: 'keep'
    }, { headers: { 'Idempotency-Key': 'stable-policy-key' } })
  })

  it('uses a signed delta request for usage reconciliation', async () => {
    post.mockResolvedValue({ data: { updated_count: 2 } })

    await batchAdjustUsage(members, {
      monthly_used_delta: 12.5,
      usage_5h_delta: -1,
      usage_1d_delta: 0,
      usage_7d_delta: 3
    }, 'stable-usage-key')

    expect(post).toHaveBeenCalledWith('/enterprise/members/batch/usage-adjustments', {
      members: [
        { id: 11, expected_version: 3 },
        { id: 22, expected_version: 7 }
      ],
      monthly_used_delta: 12.5,
      usage_5h_delta: -1,
      usage_1d_delta: 0,
      usage_7d_delta: 3
    }, { headers: { 'Idempotency-Key': 'stable-usage-key' } })
  })
})
