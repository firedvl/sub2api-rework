<template>
  <div class="relative" ref="dropdownRef">
    <button
      type="button"
      @click="toggleDropdown"
      :disabled="switching"
      class="app-icon-button operator-locale-trigger"
      :title="currentLocale?.name"
      aria-haspopup="menu"
      :aria-expanded="isOpen"
    >
      <span class="text-base leading-none">{{ currentLocale?.flag }}</span>
      <span class="hidden text-xs font-semibold sm:inline">{{ currentLocale?.code.toUpperCase() }}</span>
      <Icon
        name="chevronDown"
        size="xs"
        class="text-gray-400 transition-transform duration-200"
        :class="{ 'rotate-180': isOpen }"
      />
    </button>

    <transition name="dropdown">
      <div
        v-if="isOpen"
        role="menu"
        class="operator-menu absolute right-0 z-50 mt-1.5 w-40 overflow-hidden p-1"
      >
        <button
          v-for="locale in availableLocales"
          :key="locale.code"
          type="button"
          role="menuitemradio"
          :disabled="switching"
          @click="selectLocale(locale.code)"
          class="operator-menu-item flex w-full items-center gap-2 rounded-md px-2.5 text-sm transition-colors"
          :aria-current="locale.code === currentLocaleCode ? 'true' : undefined"
          :aria-checked="locale.code === currentLocaleCode"
          :class="{
            'operator-locale-selected':
              locale.code === currentLocaleCode
          }"
        >
          <span class="text-base">{{ locale.flag }}</span>
          <span>{{ locale.name }}</span>
          <Icon v-if="locale.code === currentLocaleCode" name="check" size="sm" class="operator-menu-check ml-auto" />
        </button>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { setLocale, availableLocales } from '@/i18n'

const { locale } = useI18n()

const isOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)
const switching = ref(false)

const currentLocaleCode = computed(() => locale.value)
const currentLocale = computed(() => availableLocales.find((l) => l.code === locale.value))

function toggleDropdown() {
  isOpen.value = !isOpen.value
}

async function selectLocale(code: string) {
  if (switching.value || code === currentLocaleCode.value) {
    isOpen.value = false
    return
  }
  switching.value = true
  try {
    await setLocale(code)
    isOpen.value = false
  } finally {
    switching.value = false
  }
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
/* Selection uses the shared popup tokens from .operator-menu; this hook marks it. */
.operator-locale-selected {
  font-weight: 500;
}

.dropdown-enter-active {
  transition:
    opacity var(--operator-duration) var(--operator-ease-out),
    transform var(--operator-duration) var(--operator-ease-out);
}

.dropdown-leave-active {
  transition:
    opacity var(--operator-duration-fast) var(--operator-ease-in),
    transform var(--operator-duration-fast) var(--operator-ease-in);
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.98);
}

.operator-menu {
  transform-origin: top right;
}
</style>
