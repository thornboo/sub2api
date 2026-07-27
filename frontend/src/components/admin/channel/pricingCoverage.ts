import type { BillingModelSource } from '@/constants/channel'
import type { PricingFormEntry } from './types'

export interface PricingCoverage {
  expectedModels: string[]
  coveredModels: string[]
  missingModels: string[]
  extraPricingModels: string[]
  indeterminate: boolean
}

export type PricingCoverageSeverity = 'none' | 'warning' | 'error' | 'unknown'

interface ModelPattern {
  value: string
  prefix: string
  wildcard: boolean
}

function toPattern(value: string): ModelPattern {
  const normalized = value.trim().toLocaleLowerCase()
  return {
    value: value.trim(),
    prefix: normalized.endsWith('*') ? normalized.slice(0, -1) : normalized,
    wildcard: normalized.endsWith('*'),
  }
}

export function naturalModelCompare(left: string, right: string): number {
  return left.localeCompare(right, undefined, {
    numeric: true,
    sensitivity: 'base',
  })
}

export function normalizeMappingOrder(
  mapping: Record<string, string>,
  configuredOrder: string[] | undefined,
): string[] {
  const existing = new Set(Object.keys(mapping))
  const seen = new Set<string>()
  const order: string[] = []

  for (const model of configuredOrder || []) {
    if (!existing.has(model) || seen.has(model)) continue
    seen.add(model)
    order.push(model)
  }

  const missing = Object.keys(mapping)
    .filter(model => !seen.has(model))
    .sort(naturalModelCompare)

  return [...order, ...missing]
}

export function pricingModelForMapping(
  source: string,
  target: string,
  billingModelSource: BillingModelSource | string,
): string | null {
  if (billingModelSource === 'requested') return source.trim() || null
  if (billingModelSource === 'channel_mapped') return target.trim() || null
  return null
}

export function patternCovers(coveringValue: string, expectedValue: string): boolean {
  const covering = toPattern(coveringValue)
  const expected = toPattern(expectedValue)
  if (!covering.value || !expected.value) return false

  if (!covering.wildcard) {
    return !expected.wildcard && covering.prefix === expected.prefix
  }
  return expected.prefix.startsWith(covering.prefix)
}

function patternsOverlap(leftValue: string, rightValue: string): boolean {
  const left = toPattern(leftValue)
  const right = toPattern(rightValue)
  if (!left.value || !right.value) return false

  if (!left.wildcard && !right.wildcard) return left.prefix === right.prefix
  if (left.wildcard && !right.wildcard) return right.prefix.startsWith(left.prefix)
  if (!left.wildcard && right.wildcard) return left.prefix.startsWith(right.prefix)
  return left.prefix.startsWith(right.prefix) || right.prefix.startsWith(left.prefix)
}

function uniqueModels(models: string[]): string[] {
  const seen = new Set<string>()
  const result: string[] = []
  for (const rawModel of models) {
    const model = rawModel.trim()
    if (!model) continue
    const key = model.toLocaleLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    result.push(model)
  }
  return result
}

export function derivePricingCoverage(
  mapping: Record<string, string>,
  mappingOrder: string[],
  pricing: PricingFormEntry[],
  billingModelSource: BillingModelSource | string,
): PricingCoverage {
  const pricingModels = uniqueModels(pricing.flatMap(entry => entry.models || []))
  if (billingModelSource === 'upstream') {
    return {
      expectedModels: [],
      coveredModels: [],
      missingModels: [],
      extraPricingModels: pricingModels,
      indeterminate: Object.keys(mapping).length > 0,
    }
  }

  const expectedModels = uniqueModels(
    normalizeMappingOrder(mapping, mappingOrder)
      .map(source => pricingModelForMapping(source, mapping[source] || '', billingModelSource))
      .filter((model): model is string => model !== null),
  )

  const coveredModels = expectedModels.filter(expected =>
    pricingModels.some(pricingModel => patternCovers(pricingModel, expected)),
  )
  const missingModels = expectedModels.filter(expected =>
    !pricingModels.some(pricingModel => patternCovers(pricingModel, expected)),
  )
  const extraPricingModels = pricingModels.filter(pricingModel =>
    !expectedModels.some(expected => patternsOverlap(pricingModel, expected)),
  )

  return {
    expectedModels,
    coveredModels,
    missingModels,
    extraPricingModels,
    indeterminate: false,
  }
}

export function pricingCoverageSeverity(
  coverage: PricingCoverage,
  restrictModels: boolean,
): PricingCoverageSeverity {
  if (coverage.indeterminate) return 'unknown'
  if (coverage.missingModels.length === 0) return 'none'
  return restrictModels ? 'error' : 'warning'
}
