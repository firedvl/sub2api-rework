<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-[1600px] space-y-4 px-4 py-5 sm:px-6">
      <div class="flex flex-col gap-2 border-b border-gray-200 pb-4 dark:border-dark-600 sm:flex-row sm:items-end sm:justify-end">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-end">
          <label class="block min-w-64 text-xs font-medium text-gray-500 dark:text-gray-400">
            {{ t('admin.modelOperations.group') }}
            <select v-model.number="selectedGroupID" class="input mt-1" @change="loadSnapshot">
              <option :value="0">{{ t('admin.modelOperations.selectGroup') }}</option>
              <option v-for="group in groups" :key="group.id" :value="group.id">
                {{ group.name }} · {{ group.platform }}
              </option>
            </select>
          </label>
          <label class="block text-xs font-medium text-gray-500 dark:text-gray-400">
            {{ t('admin.modelOperations.filter') }}
            <span class="relative mt-1 block">
              <Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model="filter" class="input pl-9" type="search" :placeholder="t('admin.modelOperations.filter')" />
            </span>
          </label>
          <button class="btn btn-secondary h-10 px-3" type="button" :disabled="!selectedGroupID || loading" :title="t('admin.modelOperations.refresh')" @click="loadSnapshot">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          </button>
        </div>
      </div>

      <p v-if="error" class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300" role="alert">{{ error }}</p>
      <p v-if="snapshot?.warning" class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200" role="status">{{ snapshot.warning }}</p>

      <div v-if="snapshot" class="flex flex-wrap gap-x-6 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
        <span>{{ t('admin.modelOperations.generatedAt', { time: formatDate(snapshot.generated_at) }) }}</span>
        <span>{{ t('admin.modelOperations.window', { start: formatDate(snapshot.window.start), end: formatDate(snapshot.window.end) }) }}</span>
      </div>

      <div class="overflow-x-auto border-y border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800">
        <table class="min-w-[1460px] w-full table-fixed text-left text-sm">
          <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-700 dark:text-gray-400">
            <tr>
              <th class="w-64 px-3 py-3 font-medium">{{ t('admin.modelOperations.columns.model') }}</th>
              <th class="w-48 px-3 py-3 font-medium">{{ t('admin.modelOperations.columns.provider') }}</th>
              <th class="w-52 px-3 py-3 font-medium">{{ t('admin.modelOperations.columns.catalog') }}</th>
              <th class="w-48 px-3 py-3 font-medium">{{ t('admin.modelOperations.columns.availability') }}</th>
              <th class="w-44 px-3 py-3 font-medium">{{ t('admin.modelOperations.columns.visibility') }}</th>
              <th class="w-36 px-3 py-3 font-medium">{{ t('admin.modelOperations.columns.routes') }}</th>
              <th class="w-48 px-3 py-3 font-medium">{{ t('admin.modelOperations.columns.usage') }}</th>
              <th class="w-72 px-3 py-3 font-medium">{{ t('admin.modelOperations.columns.performance') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-if="loading && !snapshot"><td colspan="8" class="px-3 py-10 text-center text-gray-500" role="status">{{ t('common.loading') }}</td></tr>
            <tr v-else-if="!selectedGroupID"><td colspan="8" class="px-3 py-10 text-center text-gray-500">{{ t('admin.modelOperations.selectGroup') }}</td></tr>
            <tr v-else-if="filteredModels.length === 0"><td colspan="8" class="px-3 py-10 text-center text-gray-500">{{ t('admin.modelOperations.empty') }}</td></tr>
            <tr v-for="model in filteredModels" :key="model.public_id" class="align-top hover:bg-gray-50 dark:hover:bg-dark-700/60">
              <td class="px-3 py-3">
                <div class="break-words font-mono text-xs font-semibold text-gray-900 dark:text-white">{{ model.public_id }}</div>
                <div v-if="model.model_id !== model.public_id" class="mt-1 break-words font-mono text-xs text-gray-500">{{ model.model_id }}</div>
              </td>
              <td class="px-3 py-3">
                <div class="font-medium text-gray-800 dark:text-gray-200">{{ model.actual_platform || t('admin.modelOperations.unknown') }}</div>
                <div class="mt-1 break-words text-xs text-gray-500">{{ sourceLabel(model.discovery_source) }}</div>
              </td>
              <td class="px-3 py-3"><StatusLines :items="catalogItems(model)" /></td>
              <td class="px-3 py-3"><StatusLines :items="availabilityItems(model)" /></td>
              <td class="px-3 py-3"><StatusLines :items="visibilityItems(model)" /></td>
              <td class="px-3 py-3">
                <div class="text-gray-800 dark:text-gray-200">{{ model.route_type || t('admin.modelOperations.unknown') }}</div>
                <div class="mt-1 text-xs text-gray-500">{{ model.available_route_count ?? t('admin.modelOperations.unknown') }}</div>
              </td>
              <td class="px-3 py-3 text-xs text-gray-600 dark:text-gray-300">
                <div>{{ t('admin.modelOperations.labels.requests') }} <strong>{{ formatNumber(model.recent_request_count) }}</strong></div>
                <div class="mt-1">{{ t('admin.modelOperations.labels.tokens') }} <strong>{{ formatNumber(model.recent_total_tokens) }}</strong></div>
                <div class="mt-1">{{ t('admin.modelOperations.labels.samples') }} <strong>{{ formatNumber(model.sample_count) }}</strong></div>
              </td>
              <td class="px-3 py-3 text-xs text-gray-600 dark:text-gray-300">
                <div>{{ t('admin.modelOperations.labels.latency') }} <strong>P50 {{ formatMilliseconds(model.latency_p50_ms) }}</strong> · <strong>P95 {{ formatMilliseconds(model.latency_p95_ms) }}</strong></div>
                <div class="mt-1">{{ t('admin.modelOperations.labels.ttft') }} <strong>P50 {{ formatMilliseconds(model.ttft_p50_ms) }}</strong> · <strong>P95 {{ formatMilliseconds(model.ttft_p95_ms) }}</strong></div>
                <div class="mt-1">{{ t('admin.modelOperations.labels.outputSpeed') }} <strong>{{ formatThroughput(model.output_tokens_per_second) }}</strong></div>
                <div class="mt-1">{{ t('admin.modelOperations.labels.timingSamples') }} <strong>{{ formatNumber(model.timing_sample_count) }}</strong></div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, type PropType } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { getAllIncludingInactive, getModelOperations, type ModelOperationsModel, type ModelOperationsSnapshot } from '@/api/admin/groups'
import type { AdminGroup } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

type StatusItem = { label: string; value?: boolean; text?: string; tone?: 'positive' | 'negative' | 'warning' }

const StatusLines = defineComponent({
  props: { items: { type: Array as PropType<StatusItem[]>, required: true } },
  setup(props) {
    return () => h('div', { class: 'space-y-1 text-xs' }, props.items.map((item) => h('div', { class: 'flex items-center justify-between gap-2' }, [
      h('span', { class: 'text-gray-500 dark:text-gray-400' }, item.label),
      h('span', { class: item.tone === 'negative' ? 'font-medium text-red-700 dark:text-red-400' : item.tone === 'warning' || item.value === undefined ? 'font-medium text-amber-700 dark:text-amber-400' : item.value === true || item.tone === 'positive' ? 'font-medium text-emerald-700 dark:text-emerald-400' : 'font-medium text-gray-500 dark:text-gray-400' }, item.text),
    ])))
  },
})

const { t } = useI18n()
const groups = ref<AdminGroup[]>([])
const selectedGroupID = ref(0)
const snapshot = ref<ModelOperationsSnapshot | null>(null)
const filter = ref('')
const loading = ref(false)
const error = ref('')

const filteredModels = computed(() => {
  const query = filter.value.trim().toLowerCase()
  if (!query) return snapshot.value?.models ?? []
  return (snapshot.value?.models ?? []).filter((model) => [model.public_id, model.model_id, model.actual_platform, model.discovery_source].some((value) => value.toLowerCase().includes(query)))
})

function boolText(value: boolean | undefined): string {
  if (value === undefined) return t('admin.modelOperations.unknown')
  return t(value ? 'admin.modelOperations.yes' : 'admin.modelOperations.no')
}

function item(label: string, value: boolean | undefined): StatusItem {
  return { label, value, text: boolText(value) }
}

function catalogItems(model: ModelOperationsModel): StatusItem[] {
  return [item(t('admin.modelOperations.labels.catalogMember'), model.catalog_member), item(t('admin.modelOperations.labels.configured'), model.configured), item(t('admin.modelOperations.labels.discovered'), model.discovered)]
}

function availabilityItems(model: ModelOperationsModel): StatusItem[] {
  const limited = model.rate_limited_or_cooldown
  return [
    { label: t('admin.modelOperations.availability'), value: model.current_availability === 'available' || model.current_availability === 'degraded' ? true : model.current_availability === 'unavailable' ? false : undefined, text: model.current_availability, tone: model.current_availability === 'unavailable' ? 'negative' : model.current_availability === 'degraded' || model.current_availability === 'unknown' ? 'warning' : 'positive' },
    item(t('admin.modelOperations.labels.routable'), model.routable),
    { label: t('admin.modelOperations.labels.limited'), value: limited === undefined ? undefined : !limited, text: boolText(limited), tone: limited ? 'warning' : undefined },
    item(t('admin.modelOperations.labels.healthy'), model.healthy),
  ]
}

function visibilityItems(model: ModelOperationsModel): StatusItem[] {
  return [item(t('admin.modelOperations.labels.v1Models'), model.v1_models_visible), item(t('admin.modelOperations.labels.picker'), model.codex_picker_visible)]
}

function sourceLabel(value: string): string {
  return value ? value.split('_').join(' ') : t('admin.modelOperations.unknown')
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat().format(value)
}

function formatMilliseconds(value: number | null): string {
  return value != null && Number.isFinite(value) && value > 0
    ? `${formatNumber(Math.round(value))} ms`
    : t('admin.modelOperations.notAvailable')
}

function formatThroughput(value: number | null): string {
  return value != null && Number.isFinite(value) && value > 0
    ? `${new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 }).format(value)} tok/s`
    : t('admin.modelOperations.notAvailable')
}

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

async function loadSnapshot() {
  if (!selectedGroupID.value) {
    snapshot.value = null
    return
  }
  loading.value = true
  error.value = ''
  try {
    snapshot.value = await getModelOperations(selectedGroupID.value)
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('admin.modelOperations.loadFailed'))
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  try {
    groups.value = await getAllIncludingInactive()
    const firstActive = groups.value.find((group) => group.status === 'active') ?? groups.value[0]
    if (firstActive) {
      selectedGroupID.value = firstActive.id
      await loadSnapshot()
    }
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('admin.modelOperations.loadFailed'))
  }
})
</script>
