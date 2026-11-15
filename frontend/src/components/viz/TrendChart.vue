<template>
  <div ref="host" class="trend-chart" :style="{ height: `${height}px` }">
    <svg
      v-if="width > 0 && count > 0"
      :viewBox="`0 0 ${width} ${height}`"
      :width="width"
      :height="height"
      tabindex="0"
      role="img"
      :aria-label="`${summary}. ${t('admin.viz.chartKeys')}`"
      @pointermove="onPointer"
      @pointerleave="active = null"
      @blur="active = null"
      @keydown="onKey"
    >
      <rect
        v-if="type === 'bar' && active !== null"
        class="trend-band"
        :x="xOf(active) - bandWidth / 2"
        :y="margin.top"
        :width="bandWidth"
        :height="innerHeight"
        rx="4"
      />

      <g v-for="tick in scale.ticks" :key="tick">
        <line
          :class="tick === 0 ? 'trend-axis' : 'trend-grid'"
          :x1="margin.left"
          :x2="width - margin.right"
          :y1="yOf(tick)"
          :y2="yOf(tick)"
        />
        <text :x="margin.left - 8" :y="yOf(tick) + 4" text-anchor="end">{{ axisText(tick) }}</text>
      </g>
      <text
        v-for="tick in xTicks"
        :key="`x${tick.index}`"
        :x="xOf(tick.index)"
        :y="height - 6"
        :text-anchor="tick.anchor"
      >{{ labels[tick.index] }}</text>

      <template v-if="type === 'line'">
        <defs>
          <linearGradient :id="gradientId" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" :stop-color="mainSeries.color" stop-opacity="0.22" />
            <stop offset="1" :stop-color="mainSeries.color" stop-opacity="0" />
          </linearGradient>
        </defs>
        <path v-if="area" class="trend-area" :fill="`url(#${gradientId})`" :d="areaPath" />
        <path
          v-for="item in drawOrder"
          :key="item.name"
          class="trend-line"
          :class="{ 'is-muted': item.muted }"
          pathLength="1"
          :stroke="item.color"
          :d="linePath(item.values)"
        />
        <g v-if="active !== null">
          <line class="trend-hair" :x1="xOf(active)" :x2="xOf(active)" :y1="margin.top" :y2="margin.top + innerHeight" />
          <circle
            v-for="item in drawOrder"
            :key="`dot-${item.name}`"
            class="trend-dot"
            r="4.5"
            :cx="xOf(active)"
            :cy="yOf(item.values[active] ?? 0)"
            :fill="item.color"
          />
        </g>
      </template>

      <template v-else>
        <path
          v-for="(bar, index) in bars"
          :key="index"
          class="trend-bar"
          :style="{ animationDelay: `${index * 12}ms` }"
          :fill="mainSeries.color"
          :d="bar"
        />
      </template>
    </svg>

    <div v-if="active !== null" class="trend-tip" :style="tipStyle" role="presentation">
      <div class="trend-tip-title">{{ (tips ?? labels)[active] }}</div>
      <div v-for="item in series" :key="item.name" class="trend-tip-row">
        <i :style="{ background: item.color }"></i>
        <span>{{ item.name }}</span>
        <b>{{ formatValue(item.values[active] ?? 0) }}</b>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useElementSize } from '@vueuse/core'
import { formatAxisNumber, niceScale } from '@/utils/vizFormat'

export interface TrendSeries {
  name: string
  color: string
  values: number[]
  /** A comparison series, drawn thinner and behind the main one. */
  muted?: boolean
}

/**
 * Line or column chart with one shared y axis, a pointer and arrow-key hover
 * layer, and a tooltip. Colors come from CSS variables, so a theme change
 * needs no re-render.
 */
const props = withDefaults(defineProps<{
  labels: string[]
  tips?: string[]
  series: TrendSeries[]
  /** Accessible name of the chart. */
  summary: string
  type?: 'line' | 'bar'
  height?: number
  area?: boolean
  formatValue?: (value: number) => string
  formatAxis?: (value: number, axisMax: number) => string
}>(), {
  tips: undefined,
  type: 'line',
  height: 260,
  area: true,
  formatValue: (value: number) => value.toLocaleString('en-US'),
  formatAxis: formatAxisNumber,
})

const { t } = useI18n()
const host = ref<HTMLElement | null>(null)
const { width } = useElementSize(host)
const active = ref<number | null>(null)
const gradientId = `trend-gradient-${Math.random().toString(36).slice(2, 9)}`
const margin = { left: 46, right: 12, top: 10, bottom: 26 }

const count = computed(() => props.labels.length)
const innerWidth = computed(() => Math.max(0, width.value - margin.left - margin.right))
const innerHeight = computed(() => props.height - margin.top - margin.bottom)
const mainSeries = computed(() => props.series.find((item) => !item.muted) ?? props.series[0])
// Comparison series paint first so the main series stays on top.
const drawOrder = computed(() => [...props.series].sort((left, right) => Number(right.muted ?? false) - Number(left.muted ?? false)))
const scale = computed(() => niceScale(Math.max(0, ...props.series.flatMap((item) => item.values))))
const band = computed(() => (count.value ? innerWidth.value / count.value : 0))
const barWidth = computed(() => Math.max(2, Math.min(24, band.value * 0.64)))
const bandWidth = computed(() => Math.max(barWidth.value + 10, band.value))

function xOf(index: number): number {
  if (props.type === 'bar') return margin.left + band.value * (index + 0.5)
  return margin.left + (count.value === 1 ? innerWidth.value / 2 : (index / (count.value - 1)) * innerWidth.value)
}

function yOf(value: number): number {
  return margin.top + innerHeight.value - (value / scale.value.max) * innerHeight.value
}

const axisText = (tick: number) => props.formatAxis(tick, scale.value.max)

const linePath = (values: number[]) => values
  .map((value, index) => `${index ? 'L' : 'M'}${xOf(index).toFixed(1)} ${yOf(value).toFixed(1)}`)
  .join('')

const areaPath = computed(() => {
  const base = margin.top + innerHeight.value
  return `${linePath(mainSeries.value.values)}L${xOf(count.value - 1)} ${base}L${xOf(0)} ${base}Z`
})

// Columns keep a square baseline and a rounded data end.
const bars = computed(() => mainSeries.value.values.map((value, index) => {
  const barHeight = (value / scale.value.max) * innerHeight.value
  if (barHeight < 1) return ''
  const x = xOf(index) - barWidth.value / 2
  const top = margin.top + innerHeight.value - barHeight
  const radius = Math.min(4, barWidth.value / 2, barHeight)
  const right = x + barWidth.value
  return `M${x} ${top + barHeight}V${top + radius}Q${x} ${top} ${x + radius} ${top}H${right - radius}Q${right} ${top} ${right} ${top + radius}V${top + barHeight}Z`
}))

const xTicks = computed(() => {
  const total = count.value
  const fitsAll = props.type === 'bar' && total <= 8 && band.value >= 34
  const want = fitsAll ? total : Math.min(total, width.value < 420 ? 4 : width.value < 720 ? 6 : 8)
  const indexes = new Set<number>()
  for (let step = 0; step < want; step += 1) {
    indexes.add(want === 1 ? 0 : Math.round((step * (total - 1)) / (want - 1)))
  }
  const ordered = [...indexes]
  return ordered.map((index, position) => {
    const x = xOf(index)
    const anchor = position === 0 && x - margin.left < 20
      ? 'start'
      : position === ordered.length - 1 && width.value - margin.right - x < 20 ? 'end' : 'middle'
    return { index, anchor }
  })
})

const clampIndex = (index: number) => Math.max(0, Math.min(count.value - 1, index))

function onPointer(event: PointerEvent) {
  const box = host.value?.getBoundingClientRect()
  if (!box || !count.value) return
  const x = event.clientX - box.left - margin.left
  active.value = clampIndex(props.type === 'bar'
    ? Math.floor(x / band.value)
    : Math.round((x / innerWidth.value) * (count.value - 1)))
}

function onKey(event: KeyboardEvent) {
  if (event.key === 'ArrowLeft') active.value = clampIndex((active.value ?? count.value) - 1)
  else if (event.key === 'ArrowRight') active.value = clampIndex((active.value ?? -1) + 1)
  else if (event.key === 'Escape') active.value = null
  else return
  event.preventDefault()
}

// The tooltip flips to the left of the pointer near the right edge.
const tipStyle = computed(() => {
  if (active.value === null) return {}
  const x = xOf(active.value)
  return x > width.value * 0.6
    ? { right: `${width.value - x + 14}px`, top: `${margin.top + 8}px` }
    : { left: `${x + 14}px`, top: `${margin.top + 8}px` }
})
</script>

<style scoped>
.trend-chart {
  position: relative;
  width: 100%;
}

.trend-chart svg {
  display: block;
  overflow: visible;
  outline: none;
  touch-action: pan-y;
}

.trend-chart svg:focus-visible {
  border-radius: var(--operator-radius);
  outline: 2px solid var(--operator-focus);
  outline-offset: 4px;
}

.trend-chart text {
  fill: var(--operator-subtle-foreground);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.trend-grid { stroke: var(--operator-chart-grid); }
.trend-axis { stroke: var(--operator-chart-axis); }
.trend-hair { stroke: var(--operator-chart-axis); }
.trend-band { fill: var(--operator-foreground); opacity: 0.06; }

.trend-line {
  fill: none;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-dasharray: 1;
  stroke-dashoffset: 1;
  animation: trend-draw 900ms var(--operator-ease-out) forwards;
}

.trend-line.is-muted { stroke-width: 1.5; }

.trend-area {
  opacity: 0;
  animation: trend-fade 600ms 250ms ease-out forwards;
}

.trend-dot {
  stroke: var(--operator-card);
  stroke-width: 2;
}

.trend-bar {
  transform-box: fill-box;
  transform-origin: 50% 100%;
  animation: trend-grow 600ms var(--operator-ease-out) both;
}

.trend-tip {
  position: absolute;
  z-index: 5;
  min-width: 9rem;
  padding: 0.5rem 0.625rem;
  border: 1px solid var(--operator-border-strong);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
  box-shadow: var(--operator-shadow-md);
  font-size: 0.78rem;
  pointer-events: none;
}

.trend-tip-title {
  margin-bottom: 0.3rem;
  color: var(--operator-muted-foreground);
  font-size: 0.72rem;
  white-space: nowrap;
}

.trend-tip-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  line-height: 1.7;
}

.trend-tip-row i {
  width: 8px;
  height: 8px;
  flex: none;
  border-radius: 2px;
}

.trend-tip-row span {
  flex: 1;
  color: var(--operator-muted-foreground);
  white-space: nowrap;
}

.trend-tip-row b {
  color: var(--operator-foreground);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

@keyframes trend-draw { to { stroke-dashoffset: 0; } }
@keyframes trend-fade { to { opacity: 1; } }
@keyframes trend-grow { from { transform: scaleY(0); } }
</style>
