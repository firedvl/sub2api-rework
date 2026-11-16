<template>
  <section class="viz-card pd-hero" aria-labelledby="pd-hero-label">
    <div class="pd-hero-grid">
      <div class="pd-hero-main">
        <p id="pd-hero-label" class="pd-hero-label">{{ isSimple ? t('dashboard.apiKeys') : t('dashboard.balance') }}</p>
        <b class="pd-hero-value" data-testid="dashboard-hero-value">
          <CountUp v-if="isSimple" :value="stats.total_api_keys || 0" :format="formatInt" />
          <CountUp v-else :value="balance" :format="formatBalance" />
        </b>
        <slot name="actions" />
      </div>

      <div class="pd-today" role="group" :aria-label="t('personalUi.dashboard.today')">
        <div class="ui-section-label">{{ t('personalUi.dashboard.today') }}</div>
        <dl class="pd-today-figures">
          <div :title="`${t('common.total')}: ${formatInt(stats.total_requests || 0)}`">
            <dt>{{ t('dashboard.requests') }}</dt>
            <dd><CountUp :value="stats.today_requests || 0" :format="formatInt" /></dd>
          </div>
          <div>
            <dt>{{ t('dashboard.tokens') }}</dt>
            <dd><CountUp :value="stats.today_tokens || 0" :format="formatCompact" /></dd>
            <div
              v-if="(stats.today_tokens || 0) > 0"
              class="viz-share pd-token-share"
              role="img"
              :aria-label="todayTokenSummary"
              :title="todayTokenSummary"
            >
              <i :style="{ flex: stats.today_input_tokens || 0, background: 'var(--operator-series-1)' }"></i>
              <i :style="{ flex: stats.today_output_tokens || 0, background: 'var(--operator-series-2)' }"></i>
              <i :style="{ flex: todayCache, background: 'var(--operator-series-3)' }"></i>
            </div>
          </div>
          <div :title="`${t('dashboard.standard')}: $${formatCost(stats.today_cost || 0)}`">
            <dt>{{ t('personalUi.dashboard.cost') }}</dt>
            <dd><CountUp :value="stats.today_actual_cost || 0" :format="formatMoney" /></dd>
          </div>
        </dl>
      </div>
    </div>

    <footer class="viz-card-foot pd-hero-foot">
      <span v-if="!isSimple">
        <Icon name="key" size="xs" />
        {{ t('dashboard.apiKeys') }}
        <b>{{ stats.active_api_keys || 0 }}/{{ stats.total_api_keys || 0 }}</b>
      </span>
      <span>
        <Icon name="bolt" size="xs" />
        RPM <b>{{ formatCompact(stats.rpm || 0) }}</b> · TPM <b>{{ formatCompact(stats.tpm || 0) }}</b>
      </span>
      <span>
        <Icon name="clock" size="xs" />
        {{ t('dashboard.avgResponse') }} <b>{{ formatDuration(stats.average_duration_ms || 0) }}</b>
      </span>
      <span :title="`${t('dashboard.standard')}: $${formatCost(stats.total_cost || 0)}`">
        <Icon name="dollar" size="xs" />
        {{ t('personalUi.dashboard.allTime') }} <b>${{ formatCost(stats.total_actual_cost || 0) }}</b>
      </span>
      <span :title="totalTokenSummary">
        <Icon name="database" size="xs" />
        {{ t('dashboard.totalTokens') }} <b>{{ formatCompact(stats.total_tokens || 0) }}</b>
      </span>
    </footer>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import CountUp from '@/components/ui/CountUp.vue'
import { formatCompact, formatDuration, formatInt } from '@/utils/vizFormat'
import type { UserDashboardStats as UserStatsType } from '@/api/usage'

/** Dashboard hero: balance (or key count in simple mode), today's figures, and all-time totals. */
const props = defineProps<{
  stats: UserStatsType
  balance: number
  isSimple: boolean
}>()
const { t } = useI18n()

const todayCache = computed(() => (props.stats.today_cache_creation_tokens || 0) + (props.stats.today_cache_read_tokens || 0))

const tokenSummary = (input: number, output: number, cache: number) =>
  `${t('dashboard.input')} ${formatCompact(input)} · ${t('dashboard.output')} ${formatCompact(output)} · ${t('dashboard.cache')} ${formatCompact(cache)}`

const todayTokenSummary = computed(() => tokenSummary(props.stats.today_input_tokens || 0, props.stats.today_output_tokens || 0, todayCache.value))
const totalTokenSummary = computed(() => tokenSummary(
  props.stats.total_input_tokens || 0,
  props.stats.total_output_tokens || 0,
  (props.stats.total_cache_creation_tokens || 0) + (props.stats.total_cache_read_tokens || 0),
))

const balanceFormatter = new Intl.NumberFormat('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
const formatBalance = (value: number) => `$${balanceFormatter.format(value)}`
const formatMoney = (value: number) => `$${value.toFixed(value >= 100 ? 2 : 4)}`
const formatCost = (value: number) => value.toFixed(4)
</script>

<style scoped>
.pd-hero {
  overflow: hidden;
}

.pd-hero::before {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: radial-gradient(60% 120% at 0% 0%, rgb(var(--operator-accent-rgb) / 0.28), transparent 70%);
  content: '';
  opacity: var(--operator-glow);
  pointer-events: none;
}

.pd-hero-grid {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.4fr);
  align-items: center;
  gap: 1.5rem;
  padding: 1.25rem;
}

.pd-hero-label {
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.pd-hero-value {
  display: block;
  margin: 0.125rem 0 1rem;
  color: var(--operator-foreground);
  font-size: 2.5rem;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.035em;
  line-height: 1.1;
}

.pd-today-figures {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 1rem;
}

.pd-today-figures dt {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.pd-today-figures dd {
  overflow: hidden;
  color: var(--operator-foreground);
  font-size: 1.5rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.02em;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pd-token-share {
  height: 0.375rem;
  margin-top: 0.375rem;
}

.pd-hero-foot {
  position: relative;
}

.pd-hero-foot > span {
  display: inline-flex;
  align-items: center;
  gap: 0.3125rem;
  white-space: nowrap;
}

@media (max-width: 768px) {
  .pd-hero-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 1rem;
  }

  .pd-hero-value {
    font-size: 2rem;
  }

  .pd-today-figures {
    gap: 0.75rem;
  }

  .pd-today-figures dd {
    font-size: 1.25rem;
  }
}
</style>
