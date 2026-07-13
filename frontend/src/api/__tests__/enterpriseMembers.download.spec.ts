import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { downloadImportTemplate } from '@/api/enterpriseMembers'

describe('enterprise member import template download', () => {
  const createObjectURL = vi.fn(() => 'blob:enterprise-member-template')
  const revokeObjectURL = vi.fn()
  let clickSpy: ReturnType<typeof vi.spyOn>
  let clickedFilename = ''

  beforeEach(() => {
    vi.useFakeTimers()
    get.mockReset()
    createObjectURL.mockClear()
    revokeObjectURL.mockClear()
    clickedFilename = ''
    Object.defineProperty(URL, 'createObjectURL', { configurable: true, value: createObjectURL })
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: revokeObjectURL })
    clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function () {
      clickedFilename = this.download
    })
  })

  afterEach(() => {
    clickSpy.mockRestore()
    vi.runOnlyPendingTimers()
    vi.useRealTimers()
    document.body.innerHTML = ''
  })

  it.each(['csv', 'xlsx'] as const)('downloads the server-authored %s template with an explicit filename', async (format) => {
    const blob = new Blob(['template'])
    get.mockResolvedValue({ data: blob })

    await downloadImportTemplate(format)

    expect(get).toHaveBeenCalledWith('/enterprise/members/import/template', {
      params: { format },
      responseType: 'blob',
    })
    expect(createObjectURL).toHaveBeenCalledWith(blob)
    expect(clickedFilename).toBe(`企业成员导入模板.${format}`)
    expect(document.body.querySelector('a')).toBeNull()

    vi.runOnlyPendingTimers()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:enterprise-member-template')
  })
})
