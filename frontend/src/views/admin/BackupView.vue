<template>
    <div class="ui-page ui-stagger">
      <dl class="viz-kpis backup-kpis" data-testid="backup-kpis">
        <div class="viz-card viz-kpi">
          <dt class="viz-kpi-label"><Icon name="clock" size="sm" />{{ t('admin.adminUi.backup.lastBackup') }}</dt>
          <dd class="viz-kpi-row">
            <span class="viz-kpi-value">{{ lastCompleted ? relativeAge(lastCompleted.finished_at || lastCompleted.started_at) : t('common.time.never') }}</span>
          </dd>
        </div>
        <div class="viz-card viz-kpi">
          <dt class="viz-kpi-label"><Icon name="database" size="sm" />{{ t('admin.adminUi.backup.backups') }}</dt>
          <dd class="viz-kpi-row"><span class="viz-kpi-value"><CountUp :value="backups.length" /></span></dd>
        </div>
        <div class="viz-card viz-kpi">
          <dt class="viz-kpi-label"><Icon name="layers" size="sm" />{{ t('admin.adminUi.backup.totalSize') }}</dt>
          <dd class="viz-kpi-row"><span class="viz-kpi-value"><CountUp :value="totalBytes" :format="formatSize" /></span></dd>
        </div>
        <div class="viz-card viz-kpi">
          <dt class="viz-kpi-label"><Icon name="calendar" size="sm" />{{ t('admin.adminUi.backup.schedule') }}</dt>
          <dd class="viz-kpi-row min-w-0">
            <button type="button" class="viz-kpi-value backup-schedule-value" data-testid="backup-open-schedule" @click="openSettings('schedule')">
              <span :class="savedScheduleEnabled ? 'ui-dot is-ok' : 'ui-dot'" />
              <span class="truncate" :class="savedScheduleEnabled ? 'font-mono' : ''">{{ savedScheduleEnabled ? scheduleForm.cron_expr : t('admin.adminUi.backup.off') }}</span>
              <Icon name="chevronRight" size="sm" class="backup-schedule-chevron" />
            </button>
          </dd>
        </div>
      </dl>

      <div class="ui-toolbar" role="toolbar" :aria-label="t('admin.backup.operations.title')">
        <div class="viz-seg max-w-full overflow-x-auto" role="group" :aria-label="t('admin.backup.columns.status')">
          <button
            v-for="option in statusOptions"
            :key="option.id"
            type="button"
            :aria-pressed="statusFilter === option.id"
            :data-testid="`backup-status-${option.id}`"
            @click="statusFilter = option.id"
          >
            {{ option.label }}<span class="backup-seg-count">{{ statusCounts[option.id] }}</span>
          </button>
        </div>
        <span class="ui-grow" />
        <button
          type="button"
          class="app-icon-button"
          :disabled="loadingBackups"
          :aria-label="t('common.refresh')"
          :title="t('common.refresh')"
          @click="loadBackups"
        >
          <Icon name="refresh" size="sm" :class="loadingBackups ? 'animate-spin' : ''" />
        </button>
        <ToolbarPopover :label="t('admin.adminUi.moreActions')" icon="more" icon-only menu test-id="backup-more">
          <button type="button" role="menuitem" class="dropdown-item" @click="openSettings('schedule')">
            <Icon name="calendar" size="sm" />{{ t('admin.backup.schedule.title') }}
          </button>
          <button type="button" role="menuitem" class="dropdown-item" @click="openSettings('storage')">
            <Icon name="cloud" size="sm" />{{ t('admin.backup.s3.title') }}
          </button>
          <button type="button" role="menuitem" class="dropdown-item" @click="openSettings('images')">
            <Icon name="camera" size="sm" />{{ t('admin.backup.imageStorage.title') }}
          </button>
          <button type="button" role="menuitem" class="dropdown-item" @click="showR2Guide = true">
            <Icon name="book" size="sm" />{{ t('admin.adminUi.backup.r2Guide') }}
          </button>
        </ToolbarPopover>
        <label class="backup-expire" :title="t('admin.backup.operations.expireDays')">
          <Icon name="clock" size="sm" />
          <input
            v-model.number="manualExpireDays"
            type="number"
            min="0"
            :aria-label="t('admin.backup.operations.expireDays')"
          />
          <span aria-hidden="true">d</span>
        </label>
        <button type="button" class="btn btn-primary" :disabled="creatingBackup" data-testid="backup-create" @click="createBackup">
          <Icon :name="creatingBackup ? 'sync' : 'plus'" size="sm" :class="creatingBackup ? 'animate-spin' : ''" />
          {{ creatingBackup ? t('admin.backup.operations.backing') : t('admin.backup.operations.createBackup') }}
        </button>
      </div>

      <section class="ui-card overflow-hidden">
        <div v-if="loadingBackups && backups.length === 0" class="space-y-2 p-4" aria-busy="true">
          <div v-for="index in 3" :key="index" class="skeleton h-10" />
        </div>
        <div v-else-if="visibleBackups.length === 0" class="ui-empty">
          <span class="ui-empty-icon"><Icon name="database" size="lg" /></span>
          <b>{{ backups.length ? t('admin.adminUi.noMatches') : t('admin.backup.empty') }}</b>
          <button v-if="backups.length" type="button" class="btn btn-ghost btn-sm" @click="statusFilter = 'all'">
            {{ t('admin.adminUi.clearFilters') }}
          </button>
        </div>
        <div v-else class="ui-table-wrap">
          <table class="ui-table">
            <thead>
              <tr>
                <th>{{ t('admin.backup.columns.status') }}</th>
                <th>{{ t('admin.backup.columns.fileName') }}</th>
                <th class="is-num">{{ t('admin.backup.columns.size') }}</th>
                <th class="is-num">{{ t('admin.backup.columns.parts') }}</th>
                <th>{{ t('admin.backup.columns.expiresAt') }}</th>
                <th>{{ t('admin.backup.columns.startedAt') }}</th>
                <th><span class="sr-only">{{ t('admin.backup.columns.actions') }}</span></th>
              </tr>
            </thead>
            <tbody class="ui-rows-enter">
              <tr v-for="record in visibleBackups" :key="record.id" :data-testid="`backup-row-${record.id}`">
                <td>
                  <span class="badge" :class="statusBadgeClass(record.status)" :title="record.status === 'failed' ? record.error_message || undefined : undefined">
                    {{ record.status === 'running' && record.progress
                      ? t(`admin.backup.progress.${record.progress}`)
                      : t(`admin.backup.status.${record.status}`) }}
                  </span>
                  <span v-if="restoringId === record.id || record.restore_status === 'running'" class="ui-cell-sub mt-1 flex items-center gap-1">
                    <Icon name="sync" size="xs" class="animate-spin" />{{ t('admin.adminUi.backup.restoring') }}
                  </span>
                </td>
                <td class="max-w-[22rem]">
                  <div class="ui-cell-main flex min-w-0 items-center gap-2">
                    <span class="truncate" :title="record.file_name">{{ record.file_name }}</span>
                    <span v-if="record.monthly_archive" class="badge badge-primary shrink-0">{{ t('admin.backup.archive.badge') }}</span>
                  </div>
                  <div class="ui-cell-sub truncate">
                    <span class="font-mono">{{ record.id }}</span>
                    · {{ record.triggered_by === 'scheduled' ? t('admin.backup.trigger.scheduled') : t('admin.backup.trigger.manual') }}
                    <template v-if="record.monthly_archive"> · {{ record.monthly_archive.dates.join(' / ') }}</template>
                  </div>
                </td>
                <td class="is-num">{{ formatSize(record.size_bytes) }}</td>
                <td class="is-num">{{ record.parts?.length || (record.status === 'running' ? '-' : 1) }}</td>
                <td class="whitespace-nowrap">
                  {{ record.monthly_archive
                    ? record.monthly_archive.retain_count === 0 ? t('admin.backup.archive.forever') : t('admin.backup.archive.retainLatest', { count: record.monthly_archive.retain_count })
                    : record.expires_at ? formatDate(record.expires_at) : t('admin.backup.neverExpire') }}
                </td>
                <td class="whitespace-nowrap" :title="formatDate(record.started_at)">{{ relativeAge(record.started_at) }}</td>
                <td>
                  <div class="flex items-center justify-end gap-1">
                    <button
                      v-if="record.status === 'completed'"
                      type="button"
                      class="app-icon-button"
                      :aria-label="t('admin.backup.actions.download')"
                      :title="t('admin.backup.actions.download')"
                      @click="downloadBackup(record.id)"
                    >
                      <Icon name="download" size="sm" />
                    </button>
                    <ToolbarPopover
                      v-if="record.status !== 'running'"
                      :label="t('admin.adminUi.moreActions')"
                      icon="more"
                      icon-only
                      menu
                      :test-id="`backup-more-${record.id}`"
                    >
                      <button
                        v-if="record.status === 'completed'"
                        type="button"
                        role="menuitem"
                        class="dropdown-item"
                        :disabled="restoringId === record.id"
                        @click="restoreBackup(record.id)"
                      >
                        <Icon name="swap" size="sm" />{{ restoringId === record.id ? t('common.loading') : t('admin.backup.actions.restore') }}
                      </button>
                      <button type="button" role="menuitem" class="dropdown-item text-red-600 dark:text-red-400" @click="removeBackup(record.id)">
                        <Icon name="trash" size="sm" />{{ t('common.delete') }}
                      </button>
                    </ToolbarPopover>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>

    <SideDrawer :open="settingsOpen" :title="t('admin.adminUi.backup.settings')" width="34rem" @close="settingsOpen = false">
      <template #leading>
        <span class="backup-drawer-icon" aria-hidden="true"><Icon name="cog" size="md" /></span>
      </template>
      <div class="viz-seg mb-5 w-full" role="group" :aria-label="t('admin.adminUi.backup.settings')">
        <button
          v-for="tab in settingsTabs"
          :key="tab.id"
          type="button"
          class="flex-1"
          :aria-pressed="settingsTab === tab.id"
          :data-testid="`backup-settings-${tab.id}`"
          @click="settingsTab = tab.id"
        >
          {{ tab.label }}
        </button>
      </div>

      <!-- Schedule Config -->
      <div v-show="settingsTab === 'schedule'" class="space-y-5" data-testid="backup-schedule">
        <label class="backup-switch">
          <input v-model="scheduleForm.enabled" type="checkbox" />
          <span>{{ t('admin.backup.schedule.enabled') }}</span>
        </label>
        <div>
          <label class="backup-label">
            {{ t('admin.backup.schedule.cronExpr') }}
            <HelpTooltip :content="t('admin.backup.schedule.cronHint')" />
          </label>
          <input v-model="scheduleForm.cron_expr" class="input w-full font-mono" placeholder="0 2 * * *" />
        </div>
        <section>
          <h4 class="ui-section-label flex items-center">
            {{ t('admin.backup.schedule.ordinaryRetention') }}
            <HelpTooltip :content="t('admin.backup.schedule.ordinaryHint')" />
          </h4>
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label class="backup-label">
                {{ t('admin.backup.schedule.retainDays') }}
                <HelpTooltip :content="t('admin.backup.schedule.retainDaysHint')" />
              </label>
              <input v-model.number="scheduleForm.retain_days" data-testid="backup-retain-days" type="number" min="0" step="1" class="input w-full" />
            </div>
            <div>
              <label class="backup-label">
                {{ t('admin.backup.schedule.retainCount') }}
                <HelpTooltip :content="t('admin.backup.schedule.retainCountHint')" />
              </label>
              <input v-model.number="scheduleForm.retain_count" data-testid="backup-retain-count" type="number" min="0" step="1" class="input w-full" />
            </div>
          </div>
        </section>
        <BackupArchiveSettings v-model="archiveForm" />
        <p v-if="scheduleValidationError" class="text-sm text-red-600 dark:text-red-400" role="alert">{{ scheduleValidationError }}</p>
        <div class="backup-preview" aria-live="polite">
          <p class="ui-section-label mb-1">{{ t('admin.backup.schedule.preview') }}</p>
          <p>{{ retentionPreview }}</p>
        </div>
      </div>

      <!-- S3 Storage Config -->
      <div v-show="settingsTab === 'storage'" class="space-y-4" data-testid="backup-storage">
        <div class="flex items-center justify-between gap-3">
          <h3 class="ui-section-label mb-0">{{ t('admin.backup.s3.title') }}</h3>
          <button type="button" class="btn btn-ghost btn-sm" @click="showR2Guide = true">
            <Icon name="book" size="sm" />{{ t('admin.adminUi.backup.r2Guide') }}
          </button>
        </div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div class="sm:col-span-2">
            <label class="backup-label">{{ t('admin.backup.s3.endpoint') }}</label>
            <input v-model="s3Form.endpoint" class="input w-full" placeholder="https://<account_id>.r2.cloudflarestorage.com" />
          </div>
          <div>
            <label class="backup-label">{{ t('admin.backup.s3.region') }}</label>
            <input v-model="s3Form.region" class="input w-full" placeholder="auto" />
          </div>
          <div>
            <label class="backup-label">{{ t('admin.backup.s3.bucket') }}</label>
            <input v-model="s3Form.bucket" class="input w-full" />
          </div>
          <div>
            <label class="backup-label">{{ t('admin.backup.s3.prefix') }}</label>
            <input v-model="s3Form.prefix" class="input w-full" placeholder="backups/" />
          </div>
          <div>
            <label class="backup-label">{{ t('admin.backup.s3.accessKeyId') }}</label>
            <input v-model="s3Form.access_key_id" class="input w-full" />
          </div>
          <div class="sm:col-span-2">
            <label class="backup-label">{{ t('admin.backup.s3.secretAccessKey') }}</label>
            <input v-model="s3Form.secret_access_key" type="password" class="input w-full" :placeholder="s3SecretConfigured ? t('admin.backup.s3.secretConfigured') : ''" />
          </div>
          <label class="backup-switch sm:col-span-2">
            <input v-model="s3Form.force_path_style" type="checkbox" />
            <span>{{ t('admin.backup.s3.forcePathStyle') }}</span>
          </label>
        </div>
      </div>

      <!-- Async image object storage -->
      <div v-show="settingsTab === 'images'" class="space-y-4" data-testid="backup-images">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h3 class="ui-section-label mb-0 flex items-center">
            {{ t('admin.backup.imageStorage.title') }}
            <HelpTooltip :content="t('admin.backup.imageStorage.description')" />
          </h3>
          <label class="backup-switch">
            <input v-model="imageStorageForm.enabled" type="checkbox" />
            <span>{{ t('admin.backup.imageStorage.enabled') }}</span>
          </label>
        </div>
        <label class="backup-switch">
          <input v-model="imageStorageForm.reuse_backup_s3" type="checkbox" />
          <span>{{ t('admin.backup.imageStorage.reuseBackupS3') }}</span>
        </label>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="backup-label">{{ t('admin.backup.imageStorage.bucket') }}</label>
            <input v-model="imageStorageForm.bucket" class="input w-full" :placeholder="imageStorageForm.reuse_backup_s3 ? t('admin.backup.imageStorage.bucketInherited') : ''" />
          </div>
          <div>
            <label class="backup-label">{{ t('admin.backup.imageStorage.prefix') }}</label>
            <input v-model="imageStorageForm.prefix" class="input w-full" placeholder="images/" />
          </div>

          <template v-if="!imageStorageForm.reuse_backup_s3">
            <div class="sm:col-span-2">
              <label class="backup-label">{{ t('admin.backup.s3.endpoint') }}</label>
              <input v-model="imageStorageForm.endpoint" class="input w-full" placeholder="https://<account_id>.r2.cloudflarestorage.com" />
            </div>
            <div>
              <label class="backup-label">{{ t('admin.backup.s3.region') }}</label>
              <input v-model="imageStorageForm.region" class="input w-full" placeholder="auto" />
            </div>
            <div>
              <label class="backup-label">{{ t('admin.backup.s3.accessKeyId') }}</label>
              <input v-model="imageStorageForm.access_key_id" class="input w-full" />
            </div>
            <div class="sm:col-span-2">
              <label class="backup-label">{{ t('admin.backup.s3.secretAccessKey') }}</label>
              <input v-model="imageStorageForm.secret_access_key" type="password" class="input w-full" :placeholder="imageStorageSecretConfigured ? t('admin.backup.s3.secretConfigured') : ''" />
            </div>
            <label class="backup-switch sm:col-span-2">
              <input v-model="imageStorageForm.force_path_style" type="checkbox" />
              <span>{{ t('admin.backup.s3.forcePathStyle') }}</span>
            </label>
          </template>

          <div>
            <label class="backup-label">{{ t('admin.backup.imageStorage.publicBaseUrl') }}</label>
            <input v-model="imageStorageForm.public_base_url" class="input w-full" :placeholder="t('admin.backup.imageStorage.publicBaseUrlPlaceholder')" />
          </div>
          <div>
            <label class="backup-label">{{ t('admin.backup.imageStorage.presignExpiryHours') }}</label>
            <input v-model.number="imageStorageForm.presign_expiry_hours" type="number" min="1" class="input w-full" />
          </div>
        </div>
      </div>

      <template #footer>
        <template v-if="settingsTab === 'schedule'">
          <button type="button" class="btn btn-primary" data-testid="backup-schedule-save" :disabled="savingSchedule || !!scheduleValidationError" @click="saveSchedule">
            {{ savingSchedule ? t('common.loading') : t('common.save') }}
          </button>
        </template>
        <template v-else-if="settingsTab === 'storage'">
          <button type="button" class="btn btn-primary" :disabled="savingS3" @click="saveS3Config">
            {{ savingS3 ? t('common.loading') : t('common.save') }}
          </button>
          <button type="button" class="btn btn-secondary" :disabled="testingS3" @click="testS3">
            {{ testingS3 ? t('common.loading') : t('admin.backup.s3.testConnection') }}
          </button>
        </template>
        <template v-else>
          <button type="button" class="btn btn-primary" :disabled="savingImageStorage" @click="saveImageStorageConfig">
            {{ savingImageStorage ? t('common.loading') : t('common.save') }}
          </button>
          <button type="button" class="btn btn-secondary" :disabled="testingImageStorage" @click="testImageStorage">
            {{ testingImageStorage ? t('common.loading') : t('admin.backup.s3.testConnection') }}
          </button>
        </template>
      </template>
    </SideDrawer>

    <!-- Cloudflare R2 Setup Guide Modal -->
    <teleport to="body">
      <transition name="modal">
        <div v-if="showR2Guide" class="fixed inset-0 z-50 flex items-center justify-center p-4" @mousedown.self="showR2Guide = false">
          <div class="fixed inset-0 bg-black/50" @click="showR2Guide = false"></div>
          <div class="relative max-h-[85vh] w-full max-w-2xl overflow-y-auto rounded-xl bg-white p-6 shadow-2xl dark:bg-dark-800">
            <button type="button" class="absolute right-4 top-4 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200" @click="showR2Guide = false">
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>

            <h2 class="mb-4 text-lg font-bold text-gray-900 dark:text-white">{{ t('admin.backup.r2Guide.title') }}</h2>
            <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.backup.r2Guide.intro') }}</p>

            <!-- Step 1 -->
            <div class="mb-5">
              <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">1</span>
                {{ t('admin.backup.r2Guide.step1.title') }}
              </h3>
              <ol class="ml-8 list-decimal space-y-1 text-sm text-gray-600 dark:text-gray-300">
                <li>{{ t('admin.backup.r2Guide.step1.line1') }}</li>
                <li>{{ t('admin.backup.r2Guide.step1.line2') }}</li>
                <li>{{ t('admin.backup.r2Guide.step1.line3') }}</li>
              </ol>
            </div>

            <!-- Step 2 -->
            <div class="mb-5">
              <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">2</span>
                {{ t('admin.backup.r2Guide.step2.title') }}
              </h3>
              <ol class="ml-8 list-decimal space-y-1 text-sm text-gray-600 dark:text-gray-300">
                <li>{{ t('admin.backup.r2Guide.step2.line1') }}</li>
                <li>{{ t('admin.backup.r2Guide.step2.line2') }}</li>
                <li>{{ t('admin.backup.r2Guide.step2.line3') }}</li>
                <li>{{ t('admin.backup.r2Guide.step2.line4') }}</li>
              </ol>
              <div class="mt-2 rounded-lg bg-amber-50 p-3 text-xs text-amber-700 dark:bg-amber-900/20 dark:text-amber-300">
                {{ t('admin.backup.r2Guide.step2.warning') }}
              </div>
            </div>

            <!-- Step 3 -->
            <div class="mb-5">
              <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">3</span>
                {{ t('admin.backup.r2Guide.step3.title') }}
              </h3>
              <p class="ml-8 text-sm text-gray-600 dark:text-gray-300">{{ t('admin.backup.r2Guide.step3.desc') }}</p>
              <code class="ml-8 mt-1 block rounded bg-gray-100 px-3 py-2 text-xs text-gray-800 dark:bg-dark-700 dark:text-gray-200">https://&lt;{{ t('admin.backup.r2Guide.step3.accountId') }}&gt;.r2.cloudflarestorage.com</code>
            </div>

            <!-- Step 4: Fill form -->
            <div class="mb-5">
              <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">4</span>
                {{ t('admin.backup.r2Guide.step4.title') }}
              </h3>
              <div class="ml-8 overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
                <table class="w-full text-sm">
                  <tbody>
                    <tr v-for="(row, i) in r2ConfigRows" :key="i" class="border-b border-gray-100 dark:border-dark-700 last:border-0">
                      <td class="whitespace-nowrap bg-gray-50 px-3 py-2 font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-300">{{ row.field }}</td>
                      <td class="px-3 py-2 text-gray-600 dark:text-gray-400"><code class="text-xs">{{ row.value }}</code></td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Free tier note -->
            <div class="rounded-lg bg-green-50 p-3 text-xs text-green-700 dark:bg-green-900/20 dark:text-green-300">
              {{ t('admin.backup.r2Guide.freeTier') }}
            </div>

            <div class="mt-4 text-right">
              <button type="button" class="btn btn-primary btn-sm" @click="showR2Guide = false">{{ t('common.close') }}</button>
            </div>
          </div>
        </div>
      </transition>
    </teleport>
    <!-- 分卷下载链接 -->
    <teleport to="body">
      <transition name="modal">
        <div
          v-if="downloadPartsModalOpen"
          class="fixed inset-0 z-50 flex items-center justify-center p-4"
          @mousedown.self="closeDownloadParts"
        >
          <div class="fixed inset-0 bg-black/50" @click="closeDownloadParts"></div>
          <div class="relative max-h-[85vh] w-full max-w-lg overflow-y-auto rounded-xl bg-white p-6 shadow-2xl dark:bg-dark-800">
            <button
              type="button"
              class="absolute right-4 top-4 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
              :aria-label="t('common.close')"
              @click="closeDownloadParts"
            >
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
            <h2 class="mb-1 text-lg font-bold text-gray-900 dark:text-white">{{ t('admin.backup.actions.downloadParts') }}</h2>
            <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.backup.actions.downloadPartsHint') }}</p>
            <div class="space-y-2">
              <div
                v-for="part in downloadParts"
                :key="part.index"
                class="flex items-center justify-between gap-3 rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-600"
              >
                <span class="text-sm text-gray-700 dark:text-gray-300">
                  {{ t('admin.backup.actions.partLabel', { index: part.index }) }}
                  <span class="ml-2 text-xs text-gray-500 dark:text-gray-400">{{ formatSize(part.size_bytes) }}</span>
                </span>
                <a :href="part.url" class="btn btn-secondary btn-xs" rel="noopener">
                  {{ t('admin.backup.actions.download') }}
                </a>
              </div>
            </div>
            <div class="mt-4 text-right">
              <button type="button" class="btn btn-primary btn-sm" @click="closeDownloadParts">{{ t('common.close') }}</button>
            </div>
          </div>
        </div>
      </transition>
    </teleport>
    <TotpStepUpDialog :controller="backupStepUp" />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import { useAppStore } from '@/stores'
import type {
  BackupS3Config,
  BackupScheduleConfig,
  BackupMonthlyArchiveConfig,
  BackupRecord,
  BackupDownloadPart,
  ImageStorageConfig,
} from '@/api/admin/backup'
import { useStepUp, isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import BackupArchiveSettings from '@/components/admin/BackupArchiveSettings.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import ToolbarPopover from '@/components/common/ToolbarPopover.vue'
import Icon from '@/components/icons/Icon.vue'
import CountUp from '@/components/ui/CountUp.vue'
import SideDrawer from '@/components/ui/SideDrawer.vue'

const { t } = useI18n()
const appStore = useAppStore()
const backupStepUp = useStepUp()

// 敏感操作被 2FA 门控拦截时的统一提示。
function reportStepUpBlocked(error: unknown): boolean {
  if (!isStepUpBlocked(error)) return false
  appStore.showError(
    stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
      ? t('stepUp.adminApiKeyForbidden')
      : t('stepUp.notEnabled')
  )
  return true
}

// S3 config
const s3Form = ref<BackupS3Config>({
  endpoint: '',
  region: 'auto',
  bucket: '',
  access_key_id: '',
  secret_access_key: '',
  prefix: 'backups/',
  force_path_style: false,
})
const s3SecretConfigured = ref(false)
const savingS3 = ref(false)
const testingS3 = ref(false)

// Async image object storage. Shares the S3 client with backups, so the default is
// to reuse the credentials configured above and only differ by prefix.
const imageStorageForm = ref<ImageStorageConfig>({
  enabled: false,
  reuse_backup_s3: true,
  bucket: '',
  prefix: 'images/',
  public_base_url: '',
  presign_expiry_hours: 24,
  max_download_bytes: 33554432,
  endpoint: '',
  region: 'auto',
  access_key_id: '',
  secret_access_key: '',
  force_path_style: false,
})
const imageStorageSecretConfigured = ref(false)
const savingImageStorage = ref(false)
const testingImageStorage = ref(false)

// Schedule config
const scheduleForm = ref<BackupScheduleConfig>({
  enabled: false,
  cron_expr: '0 2 * * *',
  retain_days: 14,
  retain_count: 10,
})
const savingSchedule = ref(false)
// The KPI tile reflects the persisted schedule, not unsaved drawer edits.
const savedScheduleEnabled = ref(false)
const archiveForm = ref<BackupMonthlyArchiveConfig>({ enabled: false, days: [1], include_month_end: false, retain_count: 0 })
// Disabling hides the archive parameters, so a save while disabled keeps the
// persisted parameters and only turns the rule off.
const savedArchive = ref<BackupMonthlyArchiveConfig>(cloneArchive(archiveForm.value))
const archivePayload = computed<BackupMonthlyArchiveConfig>(() =>
  archiveForm.value.enabled ? cloneArchive(archiveForm.value) : { ...cloneArchive(savedArchive.value), enabled: false },
)
function cloneArchive(config: BackupMonthlyArchiveConfig): BackupMonthlyArchiveConfig {
  return { ...config, days: [...config.days] }
}
const scheduleValidationError = computed(() => {
  const counts = [scheduleForm.value.retain_days, scheduleForm.value.retain_count]
  if (archiveForm.value.enabled) counts.push(archiveForm.value.retain_count)
  if (!counts.every(value => Number.isSafeInteger(value) && value >= 0)) {
    return t('admin.backup.archive.invalidRetention')
  }
  if (archiveForm.value.enabled && !archiveForm.value.days.length && !archiveForm.value.include_month_end) {
    return t('admin.backup.archive.selectDates')
  }
  return ''
})
const retentionPreview = computed(() => {
  if (scheduleValidationError.value) return scheduleValidationError.value
  const { retain_days: days, retain_count: count } = scheduleForm.value
  const ordinary = days > 0 && count > 0
    ? t('admin.backup.schedule.previewBoth', { days, count })
    : days > 0 ? t('admin.backup.schedule.previewDays', { days })
      : count > 0 ? t('admin.backup.schedule.previewCount', { count }) : t('admin.backup.schedule.previewUnlimited')
  if (!archiveForm.value.enabled) return ordinary
  const dates = [
    ...[...archiveForm.value.days].sort((a, b) => a - b).map(day => t('admin.backup.archive.day', { day })),
    ...(archiveForm.value.include_month_end ? [t('admin.backup.archive.monthEnd')] : []),
  ].join(', ')
  const retention = archiveForm.value.retain_count === 0 ? t('admin.backup.archive.forever') : t('admin.backup.archive.retainLatest', { count: archiveForm.value.retain_count })
  return `${ordinary} ${t('admin.backup.archive.preview', { dates, retention })}`
})

// Backups
const backups = ref<BackupRecord[]>([])
const loadingBackups = ref(false)
const creatingBackup = ref(false)
const restoringId = ref('')
const manualExpireDays = ref(14)
const downloadParts = ref<BackupDownloadPart[]>([])
const downloadPartsModalOpen = ref(false)

// List filter, summary tiles, and the settings drawer
type StatusFilter = 'all' | 'completed' | 'running' | 'failed'
type SettingsTab = 'schedule' | 'storage' | 'images'
const statusFilter = ref<StatusFilter>('all')
const settingsOpen = ref(false)
const settingsTab = ref<SettingsTab>('schedule')
const statusOptions = computed<Array<{ id: StatusFilter; label: string }>>(() => [
  { id: 'all', label: t('common.all') },
  { id: 'completed', label: t('admin.backup.status.completed') },
  { id: 'running', label: t('admin.backup.status.running') },
  { id: 'failed', label: t('admin.backup.status.failed') },
])
const settingsTabs = computed<Array<{ id: SettingsTab; label: string }>>(() => [
  { id: 'schedule', label: t('admin.adminUi.backup.schedule') },
  { id: 'storage', label: t('admin.adminUi.backup.storage') },
  { id: 'images', label: t('admin.adminUi.backup.images') },
])
function statusBucket(record: BackupRecord): Exclude<StatusFilter, 'all'> {
  if (record.status === 'completed' || record.status === 'failed') return record.status
  return 'running'
}
const statusCounts = computed(() => {
  const counts: Record<StatusFilter, number> = { all: backups.value.length, completed: 0, running: 0, failed: 0 }
  for (const record of backups.value) counts[statusBucket(record)] += 1
  return counts
})
const visibleBackups = computed(() =>
  statusFilter.value === 'all' ? backups.value : backups.value.filter(record => statusBucket(record) === statusFilter.value),
)
const lastCompleted = computed(() =>
  backups.value
    .filter(record => record.status === 'completed')
    .reduce<BackupRecord | null>((latest, record) =>
      !latest || Date.parse(record.started_at) > Date.parse(latest.started_at) ? record : latest, null),
)
const totalBytes = computed(() => backups.value.reduce((sum, record) => sum + (record.size_bytes || 0), 0))
function openSettings(tab: SettingsTab) {
  settingsTab.value = tab
  settingsOpen.value = true
}

// Polling
const pollingTimer = ref<ReturnType<typeof setInterval> | null>(null)
const restoringPollingTimer = ref<ReturnType<typeof setInterval> | null>(null)
const MAX_POLL_COUNT = 900

function updateRecordInList(updated: BackupRecord) {
  const idx = backups.value.findIndex(r => r.id === updated.id)
  if (idx >= 0) {
    backups.value[idx] = updated
  }
}

function startPolling(backupId: string) {
  stopPolling()
  let count = 0
  pollingTimer.value = setInterval(async () => {
    if (count++ >= MAX_POLL_COUNT) {
      stopPolling()
      creatingBackup.value = false
      appStore.showWarning(t('admin.backup.operations.backupRunning'))
      return
    }
    try {
      const record = await adminAPI.backup.getBackup(backupId)
      updateRecordInList(record)
      if (record.status === 'completed' || record.status === 'failed') {
        stopPolling()
        creatingBackup.value = false
        if (record.status === 'completed') {
          appStore.showSuccess(t('admin.backup.operations.backupCreated'))
        } else {
          appStore.showError(record.error_message || t('admin.backup.operations.backupFailed'))
        }
        await loadBackups()
      }
    } catch {
      // 轮询失败时不中断
    }
  }, 2000)
}

function stopPolling() {
  if (pollingTimer.value) {
    clearInterval(pollingTimer.value)
    pollingTimer.value = null
  }
}

function startRestorePolling(backupId: string) {
  stopRestorePolling()
  let count = 0
  restoringPollingTimer.value = setInterval(async () => {
    if (count++ >= MAX_POLL_COUNT) {
      stopRestorePolling()
      restoringId.value = ''
      appStore.showWarning(t('admin.backup.operations.restoreRunning'))
      return
    }
    try {
      const record = await adminAPI.backup.getBackup(backupId)
      updateRecordInList(record)
      if (record.restore_status === 'completed' || record.restore_status === 'failed') {
        stopRestorePolling()
        restoringId.value = ''
        if (record.restore_status === 'completed') {
          appStore.showSuccess(t('admin.backup.actions.restoreSuccess'))
        } else {
          appStore.showError(record.restore_error || t('admin.backup.operations.restoreFailed'))
        }
        await loadBackups()
      }
    } catch {
      // 轮询失败时不中断
    }
  }, 2000)
}

function stopRestorePolling() {
  if (restoringPollingTimer.value) {
    clearInterval(restoringPollingTimer.value)
    restoringPollingTimer.value = null
  }
}

function handleVisibilityChange() {
  if (document.hidden) {
    stopPolling()
    stopRestorePolling()
  } else {
    // 标签页恢复时刷新列表，检查是否仍有活跃操作
    loadBackups().then(() => {
      const running = backups.value.find(r => r.status === 'running')
      if (running) {
        creatingBackup.value = true
        startPolling(running.id)
      }
      const restoring = backups.value.find(r => r.restore_status === 'running')
      if (restoring) {
        restoringId.value = restoring.id
        startRestorePolling(restoring.id)
      }
    })
  }
}

// R2 guide
const showR2Guide = ref(false)
const r2ConfigRows = computed(() => [
  { field: t('admin.backup.s3.endpoint'), value: 'https://<account_id>.r2.cloudflarestorage.com' },
  { field: t('admin.backup.s3.region'), value: 'auto' },
  { field: t('admin.backup.s3.bucket'), value: t('admin.backup.r2Guide.step4.bucketValue') },
  { field: t('admin.backup.s3.prefix'), value: 'backups/' },
  { field: 'Access Key ID', value: t('admin.backup.r2Guide.step4.fromStep2') },
  { field: 'Secret Access Key', value: t('admin.backup.r2Guide.step4.fromStep2') },
  { field: t('admin.backup.s3.forcePathStyle'), value: t('admin.backup.r2Guide.step4.unchecked') },
])

async function loadS3Config() {
  try {
    const cfg = await adminAPI.backup.getS3Config()
    s3Form.value = {
      endpoint: cfg.endpoint || '',
      region: cfg.region || 'auto',
      bucket: cfg.bucket || '',
      access_key_id: cfg.access_key_id || '',
      secret_access_key: '',
      prefix: cfg.prefix || 'backups/',
      force_path_style: cfg.force_path_style,
    }
    s3SecretConfigured.value = Boolean(cfg.access_key_id)
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  }
}

async function saveS3Config() {
  savingS3.value = true
  try {
    await backupStepUp.run(() => adminAPI.backup.updateS3Config(s3Form.value))
    appStore.showSuccess(t('admin.backup.s3.saved'))
    await loadS3Config()
  } catch (error) {
    if (isStepUpCancelled(error)) {
      savingS3.value = false
      return
    }
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    savingS3.value = false
  }
}

async function loadImageStorageConfig() {
  try {
    const { config, secret_configured } = await adminAPI.backup.getImageStorageConfig()
    imageStorageForm.value = {
      ...config,
      prefix: config.prefix || 'images/',
      region: config.region || 'auto',
      secret_access_key: '',
    }
    imageStorageSecretConfigured.value = secret_configured
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  }
}

async function saveImageStorageConfig() {
  savingImageStorage.value = true
  try {
    await backupStepUp.run(() => adminAPI.backup.updateImageStorageConfig(imageStorageForm.value))
    appStore.showSuccess(t('admin.backup.imageStorage.saved'))
    await loadImageStorageConfig()
  } catch (error) {
    if (isStepUpCancelled(error)) {
      savingImageStorage.value = false
      return
    }
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    savingImageStorage.value = false
  }
}

async function testImageStorage() {
  testingImageStorage.value = true
  try {
    const result = await adminAPI.backup.testImageStorageConnection(imageStorageForm.value)
    if (result.ok) {
      appStore.showSuccess(result.message || t('admin.backup.s3.testSuccess'))
    } else {
      appStore.showError(result.message || t('admin.backup.s3.testFailed'))
    }
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    testingImageStorage.value = false
  }
}

async function testS3() {
  testingS3.value = true
  try {
    const result = await adminAPI.backup.testS3Connection(s3Form.value)
    if (result.ok) {
      appStore.showSuccess(result.message || t('admin.backup.s3.testSuccess'))
    } else {
      appStore.showError(result.message || t('admin.backup.s3.testFailed'))
    }
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    testingS3.value = false
  }
}

async function loadSchedule() {
  try {
    const cfg = await adminAPI.backup.getSchedule()
    scheduleForm.value = {
      enabled: cfg.enabled,
      cron_expr: cfg.cron_expr || '0 2 * * *',
      retain_days: cfg.retain_days ?? 14,
      retain_count: cfg.retain_count ?? 10,
    }
    archiveForm.value = {
      enabled: cfg.monthly_archive?.enabled ?? false,
      days: cfg.monthly_archive?.days ?? [1],
      include_month_end: cfg.monthly_archive?.include_month_end ?? false,
      retain_count: cfg.monthly_archive?.retain_count ?? 0,
    }
    savedArchive.value = cloneArchive(archiveForm.value)
    savedScheduleEnabled.value = scheduleForm.value.enabled
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  }
}

async function saveSchedule() {
  if (scheduleValidationError.value) return
  savingSchedule.value = true
  try {
    const archive = archivePayload.value
    await adminAPI.backup.updateSchedule({ ...scheduleForm.value, monthly_archive: archive })
    savedArchive.value = cloneArchive(archive)
    savedScheduleEnabled.value = scheduleForm.value.enabled
    appStore.showSuccess(t('admin.backup.schedule.saved'))
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    savingSchedule.value = false
  }
}

async function loadBackups() {
  loadingBackups.value = true
  try {
    const result = await adminAPI.backup.listBackups()
    backups.value = result.items || []
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  } finally {
    loadingBackups.value = false
  }
}

async function createBackup() {
  creatingBackup.value = true
  try {
    const record = await backupStepUp.run(() => adminAPI.backup.createBackup({ expire_days: manualExpireDays.value }))
    // 插入到列表顶部
    backups.value.unshift(record)
    startPolling(record.id)
  } catch (error: any) {
    if (isStepUpCancelled(error)) {
      creatingBackup.value = false
      return
    }
    if (reportStepUpBlocked(error)) {
      creatingBackup.value = false
      return
    }
    if (error?.response?.status === 409) {
      appStore.showWarning(t('admin.backup.operations.alreadyInProgress'))
    } else {
      appStore.showError(error?.message || t('errors.networkError'))
    }
    creatingBackup.value = false
  }
}

async function downloadBackup(id: string) {
  try {
    const result = await backupStepUp.run(() => adminAPI.backup.getDownloadURL(id))
    if (result.parts && result.parts.length > 0) {
      downloadParts.value = result.parts
      downloadPartsModalOpen.value = true
      return
    }
    if (!result.url) {
      throw new Error(t('admin.backup.actions.downloadFailed'))
    }
    // 预签名 URL 带 attachment disposition，同页 anchor 导航直接触发下载；
    // 不用 window.open：step-up 弹窗 await 会耗尽瞬态用户激活，新标签页会被浏览器拦截。
    const link = document.createElement('a')
    link.href = result.url
    link.rel = 'noopener'
    link.click()
  } catch (error) {
    if (isStepUpCancelled(error)) return
    if (reportStepUpBlocked(error)) return
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  }
}

function closeDownloadParts() {
  downloadPartsModalOpen.value = false
  downloadParts.value = []
}

async function restoreBackup(id: string) {
  if (!window.confirm(t('admin.backup.actions.restoreConfirm'))) return
  const password = window.prompt(t('admin.backup.actions.restorePasswordPrompt'))
  if (!password) return
  restoringId.value = id
  try {
    const record = await backupStepUp.run(() => adminAPI.backup.restoreBackup(id, password))
    updateRecordInList(record)
    startRestorePolling(id)
  } catch (error: any) {
    restoringId.value = ''
    if (isStepUpCancelled(error)) return
    if (reportStepUpBlocked(error)) return
    // apiClient 拦截器把 HTTP 错误归一化为顶层 { status } 平面对象（无 response 字段）
    if (error?.status === 409 || error?.response?.status === 409) {
      appStore.showWarning(t('admin.backup.operations.restoreRunning'))
    } else {
      appStore.showError(error?.message || t('errors.networkError'))
    }
  }
}

async function removeBackup(id: string) {
  const archived = !!backups.value.find(record => record.id === id)?.monthly_archive
  if (!window.confirm(t(archived ? 'admin.backup.archive.deleteConfirm' : 'admin.backup.actions.deleteConfirm'))) return
  try {
    await adminAPI.backup.deleteBackup(id, archived)
    appStore.showSuccess(t('admin.backup.actions.deleted'))
    await loadBackups()
  } catch (error) {
    appStore.showError((error as { message?: string })?.message || t('errors.networkError'))
  }
}

function statusBadgeClass(status: string): string {
  if (status === 'completed') return 'badge-success'
  if (status === 'running') return 'badge-primary'
  if (status === 'failed') return 'badge-danger'
  return 'badge-gray'
}

function relativeAge(value?: string): string {
  const time = value ? Date.parse(value) : Number.NaN
  if (Number.isNaN(time)) return '-'
  const minutes = Math.floor((Date.now() - time) / 60000)
  if (minutes < 1) return t('common.time.justNow')
  if (minutes < 60) return t('common.time.minutesAgo', { n: minutes })
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return t('common.time.hoursAgo', { n: hours })
  return t('common.time.daysAgo', { n: Math.floor(hours / 24) })
}

function formatSize(bytes: number): string {
  if (!bytes || bytes <= 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

function formatDate(value?: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

onMounted(async () => {
  document.addEventListener('visibilitychange', handleVisibilityChange)
  await Promise.all([loadS3Config(), loadImageStorageConfig(), loadSchedule(), loadBackups()])

  // 如果有正在 running 的备份，恢复轮询
  const runningBackup = backups.value.find(r => r.status === 'running')
  if (runningBackup) {
    creatingBackup.value = true
    startPolling(runningBackup.id)
  }
  const restoringBackup = backups.value.find(r => r.restore_status === 'running')
  if (restoringBackup) {
    restoringId.value = restoringBackup.id
    startRestorePolling(restoringBackup.id)
  }
})

onBeforeUnmount(() => {
  stopPolling()
  stopRestorePolling()
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>

<style scoped>
.backup-schedule-value {
  display: flex;
  min-width: 0;
  max-width: 100%;
  align-items: center;
  gap: 0.5rem;
  border-radius: 0.375rem;
  text-align: left;
}

.backup-schedule-value:hover {
  color: var(--operator-accent-text);
}

.backup-schedule-value:focus-visible {
  outline: 2px solid var(--operator-accent-text);
  outline-offset: 2px;
}

.backup-schedule-chevron {
  flex: none;
  color: var(--operator-subtle-foreground);
}

.backup-seg-count {
  margin-left: 0.375rem;
  color: var(--operator-subtle-foreground);
  font-variant-numeric: tabular-nums;
}

.backup-expire {
  display: inline-flex;
  height: 2.25rem;
  align-items: center;
  gap: 0.375rem;
  padding: 0 0.625rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-card);
  color: var(--operator-subtle-foreground);
  font-size: 0.8125rem;
}

.backup-expire input {
  width: 2.75rem;
  border: 0;
  background: transparent;
  color: var(--operator-foreground);
  font-variant-numeric: tabular-nums;
  text-align: right;
  outline: none;
}

.backup-expire:focus-within {
  border-color: var(--operator-accent-text);
}

.backup-drawer-icon {
  display: inline-flex;
  width: 2.25rem;
  height: 2.25rem;
  align-items: center;
  justify-content: center;
  border-radius: var(--operator-radius);
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
}

.backup-label {
  display: flex;
  align-items: center;
  margin-bottom: 0.25rem;
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
  font-weight: 500;
}

.backup-switch {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--operator-foreground);
  font-size: 0.875rem;
}

.backup-preview {
  padding: 0.75rem;
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
