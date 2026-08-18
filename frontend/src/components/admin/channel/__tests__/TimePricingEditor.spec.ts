import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import Select from '@/components/common/Select.vue'
import TimePricingEditor from '../TimePricingEditor.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('TimePricingEditor', () => {
  it('edits customer-facing type names independently from multipliers', async () => {
    const wrapper = mount(TimePricingEditor, {
      props: {
        modelValue: {
          enabled: true,
          timezone: 'Asia/Shanghai',
          default_label: '平时',
          default_multiplier: 1.1,
          rules: [{
            label: '高峰',
            start_time: '09:00',
            end_time: '12:00',
            multiplier: 2,
          }],
        },
      },
    })

    const timezoneSelect = wrapper.getComponent(Select)
    expect(timezoneSelect.props('searchable')).toBe(true)
    expect(timezoneSelect.props('creatable')).toBe(true)
    expect(timezoneSelect.props('options')).toEqual(expect.arrayContaining([
      expect.objectContaining({ value: 'Asia/Shanghai' }),
      expect.objectContaining({ value: 'UTC' }),
    ]))
    const labelInputs = wrapper.findAll('input[type="text"]')
    expect(labelInputs).toHaveLength(2)
    expect((labelInputs[0].element as HTMLInputElement).value).toBe('平时')
    expect((labelInputs[1].element as HTMLInputElement).value).toBe('高峰')
    expect(wrapper.text()).toContain('admin.channels.form.timePricingTypeName')
    expect(wrapper.text()).not.toContain('admin.channels.form.timePricingPeak')

    await labelInputs[0].setValue('常规')
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toMatchObject({
      default_label: '常规',
      default_multiplier: 1.1,
    })

    await labelInputs[1].setValue('繁忙时段')
    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toMatchObject({
      rules: [expect.objectContaining({ label: '繁忙时段', multiplier: 2 })],
    })

    await timezoneSelect.vm.$emit('update:modelValue', 'UTC')
    const timezoneUpdate = wrapper.emitted('update:modelValue')?.at(-1)?.[0]
    expect(timezoneUpdate).toMatchObject({ timezone: 'UTC', default_label: '平时', default_multiplier: 1.1 })

    await wrapper.get('input[type="number"]').setValue('0.6')
    const multiplierUpdate = wrapper.emitted('update:modelValue')?.at(-1)?.[0]
    expect(multiplierUpdate).toMatchObject({
      timezone: 'Asia/Shanghai',
      default_multiplier: '0.6',
    })
  })
})
