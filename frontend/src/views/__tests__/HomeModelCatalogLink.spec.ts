import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const source = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../HomeView.vue'),
  'utf8',
)

describe('Home model catalog entry', () => {
  it('links both the public header and Models CTA to the anonymous catalog', () => {
    expect(source.match(/to="\/model-plaza"/g)).toHaveLength(2)
    expect(source).not.toContain('to="/available-channels"')
  })

  it('hides the public entries through the existing model plaza feature flag', () => {
    expect(source).toContain('FeatureFlags.modelPlaza')
    expect(source.match(/v-if="modelPlazaEnabled"/g)).toHaveLength(2)
  })
})
