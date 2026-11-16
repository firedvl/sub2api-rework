<template>
  <button
    ref="trigger"
    type="button"
    class="app-icon-button"
    :aria-label="label"
    :title="label"
    aria-haspopup="menu"
    :aria-expanded="open"
    v-bind="$attrs"
    @click.stop="toggle"
  >
    <Icon :name="icon" size="sm" />
  </button>
  <Teleport to="body">
    <Transition name="menu-pop">
      <div
        v-if="open"
        ref="menu"
        class="dropdown overflow-menu"
        role="menu"
        :aria-label="label"
        :style="position"
        @keydown="onKeydown"
      >
        <slot :close="close" />
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'

defineOptions({ inheritAttrs: false })

/**
 * Icon trigger plus a body-level menu so table overflow never clips it. Items are
 * slotted `button.dropdown-item[role=menuitem]`; arrows move focus, Escape and Tab close.
 */
withDefaults(defineProps<{
  label: string
  icon?: 'more' | 'cog' | 'grid' | 'globe' | 'filter'
  width?: number
}>(), { icon: 'more', width: 208 })

const open = ref(false)
const trigger = ref<HTMLButtonElement | null>(null)
const menu = ref<HTMLElement | null>(null)
const position = ref<Record<string, string>>({})

function items(): HTMLElement[] {
  return [...(menu.value?.querySelectorAll<HTMLElement>('[role="menuitem"]:not([disabled]), [role="menuitemcheckbox"]:not([disabled])') ?? [])]
}

function place() {
  const rect = trigger.value?.getBoundingClientRect()
  if (!rect) return
  const width = Math.min(menu.value?.offsetWidth || 208, window.innerWidth - 16)
  const left = Math.max(8, Math.min(rect.right - width, window.innerWidth - width - 8))
  const height = menu.value?.offsetHeight || 240
  const below = window.innerHeight - rect.bottom
  position.value = below < height + 12 && rect.top > below
    ? { position: 'fixed', left: `${left}px`, bottom: `${window.innerHeight - rect.top + 6}px`, transformOrigin: 'bottom right' }
    : { position: 'fixed', left: `${left}px`, top: `${rect.bottom + 6}px` }
}

async function toggle() {
  if (open.value) return close()
  open.value = true
  place()
  await nextTick()
  place()
  items()[0]?.focus()
  document.addEventListener('pointerdown', onOutside, true)
  window.addEventListener('scroll', onScroll, true)
  window.addEventListener('resize', onResize)
}

const onResize = () => close()
function onScroll(event: Event) {
  if (menu.value?.contains(event.target as Node)) return
  close()
}

function close(refocus = false) {
  if (!open.value) return
  open.value = false
  document.removeEventListener('pointerdown', onOutside, true)
  window.removeEventListener('scroll', onScroll, true)
  window.removeEventListener('resize', onResize)
  if (refocus) trigger.value?.focus()
}

function onOutside(event: Event) {
  const target = event.target as Node
  if (menu.value?.contains(target) || trigger.value?.contains(target)) return
  close()
}

function onKeydown(event: KeyboardEvent) {
  const list = items()
  const index = list.indexOf(document.activeElement as HTMLElement)
  if (event.key === 'Escape') {
    event.stopPropagation()
    close(true)
  } else if (event.key === 'Tab') {
    close(true)
    event.preventDefault()
  } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    const step = event.key === 'ArrowDown' ? 1 : -1
    list[(index + step + list.length) % list.length]?.focus()
  } else if (event.key === 'Home' || event.key === 'End') {
    event.preventDefault()
    list[event.key === 'Home' ? 0 : list.length - 1]?.focus()
  }
}

onBeforeUnmount(() => close())
defineExpose({ close })
</script>

<style>
.overflow-menu {
  z-index: 90;
  min-width: 12rem;
  max-width: calc(100vw - 1rem);
  max-height: min(24rem, calc(100vh - 2rem));
  overflow-y: auto;
}

.overflow-menu .dropdown-item[aria-disabled='true'],
.overflow-menu .dropdown-item:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.overflow-menu .dropdown-item.is-danger {
  color: var(--operator-destructive-fill);
}

.overflow-menu hr {
  margin: 0.25rem 0.375rem;
  border-color: var(--operator-border);
}
</style>
