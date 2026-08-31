<template>
  <AppLayout>
    <div class="stats-view">
      <section class="stats-usage-section" :aria-labelledby="usageTitleId">
        <header class="stats-section-header">
          <div>
            <h2 :id="usageTitleId">{{ t('admin.stats.usage.title') }}</h2>
            <p>{{ t('admin.stats.usage.description') }}</p>
          </div>
          <span v-if="snapshotGeneratedAt">{{ t('admin.stats.updatedAt', { time: formatTimestamp(snapshotGeneratedAt) }) }}</span>
        </header>

        <div v-if="statsError" class="stats-section-state is-error" role="alert" data-testid="stats-usage-error">
          <span>{{ t('admin.stats.usage.loadFailed') }}</span>
          <button type="button" class="btn btn-secondary" @click="loadStats">
            {{ t('admin.dashboard.retry') }}
          </button>
        </div>
        <div v-else-if="statsLoading && !stats" class="stats-section-state" role="status">
          <LoadingSpinner size="sm" />
          <span>{{ t('admin.stats.usage.loading') }}</span>
        </div>
        <template v-else-if="stats">
          <dl class="stats-metric-grid">
            <div>
              <dt>{{ t('admin.stats.usage.requests') }}</dt>
              <dd>{{ formatNumber(stats.today_requests) }}</dd>
              <span>{{ formatNumber(stats.rpm) }} RPM</span>
            </div>
            <div>
              <dt>{{ t('admin.stats.usage.tokens') }}</dt>
              <dd>{{ formatCompact(stats.today_tokens) }}</dd>
              <span>{{ formatCompact(stats.tpm) }} TPM</span>
            </div>
            <div>
              <dt>{{ t('admin.stats.usage.actualCost') }}</dt>
              <dd>${{ formatCost(stats.today_actual_cost) }}</dd>
              <span>{{ t('admin.stats.usage.today') }}</span>
            </div>
            <div>
              <dt>{{ t('admin.stats.usage.accountCost') }}</dt>
              <dd>${{ formatCost(stats.today_account_cost) }}</dd>
              <span>{{ t('admin.stats.usage.today') }}</span>
            </div>
            <div>
              <dt>{{ t('admin.stats.usage.averageLatency') }}</dt>
              <dd>{{ formatDuration(stats.average_duration_ms) }}</dd>
              <span>{{ t('admin.stats.usage.averageLatencyDescription') }}</span>
            </div>
          </dl>

          <div v-if="trendPoints.length" class="stats-trend" data-testid="stats-request-trend">
            <div class="stats-trend-heading">
              <h3>{{ t('admin.stats.usage.requestTrend') }}</h3>
              <span>{{ t('admin.stats.usage.recentPeriods', { count: trendPoints.length }) }}</span>
            </div>
            <ol :aria-label="t('admin.stats.usage.requestTrend')">
              <li v-for="point in trendPoints" :key="point.date">
                <strong>{{ formatCompact(point.requests) }}</strong>
                <span
                  class="stats-trend-bar"
                  :style="{ height: `${trendHeight(point.requests)}%` }"
                  tabindex="0"
                  :aria-label="`${formatTimestamp(point.date)}: ${formatNumber(point.requests)} ${t('admin.stats.usage.requests')}`"
                />
                <small>{{ formatShortDate(point.date) }}</small>
              </li>
            </ol>
          </div>
        </template>
      </section>

      <StatsCapacitySection
        :accounts="capacityAccounts"
        :usage-by-account-id="capacityUsageByAccountId"
        :errors-by-account-id="capacityErrorsByAccountId"
        :loading="capacityLoading"
        :error="capacityError"
        @retry="loadCapacity"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import StatsCapacitySection from '@/components/admin/StatsCapacitySection.vue'
import { supportsBatchAccountUsage } from '@/utils/operatorCapacity'
import type { Account, AccountUsageInfo, DashboardStats, TrendDataPoint } from '@/types'

const { t } = useI18n()
const usageTitleId = 'stats-usage-title'
const stats = ref<DashboardStats | null>(null)
const trend = ref<TrendDataPoint[]>([])
const snapshotGeneratedAt = ref('')
const statsLoading = ref(false)
const statsError = ref(false)
const capacityAccounts = ref<Account[]>([])
const capacityUsageByAccountId = ref<Record<string, AccountUsageInfo | null>>({})
const capacityErrorsByAccountId = ref<Record<string, string | null>>({})
const capacityLoading = ref(false)
const capacityError = ref(false)
let statsLoadSequence = 0
let capacityLoadSequence = 0

const trendPoints = computed(() => trend.value.slice(-12))
const trendMaximum = computed(() => Math.max(...trendPoints.value.map((point) => point.requests), 1))
const trendHeight = (requests: number) => Math.max(4, (requests / trendMaximum.value) * 100)

const formatNumber = (value: number | null | undefined) => Number(value ?? 0).toLocaleString()
const formatCompact = (value: number | null | undefined) => {
  const number = Number(value ?? 0)
  if (number >= 1_000_000_000) return `${(number / 1_000_000_000).toFixed(1)}B`
  if (number >= 1_000_000) return `${(number / 1_000_000).toFixed(1)}M`
  if (number >= 1_000) return `${(number / 1_000).toFixed(1)}K`
  return formatNumber(number)
}
const formatCost = (value: number | null | undefined) => Number(value ?? 0).toFixed(2)
const formatDuration = (milliseconds: number) => milliseconds >= 1000
  ? `${(milliseconds / 1000).toFixed(2)}s`
  : `${Math.round(milliseconds)}ms`
const formatTimestamp = (value: string) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}
const formatShortDate = (value: string) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime())
    ? value
    : new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' }).format(date)
}

async function loadStats() {
  const sequence = ++statsLoadSequence
  statsLoading.value = true
  statsError.value = false
  try {
    const response = await adminAPI.dashboard.getSnapshotV2({
      include_stats: true,
      include_trend: true,
      include_model_stats: false,
      include_group_stats: false,
      include_users_trend: false,
    })
    if (sequence !== statsLoadSequence) return
    stats.value = response.stats ?? null
    trend.value = response.trend ?? []
    snapshotGeneratedAt.value = response.generated_at ?? ''
  } catch (error) {
    if (sequence !== statsLoadSequence) return
    statsError.value = true
    console.error('Failed to load Stats usage snapshot:', error)
  } finally {
    if (sequence === statsLoadSequence) statsLoading.value = false
  }
}

async function loadCapacity() {
  const sequence = ++capacityLoadSequence
  capacityLoading.value = true
  capacityError.value = false
  try {
    const pageSize = 1000
    const firstPage = await adminAPI.accounts.list(1, pageSize, { include_scheduler_score: '0' })
    const accounts = [...firstPage.items]
    for (let page = 2; page <= firstPage.pages; page += 1) {
      const response = await adminAPI.accounts.list(page, pageSize, { include_scheduler_score: '0' })
      if (sequence !== capacityLoadSequence) return
      accounts.push(...response.items)
    }
    if (sequence !== capacityLoadSequence) return

    const usageAccountIDs = accounts.filter(supportsBatchAccountUsage).map((account) => account.id)
    let usage: Record<string, AccountUsageInfo | null> = {}
    let errors: Record<string, string | null> = {}
    if (usageAccountIDs.length) {
      try {
        const response = await adminAPI.accounts.getBatchUsage(usageAccountIDs, false)
        usage = response.usage ?? {}
        errors = response.errors ?? {}
      } catch (error) {
        errors = Object.fromEntries(usageAccountIDs.map((id) => [String(id), t('admin.stats.capacity.snapshotFailed')]))
        console.error('Failed to load passive capacity snapshots:', error)
      }
    }
    if (sequence !== capacityLoadSequence) return
    capacityAccounts.value = accounts
    capacityUsageByAccountId.value = usage
    capacityErrorsByAccountId.value = errors
  } catch (error) {
    if (sequence !== capacityLoadSequence) return
    capacityError.value = true
    console.error('Failed to load Stats account fleet:', error)
  } finally {
    if (sequence === capacityLoadSequence) capacityLoading.value = false
  }
}

onMounted(() => {
  void Promise.all([loadStats(), loadCapacity()])
})

onBeforeUnmount(() => {
  statsLoadSequence += 1
  capacityLoadSequence += 1
})
</script>

<style scoped>
.stats-view { display: grid; gap: 1.5rem; }
.stats-usage-section {
  overflow: hidden;
  border: 1px solid var(--operator-border);
  border-radius: 0.5rem;
  background: var(--operator-card);
  box-shadow: var(--operator-shadow-xs);
}
.stats-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.25rem 1.5rem;
  border-bottom: 1px solid var(--operator-border);
}
.stats-section-header h2 { color: var(--operator-foreground); font-size: 1rem; font-weight: 650; }
.stats-section-header p,
.stats-section-header > span { color: var(--operator-muted-foreground); font-size: 0.8125rem; }
.stats-section-header p { margin-top: 0.2rem; }
.stats-section-state { display: flex; min-height: 8rem; align-items: center; justify-content: center; gap: 0.75rem; padding: 1.5rem; color: var(--operator-muted-foreground); }
.stats-section-state.is-error { justify-content: space-between; min-height: 0; color: #b91c1c; }

.stats-metric-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  border-bottom: 1px solid var(--operator-border);
}
.stats-metric-grid > div { min-width: 0; padding: 1.125rem 1.25rem; border-right: 1px solid var(--operator-border); }
.stats-metric-grid > div:last-child { border-right: 0; }
.stats-metric-grid dt,
.stats-metric-grid span { color: var(--operator-muted-foreground); font-size: 0.75rem; }
.stats-metric-grid dd { margin-top: 0.35rem; color: var(--operator-foreground); font-size: 1.35rem; font-weight: 650; }
.stats-metric-grid span { display: block; margin-top: 0.2rem; overflow-wrap: anywhere; }

.stats-trend { padding: 1.25rem 1.5rem 1.5rem; }
.stats-trend-heading { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
.stats-trend-heading h3 { color: var(--operator-foreground); font-size: 0.875rem; font-weight: 650; }
.stats-trend-heading span { color: var(--operator-muted-foreground); font-size: 0.75rem; }
.stats-trend ol { display: grid; grid-template-columns: repeat(12, minmax(1.5rem, 1fr)); gap: 0.5rem; height: 10rem; margin-top: 1rem; }
.stats-trend li { display: grid; grid-template-rows: 1rem minmax(0, 1fr) 1rem; gap: 0.25rem; min-width: 0; text-align: center; }
.stats-trend strong,
.stats-trend small { overflow: hidden; color: var(--operator-muted-foreground); font-size: 0.625rem; text-overflow: ellipsis; white-space: nowrap; }
.stats-trend-bar { align-self: end; width: 100%; min-height: 0.25rem; border-radius: 2px 2px 0 0; background: var(--operator-foreground); }
.stats-trend-bar:focus { outline: 2px solid var(--operator-focus); outline-offset: 2px; }

@media (max-width: 1024px) {
  .stats-metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .stats-metric-grid > div { border-bottom: 1px solid var(--operator-border); }
  .stats-trend ol { overflow-x: auto; grid-template-columns: repeat(12, minmax(3rem, 1fr)); }
}

@media (max-width: 640px) {
  .stats-section-header { align-items: flex-start; flex-direction: column; }
  .stats-metric-grid { grid-template-columns: 1fr; }
  .stats-metric-grid > div { border-right: 0; }
}
</style>
