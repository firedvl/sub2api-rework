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
    // Cost Breakdown moved from a hover tooltip into the record drawer (row click / Enter).
    const trigger = page.getByTestId('usage-record-row').filter({ visible: true }).first()
    await trigger.click()
    const drawer = page.getByRole('dialog')
    const tooltip = drawer.getByTestId('usage-cost-details')
    await expect(tooltip).toContainText('Cost Breakdown')
    await expect(tooltip).toContainText('$0.00000001')
    await expect(tooltip).toContainText('$0.00000003')
    await expect(tooltip).toContainText('$0.00000005')
    await expect(tooltip).toContainText('$0.00000006')
    await expect(tooltip).toContainText('$0.00000022')
    await expect(tooltip).toContainText('$0.00000042')
    await expect(tooltip).toContainText('$0.00000018')
    // The drawer slides in from the right; let it settle before measuring.
    await expect.poll(async () => { const box = await tooltip.boundingBox(); return box ? box.x + box.width : Infinity }).toBeLessThanOrEqual(width)
    const bounds = await tooltip.boundingBox()
    expect(bounds).not.toBeNull()
    expect(bounds!.x).toBeGreaterThanOrEqual(0)
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(width)
    const colors = await tooltip.evaluate(element => {
      const number = Array.from(element.querySelectorAll('dd > span')).find(span => span.textContent === '$0.00000042')
      const label = Array.from(element.querySelectorAll('dt')).find(dt => dt.textContent?.trim() === 'Input Cost')
      return { number: number && getComputedStyle(number).color, label: label && getComputedStyle(label).color }
    })
    // The drawer is a light surface (the old tooltip was dark): billed amounts keep the green tone, labels stay muted.
    expect(colors.number).toBe('rgb(21, 128, 61)')
    expect(colors.label).toBe('rgb(100, 108, 124)')
    await page.screenshot({ path: testInfo.outputPath(`usage-detail-${width}.png`), fullPage: true, animations: 'disabled' })
    await page.keyboard.press('Escape')
    await expect(drawer).not.toBeVisible()
    await expect(trigger).toBeFocused()
    await page.keyboard.press('Enter')
    await expect(tooltip).toBeVisible()
    await expect(tooltip).toContainText('$0.00000042')
    await page.keyboard.press('Escape')
    await expect(drawer).not.toBeVisible()
  })
}
