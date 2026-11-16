<template>
  <SideDrawer
    :open="open && !!apiKey"
    :title="apiKey?.name || ''"
    :subtitle="apiKey?.group?.name || t('keys.noGroup')"
    width="34rem"
    @close="emit('close')"
  >
    <template #leading>
      <ProviderMark v-if="apiKey?.group?.platform" :platform="apiKey.group.platform" :size="36" />
      <span v-else class="kd-mark" aria-hidden="true"><Icon name="key" size="sm" /></span>
    </template>
    <template #badge>
      <span v-if="apiKey" class="badge" :class="statusBadge">{{ t('keys.status.' + apiKey.status) }}</span>
    </template>

    <template v-if="apiKey">
      <section aria-labelledby="kd-connect">
        <div id="kd-connect" class="ui-section-label">{{ t('personalUi.keys.useTitle') }}</div>
        <div class="kd-fields">
          <div class="kd-field">
            <span class="kd-field-label">{{ t('personalUi.keys.baseUrl') }}</span>
            <code class="kd-field-value" data-testid="key-drawer-base-url">{{ apiBase }}</code>
            <CopyIconButton :text="apiBase" :label="t('personalUi.keys.copyBaseUrl')" :copied-label="t('keys.copied')" />
          </div>
          <div class="kd-field">
            <span class="kd-field-label">{{ t('personalUi.keys.key') }}</span>
            <code class="kd-field-value">{{ revealed ? apiKey.key : maskApiKey(apiKey.key) }}</code>
            <button
              type="button"
              class="app-icon-button kd-icon"
              :aria-label="revealed ? t('personalUi.keys.hideKey') : t('personalUi.keys.showKey')"
              :aria-pressed="revealed"
              @click="revealed = !revealed"
            >
              <Icon :name="revealed ? 'eyeOff' : 'eye'" size="sm" />
            </button>
            <CopyIconButton :text="apiKey.key" :label="t('personalUi.keys.copyKey')" :copied-label="t('keys.copied')" />
          </div>
        </div>
      </section>

      <section class="kd-snippet" :aria-label="t('personalUi.keys.snippet')">
        <div class="kd-snippet-head">
          <div class="viz-seg" role="group" :aria-label="t('personalUi.keys.snippet')">
            <button
              v-for="item in snippets"
              :key="item.id"
              type="button"
              :aria-pressed="snippetId === item.id"
              @click="snippetId = item.id"
            >{{ item.label }}</button>
          </div>
          <CopyIconButton :text="activeSnippet.code" :label="t('personalUi.keys.copySnippet')" :copied-label="t('keys.copied')" />
        </div>
        <Transition name="fade" mode="out-in">
          <pre :key="snippetId" class="code-block kd-code"><code>{{ shownSnippet }}</code></pre>
        </Transition>
      </section>

      <section v-if="hasEndpoints">
        <div class="ui-section-label">{{ t('personalUi.keys.endpoints') }}</div>
        <EndpointPopover :api-base-url="baseUrl" :custom-endpoints="customEndpoints" />
      </section>

      <section>
        <div class="viz-kpis kd-kpis">
          <div class="viz-card viz-kpi">
            <span class="viz-kpi-label">{{ t('personalUi.keys.today') }}</span>
            <span class="viz-kpi-value"><CountUp :value="usage?.today_actual_cost ?? 0" :format="money" /></span>
          </div>
          <div class="viz-card viz-kpi">
            <span class="viz-kpi-label">{{ t('personalUi.keys.last30d') }}</span>
            <span class="viz-kpi-value"><CountUp :value="usage?.total_actual_cost ?? 0" :format="money" /></span>
          </div>
        </div>
      </section>

      <section v-if="apiKey.quota > 0 || windows.length">
        <div class="ui-section-label">{{ t('personalUi.keys.limits') }}</div>
        <div class="kd-limits">
          <div v-if="apiKey.quota > 0" class="kd-limit">
            <div class="kd-limit-head">
              <span>{{ t('keys.quota') }}</span>
              <span class="tabular-nums">${{ (apiKey.quota_used ?? 0).toFixed(2) }} / ${{ apiKey.quota.toFixed(2) }}</span>
            </div>
            <QuotaMeter :value="remaining(apiKey.quota_used, apiKey.quota)" :label="t('personalUi.keys.quotaLeft')" />
          </div>
          <div v-for="window in windows" :key="window.id" class="kd-limit">
            <div class="kd-limit-head">
              <span>{{ window.id }}</span>
              <span class="tabular-nums">
                ${{ window.used.toFixed(2) }} / ${{ window.limit.toFixed(2) }}
                <small v-if="window.reset" class="text-gray-500">· {{ window.reset }}</small>
              </span>
            </div>
            <QuotaMeter :value="remaining(window.used, window.limit)" :label="window.id" />
          </div>
          <button
            v-if="windows.some((window) => window.used > 0)"
            type="button"
            class="btn btn-ghost btn-sm self-start"
            @click="emit('reset-rate-limit', apiKey)"
          >
            <Icon name="refresh" size="xs" />
            {{ t('keys.resetUsage') }}
          </button>
        </div>
      </section>

      <dl class="ui-dl">
        <dt>{{ t('keys.lastUsedAt') }}</dt>
        <dd>{{ apiKey.last_used_at ? formatDateTime(apiKey.last_used_at) : t('personalUi.keys.never') }}</dd>
        <template v-if="apiKey.last_used_ip">
          <dt>{{ t('keys.lastUsedIP') }}</dt>
          <dd class="font-mono text-xs">{{ apiKey.last_used_ip }}</dd>
        </template>
        <dt>{{ t('keys.currentConcurrency') }}</dt>
        <dd class="tabular-nums">{{ apiKey.current_concurrency ?? 0 }}</dd>
        <dt>{{ t('keys.expiresAt') }}</dt>
        <dd :class="expired ? 'text-red-600' : ''">{{ apiKey.expires_at ? formatDateTime(apiKey.expires_at) : t('keys.noExpiration') }}</dd>
        <template v-if="ipRules">
          <dt>{{ t('personalUi.keys.ipRules') }}</dt>
          <dd class="font-mono text-xs">{{ ipRules }}</dd>
        </template>
        <dt>{{ t('keys.created') }}</dt>
        <dd>{{ formatDateTime(apiKey.created_at) }}</dd>
        <dt>{{ t('keys.id') }}</dt>
        <dd class="font-mono text-xs">#{{ apiKey.id }}</dd>
      </dl>
    </template>

    <template #footer>
      <button type="button" class="btn btn-primary btn-sm" @click="apiKey && emit('all-clients', apiKey)">
        <Icon name="terminal" size="sm" />
        {{ t('personalUi.keys.allClients') }}
      </button>
      <button type="button" class="btn btn-secondary btn-sm" @click="apiKey && emit('edit', apiKey)">
        <Icon name="edit" size="sm" />
        {{ t('common.edit') }}
      </button>
      <button
        v-if="!hideCcsImport"
        type="button"
        class="btn btn-ghost btn-sm"
        @click="apiKey && emit('import-ccs', apiKey)"
      >
        <Icon name="upload" size="sm" />
        {{ t('keys.importToCcSwitch') }}
      </button>
    </template>
  </SideDrawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import CountUp from '@/components/ui/CountUp.vue'
import QuotaMeter from '@/components/ui/QuotaMeter.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import EndpointPopover from './EndpointPopover.vue'
import CopyIconButton from './CopyIconButton.vue'
import { buildKeySnippets, defaultSnippetFor, gatewayRoot, type KeySnippetId } from './keySnippets'
import { maskApiKey } from '@/utils/maskApiKey'
import { formatDateTime } from '@/utils/format'
import type { ApiKey, CustomEndpoint } from '@/types'
import type { BatchApiKeyUsageStats } from '@/api/usage'

const props = defineProps<{
  open: boolean
  apiKey: ApiKey | null
  baseUrl: string
  customEndpoints: CustomEndpoint[]
  usage?: BatchApiKeyUsageStats
  hideCcsImport?: boolean
  formatReset: (resetAt: string | null) => string
}>()

const emit = defineEmits<{
  close: []
  edit: [key: ApiKey]
  'all-clients': [key: ApiKey]
  'import-ccs': [key: ApiKey]
  'reset-rate-limit': [key: ApiKey]
}>()

const { t } = useI18n()
const revealed = ref(false)
const snippetId = ref<KeySnippetId>('curl')

const resolvedBase = computed(() => props.baseUrl || window.location.origin)
const apiBase = computed(() => `${gatewayRoot(resolvedBase.value)}/v1`)
const snippets = computed(() => props.apiKey
  ? buildKeySnippets(resolvedBase.value, props.apiKey.key, props.apiKey.group?.platform)
  : [])
const activeSnippet = computed(() => snippets.value.find((item) => item.id === snippetId.value) ?? snippets.value[0] ?? { id: 'curl', label: '', code: '' })
// The visible code masks the key until revealed; the copy button always copies the real key.
const shownSnippet = computed(() => {
  const key = props.apiKey?.key
  if (!key || revealed.value) return activeSnippet.value.code
  return activeSnippet.value.code.split(key).join(maskApiKey(key))
})
const hasEndpoints = computed(() => props.customEndpoints.length > 0)

watch(() => [props.open, props.apiKey?.id], () => {
  revealed.value = false
  snippetId.value = defaultSnippetFor(props.apiKey?.group?.platform)
}, { immediate: true })

const statusBadge = computed(() => {
  switch (props.apiKey?.status) {
    case 'active': return 'badge-success'
    case 'quota_exhausted': return 'badge-warning'
    case 'expired': return 'badge-danger'
    default: return 'badge-gray'
  }
})

const expired = computed(() => !!props.apiKey?.expires_at && new Date(props.apiKey.expires_at) < new Date())

const ipRules = computed(() => {
  const key = props.apiKey
  if (!key) return ''
  const allow = key.ip_whitelist?.length ? `+ ${key.ip_whitelist.join(', ')}` : ''
  const deny = key.ip_blacklist?.length ? `− ${key.ip_blacklist.join(', ')}` : ''
  return [allow, deny].filter(Boolean).join('  ')
})

const windows = computed(() => {
  const key = props.apiKey
  if (!key) return []
  return ([
    ['5h', key.rate_limit_5h, key.usage_5h, key.reset_5h_at],
    ['1d', key.rate_limit_1d, key.usage_1d, key.reset_1d_at],
    ['7d', key.rate_limit_7d, key.usage_7d, key.reset_7d_at],
  ] as const)
    .filter(([, limit]) => limit > 0)
    .map(([id, limit, used, reset]) => ({ id, limit, used: used ?? 0, reset: props.formatReset(reset) }))
})

function remaining(used: number | null | undefined, limit: number) {
  return Math.max(0, 100 - ((used ?? 0) / limit) * 100)
}

const money = (value: number) => `$${value.toFixed(value >= 100 ? 2 : 4)}`
</script>

<style scoped>
.kd-mark {
  display: inline-grid;
  width: 2.25rem;
  height: 2.25rem;
  flex: none;
  place-items: center;
  border-radius: 0.625rem;
  background: var(--operator-accent-soft);
  color: var(--operator-foreground);
}

.kd-fields {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius-lg);
}

.kd-field {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
  padding: 0.375rem 0.375rem 0.375rem 0.75rem;
}

.kd-field + .kd-field {
  border-top: 1px solid var(--operator-border-subtle);
}

.kd-field-label {
  width: 4.5rem;
  flex: none;
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.kd-field-value {
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
  color: var(--operator-foreground);
  font-family: theme('fontFamily.mono');
  font-size: 0.8125rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kd-icon {
  width: 1.75rem;
  height: 1.75rem;
}

.kd-snippet {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.kd-snippet-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.kd-code {
  margin: 0;
  white-space: pre;
}

.kd-kpis {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin-bottom: 0;
}

.kd-limits {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.kd-limit-head {
  display: flex;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.25rem;
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
}
</style>
