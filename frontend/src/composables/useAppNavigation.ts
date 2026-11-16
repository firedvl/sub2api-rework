import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAdminSettingsStore, useAppStore, useAuthStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import { FeatureFlags, makeSidebarFlag } from '@/utils/featureFlags'
import { resolveSiteBillingMode } from '@/utils/siteBillingMode'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
import { operatorAreas, type OperatorAreaId } from '@/router/operatorNavigation'

export type IconName = InstanceType<typeof Icon>['$props']['name']

export interface NavItem {
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

export interface NavSection {
  key: string
  title?: string
  items: NavItem[]
}

/**
 * The navigation model shared by the sidebar, the command palette, and the
 * mobile tab bar: feature flags, simple mode, and roles resolve in one place.
 */
export function useAppNavigation() {
  const { t } = useI18n()
  const appStore = useAppStore()
  const authStore = useAuthStore()
  const adminSettingsStore = useAdminSettingsStore()
  const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

  const isAdmin = computed(() => authStore.isAdmin)

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

  // The console is built for one operator: daily tools sit in one short list,
  // and secondary admin and personal pages fold into two groups below it.
  const sections = computed((): NavSection[] => {
    if (isAdmin.value) {
      const items = [...adminNavItems.value]
      if (!authStore.isSimpleMode) {
        const personal = finalizeNav(buildSelfNavItems(false))
        const keys = personal.find((item) => item.path === '/keys')
        const more = items.findIndex((item) => item.path === '/admin/more')
        if (keys) items.splice(more === -1 ? items.length : more, 0, keys)
        items.push({
          path: '/account/more',
          label: t('nav.myAccount'),
          icon: 'userCircle',
          children: personal.filter((item) => item.path !== '/keys'),
        })
      }
      return [{ key: 'operator', items }]
    }
    if (appStore.backendModeEnabled) return []
    return [{ key: 'user', items: finalizeNav(buildSelfNavItems(true)) }]
  })

  return { sections, isAdmin, adminSettingsStore, refreshBatchImageAccess }
}
