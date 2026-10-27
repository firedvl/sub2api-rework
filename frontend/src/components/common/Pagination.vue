<template>
  <div class="pagination-bar">
    <div class="flex flex-1 items-center justify-between gap-3 sm:hidden">
      <!-- Mobile pagination -->
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="page === 1"
        @click="goToPage(page - 1)"
      >
        {{ t('pagination.previous') }}
      </button>
      <span class="pagination-info">
        {{ t('pagination.pageOf', { page, total: totalPages }) }}
      </span>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="page === totalPages"
        @click="goToPage(page + 1)"
      >
        {{ t('pagination.next') }}
      </button>
    </div>

    <div class="hidden flex-wrap items-center justify-between gap-x-6 gap-y-2 sm:flex sm:flex-1">
      <div class="flex flex-wrap items-center gap-x-5 gap-y-2">
        <p class="pagination-info">
          {{ t('pagination.showing') }}
          <span class="pagination-figure">{{ fromItem }}</span>
          {{ t('pagination.to') }}
          <span class="pagination-figure">{{ toItem }}</span>
          {{ t('pagination.of') }}
          <span class="pagination-figure">{{ total }}</span>
          {{ t('pagination.results') }}
        </p>

        <div v-if="showPageSizeSelector" class="flex items-center gap-2">
          <span class="pagination-info">{{ t('pagination.perPage') }}:</span>
          <div class="page-size-select w-[4.75rem]">
            <Select
              :model-value="pageSize"
              :options="pageSizeSelectOptions"
              :aria-label="t('pagination.perPage')"
              @update:model-value="handlePageSizeChange"
            />
          </div>
        </div>

        <div v-if="showJump" class="flex items-center gap-2">
          <span class="pagination-info">{{ t('pagination.jumpTo') }}</span>
          <input
            v-model="jumpPage"
            type="number"
            min="1"
            :max="totalPages"
            class="input h-8 min-h-0 w-20 py-1"
            :placeholder="t('pagination.jumpPlaceholder')"
            @keyup.enter="submitJump"
          />
          <button type="button" class="btn btn-ghost btn-sm" @click="submitJump">
            {{ t('pagination.jumpAction') }}
          </button>
        </div>
      </div>

      <nav class="flex items-center gap-1" :aria-label="t('pagination.label')">
        <button
          type="button"
          class="pagination-button"
          :disabled="page === 1"
          :aria-label="t('pagination.previous')"
          @click="goToPage(page - 1)"
        >
          <Icon name="chevronLeft" size="sm" />
        </button>

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

        <button
          type="button"
          class="pagination-button"
          :disabled="page === totalPages"
          :aria-label="t('pagination.next')"
          @click="goToPage(page + 1)"
        >
          <Icon name="chevronRight" size="sm" />
        </button>
      </nav>
    </div>
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

const totalPages = computed(() => Math.ceil(props.total / props.pageSize))

const fromItem = computed(() => {
  if (props.total === 0) return 0
  return (props.page - 1) * props.pageSize + 1
})

const toItem = computed(() => {
  const to = props.page * props.pageSize
  return to > props.total ? props.total : to
})

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
  const maxVisible = 7
  const total = totalPages.value

  if (total <= maxVisible) {
    // Show all pages if total is small
    for (let i = 1; i <= total; i++) {
      pages.push(i)
    }
  } else {
    // Always show first page
    pages.push(1)

    const start = Math.max(2, props.page - 2)
    const end = Math.min(total - 1, props.page + 2)

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
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.625rem 1rem;
  border-top: 1px solid var(--operator-border);
  background: var(--operator-card);
}

.pagination-info {
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  font-variant-numeric: tabular-nums;
}

.pagination-figure {
  color: var(--operator-foreground);
  font-weight: 500;
}

.pagination-button {
  display: inline-flex;
  min-width: 2rem;
  height: 2rem;
  align-items: center;
  justify-content: center;
  padding: 0 0.5rem;
  border: 1px solid transparent;
  border-radius: var(--operator-radius);
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  font-variant-numeric: tabular-nums;
  font-weight: 500;
  transition:
    background-color var(--operator-duration-fast) var(--operator-ease-out),
    color var(--operator-duration-fast) var(--operator-ease-out);
}

.pagination-button:hover:not(:disabled):not(.pagination-button-current) {
  background: var(--operator-muted);
  color: var(--operator-foreground);
}

.pagination-button:disabled {
  cursor: not-allowed;
  opacity: 0.4;
}

.pagination-button-current {
  border-color: color-mix(in srgb, var(--operator-accent) 35%, transparent);
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
}

.pagination-gap {
  display: inline-flex;
  min-width: 1.5rem;
  justify-content: center;
  color: var(--operator-subtle-foreground);
  font-size: 0.8125rem;
}

.page-size-select :deep(.select-trigger) {
  min-height: 2rem;
  padding-top: 0.25rem;
  padding-bottom: 0.25rem;
  font-size: 0.8125rem;
}
</style>
