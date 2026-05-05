import { describe, expect, it } from 'vitest'
import { flattenModelsDevCatalog, searchModelCatalogEntries } from '../modelCatalog'

describe('modelCatalog', () => {
  const rawCatalog = {
    openai: {
      id: 'openai',
      name: 'OpenAI',
      models: {
        'gpt-5.5': {
          id: 'gpt-5.5',
          name: 'GPT-5.5',
          family: 'gpt',
          modalities: {
            input: ['text', 'image'],
            output: ['text']
          },
          limit: {
            context: 400000
          }
        }
      }
    },
    anthropic: {
      id: 'anthropic',
      name: 'Anthropic',
      models: {
        'claude-opus-4-7': {
          id: 'claude-opus-4-7',
          name: 'Claude Opus 4.7',
          family: 'claude-opus',
          modalities: {
            input: ['text'],
            output: ['text']
          }
        }
      }
    }
  }

  it('flattens the provider keyed models.dev catalog', () => {
    expect(flattenModelsDevCatalog(rawCatalog)).toEqual([
      {
        id: 'gpt-5.5',
        name: 'GPT-5.5',
        providerId: 'openai',
        providerName: 'OpenAI',
        family: 'gpt',
        context: 400000,
        modalities: ['text', 'image']
      },
      {
        id: 'claude-opus-4-7',
        name: 'Claude Opus 4.7',
        providerId: 'anthropic',
        providerName: 'Anthropic',
        family: 'claude-opus',
        context: undefined,
        modalities: ['text']
      }
    ])
  })

  it('searches by model id, model name, provider, and family', () => {
    const entries = flattenModelsDevCatalog(rawCatalog)

    expect(searchModelCatalogEntries(entries, 'gpt-5').map(entry => entry.id)).toEqual(['gpt-5.5'])
    expect(searchModelCatalogEntries(entries, 'opus').map(entry => entry.id)).toEqual(['claude-opus-4-7'])
    expect(searchModelCatalogEntries(entries, 'anthropic').map(entry => entry.id)).toEqual(['claude-opus-4-7'])
  })
})
