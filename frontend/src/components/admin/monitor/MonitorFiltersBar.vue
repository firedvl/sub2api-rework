<template>
  <div class="ui-toolbar monitor-toolbar" role="toolbar" :aria-label="t('admin.routingUi.filters')">
    <label class="ui-search">
      <Icon name="search" size="sm" />
      <input
        v-model="search"
        type="search"
        :placeholder="t('admin.routingUi.search')"
        :aria-label="t('admin.channelMonitor.searchPlaceholder')"
        @input="$emit('search-input')"
      />
    </label>

    <div class="ui-chips monitor-providers" role="group" :aria-label="t('admin.channelMonitor.allProviders')">
      <button
        v-for="option in providerChips"
        :key="option.value"
        type="button"
        class="ui-chip is-icon"
        :aria-pressed="provider === option.value"
        :aria-label="option.label"
        :title="option.label"
        :data-testid="`monitor-filter-${option.value}`"
        @click="setProvider(option.value)"
      >
        <ProviderMark :platform="option.value" :size="20" />
      </button>
    </div>

    <div class="viz-seg" role="group" :aria-label="t('admin.channelMonitor.enabledFilter')">
      <button
        v-for="option in enabledFilterOptions"
        :key="option.value"
        type="button"
        :aria-pressed="enabled === option.value"
        @click="setEnabled(option.value)"
      >
        {{ option.label }}
      </button>
    </div>

    <span class="ui-grow"></span>

    <button
      type="button"
      class="app-icon-button monitor-tool"
      :disabled="loading"
      :aria-label="t('common.refresh')"
      :title="t('common.refresh')"
      @click="$emit('reload')"
    >
      <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
    </button>
    <button
      type="button"
      class="app-icon-button monitor-tool"
      :aria-label="t('admin.channelMonitor.template.manageButton')"
      :title="t('admin.channelMonitor.template.manageButton')"
      @click="$emit('manage-templates')"
    >
      <Icon name="document" size="sm" />
    </button>
    <button type="button" class="btn btn-primary" @click="$emit('create')">
      <Icon name="plus" size="sm" />
      <span>{{ t('admin.routingUi.newMonitor') }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Provider } from '@/api/admin/channelMonitor'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  PROVIDER_OPENAI,
  PROVIDER_ANTHROPIC,
  PROVIDER_GEMINI,
  PROVIDER_GROK,
  PROVIDER_ANTIGRAVITY,
  PROVIDER_KIMI,
  PROVIDER_ZHIPU,
  PROVIDER_DEEPSEEK,
  PROVIDER_MINIMAX,
  PROVIDER_OPENCODE_GO,
} from '@/constants/channelMonitor'

defineProps<{
  loading: boolean
}>()

const emit = defineEmits<{
  (e: 'reload'): void
  (e: 'create'): void
  (e: 'manage-templates'): void
  (e: 'search-input'): void
}>()

const search = defineModel<string>('search', { required: true })
const provider = defineModel<Provider | ''>('provider', { required: true })
const enabled = defineModel<'' | 'true' | 'false'>('enabled', { required: true })

const { t } = useI18n()

const providerChips = computed<{ value: Provider; label: string }[]>(() => [
  { value: PROVIDER_OPENAI, label: t('monitorCommon.providers.openai') },
  { value: PROVIDER_ANTHROPIC, label: t('monitorCommon.providers.anthropic') },
  { value: PROVIDER_GEMINI, label: t('monitorCommon.providers.gemini') },
  { value: PROVIDER_GROK, label: t('monitorCommon.providers.grok') },
  { value: PROVIDER_ANTIGRAVITY, label: t('monitorCommon.providers.antigravity') },
  { value: PROVIDER_KIMI, label: t('monitorCommon.providers.kimi') },
  { value: PROVIDER_ZHIPU, label: t('monitorCommon.providers.zhipu') },
  { value: PROVIDER_DEEPSEEK, label: t('monitorCommon.providers.deepseek') },
  { value: PROVIDER_MINIMAX, label: t('monitorCommon.providers.minimax') },
  { value: PROVIDER_OPENCODE_GO, label: t('monitorCommon.providers.opencode_go') },
])

const enabledFilterOptions = computed<{ value: '' | 'true' | 'false'; label: string }[]>(() => [
  { value: '', label: t('admin.routingUi.all') },
  { value: 'true', label: t('admin.routingUi.on') },
  { value: 'false', label: t('admin.routingUi.off') },
])

// Filters apply immediately; a second click on the active provider clears it.
function setProvider(value: Provider) {
  provider.value = provider.value === value ? '' : value
  emit('reload')
}

function setEnabled(value: '' | 'true' | 'false') {
  if (enabled.value === value) return
  enabled.value = value
  emit('reload')
}
</script>

<style scoped>
.monitor-providers {
  gap: 0.25rem;
}

.monitor-providers .ui-chip.is-icon {
  width: 2.125rem;
  height: 2.125rem;
}

.monitor-providers .ui-chip[aria-pressed='true'] :deep(.provider-mark) {
  animation: ui-pop 320ms var(--operator-spring);
}

.monitor-tool {
  border: 1px solid var(--operator-border);
  background: var(--operator-card);
}

@media (max-width: 760px) {
  .monitor-toolbar .ui-search {
    max-width: none;
    flex-basis: 100%;
  }

  .monitor-providers {
    width: 100%;
    flex-wrap: nowrap;
    overflow-x: auto;
    scrollbar-width: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .monitor-providers .ui-chip[aria-pressed='true'] :deep(.provider-mark) {
    animation: none;
  }
}
</style>
