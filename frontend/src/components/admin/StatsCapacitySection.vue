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
      <article class="stats-capacity-card is-global">
        <StatsCapacityDonut
          :title="t('admin.dashboard.capacity.poolTitle')"
          :pool="globalPool"
          test-id="global-capacity-donut"
        />
      </article>

      <div class="stats-provider-grid">
        <article
          v-for="provider in providerDetails"
          :key="provider.platform"
          class="stats-capacity-card"
          :data-testid="`provider-capacity-${provider.platform}`"
        >
          <StatsCapacityDonut
            :title="providerLabel(provider.platform)"
            :pool="provider.pool"
            :test-id="`provider-capacity-donut-${provider.platform}`"
          />

          <dl class="stats-provider-stats">
            <div>
              <dt>{{ t('admin.dashboard.capacity.quotaCoverage') }}</dt>
              <dd>{{ provider.knownCount }}/{{ provider.knownCount + provider.unknownCount }}</dd>
              <small v-if="provider.unknownCount">
                {{ t('admin.dashboard.capacity.unknownCount', { count: provider.unknownCount }) }}
              </small>
            </div>
            <div>
              <dt>{{ t('admin.dashboard.capacity.schedulable') }}</dt>
              <dd>{{ provider.schedulableCount }}</dd>
            </div>
            <div>
              <dt>{{ t('admin.dashboard.capacity.lowestCapacity') }}</dt>
              <dd>{{ provider.lowestRemaining === null ? t('common.unknown') : `${formatPercent(provider.lowestRemaining)}%` }}</dd>
              <small v-if="provider.lowestAccount" :title="provider.lowestAccount.account.name">
                {{ provider.lowestAccount.account.name }}
              </small>
            </div>
            <div>
              <dt>{{ t('admin.dashboard.capacity.nextReset') }}</dt>
              <dd :title="provider.nextReset ? formatReset(provider.nextReset) : undefined">
                {{ provider.nextReset ? formatCompactReset(provider.nextReset) : t('common.unknown') }}
              </dd>
            </div>
          </dl>

          <details v-if="provider.windows.length" class="stats-window-section">
            <summary>
              <span>{{ t('admin.stats.capacity.windows') }}</span>
              <span>{{ provider.windows.length }}</span>
            </summary>
            <ul class="stats-window-list">
              <li v-for="window in provider.windows" :key="window.key">
                <div class="stats-window-heading">
                  <strong>{{ window.label }}</strong>
                  <strong>{{ window.remainingPercent === null ? t('common.unknown') : `${formatPercent(window.remainingPercent)}%` }}</strong>
                </div>
                <div
                  class="stats-window-track"
                  role="progressbar"
                  :aria-label="windowAriaLabel(window)"
                  aria-valuemin="0"
                  aria-valuemax="100"
                  :aria-valuenow="window.remainingPercent === null ? undefined : Math.round(window.remainingPercent)"
                >
                  <span v-if="window.remainingPercent !== null" :style="{ width: `${window.remainingPercent}%` }" />
                </div>
                <p>
                  {{ t('admin.stats.capacity.windowCoverage', { known: window.knownCount, unknown: window.unknownCount }) }}
                  <template v-if="window.nextReset"> · {{ t('admin.dashboard.capacity.nextLimitingReset', { time: formatReset(window.nextReset) }) }}</template>
                </p>
                <ul class="stats-window-contributions" :aria-label="t('admin.stats.capacity.windowContributions', { window: window.label })">
                  <li v-for="segment in window.segments" :key="segment.summary.account.id">
                    <span>{{ segment.summary.account.name }}</span>
                    <span>
                      {{ t('admin.dashboard.capacity.remaining', { value: formatPercent(segment.remainingPercent) }) }} ·
                      {{ t('admin.dashboard.capacity.poolContribution', { value: formatPercent(segment.contributionPercent) }) }}
                    </span>
                  </li>
                </ul>
              </li>
            </ul>
          </details>
        </article>
      </div>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import { formatDateTimeToMinute, formatDayAwareDateTime } from '@/utils/format'
import {
  buildNormalizedPoolCapacity,
  buildProviderCapacity,
  buildWindowCapacities,
  type OperatorWindowCapacity,
} from '@/utils/operatorCapacity'
import type { Account, AccountPlatform, AccountUsageInfo } from '@/types'
import StatsCapacityDonut from './StatsCapacityDonut.vue'

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
const providers = computed(() => buildProviderCapacity(
  props.accounts,
  props.usageByAccountId,
  props.errorsByAccountId,
))
const globalPool = computed(() => buildNormalizedPoolCapacity(
  providers.value.flatMap((provider) => provider.accounts),
))
const providerDetails = computed(() => providers.value.map((provider) => ({
  ...provider,
  pool: buildNormalizedPoolCapacity(provider.accounts),
  windows: buildWindowCapacities(provider.accounts),
})))

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
const formatReset = (value: string) => formatDateTimeToMinute(value) || t('common.unknown')
const formatCompactReset = (value: string) => formatDayAwareDateTime(value) || t('common.unknown')
const windowAriaLabel = (window: OperatorWindowCapacity) => window.remainingPercent === null
  ? `${window.label}: ${t('admin.dashboard.capacity.quotaUnknown')}`
  : `${window.label}: ${formatPercent(window.remainingPercent)}% ${t('admin.dashboard.capacity.poolAvailable')}`
</script>

<style scoped>
.stats-capacity-section,
.stats-capacity-card {
  border: 1px solid var(--operator-border);
  border-radius: 0.5rem;
  background: var(--operator-card);
  box-shadow: var(--operator-shadow-xs);
}

.stats-capacity-section { overflow: hidden; }
.stats-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 1.25rem 1.5rem;
  border-bottom: 1px solid var(--operator-border);
}
.stats-section-header h2 { color: var(--operator-foreground); font-size: 1rem; font-weight: 650; }
.stats-section-header p,
.stats-capacity-count { color: var(--operator-muted-foreground); font-size: 0.8125rem; }
.stats-section-header p { margin-top: 0.2rem; }
.stats-capacity-count { white-space: nowrap; }
.stats-section-state { display: flex; min-height: 8rem; align-items: center; justify-content: center; gap: 0.75rem; padding: 1.5rem; color: var(--operator-muted-foreground); }
.stats-section-state.is-error { justify-content: space-between; min-height: 0; color: #b91c1c; }
.stats-capacity-card { min-width: 0; padding: 1.25rem; }
.stats-capacity-card.is-global { margin: 1.25rem; }

.stats-provider-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
  padding: 0 1.25rem 1.25rem;
}

.stats-provider-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin-top: 1rem;
  padding-top: 0.875rem;
  border-top: 1px solid var(--operator-border);
}
.stats-provider-stats > div {
  min-width: 0;
  padding: 0 0.75rem;
}
.stats-provider-stats > div:first-child { padding-left: 0; }
.stats-provider-stats > div:last-child { padding-right: 0; }
.stats-provider-stats > div + div { border-left: 1px solid var(--operator-border-subtle); }
.stats-provider-stats dt,
.stats-provider-stats small {
  overflow: hidden;
  color: var(--operator-muted-foreground);
  font-size: 0.6875rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.stats-provider-stats dd {
  margin-top: 0.1875rem;
  overflow: hidden;
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 700;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.stats-provider-stats small {
  display: block;
  margin-top: 0.125rem;
}

.stats-window-section { margin-top: 1.25rem; padding-top: 1rem; border-top: 1px solid var(--operator-border); }
.stats-window-section summary {
  display: flex;
  min-height: 2.25rem;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  color: var(--operator-foreground);
  cursor: pointer;
  font-size: 0.8125rem;
  font-weight: 650;
}
.stats-window-section summary:focus-visible { outline: 2px solid var(--operator-focus); outline-offset: 2px; }
.stats-window-section summary span:last-child { color: var(--operator-muted-foreground); font-size: 0.75rem; font-weight: 500; }
.stats-window-list { display: grid; gap: 1rem; margin-top: 0.75rem; }
.stats-window-heading,
.stats-window-contributions li { display: flex; justify-content: space-between; gap: 0.75rem; }
.stats-window-heading { color: var(--operator-foreground); font-size: 0.75rem; }
.stats-window-track { height: 0.375rem; margin-top: 0.375rem; overflow: hidden; border-radius: 2px; background: var(--operator-muted); }
.stats-window-track > span { display: block; height: 100%; background: var(--operator-success); }
.stats-window-list p,
.stats-window-contributions { margin-top: 0.375rem; color: var(--operator-muted-foreground); font-size: 0.6875rem; }
.stats-window-contributions { display: grid; gap: 0.25rem; }
.stats-window-contributions li span:last-child { text-align: right; }

@media (max-width: 1024px) {
  .stats-provider-grid { grid-template-columns: 1fr; }
}

@media (max-width: 640px) {
  .stats-section-header { align-items: flex-start; flex-direction: column; }
  .stats-capacity-card.is-global { margin: 0.75rem; }
  .stats-provider-grid { padding: 0 0.75rem 0.75rem; }
  .stats-provider-stats { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .stats-provider-stats > div { padding: 0.625rem 0.75rem; }
  .stats-provider-stats > div:nth-child(odd) { padding-left: 0; border-left: 0; }
  .stats-provider-stats > div:nth-child(even) { padding-right: 0; }
  .stats-provider-stats > div:nth-child(n + 3) { border-top: 1px solid var(--operator-border-subtle); }
  .stats-window-contributions li { flex-direction: column; gap: 0.125rem; }
  .stats-window-contributions li span:last-child { text-align: left; }
}
</style>
