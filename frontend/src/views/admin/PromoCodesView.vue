<template>
  <AppLayout>
    <div class="ui-page ui-stagger">
      <div class="ui-toolbar">
        <label class="ui-search">
          <Icon name="search" size="sm" />
          <input
            v-model="searchQuery"
            type="search"
            :placeholder="t('admin.promo.searchCodes')"
            :aria-label="t('common.search')"
            @input="handleSearch"
          />
        </label>
        <div class="viz-seg" role="group" :aria-label="t('admin.promo.status')">
          <button
            v-for="option in filterStatusOptions"
            :key="option.value"
            type="button"
            :data-test="`promo-status-${option.value || 'all'}`"
            :aria-pressed="filters.status === option.value"
            @click="setStatusFilter(option.value)"
          >
            {{ option.value === '' ? t('common.all') : option.label }}
            <span v-if="filters.status === option.value && !loading" class="ui-chip-count">{{ pagination.total }}</span>
          </button>
        </div>
        <span class="ui-grow" />
        <button
          type="button"
          class="app-icon-button"
          :aria-label="t('common.refresh')"
          :title="t('common.refresh')"
          :disabled="loading"
          @click="loadCodes"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        </button>
        <button type="button" class="btn btn-primary" @click="showCreateDialog = true">
          <Icon name="plus" size="sm" />
          {{ t('admin.promo.createCode') }}
        </button>
      </div>

      <section class="viz-card promo-card">
        <DataTable
          :columns="columns"
          :data="codes"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          clickable-rows
          row-key="id"
          @sort="handleSort"
          @row-click="openDetail"
        >
          <template #cell-code="{ value, row }">
            <div class="flex items-center gap-1">
              <button type="button" class="promo-code" :title="t('admin.adminUi.details')" @click.stop="openDetail(row)">
                {{ value }}
              </button>
              <button
                type="button"
                class="row-action promo-copy"
                :class="copiedCode === value ? 'is-copied' : ''"
                :aria-label="copiedCode === value ? t('admin.promo.copied') : t('keys.copyToClipboard')"
                :title="copiedCode === value ? t('admin.promo.copied') : t('keys.copyToClipboard')"
                @click.stop="copyToClipboard(value)"
              >
                <Icon :name="copiedCode === value ? 'check' : 'copy'" size="sm" />
              </button>
            </div>
          </template>

          <template #cell-bonus_amount="{ value }">
            <span class="font-medium tabular-nums text-gray-900 dark:text-white">${{ value.toFixed(2) }}</span>
          </template>

          <template #cell-usage="{ row }">
            <div class="promo-usage">
              <span class="tabular-nums">{{ row.used_count }} / {{ row.max_uses === 0 ? '∞' : row.max_uses }}</span>
              <div v-if="row.max_uses > 0" :class="usageMeterClass(row)" role="img" :aria-label="`${usagePercent(row)}%`">
                <div class="ui-meter-track"><i :style="{ width: `${Math.max(usagePercent(row), 2)}%` }"></i></div>
              </div>
            </div>
          </template>

          <template #cell-status="{ value, row }">
            <span :class="['badge', getStatusClass(value, row)]">{{ getStatusLabel(value, row) }}</span>
          </template>

          <template #cell-expires_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ value ? formatDateTime(value) : t('admin.promo.neverExpires') }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-0.5" @click.stop>
              <button type="button" class="row-action" :aria-label="t('admin.promo.copyRegisterLink')" :title="t('admin.promo.copyRegisterLink')" @click="copyRegisterLink(row)">
                <Icon name="link" size="sm" />
              </button>
              <button type="button" class="row-action" :aria-label="t('common.edit')" :title="t('common.edit')" @click="handleEdit(row)">
                <Icon name="edit" size="sm" />
              </button>
              <ToolbarPopover :label="t('admin.adminUi.moreActions')" icon="more" icon-only menu>
                <button type="button" role="menuitem" class="dropdown-item" @click="handleViewUsages(row)">
                  <Icon name="eye" size="sm" />{{ t('admin.promo.viewUsages') }}
                </button>
                <button type="button" role="menuitem" class="dropdown-item promo-danger" @click="handleDelete(row)">
                  <Icon name="trash" size="sm" />{{ t('common.delete') }}
                </button>
              </ToolbarPopover>
            </div>
          </template>

          <template #empty>
            <div class="ui-empty">
              <span class="ui-empty-icon"><Icon name="ticket" size="md" /></span>
              <b>{{ searchQuery || filters.status ? t('admin.adminUi.noMatches') : t('admin.promo.noCodesYet') }}</b>
              <button v-if="!searchQuery && !filters.status" type="button" class="btn btn-primary btn-sm" @click="showCreateDialog = true">
                <Icon name="plus" size="sm" />{{ t('admin.promo.createCode') }}
              </button>
            </div>
          </template>
        </DataTable>

        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </section>
    </div>

    <SideDrawer
      :open="currentViewingCode !== null"
      :title="currentViewingCode?.code ?? ''"
      :subtitle="currentViewingCode ? `$${currentViewingCode.bonus_amount.toFixed(2)}` : ''"
      width="30rem"
      @close="closeDetail"
    >
      <template #leading>
        <span class="promo-drawer-icon"><Icon name="ticket" size="md" /></span>
      </template>
      <template v-if="currentViewingCode" #badge>
        <span :class="['badge', getStatusClass(currentViewingCode.status, currentViewingCode)]">
          {{ getStatusLabel(currentViewingCode.status, currentViewingCode) }}
        </span>
      </template>

      <div v-if="currentViewingCode" class="promo-drawer">
        <section>
          <div class="ui-section-label">{{ t('admin.promo.columns.usage') }}</div>
          <div class="promo-usage is-large">
            <span class="tabular-nums">
              <b>{{ currentViewingCode.used_count }}</b> / {{ currentViewingCode.max_uses === 0 ? '∞' : currentViewingCode.max_uses }}
            </span>
            <div v-if="currentViewingCode.max_uses > 0" :class="usageMeterClass(currentViewingCode)">
              <div class="ui-meter-track"><i :style="{ width: `${Math.max(usagePercent(currentViewingCode), 2)}%` }"></i></div>
              <b>{{ usagePercent(currentViewingCode) }}%</b>
            </div>
          </div>
        </section>

        <dl class="ui-dl">
          <dt>{{ t('admin.promo.columns.bonusAmount') }}</dt>
          <dd class="tabular-nums">${{ currentViewingCode.bonus_amount.toFixed(2) }}</dd>
          <dt>{{ t('admin.promo.columns.expiresAt') }}</dt>
          <dd>{{ currentViewingCode.expires_at ? formatDateTime(currentViewingCode.expires_at) : t('admin.promo.neverExpires') }}</dd>
          <dt>{{ t('admin.promo.columns.createdAt') }}</dt>
          <dd>{{ formatDateTime(currentViewingCode.created_at) }}</dd>
          <dt>{{ t('admin.adminUi.promoCodes.updatedAt') }}</dt>
          <dd>{{ formatDateTime(currentViewingCode.updated_at) }}</dd>
          <template v-if="currentViewingCode.notes">
            <dt>{{ t('admin.promo.notes') }}</dt>
            <dd class="whitespace-pre-wrap">{{ currentViewingCode.notes }}</dd>
          </template>
        </dl>

        <section data-test="promo-usage-records">
          <div class="ui-section-label">
            {{ t('admin.promo.usageRecords') }}
            <span v-if="usagesTotal > 0" class="ml-1 tabular-nums">{{ usagesTotal }}</span>
          </div>
          <div v-if="usagesLoading" class="space-y-2">
            <div v-for="n in 3" :key="n" class="skeleton h-12 rounded-lg"></div>
          </div>
          <div v-else-if="usages.length === 0" class="ui-empty promo-empty">
            <span class="ui-empty-icon"><Icon name="users" size="md" /></span>
            <b>{{ t('admin.promo.noUsages') }}</b>
          </div>
          <template v-else>
            <ul class="promo-usages">
              <li v-for="usage in usages" :key="usage.id">
                <span class="promo-usage-avatar"><Icon name="user" size="sm" /></span>
                <div class="min-w-0 flex-1">
                  <div class="ui-cell-main truncate">{{ usage.user?.email || t('admin.promo.userPrefix', { id: usage.user_id }) }}</div>
                  <div class="ui-cell-sub">{{ formatDateTime(usage.used_at) }}</div>
                </div>
                <span class="promo-usage-bonus">+${{ usage.bonus_amount.toFixed(2) }}</span>
              </li>
            </ul>
            <Pagination
              v-if="usagesTotal > usagesPageSize"
              :page="usagesPage"
              :total="usagesTotal"
              :page-size="usagesPageSize"
              @update:page="handleUsagesPageChange"
              @update:page-size="(size: number) => { usagesPageSize = size; usagesPage = 1; loadUsages() }"
            />
          </template>
        </section>
      </div>

      <template v-if="currentViewingCode" #footer>
        <button type="button" class="btn btn-primary" @click="editFromDetail">
          <Icon name="edit" size="sm" />{{ t('common.edit') }}
        </button>
        <button type="button" class="btn btn-secondary" @click="copyRegisterLink(currentViewingCode)">
          <Icon name="link" size="sm" />{{ t('admin.promo.copyRegisterLink') }}
        </button>
        <span class="flex-1"></span>
        <button type="button" class="btn btn-ghost promo-danger" @click="deleteFromDetail">
          <Icon name="trash" size="sm" />{{ t('common.delete') }}
        </button>
      </template>
    </SideDrawer>

    <!-- Create Dialog -->
    <BaseDialog
      :show="showCreateDialog"
      :title="t('admin.promo.createCode')"
      width="normal"
      @close="showCreateDialog = false"
    >
      <form id="create-promo-form" @submit.prevent="handleCreate" class="space-y-4">
        <div>
          <label class="input-label">
            {{ t('admin.promo.code') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('admin.promo.autoGenerate') }})</span>
          </label>
          <input
            v-model="createForm.code"
            type="text"
            class="input font-mono uppercase"
            :placeholder="t('admin.promo.codePlaceholder')"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.promo.bonusAmount') }}</label>
          <input
            v-model.number="createForm.bonus_amount"
            type="number"
            step="0.01"
            min="0"
            required
            class="input"
          />
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.promo.maxUses') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('admin.promo.zeroUnlimited') }})</span>
          </label>
          <input
            v-model.number="createForm.max_uses"
            type="number"
            min="0"
            class="input"
          />
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.promo.expiresAt') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <input
            v-model="createForm.expires_at_str"
            type="datetime-local"
            class="input"
          />
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.promo.notes') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <textarea
            v-model="createForm.notes"
            rows="2"
            class="input"
            :placeholder="t('admin.promo.notesPlaceholder')"
          ></textarea>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" @click="showCreateDialog = false" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" form="create-promo-form" :disabled="creating" class="btn btn-primary">
            {{ creating ? t('common.creating') : t('common.create') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Edit Dialog -->
    <BaseDialog
      :show="showEditDialog"
      :title="t('admin.promo.editCode')"
      width="normal"
      @close="closeEditDialog"
    >
      <form id="edit-promo-form" @submit.prevent="handleUpdate" class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.promo.code') }}</label>
          <input
            v-model="editForm.code"
            type="text"
            class="input font-mono uppercase"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.promo.bonusAmount') }}</label>
          <input
            v-model.number="editForm.bonus_amount"
            type="number"
            step="0.01"
            min="0"
            required
            class="input"
          />
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.promo.maxUses') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('admin.promo.zeroUnlimited') }})</span>
          </label>
          <input
            v-model.number="editForm.max_uses"
            type="number"
            min="0"
            class="input"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.promo.status') }}</label>
          <Select v-model="editForm.status" :options="statusOptions" />
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.promo.expiresAt') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <input
            v-model="editForm.expires_at_str"
            type="datetime-local"
            class="input"
          />
        </div>
        <div>
          <label class="input-label">
            {{ t('admin.promo.notes') }}
            <span class="ml-1 text-xs font-normal text-gray-400">({{ t('common.optional') }})</span>
          </label>
          <textarea
            v-model="editForm.notes"
            rows="2"
            class="input"
          ></textarea>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" @click="closeEditDialog" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" form="edit-promo-form" :disabled="updating" class="btn btn-primary">
            {{ updating ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>


    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.promo.deleteCode')"
      :message="t('admin.promo.deleteCodeConfirm')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import { formatDateTime, formatDateTimeLocalInput } from '@/utils/format'
import type { PromoCode, PromoCodeUsage } from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import ToolbarPopover from '@/components/common/ToolbarPopover.vue'
import SideDrawer from '@/components/ui/SideDrawer.vue'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard: clipboardCopy } = useClipboard()

// State
const codes = ref<PromoCode[]>([])
const loading = ref(false)
const creating = ref(false)
const updating = ref(false)
const searchQuery = ref('')
const copiedCode = ref<string | null>(null)

const filters = reactive({
  status: ''
})

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0
})
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

// Dialogs
const showCreateDialog = ref(false)
const showEditDialog = ref(false)
const showDeleteDialog = ref(false)

const editingCode = ref<PromoCode | null>(null)
const deletingCode = ref<PromoCode | null>(null)

// Usages
const usages = ref<PromoCodeUsage[]>([])
const usagesLoading = ref(false)
const currentViewingCode = ref<PromoCode | null>(null)
const usagesPage = ref(1)
const usagesPageSize = ref(20)
const usagesTotal = ref(0)

// Forms
const createForm = reactive({
  code: '',
  bonus_amount: 1,
  max_uses: 0,
  expires_at_str: '',
  notes: ''
})

const editForm = reactive({
  code: '',
  bonus_amount: 0,
  max_uses: 0,
  status: 'active' as 'active' | 'disabled',
  expires_at_str: '',
  notes: ''
})

// Options
const filterStatusOptions = computed(() => [
  { value: '', label: t('admin.promo.allStatus') },
  { value: 'active', label: t('admin.promo.statusActive') },
  { value: 'disabled', label: t('admin.promo.statusDisabled') }
])

const statusOptions = computed(() => [
  { value: 'active', label: t('admin.promo.statusActive') },
  { value: 'disabled', label: t('admin.promo.statusDisabled') }
])

const columns = computed<Column[]>(() => [
  { key: 'code', label: t('admin.promo.columns.code') },
  { key: 'bonus_amount', label: t('admin.promo.columns.bonusAmount'), sortable: true },
  { key: 'usage', label: t('admin.promo.columns.usage') },
  { key: 'status', label: t('admin.promo.columns.status'), sortable: true },
  { key: 'expires_at', label: t('admin.promo.columns.expiresAt'), sortable: true },
  { key: 'created_at', label: t('admin.promo.columns.createdAt'), sortable: true },
  { key: 'actions', label: t('admin.promo.columns.actions') }
])

// Helpers
const getStatusClass = (status: string, row: PromoCode) => {
  if (row.expires_at && new Date(row.expires_at) < new Date()) {
    return 'badge-danger'
  }
  if (row.max_uses > 0 && row.used_count >= row.max_uses) {
    return 'badge-gray'
  }
  return status === 'active' ? 'badge-success' : 'badge-gray'
}

const usagePercent = (row: PromoCode) => (row.max_uses > 0 ? Math.min(100, Math.round((row.used_count / row.max_uses) * 100)) : 0)

// Literal meter classes so Tailwind keeps them: amber near the limit, red when used up.
const usageMeterClass = (row: PromoCode) => {
  const value = usagePercent(row)
  if (value >= 100) return 'ui-meter promo-meter is-bad'
  if (value >= 80) return 'ui-meter promo-meter is-warn'
  return 'ui-meter promo-meter'
}

const getStatusLabel = (status: string, row: PromoCode) => {
  if (row.expires_at && new Date(row.expires_at) < new Date()) {
    return t('admin.promo.statusExpired')
  }
  if (row.max_uses > 0 && row.used_count >= row.max_uses) {
    return t('admin.promo.statusMaxUsed')
  }
  return status === 'active' ? t('admin.promo.statusActive') : t('admin.promo.statusDisabled')
}

// API calls
let abortController: AbortController | null = null

const loadCodes = async () => {
  if (abortController) {
    abortController.abort()
  }
  const currentController = new AbortController()
  abortController = currentController
  loading.value = true

  try {
    const response = await adminAPI.promo.list(
      pagination.page,
      pagination.page_size,
      {
        status: filters.status || undefined,
        search: searchQuery.value || undefined,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      },
      { signal: currentController.signal }
    )
    if (currentController.signal.aborted || abortController !== currentController) return

    codes.value = response.items
    pagination.total = response.total
  } catch (error: any) {
    if (
      currentController.signal.aborted ||
      abortController !== currentController ||
      error?.name === 'AbortError' ||
      error?.code === 'ERR_CANCELED'
    ) {
      return
    }
    appStore.showError(t('admin.promo.failedToLoad'))
    console.error('Error loading promo codes:', error)
  } finally {
    if (abortController === currentController) {
      loading.value = false
      abortController = null
    }
  }
}

let searchTimeout: ReturnType<typeof setTimeout>
const handleSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadCodes()
  }, 300)
}

const setStatusFilter = (status: string) => {
  if (filters.status === status) return
  filters.status = status
  pagination.page = 1
  loadCodes()
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

const copyToClipboard = async (text: string) => {
  const success = await clipboardCopy(text, t('admin.promo.copied'))
  if (success) {
    copiedCode.value = text
    setTimeout(() => {
      copiedCode.value = null
    }, 2000)
  }
}

// Create
const handleCreate = async () => {
  creating.value = true
  try {
    await adminAPI.promo.create({
      code: createForm.code || undefined,
      bonus_amount: createForm.bonus_amount,
      max_uses: createForm.max_uses,
      expires_at: createForm.expires_at_str ? Math.floor(new Date(createForm.expires_at_str).getTime() / 1000) : undefined,
      notes: createForm.notes || undefined
    })
    appStore.showSuccess(t('admin.promo.codeCreated'))
    showCreateDialog.value = false
    resetCreateForm()
    loadCodes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.promo.failedToCreate'))
  } finally {
    creating.value = false
  }
}

const resetCreateForm = () => {
  createForm.code = ''
  createForm.bonus_amount = 1
  createForm.max_uses = 0
  createForm.expires_at_str = ''
  createForm.notes = ''
}

// Edit
const handleEdit = (code: PromoCode) => {
  editingCode.value = code
  editForm.code = code.code
  editForm.bonus_amount = code.bonus_amount
  editForm.max_uses = code.max_uses
  editForm.status = code.status
  editForm.expires_at_str = code.expires_at
    ? formatDateTimeLocalInput(Math.floor(new Date(code.expires_at).getTime() / 1000))
    : ''
  editForm.notes = code.notes || ''
  showEditDialog.value = true
}

const closeEditDialog = () => {
  showEditDialog.value = false
  editingCode.value = null
}

const handleUpdate = async () => {
  if (!editingCode.value) return

  updating.value = true
  try {
    await adminAPI.promo.update(editingCode.value.id, {
      code: editForm.code,
      bonus_amount: editForm.bonus_amount,
      max_uses: editForm.max_uses,
      status: editForm.status,
      expires_at: editForm.expires_at_str ? Math.floor(new Date(editForm.expires_at_str).getTime() / 1000) : 0,
      notes: editForm.notes
    })
    appStore.showSuccess(t('admin.promo.codeUpdated'))
    closeEditDialog()
    loadCodes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.promo.failedToUpdate'))
  } finally {
    updating.value = false
  }
}

// Copy Register Link
const copyRegisterLink = async (code: PromoCode) => {
  const baseUrl = window.location.origin
  const registerLink = `${baseUrl}/register?promo=${encodeURIComponent(code.code)}`

  try {
    await navigator.clipboard.writeText(registerLink)
    appStore.showSuccess(t('admin.promo.registerLinkCopied'))
  } catch (error) {
    // Fallback for older browsers
    const textArea = document.createElement('textarea')
    textArea.value = registerLink
    document.body.appendChild(textArea)
    textArea.select()
    document.execCommand('copy')
    document.body.removeChild(textArea)
    appStore.showSuccess(t('admin.promo.registerLinkCopied'))
  }
}

// Delete
const handleDelete = (code: PromoCode) => {
  deletingCode.value = code
  showDeleteDialog.value = true
}

const confirmDelete = async () => {
  if (!deletingCode.value) return

  try {
    await adminAPI.promo.delete(deletingCode.value.id)
    appStore.showSuccess(t('admin.promo.codeDeleted'))
    showDeleteDialog.value = false
    deletingCode.value = null
    loadCodes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.promo.failedToDelete'))
  }
}

// Detail drawer (shows the code's usage records)
const handleViewUsages = async (code: PromoCode) => {
  currentViewingCode.value = code
  usagesPage.value = 1
  usagesTotal.value = 0
  await loadUsages()
}

const openDetail = (code: PromoCode) => {
  void handleViewUsages(code)
}

const closeDetail = () => {
  currentViewingCode.value = null
  usages.value = []
}

const editFromDetail = () => {
  const code = currentViewingCode.value
  if (!code) return
  closeDetail()
  handleEdit(code)
}

const deleteFromDetail = () => {
  const code = currentViewingCode.value
  if (!code) return
  closeDetail()
  handleDelete(code)
}

const loadUsages = async () => {
  if (!currentViewingCode.value) return
  usagesLoading.value = true
  usages.value = []

  try {
    const response = await adminAPI.promo.getUsages(
      currentViewingCode.value.id,
      usagesPage.value,
      usagesPageSize.value
    )
    usages.value = response.items
    usagesTotal.value = response.total
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.promo.failedToLoadUsages'))
  } finally {
    usagesLoading.value = false
  }
}

const handleUsagesPageChange = (page: number) => {
  usagesPage.value = page
  loadUsages()
}

onMounted(() => {
  loadCodes()
})

onUnmounted(() => {
  clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>

<style scoped>
.promo-card {
  overflow: hidden;
}

.promo-code {
  border-radius: var(--operator-radius-sm);
  color: var(--operator-foreground);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.8125rem;
  font-weight: 600;
  letter-spacing: 0.02em;
}

.promo-code:hover {
  color: var(--operator-accent-text);
}

.promo-code:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: 2px;
}

.promo-copy {
  width: 1.75rem;
  height: 1.75rem;
}

.promo-copy.is-copied {
  color: var(--operator-success);
}

.promo-usage {
  display: flex;
  min-width: 7rem;
  align-items: center;
  gap: 0.625rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.promo-meter {
  min-width: 3.5rem;
  flex: 1 1 auto;
}

.promo-usage.is-large {
  font-size: 0.9375rem;
}

.promo-usage.is-large b {
  color: var(--operator-foreground);
  font-size: 1.375rem;
  font-weight: 600;
}

.promo-danger {
  color: var(--operator-destructive);
}

.promo-drawer {
  display: grid;
  gap: 1.25rem;
}

.promo-drawer-icon {
  display: grid;
  width: 2.25rem;
  height: 2.25rem;
  flex: none;
  place-items: center;
  border-radius: var(--operator-radius);
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
}

.promo-empty {
  padding-block: 1.5rem;
}

.promo-usages {
  display: grid;
  margin-bottom: 0.5rem;
}

.promo-usages li {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.5rem 0;
  border-bottom: 1px solid var(--operator-border);
}

.promo-usages li:last-child {
  border-bottom: 0;
}

.promo-usage-avatar {
  display: grid;
  width: 2rem;
  height: 2rem;
  flex: none;
  place-items: center;
  border-radius: 999px;
  background: var(--operator-raised);
  color: var(--operator-muted-foreground);
}

.promo-usage-bonus {
  color: var(--operator-success);
  font-size: 0.8125rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
</style>
