<template>
  <div class="pagination-bar">
    <p class="pagination-info">{{ rangeText }}</p>

    <div v-if="showPageSizeSelector" class="pagination-size">
      <span class="pagination-size-label">{{ t('admin.adminUi.table.pageSize') }}</span>
      <div class="page-size-select">
        <Select
          :model-value="pageSize"
          :options="pageSizeSelectOptions"
          :aria-label="t('pagination.perPage')"
          @update:model-value="handlePageSizeChange"
        />
      </div>
    </div>

    <div v-if="showJump" class="pagination-jump">
      <input
        v-model="jumpPage"
        type="number"
        min="1"
        :max="totalPages"
        class="pagination-jump-input"
        :aria-label="t('pagination.jumpTo')"
        :placeholder="t('pagination.jumpPlaceholder')"
        @keyup.enter="submitJump"
      />
      <button type="button" class="pagination-button" @click="submitJump">
        {{ t('pagination.jumpAction') }}
      </button>
    </div>

    <nav class="pagination-nav" :aria-label="t('pagination.label')">
      <button
        type="button"
        class="pagination-button pagination-step"
        :disabled="page <= 1"
        :aria-label="t('pagination.previous')"
        @click="goToPage(page - 1)"
      >
        <Icon name="chevronLeft" size="sm" />
      </button>

      <span class="pagination-compact" aria-hidden="true">{{ page }} / {{ totalPages }}</span>

      <span class="pagination-pages">
        <template v-for="(pageNum, index) in visiblePages" :key="`${pageNum}-${index}`">
          <span v-if="typeof pageNum !== 'number'" class="pagination-gap" aria-hidden="true">…</span>
          <button
            v-else
            type="button"
            class="pagination-button"
            :class="{ 'pagination-button-current': pageNum === page }"
            :aria-label="t('pagination.goToPage', { page: pageNum })"
            :aria-current="pageNum === page ? 'page' : undefined"
            @click="goToPage(pageNum)"
          >
            {{ pageNum }}
          </button>
        </template>
      </span>

      <button
        type="button"
        class="pagination-button pagination-step"
        :disabled="page >= totalPages"
        :aria-label="t('pagination.next')"
        @click="goToPage(page + 1)"
      >
        <Icon name="chevronRight" size="sm" />
      </button>
    </nav>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Select from './Select.vue'
import { getConfiguredTablePageSizeOptions, normalizeTablePageSize } from '@/utils/tablePreferences'
import { setPersistedPageSize } from '@/composables/usePersistedPageSize'

const { t } = useI18n()

interface Props {
  total: number
  page: number
  pageSize: number
  pageSizeOptions?: number[]
  showPageSizeSelector?: boolean
  showJump?: boolean
}

interface Emits {
  (e: 'update:page', page: number): void
  (e: 'update:pageSize', pageSize: number): void
}

const props = withDefaults(defineProps<Props>(), {
  pageSizeOptions: () => getConfiguredTablePageSizeOptions(),
  showPageSizeSelector: true,
  showJump: false
})

const emit = defineEmits<Emits>()

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))

const fromItem = computed(() => {
  if (props.total === 0) return 0
  return (props.page - 1) * props.pageSize + 1
})

const toItem = computed(() => {
  const to = props.page * props.pageSize
  return to > props.total ? props.total : to
})

const rangeText = computed(() =>
  t('admin.adminUi.table.range', {
    from: fromItem.value.toLocaleString(),
    to: toItem.value.toLocaleString(),
    total: props.total.toLocaleString()
  })
)

const pageSizeSelectOptions = computed(() => {
  const options = Array.from(
    new Set([
      ...getConfiguredTablePageSizeOptions(),
      normalizeTablePageSize(props.pageSize)
    ])
  ).sort((a, b) => a - b)

  return options.map((size) => ({
    value: size,
    label: String(size)
  }))
})

const jumpPage = ref('')

const visiblePages = computed(() => {
  const pages: (number | string)[] = []
  const maxVisible = 5
  const total = totalPages.value

  if (total <= maxVisible) {
    // Show all pages if total is small
    for (let i = 1; i <= total; i++) {
      pages.push(i)
    }
  } else {
    // Always show first page
    pages.push(1)

    const start = Math.max(2, props.page - 1)
    const end = Math.min(total - 1, props.page + 1)

    // Add ellipsis before if needed
    if (start > 2) {
      pages.push('...')
    }

    // Add middle pages
    for (let i = start; i <= end; i++) {
      pages.push(i)
    }

    // Add ellipsis after if needed
    if (end < total - 1) {
      pages.push('...')
    }

    // Always show last page
    pages.push(total)
  }

  return pages
})

const goToPage = (newPage: number) => {
  if (newPage >= 1 && newPage <= totalPages.value && newPage !== props.page) {
    emit('update:page', newPage)
  }
}

const handlePageSizeChange = (value: string | number | boolean | null) => {
  if (value === null || typeof value === 'boolean') return
  const newPageSize = normalizeTablePageSize(typeof value === 'string' ? parseInt(value, 10) : value)
  setPersistedPageSize(newPageSize)
  emit('update:pageSize', newPageSize)
}

const submitJump = () => {
  const value = String(jumpPage.value).trim()
  if (!value) return
  const pageNum = Number.parseInt(value, 10)
  if (Number.isNaN(pageNum)) return
  const nextPage = Math.min(Math.max(pageNum, 1), totalPages.value)
  jumpPage.value = ''
  goToPage(nextPage)
}
</script>

<style scoped>
.pagination-bar {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem 1rem;
  padding: 0.5rem 0.75rem 0.5rem 1rem;
  border-top: 1px solid var(--operator-border);
  background: var(--operator-card);
}

.pagination-info {
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.pagination-size {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
}

.pagination-size-label {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.page-size-select {
  width: 4.5rem;
}

.page-size-select :deep(.select-trigger) {
  min-height: 1.875rem;
  padding-top: 0.125rem;
  padding-bottom: 0.125rem;
  font-size: 0.8125rem;
}

.pagination-jump {
  display: none;
  align-items: center;
  gap: 0.25rem;
}

.pagination-jump-input {
  width: 4rem;
  height: 1.875rem;
  padding: 0 0.5rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-card);
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  font-variant-numeric: tabular-nums;
  outline: none;
  transition:
    border-color var(--operator-duration-fast) var(--operator-ease-out),
    box-shadow var(--operator-duration-fast) var(--operator-ease-out);
}

.pagination-jump-input:focus {
  border-color: rgb(var(--operator-accent-rgb) / 0.6);
  box-shadow: 0 0 0 3px rgb(var(--operator-accent-rgb) / 0.15);
}

.pagination-nav {
  display: flex;
  align-items: center;
  gap: 0.125rem;
  margin-left: auto;
}

.pagination-pages {
  display: none;
  align-items: center;
  gap: 0.125rem;
}

.pagination-compact {
  min-width: 3rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  font-variant-numeric: tabular-nums;
  text-align: center;
}

.pagination-button {
  display: inline-flex;
  min-width: 1.875rem;
  height: 1.875rem;
  align-items: center;
  justify-content: center;
  padding: 0 0.4375rem;
  border: 1px solid transparent;
  border-radius: var(--operator-radius);
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  font-variant-numeric: tabular-nums;
  font-weight: 500;
  transition:
    background-color var(--operator-duration-fast) var(--operator-ease-out),
    border-color var(--operator-duration-fast) var(--operator-ease-out),
    color var(--operator-duration-fast) var(--operator-ease-out),
    transform 120ms var(--operator-ease-out);
}

.pagination-step {
  border-color: var(--operator-border);
}

.pagination-button:hover:not(:disabled):not(.pagination-button-current) {
  background: var(--operator-muted);
  color: var(--operator-foreground);
}

.pagination-button:active:not(:disabled) {
  transform: scale(0.94);
}

.pagination-button:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: 1px;
}

.pagination-button:disabled {
  cursor: not-allowed;
  opacity: 0.4;
}

.pagination-button-current {
  background: var(--operator-raised);
  box-shadow: inset 0 0 0 1px var(--operator-border-strong);
  color: var(--operator-foreground);
}

.pagination-gap {
  display: inline-flex;
  min-width: 1.25rem;
  justify-content: center;
  color: var(--operator-subtle-foreground);
  font-size: 0.8125rem;
}

@media (min-width: 640px) {
  .pagination-jump {
    display: inline-flex;
  }

  .pagination-pages {
    display: inline-flex;
  }

  .pagination-compact {
    display: none;
  }
}

@media (max-width: 639px) {
  .pagination-bar {
    padding: 0.5rem 0.75rem;
  }

  .pagination-size-label {
    display: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .pagination-button {
    transition: none;
  }
}
</style>
