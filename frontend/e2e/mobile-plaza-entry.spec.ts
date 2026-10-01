import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixturePublicSettings } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`model plaza header entry at ${width}px respects its feature flag`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'user')
    await installOperatorApiMock(page, 'user')
    await page.route('**/api/v1/groups/available*', route => route.fulfill({ json: { code: 0, data: [] } }))
    await page.route('**/api/v1/groups/rates*', route => route.fulfill({ json: { code: 0, data: {} } }))
    let enabled = true
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.route('**/api/v1/settings/public*', route => route.fulfill({ json: { code: 0, data: { ...operatorFixturePublicSettings, model_plaza_enabled: enabled } } }))
    await page.route('**/api/v1/model-plaza*', route => route.fulfill({ json: { code: 0, data: { groups: [], description: '' } } }))
    await page.goto('/keys')
    const link = page.locator('header').getByRole('link', { name: 'Model Plaza', exact: true })
    await expect(link).toBeVisible()
    await expect(link).toHaveAttribute('title', 'Model Plaza')
    expect(await link.locator('span').last().isVisible()).toBe(width >= 640)
    await page.screenshot({ path: testInfo.outputPath(`plaza-entry-${width}.png`), animations: 'disabled' })
    await link.click()
    await expect(page).toHaveURL(/\/model-plaza\?embedded=1$/)
    enabled = false
    await page.goto('/keys')
    await expect(link).toHaveCount(0)
    expect(errors).toEqual([])
  })
}
