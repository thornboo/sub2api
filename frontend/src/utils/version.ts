export const UPSTREAM_RELEASE_REPO_URL = 'https://github.com/Wei-Shaw/sub2api'
export const SECONDARY_RELEASE_REPO_URL = 'https://github.com/thornboo/sub2api'

export function normalizeReleaseURL(rawURL: string): string {
  const trimmed = rawURL.trim()
  if (!trimmed || trimmed === '#') return ''

  if (trimmed.startsWith(`${UPSTREAM_RELEASE_REPO_URL}/releases/`)) {
    return SECONDARY_RELEASE_REPO_URL + trimmed.slice(UPSTREAM_RELEASE_REPO_URL.length)
  }

  return trimmed
}
