import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { getOperatorFixtureData } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`Claude version controls preserve synced state at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 844 })
    await seedSession(page)
    await installOperatorApiMock(page)
    const base = getOperatorFixtureData('/api/v1/admin/settings')
    if (!base || typeof base !== 'object' || Array.isArray(base)) throw new Error('invalid settings fixture')
    let settings: Record<string, unknown> = {
      ...base,
      claude_code_client_version: '2.1.280',
      claude_code_client_version_synced: '2.1.281',
      claude_code_version_auto_sync_enabled: false,
    }
    const writes: Record<string, unknown>[] = []
    await page.route(/\/api\/v1\/admin\/settings(?:\?.*)?$/, async (route) => {
      if (route.request().method() === 'PUT') {
        const payload = route.request().postDataJSON()
        expect(payload).not.toHaveProperty('claude_code_client_version_synced')
        writes.push(payload)
        settings = { ...settings, ...payload }
      }
      await route.fulfill({ json: { code: 0, message: 'ok', data: settings } })
    })
    await page.route(/\/api\/v1\/admin\/settings\/web-search-emulation(?:\?.*)?$/, (route) => route.fulfill({
      json: { code: 0, message: 'ok', data: { enabled: false, providers: [] } },
    }))
    await page.goto('/admin/settings?tab=gateway')
    const version = page.getByRole('textbox', { name: 'Claude Code client version', exact: true })
    const autoSync = page.getByRole('switch', { name: 'Auto-sync Claude Code version', exact: true })
    await expect(version).toHaveValue('2.1.280')
    await expect(autoSync).toHaveAttribute('aria-checked', 'false')
    await expect(page.getByText('Currently synced: 2.1.281', { exact: true })).toBeVisible()
    await version.fill(' 2.1.300 ')
    await autoSync.focus()
    await page.keyboard.press('Space')
    await expect(autoSync).toHaveAttribute('aria-checked', 'true')
    await page.getByTestId('settings-floating-save-button').click()
    await expect.poll(() => writes.length).toBe(1)
    expect(writes[0].claude_code_client_version).toBe('2.1.300')
    expect(writes[0].claude_code_version_auto_sync_enabled).toBe(true)
    await expect(page.getByText('Settings saved successfully', { exact: true })).toBeVisible()
    await version.fill('')
    await autoSync.click()
    await page.getByTestId('settings-floating-save-button').click()
    await expect.poll(() => writes.length).toBe(2)
    expect(writes[1].claude_code_client_version).toBe('')
    expect(writes[1].claude_code_version_auto_sync_enabled).toBe(false)
    await expect(page.getByTestId('settings-floating-save-button')).toBeHidden()
    await expect(page.getByText('Currently synced: 2.1.281', { exact: true })).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
    await autoSync.scrollIntoViewIfNeeded()
    await page.screenshot({ path: testInfo.outputPath(`claude-version-${width}.png`) })
  })
}
