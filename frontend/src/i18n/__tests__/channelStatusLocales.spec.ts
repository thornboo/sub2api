import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import en from '../locales/en'
import zh from '../locales/zh'

type Messages = Record<string, string | Messages>

function flatten(messages: Messages, prefix = ''): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [key, value] of Object.entries(messages)) {
    const path = prefix ? `${prefix}.${key}` : key
    if (typeof value === 'string') out[path] = value
    else Object.assign(out, flatten(value, path))
  }
  return out
}

const testDir = dirname(fileURLToPath(import.meta.url))
const viewSource = readFileSync(resolve(testDir, '../../views/user/ChannelStatusView.vue'), 'utf8')
const routerSource = readFileSync(resolve(testDir, '../../router/index.ts'), 'utf8')
const zhMessages = flatten(zh.channelStatus as Messages)
const enMessages = flatten(en.channelStatus as Messages)

describe('model status page contracts', () => {
  it('defines every statically referenced channelStatus key in both locales', () => {
    const referenced = [...viewSource.matchAll(/t\('channelStatus\.([^']+)'/g)].map(match => match[1])
    expect(referenced.length).toBeGreaterThan(20)
    for (const key of referenced) {
      expect(zhMessages[key], `missing zh channelStatus.${key}`).toBeTypeOf('string')
      expect(enMessages[key], `missing en channelStatus.${key}`).toBeTypeOf('string')
    }
  })

  it('keeps dynamic status and time-window keys complete', () => {
    for (const key of [
      'windowTab.today',
      'windowTab.24h',
      'windowTab.7d',
      'windowTab.30d',
      'overall.operational',
      'overall.degraded',
      'overall.unavailable',
      'overall.unknown',
      'message.normal',
      'message.partial',
      'message.unavailable',
      'message.no_data',
    ]) {
      expect(zhMessages[key], `missing zh channelStatus.${key}`).toBeTypeOf('string')
      expect(enMessages[key], `missing en channelStatus.${key}`).toBeTypeOf('string')
    }
  })

  it('keeps the authenticated /monitor route bound to the model status view', () => {
    const route = routerSource.slice(
      routerSource.indexOf("path: '/monitor'"),
      routerSource.indexOf("path: '/admin/subscriptions'")
    )
    expect(route).toContain("name: 'ChannelStatus'")
    expect(route).toContain("import('@/views/user/ChannelStatusView.vue')")
    expect(route).toContain('requiresAuth: true')
    expect(route).toContain('requiresAdmin: false')
  })
})
