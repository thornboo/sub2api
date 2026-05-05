export interface ModelCatalogEntry {
  id: string
  name: string
  providerId: string
  providerName: string
  family?: string
  context?: number
  modalities: string[]
}

const MODELS_DEV_API_URL = 'https://models.dev/api.json'

let cachedCatalog: ModelCatalogEntry[] | null = null

const isRecord = (value: unknown): value is Record<string, unknown> => (
  typeof value === 'object' && value !== null
)

const readString = (record: Record<string, unknown>, key: string): string | undefined => {
  const value = record[key]
  return typeof value === 'string' && value.trim() ? value.trim() : undefined
}

const readNumber = (record: Record<string, unknown>, key: string): number | undefined => {
  const value = record[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
}

const readModalities = (model: Record<string, unknown>): string[] => {
  const modalities = model.modalities
  if (!isRecord(modalities)) return []
  const input = Array.isArray(modalities.input) ? modalities.input : []
  const output = Array.isArray(modalities.output) ? modalities.output : []
  return Array.from(
    new Set(
      [...input, ...output]
        .filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
        .map(item => item.trim())
    )
  )
}

export function flattenModelsDevCatalog(rawCatalog: unknown): ModelCatalogEntry[] {
  if (!isRecord(rawCatalog)) return []

  const entries: ModelCatalogEntry[] = []

  for (const [providerKey, rawProvider] of Object.entries(rawCatalog)) {
    if (!isRecord(rawProvider)) continue

    const rawModels = rawProvider.models
    if (!isRecord(rawModels)) continue

    const providerId = readString(rawProvider, 'id') || providerKey
    const providerName = readString(rawProvider, 'name') || providerId

    for (const [modelKey, rawModel] of Object.entries(rawModels)) {
      if (!isRecord(rawModel)) continue

      const id = readString(rawModel, 'id') || modelKey
      const name = readString(rawModel, 'name') || id
      const family = readString(rawModel, 'family')
      const rawLimit = rawModel.limit
      const context = isRecord(rawLimit) ? readNumber(rawLimit, 'context') : undefined

      entries.push({
        id,
        name,
        providerId,
        providerName,
        family,
        context,
        modalities: readModalities(rawModel)
      })
    }
  }

  return entries
}

const scoreEntry = (entry: ModelCatalogEntry, query: string): number => {
  const id = entry.id.toLowerCase()
  const name = entry.name.toLowerCase()
  const provider = entry.providerName.toLowerCase()
  const family = (entry.family || '').toLowerCase()

  if (id === query) return 0
  if (name === query) return 1
  if (id.startsWith(query)) return 2
  if (name.startsWith(query)) return 3
  if (family.startsWith(query)) return 4
  if (provider.startsWith(query)) return 5
  if (id.includes(query)) return 6
  if (name.includes(query)) return 7
  if (family.includes(query)) return 8
  if (provider.includes(query)) return 9
  return 10
}

export function searchModelCatalogEntries(
  entries: ModelCatalogEntry[],
  query: string,
  limit = 50
): ModelCatalogEntry[] {
  const normalizedQuery = query.trim().toLowerCase()
  if (!normalizedQuery) return []
  const tokens = normalizedQuery.split(/\s+/).filter(Boolean)

  return entries
    .filter(entry => {
      const haystack = [
        entry.id,
        entry.name,
        entry.providerName,
        entry.family || '',
        ...entry.modalities
      ].join(' ').toLowerCase()
      return tokens.every(token => haystack.includes(token))
    })
    .sort((left, right) => {
      const scoreDiff = scoreEntry(left, normalizedQuery) - scoreEntry(right, normalizedQuery)
      if (scoreDiff !== 0) return scoreDiff
      return left.id.localeCompare(right.id)
    })
    .slice(0, limit)
}

export async function loadModelsDevCatalog(): Promise<ModelCatalogEntry[]> {
  if (cachedCatalog) return cachedCatalog

  const response = await fetch(MODELS_DEV_API_URL)
  if (!response.ok) {
    throw new Error(`models.dev request failed: ${response.status}`)
  }

  cachedCatalog = flattenModelsDevCatalog(await response.json())
  return cachedCatalog
}
