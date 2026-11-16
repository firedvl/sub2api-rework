<template>
  <AppLayout>
    <div class="ui-page ui-stagger mx-auto w-full max-w-2xl">
      <!-- Redeem -->
      <section class="ui-card">
        <form class="flex flex-col gap-3 p-5" @submit.prevent="handleRedeem">
          <label for="code" class="sr-only">{{ t('redeem.redeemCodeLabel') }}</label>
          <div class="flex flex-col gap-2 sm:flex-row">
            <div class="relative min-w-0 flex-1">
              <Icon
                name="gift"
                size="md"
                class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400"
              />
              <input
                id="code"
                v-model="redeemCode"
                type="text"
                required
                autocomplete="off"
                spellcheck="false"
                :placeholder="t('redeem.redeemCodePlaceholder')"
                :title="t('redeem.redeemCodeHint')"
                :disabled="submitting"
                class="input h-11 pl-11 font-mono text-base"
              />
            </div>
            <button
              type="submit"
              :disabled="!redeemCode || submitting"
              class="btn btn-primary h-11 shrink-0 px-5"
            >
              <Icon v-if="submitting" name="refresh" size="sm" class="animate-spin" />
              <Icon v-else name="checkCircle" size="sm" />
              {{ submitting ? t('redeem.redeeming') : t('redeem.redeemButton') }}
            </button>
          </div>

          <!-- Result -->
          <Transition name="fade" mode="out-in">
            <div
              v-if="redeemResult"
              key="ok"
              class="redeem-result flex items-start gap-3 rounded-lg border border-emerald-200 bg-emerald-50 p-3 dark:border-emerald-800/50 dark:bg-emerald-900/20"
              role="status"
            >
              <span class="redeem-check grid h-8 w-8 shrink-0 place-items-center rounded-full bg-emerald-500 text-white">
                <Icon name="check" size="sm" :stroke-width="2.5" />
              </span>
              <div class="min-w-0 flex-1 text-sm">
                <p class="font-semibold text-gray-900 dark:text-white">{{ t('redeem.redeemSuccess') }}</p>
                <p class="text-gray-600 dark:text-gray-300">{{ redeemResult.message }}</p>
                <dl class="mt-2 flex flex-wrap gap-x-5 gap-y-1 text-xs text-gray-500">
                  <div v-if="redeemResult.type === 'balance'">
                    <dt class="inline">{{ t('redeem.added') }}</dt>
                    <dd class="ml-1 inline font-semibold text-emerald-600">+${{ redeemResult.value.toFixed(2) }}</dd>
                  </div>
                  <div v-else-if="redeemResult.type === 'concurrency'">
                    <dt class="inline">{{ t('redeem.added') }}</dt>
                    <dd class="ml-1 inline font-semibold text-emerald-600">
                      +{{ redeemResult.value }} {{ t('redeem.concurrentRequests') }}
                    </dd>
                  </div>
                  <div v-else-if="redeemResult.type === 'subscription'">
                    <dt class="inline">{{ t('redeem.subscriptionAssigned') }}</dt>
                    <dd class="ml-1 inline font-semibold text-gray-900 dark:text-white">
                      <span v-if="redeemResult.group_name">{{ redeemResult.group_name }}</span>
                      <span v-if="redeemResult.validity_days">
                        ({{ t('redeem.subscriptionDays', { days: redeemResult.validity_days }) }})</span>
                    </dd>
                  </div>
                  <div v-if="redeemResult.new_balance !== undefined">
                    <dt class="inline">{{ t('redeem.newBalance') }}</dt>
                    <dd class="ml-1 inline font-semibold text-gray-900 dark:text-white">${{ redeemResult.new_balance.toFixed(2) }}</dd>
                  </div>
                  <div v-if="redeemResult.new_concurrency !== undefined">
                    <dt class="inline">{{ t('redeem.newConcurrency') }}</dt>
                    <dd class="ml-1 inline font-semibold text-gray-900 dark:text-white">
                      {{ redeemResult.new_concurrency }} {{ t('redeem.requests') }}
                    </dd>
                  </div>
                </dl>
              </div>
            </div>
            <div
              v-else-if="errorMessage"
              key="err"
              class="flex items-start gap-3 rounded-lg border border-red-200 bg-red-50 p-3 dark:border-red-800/50 dark:bg-red-900/20"
              role="alert"
            >
              <Icon name="exclamationCircle" size="md" class="shrink-0 text-red-600" />
              <div class="min-w-0 text-sm">
                <p class="font-semibold text-gray-900 dark:text-white">{{ t('redeem.redeemFailed') }}</p>
                <p class="text-red-600">{{ errorMessage }}</p>
              </div>
            </div>
          </Transition>
        </form>

        <footer class="viz-card-foot items-center">
          <span>{{ t('redeem.currentBalance') }} <b>${{ user?.balance?.toFixed(2) || '0.00' }}</b></span>
          <span>{{ t('redeem.concurrency') }} <b>{{ user?.concurrency || 0 }}</b></span>
          <details class="redeem-rules ml-auto">
            <summary
              class="inline-flex cursor-pointer list-none items-center gap-1 rounded-md text-gray-500 hover:text-gray-900 dark:hover:text-white"
            >
              <Icon name="infoCircle" size="sm" />
              {{ t('redeem.aboutCodes') }}
            </summary>
            <ul class="mt-2 list-disc space-y-1 pl-4 text-xs text-gray-500">
              <li>{{ t('redeem.codeRule1') }}</li>
              <li>{{ t('redeem.codeRule2') }}</li>
              <li>
                {{ t('redeem.codeRule3') }}
                <span v-if="contactInfo" class="badge badge-primary ml-1">{{ contactInfo }}</span>
              </li>
              <li>{{ t('redeem.codeRule4') }}</li>
              <li>{{ t('redeem.redeemCodeHint') }}</li>
            </ul>
          </details>
        </footer>
      </section>

      <!-- History -->
      <section class="ui-card">
        <div class="viz-card-head">
          <h2 class="viz-card-title">{{ t('redeem.recentActivity') }}</h2>
          <span v-if="historyTotal > 0" class="badge badge-gray">{{ historyTotal }}</span>
        </div>
        <div class="viz-card-body">
          <div v-if="loadingHistory && history.length === 0" class="flex flex-col gap-2" aria-busy="true">
            <div v-for="i in 3" :key="i" class="skeleton h-12 rounded-lg"></div>
          </div>

          <ul
            v-else-if="history.length > 0"
            class="ui-rows-enter flex flex-col"
            :class="{ 'opacity-60': loadingHistory }"
          >
            <li
              v-for="item in history"
              :key="item.id"
              class="flex min-w-0 items-center gap-3 border-b border-gray-100 py-3 last:border-b-0 dark:border-dark-700"
            >
              <span class="grid h-9 w-9 shrink-0 place-items-center rounded-lg" :class="historyToneClass(item)">
                <Icon :name="historyIcon(item)" size="sm" />
              </span>
              <div class="min-w-0 flex-1">
                <p class="ui-cell-main text-sm">{{ getHistoryItemTitle(item) }}</p>
                <p class="ui-cell-sub">
                  {{ formatDateTime(item.used_at) }}
                  <span v-if="!isAdminAdjustment(item.type)" class="ml-1.5 font-mono">{{ item.code.slice(0, 8) }}...</span>
                  <span v-else class="ml-1.5">{{ t('redeem.adminAdjustment') }}</span>
                </p>
                <p v-if="item.notes" class="ui-cell-sub italic" :title="item.notes">{{ item.notes }}</p>
              </div>
              <span class="max-w-[45%] shrink-0 truncate text-right text-sm font-semibold tabular-nums" :class="historyValueClass(item)">
                {{ formatHistoryValue(item) }}
              </span>
            </li>
          </ul>

          <div v-else class="ui-empty py-8">
            <span class="ui-empty-icon"><Icon name="clock" size="lg" /></span>
            <span>{{ t('redeem.historyWillAppear') }}</span>
          </div>
        </div>
        <fieldset
          :disabled="loadingHistory || submitting"
          class="flex flex-wrap items-center gap-3 border-t border-gray-100 px-4 py-3 dark:border-dark-700"
        >
          <label class="flex items-center gap-2 text-xs text-gray-500">
            {{ t('pagination.perPage') }}
            <select v-model="historyPageSize" class="input h-8 w-20 py-0 text-sm" @change="fetchHistory(1)">
              <option v-for="size in [20, 50, 100]" :key="size" :value="size">{{ size }}</option>
            </select>
          </label>
          <Pagination
            v-if="historyTotal > 0"
            class="min-w-0 flex-1"
            :page="historyPage"
            :total="historyTotal"
            :page-size="historyPageSize"
            :show-page-size-selector="false"
            :show-jump="true"
            @update:page="fetchHistory"
          />
        </fieldset>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { redeemAPI, authAPI, type RedeemHistoryItem } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const subscriptionStore = useSubscriptionStore()

const user = computed(() => authStore.user)

const redeemCode = ref('')
const submitting = ref(false)
const redeemResult = ref<{
  message: string
  type: string
  value: number
  new_balance?: number
  new_concurrency?: number
  group_name?: string
  validity_days?: number
} | null>(null)
const errorMessage = ref('')

// History data
const history = ref<RedeemHistoryItem[]>([])
const loadingHistory = ref(false)
const historyPage = ref(1)
const historyPageSize = ref(20)
const historyTotal = ref(0)
let historyRequest = 0
let loadedHistoryPageSize = 20
const contactInfo = ref('')

// Helper functions for history display
const isBalanceType = (type: string) => {
  return type === 'balance' || type === 'admin_balance'
}

const isSubscriptionType = (type: string) => {
  return type === 'subscription'
}

const isAdminAdjustment = (type: string) => {
  return type === 'admin_balance' || type === 'admin_concurrency'
}

const getHistoryItemTitle = (item: RedeemHistoryItem) => {
  if (item.type === 'balance') {
    return t('redeem.balanceAddedRedeem')
  } else if (item.type === 'admin_balance') {
    return item.value >= 0 ? t('redeem.balanceAddedAdmin') : t('redeem.balanceDeductedAdmin')
  } else if (item.type === 'concurrency') {
    return t('redeem.concurrencyAddedRedeem')
  } else if (item.type === 'admin_concurrency') {
    return item.value >= 0 ? t('redeem.concurrencyAddedAdmin') : t('redeem.concurrencyReducedAdmin')
  } else if (item.type === 'subscription') {
    return t('redeem.subscriptionAssigned')
  }
  return t('common.unknown')
}

const historyIcon = (item: RedeemHistoryItem): 'dollar' | 'badge' | 'bolt' => {
  if (isBalanceType(item.type)) return 'dollar'
  if (isSubscriptionType(item.type)) return 'badge'
  return 'bolt'
}

const historyToneClass = (item: RedeemHistoryItem) => {
  if (isSubscriptionType(item.type)) return 'bg-primary-50 text-primary-600 dark:bg-primary-900/20 dark:text-primary-300'
  if (item.value < 0) return 'bg-red-50 text-red-600 dark:bg-red-900/20'
  if (isBalanceType(item.type)) return 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/20'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}

const historyValueClass = (item: RedeemHistoryItem) => {
  if (isSubscriptionType(item.type)) return 'text-primary-600 dark:text-primary-300'
  if (item.value < 0) return 'text-red-600'
  if (isBalanceType(item.type)) return 'text-emerald-600'
  return 'text-gray-900 dark:text-white'
}

const formatHistoryValue = (item: RedeemHistoryItem) => {
  if (isBalanceType(item.type)) {
    const sign = item.value >= 0 ? '+' : ''
    return `${sign}$${item.value.toFixed(2)}`
  } else if (isSubscriptionType(item.type)) {
    // 订阅类型显示有效天数和分组名称
    const days = item.validity_days || Math.round(item.value)
    const groupName = item.group?.name || ''
    return groupName ? `${days}${t('redeem.days')} - ${groupName}` : `${days}${t('redeem.days')}`
  } else {
    const sign = item.value >= 0 ? '+' : ''
    return `${sign}${item.value} ${t('redeem.requests')}`
  }
}

const fetchHistory = async (page = 1) => {
  const request = ++historyRequest
  const pageSize = historyPageSize.value
  loadingHistory.value = true
  try {
    const result = await redeemAPI.getHistory(page, pageSize)
    if (request !== historyRequest) return
    history.value = result.items
    historyTotal.value = result.total
    historyPage.value = page
    historyPageSize.value = pageSize
    loadedHistoryPageSize = pageSize
  } catch (error) {
    if (request !== historyRequest) return
    historyPageSize.value = loadedHistoryPageSize
    appStore.showError(t('redeem.historyLoadFailed'))
    console.error('Failed to fetch history:', error)
  } finally {
    if (request === historyRequest) loadingHistory.value = false
  }
}

const handleRedeem = async () => {
  if (!redeemCode.value.trim()) {
    appStore.showError(t('redeem.pleaseEnterCode'))
    return
  }

  submitting.value = true
  errorMessage.value = ''
  redeemResult.value = null

  try {
    const result = await redeemAPI.redeem(redeemCode.value.trim())

    redeemResult.value = result

    // Refresh user data to get updated balance/concurrency
    try {
      await authStore.refreshUser()
    } catch (error) {
      console.error('Failed to refresh user after redeem:', error)
      appStore.showWarning(t('redeem.userRefreshFailed'))
    }

    // If subscription type, immediately refresh subscription status
    if (result.type === 'subscription') {
      try {
        await subscriptionStore.fetchActiveSubscriptions(true) // force refresh
      } catch (error) {
        console.error('Failed to refresh subscriptions after redeem:', error)
        appStore.showWarning(t('redeem.subscriptionRefreshFailed'))
      }
    }

    // Clear the input
    redeemCode.value = ''

    // Refresh history
    await fetchHistory()

    // Show success toast
    appStore.showSuccess(t('redeem.codeRedeemSuccess'))
  } catch (error: any) {
    errorMessage.value = error.response?.data?.detail || t('redeem.failedToRedeem')

    appStore.showError(t('redeem.redeemFailed'))
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  fetchHistory()
  try {
    const settings = await authAPI.getPublicSettings()
    contactInfo.value = settings.contact_info || ''
  } catch (error) {
    console.error('Failed to load contact info:', error)
  }
})
</script>

<style scoped>
.redeem-check {
  animation: ui-pop 420ms cubic-bezier(0.34, 1.56, 0.64, 1) both;
}

.redeem-rules summary::-webkit-details-marker {
  display: none;
}

.redeem-rules summary:focus-visible {
  outline: 2px solid rgb(var(--operator-accent-rgb) / 0.6);
  outline-offset: 2px;
}

.redeem-rules[open] {
  flex-basis: 100%;
  margin-left: 0;
}

@media (prefers-reduced-motion: reduce) {
  .redeem-check {
    animation: none;
  }
}
</style>
