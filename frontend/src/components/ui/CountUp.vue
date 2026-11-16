<template>
  <span class="tabular-nums">{{ text }}</span>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'

/**
 * Animates a number from its previous value to the new one. The first render
 * counts up from zero; reduced motion and non-finite values render directly.
 */
const props = withDefaults(defineProps<{
  value: number | null | undefined
  format?: (value: number) => string
  duration?: number
}>(), {
  format: (value: number) => Math.round(value).toLocaleString('en-US'),
  duration: 650,
})

const shown = ref(0)
let frame = 0

const reduced = typeof window !== 'undefined'
  && typeof window.matchMedia === 'function'
  && window.matchMedia('(prefers-reduced-motion: reduce)').matches

const text = computed(() => {
  if (props.value == null || !Number.isFinite(props.value)) return '–'
  return props.format(shown.value)
})

function animate(to: number) {
  cancelAnimationFrame(frame)
  const from = shown.value
  if (reduced || from === to || typeof requestAnimationFrame !== 'function') {
    shown.value = to
    return
  }
  const start = performance.now()
  const step = (now: number) => {
    const k = Math.min(1, (now - start) / props.duration)
    const eased = 1 - Math.pow(1 - k, 3)
    shown.value = from + (to - from) * eased
    if (k < 1) frame = requestAnimationFrame(step)
    else shown.value = to
  }
  frame = requestAnimationFrame(step)
}

watch(
  () => props.value,
  (value) => {
    if (value == null || !Number.isFinite(value)) return
    animate(value)
  },
  { immediate: true },
)

onBeforeUnmount(() => cancelAnimationFrame(frame))
</script>
