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
        <section class="operator-stats-grid">
          <article class="card operator-stat-card">
            <div class="operator-stat-heading">
              <span>{{ t('admin.dashboard.todayRequests') }}</span>
              <span class="operator-stat-icon operator-stat-icon-neutral"><Icon name="chart" size="md" /></span>
            </div>
            <p class="operator-stat-value">{{ formatNumber(stats.today_requests) }}</p>
            <p class="operator-stat-meta">{{ formatNumber(stats.rpm) }} RPM</p>
          </article>
          <article class="card operator-stat-card">
            <div class="operator-stat-heading">
              <span>{{ t('admin.dashboard.todayTokens') }}</span>
              <span class="operator-stat-icon operator-stat-icon-neutral"><Icon name="database" size="md" /></span>
            </div>
            <p class="operator-stat-value">{{ formatTokens(stats.today_tokens) }}</p>
            <p class="operator-stat-meta">{{ formatTokens(stats.today_input_tokens) }} {{ t('admin.dashboard.input') }}</p>
          </article>
          <article class="card operator-stat-card">
            <div class="operator-stat-heading">
              <span>{{ t('admin.dashboard.todayCost') }}</span>
              <span class="operator-stat-icon operator-stat-icon-neutral"><Icon name="dollar" size="md" /></span>
            </div>
            <p class="operator-stat-value">${{ formatCost(stats.today_actual_cost) }}</p>
            <p class="operator-stat-meta">{{ t('admin.dashboard.accountCost') }} ${{ formatCost(stats.today_account_cost) }}</p>
          </article>
          <article class="card operator-stat-card">
            <div class="operator-stat-heading">
              <span>{{ t('admin.dashboard.avgResponse') }}</span>
              <span class="operator-stat-icon operator-stat-icon-neutral"><Icon name="clock" size="md" /></span>
            </div>
            <p class="operator-stat-value">{{ formatDuration(stats.average_duration_ms) }}</p>
            <p class="operator-stat-meta">{{ formatNumber(stats.hourly_active_users) }} {{ t('admin.dashboard.activeUsers') }}</p>
          </article>
        </section>
      </template>

      <OperatorCapacityOverview
        v-if="!loading"
        :accounts="capacityAccounts"
        :loading="capacityLoading"
        :error="capacityError"
        compact
        @retry="loadAccountCapacity"
      />

      <template v-if="!loading && stats">
        <section v-if="hasAccountExceptions" class="flex flex-wrap items-center justify-between gap-3 border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-100">
          <div class="flex items-center gap-2"><Icon name="server" size="sm" /><span>{{ t('admin.dashboard.accountAttention') }}</span></div>
          <button type="button" class="font-medium underline underline-offset-2" @click="router.push('/admin/accounts')">{{ t('admin.dashboard.reviewAccounts') }}</button>
        </section>

        <section class="grid gap-4 lg:grid-cols-2">
          <article class="card operator-summary-panel">
            <div class="operator-summary-header">
              <div>
                <h2>{{ t('admin.dashboard.accountHealth') }}</h2>
                <p>{{ hasAccountExceptions ? t('admin.dashboard.accountAttention') : t('admin.dashboard.noAccountExceptions') }}</p>
              </div>
              <button type="button" @click="router.push('/admin/accounts')">{{ t('admin.dashboard.viewAccounts') }}</button>
            </div>
            <p class="operator-summary-value">{{ formatNumber(stats.normal_accounts) }} <span>/ {{ formatNumber(stats.total_accounts) }}</span></p>
            <dl class="operator-summary-grid">
              <div><dt>{{ t('common.error') }}</dt><dd>{{ formatNumber(stats.error_accounts) }}</dd></div>
              <div><dt>{{ t('admin.dashboard.throttled') }}</dt><dd>{{ formatNumber(stats.ratelimit_accounts) }}</dd></div>
              <div><dt>{{ t('admin.dashboard.overloaded') }}</dt><dd>{{ formatNumber(stats.overload_accounts) }}</dd></div>
            </dl>
          </article>
          <article class="card operator-summary-panel">
            <div class="operator-summary-header">
              <div>
                <h2>{{ t('admin.dashboard.gatewayTraffic') }}</h2>
                <p>{{ hasGatewayTraffic ? t('admin.dashboard.receivingTraffic') : t('admin.dashboard.idleTraffic') }}</p>
              </div>
              <button type="button" @click="router.push('/admin/usage')">{{ t('admin.dashboard.viewActivity') }}</button>
            </div>
            <p class="operator-summary-value">{{ formatNumber(stats.rpm) }} <span>RPM</span></p>
            <dl class="operator-summary-grid">
              <div><dt>TPM</dt><dd>{{ formatTokens(stats.tpm) }}</dd></div>
              <div><dt>{{ t('admin.dashboard.activeUsers') }}</dt><dd>{{ formatNumber(stats.hourly_active_users) }}</dd></div>
              <div><dt>{{ t('common.total') }}</dt><dd>{{ formatNumber(stats.total_requests) }}</dd></div>
            </dl>
          </article>
        </section>

      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
import { adminAPI } from '@/api/admin'
import type {
  Account,
  DashboardStats
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import OperatorCapacityOverview from '@/components/admin/OperatorCapacityOverview.vue'

const appStore = useAppStore()
const router = useRouter()
const stats = ref<DashboardStats | null>(null)
const loading = ref(false)
const snapshotError = ref(false)
const capacityAccounts = ref<Account[]>([])
const capacityLoading = ref(false)
const capacityError = ref(false)
let capacityLoadSeq = 0

const hasGatewayTraffic = computed(() => (stats.value?.rpm ?? 0) > 0 || (stats.value?.tpm ?? 0) > 0)

const hasAccountExceptions = computed(() => {
  const currentStats = stats.value
  return Boolean(
    currentStats && (currentStats.error_accounts > 0 || currentStats.ratelimit_accounts > 0 || currentStats.overload_accounts > 0)
  )
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
    capacityAccounts.value = rows
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
</style>
