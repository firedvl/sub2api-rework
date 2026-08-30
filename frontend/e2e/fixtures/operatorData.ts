export type SessionRole = 'admin' | 'user'
export type RunMode = 'standard' | 'simple'

export const OPERATOR_FIXTURE_ACCOUNTS_ETAG = '"operator-review-accounts-v2"'
export const OPERATOR_FIXTURE_TOKEN = 'operator-review-session'
export const OPERATOR_FIXTURE_NOW = '2026-08-30T02:00:00Z'

const createdAt = '2026-08-01T09:00:00Z'
const updatedAt = '2026-08-30T01:45:00Z'

export const operatorFixtureUser = (
  role: SessionRole = 'admin',
  runMode: RunMode = 'standard',
) => ({
  id: role === 'admin' ? 1 : 2,
  username: role === 'admin' ? 'operator' : 'member',
  email: role === 'admin' ? 'operator@example.test' : 'member@example.test',
  role,
  balance: 0,
  concurrency: 4,
  status: 'active',
  run_mode: runMode,
  created_at: createdAt,
  updated_at: updatedAt,
})

export const operatorFixturePublicSettings = {
  site_name: 'Sub2API',
  site_logo: '',
  site_version: 'fixture-review',
  backend_mode_enabled: false,
  channel_monitor_enabled: true,
  risk_control_enabled: false,
  payment_enabled: false,
  plugin_management_enabled: false,
  affiliate_enabled: false,
  registration_enabled: false,
  password_reset_enabled: false,
  custom_menu_items: [],
}

export const operatorFixtureUpdateStatus = {
  schema_version: 1,
  current_version: '0.1.183-rework.1',
  current_git_commit: '4668fed9458d0b442c60e09efacac7f69b4d07eb',
  build_date: updatedAt,
  build_type: 'fixture',
  update_channel: 'stable',
  update_policy: 'manual',
  upstream_baseline: 'v0.1.183',
  upstream_baseline_sha: 'e8cb019fabf8b55199436229044cbf9aa7a82564',
  latest_upstream: 'v0.1.184',
  latest_upstream_url: 'https://github.com/Wei-Shaw/sub2api/releases/tag/v0.1.184',
  state: 'compatibility_pending',
  installable: false,
  release_notes: {
    upstream: '<strong>Untrusted release text stays text.</strong>',
    rework: '',
    compatibility: 'Compatibility review is pending.',
    migrations: '',
    rollback: '',
  },
  checked_at: updatedAt,
  cached: false,
  updater: {
    schema_version: 1,
    updater_version: '1.0.0',
    healthy: true,
    state: 'idle',
    busy: false,
    installed_version: '0.1.183-rework.1',
    prepared_version: '',
    rollback_version: '',
    current_migration: 232,
    updated_at: updatedAt,
  },
}

const paginated = <T>(items: T[], pageSize = 20) => ({
  items,
  total: items.length,
  page: 1,
  page_size: pageSize,
  pages: 1,
})

const makeGroup = (
  id: number,
  name: string,
  platform: string,
  overrides: Record<string, unknown> = {},
) => ({
  id,
  name,
  description: null,
  platform,
  rate_multiplier: 1,
  rpm_limit: 0,
  max_reasoning_effort: '',
  reasoning_effort_mappings: [],
  is_exclusive: false,
  status: 'active',
  subscription_type: 'standard',
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  long_context_pricing_enabled: false,
  allow_image_generation: false,
  allow_batch_image_generation: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  batch_image_discount_multiplier: 1,
  batch_image_hold_multiplier: 1,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  video_rate_independent: false,
  video_rate_multiplier: 1,
  video_price_480p: null,
  video_price_720p: null,
  video_price_1080p: null,
  web_search_price_per_call: null,
  search_price_per_1k: null,
  audio_realtime_price_per_min: null,
  audio_tts_price_per_million_chars: null,
  audio_stt_price_per_hour: null,
  peak_rate_enabled: false,
  peak_start: '00:00',
  peak_end: '00:00',
  peak_rate_multiplier: 1,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_live: false,
  require_oauth_only: false,
  require_privacy_set: false,
  model_pricing: [],
  profit_control_enabled: false,
  profit_min_margin: 0,
  profit_safety_buffer: 0,
  model_routing: null,
  model_routing_enabled: false,
  mcp_xml_inject: false,
  account_count: 0,
  active_account_count: 0,
  rate_limited_account_count: 0,
  sort_order: id,
  created_at: createdAt,
  updated_at: updatedAt,
  ...overrides,
})

export const operatorFixtureGroups = [
  makeGroup(11, 'OpenAI Production', 'openai', {
    description: 'Primary Codex and Responses traffic',
    rpm_limit: 240,
    account_count: 3,
    active_account_count: 2,
    rate_limited_account_count: 1,
    model_routing_enabled: true,
    model_routing: { 'gpt-5.2-codex': [101, 105], 'gpt-5.2': [101] },
    model_pricing: [{
      platform: 'openai',
      models: ['gpt-5.2-codex', 'gpt-5.2'],
      billing_mode: 'token',
      input_price: 0.00000125,
      output_price: 0.00001,
      cache_write_price: null,
      cache_read_price: 0.000000125,
      image_input_price: null,
      image_output_price: null,
      per_request_price: null,
      intervals: [],
      time_pricing: null,
    }],
  }),
  makeGroup(12, 'Claude Production', 'anthropic', {
    description: 'Claude Code and Messages traffic',
    rate_multiplier: 1.08,
    rpm_limit: 120,
    account_count: 2,
    active_account_count: 2,
    subscription_type: 'subscription',
    daily_limit_usd: 40,
    weekly_limit_usd: 220,
    claude_code_only: true,
  }),
  makeGroup(13, 'Balanced Coding Route', 'composite', {
    description: 'Routes coding models across healthy provider pools',
    account_count: 5,
    active_account_count: 4,
    model_routing_enabled: true,
    model_routing: { 'coding-default': [11, 12] },
  }),
]

const accountGroup = (id: number) => operatorFixtureGroups.filter((group) => group.id === id)

const makeAccount = (
  id: number,
  name: string,
  platform: string,
  type: string,
  overrides: Record<string, unknown> = {},
) => ({
  id,
  name,
  notes: null,
  platform,
  type,
  credentials: {},
  credentials_status: {},
  extra: {},
  proxy_id: null,
  concurrency: 4,
  current_concurrency: 1,
  priority: 10,
  rate_multiplier: 1,
  status: 'active',
  error_message: null,
  last_used_at: updatedAt,
  expires_at: null,
  auto_pause_on_expired: false,
  schedulable: true,
  rate_limited_at: null,
  rate_limit_reset_at: null,
  overload_until: null,
  temp_unschedulable_until: null,
  temp_unschedulable_reason: null,
  session_window_start: null,
  session_window_end: null,
  session_window_status: null,
  quota_limit: null,
  quota_used: null,
  quota_daily_limit: null,
  quota_daily_used: null,
  quota_weekly_limit: null,
  quota_weekly_used: null,
  group_ids: [],
  groups: [],
  created_at: createdAt,
  updated_at: updatedAt,
  ...overrides,
})

export const operatorFixtureAccounts = [
  makeAccount(101, 'Codex Team West', 'openai', 'oauth', {
    credentials: { email: 'codex-west@example.test', subscription_plan_type: 'team' },
    credentials_status: { has_access_token: true, has_refresh_token: true },
    extra: {
      auto_warmup_enabled: true,
      codex_auto_warmup_state: {
        status: 'succeeded',
        attempted_at: '2026-08-30T01:40:00Z',
        completed_at: '2026-08-30T01:40:01Z',
        reset_at: '2026-08-30T04:00:00Z',
        window_type: '5h',
      },
      codex_5h_used_percent: 32,
      codex_5h_reset_after_seconds: 7200,
      codex_7d_used_percent: 48,
      codex_7d_reset_after_seconds: 345600,
      codex_usage_updated_at: updatedAt,
      openai_compact_supported: true,
    },
    concurrency: 8,
    current_concurrency: 3,
    group_ids: [11, 13],
    groups: [...accountGroup(11), ...accountGroup(13)],
    scheduler_score: { base_score: 0.94, sticky_score: 1.12, sticky_weighted_enabled: true },
  }),
  makeAccount(102, 'Claude Primary', 'anthropic', 'oauth', {
    credentials: { email: 'claude-primary@example.test' },
    credentials_status: { has_access_token: true, has_refresh_token: true },
    concurrency: 6,
    current_concurrency: 2,
    group_ids: [12, 13],
    groups: [...accountGroup(12), ...accountGroup(13)],
    session_window_start: '2026-08-29T23:00:00Z',
    session_window_end: '2026-08-30T04:00:00Z',
    session_window_status: 'allowed',
  }),
  makeAccount(103, 'Antigravity Pro', 'antigravity', 'oauth', {
    credentials: { email: 'antigravity@example.test', tier_id: 'PRO' },
    credentials_status: { has_access_token: true, has_refresh_token: true },
    current_concurrency: 0,
    overload_until: '2026-08-30T04:00:00Z',
    group_ids: [13],
    groups: accountGroup(13),
  }),
  makeAccount(104, 'Gemini Recovery', 'gemini', 'oauth', {
    credentials: { email: 'gemini-recovery@example.test', tier_id: 'google_ai_pro' },
    credentials_status: { has_refresh_token: true },
    status: 'error',
    schedulable: false,
    error_message: 'Fixture: reauthorization required',
    last_used_at: '2026-08-29T21:20:00Z',
    group_ids: [13],
    groups: accountGroup(13),
  }),
  makeAccount(105, 'Codex Standby', 'openai', 'apikey', {
    credentials_status: { has_api_key: true },
    status: 'inactive',
    schedulable: false,
    last_used_at: '2026-08-27T10:00:00Z',
    quota_limit: 100,
    quota_used: 18.4,
    group_ids: [11],
    groups: accountGroup(11),
  }),
  makeAccount(106, 'Gemini Quota Limited', 'gemini', 'oauth', {
    credentials: { email: 'gemini-healthy@example.test', tier_id: 'google_ai_pro' },
    credentials_status: { has_access_token: true, has_refresh_token: true },
    current_concurrency: 0,
    group_ids: [13],
    groups: accountGroup(13),
  }),
]

const todayStats = {
  '101': { requests: 842, tokens: 2_480_000, cost: 6.24, standard_cost: 7.8, user_cost: 6.24 },
  '102': { requests: 516, tokens: 1_130_000, cost: 8.72, standard_cost: 9.42, user_cost: 8.72 },
  '103': { requests: 294, tokens: 682_000, cost: 2.31, standard_cost: 2.89, user_cost: 2.31 },
  '104': { requests: 34, tokens: 76_000, cost: 0.26, standard_cost: 0.31, user_cost: 0.26 },
  '105': { requests: 0, tokens: 0, cost: 0, standard_cost: 0, user_cost: 0 },
  '106': { requests: 186, tokens: 412_000, cost: 1.18, standard_cost: 1.44, user_cost: 1.18 },
}

const usageProgress = (utilization: number, resetsAt: string) => ({
  utilization,
  resets_at: resetsAt,
  remaining_seconds: 7200,
})

const accountUsage = {
  '101': {
    source: 'passive',
    updated_at: updatedAt,
    five_hour: usageProgress(32, '2026-08-30T04:00:00Z'),
    seven_day: usageProgress(48, '2026-09-04T00:00:00Z'),
    seven_day_sonnet: null,
  },
  '102': {
    source: 'passive',
    updated_at: updatedAt,
    five_hour: usageProgress(61, '2026-08-30T04:00:00Z'),
    seven_day: usageProgress(37, '2026-09-04T00:00:00Z'),
    seven_day_sonnet: usageProgress(96, '2026-09-04T00:00:00Z'),
  },
  '103': {
    source: 'passive',
    updated_at: updatedAt,
    five_hour: null,
    seven_day: null,
    seven_day_sonnet: null,
    antigravity_quota: {
      'gemini-2.5-pro': { utilization: 72, reset_time: '2026-08-30T08:00:00Z' },
      'claude-sonnet-4-5': { utilization: 44, reset_time: '2026-08-30T08:00:00Z' },
    },
  },
  '104': {
    source: 'passive',
    updated_at: updatedAt,
    five_hour: null,
    seven_day: null,
    seven_day_sonnet: null,
    gemini_shared_daily: null,
    error: 'Fixture: reauthorization required',
  },
  '105': {
    source: 'passive',
    updated_at: updatedAt,
    five_hour: null,
    seven_day: null,
    seven_day_sonnet: null,
  },
  '106': {
    source: 'passive',
    updated_at: updatedAt,
    five_hour: null,
    seven_day: null,
    seven_day_sonnet: null,
    gemini_shared_daily: usageProgress(100, '2026-08-31T00:00:00Z'),
  },
}

const dashboardStats = {
  total_users: 18,
  today_new_users: 2,
  active_users: 11,
  hourly_active_users: 7,
  stats_updated_at: updatedAt,
  stats_stale: false,
  total_api_keys: 26,
  active_api_keys: 23,
  total_accounts: operatorFixtureAccounts.length,
  normal_accounts: operatorFixtureAccounts.filter((account) => account.status === 'active' && account.schedulable).length,
  error_accounts: operatorFixtureAccounts.filter((account) => account.status === 'error').length,
  ratelimit_accounts: operatorFixtureAccounts.filter((account) => account.rate_limited_at).length,
  overload_accounts: operatorFixtureAccounts.filter((account) => account.overload_until).length,
  total_requests: 1_284_920,
  total_input_tokens: 812_400_000,
  total_output_tokens: 191_600_000,
  total_cache_creation_tokens: 18_100_000,
  total_cache_read_tokens: 128_400_000,
  total_tokens: 1_150_500_000,
  total_cost: 4_812.44,
  total_actual_cost: 3_965.18,
  total_account_cost: 2_438.72,
  today_requests: 4_286,
  today_input_tokens: 3_812_400,
  today_output_tokens: 964_100,
  today_cache_creation_tokens: 84_200,
  today_cache_read_tokens: 614_800,
  today_tokens: 5_475_500,
  today_cost: 24.88,
  today_actual_cost: 19.43,
  today_account_cost: 12.71,
  average_duration_ms: 1_284,
  uptime: 1_284_600,
  rpm: 42,
  tpm: 28_640,
}

const dashboardTrend = [
  ['2026-08-29T20:00:00Z', 402, 418_000, 102_000, 1.84],
  ['2026-08-29T21:00:00Z', 516, 534_000, 128_000, 2.31],
  ['2026-08-29T22:00:00Z', 684, 712_000, 166_000, 3.04],
  ['2026-08-29T23:00:00Z', 792, 864_000, 211_000, 3.82],
  ['2026-08-30T00:00:00Z', 946, 1_016_000, 246_000, 4.44],
  ['2026-08-30T01:00:00Z', 946, 1_068_400, 271_100, 3.98],
].map(([date, requests, input, output, actualCost]) => ({
  date,
  requests,
  input_tokens: input,
  output_tokens: output,
  cache_creation_tokens: 14_000,
  cache_read_tokens: 96_000,
  total_tokens: Number(input) + Number(output) + 110_000,
  cost: Number(actualCost) * 1.22,
  actual_cost: actualCost,
}))

const dashboardModels = [
  { model: 'gpt-5.2-codex', requests: 1_842, input_tokens: 1_920_000, output_tokens: 482_000, cache_creation_tokens: 28_000, cache_read_tokens: 342_000, total_tokens: 2_772_000, cost: 11.84, actual_cost: 9.21, account_cost: 5.62 },
  { model: 'claude-sonnet-4-5', requests: 1_316, input_tokens: 1_214_000, output_tokens: 288_000, cache_creation_tokens: 42_000, cache_read_tokens: 188_000, total_tokens: 1_732_000, cost: 8.44, actual_cost: 6.82, account_cost: 4.61 },
  { model: 'gemini-2.5-pro', requests: 714, input_tokens: 562_000, output_tokens: 146_000, cache_creation_tokens: 8_000, cache_read_tokens: 62_000, total_tokens: 778_000, cost: 3.48, actual_cost: 2.71, account_cost: 1.82 },
]

const usageLog = (id: number, overrides: Record<string, unknown>) => ({
  id,
  user_id: 2,
  api_key_id: 21,
  account_id: 101,
  request_id: `req_fixture_${id}`,
  model: 'gpt-5.2-codex',
  service_tier: 'default',
  reasoning_effort: 'medium',
  inbound_endpoint: '/v1/responses',
  upstream_endpoint: '/v1/responses',
  group_id: 11,
  subscription_id: null,
  input_tokens: 18_420,
  output_tokens: 2_810,
  cache_creation_tokens: 0,
  cache_read_tokens: 6_100,
  cache_creation_5m_tokens: 0,
  cache_creation_1h_tokens: 0,
  input_cost: 0.023,
  output_cost: 0.028,
  cache_creation_cost: 0,
  cache_read_cost: 0.001,
  total_cost: 0.052,
  actual_cost: 0.044,
  account_cost: 0.031,
  rate_multiplier: 1,
  long_context_billing_applied: false,
  billing_type: 1,
  request_type: 'stream',
  stream: true,
  duration_ms: 1_482,
  first_token_ms: 328,
  image_count: 0,
  image_size: null,
  image_input_size: null,
  image_output_size: null,
  image_size_source: null,
  image_size_breakdown: null,
  image_input_tokens: 0,
  image_input_cost: 0,
  image_output_tokens: 0,
  image_output_cost: 0,
  user_agent: 'Codex fixture client',
  ip_address: '192.0.2.10',
  cache_ttl_overridden: false,
  billing_mode: 'token',
  created_at: `2026-08-30T01:${48 - id}:00Z`,
  user_email: 'member@example.test',
  api_key_name: 'review-console',
  account_name: 'Codex Team West',
  group_name: 'OpenAI Production',
  ...overrides,
})

const usageLogs = [
  usageLog(1, {}),
  usageLog(2, { model: 'claude-sonnet-4-5', upstream_model: 'claude-sonnet-4-5', account_id: 102, account_name: 'Claude Primary', group_id: 12, group_name: 'Claude Production', input_tokens: 12_860, output_tokens: 1_940, duration_ms: 1_106, first_token_ms: 284, actual_cost: 0.061 }),
  usageLog(3, { model: 'coding-default', upstream_model: 'gemini-2.5-pro', model_mapping_chain: 'coding-default -> gemini-2.5-pro', account_id: 103, account_name: 'Antigravity Pro', group_id: 13, group_name: 'Balanced Coding Route', input_tokens: 9_420, output_tokens: 1_180, duration_ms: 914, first_token_ms: 241, actual_cost: 0.018 }),
  usageLog(4, { model: 'gpt-5.2', input_tokens: 6_720, output_tokens: 824, duration_ms: 742, first_token_ms: 198, stream: false, request_type: 'sync', actual_cost: 0.012 }),
]

export const operatorFixtureChannels = [
  {
    id: 31,
    name: 'Coding API',
    description: 'Responses traffic for coding clients',
    status: 'active',
    billing_model_source: 'requested',
    restrict_models: true,
    group_ids: [11, 12, 13],
    model_pricing: operatorFixtureGroups[0].model_pricing,
    model_mapping: {
      openai: { 'coding-default': 'gpt-5.2-codex' },
      anthropic: { 'coding-default': 'claude-sonnet-4-5' },
      antigravity: { 'coding-default': 'gemini-2.5-pro' },
    },
    apply_pricing_to_account_stats: true,
    account_stats_pricing_rules: [],
    created_at: createdAt,
    updated_at: updatedAt,
  },
  {
    id: 32,
    name: 'General API',
    description: 'General-purpose model routes',
    status: 'active',
    billing_model_source: 'upstream',
    restrict_models: false,
    group_ids: [11, 12],
    model_pricing: [],
    model_mapping: {},
    apply_pricing_to_account_stats: false,
    account_stats_pricing_rules: [],
    created_at: createdAt,
    updated_at: updatedAt,
  },
]

export const operatorFixtureProxies = [
  {
    id: 41,
    name: 'US West relay',
    protocol: 'https',
    host: 'proxy-west.example.test',
    port: 8443,
    username: null,
    status: 'active',
    account_count: 3,
    latency_ms: 86,
    latency_status: 'success',
    ip_address: '192.0.2.41',
    country: 'United States',
    country_code: 'US',
    region: 'Oregon',
    city: 'Portland',
    quality_status: 'healthy',
    quality_score: 94,
    quality_grade: 'A',
    expires_at: null,
    fallback_mode: 'direct',
    backup_proxy_id: null,
    expiry_warn_days: 7,
    created_at: createdAt,
    updated_at: updatedAt,
  },
  {
    id: 42,
    name: 'EU standby relay',
    protocol: 'socks5',
    host: 'proxy-eu.example.test',
    port: 1080,
    username: 'review',
    status: 'active',
    account_count: 1,
    latency_ms: 132,
    latency_status: 'success',
    ip_address: '198.51.100.42',
    country: 'Germany',
    country_code: 'DE',
    region: 'Hesse',
    city: 'Frankfurt',
    quality_status: 'healthy',
    quality_score: 88,
    quality_grade: 'B',
    expires_at: null,
    fallback_mode: 'none',
    backup_proxy_id: null,
    expiry_warn_days: 7,
    created_at: createdAt,
    updated_at: updatedAt,
  },
]

const opsOverview = {
  start_time: '2026-08-30T01:00:00Z',
  end_time: '2026-08-30T02:00:00Z',
  platform: '',
  health_score: 96,
  success_count: 2_814,
  error_count_total: 24,
  business_limited_count: 11,
  error_count_sla: 13,
  request_count_total: 2_838,
  request_count_sla: 2_827,
  token_consumed: 3_612_400,
  sla: 0.9954,
  error_rate: 0.0085,
  upstream_error_rate: 0.0046,
  upstream_error_count_excl_429_529: 6,
  upstream_429_count: 5,
  upstream_529_count: 2,
  qps: { current: 0.82, peak: 2.44, avg: 0.74 },
  tps: { current: 642, peak: 1_920, avg: 586 },
  duration: { p50_ms: 842, p90_ms: 1_924, p95_ms: 2_486, p99_ms: 4_812, avg_ms: 1_284, max_ms: 8_420 },
  ttft: { p50_ms: 248, p90_ms: 516, p95_ms: 642, p99_ms: 1_104, avg_ms: 328, max_ms: 2_180 },
}

const opsThroughput = {
  bucket: 'minute',
  points: [
    { bucket_start: '2026-08-30T01:55:00Z', request_count: 38, token_consumed: 48_200, switch_count: 2, qps: 0.63, tps: 803 },
    { bucket_start: '2026-08-30T01:56:00Z', request_count: 44, token_consumed: 52_800, switch_count: 1, qps: 0.73, tps: 880 },
    { bucket_start: '2026-08-30T01:57:00Z', request_count: 49, token_consumed: 61_400, switch_count: 3, qps: 0.82, tps: 1_023 },
    { bucket_start: '2026-08-30T01:58:00Z', request_count: 41, token_consumed: 50_100, switch_count: 1, qps: 0.68, tps: 835 },
    { bucket_start: '2026-08-30T01:59:00Z', request_count: 46, token_consumed: 56_600, switch_count: 2, qps: 0.77, tps: 943 },
  ],
  by_platform: [
    { platform: 'openai', request_count: 1_426, token_consumed: 1_824_000 },
    { platform: 'anthropic', request_count: 924, token_consumed: 1_146_000 },
    { platform: 'antigravity', request_count: 488, token_consumed: 642_400 },
  ],
  top_groups: [
    { group_id: 11, group_name: 'OpenAI Production', request_count: 1_426, token_consumed: 1_824_000 },
    { group_id: 12, group_name: 'Claude Production', request_count: 924, token_consumed: 1_146_000 },
  ],
}

const advancedSettings = {
  data_retention: {
    cleanup_enabled: true,
    cleanup_schedule: '0 3 * * *',
    error_log_retention_days: 30,
    minute_metrics_retention_days: 7,
    hourly_metrics_retention_days: 90,
  },
  aggregation: { aggregation_enabled: true },
  openai_account_quota_auto_pause: { default_threshold_5h: 0.9, default_threshold_7d: 0.95 },
  ignore_count_tokens_errors: true,
  ignore_context_canceled: true,
  ignore_no_available_accounts: false,
  ignore_invalid_api_key_errors: true,
  ignore_insufficient_balance_errors: true,
  display_openai_token_stats: false,
  display_alert_events: false,
  auto_refresh_enabled: false,
  auto_refresh_interval_seconds: 30,
}

const adminSettings = {
  site_name: 'Sub2API',
  backend_mode_enabled: false,
  ops_monitoring_enabled: true,
  ops_realtime_monitoring_enabled: false,
  ops_query_mode_default: 'auto',
  payment_enabled: false,
  registration_enabled: false,
  password_reset_enabled: false,
  account_auto_pause_enabled: true,
  account_quota_notify_enabled: true,
  account_quota_notify_emails: ['ops@example.test'],
  openai_auto_warmup_enabled: false,
  custom_menu_items: [],
  default_subscriptions: [],
  forwarded_client_ip_headers: ['X-Forwarded-For'],
  login_agreement_documents: [],
  openai_fast_policy_settings: { rules: [] },
  registration_email_suffix_whitelist: [],
  table_page_size_options: [10, 20, 50, 100],
  default_platform_quotas: {
    anthropic: { daily: 40, weekly: 220, monthly: null },
    openai: { daily: 35, weekly: 180, monthly: null },
    gemini: { daily: 20, weekly: 100, monthly: null },
    antigravity: { daily: 20, weekly: 100, monthly: null },
    grok: { daily: null, weekly: null, monthly: null },
  },
}

const usageStats = {
  total_requests: 4_286,
  total_input_tokens: 3_812_400,
  total_output_tokens: 964_100,
  total_cache_tokens: 699_000,
  total_cache_creation_tokens: 84_200,
  total_cache_read_tokens: 614_800,
  total_tokens: 5_475_500,
  total_cost: 24.88,
  total_actual_cost: 19.43,
  total_account_cost: 12.71,
  average_duration_ms: 1_284,
  endpoints: [{ endpoint: '/v1/responses', requests: 3_612, total_tokens: 4_782_000, cost: 21.34, actual_cost: 16.81 }],
  upstream_endpoints: [{ endpoint: '/v1/responses', requests: 3_018, total_tokens: 3_994_000, cost: 17.42, actual_cost: 13.94 }],
  endpoint_paths: [{ endpoint: '/v1/responses -> /v1/responses', requests: 3_018, total_tokens: 3_994_000, cost: 17.42, actual_cost: 13.94 }],
}

export function isOperatorFixtureReadRequest(method: string, pathname: string): boolean {
  if (method === 'GET' || method === 'HEAD') return true
  return method === 'POST' && [
    '/api/v1/auth/login',
    '/api/v1/admin/accounts/today-stats/batch',
    '/api/v1/admin/accounts/usage/batch',
  ].includes(pathname)
}

export function getOperatorFixtureData(
  pathname: string,
  role: SessionRole = 'admin',
  runMode: RunMode = 'standard',
): unknown {
  if (pathname === '/api/v1/auth/me') return operatorFixtureUser(role, runMode)
  if (pathname === '/api/v1/auth/login') {
    return { access_token: OPERATOR_FIXTURE_TOKEN, expires_in: 3600, user: operatorFixtureUser('admin') }
  }
  if (pathname === '/api/v1/settings/public') return operatorFixturePublicSettings
  if (pathname === '/api/v1/admin/compliance') {
    return {
      required: false,
      version: 'fixture-review',
      document_path_zh: '',
      document_path_en: '',
      document_url_zh: '',
      document_url_en: '',
      ack_phrase_zh: '',
      ack_phrase_en: '',
    }
  }
  if (pathname === '/api/v1/admin/settings') return adminSettings
  if (pathname === '/api/v1/admin/system/check-updates') return operatorFixtureUpdateStatus
  if (pathname === '/api/v1/admin/payment/config') {
    return {
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
    }
  }
  if (pathname === '/api/v1/announcements' || pathname === '/api/v1/subscriptions/active') return []
  if (pathname === '/api/v1/keys') return paginated([], 100)

  if (pathname === '/api/v1/admin/dashboard/snapshot-v2') {
    return {
      generated_at: updatedAt,
      start_date: '2026-08-29T20:00:00Z',
      end_date: '2026-08-30T02:00:00Z',
      granularity: 'hour',
      stats: dashboardStats,
      trend: dashboardTrend,
      models: dashboardModels,
      groups: [
        { group_id: 11, group_name: 'OpenAI Production', requests: 2_142, total_tokens: 2_812_000, cost: 12.62, actual_cost: 9.84, account_cost: 5.98 },
        { group_id: 12, group_name: 'Claude Production', requests: 1_316, total_tokens: 1_732_000, cost: 8.44, actual_cost: 6.82, account_cost: 4.61 },
        { group_id: 13, group_name: 'Balanced Coding Route', requests: 828, total_tokens: 931_500, cost: 3.82, actual_cost: 2.77, account_cost: 2.12 },
      ],
      users_trend: [
        { date: '2026-08-30T01:00:00Z', user_id: 2, email: 'member@example.test', username: 'member', requests: 516, tokens: 682_000, cost: 3.42, actual_cost: 2.78 },
      ],
    }
  }
  if (pathname === '/api/v1/admin/dashboard/users-trend') {
    return { trend: [], start_date: '2026-08-29', end_date: '2026-08-30', granularity: 'hour' }
  }
  if (pathname === '/api/v1/admin/dashboard/users-ranking') {
    return {
      ranking: [
        { user_id: 2, email: 'member@example.test', username: 'member', actual_cost: 8.42, requests: 1_624, tokens: 2_212_000 },
        { user_id: 3, email: 'builds@example.test', username: 'builds', actual_cost: 6.18, requests: 1_142, tokens: 1_642_000 },
        { user_id: 4, email: 'automation@example.test', username: 'automation', actual_cost: 4.83, requests: 946, tokens: 1_218_000 },
      ],
      total_actual_cost: 19.43,
      total_requests: 4_286,
      total_tokens: 5_475_500,
      start_date: '2026-08-29',
      end_date: '2026-08-30',
    }
  }
  if (pathname === '/api/v1/admin/dashboard/models') {
    return { models: dashboardModels, start_date: '2026-08-29', end_date: '2026-08-30' }
  }

  if (pathname === '/api/v1/admin/accounts') return paginated(operatorFixtureAccounts)
  if (pathname === '/api/v1/admin/accounts/today-stats/batch') return { stats: todayStats }
  if (pathname === '/api/v1/admin/accounts/usage/batch') return { usage: accountUsage, errors: {} }
  const accountUsageMatch = pathname.match(/^\/api\/v1\/admin\/accounts\/(\d+)\/usage$/)
  if (accountUsageMatch) return accountUsage[accountUsageMatch[1] as keyof typeof accountUsage] ?? accountUsage['101']
  if (pathname === '/api/v1/admin/accounts/upstream-billing-probe/settings') return { enabled: true, interval_minutes: 30 }
  if (pathname === '/api/v1/admin/accounts/ollama-cloud-usage/settings') return { enabled: false, interval_minutes: 60, debounce_minutes: 5 }

  if (pathname === '/api/v1/admin/groups') return paginated(operatorFixtureGroups)
  if (pathname === '/api/v1/admin/groups/all') return operatorFixtureGroups
  if (pathname === '/api/v1/admin/groups/usage-summary') {
    return [
      { group_id: 11, today_cost: 9.84, yesterday_cost: 8.91, total_cost: 1_842.62 },
      { group_id: 12, today_cost: 6.82, yesterday_cost: 6.14, total_cost: 1_218.46 },
      { group_id: 13, today_cost: 2.77, yesterday_cost: 2.42, total_cost: 904.1 },
    ]
  }
  if (pathname === '/api/v1/admin/groups/capacity-summary') {
    return [
      { group_id: 11, concurrency_used: 3, concurrency_max: 12, sessions_used: 4, sessions_max: 16, rpm_used: 42, rpm_max: 240 },
      { group_id: 12, concurrency_used: 2, concurrency_max: 10, sessions_used: 3, sessions_max: 12, rpm_used: 28, rpm_max: 120 },
      { group_id: 13, concurrency_used: 5, concurrency_max: 22, sessions_used: 7, sessions_max: 28, rpm_used: 70, rpm_max: 360 },
    ]
  }
  if (pathname === '/api/v1/admin/groups/live-capability') return { supported: true }
  if (pathname === '/api/v1/admin/channels') return paginated(operatorFixtureChannels)
  if (pathname === '/api/v1/admin/proxies') return paginated(operatorFixtureProxies)
  if (pathname === '/api/v1/admin/proxies/all') return operatorFixtureProxies

  if (pathname === '/api/v1/admin/usage') return paginated(usageLogs)
  if (pathname === '/api/v1/admin/usage/stats') return usageStats
  if (pathname === '/api/v1/admin/usage/search-users') {
    return [{ id: 2, email: 'member@example.test', deleted: false }]
  }
  if (pathname === '/api/v1/admin/audit-logs') {
    return paginated([
      { id: 1, user_id: 1, action: 'account.test', resource_type: 'account', resource_id: '101', details: { result: 'healthy' }, ip_address: '192.0.2.1', user_agent: 'Fixture review', created_at: '2026-08-30T01:42:00Z', user: operatorFixtureUser() },
      { id: 2, user_id: 1, action: 'settings.update', resource_type: 'settings', resource_id: null, details: { section: 'operations' }, ip_address: '192.0.2.1', user_agent: 'Fixture review', created_at: '2026-08-30T00:16:00Z', user: operatorFixtureUser() },
    ])
  }

  if (pathname === '/api/v1/admin/ops/dashboard/snapshot-v2') {
    return {
      generated_at: updatedAt,
      overview: opsOverview,
      throughput_trend: opsThroughput,
      error_trend: {
        bucket: 'minute',
        points: opsThroughput.points.map((point, index) => ({
          bucket_start: point.bucket_start,
          request_error_count: index % 2,
          upstream_error_count: index === 2 ? 2 : 1,
          business_limited_count: index === 3 ? 1 : 0,
          request_count: point.request_count,
        })),
      },
    }
  }
  if (pathname === '/api/v1/admin/ops/dashboard/overview') return opsOverview
  if (pathname === '/api/v1/admin/ops/dashboard/throughput-trend') return opsThroughput
  if (pathname === '/api/v1/admin/ops/dashboard/switch-rate-trend') {
    return { bucket: 'minute', points: opsThroughput.points }
  }
  if (pathname === '/api/v1/admin/ops/dashboard/latency-histogram') {
    return {
      start_time: opsOverview.start_time,
      end_time: opsOverview.end_time,
      platform: '',
      total_requests: 2_838,
      buckets: [
        { range: '<500ms', count: 614 },
        { range: '500ms-1s', count: 1_182 },
        { range: '1s-2s', count: 742 },
        { range: '2s-5s', count: 264 },
        { range: '>5s', count: 36 },
      ],
    }
  }
  if (pathname === '/api/v1/admin/ops/dashboard/error-trend') {
    return { bucket: 'minute', points: [] }
  }
  if (pathname === '/api/v1/admin/ops/dashboard/error-distribution') {
    return {
      total: 24,
      items: [
        { key: 'rate_limit', label: 'Rate limit', count: 11, percentage: 45.8 },
        { key: 'upstream', label: 'Upstream', count: 8, percentage: 33.3 },
        { key: 'client', label: 'Client request', count: 5, percentage: 20.9 },
      ],
    }
  }
  if (pathname === '/api/v1/admin/ops/advanced-settings') return advancedSettings
  if (pathname === '/api/v1/admin/ops/settings/metric-thresholds') {
    return { sla_percent_min: 99, ttft_p99_ms_max: 1500, request_error_rate_percent_max: 2, upstream_error_rate_percent_max: 1 }
  }
  if (pathname === '/api/v1/admin/ops/runtime/logging') {
    return { level: 'info', enable_sampling: true, sampling_initial: 100, sampling_thereafter: 100, caller: false, stacktrace_level: 'error', retention_days: 14, source: 'fixture' }
  }
  if (pathname === '/api/v1/admin/ops/system-logs/health') {
    return { queue_depth: 2, queue_capacity: 1000, dropped_count: 0, write_failed_count: 0, written_count: 1842, avg_write_delay_ms: 4 }
  }
  if (pathname === '/api/v1/admin/ops/system-logs') {
    return paginated([
      { id: 1, created_at: '2026-08-30T01:47:00Z', host: 'gateway-fixture-1', level: 'info', component: 'scheduler', message: 'Selected healthy account for request', request_id: 'req_fixture_1', account_id: 101, platform: 'openai', model: 'gpt-5.2-codex' },
      { id: 2, created_at: '2026-08-30T01:43:00Z', host: 'gateway-fixture-1', level: 'warn', component: 'quota', message: 'Account entered provider cooldown', request_id: 'req_fixture_3', account_id: 103, platform: 'antigravity', model: 'gemini-2.5-pro' },
    ])
  }

  if (pathname === '/api/v1/admin/settings/admin-api-key') return { exists: true, masked_key: 's2a_review_****' }
  if (pathname === '/api/v1/admin/settings/web-search-emulation') {
    return { enabled: true, providers: [{ provider: 'openai', enabled: true, model: 'gpt-5.2' }] }
  }
  if (pathname === '/api/v1/admin/settings/email-templates') {
    return { events: [], locales: ['en'], templates: [], placeholders: [] }
  }
  if (pathname === '/api/v1/admin/settings/beta-policy') return { rules: [] }
  if (pathname === '/api/v1/admin/settings/overload-cooldown') return { enabled: true, cooldown_minutes: 5 }
  if (pathname === '/api/v1/admin/settings/rate-limit-429-cooldown') return { enabled: true, cooldown_seconds: 60 }
  if (pathname === '/api/v1/admin/settings/panel-rate-limit') {
    return { enabled: true, user_rpm: 120, heavy_rpm: 20, exempt_admin: true, public_ip_rpm: 60 }
  }
  if (pathname === '/api/v1/admin/settings/stream-timeout') {
    return { enabled: true, action: 'temp_unschedulable', temp_unsched_minutes: 5, threshold_count: 3, threshold_window_minutes: 10 }
  }
  if (pathname === '/api/v1/admin/settings/rectifier') {
    return { enabled: true, thinking_signature_enabled: true, thinking_budget_enabled: true, apikey_signature_enabled: false, apikey_signature_patterns: [] }
  }
  if (pathname === '/api/v1/admin/payment/providers') return []
  if (pathname === '/api/v1/admin/backups/s3-config') {
    return { endpoint: '', region: 'auto', bucket: '', access_key_id: '', prefix: 'backups/', force_path_style: false, secret_configured: false }
  }
  if (pathname === '/api/v1/admin/backups/image-storage') {
    return { config: { enabled: false, reuse_backup_s3: true, bucket: '', prefix: 'images/', public_base_url: '', presign_expiry_hours: 24, max_download_bytes: 33_554_432, endpoint: '', region: 'auto', access_key_id: '', force_path_style: false }, secret_configured: false }
  }
  if (pathname === '/api/v1/admin/backups/schedule') {
    return { enabled: true, cron_expr: '0 2 * * *', retain_days: 14, retain_count: 10 }
  }
  if (pathname === '/api/v1/admin/backups') return { items: [] }

  return {}
}
