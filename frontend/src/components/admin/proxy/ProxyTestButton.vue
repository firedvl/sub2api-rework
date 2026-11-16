<template>
  <button
    type="button"
    :class="[iconOnly ? 'proxy-test is-icon' : 'btn btn-secondary btn-sm proxy-test', stateClass]"
    :disabled="state === 'testing'"
    :aria-busy="state === 'testing'"
    :aria-label="iconOnly ? ariaLabel || label : undefined"
    :title="label"
    @click.stop="emit('test')"
  >
    <span class="proxy-test-glyph" aria-hidden="true">
      <svg v-if="state === 'testing'" class="proxy-test-spin" viewBox="0 0 24 24" fill="none">
        <circle cx="12" cy="12" r="9" stroke="currentColor" stroke-width="2.5" opacity="0.25" />
        <path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" />
      </svg>
      <Icon v-else-if="state === 'ok'" key="ok" name="check" size="sm" :stroke-width="2.5" class="proxy-test-pop" />
      <Icon v-else-if="state === 'fail'" key="fail" name="x" size="sm" :stroke-width="2.5" class="proxy-test-shake" />
      <Icon v-else key="idle" name="play" size="sm" />
    </span>
    <span v-if="!iconOnly">{{ label }}</span>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'

/**
 * Proxy connection-test trigger: spinner while running, then a short
 * success (check) or failure (x) pop that settles back to idle.
 */
const props = withDefaults(defineProps<{
  state: 'idle' | 'testing' | 'ok' | 'fail'
  label: string
  ariaLabel?: string
  iconOnly?: boolean
}>(), { ariaLabel: '', iconOnly: true })

const emit = defineEmits<{ test: [] }>()

const stateClass = computed(() => {
  if (props.state === 'ok') return 'is-ok'
  if (props.state === 'fail') return 'is-fail'
  if (props.state === 'testing') return 'is-testing'
  return ''
})
</script>

<style scoped>
.proxy-test {
  position: relative;
  isolation: isolate;
  transition:
    color var(--operator-duration-fast) var(--operator-ease-out),
    background-color var(--operator-duration-fast) var(--operator-ease-out),
    border-color var(--operator-duration-fast) var(--operator-ease-out);
}

.proxy-test.is-icon {
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: var(--operator-radius);
  color: var(--operator-muted-foreground);
}

.proxy-test.is-icon:hover:not(:disabled) {
  background: var(--operator-muted);
  color: var(--operator-foreground);
}

.proxy-test:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: 2px;
}

.proxy-test:disabled {
  cursor: progress;
}

.proxy-test-glyph {
  display: inline-grid;
  width: 1rem;
  height: 1rem;
  place-items: center;
}

.proxy-test-spin {
  width: 1rem;
  height: 1rem;
  animation: proxy-test-spin 700ms linear infinite;
}

.proxy-test.is-testing {
  color: var(--operator-accent-text);
}

.proxy-test.is-ok {
  color: var(--operator-success);
}

.proxy-test.is-fail {
  color: var(--operator-destructive);
}

/* Soft ring that expands once behind the glyph on a result. */
.proxy-test.is-ok::after,
.proxy-test.is-fail::after {
  position: absolute;
  inset: 0;
  z-index: -1;
  border-radius: inherit;
  background: currentColor;
  content: '';
  opacity: 0;
  animation: proxy-test-ring 900ms var(--operator-ease-out);
}

.proxy-test-pop {
  animation: proxy-test-pop 520ms var(--operator-spring, cubic-bezier(0.34, 1.56, 0.64, 1)) both;
}

.proxy-test-shake {
  animation: proxy-test-shake 420ms var(--operator-ease-out) both;
}

@keyframes proxy-test-spin {
  to { transform: rotate(360deg); }
}

@keyframes proxy-test-pop {
  0% { opacity: 0; transform: scale(0.4); }
  60% { opacity: 1; transform: scale(1.25); }
  100% { opacity: 1; transform: scale(1); }
}

@keyframes proxy-test-shake {
  0% { opacity: 0; transform: scale(0.6); }
  30% { opacity: 1; transform: scale(1.1) translateX(0); }
  50% { transform: translateX(-2px); }
  70% { transform: translateX(2px); }
  100% { transform: translateX(0); }
}

@keyframes proxy-test-ring {
  0% { opacity: 0.22; transform: scale(0.6); }
  100% { opacity: 0; transform: scale(1.35); }
}

@media (prefers-reduced-motion: reduce) {
  .proxy-test-spin {
    animation-duration: 1.6s;
  }

  .proxy-test-pop,
  .proxy-test-shake,
  .proxy-test.is-ok::after,
  .proxy-test.is-fail::after {
    animation: none;
  }
}
</style>
