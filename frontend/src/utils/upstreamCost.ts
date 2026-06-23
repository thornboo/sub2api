export const UPSTREAM_RECHARGE_CNY_PER_USD_KEY = 'upstream_recharge_cny_per_usd'
export const UPSTREAM_REFERENCE_FX_RATE_KEY = 'upstream_reference_fx_rate'
export const UPSTREAM_GROUP_MULTIPLIER_KEY = 'upstream_group_multiplier'
export const UPSTREAM_COST_NOTE_KEY = 'upstream_cost_note'
export const UPSTREAM_COST_MODEL_FAMILIES_KEY = 'upstream_cost_model_families'
export const DEFAULT_UPSTREAM_REFERENCE_FX_RATE = 7

export type UpstreamCostMissingField =
  | 'recharge_cny_per_usd'
  | 'reference_fx_rate'
  | 'group_multiplier'

export interface UpstreamCostFamilyOverride {
  family: string
  group_multiplier?: number
  note?: string
}

export interface UpstreamCostProfile {
  recharge_cny_per_usd?: number
  reference_fx_rate?: number
  group_multiplier?: number
  note?: string
  model_families?: UpstreamCostFamilyOverride[]
}

export interface UpstreamCostCalculation {
  configured: boolean
  complete: boolean
  family: string
  source: 'default' | 'family_override'
  recharge_cny_per_usd?: number
  reference_fx_rate?: number
  group_multiplier?: number
  recharge_cost_factor?: number
  effective_discount?: number
  display_discount?: number
  label: string
  missing_fields: UpstreamCostMissingField[]
  note?: string
}

const DEFAULT_FAMILY = '__default__'

const normalizeString = (value: unknown): string => {
  return typeof value === 'string' ? value.trim() : ''
}

const toPositiveNumber = (value: unknown): number | undefined => {
  const num = typeof value === 'string' ? Number(value.trim()) : Number(value)
  if (!Number.isFinite(num) || num <= 0) return undefined
  return num
}

const normalizeFamily = (value: unknown): string => normalizeString(value)

const cloneExtraWithoutCostKeys = (base?: Record<string, unknown>): Record<string, unknown> => {
  const next: Record<string, unknown> = { ...(base || {}) }
  delete next[UPSTREAM_RECHARGE_CNY_PER_USD_KEY]
  delete next[UPSTREAM_REFERENCE_FX_RATE_KEY]
  delete next[UPSTREAM_GROUP_MULTIPLIER_KEY]
  delete next[UPSTREAM_COST_NOTE_KEY]
  delete next[UPSTREAM_COST_MODEL_FAMILIES_KEY]
  return next
}

export const normalizeUpstreamCostProfile = (profile?: UpstreamCostProfile | null): UpstreamCostProfile => {
  if (!profile) return {}

  const normalized: UpstreamCostProfile = {}
  const recharge = toPositiveNumber(profile.recharge_cny_per_usd)
  const fx = toPositiveNumber(profile.reference_fx_rate)
  const multiplier = toPositiveNumber(profile.group_multiplier)
  const note = normalizeString(profile.note)

  if (recharge !== undefined) normalized.recharge_cny_per_usd = recharge
  if (fx !== undefined) normalized.reference_fx_rate = fx
  if (multiplier !== undefined) normalized.group_multiplier = multiplier
  if (note) normalized.note = note

  const seen = new Set<string>()
  const dedupedFamilies: UpstreamCostFamilyOverride[] = []
  for (const item of profile.model_families || []) {
    const family = normalizeFamily(item.family)
    const groupMultiplier = toPositiveNumber(item.group_multiplier)
    const itemNote = normalizeString(item.note)
    if (!family || groupMultiplier === undefined) continue

    const key = family.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    dedupedFamilies.push({
      family,
      group_multiplier: groupMultiplier,
      ...(itemNote ? { note: itemNote } : {})
    })
  }
  if (dedupedFamilies.length > 0) normalized.model_families = dedupedFamilies

  return normalized
}

export const hasUpstreamCostProfile = (profile?: UpstreamCostProfile | null): boolean => {
  const normalized = normalizeUpstreamCostProfile(profile)
  return Boolean(
    normalized.recharge_cny_per_usd !== undefined ||
    normalized.reference_fx_rate !== undefined ||
    normalized.group_multiplier !== undefined ||
    normalized.note ||
    (normalized.model_families?.length ?? 0) > 0
  )
}

export const readUpstreamCostProfile = (extra?: Record<string, unknown> | null): UpstreamCostProfile => {
  if (!extra) return {}

  const profile: UpstreamCostProfile = {
    recharge_cny_per_usd: toPositiveNumber(extra[UPSTREAM_RECHARGE_CNY_PER_USD_KEY]),
    reference_fx_rate: toPositiveNumber(extra[UPSTREAM_REFERENCE_FX_RATE_KEY]),
    group_multiplier: toPositiveNumber(extra[UPSTREAM_GROUP_MULTIPLIER_KEY]),
    note: normalizeString(extra[UPSTREAM_COST_NOTE_KEY]) || undefined
  }

  const rawFamilies = extra[UPSTREAM_COST_MODEL_FAMILIES_KEY]
  const families: UpstreamCostFamilyOverride[] = []

  if (Array.isArray(rawFamilies)) {
    for (const item of rawFamilies) {
      if (!item || typeof item !== 'object') continue
      const entry = item as Record<string, unknown>
      const family = normalizeFamily(entry.family)
      const groupMultiplier = toPositiveNumber(entry.group_multiplier)
      const note = normalizeString(entry.note)
      if (!family || groupMultiplier === undefined) continue
      families.push({
        family,
        group_multiplier: groupMultiplier,
        ...(note ? { note } : {})
      })
    }
  } else if (rawFamilies && typeof rawFamilies === 'object') {
    for (const [family, value] of Object.entries(rawFamilies as Record<string, unknown>)) {
      const familyName = normalizeFamily(family)
      if (!familyName) continue
      if (value && typeof value === 'object') {
        const entry = value as Record<string, unknown>
        const groupMultiplier = toPositiveNumber(entry.group_multiplier)
        const note = normalizeString(entry.note)
        if (groupMultiplier === undefined) continue
        families.push({
          family: familyName,
          group_multiplier: groupMultiplier,
          ...(note ? { note } : {})
        })
      } else {
        const groupMultiplier = toPositiveNumber(value)
        if (groupMultiplier === undefined) continue
        families.push({ family: familyName, group_multiplier: groupMultiplier })
      }
    }
  }

  if (families.length > 0) {
    profile.model_families = families
  }

  return normalizeUpstreamCostProfile(profile)
}

export const mergeUpstreamCostProfileExtra = (
  base: Record<string, unknown> | undefined,
  profile?: UpstreamCostProfile | null
): Record<string, unknown> => {
  const next = cloneExtraWithoutCostKeys(base)
  const normalized = normalizeUpstreamCostProfile(profile)

  if (normalized.recharge_cny_per_usd !== undefined) {
    next[UPSTREAM_RECHARGE_CNY_PER_USD_KEY] = normalized.recharge_cny_per_usd
  }
  if (normalized.reference_fx_rate !== undefined) {
    next[UPSTREAM_REFERENCE_FX_RATE_KEY] = normalized.reference_fx_rate
  }
  if (normalized.group_multiplier !== undefined) {
    next[UPSTREAM_GROUP_MULTIPLIER_KEY] = normalized.group_multiplier
  }
  if (normalized.note) {
    next[UPSTREAM_COST_NOTE_KEY] = normalized.note
  }
  if (normalized.model_families && normalized.model_families.length > 0) {
    next[UPSTREAM_COST_MODEL_FAMILIES_KEY] = normalized.model_families.map((item) => ({
      family: item.family,
      group_multiplier: item.group_multiplier,
      ...(item.note ? { note: item.note } : {})
    }))
  }

  return next
}

export const maybeMergeUpstreamCostProfileExtra = (
  base: Record<string, unknown> | undefined,
  profile?: UpstreamCostProfile | null
): Record<string, unknown> | undefined => {
  const next = mergeUpstreamCostProfileExtra(base, profile)
  return Object.keys(next).length > 0 ? next : undefined
}

export const getUpstreamCostFamilies = (profile?: UpstreamCostProfile | null): string[] => {
  const normalized = normalizeUpstreamCostProfile(profile)
  return (normalized.model_families || []).map(item => item.family)
}

export const calculateUpstreamCost = (
  profile?: UpstreamCostProfile | null,
  family = DEFAULT_FAMILY
): UpstreamCostCalculation => {
  const normalized = normalizeUpstreamCostProfile(profile)
  const wantedFamily = normalizeFamily(family)
  const familyOverride = wantedFamily && wantedFamily !== DEFAULT_FAMILY
    ? normalized.model_families?.find(item => item.family.toLowerCase() === wantedFamily.toLowerCase())
    : undefined

  const recharge = normalized.recharge_cny_per_usd
  const fx = normalized.reference_fx_rate ?? DEFAULT_UPSTREAM_REFERENCE_FX_RATE
  const groupMultiplier = familyOverride?.group_multiplier ?? normalized.group_multiplier
  const note = familyOverride?.note || normalized.note

  const missingFields: UpstreamCostMissingField[] = []
  if (recharge === undefined) missingFields.push('recharge_cny_per_usd')
  if (groupMultiplier === undefined) missingFields.push('group_multiplier')

  const complete = missingFields.length === 0
  if (!complete) {
    return {
      configured: hasUpstreamCostProfile(normalized),
      complete: false,
      family: familyOverride?.family || wantedFamily || DEFAULT_FAMILY,
      source: familyOverride ? 'family_override' : 'default',
      recharge_cny_per_usd: recharge,
      reference_fx_rate: fx,
      group_multiplier: groupMultiplier,
      label: '未配置',
      missing_fields: missingFields,
      note
    }
  }

  const rechargeCostFactor = recharge! / fx!
  const effectiveDiscount = rechargeCostFactor * groupMultiplier!
  const displayDiscount = effectiveDiscount * 10

  return {
    configured: true,
    complete: true,
    family: familyOverride?.family || wantedFamily || DEFAULT_FAMILY,
    source: familyOverride ? 'family_override' : 'default',
    recharge_cny_per_usd: recharge,
    reference_fx_rate: fx,
    group_multiplier: groupMultiplier,
    recharge_cost_factor: rechargeCostFactor,
    effective_discount: effectiveDiscount,
    display_discount: displayDiscount,
    label: formatUpstreamDiscountLabel(displayDiscount),
    missing_fields: [],
    note
  }
}

export const formatUpstreamDiscountLabel = (displayDiscount?: number): string => {
  if (!Number.isFinite(displayDiscount)) return '未配置'
  return `${Number(displayDiscount).toFixed(1)}折`
}

export const formatUpstreamRatio = (value?: number): string => {
  if (!Number.isFinite(value)) return '-'
  return Number(value).toFixed(3).replace(/0+$/, '').replace(/\.$/, '')
}

export const DEFAULT_UPSTREAM_COST_FAMILY = DEFAULT_FAMILY
