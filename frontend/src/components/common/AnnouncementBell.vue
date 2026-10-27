<template>
  <div>
    <button
      type="button"
      class="app-icon-button relative"
      :class="{ 'text-gray-900 dark:text-gray-100': unreadCount > 0 }"
      :aria-label="t('announcements.title')"
      @click="openModal"
    >
      <Icon name="bell" size="sm" />
      <span
        v-if="unreadCount > 0"
        class="absolute right-1.5 top-1.5 h-2 w-2 rounded-full bg-red-500 ring-2 ring-[var(--operator-background)]"
        aria-hidden="true"
      ></span>
    </button>

    <!-- Announcement list -->
    <Teleport to="body">
      <Transition name="modal">
        <div
          v-if="isModalOpen"
          class="modal-overlay items-start overflow-y-auto pt-[10vh]"
          @click="closeModal"
        >
          <div
            class="modal-content max-w-[36rem]"
            role="dialog"
            aria-modal="true"
            aria-labelledby="announcement-list-title"
            @click.stop
          >
            <div class="modal-header">
              <div class="min-w-0">
                <h2 id="announcement-list-title" class="modal-title">{{ t('announcements.title') }}</h2>
                <p v-if="unreadCount > 0" class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                  <span class="font-medium text-gray-900 dark:text-white">{{ unreadCount }}</span>
                  {{ t('announcements.unread') }}
                </p>
              </div>
              <div class="flex items-center gap-2">
                <button
                  v-if="unreadCount > 0"
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="loading"
                  @click="markAllAsRead"
                >
                  {{ t('announcements.markAllRead') }}
                </button>
                <button
                  type="button"
                  class="operator-dialog-close inline-flex h-8 w-8 items-center justify-center rounded-md"
                  :aria-label="t('common.closeDialog')"
                  @click="closeModal"
                >
                  <Icon name="x" size="sm" />
                </button>
              </div>
            </div>

            <div class="max-h-[60vh] overflow-y-auto">
              <div v-if="loading" class="flex items-center justify-center py-14">
                <LoadingSpinner />
              </div>

              <ul v-else-if="announcements.length > 0" class="divide-y divide-gray-100 dark:divide-dark-700">
                <li v-for="item in announcements" :key="item.id">
                  <button
                    type="button"
                    class="announcement-row group"
                    :class="{ 'announcement-row-unread': !item.read_at }"
                    @click="openDetail(item)"
                  >
                    <span class="announcement-dot" :class="item.read_at ? 'opacity-0' : ''" aria-hidden="true"></span>
                    <span class="min-w-0 flex-1 text-left">
                      <span
                        class="block truncate text-sm"
                        :class="item.read_at ? 'text-gray-600 dark:text-gray-300' : 'font-medium text-gray-900 dark:text-white'"
                      >
                        {{ item.title }}
                      </span>
                      <span class="mt-0.5 flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
                        <time>{{ formatRelativeTime(item.created_at) }}</time>
                        <span v-if="!item.read_at" class="badge badge-primary">{{ t('announcements.unread') }}</span>
                      </span>
                    </span>
                    <Icon
                      name="chevronRight"
                      size="sm"
                      class="flex-shrink-0 text-gray-400 transition-transform duration-fast ease-ui-out group-hover:translate-x-0.5"
                    />
                  </button>
                </li>
              </ul>

              <div v-else class="empty-state py-12">
                <div class="mb-3 grid h-11 w-11 place-items-center rounded-xl border border-gray-200 bg-gray-50 dark:border-dark-700 dark:bg-dark-800">
                  <Icon name="inbox" size="md" class="text-gray-400" />
                </div>
                <p class="empty-state-title">{{ t('announcements.empty') }}</p>
                <p class="empty-state-description">{{ t('announcements.emptyDescription') }}</p>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- Announcement detail -->
    <Teleport to="body">
      <Transition name="modal">
        <div
          v-if="detailModalOpen && selectedAnnouncement"
          class="modal-overlay z-[110] items-start overflow-y-auto pt-[8vh]"
          @click="closeDetail"
        >
          <div
            class="modal-content max-w-[48rem]"
            role="dialog"
            aria-modal="true"
            aria-labelledby="announcement-detail-title"
            @click.stop
          >
            <div class="modal-header items-start">
              <div class="min-w-0">
                <div class="mb-2 flex items-center gap-2">
                  <span class="badge badge-gray">{{ t('announcements.title') }}</span>
                  <span v-if="!selectedAnnouncement.read_at" class="badge badge-primary">{{ t('announcements.unread') }}</span>
                </div>
                <h2 id="announcement-detail-title" class="text-lg font-semibold leading-snug text-gray-900 dark:text-white">
                  {{ selectedAnnouncement.title }}
                </h2>
                <p class="mt-1 flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400">
                  <Icon name="clock" size="sm" />
                  <time>{{ formatRelativeWithDateTime(selectedAnnouncement.created_at) }}</time>
                </p>
              </div>
              <button
                type="button"
                class="operator-dialog-close inline-flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-md"
                :aria-label="t('common.closeDialog')"
                @click="closeDetail"
              >
                <Icon name="x" size="sm" />
              </button>
            </div>

            <div class="modal-body max-h-[60vh] px-6 py-5">
              <div
                class="markdown-body prose prose-sm max-w-none dark:prose-invert"
                v-html="renderMarkdown(selectedAnnouncement.content)"
              ></div>
            </div>

            <div class="modal-footer justify-between">
              <p class="text-xs text-gray-500 dark:text-gray-400">
                {{ selectedAnnouncement.read_at ? t('announcements.readStatus') : t('announcements.markReadHint') }}
              </p>
              <div class="flex items-center gap-2">
                <button type="button" class="btn btn-secondary" @click="closeDetail">
                  {{ t('common.close') }}
                </button>
                <button
                  v-if="!selectedAnnouncement.read_at"
                  type="button"
                  class="btn btn-primary"
                  @click="markAsReadAndClose(selectedAnnouncement.id)"
                >
                  <Icon name="check" size="sm" />
                  {{ t('announcements.markRead') }}
                </button>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useAppStore } from '@/stores/app'
import { useAnnouncementStore } from '@/stores/announcements'
import { formatRelativeTime, formatRelativeWithDateTime } from '@/utils/format'
import type { UserAnnouncement } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import '@/styles/announcement-markdown.css'

const { t } = useI18n()
const appStore = useAppStore()
const announcementStore = useAnnouncementStore()

// Configure marked
marked.setOptions({
  breaks: true,
  gfm: true,
})

// Use store state (storeToRefs for reactivity)
const { announcements, loading } = storeToRefs(announcementStore)
const unreadCount = computed(() => announcementStore.unreadCount)

// Local modal state
const isModalOpen = ref(false)
const detailModalOpen = ref(false)
const selectedAnnouncement = ref<UserAnnouncement | null>(null)

// Methods
function renderMarkdown(content: string): string {
  if (!content) return ''
  const html = marked.parse(content) as string
  return DOMPurify.sanitize(html)
}

function openModal() {
  isModalOpen.value = true
}

function closeModal() {
  isModalOpen.value = false
}

function openDetail(announcement: UserAnnouncement) {
  selectedAnnouncement.value = announcement
  detailModalOpen.value = true
  if (!announcement.read_at) {
    markAsRead(announcement.id)
  }
}

function closeDetail() {
  detailModalOpen.value = false
  selectedAnnouncement.value = null
}

async function markAsRead(id: number) {
  try {
    await announcementStore.markAsRead(id)
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  }
}

async function markAsReadAndClose(id: number) {
  await markAsRead(id)
  appStore.showSuccess(t('announcements.markedAsRead'))
  closeDetail()
}

async function markAllAsRead() {
  try {
    await announcementStore.markAllAsRead()
    appStore.showSuccess(t('announcements.allMarkedAsRead'))
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  }
}

function handleEscape(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    if (detailModalOpen.value) {
      closeDetail()
    } else if (isModalOpen.value) {
      closeModal()
    }
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleEscape)
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleEscape)
  document.body.style.overflow = ''
})

watch(
  [isModalOpen, detailModalOpen, () => announcementStore.currentPopup],
  ([modal, detail, popup]) => {
    document.body.style.overflow = (modal || detail || popup) ? 'hidden' : ''
  }
)
</script>

<style scoped>
.announcement-row {
  position: relative;
  display: flex;
  width: 100%;
  min-height: 3.75rem;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem 1.25rem;
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.announcement-row:hover {
  background: var(--operator-raised);
}

.announcement-row:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: -2px;
}

.announcement-dot {
  width: 0.5rem;
  height: 0.5rem;
  flex: 0 0 auto;
  border-radius: 999px;
  background: var(--operator-accent);
}
</style>
