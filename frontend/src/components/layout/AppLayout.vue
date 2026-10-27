<template>
  <div class="app-shell" :class="{ 'app-shell-collapsed': sidebarCollapsed }">
    <a href="#main-content" class="app-skip-link">{{ t('nav.skipToContent') }}</a>

    <AppSidebar />

    <div class="app-column">
      <AppHeader />

      <main id="main-content" class="app-main" tabindex="-1">
        <div class="app-content">
          <header class="app-page-heading">
            <div class="min-w-0">
              <h1>{{ title }}</h1>
              <p v-if="description">{{ description }}</p>
            </div>
          </header>
          <OperatorContextNav />
          <slot />
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMediaQuery } from '@vueuse/core'
import { useAppStore } from '@/stores'
import { usePageMeta } from '@/composables/usePageMeta'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import OperatorContextNav from './OperatorContextNav.vue'

const { t } = useI18n()
const appStore = useAppStore()
const { title, description } = usePageMeta()
const isDesktop = useMediaQuery('(min-width: 1024px)')
const sidebarCollapsed = computed(() => isDesktop.value && appStore.sidebarCollapsed)
</script>
