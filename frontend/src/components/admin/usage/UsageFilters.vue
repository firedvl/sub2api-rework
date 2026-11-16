<template>
  <!-- Toolbar layout: date, user search, quick selects, and a Filters panel for the rest. -->
  <div v-if="layout === 'toolbar'" class="usage-filterbar">
    <div class="ui-toolbar" role="toolbar" :aria-label="t('admin.activityUi.filters')">
      <slot name="leading" />

      <div data-usage-dd="user" class="usage-filter-dropdown relative min-w-[12rem] flex-1 sm:max-w-[18rem]">
        <label class="ui-search max-w-none">
          <Icon name="search" size="sm" />
          <input
            v-model="userKeyword"
            type="text"
            :aria-label="t('admin.usage.userFilter')"
            :placeholder="t('admin.usage.searchUserPlaceholder')"
            @input="debounceUserSearch"
            @focus="showUserDropdown = true"
          />
          <button v-if="filters.user_id" type="button" class="usage-filter-clear" :aria-label="t('admin.activityUi.clearFilter', { name: t('admin.usage.userFilter') })" @click="clearUser">
            <Icon name="x" size="xs" />
          </button>
        </label>
        <div
          v-if="showUserDropdown && (userResults.length > 0 || userKeyword)"
          class="operator-menu menu-enter absolute z-50 mt-1 max-h-60 w-full min-w-[16rem] overflow-auto"
        >
          <button v-for="u in userResults" :key="u.id" type="button" class="operator-menu-item w-full px-4 py-2 text-left" @click="selectUser(u)">
            <span>{{ u.email }}<span v-if="u.deleted" class="ml-1 text-xs text-gray-400">（{{ t('admin.usage.userDeletedBadge') }}）</span></span>
            <span class="ml-2 text-xs text-gray-400">#{{ u.id }}</span>
          </button>
        </div>
      </div>

      <div class="usage-filter-quick hidden md:block">
        <Select :model-value="filters.model ?? null" :options="modelOptions" searchable :aria-label="t('usage.model')" @update:model-value="setModel" @change="emitChange" />
      </div>

      <div data-usage-dd="account" class="usage-filter-dropdown usage-filter-quick relative hidden md:block">
        <label class="ui-search max-w-none">
          <Icon name="server" size="sm" />
          <input
            v-model="accountKeyword"
            type="text"
            :aria-label="t('admin.usage.account')"
            :placeholder="t('admin.usage.account')"
            @input="debounceAccountSearch"
            @focus="showAccountDropdown = true"
          />
          <button v-if="filters.account_id" type="button" class="usage-filter-clear" :aria-label="t('admin.activityUi.clearFilter', { name: t('admin.usage.account') })" @click="clearAccount">
            <Icon name="x" size="xs" />
          </button>
        </label>
        <div
          v-if="showAccountDropdown && (accountResults.length > 0 || accountKeyword)"
          class="operator-menu menu-enter absolute z-50 mt-1 max-h-60 w-full min-w-[14rem] overflow-auto"
        >
          <button v-for="a in accountResults" :key="a.id" type="button" class="operator-menu-item w-full px-4 py-2 text-left" @click="selectAccount(a)">
            <span class="truncate">{{ a.name }}</span>
            <span class="ml-2 text-xs text-gray-400">#{{ a.id }}</span>
          </button>
        </div>
      </div>

      <button
        type="button"
        class="btn btn-secondary btn-sm usage-filter-toggle"
        data-testid="usage-filters-toggle"
        :aria-expanded="panelOpen"
        :aria-controls="panelId"
        @click="panelOpen = !panelOpen"
      >
        <Icon name="filter" size="sm" />
        <span>{{ t('admin.activityUi.filters') }}</span>
        <span v-if="panelCount" class="usage-filter-count">{{ panelCount }}</span>
      </button>

      <span class="ui-grow"></span>
      <slot name="actions" />
    </div>

    <Transition name="usage-panel">
      <div v-if="panelOpen" :id="panelId" class="usage-filter-panel" data-testid="usage-filter-panel">
        <div class="usage-filter-field md:hidden">
          <label class="input-label">{{ t('usage.model') }}</label>
          <Select :model-value="filters.model ?? null" :options="modelOptions" searchable @update:model-value="setModel" @change="emitChange" />
        </div>
        <div data-usage-dd="account" class="usage-filter-dropdown usage-filter-field relative md:hidden">
          <label class="input-label">{{ t('admin.usage.account') }}</label>
          <input
            v-model="accountKeyword"
            type="text"
            class="input"
            :placeholder="t('admin.usage.searchAccountPlaceholder')"
            @input="debounceAccountSearch"
            @focus="showAccountDropdown = true"
          />
          <div
            v-if="showAccountDropdown && (accountResults.length > 0 || accountKeyword)"
            class="operator-menu menu-enter absolute z-50 mt-1 max-h-60 w-full overflow-auto"
          >
            <button v-for="a in accountResults" :key="a.id" type="button" class="operator-menu-item w-full px-4 py-2 text-left" @click="selectAccount(a)">
              <span class="truncate">{{ a.name }}</span>
              <span class="ml-2 text-xs text-gray-400">#{{ a.id }}</span>
            </button>
          </div>
        </div>
        <div data-usage-dd="apiKey" class="usage-filter-dropdown usage-filter-field relative">
          <label class="input-label">{{ t('usage.apiKeyFilter') }}</label>
          <input
            v-model="apiKeyKeyword"
            type="text"
            class="input"
            :placeholder="t('admin.usage.searchApiKeyPlaceholder')"
            @input="debounceApiKeySearch"
            @focus="onApiKeyFocus"
          />
          <div v-if="showApiKeyDropdown && apiKeyResults.length > 0" class="operator-menu menu-enter absolute z-50 mt-1 max-h-60 w-full overflow-auto">
            <button v-for="k in apiKeyResults" :key="k.id" type="button" class="operator-menu-item w-full px-4 py-2 text-left" @click="selectApiKey(k)">
              <span class="truncate">{{ k.name || `#${k.id}` }}</span>
              <span class="ml-2 text-xs text-gray-400">#{{ k.id }}</span>
            </button>
          </div>
        </div>
        <div v-for="field in visibleSelectFields" :key="field.key" class="usage-filter-field">
          <label class="input-label">{{ field.label }}</label>
          <Select v-model="filters[field.key]" :options="field.options" :searchable="field.searchable" @change="emitChange" />
        </div>
      </div>
    </Transition>

    <TransitionGroup v-if="chips.length" name="ui-list" tag="div" class="ui-chips usage-filter-chips" :aria-label="t('admin.activityUi.activeFilters')">
      <button
        v-for="chip in chips"
        :key="chip.key"
        type="button"
        class="ui-chip is-active"
        :aria-label="t('admin.activityUi.clearFilter', { name: `${chip.label}: ${chip.value}` })"
        @click="clearChip(chip.key)"
      >
        <span class="text-gray-500">{{ chip.label }}</span>
        <span class="max-w-[12rem] truncate">{{ chip.value }}</span>
        <Icon name="x" size="xs" />
      </button>
      <button key="__clear" type="button" class="btn btn-ghost btn-sm" data-testid="usage-filters-clear-all" @click="clearAll">
        {{ t('admin.activityUi.clearAll') }}
      </button>
    </TransitionGroup>
  </div>

  <div v-else :class="flat ? 'p-4 sm:p-6' : 'card p-6'">
    <!-- Toolbar: left filters (multi-line) + right actions -->
    <div class="flex flex-wrap items-end justify-between gap-4">
      <!-- Left: filters (allowed to wrap to multiple rows) -->
      <div class="flex flex-1 flex-wrap items-end gap-4">
        <!-- User Search -->
        <div ref="userSearchRef" class="usage-filter-dropdown relative w-full sm:w-auto sm:min-w-[240px]">
          <label class="input-label">{{ t('admin.usage.userFilter') }}</label>
          <input
            v-model="userKeyword"
            type="text"
            class="input pr-8"
            :placeholder="t('admin.usage.searchUserPlaceholder')"
            @input="debounceUserSearch"
            @focus="showUserDropdown = true"
          />
          <button
            v-if="filters.user_id"
            type="button"
            @click="clearUser"
            class="absolute right-2 top-9 text-gray-400"
            aria-label="Clear user filter"
          >
            ✕
          </button>
          <div
            v-if="showUserDropdown && (userResults.length > 0 || userKeyword)"
            class="operator-menu menu-enter absolute z-50 mt-1 max-h-60 w-full overflow-auto"
          >
            <button
              v-for="u in userResults"
              :key="u.id"
              type="button"
              @click="selectUser(u)"
              class="operator-menu-item w-full px-4 py-2 text-left"
            >
              <span>{{ u.email }}<span v-if="u.deleted" class="ml-1 text-xs text-gray-400">（{{ t('admin.usage.userDeletedBadge') }}）</span></span>
              <span class="ml-2 text-xs text-gray-400">#{{ u.id }}</span>
            </button>
          </div>
        </div>

        <!-- API Key Search -->
        <div ref="apiKeySearchRef" class="usage-filter-dropdown relative w-full sm:w-auto sm:min-w-[240px]">
          <label class="input-label">{{ t('usage.apiKeyFilter') }}</label>
          <input
            v-model="apiKeyKeyword"
            type="text"
            class="input pr-8"
            :placeholder="t('admin.usage.searchApiKeyPlaceholder')"
            @input="debounceApiKeySearch"
            @focus="onApiKeyFocus"
          />
          <button
            v-if="filters.api_key_id"
            type="button"
            @click="onClearApiKey"
            class="absolute right-2 top-9 text-gray-400"
            aria-label="Clear API key filter"
          >
            ✕
          </button>
          <div
            v-if="showApiKeyDropdown && apiKeyResults.length > 0"
            class="operator-menu menu-enter absolute z-50 mt-1 max-h-60 w-full overflow-auto"
          >
            <button
              v-for="k in apiKeyResults"
              :key="k.id"
              type="button"
              @click="selectApiKey(k)"
              class="operator-menu-item w-full px-4 py-2 text-left"
            >
              <span class="truncate">{{ k.name || `#${k.id}` }}</span>
              <span class="ml-2 text-xs text-gray-400">#{{ k.id }}</span>
            </button>
          </div>
        </div>

        <!-- Model Filter -->
        <div class="w-full sm:w-auto sm:min-w-[220px]">
          <label class="input-label">{{ t('usage.model') }}</label>
          <Select v-model="filters.model" :options="modelOptions" searchable @change="emitChange" />
        </div>

        <!-- Account Filter -->
        <div ref="accountSearchRef" class="usage-filter-dropdown relative w-full sm:w-auto sm:min-w-[220px]">
          <label class="input-label">{{ t('admin.usage.account') }}</label>
          <input
            v-model="accountKeyword"
            type="text"
            class="input pr-8"
            :placeholder="t('admin.usage.searchAccountPlaceholder')"
            @input="debounceAccountSearch"
            @focus="showAccountDropdown = true"
          />
          <button
            v-if="filters.account_id"
            type="button"
            @click="clearAccount"
            class="absolute right-2 top-9 text-gray-400"
            aria-label="Clear account filter"
          >
            ✕
          </button>
          <div
            v-if="showAccountDropdown && (accountResults.length > 0 || accountKeyword)"
            class="operator-menu menu-enter absolute z-50 mt-1 max-h-60 w-full overflow-auto"
          >
            <button
              v-for="a in accountResults"
              :key="a.id"
              type="button"
              @click="selectAccount(a)"
              class="operator-menu-item w-full px-4 py-2 text-left"
            >
              <span class="truncate">{{ a.name }}</span>
              <span class="ml-2 text-xs text-gray-400">#{{ a.id }}</span>
            </button>
          </div>
        </div>

        <!-- Request Type Filter (usage only) -->
        <div v-if="mode !== 'errors'" class="w-full sm:w-auto sm:min-w-[180px]">
          <label class="input-label">{{ t('usage.type') }}</label>
          <Select v-model="filters.request_type" :options="requestTypeOptions" @change="emitChange" />
        </div>

        <!-- Native compaction is independent of the transport request type. -->
        <div v-if="mode !== 'errors'" class="w-full sm:w-auto sm:min-w-[180px]">
          <label class="input-label">{{ t('usage.compactionFilter') }}</label>
          <Select v-model="filters.native_compaction_v2" :options="compactionOptions" @change="emitChange" />
        </div>

        <!-- Billing Type Filter (usage only) -->
        <div v-if="mode !== 'errors'" class="w-full sm:w-auto sm:min-w-[200px]">
          <label class="input-label">{{ t('admin.usage.billingType') }}</label>
          <Select v-model="filters.billing_type" :options="billingTypeOptions" @change="emitChange" />
        </div>

        <!-- Billing Mode Filter (usage only；用户排行的 user-breakdown 接口不支持该维度) -->
        <div v-if="mode === 'usage'" class="w-full sm:w-auto sm:min-w-[200px]">
          <label class="input-label">{{ t('admin.usage.billingMode') }}</label>
          <Select v-model="filters.billing_mode" :options="billingModeOptions" @change="emitChange" />
        </div>

        <div v-if="mode === 'usage'" class="w-full sm:w-auto sm:min-w-[220px]">
          <label class="input-label">{{ t('admin.usage.upstreamModelAudit') }}</label>
          <Select v-model="filters.upstream_model_mismatch" :options="upstreamModelMismatchOptions" @change="emitChange" />
        </div>

        <!-- Error Phase Filter (errors only) -->
        <div v-if="mode === 'errors'" class="w-full sm:w-auto sm:min-w-[180px]">
          <label class="input-label">{{ t('admin.ops.errorLog.type') }}</label>
          <Select v-model="filters.error_phase" :options="errorPhaseOptions" @change="emitChange" />
        </div>

        <!-- Error Category Filter (errors only) -->
        <div v-if="mode === 'errors'" class="w-full sm:w-auto sm:min-w-[180px]">
          <label class="input-label">{{ t('usage.errors.category') }}</label>
          <Select v-model="filters.error_category" :options="errorCategoryOptions" @change="emitChange" />
        </div>

        <!-- Status Code Filter (errors only) -->
        <div v-if="mode === 'errors'" class="w-full sm:w-auto sm:min-w-[180px]">
          <label class="input-label">{{ t('admin.ops.errorLog.status') }}</label>
          <Select v-model="filters.status_code" :options="statusCodeOptions" @change="emitChange" />
        </div>

        <!-- Group Filter -->
        <div class="w-full sm:w-auto sm:min-w-[200px]">
          <label class="input-label">{{ t('admin.usage.group') }}</label>
          <Select v-model="filters.group_id" :options="groupOptions" searchable @change="emitChange" />
        </div>

      </div>

      <!-- Right: actions -->
      <div v-if="showActions" class="flex w-full flex-wrap items-center justify-end gap-3 sm:w-auto">
        <button type="button" @click="$emit('refresh')" class="btn btn-secondary">
          {{ t('common.refresh') }}
        </button>
        <button type="button" @click="$emit('reset')" class="btn btn-secondary">
          {{ t('common.reset') }}
        </button>
        <slot name="after-reset" />
        <template v-if="mode === 'usage'">
          <button type="button" @click="$emit('cleanup')" class="btn btn-danger">
            {{ t('admin.usage.cleanup.button') }}
          </button>
          <button type="button" @click="$emit('export')" :disabled="exporting" class="btn btn-primary">
            {{ t('usage.exportExcel') }}
          </button>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, toRef, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { COMMON_ERROR_STATUS_CODES } from '@/utils/errorBadges'
import type { SimpleApiKey, SimpleUser } from '@/api/admin/usage'

type ModelValue = Record<string, any>

interface Props {
  modelValue: ModelValue
  exporting: boolean
  startDate: string
  endDate: string
  showActions?: boolean
  modelOptions?: string[]
  /**
   * errors 模式:隐藏用量专属字段/按钮,显示错误类型+状态码(错误请求 tab 用)
   * ranking 模式:同 usage 但隐藏计费模式筛选与清理/导出按钮(用户排行 tab 用)
   */
  mode?: 'usage' | 'errors' | 'ranking'
  /** 嵌入统一卡片内使用：去掉自身卡片外观 */
  flat?: boolean
  /** toolbar: compact search + quick selects, other filters in a Filters panel with active chips */
  layout?: 'form' | 'toolbar'
}

const props = withDefaults(defineProps<Props>(), {
  showActions: true,
  mode: 'usage',
  flat: false,
  layout: 'form'
})
const emit = defineEmits([
  'update:modelValue',
  'change',
  'refresh',
  'reset',
  'export',
  'cleanup'
])

const { t } = useI18n()
const filters = toRef(props, 'modelValue')

const userSearchRef = ref<HTMLElement | null>(null)
const apiKeySearchRef = ref<HTMLElement | null>(null)
const accountSearchRef = ref<HTMLElement | null>(null)

const userKeyword = ref('')
const userResults = ref<SimpleUser[]>([])
const showUserDropdown = ref(false)
let userSearchTimeout: ReturnType<typeof setTimeout> | null = null
let userSearchSequence = 0

const apiKeyKeyword = ref('')
const apiKeyResults = ref<SimpleApiKey[]>([])
const showApiKeyDropdown = ref(false)
let apiKeySearchTimeout: ReturnType<typeof setTimeout> | null = null

interface SimpleAccount {
  id: number
  name: string
}
const accountKeyword = ref('')
const accountResults = ref<SimpleAccount[]>([])
const showAccountDropdown = ref(false)
let accountSearchTimeout: ReturnType<typeof setTimeout> | null = null

const modelOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allModels') },
  ...(props.modelOptions ?? []).map((m) => ({ value: m, label: m })),
])
const groupOptions = ref<SelectOption[]>([{ value: null, label: t('admin.usage.allGroups') }])

const requestTypeOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allTypes') },
  { value: 'ws_v2', label: t('usage.ws') },
  { value: 'live', label: t('usage.live') },
  { value: 'stream', label: t('usage.stream') },
  { value: 'sync', label: t('usage.sync') },
  { value: 'cyber', label: t('usage.cyber') }
])

const compactionOptions = ref<SelectOption[]>([
  { value: null, label: t('usage.allCompactionTypes') },
  { value: true, label: t('usage.compactionOnly') }
])

const billingTypeOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allBillingTypes') },
  { value: 0, label: t('admin.usage.billingTypeBalance') },
  { value: 1, label: t('admin.usage.billingTypeSubscription') }
])

// 错误类型对应后端 phase 参数(与错误表"类型"徽章同语义)
const errorPhaseOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('admin.usage.allTypes') },
  { value: 'upstream', label: t('admin.ops.errorLog.typeUpstream') },
  { value: 'account_auth', label: t('admin.ops.errorLog.typeAccountAuth') },
  { value: 'request', label: t('admin.ops.errorLog.typeRequest') },
  { value: 'auth', label: t('admin.ops.errorLog.typeAuth') },
  { value: 'routing', label: t('admin.ops.errorLog.typeRouting') },
  { value: 'internal', label: t('admin.ops.errorLog.typeInternal') },
])

// 分类码同用户端 /usage 错误筛选;"other" 无法反查为过滤条件,刻意不列
const errorCategoryCodes = ['auth', 'rate_limit', 'quota', 'invalid_request', 'service_unavailable', 'upstream', 'internal', 'cyber']

const errorCategoryOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.errors.allCategories') },
  ...errorCategoryCodes.map((c) => ({ value: c, label: t('usage.errors.categories.' + c) })),
])

const statusCodeOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.errors.allStatuses') },
  ...COMMON_ERROR_STATUS_CODES.map((c) => ({ value: c, label: String(c) })),
])

const billingModeOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allBillingModes') },
  { value: 'token', label: t('admin.usage.billingModeToken') },
  { value: 'per_request', label: t('admin.usage.billingModePerRequest') },
  { value: 'image', label: t('admin.usage.billingModeImage') },
  { value: 'video', label: t('admin.usage.billingModeVideo') }
])

const upstreamModelMismatchOptions = ref<SelectOption[]>([
  { value: null, label: t('admin.usage.allUpstreamModelAudit') },
  { value: true, label: t('admin.usage.upstreamModelMismatchOnly') },
  { value: false, label: t('admin.usage.upstreamModelMatchedOnly') }
])

const emitChange = () => emit('change')
// An unset model (undefined, as reset and chip clearing leave it) is the "All models" option.
const setModel = (value: string | number | boolean | null) => {
  filters.value.model = value ?? undefined
}

const clearPendingUserSearch = () => {
  if (userSearchTimeout) {
    clearTimeout(userSearchTimeout)
    userSearchTimeout = null
  }
  userSearchSequence += 1
}

const debounceUserSearch = () => {
  clearPendingUserSearch()
  const query = userKeyword.value.trim()
  if (!query) {
    userResults.value = []
    return
  }

  const sequence = userSearchSequence
  userSearchTimeout = setTimeout(async () => {
    userSearchTimeout = null
    try {
      const results = await adminAPI.usage.searchUsers(query)
      if (sequence === userSearchSequence) {
        userResults.value = results.sort((a, b) => Number(a.deleted) - Number(b.deleted))
      }
    } catch {
      if (sequence === userSearchSequence) {
        userResults.value = []
      }
    }
  }, 300)
}

const debounceApiKeySearch = () => {
  if (apiKeySearchTimeout) clearTimeout(apiKeySearchTimeout)
  apiKeySearchTimeout = setTimeout(async () => {
    try {
      apiKeyResults.value = await adminAPI.usage.searchApiKeys(
        filters.value.user_id,
        apiKeyKeyword.value || ''
      )
    } catch {
      apiKeyResults.value = []
    }
  }, 300)
}

const selectUser = async (u: SimpleUser) => {
  clearPendingUserSearch()
  userKeyword.value = u.email
  showUserDropdown.value = false
  filters.value.user_id = u.id
  clearApiKey()

  // Auto-load API keys for this user
  try {
    apiKeyResults.value = await adminAPI.usage.searchApiKeys(u.id, '')
  } catch {
    apiKeyResults.value = []
  }

  emitChange()
}

const clearUser = () => {
  clearPendingUserSearch()
  userKeyword.value = ''
  userResults.value = []
  showUserDropdown.value = false
  filters.value.user_id = undefined
  clearApiKey()
  emitChange()
}

const selectApiKey = (k: SimpleApiKey) => {
  apiKeyKeyword.value = k.name || String(k.id)
  showApiKeyDropdown.value = false
  filters.value.api_key_id = k.id
  emitChange()
}

const clearApiKey = () => {
  apiKeyKeyword.value = ''
  apiKeyResults.value = []
  showApiKeyDropdown.value = false
  filters.value.api_key_id = undefined
}

const onClearApiKey = () => {
  clearApiKey()
  emitChange()
}

const debounceAccountSearch = () => {
  if (accountSearchTimeout) clearTimeout(accountSearchTimeout)
  accountSearchTimeout = setTimeout(async () => {
    if (!accountKeyword.value) {
      accountResults.value = []
      return
    }
    try {
      const res = await adminAPI.accounts.list(1, 20, { search: accountKeyword.value })
      accountResults.value = res.items.map((a) => ({ id: a.id, name: a.name }))
    } catch {
      accountResults.value = []
    }
  }, 300)
}

const selectAccount = (a: SimpleAccount) => {
  accountKeyword.value = a.name
  showAccountDropdown.value = false
  filters.value.account_id = a.id
  emitChange()
}

const clearAccount = () => {
  accountKeyword.value = ''
  accountResults.value = []
  showAccountDropdown.value = false
  filters.value.account_id = undefined
  emitChange()
}

const onApiKeyFocus = () => {
  showApiKeyDropdown.value = true
  // Trigger search if no results yet
  if (apiKeyResults.value.length === 0) {
    debounceApiKeySearch()
  }
}

const onDocumentClick = (e: MouseEvent) => {
  const target = e.target as Node | null
  if (!target) return

  const inside = (el: HTMLElement | null, key: string) =>
    (el?.contains(target) ?? false) || Boolean((target as Element).closest?.(`[data-usage-dd="${key}"]`))
  const clickedInsideUser = inside(userSearchRef.value, 'user')
  const clickedInsideApiKey = inside(apiKeySearchRef.value, 'apiKey')
  const clickedInsideAccount = inside(accountSearchRef.value, 'account')

  if (!clickedInsideUser) showUserDropdown.value = false
  if (!clickedInsideApiKey) showApiKeyDropdown.value = false
  if (!clickedInsideAccount) showAccountDropdown.value = false
}

watch(
  () => props.startDate,
  (value) => {
    filters.value.start_date = value
  },
  { immediate: true }
)

watch(
  () => props.endDate,
  (value) => {
    filters.value.end_date = value
  },
  { immediate: true }
)

watch(
  () => filters.value.user_id,
  (userId) => {
    if (!userId) {
      clearPendingUserSearch()
      userKeyword.value = ''
      userResults.value = []
    }
  }
)

watch(
  () => filters.value.api_key_id,
  (apiKeyId) => {
    if (!apiKeyId) {
      apiKeyKeyword.value = ''
      apiKeyResults.value = []
    }
  }
)

watch(
  () => filters.value.account_id,
  (accountId) => {
    if (!accountId) {
      accountKeyword.value = ''
      accountResults.value = []
    }
  }
)

onMounted(async () => {
  document.addEventListener('click', onDocumentClick)
  try {
    const gs = await adminAPI.groups.list(1, 1000)
    groupOptions.value.push(...gs.items.map((g: any) => ({ value: g.id, label: g.name })))
  } catch {
    // Ignore filter option loading errors (page still usable)
  }
})

onUnmounted(() => {
  clearPendingUserSearch()
  document.removeEventListener('click', onDocumentClick)
})

// 供外部(如用户排行下钻)在程序化设置 user_id 后回显选中的用户邮箱
const setUserKeyword = (email: string) => {
  clearPendingUserSearch()
  userKeyword.value = email
  userResults.value = []
  showUserDropdown.value = false
}

const getUserSearchRevision = () => userSearchSequence

// ---- Toolbar layout: Filters panel + active-filter chips ----
const panelOpen = ref(false)
const panelId = `usage-filter-panel-${Math.random().toString(36).slice(2, 8)}`

interface SelectField {
  key: string
  label: string
  options: SelectOption[]
  searchable?: boolean
  modes: Array<'usage' | 'errors' | 'ranking'>
}

const selectFields = computed<SelectField[]>(() => [
  { key: 'request_type', label: t('usage.type'), options: requestTypeOptions.value, modes: ['usage', 'ranking'] },
  { key: 'native_compaction_v2', label: t('usage.compactionFilter'), options: compactionOptions.value, modes: ['usage', 'ranking'] },
  { key: 'billing_type', label: t('admin.usage.billingType'), options: billingTypeOptions.value, modes: ['usage', 'ranking'] },
  { key: 'billing_mode', label: t('admin.usage.billingMode'), options: billingModeOptions.value, modes: ['usage'] },
  { key: 'upstream_model_mismatch', label: t('admin.usage.upstreamModelAudit'), options: upstreamModelMismatchOptions.value, modes: ['usage'] },
  { key: 'error_phase', label: t('admin.ops.errorLog.type'), options: errorPhaseOptions.value, modes: ['errors'] },
  { key: 'error_category', label: t('usage.errors.category'), options: errorCategoryOptions.value, modes: ['errors'] },
  { key: 'status_code', label: t('admin.ops.errorLog.status'), options: statusCodeOptions.value, modes: ['errors'] },
  { key: 'group_id', label: t('admin.usage.group'), options: groupOptions.value, searchable: true, modes: ['usage', 'errors', 'ranking'] },
])

const visibleSelectFields = computed(() => selectFields.value.filter((field) => field.modes.includes(props.mode)))

const isSet = (value: unknown) => value !== undefined && value !== null && value !== ''

interface FilterChip {
  key: string
  label: string
  value: string
}

const chips = computed<FilterChip[]>(() => {
  const f = filters.value
  const list: FilterChip[] = []
  if (isSet(f.user_id)) list.push({ key: 'user_id', label: t('admin.usage.userFilter'), value: userKeyword.value || `#${f.user_id}` })
  if (isSet(f.api_key_id)) list.push({ key: 'api_key_id', label: t('usage.apiKeyFilter'), value: apiKeyKeyword.value || `#${f.api_key_id}` })
  if (isSet(f.model)) list.push({ key: 'model', label: t('usage.model'), value: String(f.model) })
  if (isSet(f.account_id)) list.push({ key: 'account_id', label: t('admin.usage.account'), value: accountKeyword.value || `#${f.account_id}` })
  for (const field of visibleSelectFields.value) {
    const value = f[field.key]
    if (!isSet(value)) continue
    const option = field.options.find((o) => o.value === value)
    list.push({ key: field.key, label: field.label, value: option?.label ?? String(value) })
  }
  return list
})

/** Filters living in the panel (not visible in the toolbar on desktop). */
const panelCount = computed(() => chips.value.filter((chip) => !['user_id', 'model', 'account_id'].includes(chip.key)).length)

const clearChip = (key: string) => {
  if (key === 'user_id') return clearUser()
  if (key === 'api_key_id') return onClearApiKey()
  if (key === 'account_id') return clearAccount()
  filters.value[key] = key === 'model' || key === 'group_id' ? undefined : null
  emitChange()
}

const clearAll = () => {
  clearPendingUserSearch()
  userKeyword.value = ''
  userResults.value = []
  clearApiKey()
  accountKeyword.value = ''
  accountResults.value = []
  const f = filters.value
  f.user_id = undefined
  f.account_id = undefined
  f.model = undefined
  f.group_id = undefined
  for (const field of selectFields.value) {
    if (field.key !== 'group_id') f[field.key] = null
  }
  emitChange()
}

defineExpose({ getUserSearchRevision, setUserKeyword })
</script>

<style scoped>
.usage-filterbar {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.625rem;
}
.usage-filter-quick {
  width: 12.5rem;
  flex: 0 1 12.5rem;
}
.usage-filter-quick :deep(.select-trigger) {
  height: 2.25rem;
}
.usage-filter-clear {
  display: inline-grid;
  width: 1.25rem;
  height: 1.25rem;
  flex: none;
  place-items: center;
  border-radius: 999px;
  color: var(--operator-subtle-foreground);
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out), color var(--operator-duration-fast) var(--operator-ease-out);
}
.usage-filter-clear:hover {
  background: var(--operator-muted);
  color: var(--operator-foreground);
}
.usage-filter-toggle {
  gap: 0.375rem;
}
.usage-filter-toggle[aria-expanded='true'] {
  border-color: rgb(var(--operator-accent-rgb) / 0.55);
  background: var(--operator-accent-soft);
  color: var(--operator-foreground);
}
.usage-filter-count {
  display: inline-grid;
  min-width: 1.125rem;
  height: 1.125rem;
  place-items: center;
  padding: 0 0.3125rem;
  border-radius: 999px;
  background: var(--operator-accent);
  color: #fff;
  font-size: 0.6875rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  animation: ui-pop 320ms var(--operator-spring);
}
.usage-filter-panel {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(12.5rem, 1fr));
  gap: 0.75rem 1rem;
  padding: 0.875rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-raised);
}
.usage-filter-field {
  min-width: 0;
}
.usage-filter-field .input-label {
  margin-bottom: 0.25rem;
  font-size: 0.75rem;
}
.usage-panel-enter-active,
.usage-panel-leave-active {
  transition:
    opacity var(--operator-duration) var(--operator-ease-out),
    transform var(--operator-duration) var(--operator-ease-out);
}
.usage-panel-enter-from,
.usage-panel-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
@media (max-width: 640px) {
  .usage-filter-panel {
    grid-template-columns: minmax(0, 1fr);
  }
}
@media (prefers-reduced-motion: reduce) {
  .usage-panel-enter-active,
  .usage-panel-leave-active {
    transition: none;
  }
  .usage-filter-count {
    animation: none;
  }
}
</style>
