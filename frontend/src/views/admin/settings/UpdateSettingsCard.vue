<template>
  <section class="card" aria-labelledby="update-settings-title">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 id="update-settings-title" class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.settings.updates.title') }}
          </h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.updates.description') }}
          </p>
        </div>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="load(true)">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('admin.settings.updates.check') }}
        </button>
      </div>
    </div>

    <div class="space-y-4 p-6">
      <p v-if="loading && !status" class="text-sm text-gray-500 dark:text-gray-400" role="status">
        {{ t('admin.settings.updates.loading') }}
      </p>
      <p v-else-if="error" class="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/30 dark:text-red-300" role="alert">
        {{ error }}
      </p>
      <template v-else-if="status">
        <div class="flex flex-wrap items-center justify-between gap-3 rounded-md border border-gray-200 px-4 py-3 dark:border-dark-600">
          <div>
            <p class="text-sm font-medium text-gray-900 dark:text-white">{{ stateLabel }}</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.updates.currentVersion', { version: status.current_version }) }}
            </p>
          </div>
          <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.updates.updater', { state: updaterLabel }) }}</span>
        </div>

        <p v-if="status.warning || status.updater.last_error" class="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-200">
          {{ status.warning || status.updater.last_error }}
        </p>

        <dl class="grid grid-cols-1 gap-x-6 gap-y-2 text-sm sm:grid-cols-2">
          <div><dt class="text-gray-500 dark:text-gray-400">{{ t('admin.settings.updates.upstreamBaseline') }}</dt><dd class="font-medium text-gray-900 dark:text-white">{{ status.upstream_baseline }}</dd></div>
          <div><dt class="text-gray-500 dark:text-gray-400">{{ t('admin.settings.updates.latestUpstream') }}</dt><dd class="font-medium text-gray-900 dark:text-white">{{ status.latest_upstream }}</dd></div>
          <div><dt class="text-gray-500 dark:text-gray-400">{{ t('admin.settings.updates.channel') }}</dt><dd class="font-medium text-gray-900 dark:text-white">{{ status.update_channel }}</dd></div>
          <div><dt class="text-gray-500 dark:text-gray-400">{{ t('admin.settings.updates.checkedAt') }}</dt><dd class="font-medium text-gray-900 dark:text-white">{{ formatDate(status.checked_at) }}</dd></div>
          <div v-if="status.latest_compatible_rework"><dt class="text-gray-500 dark:text-gray-400">{{ t('admin.settings.updates.availableVersion') }}</dt><dd class="font-medium text-gray-900 dark:text-white">{{ status.latest_compatible_rework }}</dd></div>
          <div v-if="status.updater.prepared_version"><dt class="text-gray-500 dark:text-gray-400">{{ t('admin.settings.updates.preparedVersion') }}</dt><dd class="font-medium text-gray-900 dark:text-white">{{ status.updater.prepared_version }}</dd></div>
        </dl>

        <div v-if="status.release_notes" class="space-y-2 border-t border-gray-100 pt-4 dark:border-dark-700">
          <h3 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.settings.updates.notes') }}</h3>
          <dl class="space-y-3">
            <div v-for="note in notes" :key="note.key">
              <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ note.label }}</dt>
              <dd class="mt-1 whitespace-pre-wrap break-words text-sm text-gray-600 dark:text-gray-300">{{ note.text }}</dd>
            </div>
          </dl>
        </div>

        <div class="flex flex-wrap gap-2 border-t border-gray-100 pt-4 dark:border-dark-700">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="!canPrepare || operating" @click="prepare">
            {{ t('admin.settings.updates.prepare') }}
          </button>
          <button type="button" class="btn btn-primary btn-sm" :disabled="!canInstall || operating" @click="openConfirmation('install')">
            {{ t('admin.settings.updates.install') }}
          </button>
          <button type="button" class="btn btn-danger btn-sm" :disabled="!canRollback || operating" @click="openConfirmation('rollback')">
            {{ t('admin.settings.updates.rollback') }}
          </button>
        </div>
      </template>
    </div>

    <ConfirmDialog
      :show="confirmation !== null"
      :title="t(`admin.settings.updates.${confirmation}`)"
      :message="confirmationMessage"
      :confirm-text="t('common.confirm')"
      :danger="confirmation === 'rollback'"
      @confirm="confirmOperation"
      @cancel="closeConfirmation"
    >
      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300" for="update-confirmation">
        {{ t('admin.settings.updates.confirmationLabel', { value: expectedConfirmation }) }}
      </label>
      <input id="update-confirmation" v-model="confirmationText" type="text" class="input mt-2 font-mono text-sm" autocomplete="off" />
    </ConfirmDialog>
    <TotpStepUpDialog :controller="stepUp" />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { checkUpdates, installUpdate, prepareUpdate, rollbackUpdate, type ReleaseNotes, type UpdateStatus } from '@/api/admin/system'
import { useStepUp, isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'
import { extractApiErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()
const stepUp = useStepUp()
const status = ref<UpdateStatus | null>(null)
const loading = ref(false)
const operating = ref(false)
const error = ref('')
const confirmation = ref<'install' | 'rollback' | null>(null)
const confirmationText = ref('')
let mounted = true

const stateLabel = computed(() => status.value ? t(`admin.settings.updates.states.${status.value.state}`) : '')
const updaterLabel = computed(() => status.value ? t(`admin.settings.updates.updaterStates.${status.value.updater.state}`) : '')
const targetVersion = computed(() => status.value?.latest_compatible_rework || '')
const rollbackVersion = computed(() => status.value?.updater.rollback_version || '')
const canPrepare = computed(() => !!status.value?.installable && !!targetVersion.value && status.value.updater.healthy && !status.value.updater.busy)
const canInstall = computed(() => canPrepare.value && status.value?.updater.prepared_version === targetVersion.value)
const canRollback = computed(() => !!rollbackVersion.value && status.value?.updater.healthy && !status.value.updater.busy)
const expectedConfirmation = computed(() => confirmation.value === 'install' ? `INSTALL ${targetVersion.value}` : confirmation.value === 'rollback' ? `ROLLBACK ${rollbackVersion.value}` : '')
const confirmationMessage = computed(() => confirmation.value ? t(`admin.settings.updates.${confirmation.value}Confirm`) : '')
const noteKeys: (keyof ReleaseNotes)[] = ['upstream', 'rework', 'compatibility', 'migrations', 'rollback']
const notes = computed(() => noteKeys.flatMap((key) => {
  const text = status.value?.release_notes?.[key]
  return text ? [{ key, label: t(`admin.settings.updates.noteLabels.${key}`), text }] : []
}))

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

async function load(force = false): Promise<boolean> {
  loading.value = true
  error.value = ''
  try {
    status.value = await checkUpdates(force)
    return true
  } catch (err) {
    error.value = extractApiErrorMessage(err, t('admin.settings.updates.loadFailed'))
    return false
  } finally {
    loading.value = false
  }
}

async function prepare() {
  if (!targetVersion.value) return
  await runOperation(() => prepareUpdate(targetVersion.value))
}

function openConfirmation(action: 'install' | 'rollback') {
  confirmation.value = action
  confirmationText.value = ''
}

function closeConfirmation() {
  confirmation.value = null
  confirmationText.value = ''
}

async function confirmOperation() {
  if (!confirmation.value || confirmationText.value !== expectedConfirmation.value) {
    appStore.showError(t('admin.settings.updates.confirmationMismatch'))
    return
  }
  const action = confirmation.value
  const version = action === 'install' ? targetVersion.value : rollbackVersion.value
  closeConfirmation()
  await runOperation(() => action === 'install' ? installUpdate(version, `INSTALL ${version}`) : rollbackUpdate(version, `ROLLBACK ${version}`))
}

async function runOperation(operation: () => Promise<unknown>) {
  operating.value = true
  try {
    await stepUp.run(operation)
    appStore.showSuccess(t('admin.settings.updates.operationAccepted'))
    let loaded = await load(true)
    while (mounted && (!loaded || status.value?.updater.busy)) {
      await new Promise(resolve => setTimeout(resolve, 1000))
      if (!mounted) break
      loaded = await load()
    }
  } catch (err) {
    if (isStepUpCancelled(err)) return
    if (isStepUpBlocked(err)) {
      appStore.showError(stepUpBlockReason(err) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN' ? t('stepUp.adminApiKeyForbidden') : t('stepUp.notEnabled'))
      return
    }
    appStore.showError(extractApiErrorMessage(err, t('admin.settings.updates.operationFailed')))
  } finally {
    operating.value = false
  }
}

onMounted(() => load())
onUnmounted(() => { mounted = false })
</script>
