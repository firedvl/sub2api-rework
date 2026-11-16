<template>
  <AppLayout>
    <div class="ui-page ui-stagger">
      <div class="ui-toolbar" role="toolbar" :aria-label="t('payment.admin.orders')">
        <label class="ui-search">
          <Icon name="search" size="sm" />
          <input
            v-model="orderSearch"
            type="search"
            :placeholder="t('payment.admin.searchOrders')"
            :aria-label="t('common.search')"
            @input="debounceLoadOrders"
          />
        </label>
        <div class="viz-seg orders-status-seg" role="group" :aria-label="t('payment.orders.status')">
          <button
            v-for="option in quickStatuses"
            :key="option.value"
            type="button"
            :aria-pressed="orderFilters.status === option.value"
            :data-testid="`orders-status-${option.value || 'all'}`"
            @click="setStatus(option.value)"
          >
            {{ option.label }}
          </button>
        </div>
        <ToolbarPopover :label="t('admin.adminUi.filters')" icon="filter" :count="activeFilterCount" test-id="orders-filters">
          <label class="orders-filter-field">
            <span class="input-label">{{ t('payment.orders.status') }}</span>
            <Select v-model="orderFilters.status" :options="statusFilterOptions" @change="reloadFromFirstPage" />
          </label>
          <label class="orders-filter-field">
            <span class="input-label">{{ t('payment.orders.paymentMethod') }}</span>
            <Select v-model="orderFilters.payment_type" :options="paymentTypeFilterOptions" @change="reloadFromFirstPage" />
          </label>
          <label class="orders-filter-field">
            <span class="input-label">{{ t('payment.orders.orderType') }}</span>
            <Select v-model="orderFilters.order_type" :options="orderTypeFilterOptions" @change="reloadFromFirstPage" />
          </label>
          <button v-if="activeFilterCount" type="button" class="btn btn-ghost btn-sm self-start" @click="clearFilters">
            {{ t('admin.adminUi.clearFilters') }}
          </button>
        </ToolbarPopover>
        <span class="ui-grow" />
        <button
          type="button"
          class="app-icon-button"
          :disabled="ordersLoading"
          :aria-label="t('common.refresh')"
          :title="t('common.refresh')"
          @click="loadOrders"
        >
          <Icon name="refresh" size="sm" :class="ordersLoading ? 'animate-spin' : ''" />
        </button>
      </div>

      <div class="ui-page">
        <AdminOrderTable
          :orders="orders"
          :loading="ordersLoading"
          :page="orderPagination.page"
          :page-size="orderPagination.page_size"
          :total="orderPagination.total"
          :selected-id="showDetailDialog ? selectedOrder?.id : null"
          :busy-ids="refundQueryingIds"
          :filtered="isFiltered"
          @detail="showOrderDetail"
          @cancel="handleCancelOrder"
          @retry="handleRetryOrder"
          @refund="openRefundDialog"
          @query-refund="handleQueryRefund"
          @clear-filters="clearFilters"
          @update:page="handleOrderPageChange"
          @update:page-size="handleOrderPageSizeChange"
        />
      </div>
    </div>

    <AdminOrderDetail
      :show="showDetailDialog"
      :order="selectedOrder"
      :audit-logs="orderAuditLogs"
      :querying="!!selectedOrder && refundQueryingIds.has(selectedOrder.id)"
      @close="showDetailDialog = false"
      @cancel="handleCancelOrder"
      @retry="handleRetryOrder"
      @refund="openRefundDialog"
      @query-refund="handleQueryRefund"
    />

    <AdminRefundDialog :show="showRefundDialog" :order="selectedOrder" :submitting="refundSubmitting" :require-force="refundRequireForce" :warning="refundWarning" @confirm="handleRefund" @cancel="closeRefundDialog" />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { PaymentOrder } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import ToolbarPopover from '@/components/common/ToolbarPopover.vue'
import Icon from '@/components/icons/Icon.vue'
import AdminRefundDialog from '@/components/admin/payment/AdminRefundDialog.vue'
import AdminOrderTable, { type AdminOrderRow } from '@/components/admin/payment/AdminOrderTable.vue'
import AdminOrderDetail, { type OrderAuditLog } from '@/components/admin/payment/AdminOrderDetail.vue'

const { t } = useI18n()
const appStore = useAppStore()

const ordersLoading = ref(false)
const orders = ref<AdminOrderRow[]>([])
const orderSearch = ref('')
const orderFilters = reactive({ status: '', payment_type: '', order_type: '' })
const orderPagination = reactive({ page: 1, page_size: 20, total: 0 })
const selectedOrder = ref<AdminOrderRow | null>(null)
const showDetailDialog = ref(false)
const showRefundDialog = ref(false)
const refundSubmitting = ref(false)
const refundRequireForce = ref(false)
const refundWarning = ref('')
const refundQueryingIds = ref(new Set<number>())
const orderAuditLogs = ref<OrderAuditLog[]>([])

let debounceTimer: ReturnType<typeof setTimeout> | null = null
function debounceLoadOrders() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => reloadFromFirstPage(), 300)
}

async function loadOrders() {
  ordersLoading.value = true
  try {
    const res = await adminPaymentAPI.getOrders({
      page: orderPagination.page, page_size: orderPagination.page_size,
      keyword: orderSearch.value || undefined, status: orderFilters.status || undefined,
      payment_type: orderFilters.payment_type || undefined, order_type: orderFilters.order_type || undefined,
    })
    orders.value = res.data.items || []
    orderPagination.total = res.data.total || 0
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally { ordersLoading.value = false }
}

function reloadFromFirstPage() { orderPagination.page = 1; loadOrders() }
function handleOrderPageChange(page: number) { orderPagination.page = page; loadOrders() }
function handleOrderPageSizeChange(size: number) { orderPagination.page_size = size; orderPagination.page = 1; loadOrders() }

function setStatus(status: string) {
  if (orderFilters.status === status) return
  orderFilters.status = status
  reloadFromFirstPage()
}

function clearFilters() {
  orderSearch.value = ''
  orderFilters.status = ''
  orderFilters.payment_type = ''
  orderFilters.order_type = ''
  reloadFromFirstPage()
}

const quickStatuses = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'PENDING', label: t('payment.status.pending') },
  { value: 'COMPLETED', label: t('payment.status.completed') },
  { value: 'REFUND_REQUESTED', label: t('payment.status.refund_requested') },
  { value: 'FAILED', label: t('payment.status.failed') },
])

/** Status counts only when it is outside the quick segment; the popover still owns the full list. */
const activeFilterCount = computed(() =>
  Number(!quickStatuses.value.some((option) => option.value === orderFilters.status))
  + Number(!!orderFilters.payment_type)
  + Number(!!orderFilters.order_type),
)

const isFiltered = computed(() =>
  !!orderSearch.value || !!orderFilters.status || !!orderFilters.payment_type || !!orderFilters.order_type,
)

const statusFilterOptions = computed(() => [
  { value: '', label: t('payment.admin.allStatuses') },
  { value: 'PENDING', label: t('payment.status.pending') },
  { value: 'PAID', label: t('payment.status.paid') },
  { value: 'COMPLETED', label: t('payment.status.completed') },
  { value: 'EXPIRED', label: t('payment.status.expired') },
  { value: 'CANCELLED', label: t('payment.status.cancelled') },
  { value: 'FAILED', label: t('payment.status.failed') },
  { value: 'REFUNDED', label: t('payment.status.refunded') },
  { value: 'REFUND_REQUESTED', label: t('payment.status.refund_requested') },
  { value: 'REFUND_PENDING', label: t('payment.status.refund_pending') },
  { value: 'REFUND_FAILED', label: t('payment.status.refund_failed') },
])

const paymentTypeFilterOptions = computed(() => [
  { value: '', label: t('payment.admin.allPaymentTypes') },
  { value: 'alipay', label: t('payment.methods.alipay') },
  { value: 'wxpay', label: t('payment.methods.wxpay') },
  { value: 'stripe', label: t('payment.methods.stripe') },
  { value: 'airwallex', label: t('payment.methods.airwallex') },
])

const orderTypeFilterOptions = computed(() => [
  { value: '', label: t('payment.admin.allOrderTypes') },
  { value: 'balance', label: t('payment.admin.balanceOrder') },
  { value: 'subscription', label: t('payment.admin.subscriptionOrder') },
])

async function showOrderDetail(order: AdminOrderRow) {
  selectedOrder.value = order
  orderAuditLogs.value = []
  showDetailDialog.value = true
  try {
    const res = await adminPaymentAPI.getOrder(order.id)
    const data = res.data as unknown as Record<string, unknown>
    // Keep the list row's user fields; the detail payload may omit them.
    if (data.order && selectedOrder.value?.id === order.id) {
      const detail = data.order as AdminOrderRow
      selectedOrder.value = {
        ...order,
        ...detail,
        user_email: detail.user_email || order.user_email,
        user_name: detail.user_name || order.user_name,
        user_notes: detail.user_notes || order.user_notes,
      }
    }
    orderAuditLogs.value = ((data.auditLogs || data.audit_logs || []) as unknown) as OrderAuditLog[]
  } catch (_err: unknown) { /* keep cached order data */ }
}

/** After an action, refresh the list and keep an open drawer in sync with the new row. */
async function refreshAfterAction() {
  await loadOrders()
  if (!showDetailDialog.value || !selectedOrder.value) return
  const fresh = orders.value.find((row) => row.id === selectedOrder.value?.id)
  if (fresh) selectedOrder.value = { ...selectedOrder.value, ...fresh }
}

async function handleCancelOrder(order: PaymentOrder) {
  try { await adminPaymentAPI.cancelOrder(order.id); appStore.showSuccess(t('payment.admin.orderCancelled')); refreshAfterAction() }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
}

async function handleRetryOrder(order: PaymentOrder) {
  try { await adminPaymentAPI.retryRecharge(order.id); appStore.showSuccess(t('payment.admin.retrySuccess')); refreshAfterAction() }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
}

function openRefundDialog(order: PaymentOrder) {
  if (selectedOrder.value?.id !== order.id) selectedOrder.value = order
  refundRequireForce.value = false
  refundWarning.value = ''
  showRefundDialog.value = true
}

function closeRefundDialog() {
  showRefundDialog.value = false
  refundRequireForce.value = false
  refundWarning.value = ''
}

function isRefundPendingWarning(warning: string | undefined): boolean {
  return /pending|处理中|待/.test(String(warning || '').toLowerCase())
}

async function handleRefund(data: { amount: number; reason: string; deduct_balance: boolean; force: boolean }) {
  if (!selectedOrder.value) return
  refundSubmitting.value = true
  try {
    const res = await adminPaymentAPI.refundOrder(selectedOrder.value.id, { amount: data.amount, reason: data.reason, deduct_balance: data.deduct_balance, force: data.force })
    if (res.data.success) {
      appStore.showSuccess(t('payment.admin.refundSuccess'))
      closeRefundDialog()
      refreshAfterAction()
      return
    }
    if (isRefundPendingWarning(res.data.warning)) {
      appStore.showSuccess(t('payment.admin.refundPending'))
      closeRefundDialog()
      refreshAfterAction()
      return
    }
    if (res.data.require_force) {
      // Backend needs an explicit force confirmation (e.g. the user spent their
      // balance after requesting the refund). Keep the dialog open and surface
      // the force checkbox instead of dropping the admin back to the list.
      refundRequireForce.value = true
      refundWarning.value = res.data.warning || ''
      return
    }
    appStore.showError(res.data.warning || t('common.error'))
  } catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
  finally { refundSubmitting.value = false }
}

async function handleQueryRefund(order: PaymentOrder) {
  refundQueryingIds.value = new Set(refundQueryingIds.value).add(order.id)
  try {
    const res = await adminPaymentAPI.queryRefund(order.id)
    if (res.data.success) {
      appStore.showSuccess(t('payment.admin.refundSuccess'))
    } else if (isRefundPendingWarning(res.data.warning)) {
      appStore.showSuccess(t('payment.admin.refundPending'))
    } else {
      appStore.showError(res.data.warning || t('common.error'))
    }
    refreshAfterAction()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    const next = new Set(refundQueryingIds.value)
    next.delete(order.id)
    refundQueryingIds.value = next
  }
}

onMounted(() => loadOrders())
</script>

<style scoped>
.orders-filter-field {
  display: grid;
  gap: 0.25rem;
}

.orders-status-seg {
  max-width: 100%;
  overflow-x: auto;
  scrollbar-width: none;
}
</style>
