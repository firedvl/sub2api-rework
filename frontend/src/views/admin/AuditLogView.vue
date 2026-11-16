<template>
  <AppLayout>
    <div class="ui-page ui-stagger">
      <section class="ui-card audit-card">
        <div class="audit-toolbar">
          <div class="ui-toolbar" role="toolbar" :aria-label="t('admin.activityUi.filters')">
            <div class="audit-range">
              <Select
                :model-value="timeRange"
                :options="timeRangeOptions"
                :aria-label="t('admin.dashboard.timeRange')"
                @update:model-value="handleTimeRangeChange"
              />
            </div>
            <label class="ui-search audit-search">
              <Icon name="search" size="sm" />
              <input
                v-model.trim="filters.q"
                type="search"
                :aria-label="t('admin.audit.filters.q')"
                :placeholder="t('admin.audit.filters.qPlaceholder')"
                @keyup.enter="search"
                @search="search"
              />
            </label>
            <div class="viz-seg" role="group" :aria-label="t('admin.audit.filters.result')">
              <button
                v-for="option in resultOptions"
                :key="option.value"
                type="button"
                :aria-pressed="filters.success === option.value"
                @click="setResult(option.value)"
              >{{ option.label }}</button>
            </div>
            <button
              type="button"
              class="btn btn-secondary btn-sm audit-filter-toggle"
              data-testid="audit-filters-toggle"
              :aria-expanded="panelOpen"
              aria-controls="audit-filter-panel"
              @click="panelOpen = !panelOpen"
            >
              <Icon name="filter" size="sm" />
              <span>{{ t('admin.activityUi.filters') }}</span>
              <span v-if="panelCount" class="audit-filter-count">{{ panelCount }}</span>
            </button>
            <span class="ui-grow"></span>
            <button
              type="button"
              class="app-icon-button"
              :aria-label="t('common.refresh')"
              :title="t('common.refresh')"
              :disabled="loading"
              @click="fetchLogs"
            >
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
            </button>
            <div ref="moreMenuRef" class="relative">
              <button
                type="button"
                class="app-icon-button"
                data-testid="audit-more-actions"
                aria-haspopup="menu"
                :aria-expanded="moreOpen"
                :aria-label="t('admin.activityUi.moreActions')"
                :title="t('admin.activityUi.moreActions')"
                @click="moreOpen = !moreOpen"
              >
                <Icon name="more" size="sm" />
              </button>
              <Transition name="menu-pop">
                <div v-if="moreOpen" class="operator-menu absolute right-0 top-full z-50 mt-1 w-52 py-1" role="menu" @keydown.esc.stop="moreOpen = false">
                  <button type="button" role="menuitem" class="operator-menu-item audit-menu-item" :disabled="loading" @click="moreOpen = false; resetFilters()">
                    <Icon name="sync" size="sm" />{{ t('common.reset') }}
                  </button>
                  <div class="operator-menu-divider my-1 h-px"></div>
                  <button type="button" role="menuitem" class="operator-menu-item operator-menu-item-destructive audit-menu-item" @click="moreOpen = false; openClearDialog()">
                    <Icon name="trash" size="sm" />{{ t('admin.audit.clearAll') }}
                  </button>
                </div>
              </Transition>
            </div>
          </div>

          <Transition name="audit-panel">
            <form v-if="panelOpen" id="audit-filter-panel" class="audit-filter-panel" @submit.prevent="search">
              <div>
                <label class="input-label" for="audit-actor">{{ t('admin.audit.filters.actorEmail') }}</label>
                <input id="audit-actor" v-model.trim="filters.actor_email" type="text" class="input" />
              </div>
              <div>
                <label class="input-label" for="audit-action">{{ t('admin.audit.filters.action') }}</label>
                <input id="audit-action" v-model.trim="filters.action" type="text" class="input" />
              </div>
              <div>
                <label class="input-label" for="audit-ip">{{ t('admin.audit.filters.clientIp') }}</label>
                <input id="audit-ip" v-model.trim="filters.client_ip" type="text" class="input" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.audit.filters.method') }}</label>
                <Select v-model="filters.method" :options="methodOptions" @change="search" />
              </div>
              <div>
                <label class="input-label">{{ t('admin.audit.filters.authMethod') }}</label>
                <Select v-model="filters.auth_method" :options="authMethodOptions" @change="search" />
              </div>
              <div class="audit-filter-actions">
                <button type="submit" class="btn btn-primary btn-sm" :disabled="loading">{{ t('common.search') }}</button>
              </div>
            </form>
          </Transition>

          <TransitionGroup v-if="chips.length" name="ui-list" tag="div" class="ui-chips" :aria-label="t('admin.activityUi.activeFilters')">
            <button
              v-for="chip in chips"
              :key="chip.key"
              type="button"
              class="ui-chip is-active"
              :aria-label="t('admin.activityUi.clearFilter', { name: `${chip.label}: ${chip.value}` })"
              @click="clearChip(chip.key)"
            >
              <span class="text-gray-500">{{ chip.label }}</span>
              <span class="max-w-[12rem] truncate">{{ chip.value }}</span>
              <Icon name="x" size="xs" />
            </button>
            <button key="__clear" type="button" class="btn btn-ghost btn-sm" @click="resetFilters">{{ t('admin.activityUi.clearAll') }}</button>
          </TransitionGroup>
        </div>

        <div class="audit-body">
          <!-- Desktop table -->
          <div class="ui-table-wrap audit-table hidden md:block" :class="{ 'is-busy': loading && logs.length > 0 }">
            <table class="ui-table">
              <thead>
                <tr>
                  <th scope="col">{{ t('admin.audit.columns.time') }}</th>
                  <th scope="col">{{ t('admin.audit.columns.result') }}</th>
                  <th scope="col">{{ t('admin.audit.columns.action') }}</th>
                  <th scope="col">{{ t('admin.audit.columns.actor') }}</th>
                  <th scope="col" class="is-num">{{ t('admin.audit.detail.latency') }}</th>
                  <th scope="col">{{ t('admin.audit.columns.clientIp') }}</th>
                </tr>
              </thead>
              <tbody v-if="loading && logs.length === 0">
                <tr v-for="i in 6" :key="i">
                  <td v-for="j in 6" :key="j"><div class="skeleton h-4 w-3/4 rounded"></div></td>
                </tr>
              </tbody>
              <tbody v-else-if="logs.length === 0">
                <tr>
                  <td colspan="6" class="audit-empty-cell">
                    <div class="ui-empty">
                      <span class="ui-empty-icon"><Icon name="shield" size="md" /></span>
                      <b>{{ t('admin.audit.empty') }}</b>
                    </div>
                  </td>
                </tr>
              </tbody>
              <tbody v-else class="ui-rows-enter">
                <tr
                  v-for="row in logs"
                  :key="row.id"
                  class="is-clickable"
                  :class="{ 'is-selected': detailVisible && detailRowId === row.id }"
                  tabindex="0"
                  :data-row-id="row.id"
                  @click="openDetail(row.id, row)"
                  @keydown.enter.prevent="openDetail(row.id, row)"
                  @keydown.space.prevent="openDetail(row.id, row)"
                >
                  <td><time :datetime="row.created_at" :title="formatTime(row.created_at)" class="audit-time">{{ formatRelativeTime(row.created_at) }}</time></td>
                  <td>
                    <span class="audit-status">
                      <span class="ui-dot" :class="statusDot(row.status_code)" aria-hidden="true"></span>
                      <span class="tabular-nums">{{ row.status_code }}</span>
                    </span>
                  </td>
                  <td>
                    <span class="ui-cell-main audit-clip font-mono text-[0.8125rem]" :title="row.action">{{ row.action }}</span>
                    <span class="ui-cell-sub audit-clip font-mono" :title="`${row.method} ${row.path}`"><b class="audit-method">{{ row.method }}</b> {{ row.path }}</span>
                  </td>
                  <td>
                    <span class="ui-cell-main audit-clip" :title="row.actor_email">{{ row.actor_email || '—' }}</span>
                    <span class="ui-cell-sub">{{ row.actor_role }}<template v-if="row.auth_method"> · {{ authMethodLabel(row.auth_method) }}</template></span>
                  </td>
                  <td class="is-num text-gray-500">{{ row.latency_ms }} ms</td>
                  <td><span class="font-mono text-xs text-gray-500">{{ row.client_ip || '—' }}</span></td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Mobile list -->
          <ul class="audit-list md:hidden" :class="{ 'is-busy': loading && logs.length > 0 }">
            <template v-if="loading && logs.length === 0">
              <li v-for="i in 5" :key="i" class="audit-item"><div class="skeleton h-9 w-full rounded"></div></li>
            </template>
            <li v-else-if="logs.length === 0" class="ui-empty">
              <span class="ui-empty-icon"><Icon name="shield" size="md" /></span>
              <b>{{ t('admin.audit.empty') }}</b>
            </li>
            <template v-else>
              <li v-for="row in logs" :key="row.id">
                <button type="button" class="audit-item" @click="openDetail(row.id, row)">
                  <span class="ui-dot" :class="statusDot(row.status_code)" aria-hidden="true"></span>
                  <span class="min-w-0 flex-1 text-left">
                    <span class="ui-cell-main font-mono text-[0.8125rem]">{{ row.action }}</span>
                    <span class="ui-cell-sub">{{ row.actor_email || '—' }} · {{ formatRelativeTime(row.created_at) }}</span>
                  </span>
                  <span class="ui-cell-sub tabular-nums">{{ row.status_code }}</span>
                </button>
              </li>
            </template>
          </ul>

          <Pagination
            v-if="total > 0"
            :total="total"
            :page="page"
            :page-size="pageSize"
            @update:page="onPageChange"
            @update:pageSize="onPageSizeChange"
          />
        </div>
      </section>
    </div>

    <!-- Detail drawer -->
    <SideDrawer
      :open="detailVisible"
      :title="detailView?.action || t('admin.audit.detail.title')"
      :subtitle="detailView ? formatTime(detailView.created_at) : ''"
      width="36rem"
      @close="detailVisible = false"
    >
      <template #leading>
        <span v-if="detailView" class="audit-drawer-mark" :class="statusTone(detailView.status_code)">
          <Icon :name="detailView.status_code < 400 ? 'checkCircle' : 'exclamationCircle'" size="md" />
        </span>
      </template>
      <template #badge>
        <span v-if="detailView" class="badge" :class="statusBadge(detailView.status_code)">{{ detailView.status_code }} {{ statusText(detailView.status_code) }}</span>
      </template>

      <template v-if="detailView">
        <div class="audit-drawer-path">
          <b>{{ detailView.method }}</b>
          <span>{{ detailView.path }}</span>
        </div>

        <dl class="ui-dl">
          <dt>{{ t('admin.audit.columns.actor') }}</dt>
          <dd>{{ detailView.actor_email || '—' }}<span v-if="detailView.actor_user_id" class="text-gray-400"> #{{ detailView.actor_user_id }}</span></dd>
          <dt>{{ t('admin.audit.detail.actorRole') }}</dt>
          <dd>{{ detailView.actor_role || '—' }}</dd>
          <dt>{{ t('admin.audit.filters.authMethod') }}</dt>
          <dd>{{ authMethodLabel(detailView.auth_method) || '—' }}</dd>
          <template v-if="detailView.credential_masked">
            <dt>{{ t('admin.audit.detail.credential') }}</dt>
            <dd class="font-mono text-xs">{{ detailView.credential_masked }}</dd>
          </template>
          <dt>{{ t('admin.audit.columns.clientIp') }}</dt>
          <dd class="font-mono text-xs">{{ detailView.client_ip || '—' }}</dd>
          <dt>{{ t('admin.audit.detail.latency') }}</dt>
          <dd class="tabular-nums">{{ detailView.latency_ms }} ms</dd>
          <template v-if="detailView.request_id">
            <dt>{{ t('admin.audit.detail.requestId') }}</dt>
            <dd class="audit-copy-row">
              <code>{{ detailView.request_id }}</code>
              <button type="button" class="app-icon-button audit-copy" :aria-label="`${t('keys.copyToClipboard')}: ${t('admin.audit.detail.requestId')}`" @click="copy(detailView.request_id)">
                <Icon :name="copiedValue === detailView.request_id ? 'check' : 'copy'" size="sm" />
              </button>
            </dd>
          </template>
          <dt>{{ t('admin.audit.detail.userAgent') }}</dt>
          <dd class="text-xs">{{ detailView.user_agent || '—' }}</dd>
        </dl>

        <div v-if="detailLoading" class="space-y-2" aria-busy="true">
          <div class="skeleton h-4 w-1/3 rounded"></div>
          <div class="skeleton h-28 w-full rounded"></div>
        </div>
        <template v-else-if="detail">
          <section v-if="detail.request_body">
            <div class="audit-code-head">
              <h3 class="ui-section-label mb-0">{{ t('admin.audit.detail.requestBody') }}</h3>
              <button type="button" class="app-icon-button audit-copy" :aria-label="`${t('keys.copyToClipboard')}: ${t('admin.audit.detail.requestBody')}`" @click="copy(prettyBody(detail.request_body))">
                <Icon :name="copiedValue === prettyBody(detail.request_body) ? 'check' : 'copy'" size="sm" />
              </button>
            </div>
            <pre class="audit-code"><code>{{ prettyBody(detail.request_body) }}</code></pre>
          </section>
          <section v-if="detail.extra && Object.keys(detail.extra).length">
            <div class="audit-code-head">
              <h3 class="ui-section-label mb-0">{{ t('admin.audit.detail.extra') }}</h3>
              <button type="button" class="app-icon-button audit-copy" :aria-label="`${t('keys.copyToClipboard')}: ${t('admin.audit.detail.extra')}`" @click="copy(JSON.stringify(detail.extra, null, 2))">
                <Icon name="copy" size="sm" />
              </button>
            </div>
            <pre class="audit-code"><code>{{ JSON.stringify(detail.extra, null, 2) }}</code></pre>
          </section>
        </template>
      </template>
    </SideDrawer>

    <!-- Custom time range dialog (与 /admin/ops 时间下拉一致的自定义范围，支持时分) -->
    <BaseDialog
      :show="showCustomTimeRangeDialog"
      :title="t('admin.ops.timeRange.custom')"
      width="narrow"
      @close="handleCustomTimeRangeCancel"
    >
      <div class="space-y-4 py-2">
        <div>
          <label class="input-label">{{ t('admin.ops.customTimeRange.startTime') }}</label>
          <input v-model="customStartTimeInput" type="datetime-local" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.ops.customTimeRange.endTime') }}</label>
          <input v-model="customEndTimeInput" type="datetime-local" class="input" />
        </div>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="handleCustomTimeRangeCancel">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="!customStartTimeInput || !customEndTimeInput"
          @click="handleCustomTimeRangeConfirm"
        >
          {{ t('common.confirm') }}
        </button>
      </template>
    </BaseDialog>

    <!-- Clear confirmation → step-up TOTP -->
    <ConfirmDialog
      :show="clearConfirmVisible"
      :title="t('admin.audit.clearConfirm.title')"
      :message="t('admin.audit.clearConfirm.message')"
      :confirm-text="t('admin.audit.clearAll')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="onClearConfirmed"
      @cancel="clearConfirmVisible = false"
    />

    <!-- TOTP prompt for the clear operation -->
    <BaseDialog
      :show="clearTotpVisible"
      :title="t('admin.audit.clearConfirm.totpTitle')"
      width="narrow"
      :z-index="60"
      @close="cancelClearTotp"
    >
      <div class="py-2">
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.audit.clearConfirm.totpHint') }}</p>
        <input
          v-model.trim="clearTotpCode"
          type="text"
          inputmode="numeric"
          maxlength="6"
          autocomplete="one-time-code"
          class="input mt-4 text-center text-lg tracking-[0.5em]"
          placeholder="••••••"
          @keyup.enter="submitClear"
        />
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="clearing" @click="cancelClearTotp">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-danger"
          :disabled="clearing || clearTotpCode.length !== 6"
          @click="submitClear"
        >
          {{ clearing ? t('common.loading') : t('admin.audit.clearAll') }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type AuditLog } from '@/api/admin'
import { totpAPI } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { formatRelativeTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const logs = ref<AuditLog[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

const filters = reactive({
  q: '',
  actor_email: '',
  action: '',
  client_ip: '',
  method: '',
  auth_method: '',
  success: ''
})

// 时间范围：预设窗口（同 /admin/ops 时间下拉）+ 自定义起止（datetime-local，支持时分）
const timeRange = ref('')
const customStartTime = ref('')
const customEndTime = ref('')
const showCustomTimeRangeDialog = ref(false)
const customStartTimeInput = ref('')
const customEndTimeInput = ref('')

const TIME_RANGE_MINUTES: Record<string, number> = {
  '30m': 30,
  '1h': 60,
  '6h': 6 * 60,
  '24h': 24 * 60,
  '7d': 7 * 24 * 60,
  '30d': 30 * 24 * 60
}

const timeRangeOptions = computed(() => [
  { value: '', label: t('admin.audit.filters.all') },
  { value: '30m', label: t('admin.ops.timeRange.30m') },
  { value: '1h', label: t('admin.ops.timeRange.1h') },
  { value: '6h', label: t('admin.ops.timeRange.6h') },
  { value: '24h', label: t('admin.ops.timeRange.24h') },
  { value: '7d', label: t('admin.ops.timeRange.7d') },
  { value: '30d', label: t('admin.ops.timeRange.30d') },
  {
    value: 'custom',
    label:
      timeRange.value === 'custom' && customStartTime.value && customEndTime.value
        ? `${t('admin.ops.timeRange.custom')} (${formatCustomTimeRangeLabel(customStartTime.value, customEndTime.value)})`
        : t('admin.ops.timeRange.custom')
  }
])

function formatCustomTimeRangeLabel(startTime: string, endTime: string): string {
  const fmt = (raw: string) => {
    const d = new Date(raw)
    if (Number.isNaN(d.getTime())) return raw
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
  }
  return `${fmt(startTime)} ~ ${fmt(endTime)}`
}

function toDatetimeLocal(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function handleTimeRangeChange(val: string | number | boolean | null) {
  const value = String(val ?? '')
  if (value === 'custom') {
    // 预填：已有自定义值沿用，否则默认最近1小时（本地时区）
    const now = new Date()
    customStartTimeInput.value = customStartTime.value || toDatetimeLocal(new Date(now.getTime() - 60 * 60 * 1000))
    customEndTimeInput.value = customEndTime.value || toDatetimeLocal(now)
    showCustomTimeRangeDialog.value = true
    return
  }
  timeRange.value = value
  search()
}

function handleCustomTimeRangeConfirm() {
  if (!customStartTimeInput.value || !customEndTimeInput.value) return
  customStartTime.value = customStartTimeInput.value
  customEndTime.value = customEndTimeInput.value
  timeRange.value = 'custom'
  showCustomTimeRangeDialog.value = false
  search()
}

function handleCustomTimeRangeCancel() {
  // 未确认不改变当前时间范围；Select 是受控组件，展示值保持不变。
  showCustomTimeRangeDialog.value = false
}

const methodOptions = computed(() => [
  { value: '', label: t('admin.audit.filters.all') },
  { value: 'POST', label: 'POST' },
  { value: 'PUT', label: 'PUT' },
  { value: 'PATCH', label: 'PATCH' },
  { value: 'DELETE', label: 'DELETE' },
  { value: 'GET', label: 'GET' }
])

const authMethodOptions = computed(() => [
  { value: '', label: t('admin.audit.filters.all') },
  { value: 'jwt', label: 'JWT' },
  { value: 'admin_api_key', label: 'Admin API Key' }
])

const resultOptions = computed(() => [
  { value: '', label: t('admin.audit.filters.all') },
  { value: 'true', label: t('admin.audit.filters.resultSuccess') },
  { value: 'false', label: t('admin.audit.filters.resultFailure') }
])

function authMethodLabel(method: string): string {
  const found = authMethodOptions.value.find((o) => o.value === method)
  return found && found.value ? found.label : method
}

function toRFC3339(local: string): string | undefined {
  if (!local) return undefined
  const d = new Date(local)
  if (Number.isNaN(d.getTime())) return undefined
  return d.toISOString()
}

function buildTimeRangeQuery(): { start_time?: string; end_time?: string } {
  if (timeRange.value === 'custom') {
    return {
      start_time: toRFC3339(customStartTime.value),
      end_time: toRFC3339(customEndTime.value)
    }
  }
  const minutes = TIME_RANGE_MINUTES[timeRange.value]
  if (!minutes) return {}
  return { start_time: new Date(Date.now() - minutes * 60 * 1000).toISOString() }
}

function buildQuery() {
  return {
    page: page.value,
    page_size: pageSize.value,
    q: filters.q || undefined,
    actor_email: filters.actor_email || undefined,
    action: filters.action || undefined,
    client_ip: filters.client_ip || undefined,
    method: filters.method || undefined,
    auth_method: filters.auth_method || undefined,
    success: filters.success || undefined,
    ...buildTimeRangeQuery()
  }
}

async function fetchLogs() {
  loading.value = true
  try {
    const res = await adminAPI.audit.list(buildQuery())
    logs.value = res.items
    total.value = res.total
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.audit.loadFailed'))
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  Object.assign(appliedFilters, filters)
  fetchLogs()
}

// Toolbar: Filters panel, active chips, and the overflow menu.
const panelOpen = ref(false)
const moreOpen = ref(false)
const moreMenuRef = ref<HTMLElement | null>(null)
const appliedFilters = reactive({ ...filters })

type FilterKey = keyof typeof filters

const chipDefs = computed<Array<{ key: FilterKey; label: string; format?: (v: string) => string }>>(() => [
  { key: 'q', label: t('admin.audit.filters.q') },
  { key: 'actor_email', label: t('admin.audit.filters.actorEmail') },
  { key: 'action', label: t('admin.audit.filters.action') },
  { key: 'client_ip', label: t('admin.audit.filters.clientIp') },
  { key: 'method', label: t('admin.audit.filters.method') },
  { key: 'auth_method', label: t('admin.audit.filters.authMethod'), format: authMethodLabel },
  { key: 'success', label: t('admin.audit.filters.result'), format: (v) => (v === 'true' ? t('admin.audit.filters.resultSuccess') : t('admin.audit.filters.resultFailure')) },
])

const chips = computed(() => chipDefs.value
  .filter((def) => appliedFilters[def.key])
  .map((def) => ({ key: def.key, label: def.label, value: def.format ? def.format(appliedFilters[def.key]) : appliedFilters[def.key] })))

const panelCount = computed(() => chips.value.filter((chip) => !['q', 'success'].includes(chip.key)).length)

function clearChip(key: FilterKey) {
  filters[key] = ''
  search()
}

function setResult(value: string) {
  if (filters.success === value) return
  filters.success = value
  search()
}

function handleMenuClickOutside(event: MouseEvent) {
  if (moreMenuRef.value && !event.composedPath().includes(moreMenuRef.value)) moreOpen.value = false
}

onMounted(() => document.addEventListener('click', handleMenuClickOutside))
onUnmounted(() => document.removeEventListener('click', handleMenuClickOutside))

function resetFilters() {
  filters.q = ''
  filters.actor_email = ''
  filters.action = ''
  filters.client_ip = ''
  filters.method = ''
  filters.auth_method = ''
  filters.success = ''
  timeRange.value = ''
  customStartTime.value = ''
  customEndTime.value = ''
  search()
}

function onPageChange(p: number) {
  page.value = p
  fetchLogs()
}

function onPageSizeChange(ps: number) {
  pageSize.value = ps
  page.value = 1
  fetchLogs()
}

// Detail drawer: the list row renders at once; the full entry (body, extra) loads behind it.
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<AuditLog | null>(null)
const detailRow = ref<AuditLog | null>(null)
const detailRowId = ref<number | null>(null)
const detailView = computed(() => detail.value ?? detailRow.value)

async function openDetail(id: number, row?: AuditLog) {
  detailVisible.value = true
  detailLoading.value = true
  detail.value = null
  detailRow.value = row ?? null
  detailRowId.value = id
  try {
    const entry = await adminAPI.audit.get(id)
    if (detailRowId.value === id) detail.value = entry
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.audit.loadFailed'))
    if (!row) detailVisible.value = false
  } finally {
    if (detailRowId.value === id) detailLoading.value = false
  }
}

const copiedValue = ref<string | null>(null)
async function copy(value: string) {
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

function prettyBody(body: string): string {
  try {
    return JSON.stringify(JSON.parse(body), null, 2)
  } catch {
    return body
  }
}

// Clear-all flow: confirm → TOTP → clear
const clearConfirmVisible = ref(false)
const clearTotpVisible = ref(false)
const clearTotpCode = ref('')
const clearing = ref(false)
const checkingTotpStatus = ref(false)

// 与其他敏感操作一致：未启用 2FA 时直接提示去个人资料启用 TOTP，
// 而不是弹出一个无法完成的验证码输入框（后端会以 TOTP_NOT_SETUP 拒绝）。
async function openClearDialog() {
  if (checkingTotpStatus.value) return
  checkingTotpStatus.value = true
  try {
    const status = await totpAPI.getStatus()
    if (!status.enabled) {
      appStore.showError(t('stepUp.notEnabled'))
      return
    }
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.audit.loadFailed'))
    return
  } finally {
    checkingTotpStatus.value = false
  }
  clearConfirmVisible.value = true
}

function onClearConfirmed() {
  clearConfirmVisible.value = false
  clearTotpCode.value = ''
  clearTotpVisible.value = true
}

function cancelClearTotp() {
  if (clearing.value) return
  clearTotpVisible.value = false
}

async function submitClear() {
  if (clearTotpCode.value.length !== 6) return
  clearing.value = true
  try {
    const res = await adminAPI.audit.clear(clearTotpCode.value)
    clearTotpVisible.value = false
    appStore.showSuccess(t('admin.audit.clearConfirm.success', { count: res.deleted }))
    search()
  } catch (err: any) {
    appStore.showError(err?.message || t('admin.audit.clearConfirm.failed'))
    clearTotpCode.value = ''
  } finally {
    clearing.value = false
  }
}

// Helpers
function formatTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

function statusText(status: number): string {
  return status < 400 ? t('admin.audit.filters.resultSuccess') : t('admin.audit.filters.resultFailure')
}

type StatusTone = 'ok' | 'warn' | 'bad'
const toneOf = (status: number): StatusTone => (status >= 500 ? 'bad' : status >= 400 ? 'warn' : 'ok')
const DOT: Record<StatusTone, string> = { ok: 'is-ok', warn: 'is-warn', bad: 'is-bad' }
const BADGE: Record<StatusTone, string> = { ok: 'badge-success', warn: 'badge-warning', bad: 'badge-danger' }
const TONE: Record<StatusTone, string> = { ok: 'is-ok', warn: 'is-warn', bad: 'is-bad' }

const statusDot = (status: number) => DOT[toneOf(status)]
const statusBadge = (status: number) => BADGE[toneOf(status)]
const statusTone = (status: number) => TONE[toneOf(status)]

onMounted(fetchLogs)
</script>

<style scoped>
.audit-body {
  min-width: 0;
  overflow: hidden;
  border-radius: 0 0 var(--operator-radius-lg) var(--operator-radius-lg);
}
.audit-toolbar {
  display: flex;
  flex-direction: column;
  gap: 0.625rem;
  padding: 0.875rem 1rem;
  border-bottom: 1px solid var(--operator-border);
}
.audit-range {
  width: 10.5rem;
  flex: 0 0 auto;
}
.audit-range :deep(.select-trigger) {
  height: 2.25rem;
}
.audit-search {
  max-width: 20rem;
}
.audit-filter-toggle {
  gap: 0.375rem;
}
.audit-filter-toggle[aria-expanded='true'] {
  border-color: rgb(var(--operator-accent-rgb) / 0.55);
  background: var(--operator-accent-soft);
}
.audit-filter-count {
  display: inline-grid;
  min-width: 1.125rem;
  height: 1.125rem;
  place-items: center;
  padding: 0 0.3125rem;
  border-radius: 999px;
  background: var(--operator-accent);
  color: #fff;
  font-size: 0.6875rem;
  font-weight: 600;
}
.audit-menu-item {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 0.625rem;
  padding: 0 0.875rem;
  text-align: left;
}
.audit-filter-panel {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(12rem, 1fr));
  align-items: end;
  gap: 0.75rem 1rem;
  padding: 0.875rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-raised);
}
.audit-filter-panel .input-label {
  margin-bottom: 0.25rem;
  font-size: 0.75rem;
}
.audit-filter-actions {
  display: flex;
  justify-content: flex-end;
}
.audit-panel-enter-active,
.audit-panel-leave-active {
  transition: opacity var(--operator-duration) var(--operator-ease-out), transform var(--operator-duration) var(--operator-ease-out);
}
.audit-panel-enter-from,
.audit-panel-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
.audit-table,
.audit-list {
  transition: opacity var(--operator-duration) var(--operator-ease-out);
}
.is-busy {
  opacity: 0.55;
}
.audit-table {
  max-height: min(72vh, 52rem);
}
.audit-table tr:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: -2px;
}
.audit-empty-cell {
  height: auto;
}
.audit-time {
  color: var(--operator-muted-foreground);
  white-space: nowrap;
}
.audit-status {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 500;
}
.audit-clip {
  max-width: 22rem;
}
.audit-method {
  color: var(--operator-muted-foreground);
  font-weight: 600;
}
.audit-list > li + li {
  border-top: 1px solid var(--operator-border-subtle);
}
.audit-item {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem 1rem;
}
.audit-item:hover {
  background: var(--table-row-hover-background);
}
.audit-item:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: -2px;
}
.audit-list > li {
  animation: ui-rise 320ms var(--operator-ease-out) backwards;
}
.audit-drawer-mark {
  display: grid;
  width: 2.25rem;
  height: 2.25rem;
  flex: none;
  place-items: center;
  border-radius: 0.625rem;
}
.audit-drawer-mark.is-ok {
  background: var(--operator-success-soft);
  color: var(--operator-success);
}
.audit-drawer-mark.is-warn {
  background: var(--operator-warning-soft);
  color: var(--operator-warning);
}
.audit-drawer-mark.is-bad {
  background: var(--operator-destructive-soft);
  color: var(--operator-destructive);
}
.audit-drawer-path {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.8125rem;
}
.audit-drawer-path b {
  color: var(--operator-accent-text);
  font-weight: 700;
}
.audit-drawer-path span {
  min-width: 0;
  overflow-wrap: anywhere;
  color: var(--operator-foreground);
}
.audit-copy-row {
  display: flex;
  align-items: center;
  gap: 0.375rem;
}
.audit-copy-row code {
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: 0.75rem;
}
.audit-copy {
  min-width: 2rem;
  height: 2rem;
}
.audit-code-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.375rem;
}
.audit-code {
  max-height: 22rem;
  overflow: auto;
  margin: 0;
  padding: 0.875rem 1rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
  color: var(--operator-foreground);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.75rem;
  line-height: 1.6;
  white-space: pre;
}
@media (max-width: 640px) {
  .audit-toolbar {
    padding: 0.75rem;
  }
  .audit-range {
    width: 100%;
  }
  .audit-search {
    max-width: none;
  }
  .audit-filter-panel {
    grid-template-columns: minmax(0, 1fr);
  }
}
@media (prefers-reduced-motion: reduce) {
  .audit-list > li {
    animation: none;
  }
  .audit-panel-enter-active,
  .audit-panel-leave-active {
    transition: none;
  }
}
</style>
