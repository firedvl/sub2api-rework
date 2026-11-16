<template>
  <section class="viz-card viz-span-8" aria-labelledby="pd-trend">
    <header class="viz-card-head">
      <h2 id="pd-trend" class="viz-card-title">{{ t('personalUi.dashboard.usage') }}</h2>
      <div class="viz-card-tools">
        <div class="viz-seg" role="group" :aria-label="t('personalUi.dashboard.metric')">
          <button
            v-for="item in metrics"
            :key="item.id"
            type="button"
            :aria-pressed="metric === item.id"
            @click="metric = item.id"
          >{{ item.label }}</button>
        </div>
        <div class="viz-seg" role="group" :aria-label="t('dashboard.granularity')">
          <button
            v-for="option in granularityOptions"
            :key="option.value"
            type="button"
            :aria-pressed="granularity === option.value"
            @click="setGranularity(option.value)"
          >{{ option.label }}</button>
        </div>
        <DateRangePicker
          :start-date="startDate"
          :end-date="endDate"
          @update:startDate="$emit('update:startDate', $event)"
          @update:endDate="$emit('update:endDate', $event)"
          @change="$emit('dateRangeChange', $event)"
        />
        <button
          type="button"
          class="app-icon-button"
          :disabled="loading"
          :aria-label="t('common.refresh')"
          :title="t('common.refresh')"
          @click="$emit('refresh')"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>
    </header>
    <div class="viz-card-body">
      <div v-if="loading && !trend.length" class="skeleton h-[240px] rounded-lg"></div>
      <div v-else-if="!trend.length" class="viz-state">
        <Icon name="chart" size="lg" class="text-gray-400" />
        <span>{{ t('dashboard.noDataAvailable') }}</span>
      </div>
      <TrendChart
        v-else
        :labels="labels"
        :tips="tips"
        :series="[{ name: activeMetric.label, color: 'var(--operator-accent-text)', values }]"
        :summary="`${activeMetric.label} · ${t('dashboard.tokenUsageTrend')}`"
        :height="240"
        :type="trend.length <= 2 ? 'bar' : 'line'"
        :format-value="activeMetric.format"
        :format-axis="activeMetric.axis"
      />
    </div>
    <footer v-if="trend.length" class="viz-card-foot">
      <span>{{ t('common.total') }} <b>{{ activeMetric.format(total) }}</b></span>
      <span v-if="peakIndex >= 0">{{ t('personalUi.dashboard.peak') }} <b>{{ activeMetric.format(values[peakIndex]) }}</b> · {{ tips[peakIndex] }}</span>
    </footer>
  </section>

  <section class="viz-card viz-span-4" aria-labelledby="pd-models">
    <header class="viz-card-head">
      <h2 id="pd-models" class="viz-card-title">{{ t('dashboard.modelDistribution') }}</h2>
      <span v-if="models.length" class="viz-card-tools text-xs text-gray-500">{{ t('dashboard.tokens') }}</span>
    </header>
    <div class="viz-card-body">
      <div v-if="loading && !models.length" class="flex flex-col gap-3">
        <div v-for="i in 5" :key="i" class="skeleton h-8 rounded-md"></div>
      </div>
      <div v-else-if="!models.length" class="viz-state">
        <Icon name="cube" size="lg" class="text-gray-400" />
        <span>{{ t('dashboard.noDataAvailable') }}</span>
      </div>
      <template v-else>
        <TransitionGroup tag="div" name="ui-list" class="viz-hbars">
          <div
            v-for="(model, index) in shownModels"
            :key="model.model"
            class="viz-hbar"
            :title="`${model.model}\n${t('dashboard.requests')}: ${formatInt(model.requests)}\n${t('dashboard.actual')}: $${formatCost(model.actual_cost)}\n${t('dashboard.standard')}: $${formatCost(model.cost)}`"
          >
            <span class="pd-model-name">
              <ProviderMark v-if="providerOfModel(model.model)" :platform="providerOfModel(model.model)" :size="22" />
              <span v-else class="pd-model-fallback" aria-hidden="true"><Icon name="cube" size="xs" /></span>
              <span class="min-w-0">
                <span class="block truncate font-mono text-[13px]">{{ model.model }}</span>
                <small class="block text-xs text-gray-500 tabular-nums">{{ formatInt(model.requests) }} · ${{ formatCost(model.actual_cost) }}</small>
              </span>
            </span>
            <span class="viz-row-value">{{ formatCompact(model.total_tokens) }}<small>{{ share(model.total_tokens) }}</small></span>
            <span class="viz-hbar-track">
              <i :style="{ width: `${(model.total_tokens / maxTokens) * 100}%`, background: barColor(model.model), animationDelay: `${index * 40}ms` }"></i>
            </span>
          </div>
        </TransitionGroup>
        <button
          v-if="sortedModels.length > TOP"
          type="button"
          class="viz-link mt-2"
          :aria-expanded="showAll"
          @click="showAll = !showAll"
        >
          {{ showAll ? t('personalUi.dashboard.showLess') : t('personalUi.dashboard.showAll', { count: sortedModels.length }) }}
          <Icon :name="showAll ? 'chevronUp' : 'chevronDown'" size="xs" />
        </button>
      </template>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import TrendChart from '@/components/viz/TrendChart.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import { providerColor, providerOfModel } from '@/utils/providerSeries'
import { formatAxisMoney, formatAxisNumber, formatCompact, formatInt } from '@/utils/vizFormat'
import type { TrendDataPoint, ModelStat } from '@/types'

const props = defineProps<{ loading: boolean, startDate: string, endDate: string, granularity: string, trend: TrendDataPoint[], models: ModelStat[] }>()
const emit = defineEmits(['update:startDate', 'update:endDate', 'update:granularity', 'dateRangeChange', 'granularityChange', 'refresh'])
const { t } = useI18n()

type Metric = 'tokens' | 'cost' | 'requests'
const TOP = 6
const metric = ref<Metric>('tokens')
const showAll = ref(false)

const money = (value: number) => `$${value.toFixed(value >= 100 ? 2 : 4)}`
const formatCost = (value: number) => value.toFixed(4)

const metrics = computed(() => [
  { id: 'tokens' as const, label: t('dashboard.tokens'), get: (point: TrendDataPoint) => point.total_tokens, format: formatCompact, axis: formatAxisNumber },
  { id: 'cost' as const, label: t('personalUi.dashboard.cost'), get: (point: TrendDataPoint) => point.actual_cost, format: money, axis: (value: number) => formatAxisMoney(value) },
  { id: 'requests' as const, label: t('dashboard.requests'), get: (point: TrendDataPoint) => point.requests, format: formatInt, axis: formatAxisNumber },
])
const activeMetric = computed(() => metrics.value.find((item) => item.id === metric.value) ?? metrics.value[0])

const granularityOptions = computed(() => [
  { value: 'day', label: t('dashboard.day') },
  { value: 'hour', label: t('dashboard.hour') },
])

function setGranularity(value: string) {
  if (value === props.granularity) return
  emit('update:granularity', value)
  emit('granularityChange')
}

// Dates arrive as "YYYY-MM-DD" or "YYYY-MM-DD HH:mm"; ticks keep the short part.
const tips = computed(() => props.trend.map((point) => point.date))
const labels = computed(() => props.trend.map((point) => {
  const [day, time] = point.date.split(/[ T]/)
  return props.granularity === 'hour' && time ? time.slice(0, 5) : day.slice(5)
}))
const values = computed(() => props.trend.map(activeMetric.value.get))
const total = computed(() => values.value.reduce((sum, value) => sum + value, 0))
const peakIndex = computed(() => {
  if (!values.value.length || Math.max(...values.value) <= 0) return -1
  return values.value.indexOf(Math.max(...values.value))
})

const sortedModels = computed(() => [...props.models].sort((left, right) => right.total_tokens - left.total_tokens))
const shownModels = computed(() => showAll.value ? sortedModels.value : sortedModels.value.slice(0, TOP))
const maxTokens = computed(() => Math.max(1, ...props.models.map((model) => model.total_tokens)))
const tokenSum = computed(() => props.models.reduce((sum, model) => sum + model.total_tokens, 0))
const share = (tokens: number) => tokenSum.value > 0 ? `${Math.round((tokens / tokenSum.value) * 100)}%` : ''
const barColor = (model: string) => {
  const provider = providerOfModel(model)
  return provider ? providerColor(provider) : 'var(--operator-accent)'
}
</script>

<style scoped>
.pd-model-name {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
  color: var(--operator-foreground);
}

.pd-model-fallback {
  display: inline-grid;
  width: 22px;
  height: 22px;
  flex: none;
  place-items: center;
  border-radius: 6px;
  background: var(--operator-raised);
  color: var(--operator-subtle-foreground);
}
</style>
