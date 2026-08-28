<template>
  <span
    v-if="version"
    class="inline-flex w-fit items-center rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-dark-800 dark:text-dark-400"
  >
    v{{ version }}
  </span>
</template>

<script setup lang="ts">
defineProps<{ version: string }>()
</script>
