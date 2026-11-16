<template>
  <div v-if="mode === 'pre_block'" data-test="pre-block-runtime-cards" class="viz-grid">
    <section data-test="pre-block-sync-card" class="viz-card viz-span-5">
      <header class="viz-card-head">
        <h2 class="viz-card-title">{{ t('admin.riskControl.preBlockSyncStatus') }}</h2>
        <HelpTooltip :content="t('admin.riskControl.preBlockSyncHint')" interactive :aria-label="t('admin.riskControl.preBlockSyncStatus')" />
        <div class="viz-card-tools">
          <span class="badge badge-gray">{{ modeText }}</span>
        </div>
      </header>
      <div class="viz-card-body">
        <dl data-test="pre-block-metric-grid" class="rc-metrics">
          <div v-for="item in preBlockMetricItems" :key="item.key" class="rc-metric" :title="item.hint">
            <dt><span :class="item.dot" aria-hidden="true"></span>{{ item.label }}</dt>
            <dd>{{ item.value }}</dd>
          </div>
        </dl>
      </div>
    </section>

    <section data-test="pre-block-api-key-load-card" class="viz-card viz-span-7">
      <header class="viz-card-head">
        <h2 class="viz-card-title">{{ t('admin.riskControl.preBlockAPIKeyLoad') }}</h2>
        <HelpTooltip :content="t('admin.riskControl.preBlockAPIKeyLoadHint')" interactive :aria-label="t('admin.riskControl.preBlockAPIKeyLoad')" />
      </header>
      <div class="viz-card-body">
        <ul v-if="apiKeyLoads.length > 0" data-test="pre-block-api-key-load-list" class="rc-loads max-h-[280px] overflow-y-auto">
          <li v-for="item in apiKeyLoads" :key="item.key_hash || item.index" class="rc-load">
            <div class="rc-load-key">
              <span :class="keyStatusDotClass(item.status)" aria-hidden="true"></span>
              <span class="rc-load-index">#{{ item.index + 1 }}</span>
              <span class="truncate font-mono">{{ item.masked || '-' }}</span>
            </div>
            <div class="ui-meter rc-load-meter" :title="t('admin.riskControl.preBlockAPIKeyTotals', { total: formatNumber(item.total), success: formatNumber(item.success), errors: formatNumber(item.errors) })">
              <div class="ui-meter-track"><i :style="{ width: loadWidth(item.total) }"></i></div>
            </div>
            <dl class="rc-load-stats">
              <div>
                <dt>{{ t('admin.riskControl.preBlockKeyActiveShort') }}</dt>
                <dd>{{ formatNumber(item.active) }}</dd>
              </div>
              <div>
                <dt>{{ t('admin.riskControl.preBlockKeyTotalShort') }}</dt>
                <dd>{{ formatNumber(item.total) }}</dd>
              </div>
              <div>
                <dt>{{ t('admin.riskControl.preBlockKeyAvgShort') }}</dt>
                <dd>{{ formatNumber(item.avg_latency_ms) }} ms</dd>
              </div>
              <div>
                <dt>{{ t('admin.riskControl.preBlockKeyLastShort') }}</dt>
                <dd>{{ formatNumber(item.last_latency_ms) }} ms</dd>
              </div>
            </dl>
          </li>
        </ul>
        <div v-else class="ui-empty">
          <span class="ui-empty-icon"><Icon name="key" size="md" /></span>
          <b>{{ t('admin.riskControl.preBlockAPIKeyLoadEmpty') }}</b>
        </div>
      </div>
      <footer class="viz-card-foot">{{ apiKeyLoadSummaryText }}</footer>
    </section>
  </div>

  <section v-else-if="mode === 'observe'" data-test="worker-runtime-card" class="viz-card">
    <header class="viz-card-head">
      <h2 class="viz-card-title">{{ t('admin.riskControl.workerStatus') }}</h2>
      <HelpTooltip :content="t('admin.riskControl.workerStatusHint')" interactive :aria-label="t('admin.riskControl.workerStatus')" />
      <div class="viz-card-tools">
        <span class="badge badge-gray">{{ modeText }}</span>
      </div>
    </header>
    <div class="viz-card-body rc-worker">
      <div class="rc-worker-side">
        <div>
          <div class="rc-worker-line">
            <span>{{ t('admin.riskControl.queueUsage') }}</span>
            <span class="tabular-nums">{{ formatNumber(status?.queue_length ?? 0) }} / {{ formatNumber(status?.queue_size ?? queueSize) }}</span>
          </div>
          <div :class="queueUsage >= 90 ? 'ui-meter is-bad' : queueUsage >= 70 ? 'ui-meter is-warn' : 'ui-meter'">
            <div class="ui-meter-track"><i :style="{ width: `${queueUsage.toFixed(1)}%` }"></i></div>
            <b>{{ queueUsage.toFixed(1) }}%</b>
          </div>
        </div>
        <dl class="rc-metrics is-two">
          <div class="rc-metric">
            <dt><span class="ui-dot rc-dot-accent" aria-hidden="true"></span>{{ t('admin.riskControl.activeWorkers') }}</dt>
            <dd>{{ status?.active_workers ?? 0 }}</dd>
          </div>
          <div class="rc-metric">
            <dt><span class="ui-dot is-ok" aria-hidden="true"></span>{{ t('admin.riskControl.idleWorkers') }}</dt>
            <dd>{{ status?.idle_workers ?? workerCount }}</dd>
          </div>
          <div class="rc-metric">
            <dt>{{ t('admin.riskControl.processed') }}</dt>
            <dd>{{ formatNumber(status?.processed ?? 0) }}</dd>
          </div>
          <div class="rc-metric">
            <dt>{{ t('admin.riskControl.droppedErrors') }}</dt>
            <dd>{{ formatNumber((status?.dropped ?? 0) + (status?.errors ?? 0)) }}</dd>
          </div>
        </dl>
      </div>
      <div>
        <div class="rc-worker-line">
          <span>{{ t('admin.riskControl.workerPool') }}</span>
          <span>{{ t('admin.riskControl.workerPoolMeta', { active: status?.active_workers ?? 0, idle: status?.idle_workers ?? workerCount, total: status?.worker_count ?? workerCount }) }}</span>
        </div>
        <ul class="rc-slots">
          <li
            v-for="worker in workerSlots"
            :key="worker.id"
            :class="worker.state === 'active' ? 'rc-slot is-active' : worker.state === 'idle' ? 'rc-slot is-idle' : 'rc-slot'"
            :title="worker.label"
          >
            #{{ worker.id }}
          </li>
        </ul>
      </div>
    </div>
    <footer class="viz-card-foot">
      <span>{{ t('admin.riskControl.autoRefresh') }}</span>
      <span v-if="status?.last_cleanup_at">{{ t('admin.riskControl.lastCleanup', { time: formatDateTime(status.last_cleanup_at) }) }}</span>
    </footer>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import type { ContentModerationAPIKeyLoad, ContentModerationRuntimeStatus, ModerationMode } from '@/api/admin/riskControl'
import { formatDateTime, formatNumber, keyStatusDotClass } from './riskControlFormat'

type WorkerSlotState = 'active' | 'idle' | 'disabled'

/** Live runtime cards: synchronous pre-block counters or the async worker pool, by mode. */
const props = defineProps<{
  status: ContentModerationRuntimeStatus | null
  mode: ModerationMode
  modeText: string
  /** Configured fallbacks used until the runtime status reports its own values. */
  workerCount: number
  queueSize: number
}>()

const { t } = useI18n()

const preBlockMetricItems = computed(() => {
  const s = props.status
  return [
    { key: 'active', label: t('admin.riskControl.preBlockActive'), hint: t('admin.riskControl.preBlockActiveHint'), value: formatNumber(s?.pre_block_active ?? 0), dot: 'ui-dot rc-dot-accent' },
    { key: 'checked', label: t('admin.riskControl.preBlockChecked'), hint: t('admin.riskControl.preBlockCheckedHint'), value: formatNumber(s?.pre_block_checked ?? 0), dot: 'ui-dot' },
    { key: 'allowed', label: t('admin.riskControl.preBlockAllowed'), hint: t('admin.riskControl.preBlockAllowedHint'), value: formatNumber(s?.pre_block_allowed ?? 0), dot: 'ui-dot is-ok' },
    { key: 'blocked', label: t('admin.riskControl.preBlockBlocked'), hint: t('admin.riskControl.preBlockBlockedHint'), value: formatNumber(s?.pre_block_blocked ?? 0), dot: 'ui-dot is-bad' },
    { key: 'errors', label: t('admin.riskControl.preBlockErrors'), hint: t('admin.riskControl.preBlockErrorsHint'), value: formatNumber(s?.pre_block_errors ?? 0), dot: 'ui-dot is-warn' },
    { key: 'latency', label: t('admin.riskControl.preBlockAvgLatency'), hint: t('admin.riskControl.preBlockAvgLatencyHint'), value: `${formatNumber(s?.pre_block_avg_latency_ms ?? 0)} ms`, dot: 'ui-dot' },
  ]
})

const apiKeyLoads = computed<ContentModerationAPIKeyLoad[]>(() => (
  [...(props.status?.pre_block_api_key_loads ?? [])].sort((a, b) => a.index - b.index)
))

const apiKeyMaxTotal = computed(() => Math.max(1, ...apiKeyLoads.value.map((item) => item.total || 0)))

const apiKeyLoadSummaryText = computed(() => t('admin.riskControl.preBlockAPIKeyLoadSummary', {
  active: formatNumber(props.status?.pre_block_api_key_active ?? 0),
  available: formatNumber(props.status?.pre_block_api_key_available_count ?? 0),
  total: formatNumber(props.status?.pre_block_api_key_total_calls ?? 0),
  workerActive: formatNumber(props.status?.active_workers ?? 0),
  workerTotal: formatNumber(props.status?.worker_count ?? props.workerCount),
}))

function loadWidth(total: number): string {
  return `${Math.min(100, Math.max(0, (total / apiKeyMaxTotal.value) * 100)).toFixed(1)}%`
}

const queueUsage = computed(() => Math.min(100, Math.max(0, props.status?.queue_usage_percent ?? 0)))

const workerSlots = computed(() => {
  const total = Math.max(0, props.status?.worker_count ?? props.workerCount)
  const active = Math.max(0, props.status?.active_workers ?? 0)
  const enabled = Boolean(props.status?.risk_control_enabled && props.status?.enabled && props.status?.mode !== 'off')
  return Array.from({ length: total }, (_, index) => ({
    id: index + 1,
    state: (!enabled ? 'disabled' : index < active ? 'active' : 'idle') as WorkerSlotState,
    label: !enabled
      ? t('admin.riskControl.workerDisabled')
      : index < active
        ? t('admin.riskControl.workerActive')
        : t('admin.riskControl.workerIdle'),
  }))
})
</script>

<style scoped>
.rc-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.5rem;
}

.rc-metrics.is-two {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.rc-metric {
  display: grid;
  gap: 0.25rem;
  min-width: 0;
  padding: 0.625rem 0.75rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
}

.rc-metric dt {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  overflow: hidden;
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.rc-metric dd {
  overflow: hidden;
  color: var(--operator-foreground);
  font-size: 1.25rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.02em;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rc-dot-accent {
  background: var(--operator-accent);
}

.rc-loads {
  display: grid;
  gap: 0.25rem;
  margin: 0 -0.25rem;
  padding: 0 0.25rem;
}

.rc-load {
  display: grid;
  grid-template-columns: minmax(8rem, 1fr) minmax(4rem, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  padding: 0.5rem 0.25rem;
  border-bottom: 1px solid var(--operator-border);
}

.rc-load:last-child {
  border-bottom: 0;
}

.rc-load-key {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
  color: var(--operator-foreground);
  font-size: 0.8125rem;
}

.rc-load-index {
  color: var(--operator-subtle-foreground);
  font-variant-numeric: tabular-nums;
}

.rc-load-meter {
  min-width: 3rem;
}

.rc-load-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(3rem, auto));
  gap: 0.75rem;
  text-align: right;
}

.rc-load-stats dt {
  color: var(--operator-subtle-foreground);
  font-size: 0.6875rem;
}

.rc-load-stats dd {
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.rc-worker {
  display: grid;
  grid-template-columns: minmax(0, 20rem) minmax(0, 1fr);
  gap: 1.5rem;
}

.rc-worker-side {
  display: grid;
  align-content: start;
  gap: 1rem;
}

.rc-worker-line {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 0.25rem 0.75rem;
  margin-bottom: 0.5rem;
  color: var(--operator-muted-foreground);
  font-size: 0.78rem;
}

.rc-slots {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(3.25rem, 1fr));
  gap: 0.375rem;
}

.rc-slot {
  display: grid;
  height: 2.25rem;
  place-items: center;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.rc-slot.is-idle {
  border-color: rgb(var(--operator-success-rgb) / 0.35);
  background: rgb(var(--operator-success-rgb) / 0.08);
  color: var(--operator-success);
}

.rc-slot.is-active {
  border-color: rgb(var(--operator-accent-rgb) / 0.45);
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
}

@media (max-width: 900px) {
  .rc-worker {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 640px) {
  .rc-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .rc-load {
    grid-template-columns: minmax(0, 1fr) minmax(3rem, 6rem);
  }

  .rc-load-stats {
    grid-column: 1 / -1;
    text-align: left;
  }
}
</style>
