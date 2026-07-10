type LocaleRecord = Record<string, unknown>

function isLocaleRecord(value: unknown): value is LocaleRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function mergeLocale(base: LocaleRecord, overlay: LocaleRecord): LocaleRecord {
  const merged: LocaleRecord = { ...base }
  for (const [key, overlayValue] of Object.entries(overlay)) {
    const baseValue = merged[key]
    merged[key] = isLocaleRecord(baseValue) && isLocaleRecord(overlayValue)
      ? mergeLocale(baseValue, overlayValue)
      : overlayValue
  }
  return merged
}
