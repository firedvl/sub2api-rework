<template>
  <AppLayout>
    <div class="viz-toolbar" role="toolbar" :aria-label="t('admin.viz.tabs.label')">
      <div class="viz-seg" role="group" :aria-label="t('admin.viz.tabs.label')">
        <button type="button" data-testid="stats-tab-usage" :aria-pressed="view === 'usage'" @click="setView('usage')">
          {{ t('admin.viz.tabs.usage') }}
        </button>
        <button type="button" data-testid="stats-tab-capacity" :aria-pressed="view === 'capacity'" @click="setView('capacity')">
          {{ t('admin.viz.tabs.capacity') }}
        </button>
      </div>

      <template v-if="view === 'usage'">
        <div class="viz-seg" role="group" :aria-label="t('admin.viz.rangeLabel')">
          <button
            v-for="option in RANGE_KEYS"
            :key="option"
            type="button"
            :data-testid="`stats-range-${option}`"
            :aria-pressed="range === option"
            @click="setRange(option)"
          >
            {{ t(`admin.viz.ranges.${option}`) }}
          </button>
        </div>
        <span class="stats-grow"></span>
        <label class="stats-compare">
          <Toggle v-model="compare" />
          <span>{{ t('admin.viz.compare') }}</span>
        </label>
        <button type="button" class="app-icon-button" :aria-label="t('admin.viz.refresh')" :title="t('admin.viz.refresh')" @click="loadUsage">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': usageLoading }" />
        </button>
      </template>
    </div>

    <StatsCapacitySection
      v-if="view === 'capacity'"
      :accounts="fleet.accounts.value"
      :usage-by-account-id="fleet.usageByAccountId.value"
      :errors-by-account-id="fleet.errorsByAccountId.value"
      :loading="fleet.loading.value"
      :error="fleet.error.value"
      @retry="fleet.load"
    />

    <template v-else>
      <div v-if="usageError" class="viz-card viz-state" role="alert" data-testid="stats-usage-error">
        <span>{{ t('admin.stats.usage.loadFailed') }}</span>
        <button type="button" class="btn btn-secondary" @click="loadUsage">{{ t('admin.dashboard.retry') }}</button>
      </div>
      <div v-else-if="usageLoading && !loaded" class="viz-state" role="status">
        <LoadingSpinner size="sm" />
        <span>{{ t('admin.stats.usage.loading') }}</span>
      </div>

      <template v-else-if="loaded">
        <dl class="viz-kpis ui-stagger" data-testid="stats-kpis">
          <div v-for="kpi in kpis" :key="kpi.key" class="viz-card viz-kpi">
            <dt class="viz-kpi-label">
              <Icon :name="kpi.icon" size="sm" />
              {{ kpi.label }}
            </dt>
            <dd class="viz-kpi-row">
              <span class="viz-kpi-value"><CountUp :value="kpi.value" :format="kpi.format" /></span>
              <SparkLine v-if="kpi.spark" :values="kpi.spark" />
            </dd>
            <span
              class="viz-delta"
              :class="kpi.delta ? `is-${kpi.delta.tone}` : ''"
              :title="kpi.delta ? t('admin.viz.vsPrevious', { range: t(`admin.viz.ranges.${range}`) }) : undefined"
            >
              <template v-if="kpi.delta">
                <Icon v-if="kpi.delta.tone !== 'flat'" :name="kpi.delta.up ? 'arrowUp' : 'arrowDown'" size="xs" />
                {{ kpi.delta.text }}
              </template>
            </span>
          </div>
        </dl>

        <div class="viz-grid ui-stagger">
          <section class="viz-card viz-span-8" aria-labelledby="stats-traffic" data-testid="stats-traffic">
            <header class="viz-card-head">
              <h2 id="stats-traffic" class="viz-card-title">{{ t('admin.viz.traffic') }}</h2>
              <div class="viz-card-tools">
                <span v-if="compare && !tableView" class="viz-key">
                  <i style="background: var(--operator-series-muted)"></i>{{ t('admin.viz.previous') }}
                </span>
                <div class="viz-seg" role="group" :aria-label="t('admin.viz.metric')">
                  <button
                    v-for="option in METRIC_KEYS"
                    :key="option"
                    type="button"
                    :data-testid="`stats-metric-${option}`"
                    :aria-pressed="metric === option"
                    @click="metric = option"
                  >
                    {{ t(`admin.viz.${option}`) }}
                  </button>
                </div>
                <button
                  type="button"
                  class="app-icon-button"
                  :aria-pressed="tableView"
                  :aria-label="tableView ? t('admin.viz.showChart') : t('admin.viz.showTable')"
                  :title="tableView ? t('admin.viz.showChart') : t('admin.viz.showTable')"
                  @click="tableView = !tableView"
                >
                  <Icon :name="tableView ? 'chartBar' : 'document'" size="sm" />
                </button>
              </div>
            </header>
            <div class="viz-card-body">
              <div v-if="tableView" class="stats-table-wrap">
                <table class="stats-table">
                  <thead>
                    <tr>
                      <th>{{ t('admin.viz.columns.period') }}</th>
                      <th class="is-number">{{ t(`admin.viz.${metric}`) }}</th>
                      <th v-if="compare" class="is-number">{{ t('admin.viz.previous') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(tip, index) in traffic.tips" :key="tip">
                      <td>{{ tip }}</td>
                      <td class="is-number">{{ metricFormat(traffic.values[index]) }}</td>
                      <td v-if="compare" class="is-number is-muted">{{ metricFormat(traffic.previous[index] ?? 0) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <TrendChart
                v-else
                :labels="traffic.labels"
                :tips="traffic.tips"
                :series="trafficSeries"
                :summary="`${t(`admin.viz.${metric}`)} · ${t(`admin.viz.ranges.${range}`)}`"
                :height="290"
                :format-value="metricFormat"
                :format-axis="metric === 'cost' ? formatAxisMoney : formatAxisNumber"
              />
            </div>
            <footer class="viz-card-foot">
              <span>{{ t('admin.viz.total') }} <b>{{ metricFormat(trafficTotal) }}</b></span>
              <span v-if="trafficPeak">{{ t('admin.viz.peak') }} <b>{{ metricFormat(trafficPeak.value) }}</b> · {{ trafficPeak.tip }}</span>
            </footer>
          </section>

          <section class="viz-card viz-span-4" aria-labelledby="stats-share" data-testid="stats-share">
            <header class="viz-card-head">
              <h2 id="stats-share" class="viz-card-title">{{ shareByProvider ? t('admin.viz.providers') : t('admin.viz.groups') }}</h2>
            </header>
            <div class="viz-card-body stats-fill">
              <div v-if="!share.length" class="viz-state">{{ t('admin.viz.noData') }}</div>
              <template v-else>
                <div class="viz-share" role="img" :aria-label="shareByProvider ? t('admin.viz.providers') : t('admin.viz.groups')">
                  <i
                    v-for="item in share"
                    :key="item.key"
                    :style="{ flex: item.requests, background: item.color }"
                    :title="`${item.name} · ${formatPercent(item.fraction)} · ${formatCompact(item.requests)}`"
                  ></i>
                </div>
                <ul class="viz-rows">
                  <li v-for="item in share" :key="item.key" class="viz-row" :title="`${formatCompact(item.requests)} ${t('admin.viz.requests').toLowerCase()} · ${formatCompact(item.tokens)} ${t('admin.viz.tokens').toLowerCase()}`">
                    <span class="viz-row-name">
                      <ProviderMark v-if="shareByProvider" :platform="item.key" :size="28" />
                      <i v-else :style="{ background: item.color }"></i>
                      <span>{{ item.name }}</span>
                    </span>
                    <span class="viz-row-value"><b>{{ formatPercent(item.fraction) }}</b><small>{{ formatCompact(item.requests) }}</small></span>
                  </li>
                </ul>
              </template>
            </div>
          </section>

          <section class="viz-card viz-span-4" aria-labelledby="stats-top" data-testid="stats-top">
            <header class="viz-card-head">
              <h2 id="stats-top" class="viz-card-title">{{ t('admin.viz.top') }}</h2>
              <div class="viz-card-tools">
                <div class="viz-seg" role="group" :aria-label="t('admin.viz.top')">
                  <button type="button" :aria-pressed="topTab === 'models'" @click="topTab = 'models'">{{ t('admin.viz.models') }}</button>
                  <button type="button" data-testid="stats-top-keys" :aria-pressed="topTab === 'keys'" @click="topTab = 'keys'">{{ t('admin.viz.apiKeys') }}</button>
                </div>
              </div>
            </header>
            <div class="viz-card-body">
              <div v-if="!topRows.length" class="viz-state">
                <LoadingSpinner v-if="topTab === 'keys' && keysLoading" size="sm" />
                <span v-else>{{ t('admin.viz.noData') }}</span>
              </div>
              <ul v-else class="viz-hbars">
                <li v-for="(row, index) in topRows" :key="row.name" class="viz-hbar" :title="row.detail">
                  <span class="viz-row-name">
                    <ProviderMark :platform="row.platform" :size="22">
                      <Icon :name="topTab === 'keys' ? 'key' : 'cube'" size="xs" />
                    </ProviderMark>
                    <span class="font-mono text-[0.8125rem]">{{ row.name }}</span>
                  </span>
                  <span class="viz-row-value">{{ formatCompact(row.requests) }}<small>{{ formatPercent(row.fraction) }}</small></span>
                  <span class="viz-hbar-track">
                    <i :style="{ width: `${row.width}%`, background: row.color, animationDelay: `${index * 40}ms` }"></i>
                  </span>
                </li>
              </ul>
            </div>
          </section>

          <section class="viz-card viz-span-4" aria-labelledby="stats-performance" data-testid="stats-performance">
            <header class="viz-card-head">
              <h2 id="stats-performance" class="viz-card-title">{{ t('admin.viz.performance') }}</h2>
              <div class="viz-card-tools">
                <div class="viz-seg" role="group" :aria-label="t('admin.viz.performance')">
                  <button
                    v-for="option in performanceTabs"
                    :key="option"
                    type="button"
                    :data-testid="`stats-performance-${option}`"
                    :aria-pressed="performanceTab === option"
                    @click="performanceChoice = option"
                  >
                    {{ t(`admin.viz.${option}`) }}
                  </button>
                </div>
              </div>
            </header>
            <div class="viz-card-body">
              <template v-if="performanceTab === 'latency' && ops.overview">
                <dl class="viz-percentiles">
                  <div v-for="item in percentiles" :key="item.label">
                    <small>{{ item.label }}</small>
                    <b>{{ item.text }}</b>
                  </div>
                </dl>
                <TrendChart
                  v-if="histogram.values.length"
                  type="bar"
                  :labels="histogram.labels"
                  :series="[{ name: t('admin.viz.requests'), color: 'var(--operator-accent)', values: histogram.values }]"
                  :summary="t('admin.viz.latencyLabel')"
                  :height="190"
                  :format-value="formatInt"
                />
              </template>

              <template v-else-if="performanceTab === 'errors' && ops.overview">
                <div class="viz-big">
                  <b>{{ formatPercent(ops.overview.error_rate, 2) }}</b>
                  <span>{{ t('admin.viz.failedOf', { failed: formatInt(ops.overview.error_count_total), total: formatCompact(ops.overview.request_count_total) }) }}</span>
                </div>
                <div v-if="!errorRows.length" class="viz-state">{{ t('admin.viz.noData') }}</div>
                <ul v-else class="viz-hbars">
                  <li v-for="(row, index) in errorRows" :key="row.code" class="viz-hbar">
                    <span class="viz-row-name"><span>{{ t('admin.viz.httpStatus', { code: row.code }) }}</span></span>
                    <span class="viz-row-value">{{ formatInt(row.total) }}</span>
                    <span class="viz-hbar-track">
                      <i :style="{ width: `${row.width}%`, background: 'var(--operator-destructive-fill)', animationDelay: `${index * 40}ms` }"></i>
                    </span>
                  </li>
                </ul>
              </template>

              <template v-else>
                <div class="viz-big">
                  <b>{{ formatPercent(cacheHitRate) }}</b>
                  <span>{{ t('admin.viz.cacheHit') }}</span>
                </div>
                <div class="viz-share" role="img" :aria-label="t('admin.viz.tokens')">
                  <i
                    v-for="item in tokenTypes"
                    :key="item.key"
                    :style="{ flex: Math.max(item.value, 0.0001), background: item.color }"
                    :title="`${item.name} · ${formatCompact(item.value)}`"
                  ></i>
                </div>
                <ul class="viz-rows">
                  <li v-for="item in tokenTypes" :key="item.key" class="viz-row">
                    <span class="viz-row-name"><i :style="{ background: item.color }"></i><span>{{ item.name }}</span></span>
                    <span class="viz-row-value"><b>{{ formatCompact(item.value) }}</b><small>{{ formatPercent(item.fraction) }}</small></span>
                  </li>
                </ul>
              </template>
            </div>
            <footer v-if="performanceTab === 'latency' && ops.overview?.ttft?.p50_ms != null" class="viz-card-foot">
              <span>{{ t('admin.viz.firstToken') }} <b>{{ formatDuration(ops.overview.ttft.p50_ms) }}</b></span>
            </footer>
          </section>

          <section class="viz-card viz-span-4" aria-labelledby="stats-busy" data-testid="stats-busy">
            <header class="viz-card-head">
              <h2 id="stats-busy" class="viz-card-title">{{ t('admin.viz.busyHours') }}</h2>
              <div class="viz-card-tools stats-heat-scale" aria-hidden="true">
                <i v-for="step in 5" :key="step" :style="{ background: `var(--operator-heat-${step})` }"></i>
              </div>
            </header>
            <div class="viz-card-body">
              <HeatGrid
                :rows="heat.rows"
                :summary="t('admin.viz.busyHoursLabel', { peak: heat.peak || '–' })"
                :format-value="formatInt"
              />
            </div>
            <footer v-if="heat.peak" class="viz-card-foot">
              <span>{{ t('admin.viz.peak') }} <b>{{ heat.peak }}</b></span>
            </footer>
          </section>
        </div>
      </template>
    </template>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import StatsCapacitySection from '@/components/admin/StatsCapacitySection.vue'
import HeatGrid from '@/components/viz/HeatGrid.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import SparkLine from '@/components/viz/SparkLine.vue'
import CountUp from '@/components/ui/CountUp.vue'
import TrendChart, { type TrendSeries } from '@/components/viz/TrendChart.vue'
import { useAccountFleet } from '@/composables/useAccountFleet'
import { providerColor, providerName, providerOfModel } from '@/utils/providerSeries'
import {
  addDays,
  bucketTrend,
  localDateString,
  slotStarts,
  startOfDay,
  sumBuckets,
  type TrendBucket,
} from '@/utils/trendBuckets'
import {
  formatAxisMoney,
  formatAxisNumber,
  formatCompact,
  formatDuration,
  formatInt,
  formatMoney,
  formatPercent,
  relativeChange,
} from '@/utils/vizFormat'
import type { ApiKeyUsageTrendPoint, GroupStat, ModelStat, TrendDataPoint } from '@/types'
import type {
  OpsDashboardOverview,
  OpsErrorDistributionItem,
  OpsLatencyHistogramBucket,
  OpsThroughputPlatformBreakdownItem,
} from '@/api/admin/ops'

type IconName = InstanceType<typeof Icon>['$props']['name']
type RangeKey = 'today' | 'd7' | 'd30' | 'd90'
type MetricKey = 'requests' | 'tokens' | 'cost'
type PerformanceKey = 'latency' | 'errors' | 'tokens'
type StatsViewKey = 'usage' | 'capacity'

// Each range is whole calendar days in the viewer's time zone, matching the API.
const RANGES: Record<RangeKey, { days: number; stepHours: number; granularity: 'hour' | 'day' }> = {
  today: { days: 1, stepHours: 1, granularity: 'hour' },
  d7: { days: 7, stepHours: 6, granularity: 'hour' },
  d30: { days: 30, stepHours: 24, granularity: 'day' },
  d90: { days: 90, stepHours: 24, granularity: 'day' },
}
const RANGE_KEYS = Object.keys(RANGES) as RangeKey[]
const METRIC_KEYS: MetricKey[] = ['requests', 'tokens', 'cost']
const TOP_ROWS = 6

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const adminSettingsStore = useAdminSettingsStore()
const fleet = useAccountFleet(() => t('admin.stats.capacity.snapshotFailed'))

const view = computed<StatsViewKey>(() => (route.query.view === 'capacity' ? 'capacity' : 'usage'))
const range = ref<RangeKey>('d7')
const metric = ref<MetricKey>('requests')
const compare = ref(true)
const tableView = ref(false)
const topTab = ref<'models' | 'keys'>('models')
const performanceChoice = ref<PerformanceKey | null>(null)

const usageLoading = ref(false)
const usageError = ref(false)
const loaded = ref(false)
const anchor = ref(new Date())
const trend = shallowRef<TrendDataPoint[]>([])
const previousTrend = shallowRef<TrendDataPoint[]>([])
const models = shallowRef<ModelStat[]>([])
const groups = shallowRef<GroupStat[]>([])
const averageDuration = ref<number | null>(null)
const previousAverageDuration = ref<number | null>(null)
const heatTrend = shallowRef<TrendDataPoint[]>([])
const keyTrend = shallowRef<ApiKeyUsageTrendPoint[]>([])
const keysLoading = ref(false)
const ops = ref<{
  overview: OpsDashboardOverview | null
  platforms: OpsThroughputPlatformBreakdownItem[]
  histogram: OpsLatencyHistogramBucket[]
  errors: OpsErrorDistributionItem[]
}>({ overview: null, platforms: [], histogram: [], errors: [] })
let usageSequence = 0
let keysSequence = 0
let keysLoadedFor = ''

// ---------- Period ----------
const period = computed(() => {
  const config = RANGES[range.value]
  const today = startOfDay(anchor.value)
  const start = addDays(today, -(config.days - 1))
  return { ...config, start, end: addDays(today, 1), previousStart: addDays(start, -config.days) }
})

const dateParams = (start: Date, days: number) => ({
  start_date: localDateString(start),
  end_date: localDateString(addDays(start, days - 1)),
})

/** Slots up to the current moment; later ones would only draw a false drop to zero. */
const buckets = computed<TrendBucket[]>(() => {
  const { start, end, days, stepHours } = period.value
  const starts = slotStarts(start, (days * 24) / stepHours, stepHours).filter((slot) => slot <= anchor.value)
  return bucketTrend(trend.value, starts, end)
})

const previousBuckets = computed<TrendBucket[]>(() => {
  const { start, previousStart, days, stepHours } = period.value
  const starts = slotStarts(previousStart, (days * 24) / stepHours, stepHours)
  // Compare like with like: the same number of slots as the current period so far.
  return bucketTrend(previousTrend.value, starts, start).slice(0, buckets.value.length)
})

const totals = computed(() => sumBuckets(buckets.value))
const previousTotals = computed(() => sumBuckets(previousBuckets.value))

// ---------- Tiles ----------
function delta(current: number, previous: number | null, goodWhenUp: boolean) {
  if (!compare.value || previous === null) return null
  const change = relativeChange(current, previous)
  if (change === null) return null
  const flat = Math.abs(change) < 0.005
  const up = change > 0
  return {
    up,
    tone: flat ? 'flat' : up === goodWhenUp ? 'good' : 'poor',
    text: `${up && !flat ? '+' : ''}${(change * 100).toFixed(1)}%`.replace('-', '−'),
  }
}

const kpis = computed(() => {
  const current = totals.value
  const previous = previousTotals.value
  const rows: Array<{
    key: string
    icon: IconName
    label: string
    value: number | null
    format: (value: number) => string
    spark?: number[]
    delta: ReturnType<typeof delta>
  }> = [
    {
      key: 'requests',
      icon: 'trendingUp',
      label: t('admin.viz.requests'),
      value: current.requests,
      format: formatCompact,
      spark: buckets.value.map((bucket) => bucket.requests),
      delta: delta(current.requests, previous.requests, true),
    },
    {
      key: 'tokens',
      icon: 'cube',
      label: t('admin.viz.tokens'),
      value: current.total_tokens,
      format: formatCompact,
      spark: buckets.value.map((bucket) => bucket.total_tokens),
      delta: delta(current.total_tokens, previous.total_tokens, true),
    },
    {
      key: 'cost',
      icon: 'dollar',
      label: t('admin.viz.cost'),
      value: current.actual_cost,
      format: formatMoney,
      spark: buckets.value.map((bucket) => bucket.actual_cost),
      delta: delta(current.actual_cost, previous.actual_cost, false),
    },
    {
      key: 'latency',
      icon: 'clock',
      label: t('admin.viz.avgResponse'),
      value: averageDuration.value,
      format: formatDuration,
      delta: averageDuration.value === null ? null : delta(averageDuration.value, previousAverageDuration.value, false),
    },
  ]
  return rows
})

// ---------- Traffic ----------
const metricOf = (bucket: TrendBucket) => (
  metric.value === 'requests' ? bucket.requests : metric.value === 'tokens' ? bucket.total_tokens : bucket.actual_cost
)
const metricFormat = (value: number) => (
  metric.value === 'requests' ? formatInt(value) : metric.value === 'tokens' ? formatCompact(value) : formatMoney(value)
)

const traffic = computed(() => {
  const { stepHours } = period.value
  const tick = new Intl.DateTimeFormat(locale.value, stepHours < 6 ? { hour: 'numeric' } : { month: 'short', day: 'numeric' })
  const tip = new Intl.DateTimeFormat(locale.value, stepHours < 24
    ? { weekday: 'short', month: 'short', day: 'numeric', hour: 'numeric' }
    : { weekday: 'short', month: 'short', day: 'numeric' })
  return {
    labels: buckets.value.map((bucket) => tick.format(bucket.start)),
    tips: buckets.value.map((bucket) => tip.format(bucket.start)),
    values: buckets.value.map(metricOf),
    previous: previousBuckets.value.map(metricOf),
  }
})

const trafficSeries = computed<TrendSeries[]>(() => {
  const series: TrendSeries[] = [
    { name: t('admin.viz.thisPeriod'), color: 'var(--operator-accent-text)', values: traffic.value.values },
  ]
  if (compare.value && traffic.value.previous.length === traffic.value.values.length) {
    series.push({ name: t('admin.viz.previous'), color: 'var(--operator-series-muted)', values: traffic.value.previous, muted: true })
  }
  return series
})

const trafficTotal = computed(() => traffic.value.values.reduce((sum, value) => sum + value, 0))
const trafficPeak = computed(() => {
  const values = traffic.value.values
  if (!values.length || trafficTotal.value === 0) return null
  const index = values.indexOf(Math.max(...values))
  return { value: values[index], tip: traffic.value.tips[index] }
})

// ---------- Share ----------
const shareByProvider = computed(() => ops.value.platforms.length > 0)
const share = computed(() => {
  const rows = shareByProvider.value
    ? ops.value.platforms.map((item) => ({
      key: item.platform,
      name: providerName(item.platform),
      color: providerColor(item.platform),
      requests: item.request_count,
      tokens: item.token_consumed,
    }))
    : groups.value.map((group, index) => ({
      key: String(group.group_id),
      name: group.group_name,
      color: `var(--operator-series-${(index % 4) + 1})`,
      requests: group.requests,
      tokens: group.total_tokens,
    }))
  const total = rows.reduce((sum, row) => sum + row.requests, 0) || 1
  return rows
    .filter((row) => row.requests > 0)
    .sort((left, right) => right.requests - left.requests)
    .slice(0, shareByProvider.value ? rows.length : 4)
    .map((row) => ({ ...row, fraction: row.requests / total }))
})

// ---------- Top ----------
const topRows = computed(() => {
  const rows = topTab.value === 'models'
    ? models.value.map((model) => {
      const platform = providerOfModel(model.model)
      return {
        name: model.model,
        platform,
        color: platform ? providerColor(platform) : 'var(--operator-accent)',
        requests: model.requests,
        detail: `${formatCompact(model.total_tokens)} ${t('admin.viz.tokens').toLowerCase()} · ${formatMoney(model.actual_cost)}`,
      }
    })
    : keyTotals.value.map((key) => ({
      name: key.name,
      platform: '',
      color: 'var(--operator-accent)',
      requests: key.requests,
      detail: `${formatCompact(key.tokens)} ${t('admin.viz.tokens').toLowerCase()}`,
    }))
  const sorted = rows.filter((row) => row.requests > 0).sort((left, right) => right.requests - left.requests)
  const total = sorted.reduce((sum, row) => sum + row.requests, 0) || 1
  const max = sorted[0]?.requests || 1
  return sorted.slice(0, TOP_ROWS).map((row) => ({ ...row, fraction: row.requests / total, width: (row.requests / max) * 100 }))
})

const keyTotals = computed(() => {
  const byKey = new Map<number, { name: string; requests: number; tokens: number }>()
  for (const point of keyTrend.value) {
    const entry = byKey.get(point.api_key_id) ?? { name: point.key_name, requests: 0, tokens: 0 }
    entry.requests += point.requests
    entry.tokens += point.tokens
    byKey.set(point.api_key_id, entry)
  }
  return [...byKey.values()]
})

// ---------- Performance ----------
const performanceTabs = computed<PerformanceKey[]>(() => (ops.value.overview ? ['latency', 'errors', 'tokens'] : ['tokens']))
const performanceTab = computed<PerformanceKey>(() => (
  performanceChoice.value && performanceTabs.value.includes(performanceChoice.value)
    ? performanceChoice.value
    : performanceTabs.value[0]
))

const percentiles = computed(() => {
  const duration = ops.value.overview?.duration
  return ([['p50', duration?.p50_ms], ['p90', duration?.p90_ms], ['p95', duration?.p95_ms], ['p99', duration?.p99_ms]] as const)
    .map(([label, value]) => ({ label, text: value == null ? '–' : formatDuration(value) }))
})

const histogram = computed(() => ({
  labels: ops.value.histogram.map((bucket) => bucket.range),
  values: ops.value.histogram.map((bucket) => bucket.count),
}))

const errorRows = computed(() => {
  const rows = ops.value.errors.filter((item) => item.total > 0).sort((left, right) => right.total - left.total).slice(0, 5)
  const max = rows[0]?.total || 1
  return rows.map((item) => ({ code: item.status_code, total: item.total, width: (item.total / max) * 100 }))
})

// One whole split into parts: an ordinal ramp of the accent, not series hues.
const tokenTypes = computed(() => {
  const current = totals.value
  const parts = [
    { key: 'cacheRead', value: current.cache_read_tokens, alpha: 1 },
    { key: 'input', value: current.input_tokens, alpha: 0.72 },
    { key: 'output', value: current.output_tokens, alpha: 0.48 },
    { key: 'cacheWrite', value: current.cache_creation_tokens, alpha: 0.28 },
  ]
  const total = parts.reduce((sum, part) => sum + part.value, 0) || 1
  return parts.map((part) => ({
    key: part.key,
    name: t(`admin.viz.tokenTypes.${part.key}`),
    value: part.value,
    fraction: part.value / total,
    color: `rgb(var(--operator-accent-rgb) / ${part.alpha})`,
  }))
})

const cacheHitRate = computed(() => {
  const readable = totals.value.input_tokens + totals.value.cache_read_tokens
  return readable ? totals.value.cache_read_tokens / readable : 0
})

// ---------- Busy hours ----------
const heat = computed(() => {
  const today = startOfDay(anchor.value)
  const first = addDays(today, -6)
  const hours = bucketTrend(heatTrend.value, slotStarts(first, 7 * 24, 1), addDays(today, 1))
  const weekday = new Intl.DateTimeFormat(locale.value, { weekday: 'short' })
  const dayTitle = new Intl.DateTimeFormat(locale.value, { weekday: 'short', month: 'short', day: 'numeric' })
  const peakFormat = new Intl.DateTimeFormat(locale.value, { weekday: 'short', hour: 'numeric' })
  let peak: TrendBucket | null = null
  const rows = Array.from({ length: 7 }, (_, day) => {
    const slice = hours.slice(day * 24, day * 24 + 24)
    for (const bucket of slice) {
      if (bucket.start > anchor.value) continue
      if (bucket.requests > 0 && (!peak || bucket.requests > peak.requests)) peak = bucket
    }
    return {
      label: weekday.format(slice[0].start),
      title: dayTitle.format(slice[0].start),
      values: slice.map((bucket) => (bucket.start > anchor.value ? null : bucket.requests)),
    }
  })
  const top = peak as TrendBucket | null
  return { rows, peak: top ? peakFormat.format(top.start) : '' }
})

// ---------- Loading ----------
async function loadOps(startTime: string, endTime: string) {
  if (!adminSettingsStore.opsMonitoringEnabled) return { overview: null, platforms: [], histogram: [], errors: [] }
  const params = { start_time: startTime, end_time: endTime }
  // Ops data is optional detail: any part that fails is left out, and the page still renders.
  const [overview, throughput, latency, errors] = await Promise.allSettled([
    adminAPI.ops.getDashboardOverview(params),
    adminAPI.ops.getThroughputTrend(params),
    adminAPI.ops.getLatencyHistogram(params),
    adminAPI.ops.getErrorDistribution(params),
  ])
  for (const result of [overview, throughput, latency, errors]) {
    if (result.status === 'rejected') console.error('Failed to load ops detail for Stats:', result.reason)
  }
  return {
    overview: overview.status === 'fulfilled' ? overview.value : null,
    platforms: throughput.status === 'fulfilled' ? throughput.value.by_platform ?? [] : [],
    histogram: latency.status === 'fulfilled' ? latency.value.buckets ?? [] : [],
    errors: errors.status === 'fulfilled' ? errors.value.items ?? [] : [],
  }
}

async function loadUsage() {
  const sequence = ++usageSequence
  usageLoading.value = true
  usageError.value = false
  const now = new Date()
  const config = RANGES[range.value]
  const start = addDays(startOfDay(now), -(config.days - 1))
  const current = dateParams(start, config.days)
  const previous = dateParams(addDays(start, -config.days), config.days)
  const trendOnly = {
    granularity: config.granularity,
    include_stats: false,
    include_trend: true,
    include_model_stats: false,
    include_group_stats: false,
    include_users_trend: false,
  } as const
  try {
    const [snapshot, before, usage, usageBefore, heatSnapshot, opsDetail] = await Promise.all([
      adminAPI.dashboard.getSnapshotV2({ ...current, ...trendOnly, include_model_stats: true, include_group_stats: true }),
      adminAPI.dashboard.getSnapshotV2({ ...previous, ...trendOnly }),
      adminAPI.usage.getStats(current),
      adminAPI.usage.getStats(previous),
      adminAPI.dashboard.getSnapshotV2({ ...dateParams(addDays(startOfDay(now), -6), 7), ...trendOnly, granularity: 'hour' }),
      loadOps(start.toISOString(), addDays(startOfDay(now), 1).toISOString()),
    ])
    if (sequence !== usageSequence) return
    const generated = new Date(snapshot.generated_at)
    anchor.value = Number.isNaN(generated.getTime()) ? now : generated
    trend.value = snapshot.trend ?? []
    models.value = snapshot.models ?? []
    groups.value = snapshot.groups ?? []
    previousTrend.value = before.trend ?? []
    averageDuration.value = usage.total_requests ? usage.average_duration_ms : null
    previousAverageDuration.value = usageBefore.total_requests ? usageBefore.average_duration_ms : null
    heatTrend.value = heatSnapshot.trend ?? []
    ops.value = opsDetail
    loaded.value = true
    if (topTab.value === 'keys') void loadKeys()
  } catch (error) {
    if (sequence !== usageSequence) return
    usageError.value = true
    console.error('Failed to load Stats usage:', error)
  } finally {
    if (sequence === usageSequence) usageLoading.value = false
  }
}

/** API-key totals load only when their tab is opened. */
async function loadKeys() {
  const config = RANGES[range.value]
  const params = dateParams(addDays(startOfDay(new Date()), -(config.days - 1)), config.days)
  const cacheKey = `${params.start_date}:${params.end_date}`
  if (keysLoadedFor === cacheKey) return
  const sequence = ++keysSequence
  keysLoading.value = true
  try {
    const response = await adminAPI.dashboard.getApiKeyUsageTrend({ ...params, granularity: 'day', limit: TOP_ROWS })
    if (sequence !== keysSequence) return
    keyTrend.value = response.trend ?? []
    keysLoadedFor = cacheKey
  } catch (error) {
    if (sequence !== keysSequence) return
    keyTrend.value = []
    console.error('Failed to load API key usage:', error)
  } finally {
    if (sequence === keysSequence) keysLoading.value = false
  }
}

function setRange(next: RangeKey) {
  if (range.value === next) return
  range.value = next
  void loadUsage()
}

function setView(next: StatsViewKey) {
  if (view.value === next) return
  void router.replace({ query: { ...route.query, view: next === 'capacity' ? 'capacity' : undefined } })
}

watch(topTab, (tab) => {
  if (tab === 'keys') void loadKeys()
})

// Capacity needs the whole account fleet, so it loads only when its tab is shown.
watch(view, (current) => {
  if (current === 'capacity' && !fleet.accounts.value.length && !fleet.loading.value) void fleet.load()
}, { immediate: true })

onMounted(loadUsage)

onBeforeUnmount(() => {
  usageSequence += 1
  keysSequence += 1
})
</script>

<style scoped>
.stats-grow {
  flex: 1 1 auto;
}

.stats-compare {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--operator-muted-foreground);
  font-size: 0.78rem;
  cursor: pointer;
  user-select: none;
}

.stats-fill {
  display: flex;
  flex-direction: column;
}

.stats-heat-scale {
  gap: 3px;
}

.stats-heat-scale i {
  width: 14px;
  height: 10px;
  border-radius: 2px;
}

.stats-table-wrap {
  max-height: 290px;
  overflow-y: auto;
}

.stats-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8125rem;
}

.stats-table th {
  position: sticky;
  top: 0;
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--operator-border);
  background: var(--table-header-background);
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
  font-weight: 500;
  text-align: left;
}

.stats-table td {
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--operator-border);
  color: var(--operator-foreground);
}

.stats-table .is-number {
  font-variant-numeric: tabular-nums;
  text-align: right;
}

.stats-table .is-muted {
  color: var(--operator-subtle-foreground);
}

/* Narrow tiles keep the figure and drop the decorative trend line. */
@media (max-width: 640px) {
  .viz-kpi :deep(.spark-line) {
    display: none;
  }
}
</style>
