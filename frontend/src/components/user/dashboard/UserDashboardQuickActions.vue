<template>
  <nav class="pd-actions" :aria-label="t('dashboard.quickActions')">
    <RouterLink to="/keys" class="ui-chip">
      <Icon name="key" size="sm" />{{ t('personalUi.keys.newKey') }}
    </RouterLink>
    <RouterLink to="/usage" class="ui-chip" :title="t('dashboard.checkDetailedLogs')">
      <Icon name="chart" size="sm" />{{ t('personalUi.dashboard.usage') }}
    </RouterLink>
    <RouterLink to="/redeem" class="ui-chip" :title="t('dashboard.addBalanceWithCode')">
      <Icon name="gift" size="sm" />{{ t('personalUi.dashboard.redeem') }}
    </RouterLink>
    <RouterLink v-if="canUseBatchImage" to="/batch-image" class="ui-chip" :title="t('dashboard.batchImageAgentDesc')">
      <Icon name="sparkles" size="sm" />{{ t('dashboard.batchImageAgent') }}
    </RouterLink>
  </nav>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'

const { t } = useI18n()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

onMounted(() => {
  void refreshBatchImageAccess()
})
</script>

<style scoped>
.pd-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}
</style>
