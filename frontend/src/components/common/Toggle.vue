<template>
  <button
    type="button"
    :disabled="disabled"
    @click="toggle"
    class="operator-toggle relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-normal ease-ui-out disabled:cursor-not-allowed disabled:opacity-50"
    :class="[modelValue ? 'operator-toggle-on' : 'operator-toggle-off']"
    role="switch"
    :aria-checked="modelValue"
  >
    <span
      class="operator-toggle-thumb pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow-sm ring-0 transition-transform duration-normal ease-ui-out"
      :class="[modelValue ? 'translate-x-5' : 'translate-x-0']"
    />
  </button>
</template>

<script setup lang="ts">
const props = defineProps<{
  modelValue: boolean
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

function toggle() {
  if (props.disabled) return
  emit('update:modelValue', !props.modelValue)
}
</script>
