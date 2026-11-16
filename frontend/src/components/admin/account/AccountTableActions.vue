<template>
  <div class="acct-actions">
    <slot name="before"></slot>
    <div class="acct-refresh">
      <button
        type="button"
        class="app-icon-button"
        :disabled="loading"
        :aria-label="t('common.refresh')"
        :title="t('common.refresh')"
        @click="$emit('refresh')"
      >
        <Icon name="refresh" size="sm" :class="[loading ? 'animate-spin' : '']" />
      </button>
      <slot name="after"></slot>
    </div>
    <slot name="beforeCreate"></slot>
    <button type="button" class="btn btn-primary acct-add" @click="$emit('create')">
      <Icon name="plus" size="sm" />
      <span>{{ t('common.add') }}</span>
    </button>
    <slot name="afterCreate"></slot>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

defineProps(['loading'])
defineEmits(['refresh', 'create'])

const { t } = useI18n()
</script>

<style scoped>
.acct-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  /* When the toolbar wraps, the actions stay right-aligned under it. */
  margin-left: auto;
}

.acct-refresh {
  display: inline-flex;
  height: 2.25rem;
  align-items: center;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-card);
}

.acct-refresh :deep(.app-icon-button) {
  height: calc(2.25rem - 2px);
  border-radius: 0;
}

.acct-refresh > :deep(:first-child),
.acct-refresh > :deep(:first-child .app-icon-button) {
  border-top-left-radius: calc(var(--operator-radius) - 1px);
  border-bottom-left-radius: calc(var(--operator-radius) - 1px);
}

.acct-refresh > :deep(:last-child),
.acct-refresh > :deep(:last-child .app-icon-button) {
  border-top-right-radius: calc(var(--operator-radius) - 1px);
  border-bottom-right-radius: calc(var(--operator-radius) - 1px);
}

.acct-add {
  height: 2.25rem;
  gap: 0.375rem;
}
</style>
