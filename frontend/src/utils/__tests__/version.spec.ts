import { describe, expect, it } from 'vitest'

import { normalizeReleaseURL } from '@/utils/version'

describe('normalizeReleaseURL', () => {
  it('rewrites upstream release URLs to the secondary development repository', () => {
    expect(normalizeReleaseURL('https://github.com/Wei-Shaw/sub2api/releases/tag/v1.4.1')).toBe(
      'https://github.com/thornboo/sub2api/releases/tag/v1.4.1'
    )
  })

  it('preserves the release path suffix while rewriting', () => {
    expect(
      normalizeReleaseURL('https://github.com/Wei-Shaw/sub2api/releases/download/v1.4.1/app.tar.gz')
    ).toBe('https://github.com/thornboo/sub2api/releases/download/v1.4.1/app.tar.gz')
  })

  it('passes through unrelated URLs after trimming whitespace', () => {
    expect(normalizeReleaseURL('  https://github.com/example/project/releases/tag/v1.4.1  ')).toBe(
      'https://github.com/example/project/releases/tag/v1.4.1'
    )
  })

  it('does not rewrite lookalike repository names', () => {
    expect(normalizeReleaseURL('https://github.com/Wei-Shaw/sub2api2/releases/tag/v1.4.1')).toBe(
      'https://github.com/Wei-Shaw/sub2api2/releases/tag/v1.4.1'
    )
  })

  it('normalizes blank and placeholder URLs to empty strings', () => {
    expect(normalizeReleaseURL('')).toBe('')
    expect(normalizeReleaseURL('   ')).toBe('')
    expect(normalizeReleaseURL('#')).toBe('')
    expect(normalizeReleaseURL('  #  ')).toBe('')
  })
})
