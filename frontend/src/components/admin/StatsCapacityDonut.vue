<template>
  <div class="stats-capacity-donut" :data-testid="testId">
    <div class="stats-capacity-chart">
      <svg viewBox="0 0 120 120" role="img" :aria-label="chartLabel">
        <title>{{ chartLabel }}</title>
        <circle class="stats-capacity-ring is-background" cx="60" cy="60" r="46" pathLength="100" />
        <circle
          v-if="pool.usedPercent === null"
          class="stats-capacity-ring is-unknown"
          cx="60"
          cy="60"
          r="46"
          pathLength="100"
          tabindex="0"
          data-testid="capacity-unknown-ring"
          :aria-label="t('admin.dashboard.capacity.quotaUnknown')"
        >
          <title>{{ t('admin.dashboard.capacity.quotaUnknown') }}</title>
        </circle>
        <circle
          v-else
          class="stats-capacity-ring is-used"
          cx="60"
          cy="60"
          r="46"
          pathLength="100"
          tabindex="0"
          data-testid="capacity-used-segment"
          :aria-label="t('admin.dashboard.capacity.poolUsedValue', { value: formatPercent(pool.usedPercent) })"
          :style="{
            strokeDasharray: `${pool.usedPercent} ${100 - pool.usedPercent}`,
          }"
        >
          <title>{{ t('admin.dashboard.capacity.poolUsedValue', { value: formatPercent(pool.usedPercent) }) }}</title>
        </circle>
        <circle
          v-for="segment in chartSegments"
          :key="segment.summary.account.id"
          class="stats-capacity-ring is-account"
          :class="segmentTone(segment.index)"
          cx="60"
          cy="60"
          r="46"
          pathLength="100"
          tabindex="0"
          data-testid="capacity-account-segment"
          :aria-label="segmentLabel(segment)"
          :style="{
            strokeDasharray: `${segment.contributionPercent} ${100 - segment.contributionPercent}`,
            strokeDashoffset: `${-segment.offset}`,
          }"
        >
          <title>{{ segmentLabel(segment) }}</title>
        </circle>
      </svg>
      <div class="stats-capacity-chart-label" aria-hidden="true">
        <strong>{{ pool.remainingPercent === null ? t('common.unknown') : `${formatPercent(pool.remainingPercent)}%` }}</strong>
        <span>{{ t('admin.dashboard.capacity.poolAvailable') }}</span>
      </div>
    </div>

    <div class="stats-capacity-copy">
      <h3>{{ title }}</h3>
      <p>
        {{ t('admin.stats.capacity.coverage', { known: pool.knownCount, unknown: pool.unknownCount }) }}
      </p>
      <ul class="stats-capacity-legend" :aria-label="t('admin.dashboard.capacity.poolLegend')">
        <li v-if="pool.usedPercent !== null">
          <span class="stats-capacity-swatch is-used" aria-hidden="true" />
          <span><strong>{{ t('admin.dashboard.capacity.poolUsed') }}</strong></span>
          <strong>{{ formatPercent(pool.usedPercent) }}%</strong>
        </li>
        <li v-for="segment in chartSegments" :key="`legend-${segment.summary.account.id}`">
          <span class="stats-capacity-swatch" :class="segmentTone(segment.index)" aria-hidden="true" />
          <span>
            <strong>{{ segment.summary.account.name }}</strong>
            <small>{{ t('admin.dashboard.capacity.remaining', { value: formatPercent(segment.summary.lowestRemaining) }) }}</small>
          </span>
          <strong>{{ t('admin.dashboard.capacity.poolContribution', { value: formatPercent(segment.contributionPercent) }) }}</strong>
        </li>
      </ul>
      <p v-if="pool.unknownAccounts.length" class="stats-capacity-unknown-list">
        <strong>{{ t('admin.dashboard.capacity.quotaUnknown') }}:</strong>
        {{ pool.unknownAccounts.map((summary) => summary.account.name).join(', ') }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OperatorPoolCapacity } from '@/utils/operatorCapacity'
import { formatDateTimeToMinute } from '@/utils/format'

const props = withDefaults(defineProps<{
  title: string
  pool: OperatorPoolCapacity
  testId?: string
}>(), {
  testId: undefined,
})

const { t } = useI18n()
const formatPercent = (value: number) => value.toFixed(2).replace(/\.?0+$/, '')
const segmentTone = (index: number) => `is-segment-${index % 6}`
const chartSegments = computed(() => {
  let offset = props.pool.usedPercent ?? 0
  return props.pool.segments.map((segment, index) => {
    const chartSegment = { ...segment, index, offset }
    offset += segment.contributionPercent
    return chartSegment
  })
})
const segmentLabel = (segment: OperatorPoolCapacity['segments'][number]) => {
  const limitingWindow = segment.summary.windows.find(
    (window) => window.remainingPercent === segment.summary.lowestRemaining,
  )
  const reset = limitingWindow?.resetsAt
    ? t('admin.dashboard.capacity.resets', {
        time: formatDateTimeToMinute(limitingWindow.resetsAt) || t('common.unknown'),
      })
    : t('admin.dashboard.capacity.resetUnknown')

  return [
    segment.summary.account.name,
    props.title,
    t('admin.dashboard.capacity.remaining', { value: formatPercent(segment.summary.lowestRemaining) }),
    t('admin.dashboard.capacity.poolContribution', { value: formatPercent(segment.contributionPercent) }),
    limitingWindow?.label,
    reset,
    t(`admin.dashboard.capacity.health.${segment.summary.health}`),
    segment.summary.account.schedulable
      ? t('admin.dashboard.capacity.schedulable')
      : t('admin.dashboard.capacity.notSchedulable'),
  ].filter(Boolean).join(', ')
}
const chartLabel = computed(() => props.pool.remainingPercent === null
  ? `${props.title}: ${t('admin.dashboard.capacity.quotaUnknown')}`
  : `${props.title}: ${formatPercent(props.pool.remainingPercent)}% ${t('admin.dashboard.capacity.poolAvailable')}, ${formatPercent(props.pool.usedPercent as number)}% ${t('admin.dashboard.capacity.poolUsed')}`)
</script>

<style scoped>
.stats-capacity-donut {
  display: grid;
  grid-template-columns: minmax(9rem, 11rem) minmax(0, 1fr);
  gap: 1.25rem;
  align-items: start;
}

.stats-capacity-chart {
  position: relative;
  width: 100%;
  aspect-ratio: 1;
}

.stats-capacity-chart svg {
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}

.stats-capacity-ring {
  fill: none;
  stroke-width: 12;
}

.stats-capacity-ring.is-background { stroke: var(--operator-border); }
.stats-capacity-ring.is-used,
.stats-capacity-swatch.is-used { stroke: var(--operator-muted-foreground); background: var(--operator-muted-foreground); }
.stats-capacity-ring.is-unknown { stroke: var(--operator-muted-foreground); stroke-dasharray: 3 4; }
.stats-capacity-ring.is-account { transition: stroke-width 120ms ease; }
.stats-capacity-ring:is(:hover, :focus) { outline: none; stroke-width: 16; }
.is-segment-0 { stroke: #16a34a; background: #16a34a; }
.is-segment-1 { stroke: #ca8a04; background: #ca8a04; }
.is-segment-2 { stroke: #be123c; background: #be123c; }
.is-segment-3 { stroke: #65a30d; background: #65a30d; }
.is-segment-4 { stroke: #ea580c; background: #ea580c; }
.is-segment-5 { stroke: #a21caf; background: #a21caf; }

.stats-capacity-chart-label {
  position: absolute;
  inset: 50% auto auto 50%;
  display: flex;
  width: 5.5rem;
  transform: translate(-50%, -50%);
  flex-direction: column;
  text-align: center;
}

.stats-capacity-chart-label strong { color: var(--operator-foreground); font-size: 1.25rem; }
.stats-capacity-chart-label span,
.stats-capacity-copy p,
.stats-capacity-legend small { color: var(--operator-muted-foreground); font-size: 0.75rem; }
.stats-capacity-copy h3 { color: var(--operator-foreground); font-size: 0.9375rem; font-weight: 650; }
.stats-capacity-copy > p { margin-top: 0.25rem; }

.stats-capacity-legend {
  display: grid;
  gap: 0.45rem;
  margin-top: 0.875rem;
}

.stats-capacity-legend li {
  display: grid;
  grid-template-columns: 0.625rem minmax(0, 1fr) auto;
  gap: 0.5rem;
  align-items: center;
  color: var(--operator-foreground);
  font-size: 0.75rem;
}

.stats-capacity-legend li > span:nth-child(2) { display: flex; min-width: 0; flex-direction: column; }
.stats-capacity-legend li > strong { text-align: right; font-size: 0.6875rem; }
.stats-capacity-swatch { width: 0.625rem; height: 0.625rem; border-radius: 2px; }
.stats-capacity-unknown-list { margin-top: 0.75rem; overflow-wrap: anywhere; }

@media (max-width: 640px) {
  .stats-capacity-donut { grid-template-columns: 1fr; }
  .stats-capacity-chart { max-width: 10rem; margin-inline: auto; }
}
</style>
