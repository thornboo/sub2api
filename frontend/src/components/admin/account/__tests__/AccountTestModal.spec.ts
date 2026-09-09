import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AccountTestModal from '../AccountTestModal.vue'

const { getAvailableModels, resolveTestModel, copyToClipboard } = vi.hoisted(() => ({
  getAvailableModels: vi.fn(),
  resolveTestModel: vi.fn(),
  copyToClipboard: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getAvailableModels,
      resolveTestModel
    }
  }
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'admin.accounts.imagePromptDefault': 'Generate a cute orange cat astronaut sticker on a clean pastel background.'
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (key === 'admin.accounts.imageReceived' && params?.count) {
          return `received-${params.count}`
        }
        if (key === 'admin.accounts.imagePreviewAlt' && params?.index) {
          return `test-image-${params.index}`
        }
        return messages[key] || key
      }
    })
  }
})

function createStreamResponse(lines: string[]) {
  const encoder = new TextEncoder()
  const chunks = lines.map((line) => encoder.encode(line))
  let index = 0

  return {
    ok: true,
    body: {
      getReader: () => ({
        read: vi.fn().mockImplementation(async () => {
          if (index < chunks.length) {
            return { done: false, value: chunks[index++] }
          }
          return { done: true, value: undefined }
        })
      })
    }
  } as Response
}

function deferredModels() {
  let resolve!: (models: Array<Record<string, unknown>>) => void
  let reject!: (reason: Error) => void
  const promise = new Promise<Array<Record<string, unknown>>>((innerResolve, innerReject) => {
    resolve = innerResolve
    reject = innerReject
  })
  return { promise, resolve, reject }
}

function mountModal(account: Record<string, unknown> = {
  id: 42,
  name: 'Gemini Image Test',
  platform: 'gemini',
  type: 'apikey',
  status: 'active'
}, stubModelSelect = true) {
  return mount(AccountTestModal, {
    props: {
      show: false,
      account
    } as any,
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        AccountTestModelSelect: stubModelSelect ? {
          props: ['modelValue', 'options'],
          emits: ['update:modelValue'],
          template: '<div class="account-test-model-select-stub">{{ modelValue }} {{ options.length }}</div>'
        } : false,
        Select: { template: '<div class="select-stub"></div>' },
        TextArea: {
          props: ['modelValue'],
          emits: ['update:modelValue'],
          template: '<textarea class="textarea-stub" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
        },
        Icon: true
      }
    }
  })
}

describe('AccountTestModal', () => {
  beforeEach(() => {
    getAvailableModels.mockReset()
    resolveTestModel.mockReset()
    getAvailableModels.mockResolvedValue([
      { id: 'gemini-2.0-flash', display_name: 'Gemini 2.0 Flash' },
      { id: 'gemini-2.5-flash-image', display_name: 'Gemini 2.5 Flash Image' },
      { id: 'gemini-3.1-flash-image', display_name: 'Gemini 3.1 Flash Image' }
    ])
    copyToClipboard.mockReset()
    Object.defineProperty(globalThis, 'localStorage', {
      value: {
        getItem: vi.fn((key: string) => (key === 'auth_token' ? 'test-token' : null)),
        setItem: vi.fn(),
        removeItem: vi.fn(),
        clear: vi.fn()
      },
      configurable: true
    })
    global.fetch = vi.fn().mockResolvedValue(
      createStreamResponse([
        'data: {"type":"test_start","model":"gemini-2.5-flash-image"}\n',
        'data: {"type":"image","image_url":"data:image/png;base64,QUJD","mime_type":"image/png"}\n',
        'data: {"type":"test_complete","success":true}\n'
      ])
    ) as any
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('gemini 图片模型测试会携带提示词并渲染图片预览', async () => {
    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()

    const promptInput = wrapper.find('textarea.textarea-stub')
    expect(promptInput.exists()).toBe(true)
    await promptInput.setValue('draw a tiny orange cat astronaut')

    const buttons = wrapper.findAll('button')
    const startButton = buttons.find((button) => button.text().includes('admin.accounts.startTest'))
    expect(startButton).toBeTruthy()

    await startButton!.trigger('click')
    await flushPromises()
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(1)
    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toEqual({
      model_id: 'gemini-3.1-flash-image',
      prompt: 'draw a tiny orange cat astronaut'
    })

    const preview = wrapper.find('img[alt="test-image-1"]')
    expect(preview.exists()).toBe(true)
    expect(preview.attributes('src')).toBe('data:image/png;base64,QUJD')
  })

  it('grok 账号测试默认选择 Grok 模型', async () => {
    getAvailableModels.mockResolvedValue([
      { id: 'grok-4.3', display_name: 'Grok 4.3' },
      { id: 'grok-build-0.1', display_name: 'Grok Build 0.1' }
    ])
    global.fetch = vi.fn().mockResolvedValue(
      createStreamResponse([
        'data: {"type":"test_start","model":"grok-4.3"}\n',
        'data: {"type":"content","text":"ok"}\n',
        'data: {"type":"test_complete","success":true}\n'
      ])
    ) as any

    const wrapper = mountModal({
      id: 13,
      name: 'Grok Account',
      platform: 'grok',
      type: 'oauth',
      status: 'active'
    })
    await wrapper.setProps({ show: true })
    await flushPromises()

    const buttons = wrapper.findAll('button')
    const startButton = buttons.find((button) => button.text().includes('admin.accounts.startTest'))
    expect(startButton).toBeTruthy()

    await startButton!.trigger('click')
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(1)
    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toEqual({
      model_id: 'grok-4.3',
      prompt: '',
      mode: 'text'
    })
  })

  it('OpenAI Compact 探测会携带 compact 测试模式', async () => {
    getAvailableModels.mockResolvedValue([
      { id: 'gpt-5.4', display_name: 'GPT-5.4' }
    ])
    global.fetch = vi.fn().mockResolvedValue(
      createStreamResponse([
        'data: {"type":"test_complete","success":true}\n'
      ])
    ) as any

    const wrapper = mountModal({
      id: 42,
      name: 'OpenAI OAuth',
      platform: 'openai',
      type: 'oauth',
      status: 'active'
    })
    await wrapper.setProps({ show: true })
    await flushPromises()

    ;(wrapper.vm as any).selectedModelId = 'gpt-5.4'
    ;(wrapper.vm as any).testMode = 'compact'
    await (wrapper.vm as any).startTest()
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(1)
    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toMatchObject({
      model_id: 'gpt-5.4',
      prompt: '',
      mode: 'compact'
    })
  })

  it('Claude API Key 测试请求会保留别名左侧 model_id', async () => {
    getAvailableModels.mockResolvedValue([
      {
        id: 'claude-alias',
        display_name: 'Claude Alias',
        upstream_model_id: 'claude-target'
      }
    ])
    global.fetch = vi.fn().mockResolvedValue(
      createStreamResponse([
        'data: {"type":"test_start","model":"claude-target"}\n',
        'data: {"type":"content","text":"ok"}\n',
        'data: {"type":"test_complete","success":true}\n'
      ])
    ) as any

    const wrapper = mountModal({
      id: 51,
      name: 'Claude API Key',
      platform: 'anthropic',
      type: 'apikey',
      status: 'active'
    })
    await wrapper.setProps({ show: true })
    await flushPromises()

    await (wrapper.vm as any).startTest()
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(1)
    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toMatchObject({
      model_id: 'claude-alias',
      prompt: ''
    })
  })

  it('OpenAI 图片别名按最终目标识别媒体请求但提交左侧 model_id', async () => {
    getAvailableModels.mockResolvedValue([
      {
        id: 'openai-image-alias',
        display_name: 'OpenAI Image Alias',
        upstream_model_id: 'gpt-image-1'
      }
    ])

    const wrapper = mountModal({
      id: 61,
      name: 'OpenAI API Key',
      platform: 'openai',
      type: 'apikey',
      status: 'active'
    })
    await wrapper.setProps({ show: true })
    await flushPromises()

    await (wrapper.vm as any).startTest()
    await flushPromises()

    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toMatchObject({
      model_id: 'openai-image-alias',
      prompt: 'Generate a cute orange cat astronaut sticker on a clean pastel background.',
      mode: 'default'
    })
  })

  it('OpenAI compact 模式会过滤图片别名并重新选择文字模型', async () => {
    getAvailableModels.mockResolvedValue([
      {
        id: 'openai-image-alias',
        display_name: 'OpenAI Image Alias',
        upstream_model_id: 'gpt-image-1'
      },
      {
        id: 'openai-text-alias',
        display_name: 'OpenAI Text Alias',
        upstream_model_id: 'gpt-5.4'
      }
    ])

    const wrapper = mountModal({
      id: 66,
      name: 'OpenAI API Key',
      platform: 'openai',
      type: 'apikey',
      status: 'active'
    })
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect((wrapper.vm as any).selectedModelId).toBe('openai-image-alias')

    ;(wrapper.vm as any).testMode = 'compact'
    await flushPromises()

    expect((wrapper.vm as any).selectedModelId).toBe('openai-text-alias')
    await (wrapper.vm as any).startTest()
    await flushPromises()

    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toMatchObject({
      model_id: 'openai-text-alias',
      prompt: '',
      mode: 'compact'
    })
  })

  it('OpenAI compact 模式会阻止通配解析出的图片目标发起测试', async () => {
    getAvailableModels.mockResolvedValue([
      {
        id: 'openai-*',
        display_name: 'OpenAI Wildcard',
        upstream_model_id: 'gpt-*',
        is_pattern: true
      }
    ])

    const wrapper = mountModal({
      id: 67,
      name: 'OpenAI API Key',
      platform: 'openai',
      type: 'apikey',
      status: 'active'
    })
    await wrapper.setProps({ show: true })
    await flushPromises()

    ;(wrapper.vm as any).testMode = 'compact'
    await flushPromises()
    ;(wrapper.vm as any).selectedModelId = 'openai-image-concrete'
    ;(wrapper.vm as any).selectedModelTarget = 'gpt-image-1'
    await flushPromises()

    await (wrapper.vm as any).startTest()

    expect(global.fetch).not.toHaveBeenCalled()
  })

  it('Gemini 图片别名按最终目标识别媒体请求但提交左侧 model_id', async () => {
    getAvailableModels.mockResolvedValue([
      {
        id: 'gemini-image-alias',
        display_name: 'Gemini Image Alias',
        upstream_model_id: 'gemini-3.1-flash-image'
      }
    ])

    const wrapper = mountModal({
      id: 62,
      name: 'Gemini API Key',
      platform: 'gemini',
      type: 'apikey',
      status: 'active'
    })
    await wrapper.setProps({ show: true })
    await flushPromises()

    await (wrapper.vm as any).startTest()
    await flushPromises()

    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toMatchObject({
      model_id: 'gemini-image-alias',
      prompt: 'Generate a cute orange cat astronaut sticker on a clean pastel background.'
    })
  })

  it('Grok 按最终目标过滤当前模式模型并提交左侧 model_id', async () => {
    getAvailableModels.mockResolvedValue([
      {
        id: 'grok-chat-alias',
        display_name: 'Grok Chat Alias',
        upstream_model_id: 'grok-4.5'
      },
      {
        id: 'grok-image-alias',
        display_name: 'Grok Image Alias',
        upstream_model_id: 'grok-imagine-image-v1'
      }
    ])

    const wrapper = mountModal({
      id: 63,
      name: 'Grok Account',
      platform: 'grok',
      type: 'oauth',
      status: 'active'
    })
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect((wrapper.vm as any).selectedModelId).toBe('grok-chat-alias')

    ;(wrapper.vm as any).grokTestMode = 'image'
    await flushPromises()
    expect((wrapper.vm as any).selectedModelId).toBe('grok-image-alias')

    await (wrapper.vm as any).startTest()
    await flushPromises()

    const [, request] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(request.body)).toMatchObject({
      model_id: 'grok-image-alias',
      prompt: 'Generate a cute orange cat astronaut sticker on a clean pastel background.',
      mode: 'image'
    })
  })

  it('默认测试模型会跳过 disabled 选项', async () => {
    getAvailableModels.mockResolvedValue([
      {
        id: 'invalid-bedrock-alias',
        display_name: 'Invalid Bedrock Alias',
        upstream_model_id: '',
        disabled: true
      },
      {
        id: 'claude-sonnet-alias',
        display_name: 'Claude Sonnet Alias',
        upstream_model_id: 'claude-sonnet-4-20250514'
      }
    ])

    const wrapper = mountModal({
      id: 65,
      name: 'Claude API Key',
      platform: 'anthropic',
      type: 'apikey',
      status: 'active'
    })
    await wrapper.setProps({ show: true })
    await flushPromises()

    expect((wrapper.vm as any).selectedModelId).toBe('claude-sonnet-alias')
  })

  it('Grok 通配解析到错误媒体目标后不能在当前模式发起测试', async () => {
    getAvailableModels.mockResolvedValue([
      {
        id: 'grok-imagine-*',
        display_name: 'Grok Image Wildcard',
        upstream_model_id: 'grok-imagine-*',
        is_pattern: true
      }
    ])

    const wrapper = mountModal({
      id: 64,
      name: 'Grok Account',
      platform: 'grok',
      type: 'oauth',
      status: 'active'
    })
    await wrapper.setProps({ show: true })
    await flushPromises()

    ;(wrapper.vm as any).grokTestMode = 'image'
    ;(wrapper.vm as any).selectedModelId = 'grok-4.5'
    ;(wrapper.vm as any).selectedModelTarget = 'grok-4.5'
    await flushPromises()

    await (wrapper.vm as any).startTest()

    expect(global.fetch).not.toHaveBeenCalled()
  })

  it('慢账号 A 模型响应不会覆盖账号 B 的模型选择', async () => {
    const accountA = {
      id: 51,
      name: 'Claude A',
      platform: 'anthropic',
      type: 'apikey',
      status: 'active'
    }
    const accountB = {
      id: 52,
      name: 'Claude B',
      platform: 'anthropic',
      type: 'apikey',
      status: 'active'
    }
    const slowAccountA = deferredModels()
    getAvailableModels
      .mockReturnValueOnce(slowAccountA.promise)
      .mockResolvedValueOnce([
        { id: 'claude-sonnet-b', display_name: 'Claude Sonnet B' }
      ])

    const wrapper = mountModal(accountA)
    await wrapper.setProps({ show: true })
    await wrapper.setProps({ account: accountB })
    await flushPromises()

    expect((wrapper.vm as any).selectedModelId).toBe('claude-sonnet-b')

    slowAccountA.resolve([
      { id: 'claude-sonnet-a', display_name: 'Claude Sonnet A' }
    ])
    await flushPromises()

    expect((wrapper.vm as any).selectedModelId).toBe('claude-sonnet-b')
    expect((wrapper.vm as any).availableModels.map((model: { id: string }) => model.id)).toEqual([
      'claude-sonnet-b'
    ])
  })

  it.each([
    { scenario: '切换账号后旧请求成功', accountId: 72, failed: false },
    { scenario: '切换账号后旧请求失败', accountId: 72, failed: true },
    { scenario: '同账号重开后旧请求成功', accountId: 71, failed: false }
  ])('$scenario 不会重置已解析的 Grok 通配模型', async ({ accountId, failed }) => {
    vi.useFakeTimers()
    const oldRequest = deferredModels()
    getAvailableModels
      .mockReturnValueOnce(oldRequest.promise)
      .mockResolvedValueOnce([
        { id: 'alias-*', display_name: 'alias-* → grok-4.3', upstream_model_id: 'grok-4.3', is_pattern: true }
      ])
    resolveTestModel.mockResolvedValue({
      model_id: 'alias-concrete',
      upstream_model_id: 'grok-4.3'
    })
    const account = {
      id: 71,
      name: 'Grok Account',
      platform: 'grok',
      type: 'apikey',
      status: 'active'
    }
    const wrapper = mountModal(account, false)

    try {
      await wrapper.setProps({ show: true })
      await wrapper.setProps({ show: false })
      await wrapper.setProps({ show: true, account: { ...account, id: accountId } })
      await flushPromises()

      const input = wrapper.get('[data-testid="account-test-concrete-model"] input')
      await input.setValue('alias-concrete')
      await vi.advanceTimersByTimeAsync(300)
      await flushPromises()
      expect(resolveTestModel).toHaveBeenCalledWith(accountId, 'alias-concrete', expect.any(AbortSignal))
      expect((wrapper.vm as any).selectedModelId).toBe('alias-concrete')
      expect((wrapper.vm as any).selectedModelTarget).toBe('grok-4.3')

      if (failed) {
        oldRequest.reject(new Error('old model request failed'))
      } else {
        oldRequest.resolve([{ id: 'grok-4', display_name: 'grok-4', upstream_model_id: 'grok-4' }])
      }
      await flushPromises()

      expect((input.element as HTMLInputElement).value).toBe('alias-concrete')
      expect((wrapper.vm as any).selectedModelId).toBe('alias-concrete')
      expect((wrapper.vm as any).selectedModelTarget).toBe('grok-4.3')

      const startButton = wrapper.findAll('button').find((button) => button.text().includes('admin.accounts.startTest'))
      expect(startButton).toBeTruthy()
      await startButton!.trigger('click')
      await flushPromises()
      expect(global.fetch).toHaveBeenCalledTimes(1)
      const [url, request] = vi.mocked(global.fetch).mock.calls[0]
      expect(String(url)).toContain(`/admin/accounts/${accountId}/test`)
      expect(JSON.parse(request!.body as string)).toMatchObject({ model_id: 'alias-concrete', mode: 'text' })
    } finally {
      wrapper.unmount()
    }
  })
})
