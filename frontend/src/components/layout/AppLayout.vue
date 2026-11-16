<template>
  <div class="app-shell" :class="{ 'app-shell-collapsed': sidebarCollapsed, 'has-tabbar': hasTabbar }">
    <a href="#main-content" class="app-skip-link">{{ t('nav.skipToContent') }}</a>

    <AppSidebar />

    <div class="app-column">
      <div class="app-frame">
        <AppHeader :scrolled="scrolled" />

        <main id="main-content" ref="mainRef" class="app-main" tabindex="-1" @scroll.passive="onScroll">
          <div class="app-content">
            <OperatorContextNav />
            <slot />
          </div>
        </main>
      </div>
    </div>

    <MobileTabBar />
    <CommandPalette />
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMediaQuery } from '@vueuse/core'
import { useAppStore, useAuthStore } from '@/stores'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import OperatorContextNav from './OperatorContextNav.vue'
import MobileTabBar from './MobileTabBar.vue'
import CommandPalette from './CommandPalette.vue'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const isDesktop = useMediaQuery('(min-width: 1024px)')
const sidebarCollapsed = computed(() => isDesktop.value && appStore.sidebarCollapsed)
const hasTabbar = computed(() => authStore.isAdmin || !appStore.backendModeEnabled)

// The header shows a hairline only once the page scrolls under it.
const mainRef = ref<HTMLElement | null>(null)
const scrolled = ref(false)
function onScroll() {
  scrolled.value = (mainRef.value?.scrollTop ?? 0) > 4
}
</script>
