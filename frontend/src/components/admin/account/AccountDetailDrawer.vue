<template>
  <SideDrawer
    :open="open && row !== null"
    :title="row?.account.name ?? ''"
    :subtitle="row ? `${providerLabel(row.account.platform)} · ${accountTypeLabel(row.account.type)}` : undefined"
    width="32rem"
    @close="emit('close')"
  >
    <template #leading>
      <ProviderMark v-if="row" :platform="row.account.platform" :size="38" />
    </template>
    <template #badge>
      <span v-if="row" class="badge" :class="STATUS_BADGE[row.status]" data-testid="account-drawer-status">
        {{ t(STATUS_KEYS[row.status]) }}
      </span>
    </template>

    <template v-if="row">
      <div
        v-if="row.reason && row.status !== 'active'"
        class="acct-notice"
        :class="row.status === 'error' ? 'is-bad' : 'is-warn'"
        role="status"
      >
        <Icon :name="row.status === 'error' ? 'exclamationTriangle' : 'clock'" size="sm" class="flex-none" />
        <span class="min-w-0 flex-1">
          {{ row.reason }}
          <small v-if="row.until">{{ t('admin.accountsUi.until', { time: formatDayAwareDateTime(row.until) }) }}</small>
        </span>
        <button
          v-if="isTempUnschedulable"
          type="button"
          class="btn btn-ghost btn-sm"
          @click="emit('temp-unsched')"
        >
          {{ t('admin.accountsUi.details') }}
        </button>
      </div>

      <section>
        <div class="acct-gauges">
          <div v-for="gauge in gauges" :key="gauge.key" class="acct-gauge" :class="{ 'is-stale': gauge.window?.current === false }">
            <RingGauge :value="gauge.window?.remainingPercent ?? null" :size="96" :label="gauge.window?.label ?? gauge.name" />
            <span class="acct-gauge-name">{{ gauge.window?.label ?? gauge.name }}</span>
            <small>{{ resetText(gauge.window) }}</small>
          </div>
        </div>
        <p v-if="row.summary.error" class="acct-muted mt-2" data-testid="account-drawer-usage-error">{{ row.summary.error }}</p>
      </section>

      <section v-if="extraWindows.length">
        <div class="ui-section-label">{{ t('admin.accountsUi.quotaWindows') }}</div>
        <ul class="acct-windows" data-testid="account-drawer-windows">
          <li v-for="window in extraWindows" :key="window.key" :title="windowTitle(window)">
            <span class="acct-window-label">{{ window.label }}</span>
            <QuotaMeter :value="window.remainingPercent" :label="window.label" />
            <small>{{ resetText(window) }}</small>
          </li>
        </ul>
      </section>

      <section>
        <div class="ui-section-label">{{ t('admin.accountsUi.today') }}</div>
        <dl class="acct-today">
          <div><dt>{{ t('admin.accountsUi.requests') }}</dt><dd><CountUp :value="row.today?.requests ?? null" /></dd></div>
          <div><dt>{{ t('admin.accountsUi.tokens') }}</dt><dd><CountUp :value="row.today?.tokens ?? null" :format="formatCompact" /></dd></div>
          <div><dt>{{ t('admin.accountsUi.cost') }}</dt><dd><CountUp :value="row.today?.cost ?? null" :format="formatCost" /></dd></div>
        </dl>
      </section>

      <section>
        <div class="acct-section-head">
          <span class="ui-section-label">{{ t('admin.accountsUi.recentRequests') }}</span>
          <button type="button" class="viz-link" @click="emit('stats')">
            {{ t('admin.accounts.viewStats') }}<Icon name="chevronRight" size="xs" />
          </button>
        </div>
        <div v-if="history.state === 'loading'" class="skeleton h-14 w-full rounded-md" />
        <p v-else-if="history.state === 'error'" class="acct-muted">{{ t('admin.accountsUi.loadFailed') }}</p>
        <div v-else-if="history.values.length > 1" class="acct-spark" :title="history.total.toLocaleString()">
          <SparkLine :values="history.values" :width="440" :height="56" />
          <span>{{ history.total.toLocaleString() }}</span>
        </div>
        <p v-else class="acct-muted">–</p>
      </section>

      <section>
        <div class="ui-section-label">{{ t('admin.accountsUi.details') }}</div>
        <dl class="ui-dl" data-testid="account-drawer-details">
          <template v-if="row.summary.identity">
            <dt>{{ t('admin.accountsUi.identity') }}</dt><dd>{{ row.summary.identity }}</dd>
          </template>
          <template v-if="row.summary.tier">
            <dt>{{ t('admin.accountsUi.plan') }}</dt><dd>{{ row.summary.tier }}</dd>
          </template>
          <dt>{{ t('admin.accounts.columns.groups') }}</dt>
          <dd>
            <span v-if="groupNames.length" class="acct-tags"><span v-for="name in groupNames" :key="name" class="badge badge-gray">{{ name }}</span></span>
            <span v-else class="acct-muted">{{ t('admin.accounts.ungroupedGroup') }}</span>
          </dd>
          <dt>{{ t('admin.accounts.columns.priority') }}</dt><dd class="tabular-nums">{{ row.account.priority }}</dd>
          <dt>{{ t('admin.accountsUi.concurrency') }}</dt>
          <dd class="tabular-nums">{{ row.account.current_concurrency ?? 0 }} / {{ row.account.concurrency }}</dd>
          <dt>{{ t('admin.accounts.columns.proxy') }}</dt>
          <dd>
            <template v-if="row.account.proxy">
              {{ row.account.proxy.name }}<span v-if="row.account.proxy.country_code" class="acct-muted"> · {{ row.account.proxy.country_code }}</span>
              <span v-if="row.account.proxy.expires_at" :class="proxyExpiryBadgeClass(row.account.proxy.expires_at, row.account.proxy.status)" class="ml-1">{{ proxyExpiryText }}</span>
            </template>
            <span v-else class="acct-muted">–</span>
            <span v-if="row.account.proxy_fallback_origin_id" class="mt-1 flex flex-wrap items-center gap-2">
              <span class="badge badge-warning" :title="t('admin.accounts.fallbackActiveTip', { origin: row.account.proxy_fallback_origin_name })">{{ t('admin.accounts.fallbackActive') }}</span>
              <button type="button" class="viz-link" @click="emit('revert-fallback')">{{ t('admin.accounts.revertProxy') }}</button>
            </span>
          </dd>
          <dt>{{ t('admin.accounts.columns.billingRateMultiplier') }}</dt>
          <dd class="tabular-nums">{{ formatMultiplier(row.account.rate_multiplier ?? 1) }}x</dd>
          <dt>{{ t('admin.accountsUi.lastUsed') }}</dt>
          <dd>{{ row.account.last_used_at ? formatRelativeWithDateTime(row.account.last_used_at) : '–' }}</dd>
          <template v-if="row.account.expires_at">
            <dt>{{ t('admin.accounts.columns.expiresAt') }}</dt>
            <dd>
              {{ formatDateTime(new Date(row.account.expires_at * 1000)) }}
              <span v-if="row.account.expires_at * 1000 <= Date.now()" class="badge badge-warning ml-1">{{ t('admin.accounts.expired') }}</span>
              <span v-if="row.account.auto_pause_on_expired" class="badge badge-success ml-1">{{ t('admin.accounts.autoPauseOnExpired') }}</span>
            </dd>
          </template>
          <dt>{{ t('admin.accounts.columns.createdAt') }}</dt><dd>{{ formatDateTime(row.account.created_at) }}</dd>
          <template v-if="row.account.notes">
            <dt>{{ t('admin.accounts.columns.notes') }}</dt><dd class="whitespace-pre-wrap">{{ row.account.notes }}</dd>
          </template>
          <template v-if="isAutoWarmupConfigurable(row.account)">
            <dt>{{ t('admin.accounts.autoWarmup.title') }}</dt>
            <dd><AutoWarmupStatus :account="row.account" :global-enabled="globalAutoWarmupEnabled" details /></dd>
          </template>
          <dt>{{ t('admin.accounts.columns.id') }}</dt><dd class="font-mono text-xs">#{{ row.account.id }}</dd>
        </dl>
      </section>
    </template>

    <template #footer>
      <template v-if="row">
        <button
          v-if="needsReauth"
          type="button"
          class="btn btn-primary btn-sm"
          @click="emit('reauth')"
        >
          <Icon name="link" size="sm" />{{ t('admin.accounts.reAuthorize') }}
        </button>
        <button v-else type="button" class="btn btn-primary btn-sm" @click="emit('test')">
          <Icon name="play" size="sm" />{{ t('admin.accountsUi.test') }}
        </button>
        <button
          v-if="canRefreshUsage"
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="usageLoading"
          @click="emit('refresh-usage')"
        >
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': usageLoading }" />{{ t('admin.accountsUi.refreshUsage') }}
        </button>
        <button
          type="button"
          class="btn btn-ghost btn-sm"
          :disabled="togglingSchedulable"
          :aria-pressed="!row.account.schedulable"
          @click="emit('toggle-schedulable')"
        >
          <Icon :name="row.account.schedulable ? 'stop' : 'play'" size="sm" />
          {{ row.account.schedulable ? t('admin.accountsUi.pause') : t('admin.accountsUi.resume') }}
        </button>
        <span class="flex-1" />
        <button type="button" class="app-icon-button" :aria-label="t('common.edit')" :title="t('common.edit')" @click="emit('edit')">
          <Icon name="edit" size="sm" />
        </button>
        <button type="button" class="app-icon-button" :aria-label="t('common.more')" :title="t('common.more')" @click="emit('more', $event)">
          <Icon name="more" size="sm" />
        </button>
      </template>
    </template>
  </SideDrawer>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import CountUp from '@/components/ui/CountUp.vue'
import QuotaMeter from '@/components/ui/QuotaMeter.vue'
import RingGauge from '@/components/viz/RingGauge.vue'
import SparkLine from '@/components/viz/SparkLine.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import Icon from '@/components/icons/Icon.vue'
import AutoWarmupStatus from '@/components/account/AutoWarmupStatus.vue'
import { adminAPI } from '@/api/admin'
import { formatDateTime, formatDayAwareDateTime, formatRelativeWithDateTime } from '@/utils/format'
import { formatMultiplier } from '@/utils/formatters'
import { isAutoWarmupConfigurable } from '@/utils/autoWarmup'
import { proxyExpiryBadgeClass, proxyExpiryLabelKey } from '@/utils/proxyExpiry'
import { supportsBatchAccountUsage, type OperatorAccountStatus, type OperatorCapacityWindow } from '@/utils/operatorCapacity'
import type { AdminGroup } from '@/types'
import { STATUS_BADGE, accountTypeLabel, formatPercent, providerLabel, type FleetRow } from './accountFleet'

const props = withDefaults(defineProps<{
  open: boolean
  row: FleetRow | null
  groups?: AdminGroup[]
  globalAutoWarmupEnabled?: boolean
  togglingSchedulable?: boolean
  usageLoading?: boolean
}>(), {
  groups: () => [],
  globalAutoWarmupEnabled: undefined,
  togglingSchedulable: false,
  usageLoading: false,
})

const emit = defineEmits<{
  close: []
  test: []
  reauth: []
  edit: []
  stats: []
  more: [event: MouseEvent]
  'refresh-usage': []
  'toggle-schedulable': []
  'revert-fallback': []
  'temp-unsched': []
}>()

const { t } = useI18n()

const STATUS_KEYS: Record<OperatorAccountStatus, string> = {
  active: 'admin.viz.states.active',
  limited: 'admin.viz.states.limited',
  error: 'admin.viz.states.error',
  disabled: 'admin.viz.states.disabled',
}

const gauges = computed(() => {
  const row = props.row
  if (!row) return []
  return [
    { key: 'short', name: t('admin.accountsUi.shortWindow'), window: row.short },
    { key: 'long', name: t('admin.accountsUi.longWindow'), window: row.long },
  ]
})

const extraWindows = computed(() => {
  const row = props.row
  if (!row) return []
  return row.summary.windows.filter((window) => window !== row.short && window !== row.long)
})

const groupNames = computed(() => {
  const account = props.row?.account
  if (!account) return []
  const byId = new Map(props.groups.map((group) => [group.id, group.name]))
  const fromIds = (account.group_ids ?? []).map((id) => byId.get(id)).filter((name): name is string => Boolean(name))
  return fromIds.length ? fromIds : (account.groups ?? []).map((group) => group.name).filter(Boolean)
})

const isShadow = computed(() => props.row?.account.parent_account_id != null)
const isOAuthLike = computed(() => props.row?.account.type === 'oauth' || props.row?.account.type === 'setup-token')
const needsReauth = computed(() => props.row?.status === 'error' && isOAuthLike.value && !isShadow.value)
const canRefreshUsage = computed(() => (props.row ? supportsBatchAccountUsage(props.row.account) : false))
const isTempUnschedulable = computed(() => {
  const until = props.row?.account.temp_unschedulable_until
  return Boolean(until && new Date(until).getTime() > Date.now())
})

const proxyExpiryText = computed(() => {
  const proxy = props.row?.account.proxy
  if (!proxy) return ''
  const { key, params } = proxyExpiryLabelKey(proxy.expires_at, proxy.status)
  return params ? t(key, params) : t(key)
})

const resetText = (window: OperatorCapacityWindow | null) => {
  if (!window) return t('admin.accountsUi.quotaUnknown')
  if (window.resetsAt) return t('admin.accountsUi.resets', { time: formatDayAwareDateTime(window.resetsAt) })
  return window.kind === 'spend' ? t('admin.accountsUi.noReset') : '–'
}

const windowTitle = (window: OperatorCapacityWindow) => {
  const parts = [`${window.label} · ${formatPercent(window.remainingPercent)}%`]
  if (window.estimated) parts.push(t('admin.accounts.usageWindow.estimated'))
  else if (window.source) parts.push(t(`admin.accounts.usageWindow.${window.source}`))
  if (window.stale || window.current === false) parts.push(t('admin.accounts.usageWindow.stale'))
  if (window.observedAt) parts.push(`${t('admin.accounts.usageWindow.observedAt')} ${formatDateTime(window.observedAt)}`)
  return parts.join(' · ')
}

const formatCompact = (value: number) => new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 }).format(value)
const formatCost = (value: number) => `$${value.toFixed(value >= 100 ? 0 : 2)}`

// Recent daily requests from the per-account stats endpoint, loaded when the drawer opens.
const HISTORY_DAYS = 14
const history = reactive<{ state: 'idle' | 'loading' | 'ready' | 'error'; values: number[]; total: number }>({
  state: 'idle',
  values: [],
  total: 0,
})
let historyRequest = 0
watch(
  () => [props.open, props.row?.account.id] as const,
  async ([open, id]) => {
    if (!open || id == null) return
    const request = ++historyRequest
    history.state = 'loading'
    try {
      const stats = await adminAPI.accounts.getStats(id, HISTORY_DAYS)
      if (request !== historyRequest) return
      history.values = (stats.history ?? []).map((day) => day.requests)
      history.total = stats.summary?.total_requests ?? history.values.reduce((sum, value) => sum + value, 0)
      history.state = 'ready'
    } catch (error) {
      if (request !== historyRequest) return
      console.error('Failed to load account history:', error)
      history.state = 'error'
    }
  },
  { immediate: true },
)
</script>

<style scoped>
.acct-notice {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.625rem 0.75rem;
  border-radius: var(--operator-radius);
  font-size: 0.8125rem;
}

.acct-notice small {
  display: block;
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
}

.acct-notice.is-warn {
  background: var(--operator-warning-soft);
  color: var(--operator-warning);
}

.acct-notice.is-bad {
  background: var(--operator-destructive-soft);
  color: var(--operator-destructive);
}

.acct-notice > span {
  color: var(--operator-foreground);
}

.acct-gauges {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.acct-gauge {
  display: grid;
  justify-items: center;
  gap: 0.375rem;
  text-align: center;
}

.acct-gauge.is-stale {
  opacity: 0.6;
}

.acct-gauge-name {
  margin-top: 0.25rem;
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  font-weight: 500;
}

.acct-gauge small,
.acct-windows small,
.acct-muted {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.acct-windows {
  display: flex;
  max-height: 16rem;
  flex-direction: column;
  gap: 0.5rem;
  overflow-y: auto;
}

.acct-windows li {
  display: grid;
  grid-template-columns: minmax(0, 7rem) minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.8125rem;
}

.acct-window-label {
  overflow: hidden;
  color: var(--operator-muted-foreground);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.acct-today {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.75rem;
}

.acct-today > div {
  padding: 0.625rem 0.75rem;
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
  box-shadow: inset 0 0 0 1px var(--operator-border);
}

.acct-today dt {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.acct-today dd {
  margin-top: 0.125rem;
  color: var(--operator-foreground);
  font-size: 1.125rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.01em;
}

.acct-section-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
}

.acct-spark {
  display: flex;
  align-items: flex-end;
  gap: 0.75rem;
}

.acct-spark :deep(svg) {
  width: 100%;
  min-width: 0;
  height: auto;
}

.acct-spark > span {
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.acct-tags {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 0.25rem;
}
</style>
