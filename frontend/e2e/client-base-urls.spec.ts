import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixturePublicSettings } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`renders protocol-specific client base URLs at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'user')
    await installOperatorApiMock(page, 'user')
    const group = { id: 11, name: 'OpenAI Fixture', platform: 'openai', status: 'active', allow_messages_dispatch: true }
    await page.route('**/api/v1/settings/public*', route => route.fulfill({ json: { code: 0, data: {
      ...operatorFixturePublicSettings, api_base_url: 'https://gateway.example.test/v1/'
    } } }))
    await page.route('**/api/v1/groups/available*', route => route.fulfill({ json: { code: 0, data: [group] } }))
    await page.route('**/api/v1/groups/rates*', route => route.fulfill({ json: { code: 0, data: {} } }))
    await page.route('**/api/v1/usage/dashboard/api-keys*', route => route.fulfill({ json: { code: 0, data: { stats: {} } } }))
    await page.route('**/api/v1/keys?**', route => route.fulfill({ json: { code: 0, data: {
      items: [{ id: 1, name: 'Fixture key', key: 'sk-fixture', user_id: 2, group_id: 11, group,
        status: 'active', quota: 0, quota_used: 0, ip_whitelist: [], ip_blacklist: [],
        created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z', expires_at: null }],
      total: 1, page: 1, page_size: 10, pages: 1
    } } }))
    await page.goto('/keys')
    await page.getByRole('button', { name: 'Use Key', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: 'Use API Key' })
    await expect(dialog.locator('pre code').filter({ hasText: 'model_provider = "OpenAI"' })).toContainText('base_url = "https://gateway.example.test/v1"')
    await dialog.getByRole('button', { name: 'Codex CLI (WebSocket)', exact: true }).click()
    await expect(dialog.locator('pre code').filter({ hasText: 'model_provider = "OpenAI"' })).toContainText('base_url = "https://gateway.example.test/v1"')
    await dialog.getByRole('button', { name: 'Claude Code', exact: true }).click()
    await expect(dialog.locator('pre code').filter({ hasText: 'ANTHROPIC_BASE_URL' }).first()).toContainText('ANTHROPIC_BASE_URL="https://gateway.example.test"')
    await page.screenshot({ path: testInfo.outputPath(`client-base-urls-${width}.png`) })
    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
  })
}
