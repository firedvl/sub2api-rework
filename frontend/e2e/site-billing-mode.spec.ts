import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { getOperatorFixtureData, operatorFixturePublicSettings } from './fixtures/operatorData'

const modes = [
  { name: 'combined', subscriptions: true, balanceDisabled: false, heading: 'Recharge / Subscription' },
  { name: 'recharge', subscriptions: false, balanceDisabled: false, heading: 'Recharge' },
  { name: 'subscription', subscriptions: true, balanceDisabled: true, heading: 'Subscription' },
  { name: 'unavailable', subscriptions: false, balanceDisabled: true, heading: 'Recharge' },
]

for (const width of [390, 1280]) {
  for (const mode of modes) {
    test(`${mode.name} site billing surfaces at ${width}px without placing orders`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 900 })
      await seedSession(page, 'user')
      await installOperatorApiMock(page, 'user')
      const pageErrors: string[] = []
      page.on('pageerror', error => pageErrors.push(error.message))
      let subscriptionReads = 0
      let mutations = 0
      await page.route('**/api/v1/payment/**', route => {
        if (route.request().method() !== 'GET') {
          mutations++
          return route.fulfill({ status: 405, json: { code: 405, message: 'Orders are forbidden in this fixture' } })
        }
        if (!new URL(route.request().url()).pathname.endsWith('/checkout-info')) return route.fallback()
        return route.fulfill({ json: { code: 0, data: {
          balance_disabled: mode.balanceDisabled, methods: {}, plans: [], min_amount: 1,
          max_amount: 1000, recharge_fee_rate: 0, balance_recharge_multiplier: 1,
        } } })
      })
      await page.route('**/api/v1/settings/public*', route => route.fulfill({ json: { code: 0, data: {
        ...operatorFixturePublicSettings, payment_enabled: true,
        subscription_enabled: mode.subscriptions, payment_balance_disabled: mode.balanceDisabled,
      } } }))
      await page.route('**/api/v1/subscriptions/active*', route => {
        subscriptionReads++
        return route.fulfill({ json: { code: 0, data: [] } })
      })
      await page.goto('/purchase?tab=subscription')
      await expect(page.getByRole('heading', { name: mode.heading, exact: true })).toBeVisible()
      await expect(page).toHaveTitle(`${mode.heading} · Gateway`)
      if (mode.name === 'combined') {
        await expect(page.getByRole('button', { name: 'Subscribe', exact: true })).toBeVisible()
      } else {
        await expect(page.getByRole('button', { name: 'Subscribe', exact: true })).toHaveCount(0)
      }
      if (mode.name === 'unavailable') {
        await expect(page.getByText('Neither top-up nor subscriptions are currently available. Please contact the administrator.')).toBeVisible()
      } else if (mode.name === 'subscription') {
        await expect(page.getByText('No subscription plans available', { exact: true })).toBeVisible()
      }
      if (!mode.subscriptions) {
        expect(subscriptionReads).toBe(0)
        await page.goto('/subscriptions')
        await expect(page).toHaveURL(/\/dashboard$/)
        expect(subscriptionReads).toBe(0)
      }
      expect(mutations).toBe(0)
      expect(pageErrors).toEqual([])
      await page.screenshot({ path: testInfo.outputPath(`site-${mode.name}-${width}.png`), animations: 'disabled' })
    })
  }
}

for (const width of [390, 1280]) {
  test(`admin billing-mode control saves both switches at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'admin')
    await installOperatorApiMock(page, 'admin')
    const writes: Record<string, unknown>[] = []
    const pageErrors: string[] = []
    page.on('pageerror', error => pageErrors.push(error.message))
    let settings = { ...getOperatorFixtureData('/api/v1/admin/settings') as Record<string, unknown>, subscription_enabled: true, payment_balance_disabled: false }
    await page.route(/\/api\/v1\/admin\/settings(?:\?.*)?$/, route => {
      if (route.request().method() === 'PUT') {
        const payload = route.request().postDataJSON() as typeof settings
        writes.push(payload)
        settings = { ...settings, ...payload }
      }
      return route.fulfill({ json: { code: 0, data: settings } })
    })
    await page.route('**/api/v1/admin/settings/web-search-emulation*', route => route.fulfill({ json: { code: 0, data: { enabled: false } } }))
    await page.goto('/admin/settings')
    await page.getByRole('tab', { name: 'Feature Switches', exact: true }).click()
    const mode = page.getByTestId('site-billing-mode')
    await mode.click()
    await page.getByRole('option', { name: 'Subscription only', exact: true }).click()
    await expect(mode).toContainText('Subscription only')
    await page.screenshot({ path: testInfo.outputPath(`site-selector-${width}.png`), animations: 'disabled' })
    await page.getByTestId('settings-floating-save-button').click()
    await expect.poll(() => writes.length).toBe(1)
    expect(writes[0]).toMatchObject({ subscription_enabled: true, payment_balance_disabled: true })
    expect(pageErrors).toEqual([])
  })
}
