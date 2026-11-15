<template>
  <div class="heat-grid" role="img" :aria-label="summary">
    <template v-for="row in rows" :key="row.label">
      <span class="heat-label">{{ row.label }}</span>
      <i
        v-for="(value, column) in row.values"
        :key="column"
        class="heat-cell"
        :class="value === null ? 'is-empty' : `heat-${level(value)}`"
        :title="value === null ? undefined : `${row.title} · ${columnTitle(column)} · ${formatValue(value)}`"
      ></i>
    </template>
    <span></span>
    <span v-for="tick in columnTicks" :key="tick.label" class="heat-tick" :style="{ gridColumn: `span ${tick.span}` }">{{ tick.label }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

export interface HeatRow {
  label: string
  /** Full row name for the cell tooltip. */
  title: string
  /** One value per column; null marks a period with no data yet. */
  values: Array<number | null>
}

/** Rows by 24 hourly columns. One sequential hue: light means little, dark means a lot. */
const props = withDefaults(defineProps<{
  rows: HeatRow[]
  /** Accessible name of the chart. */
  summary: string
  formatValue?: (value: number) => string
}>(), { formatValue: (value: number) => value.toLocaleString('en-US') })

const hourLabel = (hour: number) => `${hour % 12 || 12} ${hour < 12 ? 'AM' : 'PM'}`
const columnTitle = (column: number) => hourLabel(column)
const columnTicks = [0, 6, 12, 18].map((hour) => ({ label: hourLabel(hour).replace(' ', '').toLowerCase(), span: 6 }))

const max = computed(() => Math.max(1, ...props.rows.flatMap((row) => row.values.filter((value): value is number => value !== null))))
const level = (value: number) => (value <= 0 ? 0 : Math.max(1, Math.ceil((value / max.value) * 5)))
</script>

<style scoped>
.heat-grid {
  display: grid;
  grid-template-columns: 2.125rem repeat(24, minmax(0, 1fr));
  align-items: center;
  gap: 3px;
}

.heat-label,
.heat-tick {
  color: var(--operator-subtle-foreground);
  font-size: 0.6875rem;
}

.heat-cell {
  height: 1.625rem;
  border-radius: 4px;
  background: var(--operator-heat-0);
  outline: 2px solid transparent;
  outline-offset: -1px;
  transition:
    transform var(--operator-duration-fast) var(--operator-ease-out),
    outline-color var(--operator-duration-fast) var(--operator-ease-out);
}

.heat-cell:hover {
  position: relative;
  z-index: 1;
  outline-color: var(--operator-foreground);
  transform: scale(1.15);
}

.heat-cell.is-empty {
  border: 1px dashed var(--operator-border);
  background: transparent;
}

.heat-1 { background: var(--operator-heat-1); }
.heat-2 { background: var(--operator-heat-2); }
.heat-3 { background: var(--operator-heat-3); }
.heat-4 { background: var(--operator-heat-4); }
.heat-5 { background: var(--operator-heat-5); }
</style>
