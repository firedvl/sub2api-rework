<template>
  <ProfileSettingRow
    icon="shield"
    :title="t('profile.totp.title')"
    title-id="profile-totp-title"
    :tone="status?.enabled ? 'ok' : 'neutral'"
    data-testid="profile-totp-row"
  >
    <template #badge>
      <span v-if="!loading && status?.enabled" class="badge badge-success">{{ t('profile.totp.enabled') }}</span>
      <span v-else-if="!loading && status && status.feature_enabled" class="badge badge-gray">{{ t('profile.totp.notEnabled') }}</span>
    </template>

    <span v-if="loading" class="skeleton inline-block h-3 w-40 rounded"></span>
    <template v-else-if="status && !status.feature_enabled">{{ t('profile.totp.featureDisabledHint') }}</template>
    <template v-else-if="status?.enabled">
      <template v-if="status.enabled_at">{{ t('profile.totp.enabledAt') }}: {{ formatDate(status.enabled_at) }}</template>
    </template>
    <template v-else>{{ t('profile.totp.notEnabledHint') }}</template>

    <template #actions>
      <span v-if="!loading && status && !status.feature_enabled" class="badge badge-gray">{{ t('profile.totp.featureDisabled') }}</span>
      <button
        v-else-if="!loading && status?.enabled"
        type="button"
        class="btn btn-outline-danger btn-sm"
        aria-describedby="profile-totp-title"
        @click="showDisableDialog = true"
      >
        {{ t('profile.totp.disable') }}
      </button>
      <button
        v-else-if="!loading"
        type="button"
        class="btn btn-primary btn-sm"
        aria-describedby="profile-totp-title"
        @click="showSetupModal = true"
      >
        {{ t('profile.totp.enable') }}
      </button>
    </template>

    <template #panel>
    <!-- Teleported: the animated setting row is its own stacking context, so a
         fixed overlay rendered inside it would sit under later rows. -->
    <Teleport to="body">
      <!-- Setup Modal -->
      <TotpSetupModal
        v-if="showSetupModal"
        @close="showSetupModal = false"
        @success="handleSetupSuccess"
      />

      <!-- Disable Dialog -->
      <TotpDisableDialog
        v-if="showDisableDialog"
        @close="showDisableDialog = false"
        @success="handleDisableSuccess"
      />
    </Teleport>
    </template>
  </ProfileSettingRow>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { totpAPI } from '@/api'
import type { TotpStatus } from '@/types'
import ProfileSettingRow from './ProfileSettingRow.vue'
import TotpSetupModal from './TotpSetupModal.vue'
import TotpDisableDialog from './TotpDisableDialog.vue'

const { t } = useI18n()

const loading = ref(true)
const status = ref<TotpStatus | null>(null)
const showSetupModal = ref(false)
const showDisableDialog = ref(false)

const loadStatus = async () => {
  loading.value = true
  try {
    status.value = await totpAPI.getStatus()
  } catch (error) {
    console.error('Failed to load TOTP status:', error)
  } finally {
    loading.value = false
  }
}

const handleSetupSuccess = () => {
  showSetupModal.value = false
  loadStatus()
}

const handleDisableSuccess = () => {
  showDisableDialog.value = false
  loadStatus()
}

const formatDate = (timestamp: number) => {
  // Backend returns Unix timestamp in seconds, convert to milliseconds
  const date = new Date(timestamp * 1000)
  return date.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

onMounted(() => {
  loadStatus()
})
</script>
