<template>
  <section class="card operator-capacity" :aria-label="t('admin.dashboard.capacity.title')">
    <header class="operator-capacity-header">
      <div>
        <h2>{{ t('admin.dashboard.capacity.title') }}</h2>
        <p>{{ t('admin.dashboard.capacity.description') }}</p>
      </div>
      <div v-if="accounts.length" class="operator-capacity-counts">
        <span>{{ t('admin.dashboard.capacity.accountCount', { count: accounts.length }) }}</span>
        <span>{{ t('admin.dashboard.capacity.knownCount', { count: knownCount }) }}</span>
        <span v-if="unknownCount" class="operator-capacity-unknown-count">
          {{ t('admin.dashboard.capacity.unknownCount', { count: unknownCount }) }}
        </span>
      </div>
    </header>

    <div
      v-if="error"
      role="alert"
      data-testid="capacity-load-error"
      class="flex flex-wrap items-center justify-between gap-3 border-b border-red-200 bg-red-50 px-5 py-3 text-sm text-red-800 dark:border-red-900/70 dark:bg-red-950/30 dark:text-red-200"
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

    <div v-else-if="accounts.length" class="operator-capacity-providers">
      <section v-for="provider in providers" :key="provider.platform" class="operator-capacity-provider">
        <header class="operator-capacity-provider-header">
          <div>
            <h3>{{ providerLabel(provider.platform) }}</h3>
            <p>{{ t('admin.dashboard.capacity.providerAccounts', { count: provider.accounts.length }) }}</p>
          </div>
          <div class="operator-capacity-provider-summary">
            <span>{{ t('admin.dashboard.capacity.lowestRemaining') }}</span>
            <strong :class="capacityTone(provider.lowestRemaining)">
              {{ percentLabel(provider.lowestRemaining) }}
            </strong>
          </div>
        </header>

        <div class="operator-capacity-account-list">
          <article
            v-for="summary in provider.accounts"
            :key="summary.account.id"
            class="operator-capacity-account"
          >
            <div class="operator-capacity-account-meta">
              <div class="operator-capacity-account-title">
                <div class="min-w-0">
                  <h4>{{ summary.account.name }}</h4>
                  <p v-if="summary.identity" :title="summary.identity">{{ summary.identity }}</p>
                  <p v-else>#{{ summary.account.id }} · {{ accountTypeLabel(summary.account.type) }}</p>
                </div>
                <span :class="['operator-capacity-status', `is-${summary.health}`]">
                  <span aria-hidden="true" />
                  {{ healthLabel(summary.health) }}
                </span>
              </div>
              <div class="operator-capacity-meta-line">
                <span>{{ accountTypeLabel(summary.account.type) }}</span>
                <span v-if="summary.tier">{{ t('admin.dashboard.capacity.plan') }} {{ summary.tier }}</span>
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
          </article>
        </div>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { formatDateTimeToMinute } from '@/utils/format'
import {
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
}>(), {
  usageByAccountId: () => ({}),
  errorsByAccountId: () => ({}),
  loading: false,
  error: false,
})

const emit = defineEmits<{ retry: [] }>()
const { t } = useI18n()
const providers = computed(() => buildProviderCapacity(
  props.accounts,
  props.usageByAccountId,
  props.errorsByAccountId,
))
const knownCount = computed(() => providers.value.reduce((total, provider) => total + provider.knownCount, 0))
const unknownCount = computed(() => providers.value.reduce((total, provider) => total + provider.unknownCount, 0))

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

const formatReset = (value: string) => formatDateTimeToMinute(value) || t('common.unknown')
</script>

<style scoped>
.operator-capacity {
  overflow: hidden;
}

.operator-capacity-header,
.operator-capacity-provider-header,
.operator-capacity-account-title,
.operator-capacity-window-label,
.operator-capacity-window-detail {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
}

.operator-capacity-header {
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--operator-border);
}

.operator-capacity-header h2,
.operator-capacity-provider-header h3,
.operator-capacity-account h4 {
  color: var(--operator-foreground);
  font-weight: 600;
}

.operator-capacity-header h2 {
  font-size: 0.875rem;
}

.operator-capacity-header p,
.operator-capacity-provider-header p,
.operator-capacity-account-title p,
.operator-capacity-routing,
.operator-capacity-quota-unknown span {
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
}

.operator-capacity-header p {
  margin-top: 0.25rem;
}

.operator-capacity-counts,
.operator-capacity-meta-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem;
  color: var(--operator-muted-foreground);
  font-size: 0.6875rem;
}

.operator-capacity-counts span,
.operator-capacity-meta-line span {
  padding: 0.1875rem 0.4375rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: color-mix(in oklch, var(--operator-muted) 60%, transparent);
}

.operator-capacity-unknown-count,
.operator-capacity-meta-line .is-unschedulable {
  color: oklch(0.52 0.16 65);
}

.operator-capacity-meta-line .is-schedulable {
  color: oklch(0.46 0.13 155);
}

.operator-capacity-empty {
  display: flex;
  min-height: 7rem;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.operator-capacity-provider + .operator-capacity-provider {
  border-top: 1px solid var(--operator-border);
}

.operator-capacity-provider-header {
  padding: 0.75rem 1.25rem;
  background: color-mix(in oklch, var(--operator-muted) 55%, transparent);
}

.operator-capacity-provider-header h3 {
  font-size: 0.8125rem;
}

.operator-capacity-provider-summary {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  color: var(--operator-muted-foreground);
  font-size: 0.6875rem;
}

.operator-capacity-provider-summary strong {
  font-size: 0.75rem;
}

.operator-capacity-account {
  display: grid;
  min-width: 0;
  padding: 1rem 1.25rem;
  grid-template-columns: minmax(12rem, 0.7fr) minmax(20rem, 1.3fr);
  gap: 1.25rem;
}

.operator-capacity-account + .operator-capacity-account {
  border-top: 1px solid color-mix(in oklch, var(--operator-border) 70%, transparent);
}

.operator-capacity-account-meta,
.operator-capacity-account-title > div,
.operator-capacity-window {
  min-width: 0;
}

.operator-capacity-account h4 {
  font-size: 0.8125rem;
  overflow-wrap: anywhere;
}

.operator-capacity-account-title p {
  margin-top: 0.1875rem;
  overflow-wrap: anywhere;
}

.operator-capacity-status {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 0.3125rem;
  font-size: 0.6875rem;
  font-weight: 500;
}

.operator-capacity-status > span {
  width: 0.375rem;
  height: 0.375rem;
  border-radius: 999px;
  background: currentColor;
}

.operator-capacity-status.is-healthy { color: oklch(0.49 0.14 155); }
.operator-capacity-status.is-error { color: oklch(0.55 0.2 27); }
.operator-capacity-status.is-rate_limited,
.operator-capacity-status.is-overloaded,
.operator-capacity-status.is-paused,
.operator-capacity-status.is-unschedulable { color: oklch(0.55 0.15 65); }
.operator-capacity-status.is-inactive { color: var(--operator-muted-foreground); }

.operator-capacity-meta-line {
  margin-top: 0.625rem;
}

.operator-capacity-routing {
  margin-top: 0.625rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.operator-capacity-routing strong {
  color: var(--operator-foreground);
  font-weight: 500;
}

.operator-capacity-window-grid {
  display: grid;
  min-width: 0;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.875rem 1.25rem;
}

.operator-capacity-window-label {
  font-size: 0.75rem;
}

.operator-capacity-window-label > span {
  overflow: hidden;
  color: var(--operator-foreground);
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.operator-capacity-window-label strong {
  flex: 0 0 auto;
  font-weight: 600;
}

.operator-capacity-track {
  height: 0.375rem;
  margin-top: 0.375rem;
  overflow: hidden;
  border-radius: 999px;
  background: var(--operator-muted);
}

.operator-capacity-track > span {
  display: block;
  height: 100%;
  border-radius: inherit;
}

.operator-capacity-window-detail {
  margin-top: 0.3125rem;
  color: var(--operator-muted-foreground);
  font-size: 0.6875rem;
}

.operator-capacity-window-detail span:last-child {
  overflow: hidden;
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.is-healthy { color: oklch(0.47 0.14 155); }
.is-warning { color: oklch(0.55 0.15 65); }
.is-critical { color: oklch(0.55 0.2 27); }
.is-unknown { color: var(--operator-muted-foreground); }
.operator-capacity-track > .is-healthy { background: oklch(0.64 0.16 155); }
.operator-capacity-track > .is-warning { background: oklch(0.7 0.16 75); }
.operator-capacity-track > .is-critical { background: oklch(0.62 0.21 27); }

.operator-capacity-quota-unknown {
  display: flex;
  min-height: 3.75rem;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  padding: 0.75rem;
  border: 1px dashed var(--operator-border);
  border-radius: var(--operator-radius);
  background: color-mix(in oklch, var(--operator-muted) 40%, transparent);
}

.operator-capacity-quota-unknown strong {
  color: var(--operator-foreground);
  font-size: 0.75rem;
  font-weight: 500;
}

.operator-capacity-quota-unknown span {
  margin-top: 0.25rem;
}

@media (max-width: 1023px) {
  .operator-capacity-account {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 639px) {
  .operator-capacity-header,
  .operator-capacity-provider-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .operator-capacity-account,
  .operator-capacity-header,
  .operator-capacity-provider-header {
    padding-right: 1rem;
    padding-left: 1rem;
  }

  .operator-capacity-window-grid {
    grid-template-columns: 1fr;
  }

  .operator-capacity-provider-summary {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
