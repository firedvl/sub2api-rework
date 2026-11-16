<template>
  <AppLayout>
    <div class="ui-page monitor-page">
      <div class="monitor-head">
        <div
          class="viz-seg"
          role="tablist"
          :aria-label="t('channelMonitorV2.admin.tabAria')"
        >
          <button
            type="button"
            role="tab"
            :aria-selected="adminMonitorTab === 'v2'"
            :aria-pressed="adminMonitorTab === 'v2'"
            :title="t('channelMonitorV2.admin.tabV2')"
            @click="adminMonitorTab = 'v2'"
          >
            {{ t('admin.routingUi.monitorTabs.passive') }}
          </button>
          <button
            type="button"
            role="tab"
            :aria-selected="adminMonitorTab === 'legacy'"
            :aria-pressed="adminMonitorTab === 'legacy'"
            :title="isV1Mode ? t('channelMonitorV2.admin.tabV1Active') : t('channelMonitorV2.admin.tabV1History')"
            @click="adminMonitorTab = 'legacy'"
          >
            {{ isV1Mode ? t('admin.routingUi.monitorTabs.probes') : t('admin.routingUi.monitorTabs.history') }}
          </button>
        </div>
        <HelpTooltip interactive :aria-label="t('admin.routingUi.info')"
          :content="isV1Mode ? t('channelMonitorV2.admin.descriptionV1') : t('channelMonitorV2.admin.descriptionV2')"
        />
      </div>

      <MonitorSettingsPanel v-if="adminMonitorTab === 'v2'" />

      <template v-else>
        <MonitorFiltersBar
          v-model:search="searchQuery"
          v-model:provider="providerFilter"
          v-model:enabled="enabledFilter"
          :loading="loading"
          @reload="reload"
          @create="openCreateDialog"
          @manage-templates="showTemplateManager = true"
          @search-input="handleSearch"
        />

        <section class="ui-card monitor-card" :class="{ 'is-refreshing': loading && monitors.length > 0 }" :aria-busy="loading">
          <div class="ui-table-wrap">
            <table class="ui-table monitor-table">
              <thead>
                <tr>
                  <th v-for="col in columns" :key="col.key" :class="monitorColumnClass[col.key]" :data-col="col.key">
                    <span v-if="col.key === 'actions'" class="sr-only">{{ col.label }}</span>
                    <template v-else>{{ col.label }}</template>
                  </th>
                </tr>
              </thead>
              <tbody v-if="loading && monitors.length === 0">
                <tr v-for="n in 4" :key="`s${n}`" aria-hidden="true">
                  <td v-for="col in columns" :key="col.key" :class="monitorColumnClass[col.key]">
                    <span class="skeleton monitor-skel" :class="col.key === 'name' ? 'is-wide' : ''"></span>
                  </td>
                </tr>
              </tbody>
              <tbody v-else class="ui-rows-enter">
                <tr v-for="row in monitors" :key="row.id" :class="{ 'is-off': !row.enabled }">
                  <td v-for="col in columns" :key="col.key" :class="monitorColumnClass[col.key]" :data-col="col.key">
                    <div v-if="col.key === 'name'" class="monitor-name">
                      <span class="monitor-mark">
                        <ProviderMark :platform="row.provider" :size="30" />
                        <i class="ui-dot monitor-mark-dot" :class="statusDot(row.primary_status)" :title="statusLabel(row.primary_status)"></i>
                      </span>
                      <span class="min-w-0">
                        <span class="ui-cell-main monitor-title">
                          <span class="truncate">{{ row.name }}</span>
                          <HelpTooltip v-if="row.api_key_decrypt_failed" :content="t('admin.channelMonitor.apiKeyDecryptFailed')">
                            <Icon name="exclamationTriangle" size="sm" class="text-red-500" />
                          </HelpTooltip>
                        </span>
                        <span class="monitor-sub">
                          <span>{{ providerLabel(row.provider) }}</span>
                          <span class="monitor-mode" :class="checkModeBadgeClass(row.check_mode)">{{ checkModeLabel(row.check_mode) }}</span>
                        </span>
                      </span>
                    </div>

                    <MonitorPrimaryModelCell v-else-if="col.key === 'primary_model'" :row="row" />

                    <span
                      v-else-if="col.key === 'availability_7d'"
                      class="monitor-avail"
                      :class="availabilityTone(row)"
                      :title="formatAvailability(row)"
                    >
                      <span class="monitor-avail-track"><i v-if="row.primary_status" :style="{ width: `${Math.max(2, Math.min(100, row.availability_7d || 0))}%` }"></i></span>
                      <b>{{ formatAvailability(row) }}</b>
                    </span>

                    <span v-else-if="col.key === 'latency'" class="monitor-latency">
                      {{ formatLatency(row.primary_latency_ms) }}<small v-if="row.primary_latency_ms != null">ms</small>
                    </span>

                    <Toggle
                      v-else-if="col.key === 'enabled'"
                      :modelValue="row.enabled"
                      :aria-label="`${row.name}: ${t('admin.channelMonitor.columns.enabled')}`"
                      @update:modelValue="toggleEnabled(row)"
                    />

                    <MonitorActionsCell
                      v-else-if="col.key === 'actions'"
                      class="monitor-row-actions"
                      :row="row"
                      :running="runningId === row.id"
                      :duplicating="duplicatingIds.has(row.id)"
                      @run="handleRunNow"
                      @duplicate="handleDuplicate"
                      @edit="openEditDialog"
                      @delete="handleDelete"
                    />
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-if="!loading && monitors.length === 0" class="ui-empty">
            <span class="ui-empty-icon"><Icon name="signal" size="md" /></span>
            <template v-if="searchQuery.trim() || providerFilter || enabledFilter">
              <b>{{ t('admin.routingUi.noMatches') }}</b>
              <button type="button" class="btn btn-secondary btn-sm" @click="resetFilters">{{ t('admin.routingUi.reset') }}</button>
            </template>
            <template v-else>
              <b>{{ t('admin.channelMonitor.noMonitorsYet') }}</b>
              <button type="button" class="btn btn-primary btn-sm" @click="openCreateDialog">
                <Icon name="plus" size="sm" />{{ t('admin.routingUi.newMonitor') }}
              </button>
            </template>
          </div>
        </section>

        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="onPageChange"
          @update:pageSize="onPageSizeChange"
        />
      </template>
    </div>

    <MonitorFormDialog
      :show="showDialog"
      :monitor="editing"
      @close="closeDialog"
      @saved="reload"
    />

    <MonitorTemplateManagerDialog
      :show="showTemplateManager"
      @close="showTemplateManager = false"
      @updated="reload"
    />

    <MonitorRunResultDialog
      :show="showRunResult"
      :results="runResults"
      @close="showRunResult = false"
    />

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('common.delete')"
      :message="deleteConfirmMessage"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { adminAPI } from '@/api/admin'
import type {
  ChannelMonitor,
  CheckResult,
  ListParams,
  Provider,
} from '@/api/admin/channelMonitor'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import MonitorFiltersBar from '@/components/admin/monitor/MonitorFiltersBar.vue'
import MonitorFormDialog from '@/components/admin/monitor/MonitorFormDialog.vue'
import MonitorTemplateManagerDialog from '@/components/admin/monitor/MonitorTemplateManagerDialog.vue'
import MonitorRunResultDialog from '@/components/admin/monitor/MonitorRunResultDialog.vue'
import MonitorPrimaryModelCell from '@/components/admin/monitor/MonitorPrimaryModelCell.vue'
import MonitorActionsCell from '@/components/admin/monitor/MonitorActionsCell.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import MonitorSettingsPanel from '@/features/channel-monitor-v2/MonitorSettingsPanel.vue'
import { isChannelMonitorV1Mode } from '@/utils/featureFlags'

const { t } = useI18n()
const appStore = useAppStore()
const isV1Mode = computed(() => isChannelMonitorV1Mode())
const adminMonitorTab = ref<'v2' | 'legacy'>(isChannelMonitorV1Mode() ? 'legacy' : 'v2')
const {
  providerLabel,
  statusLabel,
  checkModeLabel,
  checkModeBadgeClass,
  formatLatency,
  formatAvailability,
} = useChannelMonitorFormat()

const monitors = ref<ChannelMonitor[]>([])
const loading = ref(false)
const runningId = ref<number | null>(null)
const searchQuery = ref('')
const providerFilter = ref<Provider | ''>('')
const enabledFilter = ref<'' | 'true' | 'false'>('')
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })

const showDialog = ref(false)
const showTemplateManager = ref(false)
const editing = ref<ChannelMonitor | null>(null)
const showDeleteDialog = ref(false)
const deleting = ref<ChannelMonitor | null>(null)
const showRunResult = ref(false)
const runResults = ref<CheckResult[]>([])
const duplicatingIds = reactive(new Set<number>())

let abortController: AbortController | null = null
let searchTimeout: ReturnType<typeof setTimeout> | null = null

// Provider and check mode render under the monitor name.
const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.channelMonitor.columns.name'), sortable: false },
  { key: 'primary_model', label: t('admin.channelMonitor.columns.primaryModel'), sortable: false },
  { key: 'availability_7d', label: t('admin.channelMonitor.columns.availability7d'), sortable: false },
  { key: 'latency', label: t('admin.channelMonitor.columns.latency'), sortable: false },
  { key: 'enabled', label: t('admin.channelMonitor.columns.enabled'), sortable: false },
  { key: 'actions', label: t('admin.channelMonitor.columns.actions'), sortable: false },
])

const monitorColumnClass: Record<string, string> = {
  name: 'monitor-col-name',
  primary_model: 'is-optional',
  latency: 'is-optional',
  enabled: 'monitor-col-enabled',
  actions: 'monitor-col-actions',
}

const statusDots: Record<string, string> = { operational: 'is-ok', degraded: 'is-warn', failed: 'is-bad', error: 'is-bad' }
const statusDot = (status: ChannelMonitor['primary_status']) => statusDots[status] ?? ''

function availabilityTone(row: ChannelMonitor): string {
  if (!row.primary_status) return 'is-none'
  const value = row.availability_7d ?? 0
  return value >= 99 ? 'is-ok' : value >= 95 ? 'is-warn' : 'is-bad'
}

function resetFilters() {
  searchQuery.value = ''
  providerFilter.value = ''
  enabledFilter.value = ''
  pagination.page = 1
  reload()
}

const deleteConfirmMessage = computed(() => {
  const name = deleting.value?.name || ''
  return t('admin.channelMonitor.deleteConfirm', { name })
})

async function reload() {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  loading.value = true
  try {
    const params: ListParams = {
      page: pagination.page,
      page_size: pagination.page_size,
    }
    if (providerFilter.value) params.provider = providerFilter.value
    if (enabledFilter.value === 'true') params.enabled = true
    if (enabledFilter.value === 'false') params.enabled = false
    if (searchQuery.value.trim()) params.search = searchQuery.value.trim()

    const res = await adminAPI.channelMonitor.list(params, { signal: ctrl.signal })
    if (ctrl.signal.aborted || abortController !== ctrl) return
    monitors.value = res.items || []
    pagination.total = res.total
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.loadError')))
  } finally {
    if (abortController === ctrl) {
      loading.value = false
      abortController = null
    }
  }
}

function handleSearch() {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    reload()
  }, 300)
}

function onPageChange(page: number) {
  pagination.page = page
  reload()
}

function onPageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  reload()
}

function openCreateDialog() {
  editing.value = null
  showDialog.value = true
}

function openEditDialog(row: ChannelMonitor) {
  editing.value = row
  showDialog.value = true
}

function closeDialog() {
  showDialog.value = false
  editing.value = null
}

async function toggleEnabled(row: ChannelMonitor) {
  const next = !row.enabled
  try {
    await adminAPI.channelMonitor.update(row.id, { enabled: next })
    row.enabled = next
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  }
}

async function handleRunNow(row: ChannelMonitor) {
  if (!isV1Mode.value) {
    appStore.showError(t('admin.channelMonitor.runFailed'))
    return
  }
  if (runningId.value != null) return
  runningId.value = row.id
  try {
    const res = await adminAPI.channelMonitor.runNow(row.id)
    runResults.value = res.results || []
    showRunResult.value = true
    appStore.showSuccess(t('admin.channelMonitor.runSuccess'))
    // Refresh row to get latest status from backend
    void reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.runFailed')))
  } finally {
    runningId.value = null
  }
}

async function handleDuplicate(row: ChannelMonitor) {
  if (row.api_key_decrypt_failed) {
    appStore.showError(t('admin.channelMonitor.duplicateKeyUnavailable'))
    return
  }
  if (duplicatingIds.has(row.id)) return

  duplicatingIds.add(row.id)
  try {
    const duplicate = await adminAPI.channelMonitor.duplicate(row.id)
    appStore.showSuccess(t('admin.channelMonitor.duplicateSuccess', { name: duplicate.name }))
    await reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.duplicateFailed')))
  } finally {
    duplicatingIds.delete(row.id)
  }
}

function handleDelete(row: ChannelMonitor) {
  deleting.value = row
  showDeleteDialog.value = true
}

async function confirmDelete() {
  if (!deleting.value) return
  try {
    await adminAPI.channelMonitor.del(deleting.value.id)
    appStore.showSuccess(t('admin.channelMonitor.deleteSuccess'))
    showDeleteDialog.value = false
    deleting.value = null
    reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  }
}

watch(adminMonitorTab, (tab) => {
  if (tab === 'legacy' && monitors.value.length === 0) void reload()
})
onMounted(() => {
  if (adminMonitorTab.value === 'legacy') void reload()
})
onUnmounted(() => {
  if (searchTimeout) clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>

<style scoped>
.monitor-head {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.monitor-head .viz-seg button {
  height: 1.875rem;
  padding: 0 0.875rem;
  font-size: 0.8125rem;
}

.monitor-card {
  overflow: hidden;
  transition: opacity var(--operator-duration) var(--operator-ease-out);
}

.monitor-card.is-refreshing {
  opacity: 0.7;
}

.monitor-table td {
  height: 3.75rem;
}

.monitor-table tr.is-off .monitor-name {
  opacity: 0.55;
}

.monitor-col-name {
  min-width: 15rem;
}

.monitor-col-enabled,
.monitor-col-actions {
  width: 1%;
  white-space: nowrap;
}

.monitor-skel {
  display: block;
  width: 4rem;
  height: 0.75rem;
  border-radius: 999px;
}

.monitor-skel.is-wide {
  width: 11rem;
}

.monitor-name {
  display: flex;
  min-width: 0;
  max-width: 24rem;
  align-items: center;
  gap: 0.75rem;
}

.monitor-mark {
  position: relative;
  display: inline-flex;
  flex: none;
}

.monitor-mark-dot {
  position: absolute;
  right: -2px;
  bottom: -2px;
  width: 0.5625rem;
  height: 0.5625rem;
  box-shadow: 0 0 0 2px var(--operator-card) !important;
}

.monitor-title {
  display: flex;
  align-items: center;
  gap: 0.375rem;
}

.monitor-sub {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
  white-space: nowrap;
}

.monitor-mode {
  display: inline-flex;
  align-items: center;
  padding: 0 0.375rem;
  border-radius: 999px;
  font-size: 0.6875rem;
  font-weight: 500;
  line-height: 1.125rem;
}

.monitor-avail {
  display: flex;
  min-width: 8rem;
  align-items: center;
  gap: 0.5rem;
}

.monitor-avail-track {
  position: relative;
  height: 0.375rem;
  flex: 1 1 auto;
  overflow: hidden;
  border-radius: 999px;
  background: var(--operator-track);
}

.monitor-avail-track i {
  position: absolute;
  inset: 0 auto 0 0;
  border-radius: inherit;
  background: var(--operator-success-fill);
  transform-origin: left center;
  animation: viz-grow-x 700ms var(--operator-ease-out) both;
}

.monitor-avail.is-warn .monitor-avail-track i { background: var(--operator-warning-fill); }
.monitor-avail.is-bad .monitor-avail-track i { background: var(--operator-destructive-fill); }

.monitor-avail b {
  min-width: 3.5rem;
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  text-align: right;
}

.monitor-avail.is-none b {
  color: var(--operator-subtle-foreground);
}

.monitor-latency {
  font-variant-numeric: tabular-nums;
}

.monitor-latency small {
  margin-left: 0.125rem;
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.monitor-row-actions {
  opacity: 0;
  transition: opacity var(--operator-duration-fast) var(--operator-ease-out);
}

.monitor-table tr:hover .monitor-row-actions,
.monitor-table tr:focus-within .monitor-row-actions {
  opacity: 1;
}

@media (hover: none) {
  .monitor-row-actions {
    opacity: 1;
  }
}

@media (max-width: 760px) {
  .monitor-table .is-optional {
    display: none;
  }

  .monitor-col-name {
    min-width: 12rem;
  }

  .monitor-table th,
  .monitor-table td {
    padding: 0 0.625rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .monitor-avail-track i {
    animation: none;
  }
}
</style>
