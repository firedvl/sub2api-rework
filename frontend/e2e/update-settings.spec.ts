import { expect, test, type Page, type Route } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureUpdateStatus } from './fixtures/operatorData.ts'

const readyStatus = {
  ...operatorFixtureUpdateStatus,
  latest_compatible_rework: '0.1.184-rework.1',
  state: 'update_ready',
  installable: true,
  updater: {
    ...operatorFixtureUpdateStatus.updater,
    state: 'prepared',
    prepared_version: '0.1.184-rework.1',
  },
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, message: 'ok', data }),
  })
}

async function mockUpdateApi(page: Page, status: typeof operatorFixtureUpdateStatus | typeof readyStatus, installs: unknown[] = []) {
  await page.route('**/api/v1/admin/system/**', async (route) => {
    const request = route.request()
    const pathname = new URL(request.url()).pathname
    if (pathname === '/api/v1/admin/system/check-updates') return fulfill(route, status)
    if (pathname === '/api/v1/admin/system/install') {
      installs.push(request.postDataJSON())
      return fulfill(route, { operation_id: 'install-fixture', action: 'install', state: 'accepted' })
    }
    return fulfill(route, { operation_id: 'fixture', action: 'prepare', state: 'accepted' })
  })
}

test.beforeEach(async ({ page }) => {
  await seedSession(page)
  await installOperatorApiMock(page)
})

test('shows compatibility pending distinctly at a narrow width and renders release text safely', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await mockUpdateApi(page, operatorFixtureUpdateStatus)
  await page.goto('/admin/settings')

  const card = page.getByRole('region', { name: 'Software Updates' })
  await expect(card).toContainText('Compatibility review pending', { timeout: 15_000 })
  await expect(card).toContainText('Upstream baseline')
  await expect(card).toContainText('v0.1.183')
  await expect(card).toContainText('Latest upstream')
  await expect(card).toContainText('v0.1.184')
  await expect(card).toContainText('<strong>Untrusted release text stays text.</strong>')
  await expect(card.locator('strong')).toHaveCount(0)
  await expect(card.getByRole('button', { name: 'Prepare' })).toBeDisabled()
  await expect(card.getByRole('button', { name: 'Install' })).toBeDisabled()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
  await page.screenshot({ path: testInfo.outputPath('update-pending-narrow.png'), fullPage: true })
})

test('confirms a ready install by keyboard at a wide width', async ({ page }, testInfo) => {
  const installs: unknown[] = []
  await page.setViewportSize({ width: 1280, height: 800 })
  await mockUpdateApi(page, readyStatus, installs)
  await page.goto('/admin/settings')

  const card = page.getByRole('region', { name: 'Software Updates' })
  await expect(card).toContainText('Approved update ready')
  const install = card.getByRole('button', { name: 'Install' })
  await expect(install).toBeEnabled()
  await install.focus()
  await page.keyboard.press('Enter')

  const dialog = page.getByRole('dialog', { name: 'Install' })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Close modal' })).toBeFocused()
  await page.keyboard.press('Tab')
  const confirmation = dialog.getByLabel('Type INSTALL 0.1.184-rework.1 to confirm')
  await expect(confirmation).toBeFocused()
  await confirmation.pressSequentially('INSTALL 0.1.184-rework.1')
  await page.keyboard.press('Tab')
  await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(dialog.getByRole('button', { name: 'Confirm' })).toBeFocused()
  await page.screenshot({ path: testInfo.outputPath('update-ready-confirmation-wide.png'), fullPage: true })
  await page.keyboard.press('Enter')

  await expect(dialog).toBeHidden()
  await expect.poll(() => installs).toEqual([{
    version: '0.1.184-rework.1',
    confirmation: 'INSTALL 0.1.184-rework.1',
  }])
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
})

test('prepares recovery separately and shows host follow-up after the gateway stops', async ({ page }, testInfo) => {
  const actions: { path: string; body: unknown }[] = []
  let stopped = false
  await page.setViewportSize({ width: 390, height: 844 })
  await page.route('**/api/v1/admin/system/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith('/check-updates')) {
      if (stopped) return route.abort('connectionrefused')
      return fulfill(route, { ...readyStatus, updater: { ...readyStatus.updater, updater_version: '1.1.5', rollback_version: '0.1.183-rework.1' } })
    }
    actions.push({ path, body: route.request().postDataJSON() })
    stopped = true
    return fulfill(route, { operation_id: 'recovery-fixture', action: 'prepare_recovery', state: 'accepted' })
  })
  await page.goto('/admin/settings')
  const card = page.getByRole('region', { name: 'Software Updates' })
  await card.getByRole('button', { name: 'Prepare recovery', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Prepare recovery', exact: true })
  await dialog.getByLabel('Type PREPARE RECOVERY 0.1.183-rework.1 to confirm').fill('PREPARE RECOVERY 0.1.183-rework.1')
  await dialog.getByRole('button', { name: 'Confirm', exact: true }).click()
  await expect(card).toContainText('On the host, query GET /v1/recovery')
  await expect(card.getByRole('button', { name: 'Restore database and roll back' })).toBeDisabled()
  await page.waitForTimeout(3500)
  expect(actions).toEqual([{ path: '/api/v1/admin/system/prepare-recovery', body: { version: '0.1.183-rework.1', confirmation: 'PREPARE RECOVERY 0.1.183-rework.1' } }])
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
  await card.getByRole('status').filter({ hasText: 'On the host, query GET /v1/recovery' }).scrollIntoViewIfNeeded()
  await page.screenshot({ path: testInfo.outputPath('recovery-host-followup-narrow.png'), fullPage: true })
})

test('executes only the exact prepared recovery consent and disables ordinary rollback', async ({ page }, testInfo) => {
  const actions: unknown[] = []
  const consent = `RESTORE PREUPDATE DATABASE 0.1.183-rework.1 USING RESCUE upd-${'1'.repeat(24)} sha256:${'a'.repeat(64)} ACK upd-${'2'.repeat(24)}`
  await page.setViewportSize({ width: 1280, height: 800 })
  await page.route('**/api/v1/admin/system/**', async (route) => {
    if (new URL(route.request().url()).pathname.endsWith('/check-updates')) return fulfill(route, {
      ...readyStatus,
      updater: { ...readyStatus.updater, updater_version: '1.1.5' },
      recovery: { source_version: '0.1.183-rework.1', current_schema: 249, source_schema: 239, rescue_sha256: 'a'.repeat(64), confirmation: consent, phase: 'prepared' },
    })
    actions.push(route.request().postDataJSON())
    return fulfill(route, { operation_id: 'recovery-fixture', action: 'recover', state: 'accepted' })
  })
  await page.goto('/admin/settings')
  const card = page.getByRole('region', { name: 'Software Updates' })
  await expect(card).toContainText('Current-data rescue backup verified and retained')
  await expect(card.getByRole('button', { name: 'Roll back', exact: true })).toBeDisabled()
  await card.getByRole('button', { name: 'Restore database and roll back' }).click()
  const dialog = page.getByRole('dialog', { name: 'Restore database and roll back' })
  const input = dialog.locator('#update-confirmation')
  await input.fill('RESTORE DATABASE AND ROLLBACK 0.1.183-rework.1')
  await dialog.getByRole('button', { name: 'Confirm', exact: true }).click()
  expect(actions).toEqual([])
  await input.fill(consent)
  await expect(dialog).toHaveCSS('opacity', '1')
  await expect(dialog.locator('.modal-content')).toHaveCSS('opacity', '1')
  expect(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true)
  await page.screenshot({ path: testInfo.outputPath('recovery-exact-consent-wide.png'), fullPage: true, animations: 'disabled' })
  await dialog.getByRole('button', { name: 'Confirm', exact: true }).click()
  await expect.poll(() => actions).toEqual([{ version: '0.1.183-rework.1', confirmation: consent }])
})
