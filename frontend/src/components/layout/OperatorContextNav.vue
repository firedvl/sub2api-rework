<template>
  <nav
    v-if="area && visibleSections.length > 1"
    ref="navRef"
    class="operator-context-nav"
    :aria-label="t('nav.areaSections', { area: t(area.labelKey) })"
  >
    <span
      v-if="pill.width"
      class="operator-context-pill"
      aria-hidden="true"
      :style="{ width: `${pill.width}px`, transform: `translateX(${pill.left}px)` }"
    ></span>
    <div class="flex min-w-max gap-6">
      <router-link
        v-for="section in visibleSections"
        :key="section.path"
        :to="section.path"
        class="operator-context-link"
        :class="{ 'operator-context-link-active': isCurrent(section) }"
        :aria-current="isCurrent(section) ? 'page' : undefined"
      >
        {{ t(section.labelKey) }}
      </router-link>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useResizeObserver } from '@vueuse/core'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { useAuthStore } from '@/stores/auth'
import { usePageMeta } from '@/composables/usePageMeta'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { matchesOperatorPath, type OperatorSection } from '@/router/operatorNavigation'

const { t } = useI18n()
const route = useRoute()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()
const { area } = usePageMeta()

function isVisible(section: OperatorSection): boolean {
  if (authStore.isSimpleMode && section.hideInSimpleMode) return false
  if (section.gate === 'ops-monitoring') return adminSettingsStore.opsMonitoringEnabled
  if (section.gate === 'channel-monitor') {
    return isFeatureFlagEnabled(FeatureFlags.channelMonitor)
  }
  if (section.gate === 'risk-control') return isFeatureFlagEnabled(FeatureFlags.riskControl)
  return true
}

function isCurrent(section: OperatorSection): boolean {
  return matchesOperatorPath(route.path, section.path)
}

const visibleSections = computed(() => area.value?.sections.filter(isVisible) ?? [])

// The active tab's surface slides between tabs.
const navRef = ref<HTMLElement | null>(null)
const pill = reactive({ left: 0, width: 0 })

function measure() {
  const nav = navRef.value
  const active = nav?.querySelector<HTMLElement>('.operator-context-link-active')
  if (!nav || !active) {
    pill.width = 0
    return
  }
  pill.left = active.offsetLeft
  pill.width = active.offsetWidth
}

watch(() => [route.path, visibleSections.value.length], () => void nextTick(measure))
onMounted(() => void nextTick(measure))
useResizeObserver(navRef, measure)
</script>
