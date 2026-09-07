import { onBeforeUnmount, watch, type WatchSource } from 'vue'

const activeLocks = new Set<symbol>()
let previousInlineOverflow: string | null = null

function syncBodyOverflow() {
  if (activeLocks.size > 0) {
    if (previousInlineOverflow === null) {
      previousInlineOverflow = document.body.style.overflow
    }
    document.body.style.overflow = 'hidden'
    return
  }

  if (previousInlineOverflow !== null) {
    document.body.style.overflow = previousInlineOverflow
    previousInlineOverflow = null
  }
}

export function setBodyScrollLock(owner: symbol, locked: boolean) {
  const hadOwner = activeLocks.has(owner)

  if (locked) {
    if (!hadOwner) {
      activeLocks.add(owner)
      syncBodyOverflow()
    }
    return
  }

  if (hadOwner) {
    activeLocks.delete(owner)
    syncBodyOverflow()
  }
}

export function useBodyScrollLock(locked: WatchSource<boolean>) {
  const owner = Symbol('body-scroll-lock')

  watch(
    locked,
    (isLocked) => {
      setBodyScrollLock(owner, isLocked)
    },
    { immediate: true },
  )

  onBeforeUnmount(() => {
    setBodyScrollLock(owner, false)
  })
}
