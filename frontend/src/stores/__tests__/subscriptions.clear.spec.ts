import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useSubscriptionStore } from '../subscriptions'
import type { UserSubscription } from '@/types'

const getActive = vi.hoisted(() => vi.fn())
vi.mock('@/api/subscriptions', () => ({ default: { getActiveSubscriptions: getActive } }))

function deferred() {
  let resolve!: (value: UserSubscription[]) => void
  let reject!: (error: Error) => void
  const promise = new Promise<UserSubscription[]>((accept, fail) => { resolve = accept; reject = fail })
  return { promise, resolve, reject }
}

beforeEach(() => {
  setActivePinia(createPinia())
  getActive.mockReset()
  vi.spyOn(console, 'error').mockImplementation(() => {})
})
afterEach(() => vi.restoreAllMocks())

describe('clearing in-flight subscriptions', () => {
  it.each(['resolve', 'reject'] as const)('resets loading without letting the old request %s overwrite new state', async (outcome) => {
    const previous = [{ id: 1 }] as UserSubscription[]
    const replacement = [{ id: 2 }] as UserSubscription[]
    const pending = deferred()
    const next = deferred()
    getActive.mockReturnValueOnce(pending.promise).mockReturnValueOnce(next.promise)
    const store = useSubscriptionStore()
    const oldRequest = store.fetchActiveSubscriptions().catch(() => undefined)
    expect(store.loading).toBe(true)
    store.clear()
    expect(store.loading).toBe(false)
    expect(store.activeSubscriptions).toEqual([])
    const newRequest = store.fetchActiveSubscriptions()
    expect(store.loading).toBe(true)
    if (outcome === 'resolve') pending.resolve(previous)
    else pending.reject(new Error('old request failed'))
    await oldRequest
    expect(store.loading).toBe(true)
    expect(store.activeSubscriptions).toEqual([])
    next.resolve(replacement)
    await newRequest
    expect(store.loading).toBe(false)
    expect(store.activeSubscriptions).toEqual(replacement)
    expect(getActive).toHaveBeenCalledTimes(2)
  })
})
