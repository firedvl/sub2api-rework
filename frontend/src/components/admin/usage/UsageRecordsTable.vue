<template>
  <div class="usage-records">
    <div v-if="showIpGeoToolbar" class="usage-records-bar">
      <span v-if="pendingIpCount > 0">{{ t('usage.ipGeo.pending', { count: pendingIpCount }) }}</span>
      <button
        type="button"
        class="btn btn-ghost btn-sm"
        :disabled="ipGeoBatchLoading || pendingIpCount === 0"
        @click="handleBatchFetchIpGeo"
      >
        <Icon name="globe" size="sm" />
        {{ ipGeoBatchLoading ? t('usage.ipGeo.batchFetching') : t('usage.ipGeo.batchFetch') }}
      </button>
    </div>

    <!-- Desktop table -->
    <div class="ui-table-wrap usage-records-table hidden md:block" :class="{ 'is-busy': loading && data.length > 0 }">
      <table class="ui-table">
        <thead>
          <tr>
            <th v-for="col in columns" :key="col.key" scope="col" :class="numericKeys.has(col.key) ? 'is-num' : ''" :aria-sort="ariaSort(col)">
              <button
                v-if="col.sortable"
                type="button"
                class="ui-sort"
                :aria-pressed="sortKey === col.key"
                @click="toggleSort(col.key)"
              >
                {{ col.label }}
                <Icon :name="sortKey === col.key ? (sortOrder === 'asc' ? 'arrowUp' : 'arrowDown') : 'arrowsUpDown'" size="xs" />
              </button>
              <template v-else>{{ col.label }}</template>
            </th>
          </tr>
        </thead>
        <tbody v-if="loading && data.length === 0">
          <tr v-for="i in 6" :key="i">
            <td v-for="col in columns" :key="col.key"><div class="skeleton h-4 w-3/4 rounded"></div></td>
          </tr>
        </tbody>
        <tbody v-else-if="data.length === 0">
          <tr>
            <td :colspan="columns.length" class="usage-records-empty-cell">
              <div class="ui-empty">
                <span class="ui-empty-icon"><Icon name="inbox" size="md" /></span>
                <b>{{ t('usage.noRecords') }}</b>
              </div>
            </td>
          </tr>
        </tbody>
        <tbody v-else class="ui-rows-enter">
          <tr
            v-for="row in data"
            :key="row.id"
            class="is-clickable"
            :class="{ 'is-selected': row.id === selectedId }"
            tabindex="0"
            data-testid="usage-record-row"
            :data-row-id="row.id"
            @click="emit('rowClick', row)"
            @keydown.enter.prevent="emit('rowClick', row)"
            @keydown.space.prevent="emit('rowClick', row)"
          >
            <td v-for="col in columns" :key="col.key" :class="[numericKeys.has(col.key) ? 'is-num' : '', `usage-col-${col.key}`]">
              <template v-if="col.key === 'created_at'">
                <time :datetime="row.created_at" :title="formatDateTime(row.created_at)" class="usage-time">{{ formatRelativeTime(row.created_at) }}</time>
              </template>

              <template v-else-if="col.key === 'user'">
                <button
                  v-if="row.user?.email"
                  type="button"
                  class="usage-user-link ui-cell-main"
                  :title="t('admin.usage.clickToViewBalance')"
                  @click.stop="emit('userClick', row.user_id, row.user?.email)"
                >
                  {{ row.user.email }}
                </button>
                <span v-else class="ui-cell-main">-</span>
                <span class="ui-cell-sub">
                  #{{ row.user_id }}
                  <span v-if="row.user?.deleted_at" class="badge badge-danger ml-1">{{ t('admin.usage.userDeletedBadge') }}</span>
                </span>
              </template>

              <template v-else-if="col.key === 'api_key'">
                <span class="ui-cell-main usage-clip">{{ row.api_key?.name || '-' }}</span>
              </template>

              <template v-else-if="col.key === 'account'">
                <span class="ui-cell-main usage-clip">{{ row.account?.name || '-' }}</span>
              </template>

              <template v-else-if="col.key === 'model'">
                <div class="usage-model">
                  <ProviderMark :platform="platformOf(row)" :size="22" />
                  <div class="min-w-0">
                    <span class="ui-cell-main usage-clip" :title="row.model">{{ row.model }}</span>
                    <span v-if="routedModel(row)" class="ui-cell-sub usage-clip" :title="row.model_mapping_chain || routedModel(row)">↳ {{ routedModel(row) }}</span>
                  </div>
                  <span
                    v-if="row.upstream_model_mismatch === true && row.upstream_response_model"
                    class="badge"
                    :class="isLikelyModelVariant(row) ? 'badge-warning' : 'badge-danger'"
                    :title="modelAuditTitle(row)"
                  >{{ isLikelyModelVariant(row) ? t('usage.modelVariant') : t('usage.modelMismatch') }}</span>
                </div>
              </template>

              <template v-else-if="col.key === 'reasoning_effort'">
                <span data-testid="reasoning-effort-cell" class="ui-cell-main">
                  {{ formatReasoningEffort(row.reasoning_effort) }}<template v-if="hasReasoningEffortMapping(row)"> → {{ formatReasoningEffort(row.upstream_reasoning_effort) }}</template>
                </span>
              </template>

              <template v-else-if="col.key === 'endpoint'">
                <span class="ui-cell-main usage-clip font-mono text-xs" :title="row.inbound_endpoint || ''">{{ row.inbound_endpoint?.trim() || '-' }}</span>
                <span v-if="row.upstream_endpoint?.trim()" class="ui-cell-sub usage-clip font-mono" :title="row.upstream_endpoint || ''">↳ {{ row.upstream_endpoint.trim() }}</span>
              </template>

              <template v-else-if="col.key === 'group'">
                <span v-if="row.group" class="badge badge-gray">{{ row.group.name }}</span>
                <span v-else class="text-gray-400">-</span>
              </template>

              <template v-else-if="col.key === 'stream'">
                <span class="inline-flex items-center gap-1">
                  <span data-testid="request-type-badge" class="badge" :class="requestTypeBadge(row)">{{ requestTypeLabel(row) }}</span>
                  <span v-if="row.native_compaction_v2" data-testid="native-compaction-badge" class="badge badge-primary">{{ t('usage.nativeCompactionV2') }}</span>
                </span>
              </template>

              <template v-else-if="col.key === 'billing_mode'">
                <span class="badge badge-gray">{{ getBillingModeLabel(getDisplayBillingMode(row), t) }}</span>
              </template>

              <template v-else-if="col.key === 'tokens'">
                <template v-if="isImageUsage(row)">
                  <span class="ui-cell-main">{{ row.image_count }}{{ t('usage.imageUnit') }}</span>
                  <span class="ui-cell-sub">{{ formatImageBillingSize(row, t) }}</span>
                </template>
                <template v-else>
                  <span class="ui-cell-main">{{ formatCompact(totalTokens(row)) }}</span>
                  <span class="ui-cell-sub usage-token-split">
                    <span :title="t('admin.usage.inputTokens')"><Icon name="arrowDown" size="xs" />{{ formatCompact(row.input_tokens) }}</span>
                    <span :title="t('admin.usage.outputTokens')"><Icon name="arrowUp" size="xs" />{{ formatCompact(row.output_tokens) }}</span>
                    <span v-if="row.cache_read_tokens > 0" :title="t('admin.usage.cacheReadTokens')" class="text-sky-600 dark:text-sky-400"><Icon name="database" size="xs" />{{ formatCompact(row.cache_read_tokens) }}</span>
                  </span>
                </template>
              </template>

              <template v-else-if="col.key === 'cost'">
                <span class="ui-cell-main usage-cost">
                  <span v-if="row.long_context_billing_applied" data-testid="long-context-billing-marker" class="badge badge-warning">x2</span>
                  {{ formatCost(row.actual_cost) }}
                </span>
                <span v-if="row.usage_observation_complete !== true" data-testid="usage-observation-label" class="ui-cell-sub text-amber-600 dark:text-amber-400">{{ t(usageObservationKey(row.usage_observation_complete)) }}</span>
              </template>

              <template v-else-if="col.key === 'latency'">
                <span class="usage-latency">
                  <span class="ui-dot" :class="DOT_CLASS[latencySeverity(row)]" aria-hidden="true"></span>
                  <span class="ui-cell-main">{{ formatDuration(row.duration_ms) }}</span>
                </span>
                <span v-if="row.first_token_ms != null" class="ui-cell-sub">{{ t('usage.latencyFirstToken') }} {{ formatDuration(row.first_token_ms) }}</span>
              </template>

              <template v-else-if="col.key === 'request_id' || col.key === 'upstream_request_id'">
                <span v-if="recordId(row, col.key)" class="usage-id">
                  <span class="usage-clip font-mono text-xs" :title="recordId(row, col.key)">{{ recordId(row, col.key) }}</span>
                  <button
                    type="button"
                    class="usage-copy"
                    :aria-label="t('keys.copyToClipboard')"
                    @click.stop="copy(recordId(row, col.key))"
                  >
                    <Icon :name="copiedValue === recordId(row, col.key) ? 'check' : 'copy'" size="xs" />
                  </button>
                </span>
                <span v-else class="text-gray-400">-</span>
              </template>

              <template v-else-if="col.key === 'user_agent'">
                <span v-if="row.user_agent" class="ui-cell-sub usage-clip usage-ua" :title="row.user_agent">{{ row.user_agent }}</span>
                <span v-else class="text-gray-400">-</span>
              </template>

              <template v-else-if="col.key === 'ip_address'">
                <div v-if="row.ip_address" @click.stop>
                  <span class="font-mono text-xs">{{ row.ip_address }}</span>
                  <IpGeoCell :ip="row.ip_address" />
                </div>
                <span v-else class="text-gray-400">-</span>
              </template>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Mobile list: the drawer carries every field. -->
    <ul class="usage-records-list md:hidden" :class="{ 'is-busy': loading && data.length > 0 }">
      <template v-if="loading && data.length === 0">
        <li v-for="i in 5" :key="i" class="usage-records-item"><div class="skeleton h-9 w-full rounded"></div></li>
      </template>
      <li v-else-if="data.length === 0" class="ui-empty">
        <span class="ui-empty-icon"><Icon name="inbox" size="md" /></span>
        <b>{{ t('usage.noRecords') }}</b>
      </li>
      <template v-else>
        <li v-for="row in data" :key="row.id">
          <button type="button" class="usage-records-item" data-testid="usage-record-row" :class="{ 'is-selected': row.id === selectedId }" @click="emit('rowClick', row)">
            <ProviderMark :platform="platformOf(row)" :size="28" />
            <span class="min-w-0 flex-1 text-left">
              <span class="ui-cell-main">{{ row.model }}</span>
              <span class="ui-cell-sub">{{ row.user?.email || `#${row.user_id}` }} · {{ formatRelativeTime(row.created_at) }}</span>
            </span>
            <span class="text-right">
              <span class="ui-cell-main tabular-nums">{{ formatCost(row.actual_cost) }}</span>
              <span class="ui-cell-sub usage-latency justify-end">
                <span class="ui-dot" :class="DOT_CLASS[latencySeverity(row)]" aria-hidden="true"></span>
                {{ formatDuration(row.duration_ms) }}
              </span>
            </span>
          </button>
        </li>
      </template>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { formatDateTime, formatRelativeTime, formatReasoningEffort } from '@/utils/format'
import { usageObservationKey } from '@/utils/usageObservation'
import { getBillingModeLabel, getDisplayBillingMode, isImageUsage } from '@/utils/billingMode'
import { formatImageBillingSize } from '@/utils/imageUsage'
import { fetchBatch, getEntry } from '@/utils/ipGeoLookup'
import Icon from '@/components/icons/Icon.vue'
import IpGeoCell from '@/components/common/IpGeoCell.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import type { AdminUsageLog } from '@/types'
import type { Column } from '@/components/common/types'
import {
  DOT_CLASS,
  formatCompact,
  formatCost,
  formatDuration,
  hasReasoningEffortMapping,
  isLikelyModelVariant,
  latencySeverity,
  platformOf,
  recordId,
  requestTypeBadge,
  routedModel,
  totalTokens,
  useRecordLabels,
} from './usageRecord'

const props = withDefaults(defineProps<{
  data: AdminUsageLog[]
  loading?: boolean
  columns: Column[]
  sortKey?: string
  sortOrder?: 'asc' | 'desc'
  selectedId?: number | null
}>(), {
  loading: false,
  sortKey: 'created_at',
  sortOrder: 'desc',
  selectedId: null,
})

const emit = defineEmits<{
  rowClick: [row: AdminUsageLog]
  userClick: [userID: number, email?: string]
  sort: [key: string, order: 'asc' | 'desc']
  ipGeoBatchFailed: []
}>()

const { t } = useI18n()
const appStore = useAppStore()
const { requestTypeLabel, modelAuditTitle } = useRecordLabels(t)

const numericKeys = new Set(['tokens', 'cost', 'latency'])

const ariaSort = (col: Column) => {
  if (!col.sortable) return undefined
  if (props.sortKey !== col.key) return 'none'
  return props.sortOrder === 'asc' ? 'ascending' : 'descending'
}

const toggleSort = (key: string) => {
  const order = props.sortKey === key && props.sortOrder === 'desc' ? 'asc' : 'desc'
  emit('sort', key, order)
}

// IP geo batch lookup for the current page (only when the IP column is shown).
const ipGeoBatchLoading = ref(false)
const showIpGeoToolbar = computed(() => props.columns.some((col) => col.key === 'ip_address'))
const currentPageIps = computed(() =>
  Array.from(new Set(props.data.map((row) => row.ip_address).filter((ip): ip is string => Boolean(ip))))
)
const pendingIpCount = computed(() => {
  if (!showIpGeoToolbar.value) return 0
  return currentPageIps.value.filter((ip) => {
    const status = getEntry(ip).status
    return status === 'idle' || status === 'error'
  }).length
})
const handleBatchFetchIpGeo = async () => {
  ipGeoBatchLoading.value = true
  try {
    const ok = await fetchBatch(currentPageIps.value)
    if (!ok) emit('ipGeoBatchFailed')
  } finally {
    ipGeoBatchLoading.value = false
  }
}

const copiedValue = ref<string | null>(null)
const copy = async (value: string) => {
  try {
    await navigator.clipboard.writeText(value)
    copiedValue.value = value
    appStore.showSuccess(t('keys.copied'))
    window.setTimeout(() => {
      if (copiedValue.value === value) copiedValue.value = null
    }, 2000)
  } catch {
    appStore.showError(t('common.copyFailed'))
  }
}
</script>

<style scoped>
.usage-records-bar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
  padding: 0.375rem 0.75rem;
  border-bottom: 1px solid var(--operator-border);
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}
.usage-records-table,
.usage-records-list {
  transition: opacity var(--operator-duration) var(--operator-ease-out);
}
.is-busy {
  opacity: 0.55;
}
.usage-records-table {
  max-height: min(70vh, 48rem);
}
.usage-records-table td {
  max-width: 18rem;
}
.usage-records-table tr:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: -2px;
}
.usage-records-empty-cell {
  height: auto;
}
.usage-clip {
  display: block;
  max-width: 14rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.usage-time {
  color: var(--operator-muted-foreground);
  white-space: nowrap;
}
.usage-user-link {
  max-width: 14rem;
  text-align: left;
  text-decoration: underline dashed transparent;
  text-underline-offset: 3px;
  transition: text-decoration-color var(--operator-duration-fast) var(--operator-ease-out), color var(--operator-duration-fast) var(--operator-ease-out);
}
.usage-user-link:hover {
  color: var(--operator-accent-text);
  text-decoration-color: currentColor;
}
.usage-model {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.625rem;
}
.usage-token-split,
.usage-latency,
.usage-cost {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.5rem;
}
.usage-token-split > span {
  display: inline-flex;
  align-items: center;
  gap: 0.125rem;
}
.usage-latency {
  gap: 0.4375rem;
}
.usage-id {
  display: inline-flex;
  max-width: 11rem;
  align-items: center;
  gap: 0.25rem;
}
.usage-copy {
  display: inline-grid;
  width: 1.5rem;
  height: 1.5rem;
  flex: none;
  place-items: center;
  border-radius: var(--operator-radius-sm);
  color: var(--operator-subtle-foreground);
}
.usage-copy:hover {
  background: var(--operator-muted);
  color: var(--operator-foreground);
}
.usage-copy:focus-visible {
  outline: 2px solid var(--operator-focus);
}
.usage-ua {
  max-width: 16rem;
}
.usage-records-list > li + li {
  border-top: 1px solid var(--operator-border-subtle);
}
.usage-records-item {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem 1rem;
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}
.usage-records-item:hover,
.usage-records-item.is-selected {
  background: var(--table-row-hover-background);
}
.usage-records-item:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: -2px;
}
.usage-records-list > li {
  animation: ui-rise 320ms var(--operator-ease-out) backwards;
}
@media (prefers-reduced-motion: reduce) {
  .usage-records-list > li {
    animation: none;
  }
}
</style>
