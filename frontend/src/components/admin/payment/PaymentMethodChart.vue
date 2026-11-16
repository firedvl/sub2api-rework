<template>
  <section class="viz-card" aria-labelledby="payment-methods-title" data-testid="payment-methods">
    <header class="viz-card-head">
      <h2 id="payment-methods-title" class="viz-card-title">{{ t('payment.admin.paymentDistribution') }}</h2>
    </header>
    <div class="viz-card-body">
      <div v-if="!methods?.length" class="ui-empty">
        <span class="ui-empty-icon"><Icon name="creditCard" size="md" /></span>
        <b>{{ t('payment.admin.noData') }}</b>
      </div>
      <div v-else class="viz-hbars">
        <div v-for="method in methods" :key="method.type" class="viz-hbar">
          <PaymentMethodMark :type="method.type" class="min-w-0 text-sm font-medium text-gray-900 dark:text-white" />
          <span class="viz-row-value">
            <span v-for="[currency, amount] in sortedAmounts(method.amount)" :key="currency" class="block">{{ formatMoney(currency, amount) }}</span>
          </span>
          <span class="viz-hbar-track" :title="`${method.count} ${t('payment.admin.orders')}`">
            <i :style="{ width: share(method) + '%', background: 'var(--operator-accent)' }"></i>
          </span>
          <span class="col-span-2 text-xs text-gray-500 dark:text-gray-400">{{ method.count.toLocaleString() }} {{ t('payment.admin.orders') }}</span>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { PaymentMethodStats } from '@/types/payment'
import PaymentMethodMark from './PaymentMethodMark.vue'
import { formatMoney, sortedAmounts } from './orderDisplay'

const { t } = useI18n()

const props = defineProps<{
  methods: PaymentMethodStats[]
}>()

/** Bars compare order counts, which stay comparable across currencies. */
const maxCount = computed(() => Math.max(1, ...props.methods.map((method) => method.count)))

function share(method: PaymentMethodStats): number {
  return Math.max(2, Math.min(100, (method.count / maxCount.value) * 100))
}
</script>
