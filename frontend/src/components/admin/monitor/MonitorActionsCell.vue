<template>
  <div class="flex items-center gap-1">
    <button
      @click="$emit('run', row)"
      :disabled="running"
      class="row-action"
                :title="t('admin.channelMonitor.runNow')"
              >
      <Icon name="refresh" size="sm" :class="running ? 'animate-spin' : ''" />
      <span class="row-action-label">{{ t('admin.channelMonitor.runNow') }}</span>
    </button>
    <button
      data-testid="monitor-duplicate"
      :title="duplicateTitle"
      :disabled="duplicating || Boolean(row.api_key_decrypt_failed)"
      @click="$emit('duplicate', row)"
      class="row-action disabled:cursor-not-allowed disabled:opacity-50"
    >
      <Icon name="copy" size="sm" />
      <span class="row-action-label">
        {{ duplicating ? t('admin.channelMonitor.duplicating') : t('admin.channelMonitor.duplicate') }}
      </span>
    </button>
    <button
      @click="$emit('edit', row)"
      class="row-action"
                :title="t('common.edit')"
              >
      <Icon name="edit" size="sm" />
      <span class="row-action-label">{{ t('common.edit') }}</span>
    </button>
    <button
      @click="$emit('delete', row)"
      class="row-action row-action-danger"
                :title="t('common.delete')"
              >
      <Icon name="trash" size="sm" />
      <span class="row-action-label">{{ t('common.delete') }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  row: ChannelMonitor
  running: boolean
  duplicating: boolean
}>()

defineEmits<{
  (e: 'run', row: ChannelMonitor): void
  (e: 'duplicate', row: ChannelMonitor): void
  (e: 'edit', row: ChannelMonitor): void
  (e: 'delete', row: ChannelMonitor): void
}>()

const { t } = useI18n()
const duplicateTitle = computed(() => {
  if (props.row.api_key_decrypt_failed) return t('admin.channelMonitor.duplicateKeyUnavailable')
  if (props.duplicating) return t('admin.channelMonitor.duplicating')
  return t('admin.channelMonitor.duplicate')
})
</script>
