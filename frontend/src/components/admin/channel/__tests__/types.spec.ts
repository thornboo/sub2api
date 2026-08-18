import { describe, expect, it } from 'vitest'
import {
  apiTimePricingToForm,
  formTimePricingToAPI,
  validateIntervals,
  validateTimePricing,
  type IntervalFormEntry,
  type PricingFormEntry,
  type TimePricingFormEntry,
} from '../types'

function makeInterval(over: Partial<IntervalFormEntry>): IntervalFormEntry {
  return {
    min_tokens: 0,
    max_tokens: null,
    tier_label: '',
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    per_request_price: null,
    sort_order: 0,
    ...over,
  }
}

function t(key: string, params?: Record<string, unknown>): string {
  return `${key}${params ? ` ${JSON.stringify(params)}` : ''}`
}

describe('validateIntervals', () => {
  describe('token mode', () => {
    it('rejects unbounded interval that is not last', () => {
      const intervals: IntervalFormEntry[] = [
        makeInterval({ min_tokens: 0, max_tokens: null, input_price: 1, output_price: 1 }),
        makeInterval({ min_tokens: 200000, max_tokens: 500000, input_price: 2, output_price: 2 }),
      ]
      expect(validateIntervals(intervals, 'token', t)).toContain('unboundedLast')
    })

    it('accepts unbounded interval at the end', () => {
      const intervals: IntervalFormEntry[] = [
        makeInterval({ min_tokens: 0, max_tokens: 200000, input_price: 1, output_price: 1 }),
        makeInterval({ min_tokens: 200000, max_tokens: null, input_price: 2, output_price: 2 }),
      ]
      expect(validateIntervals(intervals, 'token', t)).toBeNull()
    })

    it('rejects overlapping intervals', () => {
      const intervals: IntervalFormEntry[] = [
        makeInterval({ min_tokens: 0, max_tokens: 250000, input_price: 1, output_price: 1 }),
        makeInterval({ min_tokens: 200000, max_tokens: 500000, input_price: 2, output_price: 2 }),
      ]
      expect(validateIntervals(intervals, 'token', t)).toContain('overlap')
    })

    it('rejects unbounded interval in token mode', () => {
      const intervals: IntervalFormEntry[] = [
        makeInterval({ min_tokens: 0, max_tokens: null, input_price: 1, output_price: 1 }),
        makeInterval({ min_tokens: 100, max_tokens: 200, input_price: 2, output_price: 2 }),
      ]
      expect(validateIntervals(intervals, 'token', t)).toContain('unboundedLast')
    })
  })

  describe('image / per_request mode', () => {
    it('allows multiple unbounded tiers identified by label', () => {
      const intervals: IntervalFormEntry[] = [
        makeInterval({ tier_label: '1K', per_request_price: 0.04 }),
        makeInterval({ tier_label: '2K', per_request_price: 0.06 }),
        makeInterval({ tier_label: '4K', per_request_price: 0.08 }),
      ]
      expect(validateIntervals(intervals, 'image', t)).toBeNull()
      expect(validateIntervals(intervals, 'per_request', t)).toBeNull()
    })

    it('still rejects negative prices', () => {
      const intervals: IntervalFormEntry[] = [
        makeInterval({ tier_label: '1K', per_request_price: -1 }),
      ]
      expect(validateIntervals(intervals, 'image', t)).toContain('negativePrice')
    })

    it('still rejects max <= min on a single tier', () => {
      const intervals: IntervalFormEntry[] = [
        makeInterval({ tier_label: '1K', min_tokens: 100, max_tokens: 50, per_request_price: 0.04 }),
      ]
      expect(validateIntervals(intervals, 'image', t)).toContain('maxGreaterThanMin')
    })
  })
})

function makePricingEntry(timePricing?: TimePricingFormEntry): PricingFormEntry {
  return {
    models: ['deepseek-chat'],
    billing_mode: 'token',
    input_price: 3.6,
    output_price: 10.8,
    cache_write_price: null,
    cache_read_price: 0.12,
    image_input_price: null,
    image_output_price: null,
    per_request_price: null,
    intervals: [],
    time_pricing: timePricing,
    self_check_enabled_models: [],
  }
}

describe('time pricing helpers', () => {
  it('accepts a configurable all-day default without explicit windows', () => {
    expect(validateTimePricing(makePricingEntry({
      enabled: true,
      timezone: 'Asia/Shanghai',
      default_label: '平时',
      default_multiplier: 0.8,
      rules: [],
    }), t)).toBeNull()
  })

  it('round-trips disabled schedules without dropping rules', () => {
    const apiSchedule = {
      enabled: false,
      timezone: 'Asia/Shanghai',
      default_label: '平时',
      rules: [
        { label: 'legacy peak', start_time: '09:00', end_time: '12:00', multiplier: 2 },
      ],
    }

    const formSchedule = apiTimePricingToForm(apiSchedule)
    expect(formSchedule).toEqual({
      enabled: false,
      timezone: 'Asia/Shanghai',
      default_label: '平时',
      default_multiplier: 1,
      rules: [
        { label: 'legacy peak', start_time: '09:00', end_time: '12:00', multiplier: 2 },
      ],
    })
    expect(formTimePricingToAPI(formSchedule)).toEqual({
      ...apiSchedule,
      default_multiplier: 1,
    })
  })

  it('accepts multiple non-overlapping normal and cross-midnight windows', () => {
    const entry = makePricingEntry({
      enabled: true,
      timezone: 'Asia/Shanghai',
      default_label: '平时',
      default_multiplier: 0.75,
      rules: [
        { label: 'morning peak', start_time: '09:00', end_time: '12:00', multiplier: 2 },
        { label: 'night valley', start_time: '23:00', end_time: '07:00', multiplier: 0.5 },
      ],
    })
    expect(validateTimePricing(entry, t)).toBeNull()
  })

  it('rejects overlapping cross-midnight and early-morning windows', () => {
    const entry = makePricingEntry({
      enabled: true,
      timezone: 'Asia/Shanghai',
      default_label: '平时',
      default_multiplier: 1,
      rules: [
        { label: 'night valley', start_time: '23:00', end_time: '07:00', multiplier: 0.5 },
        { label: 'early promo', start_time: '06:30', end_time: '08:00', multiplier: 0 },
      ],
    })
    expect(validateTimePricing(entry, t)).toContain('overlap')
  })

  it('rejects equal ranges, bad timezone, and out-of-range multipliers', () => {
    expect(validateTimePricing(makePricingEntry({
      enabled: true,
      timezone: 'Asia/Shanghai',
      default_label: '平时',
      default_multiplier: 1,
      rules: [{ label: '高峰', start_time: '09:00', end_time: '09:00', multiplier: 2 }],
    }), t)).toContain('equalRange')

    expect(validateTimePricing(makePricingEntry({
      enabled: true,
      timezone: 'Mars/Base',
      default_label: '平时',
      default_multiplier: 1,
      rules: [{ label: '高峰', start_time: '09:00', end_time: '10:00', multiplier: 2 }],
    }), t)).toContain('invalidTimezone')

    expect(validateTimePricing(makePricingEntry({
      enabled: true,
      timezone: 'Asia/Shanghai',
      default_label: '平时',
      default_multiplier: 1,
      rules: [{ label: '高峰', start_time: '09:00', end_time: '10:00', multiplier: 101 }],
    }), t)).toContain('multiplierRange')

    expect(validateTimePricing(makePricingEntry({
      enabled: true,
      timezone: 'Asia/Shanghai',
      default_label: '平时',
      default_multiplier: -0.1,
      rules: [],
    }), t)).toContain('defaultMultiplierRange')
  })

  it('rejects enabled schedules on non-token entries', () => {
    const entry = makePricingEntry({
      enabled: true,
      timezone: 'Asia/Shanghai',
      default_label: '平时',
      default_multiplier: 1,
      rules: [{ label: '高峰', start_time: '09:00', end_time: '10:00', multiplier: 2 }],
    })
    entry.billing_mode = 'per_request'
    expect(validateTimePricing(entry, t)).toContain('tokenOnly')
  })

  it('requires independent customer-facing names for the default and each rule', () => {
    expect(validateTimePricing(makePricingEntry({
      enabled: true,
      timezone: 'Asia/Shanghai',
      default_label: '   ',
      default_multiplier: 1.1,
      rules: [],
    }), t)).toContain('defaultLabelRequired')

    expect(validateTimePricing(makePricingEntry({
      enabled: true,
      timezone: 'Asia/Shanghai',
      default_label: '平时',
      default_multiplier: 1.1,
      rules: [{ label: '', start_time: '09:00', end_time: '12:00', multiplier: 2 }],
    }), t)).toContain('ruleLabelRequired')

    expect(validateTimePricing(makePricingEntry({
      enabled: true,
      timezone: 'Asia/Shanghai',
      default_label: '平'.repeat(33),
      default_multiplier: 1.1,
      rules: [],
    }), t)).toContain('defaultLabelTooLong')
  })
})
