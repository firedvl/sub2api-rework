<template>
  <Teleport to="body">
    <div class="toast-region" aria-live="polite" aria-atomic="true">
      <TransitionGroup name="toast">
        <div
          v-for="toast in toasts"
          :key="toast.id"
          :data-tone="toast.type"
          class="operator-toast toast-item"
        >
          <div class="operator-toast-icon toast-icon" aria-hidden="true">
            <Icon :name="getToastIconName(toast.type)" size="md" />
          </div>

          <div class="min-w-0 flex-1">
            <p v-if="toast.title" class="toast-title">{{ toast.title }}</p>
            <p :class="toast.title ? 'toast-message' : 'toast-title font-normal'">{{ toast.message }}</p>
          </div>

          <button
            type="button"
            class="operator-toast-close toast-close"
            :aria-label="t('common.closeNotification')"
            @click="removeToast(toast.id)"
          >
            <Icon name="x" size="sm" />
          </button>

          <div v-if="toast.duration" class="operator-toast-track toast-track" aria-hidden="true">
            <div class="toast-progress" :style="{ animationDuration: `${toast.duration}ms` }"></div>
          </div>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

const toasts = computed(() => appStore.toasts)

const getToastIconName = (type: string): 'checkCircle' | 'xCircle' | 'exclamationTriangle' | 'infoCircle' => {
  switch (type) {
    case 'success':
      return 'checkCircle'
    case 'error':
      return 'xCircle'
    case 'warning':
      return 'exclamationTriangle'
    case 'info':
    default:
      return 'infoCircle'
  }
}

const removeToast = (id: string) => {
  appStore.hideToast(id)
}
</script>

<style scoped>
.toast-region {
  position: fixed;
  top: 1rem;
  right: 1rem;
  z-index: 9999;
  display: flex;
  width: min(24rem, calc(100vw - 2rem));
  flex-direction: column;
  gap: 0.5rem;
  pointer-events: none;
}

.toast-item {
  --toast-tone: var(--operator-accent);
  position: relative;
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  overflow: hidden;
  padding: 0.75rem 0.625rem 0.875rem 0.875rem;
  border-radius: var(--operator-radius-lg);
  pointer-events: auto;
}

.toast-item[data-tone='success'] {
  --toast-tone: var(--operator-success);
}

.toast-item[data-tone='error'] {
  --toast-tone: var(--operator-destructive);
}

.toast-item[data-tone='warning'] {
  --toast-tone: var(--operator-warning);
}

.toast-icon {
  display: flex;
  flex-shrink: 0;
  padding-top: 0.0625rem;
  color: var(--toast-tone);
}

.toast-title {
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 600;
  line-height: 1.375rem;
  overflow-wrap: anywhere;
}

.toast-message {
  margin-top: 0.125rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  line-height: 1.25rem;
  overflow-wrap: anywhere;
}

.toast-close {
  display: inline-flex;
  width: 1.75rem;
  height: 1.75rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  margin: -0.125rem 0 0;
  border-radius: var(--operator-radius-sm);
  color: var(--operator-subtle-foreground);
  transition:
    background-color var(--operator-duration-fast) var(--operator-ease-out),
    color var(--operator-duration-fast) var(--operator-ease-out);
}

.toast-close:hover {
  background: var(--operator-muted);
  color: var(--operator-foreground);
}

.toast-track {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 2px;
  background: color-mix(in srgb, var(--toast-tone) 14%, transparent);
}

/* A countdown runs at a constant rate, so linear timing is the honest curve. */
.toast-progress {
  height: 100%;
  background: color-mix(in srgb, var(--toast-tone) 70%, transparent);
  transform-origin: left center;
  animation-name: toast-countdown;
  animation-timing-function: linear;
  animation-fill-mode: forwards;
}

@keyframes toast-countdown {
  from {
    transform: scaleX(1);
  }

  to {
    transform: scaleX(0);
  }
}

.toast-enter-active {
  transition:
    opacity var(--operator-duration) var(--operator-ease-out),
    transform var(--operator-duration-slow) var(--operator-ease-out);
}

.toast-leave-active {
  transition:
    opacity var(--operator-duration-fast) var(--operator-ease-in),
    transform var(--operator-duration-fast) var(--operator-ease-in);
}

.toast-enter-from {
  opacity: 0;
  transform: translateY(-8px) scale(0.98);
}

.toast-leave-to {
  opacity: 0;
  transform: translateX(12px);
}
</style>
