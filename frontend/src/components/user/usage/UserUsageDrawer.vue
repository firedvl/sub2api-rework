<template>
  <SideDrawer
    :open="open && !!record"
    :title="record?.model || ''"
    :subtitle="record ? formatDateTime(record.created_at) : ''"
    width="32rem"
    @close="emit('close')"
  >
    <template #leading>
      <ProviderMark v-if="record && providerOfModel(record.model)" :platform="providerOfModel(record.model)" :size="36" />
      <span v-else class="uud-mark" aria-hidden="true"><Icon name="cube" size="sm" /></span>
    </template>
    <template #badge>
      <span v-if="record" class="badge badge-gray">{{ requestTypeLabel }}</span>
    </template>

    <template v-if="record">
      <div class="viz-kpis uud-kpis">
        <div class="viz-card viz-kpi">
          <span class="viz-kpi-label">{{ t('usage.actualCost') }}</span>
          <span class="viz-kpi-value">${{ record.actual_cost.toFixed(6) }}</span>
        </div>
        <div class="viz-card viz-kpi">
          <span class="viz-kpi-label">{{ t('usage.tokens') }}</span>
          <span class="viz-kpi-value">{{ formatCompact(totalTokens) }}</span>
        </div>
      </div>

      <section v-if="totalTokens > 0">
        <div class="ui-section-label">{{ t('usage.tokenDetails') }}</div>
        <div class="viz-share uud-share" role="img" :aria-label="tokenSummary" :title="tokenSummary">
          <i v-for="part in tokenParts" :key="part.label" :style="{ flex: part.value, background: part.color }"></i>
        </div>
        <div class="viz-rows uud-rows">
          <div v-for="part in tokenParts" :key="part.label" class="viz-row">
            <span class="viz-row-name"><i :style="{ background: part.color }"></i><span>{{ part.label }}</span></span>
            <span class="viz-row-value">{{ formatInt(part.value) }}</span>
          </div>
        </div>
      </section>

      <section>
        <div class="ui-section-label">{{ t('usage.costDetails') }}</div>
        <dl class="ui-dl">
          <template v-for="row in costRows" :key="row.label">
            <dt>{{ row.label }}</dt>
            <dd class="tabular-nums">{{ row.value }}</dd>
          </template>
        </dl>
      </section>

      <dl class="ui-dl">
        <template v-for="row in metaRows" :key="row.label">
          <dt>{{ row.label }}</dt>
          <dd :class="row.mono ? 'font-mono text-xs' : ''">{{ row.value }}</dd>
        </template>
      </dl>
    </template>
  </SideDrawer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import { formatDateTime, formatReasoningEffort } from '@/utils/format'
import { providerOfModel } from '@/utils/providerSeries'
import { formatCompact, formatInt } from '@/utils/vizFormat'
import { getBillingModeLabel, getDisplayBillingMode } from '@/utils/billingMode'
import { resolveUsageRequestType } from '@/utils/usageRequestType'
import { usageObservationKey } from '@/utils/usageObservation'
import type { UsageLog } from '@/types'

/** Full detail for one usage record; the table keeps only the scannable columns. */
const props = defineProps<{
  open: boolean
  record: UsageLog | null
}>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

const requestTypeLabel = computed(() => {
  if (!props.record) return ''
  const type = resolveUsageRequestType(props.record)
  const keys: Record<string, string> = { ws_v2: 'usage.ws', live: 'usage.live', stream: 'usage.stream', sync: 'usage.sync', cyber: 'usage.cyber' }
  return t(keys[type] ?? 'usage.unknown')
})

const tokenParts = computed(() => {
  const record = props.record
  if (!record) return []
  return [
    { label: t('usage.in'), value: record.input_tokens || 0, color: 'var(--operator-series-1)' },
    { label: t('usage.out'), value: record.output_tokens || 0, color: 'var(--operator-series-2)' },
    { label: t('usage.cacheHit'), value: record.cache_read_tokens || 0, color: 'var(--operator-series-3)' },
    { label: t('usage.cacheCreate'), value: record.cache_creation_tokens || 0, color: 'var(--operator-series-4)' },
  ].filter((part) => part.value > 0)
})
const totalTokens = computed(() => tokenParts.value.reduce((sum, part) => sum + part.value, 0))
const tokenSummary = computed(() => tokenParts.value.map((part) => `${part.label} ${formatInt(part.value)}`).join(' · '))

const usd = (value: number | null | undefined) => `$${(value ?? 0).toFixed(6)}`

const costRows = computed(() => {
  const record = props.record
  if (!record) return []
  const rows = [
    { label: t('usage.in'), value: usd(record.input_cost) },
    { label: t('usage.out'), value: usd(record.output_cost) },
    { label: t('usage.cacheHit'), value: usd(record.cache_read_cost) },
    { label: t('usage.cacheCreate'), value: usd(record.cache_creation_cost) },
  ].filter((row) => row.value !== usd(0))
  rows.push(
    { label: t('usage.standardCost'), value: usd(record.total_cost) },
    { label: t('usage.actualCost'), value: usd(record.actual_cost) },
    { label: '×', value: `${record.rate_multiplier}` },
  )
  if (record.image_count > 0) {
    rows.push({ label: t('usage.imageCount'), value: `${record.image_count}${record.image_size ? ` · ${record.image_size}` : ''}` })
  }
  return rows
})

const metaRows = computed(() => {
  const record = props.record
  if (!record) return []
  const rows: Array<{ label: string; value: string; mono?: boolean }> = []
  const push = (label: string, value: string | null | undefined, mono = false) => {
    if (value) rows.push({ label, value, mono })
  }
  push(t('usage.apiKeyFilter'), record.api_key?.name)
  push(t('admin.usage.group'), record.group?.name)
  push(t('usage.reasoningEffort'), record.reasoning_effort ? formatReasoningEffort(record.reasoning_effort) : '')
  push(t('admin.usage.billingMode'), getBillingModeLabel(getDisplayBillingMode(record), t))
  push(t('usage.firstToken'), record.first_token_ms != null ? `${record.first_token_ms} ms` : '')
  push(t('usage.duration'), record.duration_ms != null ? `${record.duration_ms} ms` : '')
  push(t('usage.inboundEndpoint'), record.inbound_endpoint, true)
  push('IP', record.ip_address, true)
  push(t('usage.userAgent'), record.user_agent, true)
  push(t('usage.observationCompleteness'), t(usageObservationKey(record.usage_observation_complete)))
  push('ID', record.request_id, true)
  return rows
})
</script>

<style scoped>
.uud-mark {
  display: inline-grid;
  width: 2.25rem;
  height: 2.25rem;
  flex: none;
  place-items: center;
  border-radius: 0.625rem;
  background: var(--operator-raised);
  color: var(--operator-subtle-foreground);
}

.uud-kpis {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin-bottom: 0;
}

.uud-share {
  height: 0.625rem;
}

.uud-rows {
  margin-top: 0.375rem;
}
</style>
