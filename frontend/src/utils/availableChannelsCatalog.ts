import type {
  UserAvailableChannel,
  UserAvailableGroup,
  UserPricingInterval,
  UserSupportedModelPricing,
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
        if (options.billingMode && model.pricing?.billing_mode !== options.billingMode) return

        const modelGroups = resolveAvailableModelGroupContexts(model, groups)
          .map(({ group }) => group)
        const callabilityMetadataPresent = Array.isArray(model.route_group_ids)
          || Array.isArray(model.supported_endpoints)
        if (modelGroups.length === 0 && (sectionGroups.length > 0 || callabilityMetadataPresent)) return

        const groupRows = modelGroups.length > 0 ? modelGroups.map((group) => [group]) : [[]]

        groupRows.forEach((rowGroups, groupIndex) => {
          const group = rowGroups[0] ?? null
          const pricing = resolveAvailableGroupDisplayPricing(model.pricing, group)
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

export function formatRateMultiplier(rate: number | null | undefined): string {
  if (rate == null) return '1x'
  return `${Number(rate.toFixed(4)).toString()}x`
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
    row.effectiveRateMultiplier,
  )
}

export function getRowOutputPrice(row: AvailableChannelCatalogRow): number | null {
  return applyRowRateMultiplier(
    row.interval ? row.interval.output_price : row.pricing?.output_price ?? null,
    row.effectiveRateMultiplier,
  )
}

export function getRowCacheWritePrice(row: AvailableChannelCatalogRow): number | null {
  return applyRowRateMultiplier(
    row.interval ? row.interval.cache_write_price : row.pricing?.cache_write_price ?? null,
    row.effectiveRateMultiplier,
  )
}

export function getRowCacheReadPrice(row: AvailableChannelCatalogRow): number | null {
  return applyRowRateMultiplier(
    row.interval ? row.interval.cache_read_price : row.pricing?.cache_read_price ?? null,
    row.effectiveRateMultiplier,
  )
}

export function getRowImageOutputPrice(row: AvailableChannelCatalogRow): number | null {
  return applyRowRateMultiplier(row.pricing?.image_output_price ?? null, row.effectiveRateMultiplier)
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
