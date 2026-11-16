<template>
  <AppLayout>
    <div
      data-testid="profile-shell"
      class="ui-page ui-stagger pv-shell"
    >
      <ProfileInfoCard
        :user="user"
        :linuxdo-enabled="linuxdoOAuthEnabled"
        :dingtalk-enabled="dingtalkOAuthEnabled"
        :oidc-enabled="oidcOAuthEnabled"
        :oidc-provider-name="oidcOAuthProviderName"
        :wechat-enabled="wechatOAuthEnabled"
        :wechat-open-enabled="wechatOAuthOpenEnabled"
        :wechat-mp-enabled="wechatOAuthMPEnabled"
      />

      <section class="viz-card" aria-labelledby="profile-security-title" data-testid="profile-security">
        <header class="viz-card-head">
          <h2 id="profile-security-title" class="viz-card-title inline-flex items-center gap-2">
            <Icon name="shield" size="sm" class="text-gray-400" />
            {{ t('personalUi.profile.security') }}
          </h2>
        </header>
        <ProfileSettingRow icon="lock" :title="t('common.password')" title-id="profile-password-title">
          {{ t('profile.passwordHint') }}
          <template #actions>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              data-testid="profile-password-toggle"
              :aria-expanded="passwordOpen"
              aria-controls="profile-password-panel"
              aria-describedby="profile-password-title"
              @click="passwordOpen = !passwordOpen"
            >
              {{ passwordOpen ? t('common.cancel') : t('personalUi.profile.change') }}
            </button>
          </template>
          <template #panel>
            <div
              v-show="passwordOpen"
              id="profile-password-panel"
              class="pv-panel"
            >
              <ProfilePasswordForm embedded />
            </div>
          </template>
        </ProfileSettingRow>
        <ProfileTotpCard />
        <ProfilePasskeyCard :enabled="passkeyEnabled" />
      </section>

      <ProfileBalanceNotifyCard
        v-if="user && balanceLowNotifyEnabled"
        :enabled="user.balance_notify_enabled ?? true"
        :threshold="user.balance_notify_threshold"
        :extra-emails="user.balance_notify_extra_emails ?? []"
        :system-default-threshold="systemDefaultThreshold"
        :user-email="user.email"
      />

      <section
        v-if="contactInfo"
        class="viz-card pv-support"
        :aria-label="t('common.contactSupport')"
      >
        <span class="pv-support-icon" aria-hidden="true"><Icon name="chat" size="sm" /></span>
        <span class="min-w-0">
          <span class="ui-cell-sub">{{ t('common.contactSupport') }}</span>
          <span class="ui-cell-main whitespace-normal break-words">{{ contactInfo }}</span>
        </span>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@/components/icons'
import AppLayout from '@/components/layout/AppLayout.vue'
import ProfileBalanceNotifyCard from '@/components/user/profile/ProfileBalanceNotifyCard.vue'
import ProfileInfoCard from '@/components/user/profile/ProfileInfoCard.vue'
import ProfilePasswordForm from '@/components/user/profile/ProfilePasswordForm.vue'
import ProfileSettingRow from '@/components/user/profile/ProfileSettingRow.vue'
import ProfileTotpCard from '@/components/user/profile/ProfileTotpCard.vue'
import ProfilePasskeyCard from '@/components/user/profile/ProfilePasskeyCard.vue'
import { isWeChatWebOAuthEnabled } from '@/api/auth'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const user = computed(() => authStore.user)

const passwordOpen = ref(false)
const contactInfo = ref('')
const balanceLowNotifyEnabled = ref(false)
const systemDefaultThreshold = ref(0)
const linuxdoOAuthEnabled = ref(false)
const dingtalkOAuthEnabled = ref(false)
const wechatOAuthEnabled = ref(false)
const wechatOAuthOpenEnabled = ref<boolean | undefined>(undefined)
const wechatOAuthMPEnabled = ref<boolean | undefined>(undefined)
const oidcOAuthEnabled = ref(false)
const oidcOAuthProviderName = ref('OIDC')
const passkeyEnabled = ref(false)

onMounted(async () => {
  const profileRefresh = authStore.refreshUser().catch((error) => {
    console.error('Failed to refresh profile:', error)
  })

  const settingsLoad = appStore.fetchPublicSettings()
    .then((settings) => {
      if (!settings) {
        return
      }
      contactInfo.value = settings.contact_info || ''
      balanceLowNotifyEnabled.value = settings.balance_low_notify_enabled ?? false
      systemDefaultThreshold.value = settings.balance_low_notify_threshold ?? 0
      linuxdoOAuthEnabled.value = settings.linuxdo_oauth_enabled ?? false
      dingtalkOAuthEnabled.value = settings.dingtalk_oauth_enabled ?? false
      wechatOAuthEnabled.value = isWeChatWebOAuthEnabled(settings)
      wechatOAuthOpenEnabled.value = typeof settings.wechat_oauth_open_enabled === 'boolean'
        ? settings.wechat_oauth_open_enabled
        : undefined
      wechatOAuthMPEnabled.value = typeof settings.wechat_oauth_mp_enabled === 'boolean'
        ? settings.wechat_oauth_mp_enabled
        : undefined
      oidcOAuthEnabled.value = settings.oidc_oauth_enabled ?? false
      oidcOAuthProviderName.value = settings.oidc_oauth_provider_name || 'OIDC'
      passkeyEnabled.value = settings.passkey_enabled === true
    })
    .catch((error) => {
      console.error('Failed to load settings:', error)
    })

  await Promise.all([profileRefresh, settingsLoad])
})
</script>

<style scoped>
.pv-shell {
  max-width: 56rem;
}

.pv-panel {
  max-width: 28rem;
  margin: 0.875rem 0 0.25rem 2.875rem;
  animation: pv-reveal var(--operator-duration-fast) var(--operator-ease-out);
}

@keyframes pv-reveal {
  from {
    opacity: 0;
    transform: translateY(-4px);
  }
}

.pv-support {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.875rem 1.125rem;
}

.pv-support-icon {
  display: inline-grid;
  width: 2.125rem;
  height: 2.125rem;
  flex: none;
  place-items: center;
  border-radius: var(--operator-radius-sm);
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
}

@media (prefers-reduced-motion: reduce) {
  .pv-panel {
    animation: none;
  }
}

@media (max-width: 480px) {
  .pv-panel {
    margin-left: 0;
  }
}
</style>
