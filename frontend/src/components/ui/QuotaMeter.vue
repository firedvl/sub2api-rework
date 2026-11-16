<template>
  <div
    class="ui-meter"
    :class="toneClass"
    role="img"
    :aria-label="value == null ? `${label}: –` : `${label}: ${Math.round(value)}%`"
  >
    <div class="ui-meter-track">
      <i v-if="value != null" :style="{ width: `${Math.max(Math.min(value, 100), 2)}%` }"></i>
    </div>
    <b>{{ value == null ? '–' : `${Math.round(value)}%` }}</b>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

/** Remaining-capacity bar. Green above 30%, amber above 10%, red at or below 10%. */
const props = defineProps<{
  value: number | null | undefined
  label: string
}>()

const toneClass = computed(() => {
  const value = props.value
  if (value == null) return 'is-none'
  if (value <= 10) return 'is-bad'
  if (value <= 30) return 'is-warn'
  return 'is-ok'
})
</script>
