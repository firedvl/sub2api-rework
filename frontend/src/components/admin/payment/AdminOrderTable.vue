<template>
  <section class="ui-card admin-order-card">
    <div class="ui-table-wrap">
      <table class="ui-table" data-testid="admin-orders-table">
        <thead>
          <tr>
            <th scope="col">{{ t('payment.orders.orderId') }}</th>
            <th scope="col" class="hidden sm:table-cell">{{ t('payment.admin.colUser') }}</th>
            <th scope="col" class="hidden md:table-cell">{{ t('payment.orders.paymentMethod') }}</th>
            <th scope="col">{{ t('payment.orders.status') }}</th>
            <th scope="col" class="hidden lg:table-cell">{{ t('payment.orders.createdAt') }}</th>
            <th scope="col" class="is-num">{{ t('payment.orders.payAmount') }}</th>
            <th scope="col" class="w-12"><span class="sr-only">{{ t('common.actions') }}</span></th>
          </tr>
        </thead>
        <tbody v-if="loading && !orders.length">
          <tr v-for="i in 6" :key="i" aria-hidden="true">
            <td><div class="skeleton h-4 w-28 rounded"></div></td>
            <td class="hidden sm:table-cell"><div class="skeleton h-4 w-36 rounded"></div></td>
            <td class="hidden md:table-cell"><div class="skeleton h-4 w-20 rounded"></div></td>
            <td><div class="skeleton h-5 w-16 rounded-full"></div></td>
            <td class="hidden lg:table-cell"><div class="skeleton h-4 w-28 rounded"></div></td>
            <td><div class="skeleton ml-auto h-4 w-16 rounded"></div></td>
            <td></td>
          </tr>
        </tbody>
        <tbody v-else-if="orders.length" :key="page" class="ui-rows-enter" :class="loading ? 'opacity-60' : ''">
          <tr
            v-for="row in orders"
            :key="row.id"
            class="is-clickable"
            :class="row.id === selectedId ? 'is-selected' : ''"
            tabindex="0"
            :aria-label="`#${row.id} ${row.out_trade_no}`"
            @click="emit('detail', row)"
            @keydown.enter.self="emit('detail', row)"
          >
            <td class="max-w-[14rem]">
              <span class="ui-cell-main font-mono">#{{ row.id }}</span>
              <span class="ui-cell-sub" :title="row.out_trade_no">{{ row.out_trade_no }}</span>
            </td>
            <td class="hidden max-w-[16rem] sm:table-cell">
              <span class="ui-cell-main">{{ userLabel(row) }}</span>
              <span class="ui-cell-sub">
                #{{ row.user_id }}<template v-if="row.user_notes"> · {{ row.user_notes }}</template>
              </span>
            </td>
            <td class="hidden md:table-cell">
              <PaymentMethodMark :type="row.payment_type" class="text-sm" />
              <span class="ui-cell-sub">{{ t('payment.admin.' + row.order_type + 'Order', row.order_type) }}</span>
            </td>
            <td>
              <span :class="orderStatusBadgeClass(row.status)" class="whitespace-nowrap">
                {{ t('payment.status.' + row.status.toLowerCase(), row.status) }}
              </span>
            </td>
            <td class="hidden whitespace-nowrap text-xs text-gray-500 dark:text-gray-400 lg:table-cell">
              {{ formatOrderDateTime(row.created_at) }}
            </td>
            <td class="is-num">
              <span class="ui-cell-main">{{ paymentAmountSymbol(row) }}{{ row.pay_amount.toFixed(2) }}</span>
              <span
                v-if="row.amount !== row.pay_amount || (row.bonus_amount ?? 0) > 0"
                class="ui-cell-sub"
                :title="t('payment.orders.creditedAmount')"
              >
                {{ t('admin.adminUi.orders.credited') }} {{ creditedAmountSymbol }}{{ row.amount.toFixed(2) }}
              </span>
              <span v-if="row.refund_amount" class="ui-cell-sub text-red-600 dark:text-red-400" :title="t('payment.admin.refundAmount')">
                −{{ creditedAmountSymbol }}{{ row.refund_amount.toFixed(2) }}
              </span>
            </td>
            <td class="text-right">
              <button
                v-if="orderAction(row.status)"
                type="button"
                class="app-icon-button"
                :class="orderAction(row.status)!.tone === 'danger' ? 'text-red-600 dark:text-red-400' : ''"
                :disabled="orderAction(row.status)!.kind === 'queryRefund' && busyIds?.has(row.id)"
                :aria-label="t(orderAction(row.status)!.labelKey)"
                :title="t(orderAction(row.status)!.labelKey)"
                :data-testid="`order-action-${row.id}`"
                @click.stop="emitAction(row)"
              >
                <Icon
                  :name="orderAction(row.status)!.icon"
                  size="sm"
                  :class="orderAction(row.status)!.kind === 'queryRefund' && busyIds?.has(row.id) ? 'animate-spin' : ''"
                />
              </button>
            </td>
          </tr>
        </tbody>
        <tbody v-else>
          <tr>
            <td colspan="7">
              <div class="ui-empty">
                <span class="ui-empty-icon"><Icon name="receipt" size="md" /></span>
                <b>{{ filtered ? t('admin.adminUi.noMatches') : t('payment.admin.noData') }}</b>
                <button v-if="filtered" type="button" class="btn btn-secondary btn-sm" @click="emit('clearFilters')">
                  {{ t('admin.adminUi.clearFilters') }}
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>

  <Pagination
    v-if="total > 0"
    :page="page"
    :total="total"
    :page-size="pageSize"
    @update:page="emit('update:page', $event)"
    @update:pageSize="emit('update:pageSize', $event)"
  />
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { PaymentOrder } from '@/types/payment'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatOrderDateTime } from '@/components/payment/orderUtils'
import { currencySymbol } from '@/components/payment/currency'
import PaymentMethodMark from './PaymentMethodMark.vue'
import { orderAction, orderStatusBadgeClass } from './orderDisplay'

/** Admin list rows also carry the buyer's identity. */
export type AdminOrderRow = PaymentOrder & { user_email?: string; user_name?: string; user_notes?: string }

const { t } = useI18n()

defineProps<{
  orders: AdminOrderRow[]
  loading: boolean
  page: number
  pageSize: number
  total: number
  selectedId?: number | null
  busyIds?: Set<number>
  filtered?: boolean
}>()

const emit = defineEmits<{
  (e: 'detail', order: AdminOrderRow): void
  (e: 'cancel', order: AdminOrderRow): void
  (e: 'retry', order: AdminOrderRow): void
  (e: 'refund', order: AdminOrderRow): void
  (e: 'queryRefund', order: AdminOrderRow): void
  (e: 'clearFilters'): void
  (e: 'update:page', page: number): void
  (e: 'update:pageSize', size: number): void
}>()

const creditedAmountSymbol = currencySymbol('USD')

function paymentAmountSymbol(order: PaymentOrder): string {
  return currencySymbol(order.currency)
}

function userLabel(order: AdminOrderRow): string {
  return order.user_email || order.user_name || `#${order.user_id}`
}

function emitAction(order: AdminOrderRow) {
  const action = orderAction(order.status)
  if (!action) return
  if (action.kind === 'cancel') emit('cancel', order)
  else if (action.kind === 'retry') emit('retry', order)
  else if (action.kind === 'refund') emit('refund', order)
  else emit('queryRefund', order)
}
</script>

<style scoped>
.admin-order-card {
  overflow: hidden;
  padding: 0;
}
</style>
