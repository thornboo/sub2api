import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const routerSource = readFileSync(resolve(currentDir, '../index.ts'), 'utf8')
const sidebarSource = readFileSync(resolve(currentDir, '../../components/layout/AppSidebar.vue'), 'utf8')
const appHeaderSource = readFileSync(resolve(currentDir, '../../components/layout/AppHeader.vue'), 'utf8')

describe('feedback route integration', () => {
  it('exposes admin feedback management and signed-in user feedback entry', () => {
    expect(routerSource).toMatch(/path: '\/admin\/feedback'[\s\S]*?requiresAdmin: true/)
    expect(routerSource).toContain("titleKey: 'admin.feedback.title'")
    expect(routerSource).toMatch(/path: '\/feedback'[\s\S]*?component: \(\) => import\('@\/views\/user\/FeedbackView.vue'\)/)
    expect(routerSource).toContain("titleKey: 'feedback.myTickets'")
    expect(sidebarSource).toContain("{ path: '/admin/feedback', label: t('nav.feedback'), icon: TicketIcon }")
    expect(sidebarSource).toContain("{ path: '/feedback', label: t('nav.feedback'), icon: TicketIcon }")
    expect(appHeaderSource).toContain('to="/feedback"')
  })
})
