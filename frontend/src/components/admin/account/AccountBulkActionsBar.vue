<template>
  <Teleport to="body">
    <Transition name="acct-bulk">
      <div
        v-if="selectedIds.length > 0"
        class="acct-bulk"
        role="region"
        :aria-label="t('admin.accountsUi.bulkActions')"
        data-testid="account-bulk-bar"
      >
        <div class="acct-bulk-summary">
          <span class="acct-bulk-count" aria-live="polite">
            <template v-if="allResultsSelected">{{ t('admin.accountsUi.selectedAll', { count: selectedIds.length }) }}</template>
            <template v-else>{{ t('admin.accountsUi.selected', { count: selectedIds.length }) }}</template>
          </span>
          <button v-if="pageSelectable" type="button" class="acct-bulk-link" @click="$emit('select-page')">
            {{ t('admin.accounts.bulkActions.selectCurrentPage') }}
          </button>
          <button
            v-if="!allResultsSelected && totalResults > selectedIds.length"
            type="button"
            class="acct-bulk-link"
            :disabled="selectingAll"
            @click="$emit('select-all-results')"
          >
            {{ selectingAll ? t('admin.accounts.bulkActions.selectingAll') : t('admin.accounts.bulkActions.selectAllResults', { count: totalResults }) }}
          </button>
        </div>

        <div class="acct-bulk-actions">
          <button type="button" class="btn btn-primary btn-sm" @click="$emit('edit-selected')">
            <Icon name="edit" size="sm" />{{ t('common.edit') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="$emit('toggle-schedulable', true)">
            <Icon name="play" size="sm" />{{ t('admin.accountsUi.resume') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="$emit('toggle-schedulable', false)">
            <Icon name="stop" size="sm" />{{ t('admin.accountsUi.pause') }}
          </button>
          <div ref="moreRoot" class="relative">
            <button
              type="button"
              class="app-icon-button"
              :aria-label="t('common.more')"
              :title="t('common.more')"
              :aria-expanded="moreOpen"
              @click="moreOpen = !moreOpen"
            >
              <Icon name="more" size="sm" />
            </button>
            <Transition name="menu-pop">
              <div v-if="moreOpen" class="operator-menu acct-bulk-menu" role="menu" @keydown.esc.stop="moreOpen = false">
                <button type="button" role="menuitem" class="operator-menu-item acct-bulk-menu-item" @click="run('refresh-token')">
                  <Icon name="key" size="sm" />{{ t('admin.accounts.bulkActions.refreshToken') }}
                </button>
                <button type="button" role="menuitem" class="operator-menu-item acct-bulk-menu-item" @click="run('reset-status')">
                  <Icon name="sync" size="sm" />{{ t('admin.accounts.bulkActions.resetStatus') }}
                </button>
                <button type="button" role="menuitem" class="operator-menu-item acct-bulk-menu-item" @click="run('probe-upstream-billing')">
                  <Icon name="dollar" size="sm" />{{ t('admin.accounts.bulkActions.probeUpstreamBilling') }}
                </button>
                <div class="operator-menu-divider my-1 border-t"></div>
                <button type="button" role="menuitem" class="operator-menu-item operator-menu-item-destructive acct-bulk-menu-item" @click="run('delete')">
                  <Icon name="trash" size="sm" />{{ t('admin.accounts.bulkActions.delete') }}
                </button>
              </div>
            </Transition>
          </div>
          <span class="acct-bulk-divider" aria-hidden="true"></span>
          <button
            type="button"
            class="app-icon-button"
            :aria-label="t('admin.accounts.bulkActions.clear')"
            :title="t('admin.accounts.bulkActions.clear')"
            @click="$emit('clear')"
          >
            <Icon name="x" size="sm" />
          </button>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  selectedIds: number[]
  totalResults: number
  selectingAll: boolean
  allResultsSelected: boolean
  /** Hide "select this page" when every visible row is already one page. */
  pageSelectable?: boolean
}>(), { pageSelectable: true })

type MenuEvent = 'refresh-token' | 'reset-status' | 'probe-upstream-billing' | 'delete'

const emit = defineEmits([
  'delete',
  'edit-selected',
  'clear',
  'select-page',
  'select-all-results',
  'toggle-schedulable',
  'reset-status',
  'refresh-token',
  'probe-upstream-billing'
])

const { t } = useI18n()
const moreOpen = ref(false)
const moreRoot = ref<HTMLElement | null>(null)

const run = (event: MenuEvent) => {
  moreOpen.value = false
  emit(event)
}

const onPointerDown = (event: PointerEvent) => {
  if (!moreRoot.value?.contains(event.target as Node)) moreOpen.value = false
}
watch(moreOpen, (open) => {
  if (open) document.addEventListener('pointerdown', onPointerDown, true)
  else document.removeEventListener('pointerdown', onPointerDown, true)
})
watch(() => props.selectedIds.length, (count) => {
  if (count === 0) moreOpen.value = false
})
onBeforeUnmount(() => document.removeEventListener('pointerdown', onPointerDown, true))
</script>

<style scoped>
.acct-bulk {
  position: fixed;
  bottom: 1.25rem;
  left: 50%;
  z-index: 45;
  display: flex;
  max-width: calc(100vw - 2rem);
  align-items: center;
  gap: 1rem;
  padding: 0.5rem 0.5rem 0.5rem 1rem;
  border: 1px solid var(--operator-popup-border);
  border-radius: 0.875rem;
  background: var(--operator-popup-surface);
  box-shadow: var(--operator-shadow-lg);
  transform: translateX(-50%);
}

.acct-bulk-summary {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.25rem 0.75rem;
}

.acct-bulk-count {
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.acct-bulk-link {
  color: var(--operator-accent-text);
  font-size: 0.75rem;
  font-weight: 500;
  white-space: nowrap;
}

.acct-bulk-link:hover {
  text-decoration: underline;
}

.acct-bulk-link:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.acct-bulk-actions {
  display: flex;
  align-items: center;
  gap: 0.375rem;
}

.acct-bulk-divider {
  width: 1px;
  height: 1.25rem;
  margin: 0 0.125rem;
  background: var(--operator-border);
}

.acct-bulk-menu {
  position: absolute;
  right: 0;
  bottom: calc(100% + 0.5rem);
  z-index: 1;
  width: 14rem;
  padding: 0.25rem;
  transform-origin: bottom right;
}

.acct-bulk-menu-item {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 0.5rem;
  padding: 0 0.625rem;
  border-radius: var(--operator-radius);
  text-align: left;
}

.acct-bulk-enter-active,
.acct-bulk-leave-active {
  transition:
    opacity var(--operator-duration) var(--operator-ease-out),
    transform 320ms var(--operator-spring);
}

.acct-bulk-enter-from,
.acct-bulk-leave-to {
  opacity: 0;
  transform: translate(-50%, 1.5rem);
}

@media (max-width: 1023px) {
  .acct-bulk {
    bottom: calc(var(--operator-tabbar-height, 3.75rem) + env(safe-area-inset-bottom) + 0.5rem);
  }
}

@media (max-width: 640px) {
  .acct-bulk {
    right: 0.5rem;
    left: 0.5rem;
    flex-wrap: wrap;
    gap: 0.5rem;
    max-width: none;
    padding: 0.625rem;
    transform: none;
  }

  .acct-bulk-enter-from,
  .acct-bulk-leave-to {
    transform: translateY(1.5rem);
  }

  .acct-bulk-actions {
    width: 100%;
  }

  .acct-bulk-actions .btn {
    flex: 1 1 0;
    justify-content: center;
  }
}
</style>
