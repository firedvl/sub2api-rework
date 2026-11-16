<template>
  <section class="viz-card" aria-labelledby="payment-revenue-title" data-testid="payment-revenue">
    <header class="viz-card-head">
      <h2 id="payment-revenue-title" class="viz-card-title">{{ t('payment.admin.dailyRevenue') }}</h2>
      <div v-if="metrics.length > 1" class="viz-card-tools">
        <div class="viz-seg" role="group" :aria-label="t('payment.admin.dailyRevenue')">
          <button
            v-for="option in metrics"
            :key="option.key"
            type="button"
            :aria-pressed="activeMetric === option.key"
            @click="metric = option.key"
          >
            {{ option.label }}
          </button>
        </div>
      </div>
    </header>
    <div class="viz-card-body">
      <div v-if="loading" class="skeleton h-64 rounded-lg"></div>
      <TrendChart
        v-else-if="chart"
        :type="chart.type"
        :labels="chart.labels"
        :tips="chart.tips"
        :series="chart.series"
        :summary="chart.summary"
        :height="256"
        :format-value="chart.format"
      />
      <div v-else class="ui-empty">
        <span class="ui-empty-icon"><Icon name="chart" size="md" /></span>
        <b>{{ t('payment.admin.noData') }}</b>
      </div>
    </div>
    <footer v-if="chart" class="viz-card-foot">
      <span v-for="item in totals" :key="item.key">{{ item.label }} <b>{{ item.value }}</b></span>
    </footer>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import TrendChart, { type TrendSeries } from '@/components/viz/TrendChart.vue'
import type { DailyPaymentStats } from '@/types/payment'
import { formatMoney } from './orderDisplay'

const { t } = useI18n()

const props = defineProps<{
  data: DailyPaymentStats[]
  loading?: boolean
}>()

const ORDERS = '__orders'

const currencies = computed(() => [...new Set(props.data.flatMap((day) => Object.keys(day.amount || {})))].sort())

/** One button per currency (amounts never share an axis) plus the order count. */
const metrics = computed(() => [
  ...currencies.value.map((currency) => ({ key: currency, label: currency })),
  { key: ORDERS, label: t('payment.admin.orders') },
])

const metric = ref('')
const activeMetric = computed(() =>
  metrics.value.some((option) => option.key === metric.value) ? metric.value : metrics.value[0].key,
)

const chart = computed(() => {
  if (!props.data?.length) return null
  const labels = props.data.map((day) => day.date.slice(5))
  const tips = props.data.map((day) => day.date)
  if (activeMetric.value === ORDERS) {
    const series: TrendSeries[] = [{ name: t('payment.admin.orderCount'), color: 'var(--operator-accent)', values: props.data.map((day) => day.count) }]
    return {
      type: 'bar' as const,
      labels,
      tips,
      series,
      summary: t('payment.admin.orderCount'),
      format: (value: number) => Math.round(value).toLocaleString(),
    }
  }
  const currency = activeMetric.value
  const series: TrendSeries[] = [{
    name: `${currency} ${t('payment.admin.revenue')}`,
    color: 'var(--operator-accent-text)',
    values: props.data.map((day) => day.amount?.[currency] || 0),
  }]
  return {
    type: 'line' as const,
    labels,
    tips,
    series,
    summary: `${currency} ${t('payment.admin.revenue')}`,
    format: (value: number) => formatMoney(currency, value),
  }
})

const totals = computed(() => [
  ...currencies.value.map((currency) => ({
    key: currency,
    label: currency,
    value: formatMoney(currency, props.data.reduce((sum, day) => sum + (day.amount?.[currency] || 0), 0)),
  })),
  {
    key: ORDERS,
    label: t('payment.admin.orders'),
    value: props.data.reduce((sum, day) => sum + day.count, 0).toLocaleString(),
  },
])
</script>
