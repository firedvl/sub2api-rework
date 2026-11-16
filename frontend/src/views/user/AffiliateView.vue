<template>
  <AppLayout>
    <div class="ui-page ui-stagger">
      <template v-if="loading">
        <div class="skeleton h-40 rounded-xl" aria-busy="true"></div>
        <div class="grid gap-4 lg:grid-cols-3">
          <div v-for="i in 3" :key="i" class="skeleton h-24 rounded-xl"></div>
        </div>
      </template>

      <template v-else-if="detail">
        <!-- Invite hero -->
        <section class="ui-card" data-testid="affiliate-hero">
          <div class="flex flex-col gap-5 p-5 md:flex-row md:items-center md:gap-8">
            <div class="shrink-0" :title="t('affiliate.stats.rebateRateHint')">
              <p class="text-4xl font-semibold tabular-nums tracking-tight text-primary-600 dark:text-primary-400">
                <CountUp :value="detail.effective_rebate_rate_percent ?? 0" :format="formatRate" /><span class="ml-0.5 text-xl font-medium">%</span>
              </p>
              <p class="mt-1 text-xs text-gray-500">{{ t('affiliate.stats.rebateRate') }}</p>
            </div>

            <div class="grid min-w-0 flex-1 gap-3">
              <div v-for="field in shareFields" :key="field.key" class="min-w-0">
                <p class="ui-section-label">{{ field.label }}</p>
                <div
                  class="flex flex-col items-stretch gap-2 rounded-lg border border-gray-200 bg-gray-50 py-1.5 pl-3 pr-1.5 dark:border-dark-700 dark:bg-dark-900 sm:flex-row sm:items-center"
                >
                  <code
                    class="min-w-0 break-all text-sm sm:flex-1 sm:truncate"
                    :class="field.key === 'code' ? 'font-semibold text-gray-900 dark:text-white' : 'text-gray-700 dark:text-gray-300'"
                  >{{ field.value }}</code>
                  <button
                    type="button"
                    class="btn btn-sm w-full sm:w-auto sm:shrink-0"
                    :class="field.key === 'link' ? 'btn-primary' : 'btn-secondary'"
                    @click="field.copy"
                  >
                    <Icon
                      :key="copiedKey === field.key ? 'check' : 'copy'"
                      :name="copiedKey === field.key ? 'check' : 'copy'"
                      size="sm"
                      :class="{ 'copy-pop': copiedKey === field.key }"
                    />
                    <span>{{ field.button }}</span>
                  </button>
                </div>
              </div>
            </div>
          </div>

          <details class="affiliate-tips border-t border-gray-100 px-5 py-3 text-sm dark:border-dark-700">
            <summary class="inline-flex cursor-pointer list-none items-center gap-1.5 rounded-md text-gray-500 hover:text-gray-900 dark:hover:text-white">
              <Icon name="infoCircle" size="sm" />
              {{ t('affiliate.tips.title') }}
            </summary>
            <ol class="mt-2 list-decimal space-y-1 pl-5 text-xs text-gray-500">
              <li>{{ t('affiliate.tips.line1') }}</li>
              <li>{{ t('affiliate.tips.line2', { rate: `${formattedRebateRate}%` }) }}</li>
              <li>{{ t('affiliate.tips.line3') }}</li>
              <li v-if="detail.aff_frozen_quota > 0">{{ t('affiliate.tips.line4') }}</li>
            </ol>
          </details>
        </section>

        <!-- KPIs -->
        <dl class="grid gap-4 lg:grid-cols-3">
          <div class="viz-card viz-kpi">
            <dt class="viz-kpi-label"><Icon name="users" size="sm" />{{ t('affiliate.stats.invitedUsers') }}</dt>
            <dd class="viz-kpi-row">
              <CountUp class="viz-kpi-value" :value="detail.aff_count" />
            </dd>
          </div>
          <div class="viz-card viz-kpi">
            <dt class="viz-kpi-label"><Icon name="wallet" size="sm" />{{ t('affiliate.stats.availableQuota') }}</dt>
            <dd class="viz-kpi-row flex-wrap">
              <CountUp class="viz-kpi-value text-emerald-600" :value="detail.aff_quota" :format="formatMoney" />
              <span :title="detail.aff_quota <= 0 ? t('affiliate.transfer.empty') : t('affiliate.transfer.description')">
                <button
                  type="button"
                  class="btn btn-primary btn-sm"
                  :disabled="transferring || detail.aff_quota <= 0"
                  :aria-label="t('affiliate.transfer.title')"
                  @click="transferQuota"
                >
                  <Icon v-if="transferring" name="refresh" size="sm" class="animate-spin" />
                  <Icon v-else name="arrowRight" size="sm" />
                  <span>{{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.button') }}</span>
                </button>
              </span>
            </dd>
          </div>
          <div class="viz-card viz-kpi">
            <dt class="viz-kpi-label"><Icon name="chartBar" size="sm" />{{ t('affiliate.stats.totalQuota') }}</dt>
            <dd class="viz-kpi-row flex-wrap">
              <CountUp class="viz-kpi-value" :value="detail.aff_history_quota" :format="formatMoney" />
              <span
                v-if="detail.aff_frozen_quota > 0"
                class="badge badge-warning"
                :title="t('affiliate.stats.frozenQuotaHint')"
              >
                {{ t('affiliate.stats.frozenQuota') }} {{ formatCurrency(detail.aff_frozen_quota) }}
              </span>
            </dd>
          </div>
        </dl>

        <!-- Invitees -->
        <section class="ui-card">
          <div class="viz-card-head">
            <h3 class="viz-card-title">{{ t('affiliate.invitees.title') }}</h3>
            <span class="badge badge-gray">{{ detail.invitees.length }}</span>
          </div>
          <div v-if="detail.invitees.length === 0" class="ui-empty">
            <span class="ui-empty-icon"><Icon name="userPlus" size="lg" /></span>
            <span>{{ t('affiliate.invitees.empty') }}</span>
          </div>
          <div v-else class="ui-table-wrap mt-2 max-h-[28rem]">
            <table class="ui-table min-w-[560px]">
              <thead>
                <tr>
                  <th>{{ t('affiliate.invitees.columns.email') }}</th>
                  <th>{{ t('affiliate.invitees.columns.username') }}</th>
                  <th class="is-num">{{ t('affiliate.invitees.columns.rebate') }}</th>
                  <th>{{ t('affiliate.invitees.columns.joinedAt') }}</th>
                </tr>
              </thead>
              <tbody class="ui-rows-enter">
                <tr v-for="item in detail.invitees" :key="item.user_id">
                  <td><span class="ui-cell-main max-w-[16rem]">{{ item.email || '-' }}</span></td>
                  <td class="text-gray-600 dark:text-gray-300">{{ item.username || '-' }}</td>
                  <td class="is-num font-medium text-emerald-600">{{ formatCurrency(item.total_rebate) }}</td>
                  <td class="whitespace-nowrap text-gray-500">{{ formatDateTime(item.created_at) || '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import CountUp from '@/components/ui/CountUp.vue'
import userAPI from '@/api/user'
import type { UserAffiliateDetail } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const transferring = ref(false)
const detail = ref<UserAffiliateDetail | null>(null)

const inviteLink = computed(() => {
  if (!detail.value) return ''
  if (typeof window === 'undefined') return `/register?aff=${encodeURIComponent(detail.value.aff_code)}`
  return `${window.location.origin}/register?aff=${encodeURIComponent(detail.value.aff_code)}`
})

// Rebate rate is a percentage in the range [0, 100]; backend already clamps it.
// We trim trailing zeros (e.g. 20.00 → "20", 12.50 → "12.5") for a cleaner UI.
const formattedRebateRate = computed(() => {
  const v = detail.value?.effective_rebate_rate_percent ?? 0
  const rounded = Math.round(v * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
})

function formatRate(value: number): string {
  const rounded = Math.round(value * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
}

function formatMoney(value: number): string {
  return formatCurrency(value)
}

type ShareKey = 'link' | 'code'
const copiedKey = ref<ShareKey | null>(null)
let copiedTimer: ReturnType<typeof setTimeout> | undefined

function markCopied(key: ShareKey, ok: boolean): void {
  if (!ok) return
  copiedKey.value = key
  clearTimeout(copiedTimer)
  copiedTimer = setTimeout(() => { copiedKey.value = null }, 1600)
}

const shareFields = computed(() => [
  { key: 'link' as const, label: t('affiliate.inviteLink'), value: inviteLink.value, button: t('affiliate.copyLink'), copy: copyInviteLink },
  { key: 'code' as const, label: t('affiliate.yourCode'), value: detail.value?.aff_code ?? '', button: t('affiliate.copyCode'), copy: copyCode },
])

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) {
    loading.value = true
  }
  try {
    detail.value = await userAPI.getAffiliateDetail()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

async function copyCode(): Promise<void> {
  if (!detail.value?.aff_code) return
  markCopied('code', await copyToClipboard(detail.value.aff_code, t('affiliate.codeCopied')))
}

async function copyInviteLink(): Promise<void> {
  if (!inviteLink.value) return
  markCopied('link', await copyToClipboard(inviteLink.value, t('affiliate.linkCopied')))
}

async function transferQuota(): Promise<void> {
  if (!detail.value || detail.value.aff_quota <= 0 || transferring.value) return
  transferring.value = true
  try {
    const resp = await userAPI.transferAffiliateQuota()
    appStore.showSuccess(t('affiliate.transfer.success', { amount: formatCurrency(resp.transferred_quota) }))
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.transferFailed')))
  } finally {
    transferring.value = false
  }
}

onMounted(() => {
  void loadAffiliateDetail()
})

onBeforeUnmount(() => clearTimeout(copiedTimer))
</script>

<style scoped>
.copy-pop {
  animation: ui-pop 360ms cubic-bezier(0.34, 1.56, 0.64, 1) both;
}

.affiliate-tips summary::-webkit-details-marker {
  display: none;
}

.affiliate-tips summary:focus-visible {
  outline: 2px solid rgb(var(--operator-accent-rgb) / 0.6);
  outline-offset: 2px;
}

@media (prefers-reduced-motion: reduce) {
  .copy-pop {
    animation: none;
  }
}
</style>
