import type { Page, Route } from '@playwright/test'

export type SessionRole = 'admin' | 'user'
export type RunMode = 'standard' | 'simple'

const userForRole = (role: SessionRole, runMode: RunMode = 'standard') => ({
  id: role === 'admin' ? 1 : 2,
  username: role === 'admin' ? 'operator' : 'member',
  email: role === 'admin' ? 'operator@example.test' : 'member@example.test',
  role,
  balance: 0,
  concurrency: 1,
  status: 'active',
  run_mode: runMode,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
})

const paginated = { items: [], total: 0, page: 1, page_size: 20, pages: 1 }

const dashboardStats = {
  total_users: 0,
  today_new_users: 0,
  active_users: 0,
  hourly_active_users: 0,
  stats_updated_at: '',
  stats_stale: false,
  total_api_keys: 0,
  active_api_keys: 0,
  total_accounts: 0,
  normal_accounts: 0,
  error_accounts: 0,
  ratelimit_accounts: 0,
  overload_accounts: 0,
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 0,
  total_cost: 0,
  total_actual_cost: 0,
  total_account_cost: 0,
  today_requests: 0,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 0,
  today_cost: 0,
  today_actual_cost: 0,
  today_account_cost: 0,
  average_duration_ms: 0,
  uptime: 0,
  rpm: 0,
  tpm: 0,
}

const opsOverview = {
  start_time: '',
  end_time: '',
  platform: '',
  success_count: 0,
  error_count_total: 0,
  business_limited_count: 0,
  error_count_sla: 0,
  request_count_total: 0,
  request_count_sla: 0,
  token_consumed: 0,
  sla: 1,
  error_rate: 0,
  upstream_error_rate: 0,
  upstream_error_count_excl_429_529: 0,
  upstream_429_count: 0,
  upstream_529_count: 0,
  qps: { current: 0, peak: 0, avg: 0 },
  tps: { current: 0, peak: 0, avg: 0 },
  duration: {},
  ttft: {},
}

async function fulfill(route: Route, data: unknown): Promise<void> {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, message: 'ok', data }),
  })
}

export async function seedSession(
  page: Page,
  role: SessionRole = 'admin',
  runMode: RunMode = 'standard',
): Promise<void> {
  await page.addInitScript((user) => {
    localStorage.setItem('auth_token', 'e2e-session-token')
    localStorage.setItem('auth_user', JSON.stringify(user))
    const guide = user.role === 'admin' ? 'admin_guide' : 'user_guide'
    localStorage.setItem(`${guide}_${user.id}_${user.role}_v4_interactive`, 'true')
  }, userForRole(role, runMode))
}

export async function installOperatorApiMock(
  page: Page,
  role: SessionRole = 'admin',
  runMode: RunMode = 'standard',
): Promise<void> {
  await page.route('**/setup/status*', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ data: { needs_setup: false, step: 'complete' } }),
    }),
  )

  await page.route('**/api/v1/**', async (route) => {
    const path = new URL(route.request().url()).pathname

    if (path === '/api/v1/auth/me') return fulfill(route, userForRole(role, runMode))
    if (path === '/api/v1/settings/public') {
      return fulfill(route, {
        site_name: 'Sub2API',
        site_logo: '',
        backend_mode_enabled: false,
        channel_monitor_enabled: true,
        risk_control_enabled: false,
        payment_enabled: false,
        plugin_management_enabled: false,
        affiliate_enabled: false,
        custom_menu_items: [],
      })
    }
    if (path === '/api/v1/admin/compliance') {
      return fulfill(route, {
        required: false,
        version: 'e2e',
        document_path_zh: '',
        document_path_en: '',
        document_url_zh: '',
        document_url_en: '',
        ack_phrase_zh: '',
        ack_phrase_en: '',
      })
    }
    if (path === '/api/v1/admin/settings') {
      return fulfill(route, {
        ops_monitoring_enabled: true,
        ops_realtime_monitoring_enabled: false,
        ops_query_mode_default: 'auto',
        payment_enabled: false,
        custom_menu_items: [],
        default_subscriptions: [],
        forwarded_client_ip_headers: [],
        login_agreement_documents: [],
        openai_fast_policy_settings: { rules: [] },
        registration_email_suffix_whitelist: [],
        table_page_size_options: [10, 20, 50, 100],
      })
    }
    if (path === '/api/v1/admin/payment/config') {
      return fulfill(route, {
        enabled: false,
        min_amount: 0,
        max_amount: 0,
        daily_limit: 0,
        order_timeout_minutes: 30,
        max_pending_orders: 1,
        enabled_payment_types: [],
        balance_disabled: false,
        balance_recharge_multiplier: 1,
        subscription_usd_to_cny_rate: 1,
        recharge_fee_rate: 0,
        load_balance_strategy: 'round-robin',
        product_name_prefix: '',
        product_name_suffix: '',
        help_image_url: '',
        help_text: '',
      })
    }
    if (path === '/api/v1/announcements' || path === '/api/v1/subscriptions/active') {
      return fulfill(route, [])
    }
    if (path === '/api/v1/keys') return fulfill(route, { ...paginated, page_size: 100 })
    if (path === '/api/v1/admin/dashboard/snapshot-v2') {
      return fulfill(route, {
        generated_at: '2026-01-01T00:00:00Z',
        start_date: '',
        end_date: '',
        granularity: 'hour',
        stats: dashboardStats,
        trend: [],
        models: [],
        groups: [],
        users_trend: [],
      })
    }
    if (path === '/api/v1/admin/dashboard/users-trend') {
      return fulfill(route, { trend: [], start_date: '', end_date: '', granularity: 'hour' })
    }
    if (path === '/api/v1/admin/dashboard/users-ranking') {
      return fulfill(route, {
        ranking: [],
        total_actual_cost: 0,
        total_requests: 0,
        total_tokens: 0,
        start_date: '',
        end_date: '',
      })
    }
    if (path === '/api/v1/admin/accounts' || path === '/api/v1/admin/groups' || path === '/api/v1/admin/channels') {
      return fulfill(route, paginated)
    }
    if (path === '/api/v1/admin/groups/usage-summary' || path === '/api/v1/admin/groups/capacity-summary') {
      return fulfill(route, [])
    }
    if (path === '/api/v1/admin/proxies/all' || path === '/api/v1/admin/groups/all' || path === '/api/v1/admin/payment/providers') {
      return fulfill(route, [])
    }
    if (path === '/api/v1/admin/groups/live-capability') return fulfill(route, { supported: false })
    if (path === '/api/v1/admin/accounts/upstream-billing-probe/settings') return fulfill(route, { enabled: false })
    if (path === '/api/v1/admin/accounts/ollama-cloud-usage/settings') return fulfill(route, { enabled: false })
    if (path === '/api/v1/admin/usage') return fulfill(route, paginated)
    if (path === '/api/v1/admin/usage/stats') {
      return fulfill(route, {
        total_requests: 0,
        total_input_tokens: 0,
        total_output_tokens: 0,
        total_cache_tokens: 0,
        total_cache_creation_tokens: 0,
        total_cache_read_tokens: 0,
        total_tokens: 0,
        total_cost: 0,
        total_actual_cost: 0,
        total_account_cost: 0,
        average_duration_ms: 0,
        endpoints: [],
        upstream_endpoints: [],
        endpoint_paths: [],
      })
    }
    if (path === '/api/v1/admin/dashboard/models') {
      return fulfill(route, { models: [], start_date: '', end_date: '' })
    }
    if (path === '/api/v1/admin/ops/dashboard/snapshot-v2') {
      return fulfill(route, {
        generated_at: '2026-01-01T00:00:00Z',
        overview: opsOverview,
        throughput_trend: { bucket: 'minute', points: [], by_platform: [], top_groups: [] },
        error_trend: { bucket: 'minute', points: [] },
      })
    }
    if (path === '/api/v1/admin/ops/dashboard/switch-rate-trend') return fulfill(route, { bucket: 'minute', points: [] })
    if (path === '/api/v1/admin/ops/dashboard/latency-histogram') {
      return fulfill(route, { start_time: '', end_time: '', platform: '', total_requests: 0, buckets: [] })
    }
    if (path === '/api/v1/admin/ops/dashboard/error-distribution') return fulfill(route, { total: 0, items: [] })
    if (path === '/api/v1/admin/ops/advanced-settings') {
      return fulfill(route, {
        display_alert_events: false,
        display_openai_token_stats: false,
        auto_refresh_enabled: false,
        auto_refresh_interval_seconds: 30,
      })
    }
    if (path === '/api/v1/admin/ops/settings/metric-thresholds') return fulfill(route, {})
    if (path.includes('/api/v1/admin/ops/system-logs')) return fulfill(route, paginated)
    if (path === '/api/v1/admin/settings/admin-api-key') return fulfill(route, { exists: false, masked_key: '' })
    if (path === '/api/v1/admin/settings/web-search-emulation') return fulfill(route, { enabled: false, providers: [] })
    if (path === '/api/v1/admin/settings/email-templates') {
      return fulfill(route, { events: [], locales: [], templates: [], placeholders: [] })
    }
    if (path === '/api/v1/admin/settings/beta-policy') return fulfill(route, { rules: [] })
    if (path === '/api/v1/admin/settings/overload-cooldown') return fulfill(route, { enabled: false, cooldown_minutes: 5 })
    if (path === '/api/v1/admin/settings/rate-limit-429-cooldown') return fulfill(route, { enabled: false, cooldown_seconds: 60 })
    if (path === '/api/v1/admin/settings/panel-rate-limit') {
      return fulfill(route, { enabled: false, user_rpm: 0, heavy_rpm: 0, exempt_admin: true, public_ip_rpm: 0 })
    }
    if (path === '/api/v1/admin/settings/stream-timeout') {
      return fulfill(route, {
        enabled: false,
        action: 'none',
        temp_unsched_minutes: 0,
        threshold_count: 0,
        threshold_window_minutes: 0,
      })
    }
    if (path === '/api/v1/admin/settings/rectifier') {
      return fulfill(route, {
        enabled: false,
        thinking_signature_enabled: false,
        thinking_budget_enabled: false,
        apikey_signature_enabled: false,
        apikey_signature_patterns: [],
      })
    }
    if (path === '/api/v1/admin/backups/s3-config') {
      return fulfill(route, {
        endpoint: '',
        region: 'auto',
        bucket: '',
        access_key_id: '',
        prefix: 'backups/',
        force_path_style: false,
      })
    }
    if (path === '/api/v1/admin/backups/image-storage') {
      return fulfill(route, {
        config: {
          enabled: false,
          reuse_backup_s3: true,
          bucket: '',
          prefix: 'images/',
          public_base_url: '',
          presign_expiry_hours: 24,
          max_download_bytes: 33554432,
          endpoint: '',
          region: 'auto',
          access_key_id: '',
          force_path_style: false,
        },
        secret_configured: false,
      })
    }
    if (path === '/api/v1/admin/backups/schedule') {
      return fulfill(route, { enabled: false, cron_expr: '0 2 * * *', retain_days: 14, retain_count: 10 })
    }
    if (path === '/api/v1/admin/backups') return fulfill(route, { items: [] })

    return fulfill(route, {})
  })
}
