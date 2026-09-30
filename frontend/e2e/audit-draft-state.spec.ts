import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

for (const width of [390, 1280]) {
  test(`audit engine drafts isolate keys and threshold defaults at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    await page.route('**/api/v1/settings/public*', route => route.fulfill({
      json: { code: 0, data: { risk_control_enabled: true } },
    }))
    const key = { index: 1, key_hash: 'openai-hash', masked: '********oa01', status: 'frozen', failure_count: 1, success_count: 0, last_error: '', last_latency_ms: 100, last_http_status: 429, last_tested: false, configured: true }
    const config = {
      enabled: true, engine: 'openai', mode: 'pre_block', base_url: 'https://openai.fixture', model: 'omni-moderation-latest',
      api_key_configured: true, api_key_count: 1, api_key_statuses: [key], api_key_masks: [],
      thresholds: { sexual: 0.8, harassment: 0.8 }, all_groups: true, group_ids: [],
      timeout_ms: 3000, retry_count: 2, sample_rate: 100, worker_count: 4, queue_size: 32768,
      block_status: 403, block_message: 'Fixture blocked', record_non_hits: false,
      email_on_hit: true, auto_ban_enabled: true, ban_threshold: 10, violation_window_hours: 720,
      hit_retention_days: 180, non_hit_retention_days: 3, pre_hash_check_enabled: false,
      blocked_keywords: [], model_filter: { type: 'all', models: [] },
    }
    const typesafe = { ...config, engine: 'typesafe', base_url: 'https://typesafe.fixture', model: 'jev-latest', api_key_statuses: [{ ...key, key_hash: 'typesafe-hash', masked: '********ts02', status: 'ok' }] }
    let writes = 0
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.route('**/api/v1/admin/risk-control/**', route => {
      if (route.request().method() !== 'GET') {
        writes++
        return route.fulfill({ status: 500, json: { message: 'Unexpected write' } })
      }
      const path = new URL(route.request().url()).pathname
      const data = path.endsWith('/config') ? { ...config, engine_configs: { openai: config, typesafe } }
        : path.endsWith('/status') ? { ...config, risk_control_enabled: true, queue_length: 0, queue_usage_percent: 0, active_workers: 0, max_workers: 32, pre_block_api_key_loads: [] }
          : { items: [], total: 0, page: 1, page_size: 20, pages: 0 }
      return route.fulfill({ json: { code: 0, data } })
    })
    await page.goto('/admin/risk-control')
    await page.getByRole('button', { name: 'Moderation Settings', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: 'Content Moderation Settings' })
    await expect(dialog.locator('[data-test="audit-key-statuses"]')).toContainText('********oa01')
    await dialog.locator('[data-test="audit-engine-select"] button').click()
    await page.getByRole('option', { name: 'TypeSafe AI', exact: true }).click()
    await expect(page.getByRole('listbox')).toHaveCount(0)
    const panel = dialog.locator('[data-test="audit-key-statuses"]')
    await expect(panel).toContainText('********ts02')
    await expect(panel).not.toContainText('********oa01')
    await dialog.getByRole('button', { name: 'Risk Thresholds', exact: true }).click()
    const sexual = dialog.locator('[data-test="risk-threshold-sexual"]')
    await expect(sexual).toHaveValue('80')
    await dialog.getByRole('button', { name: 'Restore defaults', exact: true }).click()
    await expect(sexual).toHaveValue('65')
    await expect(dialog.locator('[data-test="risk-threshold-harassment"]')).toHaveValue('98')
    await sexual.fill('91')
    await dialog.getByRole('button', { name: 'Basic', exact: true }).click()
    await expect(page.getByRole('listbox')).toHaveCount(0)
    await dialog.locator('[data-test="audit-engine-select"] button').click()
    await expect(page.getByRole('listbox')).toHaveCount(1)
    await page.getByRole('option', { name: 'OpenAI', exact: true }).click()
    await expect(panel).toContainText('********oa01')
    await dialog.getByRole('button', { name: 'Risk Thresholds', exact: true }).click()
    await expect(sexual).toHaveValue('80')
    await page.screenshot({ path: testInfo.outputPath(`audit-draft-${width}.png`), animations: 'disabled' })
    expect(errors).toEqual([])
    expect(writes).toBe(0)
  })
}
