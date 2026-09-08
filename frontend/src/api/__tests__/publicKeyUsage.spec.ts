import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post, get, del, requestUse } = vi.hoisted(() => ({
  post: vi.fn(),
  get: vi.fn(),
  del: vi.fn(),
  requestUse: vi.fn(),
}))

vi.mock('axios', () => ({
  default: {
    create: vi.fn(() => ({
      post,
      get,
      delete: del,
      interceptors: { request: { use: requestUse } },
    })),
  },
}))

describe('publicKeyUsageAPI', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.setItem('auth_token', 'signed-in-user-jwt')
  })

  it('sends the API Key explicitly without consulting the signed-in JWT', async () => {
    post.mockResolvedValue({ data: { code: 0, message: 'success', data: { valid: true } } })
    const { publicKeyUsageAPI } = await import('../publicKeyUsage')

    await publicKeyUsageAPI.createSession('sk-one-time-secret')

    expect(post).toHaveBeenCalledWith(
      '/key/usage-session',
      undefined,
      { headers: { Authorization: 'Bearer sk-one-time-secret' } },
    )
    expect(JSON.stringify(post.mock.calls)).not.toContain('signed-in-user-jwt')
  })

  it('fetches key announcements through the isolated public Key session client', async () => {
    get.mockResolvedValue({ data: { code: 0, message: 'success', data: [{ id: 1, title: 'Notice' }] } })
    const { publicKeyUsageAPI } = await import('../publicKeyUsage')
    const signal = new AbortController().signal

    const result = await publicKeyUsageAPI.listAnnouncements(signal)

    expect(result).toEqual([{ id: 1, title: 'Notice' }])
    expect(get).toHaveBeenCalledWith('/key/announcements', { signal })
    expect(JSON.stringify(get.mock.calls)).not.toContain('signed-in-user-jwt')
  })

  it('marks key announcements read through the isolated public Key session client', async () => {
    post.mockResolvedValue({ data: { code: 0, message: 'success', data: { message: 'ok' } } })
    const { publicKeyUsageAPI } = await import('../publicKeyUsage')
    const signal = new AbortController().signal

    await publicKeyUsageAPI.markAnnouncementRead(7, signal)

    expect(post).toHaveBeenCalledWith(
      '/key/announcements/7/read',
      undefined,
      { signal },
    )
    expect(JSON.stringify(post.mock.calls)).not.toContain('signed-in-user-jwt')
  })

  it('uses the isolated public Key session client for feedback ticket reads and writes', async () => {
    get.mockResolvedValue({ data: { code: 0, message: 'success', data: { items: [], total: 0, page: 1, page_size: 20, pages: 0 } } })
    post.mockResolvedValue({ data: { code: 0, message: 'success', data: { id: 9, status: 'closed' } } })
    const { publicKeyUsageAPI } = await import('../publicKeyUsage')
    const signal = new AbortController().signal

    await publicKeyUsageAPI.listFeedback(2, 20, { status: 'open' }, { signal })
    await publicKeyUsageAPI.listFeedbackMessages(9, 1, 20, signal)
    await publicKeyUsageAPI.replyFeedback(9, 'plain text', signal)
    await publicKeyUsageAPI.closeFeedback(9, signal)

    expect(get).toHaveBeenCalledWith('/key/feedback', { params: { page: 2, page_size: 20, status: 'open' }, signal })
    expect(get).toHaveBeenCalledWith('/key/feedback/9/messages', { params: { page: 1, page_size: 20 }, signal })
    expect(post).toHaveBeenCalledWith('/key/feedback/9/messages', { content: 'plain text' }, { signal })
    expect(post).toHaveBeenCalledWith('/key/feedback/9/close', {}, { signal })
    expect(JSON.stringify([...get.mock.calls, ...post.mock.calls])).not.toContain('signed-in-user-jwt')
  })
})
