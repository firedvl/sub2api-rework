<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <section
        v-if="!loading && snapshotError"
        role="alert"
        data-testid="dashboard-load-error"
        class="flex min-h-64 flex-col items-center justify-center border border-red-200 bg-white px-6 py-12 text-center dark:border-red-900/70 dark:bg-dark-900"
      >
        <Icon name="exclamationCircle" size="xl" class="text-red-500 dark:text-red-400" />
        <h2 class="mt-4 text-base font-semibold text-gray-900 dark:text-white">
          {{ t('admin.dashboard.failedToLoad') }}
        </h2>
        <button type="button" class="btn btn-primary mt-5" @click="loadDashboardStats">
          <Icon name="refresh" size="sm" />
          <span>{{ t('admin.dashboard.retry') }}</span>
        </button>
      </section>

      <template v-if="!loading && stats">
        <section class="operator-fleet-summary" aria-labelledby="operator-fleet-title">
          <header class="operator-fleet-heading">
            <div>
              <h2 id="operator-fleet-title">Account fleet</h2>
              <p>{{ fleetSummaryText }}</p>
            </div>
            <span v-if="accountStatusCounts.disabled" class="operator-fleet-disabled">
              {{ accountStatusCounts.disabled }} disabled
            </span>
          </header>
          <div class="operator-fleet-metrics">
            <RouterLink to="/admin/accounts" class="operator-fleet-metric">
              <span>Accounts</span>
              <strong>{{ fleetCountLabel(capacityAccounts.length) }}</strong>
              <small>Configured</small>
            </RouterLink>
            <RouterLink :to="accountStatusLink('active')" class="operator-fleet-metric is-active">
              <span>Active</span>
              <strong>{{ fleetCountLabel(accountStatusCounts.active) }}</strong>
              <small>Ready for traffic</small>
            </RouterLink>
            <RouterLink :to="accountStatusLink('limited')" class="operator-fleet-metric is-limited">
              <span>Limited</span>
              <strong>{{ fleetCountLabel(accountStatusCounts.limited) }}</strong>
              <small>Temporary pressure</small>
            </RouterLink>
            <RouterLink :to="accountStatusLink('error')" class="operator-fleet-metric is-error">
              <span>Errors</span>
              <strong>{{ fleetCountLabel(accountStatusCounts.error) }}</strong>
              <small>Needs attention</small>
            </RouterLink>
          </div>
        </section>

        <section class="operator-gateway-summary" aria-labelledby="operator-gateway-title">
          <div class="operator-gateway-heading">
            <div>
              <div class="operator-gateway-status"><span aria-hidden="true" /> Online</div>
              <h2 id="operator-gateway-title">Gateway traffic</h2>
              <p>{{ hasGatewayTraffic ? t('admin.dashboard.receivingTraffic') : t('admin.dashboard.idleTraffic') }}</p>
            </div>
            <RouterLink to="/admin/usage">{{ t('admin.dashboard.viewActivity') }}</RouterLink>
          </div>
          <dl class="operator-gateway-metrics">
            <div><dt>RPM</dt><dd>{{ formatNumber(stats.rpm) }}</dd></div>
            <div><dt>TPM</dt><dd>{{ formatTokens(stats.tpm) }}</dd></div>
            <div><dt>Today</dt><dd>{{ formatNumber(stats.today_requests) }} requests</dd></div>
            <div><dt>Tokens</dt><dd>{{ formatTokens(stats.today_tokens) }}</dd></div>
            <div><dt>Cost</dt><dd>${{ formatCost(stats.today_actual_cost) }}</dd></div>
            <div><dt>Avg response</dt><dd>{{ formatDuration(stats.average_duration_ms) }}</dd></div>
          </dl>
        </section>
      </template>

      <OperatorCapacityOverview
        v-if="!loading"
        :accounts="capacityAccounts"
        :usage-by-account-id="capacityUsageByAccountId"
        :errors-by-account-id="capacityUsageErrorByAccountId"
        :loading="capacityLoading"
        :error="capacityError"
        compact
        @retry="loadAccountCapacity"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type {
  Account,
  AccountUsageInfo,
  DashboardStats
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import OperatorCapacityOverview from '@/components/admin/OperatorCapacityOverview.vue'
import {
  buildProviderCapacity,
  classifyOperatorAccount,
  supportsBatchAccountUsage,
  type OperatorAccountStatus,
} from '@/utils/operatorCapacity'

const { t } = useI18n()
const appStore = useAppStore()
const stats = ref<DashboardStats | null>(null)
const loading = ref(false)
const snapshotError = ref(false)
const capacityAccounts = ref<Account[]>([])
const capacityUsageByAccountId = ref<Record<string, AccountUsageInfo | null>>({})
const capacityUsageErrorByAccountId = ref<Record<string, string | null>>({})
const capacityLoading = ref(false)
const capacityError = ref(false)
let capacityLoadSeq = 0

const hasGatewayTraffic = computed(() => (stats.value?.rpm ?? 0) > 0 || (stats.value?.tpm ?? 0) > 0)
const capacitySummaries = computed(() => buildProviderCapacity(
  capacityAccounts.value,
  capacityUsageByAccountId.value,
  capacityUsageErrorByAccountId.value,
).flatMap((provider) => provider.accounts))
const accountStatusCounts = computed(() => capacitySummaries.value.reduce<Record<OperatorAccountStatus, number>>(
  (counts, summary) => {
    counts[classifyOperatorAccount(summary).status] += 1
    return counts
  },
  { active: 0, limited: 0, error: 0, disabled: 0 },
))
const fleetSummaryText = computed(() => {
  if (capacityLoading.value) return 'Loading account status...'
  if (capacityError.value) return 'Account status is unavailable.'
  if (accountStatusCounts.value.error) return `${accountStatusCounts.value.error} account${accountStatusCounts.value.error === 1 ? '' : 's'} need attention.`
  if (accountStatusCounts.value.limited) return 'Temporary limits are reducing available capacity.'
  return 'No account issues need attention.'
})
const fleetCountLabel = (count: number) => capacityLoading.value ? '-' : formatNumber(count)
const accountStatusLink = (status: OperatorAccountStatus) => ({
  path: '/admin/accounts',
  query: { operator_status: status },
})

// Format helpers
const formatTokens = (value: number | undefined): string => {
  if (value === undefined || value === null) return '0'
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const toFiniteNumber = (value: unknown): number => {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : 0
}

const formatNumber = (value: number | null | undefined): string => {
  return toFiniteNumber(value).toLocaleString()
}

const formatCost = (value: number | null | undefined): string => {
  const safeValue = toFiniteNumber(value)
  if (safeValue >= 1000) {
    return (safeValue / 1000).toFixed(2) + 'K'
  } else if (safeValue >= 1) {
    return safeValue.toFixed(2)
  } else if (safeValue >= 0.01) {
    return safeValue.toFixed(3)
  }
  return safeValue.toFixed(4)
}

const formatDuration = (ms: number): string => {
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${Math.round(ms)}ms`
}

// Load data
let snapshotLoadSeq = 0
const loadDashboardSnapshot = async () => {
  const currentSeq = ++snapshotLoadSeq
  if (!stats.value) {
    loading.value = true
  }
  snapshotError.value = false
  try {
    const response = await adminAPI.dashboard.getSnapshotV2({
      include_stats: true,
      include_trend: false,
      include_model_stats: false,
      include_group_stats: false,
      include_users_trend: false
    })
    if (currentSeq !== snapshotLoadSeq) return
    if (response.stats) stats.value = response.stats
  } catch (error) {
    if (currentSeq !== snapshotLoadSeq) return
    snapshotError.value = true
    appStore.showError(t('admin.dashboard.failedToLoad'))
    console.error('Error loading dashboard snapshot:', error)
  } finally {
    if (currentSeq === snapshotLoadSeq) {
      loading.value = false
    }
  }
}

const loadAccountCapacity = async () => {
  const currentSeq = ++capacityLoadSeq
  capacityLoading.value = true
  capacityError.value = false

  try {
    const pageSize = 1000
    const firstPage = await adminAPI.accounts.list(1, pageSize, {
      include_scheduler_score: '0'
    })
    const rows = [...firstPage.items]
    for (let page = 2; page <= firstPage.pages; page += 1) {
      const result = await adminAPI.accounts.list(page, pageSize, {
        include_scheduler_score: '0'
      })
      rows.push(...result.items)
    }
    if (currentSeq !== capacityLoadSeq) return

    const usageAccountIDs = rows.filter(supportsBatchAccountUsage).map((account) => account.id)
    let usage: Record<string, AccountUsageInfo | null> = {}
    let errors: Record<string, string | null> = {}
    if (usageAccountIDs.length) {
      try {
        const result = await adminAPI.accounts.getBatchUsage(usageAccountIDs, false)
        usage = result.usage ?? {}
        errors = result.errors ?? {}
      } catch (error) {
        errors = Object.fromEntries(usageAccountIDs.map((id) => [String(id), 'Failed']))
        console.error('Error loading account usage for capacity:', error)
      }
    }
    if (currentSeq !== capacityLoadSeq) return

    capacityAccounts.value = rows
    capacityUsageByAccountId.value = usage
    capacityUsageErrorByAccountId.value = errors
  } catch (error) {
    if (currentSeq !== capacityLoadSeq) return
    capacityError.value = true
    console.error('Error loading accounts for capacity:', error)
  } finally {
    if (currentSeq === capacityLoadSeq) capacityLoading.value = false
  }
}

const loadDashboardStats = async () => {
  await Promise.all([
    loadDashboardSnapshot(),
    loadAccountCapacity()
  ])
}

onMounted(() => {
  loadDashboardStats()
})
</script>

<style scoped>
.operator-fleet-summary,
.operator-gateway-summary {
  overflow: hidden;
  border: 1px solid var(--operator-border);
  border-radius: 0.5rem;
  background: var(--operator-card);
  box-shadow: var(--operator-shadow-xs);
}

.operator-fleet-heading,
.operator-gateway-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  padding: 1rem 1.25rem;
}

.operator-fleet-heading h2,
.operator-gateway-heading h2 {
  color: var(--operator-foreground);
  font-size: 1rem;
  font-weight: 650;
}

.operator-fleet-heading p,
.operator-gateway-heading p {
  margin-top: 0.1875rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.operator-fleet-disabled {
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.operator-fleet-metrics,
.operator-gateway-metrics {
  display: grid;
  border-top: 1px solid var(--operator-border-subtle);
}

.operator-fleet-metrics { grid-template-columns: repeat(4, minmax(0, 1fr)); }
.operator-fleet-metric {
  display: flex;
  min-width: 0;
  min-height: 6.25rem;
  flex-direction: column;
  justify-content: center;
  padding: 0.875rem 1.25rem;
  color: var(--operator-foreground);
}
.operator-fleet-metric + .operator-fleet-metric,
.operator-gateway-metrics > div + div { border-left: 1px solid var(--operator-border-subtle); }
.operator-fleet-metric:hover { background: var(--operator-muted); }
.operator-fleet-metric:focus-visible {
  position: relative;
  outline: 2px solid var(--operator-focus);
  outline-offset: -3px;
}
.operator-fleet-metric span,
.operator-fleet-metric small,
.operator-gateway-metrics dt {
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
}
.operator-fleet-metric strong {
  margin: 0.125rem 0;
  font-size: 1.75rem;
  font-weight: 700;
  line-height: 1.15;
}
.operator-fleet-metric.is-active strong { color: var(--operator-success); }
.operator-fleet-metric.is-limited strong { color: var(--operator-warning); }
.operator-fleet-metric.is-error strong { color: var(--operator-destructive); }

.operator-gateway-status {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  margin-bottom: 0.375rem;
  color: var(--operator-success);
  font-size: 0.75rem;
  font-weight: 600;
}
.operator-gateway-status span {
  width: 0.4375rem;
  height: 0.4375rem;
  border-radius: 999px;
  background: currentColor;
}
.operator-gateway-heading > a {
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  font-weight: 600;
}
.operator-gateway-heading > a:hover { text-decoration: underline; text-underline-offset: 2px; }
.operator-gateway-metrics { grid-template-columns: repeat(6, minmax(0, 1fr)); }
.operator-gateway-metrics > div { min-width: 0; padding: 0.875rem 1rem; }
.operator-gateway-metrics dd {
  overflow: hidden;
  margin-top: 0.1875rem;
  color: var(--operator-foreground);
  font-size: 0.9375rem;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 1023px) {
  .operator-gateway-metrics { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .operator-gateway-metrics > div:nth-child(4) { border-left: 0; }
  .operator-gateway-metrics > div:nth-child(n + 4) { border-top: 1px solid var(--operator-border-subtle); }
}

@media (max-width: 639px) {
  .operator-fleet-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .operator-fleet-metric:nth-child(3) { border-left: 0; }
  .operator-fleet-metric:nth-child(n + 3) { border-top: 1px solid var(--operator-border-subtle); }
  .operator-gateway-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .operator-gateway-metrics > div:nth-child(3),
  .operator-gateway-metrics > div:nth-child(5) { border-left: 0; }
  .operator-gateway-metrics > div:nth-child(n + 3) { border-top: 1px solid var(--operator-border-subtle); }
}
</style>
