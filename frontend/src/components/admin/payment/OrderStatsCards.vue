<template>
  <dl class="viz-kpis order-kpis" data-testid="payment-kpis">
    <div v-for="tile in tiles" :key="tile.key" class="viz-card viz-kpi">
      <dt class="viz-kpi-label">
        <Icon :name="tile.icon" size="sm" />
        {{ tile.label }}
      </dt>
      <dd class="viz-kpi-row">
        <span class="viz-kpi-value">
          <CountUp v-if="tile.count !== undefined" :value="tile.count" />
          <CountUp v-else-if="tile.amounts.length" :value="tile.amounts[0][1]" :format="moneyFormat(tile.amounts[0][0])" />
          <template v-else>–</template>
        </span>
      </dd>
      <dd v-if="tile.amounts.length > 1 || tile.sub" class="order-kpi-sub">
        <span v-for="[currency, amount] in tile.amounts.slice(1)" :key="currency">{{ formatMoney(currency, amount) }}</span>
        <span v-if="tile.sub">{{ tile.sub }}</span>
      </dd>
    </div>
  </dl>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import CountUp from '@/components/ui/CountUp.vue'
import type { DashboardStats } from '@/types/payment'
import { formatMoney, sortedAmounts } from './orderDisplay'

const { t } = useI18n()

const props = defineProps<{
  stats: DashboardStats
}>()

type TileIcon = 'dollar' | 'receipt' | 'creditCard' | 'calculator'

interface Tile {
  key: string
  label: string
  icon: TileIcon
  amounts: [string, number][]
  count?: number
  sub?: string
}

const tiles = computed<Tile[]>(() => [
  { key: 'today', label: t('payment.admin.todayRevenue'), icon: 'dollar', amounts: sortedAmounts(props.stats.today_amount) },
  { key: 'todayCount', label: t('payment.admin.todayOrders'), icon: 'receipt', amounts: [], count: props.stats.today_count },
  {
    key: 'total',
    label: t('payment.admin.totalRevenue'),
    icon: 'creditCard',
    amounts: sortedAmounts(props.stats.total_amount),
    sub: `${props.stats.total_count.toLocaleString()} ${t('payment.admin.orders')}`,
  },
  { key: 'avg', label: t('payment.admin.avgAmount'), icon: 'calculator', amounts: sortedAmounts(props.stats.avg_amount) },
])

function moneyFormat(currency: string) {
  return (value: number) => formatMoney(currency, value)
}
</script>

<style scoped>
.order-kpis {
  margin-bottom: 0;
}

.order-kpi-sub {
  display: flex;
  flex-wrap: wrap;
  gap: 0.125rem 0.75rem;
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
}
</style>
