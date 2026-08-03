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
 * Resolve the groups in which one catalog model can actually be called.
 *
 * New responses use route_group_ids as the authoritative contract. Older
 * responses fall back to supported_endpoints, while responses that omit both
 * fields retain the rollback-compatible catalog behavior.
 */
export function resolveAvailableModelGroupContexts(
  model: UserSupportedModel,
  groups: UserAvailableGroup[],
): AvailableModelGroupContext[] {
  const routeMetadataPresent = Array.isArray(model.route_group_ids)
  const endpointMetadataPresent = Array.isArray(model.supported_endpoints)

  return groups.flatMap((group) => {
    const endpoints = (model.supported_endpoints ?? [])
      .filter((endpoint) => endpointAppliesToGroup(endpoint, group.id))
      .map((endpoint) => ({ ...endpoint, group_ids: [group.id] }))

    if (routeMetadataPresent && !model.route_group_ids?.includes(group.id)) return []
    if (!routeMetadataPresent && endpointMetadataPresent && endpoints.length === 0) return []

    return [{ group, endpoints }]
  })
}

function endpointAppliesToGroup(endpoint: UserSupportedEndpoint, groupID: number): boolean {
  return endpoint.group_ids.length === 0 || endpoint.group_ids.includes(groupID)
}
