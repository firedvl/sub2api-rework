<template>
  <div class="monitor-actions">
    <button
      type="button"
      class="app-icon-button"
      :disabled="running"
      :title="t('admin.channelMonitor.runNow')"
      @click="$emit('run', row)"
    >
      <Icon name="play" size="sm" :class="running ? 'animate-pulse' : ''" />
      <span class="sr-only">{{ t('admin.channelMonitor.runNow') }}</span>
    </button>
    <button
      type="button"
      class="app-icon-button"
      data-testid="monitor-duplicate"
      :title="duplicateTitle"
      :disabled="duplicating || Boolean(row.api_key_decrypt_failed)"
      @click="$emit('duplicate', row)"
    >
      <Icon name="copy" size="sm" />
      <span class="sr-only">
        {{ duplicating ? t('admin.channelMonitor.duplicating') : t('admin.channelMonitor.duplicate') }}
      </span>
    </button>
    <button
      type="button"
      class="app-icon-button"
      :title="t('common.edit')"
      @click="$emit('edit', row)"
    >
      <Icon name="edit" size="sm" />
      <span class="sr-only">{{ t('common.edit') }}</span>
    </button>
    <button
      type="button"
      class="app-icon-button monitor-actions-danger"
      :title="t('common.delete')"
      @click="$emit('delete', row)"
    >
      <Icon name="trash" size="sm" />
      <span class="sr-only">{{ t('common.delete') }}</span>
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

<style scoped>
.monitor-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.125rem;
}

.monitor-actions .app-icon-button:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.monitor-actions-danger:hover {
  color: var(--operator-destructive);
}
</style>
