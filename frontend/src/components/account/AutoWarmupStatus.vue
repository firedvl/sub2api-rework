<template>
  <div v-if="isAutoWarmupConfigurable(account)" class="min-w-0 text-xs text-gray-600 dark:text-gray-300" data-testid="auto-warmup-reason">
    <span>{{ t(`admin.accounts.autoWarmup.reason.${reason}`) }}</span>
    <dl v-if="details" class="mt-1 grid gap-1 break-words">
      <div v-if="observedAt"><dt class="inline">{{ t('admin.accounts.autoWarmup.observedAt') }}: </dt><dd class="inline">{{ formatTime(observedAt) }}</dd></div>
      <div v-if="attempt?.attempted_at"><dt class="inline">{{ t('admin.accounts.autoWarmup.attemptedAt') }}: </dt><dd class="inline">{{ formatTime(attempt.attempted_at) }}</dd></div>
      <div v-if="attempt?.status === 'succeeded' && attempt.completed_at && (account.platform !== 'anthropic' || attempt.window_started)"><dt class="inline">{{ t('admin.accounts.autoWarmup.succeededAt') }}: </dt><dd class="inline">{{ formatTime(attempt.completed_at) }}</dd></div>
      <div v-if="account.platform === 'anthropic' && attempt?.window_started && attempt.observed_5h_reset_at" data-testid="claude-warmup-reset-at"><dt class="inline">{{ t('admin.accounts.autoWarmup.windowResetAt') }}: </dt><dd class="inline">{{ formatTime(attempt.observed_5h_reset_at) }}</dd></div>
      <div v-if="nextAt"><dt class="inline">{{ t('admin.accounts.autoWarmup.nextEligibleAt') }}: </dt><dd class="inline">{{ formatTime(nextAt) }}</dd></div>
    </dl>
  </div>
  <div v-else-if="account.platform === 'anthropic'" class="min-w-0 text-xs text-gray-600 dark:text-gray-300" data-testid="claude-auto-warmup-unavailable">
    <span>{{ t('admin.accounts.autoWarmup.claudeUnavailable') }}</span>
    <p v-if="details" class="mt-1">{{ t(account.type === 'apikey' ? 'admin.accounts.autoWarmup.claudeAPIBilling' : 'admin.accounts.autoWarmup.claudeUnavailableReason') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account } from '@/types'
import { autoWarmupReason, isAutoWarmupConfigurable } from '@/utils/autoWarmup'

const props = defineProps<{ account: Account; globalEnabled?: boolean; details?: boolean }>()
const { t } = useI18n()
const reason = computed(() => autoWarmupReason(props.account, props.globalEnabled))
const attempt = computed(() => props.account.platform === 'anthropic' ? props.account.extra?.claude_auto_warmup_state : props.account.extra?.codex_auto_warmup_state)
const evaluation = computed(() => props.account.platform === 'anthropic' ? props.account.extra?.claude_auto_warmup_evaluation : props.account.extra?.codex_auto_warmup_evaluation)
const observedAt = computed(() => evaluation.value?.observed_at || (props.account.platform === 'openai' ? props.account.extra?.codex_usage_updated_at : evaluation.value?.checked_at))
const nextAt = computed(() => {
  if (reason.value === 'model_unavailable' && attempt.value?.attempted_at) {
    const at = Date.parse(attempt.value.attempted_at) + 10 * 60 * 1000
    if (Number.isFinite(at)) return new Date(at).toISOString()
  }
  return evaluation.value?.next_eligible_at
})
const formatTime = (value: string) => Number.isFinite(Date.parse(value)) ? new Date(value).toLocaleString() : t('admin.accounts.autoWarmup.unknownTime')
</script>
