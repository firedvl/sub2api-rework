<template>
  <div class="auth-codex-shell">
    <div v-if="isFixtureReview" class="auth-review-banner" role="status">
      Local fixture review · no production connection
    </div>

    <!-- Content Container -->
    <div class="auth-codex-frame">
      <!-- Logo/Brand -->
      <div class="auth-codex-brand">
        <!-- Custom Logo or Default Logo -->
        <template v-if="settingsLoaded">
          <div class="auth-codex-logo">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <h1>
            {{ siteName }}
          </h1>
          <p>
            {{ siteSubtitle }}
          </p>
        </template>
      </div>

      <!-- Card Container -->
      <div class="auth-codex-card">
        <slot />
      </div>

      <!-- Footer Links -->
      <div class="auth-codex-footer">
        <slot name="footer" />
      </div>

      <!-- Copyright -->
      <div class="auth-codex-copyright">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const isFixtureReview = window.__OPERATOR_REVIEW_MODE__ === true

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>
