import { describe, expect, it } from 'vitest'

import enCommon from '@/i18n/locales/en/common'
import zhCommon from '@/i18n/locales/zh/common'
import { extractI18nErrorMessage } from '@/utils/apiError'

type Messages = Record<string, unknown>

function createTranslator(messages: Messages) {
  return (key: string): string => {
    const value = key.split('.').reduce<unknown>((current, segment) => {
      if (!current || typeof current !== 'object') return undefined
      return (current as Messages)[segment]
    }, messages)

    return typeof value === 'string' ? value : key
  }
}

describe('extractI18nErrorMessage', () => {
  const invalidCredentialsError = {
    reason: 'INVALID_CREDENTIALS',
    message: 'invalid email or password',
  }

  it('localizes invalid credentials in Chinese without exposing the backend message', () => {
    const t = createTranslator(zhCommon)

    expect(
      extractI18nErrorMessage(invalidCredentialsError, t, 'auth.errors', '登录失败'),
    ).toBe('邮箱或密码错误')
  })

  it('localizes invalid credentials in English', () => {
    const t = createTranslator(enCommon)

    expect(
      extractI18nErrorMessage(invalidCredentialsError, t, 'auth.errors', 'Login failed'),
    ).toBe('Invalid email or password')
  })
})
