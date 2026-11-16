<template>
  <Teleport to="body">
    <Transition name="ui-palette" @after-leave="restoreFocus">
      <div v-if="palette.open.value" class="ui-palette-layer" @mousedown.self="palette.hide()">
        <div
          class="ui-palette"
          role="dialog"
          aria-modal="true"
          :aria-label="t('nav.paletteLabel')"
          data-testid="command-palette"
          @keydown="onKeydown"
        >
          <div class="ui-palette-input">
            <Icon name="search" size="sm" />
            <input
              ref="inputRef"
              v-model="query"
              type="text"
              role="combobox"
              aria-autocomplete="list"
              aria-expanded="true"
              :aria-controls="listId"
              :aria-activedescendant="activeId"
              :placeholder="t('nav.searchPlaceholder')"
              autocomplete="off"
              spellcheck="false"
            />
            <LoadingSpinner v-if="searching" size="sm" />
            <kbd class="ui-kbd">Esc</kbd>
          </div>

          <div :id="listId" ref="listRef" class="ui-palette-list" role="listbox">
            <template v-if="flat.length">
              <template v-for="group in groups" :key="group.key">
                <div class="ui-palette-group" role="presentation">{{ group.label }}</div>
                <div
                  v-for="item in group.items"
                  :id="optionId(item.index)"
                  :key="item.id"
                  class="ui-palette-item"
                  role="option"
                  :aria-selected="item.index === activeIndex ? 'true' : 'false'"
                  @mousemove="activeIndex = item.index"
                  @click="run(item)"
                >
                  <ProviderMark v-if="item.platform" :platform="item.platform" :size="22" class="ui-palette-icon" />
                  <span v-else-if="item.swatch" class="ui-swatch ui-palette-icon" :style="{ '--swatch': item.swatch, width: '1rem', height: '1rem' }"></span>
                  <Icon v-else-if="item.icon" :name="item.icon" size="sm" />
                  <span class="min-w-0 truncate">{{ item.label }}</span>
                  <small v-if="item.hint">{{ item.hint }}</small>
                </div>
              </template>
            </template>
            <div v-else class="ui-palette-empty">{{ t('nav.paletteNoResults') }}</div>
          </div>

          <div class="ui-palette-foot" aria-hidden="true">
            <span><kbd class="ui-kbd">↑</kbd><kbd class="ui-kbd">↓</kbd>{{ t('nav.paletteNavigate') }}</span>
            <span><kbd class="ui-kbd">↵</kbd>{{ t('nav.paletteOpen') }}</span>
            <span><kbd class="ui-kbd">Esc</kbd>{{ t('nav.paletteClose') }}</span>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, shallowRef, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { onKeyStroke, useDebounceFn } from '@vueuse/core'
import { useAppStore, useAuthStore } from '@/stores'
import { adminAPI } from '@/api/admin'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import { useAppNavigation, type IconName, type NavItem } from '@/composables/useAppNavigation'
import { useCommandPalette } from '@/composables/useCommandPalette'
import { operatorAreas } from '@/router/operatorNavigation'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { applyTheme } from '@/utils/viewTransition'
import { ACCENTS, ACCENT_SWATCH, applyAccent, type Accent } from '@/utils/accent'

interface PaletteItem {
  id: string
  label: string
  hint?: string
  icon?: IconName | null
  platform?: string
  swatch?: string
  keywords?: string
  run: () => void
}

type GroupKey = 'pages' | 'accounts' | 'actions'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const palette = useCommandPalette()
const { sections, isAdmin, adminSettingsStore } = useAppNavigation()

const query = ref('')
const activeIndex = ref(0)
const inputRef = ref<HTMLInputElement | null>(null)
const listRef = ref<HTMLElement | null>(null)
const accountResults = shallowRef<PaletteItem[]>([])
const searching = ref(false)
const listId = 'command-palette-list'
let opener: HTMLElement | null = null
let searchSequence = 0

const optionId = (index: number) => `command-palette-option-${index}`
const activeId = computed(() => (flat.value.length ? optionId(activeIndex.value) : undefined))

function go(path: string) {
  void router.push(path)
}

function sectionVisible(gate?: string, hideInSimpleMode?: boolean): boolean {
  if (authStore.isSimpleMode && hideInSimpleMode) return false
  if (gate === 'ops-monitoring') return adminSettingsStore.opsMonitoringEnabled
  if (gate === 'channel-monitor') return isFeatureFlagEnabled(FeatureFlags.channelMonitor) !== false
  if (gate === 'risk-control') return isFeatureFlagEnabled(FeatureFlags.riskControl) !== false
  return true
}

const pageItems = computed<PaletteItem[]>(() => {
  const seen = new Set<string>()
  const items: PaletteItem[] = []
  const add = (item: NavItem, hint?: string) => {
    if (seen.has(item.path) || item.path.endsWith('/more')) return
    seen.add(item.path)
    items.push({ id: `page:${item.path}`, label: item.label, hint, icon: item.icon ?? 'documentText', run: () => go(item.path) })
  }
  for (const section of sections.value) {
    for (const item of section.items) {
      if (item.children?.length) item.children.forEach((child) => add(child, item.label))
      else add(item)
    }
  }
  if (isAdmin.value) {
    for (const area of operatorAreas) {
      if (authStore.isSimpleMode && area.hideInSimpleMode) continue
      for (const section of area.sections) {
        if (!sectionVisible(section.gate, section.hideInSimpleMode)) continue
        add({ path: section.path, label: t(section.labelKey), icon: 'arrowRight' }, t(area.labelKey))
      }
    }
  }
  return items
})

const actionItems = computed<PaletteItem[]>(() => {
  const dark = document.documentElement.classList.contains('dark')
  const items: PaletteItem[] = [
    {
      id: 'action:theme',
      label: t('nav.toggleTheme'),
      hint: dark ? t('nav.themeLight') : t('nav.themeDark'),
      icon: dark ? 'sun' : 'moon',
      keywords: 'dark light mode',
      run: () => applyTheme(!document.documentElement.classList.contains('dark')),
    },
    {
      id: 'action:sidebar',
      label: t('nav.toggleSidebar'),
      hint: '⌘B',
      icon: 'sidebar',
      run: () => appStore.toggleSidebar(),
    },
    ...ACCENTS.map((accent: Accent) => ({
      id: `action:accent:${accent}`,
      label: `${t('nav.accent')}: ${t(`nav.accent${accent[0].toUpperCase()}${accent.slice(1)}`)}`,
      swatch: ACCENT_SWATCH[accent],
      keywords: 'color theme',
      run: () => applyAccent(accent),
    })),
    {
      id: 'action:logout',
      label: t('nav.logout'),
      icon: 'login',
      keywords: 'sign out',
      run: async () => {
        try {
          await authStore.logout()
        } finally {
          await router.push('/login')
        }
      },
    },
  ]
  return items
})

function matches(item: PaletteItem, needle: string): boolean {
  if (!needle) return true
  const haystack = `${item.label} ${item.hint ?? ''} ${item.keywords ?? ''}`.toLowerCase()
  return needle.split(/\s+/).every((part) => haystack.includes(part))
}

const groups = computed(() => {
  const needle = query.value.trim().toLowerCase()
  const raw: Array<{ key: GroupKey; label: string; items: PaletteItem[] }> = [
    { key: 'pages', label: t('nav.palettePages'), items: pageItems.value.filter((item) => matches(item, needle)).slice(0, needle ? 8 : 6) },
    { key: 'accounts', label: t('nav.paletteAccounts'), items: needle ? accountResults.value : [] },
    { key: 'actions', label: t('nav.paletteActions'), items: actionItems.value.filter((item) => matches(item, needle)).slice(0, needle ? 6 : 3) },
  ]
  let index = 0
  return raw
    .filter((group) => group.items.length)
    .map((group) => ({ ...group, items: group.items.map((item) => ({ ...item, index: index++ })) }))
})

const flat = computed(() => groups.value.flatMap((group) => group.items))

const searchAccounts = useDebounceFn(async (needle: string) => {
  const sequence = ++searchSequence
  if (!isAdmin.value || needle.length < 2) {
    accountResults.value = []
    searching.value = false
    return
  }
  searching.value = true
  try {
    const response = await adminAPI.accounts.list(1, 5, { search: needle, lite: '1' })
    if (sequence !== searchSequence) return
    accountResults.value = response.items.map((account) => ({
      id: `account:${account.id}`,
      label: account.name,
      hint: account.platform,
      platform: account.platform,
      run: () => go(`/admin/accounts?account=${account.id}`),
    }))
  } catch {
    if (sequence === searchSequence) accountResults.value = []
  } finally {
    if (sequence === searchSequence) searching.value = false
  }
}, 200)

watch(query, (value) => {
  activeIndex.value = 0
  void searchAccounts(value.trim())
})

watch(
  () => palette.open.value,
  async (open) => {
    if (!open) return
    opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
    query.value = ''
    accountResults.value = []
    activeIndex.value = 0
    await nextTick()
    inputRef.value?.focus()
  },
)

watch(activeIndex, async (index) => {
  await nextTick()
  listRef.value?.querySelector(`#${optionId(index)}`)?.scrollIntoView({ block: 'nearest' })
})

function run(item: PaletteItem) {
  palette.hide()
  item.run()
}

function restoreFocus() {
  if (opener?.isConnected) opener.focus()
  opener = null
}

function onKeydown(event: KeyboardEvent) {
  const count = flat.value.length
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    if (count) activeIndex.value = (activeIndex.value + 1) % count
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    if (count) activeIndex.value = (activeIndex.value - 1 + count) % count
  } else if (event.key === 'Enter') {
    event.preventDefault()
    const item = flat.value[activeIndex.value]
    if (item) run(item)
  } else if (event.key === 'Escape') {
    event.preventDefault()
    event.stopPropagation()
    palette.hide()
  } else if (event.key === 'Tab') {
    // The input is the only stop; Tab cycles results instead of leaving the dialog.
    event.preventDefault()
    if (count) activeIndex.value = (activeIndex.value + (event.shiftKey ? count - 1 : 1)) % count
  }
}

// Cmd/Ctrl+K toggles the palette anywhere; "/" opens it outside text fields.
onKeyStroke(['k', 'K'], (event) => {
  if (!(event.metaKey || event.ctrlKey) || event.altKey || event.shiftKey) return
  event.preventDefault()
  palette.toggle()
})

onKeyStroke('/', (event) => {
  const target = event.target as HTMLElement | null
  if (palette.open.value || event.metaKey || event.ctrlKey || event.altKey) return
  if (target?.closest('input, textarea, select, [contenteditable="true"]')) return
  event.preventDefault()
  palette.show()
})
</script>
