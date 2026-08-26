<template>
  <header class="operator-header">
    <div class="operator-header-inner">
      <div class="operator-brand-wrap">
        <button
          type="button"
          class="operator-icon-button lg:hidden"
          :aria-label="t('common.toggleMenu')"
          @click="appStore.toggleMobileSidebar()"
        >
          <Icon name="menu" size="md" />
        </button>
        <router-link to="/admin/dashboard" class="operator-brand">
          <span class="operator-brand-mark">
            <img v-if="settingsLoaded" :src="siteLogo || '/logo.svg'" alt="" />
          </span>
          <span>{{ siteName }}</span>
        </router-link>
      </div>

      <nav class="operator-primary-nav" :aria-label="t('nav.operatorConsole')">
        <router-link
          v-for="area in visibleAreas"
          :key="area.id"
          :to="area.primaryPath"
          class="operator-primary-link"
          :class="{ 'operator-primary-link-active': activeArea?.id === area.id }"
          :aria-current="activeArea?.id === area.id ? 'page' : undefined"
        >
          {{ t(area.labelKey) }}
        </router-link>
      </nav>

      <div class="operator-header-actions">
        <AnnouncementBell class="operator-header-secondary-action" />
        <LocaleSwitcher class="operator-header-secondary-action" />
        <button
          type="button"
          class="operator-icon-button"
          :aria-label="isDark ? t('nav.lightMode') : t('nav.darkMode')"
          @click="toggleTheme"
        >
          <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
        </button>
        <button type="button" class="operator-logout" @click="handleLogout">
          <Icon name="login" size="sm" class="rotate-180" />
          <span class="hidden xl:inline">{{ t('nav.logout') }}</span>
        </button>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore, useAuthStore } from '@/stores'
import { getOperatorArea, operatorAreas } from '@/router/operatorNavigation'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const isDark = ref(document.documentElement.classList.contains('dark'))

const visibleAreas = computed(() =>
  operatorAreas.filter((area) => !(authStore.isSimpleMode && area.hideInSimpleMode)),
)
const activeArea = computed(() => getOperatorArea(route.path))
const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() =>
  sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }),
)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

async function handleLogout() {
  try {
    await authStore.logout()
  } catch (error) {
    console.error('Logout error:', error)
  }
  await router.push('/login')
}
</script>
