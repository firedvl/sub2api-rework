<template>
  <div class="relative" ref="containerRef">
    <button
      ref="triggerRef"
      type="button"
      @click="toggle"
      :disabled="disabled"
      :aria-expanded="isOpen"
      :aria-haspopup="true"
      :id="id"
      :aria-label="ariaLabel ?? 'Select option'"
      :aria-describedby="ariaDescribedby"
      :class="[
        'select-trigger operator-control',
        isOpen && 'select-trigger-open',
        isOpen && 'operator-control-open',
        error && 'select-trigger-error',
        disabled && 'select-trigger-disabled'
      ]"
      @keydown.down.prevent="onTriggerKeyDown"
      @keydown.up.prevent="onTriggerKeyDown"
      @keydown.enter.prevent="onTriggerKeyDown"
      @keydown.esc.prevent="onTriggerKeyDown"
    >
      <span class="select-value">
        <slot name="selected" :option="selectedOption">
          {{ selectedLabel }}
        </slot>
      </span>
      <span
        v-if="clearable && hasValue && !disabled"
        class="select-clear"
        role="button"
        tabindex="-1"
        aria-label="Clear selection"
        @click.stop="clearSelection"
        @mousedown.stop
        @keydown.enter.stop.prevent="clearSelection"
      >
        <Icon name="x" size="sm" />
      </span>
      <span class="select-icon">
        <Icon
          name="chevronDown"
          size="sm"
          :class="['transition-transform duration-normal ease-ui-out', isOpen && 'rotate-180']"
        />
      </span>
    </button>

    <!-- Teleport dropdown to body to escape stacking context -->
    <Teleport to="body">
      <Transition name="select-dropdown">
        <div
          v-if="isOpen"
          ref="dropdownRef"
          class="select-dropdown-portal operator-select-menu operator-menu"
          :class="[instanceId, dropdownPosition === 'top' && 'select-dropdown-up']"
          :style="dropdownStyle"
          role="listbox"
          tabindex="-1"
          @click.stop
          @mousedown.stop
          @keydown="onDropdownKeyDown"
        >
          <!-- Search input -->
          <div v-if="isSearchable" class="select-search">
            <Icon name="search" size="sm" class="text-gray-400" />
            <input
              ref="searchInputRef"
              v-model="searchQuery"
              type="text"
              :placeholder="searchPlaceholderText"
              :aria-label="searchPlaceholderText"
              class="select-search-input"
              @click.stop
            />
          </div>

          <!-- Options list -->
          <div class="select-options" ref="optionsListRef">
            <div
              v-for="(option, index) in filteredOptions"
              :key="`${typeof getOptionValue(option)}:${String(getOptionValue(option) ?? '')}`"
              role="option"
              :aria-selected="isSelected(option)"
              :aria-disabled="isOptionDisabled(option)"
              @click.stop="!isOptionDisabled(option) && selectOption(option)"
              @pointermove="handleOptionPointerMove(option, index)"
              :class="[
                'select-option operator-menu-item',
                isGroupHeaderOption(option) && 'select-option-group',
                isSelected(option) && 'select-option-selected',
                isOptionDisabled(option) && !isGroupHeaderOption(option) && 'select-option-disabled',
                focusedIndex === index && !isGroupHeaderOption(option) && 'select-option-focused'
              ]"
            >
              <slot name="option" :option="option" :selected="isSelected(option)">
                <Icon
                  v-if="option._creatable"
                  name="search"
                  size="sm"
                  class="flex-shrink-0 text-gray-400"
                />
                <span class="select-option-label" :class="option._creatable && 'italic text-gray-500 dark:text-dark-300'">{{ getOptionLabel(option) }}</span>
                <Icon
                  v-if="isSelected(option)"
                  name="check"
                  size="sm"
                  class="operator-menu-check"
                  :stroke-width="2"
                />
              </slot>
            </div>

            <!-- Empty state -->
            <div v-if="filteredOptions.length === 0" class="select-empty">
              {{ props.loading ? t('common.loading') : emptyTextDisplay }}
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

// Instance ID for unique click-outside detection
const instanceId = `select-${Math.random().toString(36).substring(2, 9)}`

export interface SelectOption {
  value: string | number | boolean | null
  label: string
  disabled?: boolean
  [key: string]: unknown
}

interface Props {
  modelValue: string | number | boolean | null | undefined
  options: SelectOption[] | Array<Record<string, unknown>>
  placeholder?: string
  disabled?: boolean
  error?: boolean
  searchable?: boolean | 'auto'
  searchPlaceholder?: string
  emptyText?: string
  valueKey?: string
  labelKey?: string
  creatable?: boolean
  creatablePrefix?: string
  clearable?: boolean
  id?: string
  ariaLabel?: string
  ariaDescribedby?: string
  /** 远程搜索模式：输入不在本地过滤 options，而是防抖后 emit('search', query)，由父组件请求数据更新 options */
  remote?: boolean
  /** 远程搜索模式下的加载态：options 为空时下拉显示 loading 文案 */
  loading?: boolean
}

interface Emits {
  (e: 'update:modelValue', value: string | number | boolean | null): void
  (e: 'change', value: string | number | boolean | null, option: SelectOption | null): void
  (e: 'search', query: string): void
}

const props = withDefaults(defineProps<Props>(), {
  disabled: false,
  error: false,
  searchable: 'auto',
  creatable: false,
  creatablePrefix: '',
  clearable: false,
  valueKey: 'value',
  labelKey: 'label',
  remote: false,
  loading: false
})

const emit = defineEmits<Emits>()

const isOpen = ref(false)
const searchQuery = ref('')
const focusedIndex = ref(-1)
const containerRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const searchInputRef = ref<HTMLInputElement | null>(null)
const dropdownRef = ref<HTMLElement | null>(null)
const optionsListRef = ref<HTMLElement | null>(null)
const dropdownPosition = ref<'bottom' | 'top'>('bottom')
const triggerRect = ref<DOMRect | null>(null)
const dropdownViewportPadding = 8
const dropdownMinimumWidth = 200

// i18n placeholders
const placeholderText = computed(() => props.placeholder ?? t('common.selectOption'))
const searchPlaceholderText = computed(() => props.searchPlaceholder ?? t('common.searchPlaceholder'))
const emptyTextDisplay = computed(() => props.emptyText ?? t('common.noOptionsFound'))

// 远程搜索的防抖间隔（对齐 OpenAIFastPolicyUserSelector 的 300ms 惯例）。
const REMOTE_SEARCH_DEBOUNCE_MS = 300
let remoteSearchTimer: ReturnType<typeof setTimeout> | null = null

const isSearchable = computed(() => {
  // 远程搜索模式始终显示搜索框（选项只是服务端结果的一页）。
  if (props.remote) return true
  if (props.searchable === 'auto') return props.options.length > 5
  return props.searchable
})

// Computed style for teleported dropdown
const dropdownStyle = computed(() => {
  if (!triggerRect.value) return {}

  const rect = triggerRect.value
  const viewportRight = Math.max(dropdownViewportPadding, window.innerWidth - dropdownViewportPadding)
  const left = Math.min(
    Math.max(dropdownViewportPadding, rect.left),
    viewportRight
  )
  const availableWidth = Math.max(0, viewportRight - left)
  const preferredMinWidth = Math.max(dropdownMinimumWidth, rect.width)
  const minWidth = Math.min(preferredMinWidth, availableWidth)
  const style: Record<string, string> = {
    position: 'fixed',
    left: `${left}px`,
    minWidth: `${minWidth}px`,
    maxWidth: `${availableWidth}px`,
    zIndex: '100000020'
  }

  if (dropdownPosition.value === 'top') {
    style.bottom = `${window.innerHeight - rect.top + 4}px`
  } else {
    style.top = `${rect.bottom + 4}px`
  }

  return style
})

const getOptionValue = (option: any): any => {
  if (typeof option === 'object' && option !== null) {
    return option[props.valueKey]
  }
  return option
}

const getOptionLabel = (option: any): string => {
  if (typeof option === 'object' && option !== null) {
    return String(option[props.labelKey] ?? '')
  }
  return String(option ?? '')
}

const isOptionDisabled = (option: any): boolean => {
  if (typeof option === 'object' && option !== null) {
    return !!option.disabled
  }
  return false
}

const isGroupHeaderOption = (option: any): boolean => {
  if (typeof option === 'object' && option !== null) {
    return option.kind === 'group'
  }
  return false
}

const selectedOption = computed(() => {
  return props.options.find((opt) => getOptionValue(opt) === props.modelValue) || null
})

const selectedLabel = computed(() => {
  if (selectedOption.value) {
    return getOptionLabel(selectedOption.value)
  }
  // In creatable mode, show the raw value if no matching option
  if (props.creatable && props.modelValue) {
    return String(props.modelValue)
  }
  return placeholderText.value
})

const hasValue = computed(
  () => props.modelValue !== null && props.modelValue !== undefined && props.modelValue !== ''
)

const filteredOptions = computed(() => {
  let opts = props.options as any[]
  // 远程搜索模式不在本地过滤（选项即服务端搜索结果的一页）。
  if (isSearchable.value && searchQuery.value && !props.remote) {
    const query = searchQuery.value.toLowerCase()
    opts = opts.filter((opt) => {
      // Match label
      if (getOptionLabel(opt).toLowerCase().includes(query)) return true
      // Also match description if present
      if (opt.description && String(opt.description).toLowerCase().includes(query)) return true
      return false
    })
    // In creatable mode, always prepend a fuzzy search option
    if (props.creatable && searchQuery.value.trim()) {
      const trimmed = searchQuery.value.trim()
      const prefix = props.creatablePrefix || t('common.search')
      opts = [{ [props.valueKey]: trimmed, [props.labelKey]: `${prefix} "${trimmed}"`, _creatable: true }, ...opts]
    }
  }
  return opts
})

const isSelected = (option: any): boolean => {
  return getOptionValue(option) === props.modelValue
}

const findNextEnabledIndex = (startIndex: number): number => {
  const opts = filteredOptions.value
  if (opts.length === 0) return -1
  for (let offset = 0; offset < opts.length; offset++) {
    const idx = (startIndex + offset) % opts.length
    if (!isOptionDisabled(opts[idx])) return idx
  }
  return -1
}

const findPrevEnabledIndex = (startIndex: number): number => {
  const opts = filteredOptions.value
  if (opts.length === 0) return -1
  for (let offset = 0; offset < opts.length; offset++) {
    const idx = (startIndex - offset + opts.length) % opts.length
    if (!isOptionDisabled(opts[idx])) return idx
  }
  return -1
}

watch(filteredOptions, () => {
  if (!isOpen.value) return
  focusedIndex.value = findNextEnabledIndex(0)
  if (focusedIndex.value >= 0) scrollToFocused()
})

const handleOptionPointerMove = (option: any, index: number) => {
  if (isOptionDisabled(option) || isGroupHeaderOption(option)) return
  focusedIndex.value = index
}

const focusSelectedOption = () => {
  const selectedIndex = filteredOptions.value.findIndex(isSelected)
  focusedIndex.value = selectedIndex >= 0 && !isOptionDisabled(filteredOptions.value[selectedIndex])
    ? selectedIndex
    : findNextEnabledIndex(0)
}

// Update trigger rect periodically while open to follow scroll/resize
const updateTriggerRect = () => {
  if (containerRef.value) {
    triggerRect.value = containerRef.value.getBoundingClientRect()
  }
}

const calculateDropdownPosition = () => {
  if (!containerRef.value) return
  updateTriggerRect()

  nextTick(() => {
    if (!dropdownRef.value || !triggerRect.value) return
    const dropdownHeight = dropdownRef.value.offsetHeight || 240
    const spaceBelow = window.innerHeight - triggerRect.value.bottom
    const spaceAbove = triggerRect.value.top

    if (spaceBelow < dropdownHeight && spaceAbove > dropdownHeight) {
      dropdownPosition.value = 'top'
    } else {
      dropdownPosition.value = 'bottom'
    }
  })
}

const toggle = () => {
  if (props.disabled) return
  if (isOpen.value) {
    isOpen.value = false
    return
  }
  focusSelectedOption()
  isOpen.value = true
}

watch(isOpen, (open) => {
  if (open) {
    calculateDropdownPosition()
    focusSelectedOption()
    if (isSearchable.value) {
      nextTick(() => searchInputRef.value?.focus())
    } else {
      nextTick(() => dropdownRef.value?.focus())
    }
    // Add scroll listener to update position
    window.addEventListener('scroll', updateTriggerRect, { capture: true, passive: true })
    window.addEventListener('resize', calculateDropdownPosition)
  } else {
    searchQuery.value = ''
    focusedIndex.value = -1
    // 关闭时取消仍在排队的远程搜索（避免关闭后尾随 emit 一次 search(''))。
    if (remoteSearchTimer) {
      clearTimeout(remoteSearchTimer)
      remoteSearchTimer = null
    }
    window.removeEventListener('scroll', updateTriggerRect, { capture: true })
    window.removeEventListener('resize', calculateDropdownPosition)
  }
})

// 远程搜索：输入防抖后交给父组件请求（!isOpen 抑制关闭重置 searchQuery 触发的空 query）。
watch(searchQuery, (query) => {
  if (!props.remote || !isOpen.value) return
  if (remoteSearchTimer) clearTimeout(remoteSearchTimer)
  remoteSearchTimer = setTimeout(() => {
    remoteSearchTimer = null
    emit('search', query.trim())
  }, REMOTE_SEARCH_DEBOUNCE_MS)
})

const selectOption = (option: any) => {
  const value = getOptionValue(option) ?? null
  emit('update:modelValue', value)
  emit('change', value, option)
  isOpen.value = false
  triggerRef.value?.focus()
}

const clearSelection = () => {
  if (props.disabled) return
  emit('update:modelValue', null)
  emit('change', null, null)
}

const onDropdownKeyDown = (e: KeyboardEvent) => {
  switch (e.key) {
    case 'ArrowDown':
      e.preventDefault()
      focusedIndex.value = findNextEnabledIndex(focusedIndex.value + 1)
      if (focusedIndex.value >= 0) scrollToFocused()
      break
    case 'ArrowUp':
      e.preventDefault()
      focusedIndex.value = findPrevEnabledIndex(focusedIndex.value - 1)
      if (focusedIndex.value >= 0) scrollToFocused()
      break
    case 'Enter':
      e.preventDefault()
      if (focusedIndex.value >= 0 && focusedIndex.value < filteredOptions.value.length) {
        const opt = filteredOptions.value[focusedIndex.value]
        if (!isOptionDisabled(opt)) selectOption(opt)
      }
      break
    case 'Escape':
      e.preventDefault()
      isOpen.value = false
      triggerRef.value?.focus()
      break
    case 'Tab':
      isOpen.value = false
      break
  }
}

const onTriggerKeyDown = (event: KeyboardEvent) => {
  if (props.disabled) return
  if (!isOpen.value && event.key === 'Escape') return
  if (!isOpen.value) {
    isOpen.value = true
    return
  }
  onDropdownKeyDown(event)
}

const scrollToFocused = () => {
  nextTick(() => {
    const list = optionsListRef.value
    if (!list) return
    const focusedEl = list.children[focusedIndex.value] as HTMLElement
    if (!focusedEl) return

    if (focusedEl.offsetTop < list.scrollTop) {
      list.scrollTop = focusedEl.offsetTop
    } else if (focusedEl.offsetTop + focusedEl.offsetHeight > list.scrollTop + list.offsetHeight) {
      list.scrollTop = focusedEl.offsetTop + focusedEl.offsetHeight - list.offsetHeight
    }
  })
}

const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  // Check if click is inside THIS specific instance's dropdown or trigger
  const isInDropdown = !!target.closest(`.${instanceId}`)
  const isInTrigger = containerRef.value?.contains(target)

  if (!isInDropdown && !isInTrigger && isOpen.value) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  window.removeEventListener('scroll', updateTriggerRect, { capture: true })
  window.removeEventListener('resize', calculateDropdownPosition)
  if (remoteSearchTimer) {
    clearTimeout(remoteSearchTimer)
    remoteSearchTimer = null
  }
})
</script>

<style scoped>
.select-trigger {
  display: flex;
  width: 100%;
  min-height: 2.25rem;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.4375rem 0.625rem 0.4375rem 0.75rem;
  border: 1px solid var(--operator-border-strong);
  border-radius: var(--operator-radius);
  background: var(--operator-card);
  color: var(--operator-foreground);
  box-shadow: var(--operator-shadow-xs);
  cursor: pointer;
  font-size: 0.875rem;
  line-height: 1.25rem;
  text-align: left;
  transition:
    border-color var(--operator-duration-fast) var(--operator-ease-out),
    box-shadow var(--operator-duration-fast) var(--operator-ease-out),
    background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.select-trigger:hover:not(.select-trigger-open):not(:focus-visible) {
  border-color: color-mix(in srgb, var(--operator-border-strong) 70%, var(--operator-foreground));
}

.select-trigger:focus-visible,
.select-trigger-open {
  border-color: var(--operator-accent);
  outline: none;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--operator-accent) 22%, transparent);
}

.select-trigger-error,
.select-trigger-error:hover {
  border-color: var(--operator-destructive);
}

.select-trigger-error:focus-visible {
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--operator-destructive) 22%, transparent);
}

.select-trigger-disabled,
.select-trigger-disabled:hover {
  cursor: not-allowed;
  border-color: var(--operator-border);
  background: var(--operator-muted);
  color: var(--operator-muted-foreground);
  box-shadow: none;
}

.select-value {
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.select-icon {
  display: flex;
  flex-shrink: 0;
  color: var(--operator-subtle-foreground);
}

.select-clear {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: var(--operator-radius-sm);
  color: var(--operator-subtle-foreground);
  cursor: pointer;
  transition: color var(--operator-duration-fast) var(--operator-ease-out);
}

.select-clear:hover {
  color: var(--operator-foreground);
}
</style>

<style>
.select-dropdown-portal.operator-select-menu {
  width: max-content;
  min-width: 200px;
  overflow: hidden;
  border: 1px solid var(--operator-popup-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-popup-surface);
  box-shadow: var(--operator-popup-shadow);
  color: var(--operator-popup-foreground);
  pointer-events: auto !important;
  transform-origin: top left;
}

.select-dropdown-portal.operator-select-menu.select-dropdown-up {
  transform-origin: bottom left;
}

.select-dropdown-portal.operator-select-menu .select-search {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--operator-popup-border);
  background: var(--operator-popup-search);
  color: var(--operator-subtle-foreground);
}

.select-dropdown-portal.operator-select-menu .select-search-input {
  min-width: 0;
  flex: 1 1 auto;
  background: transparent;
  color: var(--operator-popup-foreground);
  font-size: 0.875rem;
  outline: none;
}

.select-dropdown-portal.operator-select-menu .select-search-input::placeholder {
  color: var(--operator-placeholder);
}

.select-dropdown-portal.operator-select-menu .select-options {
  max-height: 20rem;
  overflow-y: auto;
  padding: 0.25rem;
  outline: none;
  background: var(--operator-popup-surface);
}

.select-dropdown-portal.operator-select-menu .select-option {
  display: flex;
  min-height: 2.25rem;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.375rem 0.625rem;
  border-radius: var(--operator-radius-sm);
  background: transparent;
  color: var(--operator-popup-foreground);
  cursor: pointer;
  font-size: 0.875rem;
  pointer-events: auto !important;
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.select-dropdown-portal.operator-select-menu .select-option:hover,
.select-dropdown-portal.operator-select-menu .select-option-focused {
  background: var(--operator-popup-hover);
}

.select-dropdown-portal.operator-select-menu .select-option-selected,
.select-dropdown-portal.operator-select-menu .select-option-selected:hover {
  background: var(--operator-popup-selected);
  color: var(--operator-popup-foreground);
}

.select-dropdown-portal.operator-select-menu .select-option-disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.select-dropdown-portal.operator-select-menu .select-option-group,
.select-dropdown-portal.operator-select-menu .select-option-group:hover {
  min-height: 1.75rem;
  margin-top: 0.25rem;
  background: transparent;
  color: var(--operator-popup-muted);
  cursor: default;
  font-size: 0.75rem;
  font-weight: 600;
  user-select: none;
}

.select-dropdown-portal.operator-select-menu .select-option-label {
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.select-dropdown-portal.operator-select-menu .select-empty {
  padding: 1.5rem 1rem;
  background: transparent;
  color: var(--operator-popup-muted);
  font-size: 0.875rem;
  text-align: center;
}

.select-dropdown-enter-active {
  transition:
    opacity var(--operator-duration) var(--operator-ease-out),
    transform var(--operator-duration) var(--operator-ease-out);
}

.select-dropdown-leave-active {
  transition:
    opacity var(--operator-duration-fast) var(--operator-ease-in),
    transform var(--operator-duration-fast) var(--operator-ease-in);
}

.select-dropdown-enter-from,
.select-dropdown-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}

.select-dropdown-up.select-dropdown-enter-from,
.select-dropdown-up.select-dropdown-leave-to {
  transform: translateY(4px) scale(0.98);
}
</style>
