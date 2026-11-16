<template>
  <AppLayout>
    <div class="ui-page ui-stagger">
      <div v-if="loadErrors.config && !draft" role="alert" class="ui-card">
        <div class="ui-empty">
          <span class="ui-empty-icon"><Icon name="exclamationTriangle" size="lg" /></span>
          <b>{{ loadErrors.config }}</b>
          <button type="button" class="btn btn-secondary btn-sm" @click="loadConfig">{{ t('admin.promptAudit.actions.retry') }}</button>
        </div>
      </div>

      <template v-else>
        <div class="ui-toolbar">
          <div class="viz-seg" role="group" :aria-label="t('admin.promptAudit.title')">
            <button
              v-for="tab in pageTabs"
              :key="tab.id"
              type="button"
              :aria-pressed="activeTab === tab.id"
              :data-test="`tab-${tab.id}`"
              @click="activeTab = tab.id"
            >
              <Icon :name="tab.icon" size="sm" />
              {{ tab.label }}
            </button>
          </div>
          <div v-show="activeTab === 'config'" class="viz-seg" role="group" :aria-label="t('admin.adminUi.promptAudit.sections')">
            <button
              v-for="section in configSections"
              :key="section.id"
              type="button"
              :aria-pressed="configSection === section.id"
              :data-test="`section-${section.id}`"
              @click="configSection = section.id"
            >
              {{ section.label }}
            </button>
          </div>
          <span class="ui-grow" />
          <span v-if="runtime" class="pa-pill" :title="t(`admin.promptAudit.mode.${runtime.effective_mode}`)">
            <span :class="processDot" />
            {{ t(`admin.promptAudit.status.${runtime.process_status}`) }}
          </span>
          <span
            v-if="draft"
            class="pa-pill font-mono"
            :title="draft.updated_at ? `${t('admin.promptAudit.configVersion', { version: draft.config_version })} · ${formatDate(draft.updated_at)}` : t('admin.promptAudit.configVersion', { version: draft.config_version })"
          >
            v{{ draft.config_version }}
          </span>
        </div>

        <div v-show="activeTab === 'events'" class="flex flex-col gap-4" data-test="tab-panel-events">
          <div
            v-if="draft?.enabled && !draft.store_pass_events"
            data-test="pass-events-disabled-notice"
            role="status"
            class="pa-notice"
          >
            <span class="ui-dot is-warn" />
            <span class="min-w-0 flex-1">{{ t('admin.promptAudit.events.passEventsDisabled') }}</span>
            <button type="button" class="btn btn-ghost btn-sm" @click="activeTab = 'config'">
              {{ t('admin.promptAudit.events.openConfiguration') }}
              <Icon name="arrowRight" size="sm" />
            </button>
          </div>
          <EventWorkspace
            :events="events.items"
            :total="events.total"
            :page="events.page"
            :page-size="events.page_size"
            :filters="filters"
            :selected-ids="selectedEventIds"
            :loading="loading.events"
            :error="loadErrors.events"
            @filters-change="handleFiltersChanged"
            @search="applyEventFilters"
            @selection="selectedEventIds = $event"
            @page="changePage"
            @page-size="changePageSize"
            @view="openEvent"
            @delete="requestSingleDelete"
            @batch-delete="requestBatchDelete"
            @preview-delete="requestFilterDeletePreview"
          />
        </div>

        <div v-show="activeTab === 'config'" class="flex flex-col gap-4" data-test="tab-panel-config">
          <div v-show="configSection === 'runtime'">
            <RuntimeOverview :runtime="runtime" :loading="loading.runtime" :error="loadErrors.runtime" @refresh="loadRuntime" />
          </div>

          <template v-if="draft">
            <div v-show="configSection === 'nodes'">
              <EndpointPool
                :endpoints="draft.endpoints"
                :probe-results="probeResults"
                :probing-ids="probingIds"
                @update:endpoints="updateEndpoints"
                @probe="runProbe"
              />
            </div>
            <div v-show="configSection === 'policy'" class="flex flex-col gap-4">
              <div v-if="loadErrors.groups" role="alert" class="pa-notice">
                <span class="ui-dot is-warn" />{{ loadErrors.groups }}
              </div>
              <PolicyPanel :draft="draft" :groups="groups" @update:draft="replaceDraft" />
            </div>
          </template>

          <div v-if="draft && activeTab === 'config'" class="ui-card pa-savebar" data-test="save-bar">
            <div class="flex flex-wrap items-center gap-x-5 gap-y-2">
              <SaveToggle :label="t('admin.promptAudit.saveBar.enabled')" :model-value="draft.enabled" data-test="enabled-toggle" @update:model-value="setEnabled" />
              <SaveToggle :label="t('admin.promptAudit.saveBar.blocking')" :model-value="draft.blocking_enabled" :disabled="!draft.enabled" data-test="blocking-toggle" @update:model-value="setBlocking" />
              <SaveToggle :label="t('admin.promptAudit.saveBar.blockingLatestTurnOnly')" :model-value="draft.blocking_latest_turn_only" :disabled="!draft.enabled || !draft.blocking_enabled" data-test="blocking-latest-turn-only-toggle" @update:model-value="replaceDraft({ ...draft!, blocking_latest_turn_only: $event })" />
              <SaveToggle :label="t('admin.promptAudit.saveBar.storePass')" :model-value="draft.store_pass_events" data-test="store-pass-toggle" @update:model-value="replaceDraft({ ...draft!, store_pass_events: $event })" />
            </div>
            <div class="flex items-center gap-2">
              <span class="flex items-center gap-1.5 text-xs" :class="dirty ? 'text-amber-700 dark:text-amber-300' : 'text-gray-500 dark:text-dark-400'">
                <span :class="dirty ? 'ui-dot is-warn' : 'ui-dot is-ok'" />
                {{ dirty ? t('admin.promptAudit.saveBar.dirty') : t('admin.promptAudit.saveBar.synced') }}
              </span>
              <button type="button" class="btn btn-ghost btn-sm" :disabled="!dirty || loading.saving" @click="resetDraft">{{ t('common.reset') }}</button>
              <button type="button" class="btn btn-primary btn-sm" :disabled="!dirty || loading.saving" data-test="save-config" @click="saveConfig">
                {{ loading.saving ? t('common.saving') : t('common.save') }}
              </button>
            </div>
          </div>
        </div>
      </template>
    </div>

    <ConfirmDialog
      :show="showBlockingConfirmation"
      :title="t('admin.promptAudit.blockingConfirm.title')"
      :message="t('admin.promptAudit.blockingConfirm.message')"
      :confirm-text="t('admin.promptAudit.blockingConfirm.confirm')"
      danger
      @confirm="confirmBlocking"
      @cancel="showBlockingConfirmation = false"
    />
    <ConfirmDialog
      :show="deleteRequest.mode !== ''"
      :title="t('admin.promptAudit.events.deleteConfirmTitle')"
      :message="t('admin.promptAudit.events.deleteConfirmMessage', { count: deleteRequest.ids.length })"
      :confirm-text="t('common.delete')"
      danger
      @confirm="confirmIDDelete"
      @cancel="clearDeleteRequest"
    />
    <FilterDeleteDialog
      :show="showFilterDelete"
      :initial-filters="filters"
      :preview="deletePreview"
      :previewing="loading.previewing"
      :deleting="loading.deleting"
      @close="closeFilterDelete"
      @preview="runFilterDeletePreview"
      @confirm="confirmFilterDelete"
      @criteria-change="clearDeletePreview"
    />
    <EventDetailDialog :show="showEventDetail" :event="activeEvent" :loading="loading.detail" @close="closeEventDetail" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'
import RuntimeOverview from './components/RuntimeOverview.vue'
import EndpointPool from './components/EndpointPool.vue'
import PolicyPanel from './components/PolicyPanel.vue'
import EventWorkspace from './components/EventWorkspace.vue'
import EventDetailDialog from './components/EventDetailDialog.vue'
import FilterDeleteDialog from './components/FilterDeleteDialog.vue'
import promptAuditAPI from './api'
import type {
  PromptAuditDraft,
  PromptAuditEndpointDraft,
  PromptAuditEvent,
  PromptAuditGroup,
  PromptAuditRuntime,
  PromptDeletePreview,
  PromptEventFilters,
  PromptEventPage,
  PromptLoadErrors,
  PromptProbeResult,
} from './types'
import { buildUpdateRequest, cloneData, configToDraft, draftFingerprint, emptyEventFilters } from './viewModel'

const { t, locale } = useI18n()
const appStore = useAppStore()
type PromptAuditPageTab = 'config' | 'events'
const activeTab = ref<PromptAuditPageTab>('events')
const pageTabs = computed(() => [
  { id: 'events' as const, label: t('admin.promptAudit.tabs.events'), icon: 'shield' as const },
  { id: 'config' as const, label: t('admin.promptAudit.tabs.config'), icon: 'cog' as const },
])
type PromptAuditConfigSection = 'runtime' | 'nodes' | 'policy'
const configSection = ref<PromptAuditConfigSection>('runtime')
const configSections = computed(() => [
  { id: 'runtime' as const, label: t('admin.adminUi.promptAudit.runtime') },
  { id: 'nodes' as const, label: t('admin.adminUi.promptAudit.nodes') },
  { id: 'policy' as const, label: t('admin.adminUi.promptAudit.policy') },
])
const serverConfig = ref<PromptAuditDraft | null>(null)
const draft = ref<PromptAuditDraft | null>(null)
const runtime = ref<PromptAuditRuntime | null>(null)
const groups = ref<PromptAuditGroup[]>([])
const events = reactive<PromptEventPage>({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
const filters = ref<PromptEventFilters>(emptyEventFilters())
const appliedFilters = ref<PromptEventFilters>(emptyEventFilters())
const selectedEventIds = ref<number[]>([])
const activeEvent = ref<PromptAuditEvent | null>(null)
const showEventDetail = ref(false)
const probeResults = reactive<Record<string, PromptProbeResult>>({})
const probingIds = ref<string[]>([])
const showFilterDelete = ref(false)
const deletePreview = ref<PromptDeletePreview | null>(null)
const deletePreviewFilters = ref<PromptEventFilters | null>(null)
const showBlockingConfirmation = ref(false)
const deleteRequest = reactive<{ mode: '' | 'single' | 'batch'; ids: number[] }>({ mode: '', ids: [] })
const loading = reactive({ config: false, runtime: false, groups: false, events: false, saving: false, detail: false, deleting: false, previewing: false })
const loadErrors = reactive<PromptLoadErrors>({ config: '', runtime: '', groups: '', events: '' })
const processDot = computed(() => {
  const status = runtime.value?.process_status
  if (status === 'running') return 'ui-dot is-ok'
  if (status === 'disabled') return 'ui-dot'
  if (status === 'degraded') return 'ui-dot is-warn'
  return 'ui-dot is-bad'
})
const dirty = computed(() => draftFingerprint(draft.value) !== draftFingerprint(serverConfig.value))

const SaveToggle = defineComponent({
  inheritAttrs: false,
  props: { label: { type: String, required: true }, modelValue: { type: Boolean, required: true }, disabled: { type: Boolean, default: false } },
  emits: ['update:modelValue'],
  setup(props, { emit, attrs }) {
    return () => h('label', { class: ['flex items-center gap-2 text-sm', props.disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer'] }, [
      h('button', {
        ...attrs,
        type: 'button',
        role: 'switch',
        'aria-checked': props.modelValue,
        'aria-label': props.label,
        disabled: props.disabled,
        class: [
          'relative inline-flex h-6 w-11 shrink-0 items-center rounded-full border-2 border-transparent transition-colors duration-200 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2',
          props.modelValue ? 'bg-primary-600' : 'bg-gray-300 dark:bg-dark-600',
          props.disabled ? 'cursor-not-allowed' : 'cursor-pointer',
        ],
        onClick: (event: MouseEvent) => {
          event.preventDefault()
          if (!props.disabled) emit('update:modelValue', !props.modelValue)
        },
      }, [
        h('span', {
          class: [
            'pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transition-transform duration-200 ease-in-out',
            props.modelValue ? 'translate-x-5' : 'translate-x-0',
          ],
        }),
      ]),
      h('span', { class: 'select-none text-gray-700 dark:text-dark-200' }, props.label),
    ])
  },
})

function errorMessage(error: unknown, fallbackKey: string): string {
  const code = extractApiErrorCode(error)
  if (code) {
    const key = `admin.promptAudit.errors.${code}`
    const translated = t(key)
    if (translated !== key) return translated
  }
  return extractApiErrorMessage(error, t(fallbackKey))
}

async function loadConfig() {
  loading.config = true
  loadErrors.config = ''
  try {
    const config = await promptAuditAPI.getConfig()
    serverConfig.value = configToDraft(config)
    draft.value = configToDraft(config)
  } catch (error) {
    loadErrors.config = errorMessage(error, 'admin.promptAudit.errors.loadConfig')
  } finally {
    loading.config = false
  }
}
async function loadRuntime() {
  loading.runtime = true
  loadErrors.runtime = ''
  try { runtime.value = await promptAuditAPI.getRuntime() }
  catch (error) { loadErrors.runtime = errorMessage(error, 'admin.promptAudit.errors.loadRuntime') }
  finally { loading.runtime = false }
}
async function loadGroups() {
  loading.groups = true
  loadErrors.groups = ''
  try { groups.value = await promptAuditAPI.listGroups() }
  catch (error) { loadErrors.groups = errorMessage(error, 'admin.promptAudit.errors.loadGroups') }
  finally { loading.groups = false }
}
async function loadEvents() {
  loading.events = true
  loadErrors.events = ''
  try {
    const result = await promptAuditAPI.listEvents(appliedFilters.value, events.page, events.page_size)
    Object.assign(events, result)
    selectedEventIds.value = []
  } catch (error) {
    loadErrors.events = errorMessage(error, 'admin.promptAudit.errors.loadEvents')
  } finally {
    loading.events = false
  }
}
async function loadInitial() {
  await Promise.allSettled([loadConfig(), loadRuntime(), loadGroups(), loadEvents()])
}

function replaceDraft(value: PromptAuditDraft) { draft.value = cloneData(value) }
function updateEndpoints(value: PromptAuditEndpointDraft[]) {
  if (!draft.value) return
  replaceDraft({ ...draft.value, endpoints: value })
}
function setEnabled(value: boolean) {
  if (!draft.value) return
  replaceDraft({ ...draft.value, enabled: value, blocking_enabled: value ? draft.value.blocking_enabled : false })
}
function setBlocking(value: boolean) {
  if (!draft.value || !draft.value.enabled) return
  if (value && !draft.value.blocking_enabled) { showBlockingConfirmation.value = true; return }
  replaceDraft({ ...draft.value, blocking_enabled: value })
}
function confirmBlocking() {
  showBlockingConfirmation.value = false
  if (draft.value) replaceDraft({ ...draft.value, blocking_enabled: true })
}
function resetDraft() {
  if (serverConfig.value) draft.value = cloneData(serverConfig.value)
}
async function saveConfig() {
  if (!draft.value || !dirty.value) return
  loading.saving = true
  try {
    const saved = await promptAuditAPI.updateConfig(buildUpdateRequest(draft.value))
    serverConfig.value = configToDraft(saved)
    draft.value = configToDraft(saved)
    appStore.showSuccess(t('admin.promptAudit.messages.saved'))
    await loadRuntime()
  } catch (error) {
    const code = extractApiErrorCode(error)
    appStore.showError(errorMessage(error, code === 'prompt_audit_config_conflict' ? 'admin.promptAudit.errors.prompt_audit_config_conflict' : 'admin.promptAudit.errors.saveConfig'))
  } finally {
    loading.saving = false
  }
}
async function runProbe(endpoint: PromptAuditEndpointDraft) {
  if (probingIds.value.includes(endpoint.id)) return
  probingIds.value = [...probingIds.value, endpoint.id]
  try {
    const result = await promptAuditAPI.probeEndpoint(endpoint)
    probeResults[endpoint.id] = result
    if (result.ok) appStore.showSuccess(t('admin.promptAudit.messages.probeSucceeded'))
    else appStore.showError(`${result.error_code || result.status}: ${result.message}`)
  } catch (error) {
    appStore.showError(errorMessage(error, 'admin.promptAudit.errors.probe'))
  } finally {
    probingIds.value = probingIds.value.filter((id) => id !== endpoint.id)
  }
}

function handleFiltersChanged(value: PromptEventFilters) {
  filters.value = cloneData(value)
  clearDeletePreview()
}
function applyEventFilters(value: PromptEventFilters) {
  filters.value = cloneData(value)
  appliedFilters.value = cloneData(value)
  events.page = 1
  clearDeletePreview()
  void loadEvents()
}
function changePage(value: number) { events.page = value; void loadEvents() }
function changePageSize(value: number) { events.page_size = value; events.page = 1; void loadEvents() }
async function openEvent(id: number) {
  showEventDetail.value = true
  loading.detail = true
  activeEvent.value = null
  try { activeEvent.value = await promptAuditAPI.getEvent(id) }
  catch (error) { appStore.showError(errorMessage(error, 'admin.promptAudit.errors.loadDetail')); showEventDetail.value = false }
  finally { loading.detail = false }
}
function closeEventDetail() { showEventDetail.value = false; activeEvent.value = null }
function requestSingleDelete(id: number) { deleteRequest.mode = 'single'; deleteRequest.ids = [id] }
function requestBatchDelete() { if (selectedEventIds.value.length) { deleteRequest.mode = 'batch'; deleteRequest.ids = [...selectedEventIds.value] } }
function clearDeleteRequest() { deleteRequest.mode = ''; deleteRequest.ids = [] }
async function confirmIDDelete() {
  const mode = deleteRequest.mode
  const ids = [...deleteRequest.ids]
  clearDeleteRequest()
  if (!mode || ids.length === 0) return
  loading.deleting = true
  try {
    const result = mode === 'single' ? await promptAuditAPI.deleteEvent(ids[0]) : await promptAuditAPI.batchDeleteEvents(ids)
    appStore.showSuccess(t('admin.promptAudit.messages.deleted', { count: result.deleted_events }))
    await Promise.allSettled([loadEvents(), loadRuntime()])
  } catch (error) { appStore.showError(errorMessage(error, 'admin.promptAudit.errors.delete')) }
  finally { loading.deleting = false }
}
function clearDeletePreview() {
  deletePreview.value = null
  deletePreviewFilters.value = null
}
function requestFilterDeletePreview() {
  clearDeletePreview()
  showFilterDelete.value = true
}
function closeFilterDelete() {
  showFilterDelete.value = false
  clearDeletePreview()
}
async function runFilterDeletePreview(value: PromptEventFilters) {
  loading.previewing = true
  try {
    deletePreview.value = await promptAuditAPI.previewDelete(value)
    deletePreviewFilters.value = cloneData(value)
  } catch (error) {
    clearDeletePreview()
    appStore.showError(errorMessage(error, 'admin.promptAudit.errors.previewDelete'))
  } finally { loading.previewing = false }
}
async function confirmFilterDelete(filters?: PromptEventFilters) {
  if (loading.deleting) return
  loading.deleting = true
  try {
    let preview = deletePreview.value
    let previewFilters = deletePreviewFilters.value ? cloneData(deletePreviewFilters.value) : null
    // One-click path: no fresh preview (never requested, or cleared by a
    // criteria change) — mint the confirmation token on the fly from the
    // criteria the dialog just emitted, then delete in the same action.
    if ((!preview || !previewFilters) && filters) {
      preview = await promptAuditAPI.previewDelete(filters)
      previewFilters = cloneData(filters)
    }
    if (!preview || !previewFilters) return
    const result = await promptAuditAPI.deleteEventsByFilter(previewFilters, preview)
    closeFilterDelete()
    appStore.showSuccess(t('admin.promptAudit.messages.deleted', { count: result.deleted_events }))
    await Promise.allSettled([loadEvents(), loadRuntime()])
  } catch (error) {
    clearDeletePreview()
    appStore.showError(errorMessage(error, 'admin.promptAudit.errors.deleteConfirmation'))
  } finally { loading.deleting = false }
}
function formatDate(value: string): string {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value))
}

onMounted(loadInitial)
</script>

<style scoped>
.pa-pill {
  display: inline-flex;
  height: 2rem;
  align-items: center;
  gap: 0.375rem;
  padding: 0 0.625rem;
  border: 1px solid var(--operator-border);
  border-radius: 999px;
  background: var(--operator-card);
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
  font-weight: 500;
  white-space: nowrap;
}

.pa-notice {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 0.75rem;
  padding: 0.5rem 0.5rem 0.5rem 0.875rem;
  border: 1px solid color-mix(in srgb, var(--operator-warning) 35%, transparent);
  border-radius: var(--operator-radius);
  background: color-mix(in srgb, var(--operator-warning) 8%, transparent);
  color: var(--operator-foreground);
  font-size: 0.8125rem;
}

.pa-savebar {
  position: sticky;
  bottom: 0.75rem;
  z-index: 20;
  flex-direction: row;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem 1rem;
  padding: 0.75rem 1rem;
  background: color-mix(in srgb, var(--operator-card) 92%, transparent);
  box-shadow: var(--operator-shadow-md, var(--operator-shadow-xs));
  backdrop-filter: blur(8px);
}

@media (max-width: 1023px) {
  .pa-savebar {
    bottom: calc(var(--operator-tabbar-height) + 0.5rem);
  }
}
</style>
