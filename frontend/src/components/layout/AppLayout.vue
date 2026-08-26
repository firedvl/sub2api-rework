<template>
  <div
    class="min-h-screen bg-gray-50 dark:bg-dark-950"
    :class="{ 'operator-console': isOperatorConsole, 'fixture-review-mode': isFixtureReview }"
  >
    <!-- Background Decoration -->
    <div v-if="!isOperatorConsole" class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

    <!-- Sidebar -->
    <AppSidebar :mobile-only="isOperatorConsole" />

    <!-- Main Content Area -->
    <div
      class="relative flex h-screen flex-col transition-all duration-300"
      :class="isOperatorConsole ? 'operator-shell' : [sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <OperatorHeader v-if="isOperatorConsole" />
      <AppHeader v-else />

      <!-- Main Content -->
      <main
        class="flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto"
        :class="isOperatorConsole ? 'operator-main' : 'p-4 md:p-6 lg:p-8'"
      >
        <div v-if="isOperatorConsole" class="operator-content-shell">
          <header class="operator-page-heading">
            <h1>{{ pageTitle }}</h1>
            <p v-if="pageDescription">{{ pageDescription }}</p>
          </header>
          <OperatorContextNav />
          <slot />
        </div>
        <slot v-else />
      </main>
      <OperatorStatusBar v-if="isOperatorConsole" />
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import OperatorContextNav from './OperatorContextNav.vue'
import OperatorHeader from './OperatorHeader.vue'
import OperatorStatusBar from './OperatorStatusBar.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const route = useRoute()
const { t } = useI18n()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')
const isOperatorConsole = computed(
  () => isAdmin.value && (route.path === '/admin' || route.path.startsWith('/admin/'))
)
const isFixtureReview = window.__OPERATOR_REVIEW_MODE__ === true
const pageTitle = computed(() => {
  const key = route.meta.titleKey as string | undefined
  return key ? t(key) : (route.meta.title as string) || ''
})
const pageDescription = computed(() => {
  const key = route.meta.descriptionKey as string | undefined
  return key ? t(key) : (route.meta.description as string) || ''
})

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>
