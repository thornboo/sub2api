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
})
