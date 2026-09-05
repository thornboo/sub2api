import type { BillingMode, PricingInterval, TimePricing, TimePricingRule } from '@/api/admin/channels'

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

export interface IntervalFormEntry {
  min_tokens: number
  max_tokens: number | null
  tier_label: string
  input_price: number | string | null
  output_price: number | string | null
  cache_write_price: number | string | null
  cache_write_1h_price?: number | string | null
  cache_read_price: number | string | null
  input_multiplier: number | string | null
  output_multiplier: number | string | null
  cache_write_multiplier: number | string | null
  cache_read_multiplier: number | string | null
  per_request_price: number | string | null
  sort_order: number
}

export interface PricingFormEntry {
  _ui_id?: string
  sort_order?: number
  models: string[]
  billing_mode: BillingMode
  input_price: number | string | null
  output_price: number | string | null
  cache_write_price: number | string | null
  cache_write_1h_price?: number | string | null
  cache_read_price: number | string | null
  fast_multiplier?: number | string | null
  flex_multiplier?: number | string | null
  max_reasoning_effort_multiplier?: number | string | null
  image_input_price: number | string | null
  image_output_price: number | string | null
  per_request_price: number | string | null
  intervals: IntervalFormEntry[]
  time_pricing?: TimePricingFormEntry
  self_check_enabled_models: string[]
}

export interface TimePricingRuleFormEntry {
  label: string
  start_time: string
  end_time: string
  multiplier: number | string | null
}

export interface TimePricingFormEntry {
  enabled: boolean
  timezone: string
  default_label: string
  default_multiplier: number | string | null
  rules: TimePricingRuleFormEntry[]
  weekdays_only?: boolean
}

// 价格转换：后端存 per-token，前端显示 per-MTok ($/1M tokens)
const MTOK = 1_000_000

export function toNullableNumber(val: number | string | null | undefined): number | null {
  if (val === null || val === undefined || val === '') return null
  const num = Number(val)
  return isNaN(num) ? null : num
}

export function isValidPositiveMultiplier(val: number | string | null | undefined): boolean {
  if (val === null || val === undefined || val === '') return true
  const multiplier = Number(val)
  return Number.isFinite(multiplier) && multiplier > 0
}

/** 前端显示值($/MTok) → 后端存储值(per-token) */
export function mTokToPerToken(val: number | string | null | undefined): number | null {
  const num = toNullableNumber(val)
  return num === null ? null : parseFloat((num / MTOK).toPrecision(10))
}

/** 后端存储值(per-token) → 前端显示值($/MTok) */
export function perTokenToMTok(val: number | null | undefined): number | null {
  if (val === null || val === undefined) return null
  // toPrecision(10) 消除 IEEE 754 浮点乘法精度误差，如 5e-8 * 1e6 = 0.04999...96 → 0.05
  return parseFloat((val * MTOK).toPrecision(10))
}

export function apiIntervalsToForm(intervals: PricingInterval[]): IntervalFormEntry[] {
  return (intervals || []).map(iv => ({
    min_tokens: iv.min_tokens,
    max_tokens: iv.max_tokens,
    tier_label: iv.tier_label || '',
    input_price: perTokenToMTok(iv.input_price),
    output_price: perTokenToMTok(iv.output_price),
    cache_write_price: perTokenToMTok(iv.cache_write_price),
    cache_write_1h_price: perTokenToMTok(iv.cache_write_1h_price),
    cache_read_price: perTokenToMTok(iv.cache_read_price),
    input_multiplier: iv.input_multiplier,
    output_multiplier: iv.output_multiplier,
    cache_write_multiplier: iv.cache_write_multiplier,
    cache_read_multiplier: iv.cache_read_multiplier,
    per_request_price: iv.per_request_price,
    sort_order: iv.sort_order
  }))
}

export function formIntervalsToAPI(intervals: IntervalFormEntry[]): PricingInterval[] {
  return (intervals || []).map(iv => ({
    min_tokens: iv.min_tokens,
    max_tokens: iv.max_tokens,
    tier_label: iv.tier_label,
    input_price: mTokToPerToken(iv.input_price),
    output_price: mTokToPerToken(iv.output_price),
    cache_write_price: mTokToPerToken(iv.cache_write_price),
    cache_write_1h_price: mTokToPerToken(iv.cache_write_1h_price),
    cache_read_price: mTokToPerToken(iv.cache_read_price),
    input_multiplier: toNullableNumber(iv.input_multiplier),
    output_multiplier: toNullableNumber(iv.output_multiplier),
    cache_write_multiplier: toNullableNumber(iv.cache_write_multiplier),
    cache_read_multiplier: toNullableNumber(iv.cache_read_multiplier),
    per_request_price: toNullableNumber(iv.per_request_price),
    sort_order: iv.sort_order
  }))
}

// ── 分时定价转换与校验 ──────────────────────────────────────

const DEFAULT_TIME_PRICING_TIMEZONE = 'Asia/Shanghai'
const TIME_PATTERN = /^([01]\d|2[0-3]):([0-5]\d)$/
const MAX_TIME_PRICING_RULES = 16
const MAX_TIME_PRICING_LABEL_LENGTH = 32

export function createDefaultTimePricing(): TimePricingFormEntry {
  return {
    enabled: false,
    timezone: DEFAULT_TIME_PRICING_TIMEZONE,
    default_label: '',
    default_multiplier: 1,
    rules: [],
    weekdays_only: false,
  }
}

export function apiTimePricingToForm(timePricing?: TimePricing | null): TimePricingFormEntry | undefined {
  if (!timePricing) return undefined
  return {
    enabled: !!timePricing.enabled,
    timezone: timePricing.timezone || DEFAULT_TIME_PRICING_TIMEZONE,
    default_label: timePricing.default_label || '',
    default_multiplier: timePricing.default_multiplier ?? 1,
    rules: (timePricing.rules || []).map(rule => ({
      label: rule.label || '',
      start_time: rule.start_time || '',
      end_time: rule.end_time || '',
      multiplier: rule.multiplier,
    })),
    weekdays_only: timePricing.weekdays_only === true,
  }
}

export function formTimePricingToAPI(timePricing?: TimePricingFormEntry): TimePricing | undefined {
  if (!timePricing) return undefined
  return {
    enabled: !!timePricing.enabled,
    timezone: (timePricing.timezone || DEFAULT_TIME_PRICING_TIMEZONE).trim(),
    default_label: timePricing.default_label.trim(),
    default_multiplier: toNullableNumber(timePricing.default_multiplier) ?? 1,
    rules: (timePricing.rules || []).map(rule => ({
      label: rule.label.trim(),
      start_time: rule.start_time,
      end_time: rule.end_time,
      multiplier: toNullableNumber(rule.multiplier) ?? 1,
    })),
    weekdays_only: timePricing.weekdays_only === true,
  }
}

export function hasEnabledTimePricing(entry: Pick<PricingFormEntry, 'time_pricing'>): boolean {
  return !!entry.time_pricing?.enabled
}

export function validateTimePricing(
  entry: Pick<PricingFormEntry, 'billing_mode' | 'time_pricing'>,
  t: TranslateFn,
): string | null {
  const schedule = entry.time_pricing
  if (!schedule?.enabled) return null

  if (entry.billing_mode !== 'token') {
    return timePricingValidationMessage(t, 'tokenOnly', {})
  }

  const timezone = schedule.timezone.trim()
  if (!timezone || !isValidTimeZone(timezone)) {
    return timePricingValidationMessage(t, 'invalidTimezone', { timezone: timezone || '-' })
  }

  const defaultLabel = schedule.default_label.trim()
  if (!defaultLabel) {
    return timePricingValidationMessage(t, 'defaultLabelRequired', {})
  }
  if ([...defaultLabel].length > MAX_TIME_PRICING_LABEL_LENGTH) {
    return timePricingValidationMessage(t, 'defaultLabelTooLong', { max: MAX_TIME_PRICING_LABEL_LENGTH })
  }

  const defaultMultiplier = toNullableNumber(schedule.default_multiplier)
  if (defaultMultiplier === null || defaultMultiplier < 0 || defaultMultiplier > 100) {
    return timePricingValidationMessage(t, 'defaultMultiplierRange', {
      value: schedule.default_multiplier ?? '-',
    })
  }

  if (schedule.rules.length > MAX_TIME_PRICING_RULES) {
    return timePricingValidationMessage(t, 'tooManyRules', { max: MAX_TIME_PRICING_RULES })
  }

  const segments: Array<{ start: number, end: number, label: string, range: string }> = []
  for (let idx = 0; idx < schedule.rules.length; idx++) {
    const rule = schedule.rules[idx]
    const index = idx + 1
    const label = rule.label.trim()
    if (!label) {
      return timePricingValidationMessage(t, 'ruleLabelRequired', { index })
    }
    if ([...label].length > MAX_TIME_PRICING_LABEL_LENGTH) {
      return timePricingValidationMessage(t, 'ruleLabelTooLong', {
        index,
        max: MAX_TIME_PRICING_LABEL_LENGTH,
      })
    }
    const start = parseTimeToMinute(rule.start_time)
    const end = parseTimeToMinute(rule.end_time)
    if (start === null) {
      return timePricingValidationMessage(t, 'invalidStart', { index, value: rule.start_time || '-' })
    }
    if (end === null) {
      return timePricingValidationMessage(t, 'invalidEnd', { index, value: rule.end_time || '-' })
    }
    if (start === end) {
      return timePricingValidationMessage(t, 'equalRange', { index, range: formatTimeRange(rule) })
    }
    const multiplier = toNullableNumber(rule.multiplier)
    if (multiplier === null || multiplier < 0 || multiplier > 100) {
      return timePricingValidationMessage(t, 'multiplierRange', { index, value: rule.multiplier ?? '-' })
    }

    const range = formatTimeRange(rule)
    if (start < end) {
      segments.push({ start, end, label, range })
    } else {
      segments.push({ start, end: 24 * 60, label, range })
      segments.push({ start: 0, end, label, range })
    }
  }

  const sorted = segments.sort((a, b) => a.start - b.start || a.end - b.end)
  for (let i = 1; i < sorted.length; i++) {
    const previous = sorted[i - 1]
    const current = sorted[i]
    if (previous.end > current.start) {
      return timePricingValidationMessage(t, 'overlap', {
        first: previous.label,
        firstRange: previous.range,
        second: current.label,
        secondRange: current.range,
      })
    }
  }

  return null
}

function timePricingValidationMessage(
  t: TranslateFn,
  key: string,
  params: Record<string, unknown>,
): string {
  return t(`admin.channels.timePricingValidation.${key}`, params)
}

function parseTimeToMinute(value: string): number | null {
  const match = TIME_PATTERN.exec(value)
  if (!match) return null
  return Number(match[1]) * 60 + Number(match[2])
}

function formatTimeRange(rule: Pick<TimePricingRuleFormEntry | TimePricingRule, 'start_time' | 'end_time'>): string {
  return `${rule.start_time || '-'}-${rule.end_time || '-'}`
}

function isValidTimeZone(timezone: string): boolean {
  try {
    Intl.DateTimeFormat(undefined, { timeZone: timezone })
    return true
  } catch {
    return false
  }
}

// ── 模型模式冲突检测 ──────────────────────────────────────

interface ModelPattern {
  pattern: string
  prefix: string  // lowercase, 通配符去掉尾部 *
  wildcard: boolean
}

function toModelPattern(model: string): ModelPattern {
  const lower = model.toLowerCase()
  const wildcard = lower.endsWith('*')
  return {
    pattern: model,
    prefix: wildcard ? lower.slice(0, -1) : lower,
    wildcard,
  }
}

function patternsConflict(a: ModelPattern, b: ModelPattern): boolean {
  if (!a.wildcard && !b.wildcard) return a.prefix === b.prefix
  if (a.wildcard && !b.wildcard) return b.prefix.startsWith(a.prefix)
  if (!a.wildcard && b.wildcard) return a.prefix.startsWith(b.prefix)
  // 双通配符：任一前缀是另一前缀的前缀即冲突
  return a.prefix.startsWith(b.prefix) || b.prefix.startsWith(a.prefix)
}

/** 检测模型模式列表中的冲突，返回冲突的两个模式名；无冲突返回 null */
export function findModelConflict(models: string[]): [string, string] | null {
  const patterns = models.map(toModelPattern)
  for (let i = 0; i < patterns.length; i++) {
    for (let j = i + 1; j < patterns.length; j++) {
      if (patternsConflict(patterns[i], patterns[j])) {
        return [patterns[i].pattern, patterns[j].pattern]
      }
    }
  }
  return null
}

// ── 区间校验 ──────────────────────────────────────────────

/** 校验区间列表的合法性，返回错误消息；通过则返回 null
 *
 * mode 决定区间语义：
 * - token：区间是上下文 token 数分段 (min, max]，不能重叠，无上限段必须放最后
 * - per_request / image：区间是按 tier_label 分层（1K/2K/4K 等），后端按 label
 *   匹配，不依赖 min/max，因此跳过重叠 / last-unlimited 校验
 */
export function validateIntervals(
  intervals: IntervalFormEntry[],
  mode: BillingMode,
  t: TranslateFn,
): string | null {
  if (!intervals || intervals.length === 0) return null

  // 按 min_tokens 排序（不修改原数组）
  const sorted = [...intervals].sort((a, b) => a.min_tokens - b.min_tokens)

  for (let i = 0; i < sorted.length; i++) {
    const err = validateSingleInterval(sorted[i], i, t)
    if (err) return err
  }

  // per_request / image 模式按 tier_label 匹配，不做 token 区间重叠校验
  if (mode !== 'token') return null
  return checkIntervalOverlap(sorted, t)
}

function intervalValidationMessage(
  t: TranslateFn,
  key: string,
  params: Record<string, unknown>,
): string {
  return t(`admin.channels.intervalValidation.${key}`, params)
}

function intervalPriceLabel(t: TranslateFn, key: string): string {
  return t(`admin.channels.intervalValidation.price.${key}`)
}

function validateSingleInterval(iv: IntervalFormEntry, idx: number, t: TranslateFn): string | null {
  const index = idx + 1
  if (iv.min_tokens < 0) {
    return intervalValidationMessage(
      t,
      'negativeMin',
      { index, value: iv.min_tokens },
    )
  }
  if (iv.max_tokens != null) {
    if (iv.max_tokens <= 0) {
      return intervalValidationMessage(
        t,
        'maxPositive',
        { index, value: iv.max_tokens },
      )
    }
    if (iv.max_tokens <= iv.min_tokens) {
      return intervalValidationMessage(
        t,
        'maxGreaterThanMin',
        { index, max: iv.max_tokens, min: iv.min_tokens },
      )
    }
  }
  return validateIntervalPrices(iv, idx, t)
}

function validateIntervalPrices(iv: IntervalFormEntry, idx: number, t: TranslateFn): string | null {
  const index = idx + 1
  const prices: [string, number | string | null][] = [
    ['inputPrice', iv.input_price],
    ['outputPrice', iv.output_price],
    ['cacheWritePrice', iv.cache_write_price],
    ['cacheWrite1hPrice', iv.cache_write_1h_price ?? null],
    ['cacheReadPrice', iv.cache_read_price],
    ['perRequestPrice', iv.per_request_price],
  ]
  for (const [key, val] of prices) {
    if (val != null && val !== '' && Number(val) < 0) {
      const field = intervalPriceLabel(t, key)
      return intervalValidationMessage(
        t,
        'negativePrice',
        { index, field },
      )
    }
  }
  const multipliers: [string, number | string | null][] = [
    ['inputMultiplier', iv.input_multiplier],
    ['outputMultiplier', iv.output_multiplier],
    ['cacheWriteMultiplier', iv.cache_write_multiplier],
    ['cacheReadMultiplier', iv.cache_read_multiplier],
  ]
  for (const [key, val] of multipliers) {
    if (!isValidPositiveMultiplier(val)) {
      return intervalValidationMessage(t, 'multiplierPositive', {
        index,
        field: intervalPriceLabel(t, key),
      })
    }
  }
  return null
}

function checkIntervalOverlap(sorted: IntervalFormEntry[], t: TranslateFn): string | null {
  for (let i = 0; i < sorted.length; i++) {
    // 无上限区间必须是最后一个
    if (sorted[i].max_tokens == null && i < sorted.length - 1) {
      return intervalValidationMessage(
        t,
        'unboundedLast',
        { index: i + 1 },
      )
    }
    if (i === 0) continue
    const prev = sorted[i - 1]
    // (min, max] 语义：前一个区间上界 > 当前区间下界则重叠
    if (prev.max_tokens == null || prev.max_tokens > sorted[i].min_tokens) {
      const prevMax = prev.max_tokens == null ? '∞' : String(prev.max_tokens)
      return intervalValidationMessage(
        t,
        'overlap',
        { previousIndex: i, currentIndex: i + 1, previousMax: prevMax, currentMin: sorted[i].min_tokens },
      )
    }
  }
  return null
}

/** 平台对应的模型 tag 样式（背景+文字） */
export function getPlatformTagClass(platform: string): string {
  switch (platform) {
    case 'anthropic': return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
    case 'openai': return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
    case 'gemini': return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
    case 'antigravity': return 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400'
    case 'grok': return 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300'
    case 'kimi': return 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400'
    case 'zhipu': return 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-400'
    case 'deepseek': return 'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-400'
    default: return 'bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400'
  }
}

/** 平台对应的模型文字色（仅 text-*，用于 input/text 场景）— 与 getPlatformTagClass 同色系 */
export function getPlatformTextClass(platform: string): string {
  switch (platform) {
    case 'anthropic': return 'text-orange-700 dark:text-orange-400'
    case 'openai': return 'text-emerald-700 dark:text-emerald-400'
    case 'gemini': return 'text-blue-700 dark:text-blue-400'
    case 'antigravity': return 'text-purple-700 dark:text-purple-400'
    case 'grok': return 'text-slate-700 dark:text-slate-300'
    case 'kimi': return 'text-pink-700 dark:text-pink-400'
    case 'zhipu': return 'text-indigo-700 dark:text-indigo-400'
    case 'deepseek': return 'text-teal-700 dark:text-teal-400'
    default: return ''
  }
}
