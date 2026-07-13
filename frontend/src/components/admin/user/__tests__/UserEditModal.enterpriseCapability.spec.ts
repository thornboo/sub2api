import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const modalSource = readFileSync(resolve(dir, '../UserEditModal.vue'), 'utf8')
const createModalSource = readFileSync(resolve(dir, '../UserCreateModal.vue'), 'utf8')
const apiSource = readFileSync(resolve(dir, '../../../../api/admin/users.ts'), 'utf8')

describe('enterprise account lifecycle controls', () => {
  it('exposes a non-destructive enterprise capability toggle', () => {
    expect(modalSource).toContain('form.enterprise_enabled')
    expect(modalSource).toContain("admin.users.form.enterpriseCapability")
    expect(modalSource).toContain("admin.users.form.enterpriseCapabilityHint")
    expect(modalSource).toContain("import BaseCheckbox from '@/components/common/BaseCheckbox.vue'")
    expect(modalSource).toContain('<BaseCheckbox')
    expect(modalSource).not.toContain('v-model="form.enterprise_enabled" type="checkbox"')
    expect(modalSource).toContain("if (data.account_type === 'enterprise') data.enterprise_enabled = form.enterprise_enabled")
  })

  it('uses the shared select visual contract for account type and role fields', () => {
    for (const source of [modalSource, createModalSource]) {
      expect(source).toContain("import Select, { type SelectOption } from '@/components/common/Select.vue'")
      expect(source).toContain('v-model="form.account_type"')
      expect(source).toContain(':options="accountTypeOptions"')
      expect(source).toContain('v-model="form.role"')
      expect(source).toContain(':options="roleOptions"')
      expect(source).not.toContain('<select v-model="form.account_type"')
      expect(source).not.toContain('<select v-model="form.role"')
    }
  })

  it('forwards the enterprise account filter to the admin API', () => {
    expect(apiSource).toContain('account_type: filters?.account_type')
  })
})
