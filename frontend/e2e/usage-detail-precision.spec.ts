import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { getOperatorFixtureData } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`usage details retain tiny billable amounts at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    const fixture = getOperatorFixtureData('/api/v1/admin/usage') as { items: Record<string, unknown>[] }
    const row = { ...fixture.items[0], input_cost: 0.00000001, output_cost: 0.00000003,
      cache_creation_cost: 0.00000005, cache_read_cost: 0.00000006, total_cost: 0.00000022,
      actual_cost: 0.00000042, account_stats_cost: 0.00000012, account_rate_multiplier: 1.5 }
    await page.route('**/api/v1/admin/usage?*', route => route.fulfill({ json: { code: 0, data: {
      items: [row], total: 1, pages: 1, page: 1, page_size: 20
    } } }))
    await page.goto('/admin/usage')
    const trigger = page.getByRole('button', { name: 'Cost Breakdown', exact: true }).first()
    await trigger.hover()
    const tooltip = page.getByRole('tooltip').filter({ hasText: 'Cost Breakdown' })
    await expect(tooltip).toContainText('$0.00000001')
    await expect(tooltip).toContainText('$0.00000003')
    await expect(tooltip).toContainText('$0.00000005')
    await expect(tooltip).toContainText('$0.00000006')
    await expect(tooltip).toContainText('$0.00000022')
    await expect(tooltip).toContainText('$0.00000042')
    await expect(tooltip).toContainText('$0.00000018')
    const bounds = await tooltip.boundingBox()
    expect(bounds).not.toBeNull()
    expect(bounds!.x).toBeGreaterThanOrEqual(0)
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(width)
    const colors = await tooltip.evaluate(element => {
      const number = Array.from(element.querySelectorAll('span')).find(span => span.textContent === '$0.00000042')
      const label = Array.from(element.querySelectorAll('span')).find(span => span.textContent === 'Input Cost')
      return { number: number && getComputedStyle(number).color, label: label && getComputedStyle(label).color }
    })
    expect(colors.number).toBe('rgb(134, 239, 172)')
    expect(colors.label).toBe('rgb(203, 213, 225)')
    await page.screenshot({ path: testInfo.outputPath(`usage-detail-${width}.png`), fullPage: true, animations: 'disabled' })
    await page.mouse.move(1, 1)
    await expect(tooltip).not.toBeVisible()
    await trigger.focus()
    await expect(tooltip).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(tooltip).not.toBeVisible()
  })
}
