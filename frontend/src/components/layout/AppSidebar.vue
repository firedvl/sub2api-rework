<template>
  <aside
    class="sidebar"
    :class="[
      { 'sidebar-collapsed': sidebarCollapsed },
      mobileOpen ? 'translate-x-0' : '-translate-x-full',
      'lg:translate-x-0'
    ]"
  >
    <div class="sidebar-header">
      <router-link
        :to="homePath"
        class="sidebar-home"
        :aria-label="siteName"
        :title="sidebarCollapsed ? siteName : undefined"
        @click="handleMenuItemClick"
      >
        <span class="sidebar-logo" :class="{ 'sidebar-logo-brand': settingsLoaded && !siteLogo }">
          <img v-if="settingsLoaded && siteLogo" :src="siteLogo" alt="" aria-hidden="true" />
          <svg v-else-if="settingsLoaded" viewBox="0 0 32 32" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M7.5 10.5h8.25A4.25 4.25 0 0 1 20 14.75v.5" />
            <path d="m16.75 12 3.25 3.25L23.25 12" />
            <path d="M24.5 21.5h-8.25A4.25 4.25 0 0 1 12 17.25v-.5" />
            <path d="M15.25 20 12 16.75 8.75 20" />
          </svg>
        </span>
        <span class="sidebar-brand sidebar-brand-title" aria-hidden="true">{{ siteName }}</span>
      </router-link>
      <VersionBadge v-if="!sidebarCollapsed" :version="siteVersion" class="sidebar-version" />
      <button
        type="button"
        class="app-icon-button ml-auto lg:hidden"
        :aria-label="t('common.close')"
        @click="closeMobile"
      >
        <Icon name="x" size="md" />
      </button>
    </div>

    <nav
      ref="sidebarNavRef"
      class="sidebar-nav"
      :class="{ 'has-indicator': indicator.visible }"
      :aria-label="t('nav.mainNavigation')"
    >
      <span
        class="sidebar-indicator"
        aria-hidden="true"
        :style="{ transform: `translateY(${indicator.top}px)`, height: `${indicator.height}px`, opacity: indicator.visible ? 1 : 0 }"
      ></span>
      <div v-for="section in sections" :key="section.key" class="sidebar-section">
        <div v-if="section.title" class="sidebar-section-title">
          <span>{{ section.title }}</span>
        </div>

        <template v-for="item in section.items" :key="item.path">
          <div v-if="item.children?.length" class="sidebar-group">
            <button
              type="button"
              class="sidebar-link"
              :class="{ 'sidebar-link-active': isGroupActive(item) && !isGroupExpanded(item) }"
              :title="sidebarCollapsed ? item.label : undefined"
              :aria-expanded="isGroupExpanded(item) ? 'true' : 'false'"
              :aria-controls="groupPanelId(item)"
              @click="handleGroupClick(item)"
            >
              <Icon v-if="item.icon" :name="item.icon" class="sidebar-link-icon" />
              <span class="sidebar-link-label">{{ item.label }}</span>
              <Icon
                name="chevronDown"
                size="sm"
                class="sidebar-group-chevron transition-transform duration-normal ease-ui-out"
                :class="{ 'rotate-180': isGroupExpanded(item) }"
              />
            </button>
            <div
              :id="groupPanelId(item)"
              class="sidebar-children"
              :class="{ 'sidebar-children-open': isGroupOpen(item) }"
              :inert="isGroupOpen(item) ? undefined : true"
              @transitionend="measureIndicator"
            >
              <div>
                <div class="sidebar-children-inner">
                  <router-link
                    v-for="child in item.children"
                    :key="child.path"
                    :to="child.path"
                    class="sidebar-link"
                    :class="{ 'sidebar-link-active': route.path === child.path }"
                    :aria-current="route.path === child.path ? 'page' : undefined"
                    @click="handleMenuItemClick"
                  >
                    <span
                      v-if="child.iconSvg"
                      class="sidebar-link-icon sidebar-svg-icon"
                      v-html="sanitizeSvg(child.iconSvg)"
                    ></span>
                    <Icon v-else-if="child.icon" :name="child.icon" class="sidebar-link-icon" />
                    <span class="sidebar-link-label">{{ child.label }}</span>
                  </router-link>
                </div>
              </div>
            </div>
          </div>

          <router-link
            v-else
            :to="item.path"
            class="sidebar-link"
            :class="{ 'sidebar-link-active': isItemActive(item) }"
            :title="sidebarCollapsed ? item.label : undefined"
            :aria-current="isItemActive(item) ? 'page' : undefined"
            @click="handleMenuItemClick"
          >
            <span v-if="item.iconSvg" class="sidebar-link-icon sidebar-svg-icon" v-html="sanitizeSvg(item.iconSvg)"></span>
            <Icon v-else-if="item.icon" :name="item.icon" class="sidebar-link-icon" />
            <span class="sidebar-link-label">{{ item.label }}</span>
            <span
              v-if="item.areaId === 'accounts' && pulse.attention.value > 0"
              class="sidebar-count"
              :class="{ 'is-bad': pulse.errorAccounts.value > 0 }"
              :aria-label="t('nav.needAttention', { count: pulse.attention.value })"
            >{{ pulse.attention.value }}</span>
          </router-link>
        </template>
      </div>
    </nav>

    <div v-if="isAdmin" class="sidebar-foot">
      <router-link
        to="/admin/accounts"
        class="sidebar-status"
        :title="sidebarCollapsed ? statusLabel : undefined"
        @click="handleMenuItemClick"
      >
        <span class="viz-pulse" :class="pulseToneClass" aria-hidden="true"></span>
        <b>{{ statusLabel }}</b>
      </router-link>
    </div>

  </aside>

  <Transition name="sidebar-overlay">
    <div v-if="mobileOpen" class="sidebar-overlay lg:hidden" aria-hidden="true" @click="closeMobile"></div>
  </Transition>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { onKeyStroke, useMediaQuery } from '@vueuse/core'
import { useAppStore } from '@/stores'
import VersionBadge from '@/components/common/VersionBadge.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeSvg } from '@/utils/sanitize'
import { sanitizeUrl } from '@/utils/url'
import { getOperatorArea } from '@/router/operatorNavigation'
import { useAppNavigation, type NavItem } from '@/composables/useAppNavigation'
import { useGatewayPulse } from '@/composables/useGatewayPulse'

const route = useRoute()
const appStore = useAppStore()
const { t } = useI18n()
const { sections, isAdmin, adminSettingsStore, refreshBatchImageAccess } = useAppNavigation()

const isDesktop = useMediaQuery('(min-width: 1024px)')
const sidebarCollapsed = computed(() => isDesktop.value && appStore.sidebarCollapsed)
const mobileOpen = computed(() => appStore.mobileOpen)
const sidebarNavRef = ref<HTMLElement | null>(null)
const homePath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const groupExpandOverrides = ref<Map<string, boolean>>(new Map())

const siteName = computed(() => appStore.siteName)
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteVersion = computed(() => appStore.siteVersion)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const pulse = useGatewayPulse(() => isAdmin.value)
const pulseToneClass = computed(() => ({ ok: 'is-ok', warn: 'is-warn', bad: 'is-bad' })[pulse.tone.value])
const statusLabel = computed(() => {
  if (!pulse.loaded.value || pulse.attention.value === 0) return t('nav.statusOperational')
  return t('nav.needAttention', { count: pulse.attention.value })
})

// One highlight slides between items instead of each link painting its own.
const indicator = reactive({ top: 0, height: 0, visible: false })

function measureIndicator() {
  const nav = sidebarNavRef.value
  const active = nav?.querySelector<HTMLElement>('.sidebar-link-active')
  if (!nav || !active || active.offsetParent === null) {
    indicator.visible = false
    return
  }
  const navBox = nav.getBoundingClientRect()
  const box = active.getBoundingClientRect()
  if (box.height === 0) {
    indicator.visible = false
    return
  }
  indicator.top = box.top - navBox.top + nav.scrollTop
  indicator.height = box.height
  indicator.visible = true
}

watch(
  () => [route.path, sidebarCollapsed.value, sections.value.length],
  () => void nextTick(measureIndicator),
)
function isActive(path: string): boolean {
  return route.path === path || route.path.startsWith(`${path}/`)
}

function isItemActive(item: NavItem): boolean {
  if (item.areaId) return getOperatorArea(route.path)?.id === item.areaId
  return isActive(item.path)
}

function isGroupActive(item: NavItem): boolean {
  return item.children?.some((child) => route.path === child.path) ?? false
}

function isGroupExpanded(item: NavItem): boolean {
  return groupExpandOverrides.value.get(item.path) ?? isGroupActive(item)
}

function isGroupOpen(item: NavItem): boolean {
  return !sidebarCollapsed.value && isGroupExpanded(item)
}

function groupPanelId(item: NavItem): string {
  return `sidebar-group-${item.path.replace(/[^a-z0-9]+/gi, '-')}`
}

function handleGroupClick(item: NavItem) {
  // A collapsed rail cannot show children, so expand the sidebar and open the group.
  if (sidebarCollapsed.value) {
    appStore.setSidebarCollapsed(false)
    groupExpandOverrides.value.set(item.path, true)
    return
  }
  groupExpandOverrides.value.set(item.path, !isGroupExpanded(item))
}

function closeMobile() {
  appStore.setMobileOpen(false)
}

function handleMenuItemClick() {
  if (mobileOpen.value) closeMobile()
}

onKeyStroke('Escape', () => {
  if (mobileOpen.value) closeMobile()
})

// Close the drawer when the viewport grows into the persistent sidebar layout.
watch(isDesktop, (desktop) => {
  if (desktop && mobileOpen.value) closeMobile()
})

watch(
  isAdmin,
  (admin) => {
    if (admin) adminSettingsStore.fetch()
  },
  { immediate: true },
)

onMounted(() => {
  void refreshBatchImageAccess()
  void nextTick(measureIndicator)
  window.addEventListener('resize', measureIndicator)
  // Restore the scroll position after a route change re-mounts the sidebar.
  if (appStore.sidebarScrollTop > 0 && sidebarNavRef.value) {
    void nextTick(() => {
      if (sidebarNavRef.value) {
        sidebarNavRef.value.scrollTop = appStore.sidebarScrollTop
      }
    })
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', measureIndicator)
  if (sidebarNavRef.value) {
    appStore.sidebarScrollTop = sidebarNavRef.value.scrollTop
  }
})
</script>

<style scoped>
.sidebar-home {
  display: flex;
  min-width: 0;
  flex: 1 1 auto;
  align-items: center;
  gap: 0.625rem;
  border-radius: var(--operator-radius);
}

.sidebar-home:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: 2px;
}

.sidebar-brand {
  min-width: 0;
  white-space: nowrap;
  transition: opacity var(--operator-duration-fast) var(--operator-ease-out);
}

.sidebar-brand-title {
  overflow: hidden;
  color: var(--operator-foreground);
  font-size: 0.9375rem;
  font-weight: 600;
  letter-spacing: -0.01em;
  text-overflow: ellipsis;
}

.sidebar-version {
  flex: 0 0 auto;
}

.sidebar-group-chevron {
  flex: 0 0 auto;
  color: var(--operator-subtle-foreground);
}

/* Custom SVG icon in the sidebar: constrain size without overriding uploaded SVG colors */
.sidebar-svg-icon {
  color: currentColor;
}

.sidebar-svg-icon :deep(svg) {
  display: block;
  width: 100%;
  height: 100%;
}

.sidebar-overlay-enter-active,
.sidebar-overlay-leave-active {
  transition: opacity var(--operator-duration) var(--operator-ease-out);
}

.sidebar-overlay-enter-from,
.sidebar-overlay-leave-to {
  opacity: 0;
}
</style>
