import { i18n } from '@/i18n'
import type { RouteLocationNormalizedLoaded } from 'vue-router'
import type { CustomMenuItem } from '@/types'
import { resolveOperatorProductName } from '@/utils/branding'

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

export function resolveRouteDocumentTitle(
  route: Pick<RouteLocationNormalizedLoaded, 'name' | 'params' | 'meta'>,
  siteName: string | undefined,
  customMenuItems: CustomMenuItem[] = [],
): string {
  const id = typeof route.params.id === 'string' ? route.params.id : ''
  const menuItem = route.name === 'CustomPage' && id
    ? customMenuItems.find((item) => item.id === id)
    : undefined
  const menuTitle = menuItem?.label.trim()

  return resolveDocumentTitle(menuTitle || route.meta.title, siteName, menuTitle ? undefined : route.meta.titleKey as string)
}
