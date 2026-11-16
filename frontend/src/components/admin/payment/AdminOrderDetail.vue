<template>
  <SideDrawer
    :open="show && !!order"
    :title="order ? `#${order.id}` : t('payment.admin.orderDetail')"
    :subtitle="order?.out_trade_no"
    width="32rem"
    @close="emit('close')"
  >
    <template v-if="order" #leading>
      <PaymentMethodMark :type="order.payment_type" :size="32" icon-only />
    </template>
    <template v-if="order" #badge>
      <span :class="orderStatusBadgeClass(order.status)" class="whitespace-nowrap">
        {{ t('payment.status.' + order.status.toLowerCase(), order.status) }}
      </span>
    </template>

    <template v-if="order">
      <div class="order-hero">
        <span class="order-hero-value">{{ paymentAmountSymbol }}{{ order.pay_amount.toFixed(2) }}</span>
        <span v-if="order.amount !== order.pay_amount || (order.bonus_amount ?? 0) > 0" class="order-hero-sub">
          {{ t('admin.adminUi.orders.credited') }} {{ creditedAmountSymbol }}{{ order.amount.toFixed(2) }}
        </span>
      </div>

      <section>
        <h3 class="ui-section-label">{{ t('payment.orders.amount') }}</h3>
        <dl class="ui-dl">
          <dt>{{ t('payment.orders.baseAmount') }}</dt>
          <dd class="tabular-nums">{{ paymentAmountSymbol }}{{ baseAmount.toFixed(2) }}</dd>
          <template v-if="order.fee_rate > 0">
            <dt>{{ t('payment.orders.fee') }} ({{ order.fee_rate }}%)</dt>
            <dd class="tabular-nums">{{ paymentAmountSymbol }}{{ feeAmount.toFixed(2) }}</dd>
          </template>
          <dt>{{ t('payment.orders.payAmount') }}</dt>
          <dd class="tabular-nums">{{ paymentAmountSymbol }}{{ order.pay_amount.toFixed(2) }}</dd>
          <template v-if="(order.bonus_amount ?? 0) > 0">
            <dt>{{ t('payment.orders.bonusAmount') }}</dt>
            <dd class="tabular-nums text-amber-600 dark:text-amber-400">+{{ creditedAmountSymbol }}{{ (order.bonus_amount ?? 0).toFixed(2) }}</dd>
          </template>
          <template v-if="order.amount !== order.pay_amount || (order.bonus_amount ?? 0) > 0">
            <dt>{{ t('payment.orders.creditedAmount') }}</dt>
            <dd class="tabular-nums">{{ creditedAmountSymbol }}{{ order.amount.toFixed(2) }}</dd>
          </template>
        </dl>
      </section>

      <section>
        <h3 class="ui-section-label">{{ t('admin.adminUi.details') }}</h3>
        <dl class="ui-dl">
          <dt>{{ t('payment.orders.orderNo') }}</dt>
          <dd class="font-mono text-xs">{{ order.out_trade_no }}</dd>
          <dt>{{ t('payment.admin.colUser') }}</dt>
          <dd>
            <span v-if="order.user_email || order.user_name">{{ order.user_email || order.user_name }} </span>
            <span class="text-gray-500 dark:text-gray-400">#{{ order.user_id }}</span>
            <span v-if="order.user_notes" class="block text-xs text-gray-500 dark:text-gray-400">{{ order.user_notes }}</span>
          </dd>
          <dt>{{ t('payment.orders.paymentMethod') }}</dt>
          <dd><PaymentMethodMark :type="order.payment_type" /></dd>
          <dt>{{ t('payment.admin.orderType') }}</dt>
          <dd>{{ t('payment.admin.' + order.order_type + 'Order', order.order_type) }}</dd>
          <dt>{{ t('payment.orders.createdAt') }}</dt>
          <dd>{{ formatDateTime(order.created_at) }}</dd>
          <dt>{{ t('payment.admin.expiresAt') }}</dt>
          <dd>{{ formatDateTime(order.expires_at) }}</dd>
          <template v-if="order.paid_at">
            <dt>{{ t('payment.admin.paidAt') }}</dt>
            <dd>{{ formatDateTime(order.paid_at) }}</dd>
          </template>
          <template v-if="order.completed_at">
            <dt>{{ t('payment.admin.completedAt') }}</dt>
            <dd>{{ formatDateTime(order.completed_at) }}</dd>
          </template>
        </dl>
      </section>

      <section v-if="order.refund_amount || order.refund_reason">
        <h3 class="ui-section-label">{{ t('payment.admin.refundInfo') }}</h3>
        <dl class="ui-dl">
          <template v-if="order.refund_amount">
            <dt>{{ t('payment.admin.refundAmount') }}</dt>
            <dd class="tabular-nums font-medium text-red-600 dark:text-red-400">{{ creditedAmountSymbol }}{{ order.refund_amount.toFixed(2) }}</dd>
          </template>
          <template v-if="order.refund_reason">
            <dt>{{ t('payment.admin.refundReason') }}</dt>
            <dd>{{ order.refund_reason }}</dd>
          </template>
        </dl>
      </section>

      <section v-if="order.refund_requested_at">
        <h3 class="ui-section-label">{{ t('payment.admin.refundRequestInfo') }}</h3>
        <dl class="ui-dl">
          <dt>{{ t('payment.admin.refundRequestedAt') }}</dt>
          <dd>{{ formatDateTime(order.refund_requested_at) }}</dd>
          <dt>{{ t('payment.admin.refundRequestedBy') }}</dt>
          <dd>#{{ order.refund_requested_by }}</dd>
          <dt>{{ t('payment.admin.refundRequestReason') }}</dt>
          <dd>{{ order.refund_request_reason || '-' }}</dd>
        </dl>
      </section>

      <section v-if="auditLogs?.length">
        <h3 class="ui-section-label">{{ t('payment.admin.auditLogs') }}</h3>
        <ol class="order-log">
          <li v-for="log in auditLogs" :key="log.id">
            <div class="flex items-center justify-between gap-3">
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{ log.action }}</span>
              <span class="shrink-0 text-xs text-gray-500 dark:text-gray-400">{{ formatDateTime(log.created_at) }}</span>
            </div>
            <div v-if="log.detail" class="mt-0.5 break-all text-xs text-gray-500 dark:text-gray-400">{{ log.detail }}</div>
            <div v-if="log.operator" class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('payment.admin.operator') }}: {{ log.operator }}</div>
          </li>
        </ol>
      </section>
    </template>

    <template v-if="order && action" #footer>
      <button
        type="button"
        :class="action.tone === 'danger' ? 'btn btn-danger' : 'btn btn-primary'"
        :disabled="action.kind === 'queryRefund' && querying"
        data-testid="order-drawer-action"
        @click="runAction"
      >
        <Icon :name="action.icon" size="sm" :class="action.kind === 'queryRefund' && querying ? 'animate-spin' : ''" />
        {{ t(action.labelKey) }}
      </button>
    </template>
  </SideDrawer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import Icon from '@/components/icons/Icon.vue'
import type { PaymentOrder } from '@/types/payment'
import { formatOrderDateTime } from '@/components/payment/orderUtils'
import { currencySymbol } from '@/components/payment/currency'
import PaymentMethodMark from './PaymentMethodMark.vue'
import { orderAction, orderStatusBadgeClass } from './orderDisplay'

export interface OrderAuditLog {
  id: number
  action: string
  detail: string | null
  operator: string | null
  created_at: string
}

type DetailOrder = PaymentOrder & { user_email?: string; user_name?: string; user_notes?: string }

const { t } = useI18n()

const props = defineProps<{
  show: boolean
  order: DetailOrder | null
  auditLogs?: OrderAuditLog[]
  /** A refund-status query for this order is in flight. */
  querying?: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'cancel', order: PaymentOrder): void
  (e: 'retry', order: PaymentOrder): void
  (e: 'refund', order: PaymentOrder): void
  (e: 'queryRefund', order: PaymentOrder): void
}>()

const creditedAmountSymbol = currencySymbol('USD')

const paymentAmountSymbol = computed(() => currencySymbol(props.order?.currency))

const action = computed(() => (props.order ? orderAction(props.order.status) : null))

/** 充值金额 (base amount before fee) = pay_amount - fee = pay_amount / (1 + fee_rate/100) */
const baseAmount = computed(() => {
  if (!props.order) return 0
  const feeRate = Number(props.order.fee_rate) || 0
  if (feeRate <= 0) return props.order.pay_amount
  return props.order.pay_amount / (1 + feeRate / 100)
})

/** 手续费 = pay_amount - baseAmount */
const feeAmount = computed(() => {
  if (!props.order) return 0
  const feeRate = Number(props.order.fee_rate) || 0
  if (feeRate <= 0) return 0
  return props.order.pay_amount - baseAmount.value
})

function runAction() {
  const order = props.order
  if (!order || !action.value) return
  if (action.value.kind === 'cancel') emit('cancel', order)
  else if (action.value.kind === 'retry') emit('retry', order)
  else if (action.value.kind === 'refund') emit('refund', order)
  else emit('queryRefund', order)
}

function formatDateTime(dateStr: string): string {
  return formatOrderDateTime(dateStr)
}
</script>

<style scoped>
.order-hero {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.25rem 0.75rem;
}

.order-hero-value {
  color: var(--operator-foreground);
  font-size: 1.75rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.025em;
}

.order-hero-sub {
  color: var(--operator-subtle-foreground);
  font-size: 0.8125rem;
  font-variant-numeric: tabular-nums;
}

.order-log {
  display: grid;
  max-height: 16rem;
  gap: 0.5rem;
  overflow-y: auto;
}

.order-log > li {
  padding: 0.5rem 0.625rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
}
</style>
