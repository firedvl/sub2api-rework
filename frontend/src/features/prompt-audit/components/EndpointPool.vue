<template>
  <section aria-labelledby="prompt-pool-title" class="ui-card overflow-hidden">
    <header class="viz-card-head pb-3.5">
      <h2 id="prompt-pool-title" class="viz-card-title flex items-center">
        {{ t('admin.promptAudit.pool.title') }}
        <HelpTooltip :content="t('admin.promptAudit.pool.description')" />
      </h2>
      <span class="viz-count">{{ endpoints.length }}</span>
      <div class="viz-card-tools">
        <button type="button" class="btn btn-primary btn-sm" data-test="add-endpoint" @click="openCreate">
          <Icon name="plus" size="sm" />{{ t('admin.promptAudit.pool.add') }}
        </button>
      </div>
    </header>

    <div v-if="endpoints.length === 0" class="ui-empty border-t border-[var(--operator-border)]">
      <span class="ui-empty-icon"><Icon name="server" size="lg" /></span>
      <b>{{ t('admin.promptAudit.pool.empty') }}</b>
    </div>
    <div v-else class="ui-table-wrap border-t border-[var(--operator-border)]">
      <table class="ui-table">
        <thead>
          <tr>
            <th class="w-14"><span class="sr-only">{{ t('admin.promptAudit.saveBar.enabled') }}</span></th>
            <th>{{ t('admin.promptAudit.pool.node') }}</th>
            <th>{{ t('admin.promptAudit.pool.model') }}</th>
            <th>{{ t('admin.promptAudit.pool.limits') }}</th>
            <th>{{ t('admin.promptAudit.pool.credential') }}</th>
            <th><span class="sr-only">{{ t('admin.promptAudit.common.actions') }}</span></th>
          </tr>
        </thead>
        <tbody class="ui-rows-enter">
          <tr v-for="endpoint in endpoints" :key="endpoint.id" :data-test="`endpoint-${endpoint.id}`">
            <td>
              <button
                type="button"
                role="switch"
                :aria-checked="endpoint.enabled"
                :aria-label="t('admin.promptAudit.pool.toggleNode', { name: endpoint.name })"
                class="pa-switch"
                :class="endpoint.enabled ? 'is-on' : ''"
                @click="toggleEndpoint(endpoint.id)"
              >
                <span class="bg-white" />
              </button>
            </td>
            <td class="max-w-[18rem]">
              <div class="ui-cell-main truncate">{{ endpoint.name }}</div>
              <div class="ui-cell-sub truncate font-mono" :title="endpoint.base_url">{{ endpoint.base_url }}</div>
            </td>
            <td class="max-w-[14rem]"><span class="block truncate font-mono text-xs" :title="endpoint.model">{{ endpoint.model }}</span></td>
            <td class="whitespace-nowrap text-xs tabular-nums">
              <span class="badge badge-gray">{{ endpoint.timeout_ms }} ms</span>
              <span class="badge badge-gray ml-1">{{ endpoint.input_limit }} chars</span>
            </td>
            <td class="max-w-[18rem]">
              <span class="flex items-center gap-1.5 text-xs font-medium">
                <span :class="credentialInvalid(endpoint) ? 'ui-dot is-bad' : hasCredential(endpoint) ? 'ui-dot is-ok' : 'ui-dot'" />
                {{ credentialInvalid(endpoint) ? t('admin.promptAudit.pool.invalid') : hasCredential(endpoint) ? t('admin.promptAudit.pool.configured') : t('admin.promptAudit.pool.missing') }}
              </span>
              <p v-if="probingIds.includes(endpoint.id)" class="ui-cell-sub mt-1 text-primary-600 dark:text-primary-300">
                {{ t('admin.promptAudit.pool.probeProgress') }}
              </p>
              <p v-if="probeResults[endpoint.id]" class="mt-1 line-clamp-2 text-xs leading-5" :class="probeResults[endpoint.id].ok ? 'text-emerald-600 dark:text-emerald-300' : 'text-red-600 dark:text-red-300'" :title="probeResults[endpoint.id].message">
                {{ t('admin.promptAudit.pool.probeResult', { status: probeResults[endpoint.id].status, http: probeResults[endpoint.id].http_status || '—', latency: probeResults[endpoint.id].latency_ms }) }}
                · {{ probeResults[endpoint.id].message }}
              </p>
            </td>
            <td class="whitespace-nowrap">
              <div class="flex items-center justify-end gap-1">
                <button type="button" class="btn btn-secondary btn-sm" :disabled="probingIds.includes(endpoint.id)" @click="$emit('probe', endpoint)">
                  <Icon name="signal" size="sm" :class="probingIds.includes(endpoint.id) ? 'animate-pulse' : ''" />
                  {{ probingIds.includes(endpoint.id) ? t('admin.promptAudit.pool.probing') : t('admin.promptAudit.pool.probe') }}
                </button>
                <button type="button" class="app-icon-button" :aria-label="t('common.edit')" :title="t('common.edit')" @click="openEdit(endpoint)">
                  <Icon name="edit" size="sm" />
                </button>
                <button type="button" class="app-icon-button text-red-600 dark:text-red-400" :aria-label="t('common.delete')" :title="t('common.delete')" @click="removeEndpoint(endpoint)">
                  <Icon name="trash" size="sm" />
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <BaseDialog :show="Boolean(editing)" :title="editingIndex < 0 ? t('admin.promptAudit.pool.add') : t('admin.promptAudit.pool.edit')" width="wide" @close="closeEditor">
      <form v-if="editing" class="grid gap-4 sm:grid-cols-2" @submit.prevent="saveEditor">
        <label class="space-y-1 text-sm text-gray-700 dark:text-dark-200">
          <span>{{ t('admin.promptAudit.pool.name') }}</span>
          <input v-model="editing.name" class="input w-full" required :aria-label="t('admin.promptAudit.pool.name')" />
        </label>
        <label class="space-y-1 text-sm text-gray-700 dark:text-dark-200">
          <span>{{ t('admin.promptAudit.pool.id') }}</span>
          <input v-model="editing.id" class="input w-full" required :disabled="editingIndex >= 0" :aria-label="t('admin.promptAudit.pool.id')" />
        </label>
        <label class="space-y-1 text-sm text-gray-700 dark:text-dark-200 sm:col-span-2">
          <span>{{ t('admin.promptAudit.pool.baseUrl') }}</span>
          <input v-model="editing.base_url" class="input w-full" required inputmode="url" :aria-label="t('admin.promptAudit.pool.baseUrl')" />
        </label>
        <label class="space-y-1 text-sm text-gray-700 dark:text-dark-200 sm:col-span-2">
          <span>{{ t('admin.promptAudit.pool.apiKey') }}</span>
          <input v-model="editing.token" class="input w-full" type="password" autocomplete="new-password" :placeholder="editing.has_token ? (editing.token_status === 'invalid' ? t('admin.promptAudit.pool.reenterSecret') : t('admin.promptAudit.pool.keepSecret')) : ''" :aria-label="t('admin.promptAudit.pool.apiKey')" />
          <span class="block text-xs text-gray-500 dark:text-dark-400">{{ t('admin.promptAudit.pool.secretHint') }}</span>
        </label>
        <label v-if="editing.has_token" class="flex items-center gap-2 text-sm text-red-600 dark:text-red-300 sm:col-span-2">
          <input v-model="editing.clear_token" type="checkbox" :aria-label="t('admin.promptAudit.pool.clearSecret')" />
          {{ t('admin.promptAudit.pool.clearSecret') }}
        </label>
        <label class="space-y-1 text-sm text-gray-700 dark:text-dark-200 sm:col-span-2">
          <span>{{ t('admin.promptAudit.pool.model') }}</span>
          <input v-model="editing.model" class="input w-full" :aria-label="t('admin.promptAudit.pool.model')" />
        </label>
        <label class="space-y-1 text-sm text-gray-700 dark:text-dark-200">
          <span>{{ t('admin.promptAudit.pool.timeout') }}</span>
          <input v-model.number="editing.timeout_ms" class="input w-full" type="number" min="100" max="30000" required :aria-label="t('admin.promptAudit.pool.timeout')" />
        </label>
        <label class="space-y-1 text-sm text-gray-700 dark:text-dark-200">
          <span>{{ t('admin.promptAudit.pool.inputLimit') }}</span>
          <input v-model.number="editing.input_limit" class="input w-full" type="number" min="128" max="100000" required :aria-label="t('admin.promptAudit.pool.inputLimit')" />
        </label>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeEditor">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" data-test="save-endpoint" @click="saveEditor">{{ t('common.save') }}</button>
        </div>
      </template>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import type { PromptAuditEndpointDraft, PromptProbeResult } from '../types'
import { cloneData, createDefaultEndpoint } from '../viewModel'

const props = defineProps<{
  endpoints: PromptAuditEndpointDraft[]
  probeResults: Record<string, PromptProbeResult>
  probingIds: string[]
}>()
const emit = defineEmits<{
  (event: 'update:endpoints', value: PromptAuditEndpointDraft[]): void
  (event: 'probe', endpoint: PromptAuditEndpointDraft): void
}>()
const { t } = useI18n()
const editing = ref<PromptAuditEndpointDraft | null>(null)
const editingIndex = ref(-1)

function openCreate() {
  editingIndex.value = -1
  editing.value = createDefaultEndpoint(props.endpoints.length + 1)
}
function openEdit(endpoint: PromptAuditEndpointDraft) {
  editingIndex.value = props.endpoints.findIndex((item) => item.id === endpoint.id)
  editing.value = cloneData(endpoint)
}
function closeEditor() {
  editing.value = null
  editingIndex.value = -1
}
function saveEditor() {
  if (!editing.value?.id.trim() || !editing.value.name.trim() || !editing.value.base_url.trim()) return
  const next = props.endpoints.map((item) => cloneData(item))
  const value = cloneData(editing.value)
  if (value.token.trim()) value.clear_token = false
  if (editingIndex.value < 0) next.push(value)
  else next.splice(editingIndex.value, 1, value)
  emit('update:endpoints', next)
  closeEditor()
}
function toggleEndpoint(id: string) {
  emit('update:endpoints', props.endpoints.map((item) => item.id === id ? { ...item, enabled: !item.enabled } : cloneData(item)))
}
function removeEndpoint(endpoint: PromptAuditEndpointDraft) {
  if (!window.confirm(t('admin.promptAudit.pool.deleteConfirm', { name: endpoint.name }))) return
  emit('update:endpoints', props.endpoints.filter((item) => item.id !== endpoint.id).map((item) => cloneData(item)))
}
function hasCredential(endpoint: PromptAuditEndpointDraft): boolean {
  return Boolean(endpoint.token.trim() || (endpoint.has_token && !endpoint.clear_token))
}
function credentialInvalid(endpoint: PromptAuditEndpointDraft): boolean {
  return endpoint.token_status === 'invalid' && !endpoint.token.trim() && !endpoint.clear_token
}
</script>

<style scoped>
.pa-switch {
  position: relative;
  display: inline-flex;
  width: 2.25rem;
  height: 1.25rem;
  flex: none;
  align-items: center;
  padding: 2px;
  border-radius: 999px;
  background: var(--operator-border-strong);
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.pa-switch > span {
  width: 1rem;
  height: 1rem;
  border-radius: 999px;
  box-shadow: var(--operator-shadow-xs);
  transition: transform var(--operator-duration-fast) var(--operator-spring);
}

.pa-switch.is-on {
  background: var(--operator-accent);
}

.pa-switch.is-on > span {
  transform: translateX(1rem);
}

.pa-switch:focus-visible {
  outline: 2px solid var(--operator-accent-text);
  outline-offset: 2px;
}
</style>
