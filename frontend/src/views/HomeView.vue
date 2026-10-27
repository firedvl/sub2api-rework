<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt=""
            aria-hidden="true"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="flex h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt=""
          aria-hidden="true"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="btn btn-primary btn-lg mt-8"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <!-- Default Home Page -->
  <div v-else data-testid="default-home" class="home-shell flex min-h-screen flex-col">
    <header class="px-4 py-4 sm:px-6">
      <nav class="mx-auto flex max-w-6xl items-center justify-between gap-3">
        <div class="flex min-w-0 items-center gap-2.5">
          <span class="grid h-9 w-9 flex-shrink-0 place-items-center overflow-hidden rounded-lg border border-gray-200 bg-white p-1.5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
            <img :src="siteLogo || '/logo.svg'" alt="" aria-hidden="true" class="h-full w-full object-contain" />
          </span>
          <span class="truncate text-[0.9375rem] font-semibold text-gray-900 dark:text-white">{{ siteName }}</span>
        </div>

        <div class="flex items-center gap-1">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="app-icon-button"
            :title="t('home.viewDocs')"
            :aria-label="t('home.viewDocs')"
          >
            <Icon name="book" size="sm" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="app-icon-button"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="sm" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            type="button"
            class="app-icon-button"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            :aria-label="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="btn btn-primary btn-sm ml-1"
          >
            <span v-if="isAuthenticated" class="app-avatar h-5 w-5 bg-white text-[0.625rem] text-gray-900 dark:bg-gray-900 dark:text-white">
              {{ userInitial }}
            </span>
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex-1 px-4 pb-16 pt-10 sm:px-6 lg:pt-16">
      <div class="mx-auto max-w-6xl">
        <section class="grid items-center gap-12 lg:grid-cols-[minmax(0,1fr)_minmax(0,28rem)] lg:gap-16">
          <div class="text-center lg:text-left">
            <h1 class="text-4xl font-semibold tracking-tight text-gray-900 dark:text-white md:text-5xl">
              {{ siteName }}
            </h1>
            <p class="mx-auto mt-4 max-w-xl text-lg leading-relaxed text-gray-600 dark:text-dark-300 lg:mx-0">
              {{ siteSubtitle }}
            </p>
            <div class="mt-8 flex flex-wrap items-center justify-center gap-3 lg:justify-start">
              <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="btn btn-primary btn-lg">
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="sm" :stroke-width="2" />
              </router-link>
              <a
                v-if="docUrl"
                :href="docUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="btn btn-secondary btn-lg"
              >
                <Icon name="book" size="sm" />
                {{ t('home.docs') }}
              </a>
            </div>
            <ul class="mt-8 flex flex-wrap items-center justify-center gap-x-5 gap-y-2 text-sm text-gray-600 dark:text-dark-300 lg:justify-start">
              <li class="inline-flex items-center gap-2">
                <Icon name="swap" size="sm" class="text-primary-600" />
                {{ t('home.tags.subscriptionToApi') }}
              </li>
              <li class="inline-flex items-center gap-2">
                <Icon name="shield" size="sm" class="text-primary-600" />
                {{ t('home.tags.stickySession') }}
              </li>
              <li class="inline-flex items-center gap-2">
                <Icon name="chart" size="sm" class="text-primary-600" />
                {{ t('home.tags.realtimeBilling') }}
              </li>
            </ul>
          </div>

          <div class="terminal-window" aria-hidden="true">
            <div class="terminal-header">
              <div class="terminal-buttons">
                <span></span>
                <span></span>
                <span></span>
              </div>
              <span class="terminal-title">terminal</span>
            </div>
            <div class="terminal-body">
              <div class="code-line line-1">
                <span class="code-prompt">$</span>
                <span class="code-cmd">curl</span>
                <span class="code-flag">-X POST</span>
                <span class="code-url">/v1/messages</span>
              </div>
              <div class="code-line line-2">
                <span class="code-comment"># Routing to upstream...</span>
              </div>
              <div class="code-line line-3">
                <span class="code-success">200 OK</span>
                <span class="code-response">{ "content": "Hello!" }</span>
              </div>
              <div class="code-line line-4">
                <span class="code-prompt">$</span>
                <span class="cursor"></span>
              </div>
            </div>
          </div>
        </section>

        <section class="mt-20 grid gap-4 md:grid-cols-3">
          <article class="card p-6">
            <span class="home-feature-icon"><Icon name="server" size="md" /></span>
            <h3 class="mt-4 text-base font-semibold text-gray-900 dark:text-white">
              {{ t('home.features.unifiedGateway') }}
            </h3>
            <p class="mt-1.5 text-sm leading-relaxed text-gray-600 dark:text-dark-300">
              {{ t('home.features.unifiedGatewayDesc') }}
            </p>
          </article>
          <article class="card p-6">
            <span class="home-feature-icon"><Icon name="users" size="md" /></span>
            <h3 class="mt-4 text-base font-semibold text-gray-900 dark:text-white">
              {{ t('home.features.multiAccount') }}
            </h3>
            <p class="mt-1.5 text-sm leading-relaxed text-gray-600 dark:text-dark-300">
              {{ t('home.features.multiAccountDesc') }}
            </p>
          </article>
          <article class="card p-6">
            <span class="home-feature-icon"><Icon name="wallet" size="md" /></span>
            <h3 class="mt-4 text-base font-semibold text-gray-900 dark:text-white">
              {{ t('home.features.balanceQuota') }}
            </h3>
            <p class="mt-1.5 text-sm leading-relaxed text-gray-600 dark:text-dark-300">
              {{ t('home.features.balanceQuotaDesc') }}
            </p>
          </article>
        </section>

        <section class="mt-20 text-center">
          <h2 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-white">
            {{ t('home.providers.title') }}
          </h2>
          <p class="mt-2 text-sm text-gray-600 dark:text-dark-300">
            {{ t('home.providers.description') }}
          </p>
          <ul class="mt-8 flex flex-wrap items-center justify-center gap-3">
            <li v-for="provider in providers" :key="provider.key" class="home-provider">
              <span class="home-provider-mark" :style="{ backgroundColor: provider.color }">{{ provider.mark }}</span>
              <span class="text-sm font-medium text-gray-800 dark:text-dark-100">{{ provider.label }}</span>
              <span class="badge badge-primary">{{ t('home.providers.supported') }}</span>
            </li>
            <li class="home-provider opacity-70">
              <span class="home-provider-mark bg-gray-400 dark:bg-dark-500">+</span>
              <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ t('home.providers.more') }}</span>
              <span class="badge badge-gray">{{ t('home.providers.soon') }}</span>
            </li>
          </ul>
        </section>
      </div>
    </main>

    <footer class="border-t border-gray-200 px-4 py-6 dark:border-dark-800 sm:px-6">
      <div class="mx-auto flex max-w-6xl flex-col items-center justify-between gap-3 text-center sm:flex-row sm:text-left">
        <p class="text-sm text-gray-500 dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-4 text-sm">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-gray-500 transition-colors hover:text-gray-900 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-gray-500 transition-colors hover:text-gray-900 dark:text-dark-400 dark:hover:text-white"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { resolveOperatorProductDescriptor } from '@/utils/branding'
import { sanitizeUrl } from '@/utils/url'
import { applyTheme } from '@/utils/viewTransition'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.siteName)
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => resolveOperatorProductDescriptor(appStore.cachedPublicSettings?.site_subtitle))
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(
  () => appStore.cachedPublicSettings?.model_plaza_require_auth === true,
)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

// Supported provider chips; colors follow each provider's mark.
const providers = computed(() => [
  { key: 'claude', mark: 'C', label: t('home.providers.claude'), color: '#d97757' },
  { key: 'gpt', mark: 'G', label: 'GPT', color: '#10a37f' },
  { key: 'gemini', mark: 'G', label: t('home.providers.gemini'), color: '#4285f4' },
  { key: 'antigravity', mark: 'A', label: t('home.providers.antigravity'), color: '#e11d48' },
])

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  applyTheme(isDark.value)
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
.home-shell {
  background:
    radial-gradient(70rem 32rem at 50% -12rem, color-mix(in srgb, var(--operator-accent) 8%, transparent), transparent 70%),
    var(--operator-background);
  color: var(--operator-foreground);
}

.home-feature-icon {
  display: grid;
  width: 2.5rem;
  height: 2.5rem;
  place-items: center;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-muted);
  color: var(--operator-accent-text);
}

.home-provider {
  display: inline-flex;
  align-items: center;
  gap: 0.625rem;
  padding: 0.5rem 0.75rem 0.5rem 0.5rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-card);
  box-shadow: var(--operator-shadow-xs);
}

.home-provider-mark {
  display: grid;
  width: 1.75rem;
  height: 1.75rem;
  place-items: center;
  border-radius: var(--operator-radius);
  color: #ffffff;
  font-size: 0.75rem;
  font-weight: 700;
}

/* Terminal demo: a short, one-time sequence that shows a routed request. */
.terminal-window {
  width: 100%;
  max-width: 28rem;
  justify-self: center;
  overflow: hidden;
  border: 1px solid #27272a;
  border-radius: var(--operator-radius-xl);
  background: #111113;
  box-shadow: var(--operator-shadow-lg);
}

.terminal-header {
  display: flex;
  align-items: center;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid #222226;
  background: #18181b;
}

.terminal-buttons {
  display: flex;
  gap: 0.5rem;
}

.terminal-buttons span {
  width: 0.6875rem;
  height: 0.6875rem;
  border-radius: 999px;
  background: #3f3f46;
}

.terminal-title {
  flex: 1;
  margin-right: 3rem;
  color: #71717a;
  font-family: theme('fontFamily.mono');
  font-size: 0.75rem;
  text-align: center;
}

.terminal-body {
  padding: 1.25rem 1.5rem;
  font-family: theme('fontFamily.mono');
  font-size: 0.8125rem;
  line-height: 2;
}

.code-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  opacity: 0;
  animation: line-appear 400ms var(--operator-ease-out) forwards;
}

.line-1 {
  animation-delay: 300ms;
}

.line-2 {
  animation-delay: 900ms;
}

.line-3 {
  animation-delay: 1500ms;
}

.line-4 {
  animation-delay: 2000ms;
}

@keyframes line-appear {
  from {
    opacity: 0;
    transform: translateY(4px);
  }

  to {
    opacity: 1;
    transform: none;
  }
}

.code-prompt {
  color: #4ade80;
  font-weight: 600;
}

.code-cmd {
  color: #93c5fd;
}

.code-flag {
  color: #c4b5fd;
}

.code-url {
  color: #e4e4e7;
}

.code-comment {
  color: #71717a;
}

.code-success {
  padding: 0.0625rem 0.5rem;
  border-radius: 0.25rem;
  background: rgb(34 197 94 / 0.15);
  color: #4ade80;
  font-weight: 600;
}

.code-response {
  color: #fcd34d;
}

.cursor {
  display: inline-block;
  width: 0.5rem;
  height: 1rem;
  background: #4ade80;
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  0%,
  50% {
    opacity: 1;
  }

  51%,
  100% {
    opacity: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .code-line {
    opacity: 1;
  }

  .cursor {
    animation: none !important;
  }
}
</style>
