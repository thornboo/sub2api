import type { Account } from '@/types'

type ModelProtocolCapabilityAccount = Pick<Account, 'platform' | 'type'>

export function supportsModelProtocolCapabilities(
  account: ModelProtocolCapabilityAccount | null | undefined
): boolean {
  return account?.platform === 'openai' && account.type === 'apikey'
}
