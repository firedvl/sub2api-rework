<template>
  <SideDrawer
    :open="open && !!record"
    :title="record?.model || ''"
    :subtitle="record ? formatDateTime(record.created_at) : ''"
    width="34rem"
    @close="emit('close')"
  >
    <template #leading>
      <ProviderMark v-if="record" :platform="platformOf(record)" :size="36" />
    </template>
    <template #badge>
      <span v-if="record" class="badge" :class="requestTypeBadge(record)">{{ requestTypeLabel(record) }}</span>
    </template>

    <template v-if="record">
      <div class="usage-drawer-stats">
        <div>
          <small>{{ t('usage.userBilled') }}</small>
          <b class="text-green-600 dark:text-green-400">{{ formatCost(record.actual_cost) }}</b>
        </div>
        <div>
          <small>{{ t('usage.tokens') }}</small>
          <b>{{ isImageUsage(record) ? `${record.image_count}${t('usage.imageUnit')}` : totalTokens(record).toLocaleString() }}</b>
        </div>
        <div>
          <small>{{ t('usage.duration') }}</small>
          <b class="inline-flex items-center gap-2">
            <span class="ui-dot" :class="DOT_CLASS[latencySeverity(record)]" aria-hidden="true"></span>
            {{ formatDuration(record.duration_ms) }}
          </b>
        </div>
      </div>

      <p v-if="record.usage_observation_complete !== true" class="usage-drawer-note">
        <Icon name="exclamationTriangle" size="sm" />
        {{ t(usageObservationKey(record.usage_observation_complete)) }}
      </p>

      <section>
        <h3 class="ui-section-label">{{ t('admin.activityUi.request') }}</h3>
        <dl class="ui-dl">
          <dt>{{ t('admin.usage.user') }}</dt>
          <dd>
            <button
              v-if="record.user?.email"
              type="button"
              class="usage-drawer-link"
              :title="t('admin.usage.clickToViewBalance')"
              @click="emit('userClick', record.user_id, record.user?.email)"
            >{{ record.user.email }}</button>
            <span v-else>-</span>
            <span class="text-gray-400"> #{{ record.user_id }}</span>
            <span v-if="record.user?.deleted_at" class="badge badge-danger ml-1">{{ t('admin.usage.userDeletedBadge') }}</span>
          </dd>
          <dt>{{ t('usage.apiKeyFilter') }}</dt>
          <dd>{{ record.api_key?.name || '-' }} <span class="text-gray-400">#{{ record.api_key_id }}</span></dd>
          <dt>{{ t('admin.usage.account') }}</dt>
          <dd>{{ record.account?.name || '-' }} <span v-if="record.account_id != null" class="text-gray-400">#{{ record.account_id }}</span></dd>
          <dt>{{ t('admin.usage.group') }}</dt>
          <dd>{{ record.group?.name || '-' }}</dd>
          <dt>{{ t('usage.type') }}</dt>
          <dd class="flex flex-wrap items-center gap-1">
            <span class="badge" :class="requestTypeBadge(record)">{{ requestTypeLabel(record) }}</span>
            <span v-if="record.native_compaction_v2" class="badge badge-primary">{{ t('usage.nativeCompactionV2') }}</span>
          </dd>
          <dt>{{ t('admin.usage.billingMode') }}</dt>
          <dd>{{ getBillingModeLabel(getDisplayBillingMode(record), t) }}</dd>
          <dt>{{ t('admin.usage.billingType') }}</dt>
          <dd>{{ record.billing_type === 1 ? t('admin.usage.billingTypeSubscription') : t('admin.usage.billingTypeBalance') }}</dd>
          <template v-if="record.reasoning_effort || record.upstream_reasoning_effort">
            <dt>{{ t('usage.reasoningEffort') }}</dt>
            <dd>
              {{ formatReasoningEffort(record.reasoning_effort) }}<template v-if="hasReasoningEffortMapping(record)"> → {{ formatReasoningEffort(record.upstream_reasoning_effort) }}</template>
            </dd>
          </template>
          <dt>{{ t('usage.inboundEndpoint') }}</dt>
          <dd class="font-mono text-xs">{{ record.inbound_endpoint?.trim() || '-' }}</dd>
          <dt>{{ t('usage.upstreamEndpoint') }}</dt>
          <dd class="font-mono text-xs">{{ record.upstream_endpoint?.trim() || '-' }}</dd>
          <dt>{{ t('admin.usage.ipAddress') }}</dt>
          <dd>
            <template v-if="record.ip_address">
              <span class="font-mono text-xs">{{ record.ip_address }}</span>
              <IpGeoCell :ip="record.ip_address" />
            </template>
            <span v-else>-</span>
          </dd>
          <dt>{{ t('usage.userAgent') }}</dt>
          <dd class="text-xs">{{ record.user_agent || '-' }}</dd>
        </dl>
      </section>

      <section>
        <h3 class="ui-section-label">{{ t('usage.model') }}</h3>
        <dl class="ui-dl">
          <dt>{{ t('usage.requestedModel') }}</dt>
          <dd>{{ record.model || '-' }}</dd>
          <dt>{{ t('usage.sentUpstreamModel') }}</dt>
          <dd>{{ sentUpstreamModel(record) || '-' }}</dd>
          <template v-if="record.upstream_response_model">
            <dt>{{ t('usage.upstreamResponseModel') }}</dt>
            <dd>
              {{ record.upstream_response_model }}
              <span
                v-if="record.upstream_model_mismatch === true"
                class="badge ml-1"
                :class="isLikelyModelVariant(record) ? 'badge-warning' : 'badge-danger'"
              >{{ isLikelyModelVariant(record) ? t('usage.modelVariant') : t('usage.modelMismatch') }}</span>
            </dd>
          </template>
          <template v-if="mappingSteps(record).length">
            <dt>{{ t('usage.mapping') }}</dt>
            <dd class="usage-drawer-chain">
              <span v-for="(step, i) in mappingSteps(record)" :key="i">
                <Icon v-if="i > 0" name="chevronRight" size="xs" class="text-gray-400" />{{ step }}
              </span>
            </dd>
          </template>
        </dl>
      </section>

      <!-- Tones sit on an inner span: `.ui-dl dd` sets its own color and would override them. -->
      <section>
        <h3 class="ui-section-label">{{ t('usage.tokenDetails') }}</h3>
        <dl class="ui-dl usage-drawer-num">
          <template v-for="item in tokenRows" :key="item.label">
            <dt>{{ item.label }}</dt>
            <dd><span :class="item.tone">{{ item.value }}</span></dd>
          </template>
        </dl>
      </section>

      <section data-testid="usage-cost-details">
        <h3 class="ui-section-label">{{ t('usage.costDetails') }}</h3>
        <dl class="ui-dl usage-drawer-num">
          <template v-for="item in costRows" :key="item.label">
            <dt>{{ item.label }}</dt>
            <dd><span :class="item.tone">{{ item.value }}</span></dd>
          </template>
        </dl>
      </section>

      <section>
        <h3 class="ui-section-label">{{ t('usage.latency') }}</h3>
        <dl class="ui-dl usage-drawer-num">
          <dt>{{ t('usage.firstToken') }}</dt>
          <dd><span :class="record.first_token_ms != null ? LATENCY_TONE_CLASS[firstTokenSeverity(record.first_token_ms)] : ''">{{ formatDuration(record.first_token_ms) }}</span></dd>
          <dt>{{ t('usage.duration') }}</dt>
          <dd><span :class="LATENCY_TONE_CLASS[durationSeverity(record.duration_ms ?? 0)]">{{ formatDuration(record.duration_ms) }}</span></dd>
        </dl>
      </section>

      <section>
        <h3 class="ui-section-label">{{ t('admin.activityUi.ids') }}</h3>
        <ul class="usage-drawer-ids">
          <li v-for="item in idRows" :key="item.label">
            <span class="usage-drawer-id-label">{{ item.label }}</span>
            <code :title="item.value">{{ item.value }}</code>
            <button
              type="button"
              class="app-icon-button usage-drawer-copy"
              :aria-label="`${t('keys.copyToClipboard')}: ${item.label}`"
              @click="copy(item.value)"
            >
              <Icon :name="copiedValue === item.value ? 'check' : 'copy'" size="sm" />
            </button>
          </li>
        </ul>
      </section>
    </template>
  </SideDrawer>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { formatDateTime, formatReasoningEffort } from '@/utils/format'
import { formatMultiplier } from '@/utils/formatters'
import { formatTokenPricePerMillion } from '@/utils/usagePricing'
import { getUsageServiceTierLabel } from '@/utils/usageServiceTier'
import { usageObservationKey } from '@/utils/usageObservation'
import { durationSeverity, firstTokenSeverity } from '@/utils/latencyHealth'
import {
  BILLING_MODE_TOKEN,
  getBillingModeLabel,
  getDisplayBillingMode,
  imageUnitPrice,
  isImageUsage,
} from '@/utils/billingMode'
import {
  formatImageBillingSize,
  formatImageInputSize,
  formatImageOutputSize,
  formatImageSizeBreakdown,
  formatImageSizeSource,
  hasImageInputCost,
  hasImageInputTokens,
  hasImageOutputCost,
  hasImageOutputTokens,
  textInputTokens,
  textOutputTokens,
} from '@/utils/imageUsage'
import Icon from '@/components/icons/Icon.vue'
import IpGeoCell from '@/components/common/IpGeoCell.vue'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import type { AdminUsageLog } from '@/types'
import {
  DOT_CLASS,
  LATENCY_TONE_CLASS,
  accountBilled,
  formatCost,
  formatDuration,
  hasReasoningEffortMapping,
  isLikelyModelVariant,
  latencySeverity,
  mappingSteps,
  platformOf,
  requestTypeBadge,
  sentUpstreamModel,
  totalTokens,
  useRecordLabels,
} from './usageRecord'

const props = withDefaults(defineProps<{
  record: AdminUsageLog | null
  open: boolean
  showAccountBilling?: boolean
}>(), { showAccountBilling: true })

const emit = defineEmits<{
  close: []
  userClick: [userID: number, email?: string]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const { requestTypeLabel } = useRecordLabels(t)

interface Row {
  label: string
  value: string
  tone?: string
}

const money = (value: number | null | undefined) => `$${(value ?? 0).toFixed(8)}`

const tokenRows = computed<Row[]>(() => {
  const r = props.record
  if (!r) return []
  const rows: Row[] = []
  const imageIn = hasImageInputTokens(r)
  const imageOut = hasImageOutputTokens(r)
  rows.push({ label: t('admin.usage.inputTokens'), value: (imageIn ? textInputTokens(r) : r.input_tokens || 0).toLocaleString() })
  if (imageIn) rows.push({ label: t('usage.imageInputTokens'), value: r.image_input_tokens.toLocaleString(), tone: 'text-fuchsia-600 dark:text-fuchsia-400' })
  rows.push({ label: t('admin.usage.outputTokens'), value: (imageOut ? textOutputTokens(r) : r.output_tokens || 0).toLocaleString() })
  if (imageOut) rows.push({ label: t('usage.imageOutputTokens'), value: r.image_output_tokens.toLocaleString(), tone: 'text-pink-600 dark:text-pink-400' })
  if (r.cache_creation_5m_tokens > 0 || r.cache_creation_1h_tokens > 0) {
    if (r.cache_creation_5m_tokens > 0) rows.push({ label: t('admin.usage.cacheCreation5mTokens'), value: r.cache_creation_5m_tokens.toLocaleString() })
    if (r.cache_creation_1h_tokens > 0) rows.push({ label: t('admin.usage.cacheCreation1hTokens'), value: r.cache_creation_1h_tokens.toLocaleString() })
  } else {
    rows.push({ label: t('admin.usage.cacheCreationTokens'), value: (r.cache_creation_tokens || 0).toLocaleString() })
  }
  rows.push({ label: t('admin.usage.cacheReadTokens'), value: (r.cache_read_tokens || 0).toLocaleString(), tone: r.cache_read_tokens > 0 ? 'text-sky-600 dark:text-sky-400' : '' })
  if (r.cache_ttl_overridden) {
    rows.push({
      label: t('usage.cacheTtlOverriddenLabel'),
      value: r.cache_creation_1h_tokens > 0 ? t('usage.cacheTtlOverridden1h') : t('usage.cacheTtlOverridden5m'),
      tone: 'text-rose-600 dark:text-rose-400',
    })
  }
  if (isImageUsage(r)) {
    rows.push({ label: t('usage.imageCount'), value: `${r.image_count}${t('usage.imageUnit')}` })
  }
  rows.push({ label: t('usage.totalTokens'), value: totalTokens(r).toLocaleString(), tone: 'font-semibold' })
  return rows
})

const costRows = computed<Row[]>(() => {
  const r = props.record
  if (!r) return []
  const rows: Row[] = []
  if (r.input_cost > 0) rows.push({ label: t('admin.usage.inputCost'), value: money(r.input_cost) })
  if (hasImageInputCost(r)) rows.push({ label: t('usage.imageInputCost'), value: money(r.image_input_cost) })
  if (r.output_cost > 0) rows.push({ label: t('admin.usage.outputCost'), value: money(r.output_cost) })
  if (hasImageOutputCost(r)) rows.push({ label: t('usage.imageOutputCost'), value: money(r.image_output_cost) })
  if (!isImageUsage(r) && (!r.billing_mode || r.billing_mode === BILLING_MODE_TOKEN)) {
    const perM = t('usage.perMillionTokens')
    if (textInputTokens(r) > 0) rows.push({ label: t('usage.inputTokenPrice'), value: `${formatTokenPricePerMillion(r.input_cost, textInputTokens(r))} ${perM}` })
    if (hasImageInputTokens(r)) rows.push({ label: t('usage.imageInputTokenPrice'), value: `${formatTokenPricePerMillion(r.image_input_cost ?? 0, r.image_input_tokens)} ${perM}` })
    if (r.output_cost > 0 && textOutputTokens(r) > 0) rows.push({ label: t('usage.outputTokenPrice'), value: `${formatTokenPricePerMillion(r.output_cost, textOutputTokens(r))} ${perM}` })
    if (hasImageOutputTokens(r)) rows.push({ label: t('usage.imageOutputTokenPrice'), value: `${formatTokenPricePerMillion(r.image_output_cost ?? 0, r.image_output_tokens)} ${perM}` })
  } else if (isImageUsage(r)) {
    rows.push({ label: t('usage.imageBillingSize'), value: formatImageBillingSize(r, t) })
    rows.push({ label: t('usage.imageSizeSource'), value: formatImageSizeSource(r, t) })
    rows.push({ label: t('usage.imageInputSize'), value: formatImageInputSize(r, t) })
    rows.push({ label: t('usage.imageOutputSize'), value: formatImageOutputSize(r, t) })
    const breakdown = formatImageSizeBreakdown(r)
    if (breakdown) rows.push({ label: t('usage.imageSizeBreakdown'), value: breakdown })
    rows.push({ label: t('usage.imageUnitPrice'), value: money(imageUnitPrice(r)) })
    rows.push({ label: t('usage.imageTotalPrice'), value: money(r.total_cost) })
  } else {
    rows.push({ label: t('usage.unitPrice'), value: money(r.total_cost) })
  }
  if (r.cache_creation_cost > 0) rows.push({ label: t('admin.usage.cacheCreationCost'), value: money(r.cache_creation_cost) })
  if (r.cache_read_cost > 0) rows.push({ label: t('admin.usage.cacheReadCost'), value: money(r.cache_read_cost) })
  rows.push({ label: t('usage.serviceTier'), value: getUsageServiceTierLabel(r.service_tier, t) })
  rows.push({ label: t('usage.rate'), value: `${formatMultiplier(r.rate_multiplier || 1)}x` })
  if (r.long_context_billing_applied) rows.push({ label: t('admin.activityUi.longContext'), value: 'x2', tone: 'text-amber-600 dark:text-amber-400' })
  rows.push({ label: t('usage.original'), value: money(r.total_cost) })
  rows.push({ label: t('usage.userBilled'), value: money(r.actual_cost), tone: 'font-semibold text-green-600 dark:text-green-400' })
  if (props.showAccountBilling) {
    rows.push({ label: t('usage.accountMultiplier'), value: `${formatMultiplier(r.account_rate_multiplier ?? 1)}x` })
    rows.push({ label: t('usage.accountBilled'), value: money(accountBilled(r)), tone: 'font-semibold' })
  }
  if (r.billing_tier) rows.push({ label: t('admin.activityUi.billingTier'), value: r.billing_tier })
  rows.push({ label: t('usage.observationCompleteness'), value: t(usageObservationKey(r.usage_observation_complete)) })
  return rows
})

const idRows = computed<Row[]>(() => {
  const r = props.record
  if (!r) return []
  const rows: Row[] = [{ label: t('admin.activityUi.recordId'), value: String(r.id) }]
  if (r.request_id) rows.push({ label: t('admin.usage.requestId'), value: r.request_id })
  if (r.upstream_request_id) rows.push({ label: t('admin.usage.upstreamRequestId'), value: r.upstream_request_id })
  if (r.channel_id != null) rows.push({ label: t('admin.activityUi.channelId'), value: String(r.channel_id) })
  return rows
})

const copiedValue = ref<string | null>(null)
const copy = async (value: string) => {
  try {
    await navigator.clipboard.writeText(value)
    copiedValue.value = value
    appStore.showSuccess(t('keys.copied'))
    window.setTimeout(() => {
      if (copiedValue.value === value) copiedValue.value = null
    }, 2000)
  } catch {
    appStore.showError(t('common.copyFailed'))
  }
}
</script>

<style scoped>
.usage-drawer-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1px;
  overflow: hidden;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-border);
}
.usage-drawer-stats > div {
  display: grid;
  gap: 0.125rem;
  padding: 0.625rem 0.75rem;
  background: var(--operator-card);
}
.usage-drawer-stats small {
  color: var(--operator-subtle-foreground);
  font-size: 0.72rem;
}
.usage-drawer-stats b {
  color: var(--operator-foreground);
  font-size: 1.0625rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.usage-drawer-note {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border-radius: var(--operator-radius);
  background: var(--operator-warning-soft);
  color: var(--operator-warning);
  font-size: 0.8125rem;
}
.usage-drawer-num dd {
  font-variant-numeric: tabular-nums;
  text-align: right;
}
.usage-drawer-link {
  color: var(--operator-accent-text);
  text-align: left;
  text-decoration: underline dashed;
  text-underline-offset: 3px;
}
.usage-drawer-chain {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.25rem;
}
.usage-drawer-chain > span {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
}
.usage-drawer-ids {
  display: grid;
  gap: 0.375rem;
}
.usage-drawer-ids li {
  display: grid;
  grid-template-columns: 7.5rem minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.8125rem;
}
.usage-drawer-id-label {
  color: var(--operator-subtle-foreground);
}
.usage-drawer-ids code {
  overflow: hidden;
  padding: 0.25rem 0.5rem;
  border-radius: var(--operator-radius-sm);
  background: var(--operator-raised);
  color: var(--operator-foreground);
  font-size: 0.75rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.usage-drawer-copy {
  min-width: 2rem;
  height: 2rem;
}
</style>
