<template>
  <article class="stats-bar-panel" :data-testid="testId">
    <header class="stats-bar-heading">
      <div>
        <h3>{{ title }}</h3>
        <p>
          <strong data-testid="stats-trend-total">{{ formatCompact(total) }}</strong>
          {{ t('admin.stats.usage.total') }} ·
          <strong data-testid="stats-trend-average">{{ formatCompact(average) }}</strong>
          {{ t('admin.stats.usage.average') }} ·
          <strong data-testid="stats-trend-peak">{{ formatCompact(peak) }}</strong>
          {{ t('admin.stats.usage.peak') }}
        </p>
      </div>
      <span>{{ periodLabel }}</span>
    </header>

    <div ref="chartRoot" class="stats-bar-chart" role="group" :aria-label="chartLabel">
      <span
        v-for="grid in gridLines"
        :key="`label-${grid.ratio}`"
        class="stats-bar-y-label"
        :style="{ top: `${grid.y}%` }"
      >{{ formatCompact(grid.value) }}</span>

      <span
        v-for="grid in gridLines"
        :key="grid.ratio"
        class="stats-bar-grid-line"
        :style="{ top: `${grid.y}%` }"
        aria-hidden="true"
      />

      <button
        v-for="(point, index) in chartPoints"
        :key="point.date"
        type="button"
        class="stats-trend-bar"
        :class="{
          'is-active': activeIndex === index,
          'is-zero': point.value === 0,
        }"
        :style="{
          left: `${point.x}%`,
          bottom: `${100 - CHART_BASELINE}%`,
          height: `${Math.max(point.height, MIN_BAR_HEIGHT)}%`,
          width: `${barWidth}%`,
        }"
        :aria-label="pointLabel(point)"
        :aria-describedby="activeIndex === index ? tooltipId : undefined"
        data-testid="stats-trend-bar"
        @mouseenter="activeIndex = index"
        @mouseleave="activeIndex = null"
        @focus="activeIndex = index"
        @blur="activeIndex = null"
        @keydown.esc="activeIndex = null"
        @keydown.left.prevent="focusBar(index - 1)"
        @keydown.right.prevent="focusBar(index + 1)"
        @keydown.home.prevent="focusBar(0)"
        @keydown.end.prevent="focusBar(chartPoints.length - 1)"
      ><span /></button>

      <span
        v-for="(point, index) in chartPoints"
        v-show="labelIndexes.has(index)"
        :key="`x-${point.date}`"
        class="stats-bar-x-label"
        :class="{
          'is-first': chartPoints.length > 1 && index === 0,
          'is-middle': index > 0 && index < chartPoints.length - 1,
          'is-last': chartPoints.length > 1 && index === chartPoints.length - 1,
        }"
        :style="{ left: `${point.x}%` }"
      >{{ formatTick(point.date) }}</span>

      <div
        v-if="activePoint"
        :id="tooltipId"
        class="stats-bar-tooltip"
        :class="{ 'is-below': activePoint.y < 28 }"
        role="tooltip"
        :style="tooltipStyle"
        data-testid="stats-trend-tooltip"
      >
        <strong>{{ formatCompact(activePoint.value) }} {{ unit }}</strong>
        <span>{{ formatTimestamp(activePoint.date) }}</span>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  title: string
  unit: string
  points: Array<{ date: string; value: number }>
  granularity: string
  testId: string
}>()

const { t } = useI18n()
const CHART_TOP = 6
const CHART_BASELINE = 78
const CHART_LEFT = 12
const CHART_RIGHT = 92
const MIN_BAR_HEIGHT = 1.25
const MAX_BAR_WIDTH = 14
const chartRoot = ref<HTMLElement | null>(null)
const activeIndex = ref<number | null>(null)
const tooltipId = computed(() => `${props.testId}-tooltip`)
const values = computed(() => props.points.map((point) => Number(point.value) || 0))
const total = computed(() => values.value.reduce((sum, value) => sum + value, 0))
const average = computed(() => values.value.length ? total.value / values.value.length : 0)
const peak = computed(() => Math.max(...values.value, 0))
const scaleMaximum = computed(() => {
  if (peak.value <= 1) return 1
  const magnitude = 10 ** Math.floor(Math.log10(peak.value))
  const normalized = peak.value / magnitude
  return (normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10) * magnitude
})
const chartPoints = computed(() => props.points.map((point, index, points) => {
  const value = Number(point.value) || 0
  const height = value / scaleMaximum.value * (CHART_BASELINE - CHART_TOP)
  const bandWidth = (CHART_RIGHT - CHART_LEFT) / Math.max(points.length, 1)
  return {
    ...point,
    value,
    height,
    x: CHART_LEFT + (index + 0.5) * bandWidth,
    y: CHART_BASELINE - height,
  }
}))
const barWidth = computed(() => {
  const bandWidth = (CHART_RIGHT - CHART_LEFT) / Math.max(chartPoints.value.length, 1)
  return Math.min(MAX_BAR_WIDTH, bandWidth * 0.9)
})
const gridLines = computed(() => [1, 2 / 3, 1 / 3, 0].map((ratio) => ({
  ratio,
  value: Math.round(scaleMaximum.value * ratio),
  y: CHART_TOP + (1 - ratio) * (CHART_BASELINE - CHART_TOP),
})))
const labelIndexes = computed(() => {
  const last = chartPoints.value.length - 1
  if (last < 4) return new Set(chartPoints.value.map((_, index) => index))
  return new Set([0, Math.round(last / 3), Math.round(last * 2 / 3), last])
})
const activePoint = computed(() => activeIndex.value === null
  ? null
  : chartPoints.value[activeIndex.value] ?? null)
const tooltipStyle = computed(() => {
  const point = activePoint.value
  if (!point) return {}
  return {
    left: `${Math.min(82, Math.max(18, point.x))}%`,
    top: `${point.y}%`,
  }
})
const periodLabel = computed(() => t(
  props.granularity === 'hour'
    ? 'admin.stats.usage.recentHourlyPeriods'
    : 'admin.stats.usage.recentDailyPeriods',
  { count: props.points.length },
))
const chartLabel = computed(() => `${props.title}. ${periodLabel.value}. ${formatNumber(total.value)} ${props.unit}.`)

const formatNumber = (value: number) => value.toLocaleString()
const formatCompact = (value: number) => {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(1)}B`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`
  if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`
  return value.toLocaleString(undefined, { maximumFractionDigits: 1 })
}
const formatTimestamp = (value: string) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}
const formatTick = (value: string) => {
  const date = new Date(value)
  return Number.isNaN(date.getTime())
    ? value
    : new Intl.DateTimeFormat(undefined, props.granularity === 'hour'
        ? { hour: 'numeric' }
        : { month: 'short', day: 'numeric' })
      .format(date)
}
const pointLabel = (point: { date: string; value: number }) => (
  `${formatTimestamp(point.date)}: ${formatNumber(point.value)} ${props.unit}`
)
const focusBar = (index: number) => {
  const bars = chartRoot.value?.querySelectorAll<HTMLButtonElement>('.stats-trend-bar')
  if (!bars?.length) return
  bars[Math.min(bars.length - 1, Math.max(0, index))]?.focus()
}
</script>

<style scoped>
.stats-bar-panel { min-width: 0; padding: 1.25rem 1.5rem 1.5rem; }
.stats-bar-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; }
.stats-bar-heading h3 { color: var(--operator-foreground); font-size: 0.875rem; font-weight: 650; }
.stats-bar-heading p,
.stats-bar-heading > span { color: var(--operator-muted-foreground); font-size: 0.75rem; }
.stats-bar-heading p { margin-top: 0.2rem; }
.stats-bar-heading p strong { color: var(--operator-foreground); font-weight: 600; }

.stats-bar-chart { position: relative; height: clamp(10rem, 15vw, 12rem); margin-top: 0.875rem; overflow: hidden; }
.stats-bar-grid-line {
  position: absolute;
  right: 0;
  left: 12%;
  height: 1px;
  background: var(--operator-border-subtle);
}
.stats-bar-y-label,
.stats-bar-x-label {
  position: absolute;
  color: var(--operator-muted-foreground);
  font-size: 0.625rem;
  line-height: 1;
  pointer-events: none;
  white-space: nowrap;
}
.stats-bar-y-label { left: 0; transform: translateY(-50%); }
.stats-bar-x-label { top: 87%; transform: translateX(-50%); }
.stats-bar-x-label.is-first { transform: none; }
.stats-bar-x-label.is-last { transform: translateX(-100%); }

.stats-trend-bar {
  position: absolute;
  min-height: 0.4rem;
  transform: translateX(-50%);
  border: 0;
  background: transparent;
  cursor: default;
}
.stats-trend-bar > span {
  position: absolute;
  inset: 0;
  border: 1px solid var(--operator-card);
  border-radius: 2px 2px 0 0;
  background: var(--operator-foreground);
  opacity: 0.72;
}
.stats-trend-bar:hover > span,
.stats-trend-bar.is-active > span {
  border-color: var(--operator-focus);
  box-shadow: 0 0 0 1px var(--operator-focus), 0 2px 6px rgb(0 0 0 / 18%);
  opacity: 1;
}
.stats-trend-bar.is-zero > span { height: 2px; top: auto; }
.stats-trend-bar:focus-visible { outline: 2px solid var(--operator-focus); outline-offset: 2px; border-radius: 2px; }

.stats-bar-tooltip {
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
.stats-bar-tooltip.is-below { transform: translate(-50%, 0.6rem); }
.stats-bar-tooltip span { color: var(--operator-muted-foreground); font-size: 0.625rem; }

@media (max-width: 640px) {
  .stats-bar-panel { padding-inline: 1rem; }
  .stats-bar-heading { flex-direction: column; gap: 0.25rem; }
  .stats-bar-chart { height: 11rem; }
  .stats-bar-x-label.is-middle { display: none; }
}
</style>
