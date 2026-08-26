export type LabPage = 'overview' | 'accounts' | 'models' | 'settings'
export type PrototypeKey = 'a' | 'b' | 'c' | 'd'
export type ProviderId = 'openai' | 'claude' | 'gemini' | 'antigravity'
export type Health = 'healthy' | 'degraded' | 'critical'

export interface LabAccount {
  id: number
  provider: ProviderId
  label: string
  identity: string
  plan: string
  health: Health
  healthLabel: string
  schedulable: boolean
  scheduleLabel: string
  remaining: number | null
  consumed: number | null
  nativeQuota: string
  resetLabel: string
  resetBucket: '<1h' | '1-4h' | '>4h' | 'unknown'
  models: string[]
  groups: string[]
  concurrency: string
  requests: number
  tokens: string
  cost: string
  recentFailure: string | null
}

export const labPages: Array<{ key: LabPage; label: string }> = [
  { key: 'overview', label: 'Overview' },
  { key: 'accounts', label: 'Accounts' },
  { key: 'models', label: 'Models & Routing' },
  { key: 'settings', label: 'Settings' },
]

export const prototypes: Array<{ key: PrototypeKey; name: string; descriptor: string }> = [
  { key: 'a', name: 'Codex-LB Dense', descriptor: 'compact / mechanical / restrained' },
  { key: 'b', name: 'Data Console', descriptor: 'flat / forensic / exact' },
  { key: 'c', name: 'Operations Cockpit', descriptor: 'live / watchful / decisive' },
  { key: 'd', name: 'Sub2API Hybrid', descriptor: 'balanced / specific / composed' },
]

export const labAccounts: LabAccount[] = [
  {
    id: 101,
    provider: 'openai',
    label: 'Codex Team West',
    identity: 'codex-west@example.test',
    plan: 'Team',
    health: 'healthy',
    healthLabel: 'Healthy',
    schedulable: true,
    scheduleLabel: 'Routing',
    remaining: 82,
    consumed: 18,
    nativeQuota: '5h window: 18% used',
    resetLabel: '2h 06m',
    resetBucket: '1-4h',
    models: ['gpt-5.2-codex', 'gpt-5.2', 'o4-mini'],
    groups: ['OpenAI Production', 'Balanced Coding Route'],
    concurrency: '3 / 8',
    requests: 842,
    tokens: '2.48M',
    cost: '$6.24',
    recentFailure: null,
  },
  {
    id: 106,
    provider: 'openai',
    label: 'Codex Batch East',
    identity: 'codex-east@example.test',
    plan: 'Team',
    health: 'degraded',
    healthLabel: 'Rate limited',
    schedulable: true,
    scheduleLabel: 'Fallback only',
    remaining: 18,
    consumed: 82,
    nativeQuota: '5h window: 82% used',
    resetLabel: '47m',
    resetBucket: '<1h',
    models: ['gpt-5.2-codex', 'gpt-5.2'],
    groups: ['OpenAI Production'],
    concurrency: '1 / 6',
    requests: 318,
    tokens: '914K',
    cost: '$2.18',
    recentFailure: '4 upstream 429s in 15m',
  },
  {
    id: 102,
    provider: 'claude',
    label: 'Claude Primary',
    identity: 'claude-primary@example.test',
    plan: 'Max',
    health: 'healthy',
    healthLabel: 'Healthy',
    schedulable: true,
    scheduleLabel: 'Routing',
    remaining: 41,
    consumed: 59,
    nativeQuota: '5h window: 59% used',
    resetLabel: '2h 06m',
    resetBucket: '1-4h',
    models: ['claude-sonnet-4-5', 'claude-opus-4-1'],
    groups: ['Claude Production', 'Balanced Coding Route'],
    concurrency: '2 / 6',
    requests: 516,
    tokens: '1.13M',
    cost: '$8.72',
    recentFailure: null,
  },
  {
    id: 107,
    provider: 'claude',
    label: 'Claude Overflow',
    identity: 'claude-overflow@example.test',
    plan: 'API',
    health: 'degraded',
    healthLabel: 'Telemetry stale',
    schedulable: false,
    scheduleLabel: 'Paused until 20:15',
    remaining: null,
    consumed: null,
    nativeQuota: 'No quota telemetry',
    resetLabel: 'Probe pending',
    resetBucket: 'unknown',
    models: ['claude-sonnet-4-5'],
    groups: ['Claude Production'],
    concurrency: '0 / 4',
    requests: 127,
    tokens: '286K',
    cost: '$1.92',
    recentFailure: 'Usage probe timed out 11m ago',
  },
  {
    id: 104,
    provider: 'gemini',
    label: 'Gemini Recovery',
    identity: 'gemini-recovery@example.test',
    plan: 'Google AI Pro',
    health: 'critical',
    healthLabel: 'Reauth required',
    schedulable: false,
    scheduleLabel: 'Excluded',
    remaining: 12,
    consumed: 88,
    nativeQuota: 'Shared daily: 88% used',
    resetLabel: '5h 06m',
    resetBucket: '>4h',
    models: ['gemini-2.5-pro', 'gemini-2.5-flash'],
    groups: ['Balanced Coding Route'],
    concurrency: '0 / 4',
    requests: 34,
    tokens: '76K',
    cost: '$0.26',
    recentFailure: 'OAuth refresh rejected 22m ago',
  },
  {
    id: 108,
    provider: 'gemini',
    label: 'Gemini Pro North',
    identity: 'gemini-north@example.test',
    plan: 'Google AI Pro',
    health: 'healthy',
    healthLabel: 'Healthy',
    schedulable: true,
    scheduleLabel: 'Routing',
    remaining: 64,
    consumed: 36,
    nativeQuota: 'Shared daily: 36% used',
    resetLabel: '5h 06m',
    resetBucket: '>4h',
    models: ['gemini-2.5-pro', 'gemini-2.5-flash'],
    groups: ['Balanced Coding Route', 'Fast Multimodal'],
    concurrency: '2 / 8',
    requests: 714,
    tokens: '778K',
    cost: '$2.71',
    recentFailure: null,
  },
  {
    id: 103,
    provider: 'antigravity',
    label: 'Antigravity Pro',
    identity: 'antigravity@example.test',
    plan: 'Pro',
    health: 'healthy',
    healthLabel: 'Healthy',
    schedulable: true,
    scheduleLabel: 'Routing',
    remaining: 96,
    consumed: 4,
    nativeQuota: 'Model pool: 4% used',
    resetLabel: '5h 06m',
    resetBucket: '>4h',
    models: ['gemini-2.5-pro', 'claude-sonnet-4-5'],
    groups: ['Balanced Coding Route'],
    concurrency: '0 / 4',
    requests: 294,
    tokens: '682K',
    cost: '$2.31',
    recentFailure: null,
  },
  {
    id: 109,
    provider: 'antigravity',
    label: 'Antigravity Burst',
    identity: 'antigravity-burst@example.test',
    plan: 'Pro',
    health: 'degraded',
    healthLabel: 'Quota exhausted',
    schedulable: false,
    scheduleLabel: 'Paused until reset',
    remaining: 0,
    consumed: 100,
    nativeQuota: 'Claude pool: exhausted',
    resetLabel: '1h 12m',
    resetBucket: '1-4h',
    models: ['claude-sonnet-4-5'],
    groups: ['Balanced Coding Route'],
    concurrency: '0 / 4',
    requests: 192,
    tokens: '418K',
    cost: '$1.44',
    recentFailure: 'Capacity guard paused routing 8m ago',
  },
]

const providerMeta: Array<{ id: ProviderId; label: string; short: string }> = [
  { id: 'openai', label: 'OpenAI', short: 'OA' },
  { id: 'claude', label: 'Claude', short: 'CL' },
  { id: 'gemini', label: 'Gemini', short: 'GE' },
  { id: 'antigravity', label: 'Antigravity', short: 'AG' },
]

export const labProviders = providerMeta.map((provider) => {
  const accounts = labAccounts.filter((account) => account.provider === provider.id)
  const knownCapacity = accounts.flatMap((account) => account.remaining === null ? [] : [account.remaining])
  return {
    ...provider,
    accounts,
    knownAccounts: knownCapacity.length,
    averageRemaining: Math.round(knownCapacity.reduce((sum, value) => sum + value, 0) / knownCapacity.length),
    health: accounts.some((account) => account.health === 'critical')
      ? 'critical' as const
      : accounts.some((account) => account.health === 'degraded')
        ? 'degraded' as const
        : 'healthy' as const,
  }
})

const routableAccountCount = labAccounts.filter((account) => account.schedulable).length
const requestCount = 4_286
const exceptionCount = 20
const upstreamExceptionCount = 6

export const labRoutes = [
  {
    name: 'coding-default',
    type: 'Composite',
    status: 'healthy' as const,
    destinations: ['gpt-5.2-codex', 'claude-sonnet-4-5', 'gemini-2.5-pro'],
    groups: ['OpenAI Production', 'Claude Production', 'Balanced Coding Route'],
    success: '99.54%',
    latency: '1.28s',
    rpm: 42,
    capacity: `4 providers / ${routableAccountCount} routable accounts`,
  },
  {
    name: 'reasoning-long',
    type: 'Composite',
    status: 'degraded' as const,
    destinations: ['claude-opus-4-1', 'gpt-5.2'],
    groups: ['Claude Production', 'OpenAI Production'],
    success: '98.91%',
    latency: '2.84s',
    rpm: 11,
    capacity: '2 providers / 3 routable accounts',
  },
  {
    name: 'fast-multimodal',
    type: 'Direct',
    status: 'healthy' as const,
    destinations: ['gemini-2.5-flash'],
    groups: ['Fast Multimodal'],
    success: '99.82%',
    latency: '0.74s',
    rpm: 18,
    capacity: '1 provider / 1 routable account',
  },
]

export const labModels = [
  { name: 'gpt-5.2-codex', provider: 'OpenAI', route: 'coding-default', status: 'healthy' as const, requests: '1,842', tokens: '2.77M', latency: '1.12s' },
  { name: 'claude-sonnet-4-5', provider: 'Claude + Antigravity', route: 'coding-default', status: 'healthy' as const, requests: '1,316', tokens: '1.73M', latency: '1.46s' },
  { name: 'gemini-2.5-pro', provider: 'Gemini + Antigravity', route: 'coding-default', status: 'degraded' as const, requests: '714', tokens: '778K', latency: '0.91s' },
  { name: 'gemini-2.5-flash', provider: 'Gemini', route: 'fast-multimodal', status: 'healthy' as const, requests: '392', tokens: '412K', latency: '0.74s' },
]

export const labActivity = [
  { time: '18:59:42', route: 'coding-default', model: 'gpt-5.2-codex', account: 'Codex Team West', duration: '1.48s', status: '200' },
  { time: '18:59:31', route: 'coding-default', model: 'claude-sonnet-4-5', account: 'Claude Primary', duration: '1.11s', status: '200' },
  { time: '18:59:18', route: 'coding-default', model: 'gemini-2.5-pro', account: 'Antigravity Pro', duration: '0.91s', status: '200' },
  { time: '18:58:54', route: 'reasoning-long', model: 'gpt-5.2', account: 'Codex Batch East', duration: '4.82s', status: '429' },
  { time: '18:58:37', route: 'fast-multimodal', model: 'gemini-2.5-flash', account: 'Gemini Pro North', duration: '0.72s', status: '200' },
]

export const labRequestTrend = [
  { time: '18:30', requests: 31, errors: 1 },
  { time: '18:35', requests: 38, errors: 0 },
  { time: '18:40', requests: 35, errors: 2 },
  { time: '18:45', requests: 46, errors: 1 },
  { time: '18:50', requests: 41, errors: 0 },
  { time: '18:55', requests: 49, errors: 1 },
  { time: '19:00', requests: 42, errors: 0 },
]

export const labSettings = [
  { title: 'Capacity-aware routing', description: 'Prefer accounts with more normalized capacity remaining.', value: 'Enabled' },
  { title: 'Automatic quota pause', description: 'Pause exhausted accounts until their provider reset.', value: 'Enabled' },
  { title: 'Cross-provider failover', description: 'Use composite routes when the preferred pool is unavailable.', value: 'Enabled' },
  { title: 'Capacity warning', description: 'Notify operators when a known account falls below 20%.', value: '20%' },
  { title: 'Fixture refresh', description: 'This prototype snapshot is intentionally fixed for comparison.', value: 'Manual' },
]

export const labSummary = {
  period: '24H',
  requests: requestCount.toLocaleString('en-US'),
  rpm: '42',
  tokens: '5.48M',
  cost: '$19.43',
  accountCost: '$12.71',
  success: `${(((requestCount - exceptionCount) / requestCount) * 100).toFixed(2)}%`,
  successfulRequests: (requestCount - exceptionCount).toLocaleString('en-US'),
  errors: exceptionCount.toLocaleString('en-US'),
  upstreamErrors: upstreamExceptionCount.toLocaleString('en-US'),
  latency: '1.28s',
  routableAccounts: routableAccountCount,
  totalAccounts: labAccounts.length,
}
