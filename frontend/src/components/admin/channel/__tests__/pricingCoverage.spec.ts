import { describe, expect, it } from 'vitest'
import type { PricingFormEntry } from '../types'
import {
  derivePricingCoverage,
  naturalModelCompare,
  normalizeMappingOrder,
  patternCovers,
  pricingCoverageSeverity,
  pricingModelForMapping,
} from '../pricingCoverage'

function pricing(...models: string[]): PricingFormEntry {
  return {
    models,
    billing_mode: 'token',
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    image_input_price: null,
    image_output_price: null,
    per_request_price: null,
    intervals: [],
    self_check_enabled_models: [],
  }
}

describe('channel pricing coverage', () => {
  it('uses mapping targets for channel-mapped billing', () => {
    const coverage = derivePricingCoverage(
      { aliasA: 'modelA', aliasB: 'modelB' },
      ['aliasA', 'aliasB'],
      [pricing('modelA')],
      'channel_mapped',
    )

    expect(coverage.expectedModels).toEqual(['modelA', 'modelB'])
    expect(coverage.coveredModels).toEqual(['modelA'])
    expect(coverage.missingModels).toEqual(['modelB'])
  })

  it('uses mapping sources for requested-model billing', () => {
    const coverage = derivePricingCoverage(
      { aliasA: 'modelA', aliasB: 'modelB' },
      ['aliasB', 'aliasA'],
      [pricing('aliasB')],
      'requested',
    )

    expect(coverage.expectedModels).toEqual(['aliasB', 'aliasA'])
    expect(coverage.missingModels).toEqual(['aliasA'])
  })

  it('applies case-insensitive wildcard coverage without treating a partial exact price as complete', () => {
    expect(patternCovers('GPT-*', 'gpt-5.4')).toBe(true)
    expect(patternCovers('gpt-5.4', 'GPT-*')).toBe(false)

    const covered = derivePricingCoverage(
      { 'gpt-5.4': 'gpt-5.4' },
      ['gpt-5.4'],
      [pricing('GPT-*')],
      'channel_mapped',
    )
    expect(covered.missingModels).toEqual([])

    const partial = derivePricingCoverage(
      { 'gpt-*': 'gpt-*' },
      ['gpt-*'],
      [pricing('gpt-5.4')],
      'channel_mapped',
    )
    expect(partial.missingModels).toEqual(['gpt-*'])
  })

  it('does not guess final upstream coverage', () => {
    const coverage = derivePricingCoverage(
      { alias: 'channel-model' },
      ['alias'],
      [pricing('upstream-model')],
      'upstream',
    )

    expect(coverage.indeterminate).toBe(true)
    expect(coverage.missingModels).toEqual([])
    expect(pricingCoverageSeverity(coverage, true)).toBe('unknown')
  })

  it('blocks missing coverage only when model restriction is enabled', () => {
    const coverage = derivePricingCoverage(
      { missing: 'missing' },
      ['missing'],
      [],
      'channel_mapped',
    )

    expect(pricingCoverageSeverity(coverage, true)).toBe('error')
    expect(pricingCoverageSeverity(coverage, false)).toBe('warning')
  })

  it('preserves configured mapping order and appends new keys naturally', () => {
    const mapping = {
      'gpt-5.10': 'gpt-5.10',
      'gpt-5.2': 'gpt-5.2',
      claude: 'claude',
    }

    expect(normalizeMappingOrder(mapping, ['claude', 'removed', 'claude']))
      .toEqual(['claude', 'gpt-5.2', 'gpt-5.10'])
    expect(['gpt-5.10', 'gpt-5.2'].sort(naturalModelCompare))
      .toEqual(['gpt-5.2', 'gpt-5.10'])
  })

  it('derives the correct model for each supported billing source', () => {
    expect(pricingModelForMapping('public', 'mapped', 'requested')).toBe('public')
    expect(pricingModelForMapping('public', 'mapped', 'channel_mapped')).toBe('mapped')
    expect(pricingModelForMapping('public', 'mapped', 'upstream')).toBeNull()
  })
})
