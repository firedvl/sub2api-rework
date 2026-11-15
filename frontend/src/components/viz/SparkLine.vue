<template>
  <svg class="spark-line" :width="width" :height="height" :viewBox="`0 0 ${width} ${height}`" aria-hidden="true">
    <template v-if="points.length > 1">
      <path :d="`${path}L${width - 2} ${height}L2 ${height}Z`" :fill="color" opacity="0.12" />
      <path :d="path" fill="none" :stroke="color" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round" />
      <circle :cx="points[points.length - 1][0]" :cy="points[points.length - 1][1]" r="2.5" :fill="color" />
    </template>
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'

/** Decorative trend line beside a figure. The figure itself carries the value. */
const props = withDefaults(defineProps<{
  values: number[]
  color?: string
  width?: number
  height?: number
}>(), { color: 'var(--operator-accent-text)', width: 72, height: 30 })

const points = computed(() => {
  const values = props.values.filter((value) => Number.isFinite(value))
  if (values.length < 2) return []
  const max = Math.max(...values)
  const min = Math.min(...values)
  const span = max - min || 1
  return values.map((value, index): [number, number] => [
    (index / (values.length - 1)) * (props.width - 4) + 2,
    props.height - 3 - ((value - min) / span) * (props.height - 8),
  ])
})

const path = computed(() => points.value
  .map(([x, y], index) => `${index ? 'L' : 'M'}${x.toFixed(1)} ${y.toFixed(1)}`)
  .join(''))
</script>

<style scoped>
.spark-line {
  display: block;
  flex: none;
  overflow: visible;
}
</style>
