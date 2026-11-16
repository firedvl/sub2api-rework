<template>
  <AppLayout>
    <div class="ui-page ui-stagger">
      <div class="ui-toolbar" role="toolbar" :aria-label="t('admin.affiliates.records.search')">
        <label class="ui-search">
          <Icon name="search" size="sm" />
          <input
            v-model="filters.search"
            type="search"
            :placeholder="t('admin.affiliates.records.searchPlaceholder')"
            :aria-label="t('admin.affiliates.records.search')"
            @input="debounceLoad"
          />
        </label>
        <ToolbarPopover :label="t('admin.adminUi.filters')" icon="calendar" :count="activeFilterCount" test-id="affiliate-filters">
          <label class="affiliate-filter-field">
            <span class="input-label">{{ t('admin.affiliates.records.startAt') }}</span>
            <input v-model="filters.start_at" type="date" class="input" data-test="affiliate-start-at" @change="reloadFromFirstPage" />
          </label>
          <label class="affiliate-filter-field">
            <span class="input-label">{{ t('admin.affiliates.records.endAt') }}</span>
            <input v-model="filters.end_at" type="date" class="input" data-test="affiliate-end-at" @change="reloadFromFirstPage" />
          </label>
          <button v-if="activeFilterCount" type="button" class="btn btn-ghost btn-sm self-start" @click="clearDates">
            {{ t('admin.adminUi.clearFilters') }}
          </button>
        </ToolbarPopover>
        <span class="ui-grow" />
        <button
          type="button"
          class="app-icon-button"
          :disabled="loading"
          :aria-label="t('common.refresh')"
          :title="t('common.refresh')"
          @click="loadRecords"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        </button>
        <button
          v-if="props.type === 'transfers'"
          type="button"
          class="btn btn-primary"
          data-test="affiliate-withdraw-open"
          @click="withdrawDialog = true"
        >{{ t('admin.affiliates.withdraw.button') }}</button>
      </div>

      <div class="ui-page">
        <DataTable
          :columns="columns"
          :data="records"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          :sort-storage-key="sortStorageKey"
          @sort="handleSort"
        >
          <template #cell-inviter="{ row }">
            <UserCell
              :id="row.inviter_id"
              :email="row.inviter_email"
              :username="row.inviter_username"
              :clickable="props.type !== 'transfers'"
              @open="openUserOverview"
            />
          </template>
          <template #cell-invitee="{ row }">
            <span v-if="row.invitee_id == null" class="text-sm text-gray-400 dark:text-dark-500">-</span>
            <UserCell
              v-else
              :id="row.invitee_id"
              :email="row.invitee_email"
              :username="row.invitee_username"
              :clickable="props.type !== 'transfers'"
              @open="openUserOverview"
            />
          </template>
          <template #cell-user="{ row }">
            <UserCell
              :id="row.user_id"
              :email="row.user_email"
              :username="row.username"
              :clickable="true"
              @open="openUserOverview"
            />
          </template>
          <template #cell-aff_code="{ row }">
            <span class="font-mono text-sm text-gray-700 dark:text-gray-300">{{ row.aff_code || '-' }}</span>
          </template>
          <template #cell-order="{ row }">
            <span v-if="row.order_id == null" class="text-sm text-gray-400 dark:text-dark-500">-</span>
            <div v-else class="max-w-56">
              <span class="ui-cell-main font-mono">#{{ row.order_id }}</span>
              <span class="ui-cell-sub" :title="row.out_trade_no">{{ row.out_trade_no }}</span>
            </div>
          </template>
          <template #cell-payment_type="{ row }">
            <PaymentMethodMark v-if="row.payment_type" :type="row.payment_type" class="text-sm" />
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
          </template>
          <template #cell-order_status="{ row }">
            <span v-if="row.order_status" :class="orderStatusBadgeClass(row.order_status)" class="whitespace-nowrap">
              {{ t('payment.status.' + String(row.order_status).toLowerCase(), row.order_status) }}
            </span>
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
          </template>
          <template #cell-total_rebate="{ row }">
            <AmountText :value="row.total_rebate" />
          </template>
          <template #cell-order_amount="{ row }">
            <NullableAmountText :value="row.order_amount" />
          </template>
          <template #cell-pay_amount="{ row }">
            <span v-if="row.pay_amount == null" class="text-sm text-gray-400 dark:text-dark-500">-</span>
            <span v-else class="text-sm tabular-nums text-gray-900 dark:text-white">¥{{ formatAmount(row.pay_amount) }}</span>
          </template>
          <template #cell-rebate_amount="{ row }">
            <AmountText :value="row.rebate_amount" strong />
          </template>
          <template #cell-action="{ row }">
            <span :class="row.action === 'withdraw' ? 'badge badge-warning whitespace-nowrap' : 'badge badge-primary whitespace-nowrap'">
              {{ outflowTypeLabel(row.action) }}
            </span>
          </template>
          <template #cell-amount="{ row }">
            <AmountText :value="row.amount" strong />
          </template>
          <template #cell-balance_after="{ row }">
            <NullableAmountText :value="row.balance_after" />
          </template>
          <template #cell-available_quota_after="{ row }">
            <NullableAmountText :value="row.available_quota_after" />
          </template>
          <template #cell-frozen_quota_after="{ row }">
            <NullableAmountText :value="row.frozen_quota_after" />
          </template>
          <template #cell-history_quota_after="{ row }">
            <NullableAmountText :value="row.history_quota_after" />
          </template>
          <template #cell-created_at="{ row }">
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatDateTime(row.created_at) }}</span>
          </template>
        </DataTable>

        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </div>
    </div>

    <AffiliateOfflineWithdrawDialog
      v-if="props.type === 'transfers'"
      :show="withdrawDialog"
      @close="withdrawDialog = false"
      @success="handleWithdrawSuccess"
    />

    <SideDrawer
      :open="overviewDialog"
      :title="selectedOverview?.email || t('admin.affiliates.overview.title')"
      :subtitle="selectedOverview ? overviewSubtitle(selectedOverview) : undefined"
      width="28rem"
      @close="overviewDialog = false"
    >
      <template #leading>
        <span class="affiliate-avatar"><Icon name="userCircle" size="md" /></span>
      </template>
      <div v-if="overviewLoading" class="grid gap-3" role="status" :aria-label="t('common.loading')">
        <div class="skeleton h-20 rounded-lg"></div>
        <div class="skeleton h-32 rounded-lg"></div>
      </div>
      <template v-else-if="selectedOverview">
        <dl class="affiliate-quota">
          <div>
            <dt>{{ t('admin.affiliates.overview.availableQuota') }}</dt>
            <dd><CountUp :value="selectedOverview.available_quota" :format="formatUsd" /></dd>
          </div>
          <div>
            <dt>{{ t('admin.affiliates.overview.historyQuota') }}</dt>
            <dd><CountUp :value="selectedOverview.history_quota" :format="formatUsd" /></dd>
          </div>
        </dl>
        <section>
          <h3 class="ui-section-label">{{ t('admin.adminUi.details') }}</h3>
          <dl class="ui-dl">
            <dt>{{ t('admin.affiliates.overview.affCode') }}</dt>
            <dd class="font-mono">{{ selectedOverview.aff_code || '-' }}</dd>
            <dt>{{ t('admin.affiliates.overview.rebateRate') }}</dt>
            <dd class="tabular-nums">{{ formatPercent(selectedOverview.rebate_rate_percent) }}</dd>
            <dt>{{ t('admin.affiliates.overview.invitedCount') }}</dt>
            <dd class="tabular-nums">{{ selectedOverview.invited_count }}</dd>
            <dt>{{ t('admin.affiliates.overview.rebatedInviteeCount') }}</dt>
            <dd class="tabular-nums">{{ selectedOverview.rebated_invitee_count }}</dd>
          </dl>
        </section>
      </template>
    </SideDrawer>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref, type PropType } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ToolbarPopover from '@/components/common/ToolbarPopover.vue'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import CountUp from '@/components/ui/CountUp.vue'
import Icon from '@/components/icons/Icon.vue'
import PaymentMethodMark from '@/components/admin/payment/PaymentMethodMark.vue'
import { orderStatusBadgeClass } from '@/components/admin/payment/orderDisplay'
import AffiliateOfflineWithdrawDialog from './AffiliateOfflineWithdrawDialog.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { affiliatesAPI, type AffiliateInviteRecord, type AffiliateRebateRecord, type AffiliateTransferRecord, type AffiliateUserOverview, type ListAffiliateRecordsParams } from '@/api/admin/affiliates'
import type { PaginatedResponse } from '@/types'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTime as formatDisplayDateTime } from '@/utils/format'

type RecordType = 'invites' | 'rebates' | 'transfers'
type AffiliateRecord = AffiliateInviteRecord | AffiliateRebateRecord | AffiliateTransferRecord

const props = defineProps<{
  type: RecordType
}>()

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const records = ref<AffiliateRecord[]>([])
const filters = reactive({ search: '', start_at: '', end_at: '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const overviewDialog = ref(false)
const overviewLoading = ref(false)
const selectedOverview = ref<AffiliateUserOverview | null>(null)
const withdrawDialog = ref(false)
let debounceTimer: ReturnType<typeof setTimeout> | null = null

const activeFilterCount = computed(() => Number(!!filters.start_at) + Number(!!filters.end_at))

function clearDates() {
  filters.start_at = ''
  filters.end_at = ''
  reloadFromFirstPage()
}

const columns = computed<Column[]>(() => {
  if (props.type === 'invites') {
    return [
      { key: 'inviter', label: t('admin.affiliates.records.inviter'), sortable: true },
      { key: 'invitee', label: t('admin.affiliates.records.invitee'), sortable: true },
      { key: 'aff_code', label: t('admin.affiliates.records.affCode'), sortable: true },
      { key: 'total_rebate', label: t('admin.affiliates.records.totalRebate'), sortable: true },
      { key: 'created_at', label: t('admin.affiliates.records.invitedAt'), sortable: true },
    ]
  }
  if (props.type === 'rebates') {
    return [
      { key: 'order', label: t('admin.affiliates.records.order'), sortable: true },
      { key: 'inviter', label: t('admin.affiliates.records.inviter'), sortable: true },
      { key: 'invitee', label: t('admin.affiliates.records.invitee'), sortable: true },
      { key: 'order_amount', label: t('admin.affiliates.records.orderAmount'), sortable: true },
      { key: 'pay_amount', label: t('admin.affiliates.records.payAmount'), sortable: true },
      { key: 'rebate_amount', label: t('admin.affiliates.records.rebateAmount') },
      { key: 'payment_type', label: t('admin.affiliates.records.paymentType'), sortable: true },
      { key: 'order_status', label: t('admin.affiliates.records.orderStatus'), sortable: true },
      { key: 'created_at', label: t('admin.affiliates.records.rebatedAt'), sortable: true },
    ]
  }
  return [
    { key: 'user', label: t('admin.affiliates.records.user'), sortable: true },
    { key: 'action', label: t('admin.affiliates.records.outflowType'), sortable: true },
    { key: 'amount', label: t('admin.affiliates.records.transferAmount'), sortable: true },
    { key: 'balance_after', label: t('admin.affiliates.records.balanceAfter'), sortable: true },
    { key: 'available_quota_after', label: t('admin.affiliates.records.availableQuotaAfter'), sortable: true },
    { key: 'frozen_quota_after', label: t('admin.affiliates.records.frozenQuotaAfter'), sortable: true },
    { key: 'history_quota_after', label: t('admin.affiliates.records.historyQuotaAfter'), sortable: true },
    { key: 'created_at', label: t('admin.affiliates.records.transferredAt'), sortable: true },
  ]
})

const sortStorageKey = computed(() => `admin-affiliate-${props.type}-table-sort`)

function loadInitialSortState(): { sort_by: string; sort_order: 'asc' | 'desc' } {
  const fallback = { sort_by: 'created_at', sort_order: 'desc' as 'asc' | 'desc' }
  try {
    const raw = localStorage.getItem(sortStorageKey.value)
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as { key?: string; order?: string }
    const key = typeof parsed.key === 'string' ? parsed.key : ''
    if (!columns.value.some((column) => column.key === key && column.sortable)) return fallback
    return {
      sort_by: key,
      sort_order: parsed.order === 'asc' ? 'asc' : 'desc',
    }
  } catch {
    return fallback
  }
}

const sortState = reactive(loadInitialSortState())

function userTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

function buildParams(): ListAffiliateRecordsParams {
  return {
    page: pagination.page,
    page_size: pagination.page_size,
    search: filters.search.trim() || undefined,
    start_at: filters.start_at || undefined,
    end_at: filters.end_at || undefined,
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order,
    timezone: userTimezone(),
  }
}

async function fetchRecords(params: ListAffiliateRecordsParams): Promise<PaginatedResponse<AffiliateRecord>> {
  if (props.type === 'invites') {
    return affiliatesAPI.listInviteRecords(params)
  }
  if (props.type === 'rebates') {
    return affiliatesAPI.listRebateRecords(params)
  }
  return affiliatesAPI.listTransferRecords(params)
}

async function loadRecords() {
  loading.value = true
  try {
    const res = await fetchRecords(buildParams())
    records.value = res.items || []
    pagination.total = res.total || 0
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'admin.affiliates.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function debounceLoad() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => reloadFromFirstPage(), 300)
}

function reloadFromFirstPage() {
  pagination.page = 1
  void loadRecords()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadRecords()
}

function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  void loadRecords()
}

function handleSort(key: string, order: 'asc' | 'desc') {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  void loadRecords()
}

function handleWithdrawSuccess() {
  withdrawDialog.value = false
  reloadFromFirstPage()
}

function outflowTypeLabel(action: string | null | undefined): string {
  return action === 'withdraw'
    ? t('admin.affiliates.outflowTypes.withdraw')
    : t('admin.affiliates.outflowTypes.transfer')
}

function formatAmount(value: number | null | undefined): string {
  return Number(value || 0).toFixed(2)
}

function formatUsd(value: number): string {
  return `$${formatAmount(value)}`
}

function overviewSubtitle(overview: AffiliateUserOverview): string {
  return overview.username ? `#${overview.user_id} · ${overview.username}` : `#${overview.user_id}`
}

function formatPercent(value: number | null | undefined): string {
  const rounded = Math.round(Number(value || 0) * 100) / 100
  return `${Number.isInteger(rounded) ? rounded.toString() : rounded.toString()}%`
}

function formatDateTime(value: string | null | undefined): string {
  return value ? formatDisplayDateTime(value) : '-'
}

async function openUserOverview(userId: number) {
  if (!userId) return
  overviewDialog.value = true
  overviewLoading.value = true
  selectedOverview.value = null
  try {
    selectedOverview.value = await affiliatesAPI.getUserOverview(userId)
  } catch (error) {
    overviewDialog.value = false
    appStore.showError(extractI18nErrorMessage(error, t, 'admin.affiliates.errors', t('common.error')))
  } finally {
    overviewLoading.value = false
  }
}

const UserCell = defineComponent({
  props: {
    id: { type: Number, required: true },
    email: { type: String, default: '' },
    username: { type: String, default: '' },
    clickable: { type: Boolean, default: false },
  },
  emits: ['open'],
  setup(cellProps, { emit }) {
    return () => h('div', { class: 'max-w-56 min-w-0' }, [
      h(cellProps.clickable ? 'button' : 'span', {
        class: cellProps.clickable
          ? 'ui-cell-main max-w-full text-left text-primary-600 hover:underline dark:text-primary-400'
          : 'ui-cell-main',
        type: cellProps.clickable ? 'button' : undefined,
        title: cellProps.email || undefined,
        onClick: cellProps.clickable ? () => emit('open', cellProps.id) : undefined,
      }, cellProps.email || '-'),
      h('span', { class: 'ui-cell-sub' }, cellProps.username ? `#${cellProps.id} · ${cellProps.username}` : `#${cellProps.id}`),
    ])
  },
})

const AmountText = defineComponent({
  props: {
    value: { type: Number, default: 0 },
    strong: { type: Boolean, default: false },
  },
  setup(amountProps) {
    return () => h('span', {
      class: amountProps.strong
        ? 'text-sm font-semibold tabular-nums text-emerald-600 dark:text-emerald-400'
        : 'text-sm tabular-nums text-gray-900 dark:text-white',
    }, `$${formatAmount(amountProps.value)}`)
  },
})

const NullableAmountText = defineComponent({
  props: {
    value: { type: Number as PropType<number | null | undefined>, default: null },
  },
  setup(amountProps) {
    return () => {
      const value = amountProps.value
      if (value === null || value === undefined) {
        return h('span', { class: 'text-sm text-gray-400 dark:text-dark-500' }, '-')
      }
      return h(AmountText, { value })
    }
  },
})

onMounted(() => {
  void loadRecords()
})
</script>

<style scoped>
.affiliate-filter-field {
  display: grid;
  gap: 0.25rem;
}

.affiliate-avatar {
  display: inline-grid;
  width: 2.25rem;
  height: 2.25rem;
  flex: none;
  place-items: center;
  border-radius: 999px;
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
}

.affiliate-quota {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

.affiliate-quota > div {
  display: grid;
  gap: 0.25rem;
  padding: 0.75rem 0.875rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-raised);
}

.affiliate-quota dt {
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
}

.affiliate-quota dd {
  color: var(--operator-foreground);
  font-size: 1.375rem;
  font-weight: 600;
  letter-spacing: -0.02em;
}
</style>
