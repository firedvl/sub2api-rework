<template>
  <div class="mt-4 flex items-end justify-between gap-3">
    <div class="text-xs text-gray-500">
      {{ windowLabel }}
    </div>
    <div class="flex items-baseline gap-0.5">
      <span
        class="text-2xl font-semibold tabular-nums leading-none tracking-tight"
        :style="colorStyle"
      >
        {{ displayValue }}
      </span>
      <span
        class="text-sm font-medium leading-none"
        :style="colorStyle"
      >%</span>
    </div>
  </div>
  <div
    v-if="samplesLabel"
    class="mt-1 text-right text-xs text-gray-500"
  >
    {{ samplesLabel }}
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { hslForPct } from '@/composables/useChannelMonitorFormat'

const props = defineProps<{
  windowLabel: string
  value: number | null
  samplesLabel?: string
}>()

const { t } = useI18n()

const displayValue = computed(() => {
  if (props.value === null || Number.isNaN(props.value)) return t('monitorCommon.latencyEmpty')
  return props.value.toFixed(2)
})

const colorStyle = computed(() => {
  const colour = hslForPct(props.value)
  return colour ? { color: colour } : { color: 'var(--operator-subtle-foreground)' }
})
</script>
