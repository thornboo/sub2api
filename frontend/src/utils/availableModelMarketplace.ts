import type {
  UserAvailableChannel,
  UserAvailableGroup,
  UserSupportedEndpoint,
  UserSupportedModelPricing,
} from '@/api/channels'
import type { BillingMode } from '@/constants/channel'
import type {
  AvailableChannelGroupScope,
  AvailableChannelPriceStatus,
} from '@/utils/availableChannelsCatalog'

export interface AvailableModelRoute {
  id: string
  channelName: string
  channelDescription: string
  platform: string
  group: UserAvailableGroup
  pricing: UserSupportedModelPricing | null
  endpoints: UserSupportedEndpoint[]
}

export interface AvailableModelMarketplaceCard {
  id: string
  name: string
  group: UserAvailableGroup
  platforms: string[]
  channelNames: string[]
  endpoints: UserSupportedEndpoint[]
  pricingOptions: Array<UserSupportedModelPricing | null>
  routes: AvailableModelRoute[]
}

export interface AvailableModelMarketplaceOptions {
  billingMode?: BillingMode | ''
  groupScope?: AvailableChannelGroupScope
  priceStatus?: AvailableChannelPriceStatus
}

interface MutableModelCard {
  name: string
  group: UserAvailableGroup
  platforms: Map<string, string>
  channelNames: Map<string, string>
  endpoints: Map<string, UserSupportedEndpoint>
  pricingOptions: Map<string, UserSupportedModelPricing | null>
  routes: AvailableModelRoute[]
}

export function buildAvailableModelMarketplaceCards(
  channels: UserAvailableChannel[],
  options: AvailableModelMarketplaceOptions = {},
): AvailableModelMarketplaceCard[] {
  const cards = new Map<string, MutableModelCard>()
  const groupScope = options.groupScope ?? 'all'
  const priceStatus = options.priceStatus ?? 'all'

  channels.forEach((channel, channelIndex) => {
    channel.platforms.forEach((section, sectionIndex) => {
      const groups = filterGroups(section.groups, groupScope)
      if (section.groups.length > 0 && groups.length === 0) return

      section.supported_models.forEach((model, modelIndex) => {
        if (options.billingMode && model.pricing?.billing_mode !== options.billingMode) return

        const hasPricing = modelHasPricing(model.pricing)
        if (priceStatus === 'priced' && !hasPricing) return
        if (priceStatus === 'unpriced' && hasPricing) return

        groups.forEach((group) => {
          const routeMetadataPresent = Array.isArray(model.route_group_ids)
          const endpointMetadataPresent = Array.isArray(model.supported_endpoints)
          const endpoints = (model.supported_endpoints ?? [])
            .filter(endpoint => endpointAppliesToGroup(endpoint, group.id))
            .map(endpoint => ({ ...endpoint, group_ids: [group.id] }))

          // route_group_ids is the authoritative per-group callability contract.
          // Older responses fall back to endpoint-based filtering, while an
          // omitted endpoint list still means the rollback-compatible catalog.
          if (routeMetadataPresent && !model.route_group_ids?.includes(group.id)) return
          if (!routeMetadataPresent && endpointMetadataPresent && endpoints.length === 0) return

          const id = `${group.id}::${model.name}`
          const card = cards.get(id) ?? createMutableCard(model.name, group)
          card.platforms.set(section.platform, section.platform)
          card.channelNames.set(channel.name, channel.name)
          endpoints.forEach(endpoint => {
            card.endpoints.set(endpointKey(endpoint), endpoint)
          })
          card.pricingOptions.set(pricingKey(model.pricing), model.pricing)
          card.routes.push({
            id: [group.id, channelIndex, sectionIndex, modelIndex, channel.name, section.platform, model.name].join('::'),
            channelName: channel.name,
            channelDescription: channel.description,
            platform: section.platform,
            group,
            pricing: model.pricing,
            endpoints,
          })
          cards.set(id, card)
        })
      })
    })
  })

  return Array.from(cards.entries())
    .map(([id, card]) => ({
      id,
      name: card.name,
      group: card.group,
      platforms: Array.from(card.platforms.values()).sort(localeCompare),
      channelNames: Array.from(card.channelNames.values()).sort(localeCompare),
      endpoints: Array.from(card.endpoints.values()).sort(compareEndpoints),
      pricingOptions: Array.from(card.pricingOptions.values()),
      routes: [...card.routes].sort(compareRoutes),
    }))
    .sort((a, b) => compareGroups(a.group, b.group) || localeCompare(a.name, b.name))
}

export function modelHasPricing(pricing: UserSupportedModelPricing | null): boolean {
  if (!pricing) return false
  if (
    pricing.input_price != null ||
    pricing.output_price != null ||
    pricing.cache_write_price != null ||
    pricing.cache_read_price != null ||
    pricing.image_input_price != null ||
    pricing.image_output_price != null ||
    pricing.per_request_price != null
  ) {
    return true
  }
  return pricing.intervals.some(interval => (
    interval.input_price != null ||
    interval.output_price != null ||
    interval.cache_write_price != null ||
    interval.cache_read_price != null ||
    interval.per_request_price != null
  ))
}

function createMutableCard(name: string, group: UserAvailableGroup): MutableModelCard {
  return {
    name,
    group,
    platforms: new Map(),
    channelNames: new Map(),
    endpoints: new Map(),
    pricingOptions: new Map(),
    routes: [],
  }
}

function filterGroups(
  groups: UserAvailableGroup[],
  groupScope: AvailableChannelGroupScope,
): UserAvailableGroup[] {
  const nonSubscriptionGroups = groups.filter(group => group.subscription_type !== 'subscription')
  switch (groupScope) {
    case 'public':
      return nonSubscriptionGroups.filter(group => !group.is_exclusive)
    case 'public_exclusive':
      return nonSubscriptionGroups
    case 'exclusive':
      return nonSubscriptionGroups.filter(group => group.is_exclusive)
    default:
      return groups
  }
}

function pricingKey(pricing: UserSupportedModelPricing | null): string {
  return pricing == null ? 'unpriced' : JSON.stringify(pricing)
}

function endpointKey(endpoint: UserSupportedEndpoint): string {
  return `${endpoint.protocol}:${endpoint.path}`
}

function endpointAppliesToGroup(endpoint: UserSupportedEndpoint, groupID: number): boolean {
  return endpoint.group_ids.length === 0 || endpoint.group_ids.includes(groupID)
}

function localeCompare(a: string, b: string): number {
  return a.localeCompare(b, undefined, { numeric: true, sensitivity: 'base' })
}

function compareGroups(a: UserAvailableGroup, b: UserAvailableGroup): number {
  if (a.is_exclusive !== b.is_exclusive) return a.is_exclusive ? -1 : 1
  return localeCompare(a.name, b.name)
}

function compareEndpoints(a: UserSupportedEndpoint, b: UserSupportedEndpoint): number {
  const protocolOrder = localeCompare(a.protocol, b.protocol)
  return protocolOrder !== 0 ? protocolOrder : localeCompare(a.path, b.path)
}

function compareRoutes(a: AvailableModelRoute, b: AvailableModelRoute): number {
  const channelOrder = localeCompare(a.channelName, b.channelName)
  return channelOrder !== 0 ? channelOrder : localeCompare(a.platform, b.platform)
}
