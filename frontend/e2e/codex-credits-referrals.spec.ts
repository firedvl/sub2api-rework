import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`Codex points and fixture-only referral outcomes at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'admin')
    await installOperatorApiMock(page, 'admin')
    const pageErrors: string[] = []
    page.on('pageerror', error => pageErrors.push(error.message))
    const account = { ...operatorFixtureAccounts[0], name: width < 768 ? 'W'.repeat(80) : operatorFixtureAccounts[0].name, credentials: { plan_type: 'plus' } }
    const balance = '12345678901234567890.0123'
    let sends = 0
    let resets = 0
    let releaseSend!: () => void
    const sendPending = new Promise<void>(resolve => { releaseSend = resolve })
    const eligibility = {
      should_show: true, available_invites: 2, remaining_send_capacity: 2,
      remaining_reward_capacity: 2, requires_explicit_confirmation: true,
      program_id: 'codex_referral_consumer', fetched_at: 1770000000,
      title: 'Fixture invitation offer', rules: ['Fixture offer rule', 'R'.repeat(120)],
    }
    await page.route('**/api/v1/admin/accounts*', route => {
      if (new URL(route.request().url()).pathname !== '/api/v1/admin/accounts') return route.fallback()
      return route.fulfill({ json: { code: 0, data: { items: [account], total: 1, page: 1, page_size: 50, pages: 1 } } })
    })
    await page.route('**/api/v1/admin/openai/accounts/101/**', async route => {
      const path = new URL(route.request().url()).pathname
      if (path.endsWith('/quota/refresh')) {
        return route.fulfill({ json: { code: 0, data: {
          credits: { has_credits: true, unlimited: false, balance }, fetched_at: 1770000000,
          cache_persisted: false, credits_cache_persisted: false,
        } } })
      }
      if (path.endsWith('/reset-quota')) {
        resets++
        return route.fulfill({ status: 409, json: { code: 409, message: 'Reset is forbidden in this fixture' } })
      }
      if (path.endsWith('/referrals/refresh')) {
        return route.fulfill({ json: { code: 0, data: { eligibility, cache_persisted: true } } })
      }
      if (path.endsWith('/referrals/invite')) {
        sends++
        expect(route.request().postDataJSON()).toEqual({ email: 'friend@example.com', program_id: eligibility.program_id, confirmed: true })
        if (sends > 1) return route.abort('failed')
        await sendPending
        return route.fulfill({ json: { code: 0, data: {
          sent: true, email: 'friend@example.com', eligibility: null, cache_persisted: true, refresh_failed: true,
        } } })
      }
      return route.fallback()
    })
    await page.goto('/admin/accounts?view=technical')
    const points = page.getByTestId('codex-credits').first()
    await points.click()
    await expect(points).toContainText(balance)
    await expect(page.getByText('Showing live points, but the cache could not be saved. Query again.')).toBeVisible()
    await page.getByTestId('referral-open').first().click()
    const dialog = page.getByRole('dialog', { name: 'Invite user', exact: true })
    await expect(dialog.getByText('Fixture invitation offer')).toBeVisible()
    expect(await dialog.locator('form').evaluate(form => form.scrollWidth <= form.clientWidth)).toBe(true)
    await page.screenshot({ path: testInfo.outputPath(`referral-long-content-${width}.png`), animations: 'disabled' })
    const email = dialog.getByTestId('referral-email')
    const consent = dialog.getByTestId('referral-consent')
    const submit = dialog.getByTestId('referral-send')
    await email.fill('friend@example.com')
    await expect(submit).toBeDisabled()
    await consent.check()
    await email.press('Enter')
    await expect.poll(() => sends).toBe(1)
    await expect(submit).toBeDisabled()
    await page.keyboard.press('Escape')
    await expect(dialog).toBeVisible()
    releaseSend()
    await expect(dialog.getByRole('status').filter({ hasText: 'Invitation sent to friend@example.com' })).toBeVisible()
    await expect(dialog.getByText('The invitation was sent, but remaining capacity could not be refreshed. Query again.')).toBeVisible()
    await page.screenshot({ path: testInfo.outputPath(`referral-success-warning-${width}.png`), animations: 'disabled' })
    await dialog.getByRole('button', { name: 'Close', exact: true }).last().click()
    await page.getByTestId('referral-open').first().click()
    await expect(dialog.getByText('Fixture invitation offer')).toBeVisible()
    await email.fill('friend@example.com')
    await consent.check()
    await submit.click()
    await expect(dialog.getByRole('alert')).toHaveText('The invitation outcome is unknown. Check its status in Codex before deciding whether to retry.')
    await expect(submit).toBeDisabled()
    expect(sends).toBe(2)
    expect(resets).toBe(0)
    expect(pageErrors).toEqual([])
    const bounds = await dialog.boundingBox()
    expect(bounds).not.toBeNull()
    expect(bounds!.x).toBeGreaterThanOrEqual(0)
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(width)
    await page.screenshot({ path: testInfo.outputPath(`referral-unknown-${width}.png`), animations: 'disabled' })
  })
}
