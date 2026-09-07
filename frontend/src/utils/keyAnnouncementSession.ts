const STORAGE_KEY = 'key-usage-announcement-session'

interface DisplayedAnnouncements {
  sessionId: string
  ids: number[]
}

// Keep SPA navigation working even when the browser denies sessionStorage.
let memory: DisplayedAnnouncements | null = null

export function restoreKeyAnnouncementIds(sessionId: string): Set<number> {
  // A failed later write can leave older storage behind; memory is newer in
  // this runtime and remains authoritative for the same validated session.
  if (memory?.sessionId === sessionId) return new Set(memory.ids)
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    const saved: unknown = raw ? JSON.parse(raw) : null
    if (saved && typeof saved === 'object' && 'sessionId' in saved && 'ids' in saved
      && typeof saved.sessionId === 'string' && Array.isArray(saved.ids)
      && saved.ids.every((id: unknown) => typeof id === 'number' && Number.isSafeInteger(id) && id > 0)) {
      memory = { sessionId: saved.sessionId, ids: saved.ids }
    }
  } catch {
    // Storage is optional; the server-validated session still scopes memory.
  }
  return new Set(memory?.sessionId === sessionId ? memory.ids : [])
}

export function saveKeyAnnouncementIds(sessionId: string, ids: Set<number>) {
  memory = { sessionId, ids: [...ids] }
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(memory))
  } catch {
    // The in-memory copy survives route unmounts in this tab.
  }
}

export function clearKeyAnnouncementSession() {
  memory = null
  try {
    sessionStorage.removeItem(STORAGE_KEY)
  } catch {
    // A future restore validates the server session before using any markers.
  }
}
