<template>
  <section class="card update-card" aria-labelledby="update-settings-title">
    <div class="update-head">
      <span class="update-head-icon" aria-hidden="true">
        <Icon name="refresh" size="sm" />
      </span>
      <h2 id="update-settings-title" class="update-title">
        {{ t('admin.settings.updates.title') }}
      </h2>
      <HelpTooltip interactive width-class="w-72" :aria-label="t('admin.settingsUi.about')">{{ t('admin.settings.updates.description') }}</HelpTooltip>
      <div class="update-tools">
        <Transition name="fade" mode="out-in">
          <span v-if="status" :key="status.state" class="badge" :class="stateBadgeClass" data-testid="update-state-badge">{{ stateLabel }}</span>
        </Transition>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="load(true)">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('admin.settings.updates.check') }}
        </button>
      </div>
    </div>

    <div class="update-body">
      <p v-if="loading && !status" class="text-sm text-gray-500 dark:text-gray-400" role="status">
        {{ t('admin.settings.updates.loading') }}
      </p>
      <p v-if="recoveryPending" class="update-note update-note-warning whitespace-pre-wrap" role="status">
        {{ t('admin.settings.updates.recoveryHostFollowup') }}
      </p>
      <p v-else-if="error" class="update-note update-note-danger" role="alert">
        {{ error }}
      </p>
      <template v-if="status">
        <div class="update-versions">
          <div class="update-version">
            <span class="update-version-label">{{ t('admin.settings.updates.runningGateway') }}</span>
            <span class="update-version-value">{{ status.current_version }}</span>
          </div>
          <Icon v-if="hasNewerRelease" name="arrowRight" size="sm" class="update-version-arrow" />
          <div class="update-version">
            <span class="update-version-label">{{ t('admin.settings.updates.latestRework') }}</span>
            <span class="update-version-value" :class="{ 'update-version-new': hasNewerRelease }">{{ status.latest_rework_version || status.current_version }}</span>
          </div>
          <div class="update-meta">
            <span class="update-chip">{{ t('admin.settings.updates.channel') }} · {{ status.update_channel }}</span>
            <span class="update-chip" :class="{ 'update-chip-bad': updaterUnhealthy }">{{ t('admin.settings.updates.updater', { state: updaterLabel }) }}</span>
            <span class="update-checked">{{ t('admin.settings.updates.checkedAt') }} {{ formatDate(status.checked_at) }}</span>
          </div>
        </div>

        <p v-if="status.warning || status.updater.last_error" class="update-note update-note-warning">
          {{ status.warning || status.updater.last_error }}
        </p>

        <div class="update-actions">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="!canPrepare || operating" @click="prepare">
            {{ t('admin.settings.updates.prepare') }}
          </button>
          <button type="button" class="btn btn-primary btn-sm" :disabled="!canInstall || operating" @click="openConfirmation('install')">
            {{ t('admin.settings.updates.install') }}
          </button>
          <button type="button" class="btn btn-ghost btn-sm" :disabled="!canRollback || operating" @click="openConfirmation('rollback')">
            {{ t('admin.settings.updates.rollback') }}
          </button>
          <button v-if="recoverySupported" type="button" class="btn btn-ghost btn-sm update-danger-action" :disabled="!canPrepareRecovery || operating" @click="openConfirmation('prepareRecovery')">
            {{ t('admin.settings.updates.prepareRecovery') }}
          </button>
          <button v-if="recoverySupported" type="button" class="btn btn-danger btn-sm" :disabled="!canRecover || operating" @click="openConfirmation('recover')">
            {{ t('admin.settings.updates.recover') }}
          </button>
        </div>
        <p v-if="status.recovery?.confirmation" class="update-note update-note-warning break-words" role="status">
          {{ t('admin.settings.updates.recoveryPrepared', { sha: status.recovery.rescue_sha256 }) }}
        </p>

        <details class="update-disclosure">
          <summary>
            <Icon name="chevronDown" size="sm" class="update-disclosure-chevron" />
            {{ t('admin.settingsUi.details') }}
          </summary>
          <dl class="update-details">
            <div><dt>{{ t('admin.settings.updates.runningGateway') }}</dt><dd>{{ status.current_version }}</dd></div>
            <div><dt>{{ t('admin.settings.updates.runningRevision') }}</dt><dd class="update-mono">{{ status.current_git_commit || t('common.unknown') }}</dd></div>
            <div><dt>{{ t('admin.settings.updates.embeddedUpstream') }}</dt><dd>{{ status.upstream_baseline }}</dd></div>
            <div><dt>{{ t('admin.settings.updates.embeddedUpstreamRevision') }}</dt><dd class="update-mono">{{ status.upstream_baseline_sha || t('common.unknown') }}</dd></div>
            <div><dt>{{ t('admin.settings.updates.latestRework') }}</dt><dd>{{ status.latest_rework_version || status.current_version }}</dd></div>
            <div><dt>{{ t('admin.settings.updates.latestUpstream') }}</dt><dd>{{ status.latest_upstream }}</dd></div>
            <div><dt>{{ t('admin.settings.updates.upstreamSyncStatus') }}</dt><dd>{{ upstreamSyncLabel }}</dd></div>
            <div><dt>{{ t('admin.settings.updates.channel') }}</dt><dd>{{ status.update_channel }}</dd></div>
            <div><dt>{{ t('admin.settings.updates.checkedAt') }}</dt><dd>{{ formatDate(status.checked_at) }}</dd></div>
            <div v-if="status.latest_compatible_rework"><dt>{{ t('admin.settings.updates.availableVersion') }}</dt><dd>{{ status.latest_compatible_rework }}</dd></div>
            <div v-if="status.updater.prepared_version"><dt>{{ t('admin.settings.updates.preparedVersion') }}</dt><dd>{{ status.updater.prepared_version }}</dd></div>
          </dl>
        </details>

        <details v-if="status.release_notes && notes.length" class="update-disclosure">
          <summary>
            <Icon name="chevronDown" size="sm" class="update-disclosure-chevron" />
            {{ t('admin.settings.updates.notes') }}
          </summary>
          <dl class="update-notes">
            <div v-for="note in notes" :key="note.key">
              <dt>{{ note.label }}</dt>
              <dd>{{ note.text }}</dd>
            </div>
          </dl>
        </details>
      </template>
    </div>

    <ConfirmDialog
      :show="confirmation !== null"
      :title="confirmation ? t(`admin.settings.updates.${confirmation}`) : ''"
      :message="confirmationMessage"
      :confirm-text="t('common.confirm')"
      :danger="confirmation !== 'install'"
      @confirm="confirmOperation"
      @cancel="closeConfirmation"
    >
      <label class="block text-sm font-medium text-gray-700 [overflow-wrap:anywhere] dark:text-gray-300" for="update-confirmation">
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
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { checkUpdates, installUpdate, prepareUpdate, prepareRecovery, recoverUpdate, rollbackUpdate, type ReleaseNotes, type UpdateStatus } from '@/api/admin/system'
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
const recoveryPending = ref(false)
const confirmation = ref<'install' | 'rollback' | 'recover' | 'prepareRecovery' | null>(null)
const confirmationText = ref('')
let mounted = true

const stateLabel = computed(() => status.value ? t(`admin.settings.updates.states.${status.value.state}`) : '')
const updaterLabel = computed(() => status.value ? t(`admin.settings.updates.updaterStates.${status.value.updater.state}`) : '')
const stateBadgeClass = computed(() => {
  switch (status.value?.state) {
    case 'up_to_date': return 'badge-success'
    case 'update_ready': return 'badge-primary'
    case 'update_blocked':
    case 'update_failed': return 'badge-danger'
    default: return 'badge-warning'
  }
})
const hasNewerRelease = computed(() => !!status.value?.latest_rework_version && status.value.latest_rework_version !== status.value.current_version)
const updaterUnhealthy = computed(() => !!status.value && (!status.value.updater.healthy || ['failed', 'critical', 'unavailable'].includes(status.value.updater.state)))
const upstreamSyncLabel = computed(() => {
  if (!status.value?.latest_upstream || !status.value.upstream_baseline) return t('common.unknown')
  return t(status.value.latest_upstream === status.value.upstream_baseline
    ? 'admin.settings.updates.upstreamSyncCurrent'
    : 'admin.settings.updates.upstreamSyncAvailable')
})
const targetVersion = computed(() => status.value?.latest_compatible_rework || '')
const rollbackVersion = computed(() => status.value?.updater.rollback_version || '')
const canPrepare = computed(() => !!status.value?.installable && !!targetVersion.value && status.value.updater.healthy && !status.value.updater.busy && !recoveryPending.value && !status.value.recovery?.confirmation)
const canInstall = computed(() => canPrepare.value && status.value?.updater.prepared_version === targetVersion.value)
const recoverySupported = computed(() => supportsRecovery(status.value?.updater.updater_version))
const rollbackSchemaVerified = computed(() => !!status.value?.recovery && status.value.recovery.source_version === rollbackVersion.value && Number.isInteger(status.value.recovery.current_schema) && Number.isInteger(status.value.recovery.source_schema) && status.value.recovery.current_schema === status.value.recovery.source_schema)
const canRollback = computed(() => !!rollbackVersion.value && status.value?.updater.healthy && !status.value.updater.busy && status.value.updater.state !== 'critical' && rollbackSchemaVerified.value && !recoveryPending.value && !status.value.recovery?.confirmation)
const canPrepareRecovery = computed(() => !!rollbackVersion.value && recoverySupported.value && status.value?.updater.state !== 'unavailable' && !status.value?.updater.busy && !recoveryPending.value && !status.value?.recovery?.confirmation)
const canRecover = computed(() => recoverySupported.value && !!status.value?.recovery?.confirmation && !status.value.updater.busy && !recoveryPending.value)
const expectedConfirmation = computed(() => confirmation.value === 'install' ? `INSTALL ${targetVersion.value}` : confirmation.value === 'rollback' ? `ROLLBACK ${rollbackVersion.value}` : confirmation.value === 'prepareRecovery' ? `PREPARE RECOVERY ${rollbackVersion.value}` : confirmation.value === 'recover' ? status.value?.recovery?.confirmation || '' : '')
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

function supportsRecovery(version: string | undefined): boolean {
  const parts = version?.match(/^(\d+)\.(\d+)\.(\d+)$/)
  if (!parts) return false
  const [major, minor, patch] = parts.slice(1).map(Number)
  return major > 1 || (major === 1 && (minor > 1 || (minor === 1 && patch >= 5)))
}

async function load(force = false): Promise<boolean> {
  loading.value = true
  error.value = ''
  try {
    status.value = await checkUpdates(force)
    if (status.value.recovery?.confirmation) recoveryPending.value = false
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

function openConfirmation(action: 'install' | 'rollback' | 'recover' | 'prepareRecovery') {
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
  const version = action === 'install' ? targetVersion.value : action === 'recover' ? status.value!.recovery!.source_version : rollbackVersion.value
  const consent = expectedConfirmation.value
  closeConfirmation()
  await runOperation(() => action === 'install'
    ? installUpdate(version, `INSTALL ${version}`)
    : action === 'rollback'
      ? rollbackUpdate(version, `ROLLBACK ${version}`)
      : action === 'prepareRecovery'
        ? prepareRecovery(version, consent)
        : recoverUpdate(version, consent), action === 'prepareRecovery')
}

async function runOperation(operation: () => Promise<unknown>, preparingRecovery = false) {
  operating.value = true
  try {
    await stepUp.run(operation)
    if (preparingRecovery) recoveryPending.value = true
    appStore.showSuccess(t('admin.settings.updates.operationAccepted'))
    let loaded = await load(true)
    let failures = loaded ? 0 : 1
    while (mounted && (!loaded || status.value?.updater.busy) && failures < 3) {
      await new Promise(resolve => setTimeout(resolve, 1000))
      if (!mounted) break
      loaded = await load()
      failures = loaded ? 0 : failures + 1
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

<style scoped>
.update-head {
  display: flex;
  min-height: 3rem;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem 0.5rem;
  /* Beats the settings form's generic card padding rules. */
  padding: 0.875rem 1.25rem 0 !important;
}

.update-head-icon {
  display: grid;
  width: 1.75rem;
  height: 1.75rem;
  flex: none;
  place-items: center;
  border-radius: var(--operator-radius);
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
}

.update-title {
  margin: 0;
  color: var(--operator-foreground);
  font-size: 0.9375rem;
  font-weight: 600;
  letter-spacing: -0.005em;
}

.update-tools {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.625rem;
  margin-left: auto;
}

.update-body {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  padding: 1rem 1.25rem 1.25rem !important;
}

.update-versions {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 0.75rem 1.25rem;
}

.update-version {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.25rem;
}

.update-version-label {
  color: var(--operator-subtle-foreground);
  font-size: 0.6875rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.update-version-value {
  color: var(--operator-foreground);
  font-size: 1.375rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.02em;
  line-height: 1.15;
  overflow-wrap: anywhere;
}

.update-version-new {
  color: var(--operator-accent-text);
}

.update-version-arrow {
  margin-bottom: 0.3rem;
  color: var(--operator-subtle-foreground);
}

.update-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem 0.5rem;
  margin-left: auto;
}

.update-chip {
  display: inline-flex;
  height: 1.5rem;
  align-items: center;
  padding: 0 0.5rem;
  border: 1px solid var(--operator-border);
  border-radius: 999px;
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
  white-space: nowrap;
}

.update-chip-bad {
  border-color: transparent;
  background: var(--operator-destructive-soft);
  color: var(--operator-destructive);
}

.update-checked {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.update-note {
  margin: 0;
  padding: 0.5rem 0.75rem;
  border-radius: var(--operator-radius);
  font-size: 0.8125rem;
  overflow-wrap: anywhere;
}

.update-note-warning {
  background: var(--operator-warning-soft);
  color: var(--operator-warning);
}

.update-note-danger {
  background: var(--operator-destructive-soft);
  color: var(--operator-destructive);
}

.update-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.update-danger-action:not(:disabled) {
  color: var(--operator-destructive);
}

.update-disclosure {
  border-top: 1px solid var(--operator-border-subtle);
  padding-top: 0.75rem;
}

.update-disclosure + .update-disclosure {
  margin-top: -0.25rem;
}

.update-disclosure summary {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  border-radius: var(--operator-radius-sm);
  color: var(--operator-muted-foreground);
  cursor: pointer;
  font-size: 0.8125rem;
  font-weight: 500;
  list-style: none;
}

.update-disclosure summary::-webkit-details-marker {
  display: none;
}

.update-disclosure summary:hover {
  color: var(--operator-foreground);
}

.update-disclosure summary:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: 2px;
}

.update-disclosure-chevron {
  transform: rotate(-90deg);
  transition: transform var(--operator-duration) var(--operator-ease-out);
}

.update-disclosure[open] .update-disclosure-chevron {
  transform: none;
}

.update-disclosure[open] > :not(summary) {
  animation: update-reveal 220ms var(--operator-ease-out);
}

.update-details {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(14rem, 1fr));
  gap: 0.625rem 1.5rem;
  margin: 0.75rem 0 0;
}

.update-details dt,
.update-notes dt {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.update-details dd {
  margin: 0.125rem 0 0;
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  font-weight: 500;
  overflow-wrap: anywhere;
}

.update-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.75rem !important;
}

.update-notes {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin: 0.75rem 0 0;
}

.update-notes dd {
  margin: 0.25rem 0 0;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

@media (max-width: 479px) {
  .update-tools {
    width: 100%;
    justify-content: space-between;
    margin-left: 0;
  }

  .update-version-arrow {
    display: none;
  }

  .update-meta {
    margin-left: 0;
  }
}

@keyframes update-reveal {
  from {
    opacity: 0;
    transform: translateY(-4px);
  }
}

@media (prefers-reduced-motion: reduce) {
  .update-disclosure-chevron {
    transition: none;
  }

  .update-disclosure[open] > :not(summary) {
    animation: none;
  }
}
</style>
