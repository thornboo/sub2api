import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const modalSource = readFileSync(resolve(dir, '../UserEditModal.vue'), 'utf8')
const apiSource = readFileSync(resolve(dir, '../../../../api/admin/users.ts'), 'utf8')

describe('enterprise account lifecycle controls', () => {
  it('exposes a non-destructive enterprise capability toggle', () => {
    expect(modalSource).toContain('form.enterprise_enabled')
    expect(modalSource).toContain("admin.users.form.enterpriseCapability")
    expect(modalSource).toContain("admin.users.form.enterpriseCapabilityHint")
    expect(modalSource).toContain("if (data.account_type === 'enterprise') data.enterprise_enabled = form.enterprise_enabled")
  })

  it('forwards the enterprise account filter to the admin API', () => {
    expect(apiSource).toContain('account_type: filters?.account_type')
  })
})
