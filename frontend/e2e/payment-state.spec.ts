import { expect, test, type Page } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixturePublicSettings } from './fixtures/operatorData'

async function openPurchase(page: Page, renewal = false) {
  await seedSession(page, 'user')
  await installOperatorApiMock(page, 'user')
  await page.route('**/api/v1/settings/public**', route => route.fulfill({
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, data: { ...operatorFixturePublicSettings, payment_enabled: true } }),
  }))
  await page.route('**/purchase**', async route => {
    if (!route.request().isNavigationRequest()) return route.fallback()
    const response = await route.fetch()
    await route.fulfill({ response, body: (await response.text()).replace('"payment_enabled":false', '"payment_enabled":true') })
  })
  await page.route('**/api/v1/subscriptions/active**', route => route.fulfill({
    contentType: 'application/json', body: JSON.stringify({ code: 0, data: [] }),
  }))
  await page.route('**/api/v1/payment/checkout-info**', route => route.fulfill({
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, data: {
      methods: { wxpay: { daily_limit: 0, single_min: 0, single_max: 0, fee_rate: 0, available: true } },
      global_min: 0, global_max: 0, balance_disabled: false, balance_recharge_multiplier: 1,
      subscription_usd_to_cny_rate: 0, recharge_fee_rate: 0, stripe_publishable_key: '',
      help_text: '## Recharge help\n\n**Read first**\n\n[Support](https://example.test/help)\n\n<img src="x" onerror="window.unsafeHelp=true">',
      help_image_url: '',
      plans: renewal ? Array.from({ length: 12 }, (_, index) => ({
        id: index + 1, group_id: 123, name: `Renewal plan ${index + 1}`, price: 10,
        currency: 'USD', validity_days: 30, group_platform: 'openai', rate_multiplier: 1, features: [],
      })) : [],
    } }),
  }))
  await page.goto(renewal ? '/purchase?tab=subscription&group=123' : '/purchase')
}

test('amount rejection restores text and payment help is sanitized Markdown', async ({ page }) => {
  const writes: string[] = []
  page.on('request', request => {
    if (request.url().includes('/api/v1/payment/') && request.method() !== 'GET') writes.push(request.url())
  })
  await openPurchase(page)
  await expect(page.locator('.markdown-body h2')).toHaveText('Recharge help')
  await expect(page.locator('.markdown-body strong')).toHaveText('Read first')
  await expect(page.locator('.markdown-body a')).toHaveAttribute('href', 'https://example.test/help')
  await expect(page.locator('.markdown-body img')).not.toHaveAttribute('onerror')
  const amount = page.locator('input[inputmode="decimal"]')
  await amount.fill('12.55')
  await amount.press('a')
  await expect(amount).toHaveValue('12.55')
  await amount.press('5')
  await expect(amount).toHaveValue('12.55')
  await amount.fill('')
  await expect(amount).toHaveValue('')
  expect(writes).toEqual([])
})

test('renewal plans scroll within a narrow viewport and selecting one closes the modal', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 650 })
  await openPurchase(page, true)
  const modal = page.locator('.fixed.inset-0.z-50').filter({ has: page.getByText('Renewal plan 12', { exact: true }) })
  await expect(modal).toBeVisible()
  const content = modal.locator('.overflow-y-auto')
  expect(await content.evaluate(element => element.scrollHeight > element.clientHeight)).toBe(true)
  const panel = await modal.locator('.max-w-lg').boundingBox()
  expect(panel).not.toBeNull()
  expect(panel!.y).toBeGreaterThanOrEqual(0)
  expect(panel!.y + panel!.height).toBeLessThanOrEqual(650)
  await content.evaluate(element => { element.scrollTop = element.scrollHeight })
  const lastPlan = content.locator('.group').filter({ has: page.getByText('Renewal plan 12', { exact: true }) })
  await lastPlan.getByRole('button').click()
  await expect(modal).not.toBeVisible()
  await expect(page.getByText('Renewal plan 12', { exact: true })).toBeVisible()
})
