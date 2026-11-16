<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import Icon from '@/components/icons/Icon.vue'
import type { CustomEndpoint } from '@/types'

const props = defineProps<{
  apiBaseUrl: string
  customEndpoints: CustomEndpoint[]
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const copiedEndpoint = ref<string | null>(null)

let copiedResetTimer: number | undefined

const allEndpoints = computed(() => {
  const items: Array<{ name: string; endpoint: string; description: string; isDefault: boolean }> = []
  if (props.apiBaseUrl) {
    items.push({
      name: t('keys.endpoints.title'),
      endpoint: props.apiBaseUrl,
      description: '',
      isDefault: true,
    })
  }
  for (const ep of props.customEndpoints) {
    items.push({ ...ep, isDefault: false })
  }
  return items
})

async function copy(url: string) {
  const success = await copyToClipboard(url, t('keys.endpoints.copied'))
  if (!success) return

  copiedEndpoint.value = url
  if (copiedResetTimer !== undefined) {
    window.clearTimeout(copiedResetTimer)
  }
  copiedResetTimer = window.setTimeout(() => {
    if (copiedEndpoint.value === url) {
      copiedEndpoint.value = null
    }
  }, 1800)
}

function tooltipHint(endpoint: string): string {
  return copiedEndpoint.value === endpoint
    ? t('keys.endpoints.copiedHint')
    : t('keys.endpoints.clickToCopy')
}

function speedTestUrl(endpoint: string): string {
  return `https://www.tcptest.cn/http/${encodeURIComponent(endpoint)}`
}

onBeforeUnmount(() => {
  if (copiedResetTimer !== undefined) {
    window.clearTimeout(copiedResetTimer)
  }
})
</script>

<template>
  <ul v-if="allEndpoints.length > 0" class="endpoint-list">
    <li v-for="(item, index) in allEndpoints" :key="index" class="endpoint-row">
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-1.5">
          <span class="truncate text-xs font-medium text-gray-600 dark:text-gray-300">{{ item.name }}</span>
          <span v-if="item.isDefault" class="badge badge-primary">{{ t('keys.endpoints.default') }}</span>
        </div>
        <code
          class="endpoint-url"
          role="button"
          tabindex="0"
          @click="copy(item.endpoint)"
          @keydown.enter.prevent="copy(item.endpoint)"
          @keydown.space.prevent="copy(item.endpoint)"
        >{{ item.endpoint }}</code>
        <p v-if="item.description" class="endpoint-desc">{{ item.description }}</p>
        <p class="sr-only" aria-live="polite">{{ tooltipHint(item.endpoint) }}</p>
      </div>
      <button
        type="button"
        class="app-icon-button endpoint-action"
        :class="copiedEndpoint === item.endpoint ? 'is-copied' : ''"
        :aria-label="tooltipHint(item.endpoint)"
        :title="tooltipHint(item.endpoint)"
        @click="copy(item.endpoint)"
      >
        <Icon :name="copiedEndpoint === item.endpoint ? 'check' : 'copy'" size="sm" />
      </button>
      <a
        :href="speedTestUrl(item.endpoint)"
        target="_blank"
        rel="noopener noreferrer"
        class="app-icon-button endpoint-action"
        :aria-label="t('keys.endpoints.speedTest')"
        :title="t('keys.endpoints.speedTest')"
      >
        <Icon name="bolt" size="sm" />
      </a>
    </li>
  </ul>
</template>

<style scoped>
.endpoint-list {
  display: flex;
  min-width: 0;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius-lg);
}

.endpoint-row {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.25rem;
  padding: 0.5rem 0.375rem 0.5rem 0.75rem;
}

.endpoint-row + .endpoint-row {
  border-top: 1px solid var(--operator-border-subtle);
}

.endpoint-url {
  display: block;
  overflow: hidden;
  border-radius: var(--operator-radius-sm);
  color: var(--operator-foreground);
  cursor: pointer;
  font-family: theme('fontFamily.mono');
  font-size: 0.8125rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.endpoint-url:hover,
.endpoint-url:focus-visible {
  color: var(--operator-accent-text);
  outline: none;
  text-decoration: underline dashed;
  text-underline-offset: 3px;
}

.endpoint-desc {
  overflow-wrap: anywhere;
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.endpoint-action {
  width: 1.75rem;
  height: 1.75rem;
  flex: none;
}

.endpoint-action.is-copied {
  color: var(--operator-success-fill);
}

.endpoint-action.is-copied :deep(svg) {
  animation: ui-pop 320ms var(--operator-ease-out);
}
</style>
