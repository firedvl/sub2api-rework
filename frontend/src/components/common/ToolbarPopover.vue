<template>
  <div ref="root" class="toolbar-popover" @keydown="onKeydown">
    <button
      ref="trigger"
      type="button"
      :class="iconOnly ? 'app-icon-button toolbar-popover-icon' : 'ui-chip'"
      :aria-label="iconOnly || count ? accessibleLabel : undefined"
      :aria-expanded="open"
      :aria-haspopup="menu ? 'menu' : 'dialog'"
      :title="iconOnly ? label : undefined"
      :data-test="testId"
      @click="toggle"
    >
      <Icon :name="icon as any" size="sm" />
      <span v-if="!iconOnly">{{ label }}</span>
      <span v-if="count" class="toolbar-popover-count" aria-hidden="true">{{ count }}</span>
    </button>
    <!-- Rendered on <body> so scrolling tables and cards never clip the panel. -->
    <Teleport to="body">
      <Transition name="menu-pop">
        <div
          v-if="open"
          ref="panel"
          class="toolbar-popover-panel"
          :class="[placement.up ? 'is-up' : 'is-down', menu ? 'is-menu' : 'is-form']"
          :style="panelStyle"
          :role="menu ? 'menu' : 'dialog'"
          :aria-label="label"
          @click="onPanelClick"
          @keydown="onKeydown"
        >
          <slot :close="close" />
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'

/**
 * Toolbar popover: a "Filters" chip or an overflow (⋯) button that opens a
 * panel anchored to the trigger (teleported to <body>, flips up near the bottom). Menu mode closes after any `[role="menuitem"]` click and
 * supports arrow-key navigation; form mode keeps the panel open for edits.
 */
const props = withDefaults(defineProps<{
  label: string
  icon?: string
  /** Active filter count shown on the trigger. */
  count?: number
  iconOnly?: boolean
  align?: 'left' | 'right'
  menu?: boolean
  width?: string
  testId?: string
}>(), {
  icon: 'filter',
  count: 0,
  iconOnly: false,
  align: 'right',
  menu: false,
  width: undefined,
  testId: undefined,
})

const emit = defineEmits<{ open: []; close: [] }>()

const open = ref(false)
const root = ref<HTMLElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const panel = ref<HTMLElement | null>(null)
const placement = ref({ top: 0, left: 0, up: false, ready: false })

const accessibleLabel = computed(() => (props.count ? `${props.label} (${props.count})` : props.label))
const panelStyle = computed(() => ({
  width: props.width ?? (props.menu ? undefined : '18rem'),
  top: `${placement.value.top}px`,
  left: `${placement.value.left}px`,
  visibility: placement.value.ready ? undefined : ('hidden' as const),
}))

const GUTTER = 8
const OFFSET = 6

/** Anchors the fixed panel to the trigger, flipping up and clamping to the viewport. */
function place() {
  const anchor = trigger.value?.getBoundingClientRect()
  const box = panel.value?.getBoundingClientRect()
  if (!anchor || !box) return
  const viewportWidth = window.innerWidth
  const viewportHeight = window.innerHeight
  let left = props.align === 'left' ? anchor.left : anchor.right - box.width
  left = Math.min(Math.max(left, GUTTER), Math.max(GUTTER, viewportWidth - GUTTER - box.width))
  const below = viewportHeight - anchor.bottom - OFFSET - GUTTER
  const above = anchor.top - OFFSET - GUTTER
  const up = box.height > below && above > below
  const top = up ? Math.max(GUTTER, anchor.top - OFFSET - box.height) : anchor.bottom + OFFSET
  placement.value = { top, left, up, ready: true }
}

const focusableSelector = props.menu
  ? '[role="menuitem"]:not([disabled])'
  : 'input:not([disabled]), select:not([disabled]), textarea:not([disabled]), button:not([disabled]), [tabindex]:not([tabindex="-1"])'

function items(): HTMLElement[] {
  return panel.value ? [...panel.value.querySelectorAll<HTMLElement>(focusableSelector)] : []
}

async function show() {
  placement.value = { top: 0, left: 0, up: false, ready: false }
  open.value = true
  emit('open')
  await nextTick()
  place()
  items()[0]?.focus({ preventScroll: true })
}

function close(returnFocus = false) {
  if (!open.value) return
  open.value = false
  emit('close')
  if (returnFocus) trigger.value?.focus()
}

function toggle() {
  if (open.value) close()
  else void show()
}

function onPanelClick(event: MouseEvent) {
  if (!props.menu) return
  const target = event.target as HTMLElement | null
  if (target?.closest('[role="menuitem"]')) close(true)
}

function onKeydown(event: KeyboardEvent) {
  if (!open.value) return
  if (event.key === 'Escape') {
    event.stopPropagation()
    close(true)
    return
  }
  if (!props.menu || (event.key !== 'ArrowDown' && event.key !== 'ArrowUp')) return
  const list = items()
  if (!list.length) return
  const current = list.indexOf(document.activeElement as HTMLElement)
  const step = event.key === 'ArrowDown' ? 1 : -1
  list[(current + step + list.length) % list.length].focus()
  event.preventDefault()
}

function onDocumentPointer(event: PointerEvent) {
  const target = event.target as Node | null
  if (target && (root.value?.contains(target) || panel.value?.contains(target))) return
  // Pickers inside the panel can teleport their own popups to <body>.
  if (target instanceof Element && target.closest('.select-dropdown-portal, [data-popover-keep]')) return
  close()
}

function onViewportChange(event: Event) {
  // Scrolling inside the panel itself must not move it.
  if (event.type === 'scroll' && event.target instanceof Node && panel.value?.contains(event.target)) return
  place()
}

function listen(on: boolean) {
  const method = on ? 'addEventListener' : 'removeEventListener'
  document[method]('pointerdown', onDocumentPointer as (event: Event) => void, true)
  window[method]('resize', onViewportChange)
  window[method]('scroll', onViewportChange, true)
}

watch(open, (value) => listen(value))

onBeforeUnmount(() => listen(false))

defineExpose({ close, open })
</script>

<style scoped>
.toolbar-popover {
  position: relative;
  display: inline-flex;
}

.toolbar-popover-icon {
  border: 1px solid var(--operator-border);
  background: var(--operator-card);
}

.toolbar-popover-icon[aria-expanded='true'],
.ui-chip[aria-expanded='true'] {
  border-color: var(--operator-border-strong);
  color: var(--operator-foreground);
}

.toolbar-popover-count {
  display: inline-flex;
  min-width: 1.125rem;
  height: 1.125rem;
  align-items: center;
  justify-content: center;
  padding: 0 0.3125rem;
  border-radius: 999px;
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
  font-size: 0.6875rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.toolbar-popover-panel {
  position: fixed;
  /* Above side drawers (80), below the command palette (90). */
  z-index: 85;
  max-height: calc(100vh - 1rem);
  overflow-y: auto;
  max-width: calc(100vw - 1rem);
  padding: 0.25rem;
  border: 1px solid var(--operator-popup-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-popup-surface);
  box-shadow: var(--operator-popup-shadow);
  color: var(--operator-popup-foreground);
}

.toolbar-popover-panel.is-up {
  transform-origin: bottom center;
}

.toolbar-popover-panel.is-down {
  transform-origin: top center;
}

.toolbar-popover-panel.is-menu {
  min-width: 11rem;
}

.toolbar-popover-panel.is-form {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 0.875rem;
}
</style>
