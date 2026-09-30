import { expect, test, type Route } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

const fulfill = (route: Route, data: unknown) => route.fulfill({ json: { code: 0, data } })

for (const width of [390, 1280]) {
  test(`registration promo visibility is settled before render at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await installOperatorApiMock(page)
    await page.route('**/api/v1/settings/public*', route => fulfill(route, {
      registration_enabled: true, promo_code_enabled: false,
      registration_email_suffix_whitelist: [], login_agreement_documents: [],
    }))
    await page.goto('/register')
    await expect(page.locator('#email')).toBeVisible()
    await expect(page.locator('#promo_code')).toHaveCount(0)
    await expect(page.locator('#promo_code')).toHaveCount(0)
    await page.addInitScript(() => {
      window.__APP_CONFIG__ = { registration_enabled: true, promo_code_enabled: true } as typeof window.__APP_CONFIG__
    })
    await page.route('**/api/v1/settings/public*', route => fulfill(route, {
      registration_enabled: true, promo_code_enabled: true,
      registration_email_suffix_whitelist: [], login_agreement_documents: [],
    }))
    await page.reload()
    await expect(page.locator('#promo_code')).toBeVisible()
    await page.screenshot({ path: `test-results/auth-promo-${width}.png`, fullPage: true })
  })

  test(`TOTP verification resets and retains displayed digits at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'user')
    await installOperatorApiMock(page, 'user')
    let submissions = 0
    await page.route('**/api/v1/user/totp/**', route => {
      const path = new URL(route.request().url()).pathname
      if (path.endsWith('/status')) return fulfill(route, { feature_enabled: true, enabled: false })
      if (path.endsWith('/verification-method')) return fulfill(route, { method: 'password' })
      if (path.endsWith('/setup')) return fulfill(route, {
        secret: 'JBSWY3DPEHPK3PXP', qr_code_url: 'otpauth://fixture', setup_token: 'fixture-token',
      })
      if (path.endsWith('/enable')) {
        submissions++
        expect(route.request().postDataJSON()).toEqual({ totp_code: '123456', setup_token: 'fixture-token' })
        return route.fulfill({ status: 400, json: { code: 400, message: 'Fixture invalid code' } })
      }
      return fulfill(route, {})
    })
    await page.goto('/profile')
    await page.getByRole('button', { name: 'Enable', exact: true }).click()
    const modal = page.locator('.fixed.inset-0.z-50').filter({ hasText: 'Set Up Two-Factor Authentication' })
    await modal.locator('input[type="password"]').fill('fixture-password')
    await modal.getByRole('button', { name: 'Next', exact: true }).click()
    await modal.getByRole('button', { name: 'Next', exact: true }).click()
    const digits = modal.locator('input[maxlength="1"]')
    for (let index = 0; index < 6; index++) await digits.nth(index).fill(String(index + 1))
    await modal.getByRole('button', { name: 'Back', exact: true }).click()
    await modal.getByRole('button', { name: 'Next', exact: true }).click()
    for (let index = 0; index < 6; index++) await expect(digits.nth(index)).toHaveValue(String(index + 1))
    await modal.getByRole('button', { name: 'Verify', exact: true }).click()
    await expect.poll(() => submissions).toBe(1)
    for (let index = 0; index < 6; index++) await expect(digits.nth(index)).toHaveValue('')
    await expect(digits.first()).toBeFocused()
    await expect(modal.getByRole('button', { name: 'Verify', exact: true })).toBeDisabled()
    await page.screenshot({ path: `test-results/auth-totp-${width}.png`, fullPage: true })
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  })
}
