<template>
  <article class="stats-capacity-donut" :data-testid="testId">
    <header>
      <span v-if="platform" class="stats-capacity-donut-icon" aria-hidden="true">
        <ProviderIcon :provider="platform" :size="18" />
      </span>
      <strong>{{ title }}</strong>
    </header>

    <div class="stats-capacity-donut-chart">
      <svg viewBox="0 0 120 120" role="img" :aria-label="chartLabel">
        <title>{{ chartLabel }}</title>
        <circle class="stats-capacity-donut-ring is-background" cx="60" cy="60" r="46" pathLength="100" />
        <circle
          v-if="capacity.remainingPercent === null"
          class="stats-capacity-donut-ring is-unknown"
          cx="60"
          cy="60"
          r="46"
          pathLength="100"
        />
        <circle
          v-else
          class="stats-capacity-donut-ring is-remaining"
          :class="capacityTone(capacity.remainingPercent)"
          cx="60"
          cy="60"
          r="46"
          pathLength="100"
          :style="{
            strokeDasharray: `${capacity.remainingPercent} ${100 - capacity.remainingPercent}`,
          }"
        />
      </svg>
      <div class="stats-capacity-donut-value" aria-hidden="true">
        <strong>
          {{ capacity.remainingPercent === null
            ? t('common.unknown')
            : `${formatPercent(capacity.remainingPercent)}%` }}
        </strong>
      </div>
    </div>

    <dl>
      <div>
        <dt>{{ t('admin.stats.capacity.basis') }}</dt>
        <dd data-testid="stats-capacity-donut-basis">{{ basis }}</dd>
      </div>
      <div data-testid="stats-capacity-donut-coverage">
        <dt>{{ t('admin.stats.capacity.coverageLabel') }}</dt>
        <dd>{{ t('admin.stats.capacity.coverage', {
          known: capacity.knownCount,
          unknown: capacity.unknownCount,
        }) }}</dd>
      </div>
      <div
        v-if="capacity.lowestAccount && capacity.lowestRemaining !== null"
        data-testid="stats-capacity-donut-limit"
      >
        <dt>{{ t('admin.stats.capacity.limitingAccount') }}</dt>
        <dd class="stats-capacity-donut-limit">
          <span :title="capacity.lowestAccount.account.name">{{ limitingAccountLabel }}</span>
          <strong :class="capacityTone(capacity.lowestRemaining)">{{ formatPercent(capacity.lowestRemaining) }}%</strong>
        </dd>
      </div>
      <div v-if="capacity.nextReset" data-testid="stats-capacity-donut-reset">
        <dt>{{ t('admin.stats.capacity.nextReset') }}</dt>
        <dd>{{ formatCompactReset(capacity.nextReset) }}</dd>
      </div>
    </dl>
  </article>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import { formatDayAwareDateTime } from '@/utils/format'
import type { OperatorNormalizedCapacityAggregate } from '@/utils/operatorCapacity'
import type { AccountPlatform } from '@/types'

const props = defineProps<{
  title: string
  basis: string
  capacity: OperatorNormalizedCapacityAggregate
  platform?: AccountPlatform
  testId: string
}>()

const { t } = useI18n()
const formatPercent = (value: number) => value.toFixed(2).replace(/\.?0+$/, '')
const formatCompactReset = (value: string) => formatDayAwareDateTime(value) || t('common.unknown')
const capacityTone = (value: number) => {
  if (value === 0) return 'is-exhausted'
  if (value <= 20) return 'is-limited'
  return 'is-healthy'
}
const limitingAccountLabel = computed(() => {
  const account = props.capacity.lowestAccount
  if (!account || props.capacity.lowestRemaining === null) return ''
  const windows = account.windows
    .filter((window) => window.remainingPercent === props.capacity.lowestRemaining)
    .map((window) => window.label)
  return windows.length
    ? `${account.account.name} · ${windows.join(' / ')}`
    : account.account.name
})
const chartLabel = computed(() => {
  const coverage = t('admin.stats.capacity.coverage', {
    known: props.capacity.knownCount,
    unknown: props.capacity.unknownCount,
  })
  if (props.capacity.remainingPercent === null) {
    return `${props.title}, ${props.basis}: ${t('admin.dashboard.capacity.quotaUnknown')}. ${coverage}`
  }
  return `${props.title}, ${props.basis}: ${formatPercent(props.capacity.remainingPercent)}% ${t('admin.stats.capacity.averageLimitingRemaining')}. ${coverage}`
})
</script>

<style scoped>
.stats-capacity-donut {
  display: grid;
  min-width: 0;
  grid-template-columns: 6rem minmax(0, 1fr);
  align-items: start;
  gap: 0.75rem 1rem;
  padding: 1rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-card);
  box-shadow: var(--operator-shadow-xs);
}

.stats-capacity-donut header {
  display: flex;
  grid-column: 1 / -1;
  width: 100%;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
  min-height: 2rem;
  color: var(--operator-foreground);
  font-size: 0.8125rem;
}

.stats-capacity-donut header strong {
  line-height: 1.2;
  overflow-wrap: anywhere;
}

.stats-capacity-donut-icon {
  display: inline-grid;
  width: 1.75rem;
  height: 1.75rem;
  flex: 0 0 auto;
  place-items: center;
  border: 1px solid var(--operator-border-subtle);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
}

.stats-capacity-donut-chart {
  position: relative;
  width: 6rem;
  aspect-ratio: 1;
  align-self: center;
}

.stats-capacity-donut-chart svg {
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}

.stats-capacity-donut-ring {
  fill: none;
  stroke-width: 11;
}

.stats-capacity-donut-ring.is-background { stroke: var(--operator-border); }
.stats-capacity-donut-ring.is-unknown {
  stroke: var(--operator-muted-foreground);
  stroke-dasharray: 3 4;
}
.stats-capacity-donut-ring.is-healthy { stroke: var(--operator-success); }
.stats-capacity-donut-ring.is-limited { stroke: var(--operator-warning); }
.stats-capacity-donut-ring.is-exhausted { stroke: var(--operator-danger); }

.stats-capacity-donut-value {
  position: absolute;
  inset: 50% auto auto 50%;
  width: 4.5rem;
  transform: translate(-50%, -50%);
  text-align: center;
}

.stats-capacity-donut-value strong {
  color: var(--operator-foreground);
  font-size: 1.25rem;
  line-height: 1.1;
}

.stats-capacity-donut dl {
  display: grid;
  min-width: 0;
  align-self: stretch;
}

.stats-capacity-donut dl > div {
  min-width: 0;
  padding: 0.4rem 0;
  border-bottom: 1px solid var(--operator-border-subtle);
}

.stats-capacity-donut dl > div:first-child { padding-top: 0; }
.stats-capacity-donut dl > div:last-child { padding-bottom: 0; border-bottom: 0; }

.stats-capacity-donut dt {
  color: var(--operator-muted-foreground);
  font-size: 0.625rem;
  font-weight: 600;
}

.stats-capacity-donut dd {
  margin-top: 0.125rem;
  color: var(--operator-foreground);
  font-size: 0.6875rem;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.stats-capacity-donut-limit {
  display: flex;
  min-width: 0;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.375rem;
}

.stats-capacity-donut-limit span { min-width: 0; overflow-wrap: anywhere; }
.stats-capacity-donut-limit strong { flex: 0 0 auto; }
.stats-capacity-donut-limit .is-healthy { color: var(--operator-success); }
.stats-capacity-donut-limit .is-limited { color: var(--operator-warning); }
.stats-capacity-donut-limit .is-exhausted { color: var(--operator-danger); }
</style>
