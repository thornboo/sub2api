import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import en from '../locales/en/enterpriseMembers'
import enRoot from '../locales/en'
import zh from '../locales/zh/enterpriseMembers'
import zhRoot from '../locales/zh'

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

const zhMessages = flatten(zh)
const enMessages = flatten(en)
const testDir = dirname(fileURLToPath(import.meta.url))
const viewSource = readFileSync(resolve(testDir, '../../views/user/EnterpriseMembersView.vue'), 'utf8')

describe('enterprise member locales', () => {
  it('keeps zh and en key sets exactly symmetric', () => {
    expect(Object.keys(zhMessages).sort()).toEqual(Object.keys(enMessages).sort())
    expect(Object.values(zhMessages).every(Boolean)).toBe(true)
    expect(Object.values(enMessages).every(Boolean)).toBe(true)
  })

  it('defines every enterpriseMembers key referenced by the view', () => {
    const referenced = [...viewSource.matchAll(/t\('enterpriseMembers\.([^']+)'/g)].map(match => match[1])
    expect(referenced.length).toBeGreaterThan(250)
    for (const key of referenced) {
      expect(zhMessages[key], `missing zh enterpriseMembers.${key}`).toBeTypeOf('string')
      expect(enMessages[key], `missing en enterpriseMembers.${key}`).toBeTypeOf('string')
    }
  })

  it('does not retain the page-local bilingual helper', () => {
    expect(viewSource).not.toMatch(/\btext\(/)
    expect(viewSource).not.toContain('const text =')
  })

  it('merges console copy without replacing the existing navigation title', () => {
    expect(zhRoot.enterpriseMembers.title).toBe('企业成员')
    expect(enRoot.enterpriseMembers.title).toBe('Enterprise Members')
    expect(zhRoot.enterpriseMembers.copy.auditTrail).toBe('操作审计')
    expect(enRoot.enterpriseMembers.copy.auditTrail).toBe('Audit trail')
  })
})
