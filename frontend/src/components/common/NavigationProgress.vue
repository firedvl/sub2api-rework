<script setup lang="ts">
/**
 * 导航进度条组件
 * 在页面顶部显示加载进度，提供导航反馈
 */
import { computed } from 'vue'
import { useNavigationLoadingState } from '@/composables/useNavigationLoading'

const { isLoading } = useNavigationLoadingState()

// 进度条可见性
const isVisible = computed(() => isLoading.value)
</script>

<template>
  <Transition name="progress-fade">
    <div
      v-show="isVisible"
      class="navigation-progress"
      role="progressbar"
      aria-label="Loading"
      aria-valuenow="0"
      aria-valuemin="0"
      aria-valuemax="100"
    >
      <div class="navigation-progress-bar" />
    </div>
  </Transition>
</template>

<style scoped>
.navigation-progress {
  position: fixed;
  top: 0;
  right: 0;
  left: 0;
  z-index: 9999;
  height: 2px;
  overflow: hidden;
  background: color-mix(in srgb, var(--operator-accent) 14%, transparent);
}

/* Indeterminate: a short accent segment travels across the top edge. */
.navigation-progress-bar {
  width: 35%;
  height: 100%;
  border-radius: 0 2px 2px 0;
  background: var(--operator-accent);
  animation: progress-slide 1.2s cubic-bezier(0.4, 0, 0.2, 1) infinite;
}

@keyframes progress-slide {
  from {
    transform: translateX(-100%);
  }

  to {
    transform: translateX(300%);
  }
}

.progress-fade-enter-active {
  transition: opacity var(--operator-duration-fast) var(--operator-ease-out);
}

.progress-fade-leave-active {
  transition: opacity var(--operator-duration-slow) var(--operator-ease-out);
}

.progress-fade-enter-from,
.progress-fade-leave-to {
  opacity: 0;
}

@media (prefers-reduced-motion: reduce) {
  .navigation-progress-bar {
    width: 100%;
    animation: progress-pulse 2s ease-in-out infinite !important;
  }

  @keyframes progress-pulse {
    0%,
    100% {
      opacity: 0.35;
    }

    50% {
      opacity: 1;
    }
  }
}
</style>
