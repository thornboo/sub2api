import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ChannelModelDeliveryDialog from '@/components/admin/channel/ChannelModelDeliveryDialog.vue'
import type { ChannelModelDelivery } from '@/api/admin/channels'

const messages: Record<string, string> = {
  'admin.channels.deliveryDialog.title': 'Model API Endpoints and Routes',
  'admin.channels.deliveryDialog.publicEndpoints': 'Customer-callable API endpoints',
  'admin.channels.deliveryDialog.groupsUnit': 'groups',
  'admin.channels.deliveryDialog.actualUpstream': 'Actual upstream',
  'admin.channels.deliveryDialog.protocolStatus.available': 'Callable',
  'admin.channels.deliveryDialog.protocolStatus.blocked': 'Blocked',
  'admin.channels.deliveryDialog.reason.protocol_capability_unknown': 'Protocol support is not confirmed',
  'admin.channels.deliveryDialog.mode.native': 'Native',
  'admin.channels.deliveryDialog.mode.compatibility': 'Compatibility',
  'admin.channels.deliveryDialog.mode.mixed': 'Mixed',
  'admin.channels.form.deliveryStatus.partial': 'Partially available',
  'common.close': 'Close',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string | number>, fallback?: string) => {
      let message = messages[key] ?? fallback ?? key
      for (const [name, replacement] of Object.entries(params ?? {})) {
        message = message.replaceAll(`{${name}}`, String(replacement))
      }
      return message
    },
  }),
}))

const model: ChannelModelDelivery = {
  name: 'glm-5.2',
  platform: 'openai',
  status: 'partial',
  deliverable_group_count: 1,
  total_group_count: 1,
  route_count: 1,
  endpoints: [
    { protocol: 'openai_chat_completions', path: '/v1/chat/completions', mode: 'native', group_ids: [10] },
  ],
  protocols: [
    {
      protocol: 'openai_chat_completions',
      path: '/v1/chat/completions',
      status: 'available',
      mode: 'native',
      upstream_protocol: 'openai_chat_completions',
      reason_codes: [],
      group_ids: [10],
    },
    {
      protocol: 'openai_responses',
      path: '/v1/responses',
      status: 'blocked',
      reason_codes: ['protocol_capability_unknown'],
    },
  ],
  groups: [],
}

describe('ChannelModelDeliveryDialog protocol decisions', () => {
  it('distinguishes callable endpoints from blocked protocols and explains the reason', () => {
    const wrapper = mount(ChannelModelDeliveryDialog, {
      props: { show: true, model },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          PlatformIcon: { template: '<span />' },
        },
      },
    })

    expect(wrapper.text()).toContain('/v1/chat/completions')
    expect(wrapper.text()).toContain('Callable')
    expect(wrapper.text()).toContain('Native')
    expect(wrapper.text()).toContain('Actual upstream')
    expect(wrapper.text()).toContain('/v1/responses')
    expect(wrapper.text()).toContain('Blocked')
    expect(wrapper.text()).toContain('Protocol support is not confirmed')
  })

  it('keeps the blocking reason visible when a blocked route also has a model chain', () => {
    const modelWithBlockedRoute: ChannelModelDelivery = {
      ...model,
      groups: [{
        id: 10,
        name: 'primary',
        platform: 'openai',
        status: 'no_endpoint',
        route_count: 1,
        protocols: [],
        routes: [{
          account_id: 82,
          account_name: 'upstream-a',
          channel_mapped_model: 'public-glm',
          upstream_model: 'upstream-glm',
          endpoints: [],
          protocols: [{
            protocol: 'openai_responses',
            path: '/v1/responses',
            status: 'blocked',
            channel_mapped_model: 'public-glm',
            upstream_model: 'upstream-glm',
            reason_codes: ['protocol_capability_unknown'],
          }],
        }],
      }],
    }
    const wrapper = mount(ChannelModelDeliveryDialog, {
      props: { show: true, model: modelWithBlockedRoute },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          PlatformIcon: { template: '<span />' },
        },
      },
    })

    expect(wrapper.text().match(/Protocol support is not confirmed/g)).toHaveLength(2)
    expect(wrapper.text()).not.toContain('public-glm → upstream-glm')
  })
})
