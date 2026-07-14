import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post, put } = vi.hoisted(() => ({
  post: vi.fn(),
  put: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { post, put },
}))

import { batchReplaceGroups, commitImport, previewImport, type EnterpriseMember, type EnterpriseMemberImportPreview } from '@/api/enterpriseMembers'

describe('enterprise member import and batch group contracts', () => {
  beforeEach(() => {
    post.mockReset()
    put.mockReset()
  })

  it('commits a portable preview with an explicit system access policy', async () => {
    post.mockResolvedValue({ data: { job_id: 41, status: 'queued' } })
    const preview = { job_id: 41, token: 'preview-token' } as EnterpriseMemberImportPreview

    await commitImport(preview, [2, 4], {
      defaultGroupIds: [9, 3],
      activateMembers: true,
      idempotencyKey: 'member-import-job-41',
    })

    expect(post).toHaveBeenCalledWith('/enterprise/members/import/commit', {
      job_id: 41,
      preview_token: 'preview-token',
      selected_rows: [2, 4],
      default_group_ids: [9, 3],
      activate_members: true,
    }, { headers: { 'Idempotency-Key': 'member-import-job-41' } })
  })

  it('negotiates import policy 2 while creating the authoritative preview', async () => {
    post.mockResolvedValue({ data: { job_id: 41, import_policy_version: 2 } })

    await previewImport(new File(['成员编号,用户名称\n001,张三\n'], 'members.csv', { type: 'text/csv' }))

    const form = post.mock.calls[0][1] as FormData
    expect(form.get('format')).toBe('csv')
    expect(form.get('import_policy_version')).toBe('2')
  })

  it('sends member versions with an atomic ordered group batch', async () => {
    put.mockResolvedValue({ data: [] })
    const members = [
      { id: 7, version: 3 },
      { id: 8, version: 5 },
    ] as EnterpriseMember[]

    await batchReplaceGroups(members, [11, 4], 'append')

    expect(put).toHaveBeenCalledWith('/enterprise/members/batch/groups', {
      members: [{ id: 7, expected_version: 3 }, { id: 8, expected_version: 5 }],
      group_ids: [11, 4],
      mode: 'append',
    })
  })
})
