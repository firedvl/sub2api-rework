import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { getOperatorFixtureData } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`usage observation labels and keyboard details at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    const fixture = getOperatorFixtureData('/api/v1/admin/usage') as { items: Record<string, unknown>[] }
    const items = [false, null, true].map((complete, index) => ({
      ...fixture.items[0], id: index + 1, request_id: `observation-${index}`,
      actual_cost: 0.00000042, usage_observation_complete: complete,
    }))
    await page.route('**/api/v1/admin/usage?*', route => route.fulfill({ json: { code: 0, data: {
      items, total: 3, pages: 1, page: 1, page_size: 20,
    } } }))
    await page.goto('/admin/usage')
    await expect(page.getByTestId('usage-observation-label')).toHaveCount(2)
    await expect(page.getByTestId('usage-observation-label').nth(0)).toHaveText('Partial response observed')
    await expect(page.getByTestId('usage-observation-label').nth(1)).toHaveText('Observation completeness unknown')
    await expect(page.getByText(/Costs reflect gateway observations/)).toBeVisible()
    const triggers = page.getByRole('button', { name: 'Cost Breakdown', exact: true })
    for (const [index, label] of ['Partial response observed', 'Observation completeness unknown', 'Complete response observed'].entries()) {
      const trigger = triggers.nth(index)
      await trigger.focus()
      const tooltip = page.getByRole('tooltip').filter({ hasText: 'Cost Breakdown' })
      await expect(tooltip).toBeVisible()
      await expect(tooltip).toContainText(label)
      await expect(tooltip).toContainText('$0.00000042')
      const bounds = await tooltip.boundingBox()
      expect(bounds!.x).toBeGreaterThanOrEqual(0)
      expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(width)
      await page.keyboard.press('Escape')
      await expect(tooltip).not.toBeVisible()
    }
  })
}
