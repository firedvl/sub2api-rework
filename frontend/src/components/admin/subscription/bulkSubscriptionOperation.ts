import type { SubscriptionBulkActionRequest } from '@/api/admin/subscriptions'

export interface BulkSubscriptionOperation {
  request: SubscriptionBulkActionRequest
  key: string
  storageKey: string
  outcomeUncertain: boolean
}

function canonicalBulkSubscriptionRequest(input: SubscriptionBulkActionRequest): SubscriptionBulkActionRequest {
  if (!input || !['extend', 'reset_quota', 'revoke', 'restore'].includes(input.action) ||
    !Array.isArray(input.subscription_ids) || input.subscription_ids.length === 0 || input.subscription_ids.length > 100 ||
    input.subscription_ids.some(id => !Number.isSafeInteger(id) || id <= 0)) {
    throw new Error('Invalid saved subscription action')
  }
  const request: SubscriptionBulkActionRequest = {
    subscription_ids: [...new Set(input.subscription_ids)].sort((a, b) => a - b),
    action: input.action
  }
  if (input.action === 'extend') {
    if (typeof input.days !== 'number' || !Number.isInteger(input.days) || input.days === 0 || Math.abs(input.days) > 36500) {
      throw new Error('Invalid saved subscription adjustment')
    }
    request.days = input.days
  }
  if (input.action === 'reset_quota') {
    for (const window of ['daily', 'weekly', 'monthly'] as const) {
      if (input[window] !== undefined && typeof input[window] !== 'boolean') throw new Error('Invalid saved subscription quota windows')
    }
    request.daily = !!input.daily
    request.weekly = !!input.weekly
    request.monthly = !!input.monthly
    if (!request.daily && !request.weekly && !request.monthly) throw new Error('Invalid saved subscription quota windows')
  }
  return request
}

function bulkSubscriptionStorageScope(): { prefix: string; adminId: number } {
  const user = JSON.parse(localStorage.getItem('auth_user') ?? 'null') as { id?: unknown } | null
  const adminId = user?.id
  if (typeof adminId !== 'number' || !Number.isSafeInteger(adminId) || adminId <= 0) {
    throw new Error('Administrator identity is unavailable')
  }
  return { prefix: `sub2api:admin:subscription-bulk:${adminId}:`, adminId }
}

export function prepareBulkSubscriptionOperation(input: SubscriptionBulkActionRequest): BulkSubscriptionOperation {
  const request = canonicalBulkSubscriptionRequest(input)
  const { prefix, adminId } = bulkSubscriptionStorageScope()
  const storageKey = `${prefix}${JSON.stringify(request)}`
  const storedKey = sessionStorage.getItem(storageKey)
  const key = storedKey || `subscription-bulk-${adminId}-${globalThis.crypto.randomUUID()}`
  sessionStorage.setItem(storageKey, key)
  return { request, key, storageKey, outcomeUncertain: !!storedKey }
}

export function listPendingBulkSubscriptionOperations(): BulkSubscriptionOperation[] {
  const { prefix } = bulkSubscriptionStorageScope()
  const operations: BulkSubscriptionOperation[] = []
  for (let index = 0; index < sessionStorage.length; index++) {
    const storageKey = sessionStorage.key(index)
    if (!storageKey?.startsWith(prefix)) continue
    const serializedRequest = storageKey.slice(prefix.length)
    const request = canonicalBulkSubscriptionRequest(JSON.parse(serializedRequest))
    const key = sessionStorage.getItem(storageKey)
    if (!key || JSON.stringify(request) !== serializedRequest) throw new Error('Invalid saved subscription action')
    operations.push({ request, key, storageKey, outcomeUncertain: true })
  }
  return operations
}

export function completeBulkSubscriptionOperation(operation: BulkSubscriptionOperation) {
  sessionStorage.removeItem(operation.storageKey)
}
