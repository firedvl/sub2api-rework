<template>
  <dl class="mt-4 grid grid-cols-2 gap-3 text-xs">
    <div class="min-w-0">
      <dt class="flex items-center gap-1 text-gray-500">
        <Icon :name="primaryIcon" size="xs" />
        <span class="truncate">{{ primaryLabel }}</span>
      </dt>
      <dd class="mt-0.5 text-base font-semibold tabular-nums text-gray-900 dark:text-white">
        {{ primaryValue }}<span class="ml-0.5 text-xs font-normal text-gray-500">{{ primaryUnit }}</span>
      </dd>
    </div>
    <div class="min-w-0">
      <dt class="flex items-center gap-1 text-gray-500">
        <Icon :name="secondaryIcon" size="xs" />
        <span class="truncate">{{ secondaryLabel }}</span>
      </dt>
      <dd class="mt-0.5 text-base font-semibold tabular-nums text-gray-900 dark:text-white">
        {{ secondaryValue }}<span class="ml-0.5 text-xs font-normal text-gray-500">{{ secondaryUnit }}</span>
      </dd>
    </div>
  </dl>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  primaryLabel: string
  primaryValue: string
  primaryUnit: string
  primaryIcon: 'bolt' | 'globe' | 'clock' | 'link'
  secondaryLabel: string
  secondaryValue: string
  secondaryUnit: string
  secondaryIcon: 'bolt' | 'globe' | 'clock' | 'link'
}>()
</script>
