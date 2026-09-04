<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, useId, useTemplateRef } from 'vue'

const props = withDefaults(defineProps<{
  content?: string
  trigger?: 'hover' | 'click'
  widthClass?: string
  interactive?: boolean
  ariaLabel?: string
}>(), {
  trigger: 'hover',
  widthClass: 'w-64',
  interactive: false,
  ariaLabel: 'Help',
})

const show = ref(false)
const triggerRef = useTemplateRef<HTMLElement>('trigger')
const tooltipRef = useTemplateRef<HTMLElement>('tooltip')
const tooltipStyle = ref({ top: '0px', left: '0px' })
const clickedOpen = ref(false)
const suppressFocusOpen = ref(false)
const tooltipId = useId()

function openTooltip() {
  show.value = true
  nextTick(updatePosition)
}

function closeTooltip(restoreFocus = false) {
  show.value = false
  clickedOpen.value = false
  if (restoreFocus) {
    suppressFocusOpen.value = true
    nextTick(() => triggerRef.value?.focus())
  }
}

function onEnter() {
  if (props.trigger !== 'hover') return
  openTooltip()
}

function onTriggerLeave(event: MouseEvent) {
  if (props.trigger !== 'hover') return
  if (props.interactive && tooltipRef.value?.contains(event.relatedTarget as Node | null)) return
  if (props.interactive && clickedOpen.value) return
  closeTooltip()
}

function onClick(event: MouseEvent) {
  if (props.trigger !== 'click' && !props.interactive) return
  event.stopPropagation()
  if (props.interactive && !clickedOpen.value) {
    clickedOpen.value = true
    openTooltip()
    return
  }
  if (show.value) {
    closeTooltip()
    return
  }
  clickedOpen.value = true
  openTooltip()
}

function onFocusIn() {
  if (suppressFocusOpen.value) {
    suppressFocusOpen.value = false
    return
  }
  if (props.interactive) openTooltip()
}

function onFocusOut(event: FocusEvent) {
  if (!props.interactive || clickedOpen.value) return
  const next = event.relatedTarget as Node | null
  if (next && (triggerRef.value?.contains(next) || tooltipRef.value?.contains(next))) return
  closeTooltip()
}

function onTriggerKeydown(event: KeyboardEvent) {
  if (!props.interactive || event.key !== 'Tab' || event.shiftKey || !show.value) return
  const first = tooltipRef.value?.querySelector<HTMLElement>('a[href]')
    ?? tooltipRef.value?.querySelector<HTMLElement>('button:not([disabled]), [tabindex]:not([tabindex="-1"])')
  if (!first) return
  event.preventDefault()
  first.focus()
}

function onPanelLeave(event: MouseEvent) {
  if (!props.interactive || clickedOpen.value) return
  if (triggerRef.value?.contains(event.relatedTarget as Node | null)) return
  if (tooltipRef.value?.contains(document.activeElement)) return
  closeTooltip()
}

function onDocumentClick(event: MouseEvent) {
  if ((props.trigger !== 'click' && !props.interactive) || !show.value) return
  const target = event.target as Node | null
  if (!target) return
  if (triggerRef.value?.contains(target) || tooltipRef.value?.contains(target)) return
  closeTooltip()
}

function onDocumentKeydown(event: KeyboardEvent) {
  if (props.trigger !== 'click' && !props.interactive) return
  if (event.key === 'Escape') {
    closeTooltip(props.interactive)
  }
}

function onViewportChange() {
  if (!show.value) return
  updatePosition()
}

function updatePosition() {
  const el = triggerRef.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  tooltipStyle.value = {
    top: `${rect.top}px`,
    left: `${rect.left + rect.width / 2}px`,
  }
}

onMounted(() => {
  document.addEventListener('click', onDocumentClick, true)
  document.addEventListener('keydown', onDocumentKeydown)
  window.addEventListener('resize', onViewportChange)
  window.addEventListener('scroll', onViewportChange, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onDocumentClick, true)
  document.removeEventListener('keydown', onDocumentKeydown)
  window.removeEventListener('resize', onViewportChange)
  window.removeEventListener('scroll', onViewportChange, true)
})
</script>

<template>
  <component
    :is="props.interactive ? 'button' : 'span'"
    ref="trigger"
    :type="props.interactive ? 'button' : undefined"
    :aria-label="props.interactive ? props.ariaLabel : undefined"
    :aria-haspopup="props.interactive ? 'dialog' : undefined"
    :aria-expanded="props.interactive ? show : undefined"
    :aria-controls="props.interactive ? tooltipId : undefined"
    :class="[
      'group relative ml-1 inline-flex items-center align-middle',
      props.interactive ? 'border-0 bg-transparent p-0 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2' : '',
    ]"
    @mouseenter="onEnter"
    @mouseleave="onTriggerLeave"
    @focusin="onFocusIn"
    @focusout="onFocusOut"
    @keydown="onTriggerKeydown"
    @click="onClick"
  >
    <!-- Trigger Icon -->
    <slot name="trigger">
      <svg
        class="h-4 w-4 cursor-help text-gray-400 transition-colors hover:text-primary-600 dark:text-gray-500 dark:hover:text-primary-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        stroke-width="2"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
        />
      </svg>
    </slot>

    <!-- Teleport to body to escape modal overflow clipping -->
    <Teleport to="body">
      <div
        :id="tooltipId"
        ref="tooltip"
        v-show="show"
        :role="props.interactive ? 'dialog' : 'tooltip'"
        :aria-label="props.interactive ? props.ariaLabel : undefined"
        :class="[
          'fixed z-[99999] -translate-x-1/2 -translate-y-full rounded-lg bg-gray-900 p-3 text-xs leading-relaxed text-white shadow-xl ring-1 ring-white/10 dark:bg-gray-800',
          props.widthClass,
        ]"
        :style="{ top: `calc(${tooltipStyle.top} - 8px)`, left: tooltipStyle.left }"
        @mouseenter="props.interactive && openTooltip()"
        @mouseleave="onPanelLeave"
        @focusin="onFocusIn"
        @focusout="onFocusOut"
      >
        <button
          v-if="props.trigger === 'click' || props.interactive"
          type="button"
          class="absolute right-1.5 top-1.5 rounded p-1 text-gray-300 transition-colors hover:bg-white/10 hover:text-white"
          aria-label="Close"
          @click.stop="closeTooltip()"
        >
          <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
        <slot>{{ content }}</slot>
        <div v-if="props.interactive" aria-hidden="true" class="absolute -bottom-3 left-0 h-3 w-full"></div>
        <div class="absolute -bottom-1 left-1/2 h-2 w-2 -translate-x-1/2 rotate-45 bg-gray-900 dark:bg-gray-800"></div>
      </div>
    </Teleport>
  </component>
</template>
