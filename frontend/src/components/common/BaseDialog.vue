<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="show"
        class="modal-overlay"
        :style="zIndexStyle"
        :aria-labelledby="dialogId"
        role="dialog"
        aria-modal="true"
        @click.self="handleClose"
      >
        <!-- Modal panel -->
        <div ref="dialogRef" :class="['modal-content', widthClasses]" tabindex="-1" @click.stop>
          <!-- Header -->
          <div class="modal-header">
            <h3 :id="dialogId" class="modal-title">
              {{ title }}
            </h3>
            <button
              v-if="showCloseButton"
              @click="emit('close')"
              class="-mr-2 rounded-xl p-2 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/30 focus-visible:ring-offset-2 dark:text-dark-500 dark:hover:bg-dark-700 dark:hover:text-dark-300 dark:focus-visible:ring-offset-dark-900"
              aria-label="Close modal"
            >
              <Icon name="x" size="md" />
            </button>
          </div>

          <!-- Body -->
          <div ref="modalBodyRef" class="modal-body">
            <slot></slot>
          </div>

          <!-- Footer -->
          <div v-if="$slots.footer" class="modal-footer">
            <slot name="footer"></slot>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script lang="ts">
let dialogIdCounter = 0
interface OpenDialog {
  element: () => HTMLElement | null
  restoreTarget: HTMLElement | null
}

const openDialogs: OpenDialog[] = []
const focusableSelector =
  'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

const updateScrollLock = () => {
  document.body.classList.toggle('modal-open', openDialogs.length > 0)
}
</script>

<script setup lang="ts">
import { computed, watch, onMounted, onUnmounted, ref, nextTick } from 'vue'
import Icon from '@/components/icons/Icon.vue'

// 生成唯一ID以避免多个对话框时ID冲突
const dialogId = `modal-title-${++dialogIdCounter}`

// 焦点管理
const dialogRef = ref<HTMLElement | null>(null)
const modalBodyRef = ref<HTMLElement | null>(null)
const dialogEntry: OpenDialog = {
  element: () => dialogRef.value,
  restoreTarget: null
}
let isOpen = false

type DialogWidth = 'narrow' | 'normal' | 'wide' | 'extra-wide' | 'full'

interface Props {
  show: boolean
  title: string
  width?: DialogWidth
  closeOnEscape?: boolean
  closeOnClickOutside?: boolean
  showCloseButton?: boolean
  zIndex?: number
}

interface Emits {
  (e: 'close'): void
}

const props = withDefaults(defineProps<Props>(), {
  width: 'normal',
  closeOnEscape: true,
  closeOnClickOutside: false,
  showCloseButton: true,
  zIndex: 50
})

const emit = defineEmits<Emits>()

// Custom z-index style (overrides the default z-50 from CSS)
const zIndexStyle = computed(() => {
  return props.zIndex !== 50 ? { zIndex: props.zIndex } : undefined
})

const widthClasses = computed(() => {
  // Width guidance: narrow=confirm/short prompts, normal=standard forms,
  // wide=multi-section forms or rich content, extra-wide=analytics/tables,
  // full=full-screen or very dense layouts.
  const widths: Record<DialogWidth, string> = {
    narrow: 'max-w-md',
    normal: 'max-w-lg',
    wide: 'w-full sm:max-w-2xl md:max-w-3xl lg:max-w-4xl',
    'extra-wide': 'w-full sm:max-w-3xl md:max-w-4xl lg:max-w-5xl xl:max-w-6xl',
    full: 'w-full sm:max-w-4xl md:max-w-5xl lg:max-w-6xl xl:max-w-7xl'
  }
  return widths[props.width]
})

const handleClose = () => {
  if (props.closeOnClickOutside) {
    emit('close')
  }
}

const handleEscape = (event: KeyboardEvent) => {
  if (isTopmost() && props.closeOnEscape && event.key === 'Escape') {
    emit('close')
  }
}

const isTopmost = () => openDialogs.at(-1) === dialogEntry

const getFocusableElements = () =>
  Array.from(dialogRef.value?.querySelectorAll<HTMLElement>(focusableSelector) ?? []).filter(
    element => !element.hasAttribute('disabled') && element.tabIndex >= 0
  )

const focusInitialElement = () => {
  const [firstFocusable] = getFocusableElements()
  const focusTarget = firstFocusable ?? dialogRef.value
  focusTarget?.focus()
}

const handleTab = (event: KeyboardEvent) => {
  if (event.key !== 'Tab' || !isTopmost()) return

  const focusableElements = getFocusableElements()
  if (focusableElements.length === 0) {
    event.preventDefault()
    dialogRef.value?.focus()
    return
  }

  const firstFocusable = focusableElements[0]
  const lastFocusable = focusableElements[focusableElements.length - 1]
  const activeElement = document.activeElement

  if (event.shiftKey && (activeElement === firstFocusable || !dialogRef.value?.contains(activeElement))) {
    event.preventDefault()
    lastFocusable.focus()
  } else if (!event.shiftKey && (activeElement === lastFocusable || !dialogRef.value?.contains(activeElement))) {
    event.preventDefault()
    firstFocusable.focus()
  }
}

const handleKeydown = (event: KeyboardEvent) => {
  handleEscape(event)
  handleTab(event)
}

const removeFromStack = () => {
  const index = openDialogs.lastIndexOf(dialogEntry)
  if (index === -1) return false

  const wasTopmost = index === openDialogs.length - 1
  const [removedDialog] = openDialogs.splice(index, 1)
  const dialogAbove = openDialogs[index]
  if (dialogAbove && removedDialog.element()?.contains(dialogAbove.restoreTarget)) {
    dialogAbove.restoreTarget = removedDialog.restoreTarget
  }
  updateScrollLock()
  return wasTopmost
}

const restoreFocus = (shouldRestore: boolean) => {
  const target = dialogEntry.restoreTarget
  if (shouldRestore && target?.isConnected && typeof target.focus === 'function') {
    target.focus()
  }
  dialogEntry.restoreTarget = null
}

// Prevent body scroll when modal is open and manage focus
watch(
  () => props.show,
  async (shouldOpen) => {
    if (shouldOpen && !isOpen) {
      isOpen = true
      dialogEntry.restoreTarget = document.activeElement as HTMLElement
      openDialogs.push(dialogEntry)
      updateScrollLock()

      await nextTick()
      if (!props.show || !isTopmost()) return
      if (modalBodyRef.value) modalBodyRef.value.scrollTop = 0
      focusInitialElement()
    } else if (!shouldOpen && isOpen) {
      isOpen = false
      restoreFocus(removeFromStack())
    }
  },
  { immediate: true }
)

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
  restoreFocus(removeFromStack())
})
</script>
