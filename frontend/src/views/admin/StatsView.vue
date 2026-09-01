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
              <div>
                <h3>{{ t('admin.stats.usage.requestTrend') }}</h3>
                <p>
                  <strong data-testid="stats-trend-total">{{ formatCompact(trendTotal) }}</strong>
                  {{ t('admin.stats.usage.total') }} ·
                  <strong data-testid="stats-trend-average">{{ formatCompact(trendAverage) }}</strong>
                  {{ t('admin.stats.usage.average') }} ·
                  <strong data-testid="stats-trend-peak">{{ formatCompact(trendPeak) }}</strong>
                  {{ t('admin.stats.usage.peak') }}
                </p>
              </div>
              <span>{{ trendPeriodLabel }}</span>
            </div>
            <div class="stats-trend-chart" role="group" :aria-label="trendChartLabel">
              <svg viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
                <line
                v-for="grid in trendGridLines"
                  :key="grid.ratio"
                  class="stats-trend-grid-line"
                  x1="8"
                  x2="99"
                  :y1="grid.y"
                  :y2="grid.y"
                  vector-effect="non-scaling-stroke"
                />
                <path v-if="trendAreaPath" class="stats-trend-area" :d="trendAreaPath" />
                <path
                  v-if="trendLinePath"
                  class="stats-trend-line"
                  :d="trendLinePath"
                  vector-effect="non-scaling-stroke"
                />
              </svg>

              <span
                v-for="grid in trendGridLines"
                :key="`label-${grid.ratio}`"
                class="stats-trend-y-label"
                :style="{ top: `${grid.y}%` }"
              >{{ formatCompact(grid.value) }}</span>

              <button
                v-for="(point, index) in trendChartPoints"
                :key="point.date"
                type="button"
                class="stats-trend-point"
                :class="{ 'is-active': activeTrendIndex === index }"
                :style="{ left: `${point.x}%`, top: `${point.y}%` }"
                :aria-label="trendPointLabel(point)"
                :aria-describedby="activeTrendIndex === index ? trendTooltipId : undefined"
                data-testid="stats-trend-point"
                @mouseenter="activeTrendIndex = index"
                @mouseleave="activeTrendIndex = null"
                @focus="activeTrendIndex = index"
                @blur="activeTrendIndex = null"
                @keydown.esc="activeTrendIndex = null"
              ><span /></button>

              <span
                v-for="(point, index) in trendChartPoints"
                v-show="trendLabelIndexes.has(index)"
                :key="`x-${point.date}`"
                class="stats-trend-x-label"
                :class="{
                  'is-first': index === 0,
                  'is-middle': index > 0 && index < trendChartPoints.length - 1,
                  'is-last': index === trendChartPoints.length - 1,
                }"
                :style="{ left: `${point.x}%` }"
              >{{ formatTrendTick(point.date) }}</span>

              <div
                v-if="activeTrendPoint"
                :id="trendTooltipId"
                class="stats-trend-tooltip"
                :class="{ 'is-below': activeTrendPoint.y < 28 }"
                role="tooltip"
                :style="trendTooltipStyle"
                data-testid="stats-trend-tooltip"
              >
                <strong>{{ formatNumber(activeTrendPoint.requests) }} {{ t('admin.stats.usage.requests') }}</strong>
                <span>{{ formatTimestamp(activeTrendPoint.date) }}</span>
              </div>
            </div>
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
const trendGranularity = ref('day')
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

const TREND_TOP = 6
const TREND_BASELINE = 78
const TREND_LEFT = 8
const TREND_RIGHT = 99
const trendTooltipId = 'stats-request-trend-tooltip'
const activeTrendIndex = ref<number | null>(null)
const trendPoints = computed(() => trend.value.slice(-12))
const trendTotal = computed(() => trendPoints.value.reduce((total, point) => total + point.requests, 0))
const trendAverage = computed(() => trendPoints.value.length ? trendTotal.value / trendPoints.value.length : 0)
const trendPeak = computed(() => Math.max(...trendPoints.value.map((point) => point.requests), 0))
const trendScaleMaximum = computed(() => {
  const maximum = trendPeak.value
  if (maximum <= 1) return 1
  const magnitude = 10 ** Math.floor(Math.log10(maximum))
  const normalized = maximum / magnitude
  const step = normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10
  return step * magnitude
})
const trendChartPoints = computed(() => trendPoints.value.map((point, index, points) => ({
  ...point,
  x: points.length === 1
    ? (TREND_LEFT + TREND_RIGHT) / 2
    : TREND_LEFT + (index / (points.length - 1)) * (TREND_RIGHT - TREND_LEFT),
  y: TREND_TOP + (1 - point.requests / trendScaleMaximum.value) * (TREND_BASELINE - TREND_TOP),
})))
const trendGridLines = computed(() => [1, 2 / 3, 1 / 3, 0].map((ratio) => ({
  ratio,
  value: Math.round(trendScaleMaximum.value * ratio),
  y: TREND_TOP + (1 - ratio) * (TREND_BASELINE - TREND_TOP),
})))
const trendLinePath = computed(() => trendChartPoints.value
  .map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x} ${point.y}`)
  .join(' '))
const trendAreaPath = computed(() => {
  const points = trendChartPoints.value
  if (!points.length) return ''
  return `M ${points[0].x} ${TREND_BASELINE} ${points.map((point) => `L ${point.x} ${point.y}`).join(' ')} L ${points[points.length - 1].x} ${TREND_BASELINE} Z`
})
const trendLabelIndexes = computed(() => {
  const last = trendChartPoints.value.length - 1
  if (last < 4) return new Set(trendChartPoints.value.map((_, index) => index))
  return new Set([0, Math.round(last / 3), Math.round(last * 2 / 3), last])
})
const activeTrendPoint = computed(() => activeTrendIndex.value === null
  ? null
  : trendChartPoints.value[activeTrendIndex.value] ?? null)
const trendTooltipStyle = computed(() => {
  const point = activeTrendPoint.value
  if (!point) return {}
  return {
    left: `${Math.min(88, Math.max(15, point.x))}%`,
    top: `${point.y}%`,
  }
})
const trendPeriodLabel = computed(() => t(
  trendGranularity.value === 'hour'
    ? 'admin.stats.usage.recentHourlyPeriods'
    : 'admin.stats.usage.recentDailyPeriods',
  { count: trendPoints.value.length },
))
const trendChartLabel = computed(() => `${t('admin.stats.usage.requestTrend')}. ${trendPeriodLabel.value}. ${formatNumber(trendTotal.value)} ${t('admin.stats.usage.requests')}.`)

const formatNumber = (value: number | null | undefined) => Number(value ?? 0).toLocaleString()
const formatCompact = (value: number | null | undefined) => {
  const number = Number(value ?? 0)
  if (number >= 1_000_000_000) return `${(number / 1_000_000_000).toFixed(1)}B`
  if (number >= 1_000_000) return `${(number / 1_000_000).toFixed(1)}M`
  if (number >= 1_000) return `${(number / 1_000).toFixed(1)}K`
  return number.toLocaleString(undefined, { maximumFractionDigits: 1 })
}
const formatCost = (value: number | null | undefined) => Number(value ?? 0).toFixed(2)
const formatDuration = (milliseconds: number) => milliseconds >= 1000
  ? `${(milliseconds / 1000).toFixed(2)}s`
  : `${Math.round(milliseconds)}ms`
const formatTimestamp = (value: string) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}
const formatTrendTick = (value: string) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime())
    ? value
    : new Intl.DateTimeFormat(undefined, trendGranularity.value === 'hour'
        ? { hour: 'numeric' }
        : { month: 'short', day: 'numeric' })
      .format(date)
}
const trendPointLabel = (point: TrendDataPoint) => `${formatTimestamp(point.date)}: ${formatNumber(point.requests)} ${t('admin.stats.usage.requests')}`

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
    trendGranularity.value = response.granularity ?? 'day'
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
.stats-trend-heading p,
.stats-trend-heading > span { color: var(--operator-muted-foreground); font-size: 0.75rem; }
.stats-trend-heading p { margin-top: 0.2rem; }
.stats-trend-heading p strong { color: var(--operator-foreground); font-weight: 600; }
.stats-trend-chart { position: relative; height: clamp(10.5rem, 16vw, 13rem); margin-top: 0.875rem; overflow: hidden; }
.stats-trend-chart svg { display: block; width: 100%; height: 100%; overflow: visible; }
.stats-trend-grid-line { stroke: var(--operator-border-subtle); stroke-width: 1; }
.stats-trend-area { fill: var(--operator-muted); opacity: 0.55; }
.stats-trend-line { fill: none; stroke: var(--operator-foreground); stroke-linecap: round; stroke-linejoin: round; stroke-width: 1.75; }
.stats-trend-y-label,
.stats-trend-x-label {
  position: absolute;
  color: var(--operator-muted-foreground);
  font-size: 0.625rem;
  line-height: 1;
  pointer-events: none;
  white-space: nowrap;
}
.stats-trend-y-label { left: 0; transform: translateY(-50%); }
.stats-trend-x-label { top: 86%; transform: translateX(-50%); }
.stats-trend-x-label.is-first { transform: none; }
.stats-trend-x-label.is-last { transform: translateX(-100%); }
.stats-trend-point {
  position: absolute;
  width: 1rem;
  height: 1rem;
  transform: translate(-50%, -50%);
  border: 0;
  background: transparent;
  cursor: default;
}
.stats-trend-point > span {
  position: absolute;
  inset: 50% auto auto 50%;
  width: 0.45rem;
  height: 0.45rem;
  transform: translate(-50%, -50%);
  border: 2px solid var(--operator-card);
  border-radius: 50%;
  background: var(--operator-foreground);
  box-shadow: 0 0 0 1px var(--operator-foreground);
}
.stats-trend-point:hover > span,
.stats-trend-point.is-active > span { background: var(--operator-card); }
.stats-trend-point:focus-visible { outline: 2px solid var(--operator-focus); outline-offset: 2px; border-radius: 50%; }
.stats-trend-tooltip {
  position: absolute;
  z-index: 2;
  display: grid;
  min-width: 8.5rem;
  transform: translate(-50%, calc(-100% - 0.6rem));
  gap: 0.125rem;
  padding: 0.5rem 0.625rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
  box-shadow: var(--operator-shadow-sm);
  color: var(--operator-foreground);
  font-size: 0.6875rem;
  pointer-events: none;
}
.stats-trend-tooltip.is-below { transform: translate(-50%, 0.6rem); }
.stats-trend-tooltip span { color: var(--operator-muted-foreground); font-size: 0.625rem; }

@media (max-width: 1024px) {
  .stats-metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .stats-metric-grid > div { border-bottom: 1px solid var(--operator-border); }
}

@media (max-width: 640px) {
  .stats-section-header { align-items: flex-start; flex-direction: column; }
  .stats-metric-grid { grid-template-columns: 1fr; }
  .stats-metric-grid > div { border-right: 0; }
  .stats-trend { padding-inline: 1rem; }
  .stats-trend-heading { align-items: flex-start; flex-direction: column; gap: 0.25rem; }
  .stats-trend-chart { height: 11rem; }
  .stats-trend-x-label.is-middle { display: none; }
}
</style>
