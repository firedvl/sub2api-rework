<template>
  <nav
    v-if="area && visibleSections.length > 1"
    class="operator-context-nav mb-4 overflow-x-auto border-b border-gray-200 dark:border-dark-700"
    :aria-label="t('nav.areaSections', { area: t(area.labelKey) })"
  >
    <div class="flex min-w-max gap-5">
      <router-link
        v-for="section in visibleSections"
        :key="section.path"
        :to="section.path"
        class="operator-context-link"
        :class="{ 'operator-context-link-active': matchesOperatorPath(route.path, section.path) }"
      >
        {{ t(section.labelKey) }}
      </router-link>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { useAuthStore } from '@/stores/auth'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import {
  getOperatorArea,
  matchesOperatorPath,
  type OperatorSection,
} from '@/router/operatorNavigation'

const { t } = useI18n()
const route = useRoute()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()

const area = computed(() => {
  const currentArea = getOperatorArea(route.path)
  return authStore.isSimpleMode && currentArea?.hideInSimpleMode ? undefined : currentArea
})

function isVisible(section: OperatorSection): boolean {
  if (authStore.isSimpleMode && section.hideInSimpleMode) return false
  if (section.gate === 'ops-monitoring') return adminSettingsStore.opsMonitoringEnabled
  if (section.gate === 'channel-monitor') {
    return isFeatureFlagEnabled(FeatureFlags.channelMonitor)
  }
  if (section.gate === 'risk-control') return isFeatureFlagEnabled(FeatureFlags.riskControl)
  return true
}

const visibleSections = computed(() => area.value?.sections.filter(isVisible) ?? [])
</script>
