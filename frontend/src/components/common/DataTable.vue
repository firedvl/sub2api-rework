<template>
  <div v-if="!isDesktopViewport" class="data-list-shell">
    <div v-if="selectable && !loading && data && data.length" class="data-list-bar">
      <label class="data-list-select-all">
        <input
          type="checkbox"
          class="data-check"
          :checked="allVisibleSelected"
          :indeterminate="someVisibleSelected"
          data-test="select-all-mobile"
          @change="toggleAllVisible(($event.target as HTMLInputElement).checked)"
        />
        <span>{{ t('common.selectAll') }}</span>
      </label>
      <span v-if="selectedKeys.length" class="data-list-count">{{ selectedKeys.length }}</span>
    </div>

    <div class="data-list" :class="{ 'ui-rows-enter': animateRows }">
      <template v-if="loading">
        <div v-for="i in 4" :key="`skeleton-${i}`" class="data-card" aria-hidden="true">
          <div class="skeleton h-4" :style="{ width: skeletonWidth(i, 0) }"></div>
          <div v-for="(column, colIndex) in bodyColumns.slice(0, 3)" :key="column.key" class="data-card-field">
            <div class="skeleton h-3 w-16"></div>
            <div class="skeleton h-3" :style="{ width: skeletonWidth(i, colIndex + 1, 40) }"></div>
          </div>
        </div>
      </template>

      <div v-else-if="!data || data.length === 0" class="data-card data-card-empty">
        <slot name="empty">
          <div class="ui-empty">
            <span class="ui-empty-icon"><Icon name="inbox" size="md" /></span>
            <b>{{ t('empty.noData') }}</b>
          </div>
        </slot>
      </div>

      <template v-else>
        <div
          v-for="(row, index) in sortedData"
          :key="resolveRowKey(row, index)"
          class="data-card"
          :class="{
            'is-clickable cursor-pointer': clickableRows,
            'data-card-selected': selectable && isRowSelected(row, index)
          }"
          :tabindex="clickableRows ? 0 : undefined"
          @click="onRowClick($event, row)"
          @keydown="onRowKeydown($event, row)"
        >
          <div v-if="headColumn || selectable" class="data-card-top">
            <input
              v-if="selectable"
              type="checkbox"
              class="data-check"
              :checked="isRowSelected(row, index)"
              :aria-label="getRowSelectionLabel(row, index)"
              data-test="select-row"
              @click.stop
              @change="toggleRowSelection(row, index, ($event.target as HTMLInputElement).checked)"
            />
            <div v-if="headColumn" :data-field="headColumn.key" class="data-card-head min-w-0 flex-1">
              <div class="min-w-0 max-w-full">
                <slot :name="`cell-${headColumn.key}`" :row="row" :value="row[headColumn.key]" :expanded="actionsExpanded">
                  {{ headColumn.formatter ? headColumn.formatter(row[headColumn.key], row) : row[headColumn.key] }}
                </slot>
              </div>
            </div>
          </div>
          <div
            v-for="column in bodyColumns"
            :key="column.key"
            :data-field="column.key"
            class="data-card-field min-w-0"
          >
            <span class="data-card-label">{{ column.label }}</span>
            <div class="data-card-value min-w-0 max-w-full">
              <slot :name="`cell-${column.key}`" :row="row" :value="row[column.key]" :expanded="actionsExpanded">
                {{ column.formatter ? column.formatter(row[column.key], row) : row[column.key] }}
              </slot>
            </div>
          </div>
          <div v-if="hasActionsColumn" class="data-card-actions">
            <slot name="cell-actions" :row="row" :value="row['actions']" :expanded="actionsExpanded"></slot>
          </div>
        </div>
      </template>
    </div>
  </div>

  <div
    v-else
    ref="tableWrapperRef"
    class="table-wrapper"
    :class="{
      'actions-expanded': actionsExpanded,
      'is-scrollable': isScrollable,
      'is-scrolled-left': isScrolledLeft
    }"
    @scroll="handleTableScroll"
  >
    <table class="w-full min-w-max divide-y divide-gray-200 dark:divide-dark-700">
      <thead class="table-header">
        <tr>
          <th
            v-if="selectable"
            scope="col"
            :class="[
              'sticky-header-cell w-11 min-w-11 px-3 py-3 text-center',
              stickyFirstColumn && 'sticky-col sticky-col-left-first'
            ]"
          >
            <input
              type="checkbox"
              class="data-check"
              :checked="allVisibleSelected"
              :indeterminate="someVisibleSelected"
              :aria-label="t('common.selectAll')"
              data-test="select-all"
              @change="toggleAllVisible(($event.target as HTMLInputElement).checked)"
            />
          </th>
          <th
            v-for="(column, index) in columns"
            :key="column.key"
            scope="col"
            :aria-sort="column.sortable ? getColumnAriaSort(column.key) : undefined"
            :tabindex="column.sortable ? 0 : undefined"
            :class="[
              'sticky-header-cell py-2.5 text-left text-xs font-medium text-gray-500 dark:text-dark-400',
              getAdaptivePaddingClass(),
              { 'sortable-header cursor-pointer select-none': column.sortable },
              { 'is-sorted': column.sortable && sortKey === column.key },
              getStickyColumnClass(column, index),
              column.class
            ]"
            @click="column.sortable && handleSort(column.key)"
            @keydown="column.sortable && onHeaderKeydown($event, column.key)"
          >
            <div :class="['flex items-center gap-1', getHeaderContentAlignmentClass(column)]">
              <slot
                :name="`header-${column.key}`"
                :column="column"
                :sort-key="sortKey"
                :sort-order="sortOrder"
              >
                <span>{{ column.label }}</span>
              </slot>
              <span
                v-if="column.sortable"
                class="sort-indicator"
                :class="getSortIndicatorClass(column.key)"
                :data-sort-state="getSortState(column.key)"
                aria-hidden="true"
              >
                <!-- Idle columns show a faint up/down glyph; the sorted column shows one chevron that flips. -->
                <svg v-if="sortKey !== column.key" class="sort-idle" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M3.75 4.75 6 2.5l2.25 2.25M3.75 7.25 6 9.5l2.25-2.25" />
                </svg>
                <svg v-else class="sort-active" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M3.25 7.25 6 4.5l2.75 2.75" />
                </svg>
              </span>
            </div>
          </th>
        </tr>
      </thead>
      <tbody class="table-body" :class="{ 'ui-rows-enter': animateRows }">
        <!-- Loading skeleton -->
        <tr v-if="loading" v-for="i in 6" :key="i" class="skeleton-row" aria-hidden="true">
          <td
            v-if="selectable"
            :class="[
              'w-11 min-w-11 px-3 py-3.5',
              stickyFirstColumn && 'sticky-col sticky-col-left-first'
            ]"
          >
            <div class="skeleton mx-auto h-4 w-4"></div>
          </td>
          <td v-for="(column, colIndex) in columns" :key="column.key" :class="['whitespace-nowrap py-3.5', getAdaptivePaddingClass()]">
            <div class="skeleton h-3.5" :style="{ width: skeletonWidth(i, colIndex) }"></div>
          </td>
        </tr>

        <!-- Empty state -->
        <tr v-else-if="!data || data.length === 0">
          <td
            :colspan="tableColumnCount"
            class="p-0 text-center text-gray-500 dark:text-dark-400"
          >
            <div
              data-test="desktop-empty-state"
              class="sticky left-0 py-4"
              :style="{ width: tableViewportWidth ? `${tableViewportWidth}px` : '100%' }"
            >
              <slot name="empty">
                <div class="ui-empty">
                  <span class="ui-empty-icon"><Icon name="inbox" size="md" /></span>
                  <b>{{ t('empty.noData') }}</b>
                </div>
              </slot>
            </div>
          </td>
        </tr>

        <!-- Data rows: windowed when large, fully rendered when small (shared row/cell template) -->
        <template v-else>
          <tr v-if="virtualPaddingTop > 0" aria-hidden="true">
            <td :colspan="tableColumnCount"
                :style="{ height: virtualPaddingTop + 'px', padding: 0, border: 'none' }">
            </td>
          </tr>
          <tr
            v-for="item in renderRows"
            :key="resolveRowKey(item.row, item.index)"
            :data-row-id="resolveRowKey(item.row, item.index)"
            :data-index="item.index"
            :ref="item.measure ? measureElement : undefined"
            class="data-row"
            :class="{
              'is-clickable cursor-pointer': clickableRows,
              'data-row-selected': selectable && isRowSelected(item.row, item.index)
            }"
            :tabindex="clickableRows ? 0 : undefined"
            @click="onRowClick($event, item.row)"
            @keydown="onRowKeydown($event, item.row)"
          >
            <td
              v-if="selectable"
              :class="[
                'w-11 min-w-11 px-3 py-3 text-center',
                stickyFirstColumn && 'sticky-col sticky-col-left-first'
              ]"
            >
              <input
                type="checkbox"
                class="data-check"
                :checked="isRowSelected(item.row, item.index)"
                :aria-label="getRowSelectionLabel(item.row, item.index)"
                data-test="select-row"
                @click.stop
                @change="toggleRowSelection(item.row, item.index, ($event.target as HTMLInputElement).checked)"
              />
            </td>
            <td
              v-for="(column, colIndex) in columns"
              :key="column.key"
              :class="[
                'whitespace-nowrap py-3 text-sm text-gray-900 dark:text-gray-100',
                getAdaptivePaddingClass(),
                getStickyColumnClass(column, colIndex),
                column.class
              ]"
            >
              <slot :name="`cell-${column.key}`"
                    :row="item.row"
                    :value="item.row[column.key]"
                    :expanded="actionsExpanded">
                {{ column.formatter
                   ? column.formatter(item.row[column.key], item.row)
                   : item.row[column.key] }}
              </slot>
            </td>
          </tr>
          <tr v-if="virtualPaddingBottom > 0" aria-hidden="true">
            <td :colspan="tableColumnCount"
                :style="{ height: virtualPaddingBottom + 'px', padding: 0, border: 'none' }">
            </td>
          </tr>
        </template>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useVirtualizer, observeElementRect as observeElementRectDefault } from '@tanstack/vue-virtual'
import { useI18n } from 'vue-i18n'
import type { Column } from './types'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const desktopViewportQuery = '(min-width: 768px)'
const isDesktopViewport = ref(
  typeof window === 'undefined' ? true : window.matchMedia(desktopViewportQuery).matches
)

const emit = defineEmits<{
  sort: [key: string, order: 'asc' | 'desc']
  rowClick: [row: any]
  'update:selectedKeys': [keys: Array<string | number>]
  selectionChange: [keys: Array<string | number>]
}>()

// 表格容器引用
const tableWrapperRef = ref<HTMLElement | null>(null)
const tableViewportWidth = ref(0)
const isScrollable = ref(false)
const isScrolledLeft = ref(false)
const actionsColumnNeedsExpanding = ref(false)

// --- 虚拟滚动「整表空白」根治 ---
// 根因:本组件根 .table-wrapper 为 flex:1 / min-h-0,高度由父级 flex 链决定。@tanstack 虚拟化器
// 仅在 observeElementRect 回调里写 scrollRect;一旦该回调读到 0 高度(加载瞬间 flex 未结算,或
// 滚动中动态行高校正触发的 reflow),scrollRect 被钉死为 0 → calculateRange 返回 null → 整表空白。
// 对策(见下方 virtualizer 选项):
//   1) 覆写 observeElementRect,直接丢弃 height<=0 的读数,scrollRect 永不被钉成 0;
//   2) initialRect 给一屏兜底高度,首个有效读数到来前也有行可渲染,绝不空白。
// 兜底高度:表格区域大致 = 视口高度 - 顶栏/外边距/筛选/分页 ≈ 320px
const estimatedViewportHeight = () => {
  if (typeof window === 'undefined') return 600
  return Math.max(window.innerHeight - 320, 400)
}

// 覆写默认 observeElementRect:过滤掉 0 高度读数(根治整表空白的关键)
const observeElementRectNonZero = (
  instance: any,
  cb: (rect: { width: number; height: number }) => void
) => observeElementRectDefault(instance, (rect) => {
  if (rect.height > 0) cb(rect)
})

// 检查是否可滚动
const checkScrollable = () => {
  if (tableWrapperRef.value) {
    tableViewportWidth.value = tableWrapperRef.value.clientWidth
    isScrollable.value = tableWrapperRef.value.scrollWidth > tableWrapperRef.value.clientWidth
    isScrolledLeft.value = tableWrapperRef.value.scrollLeft > 0
  }
}

const handleTableScroll = () => {
  if (!tableWrapperRef.value) return
  isScrolledLeft.value = tableWrapperRef.value.scrollLeft > 0
}

// 检查操作列是否需要展开
const checkActionsColumnWidth = () => {
  if (!props.expandableActions) {
    actionsColumnNeedsExpanding.value = false
    actionsExpanded.value = false
    return
  }
  if (!tableWrapperRef.value) return

  // 查找第一行的操作列单元格
  const firstActionCell = tableWrapperRef.value.querySelector('tbody tr:first-child td:last-child')
  if (!firstActionCell) return

  // 查找操作列内容的容器div
  const actionsContainer = firstActionCell.querySelector('div')
  if (!actionsContainer) return

  // 临时展开以测量完整宽度
  const wasExpanded = actionsExpanded.value
  actionsExpanded.value = true

  // 等待DOM更新
  nextTick(() => {
    // 测量所有按钮的总宽度
    const actionItems = actionsContainer.querySelectorAll('button, a, [role="button"]')
    if (actionItems.length <= 2) {
      actionsColumnNeedsExpanding.value = false
      actionsExpanded.value = wasExpanded
      return
    }

    // 计算所有按钮的总宽度（包括gap）
    let totalWidth = 0
    actionItems.forEach((item, index) => {
      totalWidth += (item as HTMLElement).offsetWidth
      if (index < actionItems.length - 1) {
        totalWidth += 4 // gap-1 = 4px
      }
    })

    // 获取单元格可用宽度（减去padding）
    const cellWidth = (firstActionCell as HTMLElement).clientWidth - 32 // 减去左右padding

    // 如果总宽度超过可用宽度，需要展开功能
    actionsColumnNeedsExpanding.value = totalWidth > cellWidth

    // 恢复原来的展开状态
    actionsExpanded.value = wasExpanded
  })
}

// 监听尺寸变化
let resizeObserver: ResizeObserver | null = null
let resizeHandler: (() => void) | null = null
let desktopViewportMediaQuery: MediaQueryList | null = null
let desktopViewportListener: ((event: MediaQueryListEvent) => void) | null = null

const detachDesktopTableTracking = () => {
  resizeObserver?.disconnect()
  resizeObserver = null
  if (resizeHandler) {
    window.removeEventListener('resize', resizeHandler)
    resizeHandler = null
  }
}

const attachDesktopTableTracking = () => {
  checkScrollable()
  checkActionsColumnWidth()
  if (tableWrapperRef.value && typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(() => {
      checkScrollable()
      checkActionsColumnWidth()
    })
    resizeObserver.observe(tableWrapperRef.value)
  } else {
    // 降级方案：不支持 ResizeObserver 时使用 window resize
    resizeHandler = () => {
      checkScrollable()
      checkActionsColumnWidth()
    }
    window.addEventListener('resize', resizeHandler)
  }
}

onMounted(() => {
  if (typeof window !== 'undefined') {
    desktopViewportMediaQuery = window.matchMedia(desktopViewportQuery)
    isDesktopViewport.value = desktopViewportMediaQuery.matches
    desktopViewportListener = (event: MediaQueryListEvent) => {
      isDesktopViewport.value = event.matches
    }
    if (typeof desktopViewportMediaQuery.addEventListener === 'function') {
      desktopViewportMediaQuery.addEventListener('change', desktopViewportListener)
    } else {
      desktopViewportMediaQuery.addListener(desktopViewportListener)
    }
  }
})

onUnmounted(() => {
  clearTimeout(animateRowsTimer)
  detachDesktopTableTracking()
  if (desktopViewportMediaQuery && desktopViewportListener) {
    if (typeof desktopViewportMediaQuery.removeEventListener === 'function') {
      desktopViewportMediaQuery.removeEventListener('change', desktopViewportListener)
    } else {
      desktopViewportMediaQuery.removeListener(desktopViewportListener)
    }
    desktopViewportListener = null
  }
  desktopViewportMediaQuery = null
})

interface Props {
  columns: Column[]
  data: any[]
  loading?: boolean
  stickyFirstColumn?: boolean
  stickyActionsColumn?: boolean
  expandableActions?: boolean
  actionsCount?: number // 操作按钮总数，用于判断是否需要展开功能
  rowKey?: string | ((row: any) => string | number)
  /**
   * Default sort configuration (only applied when there is no persisted sort state)
   */
  defaultSortKey?: string
  defaultSortOrder?: 'asc' | 'desc'
  /**
   * Persist sort state (key + order) to localStorage using this key.
   * If provided, DataTable will load the stored sort state on mount.
   */
  sortStorageKey?: string
  /**
   * Enable server-side sorting mode. When true, clicking sort headers
   * will emit 'sort' events instead of performing client-side sorting.
   */
  serverSideSort?: boolean
  /** Emit 'rowClick' on row/card click and show pointer cursor (interactive cells should @click.stop) */
  clickableRows?: boolean
  /** Estimated row height in px for the virtualizer (default 56) */
  estimateRowHeight?: number
  /** Number of rows to render beyond the visible area (default 5) */
  overscan?: number
  /**
   * Only virtualize when the row count exceeds this threshold (default 100).
   * Smaller lists render in full, avoiding the scroll-compensation jank caused by
   * estimated-vs-actual row heights when rows have variable height.
   */
  virtualizeThreshold?: number
  /** Enable controlled row selection. Stable row keys are strongly recommended. */
  selectable?: boolean
  /** Selected row keys. Keys outside the current data page are preserved. */
  selectedKeys?: Array<string | number>
  /** Accessible label for a row selection checkbox. */
  selectionLabel?: string | ((row: any) => string)
}

const props = withDefaults(defineProps<Props>(), {
  loading: false,
  stickyFirstColumn: true,
  stickyActionsColumn: true,
  expandableActions: true,
  defaultSortOrder: 'asc',
  serverSideSort: false,
  selectable: false,
  selectedKeys: () => []
})

const sortKey = ref<string>('')
const sortOrder = ref<'asc' | 'desc'>('asc')
const actionsExpanded = ref(false)

type PersistedSortState = {
  key: string
  order: 'asc' | 'desc'
}

const collator = new Intl.Collator(undefined, {
  numeric: true,
  sensitivity: 'base'
})

const getSortableKeys = () => {
  const keys = new Set<string>()
  for (const col of props.columns) {
    if (col.sortable) keys.add(col.key)
  }
  return keys
}

const normalizeSortKey = (candidate: string) => {
  if (!candidate) return ''
  const sortableKeys = getSortableKeys()
  return sortableKeys.has(candidate) ? candidate : ''
}

const normalizeSortOrder = (candidate: any): 'asc' | 'desc' => {
  return candidate === 'desc' ? 'desc' : 'asc'
}

const readPersistedSortState = (): PersistedSortState | null => {
  if (!props.sortStorageKey) return null
  try {
    const raw = localStorage.getItem(props.sortStorageKey)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<PersistedSortState>
    const key = normalizeSortKey(typeof parsed.key === 'string' ? parsed.key : '')
    if (!key) return null
    return { key, order: normalizeSortOrder(parsed.order) }
  } catch (e) {
    console.error('[DataTable] Failed to read persisted sort state:', e)
    return null
  }
}

const writePersistedSortState = (state: PersistedSortState) => {
  if (!props.sortStorageKey) return
  try {
    localStorage.setItem(props.sortStorageKey, JSON.stringify(state))
  } catch (e) {
    console.error('[DataTable] Failed to persist sort state:', e)
  }
}

const resolveInitialSortState = (): PersistedSortState | null => {
  const persisted = readPersistedSortState()
  if (persisted) return persisted

  const key = normalizeSortKey(props.defaultSortKey || '')
  if (!key) return null
  return { key, order: normalizeSortOrder(props.defaultSortOrder) }
}

const applySortState = (state: PersistedSortState | null) => {
  if (!state) return
  sortKey.value = state.key
  sortOrder.value = state.order
}

const getSortState = (key: string): 'none' | 'asc' | 'desc' =>
  sortKey.value === key ? sortOrder.value : 'none'

const getSortIndicatorClass = (key: string) => {
  const state = getSortState(key)
  if (state === 'asc') return 'is-active'
  if (state === 'desc') return 'is-active is-desc'
  return ''
}

const onHeaderKeydown = (event: KeyboardEvent, key: string) => {
  if (event.target !== event.currentTarget) return
  if (event.key !== 'Enter' && event.key !== ' ') return
  event.preventDefault()
  handleSort(key)
}

// Rows and cards ignore clicks that come from their own controls or the
// mobile actions footer, so a button in a cell never also opens the row.
const interactiveSelector = 'button, a, input, select, textarea, label, [role="button"], [role="menuitem"], .data-card-actions'

const onRowClick = (event: MouseEvent, row: any) => {
  if (!props.clickableRows) return
  const target = event.target as Element | null
  const control = target?.closest(interactiveSelector)
  if (control && control !== event.currentTarget && (event.currentTarget as Element).contains(control)) return
  emit('rowClick', row)
}

const onRowKeydown = (event: KeyboardEvent, row: any) => {
  if (!props.clickableRows || event.target !== event.currentTarget) return
  if (event.key !== 'Enter' && event.key !== ' ') return
  event.preventDefault()
  emit('rowClick', row)
}

// Skeleton bars vary in width so the loading state reads as rows, not a grid.
const skeletonWidths = [72, 48, 60, 84, 40, 66]
const skeletonWidth = (row: number, column: number, scale = 100) =>
  `${Math.round((skeletonWidths[(row + column * 2) % skeletonWidths.length] * scale) / 100)}%`

// Rows rise in on the first data paint only; later pages and refreshes stay still.
const animateRows = ref(false)
let rowsAnimated = false
let animateRowsTimer: ReturnType<typeof setTimeout> | undefined
watch(
  () => !props.loading && (props.data?.length ?? 0) > 0,
  (ready) => {
    if (!ready || rowsAnimated) return
    rowsAnimated = true
    animateRows.value = true
    animateRowsTimer = setTimeout(() => {
      animateRows.value = false
    }, 700)
  },
  { immediate: true }
)

const getColumnAriaSort = (key: string) => {
  if (sortKey.value !== key) return 'none'
  return sortOrder.value === 'asc' ? 'ascending' : 'descending'
}

const getHeaderContentAlignmentClass = (column: Column) => {
  const className = column.class || ''
  if (className.includes('text-center')) return 'justify-center'
  if (className.includes('text-right')) return 'justify-end'
  return 'justify-start'
}

const isNullishOrEmpty = (value: any) => value === null || value === undefined || value === ''

const toFiniteNumberOrNull = (value: any): number | null => {
  if (typeof value === 'number') return Number.isFinite(value) ? value : null
  if (typeof value === 'boolean') return value ? 1 : 0
  if (typeof value === 'string') {
    const trimmed = value.trim()
    if (!trimmed) return null
    const n = Number(trimmed)
    return Number.isFinite(n) ? n : null
  }
  return null
}

const toSortableString = (value: any): string => {
  if (value === null || value === undefined) return ''
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  if (value instanceof Date) return value.toISOString()
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

const compareSortValues = (a: any, b: any): number => {
  const aEmpty = isNullishOrEmpty(a)
  const bEmpty = isNullishOrEmpty(b)
  if (aEmpty && bEmpty) return 0
  if (aEmpty) return 1
  if (bEmpty) return -1

  const aNum = toFiniteNumberOrNull(a)
  const bNum = toFiniteNumberOrNull(b)
  if (aNum !== null && bNum !== null) {
    if (aNum === bNum) return 0
    return aNum < bNum ? -1 : 1
  }

  const aStr = toSortableString(a)
  const bStr = toSortableString(b)
  const res = collator.compare(aStr, bStr)
  if (res === 0) return 0
  return res < 0 ? -1 : 1
}
const resolveStableRowKey = (row: any): string | number | undefined => {
  if (typeof props.rowKey === 'function') {
    const key = props.rowKey(row)
    return key ?? undefined
  }
  if (typeof props.rowKey === 'string' && props.rowKey) {
    const key = row?.[props.rowKey]
    return key ?? undefined
  }
  const key = row?.id
  return key ?? undefined
}

const resolveRowKey = (row: any, index: number) => resolveStableRowKey(row) ?? index

const dataColumns = computed(() => props.columns.filter((column) => column.key !== 'actions'))
// Mobile cards lead with the first real column as a title; the rest become label/value rows.
const headColumn = computed(() => dataColumns.value.find((column) => column.key !== 'select') ?? null)
const bodyColumns = computed(() => dataColumns.value.filter((column) => column !== headColumn.value))
const columnsSignature = computed(() =>
  props.columns.map((column) => `${column.key}:${column.sortable ? '1' : '0'}`).join('|')
)

watch(
  isDesktopViewport,
  async (isDesktop) => {
    detachDesktopTableTracking()
    if (!isDesktop) return
    await nextTick()
    attachDesktopTableTracking()
  },
  { immediate: true, flush: 'post' }
)

// 数据/列变化时重新检查滚动状态
// 注意：不能监听 actionsExpanded，因为 checkActionsColumnWidth 会临时修改它，会导致无限循环
watch(
  [() => props.data.length, columnsSignature],
  async () => {
    await nextTick()
    checkScrollable()
    checkActionsColumnWidth()
  },
  { flush: 'post' }
)

// 单独监听展开状态变化，只更新滚动状态
watch(actionsExpanded, async () => {
  await nextTick()
  checkScrollable()
})

const handleSort = (key: string) => {
  let newOrder: 'asc' | 'desc' = 'asc'
  if (sortKey.value === key) {
    newOrder = sortOrder.value === 'asc' ? 'desc' : 'asc'
  }

  if (props.serverSideSort) {
    // Server-side sort mode: emit event and update internal state for UI feedback
    sortKey.value = key
    sortOrder.value = newOrder
    emit('sort', key, newOrder)
  } else {
    // Client-side sort mode: just update internal state
    sortKey.value = key
    sortOrder.value = newOrder
  }
}

const sortedData = computed(() => {
  // Server-side sort mode: return data as-is (server handles sorting)
  if (props.serverSideSort || !sortKey.value || !props.data) return props.data

  const key = sortKey.value
  const order = sortOrder.value

  // Stable sort (tie-break with original index) to avoid jitter when values are equal.
  return props.data
    .map((row, index) => ({ row, index }))
    .sort((a, b) => {
      const cmp = compareSortValues(a.row?.[key], b.row?.[key])
      if (cmp !== 0) return order === 'asc' ? cmp : -cmp
      return a.index - b.index
    })
    .map(item => item.row)
})

const tableColumnCount = computed(() => props.columns.length + (props.selectable ? 1 : 0))
const selectedKeySet = computed(() => new Set(props.selectedKeys))
const visibleRowKeys = computed(() =>
  (sortedData.value ?? []).map((row, index) => resolveRowKey(row, index))
)
const allVisibleSelected = computed(() =>
  visibleRowKeys.value.length > 0
  && visibleRowKeys.value.every((key) => selectedKeySet.value.has(key))
)
const someVisibleSelected = computed(() => {
  if (allVisibleSelected.value) return false
  return visibleRowKeys.value.some((key) => selectedKeySet.value.has(key))
})

const emitSelection = (next: Set<string | number>) => {
  const keys = Array.from(next)
  emit('update:selectedKeys', keys)
  emit('selectionChange', keys)
}

const isRowSelected = (row: any, index: number) =>
  selectedKeySet.value.has(resolveRowKey(row, index))

const getRowSelectionLabel = (row: any, index: number) => {
  if (typeof props.selectionLabel === 'function') return props.selectionLabel(row)
  if (props.selectionLabel) return props.selectionLabel
  return `${t('common.selectOption')} ${resolveRowKey(row, index)}`
}

const toggleRowSelection = (row: any, index: number, checked: boolean) => {
  const next = new Set(props.selectedKeys)
  const key = resolveRowKey(row, index)
  if (checked) next.add(key)
  else next.delete(key)
  emitSelection(next)
}

const toggleAllVisible = (checked: boolean) => {
  const next = new Set(props.selectedKeys)
  for (const key of visibleRowKeys.value) {
    if (checked) next.add(key)
    else next.delete(key)
  }
  emitSelection(next)
}

// --- Virtual scrolling ---
// 是否启用虚拟化:仅桌面端且行数超过阈值时开启。小列表全量渲染,彻底绕开虚拟器的
// 估算/测量/滚动补偿链路,消除可变行高导致的滚动抖动。
const shouldVirtualize = computed(() =>
  isDesktopViewport.value && (sortedData.value?.length ?? 0) > (props.virtualizeThreshold ?? 100)
)

const rowVirtualizer = useVirtualizer(computed(() => ({
  count: shouldVirtualize.value ? (sortedData.value?.length ?? 0) : 0,
  getScrollElement: () => tableWrapperRef.value,
  // 用行主键(与模板 :key 一致)而非默认的 index 作为 itemSizeCache 键,
  // 这样排序/筛选/跨阈值来回都能复用正确的已测行高,而不是残留的按 index 缓存 → 消除高度校正抖动。
  getItemKey: (index: number) => {
    const row = sortedData.value?.[index]
    return row != null ? resolveRowKey(row, index) : index
  },
  estimateSize: () => props.estimateRowHeight ?? 56,
  overscan: props.overscan ?? 5,
  // 兜底高度:首个有效高度读数到来前,先按一屏渲染,避免空白帧
  initialRect: { width: 0, height: estimatedViewportHeight() },
  // 关键:过滤 0 高度读数,杜绝 scrollRect 被钉成 0 → calculateRange 返回 null → 整表空白
  observeElementRect: observeElementRectNonZero,
  // 把测量类 ResizeObserver 回调批到 rAF,避免滚动中同步 reflow 风暴导致的校正抖动/空白
  useAnimationFrameWithResizeObserver: true,
})))

const virtualItems = computed(() => rowVirtualizer.value.getVirtualItems())

const virtualPaddingTop = computed(() => {
  const items = virtualItems.value
  return items.length > 0 ? items[0].start : 0
})

const virtualPaddingBottom = computed(() => {
  const items = virtualItems.value
  if (items.length === 0) return 0
  return rowVirtualizer.value.getTotalSize() - items[items.length - 1].end
})

const measureElement = (el: any) => {
  if (el) {
    rowVirtualizer.value.measureElement(el as Element)
  }
}

type RowIdentityToken = string | number | object | symbol

const rowIdentityKeys = computed<RowIdentityToken[]>(() =>
  (sortedData.value ?? []).map((row) => {
    const stableKey = resolveStableRowKey(row)
    if (stableKey !== undefined) return stableKey

    // Object references survive pure reordering but change across page/filter results.
    // Primitive rows have no stable identity, so force conservative invalidation.
    return row !== null && typeof row === 'object' ? row : Symbol('unstable-row')
  })
)

const hasSameRowIdentitySet = (
  current: RowIdentityToken[],
  previous: RowIdentityToken[]
) => {
  if (current.length !== previous.length) return false
  const currentKeys = new Set(current)
  const previousKeys = new Set(previous)
  // Duplicate keys make row-to-cache ownership ambiguous, even when the unique
  // key set looks unchanged (for example [1, 1, 2] -> [1, 2, 2]).
  if (currentKeys.size !== current.length || previousKeys.size !== previous.length) return false
  return [...currentKeys].every(key => previousKeys.has(key))
}

watch(
  rowIdentityKeys,
  (current, previous) => {
    if (hasSameRowIdentitySet(current, previous)) return

    // The virtualizer owns caches across option updates. A new page/filter result
    // must release detached rows and sizes, while pure reordering keeps them.
    rowVirtualizer.value.measureElement(null)
    rowVirtualizer.value.measure()
  },
  { flush: 'post' }
)

// 统一的渲染行列表:虚拟化开启时只取窗口内的行(需 measure 交给虚拟器测量),
// 关闭时取全部行(无需测量)。模板据此渲染,两种模式共用同一套单元格结构。
const renderRows = computed<Array<{ index: number; row: any; measure: boolean }>>(() => {
  const data = sortedData.value ?? []
  if (shouldVirtualize.value) {
    return virtualItems.value.map(vr => ({ index: vr.index, row: data[vr.index], measure: true }))
  }
  return data.map((row, index) => ({ index, row, measure: false }))
})

const hasActionsColumn = computed(() => {
  return props.columns.some(column => column.key === 'actions')
})

const hasSelectColumn = computed(() => {
  return props.columns.length > 0 && props.columns[0].key === 'select'
})

// 生成固定列的 CSS 类
const getStickyColumnClass = (column: Column, index: number) => {
  const classes: string[] = []

  if (props.stickyFirstColumn) {
    if (props.selectable) {
      if (index === 0) {
        classes.push('sticky-col sticky-col-left-second')
      }
    // 如果第一列是勾选列，固定前两列（勾选+名称）
    } else if (hasSelectColumn.value) {
      if (index === 0) {
        classes.push('sticky-col sticky-col-left-first')
      } else if (index === 1) {
        classes.push('sticky-col sticky-col-left-second')
      }
    } else {
      // 否则只固定第一列
      if (index === 0) {
        classes.push('sticky-col sticky-col-left')
      }
    }
  }

  // 操作列固定（最后一列）
  if (props.stickyActionsColumn && column.key === 'actions') {
    classes.push('sticky-col sticky-col-right')
  }

  return classes.join(' ')
}

// 根据列数自适应调整内边距
const getAdaptivePaddingClass = () => {
  const columnCount = props.columns.length

  // 列数越多，内边距越小
  if (columnCount >= 10) {
    return 'px-2' // 8px
  } else if (columnCount >= 7) {
    return 'px-3' // 12px
  } else if (columnCount >= 5) {
    return 'px-4' // 16px
  } else {
    return 'px-6' // 24px (原始值)
  }
}

// Init + keep persisted sort state consistent with current columns
const didInitSort = ref(false)

onMounted(() => {
  const initial = resolveInitialSortState()
  applySortState(initial)
  didInitSort.value = true
})

watch(
  columnsSignature,
  () => {
    // If current sort key is no longer sortable/visible, fall back to default/persisted.
    const normalized = normalizeSortKey(sortKey.value)
    if (!sortKey.value) {
      const initial = resolveInitialSortState()
      applySortState(initial)
      return
    }

    if (!normalized) {
      const fallback = resolveInitialSortState()
      if (fallback) {
        applySortState(fallback)
      } else {
        sortKey.value = ''
        sortOrder.value = 'asc'
      }
    }
  },
  { flush: 'post' }
)

watch(
  [sortKey, sortOrder],
  ([nextKey, nextOrder]) => {
    if (!didInitSort.value) return
    if (!props.sortStorageKey) return
    const key = normalizeSortKey(nextKey)
    if (!key) return
    writePersistedSortState({ key, order: normalizeSortOrder(nextOrder) })
  },
  { flush: 'post' }
)

defineExpose({
  virtualizer: rowVirtualizer,
  shouldVirtualize,
  sortedData,
  resolveRowKey,
  tableWrapperEl: tableWrapperRef,
})
</script>

<style scoped>
.table-wrapper {
  --select-col-width: 44px;
  position: relative;
  overflow-x: auto;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
  isolation: isolate;
  font-variant-numeric: tabular-nums;
  scrollbar-width: thin;
  scrollbar-color: var(--operator-border-strong) transparent;
}

.table-wrapper::-webkit-scrollbar {
  width: 10px;
  height: 10px;
}

.table-wrapper::-webkit-scrollbar-thumb {
  border: 2px solid transparent;
  border-radius: 999px;
  background: var(--operator-border-strong);
  background-clip: padding-box;
}

.table-wrapper::-webkit-scrollbar-thumb:hover {
  background-color: var(--operator-subtle-foreground);
}

/* The header stays above scrolled rows. */
.table-wrapper .table-header {
  position: sticky;
  top: 0;
  z-index: 200;
  background-color: var(--table-header-background);
}

.sticky-header-cell {
  position: sticky;
  top: 0;
  z-index: 210;
  border-bottom: 1px solid var(--operator-border);
  background-color: var(--table-header-background);
  white-space: nowrap;
}

.sortable-header {
  transition: color var(--operator-duration-fast) var(--operator-ease-out);
}

.sortable-header:hover {
  color: var(--operator-foreground);
}

.table-body {
  position: relative;
  z-index: 0;
  background: var(--table-body-background);
}

.table-body > tr + tr > td {
  border-top: 1px solid var(--operator-border-subtle);
}

.data-row {
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.data-row:hover {
  background: var(--table-row-hover-background);
}

.data-row-selected,
.data-row-selected:hover {
  background: var(--table-row-selected-background);
}

.sticky-col {
  position: sticky;
  z-index: 20;
}

.sticky-col-left {
  left: 0;
}

.sticky-col-left-first {
  left: 0;
  width: var(--select-col-width);
  min-width: var(--select-col-width);
  max-width: var(--select-col-width);
}

.sticky-col-left-second {
  left: var(--select-col-width);
}

.sticky-col-right {
  right: 0;
}

.sticky-header-cell.sticky-col {
  z-index: 220;
}

tbody .sticky-col {
  background-color: var(--table-body-background);
}

tbody tr:hover .sticky-col {
  background-color: var(--table-row-hover-background);
}

tbody tr.data-row-selected .sticky-col {
  background-color: var(--table-row-selected-background);
}

/* Edge shadows appear only while a sticky column overlaps scrolled content. */
.is-scrollable.is-scrolled-left .sticky-col-left::after,
.is-scrollable.is-scrolled-left .sticky-col-left-second::after {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 12px;
  background: linear-gradient(to right, rgb(0 0 0 / 0.07), transparent);
  content: '';
  pointer-events: none;
  transform: translateX(100%);
}

.is-scrollable .sticky-col-right::before {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 12px;
  background: linear-gradient(to left, rgb(0 0 0 / 0.07), transparent);
  content: '';
  pointer-events: none;
  transform: translateX(-100%);
}

.dark .is-scrollable.is-scrolled-left .sticky-col-left::after,
.dark .is-scrollable.is-scrolled-left .sticky-col-left-second::after {
  background: linear-gradient(to right, rgb(0 0 0 / 0.35), transparent);
}

.dark .is-scrollable .sticky-col-right::before {
  background: linear-gradient(to left, rgb(0 0 0 / 0.35), transparent);
}

/* Sort indicator: faint glyph on idle columns, one accent chevron that flips. */
.sort-indicator {
  display: inline-grid;
  width: 0.875rem;
  height: 0.875rem;
  flex: none;
  place-items: center;
  color: var(--operator-subtle-foreground);
  opacity: 0.45;
  transition:
    opacity var(--operator-duration-fast) var(--operator-ease-out),
    color var(--operator-duration-fast) var(--operator-ease-out);
}

.sort-indicator svg {
  width: 100%;
  height: 100%;
}

.sortable-header:hover .sort-indicator,
.sortable-header:focus-visible .sort-indicator {
  opacity: 0.9;
}

.sort-indicator.is-active {
  color: var(--operator-accent-text);
  opacity: 1;
}

.sort-indicator .sort-active {
  transition: transform 220ms var(--operator-spring);
}

.sort-indicator.is-desc .sort-active {
  transform: rotate(180deg);
}

.sortable-header.is-sorted {
  color: var(--operator-foreground);
}

.sortable-header:focus-visible,
.data-row.is-clickable:focus-visible,
.data-card.is-clickable:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: -2px;
}

/* Selection checkboxes: native, accent-tinted, with a soft focus ring and press. */
.data-check {
  width: 0.9375rem;
  height: 0.9375rem;
  flex: none;
  margin: 0;
  accent-color: var(--operator-accent);
  cursor: pointer;
  vertical-align: middle;
  transition: transform 120ms var(--operator-ease-out);
}

.data-check:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: 2px;
}

.data-check:active {
  transform: scale(0.88);
}

.skeleton-row td {
  border-top-color: transparent !important;
}

/* Narrow layouts: rows stack into one surface, each a titled block of label/value pairs. */
.data-list-shell {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.5rem;
}

.data-list-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0 0.25rem;
}

.data-list-select-all {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  font-weight: 500;
}

.data-list-count {
  min-width: 1.375rem;
  padding: 0 0.4375rem;
  border-radius: 999px;
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
  font-size: 0.75rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  line-height: 1.375rem;
  text-align: center;
}

.data-list {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-card);
  box-shadow: var(--operator-shadow-xs);
}

/* Inside a redesigned card the card is already the surface. */
/* Written fully inside :global(); Vue drops anything that follows :global(). */
:global(.ui-card .data-list),
:global(.viz-card .data-list) {
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.data-card {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.375rem;
  padding: 0.75rem 0.875rem;
  font-variant-numeric: tabular-nums;
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.data-card + .data-card {
  border-top: 1px solid var(--operator-border-subtle);
}

.data-card.is-clickable:active {
  background: var(--table-row-hover-background);
}

.data-card-selected {
  background: var(--table-row-selected-background);
}

.data-card-empty {
  padding: 0;
}

.data-card-top {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.625rem;
  padding-bottom: 0.125rem;
}

.data-card-head {
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 500;
  overflow-wrap: anywhere;
}

.data-card-field {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  min-height: 1.5rem;
}

.data-card-label {
  flex: none;
  padding-top: 0.125rem;
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.data-card-value {
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  text-align: right;
  overflow-wrap: anywhere;
}

.data-card-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.25rem;
  margin-top: 0.25rem;
  padding-top: 0.5rem;
  border-top: 1px dashed var(--operator-border-subtle);
}

@media (prefers-reduced-motion: reduce) {
  .sort-indicator .sort-active,
  .data-check {
    transition: none;
  }
}
</style>
