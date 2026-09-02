<template>
  <section class="stats-capacity-section" :aria-labelledby="sectionTitleId">
    <header class="stats-section-header">
      <div>
        <h2 :id="sectionTitleId">{{ t('admin.stats.capacity.title') }}</h2>
        <p>{{ t('admin.stats.capacity.description') }}</p>
      </div>
      <span v-if="accounts.length" class="stats-capacity-count">
        {{ t('admin.dashboard.capacity.accountCount', { count: accounts.length }) }}
      </span>
    </header>

    <div v-if="error" class="stats-section-state is-error" role="alert" data-testid="stats-capacity-error">
      <span>{{ t('admin.dashboard.capacity.loadFailed') }}</span>
      <button type="button" class="btn btn-secondary" @click="emit('retry')">
        {{ t('admin.dashboard.retry') }}
      </button>
    </div>
    <div v-else-if="loading && !accounts.length" class="stats-section-state" role="status">
      <LoadingSpinner size="sm" />
      <span>{{ t('admin.dashboard.capacity.loading') }}</span>
    </div>
    <div v-else-if="!accounts.length" class="stats-section-state">
      {{ t('admin.dashboard.capacity.empty') }}
    </div>

    <template v-else>
      <div class="stats-capacity-overview" data-testid="stats-capacity-donut-overview">
        <StatsCapacityDonut
          :title="t('admin.stats.capacity.aggregateTitle')"
          :basis="t('admin.stats.capacity.mixedAverageLimitingQuota')"
          :capacity="normalizedPool"
          test-id="stats-capacity-donut-overall"
        />
        <StatsCapacityDonut
          v-for="provider in providerDetails"
          :key="provider.platform"
          :title="providerLabel(provider.platform)"
          :basis="t('admin.stats.capacity.averageLimitingQuota')"
          :capacity="provider"
          :platform="provider.platform"
          :test-id="`provider-capacity-${provider.platform}`"
        />
      </div>

      <div class="stats-capacity-comparison">
        <article
          v-for="panel in capacityPanels"
          :key="panel.key"
          class="stats-window-panel"
          :data-testid="panel.testId"
        >
          <header>
            <h3>{{ panel.title }}</h3>
            <p>{{ panel.description }}</p>
          </header>
          <ul class="stats-window-providers">
            <li
              v-for="provider in providerDetails"
              :key="provider.platform"
              :data-testid="`${panel.key}-provider-${provider.platform}`"
            >
              <div class="stats-provider-heading">
                <span class="stats-provider-icon" aria-hidden="true">
                  <ProviderIcon :provider="provider.platform" :size="18" />
                </span>
                <strong>{{ providerLabel(provider.platform) }}</strong>
              </div>
              <p v-if="!windowsForPanel(provider, panel.key).length" class="stats-window-unsupported">
                {{ t('admin.stats.capacity.notReported') }}
              </p>
              <div
                v-for="window in windowsForPanel(provider, panel.key)"
                :key="window.key"
                class="stats-window-row"
              >
                <div class="stats-window-row-heading">
                  <strong>{{ window.label }}</strong>
                  <strong :class="capacityTone(window.remainingPercent)">
                    {{ remainingLabel(window.remainingPercent) }}
                  </strong>
                </div>
                <div
                  class="stats-window-track"
                  role="progressbar"
                  :aria-label="windowAriaLabel(provider.platform, window)"
                  aria-valuemin="0"
                  aria-valuemax="100"
                  :aria-valuenow="window.remainingPercent === null ? undefined : Math.round(window.remainingPercent)"
                >
                  <span
                    v-if="window.remainingPercent !== null"
                    :class="capacityTone(window.remainingPercent)"
                    :style="{ width: `${window.remainingPercent}%` }"
                  />
                </div>
                <p>
                  {{ t('admin.stats.capacity.windowCoverage', {
                    known: window.knownCount,
                    unknown: window.unknownCount,
                  }) }}
                  <template v-if="window.nextReset">
                    · {{ t('admin.dashboard.capacity.nextLimitingReset', { time: formatCompactReset(window.nextReset) }) }}
                  </template>
                </p>
              </div>
            </li>
          </ul>
        </article>
      </div>

      <section class="stats-inspector" :aria-labelledby="inspectorTitleId" data-testid="stats-capacity-inspector">
        <div class="stats-inspector-control">
          <div>
            <h3 :id="inspectorTitleId">{{ t('admin.stats.capacity.inspectorTitle') }}</h3>
            <p>{{ t('admin.stats.capacity.inspectorDescription') }}</p>
          </div>
          <label class="stats-account-select">
            <span>{{ t('admin.stats.capacity.accountSelector') }}</span>
            <Select
              :model-value="resolvedAccountKey"
              :options="accountOptions"
              value-key="key"
              label-key="label"
              :searchable="true"
              :aria-label="t('admin.stats.capacity.inspectAccount')"
              :search-placeholder="t('admin.stats.capacity.searchAccounts')"
              @update:model-value="selectAccount"
            >
              <template #option="{ option }">
                <span class="stats-select-option">
                  <strong>{{ option.label }}</strong>
                  <small>{{ option.description }}</small>
                </span>
              </template>
            </Select>
          </label>
        </div>

        <div v-if="selectedAccount" class="stats-inspector-detail" aria-live="polite" data-testid="stats-selected-account-detail">
          <header class="stats-inspector-summary">
            <span class="stats-provider-icon" aria-hidden="true">
              <ProviderIcon :provider="selectedAccount.account.platform" :size="18" />
            </span>
            <div>
              <strong>{{ selectedAccount.account.name }}</strong>
              <span>{{ providerLabel(selectedAccount.account.platform) }} · {{ accountTypeLabel(selectedAccount.account.type) }}</span>
            </div>
            <strong class="stats-account-status" :class="`is-${selectedStatus.status}`">
              {{ t(`admin.stats.capacity.status.${selectedStatus.status}`) }}
            </strong>
          </header>

          <p v-if="selectedStatus.reason" class="stats-account-note">
            {{ selectedStatus.reason }}
            <template v-if="selectedStatus.until">
              · {{ t('admin.stats.capacity.until', { time: formatCompactReset(selectedStatus.until) }) }}
            </template>
          </p>

          <section class="stats-account-capacity" :aria-labelledby="accountCapacityTitleId">
            <header>
              <div>
                <h4 :id="accountCapacityTitleId">{{ t('admin.stats.capacity.accountCapacity') }}</h4>
                <p v-if="limitingWindow" data-testid="stats-account-limiting-window">
                  {{ t('admin.stats.capacity.limitingWindow', {
                    window: limitingWindow.label,
                    value: remainingLabel(limitingWindow.remainingPercent),
                  }) }}
                </p>
                <p v-else>{{ t('admin.dashboard.capacity.quotaUnknown') }}</p>
              </div>
            </header>

            <div v-if="selectedNonModelWindows.length" class="stats-account-window-grid">
              <article
                v-for="window in selectedNonModelWindows"
                :key="window.key"
                data-testid="stats-account-capacity-window"
              >
                <div class="stats-window-row-heading">
                  <strong>{{ window.label }}</strong>
                  <strong :class="capacityTone(window.remainingPercent)">{{ remainingLabel(window.remainingPercent) }}</strong>
                </div>
                <div
                  class="stats-window-track"
                  role="progressbar"
                  :aria-label="accountWindowAriaLabel(selectedAccount.account.platform, window)"
                  aria-valuemin="0"
                  aria-valuemax="100"
                  :aria-valuenow="Math.round(window.remainingPercent)"
                >
                  <span :class="capacityTone(window.remainingPercent)" :style="{ width: `${window.remainingPercent}%` }" />
                </div>
                <p>
                  {{ window.resetsAt
                    ? t('admin.dashboard.capacity.resets', { time: formatCompactReset(window.resetsAt) })
                    : t('admin.dashboard.capacity.resetUnknown') }}
                </p>
              </article>
            </div>

            <p v-else-if="!selectedModelWindows.length" class="stats-account-empty" data-testid="stats-account-capacity-unknown">
              {{ t('admin.stats.capacity.noAccountTelemetry') }}
            </p>
          </section>

          <section v-if="selectedModelWindows.length" class="stats-account-models" :aria-labelledby="modelCapacityTitleId">
            <div class="stats-account-models-heading">
              <div>
                <h4 :id="modelCapacityTitleId">{{ t('admin.stats.capacity.modelCapacity') }}</h4>
                <p data-testid="stats-model-limit-summary">{{ modelLimitSummary }}</p>
              </div>
              <label v-if="moreModelCount" class="stats-target-select">
                <span>{{ t('admin.stats.capacity.moreModelLimits', { count: moreModelCount }) }}</span>
                <Select
                  :model-value="selectedModelKey"
                  :options="modelOptions"
                  value-key="key"
                  label-key="label"
                  :searchable="true"
                  :aria-label="t('admin.stats.capacity.inspectModel')"
                  :search-placeholder="t('admin.stats.capacity.searchModels')"
                  @update:model-value="selectModel"
                />
              </label>
            </div>
            <ul class="stats-model-limits" :aria-label="t('admin.stats.capacity.modelCapacity')">
              <li v-for="window in visibleModelWindows" :key="window.key" data-testid="stats-model-limit-row">
                <div class="stats-window-row-heading">
                  <strong :title="window.label">{{ window.label }}</strong>
                  <strong :class="capacityTone(window.remainingPercent)">{{ remainingLabel(window.remainingPercent) }}</strong>
                </div>
                <div
                  class="stats-window-track"
                  role="progressbar"
                  :aria-label="accountWindowAriaLabel(selectedAccount.account.platform, window)"
                  aria-valuemin="0"
                  aria-valuemax="100"
                  :aria-valuenow="Math.round(window.remainingPercent)"
                >
                  <span :class="capacityTone(window.remainingPercent)" :style="{ width: `${window.remainingPercent}%` }" />
                </div>
                <p>
                  {{ window.resetsAt
                    ? t('admin.dashboard.capacity.resets', { time: formatCompactReset(window.resetsAt) })
                    : t('admin.dashboard.capacity.resetUnknown') }}
                </p>
              </li>
            </ul>
          </section>

          <section class="stats-account-operational" :aria-labelledby="operationalTitleId">
            <h4 :id="operationalTitleId">{{ t('admin.stats.capacity.operational') }}</h4>
            <dl>
              <div>
                <dt>{{ t('admin.stats.capacity.schedulable') }}</dt>
                <dd>{{ selectedAccount.account.schedulable ? t('common.yes') : t('common.no') }}</dd>
              </div>
              <div>
                <dt>{{ t('admin.stats.capacity.accountType') }}</dt>
                <dd>{{ accountTypeLabel(selectedAccount.account.type) }}</dd>
              </div>
              <div>
                <dt>{{ t('admin.stats.capacity.providerPool') }}</dt>
                <dd>{{ t('admin.dashboard.capacity.accountCount', { count: selectedProviderAccountCount }) }}</dd>
              </div>
              <div>
                <dt>{{ t('admin.stats.capacity.snapshotSource') }}</dt>
                <dd>{{ snapshotSourceLabel }}</dd>
              </div>
              <div>
                <dt>{{ t('admin.stats.capacity.snapshotUpdated') }}</dt>
                <dd>{{ selectedUsage?.updated_at ? formatCompactReset(selectedUsage.updated_at) : t('common.unknown') }}</dd>
              </div>
              <div>
                <dt>{{ t('admin.stats.capacity.groups') }}</dt>
                <dd>{{ selectedAccount.groups.length ? selectedAccount.groups.join(', ') : t('admin.stats.capacity.noGroups') }}</dd>
              </div>
            </dl>
          </section>
        </div>
      </section>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Select from '@/components/common/Select.vue'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import StatsCapacityDonut from '@/components/admin/StatsCapacityDonut.vue'
import { formatDateTimeToMinute, formatDayAwareDateTime } from '@/utils/format'
import {
  buildNormalizedPoolCapacity,
  buildProviderCapacity,
  buildWindowCapacities,
  classifyOperatorAccount,
  type OperatorAccountCapacity,
  type OperatorCapacityWindow,
  type OperatorProviderCapacity,
  type OperatorWindowCapacity,
} from '@/utils/operatorCapacity'
import type { Account, AccountPlatform, AccountUsageInfo } from '@/types'

const props = withDefaults(defineProps<{
  accounts: Account[]
  usageByAccountId?: Record<string, AccountUsageInfo | null | undefined>
  errorsByAccountId?: Record<string, string | null | undefined>
  loading?: boolean
  error?: boolean
}>(), {
  usageByAccountId: () => ({}),
  errorsByAccountId: () => ({}),
  loading: false,
  error: false,
})

const emit = defineEmits<{ retry: [] }>()
const { t } = useI18n()
const sectionTitleId = 'stats-capacity-title'
const inspectorTitleId = 'stats-capacity-inspector-title'
const accountCapacityTitleId = 'stats-account-capacity-title'
const modelCapacityTitleId = 'stats-account-model-capacity-title'
const operationalTitleId = 'stats-account-operational-title'
const MAX_MODEL_WINDOWS = 3

const providerLabel = (platform: AccountPlatform) => ({
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  gemini: 'Gemini',
  antigravity: 'Google',
  grok: 'Grok',
  kimi: 'Kimi',
  zhipu: 'Zhipu',
  deepseek: 'DeepSeek',
})[platform]
const formatPercent = (value: number) => value.toFixed(2).replace(/\.?0+$/, '')
const remainingLabel = (value: number | null) => value === null
  ? t('common.unknown')
  : `${formatPercent(value)}%`
const capacityTone = (value: number | null) => {
  if (value === null) return 'is-unknown'
  if (value === 0) return 'is-exhausted'
  if (value <= 20) return 'is-limited'
  return 'is-healthy'
}
const formatReset = (value: string) => formatDateTimeToMinute(value) || t('common.unknown')
const formatCompactReset = (value: string) => formatDayAwareDateTime(value) || t('common.unknown')
const isModelWindow = (window: OperatorWindowCapacity) => window.key.startsWith('antigravity:')
const isShortTermWindow = (key: string) => (
  key === 'five_hour'
  || key.startsWith('gemini_')
  || key === 'spend:daily'
)
const isLongTermWindow = (key: string) => (
  key.startsWith('seven_day')
  || key === 'thirty_day'
  || key === 'spend:weekly'
  || key === 'spend:total'
  || key === 'grok:monthly-spend'
)

interface ProviderDetail extends OperatorProviderCapacity {
  windows: OperatorWindowCapacity[]
  shortWindows: OperatorWindowCapacity[]
  longWindows: OperatorWindowCapacity[]
  modelWindows: OperatorWindowCapacity[]
}
const providers = computed(() => buildProviderCapacity(
  props.accounts,
  props.usageByAccountId,
  props.errorsByAccountId,
))
const summaries = computed(() => providers.value.flatMap((provider) => provider.accounts))
const normalizedPool = computed(() => buildNormalizedPoolCapacity(summaries.value))
const providerDetails = computed<ProviderDetail[]>(() => providers.value.map((provider) => {
  const windows = buildWindowCapacities(provider.accounts)
  return {
    ...provider,
    windows,
    shortWindows: windows.filter((window) => isShortTermWindow(window.key)),
    longWindows: windows.filter((window) => isLongTermWindow(window.key)),
    modelWindows: windows.filter(isModelWindow),
  }
}))

type CapacityPanelKey = 'short' | 'long'
const capacityPanels = computed(() => [
  {
    key: 'short' as const,
    testId: 'stats-short-term-capacity',
    title: t('admin.stats.capacity.shortTermTitle'),
    description: t('admin.stats.capacity.shortTermDescription'),
  },
  {
    key: 'long' as const,
    testId: 'stats-long-term-capacity',
    title: t('admin.stats.capacity.longTermTitle'),
    description: t('admin.stats.capacity.longTermDescription'),
  },
])
const windowsForPanel = (provider: ProviderDetail, panel: CapacityPanelKey) => (
  panel === 'short' ? provider.shortWindows : provider.longWindows
)

const selectedModelKey = ref('')
const selectedAccountKey = ref('')
const accountTypeLabel = (type: string) => ({
  oauth: 'OAuth',
  apikey: 'API key',
  'setup-token': 'Setup token',
})[type] ?? type
const diagnosticAccountRank = (summary: OperatorAccountCapacity) => {
  const status = classifyOperatorAccount(summary).status
  if (summary.lowestRemaining === 0) return 0
  if (status === 'limited' || (summary.lowestRemaining !== null && summary.lowestRemaining <= 20)) return 1
  if (status === 'error' || status === 'disabled' || summary.health !== 'healthy' || summary.lowestRemaining === null) return 2
  return 3
}
const inspectionAccounts = computed(() => summaries.value.slice().sort((left, right) => (
  diagnosticAccountRank(left) - diagnosticAccountRank(right)
  || (left.lowestRemaining ?? Number.POSITIVE_INFINITY) - (right.lowestRemaining ?? Number.POSITIVE_INFINITY)
  || left.account.name.localeCompare(right.account.name)
  || left.account.id - right.account.id
)))
const accountOptions = computed(() => inspectionAccounts.value.map((summary) => ({
  key: String(summary.account.id),
  label: summary.account.name,
  description: `${providerLabel(summary.account.platform)} · ${accountTypeLabel(summary.account.type)}`,
})))
const resolvedAccountKey = computed(() => inspectionAccounts.value.some(
  (summary) => String(summary.account.id) === selectedAccountKey.value,
) ? selectedAccountKey.value : String(inspectionAccounts.value[0]?.account.id ?? ''))
const selectedAccount = computed(() => inspectionAccounts.value.find(
  (summary) => String(summary.account.id) === resolvedAccountKey.value,
) ?? null)
const selectAccount = (value: string | number | boolean | null) => {
  selectedAccountKey.value = String(value ?? '')
  selectedModelKey.value = ''
}
const selectedStatus = computed(() => selectedAccount.value
  ? classifyOperatorAccount(selectedAccount.value)
  : { status: 'disabled' as const, reason: null, until: null })
const selectedUsage = computed(() => selectedAccount.value
  ? props.usageByAccountId[String(selectedAccount.value.account.id)] ?? null
  : null)
const selectedProviderAccountCount = computed(() => providerDetails.value.find(
  (provider) => provider.platform === selectedAccount.value?.account.platform,
)?.accounts.length ?? 0)
const snapshotSourceLabel = computed(() => {
  if (selectedUsage.value?.source === 'passive') return t('admin.stats.capacity.passiveSnapshot')
  if (selectedUsage.value?.source === 'active') return t('admin.stats.capacity.activeSnapshot')
  return t('admin.stats.capacity.snapshotNotReported')
})
const selectedNonModelWindows = computed(() => selectedAccount.value?.windows.filter(
  (window) => !window.key.startsWith('antigravity:'),
) ?? [])
const selectedModelWindows = computed(() => selectedAccount.value?.windows.filter(
  (window) => window.key.startsWith('antigravity:'),
) ?? [])
const limitingWindow = computed(() => selectedAccount.value?.windows.find(
  (window) => window.remainingPercent === selectedAccount.value?.lowestRemaining,
) ?? null)

const resetTimestamp = (value: string | null) => {
  if (!value) return Number.POSITIVE_INFINITY
  const timestamp = new Date(value).getTime()
  return Number.isFinite(timestamp) ? timestamp : Number.POSITIVE_INFINITY
}
const prioritizedModelWindows = computed(() => {
  return selectedModelWindows.value.slice().sort((left, right) => (
    left.remainingPercent - right.remainingPercent
    || resetTimestamp(left.resetsAt) - resetTimestamp(right.resetsAt)
    || left.label.localeCompare(right.label)
  ))
})
const modelOptions = computed(() => prioritizedModelWindows.value.map((window) => ({
  key: window.key,
  label: window.label,
  description: remainingLabel(window.remainingPercent),
})))
const selectedModel = computed(() => prioritizedModelWindows.value.find((window) => window.key === selectedModelKey.value) ?? null)
const visibleModelWindows = computed(() => {
  const visible = prioritizedModelWindows.value.slice(0, MAX_MODEL_WINDOWS)
  const selected = selectedModel.value
  if (!selected || visible.some((window) => window.key === selected.key)) return visible
  return [selected, ...visible.slice(0, MAX_MODEL_WINDOWS - 1)]
})
const selectModel = (value: string | number | boolean | null) => {
  selectedModelKey.value = String(value ?? '')
}
const moreModelCount = computed(() => Math.max(0, prioritizedModelWindows.value.length - MAX_MODEL_WINDOWS))
const limitingModel = computed(() => prioritizedModelWindows.value[0] ?? null)
const modelLimitSummary = computed(() => limitingModel.value
  ? t('admin.stats.capacity.modelMinimum', {
      value: remainingLabel(limitingModel.value.remainingPercent),
      count: prioritizedModelWindows.value.length,
      model: limitingModel.value.label,
    })
  : t('admin.dashboard.capacity.quotaUnknown'))

const windowAriaLabel = (platform: AccountPlatform, window: OperatorWindowCapacity) => {
  const value = window.remainingPercent === null
    ? t('admin.dashboard.capacity.quotaUnknown')
    : `${formatPercent(window.remainingPercent)}% ${t('admin.dashboard.capacity.poolAvailable')}`
  const reset = window.nextReset
    ? t('admin.dashboard.capacity.nextLimitingReset', { time: formatReset(window.nextReset) })
    : t('admin.dashboard.capacity.resetUnknown')
  return `${providerLabel(platform)} ${window.label}: ${value}. ${reset}`
}
const accountWindowAriaLabel = (platform: AccountPlatform, window: OperatorCapacityWindow) => {
  const reset = window.resetsAt
    ? t('admin.dashboard.capacity.resets', { time: formatReset(window.resetsAt) })
    : t('admin.dashboard.capacity.resetUnknown')
  return `${providerLabel(platform)} ${window.label}: ${formatPercent(window.remainingPercent)}% ${t('admin.stats.capacity.normalizedRemaining')}. ${reset}`
}
</script>

<style scoped>
.stats-capacity-section { display: grid; gap: 1.25rem; }
.stats-section-header {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 1rem;
}
.stats-section-header h2 { color: var(--operator-foreground); font-size: 1rem; font-weight: 650; }
.stats-section-header p,
.stats-capacity-count { color: var(--operator-muted-foreground); font-size: 0.8125rem; }
.stats-section-header p { margin-top: 0.2rem; }
.stats-capacity-count { white-space: nowrap; }
.stats-section-state {
  display: flex;
  min-height: 8rem;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  padding: 1.5rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-card);
  color: var(--operator-muted-foreground);
}
.stats-section-state.is-error { justify-content: space-between; min-height: 0; color: #dc2626; }

.stats-capacity-overview {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(16.5rem, 1fr));
  gap: 0.75rem;
}
.stats-capacity-comparison { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; }
.stats-window-panel,
.stats-inspector {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-card);
  box-shadow: var(--operator-shadow-xs);
}
.stats-window-panel > header { padding: 1rem 1.125rem; border-bottom: 1px solid var(--operator-border); }
.stats-window-panel h3,
.stats-inspector h3 { color: var(--operator-foreground); font-size: 0.875rem; font-weight: 650; }
.stats-window-panel header p,
.stats-inspector-control p { margin-top: 0.2rem; color: var(--operator-muted-foreground); font-size: 0.75rem; }
.stats-window-providers { display: grid; }
.stats-window-providers > li { min-width: 0; padding: 1rem 1.125rem; }
.stats-window-providers > li + li { border-top: 1px solid var(--operator-border-subtle); }
.stats-provider-heading { display: flex; align-items: center; gap: 0.5rem; color: var(--operator-foreground); font-size: 0.8125rem; }
.stats-provider-icon {
  display: inline-flex;
  width: 1.75rem;
  height: 1.75rem;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--operator-border-subtle);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
  color: var(--operator-foreground);
}
.stats-window-unsupported { margin-top: 0.75rem; color: var(--operator-muted-foreground); font-size: 0.75rem; }
.stats-window-row { margin-top: 0.875rem; }
.stats-window-row-heading { display: flex; min-width: 0; justify-content: space-between; gap: 0.75rem; color: var(--operator-foreground); font-size: 0.75rem; }
.stats-window-row-heading strong:first-child { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.stats-window-row-heading strong:last-child { flex: 0 0 auto; }
.stats-window-track { height: 0.5rem; margin-top: 0.4rem; overflow: hidden; border-radius: 2px; background: var(--operator-muted); }
.stats-window-track > span { display: block; height: 100%; background: var(--operator-success); }
.stats-window-track > span.is-limited { background: var(--operator-warning); }
.stats-window-track > span.is-exhausted { background: var(--operator-danger); }
.stats-window-row p,
.stats-model-limits p { margin-top: 0.375rem; color: var(--operator-muted-foreground); font-size: 0.6875rem; }
.is-healthy { color: var(--operator-success); }
.is-limited { color: var(--operator-warning); }
.is-exhausted { color: var(--operator-danger); }
.is-unknown { color: var(--operator-muted-foreground); }
.is-active { color: var(--operator-success); }
.is-error { color: var(--operator-danger); }
.is-disabled { color: var(--operator-muted-foreground); }

.stats-inspector-control {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.125rem;
  border-bottom: 1px solid var(--operator-border);
}
.stats-account-select { display: grid; width: min(22rem, 100%); gap: 0.35rem; color: var(--operator-muted-foreground); font-size: 0.6875rem; }
.stats-account-select :deep(.select-trigger),
.stats-target-select :deep(.select-trigger) {
  min-height: 2.25rem;
  padding: 0.375rem 0.625rem;
  border-radius: var(--operator-radius);
  font-size: 0.75rem;
}
.stats-select-option { display: grid; min-width: 0; gap: 0.125rem; }
.stats-select-option strong { overflow: hidden; color: inherit; font-size: 0.75rem; text-overflow: ellipsis; white-space: nowrap; }
.stats-select-option small { overflow: hidden; color: var(--operator-muted-foreground); font-size: 0.625rem; text-overflow: ellipsis; white-space: nowrap; }
.stats-inspector-detail { display: grid; grid-template-columns: minmax(0, 1.5fr) minmax(15rem, 0.75fr); gap: 1rem 1.25rem; padding: 1rem 1.125rem 1.125rem; }
.stats-inspector-summary {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.625rem;
  color: var(--operator-foreground);
  font-size: 0.75rem;
}
.stats-inspector-summary > div { display: grid; min-width: 0; gap: 0.125rem; }
.stats-inspector-summary span { color: var(--operator-muted-foreground); }
.stats-inspector-summary,
.stats-account-note,
.stats-account-models { grid-column: 1 / -1; }
.stats-account-status { padding: 0.25rem 0.5rem; border: 1px solid currentColor; border-radius: 999px; font-size: 0.625rem; line-height: 1; }
.stats-account-note { padding: 0.625rem 0.75rem; border: 1px solid var(--operator-border-subtle); border-radius: var(--operator-radius); background: var(--operator-raised); color: var(--operator-muted-foreground); font-size: 0.6875rem; }
.stats-account-capacity,
.stats-account-operational,
.stats-account-models { min-width: 0; padding-top: 0.875rem; border-top: 1px solid var(--operator-border-subtle); }
.stats-account-capacity h4,
.stats-account-operational h4,
.stats-account-models h4 { color: var(--operator-foreground); font-size: 0.75rem; font-weight: 650; }
.stats-account-capacity header p,
.stats-account-models-heading p,
.stats-account-empty { margin-top: 0.2rem; color: var(--operator-muted-foreground); font-size: 0.6875rem; }
.stats-account-window-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.75rem; margin-top: 0.75rem; }
.stats-account-window-grid article,
.stats-model-limits li { min-width: 0; padding: 0.75rem; border: 1px solid var(--operator-border-subtle); border-radius: var(--operator-radius); background: var(--operator-raised); }
.stats-account-window-grid article > p { margin-top: 0.375rem; color: var(--operator-muted-foreground); font-size: 0.6875rem; }
.stats-account-operational dl { display: grid; gap: 0.625rem; margin-top: 0.75rem; }
.stats-account-operational dl > div { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.25fr); gap: 0.75rem; padding-bottom: 0.5rem; border-bottom: 1px solid var(--operator-border-subtle); font-size: 0.6875rem; }
.stats-account-operational dl > div:last-child { padding-bottom: 0; border-bottom: 0; }
.stats-account-operational dt { color: var(--operator-muted-foreground); }
.stats-account-operational dd { color: var(--operator-foreground); text-align: right; overflow-wrap: anywhere; }
.stats-account-models-heading { display: flex; min-width: 0; align-items: end; justify-content: space-between; gap: 1rem; }
.stats-target-select { display: grid; width: min(19rem, 100%); gap: 0.35rem; }
.stats-target-select > span { color: var(--operator-muted-foreground); font-size: 0.6875rem; }
.stats-model-limits { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 0.75rem; margin-top: 0.875rem; }

@media (max-width: 1024px) {
  .stats-capacity-comparison { grid-template-columns: 1fr; }
  .stats-inspector-detail { grid-template-columns: 1fr; }
  .stats-account-capacity,
  .stats-account-operational { grid-column: 1; }
}

@media (max-width: 640px) {
  .stats-capacity-overview { grid-template-columns: 1fr; }
  .stats-section-header,
  .stats-inspector-control,
  .stats-account-models-heading { align-items: flex-start; flex-direction: column; }
  .stats-account-select,
  .stats-target-select { width: 100%; min-width: 0; }
  .stats-inspector-summary { grid-template-columns: auto minmax(0, 1fr); }
  .stats-account-status { grid-column: 2; justify-self: start; }
  .stats-account-window-grid,
  .stats-model-limits { grid-template-columns: 1fr; }
  .stats-account-operational dl > div { grid-template-columns: 1fr; gap: 0.125rem; }
  .stats-account-operational dd { text-align: left; }
}
</style>
