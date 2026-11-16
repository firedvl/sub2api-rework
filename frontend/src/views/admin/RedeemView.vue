<template>
  <AppLayout>
    <TablePageLayout class="ui-page ui-stagger">
      <template #filters>
        <div class="redeem-top">
          <div class="ui-toolbar" role="toolbar" :aria-label="t('admin.adminUi.filters')">
            <label class="ui-search">
              <Icon name="search" size="sm" />
              <input
                v-model="searchQuery"
                type="text"
                :placeholder="t('admin.redeem.searchCodes')"
                :aria-label="t('admin.redeem.searchCodes')"
                @input="handleSearch"
              />
            </label>

            <div class="viz-seg" role="group" :aria-label="t('admin.redeem.columns.status')">
              <button
                v-for="option in filterStatusOptions"
                :key="option.value"
                type="button"
                :aria-pressed="filters.status === option.value"
                @click="setStatusFilter(option.value)"
              >
                {{ option.label }}
                <span
                  v-if="filters.status === option.value && !loading && pagination.total > 0"
                  class="ui-chip-count"
                >{{ pagination.total }}</span>
              </button>
            </div>

            <ToolbarPopover
              :label="t('admin.adminUi.filters')"
              icon="filter"
              :count="filters.type ? 1 : 0"
              align="left"
            >
              <div>
                <div class="ui-section-label">{{ t('admin.redeem.columns.type') }}</div>
                <div class="ui-chips" role="group" :aria-label="t('admin.redeem.columns.type')">
                  <button
                    v-for="option in filterTypeOptions"
                    :key="option.value"
                    type="button"
                    class="ui-chip"
                    :aria-pressed="filters.type === option.value"
                    @click="setTypeFilter(option.value)"
                  >
                    <Icon v-if="option.value" :name="typeIcon(option.value)" size="sm" />
                    {{ option.label }}
                  </button>
                </div>
              </div>
            </ToolbarPopover>

            <span class="ui-grow" />

            <button
              type="button"
              class="app-icon-button"
              :disabled="loading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="loadCodes"
            >
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </button>

            <ToolbarPopover
              :label="t('admin.adminUi.moreActions')"
              icon="more"
              icon-only
              menu
              width="15rem"
            >
              <button type="button" role="menuitem" class="dropdown-item" @click="handleExportCodes">
                <Icon name="download" size="sm" />
                {{ t('admin.redeem.exportCsv') }}
              </button>
              <button
                type="button"
                role="menuitem"
                class="dropdown-item"
                :disabled="selectedCount === 0 || batchUpdating"
                @click="openBatchUpdateDialog"
              >
                <Icon name="edit" size="sm" />
                {{ t('admin.redeem.batchUpdate') }}
              </button>
              <div class="redeem-menu-divider" role="separator"></div>
              <button
                type="button"
                role="menuitem"
                class="dropdown-item is-danger"
                @click="showDeleteUnusedDialog = true"
              >
                <Icon name="trash" size="sm" />
                {{ t('admin.redeem.deleteAllUnused') }}
              </button>
            </ToolbarPopover>

            <button type="button" class="btn btn-primary" @click="showGenerateDialog = true">
              <Icon name="plus" size="sm" />
              {{ t('admin.redeem.generateCodes') }}
            </button>
          </div>

          <Transition name="fade">
            <div v-if="selectedCount > 0" class="redeem-bulkbar">
              <span class="redeem-bulkbar-count">
                {{ t('admin.redeem.selectedCount', { count: selectedCount }) }}
              </span>
              <span class="ui-grow" />
              <button
                type="button"
                data-test="batch-update-open"
                class="btn btn-secondary btn-sm"
                :disabled="batchUpdating"
                @click="openBatchUpdateDialog"
              >
                <Icon name="edit" size="sm" />
                {{ t('admin.redeem.batchUpdate') }}
              </button>
              <button type="button" class="btn btn-ghost btn-sm" @click="clearSelectedCodes">
                {{ t('admin.redeem.clearSelection') }}
              </button>
            </div>
          </Transition>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="codes"
          :loading="loading"
          clickable-rows
          :server-side-sort="true"
          default-sort-key="id"
          default-sort-order="desc"
          @sort="handleSort"
          @row-click="openDetail"
        >
          <template #header-select>
            <input
              data-test="select-all-codes"
              type="checkbox"
              class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :checked="allVisibleSelected"
              :aria-label="t('common.selectAll')"
              @click.stop
              @change="toggleSelectAllVisible($event)"
            />
          </template>

          <template #cell-select="{ row }">
            <input
              data-test="select-code"
              type="checkbox"
              class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :checked="selectedCodeIds.has(row.id)"
              :aria-label="row.code"
              @click.stop
              @change="toggleSelectRow(row.id, $event)"
            />
          </template>

          <template #cell-code="{ value }">
            <span class="redeem-code">
              <code>{{ value }}</code>
              <button
                type="button"
                class="redeem-copy"
                :class="copiedCode === value ? 'is-copied' : ''"
                :title="copiedCode === value ? t('admin.redeem.copied') : t('keys.copyToClipboard')"
                :aria-label="copiedCode === value ? t('admin.redeem.copied') : t('keys.copyToClipboard')"
                @click.stop="copyToClipboard(value)"
              >
                <Icon :name="copiedCode === value ? 'check' : 'copy'" size="sm" :stroke-width="2" />
              </button>
            </span>
          </template>

          <template #cell-type="{ value }">
            <span :class="typeBadgeClass(value)">
              {{ t('admin.redeem.types.' + value) }}
            </span>
          </template>

          <template #cell-value="{ value, row }">
            <span class="redeem-value">
              <template v-if="row.type === 'balance'">${{ value.toFixed(2) }}</template>
              <template v-else-if="row.type === 'subscription'">
                {{ row.validity_days || 30 }}{{ t('admin.redeem.days') }}
                <span v-if="row.group" class="redeem-value-group">
                  <ProviderMark :platform="row.group.platform" :size="16" />
                  {{ row.group.name }}
                </span>
              </template>
              <template v-else>{{ value }}</template>
            </span>
          </template>

          <template #cell-status="{ value }">
            <span :class="statusBadgeClass(value)">
              {{ t('admin.redeem.status.' + value) }}
            </span>
          </template>

          <template #cell-used_by="{ value, row }">
            <span class="redeem-muted">
              {{ row.user?.email || (value ? t('admin.redeem.userPrefix', { id: value }) : '-') }}
            </span>
          </template>

          <template #cell-used_at="{ value }">
            <span class="redeem-muted">{{ value ? formatDateTime(value) : '-' }}</span>
          </template>

          <template #cell-expires_at="{ value, row }">
            <span :class="row.status === 'expired' ? 'redeem-muted is-expired' : 'redeem-muted'">
              {{ value ? formatDateTime(value) : t('admin.redeem.neverExpires') }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="redeem-row-actions" @click.stop @keydown.enter.stop>
              <button
                type="button"
                class="app-icon-button"
                :title="t('admin.adminUi.details')"
                :aria-label="t('admin.adminUi.details')"
                @click="openDetail(row)"
              >
                <Icon name="eye" size="sm" />
              </button>
              <button
                v-if="row.status === 'unused'"
                type="button"
                class="app-icon-button redeem-danger"
                :title="t('common.delete')"
                :aria-label="t('common.delete')"
                @click="handleDelete(row)"
              >
                <Icon name="trash" size="sm" />
              </button>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Redeem code detail -->
    <SideDrawer
      :open="detailCode !== null"
      :title="detailCode?.code ?? ''"
      :subtitle="detailCode ? t('admin.redeem.types.' + detailCode.type) : undefined"
      width="28rem"
      @close="closeDetail"
    >
      <template #leading>
        <span v-if="detailCode" class="redeem-drawer-icon" aria-hidden="true">
          <Icon :name="typeIcon(detailCode.type)" size="md" />
        </span>
      </template>
      <template #badge>
        <span v-if="detailCode" :class="statusBadgeClass(detailCode.status)">
          {{ t('admin.redeem.status.' + detailCode.status) }}
        </span>
      </template>

      <template v-if="detailCode">
        <div class="redeem-hero">
          <span class="ui-section-label">{{ t('admin.redeem.columns.value') }}</span>
          <b v-if="detailCode.type === 'balance'">${{ detailCode.value.toFixed(2) }}</b>
          <b v-else-if="detailCode.type === 'subscription'">
            {{ detailCode.validity_days || 30 }}{{ t('admin.redeem.days') }}
          </b>
          <b v-else>{{ detailCode.value }}</b>
        </div>

        <dl class="ui-dl">
          <template v-if="detailCode.group">
            <dt>{{ t('admin.redeem.batchFields.group') }}</dt>
            <dd>
              <GroupBadge
                :name="detailCode.group.name"
                :platform="detailCode.group.platform"
                :subscription-type="detailCode.group.subscription_type"
                :rate-multiplier="detailCode.group.rate_multiplier"
              />
            </dd>
          </template>
          <dt>{{ t('admin.redeem.columns.usedBy') }}</dt>
          <dd>
            {{ detailCode.user?.email || (detailCode.used_by ? t('admin.redeem.userPrefix', { id: detailCode.used_by }) : '-') }}
          </dd>
          <dt>{{ t('admin.redeem.columns.usedAt') }}</dt>
          <dd>{{ detailCode.used_at ? formatDateTime(detailCode.used_at) : '-' }}</dd>
          <dt>{{ t('admin.redeem.columns.createdAt') }}</dt>
          <dd>{{ formatDateTime(detailCode.created_at) }}</dd>
          <dt>{{ t('admin.redeem.columns.expiresAt') }}</dt>
          <dd :class="detailCode.status === 'expired' ? 'redeem-expired' : ''">
            {{ detailCode.expires_at ? formatDateTime(detailCode.expires_at) : t('admin.redeem.neverExpires') }}
          </dd>
          <dt>{{ t('admin.redeem.batchFields.notes') }}</dt>
          <dd class="whitespace-pre-wrap">{{ detailCode.notes || '-' }}</dd>
        </dl>
      </template>

      <template #footer>
        <template v-if="detailCode">
          <button type="button" class="btn btn-secondary btn-sm" @click="copyToClipboard(detailCode.code)">
            <Icon :name="copiedCode === detailCode.code ? 'check' : 'copy'" size="sm" />
            {{ copiedCode === detailCode.code ? t('admin.redeem.copied') : t('keys.copyToClipboard') }}
          </button>
          <span class="flex-1" />
          <button
            v-if="detailCode.status === 'unused'"
            type="button"
            class="btn btn-ghost btn-sm redeem-danger"
            @click="deleteFromDrawer(detailCode)"
          >
            <Icon name="trash" size="sm" />
            {{ t('common.delete') }}
          </button>
        </template>
      </template>
    </SideDrawer>

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.redeem.deleteCode')"
      :message="t('admin.redeem.deleteCodeConfirm')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- Delete Unused Codes Dialog -->
    <ConfirmDialog
      :show="showDeleteUnusedDialog"
      :title="t('admin.redeem.deleteAllUnused')"
      :message="t('admin.redeem.deleteAllUnusedConfirm')"
      :confirm-text="t('admin.redeem.deleteAll')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDeleteUnused"
      @cancel="showDeleteUnusedDialog = false"
    />

    <!-- Generate Codes Dialog -->
    <Teleport to="body">
      <div v-if="showGenerateDialog" class="fixed inset-0 z-50 flex items-center justify-center">
        <div class="fixed inset-0 bg-black/50" @click="showGenerateDialog = false"></div>
        <div
          class="relative z-10 w-full max-w-md rounded-xl bg-white p-6 shadow-xl dark:bg-dark-800"
        >
          <h2 class="mb-4 text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.redeem.generateCodesTitle') }}
          </h2>
          <form @submit.prevent="handleGenerateCodes" class="space-y-4">
            <div>
              <label class="input-label">{{ t('admin.redeem.codeType') }}</label>
              <Select v-model="generateForm.type" :options="typeOptions" />
            </div>
            <!-- 余额/并发类型：显示数值输入 -->
            <div v-if="generateForm.type !== 'subscription' && generateForm.type !== 'invitation'">
              <label class="input-label">
                {{
                  generateForm.type === 'balance'
                    ? t('admin.redeem.amount')
                    : t('admin.redeem.columns.value')
                }}
              </label>
              <input
                v-model.number="generateForm.value"
                type="number"
                :step="generateForm.type === 'balance' ? '0.01' : '1'"
                :min="generateForm.type === 'balance' ? '0.01' : '1'"
                required
                class="input"
              />
            </div>
            <!-- 邀请码类型：显示提示信息 -->
            <div v-if="generateForm.type === 'invitation'" class="rounded-lg bg-blue-50 p-3 dark:bg-blue-900/20">
              <p class="text-sm text-blue-700 dark:text-blue-300">
                {{ t('admin.redeem.invitationHint') }}
              </p>
            </div>
            <!-- 订阅类型：显示分组选择和有效天数 -->
            <template v-if="generateForm.type === 'subscription'">
              <div>
                <label class="input-label">{{ t('admin.redeem.selectGroup') }}</label>
                <Select
                  v-model="generateForm.group_id"
                  :options="subscriptionGroupOptions"
                  :placeholder="t('admin.redeem.selectGroupPlaceholder')"
                >
                  <template #selected="{ option }">
                    <GroupBadge
                      v-if="option"
                      :name="(option as unknown as GroupOption).label"
                      :platform="(option as unknown as GroupOption).platform"
                      :subscription-type="(option as unknown as GroupOption).subscriptionType"
                      :rate-multiplier="(option as unknown as GroupOption).rate"
                    />
                    <span v-else class="text-gray-400">{{
                      t('admin.redeem.selectGroupPlaceholder')
                    }}</span>
                  </template>
                  <template #option="{ option, selected }">
                    <GroupOptionItem
                      :name="(option as unknown as GroupOption).label"
                      :platform="(option as unknown as GroupOption).platform"
                      :subscription-type="(option as unknown as GroupOption).subscriptionType"
                      :rate-multiplier="(option as unknown as GroupOption).rate"
                      :description="(option as unknown as GroupOption).description"
                      :selected="selected"
                    />
                  </template>
                </Select>
              </div>
              <div>
                <label class="input-label">{{ t('admin.redeem.validityDays') }}</label>
                <input
                  v-model.number="generateForm.validity_days"
                  type="number"
                  min="1"
                  max="36500"
                  required
                  class="input"
                />
              </div>
            </template>
            <div>
              <label class="input-label">{{ t('admin.redeem.codeExpiry') }}</label>
              <div class="grid grid-cols-2 gap-2 sm:grid-cols-5">
                <button
                  v-for="option in redeemCodeExpiryOptions"
                  :key="option.value"
                  type="button"
                  @click="generateForm.expiry_option = option.value"
                  :class="[
                    'rounded-lg border px-3 py-2 text-sm transition-colors',
                    generateForm.expiry_option === option.value
                      ? 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-400 dark:bg-primary-900/20 dark:text-primary-300'
                      : 'border-gray-200 text-gray-700 hover:bg-gray-50 dark:border-dark-600 dark:text-gray-300 dark:hover:bg-dark-700'
                  ]"
                >
                  {{ option.label }}
                </button>
              </div>
              <input
                v-if="generateForm.expiry_option === 'custom'"
                v-model.number="generateForm.custom_expiry_days"
                type="number"
                min="1"
                max="3650"
                required
                class="input mt-2"
                :placeholder="t('admin.redeem.customExpiryDays')"
              />
            </div>
            <div>
              <label class="input-label">{{ t('admin.redeem.count') }}</label>
              <input
                v-model.number="generateForm.count"
                type="number"
                min="1"
                max="100"
                required
                class="input"
              />
            </div>
            <div class="flex justify-end gap-3 pt-2">
              <button type="button" @click="showGenerateDialog = false" class="btn btn-secondary">
                {{ t('common.cancel') }}
              </button>
              <button type="submit" :disabled="generating" class="btn btn-primary">
                {{ generating ? t('admin.redeem.generating') : t('admin.redeem.generate') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>

    <!-- Batch Update Dialog -->
    <Teleport to="body">
      <div
        v-if="showBatchUpdateDialog"
        class="fixed inset-0 z-50 flex items-center justify-center p-4"
      >
        <div class="fixed inset-0 bg-black/50" @click="closeBatchUpdateDialog"></div>
        <div
          class="relative z-10 w-full max-w-lg rounded-xl bg-white p-6 shadow-xl dark:bg-dark-800"
        >
          <h2 class="mb-1 text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.redeem.batchUpdateTitle') }}
          </h2>
          <p class="mb-4 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.redeem.selectedCount', { count: selectedCount }) }}
          </p>

          <form data-test="batch-update-form" class="space-y-4" @submit.prevent="handleBatchUpdate">
            <div class="space-y-2">
              <label class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                <input
                  data-test="batch-field-status"
                  v-model="batchUpdateForm.update_status"
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                />
                {{ t('admin.redeem.batchFields.status') }}
              </label>
              <Select
                v-if="batchUpdateForm.update_status"
                v-model="batchUpdateForm.status"
                data-test="batch-status-select"
                :options="batchStatusOptions"
              />
            </div>

            <div class="space-y-2">
              <label class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                <input
                  v-model="batchUpdateForm.update_expires_at"
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                />
                {{ t('admin.redeem.batchFields.expiresAt') }}
              </label>
              <template v-if="batchUpdateForm.update_expires_at">
                <Select v-model="batchUpdateForm.expires_mode" :options="batchExpiryModeOptions" />
                <input
                  v-if="batchUpdateForm.expires_mode === 'custom'"
                  v-model="batchUpdateForm.expires_at_local"
                  type="datetime-local"
                  class="input"
                />
                <p v-if="batchUpdateForm.expires_mode === 'custom'" class="input-hint">
                  {{ t('admin.redeem.localTimeZoneHint', { timezone: browserTimeZone }) }}
                </p>
              </template>
            </div>

            <div class="space-y-2">
              <label class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                <input
                  data-test="batch-field-notes"
                  v-model="batchUpdateForm.update_notes"
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                />
                {{ t('admin.redeem.batchFields.notes') }}
              </label>
              <textarea
                v-if="batchUpdateForm.update_notes"
                data-test="batch-notes-input"
                v-model="batchUpdateForm.notes"
                rows="3"
                class="input"
                :placeholder="t('admin.redeem.batchNotesPlaceholder')"
              ></textarea>
            </div>

            <div class="space-y-2">
              <label class="flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-300">
                <input
                  v-model="batchUpdateForm.update_group_id"
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                />
                {{ t('admin.redeem.batchFields.group') }}
              </label>
              <Select
                v-if="batchUpdateForm.update_group_id"
                v-model="batchUpdateForm.group_id"
                :options="batchGroupOptions"
                :placeholder="t('admin.redeem.selectGroupPlaceholder')"
              />
            </div>

            <div class="flex justify-end gap-3 pt-2">
              <button type="button" @click="closeBatchUpdateDialog" class="btn btn-secondary">
                {{ t('common.cancel') }}
              </button>
              <button
                data-test="batch-update-submit"
                type="submit"
                :disabled="batchUpdating"
                class="btn btn-primary"
              >
                {{ batchUpdating ? t('common.submitting') : t('admin.redeem.batchUpdate') }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>

    <!-- Generated Codes Result Dialog -->
    <Teleport to="body">
      <div v-if="showResultDialog" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="fixed inset-0 bg-black/50" @click="closeResultDialog"></div>
        <div class="relative z-10 w-full max-w-lg rounded-xl bg-white shadow-xl dark:bg-dark-800">
          <!-- Header -->
          <div
            class="flex items-center justify-between border-b border-gray-200 px-5 py-4 dark:border-dark-600"
          >
            <div class="flex items-center gap-3">
              <div
                class="flex h-10 w-10 items-center justify-center rounded-full bg-green-100 dark:bg-green-900/30"
              >
                <svg
                  class="h-5 w-5 text-green-600 dark:text-green-400"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M5 13l4 4L19 7"
                  />
                </svg>
              </div>
              <div>
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">
                  {{ t('admin.redeem.generatedSuccessfully') }}
                </h2>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('admin.redeem.codesCreated', { count: generatedCodes.length }) }}
                </p>
              </div>
            </div>
            <button
              @click="closeResultDialog"
              class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-300"
            >
              <Icon name="x" size="md" :stroke-width="2" />
            </button>
          </div>
          <!-- Content -->
          <div class="p-5">
            <div class="relative">
              <textarea
                readonly
                :value="generatedCodesText"
                :style="{ height: textareaHeight }"
                class="w-full resize-none rounded-lg border border-gray-200 bg-gray-50 p-3 font-mono text-sm text-gray-800 focus:outline-none dark:border-dark-600 dark:bg-dark-700 dark:text-gray-200"
              ></textarea>
            </div>
          </div>
          <!-- Footer -->
          <div
            class="flex justify-end gap-2 rounded-b-xl border-t border-gray-200 bg-gray-50 px-5 py-4 dark:border-dark-600 dark:bg-dark-700/50"
          >
            <button
              @click="copyGeneratedCodes"
              :class="[
                'btn flex items-center gap-2 transition-all',
                copiedAll ? 'btn-success' : 'btn-secondary'
              ]"
            >
              <Icon v-if="!copiedAll" name="copy" size="sm" :stroke-width="2" />
              <svg v-else class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M5 13l4 4L19 7"
                />
              </svg>
              {{ copiedAll ? t('admin.redeem.copied') : t('admin.redeem.copyAll') }}
            </button>
            <button @click="downloadGeneratedCodes" class="btn btn-primary flex items-center gap-2">
              <Icon name="download" size="sm" :stroke-width="2" />
              {{ t('admin.redeem.download') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { useTableSelection } from '@/composables/useTableSelection'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import {
  formatDateTime,
  getBrowserTimeZone,
  parseDateTimeLocalInput
} from '@/utils/format'
import type {
  RedeemCode,
  RedeemCodeType,
  Group,
  GroupPlatform,
  SubscriptionType,
  BatchUpdateRedeemCodeFields
} from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import ToolbarPopover from '@/components/common/ToolbarPopover.vue'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard: clipboardCopy } = useClipboard()
const browserTimeZone = getBrowserTimeZone()

interface GroupOption {
  value: number
  label: string
  description: string | null
  platform: GroupPlatform
  subscriptionType: SubscriptionType
  rate: number
}

const showGenerateDialog = ref(false)
const showResultDialog = ref(false)
const generatedCodes = ref<RedeemCode[]>([])
const subscriptionGroups = ref<Group[]>([])

// 订阅类型分组选项
const subscriptionGroupOptions = computed(() => {
  return subscriptionGroups.value
    .filter((g) => g.subscription_type === 'subscription')
    .map((g) => ({
      value: g.id,
      label: g.name,
      description: g.description,
      platform: g.platform,
      subscriptionType: g.subscription_type,
      rate: g.rate_multiplier
    }))
})

const batchGroupOptions = computed(() => [
  { value: null, label: t('admin.redeem.clearGroup') },
  ...subscriptionGroupOptions.value
])

const generatedCodesText = computed(() => {
  return generatedCodes.value.map((code) => code.code).join('\n')
})

const textareaHeight = computed(() => {
  const lineCount = generatedCodes.value.length
  const lineHeight = 24 // approximate line height in px
  const padding = 24 // top + bottom padding
  const minHeight = 60
  const maxHeight = 240
  const calculatedHeight = Math.min(
    Math.max(lineCount * lineHeight + padding, minHeight),
    maxHeight
  )
  return `${calculatedHeight}px`
})

const copiedAll = ref(false)

const closeResultDialog = () => {
  showResultDialog.value = false
  generatedCodes.value = []
  copiedAll.value = false
}

const copyGeneratedCodes = async () => {
  const success = await clipboardCopy(generatedCodesText.value, t('admin.redeem.copied'))
  if (success) {
    copiedAll.value = true
    setTimeout(() => {
      copiedAll.value = false
    }, 2000)
  }
}

const downloadGeneratedCodes = () => {
  const blob = new Blob([generatedCodesText.value], { type: 'text/plain' })
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `redeem-codes-${new Date().toISOString().split('T')[0]}.txt`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(url)
}

const columns = computed<Column[]>(() => [
  { key: 'select', label: '' },
  { key: 'code', label: t('admin.redeem.columns.code') },
  { key: 'type', label: t('admin.redeem.columns.type'), sortable: true },
  { key: 'value', label: t('admin.redeem.columns.value'), sortable: true },
  { key: 'status', label: t('admin.redeem.columns.status'), sortable: true },
  { key: 'used_by', label: t('admin.redeem.columns.usedBy') },
  { key: 'used_at', label: t('admin.redeem.columns.usedAt'), sortable: true },
  { key: 'expires_at', label: t('admin.redeem.columns.expiresAt'), sortable: true },
  { key: 'actions', label: t('admin.redeem.columns.actions') }
])

const typeOptions = computed(() => [
  { value: 'balance', label: t('admin.redeem.balance') },
  { value: 'concurrency', label: t('admin.redeem.concurrency') },
  { value: 'subscription', label: t('admin.redeem.subscription') },
  { value: 'invitation', label: t('admin.redeem.invitation') }
])

const filterTypeOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'balance', label: t('admin.redeem.balance') },
  { value: 'concurrency', label: t('admin.redeem.concurrency') },
  { value: 'subscription', label: t('admin.redeem.subscription') },
  { value: 'invitation', label: t('admin.redeem.invitation') }
])

const filterStatusOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'unused', label: t('admin.redeem.unused') },
  { value: 'used', label: t('admin.redeem.used') },
  { value: 'expired', label: t('admin.redeem.status.expired') },
  { value: 'disabled', label: t('admin.redeem.status.disabled') }
])

const batchStatusOptions = computed(() => [
  { value: 'unused', label: t('admin.redeem.status.unused') },
  { value: 'disabled', label: t('admin.redeem.status.disabled') }
])

const batchExpiryModeOptions = computed(() => [
  { value: 'clear', label: t('admin.redeem.neverExpires') },
  { value: 'custom', label: t('admin.redeem.customExpiry') }
])

const codes = ref<RedeemCode[]>([])
const loading = ref(false)
const generating = ref(false)
const batchUpdating = ref(false)
const searchQuery = ref('')
const filters = reactive({
  type: '',
  status: ''
})
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})
const sortState = reactive({
  sort_by: 'id',
  sort_order: 'desc' as 'asc' | 'desc'
})

let abortController: AbortController | null = null

const showDeleteDialog = ref(false)
const showDeleteUnusedDialog = ref(false)
const showBatchUpdateDialog = ref(false)
const deletingCode = ref<RedeemCode | null>(null)
const copiedCode = ref<string | null>(null)

const {
  selectedSet: selectedCodeIds,
  selectedCount,
  allVisibleSelected,
  select,
  deselect,
  clear: clearSelectedCodes,
  toggleVisible
} = useTableSelection<RedeemCode>({
  rows: codes,
  getId: (code) => code.id
})

const batchUpdateForm = reactive({
  update_status: false,
  status: 'disabled' as 'unused' | 'disabled',
  update_expires_at: false,
  expires_mode: 'clear' as 'clear' | 'custom',
  expires_at_local: '',
  update_notes: false,
  notes: '',
  update_group_id: false,
  group_id: null as number | null
})

type RedeemCodeExpiryOption = 'never' | '1' | '3' | '7' | 'custom'

const redeemCodeExpiryOptions = computed<{ value: RedeemCodeExpiryOption; label: string }[]>(() => [
  { value: 'never', label: t('admin.redeem.neverExpires') },
  { value: '1', label: t('admin.redeem.expiryPresetDays', { days: 1 }) },
  { value: '3', label: t('admin.redeem.expiryPresetDays', { days: 3 }) },
  { value: '7', label: t('admin.redeem.expiryPresetDays', { days: 7 }) },
  { value: 'custom', label: t('admin.redeem.customExpiry') }
])

const generateForm = reactive({
  type: 'balance' as RedeemCodeType,
  value: 10,
  count: 1,
  group_id: null as number | null,
  validity_days: 30,
  expiry_option: 'never' as RedeemCodeExpiryOption,
  custom_expiry_days: 7
})

// 监听类型变化，邀请码类型时自动设置 value 为 0
watch(
  () => generateForm.type,
  (newType) => {
    if (newType === 'invitation') {
      generateForm.value = 0
    } else if (generateForm.value === 0) {
      generateForm.value = 10
    }
  }
)

const buildRedeemQueryFilters = () => ({
  type: (filters.type || undefined) as RedeemCodeType | undefined,
  status: (filters.status || undefined) as 'used' | 'expired' | 'unused' | 'disabled' | undefined,
  search: searchQuery.value || undefined,
  sort_by: sortState.sort_by,
  sort_order: sortState.sort_order
})

const loadCodes = async () => {
  if (abortController) {
    abortController.abort()
  }
  const currentController = new AbortController()
  abortController = currentController
  loading.value = true
  try {
    const response = await adminAPI.redeem.list(
      pagination.page,
      pagination.page_size,
      buildRedeemQueryFilters(),
      {
        signal: currentController.signal
      }
    )
    if (currentController.signal.aborted) {
      return
    }
    codes.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error: any) {
    if (
      currentController.signal.aborted ||
      error?.name === 'AbortError' ||
      error?.code === 'ERR_CANCELED'
    ) {
      return
    }
    appStore.showError(t('admin.redeem.failedToLoad'))
    console.error('Error loading redeem codes:', error)
  } finally {
    if (abortController === currentController && !currentController.signal.aborted) {
      loading.value = false
      abortController = null
    }
  }
}

const setStatusFilter = (status: string) => {
  if (filters.status === status) return
  filters.status = status
  pagination.page = 1
  loadCodes()
}

const setTypeFilter = (type: string) => {
  if (filters.type === type) return
  filters.type = type
  pagination.page = 1
  loadCodes()
}

const typeIcon = (type: string) => {
  if (type === 'balance' || type === 'admin_balance') return 'wallet'
  if (type === 'concurrency' || type === 'admin_concurrency') return 'bolt'
  if (type === 'subscription') return 'calendar'
  if (type === 'invitation') return 'userPlus'
  return 'ticket'
}

const typeBadgeClass = (type: string) =>
  type === 'balance' ? 'badge badge-success' : type === 'subscription' ? 'badge badge-warning' : 'badge badge-primary'

const statusBadgeClass = (status: string) =>
  status === 'unused'
    ? 'badge badge-success'
    : status === 'used'
      ? 'badge badge-gray'
      : status === 'expired'
        ? 'badge badge-warning'
        : 'badge badge-danger'

// Detail drawer: follows the row by ID so it reflects reloaded data.
const detailId = ref<number | null>(null)
const detailCode = computed(() =>
  detailId.value === null ? null : (codes.value.find((code) => code.id === detailId.value) ?? null)
)
const openDetail = (code: RedeemCode) => {
  detailId.value = code.id
}
const closeDetail = () => {
  detailId.value = null
}

let searchTimeout: ReturnType<typeof setTimeout>
const handleSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadCodes()
  }, 300)
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadCodes()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadCodes()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadCodes()
}

const toggleSelectRow = (id: number, event: Event) => {
  const target = event.target as HTMLInputElement
  if (target.checked) {
    select(id)
    return
  }
  deselect(id)
}

const toggleSelectAllVisible = (event: Event) => {
  const target = event.target as HTMLInputElement
  toggleVisible(target.checked)
}

const getRedeemCodeExpiresInDays = () => {
  if (generateForm.expiry_option === 'never') {
    return undefined
  }
  if (generateForm.expiry_option === 'custom') {
    if (
      !Number.isFinite(generateForm.custom_expiry_days) ||
      generateForm.custom_expiry_days < 1
    ) {
      return null
    }
    return Math.floor(generateForm.custom_expiry_days)
  }
  return Number(generateForm.expiry_option)
}

const toDatetimeLocalInputValue = (date: Date) => {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(
    date.getHours()
  )}:${pad(date.getMinutes())}`
}

const resetBatchUpdateForm = () => {
  batchUpdateForm.update_status = false
  batchUpdateForm.status = 'disabled'
  batchUpdateForm.update_expires_at = false
  batchUpdateForm.expires_mode = 'clear'
  batchUpdateForm.expires_at_local = toDatetimeLocalInputValue(
    new Date(Date.now() + 24 * 60 * 60 * 1000)
  )
  batchUpdateForm.update_notes = false
  batchUpdateForm.notes = ''
  batchUpdateForm.update_group_id = false
  batchUpdateForm.group_id = null
}

const openBatchUpdateDialog = () => {
  if (selectedCount.value === 0) {
    appStore.showInfo(t('admin.redeem.selectCodesFirst'))
    return
  }
  resetBatchUpdateForm()
  showBatchUpdateDialog.value = true
}

const closeBatchUpdateDialog = () => {
  showBatchUpdateDialog.value = false
}

const buildBatchUpdateFields = (): BatchUpdateRedeemCodeFields | null => {
  const fields: BatchUpdateRedeemCodeFields = {}

  if (batchUpdateForm.update_status) {
    fields.status = batchUpdateForm.status
  }
  if (batchUpdateForm.update_expires_at) {
    if (batchUpdateForm.expires_mode === 'clear') {
      fields.expires_at = null
    } else {
      const expiresAt = parseDateTimeLocalInput(batchUpdateForm.expires_at_local)
      if (expiresAt === null) {
        appStore.showError(t('admin.redeem.expiryDateRequired'))
        return null
      }
      fields.expires_at = new Date(expiresAt * 1000).toISOString()
    }
  }
  if (batchUpdateForm.update_notes) {
    fields.notes = batchUpdateForm.notes
  }
  if (batchUpdateForm.update_group_id) {
    fields.group_id =
      batchUpdateForm.group_id == null ? null : Number(batchUpdateForm.group_id)
  }

  return Object.keys(fields).length > 0 ? fields : null
}

const handleGenerateCodes = async () => {
  // 订阅类型必须选择分组
  if (generateForm.type === 'subscription' && !generateForm.group_id) {
    appStore.showError(t('admin.redeem.groupRequired'))
    return
  }

  const expiresInDays = getRedeemCodeExpiresInDays()
  if (expiresInDays === null) {
    appStore.showError(t('admin.redeem.expiryDaysRequired'))
    return
  }

  generating.value = true
  try {
    const result = await adminAPI.redeem.generate(
      generateForm.count,
      generateForm.type,
      generateForm.value,
      generateForm.type === 'subscription' ? generateForm.group_id : undefined,
      generateForm.type === 'subscription' ? generateForm.validity_days : undefined,
      expiresInDays
    )
    showGenerateDialog.value = false
    generatedCodes.value = result
    showResultDialog.value = true
    // 重置表单
    generateForm.group_id = null
    generateForm.validity_days = 30
    generateForm.expiry_option = 'never'
    generateForm.custom_expiry_days = 7
    loadCodes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToGenerate'))
    console.error('Error generating codes:', error)
  } finally {
    generating.value = false
  }
}

const copyToClipboard = async (text: string) => {
  const success = await clipboardCopy(text, t('admin.redeem.copied'))
  if (success) {
    copiedCode.value = text
    setTimeout(() => {
      copiedCode.value = null
    }, 2000)
  }
}

const handleExportCodes = async () => {
  try {
    const blob = await adminAPI.redeem.exportCodes(buildRedeemQueryFilters())

    // Create download link
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `redeem-codes-${new Date().toISOString().split('T')[0]}.csv`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)

    appStore.showSuccess(t('admin.redeem.codesExported'))
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToExport'))
    console.error('Error exporting codes:', error)
  }
}

const handleDelete = (code: RedeemCode) => {
  deletingCode.value = code
  showDeleteDialog.value = true
}

// Dialogs sit below the drawer layer, so close the drawer before confirming.
const deleteFromDrawer = (code: RedeemCode) => {
  closeDetail()
  handleDelete(code)
}

const confirmDelete = async () => {
  if (!deletingCode.value) return

  try {
    await adminAPI.redeem.delete(deletingCode.value.id)
    appStore.showSuccess(t('admin.redeem.codeDeleted'))
    showDeleteDialog.value = false
    deletingCode.value = null
    loadCodes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToDelete'))
    console.error('Error deleting code:', error)
  }
}

const confirmDeleteUnused = async () => {
  try {
    // Get all unused codes and delete them
    const unusedCodesResponse = await adminAPI.redeem.list(1, 1000, { status: 'unused' })
    const unusedCodeIds = unusedCodesResponse.items.map((code) => code.id)

    if (unusedCodeIds.length === 0) {
      appStore.showInfo(t('admin.redeem.noUnusedCodes'))
      showDeleteUnusedDialog.value = false
      return
    }

    const result = await adminAPI.redeem.batchDelete(unusedCodeIds)
    appStore.showSuccess(t('admin.redeem.codesDeleted', { count: result.deleted }))
    showDeleteUnusedDialog.value = false
    loadCodes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToDeleteUnused'))
    console.error('Error deleting unused codes:', error)
  }
}

const handleBatchUpdate = async () => {
  const ids = Array.from(selectedCodeIds.value)
  if (ids.length === 0) {
    appStore.showInfo(t('admin.redeem.selectCodesFirst'))
    return
  }

  const hasSelectedFields =
    batchUpdateForm.update_status ||
    batchUpdateForm.update_expires_at ||
    batchUpdateForm.update_notes ||
    batchUpdateForm.update_group_id
  if (!hasSelectedFields) {
    appStore.showError(t('admin.redeem.noBatchFieldsSelected'))
    return
  }

  const fields = buildBatchUpdateFields()
  if (!fields) {
    return
  }

  batchUpdating.value = true
  try {
    const result = await adminAPI.redeem.batchUpdate(ids, fields)
    appStore.showSuccess(t('admin.redeem.batchUpdateSuccess', { count: result.updated }))
    showBatchUpdateDialog.value = false
    clearSelectedCodes()
    loadCodes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToBatchUpdate'))
    console.error('Error batch updating codes:', error)
  } finally {
    batchUpdating.value = false
  }
}

// 加载订阅类型分组
const loadSubscriptionGroups = async () => {
  try {
    const groups = await adminAPI.groups.getAll()
    subscriptionGroups.value = groups
  } catch (error) {
    console.error('Error loading subscription groups:', error)
  }
}

onMounted(() => {
  loadCodes()
  loadSubscriptionGroups()
})

onUnmounted(() => {
  clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>

<style scoped>
.redeem-top {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

/* Keep the segmented status control inside narrow viewports. */
.ui-toolbar > .viz-seg {
  max-width: 100%;
  overflow-x: auto;
  scrollbar-width: none;
}

.redeem-menu-divider {
  height: 1px;
  margin: 0.25rem 0;
  background: var(--operator-border);
}

.dropdown-item.is-danger,
.redeem-danger {
  color: var(--operator-destructive);
}

.dropdown-item:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.redeem-bulkbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.625rem 0.5rem 0.875rem;
  border: 1px solid rgb(var(--operator-accent-rgb) / 0.35);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-accent-soft);
}

.redeem-bulkbar > .ui-grow {
  flex: 1 1 auto;
}

.redeem-bulkbar-count {
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.redeem-code {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
}

.redeem-code code {
  color: var(--operator-foreground);
  font-family: var(--operator-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 0.8125rem;
  letter-spacing: 0.01em;
}

.redeem-copy {
  display: inline-grid;
  width: 1.625rem;
  height: 1.625rem;
  place-items: center;
  border-radius: var(--operator-radius-sm);
  color: var(--operator-subtle-foreground);
  transition:
    background-color var(--operator-duration-fast) var(--operator-ease-out),
    color var(--operator-duration-fast) var(--operator-ease-out),
    transform 120ms var(--operator-ease-out);
}

.redeem-copy:hover {
  background: var(--operator-raised);
  color: var(--operator-foreground);
}

.redeem-copy:active {
  transform: scale(0.92);
}

.redeem-copy:focus-visible {
  outline: 2px solid rgb(var(--operator-accent-rgb) / 0.6);
  outline-offset: 1px;
}

.redeem-copy.is-copied {
  color: var(--operator-success);
}

.redeem-value {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.redeem-value-group {
  display: inline-flex;
  align-items: center;
  gap: 0.3125rem;
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
  font-weight: 400;
}

.redeem-muted {
  color: var(--operator-muted-foreground);
  font-size: 0.875rem;
  white-space: nowrap;
}

.redeem-muted.is-expired,
.redeem-expired {
  color: var(--operator-destructive);
}

.redeem-row-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.125rem;
}

.redeem-drawer-icon {
  display: inline-grid;
  width: 2.25rem;
  height: 2.25rem;
  flex: none;
  place-items: center;
  border-radius: 0.625rem;
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
}

.redeem-hero {
  display: flex;
  flex-direction: column;
}

.redeem-hero .ui-section-label {
  margin-bottom: 0.25rem;
}

.redeem-hero b {
  color: var(--operator-foreground);
  font-size: 1.75rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.02em;
  line-height: 1.1;
}
</style>
