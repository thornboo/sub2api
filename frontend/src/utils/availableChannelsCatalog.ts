import type {
  UserTimePricing,
  UserTimePricingRule,
  UserAvailableChannel,
  UserAvailableGroup,
  UserPricingInterval,
  UserSupportedModelPricing,
  UserSupportedModel,
} from '@/api/channels'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_PER_REQUEST,
  BILLING_MODE_TOKEN,
  type BillingMode,
} from '@/constants/channel'
import { resolveAvailableModelGroupContexts } from '@/utils/availableChannelCallability'
import { formatScaled } from '@/utils/pricing'

export type AvailableChannelGroupScope = 'all' | 'public' | 'public_exclusive' | 'exclusive'
export type AvailableChannelPriceStatus = 'all' | 'priced' | 'unpriced'
export type AvailableChannelPricingStatus = Exclude<AvailableChannelPriceStatus, 'all'>
export type AvailableChannelStatusScope = 'all' | 'active' | 'disabled'
export type AvailableChannelSortOrder = 'asc' | 'desc'
export type AvailableChannelSortKey =
  | 'model'
  | 'platform'
  | 'channel'
  | 'billingMode'
  | 'interval'
  | 'inputPrice'
  | 'outputPrice'
  | 'cacheWritePrice'
  | 'cacheReadPrice'
  | 'imageOutputPrice'
  | 'perRequestPrice'
export interface AvailableChannelCatalogRow {
  id: string
  channelName: string
  channelDescription: string
  channelStatus?: string
  platform: string
  modelName: string
  groups: UserAvailableGroup[]
  effectiveRateMultiplier: number
  pricing: UserSupportedModelPricing | null
  interval: UserPricingInterval | null
  intervalLabel: string
  priceStatus: AvailableChannelPricingStatus
}

export interface AvailableChannelCatalogOptions {
  includeSubscriptionGroups?: boolean
  groupScope?: AvailableChannelGroupScope
  billingMode?: BillingMode | ''
  priceStatus?: AvailableChannelPriceStatus
  statusScope?: AvailableChannelStatusScope
  expandIntervals?: boolean
  sortBy?: AvailableChannelSortKey
  sortOrder?: AvailableChannelSortOrder
  activeOnly?: boolean
  userGroupRates?: Record<number, number>
}

export interface AvailableChannelPricingLabels {
  billingModeToken: string
  billingModePerRequest: string
  billingModeImage: string
  noPricing: string
  unitPerMillion: string
  unitPerRequest: string
}

export interface TimePricingDisplayRow {
  id: string
  source: 'implicit' | 'rule'
  startTime: string | null
  endTime: string | null
  windowLabel: string
  label: string
  multiplier: number
  active: boolean
  inputPrice: number | null
  outputPrice: number | null
  cacheWritePrice: number | null
  cacheReadPrice: number | null
}

export interface AvailableChannelExportLabels extends AvailableChannelPricingLabels {
  sheetName: string
  channel: string
  status: string
  description: string
  platform: string
  model: string
  groups: string
  billingMode: string
  interval: string
  inputPrice: string
  outputPrice: string
  cacheWritePrice: string
  cacheReadPrice: string
  imageOutputPrice: string
  perRequestPrice: string
  statusActive: string
  statusDisabled: string
  statusUnknown: string
}

const PER_MILLION_SCALE = 1_000_000
const EXCEL_MIME = 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
const ACTIVE_STATUS = 'active'
const DISABLED_STATUS = 'disabled'

function arrayOrEmpty<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}

export function buildAvailableChannelCatalogRows(
  channels: UserAvailableChannel[],
  options: AvailableChannelCatalogOptions = {},
): AvailableChannelCatalogRow[] {
  const rows: AvailableChannelCatalogRow[] = []
  const includeSubscriptionGroups = options.includeSubscriptionGroups ?? true
  const groupScope = options.groupScope ?? 'all'
  const expandIntervals = options.expandIntervals ?? false
  const sortBy = options.sortBy ?? 'model'
  const sortOrder = options.sortOrder ?? 'asc'
  const priceStatus = options.priceStatus ?? 'all'
  const statusScope = options.statusScope ?? 'all'

  channels.forEach((channel, channelIndex) => {
    const channelStatus = getChannelStatus(channel)
    if (!matchesChannelStatus(channelStatus, statusScope, options.activeOnly)) return

    arrayOrEmpty(channel.platforms).forEach((section, sectionIndex) => {
      const sectionGroups = arrayOrEmpty(section.groups)
      const supportedModels = arrayOrEmpty(section.supported_models)
      const groups = filterCatalogGroups(sectionGroups, includeSubscriptionGroups, groupScope)
      if (sectionGroups.length > 0 && groups.length === 0) return

      supportedModels.forEach((model, modelIndex) => {
        const modelGroups = resolveAvailableModelGroupContexts(model, groups)
          .map(({ group }) => group)
        const callabilityMetadataPresent = Array.isArray(model.route_group_ids)
          || Array.isArray(model.supported_endpoints)
        if (modelGroups.length === 0 && (sectionGroups.length > 0 || callabilityMetadataPresent)) return

        const groupRows = modelGroups.length > 0 ? modelGroups.map((group) => [group]) : [[]]

        groupRows.forEach((rowGroups, groupIndex) => {
          const group = rowGroups[0] ?? null
          const pricing = resolveAvailableModelGroupPricing(model, group)
          if (options.billingMode && pricing?.billing_mode !== options.billingMode) return
          const intervals = expandIntervals ? getValuedIntervals(pricing) : []
          const baseRow = {
            channelName: channel.name,
            channelDescription: channel.description || '',
            channelStatus,
            platform: section.platform,
            modelName: model.name,
            groups: rowGroups,
            effectiveRateMultiplier: resolveAvailableGroupPriceMultiplier(
              group,
              options.userGroupRates,
              pricing?.billing_mode,
            ),
            pricing,
          }

          const pushRow = (interval: UserPricingInterval | null, intervalIndex: number) => {
            const row: AvailableChannelCatalogRow = {
              ...baseRow,
              id: [
                channel.name,
                section.platform,
                model.name,
                group?.id ?? 'no-group',
                intervalIndex,
                channelIndex,
                sectionIndex,
                modelIndex,
                groupIndex,
              ].join('::'),
              interval,
              intervalLabel: formatAvailableChannelIntervalLabel(interval),
              priceStatus: rowHasPricing({ pricing, interval }) ? 'priced' : 'unpriced',
            }
            if (priceStatus !== 'all' && row.priceStatus !== priceStatus) return
            rows.push(row)
          }

          if (intervals.length > 0) {
            intervals.forEach((interval, intervalIndex) => pushRow(interval, intervalIndex))
            return
          }
          pushRow(null, 0)
        })
      })
    })
  })

  return sortAvailableChannelCatalogRows(rows, sortBy, sortOrder)
}

export function sortAvailableChannelCatalogRows(
  rows: AvailableChannelCatalogRow[],
  sortBy: AvailableChannelSortKey,
  sortOrder: AvailableChannelSortOrder,
): AvailableChannelCatalogRow[] {
  const direction = sortOrder === 'asc' ? 1 : -1
  return [...rows].sort((a, b) => {
    const primary = compareSortValue(getSortValue(a, sortBy), getSortValue(b, sortBy), direction)
    if (primary !== 0) return primary
    return compareDefaultOrder(a, b)
  })
}

export function formatBillingMode(
  pricing: UserSupportedModelPricing | null,
  labels: AvailableChannelPricingLabels,
): string {
  switch (pricing?.billing_mode) {
    case BILLING_MODE_TOKEN:
      return labels.billingModeToken
    case BILLING_MODE_PER_REQUEST:
      return labels.billingModePerRequest
    case BILLING_MODE_IMAGE:
      return labels.billingModeImage
    default:
      return labels.noPricing
  }
}

export function formatTokenPrice(value: number | null, labels: AvailableChannelPricingLabels): string {
  if (value == null) return '-'
  return `${formatScaled(value, PER_MILLION_SCALE)} ${labels.unitPerMillion}`
}

export function formatCompactTokenPrice(value: number | null): string {
  if (value == null) return '-'
  return formatScaled(value, PER_MILLION_SCALE)
}

export function formatTimePricingTokenPrice(value: number | null): string {
  if (value == null) return '—'
  return formatScaled(value, PER_MILLION_SCALE, 4)
}

export function formatRequestPrice(value: number | null, labels: AvailableChannelPricingLabels): string {
  if (value == null) return '-'
  return `${formatScaled(value, 1)} ${labels.unitPerRequest}`
}

export function formatCompactRequestPrice(value: number | null): string {
  if (value == null) return '-'
  return formatScaled(value, 1)
}

export function formatAvailableChannelGroups(
  groups: UserAvailableGroup[],
  userGroupRates: Record<number, number>,
): string {
  if (groups.length === 0) return '-'
  return groups
    .map((group) => {
      const rate = userGroupRates[group.id] ?? group.rate_multiplier
      return `${group.name} ${formatRateMultiplier(rate)}`
    })
    .join('; ')
}

export function resolveAvailableGroupRateMultiplier(
  group: UserAvailableGroup | null | undefined,
  userGroupRates: Record<number, number> = {},
): number {
  if (!group) return 1
  return userGroupRates[group.id] ?? group.rate_multiplier ?? 1
}

/**
 * Resolve the multiplier used by the actual billing path. Image models can opt
 * into a dedicated multiplier which replaces, rather than compounds with, the
 * group or user-specific token multiplier.
 */
export function resolveAvailableGroupPriceMultiplier(
  group: UserAvailableGroup | null | undefined,
  userGroupRates: Record<number, number> = {},
  billingMode?: BillingMode | null,
): number {
  if (billingMode === BILLING_MODE_IMAGE && group?.image_rate_independent) {
    return group.image_rate_multiplier ?? 1
  }
  return resolveAvailableGroupRateMultiplier(group, userGroupRates)
}

/**
 * Build group-specific image tiers using the same precedence as settlement:
 * group tier price, channel tier price, then the channel default request price.
 * A clone is returned so shared channel pricing remains immutable.
 */
export function resolveAvailableGroupDisplayPricing(
  pricing: UserSupportedModelPricing | null,
  group: UserAvailableGroup | null | undefined,
): UserSupportedModelPricing | null {
  if (pricing?.billing_mode !== BILLING_MODE_IMAGE || !group) return pricing

  const groupTiers = [
    { label: '1K', price: group.image_price_1k },
    { label: '2K', price: group.image_price_2k },
    { label: '4K', price: group.image_price_4k },
  ]
  if (groupTiers.every((tier) => tier.price == null)) return pricing

  const intervals = groupTiers.flatMap(({ label, price }) => {
    const channelTierPrice = arrayOrEmpty(pricing.intervals).find(
      (interval) => interval.tier_label === label && interval.per_request_price != null,
    )?.per_request_price
    const effectivePrice = price ?? channelTierPrice ?? pricing.per_request_price
    if (effectivePrice == null) return []
    return [{
      min_tokens: 0,
      max_tokens: null,
      tier_label: label,
      input_price: null,
      output_price: null,
      cache_write_price: null,
      cache_read_price: null,
      per_request_price: effectivePrice,
    } satisfies UserPricingInterval]
  })

  return { ...pricing, intervals }
}

export function resolveAvailableModelGroupPricing(
  model: Pick<UserSupportedModel, 'pricing' | 'group_pricing'>,
  group: UserAvailableGroup | null | undefined,
): UserSupportedModelPricing | null {
  const groupPricing = group == null
    ? undefined
    : arrayOrEmpty(model.group_pricing).find((entry) => entry.group_id === group.id)?.pricing
  return resolveAvailableGroupDisplayPricing(groupPricing ?? model.pricing, group)
}

export function formatRateMultiplier(rate: number | null | undefined): string {
  if (rate == null) return '1x'
  return `${Number(rate.toFixed(4)).toString()}x`
}

export function hasEnabledTimePricing(
  pricing: Pick<UserSupportedModelPricing, 'billing_mode' | 'time_pricing'> | null | undefined,
): boolean {
  return pricing?.billing_mode === BILLING_MODE_TOKEN
    && Boolean(pricing.time_pricing?.enabled)
    && isValidTimeZone(pricing.time_pricing?.timezone)
}

export function getActiveTimePricingMultiplier(
  pricing: Pick<UserSupportedModelPricing, 'billing_mode' | 'time_pricing'> | null | undefined,
  at: Date = new Date(),
): number {
  if (!hasEnabledTimePricing(pricing)) return 1
  const activeRule = findActiveTimePricingRule(pricing?.time_pricing, at)
  return activeRule == null
    ? resolveDefaultTimePricingMultiplier(pricing?.time_pricing)
    : normalizeTimePricingMultiplier(activeRule.multiplier)
}

export function buildTimePricingDisplayRows(
  pricing: UserSupportedModelPricing,
  labels: { otherTimes: string; unnamedType: string },
  at: Date = new Date(),
): TimePricingDisplayRow[] {
  if (!hasEnabledTimePricing(pricing)) return []

  const timePricing = pricing.time_pricing as UserTimePricing
  const activeRule = findActiveTimePricingRule(timePricing, at)
  const explicitRows = timePricing.rules
    .filter((rule) => isValidRuleShape(rule))
    .map((rule, index) => buildTimePricingRow({
      id: `rule-${index}-${rule.start_time}-${rule.end_time}`,
      source: 'rule',
      startTime: rule.start_time,
      endTime: rule.end_time,
      windowLabel: formatTimePricingWindow(rule.start_time, rule.end_time),
      label: normalizeTimePricingLabel(rule.label, labels.unnamedType),
      multiplier: normalizeTimePricingMultiplier(rule.multiplier),
      active: rule === activeRule,
      pricing,
    }))

  return [
    buildTimePricingRow({
      id: 'implicit-default',
      source: 'implicit',
      startTime: null,
      endTime: null,
      windowLabel: labels.otherTimes,
      label: normalizeTimePricingLabel(timePricing.default_label, labels.unnamedType),
      multiplier: resolveDefaultTimePricingMultiplier(timePricing),
      active: activeRule == null,
      pricing,
    }),
    ...explicitRows,
  ]
}

export function formatChannelStatus(
  status: string | undefined,
  labels: Pick<AvailableChannelExportLabels, 'statusActive' | 'statusDisabled' | 'statusUnknown'>,
): string {
  switch (status) {
    case ACTIVE_STATUS:
      return labels.statusActive
    case DISABLED_STATUS:
      return labels.statusDisabled
    default:
      return status || labels.statusUnknown
  }
}

export function formatAvailableChannelIntervalLabel(interval: UserPricingInterval | null): string {
  if (!interval) return '-'
  return interval.tier_label || formatIntervalRange(interval.min_tokens, interval.max_tokens)
}

export function formatAvailableChannelIntervals(
  pricing: UserSupportedModelPricing | null,
  labels: AvailableChannelPricingLabels,
  options: { compact?: boolean } = {},
): string {
  const intervals = getValuedIntervals(pricing)
  if (intervals.length === 0) return '-'

  return intervals
    .map((interval) => {
      const range = formatAvailableChannelIntervalLabel(interval)
      if (
        pricing?.billing_mode === BILLING_MODE_PER_REQUEST ||
        pricing?.billing_mode === BILLING_MODE_IMAGE
      ) {
        const value = options.compact
          ? formatCompactRequestPrice(interval.per_request_price)
          : formatRequestPrice(interval.per_request_price, labels)
        return `${range}: ${value}`
      }

      const input = interval.input_price == null ? '-' : formatScaled(interval.input_price, PER_MILLION_SCALE)
      const output = interval.output_price == null ? '-' : formatScaled(interval.output_price, PER_MILLION_SCALE)
      const unit = options.compact ? '' : ` ${labels.unitPerMillion}`
      return `${range}: ${input} / ${output}${unit}`
    })
    .join('; ')
}

export function rowHasPricing(row: Pick<AvailableChannelCatalogRow, 'pricing' | 'interval'>): boolean {
  if (row.interval) return isPricingIntervalValued(row.interval)
  const pricing = row.pricing
  if (!pricing) return false
  if (
    pricing.input_price != null ||
    pricing.output_price != null ||
    pricing.cache_write_price != null ||
    pricing.cache_read_price != null ||
    pricing.image_output_price != null ||
    pricing.per_request_price != null
  ) {
    return true
  }
  return getValuedIntervals(pricing).length > 0
}

export function getRowInputPrice(row: AvailableChannelCatalogRow): number | null {
  return applyRowRateMultiplier(
    row.interval ? row.interval.input_price : row.pricing?.input_price ?? null,
    rowTokenRateMultiplier(row),
  )
}

export function getRowOutputPrice(row: AvailableChannelCatalogRow): number | null {
  return applyRowRateMultiplier(
    row.interval ? row.interval.output_price : row.pricing?.output_price ?? null,
    rowTokenRateMultiplier(row),
  )
}

export function getRowCacheWritePrice(row: AvailableChannelCatalogRow): number | null {
  return applyRowRateMultiplier(
    row.interval ? row.interval.cache_write_price : row.pricing?.cache_write_price ?? null,
    rowTokenRateMultiplier(row),
  )
}

export function getRowCacheReadPrice(row: AvailableChannelCatalogRow): number | null {
  return applyRowRateMultiplier(
    row.interval ? row.interval.cache_read_price : row.pricing?.cache_read_price ?? null,
    rowTokenRateMultiplier(row),
  )
}

export function getRowImageOutputPrice(row: AvailableChannelCatalogRow): number | null {
  const multiplier = row.pricing?.billing_mode === BILLING_MODE_TOKEN
    ? rowTokenRateMultiplier(row)
    : row.effectiveRateMultiplier
  return applyRowRateMultiplier(row.pricing?.image_output_price ?? null, multiplier)
}

function rowTokenRateMultiplier(row: AvailableChannelCatalogRow): number {
  if (hasEnabledTimePricing(row.pricing)) {
    return getActiveTimePricingMultiplier(row.pricing)
  }
  return row.effectiveRateMultiplier
}

export function getRowPerRequestPrice(row: AvailableChannelCatalogRow): number | null {
  return applyRowRateMultiplier(
    row.interval ? row.interval.per_request_price : row.pricing?.per_request_price ?? null,
    row.effectiveRateMultiplier,
  )
}

export async function exportAvailableChannelsCatalog(
  rows: AvailableChannelCatalogRow[],
  labels: AvailableChannelExportLabels,
  userGroupRates: Record<number, number>,
): Promise<void> {
  const [XLSX, fileSaver] = await Promise.all([
    import('xlsx'),
    import('file-saver'),
  ])

  const worksheetRows = rows.map((row) => [
    row.modelName,
    row.platform,
    row.channelName,
    formatChannelStatus(row.channelStatus, labels),
    row.channelDescription,
    formatAvailableChannelGroups(row.groups, userGroupRates),
    formatBillingMode(row.pricing, labels),
    row.intervalLabel,
    formatCompactTokenPrice(getRowInputPrice(row)),
    formatCompactTokenPrice(getRowOutputPrice(row)),
    formatCompactTokenPrice(getRowCacheWritePrice(row)),
    formatCompactTokenPrice(getRowCacheReadPrice(row)),
    formatCompactRequestPrice(getRowImageOutputPrice(row)),
    formatCompactRequestPrice(getRowPerRequestPrice(row)),
  ])

  const worksheet = XLSX.utils.aoa_to_sheet([
    [
      labels.model,
      labels.platform,
      labels.channel,
      labels.status,
      labels.description,
      labels.groups,
      labels.billingMode,
      labels.interval,
      labels.inputPrice,
      labels.outputPrice,
      labels.cacheWritePrice,
      labels.cacheReadPrice,
      labels.imageOutputPrice,
      labels.perRequestPrice,
    ],
    ...worksheetRows,
  ])
  worksheet['!cols'] = [
    { wch: 32 },
    { wch: 14 },
    { wch: 22 },
    { wch: 12 },
    { wch: 26 },
    { wch: 32 },
    { wch: 14 },
    { wch: 18 },
    { wch: 14 },
    { wch: 14 },
    { wch: 14 },
    { wch: 14 },
    { wch: 14 },
    { wch: 14 },
  ]

  const workbook = XLSX.utils.book_new()
  XLSX.utils.book_append_sheet(workbook, worksheet, labels.sheetName.slice(0, 31))
  const data = XLSX.write(workbook, { bookType: 'xlsx', type: 'array' })
  fileSaver.saveAs(new Blob([data], { type: EXCEL_MIME }), buildExportFilename())
}

function filterCatalogGroups(
  groups: UserAvailableGroup[] | null | undefined,
  includeSubscriptionGroups: boolean,
  groupScope: AvailableChannelGroupScope,
): UserAvailableGroup[] {
  const safeGroups = arrayOrEmpty(groups)
  const nonSubscriptionGroups = safeGroups.filter((group) => group.subscription_type !== 'subscription')
  const baseGroups = includeSubscriptionGroups ? safeGroups : nonSubscriptionGroups

  switch (groupScope) {
    case 'public':
      return nonSubscriptionGroups.filter((group) => !group.is_exclusive)
    case 'public_exclusive':
      return nonSubscriptionGroups
    case 'exclusive':
      return nonSubscriptionGroups.filter((group) => group.is_exclusive)
    default:
      return baseGroups
  }
}

function getValuedIntervals(pricing: UserSupportedModelPricing | null): UserPricingInterval[] {
  return pricing?.intervals?.filter(isPricingIntervalValued) ?? []
}

function matchesChannelStatus(
  status: string | undefined,
  statusScope: AvailableChannelStatusScope,
  activeOnly?: boolean,
): boolean {
  if (activeOnly && status && status !== ACTIVE_STATUS) return false
  switch (statusScope) {
    case 'active':
      return !status || status === ACTIVE_STATUS
    case 'disabled':
      return status === DISABLED_STATUS
    default:
      return true
  }
}

function isPricingIntervalValued(interval: UserPricingInterval): boolean {
  return (
    interval.input_price != null ||
    interval.output_price != null ||
    interval.cache_write_price != null ||
    interval.cache_read_price != null ||
    interval.per_request_price != null
  )
}

function getChannelStatus(channel: UserAvailableChannel): string | undefined {
  return (channel as UserAvailableChannel & { status?: string }).status
}

function getSortValue(row: AvailableChannelCatalogRow, sortBy: AvailableChannelSortKey): string | number | null {
  switch (sortBy) {
    case 'platform':
      return row.platform
    case 'channel':
      return row.channelName
    case 'billingMode':
      return row.pricing?.billing_mode ?? ''
    case 'interval':
      return row.intervalLabel
    case 'inputPrice':
      return getRowInputPrice(row)
    case 'outputPrice':
      return getRowOutputPrice(row)
    case 'cacheWritePrice':
      return getRowCacheWritePrice(row)
    case 'cacheReadPrice':
      return getRowCacheReadPrice(row)
    case 'imageOutputPrice':
      return getRowImageOutputPrice(row)
    case 'perRequestPrice':
      return getRowPerRequestPrice(row)
    default:
      return row.modelName
  }
}

function compareSortValue(a: string | number | null, b: string | number | null, direction: number): number {
  if (a == null && b == null) return 0
  if (a == null) return 1
  if (b == null) return -1
  if (typeof a === 'number' && typeof b === 'number') {
    if (a === b) return 0
    return a > b ? direction : -direction
  }
  const result = String(a).localeCompare(String(b), undefined, { numeric: true, sensitivity: 'base' })
  return result * direction
}

function compareDefaultOrder(a: AvailableChannelCatalogRow, b: AvailableChannelCatalogRow): number {
  const modelOrder = a.modelName.localeCompare(b.modelName, undefined, { numeric: true, sensitivity: 'base' })
  if (modelOrder !== 0) return modelOrder
  const platformOrder = a.platform.localeCompare(b.platform, undefined, { numeric: true, sensitivity: 'base' })
  if (platformOrder !== 0) return platformOrder
  const channelOrder = a.channelName.localeCompare(b.channelName, undefined, { numeric: true, sensitivity: 'base' })
  if (channelOrder !== 0) return channelOrder
  const groupOrder = (a.groups[0]?.name ?? '').localeCompare(b.groups[0]?.name ?? '', undefined, {
    numeric: true,
    sensitivity: 'base',
  })
  if (groupOrder !== 0) return groupOrder
  return a.intervalLabel.localeCompare(b.intervalLabel, undefined, { numeric: true, sensitivity: 'base' })
}

function applyRowRateMultiplier(value: number | null, multiplier: number): number | null {
  return value == null ? null : value * multiplier
}

function buildTimePricingRow(options: {
  id: string
  source: 'implicit' | 'rule'
  startTime: string | null
  endTime: string | null
  windowLabel: string
  label: string
  multiplier: number
  active: boolean
  pricing: UserSupportedModelPricing
}): TimePricingDisplayRow {
  const multiplier = normalizeTimePricingMultiplier(options.multiplier)
  const scale = (value: number | null) => applyRowRateMultiplier(value, multiplier)
  return {
    id: options.id,
    source: options.source,
    startTime: options.startTime,
    endTime: options.endTime,
    windowLabel: options.windowLabel,
    label: options.label,
    multiplier,
    active: options.active,
    inputPrice: scale(options.pricing.input_price),
    outputPrice: scale(options.pricing.output_price),
    cacheWritePrice: scale(options.pricing.cache_write_price),
    cacheReadPrice: scale(options.pricing.cache_read_price),
  }
}

function findActiveTimePricingRule(
  timePricing: UserTimePricing | null | undefined,
  at: Date,
): UserTimePricingRule | null {
  if (!timePricing?.enabled || !isValidTimeZone(timePricing.timezone)) return null
  const minuteOfDay = getZonedMinuteOfDay(at, timePricing.timezone)
  if (minuteOfDay == null) return null
  return arrayOrEmpty(timePricing.rules).find((rule) => {
    if (!isValidRuleShape(rule)) return false
    return isMinuteInTimePricingWindow(
      minuteOfDay,
      parseTimeToMinute(rule.start_time) as number,
      parseTimeToMinute(rule.end_time) as number,
    )
  }) ?? null
}

function isMinuteInTimePricingWindow(minuteOfDay: number, startMinute: number, endMinute: number): boolean {
  if (startMinute === endMinute) return false
  if (startMinute < endMinute) {
    return minuteOfDay >= startMinute && minuteOfDay < endMinute
  }
  return minuteOfDay >= startMinute || minuteOfDay < endMinute
}

function getZonedMinuteOfDay(at: Date, timezone: string): number | null {
  try {
    const parts = new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
      hourCycle: 'h23',
    }).formatToParts(at)
    const hour = Number(parts.find((part) => part.type === 'hour')?.value)
    const minute = Number(parts.find((part) => part.type === 'minute')?.value)
    if (!Number.isInteger(hour) || !Number.isInteger(minute)) return null
    return hour * 60 + minute
  } catch {
    return null
  }
}

function isValidTimeZone(timezone: string | null | undefined): boolean {
  if (!timezone) return false
  try {
    new Intl.DateTimeFormat('en-US', { timeZone: timezone }).format(new Date())
    return true
  } catch {
    return false
  }
}

function isValidRuleShape(rule: UserTimePricingRule | null | undefined): rule is UserTimePricingRule {
  if (!rule) return false
  const start = parseTimeToMinute(rule.start_time)
  const end = parseTimeToMinute(rule.end_time)
  return start != null
    && end != null
    && start !== end
    && Number.isFinite(rule.multiplier)
    && rule.multiplier >= 0
    && rule.multiplier <= 100
}

function parseTimeToMinute(value: string | null | undefined): number | null {
  const match = /^([01]\d|2[0-3]):([0-5]\d)$/.exec(value ?? '')
  if (!match) return null
  return Number(match[1]) * 60 + Number(match[2])
}

function formatTimePricingWindow(startTime: string, endTime: string): string {
  return `${startTime}-${endTime}`
}

function normalizeTimePricingMultiplier(multiplier: number): number {
  if (!Number.isFinite(multiplier) || multiplier < 0 || multiplier > 100) return 1
  return multiplier
}

function resolveDefaultTimePricingMultiplier(
  timePricing: UserTimePricing | null | undefined,
): number {
  return normalizeTimePricingMultiplier(timePricing?.default_multiplier ?? 1)
}

function normalizeTimePricingLabel(label: string | null | undefined, fallback: string): string {
  const normalized = label?.trim()
  return normalized || fallback
}

function formatIntervalRange(min: number, max: number | null): string {
  const maxLabel = max == null ? '∞' : String(max)
  return `(${min}, ${maxLabel}]`
}

function buildExportFilename(): string {
  const stamp = new Date()
    .toISOString()
    .slice(0, 19)
    .replace(/[-:]/g, '')
    .replace('T', '-')
  return `available-channels-${stamp}.xlsx`
}
