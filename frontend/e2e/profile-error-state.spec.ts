import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

for (const width of [390, 1280]) {
  test(`profile failures preserve inputs and permit recovery at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'user')
    await installOperatorApiMock(page, 'user')
    await page.route('**/api/v1/user/totp/status*', route => route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ code: 0, data: { enabled: false, enabled_at: null, feature_enabled: true } }),
    }))
    await page.route('**/api/v1/user/totp/verification-method*', route => route.fulfill({
      contentType: 'application/json', body: JSON.stringify({ code: 0, data: { method: 'password' } }),
    }))
    for (const [path, message] of [
      ['/user', 'Profile update rejected'],
      ['/user/password', 'Current password rejected'],
      ['/user/totp/setup', 'TOTP verification rejected'],
    ]) {
      await page.route(`**/api/v1${path}`, route => {
        if (route.request().method() === 'GET') return route.fallback()
        return route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify({ code: 'INVALID_INPUT', message }) })
      })
    }
    await page.goto('/profile')
    await page.locator('#username').fill('Updated username')
    await page.getByRole('button', { name: 'Update Profile', exact: true }).click()
    await expect(page.getByText('Profile update rejected', { exact: true })).toBeVisible()
    await expect(page.locator('#username')).toHaveValue('Updated username')
    await expect(page.getByRole('button', { name: 'Update Profile', exact: true })).toBeEnabled()

    await page.locator('#old_password').fill('old-password')
    await page.locator('#new_password').fill('new-password')
    await page.locator('#confirm_password').fill('new-password')
    await page.getByRole('button', { name: 'Change Password', exact: true }).click()
    await expect(page.getByText('Current password rejected', { exact: true })).toBeVisible()
    await expect(page.locator('#new_password')).toHaveValue('new-password')
    await expect(page.getByRole('button', { name: 'Change Password', exact: true })).toBeEnabled()

    await page.getByRole('button', { name: 'Enable', exact: true }).click()
    const modal = page.locator('.fixed.inset-0.z-50').filter({ has: page.getByRole('heading', { name: 'Set Up Two-Factor Authentication' }) })
    await modal.locator('input[type="password"]').fill('old-password')
    await modal.getByRole('button', { name: 'Next', exact: true }).click()
    await expect(page.getByText('TOTP verification rejected', { exact: true })).toBeVisible()
    await expect(modal.locator('input[type="password"]')).toHaveValue('old-password')
    await expect(modal.getByRole('button', { name: 'Next', exact: true })).toBeEnabled()
    await page.screenshot({ path: `/Users/ryanlb/.codex/reconciliation/sub2api-v028/profile-errors-${width}.png`, animations: 'disabled' })
    await modal.getByRole('button', { name: 'Cancel', exact: true }).click()
    await expect(modal).not.toBeVisible()
  })
}
