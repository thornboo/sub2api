export const tableSelectionCheckboxClasses = (active: boolean) => [
  'inline-flex h-5 w-5 items-center justify-center rounded-md border shadow-sm transition-all duration-150',
  'focus:outline-none focus:ring-2 focus:ring-emerald-500/35 focus:ring-offset-1 focus:ring-offset-white',
  'dark:focus:ring-offset-black',
  active
    ? 'border-emerald-500 bg-emerald-500 text-neutral-950 shadow-emerald-500/20 hover:border-emerald-400 hover:bg-emerald-400 dark:border-emerald-400 dark:bg-emerald-400 dark:text-black dark:hover:bg-emerald-300'
    : 'border-stone-300/80 bg-white/80 text-transparent hover:border-emerald-500/40 hover:bg-emerald-50/60 dark:border-white/10 dark:bg-neutral-950/70 dark:shadow-black/20 dark:hover:border-emerald-400/45 dark:hover:bg-emerald-500/5'
]

export const tableSelectionLabel = (label: unknown, fallback: string) => {
  const text = typeof label === 'string' ? label.trim() : String(label ?? '').trim()
  return text || fallback
}
