<template>
  <AppLayout>
    <div class="ui-page ui-stagger announcements-page">
      <div class="ui-toolbar" role="toolbar" :aria-label="t('admin.adminUi.filters')">
        <label class="ui-search">
          <Icon name="search" size="sm" />
          <input
            v-model="searchQuery"
            type="search"
            :placeholder="t('admin.announcements.searchAnnouncements')"
            :aria-label="t('admin.announcements.searchAnnouncements')"
            @input="handleSearch"
          />
        </label>

        <div class="viz-seg" role="group" :aria-label="t('admin.announcements.status')">
          <button
            v-for="option in statusSegments"
            :key="option.value"
            type="button"
            :aria-pressed="filters.status === option.value"
            :data-test="`announcement-status-${option.value || 'all'}`"
            @click="setStatusFilter(option.value)"
          >
            {{ option.label }}
            <span v-if="option.count !== null" class="ann-seg-count">{{ option.count }}</span>
          </button>
        </div>

        <span class="ui-grow" />

        <button
          type="button"
          class="app-icon-button"
          :disabled="loading"
          :title="t('common.refresh')"
          :aria-label="t('common.refresh')"
          @click="reloadAll"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        </button>

        <button type="button" class="btn btn-primary" @click="openCreateDialog">
          <Icon name="plus" size="sm" />
          <span>{{ t('admin.announcements.createAnnouncement') }}</span>
        </button>
      </div>

      <section class="ui-card ann-card">
        <DataTable
          :columns="columns"
          :data="announcements"
          :loading="loading"
          :server-side-sort="true"
          :clickable-rows="true"
          row-key="id"
          default-sort-key="created_at"
          default-sort-order="desc"
          @sort="handleSort"
          @row-click="openDetail"
        >
          <template #cell-title="{ value, row }">
            <button
              type="button"
              class="ann-title"
              :aria-label="`${t('admin.adminUi.details')}: ${value}`"
              @click.stop="openDetail(row)"
            >
              <span class="ann-title-main">
                <Icon
                  v-if="row.notify_mode === 'popup'"
                  name="bell"
                  size="xs"
                  class="ann-popup-icon"
                  :aria-label="t('admin.announcements.notifyModeLabels.popup')"
                  :title="t('admin.announcements.notifyModeLabels.popup')"
                />
                <span class="ui-cell-main">{{ value }}</span>
              </span>
              <span class="ui-cell-sub">{{ excerpt(row.content) || `#${row.id}` }}</span>
            </button>
          </template>

          <template #cell-status="{ value }">
            <span :class="statusBadgeClass(value)">{{ statusLabel(value) }}</span>
          </template>

          <template #cell-targeting="{ row }">
            <span class="ann-target">
              <Icon :name="hasCustomTargeting(row.targeting) ? 'filter' : 'users'" size="xs" />
              {{ targetingSummary(row.targeting) }}
            </span>
          </template>

          <template #cell-timeRange="{ row }">
            <span class="ann-window" :title="scheduleTitle(row)">
              <span :class="windowDotClass(row)" />
              <span class="ann-window-text">
                {{ row.starts_at ? formatDateTime(row.starts_at) : t('admin.announcements.timeImmediate') }}
                <Icon name="arrowRight" size="xs" class="ann-window-arrow" />
                {{ row.ends_at ? formatDateTime(row.ends_at) : t('admin.announcements.timeNever') }}
              </span>
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="ann-muted">{{ formatDateTime(value) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="ann-row-actions" @click.stop>
              <button
                type="button"
                class="ann-row-action"
                :title="t('common.edit')"
                :aria-label="`${t('common.edit')} ${row.title}`"
                @click.stop="openEditDialog(row)"
              >
                <Icon name="edit" size="sm" />
              </button>
              <ToolbarPopover
                :label="`${t('admin.adminUi.moreActions')} ${row.title}`"
                icon="more"
                icon-only
                menu
                width="12rem"
              >
                <button type="button" role="menuitem" class="dropdown-item" @click="openReadStatus(row)">
                  <Icon name="chartBar" size="sm" />
                  {{ t('admin.announcements.readStatus') }}
                </button>
                <button type="button" role="menuitem" class="dropdown-item" @click="openPreview(row)">
                  <Icon name="eye" size="sm" />
                  {{ t('admin.announcements.preview') }}
                </button>
                <button type="button" role="menuitem" class="dropdown-item ann-danger" @click="handleDelete(row)">
                  <Icon name="trash" size="sm" />
                  {{ t('common.delete') }}
                </button>
              </ToolbarPopover>
            </div>
          </template>

          <template #empty>
            <div class="ui-empty">
              <span class="ui-empty-icon"><Icon name="bell" size="md" /></span>
              <b>{{ searchQuery || filters.status ? t('admin.adminUi.noMatches') : t('admin.announcements.createFirstAnnouncement') }}</b>
              <button v-if="searchQuery || filters.status" type="button" class="btn btn-secondary btn-sm" @click="clearAllQuery">
                {{ t('admin.adminUi.clearFilters') }}
              </button>
              <button v-else type="button" class="btn btn-primary btn-sm" @click="openCreateDialog">
                <Icon name="plus" size="sm" />
                {{ t('admin.announcements.createAnnouncement') }}
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
      :open="!!detailAnnouncement"
      :title="detailAnnouncement?.title || ''"
      :subtitle="detailAnnouncement ? `#${detailAnnouncement.id} · ${formatDateTime(detailAnnouncement.created_at)}` : ''"
      width="34rem"
      @close="closeDetail"
    >
      <template #leading>
        <span class="ann-drawer-icon" :class="detailAnnouncement?.notify_mode === 'popup' ? 'is-popup' : ''" aria-hidden="true">
          <Icon :name="detailAnnouncement?.notify_mode === 'popup' ? 'bell' : 'inbox'" size="md" />
        </span>
      </template>
      <template #badge>
        <span v-if="detailAnnouncement" :class="statusBadgeClass(detailAnnouncement.status)">
          {{ statusLabel(detailAnnouncement.status) }}
        </span>
      </template>

      <template v-if="detailAnnouncement">
        <article class="ann-preview">
          <div class="markdown-body prose prose-sm max-w-none dark:prose-invert" v-html="detailContent" />
        </article>

        <section>
          <h3 class="ui-section-label">{{ t('admin.announcements.columns.timeRange') }}</h3>
          <dl class="ui-dl">
            <dt>{{ t('admin.announcements.form.startsAt') }}</dt>
            <dd>{{ detailAnnouncement.starts_at ? formatDateTime(detailAnnouncement.starts_at) : t('admin.announcements.timeImmediate') }}</dd>
            <dt>{{ t('admin.announcements.form.endsAt') }}</dt>
            <dd>{{ detailAnnouncement.ends_at ? formatDateTime(detailAnnouncement.ends_at) : t('admin.announcements.timeNever') }}</dd>
            <dt>{{ t('admin.announcements.columns.notifyMode') }}</dt>
            <dd>{{ detailAnnouncement.notify_mode === 'popup' ? t('admin.announcements.notifyModeLabels.popup') : t('admin.announcements.notifyModeLabels.silent') }}</dd>
            <dt>{{ t('admin.adminUi.announcements.updated') }}</dt>
            <dd>{{ formatDateTime(detailAnnouncement.updated_at) }}</dd>
          </dl>
        </section>

        <section>
          <h3 class="ui-section-label">{{ t('admin.announcements.columns.targeting') }}</h3>
          <p v-if="!hasCustomTargeting(detailAnnouncement.targeting)" class="ann-target">
            <Icon name="users" size="xs" />
            {{ t('admin.announcements.targetingSummaryAll') }}
          </p>
          <ol v-else class="ann-rules">
            <li v-for="(group, groupIndex) in detailAnnouncement.targeting.any_of" :key="groupIndex" class="ann-rule">
              <span v-if="groupIndex > 0" class="ann-rule-join">{{ t('admin.adminUi.announcements.or') }}</span>
              <span class="ann-rule-chips">
                <template v-for="(condition, conditionIndex) in group.all_of || []" :key="conditionIndex">
                  <span v-if="conditionIndex > 0" class="ann-rule-and">+</span>
                  <span class="ann-chip">{{ conditionLabel(condition) }}</span>
                </template>
              </span>
            </li>
          </ol>
        </section>

        <button type="button" class="ann-read-entry" @click="runFromDrawer(openReadStatus)">
          <Icon name="chartBar" size="sm" />
          <span>{{ t('admin.announcements.readStatus') }}</span>
          <Icon name="chevronRight" size="sm" class="ann-read-chevron" />
        </button>
      </template>

      <template #footer>
        <template v-if="detailAnnouncement">
          <button type="button" class="btn btn-primary btn-sm" @click="runFromDrawer(openEditDialog)">
            <Icon name="edit" size="sm" />
            {{ t('common.edit') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="runFromDrawer(openPreview)">
            <Icon name="eye" size="sm" />
            {{ t('admin.announcements.preview') }}
          </button>
          <span class="flex-1" />
          <button type="button" class="btn btn-ghost btn-sm ann-danger" @click="runFromDrawer(handleDelete)">
            <Icon name="trash" size="sm" />
            {{ t('common.delete') }}
          </button>
        </template>
      </template>
    </SideDrawer>

    <!-- Create/Edit Dialog -->
    <BaseDialog
      :show="showEditDialog"
      :title="isEditing ? t('admin.announcements.editAnnouncement') : t('admin.announcements.createAnnouncement')"
      width="wide"
      @close="closeEdit"
    >
      <form id="announcement-form" @submit.prevent="handleSave" class="space-y-4">
        <div>
          <label class="input-label">{{ t('admin.announcements.form.title') }}</label>
          <input v-model="form.title" type="text" class="input" required />
        </div>

        <div>
          <label class="input-label">{{ t('admin.announcements.form.content') }}</label>
          <textarea v-model="form.content" rows="6" class="input" required></textarea>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.announcements.form.status') }}</label>
            <Select v-model="form.status" :options="statusOptions" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.announcements.form.notifyMode') }}</label>
            <Select v-model="form.notify_mode" :options="notifyModeOptions" />
            <p class="input-hint">{{ t('admin.announcements.form.notifyModeHint') }}</p>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.announcements.form.startsAt') }}</label>
            <input v-model="form.starts_at_str" type="datetime-local" max="9999-12-31T23:59" class="input" />
            <p class="input-hint">{{ t('admin.announcements.form.startsAtHint') }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.announcements.form.endsAt') }}</label>
            <input v-model="form.ends_at_str" type="datetime-local" max="9999-12-31T23:59" class="input" />
            <p class="input-hint">{{ t('admin.announcements.form.endsAtHint') }}</p>
          </div>
        </div>

        <AnnouncementTargetingEditor
          v-model="form.targeting"
          :groups="subscriptionGroups"
        />
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" @click="closeEdit" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" form="announcement-form" :disabled="saving" class="btn btn-primary">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Delete Confirmation -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.announcements.deleteAnnouncement')"
      :message="t('admin.announcements.deleteConfirm')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- Read Status Dialog -->
    <AnnouncementReadStatusDialog
      :show="showReadStatusDialog"
      :announcement-id="readStatusAnnouncementId"
      @close="showReadStatusDialog = false"
    />

    <AnnouncementPopup
      :announcement="previewAnnouncement"
      preview
      @close="previewAnnouncement = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import { formatDateTime, formatDateTimeLocalInput, parseDateTimeLocalInput } from '@/utils/format'
import type { AdminGroup, Announcement, AnnouncementCondition, AnnouncementStatus, AnnouncementTargeting } from '@/types'
import type { Column } from '@/components/common/types'

import { marked } from 'marked'
import DOMPurify from 'dompurify'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import ToolbarPopover from '@/components/common/ToolbarPopover.vue'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import Icon from '@/components/icons/Icon.vue'

import AnnouncementTargetingEditor from '@/components/admin/announcements/AnnouncementTargetingEditor.vue'
import AnnouncementReadStatusDialog from '@/components/admin/announcements/AnnouncementReadStatusDialog.vue'
import AnnouncementPopup from '@/components/common/AnnouncementPopup.vue'
import '@/styles/announcement-markdown.css'

const { t } = useI18n()
const appStore = useAppStore()

const announcements = ref<Announcement[]>([])
const loading = ref(false)

type StatusFilter = '' | AnnouncementStatus
const STATUS_FILTERS: AnnouncementStatus[] = ['active', 'draft', 'archived']

const filters = reactive({
  status: '' as StatusFilter,
})
// Per-status totals for the status segment (same search scope as the list).
const statusCounts = reactive<Record<AnnouncementStatus, number | null>>({
  active: null,
  draft: null,
  archived: null
})
const searchQuery = ref('')

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

const statusSegments = computed(() => {
  const known = STATUS_FILTERS.every((status) => statusCounts[status] !== null)
  return [
    {
      value: '' as StatusFilter,
      label: t('common.all'),
      count: known ? STATUS_FILTERS.reduce((sum, status) => sum + (statusCounts[status] ?? 0), 0) : null
    },
    ...STATUS_FILTERS.map((status) => ({ value: status as StatusFilter, label: statusLabel(status), count: statusCounts[status] }))
  ]
})

const statusOptions = computed(() => [
  { value: 'draft', label: t('admin.announcements.statusLabels.draft') },
  { value: 'active', label: t('admin.announcements.statusLabels.active') },
  { value: 'archived', label: t('admin.announcements.statusLabels.archived') }
])

const notifyModeOptions = computed(() => [
  { value: 'silent', label: t('admin.announcements.notifyModeLabels.silent') },
  { value: 'popup', label: t('admin.announcements.notifyModeLabels.popup') }
])

// Notify mode shows as a bell beside popup titles; full detail lives in the drawer.
const columns = computed<Column[]>(() => [
  { key: 'title', label: t('admin.announcements.columns.title'), sortable: true, class: 'ann-col-title' },
  { key: 'status', label: t('admin.announcements.columns.status'), sortable: true },
  { key: 'timeRange', label: t('admin.announcements.columns.timeRange') },
  { key: 'targeting', label: t('admin.announcements.columns.targeting') },
  { key: 'created_at', label: t('admin.announcements.columns.createdAt'), sortable: true },
  { key: 'actions', label: t('admin.announcements.columns.actions') }
])

const statusBadgeClass = (status: string) => {
  if (status === 'active') return 'badge badge-success'
  if (status === 'draft') return 'badge badge-gray'
  return 'badge badge-warning'
}

// Plain-text excerpt of the markdown body for the row subtitle.
const excerpt = (content: string) =>
  (content || '')
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/[#>*_`~|]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 140)

type WindowState = 'live' | 'upcoming' | 'ended'
const windowState = (row: Announcement): WindowState => {
  const now = Date.now()
  if (row.starts_at && new Date(row.starts_at).getTime() > now) return 'upcoming'
  if (row.ends_at && new Date(row.ends_at).getTime() <= now) return 'ended'
  return 'live'
}
const WINDOW_DOT: Record<WindowState, string> = {
  live: 'ui-dot is-ok',
  upcoming: 'ui-dot is-warn',
  ended: 'ui-dot'
}
const windowDotClass = (row: Announcement) => WINDOW_DOT[windowState(row)]
const scheduleTitle = (row: Announcement) => {
  const state = windowState(row)
  if (state === 'upcoming') return t('admin.adminUi.announcements.windowUpcoming')
  if (state === 'ended') return t('admin.adminUi.announcements.windowEnded')
  return t('admin.adminUi.announcements.windowLive')
}

const statusLabel = (status: string) => {
  if (status === 'draft') return t('admin.announcements.statusLabels.draft')
  if (status === 'active') return t('admin.announcements.statusLabels.active')
  if (status === 'archived') return t('admin.announcements.statusLabels.archived')
  return status
}

const hasCustomTargeting = (targeting?: AnnouncementTargeting) => (targeting?.any_of ?? []).length > 0

const groupNames = computed(() => new Map(subscriptionGroups.value.map((group) => [group.id, group.name])))
const conditionLabel = (condition: AnnouncementCondition) => {
  if (condition.type === 'subscription') {
    const names = (condition.group_ids ?? []).map((id) => groupNames.value.get(id) ?? `#${id}`)
    return `${t('admin.announcements.form.conditionSubscription')}: ${names.join(', ') || '—'}`
  }
  const operator = ['gt', 'gte', 'lt', 'lte', 'eq'].includes(condition.operator)
    ? t(`admin.announcements.operators.${condition.operator}`)
    : condition.operator
  return `${t('admin.announcements.form.conditionBalance')} ${operator} ${condition.value ?? 0}`
}

const targetingSummary = (targeting: AnnouncementTargeting) => {
  const anyOf = targeting?.any_of ?? []
  if (!anyOf || anyOf.length === 0) return t('admin.announcements.targetingSummaryAll')
  return t('admin.announcements.targetingSummaryCustom', { groups: anyOf.length })
}

// ===== CRUD / list =====
let currentController: AbortController | null = null

async function loadAnnouncements() {
  currentController?.abort()
  const requestController = new AbortController()
  currentController = requestController
  const { signal } = requestController

  try {
    loading.value = true
    const res = await adminAPI.announcements.list(pagination.page, pagination.page_size, {
      status: filters.status || undefined,
      search: searchQuery.value || undefined,
      sort_by: sortState.sort_by,
      sort_order: sortState.sort_order
    }, { signal })

    if (signal.aborted || currentController !== requestController) return

    announcements.value = res.items
    pagination.total = res.total
    pagination.pages = res.pages
    pagination.page = res.page
    pagination.page_size = res.page_size
  } catch (error: any) {
    if (
      signal.aborted ||
      currentController !== requestController ||
      error?.name === 'AbortError' ||
      error?.code === 'ERR_CANCELED'
    ) {
      return
    }
    console.error('Error loading announcements:', error)
    appStore.showError(error.response?.data?.detail || t('admin.announcements.failedToLoad'))
  } finally {
    if (currentController === requestController) {
      loading.value = false
      currentController = null
    }
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  loadAnnouncements()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadAnnouncements()
}

let countsRequestId = 0
async function loadStatusCounts() {
  const requestId = ++countsRequestId
  try {
    const totals = await Promise.all(
      STATUS_FILTERS.map((status) =>
        adminAPI.announcements.list(1, 1, { status, search: searchQuery.value || undefined })
      )
    )
    if (requestId !== countsRequestId) return
    STATUS_FILTERS.forEach((status, index) => {
      statusCounts[status] = totals[index]?.total ?? null
    })
  } catch (error) {
    if (requestId === countsRequestId) {
      STATUS_FILTERS.forEach((status) => { statusCounts[status] = null })
    }
    console.error('Error loading announcement status counts:', error)
  }
}

// Reload the list and the status counts (scope or data changed).
function reloadAll() {
  loadStatusCounts()
  return loadAnnouncements()
}

function setStatusFilter(status: StatusFilter) {
  if (filters.status === status) return
  filters.status = status
  pagination.page = 1
  loadAnnouncements()
}

function clearAllQuery() {
  if (searchDebounceTimer) window.clearTimeout(searchDebounceTimer)
  searchQuery.value = ''
  filters.status = ''
  pagination.page = 1
  reloadAll()
}

function handleSort(key: string, order: 'asc' | 'desc') {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadAnnouncements()
}

let searchDebounceTimer: number | null = null
function handleSearch() {
  if (searchDebounceTimer) window.clearTimeout(searchDebounceTimer)
  searchDebounceTimer = window.setTimeout(() => {
    pagination.page = 1
    reloadAll()
  }, 300)
}

// ===== Create/Edit dialog =====
const showEditDialog = ref(false)
const saving = ref(false)
const editingAnnouncement = ref<Announcement | null>(null)

const isEditing = computed(() => !!editingAnnouncement.value)

const form = reactive({
  title: '',
  content: '',
  status: 'draft',
  notify_mode: 'silent',
  starts_at_str: '',
  ends_at_str: '',
  targeting: { any_of: [] } as AnnouncementTargeting
})

const subscriptionGroups = ref<AdminGroup[]>([])

async function loadSubscriptionGroups() {
  try {
    const all = await adminAPI.groups.getAll()
    subscriptionGroups.value = (all || []).filter((g) => g.subscription_type === 'subscription')
  } catch (error: any) {
    console.error('Error loading groups:', error)
    // not fatal
  }
}

function resetForm() {
  form.title = ''
  form.content = ''
  form.status = 'draft'
  form.notify_mode = 'silent'
  form.starts_at_str = ''
  form.ends_at_str = ''
  form.targeting = { any_of: [] }
}

function fillFormFromAnnouncement(a: Announcement) {
  form.title = a.title
  form.content = a.content
  form.status = a.status
  form.notify_mode = a.notify_mode || 'silent'

  // Backend returns RFC3339 strings
  form.starts_at_str = a.starts_at ? formatDateTimeLocalInput(Math.floor(new Date(a.starts_at).getTime() / 1000)) : ''
  form.ends_at_str = a.ends_at ? formatDateTimeLocalInput(Math.floor(new Date(a.ends_at).getTime() / 1000)) : ''

  form.targeting = a.targeting ?? { any_of: [] }
}

function openCreateDialog() {
  editingAnnouncement.value = null
  resetForm()
  showEditDialog.value = true
}

function openEditDialog(row: Announcement) {
  editingAnnouncement.value = row
  fillFormFromAnnouncement(row)
  showEditDialog.value = true
}

function closeEdit() {
  showEditDialog.value = false
  editingAnnouncement.value = null
}

function buildCreatePayload() {
  const startsAt = parseDateTimeLocalInput(form.starts_at_str)
  const endsAt = parseDateTimeLocalInput(form.ends_at_str)

  return {
    title: form.title,
    content: form.content,
    status: form.status as any,
    notify_mode: form.notify_mode as any,
    targeting: form.targeting,
    starts_at: startsAt ?? undefined,
    ends_at: endsAt ?? undefined
  }
}

function buildUpdatePayload(original: Announcement) {
  const payload: any = {}

  if (form.title !== original.title) payload.title = form.title
  if (form.content !== original.content) payload.content = form.content
  if (form.status !== original.status) payload.status = form.status
  if (form.notify_mode !== (original.notify_mode || 'silent')) payload.notify_mode = form.notify_mode

  // starts_at / ends_at: distinguish unchanged vs clear(0) vs set
  const originalStarts = original.starts_at ? Math.floor(new Date(original.starts_at).getTime() / 1000) : null
  const originalEnds = original.ends_at ? Math.floor(new Date(original.ends_at).getTime() / 1000) : null

  const newStarts = parseDateTimeLocalInput(form.starts_at_str)
  const newEnds = parseDateTimeLocalInput(form.ends_at_str)

  if (newStarts !== originalStarts) {
    payload.starts_at = newStarts === null ? 0 : newStarts
  }
  if (newEnds !== originalEnds) {
    payload.ends_at = newEnds === null ? 0 : newEnds
  }

  // targeting: do shallow compare by JSON
  if (JSON.stringify(form.targeting ?? {}) !== JSON.stringify(original.targeting ?? {})) {
    payload.targeting = form.targeting
  }

  return payload
}

async function handleSave() {
  // Frontend validation for targeting (to avoid ANNOUNCEMENT_INVALID_TARGET)
  const anyOf = form.targeting?.any_of ?? []
  if (anyOf.length > 50) {
    appStore.showError(t('admin.announcements.failedToCreate'))
    return
  }
  for (const g of anyOf) {
    const allOf = g?.all_of ?? []
    if (allOf.length > 50) {
      appStore.showError(t('admin.announcements.failedToCreate'))
      return
    }
  }

  saving.value = true
  try {
    if (!editingAnnouncement.value) {
      const payload = buildCreatePayload()
      await adminAPI.announcements.create(payload)
      appStore.showSuccess(t('common.success'))
      showEditDialog.value = false
      await reloadAll()
      return
    }

    const original = editingAnnouncement.value
    const payload = buildUpdatePayload(original)
    await adminAPI.announcements.update(original.id, payload)
    appStore.showSuccess(t('common.success'))
    showEditDialog.value = false
    editingAnnouncement.value = null
    await reloadAll()
  } catch (error: any) {
    console.error('Failed to save announcement:', error)
    appStore.showError(error.response?.data?.detail || (editingAnnouncement.value ? t('admin.announcements.failedToUpdate') : t('admin.announcements.failedToCreate')))
  } finally {
    saving.value = false
  }
}

// ===== Delete =====
const showDeleteDialog = ref(false)
const deletingAnnouncement = ref<Announcement | null>(null)

function handleDelete(row: Announcement) {
  deletingAnnouncement.value = row
  showDeleteDialog.value = true
}

async function confirmDelete() {
  if (!deletingAnnouncement.value) return

  try {
    await adminAPI.announcements.delete(deletingAnnouncement.value.id)
    appStore.showSuccess(t('common.success'))
    showDeleteDialog.value = false
    deletingAnnouncement.value = null
    await reloadAll()
  } catch (error: any) {
    console.error('Failed to delete announcement:', error)
    appStore.showError(error.response?.data?.detail || t('admin.announcements.failedToDelete'))
  }
}

// ===== Read status =====
const showReadStatusDialog = ref(false)
const readStatusAnnouncementId = ref<number | null>(null)
const previewAnnouncement = ref<Announcement | null>(null)

function openPreview(row: Announcement) {
  previewAnnouncement.value = row
}

function openReadStatus(row: Announcement) {
  readStatusAnnouncementId.value = row.id
  showReadStatusDialog.value = true
}

// ===== Detail drawer =====
const detailId = ref<number | null>(null)
const detailAnnouncement = computed(() =>
  detailId.value === null ? null : announcements.value.find((a) => a.id === detailId.value) ?? null
)
const detailContent = computed(() => {
  const content = detailAnnouncement.value?.content
  if (!content) return ''
  return DOMPurify.sanitize(marked.parse(content, { breaks: true, gfm: true }) as string)
})

function openDetail(row: Announcement) {
  detailId.value = row.id
}

function closeDetail() {
  detailId.value = null
}

// Dialogs sit below the drawer, so close it before handing off.
function runFromDrawer(action: (row: Announcement) => unknown) {
  const row = detailAnnouncement.value
  if (!row) return
  closeDetail()
  action(row)
}

onMounted(async () => {
  await loadSubscriptionGroups()
  await reloadAll()
})

onUnmounted(() => {
  if (searchDebounceTimer) window.clearTimeout(searchDebounceTimer)
  currentController?.abort()
})
</script>

<style scoped>
.ann-seg-count {
  margin-left: 0.25rem;
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
}

.ann-card {
  overflow: hidden;
}

.ann-card :deep(.ann-col-title) {
  width: 34%;
}

.ann-title {
  display: block;
  width: 100%;
  min-width: 0;
  max-width: 28rem;
  border-radius: var(--operator-radius-sm);
  text-align: left;
}

.ann-title:hover .ui-cell-main {
  color: var(--operator-accent-text);
}

.ann-title:focus-visible,
.ann-row-action:focus-visible,
.ann-read-entry:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: 2px;
}

.ann-title-main {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.375rem;
}

.ann-popup-icon {
  flex: none;
  color: var(--operator-warning);
}

.ann-target,
.ann-window {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 0.4375rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  white-space: nowrap;
}

.ann-window-text {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  font-variant-numeric: tabular-nums;
}

.ann-window-arrow {
  color: var(--operator-subtle-foreground);
}

.ann-muted {
  color: var(--operator-subtle-foreground);
  font-size: 0.8125rem;
  white-space: nowrap;
}

.ann-row-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.125rem;
}

.ann-row-action {
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  align-items: center;
  justify-content: center;
  border-radius: var(--operator-radius);
  color: var(--operator-muted-foreground);
  transition: color var(--operator-duration-fast) var(--operator-ease-out), background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.ann-row-action:hover {
  background: var(--operator-muted);
  color: var(--operator-foreground);
}

.ann-row-actions :deep(.toolbar-popover-icon) {
  border-color: transparent;
  background: transparent;
}

.ann-danger,
.ann-danger:hover {
  color: var(--operator-destructive);
}

/* Drawer */
.ann-drawer-icon {
  display: grid;
  width: 2.5rem;
  height: 2.5rem;
  flex: none;
  place-items: center;
  border-radius: var(--operator-radius-lg);
  background: var(--operator-raised);
  box-shadow: inset 0 0 0 1px var(--operator-border);
  color: var(--operator-muted-foreground);
}

.ann-drawer-icon.is-popup {
  background: rgb(var(--operator-warning-rgb) / 0.12);
  color: var(--operator-warning);
}

.ann-preview {
  max-height: 22rem;
  overflow-y: auto;
  padding: 1rem 1.125rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-card);
}

.ann-preview .markdown-body {
  font-size: 0.875rem;
}

.ann-rules {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.ann-rule {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.ann-rule-join {
  color: var(--operator-subtle-foreground);
  font-size: 0.6875rem;
  font-weight: 600;
  text-transform: uppercase;
}

.ann-rule-chips {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem;
}

.ann-rule-and {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.ann-chip {
  padding: 0.1875rem 0.5rem;
  border: 1px solid var(--operator-border);
  border-radius: 999px;
  background: var(--operator-raised);
  color: var(--operator-foreground);
  font-size: 0.75rem;
  overflow-wrap: anywhere;
}

.ann-read-entry {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 0.625rem;
  padding: 0.75rem 0.875rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius-lg);
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 500;
  text-align: left;
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out), border-color var(--operator-duration-fast) var(--operator-ease-out);
}

.ann-read-entry:hover {
  border-color: var(--operator-border-strong);
  background: var(--operator-muted);
}

.ann-read-chevron {
  margin-left: auto;
  color: var(--operator-subtle-foreground);
  transition: transform var(--operator-duration-fast) var(--operator-ease-out);
}

.ann-read-entry:hover .ann-read-chevron {
  transform: translateX(2px);
}

@media (prefers-reduced-motion: reduce) {
  .ann-read-entry:hover .ann-read-chevron {
    transform: none;
  }
}
</style>
