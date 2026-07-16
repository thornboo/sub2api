import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, patch, put } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  patch: vi.fn(),
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post, patch, put },
}))

import {
  batchReplaceGroups,
  create,
  list,
  replaceGroups,
  restore,
  setStatus,
  update,
  type CreateEnterpriseMemberInput,
  type EnterpriseMember,
} from '@/api/enterpriseMembers'

const member: EnterpriseMember = {
  id: 15,
  enterprise_user_id: 7,
  member_code: 'ceshi1',
  name: '测试',
  status: 'disabled',
  monthly_limit_usd: 0,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
  version: 1,
  group_ids: [],
  key_count: 0,
  created_at: '2026-07-17T08:09:10Z',
  updated_at: '2026-07-17T08:09:10Z',
}

const createInput: CreateEnterpriseMemberInput = {
  member_code: member.member_code,
  name: member.name,
  monthly_limit_usd: 0,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  group_ids: [],
  monthly_used_usd: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
}

describe('enterprise member response contracts', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    patch.mockReset()
    put.mockReset()
  })

  it('normalizes legacy null group IDs in list responses', async () => {
    get.mockResolvedValue({ data: [{ ...member, group_ids: null }] })

    const members = await list()

    expect(get).toHaveBeenCalledWith('/enterprise/members', { params: { include_archived: false } })
    expect(members[0]?.group_ids).toEqual([])
  })

  it('normalizes legacy null group IDs in member mutation responses', async () => {
    post.mockResolvedValueOnce({ data: { ...member, group_ids: null } })
    patch.mockResolvedValueOnce({ data: { ...member, group_ids: null } })
    post.mockResolvedValueOnce({ data: { ...member, group_ids: null } })
    post.mockResolvedValueOnce({ data: { ...member, group_ids: null } })

    const results = await Promise.all([
      create(createInput),
      update(member, { name: '更新后' }),
      setStatus(member, 'active'),
      restore(member),
    ])

    expect(results.map(result => result.group_ids)).toEqual([[], [], [], []])
  })

  it('normalizes legacy null group IDs in group mutation responses', async () => {
    put.mockResolvedValueOnce({ data: { version: 2, group_ids: null } })
    put.mockResolvedValueOnce({
      data: [{
        id: member.id,
        version: 2,
        group_ids: null,
        status: member.status,
        updated_at: member.updated_at,
      }],
    })

    const replaced = await replaceGroups(member, [])
    const batchReplaced = await batchReplaceGroups([member], [], 'replace')

    expect(replaced.group_ids).toEqual([])
    expect(batchReplaced[0]?.group_ids).toEqual([])
  })
})
