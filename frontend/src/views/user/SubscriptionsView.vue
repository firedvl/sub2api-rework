<template>
  <AppLayout>
    <div class="ui-page ui-stagger">
      <!-- Loading -->
      <div v-if="loading" class="grid gap-4 lg:grid-cols-2" aria-busy="true">
        <div v-for="i in 2" :key="i" class="ui-card gap-3 p-5">
          <div class="skeleton h-6 w-1/2 rounded-md"></div>
          <div class="skeleton h-3 w-full rounded-full"></div>
          <div class="skeleton h-3 w-4/5 rounded-full"></div>
        </div>
      </div>

      <!-- Empty -->
      <div v-else-if="subscriptions.length === 0" class="ui-card">
        <div class="ui-empty">
          <span class="ui-empty-icon"><Icon name="creditCard" size="lg" /></span>
          <b>{{ t('userSubscriptions.noActiveSubscriptions') }}</b>
          <span class="max-w-sm text-xs">{{ t('userSubscriptions.noActiveSubscriptionsDesc') }}</span>
        </div>
      </div>

      <!-- Subscriptions -->
      <div v-else class="grid gap-4 lg:grid-cols-2">
        <article
          v-for="subscription in subscriptions"
          :key="subscription.id"
          class="ui-card"
          :data-testid="`subscription-card-${subscription.id}`"
        >
          <header class="flex min-w-0 items-start gap-3 px-5 pt-4">
            <ProviderMark
              :platform="subscription.group?.platform || ''"
              :size="36"
              :label="platformLabel(subscription.group?.platform || '')"
            />
            <div class="min-w-0 flex-1">
              <h3 class="ui-cell-main text-[0.9375rem]">
                {{ subscription.group?.name || `Group #${subscription.group_id}` }}
              </h3>
              <div class="mt-0.5 flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-xs text-gray-500">
                <span>{{ platformLabel(subscription.group?.platform || '') }}</span>
                <span :title="t('payment.planCard.rate')">×{{ subscription.group?.rate_multiplier ?? 1 }}</span>
                <span
                  v-if="subscriptionHasPeakRate(subscription)"
                  class="inline-flex items-center gap-1 text-amber-600"
                  :title="t('payment.planCard.peakRate')"
                >
                  <Icon name="clock" size="xs" />{{ subscriptionPeakRateLabel(subscription) }}
                </span>
              </div>
              <p v-if="subscription.group?.description" class="ui-cell-sub mt-1" :title="subscription.group.description">
                {{ subscription.group.description }}
              </p>
            </div>
            <span
              class="badge shrink-0"
              :class="statusBadgeClass(subscription.status)"
            >
              {{ t(`userSubscriptions.status.${subscription.status}`) }}
            </span>
          </header>

          <div class="flex flex-col gap-4 px-5 py-4">
            <div v-for="row in rowsById[subscription.id]" :key="row.key" class="min-w-0">
              <div class="mb-1.5 flex min-w-0 items-baseline justify-between gap-3 text-sm">
                <span class="min-w-0 truncate">
                  <span class="font-medium text-gray-900 dark:text-white">{{ row.label }}</span>
                  <span v-if="row.reset" class="ml-2 text-xs text-gray-500">{{ row.reset }}</span>
                </span>
                <span class="shrink-0 tabular-nums text-gray-500">
                  <span class="text-gray-900 dark:text-white">${{ row.used.toFixed(2) }}</span>
                  / ${{ row.limit.toFixed(2) }}
                </span>
              </div>
              <QuotaMeter :value="row.remaining" :label="row.label" />
            </div>

            <div
              v-if="rowsById[subscription.id].length === 0"
              class="flex items-center gap-3 rounded-lg bg-gray-50 px-3 py-2.5 dark:bg-dark-800"
              :title="t('userSubscriptions.unlimitedDesc')"
            >
              <span class="text-2xl leading-none text-emerald-600" aria-hidden="true">∞</span>
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{ t('userSubscriptions.unlimited') }}</span>
            </div>
          </div>

          <footer class="viz-card-foot mt-auto items-center justify-between">
            <span class="inline-flex min-w-0 items-center gap-1.5">
              <Icon name="calendar" size="sm" />
              <span class="sr-only">{{ t('userSubscriptions.expires') }}</span>
              <span
                v-if="subscription.expires_at"
                class="truncate"
                :class="getExpirationClass(subscription.expires_at)"
              >
                {{ formatExpirationDate(subscription.expires_at) }}
              </span>
              <span v-else>{{ t('userSubscriptions.noExpiration') }}</span>
            </span>
            <button
              v-if="subscription.status === 'active'"
              type="button"
              class="btn btn-secondary btn-sm"
              @click="router.push({ path: '/purchase', query: { tab: 'subscription', group: String(subscription.group_id) } })"
            >
              <Icon name="refresh" size="xs" />
              {{ t('payment.renewNow') }}
            </button>
          </footer>
        </article>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import subscriptionsAPI from '@/api/subscriptions'
import type { UserSubscription } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import QuotaMeter from '@/components/ui/QuotaMeter.vue'
import { formatDateTimeToMinute } from '@/utils/format'
import { hasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { platformLabel } from '@/utils/platformColors'
import {
  getExpirationDateRelation,
  getRemainingDurationParts,
  isOneTimeDailyQuota,
  type RemainingDurationParts
} from '@/utils/subscriptionQuota'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const subscriptions = ref<UserSubscription[]>([])
const loading = ref(true)

function subscriptionHasPeakRate(subscription: UserSubscription): boolean {
  return hasPeakRate(subscription.group)
}

function subscriptionPeakRateLabel(subscription: UserSubscription): string {
  return formatPeakRateWindow(subscription.group, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
}

async function loadSubscriptions() {
  try {
    loading.value = true
    subscriptions.value = await subscriptionsAPI.getMySubscriptions()
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    appStore.showError(t('userSubscriptions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

interface QuotaRow {
  key: string
  label: string
  used: number
  limit: number
  remaining: number
  reset: string
}

function quotaRows(subscription: UserSubscription): QuotaRow[] {
  const group = subscription.group
  if (!group) return []
  const windows = [
    { key: 'daily', limit: group.daily_limit_usd, used: subscription.daily_usage_usd, start: subscription.daily_window_start, reset: () => formatDailyUsageWindow(subscription) },
    { key: 'weekly', limit: group.weekly_limit_usd, used: subscription.weekly_usage_usd, start: subscription.weekly_window_start, reset: () => t('userSubscriptions.resetIn', { time: formatResetTime(subscription.weekly_window_start, 168) }) },
    { key: 'monthly', limit: group.monthly_limit_usd, used: subscription.monthly_usage_usd, start: subscription.monthly_window_start, reset: () => t('userSubscriptions.resetIn', { time: formatResetTime(subscription.monthly_window_start, 720) }) },
  ]
  return windows
    .filter((window) => !!window.limit)
    .map((window) => {
      const limit = window.limit as number
      const used = window.used || 0
      return {
        key: window.key,
        label: t(`userSubscriptions.${window.key}`),
        used,
        limit,
        remaining: Math.max(0, 100 - (used / limit) * 100),
        reset: window.start ? window.reset() : '',
      }
    })
}

const rowsById = computed(() =>
  Object.fromEntries(subscriptions.value.map((subscription) => [subscription.id, quotaRows(subscription)])) as Record<number, QuotaRow[]>
)

function statusBadgeClass(status: string): string {
  if (status === 'active') return 'badge-success'
  if (status === 'expired') return 'badge-gray'
  return 'badge-danger'
}

function formatExpirationDate(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  const relation = getExpirationDateRelation(expires, now)

  if (relation === null) return ''

  if (relation === 'expired') {
    return t('userSubscriptions.status.expired')
  }

  const dateStr = formatDateTimeToMinute(expires)

  if (relation === 'today') {
    return `${dateStr} (${t('common.today')})`
  }
  if (relation === 'tomorrow') {
    return `${dateStr} (${t('common.tomorrow')})`
  }

  return t('userSubscriptions.daysRemaining', { days }) + ` (${dateStr})`
}

function getExpirationClass(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (diff <= 0) return 'font-medium text-red-600'
  if (days <= 3) return 'text-red-600'
  if (days <= 7) return 'text-orange-600'
  return 'text-gray-700 dark:text-gray-300'
}

function formatDurationParts(parts: RemainingDurationParts): string {
  if (parts.days > 0) {
    return `${parts.days}d ${parts.hours}h`
  }

  if (parts.hours > 0) {
    return `${parts.hours}h ${parts.minutes}m`
  }

  return `${parts.minutes}m`
}

function formatDailyUsageWindow(subscription: UserSubscription): string {
  if (isOneTimeDailyQuota(subscription) && subscription.expires_at) {
    const parts = getRemainingDurationParts(subscription.expires_at)
    if (!parts) return t('userSubscriptions.windowNotActive')
    return t('userSubscriptions.quotaEndsIn', { time: formatDurationParts(parts) })
  }

  return t('userSubscriptions.resetIn', {
    time: formatResetTime(subscription.daily_window_start, 24)
  })
}

function formatResetTime(windowStart: string | null, windowHours: number): string {
  if (!windowStart) return t('userSubscriptions.windowNotActive')

  const start = new Date(windowStart)
  const end = new Date(start.getTime() + windowHours * 60 * 60 * 1000)
  const parts = getRemainingDurationParts(end)

  return parts ? formatDurationParts(parts) : t('userSubscriptions.windowNotActive')
}

onMounted(() => {
  loadSubscriptions()
})
</script>
