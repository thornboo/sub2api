import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const routerSource = readFileSync(resolve(currentDir, '../index.ts'), 'utf8')
const sidebarSource = readFileSync(resolve(currentDir, '../../components/layout/AppSidebar.vue'), 'utf8')

describe('enterprise account access policy', () => {
  it('keeps member administration behind enabled capability while preserving historical usage access', () => {
    expect(routerSource).toMatch(/path: '\/enterprise\/members',[\s\S]*?requiresEnterprise: true/)
    expect(routerSource).toMatch(/path: '\/enterprise\/member-usage',[\s\S]*?requiresEnterpriseAccount: true/)
    expect(routerSource).toContain('if (to.meta.requiresEnterpriseAccount')
    expect(routerSource).toContain("authStore.user?.account_type !== 'enterprise'")
  })

  it('keeps the historical member-usage navigation visible after enterprise writes are disabled', () => {
    expect(sidebarSource).toContain('const flagEnterpriseAccount = () =>')
    expect(sidebarSource).toContain('const flagEnterpriseMembers = () => flagEnterpriseAccount() && !authStore.user?.enterprise_disabled_at')
    expect(sidebarSource).toMatch(/path: '\/enterprise\/members'.*featureFlag: flagEnterpriseMembers/)
    expect(sidebarSource).toMatch(/path: '\/enterprise\/member-usage'.*featureFlag: flagEnterpriseAccount/)
  })
})
