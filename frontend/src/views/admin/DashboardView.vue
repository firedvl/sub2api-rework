<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <section
        v-else-if="snapshotError"
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

      <template v-else-if="stats">
        <section class="operator-stats-grid">
          <article class="card operator-stat-card">
            <div class="operator-stat-heading">
              <span>{{ t('admin.dashboard.todayRequests') }}</span>
              <span class="operator-stat-icon bg-blue-50 text-blue-600 dark:bg-blue-950/50 dark:text-blue-300"><Icon name="chart" size="md" /></span>
            </div>
            <p class="operator-stat-value">{{ formatNumber(stats.today_requests) }}</p>
            <p class="operator-stat-meta">{{ formatNumber(stats.rpm) }} RPM</p>
          </article>
          <article class="card operator-stat-card">
            <div class="operator-stat-heading">
              <span>{{ t('admin.dashboard.todayTokens') }}</span>
              <span class="operator-stat-icon bg-violet-50 text-violet-600 dark:bg-violet-950/50 dark:text-violet-300"><Icon name="database" size="md" /></span>
            </div>
            <p class="operator-stat-value">{{ formatTokens(stats.today_tokens) }}</p>
            <p class="operator-stat-meta">{{ formatTokens(stats.today_input_tokens) }} {{ t('admin.dashboard.input') }}</p>
          </article>
          <article class="card operator-stat-card">
            <div class="operator-stat-heading">
              <span>{{ t('admin.dashboard.todayCost') }}</span>
              <span class="operator-stat-icon bg-emerald-50 text-emerald-600 dark:bg-emerald-950/50 dark:text-emerald-300"><Icon name="dollar" size="md" /></span>
            </div>
            <p class="operator-stat-value">${{ formatCost(stats.today_actual_cost) }}</p>
            <p class="operator-stat-meta">{{ t('admin.dashboard.accountCost') }} ${{ formatCost(stats.today_account_cost) }}</p>
          </article>
          <article class="card operator-stat-card">
            <div class="operator-stat-heading">
              <span>{{ t('admin.dashboard.avgResponse') }}</span>
              <span class="operator-stat-icon bg-amber-50 text-amber-600 dark:bg-amber-950/50 dark:text-amber-300"><Icon name="clock" size="md" /></span>
            </div>
            <p class="operator-stat-value">{{ formatDuration(stats.average_duration_ms) }}</p>
            <p class="operator-stat-meta">{{ formatNumber(stats.hourly_active_users) }} {{ t('admin.dashboard.activeUsers') }}</p>
          </article>
        </section>

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

        <section class="space-y-4">
          <div class="card p-4">
            <div class="flex flex-wrap items-center gap-4">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t('admin.dashboard.timeRange') }}:</span
                >
                <DateRangePicker
                  v-model:start-date="startDate"
                  v-model:end-date="endDate"
                  @change="onDateRangeChange"
                />
              </div>
              <button @click="loadDashboardStats" :disabled="chartsLoading" class="btn btn-secondary">
                {{ t('common.refresh') }}
              </button>
              <div class="ml-auto flex items-center gap-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t('admin.dashboard.granularity') }}:</span
                >
                <div class="w-28">
                  <Select
                    v-model="granularity"
                    :options="granularityOptions"
                    @change="loadChartData"
                  />
                </div>
              </div>
            </div>
          </div>

          <div v-if="chartsLoading || rankingLoading || rankingError || modelStats.length || rankingItems.length || trendData.length" class="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <ModelDistributionChart
              v-if="chartsLoading || rankingLoading || rankingError || modelStats.length || rankingItems.length"
              :model-stats="modelStats"
              :enable-ranking-view="true"
              :ranking-items="rankingItems"
              :ranking-total-actual-cost="rankingTotalActualCost"
              :ranking-total-requests="rankingTotalRequests"
              :ranking-total-tokens="rankingTotalTokens"
              :loading="chartsLoading"
              :ranking-loading="rankingLoading"
              :ranking-error="rankingError"
              :start-date="startDate"
              :end-date="endDate"
              @ranking-click="goToUserUsage"
            />
            <TokenUsageTrend v-if="chartsLoading || trendData.length" :trend-data="trendData" :loading="chartsLoading" />
          </div>

          <div v-if="userTrendLoading || userTrendChartData" class="card p-4">
            <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.dashboard.recentUsage') }} (Top 12)
            </h3>
            <div class="h-64">
              <div v-if="userTrendLoading" class="flex h-full items-center justify-center">
                <LoadingSpinner size="md" />
              </div>
              <Line v-else-if="userTrendChartData" :data="userTrendChartData" :options="lineOptions" />
              <div
                v-else
                class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('admin.dashboard.noDataAvailable') }}
              </div>
            </div>
          </div>

          <button v-if="canUseBatchImage" type="button" class="inline-flex items-center gap-2 text-sm font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200" @click="router.push('/batch-image')">
            <Icon name="sparkles" size="sm" />{{ t('admin.dashboard.batchImage') }}
          </button>
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
  DashboardStats,
  TrendDataPoint,
  ModelStat,
  UserUsageTrendPoint,
  UserSpendingRankingItem
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'

import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'

// Register Chart.js components
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
)

const appStore = useAppStore()
const router = useRouter()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()
const stats = ref<DashboardStats | null>(null)
const loading = ref(false)
const chartsLoading = ref(false)
const userTrendLoading = ref(false)
const rankingLoading = ref(false)
const rankingError = ref(false)
const snapshotError = ref(false)

// Chart data
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const userTrend = ref<UserUsageTrendPoint[]>([])
const rankingItems = ref<UserSpendingRankingItem[]>([])
const rankingTotalActualCost = ref(0)
const rankingTotalRequests = ref(0)
const rankingTotalTokens = ref(0)
let chartLoadSeq = 0
let usersTrendLoadSeq = 0
let rankingLoadSeq = 0
const rankingLimit = 12

// Helper function to format date in local timezone
const formatLocalDate = (date: Date): string => {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLocalDate(start),
    end: formatLocalDate(end)
  }
}

// Date range
const granularity = ref<'day' | 'hour'>('hour')
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)

// Granularity options for Select component
const granularityOptions = computed(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') }
])

const hasGatewayTraffic = computed(() => (stats.value?.rpm ?? 0) > 0 || (stats.value?.tpm ?? 0) > 0)

const hasAccountExceptions = computed(() => {
  const currentStats = stats.value
  return Boolean(
    currentStats && (currentStats.error_accounts > 0 || currentStats.ratelimit_accounts > 0 || currentStats.overload_accounts > 0)
  )
})

// Dark mode detection
const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

// Chart colors
const chartColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb'
}))

// Line chart options (for user trend chart)
const lineOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 15,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      itemSort: (a: any, b: any) => {
        const aValue = typeof a?.raw === 'number' ? a.raw : Number(a?.parsed?.y ?? 0)
        const bValue = typeof b?.raw === 'number' ? b.raw : Number(b?.parsed?.y ?? 0)
        return bValue - aValue
      },
      callbacks: {
        label: (context: any) => {
          return `${context.dataset.label}: ${formatTokens(context.raw)}`
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        }
      }
    },
    y: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => formatTokens(Number(value))
      }
    }
  }
}))

// User trend chart data
const userTrendChartData = computed(() => {
  if (!userTrend.value?.length) return null

  const getDisplayName = (point: UserUsageTrendPoint): string => {
    const username = point.username?.trim()
    if (username) {
      return username
    }

    const email = point.email?.trim()
    if (email) {
      return email
    }

    return t('admin.redeem.userPrefix', { id: point.user_id })
  }

  // Group by user_id to avoid merging different users with the same display name
  const userGroups = new Map<number, { name: string; data: Map<string, number> }>()
  const allDates = new Set<string>()

  userTrend.value.forEach((point) => {
    allDates.add(point.date)
    const key = point.user_id
    if (!userGroups.has(key)) {
      userGroups.set(key, { name: getDisplayName(point), data: new Map() })
    }
    userGroups.get(key)!.data.set(point.date, point.tokens)
  })

  const sortedDates = Array.from(allDates).sort()
  const colors = [
    '#3b82f6',
    '#10b981',
    '#f59e0b',
    '#ef4444',
    '#8b5cf6',
    '#ec4899',
    '#14b8a6',
    '#f97316',
    '#6366f1',
    '#84cc16',
    '#06b6d4',
    '#a855f7'
  ]

  const datasets = Array.from(userGroups.values()).map((group, idx) => ({
    label: group.name,
    data: sortedDates.map((date) => group.data.get(date) || 0),
    borderColor: colors[idx % colors.length],
    backgroundColor: `${colors[idx % colors.length]}20`,
    fill: false,
    tension: 0.3
  }))

  return {
    labels: sortedDates,
    datasets
  }
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

const goToUserUsage = (item: UserSpendingRankingItem) => {
  void router.push({
    path: '/admin/usage',
    query: {
      user_id: String(item.user_id),
      start_date: startDate.value,
      end_date: endDate.value
    }
  })
}

// Date range change handler
const onDateRangeChange = (range: {
  startDate: string
  endDate: string
  preset: string | null
}) => {
  // Auto-select granularity based on date range
  const start = new Date(range.startDate)
  const end = new Date(range.endDate)
  const daysDiff = Math.ceil((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24))

  // If range is 1 day, use hourly granularity
  if (daysDiff <= 1) {
    granularity.value = 'hour'
  } else {
    granularity.value = 'day'
  }

  loadChartData()
}

// Load data
const loadDashboardSnapshot = async (includeStats: boolean) => {
  const currentSeq = ++chartLoadSeq
  if (includeStats && !stats.value) {
    loading.value = true
  }
  if (includeStats) {
    snapshotError.value = false
  }
  chartsLoading.value = true
  try {
    const response = await adminAPI.dashboard.getSnapshotV2({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      include_stats: includeStats,
      include_trend: true,
      include_model_stats: true,
      include_group_stats: false,
      include_users_trend: false
    })
    if (currentSeq !== chartLoadSeq) return
    if (includeStats && response.stats) {
      stats.value = response.stats
    }
    trendData.value = response.trend || []
    modelStats.value = response.models || []
  } catch (error) {
    if (currentSeq !== chartLoadSeq) return
    if (includeStats) {
      snapshotError.value = true
    }
    appStore.showError(t('admin.dashboard.failedToLoad'))
    console.error('Error loading dashboard snapshot:', error)
  } finally {
    if (currentSeq === chartLoadSeq) {
      loading.value = false
      chartsLoading.value = false
    }
  }
}

const loadUsersTrend = async () => {
  const currentSeq = ++usersTrendLoadSeq
  userTrendLoading.value = true
  try {
    const response = await adminAPI.dashboard.getUserUsageTrend({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      limit: 12
    })
    if (currentSeq !== usersTrendLoadSeq) return
    userTrend.value = response.trend || []
  } catch (error) {
    if (currentSeq !== usersTrendLoadSeq) return
    console.error('Error loading users trend:', error)
    userTrend.value = []
  } finally {
    if (currentSeq === usersTrendLoadSeq) {
      userTrendLoading.value = false
    }
  }
}

const loadUserSpendingRanking = async () => {
  const currentSeq = ++rankingLoadSeq
  rankingLoading.value = true
  rankingError.value = false
  try {
    const response = await adminAPI.dashboard.getUserSpendingRanking({
      start_date: startDate.value,
      end_date: endDate.value,
      limit: rankingLimit
    })
    if (currentSeq !== rankingLoadSeq) return
    rankingItems.value = response.ranking || []
    rankingTotalActualCost.value = response.total_actual_cost || 0
    rankingTotalRequests.value = response.total_requests || 0
    rankingTotalTokens.value = response.total_tokens || 0
  } catch (error) {
    if (currentSeq !== rankingLoadSeq) return
    console.error('Error loading user spending ranking:', error)
    rankingItems.value = []
    rankingTotalActualCost.value = 0
    rankingTotalRequests.value = 0
    rankingTotalTokens.value = 0
    rankingError.value = true
  } finally {
    if (currentSeq === rankingLoadSeq) {
      rankingLoading.value = false
    }
  }
}

const loadDashboardStats = async () => {
  await Promise.all([
    loadDashboardSnapshot(true),
    loadUsersTrend(),
    loadUserSpendingRanking()
  ])
}

const loadChartData = async () => {
  await Promise.all([
    loadDashboardSnapshot(false),
    loadUsersTrend(),
    loadUserSpendingRanking()
  ])
}

onMounted(() => {
  void refreshBatchImageAccess()
  loadDashboardStats()
})
</script>

<style scoped>
</style>
