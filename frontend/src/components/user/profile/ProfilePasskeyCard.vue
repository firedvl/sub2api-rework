<template>
  <ProfileSettingRow
    icon="key"
    :title="t('profile.passkey.title')"
    title-id="profile-passkey-title"
    :tone="credentials.length ? 'ok' : 'neutral'"
    data-testid="profile-passkey-row"
  >
    <template #badge>
      <span v-if="credentials.length" class="badge badge-success tabular-nums">{{ credentials.length }}</span>
    </template>

    <span v-if="!enabled">{{ t('profile.passkey.featureDisabled') }}</span>
    <span v-else-if="!supported" class="text-amber-600 dark:text-amber-400">{{ t('profile.passkey.unsupported') }}</span>
    <span v-else-if="!loading && credentials.length === 0">{{ t('profile.passkey.empty') }}</span>
    <span v-else>{{ t('profile.passkey.description') }}</span>

    <template #actions>
      <button
        v-if="enabled && supported && !showAddForm"
        type="button"
        class="btn btn-secondary btn-sm"
        aria-describedby="profile-passkey-title"
        :disabled="busy"
        @click="showAddForm = true"
      >
        <Icon name="plus" size="xs" />
        {{ t('profile.passkey.add') }}
      </button>
    </template>

    <template #panel>
      <Transition name="fade">
        <form
          v-if="enabled && supported && showAddForm"
          class="pk-panel"
          @submit.prevent="addPasskey"
        >
          <div class="grid gap-3 sm:grid-cols-2">
            <div>
              <label for="passkey-name" class="input-label">{{ t('profile.passkey.name') }}</label>
              <input
                id="passkey-name"
                v-model="newName"
                class="input"
                maxlength="100"
                :placeholder="t('profile.passkey.namePlaceholder')"
                autofocus
              />
            </div>
            <div>
              <label for="passkey-add-password" class="input-label">{{
                t('profile.currentPassword')
              }}</label>
              <input
                id="passkey-add-password"
                v-model="newPassword"
                type="password"
                autocomplete="current-password"
                class="input"
                :placeholder="t('profile.passkey.passwordPlaceholder')"
              />
            </div>
          </div>
          <div class="flex justify-end gap-2">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="cancelAdd">
              {{ t('common.cancel') }}
            </button>
            <button type="submit" class="btn btn-primary btn-sm" :disabled="busy || newPassword.length === 0">
              {{ busy ? t('common.processing') : t('profile.passkey.continue') }}
            </button>
          </div>
        </form>
      </Transition>

      <div v-if="loading" class="pk-list">
        <div class="skeleton h-10 rounded-lg"></div>
      </div>
      <ul v-else-if="credentials.length" class="pk-list ui-rows-enter">
        <li
          v-for="credential in credentials"
          :key="credential.id"
          class="pk-item"
        >
          <div class="min-w-0 flex-1">
            <div class="flex min-w-0 items-center gap-2">
              <span class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ credential.name }}</span>
              <span v-if="credential.backup" class="badge badge-success">{{ t('profile.passkey.synced') }}</span>
            </div>
            <p class="ui-cell-sub">
              {{ t('profile.passkey.createdAt', { date: formatDate(credential.created_at) }) }}
              <template v-if="credential.last_used_at">
                · {{ t('profile.passkey.lastUsed', { date: formatDate(credential.last_used_at) }) }}
              </template>
            </p>
          </div>
          <div class="flex shrink-0 gap-1">
            <button
              type="button"
              class="app-icon-button"
              :disabled="busy"
              :aria-label="`${t('common.edit')}: ${credential.name}`"
              :title="t('common.edit')"
              @click="renamePasskey(credential)"
            >
              <Icon name="edit" size="sm" />
            </button>
            <button
              type="button"
              class="app-icon-button pk-danger"
              :disabled="busy"
              :aria-label="`${t('common.delete')}: ${credential.name}`"
              :title="t('common.delete')"
              @click="deletePasskey(credential)"
            >
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </li>
      </ul>

    <!-- 删除确认：吊销凭据需验证当前密码，防止被窃会话静默移除 Passkey -->
    <div v-if="deleteTarget" class="fixed inset-0 z-50 overflow-y-auto">
      <div class="flex min-h-full items-center justify-center p-4">
        <div class="fixed inset-0 bg-black/50 transition-opacity" @click="closeDeleteDialog"></div>
        <div
          class="relative w-full max-w-md transform rounded-xl bg-white p-6 shadow-xl transition-all dark:bg-dark-800"
        >
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('profile.passkey.deleteTitle') }}
          </h3>
          <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
            {{ t('profile.passkey.deleteConfirm', { name: deleteTarget.name }) }}
          </p>
          <form class="mt-4 space-y-4" @submit.prevent="confirmDelete">
            <div>
              <label for="passkey-delete-password" class="input-label">{{
                t('profile.currentPassword')
              }}</label>
              <input
                id="passkey-delete-password"
                v-model="deletePassword"
                type="password"
                autocomplete="current-password"
                class="input"
                :placeholder="t('profile.passkey.passwordPlaceholder')"
                autofocus
              />
            </div>
            <div class="flex justify-end gap-3">
              <button type="button" class="btn btn-secondary" :disabled="busy" @click="closeDeleteDialog">
                {{ t('common.cancel') }}
              </button>
              <button
                type="submit"
                class="btn btn-danger"
                :disabled="busy || deletePassword.length === 0"
              >
                {{ busy ? t('common.processing') : t('common.delete') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
    </template>
  </ProfileSettingRow>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { passkeyAPI, type PasskeyCredentialSummary } from '@/api'
import { Icon } from '@/components/icons'
import { useAppStore } from '@/stores/app'
import ProfileSettingRow from './ProfileSettingRow.vue'

const props = defineProps<{ enabled: boolean }>()

const { t } = useI18n()
const appStore = useAppStore()
const supported = passkeyAPI.isSupported()
const loading = ref(false)
const busy = ref(false)
const showAddForm = ref(false)
const newName = ref('')
const newPassword = ref('')
const deleteTarget = ref<PasskeyCredentialSummary | null>(null)
const deletePassword = ref('')
const credentials = ref<PasskeyCredentialSummary[]>([])

// apiClient 拦截器把错误规范化为 { code, reason, message }；
// 透出后端消息（如密码错误），否则回退到通用文案。
function extractErrorMessage(error: unknown, fallback: string): string {
  const message = (error as { message?: string }).message
  return typeof message === 'string' && message.length > 0 ? message : fallback
}

async function loadCredentials(): Promise<void> {
  if (!props.enabled) {
    credentials.value = []
    return
  }
  loading.value = true
  try {
    credentials.value = await passkeyAPI.list()
  } catch (error) {
    // 字符串错误码在 reason 字段（code 是数字状态码）；
    // 设置变更竞态下后端仍可能返回 PASSKEY_DISABLED，静默处理
    const reason = (error as { reason?: string }).reason
    if (reason !== 'PASSKEY_DISABLED') {
      appStore.showError(t('profile.passkey.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

async function addPasskey(): Promise<void> {
  if (newPassword.value.length === 0) return
  busy.value = true
  try {
    await passkeyAPI.register(newName.value.trim(), newPassword.value)
    appStore.showSuccess(t('profile.passkey.added'))
    cancelAdd()
    await loadCredentials()
  } catch (error) {
    if (!(error instanceof DOMException && error.name === 'NotAllowedError')) {
      appStore.showError(extractErrorMessage(error, t('profile.passkey.addFailed')))
    }
  } finally {
    busy.value = false
  }
}

function cancelAdd(): void {
  showAddForm.value = false
  newName.value = ''
  newPassword.value = ''
}

async function renamePasskey(credential: PasskeyCredentialSummary): Promise<void> {
  const name = window.prompt(t('profile.passkey.renamePrompt'), credential.name)?.trim()
  if (!name || name === credential.name) return
  busy.value = true
  try {
    await passkeyAPI.rename(credential.id, name)
    credential.name = name
    appStore.showSuccess(t('profile.passkey.renamed'))
  } catch {
    appStore.showError(t('profile.passkey.renameFailed'))
  } finally {
    busy.value = false
  }
}

function deletePasskey(credential: PasskeyCredentialSummary): void {
  deleteTarget.value = credential
  deletePassword.value = ''
}

function closeDeleteDialog(): void {
  deleteTarget.value = null
  deletePassword.value = ''
}

async function confirmDelete(): Promise<void> {
  const credential = deleteTarget.value
  if (!credential || deletePassword.value.length === 0) return
  busy.value = true
  try {
    await passkeyAPI.remove(credential.id, deletePassword.value)
    credentials.value = credentials.value.filter((item) => item.id !== credential.id)
    appStore.showSuccess(t('profile.passkey.deleted'))
    closeDeleteDialog()
  } catch (error) {
    // 密码错误等失败保持对话框打开，允许重试
    appStore.showError(extractErrorMessage(error, t('profile.passkey.deleteFailed')))
  } finally {
    busy.value = false
  }
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  }).format(new Date(value))
}

watch(
  () => props.enabled,
  () => {
    void loadCredentials()
  },
  { immediate: true }
)
</script>

<style scoped>
.pk-panel {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin: 0.875rem 0 0 2.875rem;
  padding: 0.875rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-muted);
}

.pk-list {
  margin: 0.75rem 0 0 2.875rem;
}

.pk-item {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.75rem;
  padding: 0.5rem 0;
}

.pk-item + .pk-item {
  border-top: 1px solid var(--operator-border-subtle);
}

.pk-danger:hover:not(:disabled) {
  color: var(--operator-destructive-fill);
}

@media (max-width: 480px) {
  .pk-panel,
  .pk-list {
    margin-left: 0;
  }
}
</style>
