import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixturePublicSettings } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  for (const role of ['user', 'admin'] as const) {
    test(`hidden monitor ranking and neutral TTFT for ${role} at ${width}px`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 900 })
      await seedSession(page, role)
      await installOperatorApiMock(page, role)
      await page.route('**/api/v1/settings/public*', route => route.fulfill({ json: { code: 0, data: {
        ...operatorFixturePublicSettings, channel_monitor_mode: 'v2', channel_monitor_hide_user_ranking: true,
      } } }))
      const latency = { sample_count: 0, p50_ms: null, p90_ms: null, p95_ms: null, avg_ms: null }
      const metrics = { success_requests: 200, error_requests: 0, request_count: 200, token_count: 400,
        rpm: 1, tpm: 2, error_rate: 0, cache_rate: 0, cache_rate_numerator: 0, cache_rate_denominator: 200,
        ttft: latency, duration: latency }
      const health = { overall: 'healthy', error_rate: 'healthy', ttft: 'critical', cache: 'unknown', minimum_sample: 10, score: 100, error_rate_score: 100, ttft_score: null }
      const coverage = { requested_start: '2026-09-30T00:00:00Z', requested_end: '2026-10-01T00:00:00Z', coverage_start: '2026-09-30T00:00:00Z', data_through: '2026-10-01T00:00:00Z', computed_at: '2026-10-01T00:00:00Z', aggregation_lag_seconds: 0, coverage_complete: true, bucket_seconds: 300 }
      const config = { version: 1, enabled: true, refresh_interval_seconds: 300, platforms: [], group_ids: [], health_thresholds: { minimum_sample: 10, warning_error_rate: 0.1, critical_error_rate: 0.2, target_ttft_ms: 500, warning_ttft_ms: 2000, critical_ttft_ms: 5000, warning_cache_rate: 0.2, critical_cache_rate: 0.1, error_weight: 1, ttft_weight: 1, cache_weight: 0 } }
      let userReads = 0
      const errors: string[] = []
      page.on('pageerror', error => errors.push(error.message))
      await page.route(/\/api\/v1\/(?:admin\/)?channel-monitor-v2\//, route => {
        const path = new URL(route.request().url()).pathname
        let data: unknown = { coverage, items: [] }
        if (path.endsWith('/dimensions')) data = { platforms: [], groups: [], models: [] }
        if (path.endsWith('/snapshot')) data = { config, coverage, metrics, health, trend: [] }
        if (path.endsWith('/matrix')) data = { coverage, group_by: 'platform_group', items: [] }
        if (path.endsWith('/users')) userReads++
        return route.fulfill({ json: { code: 0, data } })
      })
      await page.goto('/monitor?tab=users')
      const ranking = page.getByRole('tab', { name: 'User ranking', exact: true })
      if (role === 'admin') {
        await expect(ranking).toBeVisible()
        await expect.poll(() => userReads).toBeGreaterThan(0)
      } else {
        await expect(ranking).toHaveCount(0)
        await expect(page.getByRole('tab', { name: 'Models', exact: true })).toBeVisible()
        expect(userReads).toBe(0)
      }
      const cell = page.locator('.stat-card').filter({ has: page.getByText('First token P50', { exact: true }) })
      await expect(cell.locator('.stat-value')).toHaveText('-')
      expect(await cell.locator('.text-red-600').count()).toBe(0)
      expect(errors).toEqual([])
      await page.screenshot({ path: testInfo.outputPath(`monitor-${role}-${width}.png`), animations: 'disabled' })
    })
  }
}
