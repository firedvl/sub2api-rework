import { expect, test, type Route } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

const fulfill = (route: Route, data: unknown) =>
  route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, message: 'ok', data })
  })

test('qualifies OpenAI vision in two explicit gates before separate promotion', async ({ page }) => {
  await seedSession(page)
  await installOperatorApiMock(page)

  let state = 'UNQUALIFIED'
  let promoted = false
  await page.route('**/api/v1/admin/accounts/101/vision-qualification**', async (route) => {
    const request = route.request()
    const pathname = new URL(request.url()).pathname
    if (pathname.endsWith('/promote')) {
      promoted = true
      return fulfill(route, report(false, true))
    }
    if (request.method() === 'POST') {
      const stage = request.postDataJSON().stage
      state = stage === 'preliminary' ? 'PRELIMINARY' : 'QUALIFIED'
    }
    return fulfill(route, report(state === 'QUALIFIED', false))
  })

  await page.setViewportSize({ width: 1280, height: 800 })
  await page.goto('/admin/accounts')
  await page.getByRole('button', { name: 'More Codex Team West' }).click()
  await page.getByRole('button', { name: 'Test Connection' }).click()

  const qualification = page.getByTestId('vision-qualification')
  await expect(qualification).toBeVisible()
  await expect(page.getByTestId('vision-qualification-status')).toHaveText('Unqualified')

  await qualification.getByRole('button', { name: 'Run preliminary' }).focus()
  await page.keyboard.press('Enter')
  await expect(page.getByTestId('vision-qualification-status')).toHaveText('Preliminary 2/2')
  await expect(qualification.getByRole('button', { name: 'Add vision input' })).toHaveCount(0)

  await qualification.getByRole('button', { name: 'Run reliability' }).click()
  await expect(page.getByTestId('vision-qualification-status')).toHaveText('Qualified 10/10')
  await page.screenshot({ path: '/tmp/sub2api-vision-qualified-wide.png', fullPage: true })
  expect(promoted).toBe(false)

  await qualification.getByRole('button', { name: 'Add vision input' }).click()
  await expect(qualification.getByRole('button', { name: 'Add vision input' })).toHaveCount(0)
  expect(promoted).toBe(true)

  await page.setViewportSize({ width: 390, height: 844 })
  await expect(qualification).toBeVisible()
  await page.screenshot({ path: '/tmp/sub2api-vision-qualified-narrow.png', fullPage: true })
  const overflow = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth
  }))
  expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.clientWidth + 1)

  function report(qualified: boolean, promotedAt: boolean) {
    return {
      account_id: 101,
      state,
      model: 'gpt-5.6-sol',
      endpoint: 'responses',
      preliminary:
        state === 'UNQUALIFIED'
          ? undefined
          : { required: 2, completed: 2, passed: true, attempts: [] },
      reliability: qualified ? { required: 10, completed: 10, passed: true, attempts: [] } : undefined,
      promotion_eligible: qualified && !promotedAt,
      promoted_at: promotedAt ? '2026-09-06T12:00:00Z' : undefined
    }
  }
})

test('offers capability reconciliation for a retained promoted report', async ({ page }) => {
  await seedSession(page)
  await installOperatorApiMock(page)

  let promotionCalls = 0
  const retainedReport = {
    account_id: 101,
    state: 'QUALIFIED',
    model: 'gpt-5.6-sol',
    endpoint: 'responses',
    preliminary: { required: 2, completed: 2, passed: true, attempts: [] },
    reliability: { required: 10, completed: 10, passed: true, attempts: [] },
    promoted_at: '2026-09-06T12:00:00Z'
  }
  await page.route('**/api/v1/admin/accounts/101/vision-qualification**', async (route) => {
    if (new URL(route.request().url()).pathname.endsWith('/promote')) {
      promotionCalls++
      return fulfill(route, { ...retainedReport, promotion_eligible: false })
    }
    return fulfill(route, { ...retainedReport, promotion_eligible: true })
  })

  await page.goto('/admin/accounts')
  await page.getByRole('button', { name: 'More Codex Team West' }).click()
  await page.getByRole('button', { name: 'Test Connection' }).click()

  const qualification = page.getByTestId('vision-qualification')
  const promoteButton = qualification.getByRole('button', { name: 'Add vision input' })
  await expect(page.getByTestId('vision-qualification-status')).toHaveText('Qualified 10/10')
  await expect(promoteButton).toBeVisible()
  await promoteButton.click()
  await expect(promoteButton).toHaveCount(0)
  await expect(page.getByTestId('vision-qualification-status')).toHaveText('Qualified 10/10')
  expect(promotionCalls).toBe(1)
})
