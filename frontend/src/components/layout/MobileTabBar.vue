<template>
  <nav v-if="tabs.length" class="app-tabbar lg:hidden" :aria-label="t('nav.quickNavigation')" data-testid="mobile-tabbar">
    <router-link
      v-for="tab in tabs"
      :key="tab.path"
      :to="tab.path"
      :aria-current="isActive(tab) ? 'page' : undefined"
    >
      <Icon :name="tab.icon" size="md" />
      <span>{{ tab.label }}</span>
    </router-link>
    <button type="button" :aria-label="t('nav.search')" @click="palette.show()">
      <Icon name="search" size="md" />
      <span>{{ t('nav.search') }}</span>
    </button>
    <button
      type="button"
      :aria-expanded="appStore.mobileOpen ? 'true' : 'false'"
      @click="appStore.toggleMobileSidebar()"
    >
      <Icon name="menu" size="md" />
      <span>{{ t('nav.more') }}</span>
    </button>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import type { IconName } from '@/composables/useAppNavigation'
import { useCommandPalette } from '@/composables/useCommandPalette'
import { getOperatorArea, type OperatorAreaId } from '@/router/operatorNavigation'

interface Tab {
  path: string
  label: string
  icon: IconName
  areaId?: OperatorAreaId
}

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const palette = useCommandPalette()

// Thumb-reach shortcuts for the pages used most; everything else lives under More.
const tabs = computed<Tab[]>(() => {
  if (authStore.isAdmin) {
    return [
      { path: '/admin/dashboard', label: t('nav.overview'), icon: 'grid', areaId: 'overview' },
      { path: '/admin/stats', label: t('nav.stats'), icon: 'chartBar', areaId: 'stats' },
      { path: '/admin/accounts', label: t('nav.accounts'), icon: 'server', areaId: 'accounts' },
    ]
  }
  if (appStore.backendModeEnabled) return []
  return [
    { path: '/dashboard', label: t('nav.dashboard'), icon: 'grid' },
    { path: '/keys', label: t('nav.apiKeys'), icon: 'key' },
    { path: '/usage', label: t('nav.usage'), icon: 'chart' },
  ]
})

function isActive(tab: Tab): boolean {
  if (tab.areaId) return getOperatorArea(route.path)?.id === tab.areaId
  return route.path === tab.path || route.path.startsWith(`${tab.path}/`)
}
</script>
