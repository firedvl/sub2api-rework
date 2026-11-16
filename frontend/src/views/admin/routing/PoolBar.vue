<template>
  <div class="pool-bar" :class="{ 'is-large': large }" role="img" :aria-label="summary" :title="summary">
    <div class="pool-track">
      <template v-if="split.total">
        <i v-if="split.available" class="pool-seg is-ok" :style="{ flexGrow: split.available }"></i>
        <i v-if="split.limited" class="pool-seg is-warn" :style="{ flexGrow: split.limited }"></i>
        <i v-if="split.unavailable" class="pool-seg is-bad" :style="{ flexGrow: split.unavailable }"></i>
      </template>
    </div>
    <b v-if="!large">{{ split.available }}<small>/{{ split.total }}</small></b>
  </div>
  <div v-if="large" class="pool-legend">
    <span><i class="ui-dot is-ok"></i>{{ t('admin.routingUi.pool.available') }} <b>{{ split.available }}</b></span>
    <span><i class="ui-dot is-warn"></i>{{ t('admin.routingUi.pool.limited') }} <b>{{ split.limited }}</b></span>
    <span><i class="ui-dot is-bad"></i>{{ t('admin.routingUi.pool.unavailable') }} <b>{{ split.unavailable }}</b></span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PoolSplit } from './routeSummary'

/** Stacked account-pool bar: available, temporarily limited, unavailable. */
const props = defineProps<{ split: PoolSplit; large?: boolean }>()
const { t } = useI18n()

const summary = computed(() =>
  props.split.total
    ? t('admin.routingUi.pool.summary', {
        available: props.split.available,
        limited: props.split.limited,
        unavailable: props.split.unavailable,
        total: props.split.total,
      })
    : t('admin.routingUi.pool.empty'),
)
</script>

<style scoped>
.pool-bar {
  display: flex;
  min-width: 7rem;
  align-items: center;
  gap: 0.5rem;
}

.pool-track {
  display: flex;
  height: 0.375rem;
  flex: 1 1 auto;
  gap: 2px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--operator-track);
}

.is-large .pool-track {
  height: 0.625rem;
}

.pool-seg {
  display: block;
  min-width: 4px;
  flex-basis: 0;
  transform-origin: left center;
  animation: viz-grow-x 700ms var(--operator-ease-out) both;
}

.pool-seg.is-ok { background: var(--operator-success-fill); }
.pool-seg.is-warn { background: var(--operator-warning-fill); animation-delay: 80ms; }
.pool-seg.is-bad { background: var(--operator-destructive-fill); animation-delay: 160ms; }

.pool-bar > b {
  min-width: 2.25rem;
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  text-align: right;
}

.pool-bar > b small {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
  font-weight: 400;
}

.pool-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem 1rem;
  margin-top: 0.625rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.pool-legend span {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}

.pool-legend b {
  color: var(--operator-foreground);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

@media (prefers-reduced-motion: reduce) {
  .pool-seg {
    animation: none;
  }
}
</style>
