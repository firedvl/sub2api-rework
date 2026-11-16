<template>
  <span class="action-menu">
    <button
      ref="trigger"
      type="button"
      :class="triggerClass"
      :aria-label="label"
      :title="label"
      aria-haspopup="menu"
      :aria-expanded="open"
      :data-testid="testId"
      @click.stop="toggle"
    >
      <Icon :name="icon as any" size="sm" />
    </button>
    <Teleport to="body">
      <Transition name="menu-pop">
        <div
          v-if="open"
          ref="panel"
          class="action-menu-panel"
          role="menu"
          :aria-label="label"
          :style="panelStyle"
          @keydown="onKeydown"
          @click.stop
        >
          <template v-for="(item, index) in items" :key="item.key">
            <div v-if="item.section && item.section !== items[index - 1]?.section" class="action-menu-section">
              {{ item.section }}
            </div>
            <button
              type="button"
              :role="item.pressed === undefined ? 'menuitem' : 'menuitemcheckbox'"
              class="action-menu-item"
              :class="{ 'is-danger': item.danger }"
              :aria-checked="item.pressed === undefined ? undefined : item.pressed"
              :disabled="item.disabled"
              :data-testid="item.testId"
              @click="select(item)"
            >
              <Icon v-if="item.icon" :name="item.icon as any" size="sm" class="action-menu-icon" />
              <span class="action-menu-label">{{ item.label }}</span>
              <small v-if="item.hint">{{ item.hint }}</small>
              <Icon v-if="item.pressed" name="check" size="sm" class="action-menu-check" :stroke-width="2" />
            </button>
          </template>
        </div>
      </Transition>
    </Teleport>
  </span>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'

export interface ActionMenuItem {
  key: string
  label: string
  icon?: string
  hint?: string
  section?: string
  danger?: boolean
  disabled?: boolean
  /** Set for toggle items; renders a check and keeps the menu open. */
  pressed?: boolean
  testId?: string
  action: () => void
}

/**
 * Overflow menu rendered in a fixed layer so it is never clipped by a
 * scrolling table. Arrow keys move, Escape closes and returns focus.
 */
withDefaults(defineProps<{
  label: string
  items: ActionMenuItem[]
  icon?: string
  triggerClass?: string
  testId?: string
}>(), { icon: 'more', triggerClass: 'app-icon-button', testId: undefined })

const open = ref(false)
const trigger = ref<HTMLButtonElement | null>(null)
const panel = ref<HTMLElement | null>(null)
const panelStyle = ref<Record<string, string>>({})

function place() {
  const rect = trigger.value?.getBoundingClientRect()
  if (!rect) return
  const width = panel.value?.offsetWidth ?? 220
  const height = panel.value?.offsetHeight ?? 0
  const gutter = 8
  const left = Math.min(Math.max(gutter, rect.right - width), window.innerWidth - width - gutter)
  const below = rect.bottom + 6
  const top = height && below + height > window.innerHeight - gutter ? Math.max(gutter, rect.top - height - 6) : below
  panelStyle.value = { top: `${top}px`, left: `${left}px` }
}

async function show() {
  open.value = true
  place()
  await nextTick()
  place()
  panel.value?.querySelector<HTMLElement>('button:not([disabled])')?.focus()
}

function close(returnFocus = false) {
  if (!open.value) return
  open.value = false
  if (returnFocus) trigger.value?.focus()
}

function toggle() {
  if (open.value) close()
  else void show()
}

function select(item: ActionMenuItem) {
  if (item.disabled) return
  item.action()
  if (item.pressed === undefined) close(true)
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' || event.key === 'Tab') {
    event.stopPropagation()
    if (event.key === 'Escape') event.preventDefault()
    close(true)
    return
  }
  if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
  const list = [...(panel.value?.querySelectorAll<HTMLElement>('button:not([disabled])') ?? [])]
  if (!list.length) return
  const current = list.indexOf(document.activeElement as HTMLElement)
  const step = event.key === 'ArrowDown' ? 1 : -1
  list[(current + step + list.length) % list.length].focus()
  event.preventDefault()
}

function onOutside(event: Event) {
  const target = event.target as Node | null
  if (target && (panel.value?.contains(target) || trigger.value?.contains(target))) return
  close()
}

function onViewportChange(event: Event) {
  // Scrolling the menu's own list must not close it; only page or ancestor scrolls move the trigger.
  if (event.type === 'scroll' && event.target instanceof Node && panel.value?.contains(event.target)) return
  close()
}

watch(open, (value) => {
  const method = value ? 'addEventListener' : 'removeEventListener'
  document[method]('pointerdown', onOutside, true)
  window[method]('resize', onViewportChange)
  window[method]('scroll', onViewportChange, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onOutside, true)
  window.removeEventListener('resize', onViewportChange)
  window.removeEventListener('scroll', onViewportChange, true)
})

defineExpose({ close, open })
</script>

<style scoped>
.action-menu {
  display: inline-flex;
}

.action-menu > button[aria-expanded='true'] {
  background: var(--operator-muted);
  color: var(--operator-foreground);
}

.action-menu-panel {
  position: fixed;
  z-index: 85;
  min-width: 12rem;
  max-width: calc(100vw - 1rem);
  max-height: min(28rem, calc(100vh - 1rem));
  overflow-y: auto;
  padding: 0.25rem;
  border: 1px solid var(--operator-popup-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-popup-surface);
  box-shadow: var(--operator-popup-shadow);
  color: var(--operator-popup-foreground);
}

.action-menu-section {
  padding: 0.5rem 0.625rem 0.25rem;
  color: var(--operator-subtle-foreground);
  font-size: 0.6875rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.action-menu-section:not(:first-child) {
  margin-top: 0.25rem;
  border-top: 1px solid var(--operator-popup-border);
  padding-top: 0.625rem;
}

.action-menu-item {
  display: flex;
  width: 100%;
  min-height: 2.125rem;
  align-items: center;
  gap: 0.625rem;
  padding: 0 0.625rem;
  border-radius: var(--operator-radius);
  color: var(--operator-popup-foreground);
  font-size: 0.8125rem;
  text-align: left;
  white-space: nowrap;
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.action-menu-item:hover:not(:disabled),
.action-menu-item:focus-visible {
  background: var(--operator-popup-hover);
  outline: none;
}

.action-menu-item:focus-visible {
  box-shadow: inset 0 0 0 2px var(--operator-focus);
}

.action-menu-item:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.action-menu-item.is-danger {
  color: var(--operator-destructive);
}

.action-menu-icon {
  flex: none;
  color: var(--operator-subtle-foreground);
}

.is-danger .action-menu-icon {
  color: inherit;
}

.action-menu-label {
  flex: 1 1 auto;
}

.action-menu-item small {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.action-menu-check {
  flex: none;
  color: var(--operator-accent-text);
  animation: ui-pop 260ms var(--operator-spring);
}

@media (prefers-reduced-motion: reduce) {
  .action-menu-check {
    animation: none;
  }
}
</style>
