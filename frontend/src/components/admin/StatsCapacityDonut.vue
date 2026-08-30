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
          role="button"
          tabindex="0"
          data-testid="capacity-unknown-ring"
          :aria-label="t('admin.dashboard.capacity.quotaUnknown')"
          :aria-pressed="resolvedSelectedKey === 'unknown'"
          @mouseenter="hoveredKey = 'unknown'"
          @mouseleave="hoveredKey = null"
          @focus="hoveredKey = 'unknown'"
          @blur="hoveredKey = null"
          @click="selectedKey = 'unknown'"
          @keydown.enter.prevent="selectedKey = 'unknown'"
          @keydown.space.prevent="selectedKey = 'unknown'"
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
          role="button"
          tabindex="0"
          data-testid="capacity-used-segment"
          :aria-label="t('admin.dashboard.capacity.poolUsedValue', { value: formatPercent(pool.usedPercent) })"
          :aria-pressed="resolvedSelectedKey === 'used'"
          :style="{
            strokeDasharray: `${pool.usedPercent} ${100 - pool.usedPercent}`,
          }"
          @mouseenter="hoveredKey = 'used'"
          @mouseleave="hoveredKey = null"
          @focus="hoveredKey = 'used'"
          @blur="hoveredKey = null"
          @click="selectedKey = 'used'"
          @keydown.enter.prevent="selectedKey = 'used'"
          @keydown.space.prevent="selectedKey = 'used'"
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
          role="button"
          tabindex="0"
          data-testid="capacity-account-segment"
          :aria-label="segmentLabel(segment)"
          :aria-pressed="resolvedSelectedKey === segmentKey(segment)"
          :style="{
            strokeDasharray: `${segment.contributionPercent} ${100 - segment.contributionPercent}`,
            strokeDashoffset: `${-segment.offset}`,
          }"
          @mouseenter="hoveredKey = segmentKey(segment)"
          @mouseleave="hoveredKey = null"
          @focus="hoveredKey = segmentKey(segment)"
          @blur="hoveredKey = null"
          @click="selectedKey = segmentKey(segment)"
          @keydown.enter.prevent="selectedKey = segmentKey(segment)"
          @keydown.space.prevent="selectedKey = segmentKey(segment)"
        >
          <title>{{ segmentLabel(segment) }}</title>
        </circle>
      </svg>
      <div class="stats-capacity-chart-label" aria-hidden="true">
        <strong>{{ pool.remainingPercent === null ? t('common.unknown') : `${formatPercent(pool.remainingPercent)}%` }}</strong>
        <span>{{ t('admin.dashboard.capacity.poolAvailable') }}</span>
      </div>
      <div v-if="hoveredDetail" class="stats-capacity-tooltip" role="status">
        <strong>{{ hoveredDetail.label }}</strong>
        <span>{{ hoveredDetail.value }}</span>
      </div>
    </div>

    <div class="stats-capacity-copy">
      <h3>{{ title }}</h3>
      <p>
        {{ t('admin.stats.capacity.coverage', { known: pool.knownCount, unknown: pool.unknownCount }) }}
      </p>
      <div class="stats-capacity-select-label">
        <span>Inspect segment</span>
        <Select
          :model-value="resolvedSelectedKey"
          :options="detailOptions"
          value-key="key"
          label-key="label"
          aria-label="Inspect segment"
          @update:model-value="selectFromControl"
        />
      </div>
      <div v-if="selectedDetail" class="stats-capacity-selected" data-testid="capacity-selected-detail">
        <span :class="['stats-capacity-swatch', selectedDetail.tone]" aria-hidden="true" />
        <div>
          <strong>{{ selectedDetail.label }}</strong>
          <p :title="selectedDetail.detail">{{ selectedDetail.detail }}</p>
        </div>
        <strong>{{ selectedDetail.value }}</strong>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
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
const segmentKey = (segment: OperatorPoolCapacity['segments'][number]) => `account:${segment.summary.account.id}`
interface CapacityDetail {
  [key: string]: unknown
  key: string
  label: string
  value: string
  detail: string
  tone: string
}
const detailOptions = computed<CapacityDetail[]>(() => {
  const details: CapacityDetail[] = []
  if (props.pool.usedPercent !== null) {
    details.push({
      key: 'used',
      label: t('admin.dashboard.capacity.poolUsed'),
      value: `${formatPercent(props.pool.usedPercent)}%`,
      detail: props.title,
      tone: 'is-used',
    })
  }
  details.push(...chartSegments.value.map((segment) => ({
    key: segmentKey(segment),
    label: segment.summary.account.name,
    value: t('admin.dashboard.capacity.remaining', { value: formatPercent(segment.summary.lowestRemaining) }),
    detail: segmentLabel(segment),
    tone: segmentTone(segment.index),
  })))
  if (props.pool.unknownAccounts.length) {
    details.push({
      key: 'unknown',
      label: t('admin.dashboard.capacity.quotaUnknown'),
      value: String(props.pool.unknownAccounts.length),
      detail: props.pool.unknownAccounts.map((summary) => summary.account.name).join(', '),
      tone: 'is-unknown',
    })
  }
  return details
})
const selectedKey = ref('')
const hoveredKey = ref<string | null>(null)
const resolvedSelectedKey = computed(() => detailOptions.value.some((option) => option.key === selectedKey.value)
  ? selectedKey.value
  : detailOptions.value[0]?.key ?? '')
const selectedDetail = computed(() => detailOptions.value.find((option) => option.key === resolvedSelectedKey.value) ?? null)
const hoveredDetail = computed(() => detailOptions.value.find((option) => option.key === hoveredKey.value) ?? null)
const selectFromControl = (value: string | number | boolean | null) => {
  selectedKey.value = String(value ?? '')
}
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
.stats-capacity-swatch.is-unknown { background: var(--operator-muted-foreground); }
.stats-capacity-ring:is(.is-account, .is-used, .is-unknown) { cursor: pointer; transition: stroke-width 120ms ease; }
.stats-capacity-ring:is(:hover, :focus) { outline: none; stroke-width: 16; }
.is-segment-0 { stroke: #16a34a; background: #16a34a; }
.is-segment-1 { stroke: #ca8a04; background: #ca8a04; }
.is-segment-2 { stroke: #0891b2; background: #0891b2; }
.is-segment-3 { stroke: #65a30d; background: #65a30d; }
.is-segment-4 { stroke: #2563eb; background: #2563eb; }
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
.stats-capacity-copy p { color: var(--operator-muted-foreground); font-size: 0.75rem; }
.stats-capacity-copy h3 { color: var(--operator-foreground); font-size: 0.9375rem; font-weight: 650; }
.stats-capacity-copy > p { margin-top: 0.25rem; }

.stats-capacity-tooltip {
  position: absolute;
  right: 0.25rem;
  bottom: 0.25rem;
  left: 0.25rem;
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.4rem 0.55rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: color-mix(in oklch, var(--operator-card) 94%, transparent);
  box-shadow: var(--operator-shadow-xs);
  color: var(--operator-foreground);
  font-size: 0.6875rem;
  pointer-events: none;
}

.stats-capacity-tooltip strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stats-capacity-select-label {
  display: grid;
  gap: 0.35rem;
  margin-top: 0.875rem;
  color: var(--operator-muted-foreground);
  font-size: 0.6875rem;
}

.stats-capacity-select-label :deep(.select-trigger) {
  min-height: 2.25rem;
  padding: 0.375rem 0.625rem;
  border-radius: var(--operator-radius);
  font-size: 0.75rem;
}

.stats-capacity-selected {
  display: grid;
  margin-top: 0.625rem;
  padding: 0.625rem;
  grid-template-columns: 0.625rem minmax(0, 1fr) auto;
  gap: 0.5rem;
  align-items: center;
  border: 1px solid var(--operator-border-subtle);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
  color: var(--operator-foreground);
  font-size: 0.75rem;
}
.stats-capacity-selected > div { min-width: 0; }
.stats-capacity-selected p {
  overflow: hidden;
  margin-top: 0.125rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.stats-capacity-selected > strong { text-align: right; font-size: 0.6875rem; }
.stats-capacity-swatch { width: 0.625rem; height: 0.625rem; border-radius: 2px; }

@media (max-width: 640px) {
  .stats-capacity-donut { grid-template-columns: 1fr; }
  .stats-capacity-chart { max-width: 10rem; margin-inline: auto; }
}
</style>
