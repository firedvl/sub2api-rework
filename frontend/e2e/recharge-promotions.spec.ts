import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { getOperatorFixtureData, operatorFixturePublicSettings } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`recharge discount and bonus quotes, limits, and recovery at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'user')
    await installOperatorApiMock(page, 'user')
    await page.route('**/api/v1/settings/public**', route => route.fulfill({ json: { code: 0, data: { ...operatorFixturePublicSettings, payment_enabled: true } } }))
    await page.route('**/purchase**', async route => {
      if (!route.request().isNavigationRequest()) return route.fallback()
      const response = await route.fetch()
      await route.fulfill({ response, body: (await response.text()).replace('"payment_enabled":false', '"payment_enabled":true') })
    })
    let mode = 'discount'
    await page.route('**/api/v1/payment/checkout-info**', route => route.fulfill({ json: { code: 0, data: {
      methods: { alipay: { currency: 'CNY', single_min: 1, single_max: 90, available: true } },
      global_min: 1, global_max: 90, input_min: 1, input_max: 1000, plans: [], balance_disabled: false,
      balance_recharge_multiplier: 0.14, subscription_usd_to_cny_rate: 0, recharge_fee_rate: 2.5,
      recharge_bonus_mode: mode, recharge_bonus_tiers: [{ min_amount: 100, bonus_percent: 20 }],
      recharge_bonus_notice: '**Promotion**<script>window.unsafePromotion=true</script>', help_text: '', help_image_url: '',
    } } }))
    const orders: unknown[] = []
    await page.route('**/api/v1/payment/orders', route => {
      orders.push(route.request().postDataJSON())
      return route.fulfill({ status: 503, json: { code: 503, message: 'Synthetic provider unavailable' } })
    })
    await page.goto('/purchase')
    const amount = page.locator('input[inputmode="decimal"]')
    await amount.fill('100')
    await expect(page.getByTestId('quick-amount-100')).toBeVisible()
    await expect(page.getByTestId('recharge-discount-row')).toContainText('20')
    await expect(page.getByTestId('recharge-credited-row')).toContainText('$14.00')
    await expect(page.getByTestId('recharge-bonus-notice').locator('script')).toHaveCount(0)
    expect(await page.evaluate(() => 'unsafePromotion' in window)).toBe(false)
    const pay = page.getByRole('button', { name: /Confirm Payment|确认支付/ })
    await expect(pay).toBeEnabled()
    await pay.click()
    await expect(pay).toBeEnabled()
    expect(orders).toEqual([expect.objectContaining({ amount: 100, order_type: 'balance' })])
    await amount.fill('200')
    await expect(pay).toBeDisabled()
    mode = 'bonus'
    await page.reload()
    await amount.fill('100')
    await expect(page.getByTestId('recharge-bonus-row')).toContainText('$2.80')
    await expect(page.getByTestId('recharge-credited-row')).toContainText('$16.80')
    await expect(pay).toBeDisabled()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    await page.screenshot({ path: testInfo.outputPath(`recharge-${width}.png`), fullPage: true })
  })

  test(`admin promotion editor rejects duplicate and invalid discount at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'admin')
    await installOperatorApiMock(page, 'admin')
    const settings = { ...getOperatorFixtureData('/api/v1/admin/settings', 'admin') as object,
      payment_enabled: true, payment_recharge_bonus_mode: 'bonus',
      payment_recharge_bonus_tiers: [{ min_amount: 100, bonus_percent: 20 }], payment_recharge_bonus_notice: '' }
    const saves: unknown[] = []
    await page.route(/\/api\/v1\/admin\/settings(?:\?.*)?$/, route => {
      if (route.request().method() === 'GET') return route.fulfill({ json: { code: 0, data: settings } })
      saves.push(route.request().postDataJSON())
      return route.fulfill({ json: { code: 0, data: { ...settings, ...route.request().postDataJSON() } } })
    })
    await page.goto('/admin/settings')
    await page.locator('#settings-tab-payment').click()
    await page.getByTestId('recharge-bonus-tier-add').click()
    const row = page.getByTestId('recharge-bonus-tier-row').last()
    await row.getByTestId('recharge-bonus-tier-min-input').fill('100')
    await row.getByTestId('recharge-bonus-tier-percent-input').fill('25')
    await expect(row.getByTestId('recharge-bonus-tier-error')).toBeVisible()
    await page.getByTestId('settings-floating-save-button').click()
    expect(saves).toHaveLength(0)
    await row.getByTestId('recharge-bonus-tier-min-input').fill('500')
    await page.getByTestId('recharge-bonus-mode-discount').click()
    await row.getByTestId('recharge-bonus-tier-percent-input').fill('100')
    await expect(row.getByTestId('recharge-bonus-tier-error')).toBeVisible()
    await page.getByTestId('settings-floating-save-button').click()
    expect(saves).toHaveLength(0)
    await row.getByTestId('recharge-bonus-tier-percent-input').fill('25')
    await page.getByTestId('recharge-bonus-notice-input').fill('**Promotion**')
    await page.getByTestId('settings-floating-save-button').click()
    await expect.poll(() => saves.length).toBe(1)
    expect(saves[0]).toEqual(expect.objectContaining({ payment_recharge_bonus_mode: 'discount', payment_recharge_bonus_tiers: [{ min_amount: 100, bonus_percent: 20 }, { min_amount: 500, bonus_percent: 25 }] }))
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    await expect(page.getByText('Discount percent must be below 100', { exact: true })).toHaveCount(0, { timeout: 15000 })
    await page.getByTestId('recharge-bonus-tier-editor').screenshot({ path: testInfo.outputPath(`admin-recharge-${width}.png`) })
  })
}

test('decimal ties and backend currency precision govern rendered quotes and provider eligibility', async ({ page }) => {
  await seedSession(page, 'user')
  await installOperatorApiMock(page, 'user')
  await page.route('**/api/v1/settings/public**', route => route.fulfill({ json: { code: 0, data: { ...operatorFixturePublicSettings, payment_enabled: true } } }))
  let currency = 'USD'
  let mode = 'discount'
  let fee = 0
  let usdMax = 2.13
  await page.route('**/api/v1/payment/checkout-info**', route => route.fulfill({ json: { code: 0, data: {
    methods: { alipay: { currency, single_min: 0, single_max: currency === 'USD' ? usdMax : 85.849, available: true } },
    global_min: 0, global_max: currency === 'USD' ? usdMax : 85.849, input_min: 0, input_max: 1000,
    plans: [], balance_disabled: false, balance_recharge_multiplier: 1, recharge_fee_rate: fee,
    recharge_bonus_mode: mode, recharge_bonus_tiers: [{ min_amount: 1, bonus_percent: currency === 'USD' ? 50 : 15 }],
    recharge_bonus_notice: '', help_text: '', help_image_url: '',
  } } }))
  await page.goto('/purchase')
  const amount = page.locator('input[inputmode="decimal"]')
  const pay = page.getByRole('button', { name: /Confirm Payment|确认支付/ })
  await amount.fill('4.27')
  await expect(pay).toContainText('2.14')
  await expect(pay).toBeDisabled()
  fee = 5
  usdMax = 1.47
  await page.reload()
  await amount.fill('2.80')
  await expect(pay).toContainText('1.47')
  await expect(pay).toBeEnabled()
  usdMax = 1.469
  await page.reload()
  await amount.fill('2.80')
  await expect(pay).toContainText('1.47')
  await expect(pay).toBeDisabled()
  fee = 0
  mode = 'bonus'
  await page.reload()
  await amount.fill('4.27')
  await expect(page.getByTestId('recharge-bonus-row')).toContainText('$2.14')
  await expect(page.getByTestId('recharge-credited-row')).toContainText('$6.41')
  currency = 'IQD'
  mode = 'discount'
  await page.reload()
  await amount.fill('101')
  await expect(pay).toContainText('85.850')
  await expect(pay).toBeDisabled()
})
