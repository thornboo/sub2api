export const UPSTREAM_RECHARGE_CNY_PER_USD_KEY = 'upstream_recharge_cny_per_usd'
export const UPSTREAM_REFERENCE_FX_RATE_KEY = 'upstream_reference_fx_rate'
export const UPSTREAM_GROUP_MULTIPLIER_KEY = 'upstream_group_multiplier'
export const UPSTREAM_COST_NOTE_KEY = 'upstream_cost_note'
export const UPSTREAM_COST_MODEL_FAMILIES_KEY = 'upstream_cost_model_families'
export const UPSTREAM_BALANCE_QUERY_ENABLED_KEY = 'upstream_balance_query_enabled'
export const UPSTREAM_BALANCE_PROVIDER_KEY = 'upstream_balance_provider'
export const UPSTREAM_BALANCE_ENDPOINT_KEY = 'upstream_balance_endpoint'
export const UPSTREAM_BALANCE_AUTH_MODE_KEY = 'upstream_balance_auth_mode'
export const UPSTREAM_BALANCE_AUTH_HEADER_KEY = 'upstream_balance_auth_header'
export const UPSTREAM_BALANCE_SNAPSHOT_KEY = 'upstream_balance_snapshot'
export const DEFAULT_UPSTREAM_REFERENCE_FX_RATE = 7
export const UPSTREAM_PRICE_REFERENCE_CURRENCY_CNY = 'CNY'
export const UPSTREAM_PRICE_REFERENCE_CURRENCY_USD = 'USD'
export const UPSTREAM_BALANCE_PROVIDER_SUB2API = 'sub2api'
export const UPSTREAM_BALANCE_PROVIDER_NEW_API = 'new_api_compatible'
export const DEFAULT_UPSTREAM_BALANCE_PROVIDER = UPSTREAM_BALANCE_PROVIDER_SUB2API
export const DEFAULT_UPSTREAM_BALANCE_ENDPOINT = '/v1/usage'
export const SUB2API_PROFILE_UPSTREAM_BALANCE_ENDPOINT = '/api/v1/user/profile'
export const NEW_API_UPSTREAM_BALANCE_ENDPOINT = '/api/usage/token/'
export const UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY = 'account_api_key'
export const UPSTREAM_BALANCE_AUTH_MODE_BEARER_TOKEN = 'bearer_token'
export const UPSTREAM_BALANCE_AUTH_MODE_CUSTOM_HEADER = 'custom_header'

export type UpstreamBalanceProvider =
  | typeof UPSTREAM_BALANCE_PROVIDER_SUB2API
  | typeof UPSTREAM_BALANCE_PROVIDER_NEW_API

export type UpstreamBalanceAuthMode =
  | typeof UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY
  | typeof UPSTREAM_BALANCE_AUTH_MODE_BEARER_TOKEN
  | typeof UPSTREAM_BALANCE_AUTH_MODE_CUSTOM_HEADER

export type UpstreamPriceReferenceCurrency =
  | typeof UPSTREAM_PRICE_REFERENCE_CURRENCY_CNY
  | typeof UPSTREAM_PRICE_REFERENCE_CURRENCY_USD

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
  balance_query_enabled?: boolean
  balance_provider?: UpstreamBalanceProvider
  balance_endpoint?: string
  balance_auth_mode?: UpstreamBalanceAuthMode
  balance_auth_header?: string
}

export interface UpstreamBalanceSnapshot {
  provider?: string
  status?: 'ok' | 'error' | string
  endpoint?: string
  raw_unit?: string
  raw_available?: number | null
  raw_used?: number | null
  raw_granted?: number | null
  available_usd?: number | null
  unlimited?: boolean
  expires_at?: string | null
  fetched_at?: string
  status_code?: number
  error?: string
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

export const normalizeUpstreamPriceReferenceCurrency = (
  value: unknown
): UpstreamPriceReferenceCurrency => (
  value === UPSTREAM_PRICE_REFERENCE_CURRENCY_CNY
    ? UPSTREAM_PRICE_REFERENCE_CURRENCY_CNY
    : UPSTREAM_PRICE_REFERENCE_CURRENCY_USD
)

export const calculateUpstreamBindingEffectiveFactor = (
  currentEffectiveCNYPerUSD: unknown,
  referenceFXRate: unknown,
  groupMultiplier: unknown,
  priceReferenceCurrency: unknown
): number | null => {
  const cost = toPositiveNumber(currentEffectiveCNYPerUSD)
  const multiplier = toPositiveNumber(groupMultiplier)
  if (cost === undefined || multiplier === undefined) return null

  const currency = normalizeUpstreamPriceReferenceCurrency(priceReferenceCurrency)
  if (currency === UPSTREAM_PRICE_REFERENCE_CURRENCY_CNY) {
    return cost * multiplier
  }

  const fx = toPositiveNumber(referenceFXRate)
  if (fx === undefined) return null
  return (cost / fx) * multiplier
}

const normalizeFamily = (value: unknown): string => normalizeString(value)

export const defaultUpstreamBalanceEndpoint = (
  provider: UpstreamBalanceProvider = DEFAULT_UPSTREAM_BALANCE_PROVIDER
): string => (
  provider === UPSTREAM_BALANCE_PROVIDER_NEW_API
    ? NEW_API_UPSTREAM_BALANCE_ENDPOINT
    : DEFAULT_UPSTREAM_BALANCE_ENDPOINT
)

export const defaultUpstreamBalanceAuthMode = (
  provider: UpstreamBalanceProvider = DEFAULT_UPSTREAM_BALANCE_PROVIDER
): UpstreamBalanceAuthMode => (
  provider === UPSTREAM_BALANCE_PROVIDER_NEW_API || provider === UPSTREAM_BALANCE_PROVIDER_SUB2API
    ? UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY
    : UPSTREAM_BALANCE_AUTH_MODE_BEARER_TOKEN
)

export const normalizeUpstreamBalanceAuthMode = (
  provider: UpstreamBalanceProvider = DEFAULT_UPSTREAM_BALANCE_PROVIDER,
  authMode?: UpstreamBalanceAuthMode
): UpstreamBalanceAuthMode => {
  return authMode || defaultUpstreamBalanceAuthMode(provider)
}

const normalizeEndpointPath = (endpoint?: string): string => {
  const value = normalizeString(endpoint)
  if (!value) return ''
  try {
    const parsed = new URL(value, 'http://local.invalid')
    return parsed.pathname.replace(/\/+$/, '') || '/'
  } catch {
    const withSlash = value.startsWith('/') ? value : `/${value}`
    return withSlash.replace(/\/+$/, '') || '/'
  }
}

export const normalizeUpstreamBalanceEndpoint = (
  provider: UpstreamBalanceProvider = DEFAULT_UPSTREAM_BALANCE_PROVIDER,
  endpoint?: string,
  authMode?: UpstreamBalanceAuthMode
): string => {
  const mode = normalizeUpstreamBalanceAuthMode(provider, authMode)
  const normalizedEndpoint = normalizeEndpointPath(endpoint)
  if (
    provider === UPSTREAM_BALANCE_PROVIDER_SUB2API &&
    mode === UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY &&
    normalizedEndpoint === SUB2API_PROFILE_UPSTREAM_BALANCE_ENDPOINT
  ) {
    return DEFAULT_UPSTREAM_BALANCE_ENDPOINT
  }
  return normalizeString(endpoint) || defaultUpstreamBalanceEndpoint(provider)
}

const cloneExtraWithoutCostKeys = (base?: Record<string, unknown>): Record<string, unknown> => {
  const next: Record<string, unknown> = { ...(base || {}) }
  delete next[UPSTREAM_RECHARGE_CNY_PER_USD_KEY]
  delete next[UPSTREAM_REFERENCE_FX_RATE_KEY]
  delete next[UPSTREAM_GROUP_MULTIPLIER_KEY]
  delete next[UPSTREAM_COST_NOTE_KEY]
  delete next[UPSTREAM_COST_MODEL_FAMILIES_KEY]
  delete next[UPSTREAM_BALANCE_QUERY_ENABLED_KEY]
  delete next[UPSTREAM_BALANCE_PROVIDER_KEY]
  delete next[UPSTREAM_BALANCE_ENDPOINT_KEY]
  delete next[UPSTREAM_BALANCE_AUTH_MODE_KEY]
  delete next[UPSTREAM_BALANCE_AUTH_HEADER_KEY]
  delete next.upstream_account_balance_query_enabled
  delete next.upstream_account_balance_provider
  delete next.upstream_account_balance_endpoint
  delete next.upstream_account_balance_auth_mode
  delete next.upstream_account_balance_auth_header
  return next
}

export const normalizeUpstreamCostProfile = (profile?: UpstreamCostProfile | null): UpstreamCostProfile => {
  if (!profile) return {}

  const normalized: UpstreamCostProfile = {}
  const recharge = toPositiveNumber(profile.recharge_cny_per_usd)
  const fx = toPositiveNumber(profile.reference_fx_rate)
  const multiplier = toPositiveNumber(profile.group_multiplier)
  const note = normalizeString(profile.note)
  const balanceEndpoint = normalizeString(profile.balance_endpoint)
  const balanceAuthHeader = normalizeString(profile.balance_auth_header)

  if (recharge !== undefined) normalized.recharge_cny_per_usd = recharge
  if (fx !== undefined) normalized.reference_fx_rate = fx
  if (multiplier !== undefined) normalized.group_multiplier = multiplier
  if (note) normalized.note = note
  if (typeof profile.balance_query_enabled === 'boolean') {
    normalized.balance_query_enabled = profile.balance_query_enabled
  }
  if (
    profile.balance_provider === UPSTREAM_BALANCE_PROVIDER_SUB2API ||
    profile.balance_provider === UPSTREAM_BALANCE_PROVIDER_NEW_API
  ) {
    normalized.balance_provider = profile.balance_provider
  }
  if (balanceEndpoint) normalized.balance_endpoint = balanceEndpoint
  if (
    profile.balance_auth_mode === UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY ||
    profile.balance_auth_mode === UPSTREAM_BALANCE_AUTH_MODE_BEARER_TOKEN ||
    profile.balance_auth_mode === UPSTREAM_BALANCE_AUTH_MODE_CUSTOM_HEADER
  ) {
    normalized.balance_auth_mode = profile.balance_auth_mode
  }
  if (balanceAuthHeader) normalized.balance_auth_header = balanceAuthHeader

  if (normalized.balance_query_enabled === true) {
    const provider = normalized.balance_provider || DEFAULT_UPSTREAM_BALANCE_PROVIDER
    normalized.balance_auth_mode = normalizeUpstreamBalanceAuthMode(provider, normalized.balance_auth_mode)
    normalized.balance_endpoint = normalizeUpstreamBalanceEndpoint(provider, normalized.balance_endpoint, normalized.balance_auth_mode)
  }

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
    note: normalizeString(extra[UPSTREAM_COST_NOTE_KEY]) || undefined,
    balance_query_enabled: typeof extra[UPSTREAM_BALANCE_QUERY_ENABLED_KEY] === 'boolean'
      ? (extra[UPSTREAM_BALANCE_QUERY_ENABLED_KEY] as boolean)
      : undefined,
    balance_provider: extra[UPSTREAM_BALANCE_PROVIDER_KEY] === UPSTREAM_BALANCE_PROVIDER_SUB2API ||
      extra[UPSTREAM_BALANCE_PROVIDER_KEY] === UPSTREAM_BALANCE_PROVIDER_NEW_API
      ? (extra[UPSTREAM_BALANCE_PROVIDER_KEY] as UpstreamBalanceProvider)
      : undefined,
    balance_endpoint: normalizeString(extra[UPSTREAM_BALANCE_ENDPOINT_KEY]) || undefined,
    balance_auth_mode: (
      extra[UPSTREAM_BALANCE_AUTH_MODE_KEY] === UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY ||
      extra[UPSTREAM_BALANCE_AUTH_MODE_KEY] === UPSTREAM_BALANCE_AUTH_MODE_BEARER_TOKEN ||
      extra[UPSTREAM_BALANCE_AUTH_MODE_KEY] === UPSTREAM_BALANCE_AUTH_MODE_CUSTOM_HEADER
    )
      ? extra[UPSTREAM_BALANCE_AUTH_MODE_KEY] as UpstreamBalanceAuthMode
      : undefined,
    balance_auth_header: normalizeString(extra[UPSTREAM_BALANCE_AUTH_HEADER_KEY]) || undefined
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
  if (typeof normalized.balance_query_enabled === 'boolean') {
    next[UPSTREAM_BALANCE_QUERY_ENABLED_KEY] = normalized.balance_query_enabled
    if (normalized.balance_query_enabled) {
      const provider = normalized.balance_provider || DEFAULT_UPSTREAM_BALANCE_PROVIDER
      const authMode = normalizeUpstreamBalanceAuthMode(provider, normalized.balance_auth_mode)
      next[UPSTREAM_BALANCE_PROVIDER_KEY] = provider
      next[UPSTREAM_BALANCE_ENDPOINT_KEY] = normalizeUpstreamBalanceEndpoint(provider, normalized.balance_endpoint, authMode)
      next[UPSTREAM_BALANCE_AUTH_MODE_KEY] = authMode
      if (authMode === UPSTREAM_BALANCE_AUTH_MODE_CUSTOM_HEADER) {
        next[UPSTREAM_BALANCE_AUTH_HEADER_KEY] = normalized.balance_auth_header || 'Authorization'
      }
    }
  }
  return next
}

export const readUpstreamBalanceSnapshot = (extra?: Record<string, unknown> | null): UpstreamBalanceSnapshot | null => {
  const raw = extra?.[UPSTREAM_BALANCE_SNAPSHOT_KEY]
  if (!raw || typeof raw !== 'object') return null
  return raw as UpstreamBalanceSnapshot
}

export const isUpstreamBalanceQueryEnabled = (extra?: Record<string, unknown> | null): boolean => {
  return extra?.[UPSTREAM_BALANCE_QUERY_ENABLED_KEY] === true
}

export const readUpstreamKeyQuotaSnapshot = readUpstreamBalanceSnapshot

export const isUpstreamKeyQuotaQueryEnabled = isUpstreamBalanceQueryEnabled

export const requiresUpstreamBalanceAuthToken = (profile?: UpstreamCostProfile | null): boolean => {
  const normalized = normalizeUpstreamCostProfile(profile)
  if (normalized.balance_query_enabled !== true) return false
  const provider = normalized.balance_provider || DEFAULT_UPSTREAM_BALANCE_PROVIDER
  return normalizeUpstreamBalanceAuthMode(provider, normalized.balance_auth_mode) !== UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY
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
  family = DEFAULT_FAMILY,
  labelOptions: UpstreamDiscountLabelOptions = {}
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
      label: labelOptions.notConfiguredLabel ?? '未配置',
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
    label: formatUpstreamDiscountLabel(displayDiscount, labelOptions),
    missing_fields: [],
    note
  }
}

export interface UpstreamDiscountLabelOptions {
  suffix?: string
  notConfiguredLabel?: string
}

export const formatUpstreamDiscountLabel = (
  displayDiscount?: number,
  options: UpstreamDiscountLabelOptions = {}
): string => {
  if (!Number.isFinite(displayDiscount)) return options.notConfiguredLabel ?? '未配置'
  return `${Number(displayDiscount).toFixed(1)}${options.suffix ?? '折'}`
}

export const formatUpstreamRatio = (value?: number): string => {
  if (!Number.isFinite(value)) return '-'
  return Number(value).toFixed(3).replace(/0+$/, '').replace(/\.$/, '')
}

export const DEFAULT_UPSTREAM_COST_FAMILY = DEFAULT_FAMILY
