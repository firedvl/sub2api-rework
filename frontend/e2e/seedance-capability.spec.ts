import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`Seedance capability stays opt-in and survives a two-capability save at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'admin', 'simple')
    await installOperatorApiMock(page, 'admin', 'simple')
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    const account = { ...operatorFixtureAccounts[0], name: 'Ark Fixture', type: 'apikey',
      credentials: { base_url: 'https://ark.example/api/v3', api_key: 'fixture-key' },
      extra: {}, group_ids: [], groups: [], credentials_status: { has_api_key: true } }
    await page.route('**/api/v1/admin/accounts?*', route => route.fulfill({ json: { code: 0, data: {
      items: [account], total: 1, page: 1, page_size: 20, pages: 1,
    } } }))
    const writes: Record<string, unknown>[] = []
    await page.route(/\/api\/v1\/admin\/accounts\/101(?:\?.*)?$/, route => {
      if (route.request().method() === 'PUT') writes.push(route.request().postDataJSON())
      return route.fulfill({ json: { code: 0, data: account } })
    })
    await page.goto('/admin/accounts')
    await page.getByRole('button', { name: 'Edit Ark Fixture', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: 'Edit Account', exact: true })
    const capability = dialog.getByTestId('openai-endpoint-capability-seedance')
    await expect(capability).not.toBeChecked()
    await capability.check()
    await dialog.getByTestId('openai-endpoint-capability-embeddings').uncheck()
    await expect(dialog.getByTestId('openai-endpoint-capability-chat_completions')).toBeChecked()
    await capability.scrollIntoViewIfNeeded()
    await page.screenshot({ path: testInfo.outputPath(`seedance-capability-${width}.png`), animations: 'disabled' })
    await dialog.getByRole('button', { name: 'Update', exact: true }).click()
    await expect(dialog).not.toBeVisible()
    expect(writes).toHaveLength(1)
    expect(writes[0].credentials).toMatchObject({ openai_capabilities: ['chat_completions', 'seedance'] })
    expect(errors).toEqual([])
  })
}
