<template>
  <section aria-labelledby="prompt-events-title" class="ui-card">
    <h2 id="prompt-events-title" class="sr-only">{{ t('admin.promptAudit.events.title') }}</h2>
    <form class="ui-toolbar pa-events-toolbar" role="toolbar" :aria-label="t('admin.promptAudit.events.title')" @submit.prevent="applyFilters">
      <label class="ui-search">
        <Icon name="search" size="sm" />
        <input
          v-model="localFilters.keyword"
          type="search"
          :placeholder="t('admin.adminUi.promptAudit.searchPrompts')"
          :aria-label="t('admin.promptAudit.events.keyword')"
          @change="filtersChanged"
        />
      </label>
      <div class="viz-seg max-w-full overflow-x-auto" role="group" :aria-label="t('admin.promptAudit.events.decision')">
        <button
          v-for="option in decisionOptions"
          :key="option.id"
          type="button"
          :aria-pressed="localFilters.decision === option.id"
          :data-test="`decision-${option.id || 'all'}`"
          @click="setDecision(option.id)"
        >
          {{ option.label }}
        </button>
      </div>
      <ToolbarPopover :label="t('admin.adminUi.filters')" icon="filter" :count="activeFilterCount" width="24rem" test-id="event-filters">
        <div class="grid gap-3 sm:grid-cols-2">
          <label class="pa-field">
            <span>{{ t('admin.promptAudit.events.risk') }}</span>
            <select v-model="localFilters.risk_level" class="input w-full" :aria-label="t('admin.promptAudit.events.risk')" @change="filtersChanged">
              <option value="">{{ t('common.all') }}</option>
              <option value="low">{{ t('admin.promptAudit.riskLevels.low') }}</option>
              <option value="medium">{{ t('admin.promptAudit.riskLevels.medium') }}</option>
              <option value="high">{{ t('admin.promptAudit.riskLevels.high') }}</option>
              <option value="critical">{{ t('admin.promptAudit.riskLevels.critical') }}</option>
            </select>
          </label>
          <FilterInput v-model="localFilters.endpoint" :label="t('admin.promptAudit.events.endpoint')" @change="filtersChanged" />
          <FilterInput v-model="localFilters.group_id" :label="t('admin.promptAudit.events.groupId')" type="number" @change="filtersChanged" />
          <FilterInput v-model="localFilters.user_id" :label="t('admin.promptAudit.events.userId')" type="number" @change="filtersChanged" />
          <FilterInput v-model="localFilters.api_key_id" :label="t('admin.promptAudit.events.apiKeyId')" type="number" @change="filtersChanged" />
          <FilterInput v-model="localFilters.request_id" :label="t('admin.promptAudit.events.requestId')" @change="filtersChanged" />
          <FilterInput v-model="localFilters.prompt_hash" class="sm:col-span-2" :label="t('admin.promptAudit.events.promptHash')" @change="filtersChanged" />
          <label class="pa-field">
            <span>{{ t('admin.promptAudit.events.startAt') }}</span>
            <input v-model="localFilters.start_at" type="datetime-local" class="input w-full" :aria-label="t('admin.promptAudit.events.startAt')" @change="filtersChanged" />
          </label>
          <label class="pa-field">
            <span>{{ t('admin.promptAudit.events.endAt') }}</span>
            <input v-model="localFilters.end_at" type="datetime-local" class="input w-full" :aria-label="t('admin.promptAudit.events.endAt')" @change="filtersChanged" />
          </label>
          <div class="flex justify-end gap-2 sm:col-span-2">
            <button type="button" class="btn btn-ghost btn-sm" @click="resetFilters">{{ t('common.reset') }}</button>
            <button type="button" class="btn btn-primary btn-sm" data-test="apply-filters" @click="applyFilters">{{ t('common.apply') }}</button>
          </div>
        </div>
      </ToolbarPopover>
      <span class="ui-grow" />
      <Transition name="fade">
        <button v-if="selectedIds.length" type="button" class="btn btn-danger btn-sm" data-test="batch-delete" @click="$emit('batch-delete')">
          <Icon name="trash" size="sm" />
          {{ t('admin.promptAudit.events.deleteSelected', { count: selectedIds.length }) }}
        </button>
      </Transition>
      <button type="button" class="app-icon-button" :disabled="loading" :aria-label="t('common.refresh')" :title="t('common.refresh')" @click="applyFilters">
        <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
      </button>
      <ToolbarPopover :label="t('admin.adminUi.moreActions')" icon="more" icon-only menu test-id="events-more">
        <button type="button" role="menuitem" class="dropdown-item text-red-600 dark:text-red-400" data-test="filter-delete" @click="$emit('preview-delete')">
          <Icon name="trash" size="sm" />{{ t('admin.promptAudit.events.deleteByFilter') }}
        </button>
      </ToolbarPopover>
    </form>

    <div v-if="error" role="alert" class="pa-alert">{{ error }}</div>
    <div class="ui-table-wrap overflow-x-auto">
      <table class="ui-table">
        <thead>
          <tr>
            <th class="w-10"><input type="checkbox" :checked="allSelected" :aria-label="t('admin.promptAudit.events.selectAll')" @change="toggleAll" /></th>
            <th>{{ t('admin.promptAudit.events.time') }}</th>
            <th>{{ t('admin.promptAudit.events.user') }}</th>
            <th>{{ t('admin.promptAudit.events.group') }}</th>
            <th>{{ t('admin.promptAudit.events.route') }}</th>
            <th>{{ t('admin.promptAudit.events.result') }}</th>
            <th class="hidden xl:table-cell">{{ t('admin.promptAudit.events.preview') }}</th>
            <th><span class="sr-only">{{ t('admin.promptAudit.common.actions') }}</span></th>
          </tr>
        </thead>
        <tbody v-if="loading && events.length === 0">
          <tr v-for="index in 4" :key="index"><td colspan="8"><div class="skeleton h-8" /></td></tr>
        </tbody>
        <tbody v-else-if="events.length === 0">
          <tr>
            <td colspan="8">
              <div class="ui-empty">
                <span class="ui-empty-icon"><Icon name="shield" size="lg" /></span>
                <b>{{ t('admin.promptAudit.events.empty') }}</b>
              </div>
            </td>
          </tr>
        </tbody>
        <tbody v-else class="ui-rows-enter" :aria-busy="loading">
          <tr
            v-for="event in events"
            :key="event.id"
            :data-test="`event-${event.id}`"
            class="is-clickable"
            :class="selectedIds.includes(event.id) ? 'is-selected' : ''"
            tabindex="0"
            @click="$emit('view', event.id)"
            @keydown.enter.self="$emit('view', event.id)"
          >
            <td @click.stop><input type="checkbox" :checked="selectedIds.includes(event.id)" :aria-label="t('admin.promptAudit.events.selectEvent', { id: event.id })" @change="toggleOne(event.id)" /></td>
            <td class="whitespace-nowrap text-xs">{{ formatDate(event.created_at) }}</td>
            <td class="max-w-[14rem]">
              <div class="ui-cell-main truncate">{{ event.snapshot.username || '—' }}</div>
              <div class="ui-cell-sub truncate" :title="identitySub(event)">{{ identitySub(event) }}</div>
            </td>
            <td class="whitespace-nowrap">{{ event.snapshot.group_name || '—' }}</td>
            <td class="max-w-[16rem]">
              <div class="ui-cell-main flex min-w-0 items-center gap-1.5">
                <ProviderMark v-if="event.snapshot.provider" :platform="event.snapshot.provider" :size="14" :label="event.snapshot.provider" />
                <span class="truncate font-mono text-xs">{{ event.snapshot.endpoint }}</span>
              </div>
              <div class="ui-cell-sub truncate">{{ event.snapshot.model }} · {{ event.snapshot.protocol }} · {{ event.snapshot.stage || 'http' }}</div>
            </td>
            <td>
              <span class="badge whitespace-nowrap" :class="decisionClass(event.decision)">{{ formatDecisionRisk(event.decision, event.risk_level) }}</span>
              <div class="ui-cell-sub mt-1 max-w-48 truncate" :title="formatCategories(event.categories)">{{ formatCategories(event.categories) }}</div>
            </td>
            <td class="hidden max-w-xs xl:table-cell"><p class="line-clamp-2 break-words text-xs text-gray-600 dark:text-dark-300">{{ event.snapshot.redacted_preview || '—' }}</p></td>
            <td class="whitespace-nowrap text-right" @click.stop>
              <button type="button" class="app-icon-button" :aria-label="t('common.view')" :title="t('common.view')" @click="$emit('view', event.id)">
                <Icon name="eye" size="sm" />
              </button>
              <button type="button" class="app-icon-button text-red-600 dark:text-red-400" :aria-label="t('common.delete')" :title="t('common.delete')" @click="$emit('delete', event.id)">
                <Icon name="trash" size="sm" />
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <Pagination :total="total" :page="page" :page-size="pageSize" @update:page="$emit('page', $event)" @update:page-size="$emit('page-size', $event)" />
  </section>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Pagination from '@/components/common/Pagination.vue'
import ToolbarPopover from '@/components/common/ToolbarPopover.vue'
import Icon from '@/components/icons/Icon.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import type { PromptAuditEvent, PromptEventFilters } from '../types'
import { cloneData, emptyEventFilters, SCANNER_CATALOG } from '../viewModel'

const props = defineProps<{
  events: PromptAuditEvent[]; total: number; page: number; pageSize: number
  filters: PromptEventFilters; selectedIds: number[]; loading: boolean; error: string
}>()
const emit = defineEmits<{
  (event: 'filters-change', value: PromptEventFilters): void
  (event: 'search', value: PromptEventFilters): void
  (event: 'selection', value: number[]): void
  (event: 'page', value: number): void
  (event: 'page-size', value: number): void
  (event: 'view', id: number): void
  (event: 'delete', id: number): void
  (event: 'batch-delete'): void
  (event: 'preview-delete'): void
}>()
const { t, locale } = useI18n()
const localFilters = reactive<PromptEventFilters>(cloneData(props.filters))
watch(() => props.filters, (value) => Object.assign(localFilters, cloneData(value)), { deep: true })
const allSelected = computed(() => props.events.length > 0 && props.events.every((event) => props.selectedIds.includes(event.id)))

const FilterInput = defineComponent({
  props: { modelValue: { type: String, required: true }, label: { type: String, required: true }, type: { type: String, default: 'text' } },
  emits: ['update:modelValue', 'change'],
  setup(componentProps, { emit: componentEmit }) {
    return () => h('label', { class: 'pa-field' }, [
      h('span', componentProps.label),
      h('input', {
        value: componentProps.modelValue, type: componentProps.type, class: 'input w-full', 'aria-label': componentProps.label,
        onInput: (event: Event) => componentEmit('update:modelValue', (event.target as HTMLInputElement).value),
        onChange: () => componentEmit('change'),
      }),
    ])
  },
})

const decisionOptions = computed(() => [
  { id: '', label: t('common.all') },
  { id: 'pass', label: t('admin.promptAudit.decisions.pass') },
  { id: 'flag', label: t('admin.promptAudit.decisions.flag') },
  { id: 'critical', label: t('admin.promptAudit.decisions.critical') },
])
// Fields that live in the Filters popover; keyword and decision sit in the toolbar.
const POPOVER_FILTER_KEYS = ['risk_level', 'endpoint', 'group_id', 'user_id', 'api_key_id', 'request_id', 'prompt_hash', 'start_at', 'end_at'] as const
const activeFilterCount = computed(() => POPOVER_FILTER_KEYS.filter((key) => String(props.filters[key] ?? '').trim() !== '').length)

function setDecision(value: string) {
  localFilters.decision = value
  applyFilters()
}
function identitySub(event: PromptAuditEvent): string {
  return [event.snapshot.user_email, event.snapshot.api_key_name].filter(Boolean).join(' · ') || '—'
}

function filtersChanged() {
  emit('filters-change', cloneData(localFilters))
}
function applyFilters() {
  const value = cloneData(localFilters)
  emit('filters-change', value)
  emit('search', value)
}
function resetFilters() {
  Object.assign(localFilters, emptyEventFilters())
  applyFilters()
}
function toggleOne(id: number) {
  const selected = new Set(props.selectedIds)
  if (selected.has(id)) selected.delete(id)
  else selected.add(id)
  emit('selection', [...selected])
}
function toggleAll() {
  emit('selection', allSelected.value ? [] : props.events.map((event) => event.id))
}
function formatDate(value: string): string {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(value))
}
function decisionClass(decision: string): string {
  if (decision === 'critical') return 'badge-danger'
  if (decision === 'flag') return 'badge-warning'
  return 'badge-success'
}
const DECISIONS = new Set(['pass', 'flag', 'critical'])
const RISK_LEVELS = new Set(['low', 'medium', 'high', 'critical'])

function translateDecision(decision: string): string {
  return DECISIONS.has(decision) ? t(`admin.promptAudit.decisions.${decision}`) : decision
}
function translateRiskLevel(riskLevel: string): string {
  return RISK_LEVELS.has(riskLevel) ? t(`admin.promptAudit.riskLevels.${riskLevel}`) : riskLevel
}
function translateCategory(category: string): string {
  return SCANNER_CATALOG.some((scanner) => scanner.id === category)
    ? t(`admin.promptAudit.scanners.${category}`)
    : category
}
function formatDecisionRisk(decision: string, riskLevel: string): string {
  return `${translateDecision(decision)} · ${translateRiskLevel(riskLevel)}`
}
function formatCategories(categories: string[]): string {
  if (!categories.length) return '—'
  return categories.map(translateCategory).join(', ')
}
</script>

<style scoped>
.pa-events-toolbar {
  padding: 0.875rem 1rem;
  border-bottom: 1px solid var(--operator-border);
}

.pa-alert {
  margin: 0.75rem 1rem 0;
  padding: 0.625rem 0.875rem;
  border-radius: var(--operator-radius);
  background: color-mix(in srgb, var(--operator-destructive) 10%, transparent);
  color: var(--operator-destructive);
  font-size: 0.875rem;
}

:deep(.pa-field) {
  display: grid;
  gap: 0.25rem;
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
  font-weight: 500;
}
</style>
