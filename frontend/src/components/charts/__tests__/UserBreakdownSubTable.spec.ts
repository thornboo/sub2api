import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import UserBreakdownSubTable from '../UserBreakdownSubTable.vue'

const messages: Record<string, string> = {
  'admin.dashboard.noDataAvailable': 'No data available',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

describe('UserBreakdownSubTable', () => {
  it('renders compact ranks for each user breakdown row', () => {
    const wrapper = mount(UserBreakdownSubTable, {
      props: {
        items: [
          {
            user_id: 4,
            email: 'first@example.com',
            requests: 12,
            total_tokens: 1500,
            cost: 1.2,
            actual_cost: 1,
            account_cost: 0.8,
          },
          {
            user_id: 8,
            email: 'second@example.com',
            requests: 3,
            total_tokens: 400,
            cost: 0.2,
            actual_cost: 0.1,
            account_cost: 0.09,
          },
        ],
      },
      global: {
        stubs: {
          LoadingSpinner: true,
        },
      },
    })

    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('#1')
    expect(rows[0].text()).toContain('first@example.com')
    expect(rows[1].text()).toContain('#2')
    expect(rows[1].text()).toContain('second@example.com')
  })
})
