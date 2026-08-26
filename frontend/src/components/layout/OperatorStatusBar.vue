<template>
  <footer class="operator-status-bar">
    <div class="operator-status-inner">
      <span v-if="isFixtureReview" class="operator-review-status">
        <span class="operator-status-dot bg-blue-500" />
        Local fixture review · no production connection
      </span>
      <span v-else class="operator-status-item">
        <span class="operator-status-dot bg-emerald-500" />
        {{ authStore.isAuthenticated ? 'Operator authenticated' : 'Session unavailable' }}
      </span>
      <span v-if="area" class="operator-status-item">
        <Icon name="swap" size="xs" />
        <strong>Area</strong> {{ t(area.labelKey) }}
      </span>
      <span v-if="appStore.siteVersion" class="operator-status-item operator-status-secondary">
        <strong>Version</strong> {{ appStore.siteVersion }}
      </span>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore, useAuthStore } from '@/stores'
import { getOperatorArea } from '@/router/operatorNavigation'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const area = computed(() => getOperatorArea(route.path))
const isFixtureReview = window.__OPERATOR_REVIEW_MODE__ === true
</script>
