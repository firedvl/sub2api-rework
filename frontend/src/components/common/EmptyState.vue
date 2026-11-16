<template>
  <div class="ui-empty empty-state">
    <span class="ui-empty-icon" aria-hidden="true">
      <slot name="icon">
        <Icon v-if="typeof icon === 'string'" :name="icon as any" size="lg" />
        <component :is="icon" v-else-if="icon" class="h-6 w-6" />
        <Icon v-else name="inbox" size="lg" />
      </slot>
    </span>

    <b class="empty-state-title">{{ displayTitle }}</b>

    <p v-if="description" class="empty-state-description">{{ description }}</p>

    <div v-if="actionText || $slots.action" class="empty-state-action">
      <slot name="action">
        <component
          :is="actionTo ? 'RouterLink' : 'button'"
          v-if="actionText"
          :to="actionTo"
          :type="actionTo ? undefined : 'button'"
          class="btn btn-primary btn-sm"
          @click="!actionTo && $emit('action')"
        >
          <Icon v-if="actionIcon" name="plus" size="sm" />
          {{ actionText }}
        </component>
      </slot>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Component } from 'vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

interface Props {
  /** A component, or an `Icon` name such as "search" or "users". */
  icon?: Component | string
  title?: string
  description?: string
  actionText?: string
  actionTo?: string | object
  actionIcon?: boolean
  message?: string
}

const props = withDefaults(defineProps<Props>(), {
  description: '',
  actionIcon: true
})

const displayTitle = computed(() => props.title || t('common.noData'))

defineEmits(['action'])
</script>

<style scoped>
.empty-state-description {
  max-width: 22rem;
  margin-top: -0.25rem;
  color: var(--operator-subtle-foreground);
  font-size: 0.8125rem;
  line-height: 1.25rem;
}

.empty-state-action {
  margin-top: 0.375rem;
}
</style>
