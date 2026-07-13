export function escapeCSVValue(value: unknown): string {
  if (value == null) return ''
  const raw = String(value)
  const escaped = raw.replace(/"/g, '""')
  if (/^[=+\-@\t\r]/.test(raw)) return `"'${escaped}"`
  if (/[,"\n\r]/.test(raw)) return `"${escaped}"`
  return raw
}

export function serializeCSV(headers: readonly unknown[], rows: ReadonlyArray<readonly unknown[]>): string {
  return [
    headers.map(escapeCSVValue).join(','),
    ...rows.map((row) => row.map(escapeCSVValue).join(',')),
  ].join('\n')
}
