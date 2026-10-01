import { describe, expect, it, vi } from 'vitest'
import { PURCHASE_ROUTE_NAME, resolveRouteDocumentTitle, resolveRouteMetaKeys } from '../title'

vi.mock('@/i18n', () => ({ i18n: { global: { t: (key: string) => ({ 'nav.recharge': 'Recharge', 'nav.subscribe': 'Subscription', 'nav.buySubscription': 'Recharge / Subscription' })[key] ?? key } } }))

describe('site billing titles', () => {
  const route = { name: PURCHASE_ROUTE_NAME, params: {}, meta: { title: 'Purchase', titleKey: 'nav.buySubscription', descriptionKey: 'purchase.description' } }
  it.each([
    ['recharge_only', 'nav.recharge', 'purchase.rechargeDescription', 'Recharge'],
    ['subscription_only', 'nav.subscribe', 'purchase.subscriptionDescription', 'Subscription'],
    ['recharge_and_subscription', 'nav.buySubscription', 'purchase.description', 'Recharge / Subscription'],
  ] as const)('uses %s labels without changing the rework brand', (billingMode, titleKey, descriptionKey, label) => {
    expect(resolveRouteMetaKeys(route, { billingMode })).toEqual({ titleKey, descriptionKey })
    expect(resolveRouteDocumentTitle(route, 'Sub2API', [], { billingMode })).toBe(`${label} · Gateway`)
  })
  it('does not change other routes or custom page titles', () => {
    expect(resolveRouteMetaKeys({ name: 'Usage', meta: { titleKey: 'usage.title' } }, { billingMode: 'recharge_only' })).toEqual({ titleKey: 'usage.title', descriptionKey: undefined })
    expect(resolveRouteDocumentTitle({ name: 'CustomPage', params: { id: 'custom' }, meta: {} }, 'Gateway', [{ id: 'custom', label: 'Custom title', url: '/custom', visibility: 'user', sort_order: 0 }], { billingMode: 'subscription_only' })).toBe('Custom title · Gateway')
  })
})
