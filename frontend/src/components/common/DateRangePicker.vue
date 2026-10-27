<template>
  <div class="relative" ref="containerRef">
    <button
      type="button"
      @click="toggle"
      :class="['date-picker-trigger operator-control', isOpen && 'date-picker-trigger-open operator-control-open']"
    >
      <span class="date-picker-icon">
        <Icon name="calendar" size="sm" />
      </span>
      <span class="date-picker-value">
        {{ displayValue }}
      </span>
      <span class="date-picker-chevron">
        <Icon
          name="chevronDown"
          size="sm"
          :class="['transition-transform duration-200', isOpen && 'rotate-180']"
        />
      </span>
    </button>

    <Transition name="date-picker-dropdown">
      <div v-if="isOpen" class="date-picker-dropdown operator-menu">
        <!-- Quick presets -->
        <div class="date-picker-presets">
          <button
            v-for="preset in presets"
            :key="preset.value"
            @click="selectPreset(preset)"
            :aria-pressed="isPresetActive(preset)"
            :class="['date-picker-preset operator-menu-item', isPresetActive(preset) && 'date-picker-preset-active']"
          >
            {{ t(preset.labelKey) }}
          </button>
        </div>

        <div class="date-picker-divider operator-menu-divider"></div>

        <!-- Custom date range inputs -->
        <div class="date-picker-custom">
          <div class="date-picker-field">
            <label class="date-picker-label">{{ t('dates.startDate') }}</label>
            <input
              type="date"
              v-model="localStartDate"
              :max="localEndDate || tomorrow()"
              class="date-picker-input operator-control"
              @change="onDateChange"
            />
          </div>
          <div class="date-picker-separator">
            <Icon name="arrowRight" size="sm" class="text-gray-400" />
          </div>
          <div class="date-picker-field">
            <label class="date-picker-label">{{ t('dates.endDate') }}</label>
            <input
              type="date"
              v-model="localEndDate"
              :min="localStartDate"
              :max="tomorrow()"
              class="date-picker-input operator-control"
              @change="onDateChange"
            />
          </div>
        </div>

        <!-- Apply button -->
        <div class="date-picker-actions">
          <button @click="apply" class="date-picker-apply operator-menu-primary">
            {{ t('dates.apply') }}
          </button>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

interface DatePreset {
  labelKey: string
  value: string
  getRange: () => { start: string; end: string }
}

interface Props {
  startDate: string
  endDate: string
}

interface Emits {
  (e: 'update:startDate', value: string): void
  (e: 'update:endDate', value: string): void
  (e: 'change', range: { startDate: string; endDate: string; preset: string | null }): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t, locale } = useI18n()

const isOpen = ref(false)
const containerRef = ref<HTMLElement | null>(null)
const localStartDate = ref(props.startDate)
const localEndDate = ref(props.endDate)
const activePreset = ref<string | null>('last24Hours')

const today = () => formatDateToString(new Date())

// Tomorrow's date - used for max date to handle timezone differences
// When user is in a timezone behind the server, "today" on server might be "tomorrow" locally
const tomorrow = () => {
  const d = new Date()
  d.setDate(d.getDate() + 1)
  return formatDateToString(d)
}

// Helper function to format date to YYYY-MM-DD using local timezone
const formatDateToString = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const presets: DatePreset[] = [
  {
    labelKey: 'dates.today',
    value: 'today',
    getRange: () => {
      const t = today()
      return { start: t, end: t }
    }
  },
  {
    labelKey: 'dates.yesterday',
    value: 'yesterday',
    getRange: () => {
      const d = new Date()
      d.setDate(d.getDate() - 1)
      const yesterday = formatDateToString(d)
      return { start: yesterday, end: yesterday }
    }
  },
  {
    labelKey: 'dates.last24Hours',
    value: 'last24Hours',
    getRange: () => {
      const end = new Date()
      const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
      return {
        start: formatDateToString(start),
        end: formatDateToString(end)
      }
    }
  },
  {
    labelKey: 'dates.last7Days',
    value: '7days',
    getRange: () => {
      const end = today()
      const d = new Date()
      d.setDate(d.getDate() - 6)
      const start = formatDateToString(d)
      return { start, end }
    }
  },
  {
    labelKey: 'dates.last14Days',
    value: '14days',
    getRange: () => {
      const end = today()
      const d = new Date()
      d.setDate(d.getDate() - 13)
      const start = formatDateToString(d)
      return { start, end }
    }
  },
  {
    labelKey: 'dates.last30Days',
    value: '30days',
    getRange: () => {
      const end = today()
      const d = new Date()
      d.setDate(d.getDate() - 29)
      const start = formatDateToString(d)
      return { start, end }
    }
  },
  {
    labelKey: 'dates.thisMonth',
    value: 'thisMonth',
    getRange: () => {
      const now = new Date()
      const start = formatDateToString(new Date(now.getFullYear(), now.getMonth(), 1))
      return { start, end: today() }
    }
  },
  {
    labelKey: 'dates.lastMonth',
    value: 'lastMonth',
    getRange: () => {
      const now = new Date()
      const start = formatDateToString(new Date(now.getFullYear(), now.getMonth() - 1, 1))
      const end = formatDateToString(new Date(now.getFullYear(), now.getMonth(), 0))
      return { start, end }
    }
  }
]

const displayValue = computed(() => {
  if (activePreset.value) {
    const preset = presets.find((p) => p.value === activePreset.value)
    if (preset) return t(preset.labelKey)
  }

  if (localStartDate.value && localEndDate.value) {
    if (localStartDate.value === localEndDate.value) {
      return formatDate(localStartDate.value)
    }
    return `${formatDate(localStartDate.value)} - ${formatDate(localEndDate.value)}`
  }

  return t('dates.selectDateRange')
})

const formatDate = (dateStr: string): string => {
  const date = new Date(dateStr + 'T00:00:00')
  const dateLocale = locale.value === 'zh' ? 'zh-CN' : 'en-US'
  return date.toLocaleDateString(dateLocale, { month: 'short', day: 'numeric' })
}

const isPresetActive = (preset: DatePreset): boolean => {
  return activePreset.value === preset.value
}

const selectPreset = (preset: DatePreset) => {
  const range = preset.getRange()
  localStartDate.value = range.start
  localEndDate.value = range.end
  activePreset.value = preset.value
}

const onDateChange = () => {
  // Check if current dates match any preset
  activePreset.value = null
  for (const preset of presets) {
    const range = preset.getRange()
    if (range.start === localStartDate.value && range.end === localEndDate.value) {
      activePreset.value = preset.value
      break
    }
  }
}

const toggle = () => {
  isOpen.value = !isOpen.value
}

const apply = () => {
  emit('update:startDate', localStartDate.value)
  emit('update:endDate', localEndDate.value)
  emit('change', {
    startDate: localStartDate.value,
    endDate: localEndDate.value,
    preset: activePreset.value
  })
  isOpen.value = false
}

const handleClickOutside = (event: MouseEvent) => {
  if (containerRef.value && !containerRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

const handleEscape = (event: KeyboardEvent) => {
  if (event.key === 'Escape' && isOpen.value) {
    isOpen.value = false
  }
}

// Restore the applied range after dismissal, including parent updates from Apply.
watch(isOpen, (open) => {
  if (open) return
  localStartDate.value = props.startDate
  localEndDate.value = props.endDate
  onDateChange()
}, { flush: 'post' })

// Sync local state with props
watch(
  () => props.startDate,
  (val) => {
    localStartDate.value = val
    onDateChange()
  }
)

watch(
  () => props.endDate,
  (val) => {
    localEndDate.value = val
    onDateChange()
  }
)

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  document.addEventListener('keydown', handleEscape)
  // Initialize active preset detection
  onDateChange()
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  document.removeEventListener('keydown', handleEscape)
})
</script>

<style scoped>
.date-picker-trigger {
  display: flex;
  min-height: 2.25rem;
  align-items: center;
  gap: 0.5rem;
  padding: 0 0.75rem;
  border: 1px solid var(--operator-border-strong);
  border-radius: var(--operator-radius);
  background: var(--operator-card);
  color: var(--operator-foreground);
  box-shadow: var(--operator-shadow-xs);
  cursor: pointer;
  font-size: 0.875rem;
  transition:
    border-color var(--operator-duration-fast) var(--operator-ease-out),
    box-shadow var(--operator-duration-fast) var(--operator-ease-out);
}

.date-picker-trigger:hover:not(.date-picker-trigger-open):not(:focus-visible) {
  border-color: color-mix(in srgb, var(--operator-border-strong) 70%, var(--operator-foreground));
}

.date-picker-trigger:focus-visible,
.date-picker-trigger-open {
  border-color: var(--operator-accent);
  outline: none;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--operator-accent) 22%, transparent);
}

.date-picker-icon,
.date-picker-chevron {
  display: flex;
  color: var(--operator-subtle-foreground);
}

.date-picker-value {
  font-variant-numeric: tabular-nums;
  font-weight: 500;
}

.date-picker-dropdown {
  position: absolute;
  left: 0;
  z-index: 100;
  min-width: 20rem;
  margin-top: 0.375rem;
  overflow: hidden;
  border: 1px solid var(--operator-popup-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-popup-surface);
  box-shadow: var(--operator-popup-shadow);
  transform-origin: top left;
}

.date-picker-presets {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.125rem;
  padding: 0.375rem;
}

.date-picker-preset {
  min-height: 2rem;
  padding: 0 0.625rem;
  border-radius: var(--operator-radius-sm);
  color: var(--operator-popup-foreground);
  font-size: 0.8125rem;
  font-weight: 500;
  text-align: left;
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.date-picker-preset:hover {
  background: var(--operator-popup-hover);
}

.date-picker-preset-active,
.date-picker-preset-active:hover {
  background: var(--operator-popup-selected);
  color: var(--operator-popup-foreground);
}

.date-picker-divider {
  border-top: 1px solid var(--operator-popup-border);
}

.date-picker-custom {
  display: flex;
  align-items: flex-end;
  gap: 0.5rem;
  padding: 0.75rem;
}

.date-picker-field {
  flex: 1 1 0;
}

.date-picker-label {
  display: block;
  margin-bottom: 0.25rem;
  color: var(--operator-popup-muted);
  font-size: 0.75rem;
  font-weight: 500;
}

.date-picker-input {
  width: 100%;
  min-height: 2rem;
  padding: 0 0.5rem;
  border: 1px solid var(--operator-border-strong);
  border-radius: var(--operator-radius-sm);
  background: var(--operator-card);
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  font-variant-numeric: tabular-nums;
}

.date-picker-input:focus {
  border-color: var(--operator-accent);
  outline: none;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--operator-accent) 22%, transparent);
}

.date-picker-input::-webkit-calendar-picker-indicator {
  cursor: pointer;
  opacity: 0.6;
}

.date-picker-input::-webkit-calendar-picker-indicator:hover {
  opacity: 1;
}

.date-picker-separator {
  display: flex;
  align-items: center;
  justify-content: center;
  padding-bottom: 0.375rem;
}

.date-picker-actions {
  display: flex;
  justify-content: flex-end;
  padding: 0 0.75rem 0.75rem;
}

.date-picker-apply {
  min-height: 2rem;
  padding: 0 0.875rem;
  border-radius: var(--operator-radius);
  font-size: 0.8125rem;
  font-weight: 500;
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.date-picker-dropdown-enter-active {
  transition:
    opacity var(--operator-duration) var(--operator-ease-out),
    transform var(--operator-duration) var(--operator-ease-out);
}

.date-picker-dropdown-leave-active {
  transition:
    opacity var(--operator-duration-fast) var(--operator-ease-in),
    transform var(--operator-duration-fast) var(--operator-ease-in);
}

.date-picker-dropdown-enter-from,
.date-picker-dropdown-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}
</style>
