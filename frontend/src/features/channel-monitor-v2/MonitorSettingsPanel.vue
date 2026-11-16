<template>
  <section class="msp ui-stagger">
    <div class="ui-card msp-bar">
      <span class="viz-pulse" :class="draft?.enabled && systemModeV2 ? '' : 'is-idle'" aria-hidden="true"></span>
      <strong class="msp-bar-title">{{ t('channelMonitorV2.settings.enableTitle') }}</strong>
      <HelpTooltip interactive :aria-label="t('admin.routingUi.info')" :content="`${t('channelMonitorV2.settings.description')} ${t('channelMonitorV2.settings.enableHint')}`" />
      <Toggle
        v-if="draft"
        v-model="draft.enabled"
        :aria-label="t('channelMonitorV2.settings.enableTitle')"
      />
      <span class="msp-divider" aria-hidden="true"></span>
      <div
        v-if="draft"
        class="viz-seg"
        role="group"
        :aria-label="t('channelMonitorV2.settings.refreshAria')"
        :title="t('channelMonitorV2.settings.refreshHint')"
      >
        <button
          type="button"
          :aria-pressed="draft.refresh_interval_seconds === 60"
          @click="draft.refresh_interval_seconds = 60"
        >
          1 min
        </button>
        <button
          type="button"
          :aria-pressed="draft.refresh_interval_seconds === 300"
          @click="draft.refresh_interval_seconds = 300"
        >
          5 min
        </button>
      </div>
      <span class="ui-grow"></span>
      <span v-if="draft" class="msp-named" :title="namedModelCount === 0 ? t('channelMonitorV2.settings.namedModelsEmpty') : t('channelMonitorV2.settings.namedModelsCount', { count: namedModelCount })">
        <Icon name="cube" size="xs" />{{ namedModelCount || t('channelMonitorV2.settings.badgeAllModels') }}
      </span>
      <HelpTooltip width-class="w-80" interactive :aria-label="t('channelMonitorV2.settings.userContractTitle')">
        <template #trigger>
          <Icon name="infoCircle" size="sm" class="msp-info" />
        </template>
        <div class="space-y-1.5 text-xs">
          <p class="font-medium">{{ t('channelMonitorV2.settings.userContractTitle') }}</p>
          <ul class="list-disc space-y-0.5 pl-4">
            <li>{{ t('channelMonitorV2.settings.userContract.health') }}</li>
            <li>{{ t('channelMonitorV2.settings.userContract.trend') }}</li>
            <li>{{ t('channelMonitorV2.settings.userContract.latency') }}</li>
            <li>{{ t('channelMonitorV2.settings.userContract.models') }}</li>
          </ul>
        </div>
      </HelpTooltip>
      <button
        type="button"
        class="btn btn-primary"
        :class="{ 'msp-dirty': dirty }"
        :disabled="saving || !dirty"
        @click="save"
      >
        <Icon name="check" size="sm" />
        {{ t('channelMonitorV2.settings.save') }}
      </button>
    </div>

    <div v-if="!systemModeV2" class="msp-notice" role="status">
      <Icon name="exclamationTriangle" size="sm" />
      <span>
        {{
          t('channelMonitorV2.settings.modeBanner', {
            mode: systemModeLabel,
            modeV2: t('channelMonitorV2.settings.modeV2'),
          })
        }}
        <router-link class="font-medium underline" to="/admin/settings">{{ t('admin.settings.tabs.features') }}</router-link>
      </span>
    </div>

    <div v-if="loading" class="viz-grid" aria-busy="true">
      <div v-for="n in 4" :key="n" class="ui-card msp-skel-card" :class="n % 2 === 0 ? 'viz-span-5' : 'viz-span-7'">
        <span class="skeleton msp-skel"></span>
        <span class="skeleton msp-skel is-short"></span>
      </div>
    </div>

    <div v-else-if="draft" class="viz-grid">
      <section class="ui-card viz-span-7">
        <header class="viz-card-head">
          <h3 class="viz-card-title">{{ t('channelMonitorV2.settings.platformsTitle') }}</h3>
          <HelpTooltip interactive :aria-label="t('admin.routingUi.info')" :content="t('channelMonitorV2.settings.platformsHint')" />
        </header>
        <div class="viz-card-body msp-platforms">
          <div
            v-for="platform in draft.platforms"
            :key="platform.platform"
            class="msp-platform"
            :class="{ 'is-off': !platform.enabled }"
          >
            <Toggle v-model="platform.enabled" :aria-label="platformLabel(platform.platform)" />
            <ProviderMark :platform="platform.platform" :size="24" />
            <strong>{{ platformLabel(platform.platform) }}</strong>
            <input
              class="input msp-models"
              :value="platform.models.join(', ')"
              type="text"
              :placeholder="t('channelMonitorV2.settings.badgeAllModels')"
              :title="t('channelMonitorV2.settings.modelsPlaceholder')"
              :aria-label="`${platformLabel(platform.platform)} · ${t('channelMonitorV2.settings.modelsPlaceholder')}`"
              @change="setModels(platform, $event)"
            />
            <span
              class="badge"
              :class="platform.models.length ? 'badge-gray' : 'badge-primary'"
            >
              {{ platform.models.length ? t('channelMonitorV2.settings.badgeOther') : t('channelMonitorV2.settings.badgeAllModels') }}
            </span>
          </div>
        </div>
      </section>

      <section class="ui-card viz-span-5">
        <header class="viz-card-head">
          <h3 class="viz-card-title">{{ t('channelMonitorV2.settings.groupsTitle') }}</h3>
          <span class="badge" :class="draft.group_ids.length ? 'badge-primary' : 'badge-gray'">
            {{
              draft.group_ids.length
                ? t('channelMonitorV2.settings.groupsSelected', { count: draft.group_ids.length })
                : t('channelMonitorV2.settings.groupsAll')
            }}
          </span>
          <div class="viz-card-tools">
            <button
              v-if="draft.group_ids.length"
              type="button"
              class="btn btn-ghost btn-sm"
              @click="draft.group_ids = []"
            >
              {{ t('channelMonitorV2.settings.groupsAll') }}
            </button>
          </div>
        </header>
        <div class="viz-card-body max-h-[min(40vh,280px)] overflow-y-auto">
          <div class="msp-checks">
            <label v-for="group in groups" :key="group.id" class="msp-check">
              <input
                type="checkbox"
                :checked="draft.group_ids.includes(group.id)"
                @change="toggleGroup(group.id)"
              />
              <ProviderMark :platform="group.platform" :size="18" :label="platformLabel(group.platform)" />
              <span class="msp-check-name">{{ group.name }}</span>
              <small>#{{ group.id }}</small>
            </label>
          </div>
          <p v-if="groups.length === 0" class="msp-empty">{{ t('channelMonitorV2.settings.groupsEmpty') }}</p>
        </div>
      </section>

      <section class="ui-card viz-span-7">
        <header class="viz-card-head">
          <h3 class="viz-card-title">{{ t('channelMonitorV2.settings.errorsTitle') }}</h3>
          <HelpTooltip interactive :aria-label="t('admin.routingUi.info')" :content="t('channelMonitorV2.settings.errorsHint')" />
        </header>
        <div class="viz-card-body max-h-[min(40vh,320px)] overflow-y-auto">
          <div class="msp-checks">
            <label v-for="category in errorCategories" :key="category" class="msp-check" :title="category">
              <input
                type="checkbox"
                :checked="isCategoryIgnored(category)"
                @change="toggleIgnoredCategory(category)"
              />
              <span class="msp-check-name">{{ categoryLabel(category) }}</span>
            </label>
          </div>
        </div>
        <div class="viz-card-foot">
          {{
            t('channelMonitorV2.settings.ignoredSummary', {
              ignored: draft.ignored_error_categories?.length || 0,
              counted: countedErrorCategoryCount,
            })
          }}
        </div>
      </section>

      <section class="ui-card viz-span-5">
        <header class="viz-card-head">
          <h3 class="viz-card-title">{{ t('channelMonitorV2.settings.healthTitle') }}</h3>
          <HelpTooltip interactive :aria-label="t('admin.routingUi.info')" :content="t('channelMonitorV2.settings.healthHint')" />
        </header>
        <div class="viz-card-body msp-fields">
          <label>
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.minimumSample') }}</span>
            <input v-model.number="draft.health_thresholds.minimum_sample" class="input" type="number" min="1" max="10000" />
          </label>
          <label>
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.targetTtft') }}</span>
            <input v-model.number="draft.health_thresholds.target_ttft_ms" class="input" type="number" min="1" step="100" />
          </label>
          <label>
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.warningError') }}</span>
            <input v-model.number="warningErrorPercent" class="input" type="number" min="0" max="100" step="0.1" />
          </label>
          <label>
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.criticalError') }}</span>
            <input v-model.number="criticalErrorPercent" class="input" type="number" min="0" max="100" step="0.1" />
          </label>
          <label>
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.warningTtft') }}</span>
            <input v-model.number="draft.health_thresholds.warning_ttft_ms" class="input" type="number" min="1" step="100" />
          </label>
          <label>
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.criticalTtft') }}</span>
            <input v-model.number="draft.health_thresholds.critical_ttft_ms" class="input" type="number" min="1" step="100" />
          </label>
          <label>
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.warningCache') }}</span>
            <input v-model.number="warningCachePercent" class="input" type="number" min="0" max="100" step="0.1" />
          </label>
          <label>
            <span class="input-label">{{ t('channelMonitorV2.settings.fields.criticalCache') }}</span>
            <input v-model.number="criticalCachePercent" class="input" type="number" min="0" max="100" step="0.1" />
          </label>
        </div>
      </section>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { getChannelMonitorMode, isChannelMonitorV2Mode } from '@/utils/featureFlags'
import {
  getConfig,
  updateConfig,
  MONITOR_ERROR_CATEGORIES,
  type MonitorConfig,
} from '@/api/channelMonitorV2'
import { adminAPI } from '@/api/admin'
import type { AdminGroup } from '@/types'

const { t, te } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const saving = ref(false)
const draft = ref<MonitorConfig | null>(null)
const original = ref('')
const groups = ref<AdminGroup[]>([])

const dirty = computed(() => (draft.value ? JSON.stringify(draft.value) !== original.value : false))
const namedModelCount = computed(
  () => draft.value?.platforms.filter((p) => p.enabled).reduce((sum, p) => sum + p.models.length, 0) || 0
)
const errorCategories = MONITOR_ERROR_CATEGORIES
const countedErrorCategoryCount = computed(
  () => errorCategories.length - (draft.value?.ignored_error_categories?.length || 0)
)
/** System settings mode must be v2 for aggregation to run; config remains editable for prep. */
const systemModeV2 = computed(() => isChannelMonitorV2Mode())
const systemModeLabel = computed(() => {
  if (!appStore.cachedPublicSettings?.channel_monitor_enabled) {
    return t('channelMonitorV2.settings.modeClosed')
  }
  return getChannelMonitorMode() === 'v1'
    ? t('channelMonitorV2.settings.modeV1')
    : t('channelMonitorV2.settings.modeV2')
})
const defaultThresholds = {
  minimum_sample: 50,
  warning_error_rate: 0.05,
  critical_error_rate: 0.20,
  target_ttft_ms: 3000,
  warning_ttft_ms: 3000,
  critical_ttft_ms: 10000,
  // Higher is better: below 85% watch, below 60% critical.
  warning_cache_rate: 0.85,
  critical_cache_rate: 0.60,
  error_weight: 0.60,
  ttft_weight: 0.20,
  cache_weight: 0.20,
}

/** Factory ignored categories (matches backend DefaultChannelMonitorV2IgnoredErrorCategories). */
const defaultIgnoredErrorCategories = [
  'authentication',
  'client_cancelled',
  'content_policy',
  'context_limit',
  'group_access',
  'model_unsupported',
  'not_found',
  'quota_or_balance',
] as const
function percentModel(key: 'warning_error_rate' | 'critical_error_rate' | 'warning_cache_rate' | 'critical_cache_rate') {
  return computed({
    get: () => ((draft.value?.health_thresholds?.[key] ?? defaultThresholds[key]) * 100),
    set: (value: number) => {
      if (!draft.value) return
      draft.value.health_thresholds[key] = Math.max(0, Math.min(100, Number(value) || 0)) / 100
    },
  })
}
const warningErrorPercent = percentModel('warning_error_rate')
const criticalErrorPercent = percentModel('critical_error_rate')
const warningCachePercent = percentModel('warning_cache_rate')
const criticalCachePercent = percentModel('critical_cache_rate')

function setModels(platform: MonitorConfig['platforms'][number], event: Event) {
  platform.models = [
    ...new Set(
      (event.target as HTMLInputElement).value
        .split(',')
        .map((v) => v.trim())
        .filter(Boolean)
    ),
  ].sort()
}

function toggleGroup(id: number) {
  if (!draft.value) return
  draft.value.group_ids = draft.value.group_ids.includes(id)
    ? draft.value.group_ids.filter((value) => value !== id)
    : [...draft.value.group_ids, id].sort((a, b) => a - b)
}

function isCategoryIgnored(category: string): boolean {
  return Boolean(draft.value?.ignored_error_categories?.includes(category))
}

function toggleIgnoredCategory(category: string) {
  if (!draft.value) return
  const current = new Set(draft.value.ignored_error_categories || [])
  if (current.has(category)) current.delete(category)
  else current.add(category)
  draft.value.ignored_error_categories = [...current].sort()
}

function categoryLabel(category: string) {
  const key = `channelMonitorV2.errorCategories.${category}`
  return te(key) ? t(key) : category
}

function platformLabel(value: string) {
  return (
    {
      anthropic: 'Claude',
      openai: 'OpenAI',
      grok: 'Grok',
      kiro: 'Kiro',
      gemini: 'Gemini',
      antigravity: 'Antigravity',
      kimi: 'Kimi',
      zhipu: 'Zhipu GLM',
      deepseek: 'DeepSeek',
      minimax: 'MiniMax',
      composite: 'Composite',
    } as Record<string, string>
  )[value] || value
}

function normalizeConfig(value: MonitorConfig): MonitorConfig {
  const ignored = value.ignored_error_categories
  return {
    ...value,
    health_thresholds: { ...defaultThresholds, ...(value.health_thresholds || {}) },
    // Preserve explicit empty arrays from the server (operator cleared all).
    ignored_error_categories: [
      ...(ignored == null ? [...defaultIgnoredErrorCategories] : ignored),
    ].sort(),
  }
}

async function load() {
  loading.value = true
  try {
    const [value, groupRows] = await Promise.all([getConfig(), adminAPI.groups.getAllIncludingInactive()])
    const normalized = normalizeConfig(value)
    draft.value = structuredClone(normalized)
    groups.value = groupRows
    original.value = JSON.stringify(normalized)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('channelMonitorV2.settings.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function save() {
  if (!draft.value) return
  saving.value = true
  try {
    const payload = normalizeConfig(draft.value)
    const value = await updateConfig(payload)
    const normalized = normalizeConfig(value)
    draft.value = structuredClone(normalized)
    original.value = JSON.stringify(normalized)
    appStore.showSuccess(t('channelMonitorV2.settings.saveSuccess'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('channelMonitorV2.settings.saveFailed')))
    await load()
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.msp {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.msp-bar {
  flex-direction: row;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.625rem 0.75rem;
  padding: 0.75rem 1rem;
}

.msp-bar-title {
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 600;
}

.msp-divider {
  width: 1px;
  height: 1.25rem;
  background: var(--operator-border);
}

.msp-named {
  display: inline-flex;
  height: 1.5rem;
  align-items: center;
  gap: 0.3125rem;
  padding: 0 0.5rem;
  border-radius: 999px;
  background: var(--operator-muted);
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
}

.msp-info {
  color: var(--operator-subtle-foreground);
  cursor: help;
}

.msp-dirty {
  animation: ui-pop 360ms var(--operator-spring);
}

.msp-notice {
  display: flex;
  align-items: flex-start;
  gap: 0.625rem;
  padding: 0.75rem 1rem;
  border: 1px solid rgb(var(--operator-warning-rgb) / 0.35);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-warning-soft);
  color: var(--operator-foreground);
  font-size: 0.8125rem;
}

.msp-notice > :first-child {
  flex: none;
  margin-top: 0.125rem;
  color: var(--operator-warning);
}

.msp-skel-card {
  gap: 0.75rem;
  padding: 1.25rem;
}

.msp-skel {
  display: block;
  height: 0.875rem;
  border-radius: 999px;
}

.msp-skel.is-short {
  width: 60%;
}

.msp-platforms {
  display: grid;
  gap: 0.25rem;
}

.msp-platform {
  display: grid;
  grid-template-columns: auto auto 6.5rem minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  padding: 0.375rem 0;
  transition: opacity var(--operator-duration) var(--operator-ease-out);
}

.msp-platform.is-off > :not(:first-child) {
  opacity: 0.5;
}

.msp-platform strong {
  overflow: hidden;
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.msp-models {
  height: 2rem;
  font-size: 0.8125rem;
}

.msp-checks {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(12rem, 1fr));
  gap: 0.125rem;
}

.msp-check {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.625rem;
  padding: 0.4375rem 0.5rem;
  border-radius: var(--operator-radius);
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  cursor: pointer;
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.msp-check:hover {
  background: var(--operator-muted);
}

.msp-check input {
  width: 1rem;
  height: 1rem;
  flex: none;
  accent-color: rgb(var(--operator-accent-rgb));
}

.msp-check-name {
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.msp-check small {
  color: var(--operator-subtle-foreground);
  font-size: 0.6875rem;
}

.msp-empty {
  padding: 2rem 0;
  color: var(--operator-subtle-foreground);
  font-size: 0.875rem;
  text-align: center;
}

.msp-fields {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
}

@media (max-width: 640px) {
  .msp-platform {
    grid-template-columns: auto auto minmax(0, 1fr) auto;
  }

  .msp-models {
    grid-column: 1 / -1;
  }

  .msp-divider {
    display: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .msp-dirty {
    animation: none;
  }
}
</style>
