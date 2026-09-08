import type {
  UserAvailableGroup,
  UserSupportedEndpoint,
  UserSupportedModel,
} from '@/api/channels'

export interface AvailableModelGroupContext {
  group: UserAvailableGroup
  endpoints: UserSupportedEndpoint[]
}

/**
 * Resolve the groups in which one catalog model should be displayed.
 *
 * New /channels/available responses use catalog_group_ids for the customer
 * catalog. Endpoint metadata still describes only the concrete callable
 * protocols for each group. Older responses fall back to route_group_ids, then
 * supported_endpoints, while responses that omit all metadata keep the
 * rollback-compatible catalog behavior.
 */
export function resolveAvailableModelGroupContexts(
  model: UserSupportedModel,
  groups: UserAvailableGroup[],
): AvailableModelGroupContext[] {
  const catalogMetadataPresent = Array.isArray(model.catalog_group_ids)
  const routeMetadataPresent = Array.isArray(model.route_group_ids)
  const endpointMetadataPresent = Array.isArray(model.supported_endpoints)
  const displayGroupIDs = catalogMetadataPresent ? model.catalog_group_ids : model.route_group_ids

  return groups.flatMap((group) => {
    const endpoints = (model.supported_endpoints ?? [])
      .filter((endpoint) => endpointAppliesToGroup(endpoint, group.id))
      .map((endpoint) => ({ ...endpoint, group_ids: [group.id] }))

    if ((catalogMetadataPresent || routeMetadataPresent) && !displayGroupIDs?.includes(group.id)) return []
    if (!catalogMetadataPresent && !routeMetadataPresent && endpointMetadataPresent && endpoints.length === 0) return []

    return [{ group, endpoints }]
  })
}

function endpointAppliesToGroup(endpoint: UserSupportedEndpoint, groupID: number): boolean {
  return endpoint.group_ids.length === 0 || endpoint.group_ids.includes(groupID)
}
