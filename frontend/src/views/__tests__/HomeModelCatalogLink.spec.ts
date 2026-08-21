import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(dir, '../HomeView.vue'), 'utf8')
const appHeaderSource = readFileSync(resolve(dir, '../../components/layout/AppHeader.vue'), 'utf8')

describe('Home model catalog entry', () => {
  it('links compact and default home entries to the model catalog', () => {
    expect(source.match(/to="\/model-plaza"/g)).toHaveLength(3)
    expect(source).not.toContain('to="/available-channels"')
  })

  it('gates every public entry through the shared feature and auth decision', () => {
    expect(source).toContain('FeatureFlags.modelPlaza')
    expect(source.match(/v-if="showModelPlazaEntry"/g)).toHaveLength(3)
  })

  it('does not duplicate the public catalog entry in the authenticated console header', () => {
    expect(appHeaderSource).not.toContain("path: '/model-plaza'")
    expect(appHeaderSource).not.toContain("t('nav.modelPlaza')")
  })
})
