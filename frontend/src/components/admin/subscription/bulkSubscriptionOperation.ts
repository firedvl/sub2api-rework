import type { SubscriptionBulkActionRequest } from '@/api/admin/subscriptions'

export interface BulkSubscriptionOperation {
  request: SubscriptionBulkActionRequest
  key: string
  storageKey: string
  outcomeUncertain: boolean
}

export function prepareBulkSubscriptionOperation(input: SubscriptionBulkActionRequest): BulkSubscriptionOperation {
  const request: SubscriptionBulkActionRequest = {
    subscription_ids: [...new Set(input.subscription_ids)].sort((a, b) => a - b),
    action: input.action
  }
  if (input.action === 'extend') request.days = input.days
  if (input.action === 'reset_quota') {
    request.daily = !!input.daily
    request.weekly = !!input.weekly
    request.monthly = !!input.monthly
  }
  const user = JSON.parse(localStorage.getItem('auth_user') ?? 'null') as { id?: unknown } | null
  const adminId = user?.id
  if (typeof adminId !== 'number' || !Number.isSafeInteger(adminId) || adminId <= 0) {
    throw new Error('Administrator identity is unavailable')
  }
  const storageKey = `sub2api:admin:subscription-bulk:${adminId}:${JSON.stringify(request)}`
  const storedKey = sessionStorage.getItem(storageKey)
  const key = storedKey || `subscription-bulk-${adminId}-${globalThis.crypto.randomUUID()}`
  sessionStorage.setItem(storageKey, key)
  return { request, key, storageKey, outcomeUncertain: !!storedKey }
}

export function completeBulkSubscriptionOperation(operation: BulkSubscriptionOperation) {
  sessionStorage.removeItem(operation.storageKey)
}
