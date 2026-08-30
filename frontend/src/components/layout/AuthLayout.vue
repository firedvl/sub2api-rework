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
            <img :src="siteLogo || '/logo.svg'" alt="" aria-hidden="true" class="h-full w-full object-contain" />
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

      <!-- Technical attribution -->
      <div class="auth-codex-copyright">
        {{ t('auth.poweredBy', { product: TECHNICAL_PRODUCT_NAME }) }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import {
  TECHNICAL_PRODUCT_NAME,
  resolveOperatorProductDescriptor,
  resolveOperatorProductName,
} from '@/utils/branding'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const appStore = useAppStore()

const siteName = computed(() => resolveOperatorProductName(appStore.siteName))
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() =>
  resolveOperatorProductDescriptor(appStore.cachedPublicSettings?.site_subtitle),
)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const isFixtureReview = window.__OPERATOR_REVIEW_MODE__ === true

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>
