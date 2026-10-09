<template>
  <section aria-labelledby="user-usage-trend-title" class="border-t border-gray-200 pt-4 dark:border-dark-700">
    <header class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <h2 id="user-usage-trend-title" class="text-sm font-semibold">{{ t('admin.dashboard.recentUsage') }} (Top 12)</h2>
      <div role="group" :aria-label="t('admin.dashboard.recentUsage')" class="flex gap-1">
        <button v-for="choice in metrics" :key="choice" type="button" class="rounded px-3 py-1.5 text-xs"
          :class="metric === choice ? 'bg-primary-600 text-white' : 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200'"
          :aria-pressed="metric === choice" @click="changeMetric(choice)">
          {{ t(choice === 'tokens' ? 'admin.dashboard.tokens' : 'admin.dashboard.actualSpending') }}
        </button>
      </div>
    </header>
    <div class="h-64">
      <div v-if="loading" class="flex h-full items-center justify-center"><LoadingSpinner /></div>
      <div v-else-if="error" role="alert" class="flex h-full flex-col items-center justify-center gap-3">
        <p class="text-sm text-red-600">{{ t('admin.dashboard.failedToLoad') }}</p>
        <button type="button" class="btn btn-secondary" @click="load"><Icon name="refresh" size="sm" />{{ t('admin.dashboard.retry') }}</button>
      </div>
      <Line v-else-if="points.length" :data="chartData" :options="chartOptions" />
      <div v-else class="flex h-full items-center justify-center text-sm text-gray-500">{{ t('admin.dashboard.noDataAvailable') }}</div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, type ChartOptions } from 'chart.js'
import { Line } from 'vue-chartjs'
import { adminAPI } from '@/api/admin'
import type { UserUsageTrendPoint } from '@/types'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)
const { t } = useI18n()
const metrics = ['tokens', 'actual_cost'] as const
type Metric = typeof metrics[number]
const metric = ref<Metric>('tokens')
const points = ref<UserUsageTrendPoint[]>([])
const loading = ref(false)
const error = ref(false)
let sequence = 0
const colors = ['#008c82', '#2563eb', '#dc2626', '#7c3aed', '#b45309', '#0891b2', '#475569', '#be185d', '#4d7c0f', '#9333ea', '#c2410c', '#0f766e']
const chartData = computed(() => {
  const labels = [...new Set(points.value.map(point => point.date))].sort()
  const users = [...new Set(points.value.map(point => point.user_id))]
  return { labels, datasets: users.map((id, index) => {
    const entries = points.value.filter(point => point.user_id === id)
    const values = new Map(entries.map(point => [point.date, metric.value === 'tokens' ? point.tokens : point.actual_cost]))
    return { label: entries[0]?.username || entries[0]?.email || `#${id}`, data: labels.map(date => values.get(date) ?? 0), borderColor: colors[index % colors.length], pointRadius: 2 }
  }) }
})
const formatValue = (value: number) => metric.value === 'actual_cost' ? `$${value.toFixed(2)}` : value.toLocaleString()
const chartOptions = computed<ChartOptions<'line'>>(() => ({
  responsive: true, maintainAspectRatio: false,
  plugins: { tooltip: { callbacks: { label: context => `${context.dataset.label}: ${formatValue(Number(context.raw))}` } } },
  scales: { y: { beginAtZero: true, ticks: { callback: value => formatValue(Number(value)) } } }
}))
async function load() {
  const current = ++sequence
  loading.value = true
  error.value = false
  try {
    const result = await adminAPI.dashboard.getUserUsageTrend({ granularity: 'day', limit: 12, metric: metric.value })
    if (current === sequence) points.value = result.trend || []
  } catch {
    if (current === sequence) error.value = true
  } finally {
    if (current === sequence) loading.value = false
  }
}
function changeMetric(next: Metric) {
  if (metric.value === next) return
  metric.value = next
  points.value = []
  void load()
}
onMounted(load)
onBeforeUnmount(() => { sequence += 1 })
</script>
