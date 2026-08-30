<template>
  <Teleport to="body">
    <div v-if="show && position">
      <!-- Backdrop: click anywhere outside to close -->
      <div class="fixed inset-0 z-[9998]" @click="emit('close')"></div>
      <div
        ref="menuContent"
        class="action-menu-content operator-menu fixed z-[9999] w-52 overflow-y-auto rounded-xl bg-white shadow-lg ring-1 ring-black/5 dark:bg-dark-800"
        :style="menuStyle"
        @click.stop
      >
        <div class="py-1">
          <template v-if="account">
            <button @click="$emit('test', account); $emit('close')" class="operator-menu-item flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="play" size="sm" :stroke-width="2" />
              {{ t('admin.accounts.testConnection') }}
            </button>
            <button @click="$emit('stats', account); $emit('close')" class="operator-menu-item flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="chart" size="sm" />
              {{ t('admin.accounts.viewStats') }}
            </button>
            <button @click="$emit('schedule', account); $emit('close')" class="operator-menu-item flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="clock" size="sm" />
              {{ t('admin.scheduledTests.schedule') }}
            </button>
            <button v-if="canDuplicate" @click="$emit('duplicate', account); $emit('close')" class="operator-menu-item flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="copy" size="sm" />
              {{ t('admin.accounts.duplicateAccount') }}
            </button>
            <!-- 影子账号不持凭据:重授权/刷新 token 对其无效(后端拒绝),故隐藏(外审 G4)。 -->
            <template v-if="(account.type === 'oauth' || account.type === 'setup-token') && !isShadow">
              <button @click="$emit('reauth', account); $emit('close')" class="operator-menu-item flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700">
                <Icon name="link" size="sm" />
                {{ t('admin.accounts.reAuthorize') }}
              </button>
              <button @click="$emit('refresh-token', account); $emit('close')" class="operator-menu-item flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700">
                <Icon name="refresh" size="sm" />
                {{ t('admin.accounts.refreshToken') }}
              </button>
            </template>
            <button v-if="isOpenAIOAuthParent" @click="$emit('create-spark-shadow', account); $emit('close')" class="operator-menu-item flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="sparkles" size="sm" />
              {{ t('admin.accounts.createSparkShadow') }}
            </button>
            <button v-if="supportsPrivacy" @click="$emit('set-privacy', account); $emit('close')" class="operator-menu-item flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="shield" size="sm" />
              {{ t('admin.accounts.setPrivacy') }}
            </button>
            <div v-if="hasRecoverableState" class="operator-menu-divider my-1 border-t border-gray-100 dark:border-dark-700"></div>
            <button v-if="hasRecoverableState" @click="$emit('recover-state', account); $emit('close')" class="operator-menu-item operator-menu-item-recovery flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="sync" size="sm" />
              {{ t('admin.accounts.recoverState') }}
            </button>
            <button v-if="hasQuotaLimit" @click="$emit('reset-quota', account); $emit('close')" class="operator-menu-item operator-menu-item-caution flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="refresh" size="sm" />
              {{ t('admin.accounts.resetQuota') }}
            </button>
            <div class="operator-menu-divider my-1 border-t border-gray-100 dark:border-dark-700"></div>
            <button @click="$emit('delete', account); $emit('close')" class="operator-menu-item operator-menu-item-destructive flex w-full items-center gap-2 px-4 py-2 text-sm hover:bg-gray-100 dark:hover:bg-dark-700">
              <Icon name="trash" size="sm" />
              {{ t('admin.accounts.deleteAccount') }}
            </button>
          </template>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@/components/icons'
import type { Account } from '@/types'
import { getActiveModelRateLimits } from '@/utils/modelRateLimits'

const props = defineProps<{ show: boolean; account: Account | null; position: { top: number; left: number } | null }>()
const emit = defineEmits(['close', 'test', 'stats', 'schedule', 'duplicate', 'reauth', 'refresh-token', 'recover-state', 'reset-quota', 'set-privacy', 'create-spark-shadow', 'delete'])
const { t } = useI18n()
const menuContent = ref<HTMLElement | null>(null)
const menuStyle = ref<Record<string, string>>({})
const canDuplicate = computed(() => {
  if (!props.account || props.account.parent_account_id != null) return false
  return ['apikey', 'upstream', 'bedrock', 'service_account'].includes(props.account.type)
})
const isRateLimited = computed(() => {
  if (props.account?.rate_limit_reset_at && new Date(props.account.rate_limit_reset_at) > new Date()) {
    return true
  }
  return getActiveModelRateLimits(props.account?.extra).length > 0
})
const isOverloaded = computed(() => props.account?.overload_until && new Date(props.account.overload_until) > new Date())
const isTempUnschedulable = computed(() => props.account?.temp_unschedulable_until && new Date(props.account.temp_unschedulable_until) > new Date())
const hasRecoverableState = computed(() => {
  return props.account?.status === 'error' || Boolean(isRateLimited.value) || Boolean(isOverloaded.value) || Boolean(isTempUnschedulable.value)
})
const isAntigravityOAuth = computed(() => props.account?.platform === 'antigravity' && props.account?.type === 'oauth')
const isOpenAIOAuth = computed(() => props.account?.platform === 'openai' && props.account?.type === 'oauth')
// 影子账号(链接型,持 parent_account_id)不持凭据、type 不可变,凭据/隐私类操作对其无效。
const isShadow = computed(() => props.account?.parent_account_id != null)
// A "parent" OpenAI OAuth account is one that is NOT itself a shadow (parent_account_id == null)
const isOpenAIOAuthParent = computed(() => isOpenAIOAuth.value && !isShadow.value)
const supportsPrivacy = computed(() => (isAntigravityOAuth.value || isOpenAIOAuth.value) && !isShadow.value)
const hasQuotaLimit = computed(() => {
  return (props.account?.type === 'apikey' || props.account?.type === 'bedrock') && (
    (props.account?.quota_limit ?? 0) > 0 ||
    (props.account?.quota_daily_limit ?? 0) > 0 ||
    (props.account?.quota_weekly_limit ?? 0) > 0
  )
})

const handleKeydown = (event: KeyboardEvent) => {
  if (event.key === 'Escape') emit('close')
}

const updatePosition = () => {
  if (!menuContent.value || !props.position) return

  const padding = 8
  const statusBarTop = document.querySelector<HTMLElement>('.operator-status-bar')?.getBoundingClientRect().top
  const viewportBottom = Math.min(window.innerHeight, statusBarTop ?? window.innerHeight)
  const maxHeight = Math.max(0, viewportBottom - padding * 2)
  const menuHeight = Math.min(menuContent.value.scrollHeight, maxHeight)
  const menuWidth = menuContent.value.getBoundingClientRect().width

  menuStyle.value = {
    top: `${Math.max(padding, Math.min(props.position.top, viewportBottom - menuHeight - padding))}px`,
    left: `${Math.max(padding, Math.min(props.position.left, window.innerWidth - menuWidth - padding))}px`,
    maxHeight: `${maxHeight}px`,
  }
}

watch(
  () => [props.show, props.position, props.account] as const,
  async ([visible, position]) => {
    if (!visible || !position) return
    menuStyle.value = { top: `${position.top}px`, left: `${position.left}px` }
    await nextTick()
    updatePosition()
  },
  { immediate: true },
)

watch(
  () => props.show,
  (visible) => {
    if (visible) {
      window.addEventListener('keydown', handleKeydown)
      window.addEventListener('resize', updatePosition)
    } else {
      window.removeEventListener('keydown', handleKeydown)
      window.removeEventListener('resize', updatePosition)
    }
  },
  { immediate: true }
)

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('resize', updatePosition)
})
</script>
