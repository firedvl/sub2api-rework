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
        <span class="sidebar-logo">
          <img v-if="settingsLoaded" :src="siteLogo || '/logo.svg'" alt="" aria-hidden="true" />
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

    <nav ref="sidebarNavRef" class="sidebar-nav" :aria-label="t('nav.mainNavigation')">
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
          </router-link>
        </template>
      </div>
    </nav>

    <div class="sidebar-footer hidden lg:flex">
      <button
        type="button"
        class="sidebar-link"
        :title="sidebarCollapsed ? t('nav.expand') : undefined"
        :aria-label="sidebarCollapsed ? t('nav.expand') : t('nav.collapse')"
        :aria-expanded="sidebarCollapsed ? 'false' : 'true'"
        @click="toggleSidebar"
      >
        <Icon :name="sidebarCollapsed ? 'chevronDoubleRight' : 'chevronDoubleLeft'" class="sidebar-link-icon" />
        <span class="sidebar-link-label">{{ t('nav.collapse') }}</span>
      </button>
    </div>
  </aside>

  <Transition name="sidebar-overlay">
    <div v-if="mobileOpen" class="sidebar-overlay lg:hidden" aria-hidden="true" @click="closeMobile"></div>
  </Transition>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { onKeyStroke, useMediaQuery } from '@vueuse/core'
import { useAdminSettingsStore, useAppStore, useAuthStore } from '@/stores'
import VersionBadge from '@/components/common/VersionBadge.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeSvg } from '@/utils/sanitize'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, makeSidebarFlag } from '@/utils/featureFlags'
import { resolveSiteBillingMode } from '@/utils/siteBillingMode'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
import { getOperatorArea, operatorAreas, type OperatorAreaId } from '@/router/operatorNavigation'

type IconName = InstanceType<typeof Icon>['$props']['name']

interface NavItem {
  path: string
  label: string
  icon: IconName | null
  iconSvg?: string
  hideInSimpleMode?: boolean
  /** Operator area that owns this entry; its sections share the active state. */
  areaId?: OperatorAreaId
  children?: NavItem[]
  /**
   * Feature gate. `false` hides the entry; `undefined` (settings not loaded
   * yet) keeps it visible so the menu does not flicker on refresh.
   */
  featureFlag?: () => boolean | undefined
}

interface NavSection {
  key: string
  title?: string
  items: NavItem[]
}

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

const isDesktop = useMediaQuery('(min-width: 1024px)')
const sidebarCollapsed = computed(() => isDesktop.value && appStore.sidebarCollapsed)
const mobileOpen = computed(() => appStore.mobileOpen)
const isAdmin = computed(() => authStore.isAdmin)
const sidebarNavRef = ref<HTMLElement | null>(null)
const homePath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const groupExpandOverrides = ref<Map<string, boolean>>(new Map())

const siteName = computed(() => appStore.siteName)
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteVersion = computed(() => appStore.siteVersion)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const areaIcons: Record<OperatorAreaId, IconName> = {
  overview: 'grid',
  stats: 'chartBar',
  accounts: 'server',
  'models-routing': 'layers',
  activity: 'trendingUp',
  settings: 'cog',
}

// Public-settings flags go through the registry in utils/featureFlags.ts,
// which resolves unloaded settings without flicker. Admin-only flags stay inline.
const flagChannelMonitor = makeSidebarFlag(FeatureFlags.channelMonitor)
const flagPayment = makeSidebarFlag(FeatureFlags.payment)
const flagAvailableChannels = makeSidebarFlag(FeatureFlags.availableChannels)
const flagSubscription = makeSidebarFlag(FeatureFlags.subscription)
const flagAffiliate = makeSidebarFlag(FeatureFlags.affiliate)
const flagPluginManagement = makeSidebarFlag(FeatureFlags.pluginManagement)
const flagAdminPayment = () => adminSettingsStore.paymentEnabled
const flagBatchImageAccess = () => canUseBatchImage.value

const purchaseNavLabel = computed(() => {
  switch (resolveSiteBillingMode(appStore.cachedPublicSettings)) {
    case 'recharge_only':
      return t('nav.recharge')
    case 'subscription_only':
      return t('nav.subscribe')
    default:
      return t('nav.buySubscription')
  }
})

function applyFeatureFlags(items: NavItem[]): NavItem[] {
  const out: NavItem[] = []
  for (const item of items) {
    if (item.featureFlag && item.featureFlag() === false) continue
    out.push(item.children ? { ...item, children: applyFeatureFlags(item.children) } : item)
  }
  return out
}

function applySimpleMode(items: NavItem[]): NavItem[] {
  return items.flatMap((item) => {
    if (item.hideInSimpleMode) return []
    if (!item.children) return [item]
    const children = applySimpleMode(item.children)
    return children.length ? [{ ...item, children }] : []
  })
}

function finalizeNav(items: NavItem[]): NavItem[] {
  const visible = applyFeatureFlags(items)
  const filtered = authStore.isSimpleMode ? applySimpleMode(visible) : visible
  return filtered.filter((item) => !item.children || item.children.length > 0)
}

const customMenuItemsForUser = computed(() =>
  (appStore.cachedPublicSettings?.custom_menu_items ?? [])
    .filter((item) => item.visibility === 'user')
    .sort((a, b) => a.sort_order - b.sort_order),
)

const customMenuItemsForAdmin = computed(() =>
  adminSettingsStore.customMenuItems
    .filter((item) => item.visibility === 'admin')
    .sort((a, b) => a.sort_order - b.sort_order),
)

// Personal entries are shared by the user menu and the admin "My Account" section.
// Order: keys, usage, available channels, channel status, billing, profile.
function buildSelfNavItems(withDashboard: boolean): NavItem[] {
  const items: NavItem[] = []
  if (withDashboard) {
    items.push({ path: '/dashboard', label: t('nav.dashboard'), icon: 'grid' })
  }
  items.push(
    { path: '/keys', label: t('nav.apiKeys'), icon: 'key' },
    { path: '/batch-image', label: t('nav.batchImage'), icon: 'camera', hideInSimpleMode: true, featureFlag: flagBatchImageAccess },
    { path: '/usage', label: t('nav.usage'), icon: 'chart', hideInSimpleMode: true },
    { path: '/available-channels', label: t('nav.availableChannels'), icon: 'layers', hideInSimpleMode: true, featureFlag: flagAvailableChannels },
    { path: '/monitor', label: t('nav.channelStatus'), icon: 'signal', featureFlag: flagChannelMonitor },
    { path: '/subscriptions', label: t('nav.mySubscriptions'), icon: 'creditCard', hideInSimpleMode: true, featureFlag: flagSubscription },
    { path: '/purchase', label: purchaseNavLabel.value, icon: 'wallet', hideInSimpleMode: true, featureFlag: flagPayment },
    { path: '/orders', label: t('nav.myOrders'), icon: 'documentText', hideInSimpleMode: true, featureFlag: flagPayment },
    { path: '/redeem', label: t('nav.redeem'), icon: 'gift', hideInSimpleMode: true },
    { path: '/affiliate', label: t('nav.affiliate'), icon: 'users', hideInSimpleMode: true, featureFlag: flagAffiliate },
    { path: '/profile', label: t('nav.profile'), icon: 'user' },
    ...customMenuItemsForUser.value.map((item): NavItem => ({
      path: `/custom/${item.id}`,
      label: item.label,
      icon: null,
      iconSvg: item.icon_svg,
    })),
  )
  return items
}

// The operator areas come from the shared area map, so the sidebar, the
// area tabs, and the breadcrumb always agree. Secondary routes stay under More.
const adminNavItems = computed((): NavItem[] => {
  const areaItems: NavItem[] = operatorAreas.map((area) => ({
    path: area.primaryPath,
    label: t(area.labelKey),
    icon: areaIcons[area.id],
    areaId: area.id,
    hideInSimpleMode: area.hideInSimpleMode,
  }))
  const moreItem: NavItem = {
    path: '/admin/more',
    label: t('nav.administration'),
    icon: 'folder',
    children: [
      { path: '/admin/users', label: t('nav.users'), icon: 'users', hideInSimpleMode: true },
      { path: '/admin/subscriptions', label: t('nav.subscriptions'), icon: 'creditCard', hideInSimpleMode: true, featureFlag: flagSubscription },
      { path: '/admin/plugins', label: t('nav.plugins'), icon: 'cube', featureFlag: flagPluginManagement },
      { path: '/admin/announcements', label: t('nav.announcements'), icon: 'bell' },
      { path: '/admin/redeem', label: t('nav.redeemCodes'), icon: 'ticket', hideInSimpleMode: true },
      { path: '/admin/promo-codes', label: t('nav.promoCodes'), icon: 'gift', hideInSimpleMode: true },
      { path: '/admin/affiliates/invites', label: t('nav.affiliateInviteRecords'), icon: 'userPlus', hideInSimpleMode: true, featureFlag: flagAffiliate },
      { path: '/admin/affiliates/rebates', label: t('nav.affiliateRebateRecords'), icon: 'receipt', hideInSimpleMode: true, featureFlag: flagAffiliate },
      { path: '/admin/affiliates/transfers', label: t('nav.affiliateTransferRecords'), icon: 'swap', hideInSimpleMode: true, featureFlag: flagAffiliate },
      { path: '/admin/orders/dashboard', label: t('nav.paymentDashboard'), icon: 'chartBar', hideInSimpleMode: true, featureFlag: flagAdminPayment },
      { path: '/admin/orders', label: t('nav.orderManagement'), icon: 'receipt', hideInSimpleMode: true, featureFlag: flagAdminPayment },
      { path: '/admin/orders/plans', label: t('nav.paymentPlans'), icon: 'creditCard', hideInSimpleMode: true, featureFlag: flagAdminPayment },
      ...customMenuItemsForAdmin.value.map((item): NavItem => ({
        path: `/custom/${item.id}`,
        label: item.label,
        icon: null,
        iconSvg: item.icon_svg,
      })),
    ],
  }

  const items = finalizeNav([...areaItems, moreItem])
  if (authStore.isSimpleMode) {
    items.push({ path: '/keys', label: t('nav.apiKeys'), icon: 'key' })
  }
  return items
})

const sections = computed((): NavSection[] => {
  if (isAdmin.value) {
    const adminSections: NavSection[] = [
      { key: 'operator', title: t('nav.operatorConsole'), items: adminNavItems.value },
    ]
    if (!authStore.isSimpleMode) {
      adminSections.push({ key: 'personal', title: t('nav.myAccount'), items: finalizeNav(buildSelfNavItems(false)) })
    }
    return adminSections
  }
  if (appStore.backendModeEnabled) return []
  return [{ key: 'user', items: finalizeNav(buildSelfNavItems(true)) }]
})

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

function toggleSidebar() {
  appStore.toggleSidebar()
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
