import type { Channel } from '@/api/admin/channels'

export interface ChannelModelRecommendation {
  model: string
  title: string
  source: 'mapping' | 'pricing'
}

const normalizeModel = (model: string) => model.trim()

const groupMatches = (channel: Channel, groupIds: number[]) => {
  if (groupIds.length === 0) return false
  return channel.group_ids.some(groupId => groupIds.includes(groupId))
}

const addRecommendation = (
  recommendations: ChannelModelRecommendation[],
  seen: Set<string>,
  model: string,
  title: string,
  source: ChannelModelRecommendation['source']
) => {
  const normalized = normalizeModel(model)
  if (!normalized || seen.has(normalized)) return
  seen.add(normalized)
  recommendations.push({ model: normalized, title, source })
}

export function buildChannelModelRecommendations(
  channels: Channel[],
  groupIds: number[],
  platform: string
): ChannelModelRecommendation[] {
  const recommendations: ChannelModelRecommendation[] = []
  const seen = new Set<string>()
  const relevantChannels = channels.filter(channel => groupMatches(channel, groupIds))

  for (const channel of relevantChannels) {
    const platformMapping = channel.model_mapping?.[platform] || {}
    for (const [sourceModel, targetModel] of Object.entries(platformMapping)) {
      addRecommendation(
        recommendations,
        seen,
        targetModel,
        `${channel.name}: ${sourceModel} -> ${targetModel}`,
        'mapping'
      )
    }
  }

  if (recommendations.length > 0) {
    return recommendations
  }

  for (const channel of relevantChannels) {
    for (const pricing of channel.model_pricing || []) {
      if (pricing.platform && pricing.platform !== platform) continue
      for (const model of pricing.models || []) {
        addRecommendation(recommendations, seen, model, `${channel.name}: ${model}`, 'pricing')
      }
    }
  }

  return recommendations
}
