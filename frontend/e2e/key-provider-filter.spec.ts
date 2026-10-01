import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

for (const width of [390, 1280]) {
  test(`provider filtering clears stale groups and creates the selected key at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'user')
    await installOperatorApiMock(page, 'user')
    const groups = ['anthropic', 'openai', 'deepseek', 'composite'].map((platform, index) => ({
      id: index + 1, name: `Group ${index + 1}`, platform, subscription_type: 'standard',
      rate_multiplier: 1, peak_rate_enabled: false,
    }))
    await page.route('**/api/v1/groups/available*', route => route.fulfill({ json: { code: 0, data: groups } }))
    await page.route('**/api/v1/groups/rates*', route => route.fulfill({ json: { code: 0, data: {} } }))
    await page.route('**/api/v1/keys?*', route => route.fulfill({ json: { code: 0, data: { items: [], total: 0, page: 1, page_size: 20, pages: 1 } } }))
    const writes: Record<string, unknown>[] = []
    await page.route(/\/api\/v1\/keys$/, route => {
      writes.push(route.request().postDataJSON())
      return route.fulfill({ json: { code: 0, data: { id: 91, name: 'Provider fixture', key: 'sk-fixture-only', group_id: 2 } } })
    })
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.goto('/keys')
    await page.getByTestId('keys-create-btn').click()
    const dialog = page.getByRole('dialog', { name: 'Create API Key', exact: true })
    await dialog.getByTestId('key-form-name').fill('Provider fixture')
    const group = dialog.getByTestId('key-form-group')
    await group.click()
    await expect(page.getByRole('option', { name: /Group 1/ })).toHaveCount(1)
    await expect(page.getByRole('option', { name: /Group 2/ })).toHaveCount(0)
    await page.getByRole('option', { name: /Group 1/ }).click()
    await dialog.getByText('OpenAI (1)', { exact: true }).click()
    const openAI = dialog.getByRole('radio', { name: 'OpenAI (1)', exact: true })
    await expect(openAI).toBeChecked()
    await openAI.focus()
    await page.keyboard.press('ArrowLeft')
    await expect(dialog.getByRole('radio', { name: 'Anthropic (1)', exact: true })).toBeChecked()
    await page.keyboard.press('ArrowRight')
    await expect(openAI).toBeChecked()
    await expect(group).toContainText('Select a group')
    await dialog.getByRole('button', { name: 'Create', exact: true }).click()
    await expect(page.getByText('Please select a group', { exact: true })).toBeVisible()
    expect(writes).toHaveLength(0)
    await group.click()
    await expect(page.getByRole('option', { name: /Group 1/ })).toHaveCount(0)
    await page.getByRole('option', { name: /Group 2/ }).click()
    expect(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true)
    await page.screenshot({ path: testInfo.outputPath(`key-provider-${width}.png`), animations: 'disabled' })
    await dialog.getByRole('button', { name: 'Create', exact: true }).click()
    await expect(dialog).not.toBeVisible()
    expect(writes).toHaveLength(1)
    expect(writes[0]).toMatchObject({ name: 'Provider fixture', group_id: 2 })
    expect(errors).toEqual([])
  })
}
