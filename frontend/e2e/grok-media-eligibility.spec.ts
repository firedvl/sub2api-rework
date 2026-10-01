import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`Grok eligibility retries and saves one atomic account update at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'admin', 'simple')
    await installOperatorApiMock(page, 'admin', 'simple')
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    let account = { ...operatorFixtureAccounts[0], id: 501, name: 'Grok Eligibility Fixture', platform: 'grok',
      type: 'oauth', credentials: {}, credentials_status: { has_access_token: true, has_refresh_token: true },
      extra: { unrelated_setting: 'retain' }, group_ids: [], groups: [], ollama_cloud_usage: undefined,
      opencode_go_usage: undefined }
    await page.route('**/api/v1/admin/accounts?*', route => route.fulfill({ json: { code: 0, data: {
      items: [account], total: 1, page: 1, page_size: 20, pages: 1
    } } }))
    let reads = 0
    await page.route('**/api/v1/admin/accounts/501/grok-media-eligibility*', route => {
      expect(route.request().method()).toBe('GET')
      reads++
      if (reads === 1) return route.fulfill({ status: 503, json: { code: 503, message: 'Eligibility unavailable' } })
      return route.fulfill({ json: { code: 0, data: { account_id: 501, mode: 'auto', eligible: true, reason: 'billing_inconclusive' } } })
    })
    const saves: Record<string, unknown>[] = []
    await page.route(/\/api\/v1\/admin\/accounts\/501(?:\?.*)?$/, route => {
      if (route.request().method() !== 'PUT') return route.fulfill({ json: { code: 0, data: account } })
      const payload = route.request().postDataJSON() as Record<string, unknown>
      saves.push(payload)
      if (saves.length === 1) return route.fulfill({ status: 503, json: { code: 503, message: 'Atomic account update unavailable' } })
      account = { ...account, extra: payload.extra as typeof account.extra }
      return route.fulfill({ json: { code: 0, data: account } })
    })
    await page.goto('/admin/accounts')
    await page.getByRole('button', { name: 'Edit Grok Eligibility Fixture', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: 'Edit Account', exact: true })
    const panel = dialog.getByTestId('grok-media-eligibility-card')
    await expect(panel).toContainText('Eligibility unavailable')
    await expect(panel.getByRole('button', { name: 'Media Generation Eligibility', exact: true })).toBeDisabled()
    await panel.getByRole('button', { name: 'Refresh', exact: true }).click()
    await expect(panel.getByTestId('grok-media-eligibility-status')).toContainText('Billing information inconclusive')
    await panel.getByRole('button', { name: 'Media Generation Eligibility', exact: true }).click()
    await page.getByRole('option', { name: 'Force disable', exact: true }).click()
    await dialog.getByRole('button', { name: 'Update', exact: true }).click()
    await expect(page.getByText('Atomic account update unavailable', { exact: true })).toBeVisible()
    await expect(dialog).toBeVisible()
    expect(saves).toHaveLength(1)
    expect(saves[0].extra).toMatchObject({ unrelated_setting: 'retain', grok_media_eligible: false })
    await page.keyboard.press('Tab')
    expect(await dialog.evaluate(element => element.contains(document.activeElement))).toBe(true)
    await panel.scrollIntoViewIfNeeded()
    await page.screenshot({ path: testInfo.outputPath(`grok-eligibility-${width}.png`), fullPage: true, animations: 'disabled' })
    await dialog.getByRole('button', { name: 'Update', exact: true }).click()
    await expect(dialog).not.toBeVisible()
    expect(saves).toHaveLength(2)
    expect(saves[1].extra).toMatchObject({ unrelated_setting: 'retain', grok_media_eligible: false })
    expect(reads).toBe(2)
    expect(errors).toEqual([])
  })
}
