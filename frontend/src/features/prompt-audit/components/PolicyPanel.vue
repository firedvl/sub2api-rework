<template>
  <section aria-labelledby="prompt-policy-title" class="viz-grid">
    <h2 id="prompt-policy-title" class="sr-only">{{ t('admin.promptAudit.policy.title') }}</h2>

    <div class="viz-card viz-span-8">
      <header class="viz-card-head">
        <h3 class="viz-card-title flex items-center">
          {{ t('admin.promptAudit.policy.scope') }}
          <HelpTooltip :content="t('admin.promptAudit.policy.description')" />
        </h3>
        <div class="viz-card-tools">
          <div class="viz-seg" role="group" :aria-label="t('admin.promptAudit.policy.scope')">
            <button type="button" :aria-pressed="draft.all_groups" data-test="scope-all" @click="patch({ all_groups: true, group_ids: [] })">
              {{ t('admin.promptAudit.policy.allGroups') }}
            </button>
            <button type="button" :aria-pressed="!draft.all_groups" data-test="scope-selected" @click="patch({ all_groups: false })">
              {{ t('admin.promptAudit.policy.selectedGroups') }}
            </button>
          </div>
        </div>
      </header>
      <div class="viz-card-body space-y-5">
        <div v-if="!draft.all_groups" class="space-y-3">
          <div class="flex flex-wrap items-center gap-3">
            <label class="ui-search">
              <Icon name="search" size="sm" />
              <input v-model="groupSearch" type="search" :placeholder="t('admin.promptAudit.policy.searchGroups')" :aria-label="t('admin.promptAudit.policy.searchGroups')" />
            </label>
            <span class="ui-cell-sub">{{ t('admin.promptAudit.policy.selectedCount', { count: draft.group_ids.length }) }}</span>
          </div>
          <div class="max-h-56 overflow-y-auto rounded-[var(--operator-radius)] border border-[var(--operator-border)] p-1.5">
            <label v-for="group in filteredGroups" :key="group.id" class="pa-group-row">
              <input type="checkbox" :checked="draft.group_ids.includes(group.id)" @change="toggleGroup(group.id)" />
              <ProviderMark v-if="group.platform" :platform="group.platform" :size="16" :label="group.platform" />
              <span class="min-w-0 flex-1 truncate">{{ group.name }}</span>
              <span :class="group.status === 'active' ? 'ui-dot is-ok' : 'ui-dot'" :title="group.status" />
            </label>
            <p v-if="filteredGroups.length === 0" class="px-2 py-4 text-center text-sm text-gray-500">{{ t('admin.promptAudit.policy.noGroups') }}</p>
          </div>
          <p v-if="missingGroupIds.length" class="pa-warn">
            <Icon name="exclamationTriangle" size="sm" />
            {{ t('admin.promptAudit.policy.missingGroups') }}: {{ missingGroupIds.join(', ') }}
          </p>
        </div>

        <fieldset>
          <legend class="ui-section-label">{{ t('admin.promptAudit.policy.scanners') }}</legend>
          <div class="ui-chips flex-wrap">
            <button
              v-for="scanner in SCANNER_CATALOG"
              :key="scanner.id"
              type="button"
              class="ui-chip"
              :aria-pressed="draft.scanners.includes(scanner.id)"
              :aria-label="scannerLabel(scanner.id)"
              :title="t(`admin.promptAudit.scannerDescriptions.${scanner.id}`)"
              @click="toggleScanner(scanner.id)"
            >
              <Icon :name="draft.scanners.includes(scanner.id) ? 'checkCircle' : 'plus'" size="xs" />
              {{ scannerLabel(scanner.id) }}
            </button>
          </div>
        </fieldset>
      </div>
    </div>

    <div class="viz-card viz-span-4">
      <header class="viz-card-head">
        <h3 class="viz-card-title">{{ t('admin.adminUi.promptAudit.workers') }}</h3>
      </header>
      <div class="viz-card-body space-y-4">
        <label class="block text-xs font-medium text-gray-600 dark:text-dark-300">
          <span>{{ t('admin.promptAudit.policy.workerCount') }}</span>
          <input :value="draft.worker_count" type="number" min="1" max="32" class="input mt-1 w-full" :aria-label="t('admin.promptAudit.policy.workerCount')" @input="patch({ worker_count: Number(($event.target as HTMLInputElement).value) })" />
        </label>
        <label class="block text-xs font-medium text-gray-600 dark:text-dark-300">
          <span>{{ t('admin.promptAudit.policy.queueCapacity') }}</span>
          <input :value="draft.queue_capacity" type="number" min="1" max="100000" class="input mt-1 w-full" :aria-label="t('admin.promptAudit.policy.queueCapacity')" @input="patch({ queue_capacity: Number(($event.target as HTMLInputElement).value) })" />
        </label>
        <div class="text-sm">
          <p class="ui-section-label mb-1 flex items-center">
            {{ t('admin.promptAudit.policy.strategy') }}
            <HelpTooltip :content="t('admin.promptAudit.policy.strategyHint')" />
          </p>
          <span class="badge badge-gray font-mono">priority</span>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import type { PromptAuditDraft, PromptAuditGroup } from '../types'
import { cloneData, SCANNER_CATALOG } from '../viewModel'

const props = defineProps<{ draft: PromptAuditDraft; groups: PromptAuditGroup[] }>()
const emit = defineEmits<{ (event: 'update:draft', value: PromptAuditDraft): void }>()
const { t } = useI18n()
const groupSearch = ref('')

const filteredGroups = computed(() => {
  const query = groupSearch.value.trim().toLowerCase()
  if (!query) return props.groups
  return props.groups.filter((group) => `${group.name} ${group.id} ${group.platform}`.toLowerCase().includes(query))
})
const knownGroupIds = computed(() => new Set(props.groups.map((group) => group.id)))
const missingGroupIds = computed(() => props.draft.group_ids.filter((id) => !knownGroupIds.value.has(id)))

function patch(value: Partial<PromptAuditDraft>) {
  emit('update:draft', { ...cloneData(props.draft), ...value })
}
function toggleGroup(id: number) {
  const selected = new Set(props.draft.group_ids)
  if (selected.has(id)) selected.delete(id)
  else selected.add(id)
  patch({ group_ids: [...selected].sort((a, b) => a - b) })
}
function toggleScanner(id: string) {
  const selected = new Set(props.draft.scanners)
  if (selected.has(id)) selected.delete(id)
  else selected.add(id)
  patch({ scanners: SCANNER_CATALOG.map((item) => item.id).filter((item) => selected.has(item)) })
}
function scannerLabel(id: string): string {
  return t(`admin.promptAudit.scanners.${id}`)
}
</script>

<style scoped>
.pa-group-row {
  display: flex;
  min-width: 0;
  cursor: pointer;
  align-items: center;
  gap: 0.625rem;
  padding: 0.5rem 0.625rem;
  border-radius: var(--operator-radius-sm, 0.375rem);
  font-size: 0.875rem;
}

.pa-group-row:hover {
  background: var(--operator-raised);
}

.pa-warn {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--operator-warning);
  font-size: 0.8125rem;
}
</style>
