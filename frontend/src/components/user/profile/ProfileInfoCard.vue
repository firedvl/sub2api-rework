<template>
  <div class="pf-stack">
    <section
      data-testid="profile-overview-hero"
      class="viz-card pf-identity"
      aria-labelledby="profile-identity-name"
    >
      <div class="pf-identity-top">
        <div class="pf-avatar">
          <img
            v-if="avatarUrl"
            :src="avatarUrl"
            :alt="displayName"
            class="h-full w-full object-cover"
          >
          <span v-else aria-hidden="true">{{ avatarInitial }}</span>
        </div>

        <div class="min-w-0 flex-1">
          <div class="flex min-w-0 flex-wrap items-center gap-2">
            <h2 id="profile-identity-name" class="pf-name">{{ displayName }}</h2>
            <span :class="['badge', user?.role === 'admin' ? 'badge-primary' : 'badge-gray']">
              {{ user?.role === 'admin' ? t('profile.administrator') : t('profile.user') }}
            </span>
            <span :class="['badge', user?.status === 'active' ? 'badge-success' : 'badge-danger']">
              {{ user?.status === 'active' ? t('common.active') : t('common.disabled') }}
            </span>
          </div>
          <p v-if="primaryEmailDisplay" class="pf-email">
            <Icon name="mail" size="xs" />
            <span class="truncate">{{ primaryEmailDisplay }}</span>
          </p>
          <ul
            v-if="sourceHints.length"
            class="ui-chips pf-hints"
            :aria-label="t('profile.linkedProfileSources')"
            :title="t('profile.linkedProfileSourcesDescription')"
          >
            <li v-for="hint in sourceHints" :key="hint.key" class="ui-chip">
              <Icon name="link" size="xs" />
              {{ hint.text }}
            </li>
          </ul>
        </div>

        <button
          type="button"
          class="btn btn-secondary btn-sm pf-edit"
          data-testid="profile-edit-toggle"
          :aria-expanded="editing"
          aria-controls="profile-basics"
          @click="editing = !editing"
        >
          <Icon :name="editing ? 'x' : 'edit'" size="xs" />
          {{ editing ? t('common.close') : t('profile.editProfile') }}
        </button>
      </div>

      <dl class="pf-metrics">
        <div data-testid="profile-overview-metric-balance">
          <dt>{{ t('profile.accountBalance') }}</dt>
          <dd>{{ formatCurrency(user?.balance || 0) }}</dd>
        </div>
        <div data-testid="profile-overview-metric-concurrency">
          <dt>{{ t('profile.concurrencyLimit') }}</dt>
          <dd>{{ user?.concurrency || 0 }}</dd>
        </div>
        <div data-testid="profile-overview-metric-member-since">
          <dt>{{ t('profile.memberSince') }}</dt>
          <dd>{{ memberSinceLabel }}</dd>
        </div>
      </dl>

      <div
        v-show="editing"
        id="profile-basics"
        data-testid="profile-basics-panel"
        class="pf-basics"
        :aria-label="t('profile.basicsTitle')"
        role="region"
      >
        <ProfileAvatarCard
          :user="user"
          embedded
        />
        <ProfileEditForm
          :initial-username="user?.username || ''"
          embedded
        />
      </div>
    </section>

    <section
      data-testid="profile-auth-bindings-panel"
      class="viz-card"
      aria-labelledby="profile-connections-title"
    >
      <header class="viz-card-head">
        <h2
          id="profile-connections-title"
          class="viz-card-title inline-flex items-center gap-2"
          :title="t('profile.authBindings.description')"
        >
          <Icon name="link" size="sm" class="text-gray-400" />
          {{ t('profile.authBindings.title') }}
        </h2>
      </header>
      <ProfileIdentityBindingsSection
        :user="user"
        :linuxdo-enabled="linuxdoEnabled"
        :dingtalk-enabled="dingtalkEnabled"
        :oidc-enabled="oidcEnabled"
        :oidc-provider-name="oidcProviderName"
        :wechat-enabled="wechatEnabled"
        :wechat-open-enabled="wechatOpenEnabled"
        :wechat-mp-enabled="wechatMpEnabled"
        embedded
        compact
      />
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ProfileAvatarCard from '@/components/user/profile/ProfileAvatarCard.vue'
import ProfileEditForm from '@/components/user/profile/ProfileEditForm.vue'
import ProfileIdentityBindingsSection from '@/components/user/profile/ProfileIdentityBindingsSection.vue'
import type { User, UserAuthBindingStatus, UserAuthProvider, UserProfileSourceContext } from '@/types'

const props = withDefaults(defineProps<{
  user: User | null
  linuxdoEnabled?: boolean
  dingtalkEnabled?: boolean
  oidcEnabled?: boolean
  oidcProviderName?: string
  wechatEnabled?: boolean
  wechatOpenEnabled?: boolean
  wechatMpEnabled?: boolean
}>(), {
  linuxdoEnabled: false,
  dingtalkEnabled: false,
  oidcEnabled: false,
  oidcProviderName: 'OIDC',
  wechatEnabled: false,
  wechatOpenEnabled: undefined,
  wechatMpEnabled: undefined,
})

const { t } = useI18n()
const editing = ref(false)

function normalizeBindingStatus(binding: boolean | UserAuthBindingStatus | undefined): boolean | null {
  if (typeof binding === 'boolean') {
    return binding
  }
  if (!binding) {
    return null
  }
  if (typeof binding.bound === 'boolean') {
    return binding.bound
  }
  return Boolean(binding.provider_subject || binding.issuer || binding.provider_key)
}

function isEmailBound(user: User | null | undefined): boolean {
  if (typeof user?.email_bound === 'boolean') {
    return user.email_bound
  }

  const nested = user?.auth_bindings?.email ?? user?.identity_bindings?.email
  const normalized = normalizeBindingStatus(nested)
  return normalized ?? false
}

const avatarUrl = computed(() => props.user?.avatar_url?.trim() || '')
const displayName = computed(() => props.user?.username?.trim() || props.user?.email?.trim() || t('profile.user'))
const primaryEmailDisplay = computed(() => {
  const email = props.user?.email?.trim() || ''
  if (!email) {
    return ''
  }
  if (email.endsWith('.invalid') && !isEmailBound(props.user)) {
    return ''
  }
  return email
})
const avatarInitial = computed(() => displayName.value.charAt(0).toUpperCase() || 'U')
const memberSinceLabel = computed(() => {
  const raw = props.user?.created_at?.trim()
  if (!raw) {
    return '-'
  }

  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) {
    return '-'
  }

  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: 'short',
  }).format(date)
})

const providerLabels = computed<Record<UserAuthProvider, string>>(() => ({
  email: t('profile.authBindings.providers.email'),
  linuxdo: t('profile.authBindings.providers.linuxdo'),
  dingtalk: t('profile.authBindings.providers.dingtalk'),
  oidc: t('profile.authBindings.providers.oidc', { providerName: props.oidcProviderName }),
  wechat: t('profile.authBindings.providers.wechat'),
  github: 'GitHub',
  google: 'Google'
}))

function formatCurrency(value: number): string {
  return `$${value.toFixed(2)}`
}

function normalizeProvider(value: string): UserAuthProvider | null {
  const normalized = value.trim().toLowerCase()
  if (
    normalized === 'email' ||
    normalized === 'linuxdo' ||
    normalized === 'dingtalk' ||
    normalized === 'wechat' ||
    normalized === 'github' ||
    normalized === 'google'
  ) {
    return normalized
  }
  if (normalized === 'oidc' || normalized.startsWith('oidc:') || normalized.startsWith('oidc/')) {
    return 'oidc'
  }
  return null
}

function readObjectString(source: Record<string, unknown>, ...keys: string[]): string {
  for (const key of keys) {
    const value = source[key]
    if (typeof value === 'string' && value.trim()) {
      return value.trim()
    }
  }
  return ''
}

function resolveThirdPartySource(
  rawSource: string | UserProfileSourceContext | null | undefined
): { provider: UserAuthProvider; label: string } | null {
  if (!rawSource) {
    return null
  }

  if (typeof rawSource === 'string') {
    const provider = normalizeProvider(rawSource)
    if (!provider || provider === 'email') {
      return null
    }
    return {
      provider,
      label: providerLabels.value[provider]
    }
  }

  const sourceRecord = rawSource as Record<string, unknown>
  const provider = normalizeProvider(
    readObjectString(sourceRecord, 'provider', 'source', 'provider_type', 'auth_provider')
  )
  if (!provider || provider === 'email') {
    return null
  }

  const explicitLabel = readObjectString(
    sourceRecord,
    'provider_label',
    'label',
    'provider_name',
    'providerName'
  )

  return {
    provider,
    label: explicitLabel || providerLabels.value[provider]
  }
}

const sourceHints = computed(() => {
  const currentUser = props.user
  if (!currentUser) {
    return []
  }

  const hints: Array<{ key: string; text: string }> = []
  const avatarSource = resolveThirdPartySource(
    currentUser.profile_sources?.avatar ?? currentUser.avatar_source
  )
  const usernameSource = resolveThirdPartySource(
    currentUser.profile_sources?.username ??
      currentUser.profile_sources?.display_name ??
      currentUser.profile_sources?.nickname ??
      currentUser.display_name_source ??
      currentUser.username_source ??
      currentUser.nickname_source
  )

  if (avatarSource) {
    hints.push({
      key: 'avatar',
      text: t('profile.authBindings.source.avatar', { providerName: avatarSource.label })
    })
  }

  if (usernameSource) {
    hints.push({
      key: 'username',
      text: t('profile.authBindings.source.username', { providerName: usernameSource.label })
    })
  }

  return hints
})
</script>

<style scoped>
.pf-stack {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.pf-identity {
  overflow: hidden;
}

.pf-identity::before {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: radial-gradient(55% 140% at 0% 0%, rgb(var(--operator-accent-rgb) / 0.22), transparent 70%);
  content: '';
  opacity: var(--operator-glow);
  pointer-events: none;
}

.pf-identity-top {
  position: relative;
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 1rem;
  padding: 1.25rem 1.25rem 1rem;
}

.pf-avatar {
  display: grid;
  width: 3.5rem;
  height: 3.5rem;
  flex: none;
  place-items: center;
  overflow: hidden;
  border-radius: 999px;
  background: var(--operator-accent);
  box-shadow: 0 0 0 3px var(--operator-accent-soft);
  color: var(--operator-accent-foreground);
  font-size: 1.375rem;
  font-weight: 650;
}

.pf-name {
  overflow: hidden;
  color: var(--operator-foreground);
  font-size: 1.25rem;
  font-weight: 650;
  letter-spacing: -0.02em;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pf-email {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.375rem;
  margin-top: 0.25rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.pf-hints {
  margin-top: 0.5rem;
}

.pf-edit {
  flex: none;
}

.pf-metrics {
  position: relative;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border-top: 1px solid var(--operator-border-subtle);
}

.pf-metrics > div {
  min-width: 0;
  padding: 0.75rem 1.25rem;
}

.pf-metrics > div + div {
  border-left: 1px solid var(--operator-border-subtle);
}

.pf-metrics dt {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.pf-metrics dd {
  overflow: hidden;
  color: var(--operator-foreground);
  font-size: 1.125rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pf-basics {
  position: relative;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1.5rem;
  padding: 1.125rem 1.25rem 1.25rem;
  border-top: 1px solid var(--operator-border-subtle);
  background: var(--operator-muted);
  animation: pf-reveal var(--operator-duration-fast) var(--operator-ease-out);
}

@keyframes pf-reveal {
  from {
    opacity: 0;
    transform: translateY(-4px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .pf-basics {
    animation: none;
  }
}

@media (max-width: 640px) {
  .pf-identity-top {
    flex-wrap: wrap;
    padding: 1rem;
  }

  .pf-avatar {
    width: 3rem;
    height: 3rem;
    font-size: 1.125rem;
  }

  .pf-edit {
    order: 3;
    margin-left: 4rem;
  }

  .pf-metrics > div {
    padding: 0.625rem 0.75rem;
  }

  .pf-metrics dd {
    font-size: 1rem;
  }

  .pf-basics {
    grid-template-columns: minmax(0, 1fr);
    padding: 1rem;
  }
}
</style>
