<template>
  <AppLayout>
    <div class="ui-page ui-stagger">
      <div class="ui-toolbar" role="toolbar" :aria-label="t('payment.admin.plansPageTitle')">
        <label class="ui-search">
          <Icon name="search" size="sm" />
          <input v-model="planSearch" type="search" :placeholder="t('common.search')" :aria-label="t('common.search')" />
        </label>
        <div class="viz-seg" role="group" :aria-label="t('payment.admin.forSale')">
          <button
            v-for="option in saleOptions"
            :key="option.value"
            type="button"
            :aria-pressed="saleFilter === option.value"
            :data-testid="`plans-sale-${option.value}`"
            @click="saleFilter = option.value"
          >
            {{ option.label }}
            <span class="plans-seg-count">{{ option.count }}</span>
          </button>
        </div>
        <span class="ui-grow" />
        <button
          type="button"
          class="app-icon-button"
          :disabled="plansLoading"
          :aria-label="t('common.refresh')"
          :title="t('common.refresh')"
          @click="loadPlans"
        >
          <Icon name="refresh" size="sm" :class="plansLoading ? 'animate-spin' : ''" />
        </button>
        <button type="button" class="btn btn-primary" data-testid="plans-create" @click="openPlanEdit(null)">
          <Icon name="plus" size="sm" />
          {{ t('admin.adminUi.orders.newPlan') }}
        </button>
      </div>

      <div v-if="plansLoading && !plans.length" class="plans-grid" aria-hidden="true">
        <div v-for="i in 3" :key="i" class="skeleton h-48 rounded-xl"></div>
      </div>

      <TransitionGroup v-else-if="visiblePlans.length" name="ui-list" tag="div" class="plans-grid" data-testid="plans-grid">
        <article
          v-for="plan in visiblePlans"
          :key="plan.id"
          class="ui-card plan-card"
          :class="plan.for_sale ? '' : 'is-off'"
          :data-testid="`plan-card-${plan.id}`"
        >
          <header class="plan-card-head">
            <ProviderMark v-if="getGroup(plan.group_id)?.platform" :platform="getGroup(plan.group_id)!.platform" :size="32" />
            <span v-else class="plan-card-mark"><Icon name="ticket" size="sm" /></span>
            <div class="min-w-0 flex-1">
              <h2 class="ui-cell-main" :class="getPlanNameClass(plan.group_id)" :title="plan.name">{{ plan.name }}</h2>
              <span class="ui-cell-sub">#{{ plan.id }} · {{ t('payment.admin.sortOrder') }} {{ plan.sort_order }}</span>
            </div>
            <span :class="plan.for_sale ? 'badge badge-success' : 'badge badge-gray'">
              {{ plan.for_sale ? t('payment.admin.onSale') : t('payment.admin.offSale') }}
            </span>
          </header>

          <div class="plan-card-price">
            <span class="plan-card-amount">{{ planCurrencySymbol(plan.currency) }}{{ (plan.price ?? 0).toFixed(2) }}</span>
            <span v-if="plan.currency" class="plan-card-currency">{{ plan.currency }}</span>
            <span v-if="plan.original_price" class="plan-card-original">{{ planCurrencySymbol(plan.currency) }}{{ plan.original_price.toFixed(2) }}</span>
            <span class="plan-card-validity">/ {{ plan.validity_days }} {{ t('payment.admin.' + (plan.validity_unit || 'days')) }}</span>
          </div>

          <p v-if="plan.description" class="plan-card-desc" :title="plan.description">{{ plan.description }}</p>

          <div class="plan-card-group">
            <span v-if="isGroupMissing(plan.group_id)" class="text-sm">
              <span class="text-gray-400">#{{ plan.group_id }}</span>
              <span class="ml-1 badge badge-danger">{{ t('payment.admin.groupMissing') }}</span>
            </span>
            <GroupBadge
              v-else-if="getGroup(plan.group_id)"
              :name="getGroup(plan.group_id)!.name"
              :platform="getGroup(plan.group_id)!.platform"
              :rate-multiplier="getGroup(plan.group_id)!.rate_multiplier"
            />
            <span v-else class="text-sm text-gray-400">-</span>
          </div>

          <footer class="plan-card-foot">
            <label class="plan-card-sale">
              <Toggle
                :model-value="plan.for_sale"
                :aria-label="t('payment.admin.forSale')"
                :data-testid="`plan-sale-${plan.id}`"
                @update:model-value="toggleForSale(plan)"
              />
              <span>{{ t('payment.admin.forSale') }}</span>
            </label>
            <span class="ui-grow" />
            <button type="button" class="app-icon-button" :aria-label="t('common.edit')" :title="t('common.edit')" @click="openPlanEdit(plan)">
              <Icon name="edit" size="sm" />
            </button>
            <button
              type="button"
              class="app-icon-button text-red-600 dark:text-red-400"
              :aria-label="t('common.delete')"
              :title="t('common.delete')"
              @click="confirmDeletePlan(plan)"
            >
              <Icon name="trash" size="sm" />
            </button>
          </footer>
        </article>
      </TransitionGroup>

      <div v-else class="ui-card ui-empty">
        <span class="ui-empty-icon"><Icon name="ticket" size="md" /></span>
        <b>{{ plans.length ? t('admin.adminUi.noMatches') : t('payment.admin.noData') }}</b>
        <button v-if="plans.length" type="button" class="btn btn-secondary btn-sm" @click="clearFilters">
          {{ t('admin.adminUi.clearFilters') }}
        </button>
        <button v-else type="button" class="btn btn-primary btn-sm" @click="openPlanEdit(null)">
          <Icon name="plus" size="sm" />
          {{ t('admin.adminUi.orders.newPlan') }}
        </button>
      </div>
    </div>

    <!-- Plan Edit Dialog -->
    <PlanEditDialog :show="showPlanDialog" :plan="editingPlan" :groups="groups" :payment-config="paymentConfig" @close="showPlanDialog = false" @saved="loadPlans" />

    <ConfirmDialog :show="showDeletePlanDialog" :title="t('payment.admin.deletePlan')" :message="t('payment.admin.deletePlanConfirm')" :confirm-text="t('common.delete')" danger @confirm="handleDeletePlan" @cancel="showDeletePlanDialog = false" />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { AdminPaymentConfig } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import adminAPI from '@/api/admin'
import type { SubscriptionPlan } from '@/types/payment'
import type { AdminGroup } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import PlanEditDialog from './PlanEditDialog.vue'
import { currencySymbol } from '@/components/payment/currency'
import { platformTextClass } from '@/utils/platformColors'

const { t } = useI18n()
const appStore = useAppStore()

function planCurrencySymbol(currency?: string): string {
  return currencySymbol(currency || 'USD')
}

// ==================== Groups ====================

const groups = ref<AdminGroup[]>([])
const paymentConfig = ref<AdminPaymentConfig | null>(null)

async function loadGroups() {
  try {
    groups.value = await adminAPI.groups.getAll()
  } catch { /* ignore */ }
}

async function loadPaymentConfig() {
  try {
    const res = await adminPaymentAPI.getConfig()
    paymentConfig.value = res.data
  } catch { /* preview only */ }
}

function getGroup(id: number): AdminGroup | undefined {
  return groups.value.find(g => g.id === id)
}

function isGroupMissing(id: number): boolean {
  return id > 0 && !groups.value.find(g => g.id === id)
}

function getPlanNameClass(groupId: number): string {
  const group = getGroup(groupId)
  return group ? platformTextClass(group.platform) : ''
}


// ==================== Plans ====================

const plansLoading = ref(false)
const plans = ref<SubscriptionPlan[]>([])
const showPlanDialog = ref(false)
const showDeletePlanDialog = ref(false)
const editingPlan = ref<SubscriptionPlan | null>(null)
const deletingPlanId = ref<number | null>(null)
const planSearch = ref('')
const saleFilter = ref<'all' | 'on' | 'off'>('all')

const saleOptions = computed(() => [
  { value: 'all' as const, label: t('common.all'), count: plans.value.length },
  { value: 'on' as const, label: t('payment.admin.onSale'), count: plans.value.filter(p => p.for_sale).length },
  { value: 'off' as const, label: t('payment.admin.offSale'), count: plans.value.filter(p => !p.for_sale).length },
])

const visiblePlans = computed(() => {
  const query = planSearch.value.trim().toLowerCase()
  return plans.value.filter((plan) => {
    if (saleFilter.value === 'on' && !plan.for_sale) return false
    if (saleFilter.value === 'off' && plan.for_sale) return false
    if (!query) return true
    const group = getGroup(plan.group_id)
    return [plan.name, plan.description, group?.name, String(plan.id)].some(value => value?.toLowerCase().includes(query))
  })
})

function clearFilters() {
  planSearch.value = ''
  saleFilter.value = 'all'
}

async function loadPlans() {
  plansLoading.value = true
  try {
    const res = await adminPaymentAPI.getPlans()
    // Backend returns features as newline-separated string; parse to array
    plans.value = (res.data || []).map((p: Omit<SubscriptionPlan, 'features'> & { features: string | string[] }) => ({
      ...p,
      features: typeof p.features === 'string'
        ? p.features.split('\n').map((f: string) => f.trim()).filter(Boolean)
        : (p.features || []),
    }))
  }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
  finally { plansLoading.value = false }
}

function openPlanEdit(plan: SubscriptionPlan | null) {
  editingPlan.value = plan
  showPlanDialog.value = true
}


/** Quick toggle for_sale from the list */
async function toggleForSale(plan: SubscriptionPlan) {
  try {
    await adminPaymentAPI.updatePlan(plan.id, { for_sale: !plan.for_sale })
    plan.for_sale = !plan.for_sale
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

function confirmDeletePlan(plan: SubscriptionPlan) { deletingPlanId.value = plan.id; showDeletePlanDialog.value = true }
async function handleDeletePlan() {
  if (!deletingPlanId.value) return
  try { await adminPaymentAPI.deletePlan(deletingPlanId.value); appStore.showSuccess(t('common.deleted')); showDeletePlanDialog.value = false; loadPlans() }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
}

// ==================== Lifecycle ====================

onMounted(() => {
  loadGroups()
  loadPaymentConfig()
  loadPlans()
})
</script>

<style scoped>
.plans-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 19rem), 1fr));
  gap: 1rem;
}

.plans-seg-count {
  color: var(--operator-subtle-foreground);
  font-variant-numeric: tabular-nums;
}

.plan-card {
  gap: 0.875rem;
  padding: 1rem 1.125rem;
}

.plan-card.is-off .plan-card-head,
.plan-card.is-off .plan-card-price,
.plan-card.is-off .plan-card-desc {
  opacity: 0.6;
}

.plan-card-head {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.75rem;
}

.plan-card-head h2 {
  font-size: 0.9375rem;
  font-weight: 600;
}

.plan-card-mark {
  display: inline-grid;
  width: 2rem;
  height: 2rem;
  flex: none;
  place-items: center;
  border-radius: 0.5rem;
  background: var(--operator-raised);
  color: var(--operator-subtle-foreground);
}

.plan-card-price {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.25rem 0.5rem;
}

.plan-card-amount {
  color: var(--operator-foreground);
  font-size: 1.625rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.025em;
}

.plan-card-currency,
.plan-card-validity {
  color: var(--operator-subtle-foreground);
  font-size: 0.8125rem;
}

.plan-card-original {
  color: var(--operator-subtle-foreground);
  font-size: 0.8125rem;
  text-decoration: line-through;
}

.plan-card-desc {
  display: -webkit-box;
  overflow: hidden;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.plan-card-foot {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  margin: auto -0.375rem -0.25rem 0;
  padding-top: 0.75rem;
  border-top: 1px solid var(--operator-border);
}

.plan-card-sale {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.plan-card-foot > .ui-grow {
  flex: 1 1 auto;
}
</style>
