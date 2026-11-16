<template>
  <button
    type="button"
    class="ui-card group w-full p-4 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50"
    :data-testid="`monitor-card-${item.id}`"
    @click="emit('click')"
  >
    <!-- Header: provider mark + name/model + status -->
    <div class="flex min-w-0 items-start gap-3">
      <ProviderMark :platform="item.provider" :size="32" :label="providerLabel(item.provider)" />
      <div class="min-w-0 flex-1">
        <div class="ui-cell-main">{{ item.name }}</div>
        <div class="mt-0.5 flex min-w-0 items-center gap-1.5 text-xs text-gray-500">
          <!-- 纯配额模式主模型是占位符 "quota"，展示层替换为本地化「配额」标签 -->
          <span class="truncate font-mono">{{ formatMonitorModel(item.primary_model) }}</span>
          <span v-if="item.group_name" class="badge badge-gray shrink-0">{{ item.group_name }}</span>
        </div>
      </div>
      <span class="inline-flex shrink-0 items-center gap-1.5 text-xs font-medium text-gray-700 dark:text-gray-200">
        <span class="ui-dot" :class="statusDotClass" aria-hidden="true"></span>
        {{ statusLabel(item.primary_status) }}
      </span>
    </div>

    <!-- Availability -->
    <MonitorAvailabilityRow
      :window-label="availabilityLabel"
      :value="availabilityValue"
      :samples-label="extraModelsCountLabel"
    />

    <!-- Timeline -->
    <MonitorTimeline
      :buckets="item.timeline"
      :countdown-seconds="countdownSeconds"
    />

    <!-- Metrics -->
    <MonitorMetricPair
      primary-icon="bolt"
      :primary-label="t('monitorCommon.dialogLatency')"
      :primary-value="formatLatency(item.primary_latency_ms)"
      primary-unit="ms"
      secondary-icon="globe"
      :secondary-label="t('monitorCommon.endpointPing')"
      :secondary-value="formatLatency(item.primary_ping_latency_ms)"
      secondary-unit="ms"
    />

    <!-- 配额模式：最新用量/余额快照（服务端已按系统开关剥离，此处 flag 为纵深防御） -->
    <MonitorQuotaView v-if="quotaVisible" :snapshot="item.latest_quota" class="mt-3" />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserMonitorView } from '@/api/channelMonitor'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import { STATUS_DEGRADED, STATUS_FAILED, STATUS_OPERATIONAL } from '@/constants/channelMonitor'
import { isChannelMonitorQuotaVisible } from '@/utils/featureFlags'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import MonitorMetricPair from './MonitorMetricPair.vue'
import MonitorAvailabilityRow from './MonitorAvailabilityRow.vue'
import MonitorTimeline from './MonitorTimeline.vue'
import MonitorQuotaView from '@/components/common/MonitorQuotaView.vue'

const props = defineProps<{
  item: UserMonitorView
  window: '7d' | '15d' | '30d'
  availabilityValue: number | null
  countdownSeconds: number
}>()

const emit = defineEmits<{
  (e: 'click'): void
}>()

const { t } = useI18n()
const {
  statusLabel,
  providerLabel,
  formatLatency,
  formatMonitorModel,
} = useChannelMonitorFormat()

const statusDotClass = computed(() => {
  switch (props.item.primary_status) {
    case STATUS_OPERATIONAL: return 'is-ok'
    case STATUS_DEGRADED: return 'is-warn'
    case STATUS_FAILED: return 'is-bad'
    default: return ''
  }
})

const quotaVisible = computed(
  () => isChannelMonitorQuotaVisible() && !!props.item.latest_quota
)

const availabilityLabel = computed(() => {
  const win = t(`channelStatus.windowTab.${props.window}`)
  return `${t('monitorCommon.availabilityPrefix')} · ${win}`
})

const extraModelsCountLabel = computed(() => {
  const count = props.item.extra_models?.length ?? 0
  if (count === 0) return undefined
  return t('monitorCommon.extraModelsCount', { n: count })
})
</script>
