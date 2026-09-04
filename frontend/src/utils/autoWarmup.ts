import type { Account } from '@/types'

export function isOpenAIAutoWarmupConfigurable(account: Pick<Account, 'platform' | 'type' | 'parent_account_id'>): boolean {
  return account.platform === 'openai' && account.type === 'oauth' && account.parent_account_id == null
}
