<template>
  <AppLayout>
    <div v-if="loading" class="viz-state" role="status">
      <LoadingSpinner />
    </div>

    <div v-else class="viz-grid overview">
      <!-- A failed snapshot replaces only the status card; accounts and requests still load. -->
      <section v-if="snapshotError && !stats" class="viz-card viz-span-12 viz-state" role="alert" data-testid="dashboard-load-error">
        <Icon name="exclamationCircle" size="xl" class="text-red-500" />
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.dashboard.failedToLoad') }}</h2>
        <button type="button" class="btn btn-primary" @click="loadSnapshot()">
          <Icon name="refresh" size="sm" />
          <span>{{ t('admin.dashboard.retry') }}</span>
        </button>
      </section>

      <section v-if="stats" class="viz-card viz-span-12 overview-hero" aria-labelledby="overview-status">
        <div class="overview-hero-grid">
          <div>
            <h2 id="overview-status" class="overview-status">
              <span class="viz-pulse" :class="statusTone" aria-hidden="true"></span>
              {{ hasTraffic ? t('admin.viz.status.serving') : t('admin.viz.status.idle') }}
            </h2>

            <ul v-if="providers.length" class="overview-providers">
              <li v-for="provider in providers" :key="provider.platform">
                <RouterLink
                  to="/admin/accounts"
                  class="overview-provider"
                  :title="`${providerName(provider.platform)} · ${t('admin.viz.serving', { serving: provider.serving, total: provider.total })}`"
                  :aria-label="`${providerName(provider.platform)}: ${t('admin.viz.serving', { serving: provider.serving, total: provider.total })}`"
                >
                  <ProviderMark :platform="provider.platform" :size="30" />
                  <i :class="`is-${provider.state}`"></i>
                </RouterLink>
              </li>
            </ul>

            <dl class="overview-today">
              <div>
                <dt>{{ t('admin.viz.requestsToday') }}</dt>
                <dd>{{ formatInt(stats.today_requests) }}</dd>
              </div>
              <div>
                <dt>{{ t('admin.viz.tokens') }}</dt>
                <dd>{{ formatCompact(stats.today_tokens) }}</dd>
              </div>
              <div>
                <dt>{{ t('admin.viz.cost') }}</dt>
                <dd>{{ formatMoney(stats.today_actual_cost) }}</dd>
              </div>
              <div>
                <dt>{{ t('admin.viz.avgResponse') }}</dt>
                <dd>{{ formatDuration(stats.average_duration_ms) }}</dd>
              </div>
            </dl>
          </div>

          <div>
            <div class="overview-live">
              <b>{{ formatInt(stats.rpm) }}</b>
              <span>{{ t('admin.viz.requestsPerMin') }}</span>
              <RouterLink to="/admin/stats" class="viz-link">
                {{ t('nav.stats') }}
                <Icon name="arrowRight" size="xs" />
              </RouterLink>
            </div>
            <TrendChart
              :labels="hourly.labels"
              :tips="hourly.tips"
              :series="[{ name: t('admin.viz.requests'), color: 'var(--operator-accent-text)', values: hourly.values }]"
              :summary="t('admin.viz.hourlyRequests')"
              :height="150"
              :format-value="formatInt"
            />
          </div>
        </div>
      </section>

      <section class="viz-card viz-span-5" aria-labelledby="overview-attention">
        <header class="viz-card-head">
          <h2 id="overview-attention" class="viz-card-title">
            {{ t('admin.viz.needsAttention') }}
            <span v-if="attention.length" class="viz-count">{{ attention.length }}</span>
          </h2>
          <div class="viz-card-tools overview-states">
            <RouterLink
              v-for="state in stateLinks"
              :key="state.status"
              :to="accountStatusLink(state.status)"
              class="badge"
              :class="state.badge"
              :data-testid="`overview-state-${state.status}`"
            >
              {{ fleet.loading.value ? '–' : state.count }} {{ t(`admin.viz.states.${state.status}`) }}
            </RouterLink>
          </div>
        </header>
        <div class="viz-card-body">
          <div v-if="fleet.error.value" class="viz-state" role="alert" data-testid="overview-fleet-error">
            <span>{{ t('admin.viz.loadFailed') }}</span>
            <button type="button" class="btn btn-secondary" @click="fleet.load">{{ t('admin.viz.retry') }}</button>
          </div>
          <div v-else-if="fleet.loading.value && !fleet.accounts.value.length" class="viz-state" role="status">
            <LoadingSpinner size="sm" />
          </div>
          <div v-else-if="!attention.length" class="viz-state">
            <Icon name="checkCircle" size="lg" class="text-green-600" />
            <span>{{ t('admin.viz.allHealthy') }}</span>
          </div>
          <ul v-else class="overview-attention">
            <li v-for="item in attention.slice(0, 6)" :key="item.id">
              <RouterLink
                :to="accountStatusLink(item.status)"
                class="overview-attention-item"
                :aria-label="t('admin.viz.openAccount', { name: item.name })"
              >
                <ProviderMark :platform="item.platform" :size="32" />
                <span class="overview-attention-text">
                  <b>{{ item.name }}</b>
                  <small>{{ item.reason }}</small>
                </span>
                <span class="badge" :class="item.status === 'error' ? 'badge-danger' : 'badge-warning'">
                  {{ t(`admin.viz.states.${item.status}`) }}
                </span>
                <Icon name="chevronRight" size="sm" class="text-gray-400" />
              </RouterLink>
            </li>
          </ul>
        </div>
      </section>

      <section class="viz-card viz-span-7" aria-labelledby="overview-capacity">
        <header class="viz-card-head">
          <h2 id="overview-capacity" class="viz-card-title">{{ t('admin.viz.capacityLeft') }}</h2>
          <div class="viz-card-tools">
            <RouterLink to="/admin/stats?view=capacity" class="viz-link">
              {{ t('nav.stats') }}
              <Icon name="arrowRight" size="xs" />
            </RouterLink>
          </div>
        </header>
        <div class="viz-card-body">
          <div v-if="fleet.loading.value && !providers.length" class="viz-state" role="status">
            <LoadingSpinner size="sm" />
          </div>
          <div v-else-if="!providers.length" class="viz-state">{{ t('admin.viz.noData') }}</div>
          <ul v-else class="overview-gauges">
            <li v-for="provider in providers" :key="provider.platform">
              <RingGauge :value="provider.remaining" :label="providerName(provider.platform)" />
              <span class="overview-gauge-name">
                <ProviderMark :platform="provider.platform" :size="20" />
                {{ providerName(provider.platform) }}
              </span>
              <small>{{ t('admin.viz.accountsCount', { count: provider.total }, provider.total) }}</small>
            </li>
          </ul>
        </div>
      </section>

      <section class="viz-card viz-span-12" aria-labelledby="overview-recent">
        <header class="viz-card-head">
          <h2 id="overview-recent" class="viz-card-title">{{ t('admin.viz.recentRequests') }}</h2>
          <div class="viz-card-tools">
            <RouterLink to="/admin/usage" class="viz-link">
              {{ t('admin.viz.allActivity') }}
              <Icon name="arrowRight" size="xs" />
            </RouterLink>
          </div>
        </header>
        <div class="viz-card-body is-flush">
          <div v-if="!recent.length" class="viz-state">{{ t('admin.viz.noRequests') }}</div>
          <div v-else class="overflow-x-auto">
            <table class="overview-table">
              <thead>
                <tr>
                  <th>{{ t('admin.viz.columns.model') }}</th>
                  <th>{{ t('admin.viz.columns.account') }}</th>
                  <th class="is-number">{{ t('admin.viz.columns.tokens') }}</th>
                  <th class="is-number">{{ t('admin.viz.columns.duration') }}</th>
                  <th class="is-number">{{ t('admin.viz.cost') }}</th>
                  <th class="is-number">{{ t('admin.viz.columns.when') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="row in recent" :key="row.id">
                  <td>
                    <span class="overview-model">
                      <ProviderMark :platform="providerOfModel(row.model)" :size="22">
                        <Icon name="cube" size="xs" />
                      </ProviderMark>
                      <code>{{ row.model }}</code>
                    </span>
                  </td>
                  <td>{{ row.account?.name ?? '–' }}</td>
                  <td class="is-number">{{ formatInt(tokensOf(row)) }}</td>
                  <td class="is-number">{{ row.duration_ms == null ? '–' : formatDuration(row.duration_ms) }}</td>
                  <td class="is-number">${{ Number(row.actual_cost ?? 0).toFixed(4) }}</td>
                  <td class="is-number is-muted">{{ timeAgo(row.created_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, shallowRef } from 'vue'
import { useDocumentVisibility, useIntervalFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { AdminUsageLog, DashboardStats, TrendDataPoint } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import RingGauge from '@/components/viz/RingGauge.vue'
import TrendChart from '@/components/viz/TrendChart.vue'
import { useAccountFleet } from '@/composables/useAccountFleet'
import { buildProviderCapacity, classifyOperatorAccount, type OperatorAccountStatus } from '@/utils/operatorCapacity'
import { providerName, providerOfModel } from '@/utils/providerSeries'
import { addDays, addHours, bucketTrend, localDateString, slotStarts } from '@/utils/trendBuckets'
import { formatCompact, formatDuration, formatInt, formatMoney } from '@/utils/vizFormat'

const REFRESH_MS = 60_000
const RECENT_ROWS = 6

const { t, locale } = useI18n()
const appStore = useAppStore()
const visibility = useDocumentVisibility()
const fleet = useAccountFleet(() => t('admin.stats.capacity.snapshotFailed'))

const stats = ref<DashboardStats | null>(null)
const trend = shallowRef<TrendDataPoint[]>([])
const recent = shallowRef<AdminUsageLog[]>([])
const loading = ref(false)
const snapshotError = ref(false)
const now = ref(new Date())
let snapshotSequence = 0

const hasTraffic = computed(() => (stats.value?.rpm ?? 0) > 0 || (stats.value?.tpm ?? 0) > 0)

const providerCapacity = computed(() => buildProviderCapacity(
  fleet.accounts.value,
  fleet.usageByAccountId.value,
  fleet.errorsByAccountId.value,
  now.value,
))

const classified = computed(() => providerCapacity.value.flatMap((provider) => provider.accounts.map((summary) => ({
  summary,
  platform: provider.platform,
  info: classifyOperatorAccount(summary),
}))))

const statusCounts = computed(() => classified.value.reduce<Record<OperatorAccountStatus, number>>(
  (counts, item) => {
    counts[item.info.status] += 1
    return counts
  },
  { active: 0, limited: 0, error: 0, disabled: 0 },
))

const stateLinks = computed(() => ([
  { status: 'active', badge: 'badge-success' },
  { status: 'limited', badge: 'badge-warning' },
  { status: 'error', badge: 'badge-danger' },
] as const).map((state) => ({ ...state, count: statusCounts.value[state.status] })))

// Errors first, then limits; a disabled account is a choice, not a problem.
const attention = computed(() => classified.value
  .filter((item) => item.info.status === 'error' || item.info.status === 'limited')
  .sort((left, right) => Number(right.info.status === 'error') - Number(left.info.status === 'error'))
  .map((item) => ({
    id: item.summary.account.id,
    name: item.summary.account.name,
    platform: item.platform,
    status: item.info.status,
    reason: item.info.reason ?? '',
  })))

const providers = computed(() => providerCapacity.value.map((provider) => {
  const states = provider.accounts.map((summary) => classifyOperatorAccount(summary).status)
  const serving = states.filter((status) => status === 'active').length
  const trouble = states.filter((status) => status === 'error' || status === 'limited').length
  return {
    platform: provider.platform,
    total: provider.accounts.length,
    serving,
    remaining: provider.remainingPercent,
    state: !trouble ? 'ok' : serving ? 'warn' : 'bad',
  }
}))

const statusTone = computed(() => {
  if (!hasTraffic.value) return 'is-idle'
  return statusCounts.value.error ? 'is-warn' : ''
})

// The last 24 hours as complete hourly slots; the API omits hours with no traffic.
const hourly = computed(() => {
  const end = new Date(now.value)
  end.setMinutes(0, 0, 0)
  const starts = slotStarts(addHours(end, -23), 24, 1)
  const buckets = bucketTrend(trend.value, starts, addHours(end, 1))
  const hour = new Intl.DateTimeFormat(locale.value, { hour: 'numeric' })
  const full = new Intl.DateTimeFormat(locale.value, { weekday: 'short', hour: 'numeric' })
  return {
    labels: buckets.map((bucket) => hour.format(bucket.start)),
    tips: buckets.map((bucket) => full.format(bucket.start)),
    values: buckets.map((bucket) => bucket.requests),
  }
})

const accountStatusLink = (status: OperatorAccountStatus) => ({
  path: '/admin/accounts',
  query: { operator_status: status },
})

const tokensOf = (row: AdminUsageLog) => row.input_tokens + row.output_tokens + row.cache_creation_tokens + row.cache_read_tokens

function timeAgo(value: string): string {
  const seconds = Math.max(0, Math.round((now.value.getTime() - new Date(value).getTime()) / 1000))
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.round(seconds / 60)}m`
  if (seconds < 86_400) return `${Math.round(seconds / 3600)}h`
  return `${Math.round(seconds / 86_400)}d`
}

async function loadSnapshot(silent = false) {
  const sequence = ++snapshotSequence
  if (!stats.value) loading.value = true
  if (!silent) snapshotError.value = false
  try {
    const today = new Date()
    const response = await adminAPI.dashboard.getSnapshotV2({
      start_date: localDateString(addDays(today, -1)),
      end_date: localDateString(today),
      granularity: 'hour',
      include_stats: true,
      include_trend: true,
      include_model_stats: false,
      include_group_stats: false,
      include_users_trend: false,
    })
    if (sequence !== snapshotSequence) return
    if (response.stats) stats.value = response.stats
    trend.value = response.trend ?? []
    // Anchor on the server's clock so the hourly window matches its buckets.
    const generated = new Date(response.generated_at)
    now.value = Number.isNaN(generated.getTime()) ? new Date() : generated
  } catch (error) {
    if (sequence !== snapshotSequence) return
    // A background refresh keeps the last good numbers on screen.
    if (!silent) {
      snapshotError.value = true
      appStore.showError(t('admin.dashboard.failedToLoad'))
    }
    console.error('Error loading dashboard snapshot:', error)
  } finally {
    if (sequence === snapshotSequence) loading.value = false
  }
}

async function loadRecent() {
  try {
    const response = await adminAPI.usage.list({ page: 1, page_size: RECENT_ROWS })
    recent.value = response.items.slice(0, RECENT_ROWS)
  } catch (error) {
    // The table is secondary; the rest of the page stays usable.
    console.error('Error loading recent requests:', error)
  }
}

async function loadAll() {
  await Promise.all([loadSnapshot(), fleet.load(), loadRecent()])
}

useIntervalFn(() => {
  if (visibility.value !== 'visible') return
  void loadSnapshot(true)
  void loadRecent()
}, REFRESH_MS)

onMounted(loadAll)
</script>

<style scoped>
.overview-hero {
  overflow: hidden;
}

.overview-hero::before {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: radial-gradient(60% 120% at 0% 0%, rgb(var(--operator-accent-rgb) / 0.28), transparent 70%);
  content: '';
  opacity: var(--operator-glow);
  pointer-events: none;
}

.overview-hero-grid {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.25fr);
  align-items: center;
  gap: 1.5rem;
  padding: 1.25rem;
}

.overview-status {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  color: var(--operator-foreground);
  font-size: 1.375rem;
  font-weight: 600;
  letter-spacing: -0.015em;
}

.overview-providers {
  display: flex;
  flex-wrap: wrap;
  gap: 0.625rem;
  margin-top: 0.875rem;
}

.overview-provider {
  position: relative;
  display: inline-flex;
  border-radius: 9px;
  transition: transform var(--operator-duration-fast) var(--operator-ease-out);
}

.overview-provider:hover {
  transform: translateY(-2px);
}

.overview-provider i {
  position: absolute;
  right: -3px;
  bottom: -3px;
  box-sizing: content-box;
  width: 10px;
  height: 10px;
  border: 2px solid var(--operator-card);
  border-radius: 50%;
  background: var(--operator-success-fill);
}

.overview-provider i.is-warn { background: var(--operator-warning-fill); }
.overview-provider i.is-bad { background: var(--operator-destructive-fill); }

.overview-today {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 2rem;
  margin-top: 1.25rem;
}

.overview-today dt {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.overview-today dd {
  color: var(--operator-foreground);
  font-size: 1.5rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.02em;
  line-height: 1.25;
}

.overview-live {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  margin-bottom: 0.25rem;
  color: var(--operator-muted-foreground);
  font-size: 0.78rem;
}

.overview-live b {
  color: var(--operator-foreground);
  font-size: 1.75rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.03em;
}

.overview-live .viz-link {
  margin-left: auto;
}

.overview-states {
  gap: 0.375rem;
}

.overview-attention li + li {
  border-top: 1px solid var(--operator-border);
}

.overview-attention-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin: 0 -0.5rem;
  padding: 0.625rem 0.5rem;
  border-radius: var(--operator-radius);
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.overview-attention-item:hover {
  background: var(--operator-muted);
}

.overview-attention-text {
  display: grid;
  min-width: 0;
  flex: 1;
}

.overview-attention-text b {
  overflow: hidden;
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.overview-attention-text small {
  overflow: hidden;
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.overview-gauges {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(7rem, 1fr));
  gap: 0.75rem;
}

.overview-gauges li {
  display: grid;
  justify-items: center;
  gap: 0.5rem;
  padding: 0.75rem 0.5rem;
  border-radius: var(--operator-radius);
}

.overview-gauges small {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.overview-gauge-name {
  display: flex;
  align-items: center;
  gap: 0.4375rem;
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  font-weight: 500;
  white-space: nowrap;
}

.overview-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}

.overview-table th {
  padding: 0.5625rem 1.125rem;
  border-bottom: 1px solid var(--operator-border);
  background: var(--table-header-background);
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
  font-weight: 500;
  text-align: left;
  white-space: nowrap;
}

.overview-table td {
  padding: 0.625rem 1.125rem;
  border-bottom: 1px solid var(--operator-border);
  color: var(--operator-foreground);
  white-space: nowrap;
}

.overview-table tbody tr:last-child td {
  border-bottom: 0;
}

.overview-table tbody tr:hover {
  background: var(--table-row-hover-background);
}

.overview-table .is-number {
  font-variant-numeric: tabular-nums;
  text-align: right;
}

.overview-table .is-muted {
  color: var(--operator-subtle-foreground);
}

.overview-model {
  display: inline-flex;
  align-items: center;
  gap: 0.625rem;
}

.overview-model code {
  font-size: 0.8125rem;
  font-weight: 500;
}

@media (max-width: 1100px) {
  .overview-hero-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 640px) {
  .overview-today dd {
    font-size: 1.25rem;
  }

  .overview-table th:nth-child(2),
  .overview-table td:nth-child(2),
  .overview-table th:nth-child(5),
  .overview-table td:nth-child(5) {
    display: none;
  }

  .overview-table th,
  .overview-table td {
    padding-right: 0.75rem;
    padding-left: 0.75rem;
  }
}
</style>
