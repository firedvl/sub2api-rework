<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="displayedAnnouncement"
        class="modal-overlay z-[120] items-start overflow-y-auto pt-[8vh]"
      >
        <div
          class="modal-content max-w-[42rem]"
          role="dialog"
          aria-modal="true"
          aria-labelledby="announcement-popup-title"
          @click.stop
        >
          <div class="modal-header items-start">
            <div class="min-w-0">
              <div class="mb-2 flex items-center gap-2">
                <span class="badge badge-warning">
                  <Icon name="bell" size="xs" />
                  {{ t('announcements.title') }}
                </span>
                <span class="badge badge-primary">{{ t('announcements.unread') }}</span>
              </div>
              <h2 id="announcement-popup-title" class="text-lg font-semibold leading-snug text-gray-900 dark:text-white">
                {{ displayedAnnouncement.title }}
              </h2>
              <p class="mt-1 flex items-center gap-1.5 text-sm text-gray-500 dark:text-gray-400">
                <Icon name="clock" size="sm" />
                <time>{{ formatRelativeWithDateTime(displayedAnnouncement.created_at) }}</time>
              </p>
            </div>
          </div>

          <div class="modal-body max-h-[50vh] px-6 py-5">
            <div
              class="markdown-body prose prose-sm max-w-none dark:prose-invert"
              v-html="renderedContent"
            ></div>
          </div>

          <div class="modal-footer">
            <button
              type="button"
              data-testid="announcement-popup-dismiss"
              class="btn btn-primary"
              @click="handleDismiss"
            >
              <Icon :name="preview ? 'x' : 'check'" size="sm" />
              {{ preview ? t('common.close') : t('announcements.markRead') }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useAnnouncementStore } from '@/stores/announcements'
import { formatRelativeWithDateTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'
import type { Announcement, UserAnnouncement } from '@/types'
import '@/styles/announcement-markdown.css'

type PreviewAnnouncement = Pick<Announcement | UserAnnouncement, 'title' | 'content' | 'created_at'>

const props = withDefaults(defineProps<{
  announcement?: PreviewAnnouncement | null
  preview?: boolean
}>(), {
  announcement: null,
  preview: false,
})

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const announcementStore = useAnnouncementStore()
const displayedAnnouncement = computed(() => (
  props.preview ? props.announcement : announcementStore.currentPopup
))

marked.setOptions({
  breaks: true,
  gfm: true,
})

const renderedContent = computed(() => {
  const content = displayedAnnouncement.value?.content
  if (!content) return ''
  const html = marked.parse(content) as string
  return DOMPurify.sanitize(html)
})

function handleDismiss() {
  if (props.preview) {
    emit('close')
    return
  }
  announcementStore.dismissPopup()
}

// Manage body overflow — only set, never unset (bell component handles restore)
watch(
  displayedAnnouncement,
  (popup) => {
    if (popup) {
      document.body.style.overflow = 'hidden'
    } else if (props.preview) {
      document.body.style.overflow = ''
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  if (props.preview) {
    document.body.style.overflow = ''
  }
})
</script>
