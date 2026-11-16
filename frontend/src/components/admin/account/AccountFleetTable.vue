<template>
  <div class="fleet" data-testid="account-fleet">
    <div v-if="error && !rows.length" class="ui-empty" role="alert" data-testid="capacity-load-error">
      <span class="ui-empty-icon"><Icon name="exclamationTriangle" size="md" /></span>
      <b>{{ t('admin.accountsUi.loadFailed') }}</b>
      <button type="button" class="btn btn-secondary btn-sm" @click="emit('retry')">{{ t('admin.accountsUi.retry') }}</button>
    </div>

    <div v-else-if="loading && !rows.length" class="fleet-skeleton" role="status" :aria-label="t('common.loading')">
      <div v-for="index in 6" :key="index" class="fleet-skeleton-row">
        <span class="skeleton h-7 w-7 rounded-lg" />
        <span class="skeleton h-3 w-40 rounded" />
        <span class="skeleton ml-auto h-2 w-24 rounded-full" />
        <span class="skeleton h-2 w-24 rounded-full" />
      </div>
    </div>

    <div v-else-if="!rows.length" class="ui-empty" data-testid="capacity-filter-empty">
      <span class="ui-empty-icon"><Icon :name="filtered ? 'search' : 'server'" size="md" /></span>
      <b>{{ filtered ? t('admin.accountsUi.noMatches') : t('admin.accounts.noAccountsYet') }}</b>
      <button v-if="filtered" type="button" class="btn btn-secondary btn-sm" @click="emit('reset-filters')">
        {{ t('admin.accountsUi.resetFilters') }}
      </button>
      <button v-else type="button" class="btn btn-primary btn-sm" @click="emit('create')">
        <Icon name="plus" size="sm" />{{ t('common.add') }}
      </button>
    </div>

    <div v-else class="ui-table-wrap fleet-wrap">
      <table class="ui-table fleet-table">
        <thead>
          <tr>
            <th class="fleet-check">
              <input
                type="checkbox"
                class="operator-checkbox h-4 w-4 rounded"
                :checked="allSelected"
                :indeterminate="someSelected && !allSelected"
                :aria-label="t('admin.accountsUi.selectVisible')"
                @change="emit('toggle-visible', ($event.target as HTMLInputElement).checked)"
              />
            </th>
            <th v-for="column in columns" :key="column.key" :class="column.class" :aria-sort="ariaSort(column.key)">
              <button type="button" class="ui-sort" :aria-pressed="sort.key === column.key" @click="emit('sort', column.key)">
                {{ column.label }}
                <Icon :name="sortIcon(column.key)" size="xs" />
              </button>
            </th>
            <th class="fleet-more"><span class="sr-only">{{ t('admin.accounts.columns.actions') }}</span></th>
          </tr>
        </thead>
        <tbody
          v-for="(group, index) in groups"
          :key="group.platform"
          class="ui-rows-enter fleet-section"
          :data-testid="`provider-section-${group.platform}`"
        >
          <tr v-if="index > 0" class="fleet-gap" aria-hidden="true"><td :colspan="columns.length + 2"></td></tr>
          <tr class="fleet-group">
            <td :colspan="columns.length + 2">
              <button
                type="button"
                class="fleet-group-toggle"
                :aria-expanded="!collapsed.has(group.platform)"
                :data-testid="`provider-toggle-${group.platform}`"
                @click="toggleGroup(group.platform)"
              >
                <Icon name="chevronDown" size="sm" class="fleet-group-chevron" :class="{ 'is-collapsed': collapsed.has(group.platform) }" />
                <ProviderMark :platform="group.platform" :size="26" />
                <span class="fleet-group-name">{{ providerLabel(group.platform) }}</span>
                <span class="fleet-group-count">
                  {{ group.rows.length }}
                  <small>
                    · {{ t('admin.dashboard.capacity.knownCount', { count: group.knownCount }) }}
                    <template v-if="group.unknownCount"> · {{ t('admin.dashboard.capacity.unknownCount', { count: group.unknownCount }) }}</template>
                  </small>
                </span>
                <span class="fleet-group-meter">
                  <QuotaMeter :value="group.remaining" :label="providerLabel(group.platform)" />
                </span>
              </button>
            </td>
          </tr>
          <template v-if="!collapsed.has(group.platform)">
            <tr
              v-for="row in group.rows"
              :key="row.account.id"
              class="is-clickable"
              :class="{ 'is-selected': selected.has(row.account.id), 'ui-flash': flashing.has(row.account.id) }"
              tabindex="0"
              data-testid="account-capacity-row"
              :data-status="row.status"
              :aria-label="t('admin.accountsUi.open', { name: row.account.name })"
              @click="emit('open', row.account)"
              @keydown.enter.self.prevent="emit('open', row.account)"
            >
              <td class="fleet-check" @click.stop>
                <input
                  type="checkbox"
                  class="operator-checkbox h-4 w-4 rounded"
                  :checked="selected.has(row.account.id)"
                  :aria-label="t('admin.accountsUi.selectRow', { name: row.account.name })"
                  @change="emit('toggle', row.account.id)"
                />
              </td>
              <td class="fleet-name">
                <span class="fleet-identity" :title="row.summary.identity || undefined">
                  <ProviderMark :platform="row.account.platform" :size="30" />
                  <span class="min-w-0">
                    <span class="ui-cell-main">{{ row.account.name }}</span>
                    <span class="ui-cell-sub">{{ providerLabel(row.account.platform) }} · {{ accountTypeLabel(row.account.type) }}</span>
                  </span>
                </span>
              </td>
              <td :data-label="t('common.status')">
                <span class="badge" :class="STATUS_BADGE[row.status]" :title="statusTitle(row)">
                  {{ statusLabel(row.status) }}
                  <Icon v-if="row.reason" name="infoCircle" size="xs" class="opacity-60" />
                </span>
              </td>
              <td :data-label="t('admin.accountsUi.shortWindow')">
                <div :title="windowTitle(row.short)">
                  <div :class="{ 'is-stale': row.short?.current === false }">
                    <QuotaMeter :value="row.short?.remainingPercent ?? null" :label="row.short?.label ?? t('admin.accountsUi.shortWindow')" />
                  </div>
                  <small v-if="row.short && windowCaption(row.short)" class="fleet-reset">
                    <span v-if="row.short.current === false" class="fleet-stale">{{ staleLabel(row.short) }}</span>
                    <span v-if="row.short.current === false && resetCountdown(row.short)" aria-hidden="true"> · </span>
                    <span v-if="resetCountdown(row.short)" data-testid="fleet-reset-countdown">{{ t('admin.accountsUi.resetsIn', { time: resetCountdown(row.short) }) }}</span>
                  </small>
                </div>
              </td>
              <td :data-label="t('admin.accountsUi.longWindow')">
                <div :title="windowTitle(row.long)">
                  <div :class="{ 'is-stale': row.long?.current === false }">
                    <QuotaMeter :value="row.long?.remainingPercent ?? null" :label="row.long?.label ?? t('admin.accountsUi.longWindow')" />
                  </div>
                  <small v-if="row.long && windowCaption(row.long)" class="fleet-reset">
                    <span v-if="row.long.current === false" class="fleet-stale">{{ staleLabel(row.long) }}</span>
                    <span v-if="row.long.current === false && resetCountdown(row.long)" aria-hidden="true"> · </span>
                    <span v-if="resetCountdown(row.long)" data-testid="fleet-reset-countdown">{{ t('admin.accountsUi.resetsIn', { time: resetCountdown(row.long) }) }}</span>
                  </small>
                </div>
              </td>
              <td class="is-num" :data-label="t('admin.accountsUi.today')">
                <span v-if="row.today === null && todayLoading" class="skeleton inline-block h-3 w-10 rounded" />
                <span v-else :class="{ 'text-gray-400': !row.today?.requests }">{{ formatCount(row.today?.requests) }}</span>
              </td>
              <td class="is-num fleet-last" :data-label="t('admin.accountsUi.lastUsed')">
                {{ row.account.last_used_at ? formatRelativeTime(row.account.last_used_at) : '–' }}
              </td>
              <td class="fleet-more" @click.stop>
                <button
                  type="button"
                  class="app-icon-button"
                  :aria-label="t('admin.accountsUi.moreFor', { name: row.account.name })"
                  @click="emit('more', row.account, $event)"
                >
                  <Icon name="more" size="sm" />
                </button>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import QuotaMeter from '@/components/ui/QuotaMeter.vue'
import { aggregateNormalizedCapacity, type OperatorAccountStatus, type OperatorCapacityWindow } from '@/utils/operatorCapacity'
import { formatCountdown, formatDayAwareDateTime, formatRelativeTime } from '@/utils/format'
import type { Account, AccountPlatform } from '@/types'
import {
  PLATFORM_ORDER,
  STATUS_BADGE,
  accountTypeLabel,
  fleetRowSignature,
  formatPercent,
  providerLabel,
  type FleetRow,
  type FleetSort,
  type FleetSortKey,
} from './accountFleet'

const props = withDefaults(defineProps<{
  rows: FleetRow[]
  sort: FleetSort
  selected: Set<number>
  loading?: boolean
  error?: boolean
  filtered?: boolean
  todayLoading?: boolean
}>(), {
  loading: false,
  error: false,
  filtered: false,
  todayLoading: false,
})

const emit = defineEmits<{
  open: [account: Account]
  more: [account: Account, event: MouseEvent]
  toggle: [id: number]
  'toggle-visible': [checked: boolean]
  sort: [key: FleetSortKey]
  retry: []
  'reset-filters': []
  create: []
}>()

const { t } = useI18n()

const columns = computed<Array<{ key: FleetSortKey; label: string; class?: string }>>(() => [
  { key: 'name', label: t('admin.accountsUi.account'), class: 'fleet-name-head' },
  { key: 'status', label: t('common.status') },
  { key: 'short', label: t('admin.accountsUi.shortWindow'), class: 'fleet-meter-head' },
  { key: 'long', label: t('admin.accountsUi.longWindow'), class: 'fleet-meter-head' },
  { key: 'today', label: t('admin.accountsUi.today'), class: 'is-num' },
  { key: 'last_used', label: t('admin.accountsUi.lastUsed'), class: 'is-num' },
])

const ariaSort = (key: FleetSortKey) => (props.sort.key === key ? (props.sort.dir === 1 ? 'ascending' : 'descending') : undefined)
const sortIcon = (key: FleetSortKey) => (props.sort.key !== key ? 'arrowsUpDown' : props.sort.dir === 1 ? 'arrowUp' : 'arrowDown')

const allSelected = computed(() => props.rows.length > 0 && props.rows.every((row) => props.selected.has(row.account.id)))
const someSelected = computed(() => props.rows.some((row) => props.selected.has(row.account.id)))

const STATUS_KEYS: Record<OperatorAccountStatus, string> = {
  active: 'admin.viz.states.active',
  limited: 'admin.viz.states.limited',
  error: 'admin.viz.states.error',
  disabled: 'admin.viz.states.disabled',
}
const statusLabel = (status: OperatorAccountStatus) => t(STATUS_KEYS[status])
const statusTitle = (row: FleetRow) => {
  if (!row.reason) return undefined
  const until = row.until ? formatDayAwareDateTime(row.until) : ''
  return until ? `${row.reason} · ${t('admin.accountsUi.until', { time: until })}` : row.reason
}

const staleLabel = (window: OperatorCapacityWindow) => t(
  window.estimated ? 'admin.accounts.usageWindow.estimated' : 'admin.accounts.usageWindow.stale'
)
const windowTitle = (window: OperatorCapacityWindow | null) => {
  if (!window) return t('admin.accountsUi.quotaUnknown')
  const parts = [`${window.label} · ${formatPercent(window.remainingPercent)}%`]
  if (window.resetsAt) parts.push(t('admin.accountsUi.resets', { time: formatDayAwareDateTime(window.resetsAt) }))
  if (window.current === false) parts.push(t(window.estimated ? 'admin.accounts.usageWindow.estimated' : 'admin.accounts.usageWindow.stale'))
  return parts.join(' · ')
}

// Reset countdowns tick once a minute so "Resets in 2h 14m" stays honest without re-fetching.
const now = ref(Date.now())
const nowTimer = setInterval(() => { now.value = Date.now() }, 30_000)
const resetCountdown = (window: OperatorCapacityWindow) => {
  void now.value
  return formatCountdown(window.resetsAt)
}
const windowCaption = (window: OperatorCapacityWindow) => window.current === false || Boolean(resetCountdown(window))

const formatCount = (value: number | null | undefined) => (value == null ? '–' : value.toLocaleString())

// Rows are always split into provider sections (collapsed state survives reloads within the session).
const COLLAPSE_KEY = 'account-fleet-collapsed-providers'
const loadCollapsed = () => {
  try {
    const stored = JSON.parse(sessionStorage.getItem(COLLAPSE_KEY) ?? '[]')
    return new Set<string>(Array.isArray(stored) ? stored.filter((value) => typeof value === 'string') : [])
  } catch {
    return new Set<string>()
  }
}
const collapsed = ref(loadCollapsed())
const toggleGroup = (platform: string) => {
  const next = new Set(collapsed.value)
  if (!next.delete(platform)) next.add(platform)
  collapsed.value = next
  try {
    sessionStorage.setItem(COLLAPSE_KEY, JSON.stringify([...next]))
  } catch {
    // Storage may be unavailable; grouping still works for this view.
  }
}

interface FleetGroup {
  platform: AccountPlatform
  rows: FleetRow[]
  remaining: number | null
  knownCount: number
  unknownCount: number
}

const groups = computed<FleetGroup[]>(() => {
  const byPlatform = new Map<AccountPlatform, FleetRow[]>()
  for (const row of props.rows) {
    const list = byPlatform.get(row.account.platform) ?? []
    list.push(row)
    byPlatform.set(row.account.platform, list)
  }
  const rank = (platform: string) => {
    const index = PLATFORM_ORDER.indexOf(platform)
    return index === -1 ? PLATFORM_ORDER.length : index
  }
  return [...byPlatform.entries()]
    .sort(([left], [right]) => rank(left) - rank(right))
    .map(([platform, rows]) => {
      const aggregate = aggregateNormalizedCapacity(rows.map((row) => row.summary))
      return {
        platform,
        rows,
        remaining: aggregate.remainingPercent,
        knownCount: aggregate.knownCount,
        unknownCount: aggregate.unknownCount,
      }
    })
})

// Flash rows whose visible data changed after a refresh; the first paint stays still.
const signatures = new Map<number, string>()
const flashing = ref(new Set<number>())
const flashTimers = new Map<number, ReturnType<typeof setTimeout>>()
watch(
  () => props.rows,
  (rows) => {
    const primed = signatures.size > 0
    const changed: number[] = []
    for (const row of rows) {
      const signature = fleetRowSignature(row)
      const previous = signatures.get(row.account.id)
      if (primed && previous !== undefined && previous !== signature) changed.push(row.account.id)
      signatures.set(row.account.id, signature)
    }
    if (!changed.length) return
    const next = new Set(flashing.value)
    for (const id of changed) {
      next.add(id)
      clearTimeout(flashTimers.get(id))
      flashTimers.set(id, setTimeout(() => {
        flashTimers.delete(id)
        const remaining = new Set(flashing.value)
        remaining.delete(id)
        flashing.value = remaining
      }, 1400))
    }
    flashing.value = next
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  flashTimers.forEach((timer) => clearTimeout(timer))
  clearInterval(nowTimer)
})
</script>

<style scoped>
.fleet-wrap {
  max-height: calc(100vh - 13rem);
}

.fleet-table {
  min-width: 52rem;
}

.fleet-table th.fleet-check,
.fleet-table td.fleet-check {
  width: 2.75rem;
  padding-right: 0;
}

.fleet-table th.fleet-more,
.fleet-table td.fleet-more {
  width: 3rem;
  padding: 0 0.5rem 0 0;
  text-align: right;
}

.fleet-table :deep(.fleet-meter-head) {
  width: 11rem;
}

.fleet-table tbody tr.is-clickable:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: -2px;
}

.fleet-identity {
  display: flex;
  min-width: 0;
  max-width: 22rem;
  align-items: center;
  gap: 0.75rem;
}

.fleet-last {
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.fleet-more .app-icon-button {
  opacity: 0.55;
  transition: opacity var(--operator-duration-fast) var(--operator-ease-out);
}

.fleet-table tr:hover .fleet-more .app-icon-button,
.fleet-more .app-icon-button:focus-visible {
  opacity: 1;
}

.is-stale {
  opacity: 0.55;
}

.fleet-reset {
  display: block;
  margin-top: 0.125rem;
  color: var(--operator-subtle-foreground);
  font-size: 0.6875rem;
  font-variant-numeric: tabular-nums;
  line-height: 1rem;
  white-space: nowrap;
}

.fleet-stale {
  color: var(--operator-warning);
}

.fleet-table tbody tr.fleet-gap,
.fleet-table tbody tr.fleet-gap:hover {
  background: transparent;
}

.fleet-table tbody tr.fleet-gap td {
  height: 1.25rem;
  padding: 0;
  border-bottom: 0;
}

.fleet-group td {
  position: sticky;
  top: 2.5rem;
  z-index: 1;
  height: 3rem;
  padding: 0 0.5rem;
  border-top: 1px solid var(--operator-border);
  border-bottom: 1px solid var(--operator-border);
  background: var(--operator-raised);
}

.fleet-section:first-of-type .fleet-group td {
  border-top: 0;
}

.fleet-table tbody tr.fleet-group:hover {
  background: transparent;
}

.fleet-group-toggle {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 0.625rem;
  padding: 0 0.5rem;
  border-radius: var(--operator-radius);
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 600;
  text-align: left;
}

.fleet-group-name {
  font-size: 0.9375rem;
}

.fleet-group-chevron {
  color: var(--operator-subtle-foreground);
  transition: transform var(--operator-duration) var(--operator-ease-out);
}

.fleet-group-chevron.is-collapsed {
  transform: rotate(-90deg);
}

.fleet-group-count {
  color: var(--operator-subtle-foreground);
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}

.fleet-group-count small {
  font-size: 0.75rem;
  font-weight: 400;
}

@media (max-width: 640px) {
  .fleet-group-count small {
    display: none;
  }
}

.fleet-group-meter {
  width: 11rem;
  margin-left: auto;
  font-weight: 400;
}

.fleet-skeleton {
  display: flex;
  flex-direction: column;
}

.fleet-skeleton-row {
  display: flex;
  height: 3.25rem;
  align-items: center;
  gap: 0.875rem;
  padding: 0 1rem;
  border-bottom: 1px solid var(--operator-border-subtle);
}

/* Phones: rows become compact cards. */
@media (max-width: 640px) {
  .fleet-wrap {
    max-height: none;
    overflow: visible;
  }

  .fleet-table {
    min-width: 0;
  }

  .fleet-table thead {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }

  .fleet-table,
  .fleet-table tbody {
    display: block;
  }

  .fleet-table tbody tr.is-clickable {
    position: relative;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.625rem 1rem;
    padding: 0.875rem 1rem 0.875rem 2.75rem;
    border-bottom: 1px solid var(--operator-border-subtle);
  }

  .fleet-table tbody tr.fleet-group {
    display: block;
  }

  .fleet-table tbody tr.fleet-group td {
    position: static;
    display: block;
    padding: 0.375rem;
  }

  .fleet-table tbody tr.fleet-gap,
  .fleet-table tbody tr.fleet-gap td {
    display: block;
    height: 1rem;
  }

  .fleet-table tbody tr.is-clickable td {
    display: block;
    height: auto;
    padding: 0;
    border: 0;
    text-align: left;
  }

  .fleet-table tbody tr.is-clickable td[data-label]::before {
    display: block;
    margin-bottom: 0.25rem;
    color: var(--operator-subtle-foreground);
    font-size: 0.6875rem;
    content: attr(data-label);
  }

  .fleet-table td.fleet-name {
    grid-column: 1 / -1;
    padding-right: 2.5rem !important;
  }

  .fleet-table td.fleet-check {
    position: absolute;
    top: 1.375rem;
    left: 1rem;
    width: auto;
  }

  .fleet-table td.fleet-more {
    position: absolute;
    top: 0.75rem;
    right: 0.5rem;
    width: auto;
  }

  .fleet-more .app-icon-button {
    opacity: 1;
  }

  .fleet-group-meter {
    width: 7rem;
  }
}
</style>
