<template>
  <section aria-labelledby="prompt-runtime-title" class="flex flex-col gap-4">
    <div class="ui-toolbar">
      <h2 id="prompt-runtime-title" class="flex items-center text-sm font-semibold">
        {{ t('admin.promptAudit.runtime.title') }}
        <HelpTooltip :content="t('admin.promptAudit.runtime.description')" />
      </h2>
      <span class="ui-grow" />
      <button
        type="button"
        class="app-icon-button"
        :disabled="loading"
        :aria-label="t('admin.promptAudit.actions.refresh')"
        :title="t('admin.promptAudit.actions.refresh')"
        @click="$emit('refresh')"
      >
        <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
      </button>
    </div>

    <div v-if="error" role="alert" class="pa-runtime-alert">{{ error }}</div>
    <div v-else-if="loading && !runtime" class="viz-kpis" aria-busy="true">
      <div v-for="index in 4" :key="index" class="skeleton h-20" />
    </div>
    <template v-else-if="runtime">
      <dl class="viz-kpis !mb-0">
        <div class="viz-card viz-kpi">
          <dt class="viz-kpi-label">{{ t('admin.promptAudit.runtime.process') }}</dt>
          <dd class="viz-kpi-row">
            <span class="viz-kpi-value pa-kpi-status">
              <span :class="statusDot(runtime.process_status)" />
              <span class="truncate">{{ t(`admin.promptAudit.status.${runtime.process_status}`) }}</span>
            </span>
          </dd>
          <span class="ui-cell-sub truncate">{{ t(`admin.promptAudit.mode.${runtime.effective_mode}`) }}</span>
        </div>
        <div class="viz-card viz-kpi">
          <dt class="viz-kpi-label">{{ t('admin.adminUi.promptAudit.workers') }}</dt>
          <dd class="viz-kpi-row">
            <span class="viz-kpi-value"><CountUp :value="runtime.worker_active" /><small class="pa-kpi-of"> / {{ runtime.worker_total }}</small></span>
          </dd>
          <span class="ui-cell-sub">{{ t('admin.promptAudit.runtime.workers') }}</span>
        </div>
        <div class="viz-card viz-kpi">
          <dt class="viz-kpi-label">{{ t('admin.adminUi.promptAudit.queue') }}</dt>
          <dd class="viz-kpi-row">
            <span class="viz-kpi-value"><CountUp :value="runtime.queue.active" /><small class="pa-kpi-of"> / {{ runtime.queue_capacity }}</small></span>
          </dd>
          <div class="pa-meter" aria-hidden="true"><i :style="{ width: `${queueRatio}%` }" :class="queueRatio > 80 ? 'is-bad' : queueRatio > 50 ? 'is-warn' : ''" /></div>
        </div>
        <div class="viz-card viz-kpi">
          <dt class="viz-kpi-label">{{ t('admin.adminUi.promptAudit.version') }}</dt>
          <dd class="viz-kpi-row">
            <span class="viz-kpi-value pa-kpi-status">
              <span :class="runtime.active_config_version === runtime.expected_config_version ? 'ui-dot is-ok' : 'ui-dot is-warn'" />
              v{{ runtime.active_config_version }}<small class="pa-kpi-of"> / v{{ runtime.expected_config_version }}</small>
            </span>
          </dd>
          <span class="ui-cell-sub">{{ t('admin.promptAudit.runtime.version') }}</span>
        </div>
      </dl>

      <div class="viz-grid">
        <section class="viz-card viz-span-8">
          <header class="viz-card-head">
            <h3 class="viz-card-title">{{ t('admin.promptAudit.runtime.guardMetrics') }}</h3>
          </header>
          <div class="viz-card-body">
            <dl class="pa-metrics">
              <div v-for="metric in guardMetricItems" :key="metric.label">
                <dt>{{ metric.label }}</dt>
                <dd>{{ metric.value }}</dd>
              </div>
            </dl>
          </div>
          <footer class="viz-card-foot pa-runtime-foot">
            <span>{{ t('admin.promptAudit.runtime.queueBreakdown', {
              queued: runtime.queue.queued,
              processing: runtime.queue.processing,
              retry: runtime.queue.retry,
              done: runtime.queue.done,
              failed: runtime.queue.failed,
            }) }}</span>
            <span>{{ t('admin.promptAudit.runtime.deliveryTotals', { enqueued: runtime.enqueued_total, dropped: runtime.dropped_total, processed: runtime.processed_total, failed: runtime.failed_total }) }}</span>
          </footer>
        </section>
        <section class="viz-card viz-span-4">
          <header class="viz-card-head">
            <h3 class="viz-card-title">{{ t('admin.promptAudit.runtime.latest') }}</h3>
          </header>
          <div class="viz-card-body space-y-3 text-sm">
            <p class="flex items-center gap-2">
              <Icon name="clock" size="sm" class="text-gray-400" />
              {{ runtime.last_processed_at ? formatDate(runtime.last_processed_at) : t('admin.promptAudit.common.never') }}
            </p>
            <p v-if="runtime.last_error_code" class="break-words text-red-600 dark:text-red-300">
              {{ runtime.last_error_code }}<span v-if="runtime.last_error_message"> · {{ runtime.last_error_message }}</span>
            </p>
            <div>
              <p class="ui-section-label mb-1.5">{{ t('admin.promptAudit.runtime.dependencies') }}</p>
              <div class="flex flex-wrap gap-3 text-xs">
                <span class="pa-dep"><span :class="dependencyDot(runtime.database_status)" />DB {{ runtime.database_status }}</span>
                <span class="pa-dep"><span :class="dependencyDot(runtime.redis_status)" />Redis {{ runtime.redis_status }}</span>
              </div>
            </div>
            <div v-if="Object.keys(runtime.endpoints).length" class="flex flex-wrap gap-1.5">
              <span v-for="(probe, id) in runtime.endpoints" :key="id" class="badge" :class="probe.ok ? 'badge-success' : 'badge-danger'">
                {{ id }} · {{ probe.status }} · {{ probe.latency_ms }} ms
              </span>
            </div>
          </div>
        </section>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import CountUp from '@/components/ui/CountUp.vue'
import type { PromptAuditRuntime } from '../types'

const props = defineProps<{ runtime: PromptAuditRuntime | null; loading: boolean; error: string }>()
defineEmits<{ (event: 'refresh'): void }>()
const { t, locale } = useI18n()

const queueRatio = computed(() => {
  const runtime = props.runtime
  if (!runtime || !runtime.queue_capacity) return 0
  return Math.min(100, Math.round((runtime.queue.active / runtime.queue_capacity) * 100))
})

const guardMetricItems = computed(() => {
  const metrics = props.runtime?.guard_metrics
  if (!metrics) return []
  return [
    { label: t('admin.promptAudit.metrics.total'), value: metrics.total },
    { label: t('admin.promptAudit.metrics.allowed'), value: metrics.allowed },
    { label: t('admin.promptAudit.metrics.flagged'), value: metrics.flagged },
    { label: t('admin.promptAudit.metrics.blocked'), value: metrics.blocked },
    { label: t('admin.promptAudit.metrics.unavailable'), value: metrics.unavailable },
    { label: t('admin.promptAudit.metrics.timeouts'), value: metrics.timeouts },
    { label: t('admin.promptAudit.metrics.failovers'), value: metrics.failovers },
    { label: 'P95', value: metrics.latency_p95_ms != null ? `${metrics.latency_p95_ms} ms` : '—' },
  ]
})

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value))
}

function statusDot(status: string): string {
  if (status === 'running') return 'ui-dot is-ok'
  if (status === 'disabled') return 'ui-dot'
  if (status === 'degraded') return 'ui-dot is-warn'
  return 'ui-dot is-bad'
}

function dependencyDot(status: string): string {
  return status === 'ok' || status === 'healthy' ? 'ui-dot is-ok' : 'ui-dot is-bad'
}
</script>

<style scoped>
.pa-runtime-alert {
  padding: 0.625rem 0.875rem;
  border-radius: var(--operator-radius);
  background: color-mix(in srgb, var(--operator-destructive) 10%, transparent);
  color: var(--operator-destructive);
  font-size: 0.875rem;
}

.pa-kpi-status {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
}

.pa-kpi-of {
  color: var(--operator-subtle-foreground);
  font-size: 0.8125rem;
  font-weight: 500;
}

.pa-meter {
  height: 4px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--operator-raised);
}

.pa-meter > i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--operator-success);
  transition: width var(--operator-duration) var(--operator-ease-out);
}

.pa-meter > i.is-warn {
  background: var(--operator-warning);
}

.pa-meter > i.is-bad {
  background: var(--operator-destructive);
}

.pa-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.875rem 1rem;
}

.pa-metrics dt {
  color: var(--operator-subtle-foreground);
  font-size: 0.6875rem;
}

.pa-metrics dd {
  font-size: 1.0625rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.pa-runtime-foot {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem 1rem;
  font-size: 0.75rem;
}

.pa-dep {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
}

@media (max-width: 640px) {
  .pa-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
