<template>
  <section class="viz-card" aria-labelledby="pd-recent">
    <header class="viz-card-head">
      <h2 id="pd-recent" class="viz-card-title">{{ t('dashboard.recentUsage') }}</h2>
      <RouterLink to="/usage" class="viz-link viz-card-tools">
        {{ t('dashboard.viewAllUsage') }}
        <Icon name="arrowRight" size="xs" />
      </RouterLink>
    </header>
    <div class="viz-card-body is-flush">
      <div v-if="loading" class="flex flex-col gap-2 px-[1.125rem] pb-4">
        <div v-for="i in 5" :key="i" class="skeleton h-11 rounded-lg"></div>
      </div>
      <div v-else-if="data.length === 0" class="ui-empty">
        <span class="ui-empty-icon"><Icon name="inbox" size="md" /></span>
        <b>{{ t('dashboard.noUsageRecords') }}</b>
        <RouterLink to="/keys" class="btn btn-secondary btn-sm">{{ t('personalUi.keys.newKey') }}</RouterLink>
      </div>
      <ul v-else class="pd-recent ui-rows-enter">
        <li v-for="log in data" :key="log.id" class="pd-recent-row">
          <ProviderMark v-if="providerOfModel(log.model)" :platform="providerOfModel(log.model)" :size="28" />
          <span v-else class="pd-recent-fallback" aria-hidden="true"><Icon name="cube" size="xs" /></span>
          <span class="min-w-0 flex-1">
            <span class="ui-cell-main font-mono text-[13px]">{{ log.model }}</span>
            <span class="ui-cell-sub" :title="formatDateTime(log.created_at)">{{ formatRelativeTime(log.created_at) }}</span>
          </span>
          <span class="pd-recent-figures">
            <span class="text-xs text-gray-500 tabular-nums">{{ formatCompact(log.input_tokens + log.output_tokens) }} tok</span>
            <b class="tabular-nums" :title="`${t('dashboard.actual')} / ${t('dashboard.standard')}: $${formatCost(log.actual_cost)} / $${formatCost(log.total_cost)}`">${{ formatCost(log.actual_cost) }}</b>
          </span>
        </li>
      </ul>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import { formatDateTime, formatRelativeTime } from '@/utils/format'
import { providerOfModel } from '@/utils/providerSeries'
import { formatCompact } from '@/utils/vizFormat'
import type { UsageLog } from '@/types'

defineProps<{
  data: UsageLog[]
  loading: boolean
}>()
const { t } = useI18n()
const formatCost = (c: number) => c.toFixed(4)
</script>

<style scoped>
.pd-recent-row {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.75rem;
  padding: 0.625rem 1.125rem;
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.pd-recent-row + .pd-recent-row {
  border-top: 1px solid var(--operator-border-subtle);
}

.pd-recent-row:hover {
  background: var(--table-row-hover-background);
}

.pd-recent-row:last-child {
  border-bottom-left-radius: var(--operator-radius-lg);
  border-bottom-right-radius: var(--operator-radius-lg);
}

.pd-recent-fallback {
  display: inline-grid;
  width: 28px;
  height: 28px;
  flex: none;
  place-items: center;
  border-radius: 8px;
  background: var(--operator-raised);
  color: var(--operator-subtle-foreground);
}

.pd-recent-figures {
  display: flex;
  flex: none;
  flex-direction: column;
  align-items: flex-end;
  font-size: 0.875rem;
}
</style>
