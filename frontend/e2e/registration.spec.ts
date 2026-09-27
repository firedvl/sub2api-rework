import { expect, test } from '@playwright/test'

for (const width of [390, 1280]) {
  test(`registration confirmation and visibility at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    let enabled = true
    let submissions = 0
    await page.route('**/setup/status*', route => route.fulfill({
      json: { data: { needs_setup: false } },
    }))
    await page.route('**/api/v1/**', async route => {
      const path = new URL(route.request().url()).pathname
      if (path.endsWith('/settings/public')) {
        return route.fulfill({ json: { code: 0, data: {
          registration_enabled: enabled,
          email_verify_enabled: true,
          registration_email_suffix_whitelist: [],
          site_name: 'Gateway',
          login_agreement_documents: [],
        } } })
      }
      if (path.endsWith('/auth/register')) submissions++
      return route.fulfill({ json: { code: 0, data: {} } })
    })

    await page.goto('/register')
    await expect(page.locator('#confirmPassword')).toBeEnabled()
    await page.locator('#email').fill('fixture@example.com')
    await page.locator('#password').fill('fixture-password')
    await page.locator('#confirmPassword').fill('mismatch')
    await page.locator('button[type="submit"]').click()
    await expect(page.locator('#confirmPassword')).toHaveClass(/input-error/)
    expect(await page.evaluate(() => sessionStorage.getItem('register_data'))).toBeNull()
    await page.locator('#confirmPassword').fill('fixture-password')
    const toggle = page.locator('#confirmPassword').locator('..').locator('button')
    await toggle.click()
    await expect(page.locator('#confirmPassword')).toHaveAttribute('type', 'text')
    await toggle.click()
    await expect(page.locator('#confirmPassword')).toHaveAttribute('type', 'password')
    await page.screenshot({ path: `test-results/registration-${width}.png`, fullPage: true })
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
    await page.locator('button[type="submit"]').click()
    await expect(page).toHaveURL(/\/email-verify/)
    const saved = await page.evaluate(() => JSON.parse(sessionStorage.getItem('register_data') || '{}'))
    expect(saved).toEqual({ email: 'fixture@example.com', password: 'fixture-password' })
    expect(submissions).toBe(0)

    await page.goto('/login')
    await expect(page.locator('a[href="/register"]')).toBeVisible()
    enabled = false
    await page.reload()
    await expect(page.locator('#email')).toBeEnabled()
    await expect(page.locator('a[href="/register"]')).toHaveCount(0)
  })
}
