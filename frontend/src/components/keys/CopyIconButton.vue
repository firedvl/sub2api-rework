<template>
  <button
    type="button"
    class="copy-icon-button"
    :class="{ 'is-copied': copied }"
    :aria-label="copied ? copiedLabel : label"
    :title="copied ? copiedLabel : label"
    @click.stop="copy"
  >
    <Transition name="copy-swap" mode="out-in">
      <Icon v-if="copied" key="check" name="check" size="sm" :stroke-width="2.25" />
      <Icon v-else key="copy" name="copy" size="sm" />
    </Transition>
  </button>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'

/** Icon-only copy button whose glyph morphs into a check after a successful copy. */
const props = withDefaults(defineProps<{
  text: string
  label: string
  copiedLabel: string
  /** Toast text; empty keeps the copy silent apart from the check. */
  toast?: string
}>(), { toast: '' })
const emit = defineEmits<{ copied: [] }>()

const { copyToClipboard } = useClipboard()
const copied = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined

async function copy() {
  const ok = await copyToClipboard(props.text, props.toast || props.copiedLabel)
  if (!ok) return
  copied.value = true
  emit('copied')
  clearTimeout(timer)
  timer = setTimeout(() => { copied.value = false }, 1600)
}

onBeforeUnmount(() => clearTimeout(timer))
</script>

<style scoped>
.copy-icon-button {
  display: inline-grid;
  width: 1.75rem;
  height: 1.75rem;
  flex: none;
  place-items: center;
  border-radius: var(--operator-radius-sm);
  color: var(--operator-subtle-foreground);
  transition:
    background-color var(--operator-duration-fast) var(--operator-ease-out),
    color var(--operator-duration-fast) var(--operator-ease-out),
    transform 120ms var(--operator-ease-out);
}

.copy-icon-button:hover {
  background: var(--operator-muted);
  color: var(--operator-foreground);
}

.copy-icon-button:active {
  transform: scale(0.9);
}

.copy-icon-button:focus-visible {
  outline: 2px solid rgb(var(--operator-accent-rgb) / 0.7);
  outline-offset: 1px;
}

.copy-icon-button.is-copied {
  background: rgb(var(--operator-success-rgb) / 0.14);
  color: var(--operator-success-fill);
}

.copy-icon-button.is-copied :deep(svg) {
  animation: ui-pop 320ms var(--operator-ease-out);
}

.copy-swap-enter-active,
.copy-swap-leave-active {
  transition: opacity 120ms var(--operator-ease-out), transform 120ms var(--operator-ease-out);
}

.copy-swap-enter-from,
.copy-swap-leave-to {
  opacity: 0;
  transform: scale(0.6) rotate(-20deg);
}

@media (prefers-reduced-motion: reduce) {
  .copy-icon-button.is-copied :deep(svg) {
    animation: none;
  }

  .copy-swap-enter-active,
  .copy-swap-leave-active {
    transition: none;
  }
}
</style>
