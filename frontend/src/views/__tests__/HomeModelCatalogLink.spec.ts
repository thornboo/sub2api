import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(dir, '../HomeView.vue'), 'utf8')
const appHeaderSource = readFileSync(resolve(dir, '../../components/layout/AppHeader.vue'), 'utf8')

describe('Home model catalog entry', () => {
  it('links both the public header and Models CTA to the anonymous catalog', () => {
    expect(source.match(/to="\/model-plaza"/g)).toHaveLength(2)
    expect(source).not.toContain('to="/available-channels"')
  })

  it('hides the public entries through the existing model plaza feature flag', () => {
    expect(source).toContain('FeatureFlags.modelPlaza')
    expect(source.match(/v-if="modelPlazaEnabled"/g)).toHaveLength(2)
  })

  it('does not duplicate the public catalog entry in the authenticated console header', () => {
    expect(appHeaderSource).not.toContain("path: '/model-plaza'")
    expect(appHeaderSource).not.toContain("t('nav.modelPlaza')")
  })
})
