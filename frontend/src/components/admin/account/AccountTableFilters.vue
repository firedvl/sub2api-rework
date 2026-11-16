<template>
  <div class="contents">
    <label class="ui-search acct-search">
      <Icon name="search" size="sm" />
      <input
        type="search"
        :value="searchQuery"
        :placeholder="t('admin.accountsUi.search')"
        :aria-label="t('admin.accountsUi.search')"
        @input="onSearch(($event.target as HTMLInputElement).value)"
        @keydown.esc.stop="searchQuery && onSearch('')"
      />
    </label>

    <slot />

    <div
      v-if="platforms.length > 1 || filters.platform"
      class="ui-chips acct-providers"
      role="group"
      :aria-label="t('admin.accountsUi.providers')"
    >
      <button
        v-for="platform in platforms"
        :key="platform.value"
        type="button"
        class="ui-chip is-icon"
        :aria-pressed="filters.platform === platform.value"
        :aria-label="`${platform.label} (${platform.count})`"
        :title="`${platform.label} · ${platform.count}`"
        :data-testid="`provider-chip-${platform.value}`"
        @click="togglePlatform(platform.value)"
      >
        <ProviderMark :platform="platform.value" :size="20" />
      </button>
    </div>

    <div ref="popoverRoot" class="relative">
      <button
        type="button"
        class="btn btn-secondary acct-filter-trigger"
        :class="{ 'is-active': activeChips.length > 0 }"
        :aria-expanded="open"
        aria-haspopup="dialog"
        @click="open = !open"
      >
        <Icon name="filter" size="sm" />
        <span>{{ t('admin.accountsUi.filters') }}</span>
        <span v-if="activeChips.length" class="acct-filter-count">{{ activeChips.length }}</span>
      </button>
      <Transition name="menu-pop">
        <div
          v-if="open"
          class="operator-menu acct-filter-pop"
          role="dialog"
          :aria-label="t('admin.accountsUi.filters')"
          @keydown.esc.stop="close(true)"
        >
          <div class="acct-filter-grid">
            <label>
              <span>{{ t('admin.accounts.columns.type') }}</span>
              <Select :model-value="filters.type" :options="tOpts" :aria-label="t('admin.accounts.columns.type')" @update:model-value="update('type', $event)" @change="$emit('change')" />
            </label>
            <label v-if="showStatus !== false">
              <span>{{ t('common.status') }}</span>
              <Select :model-value="filters.status" :options="sOpts" :aria-label="t('common.status')" @update:model-value="update('status', $event)" @change="$emit('change')" />
            </label>
            <label>
              <span>{{ t('admin.accountsUi.privacy') }}</span>
              <Select :model-value="filters.privacy_mode" :options="privacyOpts" :aria-label="t('admin.accountsUi.privacy')" @update:model-value="update('privacy_mode', $event)" @change="$emit('change')" />
            </label>
            <label>
              <span>{{ t('admin.accountsUi.autoWarmup') }}</span>
              <Select :model-value="filters.auto_warmup" :options="autoWarmupOpts" :aria-label="t('admin.accountsUi.autoWarmup')" @update:model-value="update('auto_warmup', $event)" @change="$emit('change')" />
            </label>
            <label>
              <span>{{ t('admin.accounts.columns.groups') }}</span>
              <Select :model-value="filters.group" :options="gOpts" :searchable="gOpts.length > 8" :aria-label="t('admin.accounts.columns.groups')" @update:model-value="update('group', $event)" @change="$emit('change')" />
            </label>
            <label v-if="sortOptions?.length">
              <span>{{ t('admin.accountsUi.sort') }}</span>
              <Select :model-value="sortValue ?? null" :options="sortOptions" :aria-label="t('admin.accountsUi.sort')" @update:model-value="$emit('update:sort', String($event))" />
            </label>
          </div>
          <div class="acct-filter-foot">
            <button type="button" class="btn btn-ghost btn-sm" :disabled="!activeChips.length" @click="clearAll">
              {{ t('admin.accountsUi.clearFilters') }}
            </button>
            <button type="button" class="btn btn-secondary btn-sm" @click="close(true)">{{ t('common.close') }}</button>
          </div>
        </div>
      </Transition>
    </div>

    <TransitionGroup v-if="activeChips.length" tag="div" name="ui-list" class="acct-active" :aria-label="t('admin.accountsUi.filters')" role="group">
      <button
        v-for="chip in activeChips"
        :key="chip.key"
        type="button"
        class="ui-chip is-active acct-active-chip"
        :aria-label="t('admin.accountsUi.removeFilter', { label: `${chip.label}: ${chip.value}` })"
        @click="clearOne(chip.key)"
      >
        <span class="acct-active-key">{{ chip.label }}</span>{{ chip.value }}
        <Icon name="x" size="xs" />
      </button>
      <button key="__clear" type="button" class="viz-link acct-active-clear" @click="clearAll">{{ t('admin.accountsUi.clearFilters') }}</button>
    </TransitionGroup>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import type { AdminGroup } from '@/types'

type FilterKey = 'type' | 'status' | 'privacy_mode' | 'auto_warmup' | 'group'

const props = withDefaults(defineProps<{
  searchQuery: string
  filters: Record<string, any>
  groups?: AdminGroup[]
  showStatus?: boolean
  platforms?: Array<{ value: string; label: string; count: number }>
  sortValue?: string
  sortOptions?: SelectOption[]
}>(), {
  groups: () => [],
  showStatus: true,
  platforms: () => [],
  sortValue: undefined,
  sortOptions: undefined,
})
const emit = defineEmits(['update:searchQuery', 'update:filters', 'change', 'update:sort'])
const { t } = useI18n()

const open = ref(false)
const popoverRoot = ref<HTMLElement | null>(null)

const onSearch = (value: string) => {
  emit('update:searchQuery', value)
}

const update = (key: FilterKey | 'platform', value: string | number | boolean | null) => {
  emit('update:filters', { ...props.filters, [key]: value ?? '' })
}

const togglePlatform = (value: string) => {
  update('platform', props.filters.platform === value ? '' : value)
  emit('change')
}

const tOpts = computed(() => [{ value: '', label: t('admin.accounts.allTypes') }, { value: 'oauth', label: t('admin.accounts.oauthType') }, { value: 'setup-token', label: t('admin.accounts.setupToken') }, { value: 'apikey', label: t('admin.accounts.apiKey') }, { value: 'bedrock', label: 'AWS Bedrock' }])
const sOpts = computed(() => [{ value: '', label: t('admin.accounts.allStatus') }, { value: 'active', label: t('admin.accounts.status.active') }, { value: 'inactive', label: t('admin.accounts.status.inactive') }, { value: 'error', label: t('admin.accounts.status.error') }, { value: 'rate_limited', label: t('admin.accounts.status.rateLimited') }, { value: 'temp_unschedulable', label: t('admin.accounts.status.tempUnschedulable') }, { value: 'unschedulable', label: t('admin.accounts.status.unschedulable') }])
const privacyOpts = computed(() => [
  { value: '', label: t('admin.accounts.allPrivacyModes') },
  { value: '__unset__', label: t('admin.accounts.privacyUnset') },
  { value: 'training_off', label: 'Privacy' },
  { value: 'training_set_cf_blocked', label: 'CF' },
  { value: 'training_set_failed', label: 'Fail' }
])
const autoWarmupOpts = computed(() => [
  { value: '', label: t('admin.accounts.autoWarmup.filterAll') },
  { value: 'enabled', label: t('admin.accounts.autoWarmup.filterEnabled') },
  { value: 'disabled', label: t('admin.accounts.autoWarmup.filterDisabled') }
])
const gOpts = computed(() => [
  { value: '', label: t('admin.accounts.allGroups') },
  { value: 'ungrouped', label: t('admin.accounts.ungroupedGroup') },
  ...props.groups.map(g => ({ value: String(g.id), label: g.name }))
])

const activeChips = computed(() => {
  const fields: Array<{ key: FilterKey; label: string; options: Array<{ value: string; label: string }> }> = [
    { key: 'type', label: t('admin.accounts.columns.type'), options: tOpts.value },
    ...(props.showStatus !== false ? [{ key: 'status' as const, label: t('common.status'), options: sOpts.value }] : []),
    { key: 'privacy_mode', label: t('admin.accountsUi.privacy'), options: privacyOpts.value },
    { key: 'auto_warmup', label: t('admin.accountsUi.autoWarmup'), options: autoWarmupOpts.value },
    { key: 'group', label: t('admin.accounts.columns.groups'), options: gOpts.value },
  ]
  return fields
    .filter((field) => props.filters[field.key])
    .map((field) => {
      const raw = String(props.filters[field.key])
      return { key: field.key, label: field.label, value: field.options.find((option) => option.value === raw)?.label ?? raw }
    })
})

const clearOne = (key: FilterKey) => {
  update(key, '')
  emit('change')
}

const clearAll = () => {
  const next: Record<string, any> = { ...props.filters }
  for (const chip of activeChips.value) next[chip.key] = ''
  emit('update:filters', next)
  emit('change')
}

const close = (focusTrigger = false) => {
  open.value = false
  if (focusTrigger) popoverRoot.value?.querySelector<HTMLElement>('.acct-filter-trigger')?.focus()
}

// Close on outside press; Select menus are teleported to <body>, so they count as inside.
const onPointerDown = (event: PointerEvent) => {
  const target = event.target as Element | null
  if (!target || popoverRoot.value?.contains(target) || target.closest('.select-dropdown-portal')) return
  close()
}
watch(open, (value) => {
  if (value) document.addEventListener('pointerdown', onPointerDown, true)
  else document.removeEventListener('pointerdown', onPointerDown, true)
})
onBeforeUnmount(() => document.removeEventListener('pointerdown', onPointerDown, true))
</script>

<style scoped>
.acct-search {
  max-width: 18rem;
}

.acct-search input::-webkit-search-cancel-button {
  cursor: pointer;
}

.acct-providers {
  flex-wrap: nowrap;
}

.acct-filter-trigger {
  height: 2.25rem;
  gap: 0.375rem;
}

.acct-filter-trigger.is-active {
  border-color: rgb(var(--operator-accent-rgb) / 0.55);
}

.acct-filter-count {
  display: inline-flex;
  min-width: 1.125rem;
  height: 1.125rem;
  align-items: center;
  justify-content: center;
  padding: 0 0.3125rem;
  border-radius: 999px;
  background: var(--operator-accent);
  color: #fff;
  font-size: 0.6875rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.acct-filter-pop {
  position: absolute;
  top: calc(100% + 0.375rem);
  left: 0;
  z-index: 40;
  width: min(22rem, calc(100vw - 2rem));
  padding: 0.875rem;
  transform-origin: top left;
}

.acct-filter-grid {
  display: grid;
  gap: 0.75rem;
}

.acct-filter-grid label {
  display: grid;
  grid-template-columns: 6.5rem minmax(0, 1fr);
  align-items: center;
  gap: 0.75rem;
  font-size: 0.8125rem;
}

.acct-filter-grid label > span {
  color: var(--operator-muted-foreground);
}

.acct-filter-foot {
  display: flex;
  justify-content: space-between;
  gap: 0.5rem;
  margin-top: 0.875rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--operator-popup-border);
}

.acct-active {
  position: relative;
  display: flex;
  flex-basis: 100%;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem;
}

.acct-active-chip {
  height: 1.75rem;
  font-size: 0.75rem;
}

.acct-active-key {
  color: var(--operator-subtle-foreground);
}

.acct-active-key::after {
  content: ':';
}

.acct-active-clear {
  margin-left: 0.25rem;
  font-size: 0.75rem;
}

@media (max-width: 640px) {
  .acct-search {
    max-width: none;
    flex-basis: 100%;
  }

  .acct-providers {
    max-width: calc(100vw - 9rem);
    overflow-x: auto;
    scrollbar-width: none;
  }

  .acct-filter-pop {
    position: fixed;
    top: auto;
    right: 0.75rem;
    bottom: calc(var(--operator-tabbar-height, 0px) + 0.75rem);
    left: 0.75rem;
    width: auto;
  }
}
</style>
