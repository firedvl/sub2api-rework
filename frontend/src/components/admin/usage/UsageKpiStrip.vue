<template>
  <dl class="viz-kpis mb-0" data-testid="usage-kpis">
    <div class="viz-card viz-kpi">
      <dt class="viz-kpi-label"><Icon name="document" size="sm" />{{ t('usage.totalRequests') }}</dt>
      <dd class="viz-kpi-row">
        <CountUp class="viz-kpi-value" :value="stats ? stats.total_requests ?? 0 : null" />
        <SparkLine v-if="requestSpark.length > 1" :values="requestSpark" />
      </dd>
    </div>

    <div class="viz-card viz-kpi">
      <dt class="viz-kpi-label"><Icon name="cube" size="sm" />{{ t('usage.totalTokens') }}</dt>
      <dd class="viz-kpi-row">
        <CountUp class="viz-kpi-value" :value="stats ? stats.total_tokens ?? 0 : null" :format="formatTokens" />
        <SparkLine v-if="tokenSpark.length > 1" :values="tokenSpark" />
      </dd>
      <dd class="usage-kpi-foot" :title="cacheTitle">
        <span><Icon name="arrowDown" size="xs" />{{ formatTokens(stats?.total_input_tokens || 0) }}</span>
        <span><Icon name="arrowUp" size="xs" />{{ formatTokens(stats?.total_output_tokens || 0) }}</span>
        <span><Icon name="database" size="xs" />{{ formatTokens(stats?.total_cache_tokens || 0) }}</span>
      </dd>
    </div>

    <div class="viz-card viz-kpi">
      <dt class="viz-kpi-label"><Icon name="dollar" size="sm" />{{ t('usage.totalCost') }}</dt>
      <dd class="usage-kpi-help" tabindex="0" :aria-label="t('admin.activityUi.costNote')" :aria-describedby="noteId">
        <Icon name="questionCircle" size="sm" />
        <span :id="noteId" role="tooltip" class="usage-kpi-note">{{ t('usage.observationCostDisclaimer') }}</span>
      </dd>
      <dd class="viz-kpi-row">
        <CountUp class="viz-kpi-value" :value="stats ? stats.total_actual_cost ?? 0 : null" :format="formatMoney" />
        <SparkLine v-if="costSpark.length > 1" :values="costSpark" />
      </dd>
      <dd class="usage-kpi-foot">
        <span :title="t('usage.standardCost')">{{ t('usage.standardCost') }} {{ formatMoney(stats?.total_cost || 0) }}</span>
        <span v-if="totalAccountCost != null" :title="t('usage.accountBilled')">{{ t('usage.accountCost') }} {{ formatMoney(totalAccountCost) }}</span>
      </dd>
    </div>

    <div class="viz-card viz-kpi">
      <dt class="viz-kpi-label"><Icon name="clock" size="sm" />{{ t('usage.avgDuration') }}</dt>
      <dd class="viz-kpi-row">
        <CountUp class="viz-kpi-value" :value="stats ? stats.average_duration_ms ?? 0 : null" :format="formatDuration" />
      </dd>
    </div>
  </dl>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'
import type { TrendDataPoint } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import CountUp from '@/components/ui/CountUp.vue'
import SparkLine from '@/components/viz/SparkLine.vue'

const props = defineProps<{
  stats: AdminUsageStatsResponse | null
  /** Trend buckets already loaded for the charts; drives the sparklines. */
  trend?: TrendDataPoint[]
}>()

const { t } = useI18n()
const noteId = `usage-cost-note-${Math.random().toString(36).slice(2, 8)}`

const totalAccountCost = computed(() => {
  const stats = props.stats as (AdminUsageStatsResponse & { total_account_cost?: number }) | null
  return stats?.total_account_cost ?? null
})

const requestSpark = computed(() => (props.trend ?? []).map((point) => point.requests))
const tokenSpark = computed(() => (props.trend ?? []).map((point) => point.total_tokens))
const costSpark = computed(() => (props.trend ?? []).map((point) => point.actual_cost))

const cacheTitle = computed(() => [
  `${t('usage.in')}: ${(props.stats?.total_input_tokens || 0).toLocaleString()}`,
  `${t('usage.out')}: ${(props.stats?.total_output_tokens || 0).toLocaleString()}`,
  `${t('usage.cacheCreationTokensLabel')}: ${(props.stats?.total_cache_creation_tokens || 0).toLocaleString()}`,
  `${t('usage.cacheReadTokensLabel')}: ${(props.stats?.total_cache_read_tokens || 0).toLocaleString()}`,
].join('\n'))

function formatTokens(value: number): string {
  if (value >= 1e9) return (value / 1e9).toFixed(2) + 'B'
  if (value >= 1e6) return (value / 1e6).toFixed(2) + 'M'
  if (value >= 1e3) return (value / 1e3).toFixed(1) + 'K'
  return Math.round(value).toLocaleString()
}

function formatMoney(value: number): string {
  return '$' + value.toFixed(value >= 100 ? 2 : 4)
}

function formatDuration(ms: number): string {
  return ms < 1000 ? `${ms.toFixed(0)}ms` : `${(ms / 1000).toFixed(2)}s`
}
</script>

<style scoped>
.usage-kpi-foot {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem 0.75rem;
  overflow: hidden;
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.usage-kpi-foot > span {
  display: inline-flex;
  align-items: center;
  gap: 0.1875rem;
}
.usage-kpi-help {
  position: absolute;
  top: 0.75rem;
  right: 0.75rem;
  display: inline-flex;
  border-radius: 999px;
  color: var(--operator-subtle-foreground);
  cursor: help;
}
.usage-kpi-help:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: 2px;
}
.usage-kpi-note {
  position: absolute;
  top: calc(100% + 0.5rem);
  right: -0.25rem;
  z-index: 30;
  width: 16rem;
  padding: 0.625rem 0.75rem;
  border: 1px solid var(--operator-popup-border);
  border-radius: var(--operator-radius);
  background: var(--operator-popup-surface);
  box-shadow: var(--operator-popup-shadow);
  color: var(--operator-popup-foreground);
  font-size: 0.75rem;
  line-height: 1.45;
  white-space: normal;
  opacity: 0;
  pointer-events: none;
  transform: translateY(-4px);
  transition: opacity var(--operator-duration-fast) var(--operator-ease-out), transform var(--operator-duration-fast) var(--operator-ease-out);
}
.usage-kpi-help:hover .usage-kpi-note,
.usage-kpi-help:focus-visible .usage-kpi-note {
  opacity: 1;
  transform: translateY(0);
}
/* The tile clips overflow; the note needs to escape it. */
.viz-kpi:has(.usage-kpi-help:hover, .usage-kpi-help:focus-visible) {
  overflow: visible;
  z-index: 5;
}
@media (max-width: 640px) {
  .viz-kpi :deep(.spark-line) {
    display: none;
  }
  .usage-kpi-note {
    width: min(16rem, 80vw);
  }
}
@media (prefers-reduced-motion: reduce) {
  .usage-kpi-note {
    transition: none;
  }
}
</style>
