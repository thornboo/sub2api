import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import EndpointDistributionChart from '../EndpointDistributionChart.vue'

const messages: Record<string, string> = {
  'usage.endpointDistribution': 'Endpoint Distribution',
  'usage.endpoint': 'Endpoint',
  'usage.inbound': 'Inbound',
  'usage.upstream': 'Upstream',
  'usage.path': 'Path',
  'admin.dashboard.requests': 'Requests',
  'admin.dashboard.tokens': 'Tokens',
  'admin.dashboard.actual': 'Actual',
  'admin.dashboard.standard': 'Standard',
  'admin.dashboard.metricTokens': 'By Tokens',
  'admin.dashboard.metricActualCost': 'By Actual Cost',
  'admin.dashboard.noDataAvailable': 'No data available',
  'common.expand': 'Expand',
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

vi.mock('vue-chartjs', () => ({
  Doughnut: {
    props: ['data'],
    template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>',
  },
}))

describe('EndpointDistributionChart', () => {
  const endpointStats = [
    {
      endpoint: '/v1/messages',
      requests: 4,
      total_tokens: 400,
      cost: 0.6,
      actual_cost: 0.2,
    },
    {
      endpoint: '/v1/responses',
      requests: 12,
      total_tokens: 1200,
      cost: 1.8,
      actual_cost: 0.9,
    },
  ]

  it('renders current-order ranks for endpoint rows', () => {
    const wrapper = mount(EndpointDistributionChart, {
      props: {
        endpointStats,
      },
      global: {
        stubs: {
          LoadingSpinner: true,
        },
      },
    })

    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('#1')
    expect(rows[0].text()).toContain('/v1/responses')
    expect(rows[1].text()).toContain('#2')
    expect(rows[1].text()).toContain('/v1/messages')
  })

  it('emits expand when the chart expand button is enabled', async () => {
    const wrapper = mount(EndpointDistributionChart, {
      props: {
        endpointStats,
        showExpandButton: true,
      },
      global: {
        stubs: {
          LoadingSpinner: true,
        },
      },
    })

    await wrapper.find('button[aria-label="Expand"]').trigger('click')

    expect(wrapper.emitted('expand')).toHaveLength(1)
  })
})
