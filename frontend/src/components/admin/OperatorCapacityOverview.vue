<template>
  <section
    class="operator-capacity"
    :class="{ 'operator-capacity-compact': compact }"
    :aria-label="t('admin.dashboard.capacity.title')"
  >
    <header class="operator-capacity-header">
      <div>
        <h2>{{ t('admin.dashboard.capacity.title') }}</h2>
        <p>{{ t('admin.dashboard.capacity.description') }}</p>
      </div>
      <div v-if="accounts.length" class="operator-capacity-counts" aria-label="Account fleet summary">
        <span>{{ t('admin.dashboard.capacity.accountCount', { count: accountCount }) }}</span>
        <span class="is-schedulable">
          {{ t('admin.dashboard.capacity.schedulableCount', { count: schedulableCount }) }}
        </span>
        <span v-if="warningCount" class="is-warning">
          {{ t('admin.dashboard.capacity.warningCount', { count: warningCount }) }}
        </span>
        <span v-if="unavailableCount" class="is-critical">
          {{ t('admin.dashboard.capacity.unavailableCount', { count: unavailableCount }) }}
        </span>
      </div>
    </header>

    <div
      v-if="error"
      role="alert"
      data-testid="capacity-load-error"
      class="operator-capacity-error"
    >
      <span>{{ t('admin.dashboard.capacity.loadFailed') }}</span>
      <button type="button" class="btn btn-secondary" @click="emit('retry')">
        {{ t('admin.dashboard.retry') }}
      </button>
    </div>

    <div v-if="loading && !accounts.length" class="operator-capacity-empty" role="status">
      <LoadingSpinner size="sm" />
      <span>{{ t('admin.dashboard.capacity.loading') }}</span>
    </div>
    <div v-else-if="!accounts.length && !error" class="operator-capacity-empty">
      {{ t('admin.dashboard.capacity.empty') }}
    </div>

    <template v-else-if="accounts.length">
      <section
        v-if="compact"
        class="operator-pool-capacity"
        data-testid="account-pool-capacity"
        :aria-labelledby="poolTitleId"
      >
        <div class="operator-pool-heading">
          <div>
            <h3 :id="poolTitleId">{{ t('admin.dashboard.capacity.poolTitle') }}</h3>
            <p>{{ t('admin.dashboard.capacity.poolDescription') }}</p>
          </div>
          <span v-if="normalizedPool.unknownCount" class="operator-pool-unknown-count">
            {{ t('admin.dashboard.capacity.poolUnknownExcluded', { count: normalizedPool.unknownCount }) }}
          </span>
        </div>

        <div class="operator-pool-layout">
          <div class="operator-pool-chart">
            <svg
              viewBox="0 0 120 120"
              role="img"
              :aria-label="poolAriaLabel"
            >
              <title>{{ poolAriaLabel }}</title>
              <circle
                class="operator-pool-ring"
                :class="normalizedPool.usedPercent === null ? 'is-unknown' : 'is-used'"
                cx="60"
                cy="60"
                r="46"
                pathLength="100"
                tabindex="0"
              >
                <title>
                  {{ normalizedPool.usedPercent === null
                    ? t('admin.dashboard.capacity.quotaUnknown')
                    : t('admin.dashboard.capacity.poolUsedValue', { value: formatPercent(normalizedPool.usedPercent) }) }}
                </title>
              </circle>
              <circle
                v-for="segment in poolChartSegments"
                :key="segment.summary.account.id"
                class="operator-pool-ring operator-pool-segment"
                :class="segmentTone(segment.index)"
                cx="60"
                cy="60"
                r="46"
                pathLength="100"
                tabindex="0"
                data-testid="account-pool-segment"
                :style="{
                  strokeDasharray: `${segment.contributionPercent} ${100 - segment.contributionPercent}`,
                  strokeDashoffset: `${-segment.offset}`,
                }"
              >
                <title>
                  {{ segment.summary.account.name }}:
                  {{ t('admin.dashboard.capacity.remaining', { value: formatPercent(segment.summary.lowestRemaining) }) }},
                  {{ t('admin.dashboard.capacity.poolContribution', { value: formatPercent(segment.contributionPercent) }) }}
                </title>
              </circle>
            </svg>
            <div class="operator-pool-chart-label" aria-hidden="true">
              <strong>
                {{ normalizedPool.remainingPercent === null
                  ? t('common.unknown')
                  : `${formatPercent(normalizedPool.remainingPercent)}%` }}
              </strong>
              <span>{{ t('admin.dashboard.capacity.poolAvailable') }}</span>
            </div>
          </div>

          <ul class="operator-pool-legend" :aria-label="t('admin.dashboard.capacity.poolLegend')">
            <li v-if="normalizedPool.usedPercent !== null">
              <span class="operator-pool-swatch is-used" aria-hidden="true" />
              <span class="operator-pool-legend-label">
                <strong>{{ t('admin.dashboard.capacity.poolUsed') }}</strong>
                <small>{{ t('admin.dashboard.capacity.poolNormalized') }}</small>
              </span>
              <strong>{{ formatPercent(normalizedPool.usedPercent) }}%</strong>
            </li>
            <li v-for="segment in poolChartSegments" :key="`legend-${segment.summary.account.id}`">
              <span
                class="operator-pool-swatch"
                :class="segmentTone(segment.index)"
                aria-hidden="true"
              />
              <span class="operator-pool-legend-label">
                <strong>{{ segment.summary.account.name }}</strong>
                <small>{{ providerLabel(segment.summary.account.platform) }}</small>
              </span>
              <span class="operator-pool-legend-value">
                <strong>{{ formatPercent(segment.summary.lowestRemaining) }}%</strong>
                <small>
                  {{ t('admin.dashboard.capacity.poolContribution', { value: formatPercent(segment.contributionPercent) }) }}
                </small>
              </span>
            </li>
          </ul>
        </div>

        <p v-if="normalizedPool.unknownAccounts.length" class="operator-pool-unknown-list">
          <strong>{{ t('admin.dashboard.capacity.quotaUnknown') }}:</strong>
          {{ normalizedPool.unknownAccounts.map((summary) => summary.account.name).join(', ') }}
        </p>
      </section>

      <section class="operator-capacity-fleet">
        <div class="operator-capacity-section-heading">
          <h3>{{ t('admin.dashboard.capacity.byProvider') }}</h3>
          <span v-if="unknownCount">
            {{ t('admin.dashboard.capacity.unknownCount', { count: unknownCount }) }}
          </span>
        </div>
        <div class="operator-capacity-fleet-grid">
          <article
            v-for="provider in providers"
            :key="`fleet-${provider.platform}`"
            class="operator-capacity-fleet-provider"
          >
            <div class="operator-capacity-fleet-label">
              <div>
                <strong>{{ providerLabel(provider.platform) }}</strong>
                <span>{{ t('admin.dashboard.capacity.providerAccounts', { count: provider.accounts.length }) }}</span>
              </div>
              <strong :class="capacityTone(provider.lowestRemaining)">
                {{ percentLabel(provider.lowestRemaining) }}
              </strong>
            </div>
            <div
              class="operator-capacity-track"
              :class="{ 'is-unknown': provider.lowestRemaining === null }"
              role="progressbar"
              :aria-label="`${providerLabel(provider.platform)} ${percentLabel(provider.lowestRemaining)}`"
              aria-valuemin="0"
              aria-valuemax="100"
              :aria-valuenow="provider.lowestRemaining === null ? undefined : Math.round(provider.lowestRemaining)"
            >
              <span
                v-if="provider.lowestRemaining !== null"
                :class="capacityTone(provider.lowestRemaining)"
                :style="{ width: `${provider.lowestRemaining}%` }"
              />
            </div>
            <ul v-if="compact" class="operator-capacity-compact-accounts">
              <li
                v-for="summary in provider.accounts"
                :key="summary.account.id"
                class="operator-capacity-compact-account"
              >
                <span :title="summary.account.name">{{ summary.account.name }}</span>
                <span class="operator-capacity-compact-meta">
                  <span :class="['operator-capacity-status', `is-${summary.health}`]">
                    <span aria-hidden="true" />
                    {{ healthLabel(summary.health) }}
                  </span>
                  <strong :class="capacityTone(summary.lowestRemaining)">
                    {{ percentLabel(summary.lowestRemaining) }}
                  </strong>
                </span>
              </li>
            </ul>
          </article>
        </div>
      </section>

      <div v-if="!compact" class="operator-capacity-providers">
        <section v-for="provider in providers" :key="provider.platform" class="operator-capacity-provider">
          <header class="operator-capacity-provider-header">
            <button
              type="button"
              class="operator-capacity-provider-toggle"
              :aria-expanded="!isProviderCollapsed(provider.platform)"
              :aria-controls="providerPanelId(provider.platform)"
              :data-testid="`provider-toggle-${provider.platform}`"
              @click="toggleProvider(provider.platform)"
            >
              <span class="operator-capacity-provider-identity">
                <Icon
                  name="chevronDown"
                  size="sm"
                  :class="{ 'is-collapsed': isProviderCollapsed(provider.platform) }"
                  aria-hidden="true"
                />
                <span>
                  <span class="operator-capacity-provider-name">{{ providerLabel(provider.platform) }}</span>
                  <span>{{ t('admin.dashboard.capacity.providerAccounts', { count: provider.accounts.length }) }}</span>
                </span>
              </span>
              <span class="operator-capacity-provider-summary">
                <span>{{ t('admin.dashboard.capacity.lowestRemaining') }}</span>
                <strong :class="capacityTone(provider.lowestRemaining)">
                  {{ percentLabel(provider.lowestRemaining) }}
                </strong>
              </span>
            </button>
          </header>

          <div
            v-show="!isProviderCollapsed(provider.platform)"
            :id="providerPanelId(provider.platform)"
            class="operator-capacity-account-list"
          >
            <article
              v-for="summary in provider.accounts"
              :key="summary.account.id"
              class="operator-capacity-account"
            >
              <header class="operator-capacity-account-heading">
                <div class="operator-capacity-account-title">
                  <h4>{{ summary.account.name }}</h4>
                  <p v-if="summary.identity" :title="summary.identity">{{ summary.identity }}</p>
                  <p v-else>#{{ summary.account.id }} &middot; {{ accountTypeLabel(summary.account.type) }}</p>
                </div>
                <div class="operator-capacity-account-primary">
                  <span :class="['operator-capacity-status', `is-${summary.health}`]">
                    <span aria-hidden="true" />
                    {{ healthLabel(summary.health) }}
                  </span>
                  <strong class="operator-capacity-account-remaining" :class="capacityTone(summary.lowestRemaining)">
                    {{ percentLabel(summary.lowestRemaining) }}
                  </strong>
                  <div v-if="$slots['account-actions']" class="operator-capacity-account-actions">
                    <slot name="account-actions" :account="summary.account" />
                  </div>
                </div>
              </header>

              <div class="operator-capacity-account-body">
                <div class="operator-capacity-account-meta">
                  <div class="operator-capacity-meta-line">
                    <span>{{ accountTypeLabel(summary.account.type) }}</span>
                    <span v-if="summary.tier">{{ t('admin.dashboard.capacity.plan') }} {{ summary.tier }}</span>
                    <span v-if="modelMappingCount(summary.account)">
                      {{ t('admin.dashboard.capacity.modelCount', { count: modelMappingCount(summary.account) }) }}
                    </span>
                    <span :class="summary.account.schedulable ? 'is-schedulable' : 'is-unschedulable'">
                      {{ summary.account.schedulable
                        ? t('admin.dashboard.capacity.schedulable')
                        : t('admin.dashboard.capacity.notSchedulable') }}
                    </span>
                  </div>
                  <p v-if="summary.groups.length" class="operator-capacity-routing" :title="summary.groups.join(', ')">
                    <strong>{{ t('admin.dashboard.capacity.routing') }}</strong>
                    {{ summary.groups.join(', ') }}
                  </p>
                  <p v-if="summary.error" class="operator-capacity-account-error">{{ summary.error }}</p>
                </div>

                <div v-if="summary.windows.length" class="operator-capacity-window-grid">
                  <div v-for="window in summary.windows" :key="window.key" class="operator-capacity-window">
                    <div class="operator-capacity-window-label">
                      <span :title="window.label">{{ window.label }}</span>
                      <strong :class="capacityTone(window.remainingPercent)">
                        {{ percentLabel(window.remainingPercent) }}
                      </strong>
                    </div>
                    <div
                      class="operator-capacity-track"
                      role="progressbar"
                      :aria-label="`${window.label} ${percentLabel(window.remainingPercent)}`"
                      aria-valuemin="0"
                      aria-valuemax="100"
                      :aria-valuenow="Math.round(window.remainingPercent)"
                    >
                      <span
                        :class="capacityTone(window.remainingPercent)"
                        :style="{ width: `${window.remainingPercent}%` }"
                      />
                    </div>
                    <div class="operator-capacity-window-detail">
                      <span>{{ t('admin.dashboard.capacity.used', { value: formatPercent(100 - window.remainingPercent) }) }}</span>
                      <span v-if="window.resetsAt" :title="formatReset(window.resetsAt)">
                        {{ t('admin.dashboard.capacity.resets', { time: formatReset(window.resetsAt) }) }}
                      </span>
                      <span v-else-if="window.kind === 'spend'">{{ t('admin.dashboard.capacity.noReset') }}</span>
                      <span v-else>{{ t('admin.dashboard.capacity.resetUnknown') }}</span>
                    </div>
                  </div>
                </div>
                <div v-else class="operator-capacity-quota-unknown">
                  <strong>{{ t('admin.dashboard.capacity.quotaUnknown') }}</strong>
                  <span>{{ summary.error || t('admin.dashboard.capacity.quotaUnknownHint') }}</span>
                </div>
              </div>
            </article>
          </div>
        </section>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTimeToMinute } from '@/utils/format'
import {
  buildNormalizedPoolCapacity,
  buildProviderCapacity,
  type OperatorAccountHealth,
} from '@/utils/operatorCapacity'
import type { Account, AccountPlatform, AccountType, AccountUsageInfo } from '@/types'

const props = withDefaults(defineProps<{
  accounts: Account[]
  usageByAccountId?: Record<string, AccountUsageInfo | null | undefined>
  errorsByAccountId?: Record<string, string | null | undefined>
  loading?: boolean
  error?: boolean
  compact?: boolean
  totalCount?: number
}>(), {
  usageByAccountId: () => ({}),
  errorsByAccountId: () => ({}),
  loading: false,
  error: false,
  compact: false,
  totalCount: undefined,
})

const emit = defineEmits<{ retry: [] }>()
const { t } = useI18n()
const providers = computed(() => buildProviderCapacity(
  props.accounts,
  props.usageByAccountId,
  props.errorsByAccountId,
))
const summaries = computed(() => providers.value.flatMap((provider) => provider.accounts))
const normalizedPool = computed(() => buildNormalizedPoolCapacity(summaries.value))
const poolChartSegments = computed(() => {
  let offset = 0
  return normalizedPool.value.segments.map((segment, index) => {
    const chartSegment = { ...segment, index, offset }
    offset += segment.contributionPercent
    return chartSegment
  })
})
const accountCount = computed(() => props.totalCount ?? props.accounts.length)
const schedulableCount = computed(() => summaries.value.filter((summary) => summary.account.schedulable).length)
const unavailableCount = computed(() => summaries.value.filter(
  (summary) => summary.health === 'error' || summary.health === 'inactive',
).length)
const warningCount = computed(() => summaries.value.filter((summary) => (
  summary.health !== 'healthy'
  && summary.health !== 'error'
  && summary.health !== 'inactive'
) || (summary.lowestRemaining !== null && summary.lowestRemaining <= 20)).length)
const unknownCount = computed(() => providers.value.reduce((total, provider) => total + provider.unknownCount, 0))
const poolTitleId = 'operator-account-pool-title'
const PROVIDER_COLLAPSE_STORAGE_KEY = 'operator-capacity-collapsed-providers'

const loadCollapsedProviders = (): Set<AccountPlatform> => {
  if (typeof window === 'undefined') return new Set()
  try {
    const stored = JSON.parse(sessionStorage.getItem(PROVIDER_COLLAPSE_STORAGE_KEY) ?? '[]')
    return Array.isArray(stored) ? new Set(stored.filter((value): value is AccountPlatform => typeof value === 'string')) : new Set()
  } catch {
    return new Set()
  }
}

const collapsedProviders = ref(loadCollapsedProviders())
const isProviderCollapsed = (platform: AccountPlatform) => collapsedProviders.value.has(platform)
const providerPanelId = (platform: AccountPlatform) => `operator-provider-${platform}-accounts`
const toggleProvider = (platform: AccountPlatform) => {
  const next = new Set(collapsedProviders.value)
  if (next.has(platform)) next.delete(platform)
  else next.add(platform)
  collapsedProviders.value = next
  try {
    sessionStorage.setItem(PROVIDER_COLLAPSE_STORAGE_KEY, JSON.stringify([...next]))
  } catch {
    // Session persistence is optional; the accordion still works without storage.
  }
}

const recordEntries = (value: unknown): Array<[string, string]> => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return []
  return Object.entries(value)
    .filter((entry): entry is [string, string] => typeof entry[1] === 'string' && entry[1].trim().length > 0)
}

const modelMappings = (account: Account) => [
  ...recordEntries(account.credentials?.model_mapping).map(([source, target]) => ({
    key: `model:${source}:${target}`,
    kind: 'model' as const,
    source,
    target,
  })),
  ...recordEntries(account.credentials?.compact_model_mapping).map(([source, target]) => ({
    key: `compact:${source}:${target}`,
    kind: 'compact' as const,
    source,
    target,
  })),
]

const modelMappingCount = (account: Account) => modelMappings(account).length

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

const accountTypeLabel = (type: AccountType) => ({
  oauth: 'OAuth',
  'setup-token': 'Setup token',
  apikey: 'API key',
  upstream: 'Upstream',
  bedrock: 'Bedrock',
  service_account: 'Service account',
})[type]

const healthLabel = (health: OperatorAccountHealth) => t(`admin.dashboard.capacity.health.${health}`)

const formatPercent = (value: number) => Number.isInteger(value)
  ? String(value)
  : value.toFixed(1).replace(/\.0$/, '')

const percentLabel = (value: number | null) => value === null
  ? t('common.unknown')
  : t('admin.dashboard.capacity.remaining', { value: formatPercent(value) })

const capacityTone = (value: number | null) => {
  if (value === null) return 'is-unknown'
  if (value <= 20) return 'is-critical'
  if (value <= 50) return 'is-warning'
  return 'is-healthy'
}

const segmentTone = (index: number) => `is-segment-${index % 5}`
const poolAriaLabel = computed(() => normalizedPool.value.remainingPercent === null
  ? `${t('admin.dashboard.capacity.poolTitle')}: ${t('admin.dashboard.capacity.quotaUnknown')}`
  : `${t('admin.dashboard.capacity.poolTitle')}: ${formatPercent(normalizedPool.value.remainingPercent)}% ${t('admin.dashboard.capacity.poolAvailable')}, ${formatPercent(normalizedPool.value.usedPercent as number)}% ${t('admin.dashboard.capacity.poolUsed')}`)

const formatReset = (value: string) => formatDateTimeToMinute(value) || t('common.unknown')
</script>

<style scoped>
.operator-capacity {
  overflow: hidden;
  border: 1px solid var(--operator-border);
  border-radius: 0.5rem;
  background: var(--operator-card);
  box-shadow: var(--operator-shadow-xs);
}

.operator-capacity-header,
.operator-capacity-provider-header,
.operator-capacity-account-heading,
.operator-capacity-window-label,
.operator-capacity-window-detail,
.operator-capacity-fleet-label,
.operator-capacity-section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.operator-capacity-header {
  padding: 1.25rem 1.5rem;
  border-bottom: 1px solid var(--operator-border);
}

.operator-capacity-header h2,
.operator-capacity-section-heading h3,
.operator-capacity-provider-name,
.operator-capacity-account h4 {
  color: var(--operator-foreground);
  font-weight: 650;
}

.operator-capacity-header h2 { font-size: 1.125rem; }
.operator-capacity-header p {
  margin-top: 0.25rem;
  color: var(--operator-muted-foreground);
  font-size: 0.875rem;
  line-height: 1.5;
}

.operator-capacity-counts,
.operator-capacity-meta-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.operator-capacity-counts span,
.operator-capacity-meta-line span {
  padding: 0.3125rem 0.5625rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-muted);
  white-space: nowrap;
}

.operator-capacity-counts .is-schedulable,
.operator-capacity-meta-line .is-schedulable { color: var(--operator-success); }
.operator-capacity-counts .is-warning,
.operator-capacity-meta-line .is-unschedulable { color: var(--operator-warning); }
.operator-capacity-counts .is-critical { color: var(--operator-destructive); }

.operator-capacity-error {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.75rem 1.5rem;
  border-bottom: 1px solid color-mix(in oklch, var(--operator-destructive) 45%, var(--operator-border));
  background: color-mix(in oklch, var(--operator-destructive) 8%, var(--operator-card));
  color: var(--operator-destructive);
  font-size: 0.875rem;
}

.operator-capacity-empty {
  display: flex;
  min-height: 8rem;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  color: var(--operator-muted-foreground);
  font-size: 0.875rem;
}

.operator-pool-capacity {
  padding: 1.25rem 1.5rem 1.5rem;
  border-bottom: 1px solid var(--operator-border);
}

.operator-pool-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.operator-pool-heading h3 {
  color: var(--operator-foreground);
  font-size: 1rem;
  font-weight: 650;
}

.operator-pool-heading p {
  max-width: 52rem;
  margin-top: 0.25rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  line-height: 1.45;
}

.operator-pool-unknown-count {
  flex: 0 0 auto;
  padding: 0.3125rem 0.5625rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-muted);
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.operator-pool-layout {
  display: grid;
  margin-top: 1.125rem;
  grid-template-columns: minmax(11rem, 15rem) minmax(0, 1fr);
  align-items: center;
  gap: 1.5rem 2rem;
}

.operator-pool-chart {
  position: relative;
  width: min(100%, 14rem);
  aspect-ratio: 1;
  justify-self: center;
}

.operator-pool-chart svg {
  display: block;
  width: 100%;
  height: 100%;
  overflow: visible;
  transform: rotate(-90deg);
}

.operator-pool-ring {
  fill: none;
  stroke-width: 16;
}

.operator-pool-ring.is-used {
  stroke: color-mix(in oklch, var(--operator-muted-foreground) 55%, var(--operator-track));
}

.operator-pool-ring.is-unknown {
  stroke: var(--operator-track);
  stroke-dasharray: 2 3;
}

.operator-pool-segment {
  transition: stroke-width 150ms ease;
}

.operator-pool-segment:hover,
.operator-pool-segment:focus-visible {
  stroke-width: 19;
  outline: none;
}

.operator-pool-segment:focus-visible {
  filter: drop-shadow(0 0 2px var(--operator-focus));
}

.operator-pool-chart-label {
  position: absolute;
  inset: 25%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  text-align: center;
}

.operator-pool-chart-label strong {
  color: var(--operator-foreground);
  font-size: 1.75rem;
  font-weight: 700;
  line-height: 1.1;
}

.operator-pool-chart-label span {
  margin-top: 0.25rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.operator-pool-legend {
  display: grid;
  min-width: 0;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.375rem 1.5rem;
}

.operator-pool-legend li {
  display: grid;
  min-width: 0;
  min-height: 3rem;
  padding: 0.5rem 0;
  grid-template-columns: 0.75rem minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.625rem;
  border-bottom: 1px solid var(--operator-border-subtle);
}

.operator-pool-swatch {
  width: 0.75rem;
  height: 0.75rem;
  border-radius: 999px;
  background: currentColor;
}

.operator-pool-swatch.is-used {
  color: color-mix(in oklch, var(--operator-muted-foreground) 55%, var(--operator-track));
}

.is-segment-0 { color: var(--operator-success-fill); stroke: var(--operator-success-fill); }
.is-segment-1 {
  color: color-mix(in oklch, var(--operator-success-fill) 62%, var(--operator-foreground));
  stroke: color-mix(in oklch, var(--operator-success-fill) 62%, var(--operator-foreground));
}
.is-segment-2 { color: var(--operator-warning-fill); stroke: var(--operator-warning-fill); }
.is-segment-3 {
  color: color-mix(in oklch, var(--operator-foreground) 68%, var(--operator-border));
  stroke: color-mix(in oklch, var(--operator-foreground) 68%, var(--operator-border));
}
.is-segment-4 {
  color: color-mix(in oklch, var(--operator-warning-fill) 55%, var(--operator-foreground));
  stroke: color-mix(in oklch, var(--operator-warning-fill) 55%, var(--operator-foreground));
}

.operator-pool-legend-label,
.operator-pool-legend-value {
  display: flex;
  min-width: 0;
  flex-direction: column;
}

.operator-pool-legend-label strong {
  overflow: hidden;
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.operator-pool-legend small {
  margin-top: 0.125rem;
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
  line-height: 1.25;
}

.operator-pool-legend-value {
  align-items: flex-end;
  text-align: right;
}

.operator-pool-legend-value > strong,
.operator-pool-legend > li > strong {
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 700;
}

.operator-pool-unknown-list {
  margin-top: 0.875rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  line-height: 1.45;
}

.operator-pool-unknown-list strong {
  color: var(--operator-foreground);
  font-weight: 600;
}

.operator-capacity-fleet { padding: 1.25rem 1.5rem 1.5rem; }
.operator-capacity-section-heading { margin-bottom: 0.875rem; }
.operator-capacity-section-heading h3 { font-size: 1rem; }
.operator-capacity-section-heading > span,
.operator-capacity-fleet-label > div span {
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.operator-capacity-fleet-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(13rem, 1fr));
  gap: 0.75rem;
}

.operator-capacity-fleet-provider {
  min-width: 0;
  padding: 0.875rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
}

.operator-capacity-fleet-label > div {
  display: flex;
  min-width: 0;
  flex-direction: column;
}

.operator-capacity-fleet-label strong {
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 650;
}
.operator-capacity-fleet-label > strong { flex: 0 0 auto; }

.operator-capacity-compact-accounts {
  display: grid;
  margin-top: 0.75rem;
  padding-top: 0.625rem;
  border-top: 1px solid var(--operator-border);
  gap: 0.5rem;
}
.operator-capacity-compact-account {
  display: grid;
  min-width: 0;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.8125rem;
}
.operator-capacity-compact-account > span:first-child {
  overflow: hidden;
  color: var(--operator-foreground);
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.operator-capacity-compact-meta {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  white-space: nowrap;
}
.operator-capacity-compact-meta .operator-capacity-status { font-size: 0.75rem; }
.operator-capacity-compact-meta > strong { font-weight: 700; }

.operator-capacity-providers { border-top: 1px solid var(--operator-border); }
.operator-capacity-provider + .operator-capacity-provider { border-top: 1px solid var(--operator-border); }
.operator-capacity-provider-header {
  padding: 0;
  background: var(--operator-muted);
}
.operator-capacity-provider-name { font-size: 1rem; }
.operator-capacity-provider-identity > span > span {
  margin-top: 0.125rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.operator-capacity-provider-toggle {
  display: flex;
  width: 100%;
  min-height: 4.5rem;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.875rem 1.5rem;
  color: var(--operator-foreground);
  text-align: left;
  transition: background-color 150ms ease;
}

.operator-capacity-provider-toggle:hover {
  background: color-mix(in oklch, var(--operator-muted) 68%, var(--operator-card));
}

.operator-capacity-provider-toggle:focus-visible {
  position: relative;
  z-index: 1;
  outline: 2px solid var(--operator-focus);
  outline-offset: -3px;
}

.operator-capacity-provider-identity {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.75rem;
}

.operator-capacity-provider-identity > svg {
  flex: 0 0 auto;
  transition: transform 150ms ease;
}

.operator-capacity-provider-identity > svg.is-collapsed {
  transform: rotate(-90deg);
}

.operator-capacity-provider-identity > span {
  display: flex;
  min-width: 0;
  flex-direction: column;
}

.operator-capacity-provider-summary {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}
.operator-capacity-provider-summary strong { font-size: 0.875rem; }

.operator-capacity-account-list {
  display: grid;
  padding: 0.625rem;
  background: var(--operator-muted);
  gap: 0.625rem;
}

.operator-capacity-account {
  min-width: 0;
  padding: 1.125rem 1.25rem 1.25rem;
  border: 1px solid var(--operator-border-subtle);
  border-radius: 0.25rem;
  background: var(--operator-card);
}
.operator-capacity-account-title,
.operator-capacity-account-meta,
.operator-capacity-window { min-width: 0; }
.operator-capacity-account h4 {
  font-size: 0.9375rem;
  overflow-wrap: anywhere;
}
.operator-capacity-account-title p {
  margin-top: 0.1875rem;
  overflow: hidden;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.operator-capacity-account-primary {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.875rem;
}
.operator-capacity-status {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  font-size: 0.8125rem;
  font-weight: 600;
  white-space: nowrap;
}
.operator-capacity-status > span {
  width: 0.5rem;
  height: 0.5rem;
  flex: 0 0 auto;
  border-radius: 999px;
  background: currentColor;
}
.operator-capacity-status.is-healthy { color: var(--operator-success); }
.operator-capacity-status.is-error { color: var(--operator-destructive); }
.operator-capacity-status.is-rate_limited,
.operator-capacity-status.is-overloaded,
.operator-capacity-status.is-paused,
.operator-capacity-status.is-unschedulable { color: var(--operator-warning); }
.operator-capacity-status.is-inactive { color: var(--operator-muted-foreground); }

.operator-capacity-account-remaining {
  min-width: 7.5rem;
  font-size: 1rem;
  font-weight: 700;
  text-align: right;
}
.operator-capacity-account-actions {
  display: flex;
  align-items: center;
  gap: 0.375rem;
}
.operator-capacity-account-body {
  display: grid;
  min-width: 0;
  margin-top: 1rem;
  grid-template-columns: minmax(13rem, 0.55fr) minmax(0, 1.45fr);
  gap: 1.5rem;
}

.operator-capacity-routing,
.operator-capacity-account-error {
  margin-top: 0.75rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  line-height: 1.45;
}
.operator-capacity-routing strong {
  color: var(--operator-foreground);
  font-weight: 600;
}
.operator-capacity-account-error { color: var(--operator-destructive); }

.operator-capacity-window-grid {
  display: grid;
  min-width: 0;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem 1.25rem;
}
.operator-capacity-window-label { font-size: 0.875rem; }
.operator-capacity-window-label > span {
  overflow: hidden;
  color: var(--operator-foreground);
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.operator-capacity-window-label strong {
  flex: 0 0 auto;
  font-weight: 700;
}
.operator-capacity-track {
  height: 0.625rem;
  margin-top: 0.5rem;
  overflow: hidden;
  border-radius: 999px;
  background: var(--operator-track);
}
.operator-capacity-track.is-unknown {
  border: 1px dashed var(--operator-border);
  background: transparent;
}
.operator-capacity-track > span {
  display: block;
  height: 100%;
  border-radius: inherit;
}
.operator-capacity-window-detail {
  margin-top: 0.4375rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}
.operator-capacity-window-detail span:last-child {
  overflow: hidden;
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.is-healthy { color: var(--operator-success); }
.is-warning { color: var(--operator-warning); }
.is-critical { color: var(--operator-destructive); }
.is-unknown { color: var(--operator-muted-foreground); }
.operator-capacity-track > .is-healthy { background: var(--operator-success-fill); }
.operator-capacity-track > .is-warning { background: var(--operator-warning-fill); }
.operator-capacity-track > .is-critical { background: var(--operator-destructive-fill); }

.operator-capacity-quota-unknown {
  display: flex;
  min-height: 4.5rem;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  padding: 0.875rem;
  border: 1px dashed var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
}
.operator-capacity-quota-unknown strong {
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 600;
}
.operator-capacity-quota-unknown span {
  margin-top: 0.25rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  line-height: 1.4;
}
.operator-capacity-compact .operator-capacity-fleet { padding-bottom: 1.25rem; }

@media (max-width: 1023px) {
  .operator-capacity-header,
  .operator-capacity-account-heading {
    align-items: flex-start;
    flex-direction: column;
  }
  .operator-capacity-account-primary {
    width: 100%;
    flex-wrap: wrap;
  }
  .operator-capacity-account-remaining {
    min-width: 0;
    margin-left: auto;
  }
  .operator-capacity-account-body { grid-template-columns: 1fr; }
  .operator-pool-legend { grid-template-columns: 1fr; }
}

@media (max-width: 639px) {
  .operator-capacity-header,
  .operator-pool-heading,
  .operator-capacity-provider-toggle {
    align-items: flex-start;
    flex-direction: column;
  }
  .operator-capacity-account,
  .operator-capacity-header,
  .operator-capacity-fleet,
  .operator-pool-capacity {
    padding-right: 1rem;
    padding-left: 1rem;
  }
  .operator-capacity-provider-toggle {
    padding-right: 1rem;
    padding-left: 1rem;
  }
  .operator-pool-layout { grid-template-columns: 1fr; }
  .operator-pool-chart { width: min(100%, 12rem); }
  .operator-pool-unknown-count { white-space: normal; }
  .operator-capacity-provider-summary {
    width: 100%;
    justify-content: space-between;
  }
  .operator-capacity-window-grid { grid-template-columns: 1fr; }
  .operator-capacity-window-detail {
    align-items: flex-start;
    flex-direction: column;
    gap: 0.125rem;
  }
  .operator-capacity-window-detail span:last-child {
    text-align: left;
    white-space: normal;
  }
}
</style>
