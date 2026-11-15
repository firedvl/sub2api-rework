<template>
  <div
    class="ring-gauge"
    :style="{ width: `${size}px`, height: `${size}px`, fontSize: `${Math.round(size * 0.2)}px` }"
    role="img"
    :aria-label="label ? `${label}: ${text}` : text"
  >
    <svg :width="size" :height="size" :viewBox="`0 0 ${size} ${size}`" aria-hidden="true">
      <circle class="ring-track" :cx="size / 2" :cy="size / 2" :r="radius" :stroke-width="stroke" />
      <circle
        v-if="value !== null"
        class="ring-value"
        :class="`is-${tone}`"
        :cx="size / 2"
        :cy="size / 2"
        :r="radius"
        :stroke-width="stroke"
        :stroke-dasharray="circumference"
        :stroke-dashoffset="circumference * (1 - clamped / 100)"
      />
    </svg>
    <b>{{ text }}</b>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

/** Remaining capacity as a ring. A null value renders an empty ring and a dash. */
const props = withDefaults(defineProps<{
  value: number | null
  size?: number
  label?: string
}>(), { size: 84, label: '' })

const stroke = computed(() => (props.size >= 96 ? 9 : 7))
const radius = computed(() => (props.size - stroke.value - 3) / 2)
const circumference = computed(() => 2 * Math.PI * radius.value)
const clamped = computed(() => Math.min(100, Math.max(0, props.value ?? 0)))
const tone = computed(() => (clamped.value <= 10 ? 'bad' : clamped.value <= 30 ? 'warn' : 'ok'))
const text = computed(() => (props.value === null ? '–' : `${Math.round(clamped.value)}%`))
</script>

<style scoped>
.ring-gauge {
  position: relative;
  flex: none;
}

.ring-gauge svg {
  transform: rotate(-90deg);
}

.ring-gauge circle {
  fill: none;
}

.ring-track {
  stroke: var(--operator-raised);
}

.ring-value {
  stroke-linecap: round;
  transition: stroke-dashoffset 900ms var(--operator-ease-out);
}

.ring-value.is-ok { stroke: var(--operator-success-fill); }
.ring-value.is-warn { stroke: var(--operator-warning-fill); }
.ring-value.is-bad { stroke: var(--operator-destructive-fill); }

.ring-gauge b {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  color: var(--operator-foreground);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.02em;
}
</style>
