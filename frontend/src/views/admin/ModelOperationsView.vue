<template>
  <AppLayout>
    <div class="ui-page mo-page">
      <div class="ui-toolbar" role="toolbar" :aria-label="t('admin.routingUi.filters')">
        <label class="mo-group">
          <ProviderMark v-if="selectedGroup" :platform="selectedGroup.platform" :size="22" />
          <select
            v-model.number="selectedGroupID"
            class="mo-group-select"
            :aria-label="t('admin.modelOperations.group')"
            @change="loadSnapshot"
          >
            <option :value="0">{{ t('admin.modelOperations.selectGroup') }}</option>
            <option v-for="group in groups" :key="group.id" :value="group.id">
              {{ group.name }} · {{ group.platform }}
            </option>
          </select>
          <Icon name="chevronDown" size="xs" class="mo-group-caret" />
        </label>
        <label class="ui-search">
          <Icon name="search" size="sm" />
          <input v-model="filter" type="search" :placeholder="t('admin.routingUi.search')" :aria-label="t('admin.modelOperations.filter')" />
        </label>
        <span class="ui-grow"></span>
        <span
          v-if="snapshot"
          class="mo-stamp"
          :title="t('admin.modelOperations.window', { start: formatDate(snapshot.window.start), end: formatDate(snapshot.window.end) })"
        >
          <Icon name="clock" size="xs" />
          {{ t('admin.modelOperations.generatedAt', { time: formatDate(snapshot.generated_at) }) }}
        </span>
        <button
          class="app-icon-button mo-tool"
          type="button"
          :disabled="!selectedGroupID || loading"
          :aria-label="t('admin.modelOperations.refresh')"
          :title="t('admin.modelOperations.refresh')"
          @click="loadSnapshot"
        >
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
        </button>
      </div>

      <p v-if="error" class="mo-notice is-bad" role="alert"><Icon name="exclamationCircle" size="sm" />{{ error }}</p>
      <p v-if="snapshot?.warning" class="mo-notice is-warn" role="status"><Icon name="exclamationTriangle" size="sm" />{{ snapshot.warning }}</p>

      <div v-if="snapshot" class="viz-kpis mo-kpis ui-stagger">
        <div class="viz-card viz-kpi">
          <span class="viz-kpi-label"><Icon name="cube" size="xs" />{{ t('admin.modelOperations.columns.model') }}</span>
          <div class="viz-kpi-row"><span class="viz-kpi-value"><CountUp :value="snapshot.models.length" /></span></div>
        </div>
        <div class="viz-card viz-kpi">
          <span class="viz-kpi-label"><i class="ui-dot" :class="servingTone"></i>{{ t('admin.modelOperations.columns.availability') }}</span>
          <div class="viz-kpi-row">
            <span class="viz-kpi-value"><CountUp :value="servingCount" /><small class="mo-kpi-of">/{{ snapshot.models.length }}</small></span>
          </div>
        </div>
        <div class="viz-card viz-kpi">
          <span class="viz-kpi-label"><Icon name="chartBar" size="xs" />{{ t('admin.modelOperations.labels.requests') }}</span>
          <div class="viz-kpi-row"><span class="viz-kpi-value"><CountUp :value="totals.requests" :format="formatCompact" /></span></div>
        </div>
        <div class="viz-card viz-kpi">
          <span class="viz-kpi-label"><Icon name="database" size="xs" />{{ t('admin.modelOperations.labels.tokens') }}</span>
          <div class="viz-kpi-row"><span class="viz-kpi-value"><CountUp :value="totals.tokens" :format="formatCompact" /></span></div>
        </div>
      </div>

      <section class="ui-card mo-card" :aria-busy="loading">
        <div class="ui-table-wrap">
          <table class="ui-table mo-table">
            <thead>
              <tr>
                <th>{{ t('admin.modelOperations.columns.model') }}</th>
                <th>{{ t('admin.modelOperations.columns.availability') }}</th>
                <th class="is-optional">{{ t('admin.modelOperations.columns.catalog') }}</th>
                <th class="is-optional">{{ t('admin.modelOperations.columns.visibility') }}</th>
                <th class="is-optional">{{ t('admin.modelOperations.columns.routes') }}</th>
                <th class="is-optional">{{ t('admin.modelOperations.labels.requests') }}</th>
                <th class="is-optional">{{ t('admin.modelOperations.labels.latency') }}</th>
              </tr>
            </thead>
            <tbody v-if="loading && !snapshot">
              <tr v-for="n in 4" :key="n" aria-hidden="true">
                <td v-for="c in 7" :key="c" :class="c > 2 ? 'is-optional' : ''"><span class="skeleton mo-skel" :class="c === 1 ? 'is-wide' : ''"></span></td>
              </tr>
            </tbody>
            <tbody v-else class="ui-rows-enter">
              <tr
                v-for="model in filteredModels"
                :key="model.public_id"
                class="is-clickable"
                :class="{ 'is-selected': drawerModel?.public_id === model.public_id }"
                tabindex="0"
                :aria-label="model.public_id"
                @click="drawerModel = model"
                @keydown.enter.self.prevent="drawerModel = model"
              >
                <td>
                  <div class="mo-model">
                    <ProviderMark :platform="model.actual_platform || selectedGroup?.platform || ''" :size="26" :label="model.actual_platform || t('admin.modelOperations.unknown')" />
                    <span class="min-w-0">
                      <span class="ui-cell-main font-mono text-[13px]">{{ model.public_id }}</span>
                      <span class="ui-cell-sub">{{ model.model_id !== model.public_id ? model.model_id : sourceLabel(model.discovery_source) }}</span>
                    </span>
                  </div>
                </td>
                <td>
                  <span class="mo-status">
                    <i class="ui-dot" :class="availabilityDot[model.current_availability]"></i>
                    {{ model.current_availability }}
                  </span>
                  <span class="mo-flags">
                    <span v-for="flag in availabilityFlags(model)" :key="flag.label" class="mo-flag" :class="flagClass(flag)" :title="`${flag.label}: ${flag.text}`">
                      <Icon :name="flagIcon(flag)" size="xs" /><span class="sr-only">{{ flag.label }}: {{ flag.text }}</span>
                    </span>
                  </span>
                </td>
                <td class="is-optional">
                  <span class="mo-flags is-labeled">
                    <span v-for="flag in catalogItems(model)" :key="flag.label" class="mo-flag" :class="flagClass(flag)" :title="`${flag.label}: ${flag.text}`">
                      <Icon :name="flagIcon(flag)" size="xs" />{{ flag.label }}<span class="sr-only">: {{ flag.text }}</span>
                    </span>
                  </span>
                </td>
                <td class="is-optional">
                  <span class="mo-flags is-labeled">
                    <span v-for="flag in visibilityItems(model)" :key="flag.label" class="mo-flag" :class="flagClass(flag)" :title="`${flag.label}: ${flag.text}`">
                      <Icon :name="flagIcon(flag)" size="xs" />{{ flag.label }}<span class="sr-only">: {{ flag.text }}</span>
                    </span>
                  </span>
                </td>
                <td class="is-optional">
                  <span class="ui-cell-main">{{ model.route_type || t('admin.modelOperations.unknown') }}</span>
                  <span class="ui-cell-sub">{{ model.available_route_count ?? t('admin.modelOperations.unknown') }}</span>
                </td>
                <td class="is-optional">
                  <div class="mo-usage" :title="`${t('admin.modelOperations.labels.samples')} ${formatNumber(model.sample_count)}`">
                    <b>{{ formatNumber(model.recent_request_count) }}</b>
                    <span class="mo-usage-track"><i :style="{ width: `${usageShare(model)}%` }"></i></span>
                    <span class="ui-cell-sub">{{ formatCompact(model.recent_total_tokens) }} {{ t('admin.modelOperations.labels.tokens') }}</span>
                  </div>
                </td>
                <td class="is-optional">
                  <span class="ui-cell-main mo-latency">P50 {{ formatMilliseconds(model.latency_p50_ms) }}<small> · P95 {{ formatMilliseconds(model.latency_p95_ms) }}</small></span>
                  <span class="ui-cell-sub" :title="`${t('admin.modelOperations.labels.timingSamples')} ${formatNumber(model.timing_sample_count)}`">
                    {{ t('admin.modelOperations.labels.ttft') }} {{ formatMilliseconds(model.ttft_p50_ms) }} · {{ formatThroughput(model.output_tokens_per_second) }}
                    <span class="sr-only">{{ t('admin.modelOperations.labels.timingSamples') }} {{ formatNumber(model.timing_sample_count) }}</span>
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="!(loading && !snapshot) && filteredModels.length === 0" class="ui-empty">
          <span class="ui-empty-icon"><Icon name="cube" size="md" /></span>
          <b>{{ !selectedGroupID ? t('admin.modelOperations.selectGroup') : t('admin.modelOperations.empty') }}</b>
        </div>
      </section>
    </div>

    <SideDrawer
      :open="!!drawerModel"
      :title="drawerModel?.public_id ?? ''"
      :subtitle="drawerModel && drawerModel.model_id !== drawerModel.public_id ? drawerModel.model_id : sourceLabel(drawerModel?.discovery_source ?? '')"
      @close="drawerModel = null"
    >
      <template #leading>
        <ProviderMark v-if="drawerModel" :platform="drawerModel.actual_platform || selectedGroup?.platform || ''" :size="36" />
      </template>
      <template #badge>
        <span v-if="drawerModel" class="badge" :class="availabilityBadge[drawerModel.current_availability]">{{ drawerModel.current_availability }}</span>
      </template>
      <template v-if="drawerModel">
        <section>
          <div class="ui-section-label">{{ t('admin.modelOperations.columns.availability') }}</div>
          <StatusLines :items="availabilityItems(drawerModel)" />
        </section>
        <section>
          <div class="ui-section-label">{{ t('admin.modelOperations.columns.catalog') }}</div>
          <StatusLines :items="catalogItems(drawerModel)" />
        </section>
        <section>
          <div class="ui-section-label">{{ t('admin.modelOperations.columns.visibility') }}</div>
          <StatusLines :items="visibilityItems(drawerModel)" />
        </section>
        <section>
          <div class="ui-section-label">{{ t('admin.modelOperations.columns.performance') }}</div>
          <dl class="ui-dl">
            <dt>{{ t('admin.modelOperations.columns.provider') }}</dt>
            <dd>{{ drawerModel.actual_platform || t('admin.modelOperations.unknown') }} · {{ sourceLabel(drawerModel.discovery_source) }}</dd>
            <dt>{{ t('admin.modelOperations.columns.routes') }}</dt>
            <dd>{{ drawerModel.route_type || t('admin.modelOperations.unknown') }} · {{ drawerModel.available_route_count ?? t('admin.modelOperations.unknown') }}</dd>
            <dt>{{ t('admin.modelOperations.labels.requests') }}</dt>
            <dd class="tabular-nums">{{ formatNumber(drawerModel.recent_request_count) }}</dd>
            <dt>{{ t('admin.modelOperations.labels.tokens') }}</dt>
            <dd class="tabular-nums">{{ formatNumber(drawerModel.recent_total_tokens) }}</dd>
            <dt>{{ t('admin.modelOperations.labels.samples') }}</dt>
            <dd class="tabular-nums">{{ formatNumber(drawerModel.sample_count) }}</dd>
            <dt>{{ t('admin.modelOperations.labels.latency') }}</dt>
            <dd class="tabular-nums">P50 {{ formatMilliseconds(drawerModel.latency_p50_ms) }} · P95 {{ formatMilliseconds(drawerModel.latency_p95_ms) }}</dd>
            <dt>{{ t('admin.modelOperations.labels.ttft') }}</dt>
            <dd class="tabular-nums">P50 {{ formatMilliseconds(drawerModel.ttft_p50_ms) }} · P95 {{ formatMilliseconds(drawerModel.ttft_p95_ms) }}</dd>
            <dt>{{ t('admin.modelOperations.labels.outputSpeed') }}</dt>
            <dd class="tabular-nums">{{ formatThroughput(drawerModel.output_tokens_per_second) }}</dd>
            <dt>{{ t('admin.modelOperations.labels.timingSamples') }}</dt>
            <dd class="tabular-nums">{{ formatNumber(drawerModel.timing_sample_count) }}</dd>
          </dl>
        </section>
      </template>
    </SideDrawer>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, ref, type PropType } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import CountUp from '@/components/ui/CountUp.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import { getAllIncludingInactive, getModelOperations, type ModelOperationsModel, type ModelOperationsSnapshot } from '@/api/admin/groups'
import type { AdminGroup } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

type StatusItem = { label: string; value?: boolean; text?: string; tone?: 'positive' | 'negative' | 'warning' }

const StatusLines = defineComponent({
  props: { items: { type: Array as PropType<StatusItem[]>, required: true } },
  setup(props) {
    return () => h('div', { class: 'space-y-1.5 text-sm' }, props.items.map((item) => h('div', { class: 'flex items-center justify-between gap-2' }, [
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

const drawerModel = ref<ModelOperationsModel | null>(null)
const selectedGroup = computed(() => groups.value.find((group) => group.id === selectedGroupID.value) ?? null)

const availabilityDot: Record<ModelOperationsModel['current_availability'], string> = {
  available: 'is-ok',
  degraded: 'is-warn',
  unavailable: 'is-bad',
  unknown: '',
}
const availabilityBadge: Record<ModelOperationsModel['current_availability'], string> = {
  available: 'badge-success',
  degraded: 'badge-warning',
  unavailable: 'badge-danger',
  unknown: 'badge-gray',
}

const servingCount = computed(() => (snapshot.value?.models ?? []).filter((m) => m.current_availability === 'available' || m.current_availability === 'degraded').length)
const servingTone = computed(() => {
  const total = snapshot.value?.models.length ?? 0
  if (!total) return ''
  return servingCount.value === total ? 'is-ok' : servingCount.value === 0 ? 'is-bad' : 'is-warn'
})
const totals = computed(() => (snapshot.value?.models ?? []).reduce(
  (sum, m) => ({ requests: sum.requests + (m.recent_request_count || 0), tokens: sum.tokens + (m.recent_total_tokens || 0) }),
  { requests: 0, tokens: 0 },
))
const maxRequests = computed(() => Math.max(1, ...(snapshot.value?.models ?? []).map((m) => m.recent_request_count || 0)))
const usageShare = (model: ModelOperationsModel) => Math.round(((model.recent_request_count || 0) / maxRequests.value) * 100)

/** Routable, not limited, and healthy read as positive; limited inverts. */
function availabilityFlags(model: ModelOperationsModel): StatusItem[] {
  return availabilityItems(model).slice(1)
}

function flagClass(flag: StatusItem): string {
  if (flag.tone === 'negative' || flag.value === false) return 'is-no'
  if (flag.tone === 'warning' || flag.value === undefined) return 'is-unknown'
  return 'is-yes'
}

function flagIcon(flag: StatusItem): 'check' | 'x' | 'questionCircle' {
  const cls = flagClass(flag)
  return cls === 'is-yes' ? 'check' : cls === 'is-no' ? 'x' : 'questionCircle'
}

const compactFormatter = new Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 })
function formatCompact(value: number): string {
  return compactFormatter.format(Math.round(value))
}

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
  drawerModel.value = null
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

<style scoped>
.mo-group {
  position: relative;
  display: inline-flex;
  height: 2.25rem;
  min-width: 14rem;
  align-items: center;
  gap: 0.5rem;
  padding: 0 0.625rem 0 0.375rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-card);
  transition: border-color var(--operator-duration-fast) var(--operator-ease-out);
}

.mo-group:hover {
  border-color: var(--operator-border-strong);
}

.mo-group:focus-within {
  border-color: rgb(var(--operator-accent-rgb) / 0.6);
  box-shadow: 0 0 0 3px rgb(var(--operator-accent-rgb) / 0.15);
}

.mo-group-select {
  min-width: 0;
  height: 100%;
  flex: 1 1 auto;
  appearance: none;
  border: 0;
  background: transparent;
  padding: 0 1.25rem 0 0;
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 500;
  outline: none;
  cursor: pointer;
}

.mo-group-caret {
  position: absolute;
  right: 0.625rem;
  color: var(--operator-subtle-foreground);
  pointer-events: none;
}

.mo-stamp {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.mo-tool {
  border: 1px solid var(--operator-border);
  background: var(--operator-card);
}

.mo-notice {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.625rem 0.875rem;
  border-radius: var(--operator-radius-lg);
  font-size: 0.8125rem;
}

.mo-notice.is-bad {
  background: var(--operator-destructive-soft);
  color: var(--operator-destructive);
}

.mo-notice.is-warn {
  background: var(--operator-warning-soft);
  color: var(--operator-warning);
}

.mo-kpis {
  margin-bottom: 0;
}

.mo-kpi-of {
  margin-left: 0.125rem;
  color: var(--operator-subtle-foreground);
  font-size: 0.9375rem;
  font-weight: 500;
}

.mo-card {
  overflow: hidden;
}

.mo-table td {
  height: 3.75rem;
}

.mo-table tr.is-clickable:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: -2px;
}

.mo-skel {
  display: block;
  width: 4rem;
  height: 0.75rem;
  border-radius: 999px;
}

.mo-skel.is-wide {
  width: 10rem;
}

.mo-model {
  display: flex;
  min-width: 0;
  max-width: 20rem;
  align-items: center;
  gap: 0.625rem;
}

.mo-status {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  text-transform: capitalize;
}

.mo-flags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.25rem;
  margin-top: 0.25rem;
}

.mo-flags.is-labeled {
  margin-top: 0;
}

.mo-flag {
  display: inline-flex;
  height: 1.25rem;
  min-width: 1.25rem;
  align-items: center;
  justify-content: center;
  gap: 0.25rem;
  border-radius: 999px;
  font-size: 0.6875rem;
  font-weight: 500;
  white-space: nowrap;
}

.is-labeled .mo-flag {
  padding: 0 0.4375rem 0 0.3125rem;
}

.mo-flag.is-yes {
  background: var(--operator-success-soft);
  color: var(--operator-success);
}

.mo-flag.is-no {
  background: var(--operator-muted);
  color: var(--operator-subtle-foreground);
}

.mo-flag.is-unknown {
  background: var(--operator-warning-soft);
  color: var(--operator-warning);
}

.mo-usage {
  display: grid;
  min-width: 7rem;
  grid-template-columns: auto minmax(3rem, 1fr);
  align-items: center;
  gap: 0.125rem 0.5rem;
}

.mo-usage b {
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}

.mo-usage-track {
  height: 0.3125rem;
  overflow: hidden;
  border-radius: 999px;
  background: var(--operator-track);
}

.mo-usage-track i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--operator-accent);
  transform-origin: left center;
  animation: viz-grow-x 700ms var(--operator-ease-out) both;
}

.mo-usage .ui-cell-sub {
  grid-column: 1 / -1;
}

.mo-latency small {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
  font-weight: 400;
}

@media (max-width: 760px) {
  .mo-table .is-optional {
    display: none;
  }

  .mo-group {
    min-width: 0;
    flex: 1 1 100%;
  }

  .mo-stamp {
    display: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .mo-usage-track i {
    animation: none;
  }
}
</style>
