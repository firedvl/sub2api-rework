<template>
  <span
    class="provider-mark"
    :style="{ '--mark-size': `${size}px`, '--mark-color': color }"
    :role="label ? 'img' : undefined"
    :aria-label="label || undefined"
    :aria-hidden="label ? undefined : 'true'"
  >
    <PlatformIcon v-if="platform" :platform="platform as never" />
    <slot v-else />
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { providerColor } from '@/utils/providerSeries'

/** Provider logo on a tile tinted with the provider's chart color. */
const props = withDefaults(defineProps<{
  platform: string
  size?: number
  /** Accessible name. Omit when the provider's name is visible beside the mark. */
  label?: string
}>(), { size: 24, label: '' })

const color = computed(() => providerColor(props.platform))
</script>

<style scoped>
.provider-mark {
  display: inline-grid;
  width: var(--mark-size);
  height: var(--mark-size);
  flex: none;
  place-items: center;
  border-radius: calc(var(--mark-size) * 0.28);
  background: color-mix(in srgb, var(--mark-color) 18%, var(--operator-card));
  box-shadow: inset 0 0 0 1.5px color-mix(in srgb, var(--mark-color) 62%, transparent);
  color: var(--operator-foreground);
}

.provider-mark :deep(svg) {
  width: 58%;
  height: 58%;
}
</style>
