import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAppStore, useAuthStore } from '@/stores'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { resolveRouteMetaKeys } from '@/router/title'
import { getOperatorArea } from '@/router/operatorNavigation'
import { resolveSiteBillingMode } from '@/utils/siteBillingMode'

/**
 * Title, description, and operator area for the current route. The page
 * heading and the header breadcrumb read from here so they never disagree.
 */
export function usePageMeta() {
  const { t } = useI18n()
  const route = useRoute()
  const appStore = useAppStore()
  const authStore = useAuthStore()
  const adminSettingsStore = useAdminSettingsStore()

  const metaKeys = computed(() =>
    resolveRouteMetaKeys(route, {
      billingMode: resolveSiteBillingMode(appStore.cachedPublicSettings),
    }),
  )

  const title = computed(() => {
    if (route.name === 'CustomPage') {
      const id = route.params.id as string
      const publicItems = appStore.cachedPublicSettings?.custom_menu_items ?? []
      const menuItem = publicItems.find((item) => item.id === id)
        ?? (authStore.isAdmin ? adminSettingsStore.customMenuItems.find((item) => item.id === id) : undefined)
      if (menuItem?.label) return menuItem.label
    }
    const titleKey = metaKeys.value.titleKey
    return titleKey ? t(titleKey) : (route.meta.title as string) || ''
  })

  const description = computed(() => {
    const descriptionKey = metaKeys.value.descriptionKey
    return descriptionKey ? t(descriptionKey) : (route.meta.description as string) || ''
  })

  const area = computed(() => {
    if (!authStore.isAdmin) return undefined
    const currentArea = getOperatorArea(route.path)
    return authStore.isSimpleMode && currentArea?.hideInSimpleMode ? undefined : currentArea
  })

  return { title, description, area }
}
