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
          :title="t('admin.dashboard.capacity.poolTitle')"
          :capacity="normalizedPool"
          test-id="stats-capacity-donut-overall"
        />
        <StatsCapacityDonut
          v-for="provider in providerDetails"
          :key="provider.platform"
          :title="providerLabel(provider.platform)"
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
          <label :for="inspectorSelectId">
            <span class="sr-only">{{ t('admin.stats.capacity.inspectWindow') }}</span>
            <select
              :id="inspectorSelectId"
              class="operator-control"
              :value="resolvedInspectionKey"
              @change="selectInspection"
              @keydown.down.prevent="stepInspection(1)"
              @keydown.up.prevent="stepInspection(-1)"
            >
              <option v-for="option in inspectionOptions" :key="option.key" :value="option.key">
                {{ option.label }}
              </option>
            </select>
          </label>
        </div>

        <div v-if="selectedInspection" class="stats-inspector-detail" aria-live="polite">
          <div class="stats-inspector-summary">
            <span class="stats-provider-icon" aria-hidden="true">
              <ProviderIcon :provider="selectedInspection.platform" :size="18" />
            </span>
            <div>
              <strong>{{ providerLabel(selectedInspection.platform) }} · {{ inspectionDetailLabel }}</strong>
              <span>{{ inspectionCoverageLabel }}</span>
            </div>
            <strong :class="capacityTone(inspectionRemaining)">
              {{ remainingLabel(inspectionRemaining) }}
            </strong>
          </div>

          <template v-if="selectedInspection.kind === 'window'">
            <div class="stats-inspector-tools">
              <p data-testid="stats-inspector-account-summary">{{ inspectionAccountSummary }}</p>
              <label v-if="inspectionAccounts.length > MAX_INSPECTION_ACCOUNTS" class="stats-target-select">
                <span>{{ t('admin.stats.capacity.inspectAccount') }}</span>
                <Select
                  :model-value="selectedInspectionAccountKey"
                  :options="inspectionAccountOptions"
                  value-key="key"
                  label-key="label"
                  :searchable="true"
                  :aria-label="t('admin.stats.capacity.inspectAccount')"
                  :search-placeholder="t('admin.stats.capacity.searchAccounts')"
                  @update:model-value="selectInspectionAccount"
                />
              </label>
            </div>
            <ul
              class="stats-inspector-accounts"
              :aria-label="t('admin.stats.capacity.windowContributions', { window: selectedInspection.window.label })"
            >
              <li
                v-for="entry in visibleInspectionAccounts"
                :key="entry.summary.account.id"
                :class="{ 'is-unknown': entry.remainingPercent === null }"
                data-testid="stats-inspector-account-row"
              >
                <span>{{ entry.summary.account.name }}</span>
                <span v-if="entry.remainingPercent !== null">
                  {{ t('admin.dashboard.capacity.remaining', { value: formatPercent(entry.remainingPercent) }) }} ·
                  {{ t('admin.dashboard.capacity.poolContribution', { value: formatPercent(entry.contributionPercent as number) }) }}
                </span>
                <span v-else>{{ t('admin.dashboard.capacity.quotaUnknown') }}</span>
              </li>
            </ul>
          </template>

          <template v-else>
            <div class="stats-inspector-tools">
              <p data-testid="stats-model-limit-summary">{{ modelLimitSummary }}</p>
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
            <ul class="stats-model-limits" :aria-label="t('admin.stats.capacity.modelLimits')">
              <li v-for="window in visibleModelWindows" :key="window.key" data-testid="stats-model-limit-row">
                <div class="stats-window-row-heading">
                  <strong :title="window.label">{{ window.label }}</strong>
                  <strong :class="capacityTone(window.remainingPercent)">{{ remainingLabel(window.remainingPercent) }}</strong>
                </div>
                <div
                  class="stats-window-track"
                  role="progressbar"
                  :aria-label="windowAriaLabel(selectedInspection.platform, window)"
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
                  {{ t('admin.stats.capacity.windowCoverage', { known: window.knownCount, unknown: window.unknownCount }) }}
                  <template v-if="window.nextReset">
                    · {{ t('admin.dashboard.capacity.resets', { time: formatCompactReset(window.nextReset) }) }}
                  </template>
                </p>
              </li>
            </ul>
          </template>
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
const inspectorSelectId = 'stats-capacity-inspector-select'
const MAX_INSPECTION_ACCOUNTS = 5
const MAX_MODEL_WINDOWS = 3

const providerLabel = (platform: AccountPlatform) => ({
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  gemini: 'Gemini',
  antigravity: 'Antigravity',
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

interface WindowInspectionOption {
  key: string
  label: string
  platform: AccountPlatform
  kind: 'window'
  window: OperatorWindowCapacity
}
interface ModelInspectionOption {
  key: string
  label: string
  platform: AccountPlatform
  kind: 'models'
  windows: OperatorWindowCapacity[]
}
type InspectionOption = WindowInspectionOption | ModelInspectionOption

const inspectionOptions = computed<InspectionOption[]>(() => providerDetails.value.flatMap((provider) => {
  const options: InspectionOption[] = provider.windows
    .filter((window) => !isModelWindow(window))
    .map((window) => ({
      key: `${provider.platform}:${window.key}`,
      label: `${providerLabel(provider.platform)} · ${window.label}`,
      platform: provider.platform,
      kind: 'window',
      window,
    }))
  if (provider.modelWindows.length) {
    options.push({
      key: `${provider.platform}:model-limits`,
      label: `${providerLabel(provider.platform)} · ${t('admin.stats.capacity.modelLimits')} (${provider.modelWindows.length})`,
      platform: provider.platform,
      kind: 'models',
      windows: provider.modelWindows,
    })
  }
  return options
}))
const selectedInspectionKey = ref('')
const resolvedInspectionKey = computed(() => inspectionOptions.value.some((option) => option.key === selectedInspectionKey.value)
  ? selectedInspectionKey.value
  : inspectionOptions.value[0]?.key ?? '')
const selectedInspection = computed(() => inspectionOptions.value.find((option) => option.key === resolvedInspectionKey.value) ?? null)
const selectedInspectionAccountKey = ref('')
const selectedModelKey = ref('')
const setInspectionKey = (value: string) => {
  selectedInspectionKey.value = value
  selectedInspectionAccountKey.value = ''
  selectedModelKey.value = ''
}
const selectInspection = (event: Event) => setInspectionKey((event.target as HTMLSelectElement).value)
const stepInspection = (direction: -1 | 1) => {
  const currentIndex = inspectionOptions.value.findIndex((option) => option.key === resolvedInspectionKey.value)
  const nextIndex = Math.min(
    inspectionOptions.value.length - 1,
    Math.max(0, currentIndex + direction),
  )
  setInspectionKey(inspectionOptions.value[nextIndex]?.key ?? '')
}

interface InspectionAccount {
  summary: OperatorAccountCapacity
  remainingPercent: number | null
  contributionPercent: number | null
}
const diagnosticAccountRank = (entry: InspectionAccount) => {
  if (entry.remainingPercent === 0) return 0
  if (entry.remainingPercent !== null && entry.remainingPercent <= 20) return 1
  if (classifyOperatorAccount(entry.summary).status !== 'active' || entry.summary.health !== 'healthy') return 2
  if (entry.remainingPercent === null) return 3
  return 4
}
const inspectionAccounts = computed<InspectionAccount[]>(() => {
  if (selectedInspection.value?.kind !== 'window') return []
  return [
    ...selectedInspection.value.window.segments.map((segment) => ({
      summary: segment.summary,
      remainingPercent: segment.remainingPercent,
      contributionPercent: segment.contributionPercent,
    })),
    ...selectedInspection.value.window.unknownAccounts.map((summary) => ({
      summary,
      remainingPercent: null,
      contributionPercent: null,
    })),
  ].sort((left, right) => (
    diagnosticAccountRank(left) - diagnosticAccountRank(right)
    || (left.remainingPercent ?? Number.POSITIVE_INFINITY) - (right.remainingPercent ?? Number.POSITIVE_INFINITY)
    || left.summary.account.name.localeCompare(right.summary.account.name)
    || left.summary.account.id - right.summary.account.id
  ))
})
const inspectionAccountOptions = computed(() => inspectionAccounts.value.map((entry) => ({
  key: String(entry.summary.account.id),
  label: entry.summary.account.name,
  description: entry.summary.identity || `Account ${entry.summary.account.id}`,
})))
const selectedInspectionAccount = computed(() => inspectionAccounts.value.find(
  (entry) => String(entry.summary.account.id) === selectedInspectionAccountKey.value,
) ?? null)
const visibleInspectionAccounts = computed(() => {
  const visible = inspectionAccounts.value.slice(0, MAX_INSPECTION_ACCOUNTS)
  const selected = selectedInspectionAccount.value
  if (!selected || visible.some((entry) => entry.summary.account.id === selected.summary.account.id)) return visible
  return [selected, ...visible.slice(0, MAX_INSPECTION_ACCOUNTS - 1)]
})
const selectInspectionAccount = (value: string | number | boolean | null) => {
  selectedInspectionAccountKey.value = String(value ?? '')
}
const inspectionAccountSummary = computed(() => t('admin.stats.capacity.inspectionSummary', {
  total: inspectionAccounts.value.length,
  exhausted: inspectionAccounts.value.filter((entry) => entry.remainingPercent === 0).length,
  unknown: inspectionAccounts.value.filter((entry) => entry.remainingPercent === null).length,
  hidden: Math.max(0, inspectionAccounts.value.length - MAX_INSPECTION_ACCOUNTS),
}))

const resetTimestamp = (value: string | null) => {
  if (!value) return Number.POSITIVE_INFINITY
  const timestamp = new Date(value).getTime()
  return Number.isFinite(timestamp) ? timestamp : Number.POSITIVE_INFINITY
}
const prioritizedModelWindows = computed(() => {
  if (selectedInspection.value?.kind !== 'models') return []
  return selectedInspection.value.windows.slice().sort((left, right) => (
    (left.remainingPercent ?? Number.POSITIVE_INFINITY) - (right.remainingPercent ?? Number.POSITIVE_INFINITY)
    || resetTimestamp(left.nextReset) - resetTimestamp(right.nextReset)
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

const inspectionDetailLabel = computed(() => {
  const inspection = selectedInspection.value
  if (!inspection) return ''
  return inspection.kind === 'window' ? inspection.window.label : t('admin.stats.capacity.modelLimits')
})
const inspectionRemaining = computed(() => {
  const inspection = selectedInspection.value
  if (!inspection) return null
  return inspection.kind === 'window' ? inspection.window.remainingPercent : limitingModel.value?.remainingPercent ?? null
})
const inspectionCoverageLabel = computed(() => {
  const inspection = selectedInspection.value
  if (!inspection) return ''
  if (inspection.kind === 'models') return modelLimitSummary.value
  return t('admin.stats.capacity.windowCoverage', {
    known: inspection.window.knownCount,
    unknown: inspection.window.unknownCount,
  })
})

const windowAriaLabel = (platform: AccountPlatform, window: OperatorWindowCapacity) => {
  const value = window.remainingPercent === null
    ? t('admin.dashboard.capacity.quotaUnknown')
    : `${formatPercent(window.remainingPercent)}% ${t('admin.dashboard.capacity.poolAvailable')}`
  const reset = window.nextReset
    ? t('admin.dashboard.capacity.nextLimitingReset', { time: formatReset(window.nextReset) })
    : t('admin.dashboard.capacity.resetUnknown')
  return `${providerLabel(platform)} ${window.label}: ${value}. ${reset}`
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
  grid-template-columns: repeat(auto-fit, minmax(9.5rem, 1fr));
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

.stats-inspector-control {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.125rem;
  border-bottom: 1px solid var(--operator-border);
}
.stats-inspector-control label { min-width: min(18rem, 100%); }
.stats-inspector-control select { width: 100%; min-height: 2.25rem; padding: 0.375rem 2rem 0.375rem 0.625rem; font-size: 0.75rem; }
.stats-inspector-detail { padding: 1rem 1.125rem; }
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
.stats-inspector-tools {
  display: flex;
  min-width: 0;
  align-items: end;
  justify-content: space-between;
  gap: 1rem;
  margin-top: 0.875rem;
  color: var(--operator-muted-foreground);
  font-size: 0.6875rem;
}
.stats-target-select { display: grid; width: min(19rem, 100%); gap: 0.35rem; }
.stats-target-select :deep(.select-trigger) {
  min-height: 2.25rem;
  padding: 0.375rem 0.625rem;
  border-radius: var(--operator-radius);
  font-size: 0.75rem;
}
.stats-inspector-accounts,
.stats-model-limits { display: grid; gap: 0.375rem; margin-top: 0.875rem; }
.stats-inspector-accounts li {
  display: flex;
  justify-content: space-between;
  gap: 0.75rem;
  padding-top: 0.375rem;
  border-top: 1px solid var(--operator-border-subtle);
  color: var(--operator-foreground);
  font-size: 0.6875rem;
}
.stats-inspector-accounts li span:first-child { min-width: 0; overflow-wrap: anywhere; }
.stats-inspector-accounts li span:last-child { color: var(--operator-muted-foreground); text-align: right; }
.stats-model-limits { grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 0.75rem; }
.stats-model-limits li {
  min-width: 0;
  padding: 0.75rem;
  border: 1px solid var(--operator-border-subtle);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
}

@media (max-width: 1024px) {
  .stats-capacity-comparison { grid-template-columns: 1fr; }
}

@media (max-width: 640px) {
  .stats-section-header,
  .stats-inspector-control,
  .stats-inspector-tools { align-items: flex-start; flex-direction: column; }
  .stats-inspector-control label,
  .stats-target-select { width: 100%; min-width: 0; }
  .stats-inspector-accounts li { flex-direction: column; gap: 0.125rem; }
  .stats-inspector-accounts li span:last-child { text-align: left; }
  .stats-model-limits { grid-template-columns: 1fr; }
}
</style>
