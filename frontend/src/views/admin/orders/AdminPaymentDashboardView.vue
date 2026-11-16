<template>
  <AppLayout>
    <div class="ui-page ui-stagger">
      <div class="ui-toolbar" role="toolbar" :aria-label="t('payment.admin.dashboardTitle')">
        <div class="viz-seg" role="group" :aria-label="t('admin.adminUi.orders.range')">
          <button
            v-for="d in DAYS_OPTIONS"
            :key="d"
            type="button"
            :aria-pressed="days === d"
            @click="days = d"
          >
            {{ d }}{{ t('payment.admin.daySuffix') }}
          </button>
        </div>
        <span class="ui-grow" />
        <button
          type="button"
          class="app-icon-button"
          :disabled="loading"
          :aria-label="t('common.refresh')"
          :title="t('common.refresh')"
          @click="loadDashboard"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <template v-if="stats">
        <OrderStatsCards :stats="stats" />
        <div class="viz-grid">
          <DailyRevenueChart class="viz-span-8" :data="stats.daily_series || []" :loading="loading" />
          <PaymentMethodChart class="viz-span-4" :methods="stats.payment_methods || []" />
          <TopUsersLeaderboard class="viz-span-12" :users="stats.top_users || {}" />
        </div>
      </template>
      <template v-else-if="loading">
        <div class="viz-kpis mb-0" aria-hidden="true">
          <div v-for="i in 4" :key="i" class="skeleton h-[5.25rem] rounded-xl"></div>
        </div>
        <div class="viz-grid" role="status" :aria-label="t('common.loading')">
          <div class="skeleton viz-span-8 h-80 rounded-xl"></div>
          <div class="skeleton viz-span-4 h-80 rounded-xl"></div>
        </div>
      </template>
      <div v-else class="ui-card ui-empty">
        <span class="ui-empty-icon"><Icon name="chartBar" size="md" /></span>
        <b>{{ t('payment.admin.noData') }}</b>
        <button type="button" class="btn btn-secondary btn-sm" @click="loadDashboard">{{ t('common.refresh') }}</button>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { DashboardStats } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import OrderStatsCards from '@/components/admin/payment/OrderStatsCards.vue'
import DailyRevenueChart from '@/components/admin/payment/DailyRevenueChart.vue'
import PaymentMethodChart from '@/components/admin/payment/PaymentMethodChart.vue'
import TopUsersLeaderboard from '@/components/admin/payment/TopUsersLeaderboard.vue'

const { t } = useI18n()
const appStore = useAppStore()

const DAYS_OPTIONS = [7, 30, 90] as const
const days = ref<number>(30)
const loading = ref(false)
const stats = ref<DashboardStats | null>(null)

async function loadDashboard() {
  loading.value = true
  try {
    const res = await adminPaymentAPI.getDashboard(days.value)
    stats.value = res.data
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

watch(days, () => loadDashboard())
onMounted(() => loadDashboard())
</script>
