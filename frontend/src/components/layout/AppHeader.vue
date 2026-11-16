<template>
  <header class="app-header" :class="{ 'is-scrolled': scrolled }">
    <div class="app-header-context">
      <button
        type="button"
        class="app-icon-button -ml-1 lg:hidden"
        :aria-label="t('common.toggleMenu')"
        :aria-expanded="appStore.mobileOpen ? 'true' : 'false'"
        @click="appStore.toggleMobileSidebar()"
      >
        <Icon name="menu" size="md" />
      </button>

      <button
        type="button"
        class="app-icon-button -ml-2 hidden lg:inline-flex"
        data-testid="sidebar-toggle"
        :aria-label="appStore.sidebarCollapsed ? t('nav.expand') : t('nav.collapse')"
        :title="appStore.sidebarCollapsed ? t('nav.expand') : t('nav.collapse')"
        :aria-pressed="appStore.sidebarCollapsed ? 'true' : 'false'"
        @click="appStore.toggleSidebar()"
      >
        <Icon name="sidebar" size="sm" />
      </button>

      <nav class="app-breadcrumb" :aria-label="t('nav.breadcrumb')">
        <template v-if="area && area.primaryPath !== route.path">
          <router-link :to="area.primaryPath" class="hidden truncate sm:inline">{{ t(area.labelKey) }}</router-link>
          <Icon name="chevronRight" size="xs" class="hidden flex-shrink-0 sm:block" aria-hidden="true" />
        </template>
        <h1 class="app-breadcrumb-current" aria-current="page">{{ title }}</h1>
      </nav>

      <span v-if="isFixtureReview" class="app-review-badge app-header-secondary" role="status">
        {{ t('nav.fixtureReview') }}
      </span>
    </div>

    <div class="app-header-actions">
      <button
        type="button"
        class="app-search-trigger hidden md:inline-flex"
        data-testid="command-palette-trigger"
        :aria-label="t('nav.search')"
        aria-keyshortcuts="Meta+K Control+K"
        @click="palette.show()"
      >
        <Icon name="search" size="sm" />
        <span>{{ t('nav.search') }}</span>
        <kbd class="ui-kbd">{{ shortcutLabel }}</kbd>
      </button>
      <button
        type="button"
        class="app-icon-button md:hidden"
        :aria-label="t('nav.search')"
        @click="palette.show()"
      >
        <Icon name="search" size="sm" />
      </button>

      <a
        v-if="docUrl"
        :href="docUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="app-icon-button app-header-secondary"
        :title="t('nav.docs')"
      >
        <Icon name="book" size="sm" />
        <span class="hidden xl:inline">{{ t('nav.docs') }}</span>
      </a>

      <router-link
        v-if="user && modelPlazaEnabled"
        :to="{ path: '/model-plaza', query: { embedded: '1' } }"
        :title="t('nav.modelPlaza')"
        :aria-label="t('nav.modelPlaza')"
        class="app-icon-button"
      >
        <Icon name="grid" size="sm" />
        <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
      </router-link>

      <AnnouncementBell v-if="user" />
      <LocaleSwitcher />

      <button
        type="button"
        class="app-icon-button"
        :aria-label="isDark ? t('nav.lightMode') : t('nav.darkMode')"
        :title="isDark ? t('nav.lightMode') : t('nav.darkMode')"
        @click="toggleTheme"
      >
        <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
      </button>

      <template v-if="user && showPersonalStatus">
        <SubscriptionProgressMini v-if="appStore.publicSettingsLoaded && subscriptionFeatureEnabled" />
        <div class="app-balance hidden sm:inline-flex" :title="balanceTitle">
          <Icon name="wallet" size="sm" class="text-gray-500" aria-hidden="true" />
          <span>{{ formatHeaderMoney(availableBalance) }}</span>
          <span v-if="frozenBalance > 0" class="badge badge-warning">{{ balanceFrozenLabel }}</span>
        </div>
      </template>

      <div v-if="user" ref="userMenuRef" class="relative">
        <button
          type="button"
          class="app-icon-button gap-2 pl-1 pr-1.5"
          :aria-label="t('common.userMenu')"
          :aria-expanded="userMenuOpen ? 'true' : 'false'"
          aria-controls="app-user-menu"
          @click="userMenuOpen = !userMenuOpen"
        >
          <span class="app-avatar">
            <img v-if="avatarUrl" :src="avatarUrl" alt="" />
            <span v-else>{{ userInitials }}</span>
          </span>
          <span class="hidden max-w-[9rem] truncate text-sm font-medium text-gray-900 dark:text-gray-100 md:inline">
            {{ displayName }}
          </span>
          <Icon
            name="chevronDown"
            size="xs"
            class="hidden transition-transform duration-normal ease-ui-out md:block"
            :class="{ 'rotate-180': userMenuOpen }"
          />
        </button>

        <Transition name="menu-pop">
          <div v-if="userMenuOpen" id="app-user-menu" class="app-menu-panel">
            <div class="app-menu-section app-menu-identity">
              <strong>{{ displayName }}</strong>
              <span>{{ user.email }}</span>
              <span class="mt-1">{{ t('admin.users.roles.' + user.role) }}</span>
            </div>

            <div v-if="showPersonalStatus" class="app-menu-section app-menu-identity sm:hidden">
              <span>{{ t('common.balance') }}</span>
              <strong>{{ formatHeaderMoney(availableBalance) }}</strong>
              <span v-if="frozenBalance > 0" class="text-amber-600">
                {{ balanceFrozenText }} {{ formatHeaderMoney(frozenBalance) }}
              </span>
            </div>

            <div class="app-menu-section" role="group" :aria-label="t('nav.appearance')">
              <div class="ui-section-label px-2.5 pt-1.5" aria-hidden="true">{{ t('nav.appearance') }}</div>
              <div class="viz-seg mx-2.5 mb-1 flex" role="group" :aria-label="t('nav.toggleTheme')">
                <button type="button" class="flex-1" :aria-pressed="isDark ? 'false' : 'true'" @click="setTheme(false)">
                  <Icon name="sun" size="xs" />
                  {{ t('nav.themeLight') }}
                </button>
                <button type="button" class="flex-1" :aria-pressed="isDark ? 'true' : 'false'" @click="setTheme(true)">
                  <Icon name="moon" size="xs" />
                  {{ t('nav.themeDark') }}
                </button>
              </div>
              <div class="ui-swatches" role="group" :aria-label="t('nav.accent')">
                <button
                  v-for="option in accentOptions"
                  :key="option.id"
                  type="button"
                  class="ui-swatch"
                  :style="{ '--swatch': option.swatch }"
                  :aria-label="option.label"
                  :title="option.label"
                  :aria-pressed="accent === option.id ? 'true' : 'false'"
                  @click="setAccent(option.id)"
                ></button>
              </div>
            </div>

            <div class="app-menu-section">
              <router-link to="/profile" class="dropdown-item" @click="closeUserMenu()">
                <Icon name="user" size="sm" />
                {{ t('nav.profile') }}
              </router-link>
              <router-link to="/keys" class="dropdown-item" @click="closeUserMenu()">
                <Icon name="key" size="sm" />
                {{ t('nav.apiKeys') }}
              </router-link>
              <a
                v-if="authStore.isAdmin"
                href="https://github.com/Wei-Shaw/sub2api"
                target="_blank"
                rel="noopener noreferrer"
                class="dropdown-item"
                @click="closeUserMenu()"
              >
                <Icon name="externalLink" size="sm" />
                {{ t('nav.github') }}
              </a>
            </div>

            <div v-if="contactInfo" class="app-menu-section app-menu-identity">
              <span>{{ t('common.contactSupport') }}</span>
              <strong class="font-medium">{{ contactInfo }}</strong>
            </div>

            <div class="app-menu-section">
              <button type="button" class="dropdown-item text-red-600" @click="handleLogout">
                <Icon name="login" size="sm" class="rotate-180" />
                {{ t('nav.logout') }}
              </button>
            </div>
          </div>
        </Transition>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { onClickOutside, onKeyStroke, useMutationObserver } from '@vueuse/core'
import { useAppStore, useAuthStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import SubscriptionProgressMini from '@/components/common/SubscriptionProgressMini.vue'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'
import { useCommandPalette } from '@/composables/useCommandPalette'
import { ACCENTS, ACCENT_SWATCH, applyAccent, readAccent, type Accent } from '@/utils/accent'
import Icon from '@/components/icons/Icon.vue'
import { usePageMeta } from '@/composables/usePageMeta'
import { sanitizeUrl } from '@/utils/url'
import { applyTheme } from '@/utils/viewTransition'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

defineProps<{ scrolled?: boolean }>()

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { title, area } = usePageMeta()

const user = computed(() => authStore.user)
const palette = useCommandPalette()
const accent = ref<Accent>(readAccent())
const shortcutLabel = /Mac|iPhone|iPad/.test(navigator.platform) ? '⌘K' : 'Ctrl K'
const accentOptions = computed(() => ACCENTS.map((id) => ({
  id,
  swatch: ACCENT_SWATCH[id],
  label: t(`nav.accent${id[0].toUpperCase()}${id.slice(1)}`),
})))
const userMenuOpen = ref(false)
const userMenuRef = ref<HTMLElement | null>(null)
const isDark = ref(document.documentElement.classList.contains('dark'))
// The palette and other surfaces can switch the theme too; follow <html>.
useMutationObserver(document.documentElement, () => {
  isDark.value = document.documentElement.classList.contains('dark')
}, { attributes: true, attributeFilter: ['class'] })
const isFixtureReview = window.__OPERATOR_REVIEW_MODE__ === true

const contactInfo = computed(() => appStore.contactInfo)
const docUrl = computed(() => sanitizeUrl(appStore.docUrl))
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))
// The subscription badge fetches on mount, so it is not mounted when the feature is off.
const subscriptionFeatureEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.subscription))
// Balance and subscription progress belong to personal pages, not operator routes.
const showPersonalStatus = computed(() => !route.path.startsWith('/admin'))

const avatarUrl = computed(() => user.value?.avatar_url?.trim() || '')
const availableBalance = computed(() => Number(user.value?.balance || 0))
const frozenBalance = computed(() => Number(user.value?.frozen_balance || 0))
const totalBalance = computed(() => availableBalance.value + frozenBalance.value)
const balanceFrozenText = computed(() => t('common.frozenBalance'))
const balanceFrozenLabel = computed(() => `${balanceFrozenText.value} ${formatHeaderMoney(frozenBalance.value)}`)
const balanceTitle = computed(() => [
  `${t('common.availableBalance')}: ${formatHeaderMoney(availableBalance.value)}`,
  `${t('common.frozenBalance')}: ${formatHeaderMoney(frozenBalance.value)}`,
  `${t('common.totalBalance')}: ${formatHeaderMoney(totalBalance.value)}`,
].join('\n'))

const displayName = computed(() => user.value?.username || user.value?.email?.split('@')[0] || '')
const userInitials = computed(() => {
  if (!user.value) return ''
  const source = user.value.username || user.value.email?.split('@')[0] || ''
  return source.substring(0, 2).toUpperCase()
})

function formatHeaderMoney(value: number) {
  if (!Number.isFinite(value)) return '$0.00'
  return `$${value.toFixed(2)}`
}

function toggleTheme() {
  setTheme(!isDark.value)
}

function setTheme(dark: boolean) {
  if (dark === isDark.value) return
  isDark.value = dark
  applyTheme(dark)
}

function setAccent(next: Accent) {
  accent.value = next
  applyAccent(next)
}

function closeUserMenu(restoreFocus = false) {
  userMenuOpen.value = false
  if (restoreFocus) {
    userMenuRef.value?.querySelector<HTMLButtonElement>('button')?.focus()
  }
}

onClickOutside(userMenuRef, () => {
  if (userMenuOpen.value) closeUserMenu()
})

onKeyStroke('Escape', () => {
  if (userMenuOpen.value) closeUserMenu(true)
})

// Ctrl or Cmd + B toggles the sidebar, as in most editors.
onKeyStroke(['b', 'B'], (event) => {
  if (!(event.metaKey || event.ctrlKey) || event.altKey || event.shiftKey) return
  event.preventDefault()
  appStore.toggleSidebar()
})

async function handleLogout() {
  closeUserMenu()
  try {
    await authStore.logout()
  } catch (error) {
    // Logout failures still return the user to the sign-in page.
    console.error('Logout error:', error)
  }
  await router.push('/login')
}
</script>
