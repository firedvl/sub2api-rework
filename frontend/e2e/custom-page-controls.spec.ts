import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixturePublicSettings } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`custom-page button drag and visibility at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'user')
    await installOperatorApiMock(page, 'user')
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.route('https://embed.example/**', route => route.fulfill({ contentType: 'text/html', body: '<!doctype html><html><body style="margin:24px;font:16px system-ui">Fixture embedded page</body></html>' }))
    await page.route('**/api/v1/settings/public*', route => route.fulfill({ json: { code: 0, data: {
      ...operatorFixturePublicSettings, custom_menu_items: [
        { id: 'visible', label: 'Visible page', url: 'https://embed.example/page', visibility: 'user', sort_order: 0, icon_svg: '' },
        { id: 'hidden', label: 'Hidden page', url: 'https://embed.example/hidden', visibility: 'user', sort_order: 1, icon_svg: '', hide_open_button: true },
      ],
    } } }))
    await page.goto('/custom/visible')
    const button = page.locator('.custom-open-fab')
    const shell = page.locator('.custom-embed-shell')
    await expect(button).toBeVisible()
    await expect(page.locator('iframe')).toHaveAttribute('src', /https:\/\/embed.example\/page/)
    const initial = await button.boundingBox()
    const shellBounds = await shell.boundingBox()
    expect(initial).not.toBeNull()
    expect(shellBounds).not.toBeNull()
    const popups: string[] = []
    page.on('popup', popup => { popups.push(popup.url()); void popup.close() })
    await page.mouse.move(initial!.x + initial!.width / 2, initial!.y + initial!.height / 2)
    await page.mouse.down()
    await page.mouse.move(shellBounds!.x + 40, shellBounds!.y + 180, { steps: 8 })
    await page.mouse.up()
    await expect.poll(async () => (await button.boundingBox())!.y).toBeGreaterThan(initial!.y + 50)
    expect(popups).toEqual([])
    await page.setViewportSize({ width: Math.min(width, 390), height: 640 })
    await expect.poll(async () => {
      const current = (await button.boundingBox())!
      const bounds = (await shell.boundingBox())!
      return current.x >= bounds.x && current.y >= bounds.y && current.x + current.width <= bounds.x + bounds.width + 1 && current.y + current.height <= bounds.y + bounds.height + 1
    }).toBe(true)
    await button.focus()
    await expect(button).toBeFocused()
    await page.screenshot({ path: testInfo.outputPath(`custom-drag-${width}.png`), animations: 'disabled' })
    await button.click()
    await expect.poll(() => popups.length).toBe(1)
    await page.goto('/custom/hidden')
    await expect(button).toHaveCount(0)
    await expect(page.locator('iframe')).toHaveAttribute('src', /https:\/\/embed.example\/hidden/)
    expect(errors).toEqual([])
  })
}
