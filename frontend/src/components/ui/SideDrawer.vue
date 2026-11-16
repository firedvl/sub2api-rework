<template>
  <Teleport to="body">
    <Transition name="ui-drawer" @after-leave="restoreFocus">
      <div v-if="open" class="ui-drawer-layer" @keydown="onKeydown">
        <div class="ui-drawer-scrim" aria-hidden="true" @click="close"></div>
        <aside
          ref="panel"
          class="ui-drawer"
          role="dialog"
          aria-modal="true"
          :aria-labelledby="titleId"
          :style="width ? { '--drawer-width': width } : undefined"
        >
          <header class="ui-drawer-head">
            <slot name="leading" />
            <div class="min-w-0 flex-1">
              <h2 :id="titleId">{{ title }}</h2>
              <p v-if="subtitle" class="ui-cell-sub">{{ subtitle }}</p>
            </div>
            <slot name="badge" />
            <button ref="closeButton" type="button" class="app-icon-button" :aria-label="t('common.close')" @click="close">
              <Icon name="x" size="sm" />
            </button>
          </header>
          <div class="ui-drawer-body">
            <slot />
          </div>
          <footer v-if="$slots.footer" class="ui-drawer-foot">
            <slot name="footer" />
          </footer>
        </aside>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

/**
 * Right-side detail drawer with a focus trap. Escape, the scrim, and the close
 * button emit `close`; focus returns to whatever opened it.
 */
const props = defineProps<{
  open: boolean
  title: string
  subtitle?: string
  width?: string
}>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

const titleId = `drawer-${Math.random().toString(36).slice(2, 9)}`
const panel = ref<HTMLElement | null>(null)
const closeButton = ref<HTMLButtonElement | null>(null)
let opener: HTMLElement | null = null

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
    await nextTick()
    closeButton.value?.focus()
  },
  { immediate: true },
)

function close() {
  emit('close')
}

// Return focus only if it was left in the closed drawer (or dropped to the body);
// a dialog opened from the drawer keeps its own focus.
function restoreFocus() {
  const active = document.activeElement
  const stranded = !active || active === document.body || !active.isConnected || panel.value?.contains(active)
  if (stranded && opener?.isConnected) opener.focus()
  opener = null
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.stopPropagation()
    close()
    return
  }
  if (event.key !== 'Tab' || !panel.value) return
  const focusable = [...panel.value.querySelectorAll<HTMLElement>(
    'button:not([disabled]), a[href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
  )]
  if (!focusable.length) return
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    last.focus()
    event.preventDefault()
  } else if (!event.shiftKey && document.activeElement === last) {
    first.focus()
    event.preventDefault()
  }
}
</script>
