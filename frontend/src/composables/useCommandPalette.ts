import { ref } from 'vue'

const open = ref(false)

/** Shared open state so the header button, the tab bar, and shortcuts drive one palette. */
export function useCommandPalette() {
  return {
    open,
    show: () => {
      open.value = true
    },
    hide: () => {
      open.value = false
    },
    toggle: () => {
      open.value = !open.value
    },
  }
}
