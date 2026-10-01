import { i18n } from '@/i18n'
import type { RouteLocationNormalizedLoaded } from 'vue-router'
import type { CustomMenuItem } from '@/types'
import { resolveOperatorProductName } from '@/utils/branding'
import type { SiteBillingMode } from '@/utils/siteBillingMode'

/**
 * 统一生成页面标题，避免多处写入 document.title 产生覆盖冲突。
 * 优先使用 titleKey 通过 i18n 翻译，fallback 到静态 routeTitle。
 */
export function resolveDocumentTitle(routeTitle: unknown, siteName?: string, titleKey?: string): string {
  const normalizedSiteName = resolveOperatorProductName(siteName)
  let pageTitle = ''

  if (typeof titleKey === 'string' && titleKey.trim()) {
    const translated = i18n.global.t(titleKey)
    if (translated && translated !== titleKey) {
      pageTitle = translated
    }
  }

  if (!pageTitle && typeof routeTitle === 'string' && routeTitle.trim()) {
    pageTitle = routeTitle.trim()
  }

  return !pageTitle || pageTitle.toLocaleLowerCase() === normalizedSiteName.toLocaleLowerCase()
    ? normalizedSiteName
    : `${pageTitle} · ${normalizedSiteName}`
}

export interface RouteTitleOptions {
  billingMode?: SiteBillingMode
}

export const PURCHASE_ROUTE_NAME = 'PurchaseSubscription'

export function resolveRouteMetaKeys(
  route: Pick<RouteLocationNormalizedLoaded, 'name' | 'meta'>,
  options: RouteTitleOptions = {},
): { titleKey?: string; descriptionKey?: string } {
  if (route.name === PURCHASE_ROUTE_NAME) {
    if (options.billingMode === 'recharge_only') {
      return { titleKey: 'nav.recharge', descriptionKey: 'purchase.rechargeDescription' }
    }
    if (options.billingMode === 'subscription_only') {
      return { titleKey: 'nav.subscribe', descriptionKey: 'purchase.subscriptionDescription' }
    }
  }
  return { titleKey: route.meta.titleKey, descriptionKey: route.meta.descriptionKey }
}

export function resolveRouteDocumentTitle(
  route: Pick<RouteLocationNormalizedLoaded, 'name' | 'params' | 'meta'>,
  siteName: string | undefined,
  customMenuItems: CustomMenuItem[] = [],
  options: RouteTitleOptions = {},
): string {
  const id = typeof route.params.id === 'string' ? route.params.id : ''
  const menuItem = route.name === 'CustomPage' && id
    ? customMenuItems.find((item) => item.id === id)
    : undefined
  const menuTitle = menuItem?.label.trim()

  return resolveDocumentTitle(menuTitle || route.meta.title, siteName, menuTitle ? undefined : resolveRouteMetaKeys(route, options).titleKey)
}
