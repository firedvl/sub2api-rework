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
        class="operator-capacity-summary"
        data-testid="account-pool-capacity"
        :aria-labelledby="poolTitleId"
      >
        <div class="operator-capacity-summary-value">
          <span :id="poolTitleId">{{ t('admin.dashboard.capacity.poolTitle') }}</span>
          <strong>
            {{ normalizedPool.remainingPercent === null
              ? t('common.unknown')
              : `${formatPercent(normalizedPool.remainingPercent)}%` }}
          </strong>
          <small>{{ t('admin.dashboard.capacity.poolAvailable') }}</small>
        </div>
        <div
          class="operator-capacity-summary-track"
          role="progressbar"
          :aria-label="poolAriaLabel"
          aria-valuemin="0"
          aria-valuemax="100"
          :aria-valuenow="normalizedPool.remainingPercent === null ? undefined : Math.round(normalizedPool.remainingPercent)"
        >
          <span
            v-if="normalizedPool.remainingPercent !== null"
            :style="{ width: `${normalizedPool.remainingPercent}%` }"
          />
        </div>
        <dl class="operator-capacity-summary-meta">
          <div>
            <dt>{{ t('admin.dashboard.capacity.knownCount', { count: normalizedPool.knownCount }) }}</dt>
            <dd>{{ t('admin.dashboard.capacity.unknownCount', { count: normalizedPool.unknownCount }) }}</dd>
          </div>
          <div v-if="normalizedPool.lowestAccount && normalizedPool.lowestRemaining !== null">
            <dt>{{ t('admin.dashboard.capacity.lowestRemaining') }}</dt>
            <dd>{{ normalizedPool.lowestAccount.account.name }} · {{ formatPercent(normalizedPool.lowestRemaining) }}%</dd>
          </div>
        </dl>
        <RouterLink class="btn btn-secondary" to="/admin/stats">
          {{ t('admin.dashboard.viewStats') }}
        </RouterLink>
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
                  <span class="operator-capacity-provider-counts">
                    {{ t('admin.dashboard.capacity.providerAccounts', { count: provider.accounts.length }) }}
                    <span aria-hidden="true">&middot;</span>
                    {{ t('admin.dashboard.capacity.knownCount', { count: provider.knownCount }) }}
                    <template v-if="provider.unknownCount">
                      <span aria-hidden="true">&middot;</span>
                      {{ t('admin.dashboard.capacity.unknownCount', { count: provider.unknownCount }) }}
                    </template>
                  </span>
                </span>
              </span>
              <span class="operator-capacity-provider-summary">
                <strong :class="capacityTone(provider.remainingPercent)">
                  {{ normalizedRemainingLabel(provider.remainingPercent) }}
                </strong>
                <span v-if="provider.lowestAccount && provider.lowestRemaining !== null">
                  {{ t('admin.dashboard.capacity.lowestAccountRemaining', {
                    value: formatPercent(provider.lowestRemaining),
                    account: provider.lowestAccount.account.name,
                  }) }}
                </span>
                <span v-if="provider.nextReset">
                  {{ t('admin.dashboard.capacity.nextLimitingReset', { time: formatReset(provider.nextReset) }) }}
                </span>
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

const formatPercent = (value: number) => value.toFixed(2).replace(/\.?0+$/, '')

const percentLabel = (value: number | null) => value === null
  ? t('common.unknown')
  : t('admin.dashboard.capacity.remaining', { value: formatPercent(value) })

const normalizedRemainingLabel = (value: number | null) => value === null
  ? t('admin.dashboard.capacity.quotaUnknown')
  : t('admin.dashboard.capacity.normalizedRemaining', { value: formatPercent(value) })

const capacityTone = (value: number | null) => {
  if (value === null) return 'is-unknown'
  if (value <= 20) return 'is-critical'
  if (value <= 50) return 'is-warning'
  return 'is-healthy'
}

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
.operator-capacity-window-detail {
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

.operator-capacity-summary {
  display: grid;
  grid-template-columns: minmax(9rem, 0.45fr) minmax(12rem, 1.25fr) minmax(12rem, 0.9fr) auto;
  align-items: center;
  gap: 1rem 1.5rem;
  padding: 1.25rem 1.5rem;
}

.operator-capacity-summary-value {
  display: flex;
  min-width: 0;
  flex-direction: column;
}

.operator-capacity-summary-value > span,
.operator-capacity-summary-value > small,
.operator-capacity-summary-meta {
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.operator-capacity-summary-value > strong {
  margin: 0.125rem 0;
  color: var(--operator-foreground);
  font-size: 1.75rem;
  font-weight: 700;
  line-height: 1.1;
}

.operator-capacity-summary-track {
  height: 0.625rem;
  overflow: hidden;
  border-radius: 999px;
  background: var(--operator-track);
}

.operator-capacity-summary-track > span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--operator-success-fill);
}

.operator-capacity-summary-meta {
  display: grid;
  gap: 0.5rem;
}

.operator-capacity-summary-meta > div {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 0.25rem 0.75rem;
}

.operator-capacity-summary-meta dd {
  color: var(--operator-foreground);
  text-align: right;
}

.operator-capacity-summary > .btn { justify-self: end; }

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
.operator-capacity-provider-counts > span { margin: 0 0.2rem; }

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
  min-width: 0;
  align-items: flex-end;
  flex-direction: column;
  gap: 0.125rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  text-align: right;
}
.operator-capacity-provider-summary strong {
  font-size: 0.875rem;
  font-weight: 700;
}

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
  .operator-capacity-summary { grid-template-columns: minmax(0, 1fr) minmax(12rem, 1fr); }
  .operator-capacity-summary > .btn { justify-self: start; }
}

@media (max-width: 639px) {
  .operator-capacity-header,
  .operator-capacity-provider-toggle {
    align-items: flex-start;
    flex-direction: column;
  }
  .operator-capacity-account,
  .operator-capacity-header,
  .operator-capacity-summary {
    padding-right: 1rem;
    padding-left: 1rem;
  }
  .operator-capacity-summary { grid-template-columns: 1fr; }
  .operator-capacity-summary-meta dd { text-align: left; }
  .operator-capacity-provider-toggle {
    padding-right: 1rem;
    padding-left: 1rem;
  }
  .operator-capacity-provider-summary {
    width: 100%;
    align-items: flex-start;
    text-align: left;
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
