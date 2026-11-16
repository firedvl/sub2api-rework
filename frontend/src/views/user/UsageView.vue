<template>
  <AppLayout>
    <div class="ui-page ui-stagger">
      <div class="ui-toolbar uv-toolbar" role="toolbar" :aria-label="t('usage.title')">
        <div v-if="errorViewEnabled" class="viz-seg" role="group" :aria-label="t('usage.title')">
          <button type="button" :aria-pressed="activeTab === 'usage'" data-testid="usage-tab-usage" @click="activeTab = 'usage'">
            {{ t('usage.tabs.usage') }}
          </button>
          <button type="button" :aria-pressed="activeTab === 'errors'" data-testid="usage-tab-errors" @click="switchToErrors">
            {{ t('usage.tabs.errors') }}
          </button>
        </div>
        <DateRangePicker
          v-model:start-date="startDate"
          v-model:end-date="endDate"
          @change="onDateRangeChange"
        />
        <template v-if="activeTab === 'errors'">
          <Select v-model="errorFilter.api_key_id" class="uv-filter" :aria-label="t('usage.errors.keyName')" :options="errorKeyOptions" @change="applyErrorFilters" />
          <Select
            v-model="errorFilter.model"
            class="uv-filter"
            :aria-label="t('usage.errors.model')"
            :options="errorModelOptions"
            searchable
            creatable
            clearable
            :placeholder="t('usage.errors.modelPlaceholder')"
            @change="applyErrorFilters"
          />
        </template>
        <template v-else>
          <Select v-model="filters.api_key_id" class="uv-filter" :aria-label="t('usage.apiKeyFilter')" :options="apiKeyOptions" @change="applyFilters" />
          <Select v-model="filters.model" class="uv-filter" :aria-label="t('usage.model')" :options="modelOptions" searchable @change="applyFilters" />
        </template>

        <div ref="filterPopoverRef" class="relative">
          <button
            type="button"
            class="btn btn-secondary"
            :aria-expanded="showFilterPopover"
            aria-controls="usage-more-filters"
            data-testid="usage-more-filters"
            @click="showFilterPopover = !showFilterPopover"
          >
            <Icon name="filter" size="sm" />
            <span>{{ t('personalUi.usage.filters') }}</span>
            <span v-if="extraFilterCount" class="uv-filter-count">{{ extraFilterCount }}</span>
          </button>
          <Transition name="menu-pop">
            <div
              v-show="showFilterPopover"
              id="usage-more-filters"
              class="dropdown uv-popover"
              role="group"
              :aria-label="t('personalUi.usage.filters')"
              @keydown.esc.stop="showFilterPopover = false"
            >
              <template v-if="activeTab === 'errors'">
                <label class="uv-field">
                  <span class="input-label">{{ t('usage.errors.category') }}</span>
                  <Select v-model="errorFilter.category" :options="errorCategoryOptions" @change="applyErrorFilters" />
                </label>
                <label class="uv-field">
                  <span class="input-label">{{ t('usage.errors.status') }}</span>
                  <Select v-model="errorFilter.status_code" :options="errorStatusOptions" @change="applyErrorFilters" />
                </label>
              </template>
              <template v-else>
                <label class="uv-field">
                  <span class="input-label">{{ t('admin.usage.group') }}</span>
                  <Select v-model="filters.group_id" :options="groupOptions" searchable @change="applyFilters" />
                </label>
                <label class="uv-field">
                  <span class="input-label">{{ t('usage.type') }}</span>
                  <Select v-model="filters.request_type" :options="requestTypeOptions" @change="applyFilters" />
                </label>
                <label class="uv-field">
                  <span class="input-label">{{ t('usage.compactionFilter') }}</span>
                  <Select v-model="filters.native_compaction_v2" :options="compactionOptions" @change="applyFilters" />
                </label>
                <label v-if="subscriptionFeatureEnabled" class="uv-field">
                  <span class="input-label">{{ t('admin.usage.billingType') }}</span>
                  <Select v-model="filters.billing_type" :options="billingTypeOptions" @change="applyFilters" />
                </label>
                <label class="uv-field">
                  <span class="input-label">{{ t('admin.usage.billingMode') }}</span>
                  <Select v-model="filters.billing_mode" :options="billingModeOptions" @change="applyFilters" />
                </label>
              </template>
              <div class="uv-popover-foot">
                <button type="button" class="btn btn-ghost btn-sm" @click="resetFilters">{{ t('common.reset') }}</button>
                <button type="button" class="btn btn-secondary btn-sm" @click="showFilterPopover = false">{{ t('common.close') }}</button>
              </div>
            </div>
          </Transition>
        </div>

        <span class="ui-grow"></span>
        <button
          type="button"
          class="app-icon-button"
          :disabled="activeTab === 'errors' ? errorLoading : loading"
          :aria-label="t('common.refresh')"
          :title="t('common.refresh')"
          @click="refreshData"
        >
          <Icon name="refresh" size="sm" :class="(activeTab === 'errors' ? errorLoading : loading) ? 'animate-spin' : ''" />
        </button>
        <OverflowMenu :label="t('admin.users.columnSettings')" icon="grid" data-testid="usage-column-settings">
          <button
            v-for="col in currentToggleableColumns"
            :key="col.key"
            type="button"
            class="dropdown-item justify-between"
            role="menuitemcheckbox"
            :aria-checked="isCurrentColumnVisible(col.key)"
            :data-testid="`usage-column-toggle-${col.key}`"
            @click="toggleCurrentColumn(col.key)"
          >
            <span>{{ col.label }}</span>
            <Icon v-if="isCurrentColumnVisible(col.key)" name="check" size="sm" class="text-primary-500" />
          </button>
        </OverflowMenu>
        <button v-if="activeTab !== 'errors'" type="button" class="btn btn-secondary" :disabled="exporting" @click="exportToCSV">
          <Icon name="download" size="sm" />
          <span>{{ exporting ? t('usage.exporting') : t('usage.exportCsv') }}</span>
        </button>
      </div>

      <TransitionGroup v-if="activeChips.length" tag="div" name="ui-list" class="ui-chips" :aria-label="t('personalUi.usage.activeFilters')">
        <button
          v-for="chip in activeChips"
          :key="chip.id"
          type="button"
          class="ui-chip is-active"
          :aria-label="`${t('personalUi.usage.removeFilter')}: ${chip.label}`"
          @click="clearChip(chip.id)"
        >
          <span class="text-gray-500">{{ chip.name }}</span>
          <span>{{ chip.label }}</span>
          <Icon name="x" size="xs" />
        </button>
        <button key="__reset" type="button" class="viz-link" @click="resetFilters">{{ t('common.reset') }}</button>
      </TransitionGroup>

      <div class="viz-kpis uv-kpis">
        <div class="viz-card viz-kpi">
          <span class="viz-kpi-label"><Icon name="chart" size="xs" />{{ t('usage.totalRequests') }}</span>
          <div class="viz-kpi-row">
            <span class="viz-kpi-value"><CountUp :value="usageStats?.total_requests ?? null" :format="formatInt" /></span>
            <SparkLine v-if="trendData.length > 1" :values="trendData.map((point) => point.requests)" />
          </div>
        </div>
        <div class="viz-card viz-kpi" :title="tokenSummary">
          <span class="viz-kpi-label"><Icon name="database" size="xs" />{{ t('usage.totalTokens') }}</span>
          <div class="viz-kpi-row">
            <span class="viz-kpi-value"><CountUp :value="usageStats?.total_tokens ?? null" :format="formatCompact" /></span>
            <SparkLine v-if="trendData.length > 1" :values="trendData.map((point) => point.total_tokens)" />
          </div>
          <div v-if="(usageStats?.total_tokens ?? 0) > 0" class="viz-share uv-share" role="img" :aria-label="tokenSummary">
            <i :style="{ flex: usageStats?.total_input_tokens || 0, background: 'var(--operator-series-1)' }"></i>
            <i :style="{ flex: usageStats?.total_output_tokens || 0, background: 'var(--operator-series-2)' }"></i>
            <i :style="{ flex: usageStats?.total_cache_tokens || 0, background: 'var(--operator-series-3)' }"></i>
          </div>
        </div>
        <div class="viz-card viz-kpi">
          <span class="viz-kpi-label">
            <Icon name="dollar" size="xs" />{{ t('usage.totalCost') }}
            <span class="uv-info" tabindex="0" :title="t('usage.observationCostDisclaimer')" :aria-label="t('usage.observationCostDisclaimer')">
              <Icon name="infoCircle" size="xs" />
            </span>
          </span>
          <div class="viz-kpi-row">
            <span class="viz-kpi-value"><CountUp :value="usageStats?.total_actual_cost ?? null" :format="formatMoney" /></span>
            <SparkLine v-if="trendData.length > 1" :values="trendData.map((point) => point.actual_cost)" />
          </div>
          <span v-if="usageStats" class="viz-delta" :title="t('usage.standardCost')"><s>{{ formatMoney(usageStats.total_cost) }}</s></span>
        </div>
        <div class="viz-card viz-kpi">
          <span class="viz-kpi-label"><Icon name="clock" size="xs" />{{ t('usage.avgDuration') }}</span>
          <span class="viz-kpi-value"><CountUp :value="usageStats?.average_duration_ms ?? null" :format="formatDuration" /></span>
        </div>
      </div>

      <div class="viz-grid">
        <section class="viz-card viz-span-7" aria-labelledby="uv-trend">
          <header class="viz-card-head">
            <h2 id="uv-trend" class="viz-card-title">{{ t('admin.dashboard.tokenUsageTrend') }}</h2>
            <div class="viz-card-tools">
              <div class="viz-seg" role="group" :aria-label="t('personalUi.dashboard.metric')">
                <button
                  v-for="item in trendMetrics"
                  :key="item.id"
                  type="button"
                  :aria-pressed="trendMetric === item.id"
                  @click="trendMetric = item.id"
                >{{ item.label }}</button>
              </div>
              <div class="viz-seg" role="group" :aria-label="t('admin.dashboard.granularity')">
                <button
                  v-for="option in granularityOptions"
                  :key="String(option.value)"
                  type="button"
                  :aria-pressed="granularity === option.value"
                  @click="setGranularity(option.value as 'day' | 'hour')"
                >{{ option.label }}</button>
              </div>
            </div>
          </header>
          <div class="viz-card-body">
            <div v-if="chartsLoading && !trendData.length" class="skeleton h-[220px] rounded-lg"></div>
            <div v-else-if="!trendData.length" class="viz-state">
              <Icon name="chart" size="lg" class="text-gray-400" />
              <span>{{ t('dashboard.noDataAvailable') }}</span>
            </div>
            <TrendChart
              v-else
              :labels="trendLabels"
              :tips="trendData.map((point) => point.date)"
              :series="[{ name: activeTrendMetric.label, color: 'var(--operator-accent-text)', values: trendData.map(activeTrendMetric.get) }]"
              :summary="activeTrendMetric.label"
              :height="220"
              :type="trendData.length <= 2 ? 'bar' : 'line'"
              :format-value="activeTrendMetric.format"
            />
          </div>
        </section>

        <div class="viz-span-5 uv-breakdown">
          <div class="viz-seg self-start" role="group" :aria-label="t('personalUi.usage.breakdown')">
            <button type="button" :aria-pressed="breakdown === 'models'" @click="breakdown = 'models'">{{ t('usage.model') }}</button>
            <button type="button" :aria-pressed="breakdown === 'groups'" @click="breakdown = 'groups'">{{ t('admin.usage.group') }}</button>
            <button type="button" :aria-pressed="breakdown === 'endpoints'" @click="breakdown = 'endpoints'">{{ t('usage.endpoint') }}</button>
          </div>
          <ModelDistributionChart
            v-show="breakdown === 'models'"
            v-model:metric="modelDistributionMetric"
            :model-stats="requestedModelStats"
            :loading="modelStatsLoading"
            :show-source-toggle="false"
            :show-metric-toggle="true"
            :enable-breakdown="false"
            :show-account-cost="false"
            :start-date="startDate"
            :end-date="endDate"
          />
          <GroupDistributionChart
            v-show="breakdown === 'groups'"
            v-model:metric="groupDistributionMetric"
            :group-stats="groupStats"
            :loading="chartsLoading"
            :show-metric-toggle="true"
            :enable-breakdown="false"
            :show-account-cost="false"
            :start-date="startDate"
            :end-date="endDate"
          />
          <EndpointDistributionChart
            v-show="breakdown === 'endpoints'"
            v-model:source="endpointDistributionSource"
            v-model:metric="endpointDistributionMetric"
            :endpoint-stats="inboundEndpointStats"
            :upstream-endpoint-stats="upstreamEndpointStats"
            :endpoint-path-stats="endpointPathStats"
            :loading="endpointStatsLoading"
            :show-source-toggle="false"
            :show-metric-toggle="true"
            :enable-breakdown="false"
            :title="t('usage.endpointDistribution')"
            :start-date="startDate"
            :end-date="endDate"
          />
        </div>
      </div>

      <template v-if="activeTab === 'usage'">
        <section
          class="ui-card uv-table"
          :aria-label="t('usage.title')"
          @click="onTableClick"
          @keydown.enter="onTableKeydown"
        >
          <UsageTable
            flat
            :data="usageLogs"
            :loading="loading"
            :columns="visibleColumns"
            :server-side-sort="true"
            :show-account-billing="false"
            :show-upstream-endpoint="false"
            default-sort-key="created_at"
            default-sort-order="desc"
            @sort="handleSort"
            @ipGeoBatchFailed="handleIpGeoBatchFailed"
          />
        </section>

        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>

      <UserErrorRequestsTable
        v-else-if="errorViewEnabled"
        :rows="errorRows"
        :total="errorTotal"
        :loading="errorLoading"
        :page="errorPage"
        :page-size="errorPageSize"
        :visible-column-keys="errVisibleColumnKeys"
        @sort="onErrorSort"
        @update:page="onErrorPage"
        @update:pageSize="onErrorPageSize"
        @ipGeoBatchFailed="handleIpGeoBatchFailed"
      />
    </div>

    <UserUsageDrawer :open="detailRecord !== null" :record="detailRecord" @close="detailRecord = null" />
  </AppLayout>

</template>

<script setup lang="ts">
import { usageObservationKey } from '@/utils/usageObservation'
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { FeatureFlags, resolveFeatureFlag } from '@/utils/featureFlags'
import { keysAPI, usageAPI, userGroupsAPI } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import UsageTable from '@/components/admin/usage/UsageTable.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import GroupDistributionChart from '@/components/charts/GroupDistributionChart.vue'
import EndpointDistributionChart from '@/components/charts/EndpointDistributionChart.vue'
import OverflowMenu from '@/components/keys/OverflowMenu.vue'
import CountUp from '@/components/ui/CountUp.vue'
import SparkLine from '@/components/viz/SparkLine.vue'
import TrendChart from '@/components/viz/TrendChart.vue'
import UserUsageDrawer from '@/components/user/usage/UserUsageDrawer.vue'
import { formatCompact, formatDuration, formatInt } from '@/utils/vizFormat'
import Icon from '@/components/icons/Icon.vue'
import UserErrorRequestsTable from '@/components/user/UserErrorRequestsTable.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatReasoningEffort } from '@/utils/format'
import { getBillingModeLabel, getDisplayBillingMode as resolveDisplayBillingMode } from '@/utils/billingMode'
import { resolveUsageRequestType, requestTypeToLegacyStream } from '@/utils/usageRequestType'
import type {
  ApiKey,
  EndpointStat,
  Group,
  GroupStat,
  ModelStat,
  TrendDataPoint,
  UsageLog,
  UsageQueryParams,
  UsageStatsResponse,
  UserErrorRequest,
} from '@/types'
import type { Column } from '@/components/common/types'
import { COMMON_ERROR_STATUS_CODES } from '@/utils/errorBadges'

const { t } = useI18n()
const appStore = useAppStore()

type DistributionMetric = 'tokens' | 'actual_cost'
type EndpointSource = 'inbound' | 'upstream' | 'path'

const usageStats = ref<UsageStatsResponse | null>(null)
const usageLogs = ref<UsageLog[]>([])
const trendData = ref<TrendDataPoint[]>([])
const requestedModelStats = ref<ModelStat[]>([])
const groupStats = ref<GroupStat[]>([])
const inboundEndpointStats = ref<EndpointStat[]>([])
const upstreamEndpointStats = ref<EndpointStat[]>([])
const endpointPathStats = ref<EndpointStat[]>([])

const loading = ref(false)
const chartsLoading = ref(false)
const modelStatsLoading = ref(false)
const endpointStatsLoading = ref(false)
const exporting = ref(false)
const errorRows = ref<UserErrorRequest[]>([])
const errorLoading = ref(false)
const errorPage = ref(1)
const errorPageSize = ref(20)
const errorSortBy = ref('created_at')
const errorSortOrder = ref<'asc' | 'desc'>('desc')
const errorTotal = ref(0)
const errorFilter = ref<{ model: string | null; category: string; api_key_id: number | null; status_code: number | null }>({
  model: '',
  category: '',
  api_key_id: null,
  status_code: null,
})

const errorKeyOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.errors.allKeys') },
  ...apiKeys.value.map((k) => ({ value: k.id, label: k.name })),
])

// 模型候选取自当前已加载错误中出现过的模型；creatable 允许输入任意片段做后端模糊。
const errorModelOptions = computed<SelectOption[]>(() => {
  const seen = new Set<string>()
  const opts: SelectOption[] = []
  for (const r of errorRows.value) {
    if (r.model && !seen.has(r.model)) {
      seen.add(r.model)
      opts.push({ value: r.model, label: r.model })
    }
  }
  return opts
})

const errorCategoryCodes = ['auth', 'rate_limit', 'quota', 'invalid_request', 'service_unavailable', 'upstream', 'internal', 'cyber']

const errorCategoryOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('usage.errors.allCategories') },
  ...errorCategoryCodes.map((c) => ({ value: c, label: t('usage.errors.categories.' + c) })),
])

// 状态码候选用固定常用列表(与管理端 UsageFilters 共用常量),不受当前页数据限制:
// 后端 status_code 过滤对全量生效,若只列当前页出现过的码,用户就选不到仅在后续页的码。
const errorStatusOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.errors.allStatuses') },
  ...COMMON_ERROR_STATUS_CODES.map((c) => ({ value: c, label: String(c) })),
])

const applyErrorFilters = () => {
  errorPage.value = 1
  void loadErrors()
}

let abortController: AbortController | null = null
let chartReqSeq = 0
let statsReqSeq = 0
let modelStatsReqSeq = 0

const formatLocalDate = (date: Date): string =>
  `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`

const getLast24HoursRangeDates = () => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return { start: formatLocalDate(start), end: formatLocalDate(end) }
}

const getGranularityForRange = (start: string, end: string): 'day' | 'hour' => {
  const startTime = new Date(`${start}T00:00:00`).getTime()
  const endTime = new Date(`${end}T00:00:00`).getTime()
  return Math.ceil((endTime - startTime) / (1000 * 60 * 60 * 24)) <= 1 ? 'hour' : 'day'
}

const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)
const granularity = ref<'day' | 'hour'>(getGranularityForRange(startDate.value, endDate.value))

const modelDistributionMetric = ref<DistributionMetric>('tokens')
const groupDistributionMetric = ref<DistributionMetric>('tokens')
const endpointDistributionMetric = ref<DistributionMetric>('tokens')
const endpointDistributionSource = ref<EndpointSource>('inbound')
const activeTab = ref<'usage' | 'errors'>('usage')
const errorViewEnabled = computed(() => appStore.cachedPublicSettings?.allow_user_view_error_requests ?? false)

const filters = ref<UsageQueryParams>({
  start_date: startDate.value,
  end_date: endDate.value,
  request_type: undefined,
  native_compaction_v2: null,
  billing_type: null,
  billing_mode: null,
})

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
})
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc',
})

const granularityOptions = computed<SelectOption[]>(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') },
])
const requestTypeOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allTypes') },
  { value: 'ws_v2', label: t('usage.ws') },
  { value: 'live', label: t('usage.live') },
  { value: 'stream', label: t('usage.stream') },
  { value: 'sync', label: t('usage.sync') },
])
const compactionOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.allCompactionTypes') },
  { value: true, label: t('usage.compactionOnly') },
])
// 订阅功能关闭后只剩余额计费，「计费类型」筛选（余额/订阅）失去意义，整块隐藏。
const subscriptionFeatureEnabled = computed(() => resolveFeatureFlag(appStore.cachedPublicSettings, FeatureFlags.subscription))
const billingTypeOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allBillingTypes') },
  { value: 0, label: t('admin.usage.billingTypeBalance') },
  { value: 1, label: t('admin.usage.billingTypeSubscription') },
])
const billingModeOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allBillingModes') },
  { value: 'token', label: t('admin.usage.billingModeToken') },
  { value: 'per_request', label: t('admin.usage.billingModePerRequest') },
  { value: 'image', label: t('admin.usage.billingModeImage') },
  { value: 'video', label: t('admin.usage.billingModeVideo') },
])

const apiKeys = ref<ApiKey[]>([])
const groups = ref<Group[]>([])
const modelOptionValues = ref<string[]>([])

const apiKeyOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.allApiKeys') },
  ...apiKeys.value.map((key) => ({ value: key.id, label: key.name })),
])
const groupOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allGroups') },
  ...groups.value.map((group) => ({ value: group.id, label: group.name })),
])
const modelOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allModels') },
  ...modelOptionValues.value.map((model) => ({ value: model, label: model })),
])

const normalizedFilters = computed<UsageQueryParams>(() => {
  const requestType = filters.value.request_type
  const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
  return {
    ...filters.value,
    start_date: startDate.value,
    end_date: endDate.value,
    stream: legacyStream === null ? undefined : legacyStream,
  }
})

const buildUsageListParams = (page: number, pageSize: number): UsageQueryParams => ({
  page,
  page_size: pageSize,
  ...normalizedFilters.value,
  sort_by: sortState.sort_by,
  sort_order: sortState.sort_order,
})

const loadLogs = async () => {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  loading.value = true
  try {
    const res = await usageAPI.query(buildUsageListParams(pagination.page, pagination.page_size), {
      signal: controller.signal,
    })
    if (!controller.signal.aborted) {
      usageLogs.value = res.items
      pagination.total = res.total
    }
  } catch (error: any) {
    if (error?.name !== 'AbortError' && error?.code !== 'ERR_CANCELED') {
      appStore.showError(t('usage.failedToLoad'))
    }
  } finally {
    if (abortController === controller) loading.value = false
  }
}

const loadStats = async () => {
  const seq = ++statsReqSeq
  endpointStatsLoading.value = true
  try {
    const stats = await usageAPI.getStats(normalizedFilters.value)
    if (seq !== statsReqSeq) return
    usageStats.value = stats
    inboundEndpointStats.value = stats.endpoints || []
    upstreamEndpointStats.value = []
    endpointPathStats.value = []
  } catch (error) {
    if (seq !== statsReqSeq) return
    console.error('Failed to load usage stats:', error)
    inboundEndpointStats.value = []
    upstreamEndpointStats.value = []
    endpointPathStats.value = []
  } finally {
    if (seq === statsReqSeq) endpointStatsLoading.value = false
  }
}

const loadModelStats = async () => {
  const seq = ++modelStatsReqSeq
  modelStatsLoading.value = true
  try {
    const response = await usageAPI.getDashboardModels({
      ...normalizedFilters.value,
      model_source: 'requested',
    })
    if (seq !== modelStatsReqSeq) return
    requestedModelStats.value = response.models || []
    refreshModelOptions(response.models || [])
  } catch (error) {
    if (seq !== modelStatsReqSeq) return
    console.error('Failed to load model stats:', error)
    requestedModelStats.value = []
  } finally {
    if (seq === modelStatsReqSeq) modelStatsLoading.value = false
  }
}

const loadChartData = async () => {
  const seq = ++chartReqSeq
  chartsLoading.value = true
  try {
    const snapshot = await usageAPI.getDashboardSnapshotV2({
      ...normalizedFilters.value,
      granularity: granularity.value,
      include_trend: true,
      include_model_stats: false,
      include_group_stats: true,
    })
    if (seq !== chartReqSeq) return
    trendData.value = snapshot.trend || []
    groupStats.value = snapshot.groups || []
  } catch (error) {
    if (seq !== chartReqSeq) return
    console.error('Failed to load chart data:', error)
    trendData.value = []
    groupStats.value = []
  } finally {
    if (seq === chartReqSeq) chartsLoading.value = false
  }
}

const refreshModelOptions = (models: ModelStat[]) => {
  const current = filters.value.model
  const set = new Set(modelOptionValues.value)
  models.forEach((item) => {
    if (item.model) set.add(item.model)
  })
  if (current) set.add(current)
  modelOptionValues.value = Array.from(set).sort()
}

const applyFilters = () => {
  pagination.page = 1
  void loadLogs()
  void loadStats()
  void loadModelStats()
  void loadChartData()
  resetErrorRows()
}

watch(subscriptionFeatureEnabled, (enabled) => {
  if (!enabled && filters.value.billing_type != null) {
    filters.value.billing_type = null
    applyFilters()
  }
})

const refreshData = () => {
  void loadLogs()
  void loadStats()
  void loadModelStats()
  void loadChartData()
  if (activeTab.value === 'errors') void loadErrors()
}

const resetFilters = () => {
  const range = getLast24HoursRangeDates()
  startDate.value = range.start
  endDate.value = range.end
  filters.value = {
    start_date: range.start,
    end_date: range.end,
    request_type: undefined,
    native_compaction_v2: null,
    billing_type: null,
    billing_mode: null,
  }
  granularity.value = getGranularityForRange(range.start, range.end)
  applyFilters()
  if (activeTab.value === 'errors') {
    errorFilter.value = { model: '', category: '', api_key_id: null, status_code: null }
    applyErrorFilters()
  }
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  filters.value.start_date = range.startDate
  filters.value.end_date = range.endDate
  granularity.value = getGranularityForRange(range.startDate, range.endDate)
  applyFilters()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  void loadLogs()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  void loadLogs()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  void loadLogs()
}

const handleIpGeoBatchFailed = () => {
  appStore.showError(t('usage.ipGeo.batchFailed'))
}

const getRequestTypeExportText = (log: UsageLog): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'cyber') return 'Cyber'
  if (requestType === 'live') return 'Live'
  if (requestType === 'ws_v2') return 'WS'
  if (requestType === 'stream') return 'Stream'
  if (requestType === 'sync') return 'Sync'
  return 'Unknown'
}

const getDisplayBillingMode = (
  row: Pick<UsageLog, 'billing_mode' | 'image_count'> | null | undefined
): string | null | undefined => resolveDisplayBillingMode(row)

const escapeCSVValue = (value: unknown): string => {
  if (value == null) return ''
  const str = String(value)
  const escaped = str.replace(/"/g, '""')
  if (str === '-') return str
  if (/^[=+\-@\t\r]/.test(str)) return `"\'${escaped}"`
  if (/[,"\n\r]/.test(str)) return `"${escaped}"`
  return str
}

const exportToCSV = async () => {
  if (pagination.total === 0) {
    appStore.showWarning(t('usage.noDataToExport'))
    return
  }
  exporting.value = true
  appStore.showInfo(t('usage.preparingExport'))
  try {
    const allLogs: UsageLog[] = []
    const pageSize = 100
    const exportParams = buildUsageListParams(1, pageSize)
    const totalPages = Math.ceil(pagination.total / pageSize)
    for (let page = 1; page <= totalPages; page++) {
      const response = await usageAPI.query({ ...exportParams, page })
      allLogs.push(...response.items)
    }
    if (allLogs.length === 0) {
      appStore.showWarning(t('usage.noDataToExport'))
      return
    }
    const headers = [
      'Time',
      'API Key Name',
      'Model',
      'Reasoning Effort',
      'Inbound Endpoint',
      'IP Address',
      'Type',
      'Billing Mode',
      'Input Tokens',
      'Output Tokens',
      'Cache Read Tokens',
      'Cache Creation Tokens',
      'Rate Multiplier',
      'Billed Cost',
      'Original Cost',
      'First Token (ms)',
      'Duration (ms)',
      t('usage.observationCompleteness'),
    ]
    const rows = allLogs.map((log) => [
      log.created_at,
      log.api_key?.name || '',
      log.model,
      formatReasoningEffort(log.reasoning_effort),
      log.inbound_endpoint || '',
      log.ip_address || '',
      getRequestTypeExportText(log),
      getBillingModeLabel(getDisplayBillingMode(log), t),
      log.input_tokens,
      log.output_tokens,
      log.cache_read_tokens,
      log.cache_creation_tokens,
      log.rate_multiplier,
      log.actual_cost.toFixed(8),
      log.total_cost.toFixed(8),
      log.first_token_ms ?? '',
      log.duration_ms ?? '',
      t(usageObservationKey(log.usage_observation_complete)),
    ].map(escapeCSVValue))
    const csvContent = [
      headers.map(escapeCSVValue).join(','),
      ...rows.map((row) => row.join(',')),
    ].join('\n')
    const blob = new Blob(['\uFEFF' + csvContent], { type: 'text/csv;charset=utf-8;' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `usage_${exportParams.start_date}_to_${exportParams.end_date}.csv`
    link.click()
    window.URL.revokeObjectURL(url)
    appStore.showSuccess(t('usage.exportSuccess'))
  } catch (error) {
    console.error('CSV Export failed:', error)
    appStore.showError(t('usage.exportFailed'))
  } finally {
    exporting.value = false
  }
}

const ALWAYS_VISIBLE = ['created_at']
const DEFAULT_HIDDEN_COLUMNS = ['user_agent']
const HIDDEN_COLUMNS_KEY = 'user-usage-hidden-columns'

const allColumns = computed<Column[]>(() => [
  { key: 'api_key', label: t('usage.apiKeyFilter'), sortable: false },
  { key: 'model', label: t('usage.model'), sortable: true },
  { key: 'reasoning_effort', label: t('usage.reasoningEffort'), sortable: false },
  { key: 'endpoint', label: t('usage.endpoint'), sortable: false },
  { key: 'ip_address', label: 'IP', sortable: false },
  { key: 'group', label: t('admin.usage.group'), sortable: false },
  { key: 'stream', label: t('usage.type'), sortable: false },
  { key: 'billing_mode', label: t('admin.usage.billingMode'), sortable: false },
  { key: 'tokens', label: t('usage.tokens'), sortable: false },
  { key: 'cost', label: t('usage.cost'), sortable: false },
  { key: 'latency', label: t('usage.latency'), sortable: false },
  { key: 'created_at', label: t('usage.time'), sortable: true },
  { key: 'user_agent', label: t('usage.userAgent'), sortable: false },
])

const hiddenColumns = reactive<Set<string>>(new Set())
const toggleableColumns = computed(() => allColumns.value.filter((col) => !ALWAYS_VISIBLE.includes(col.key)))
const visibleColumns = computed(() =>
  allColumns.value.filter((col) => ALWAYS_VISIBLE.includes(col.key) || !hiddenColumns.has(col.key))
)
const isColumnVisible = (key: string) => !hiddenColumns.has(key)
const toggleColumn = (key: string) => {
  if (hiddenColumns.has(key)) hiddenColumns.delete(key)
  else hiddenColumns.add(key)
  localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
}
const loadSavedColumns = () => {
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    const values = saved ? JSON.parse(saved) as string[] : DEFAULT_HIDDEN_COLUMNS
    values.forEach((key) => hiddenColumns.add(key))
  } catch {
    DEFAULT_HIDDEN_COLUMNS.forEach((key) => hiddenColumns.add(key))
  }
}

// 错误请求 tab 独立列设置(机制同用量列设置,存储互不影响)
const ERR_ALWAYS_VISIBLE = ['status', 'created_at']
const ERR_DEFAULT_HIDDEN_COLUMNS = ['user_agent']
const ERR_HIDDEN_COLUMNS_KEY = 'user-usage-error-hidden-columns'

// key 须与 UserErrorRequestsTable 的 allColumns 一致
const errAllColumns = computed<Column[]>(() => [
  { key: 'key_name', label: t('usage.errors.keyName') },
  { key: 'model', label: t('usage.errors.model') },
  { key: 'endpoint', label: t('usage.errors.endpoint') },
  { key: 'client_ip', label: 'IP' },
  { key: 'group', label: t('admin.usage.group') },
  { key: 'type', label: t('usage.type') },
  { key: 'platform', label: t('usage.errors.platform') },
  { key: 'category', label: t('usage.errors.category') },
  { key: 'status', label: t('usage.errors.status') },
  { key: 'message', label: t('usage.errors.message') },
  { key: 'created_at', label: t('usage.errors.time') },
  { key: 'user_agent', label: t('usage.userAgent') },
])

const errHiddenColumns = reactive<Set<string>>(new Set())
const errToggleableColumns = computed(() =>
  errAllColumns.value.filter((col) => !ERR_ALWAYS_VISIBLE.includes(col.key))
)
const errVisibleColumnKeys = computed(() =>
  errAllColumns.value
    .filter((col) => ERR_ALWAYS_VISIBLE.includes(col.key) || !errHiddenColumns.has(col.key))
    .map((col) => col.key)
)
const isErrColumnVisible = (key: string) => !errHiddenColumns.has(key)
const toggleErrColumn = (key: string) => {
  if (errHiddenColumns.has(key)) errHiddenColumns.delete(key)
  else errHiddenColumns.add(key)
  localStorage.setItem(ERR_HIDDEN_COLUMNS_KEY, JSON.stringify([...errHiddenColumns]))
}
const loadSavedErrColumns = () => {
  try {
    const saved = localStorage.getItem(ERR_HIDDEN_COLUMNS_KEY)
    const values = saved ? (JSON.parse(saved) as string[]) : ERR_DEFAULT_HIDDEN_COLUMNS
    values.forEach((key) => errHiddenColumns.add(key))
  } catch {
    ERR_DEFAULT_HIDDEN_COLUMNS.forEach((key) => errHiddenColumns.add(key))
  }
}

// 列设置下拉按当前 tab 分发
const currentToggleableColumns = computed(() =>
  activeTab.value === 'errors' ? errToggleableColumns.value : toggleableColumns.value
)
const isCurrentColumnVisible = (key: string) =>
  activeTab.value === 'errors' ? isErrColumnVisible(key) : isColumnVisible(key)
const toggleCurrentColumn = (key: string) => {
  if (activeTab.value === 'errors') toggleErrColumn(key)
  else toggleColumn(key)
}

const showFilterPopover = ref(false)
const filterPopoverRef = ref<HTMLElement | null>(null)
// Select menus render in a body portal, so clicks inside them must not close the popover.
const handleFilterClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (!showFilterPopover.value || filterPopoverRef.value?.contains(target)) return
  if (target.closest?.('.select-dropdown-portal')) return
  showFilterPopover.value = false
}

const formatMoney = (value: number) => `$${(value ?? 0).toFixed((value ?? 0) >= 100 ? 2 : 4)}`

const tokenSummary = computed(() => {
  const stats = usageStats.value
  if (!stats) return ''
  return `${t('usage.in')} ${formatCompact(stats.total_input_tokens)} · ${t('usage.out')} ${formatCompact(stats.total_output_tokens)} · ${t('usage.cacheTotal')} ${formatCompact(stats.total_cache_tokens)}`
})

type TrendMetric = 'tokens' | 'cost' | 'requests'
const trendMetric = ref<TrendMetric>('tokens')
const trendMetrics = computed(() => [
  { id: 'tokens' as const, label: t('usage.tokens'), get: (point: TrendDataPoint) => point.total_tokens, format: formatCompact },
  { id: 'cost' as const, label: t('usage.cost'), get: (point: TrendDataPoint) => point.actual_cost, format: formatMoney },
  { id: 'requests' as const, label: t('dashboard.requests'), get: (point: TrendDataPoint) => point.requests, format: formatInt },
])
const activeTrendMetric = computed(() => trendMetrics.value.find((item) => item.id === trendMetric.value) ?? trendMetrics.value[0])
const trendLabels = computed(() => trendData.value.map((point) => {
  const [day, time] = point.date.split(/[ T]/)
  return granularity.value === 'hour' && time ? time.slice(0, 5) : day.slice(5)
}))
const setGranularity = (value: 'day' | 'hour') => {
  if (granularity.value === value) return
  granularity.value = value
  void loadChartData()
}

const breakdown = ref<'models' | 'groups' | 'endpoints'>('models')

// Extra filters live in the popover; chips summarize every active filter.
const optionLabel = (options: SelectOption[], value: unknown) =>
  String(options.find((option) => option.value === value)?.label ?? value)

const activeChips = computed(() => {
  const chips: Array<{ id: string; name: string; label: string }> = []
  if (activeTab.value === 'errors') {
    const f = errorFilter.value
    if (f.api_key_id != null) chips.push({ id: 'e:key', name: t('usage.errors.keyName'), label: optionLabel(errorKeyOptions.value, f.api_key_id) })
    if (f.model) chips.push({ id: 'e:model', name: t('usage.errors.model'), label: f.model })
    if (f.category) chips.push({ id: 'e:category', name: t('usage.errors.category'), label: optionLabel(errorCategoryOptions.value, f.category) })
    if (f.status_code != null) chips.push({ id: 'e:status', name: t('usage.errors.status'), label: String(f.status_code) })
    return chips
  }
  const f = filters.value
  if (f.api_key_id != null) chips.push({ id: 'api_key_id', name: t('usage.apiKeyFilter'), label: optionLabel(apiKeyOptions.value, f.api_key_id) })
  if (f.model) chips.push({ id: 'model', name: t('usage.model'), label: f.model })
  if (f.group_id != null) chips.push({ id: 'group_id', name: t('admin.usage.group'), label: optionLabel(groupOptions.value, f.group_id) })
  if (f.request_type) chips.push({ id: 'request_type', name: t('usage.type'), label: optionLabel(requestTypeOptions.value, f.request_type) })
  if (f.native_compaction_v2 != null) chips.push({ id: 'native_compaction_v2', name: t('usage.compactionFilter'), label: optionLabel(compactionOptions.value, f.native_compaction_v2) })
  if (f.billing_type != null && subscriptionFeatureEnabled.value) chips.push({ id: 'billing_type', name: t('admin.usage.billingType'), label: optionLabel(billingTypeOptions.value, f.billing_type) })
  if (f.billing_mode) chips.push({ id: 'billing_mode', name: t('admin.usage.billingMode'), label: optionLabel(billingModeOptions.value, f.billing_mode) })
  return chips
})

const extraFilterCount = computed(() => activeChips.value.filter((chip) =>
  !['api_key_id', 'model', 'e:key', 'e:model'].includes(chip.id)).length)

const clearChip = (id: string) => {
  if (id.startsWith('e:')) {
    const field = id.slice(2)
    if (field === 'key') errorFilter.value.api_key_id = null
    else if (field === 'model') errorFilter.value.model = ''
    else if (field === 'category') errorFilter.value.category = ''
    else if (field === 'status') errorFilter.value.status_code = null
    applyErrorFilters()
    return
  }
  const cleared: Partial<UsageQueryParams> = {
    api_key_id: undefined,
    model: undefined,
    group_id: undefined,
    request_type: undefined,
    native_compaction_v2: null,
    billing_type: null,
    billing_mode: null,
  }
  filters.value = { ...filters.value, [id]: cleared[id as keyof UsageQueryParams] }
  applyFilters()
}

// UsageTable has no row event; desktop rows carry data-index, mobile cards keep data order.
const detailRecord = ref<UsageLog | null>(null)
const recordFromEvent = (event: Event): UsageLog | null => {
  const target = event.target as HTMLElement
  if (target.closest('button, a, input, select, textarea, [role="button"]')) return null
  const row = target.closest<HTMLElement>('tr.data-row[data-index]')
  if (row) return usageLogs.value[Number(row.dataset.index)] ?? null
  const card = target.closest<HTMLElement>('.data-card')
  if (!card?.parentElement) return null
  const cards = [...card.parentElement.querySelectorAll(':scope > .data-card')]
  return usageLogs.value[cards.indexOf(card)] ?? null
}
const onTableClick = (event: MouseEvent) => {
  const record = recordFromEvent(event)
  if (record) detailRecord.value = record
}
const onTableKeydown = (event: KeyboardEvent) => {
  const record = recordFromEvent(event)
  if (record) detailRecord.value = record
}
// Rows become focusable so the drawer is reachable from the keyboard.
watch([usageLogs, loading], async () => {
  await nextTick()
  document.querySelectorAll<HTMLElement>('.uv-table tr.data-row').forEach((row) => {
    row.tabIndex = 0
  })
})

const loadApiKeys = async () => {
  const firstPage = await keysAPI.list(1, 100)
  const keys = [...firstPage.items]
  for (let page = 2; page <= firstPage.pages && keys.length > 0; page++) {
    const response = await keysAPI.list(page, 100)
    if (response.items.length === 0) break
    keys.push(...response.items)
  }
  return keys
}

const loadFilterOptions = async () => {
  try {
    const [keys, availableGroups] = await Promise.all([
      loadApiKeys(),
      userGroupsAPI.getAvailable(),
    ])
    apiKeys.value = keys
    groups.value = availableGroups
  } catch (error) {
    console.error('Failed to load usage filter options:', error)
  }
}

const resetErrorRows = () => {
  errorPage.value = 1
  if (activeTab.value === 'errors') {
    void loadErrors()
  } else {
    errorRows.value = []
    errorTotal.value = 0
  }
}

const loadErrors = async () => {
  errorLoading.value = true
  try {
    const resp = await usageAPI.listMyErrorRequests({
      page: errorPage.value,
      page_size: errorPageSize.value,
      start_date: startDate.value,
      end_date: endDate.value,
      model: (errorFilter.value.model ?? '').trim() || undefined,
      category: errorFilter.value.category || undefined,
      api_key_id: errorFilter.value.api_key_id ?? undefined,
      status_code: errorFilter.value.status_code ?? undefined,
      sort_by: errorSortBy.value,
      sort_order: errorSortOrder.value,
    })
    errorRows.value = resp.items
    errorTotal.value = resp.total
  } catch (error) {
    console.error('[UsageView] loadErrors failed:', error)
    appStore.showError(t('usage.errors.failedToLoad'))
  } finally {
    errorLoading.value = false
  }
}

const onErrorSort = (sortBy: string, sortOrder: 'asc' | 'desc') => {
  errorSortBy.value = sortBy
  errorSortOrder.value = sortOrder
  errorPage.value = 1
  void loadErrors()
}

const onErrorPage = (page: number) => {
  errorPage.value = page
  void loadErrors()
}

const onErrorPageSize = (pageSize: number) => {
  errorPageSize.value = pageSize
  errorPage.value = 1
  void loadErrors()
}

const switchToErrors = () => {
  activeTab.value = 'errors'
  if (errorRows.value.length === 0) void loadErrors()
}

onMounted(() => {
  loadSavedColumns()
  loadSavedErrColumns()
  document.addEventListener('click', handleFilterClickOutside)
  void loadFilterOptions()
  refreshData()
})

onUnmounted(() => {
  abortController?.abort()
  document.removeEventListener('click', handleFilterClickOutside)
})

watch(endpointDistributionSource, () => {
  // Endpoint source switching is handled by the chart component using already loaded stats.
})
</script>

<style scoped>
/* The stagger animation makes each page section its own stacking context; lift the
   toolbar so its Filters popover and dropdowns paint above the sections below it. */
.uv-toolbar {
  position: relative;
  z-index: 10;
}

.uv-filter {
  width: 11rem;
}

.uv-filter-count {
  display: inline-grid;
  min-width: 1.125rem;
  height: 1.125rem;
  place-items: center;
  padding: 0 0.25rem;
  border-radius: 999px;
  background: var(--operator-accent);
  color: var(--operator-accent-foreground);
  font-size: 0.6875rem;
  font-weight: 600;
}

.uv-popover {
  top: calc(100% + 0.375rem);
  left: 0;
  display: grid;
  width: min(20rem, calc(100vw - 2rem));
  gap: 0.75rem;
  padding: 0.875rem;
}

.uv-field {
  display: block;
}

.uv-popover-foot {
  display: flex;
  justify-content: flex-end;
  gap: 0.5rem;
  padding-top: 0.25rem;
  border-top: 1px solid var(--operator-border);
}

.uv-kpis {
  margin-bottom: 0;
}

.uv-share {
  height: 0.3125rem;
}

.uv-info {
  display: inline-flex;
  border-radius: 999px;
  color: var(--operator-subtle-foreground);
  cursor: help;
}

.uv-info:focus-visible {
  outline: 2px solid rgb(var(--operator-accent-rgb) / 0.7);
}

.uv-breakdown {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.5rem;
}

.uv-table {
  overflow: hidden;
}

.uv-table :deep(tr.data-row) {
  cursor: pointer;
}

.uv-table :deep(tr.data-row:focus-visible) {
  outline: 2px solid rgb(var(--operator-accent-rgb) / 0.7);
  outline-offset: -2px;
}

.uv-table :deep(.data-card) {
  cursor: pointer;
}

@media (max-width: 640px) {
  .uv-filter {
    width: auto;
    flex: 1 1 10rem;
  }
}
</style>
