<template>
  <SideDrawer
    :open="open && !!group"
    :title="group?.name ?? ''"
    :subtitle="subtitle"
    width="34rem"
    @close="$emit('close')"
  >
    <template #leading>
      <ProviderMark v-if="group" :platform="group.platform" :size="36" />
    </template>
    <template #badge>
      <span v-if="group" class="badge" :class="group.status === 'active' ? 'badge-success' : 'badge-gray'">
        {{ t('admin.accounts.status.' + group.status) }}
      </span>
    </template>

    <template v-if="group">
      <p v-if="group.description" class="route-desc">{{ group.description }}</p>

      <!-- Route: clients -> this group -> member accounts -->
      <section :aria-label="t('admin.routingUi.drawer.route')">
        <div class="ui-section-label">{{ t('admin.routingUi.drawer.route') }}</div>
        <div class="route-map" data-testid="group-route-map">
          <div class="route-node is-client" :title="t('admin.routingUi.drawer.clients')">
            <Icon name="key" size="sm" />
            <small>{{ t('admin.routingUi.drawer.clients') }}</small>
          </div>
          <svg class="route-wire" viewBox="0 0 32 12" aria-hidden="true">
            <path class="route-path is-ok" pathLength="1" d="M0 6H32" />
          </svg>
          <div class="route-node is-group">
            <ProviderMark :platform="group.platform" :size="28" />
            <small class="route-mult">{{ formatMultiplier(group.rate_multiplier) }}</small>
          </div>
          <svg
            class="route-fan"
            :viewBox="`0 0 40 ${fanHeight}`"
            :style="{ height: `${fanHeight}px` }"
            preserveAspectRatio="none"
            aria-hidden="true"
          >
            <path
              v-for="(member, index) in visibleMembers"
              :key="member.id"
              class="route-path"
              pathLength="1"
              :class="toneClass[member.tone]"
              :style="{ animationDelay: `${index * 50}ms` }"
              :d="`M0 ${fanHeight / 2} C20 ${fanHeight / 2} 20 ${index * ROW + ROW / 2} 40 ${index * ROW + ROW / 2}`"
            />
          </svg>
          <ul class="route-members" :style="{ minHeight: `${fanHeight}px` }">
            <template v-if="membersLoading">
              <li v-for="n in 3" :key="n" class="route-member"><span class="skeleton route-skel"></span></li>
            </template>
            <li v-else-if="!members.length" class="route-member is-empty">{{ t('admin.routingUi.drawer.noMembers') }}</li>
            <template v-else>
              <li v-for="member in visibleMembers" :key="member.id" class="route-member" :title="`${member.name} · ${t('admin.routingUi.tone.' + member.tone)}`">
                <span class="ui-dot" :class="dotClass[member.tone]"></span>
                <ProviderMark :platform="member.platform" :size="18" />
                <span class="route-member-name">{{ member.name }}</span>
              </li>
            </template>
          </ul>
        </div>
        <router-link
          v-if="members.length"
          class="viz-link route-more"
          :to="{ path: '/admin/accounts', query: { group: String(group.id) } }"
        >
          {{ hiddenMembers > 0 ? t('admin.routingUi.drawer.moreMembers', { count: hiddenMembers }) : t('admin.routingUi.drawer.openAccounts') }}
          <Icon name="arrowRight" size="xs" />
        </router-link>
      </section>

      <section>
        <div class="ui-section-label">{{ t('admin.routingUi.pool.title') }}</div>
        <PoolBar :split="split" large />
      </section>

      <section v-if="capacity" class="route-load">
        <div v-for="item in loadItems" :key="item.key" class="route-load-item">
          <small>{{ item.label }}</small>
          <b>{{ item.used }}<span>/{{ item.max }}</span></b>
          <QuotaMeter :value="item.left" :label="item.label" />
        </div>
      </section>

      <section>
        <div class="ui-section-label">{{ t('admin.routingUi.drawer.models') }}</div>
        <div v-if="models.length" class="route-models">
          <code v-for="model in shownModels" :key="model">{{ model }}</code>
          <button
            v-if="models.length > MODEL_LIMIT"
            type="button"
            class="ui-chip route-models-toggle"
            :aria-expanded="showAllModels"
            @click="showAllModels = !showAllModels"
          >
            {{ showAllModels ? t('admin.routingUi.drawer.fewer') : `+${models.length - MODEL_LIMIT}` }}
          </button>
        </div>
        <p v-else class="route-muted">{{ t('admin.routingUi.drawer.platformDefault') }}</p>
      </section>

      <section>
        <div class="ui-section-label">{{ t('admin.routingUi.drawer.billing') }}</div>
        <div v-if="limitMeters.length" class="route-limits">
          <div v-for="limit in limitMeters" :key="limit.key" class="route-limit">
            <span>{{ limit.label }}</span>
            <QuotaMeter :value="limit.left" :label="limit.label" />
            <small>{{ limit.text }}</small>
          </div>
        </div>
        <dl class="ui-dl">
          <dt>{{ t('admin.groups.columns.billingType') }}</dt>
          <dd>{{ group.subscription_type === 'subscription' ? t('admin.groups.subscription.subscription') : t('admin.groups.subscription.standard') }}</dd>
          <dt>{{ t('admin.groups.columns.rateMultiplier') }}</dt>
          <dd class="tabular-nums">{{ formatMultiplier(group.rate_multiplier) }}</dd>
          <dt>RPM</dt>
          <dd class="tabular-nums">{{ group.rpm_limit ? group.rpm_limit : '∞' }}</dd>
          <template v-if="usage">
            <dt>{{ t('admin.groups.usageToday') }}</dt>
            <dd class="tabular-nums">{{ formatUsd(usage.today_cost) }}</dd>
            <dt>{{ t('admin.groups.usageYesterday') }}</dt>
            <dd class="tabular-nums">{{ formatUsd(usage.yesterday_cost) }}</dd>
            <dt>{{ t('admin.groups.usageTotal') }}</dt>
            <dd class="tabular-nums">{{ formatUsd(usage.total_cost) }}</dd>
          </template>
          <dt>{{ t('admin.groups.columns.type') }}</dt>
          <dd>{{ group.is_exclusive ? t('admin.groups.exclusive') : t('admin.groups.public') }}</dd>
          <dt>{{ t('admin.groups.columns.id') }}</dt>
          <dd class="font-mono text-xs">#{{ group.id }}</dd>
        </dl>
      </section>
    </template>

    <template #footer>
      <template v-if="group">
        <button type="button" class="btn btn-primary btn-sm" @click="$emit('edit', group)">
          <Icon name="edit" size="sm" />{{ t('common.edit') }}
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="duplicating"
          @click="$emit('duplicate', group)"
        >
          <Icon name="copy" size="sm" />{{ duplicating ? t('admin.groups.duplicating') : t('admin.groups.duplicate') }}
        </button>
        <button
          v-if="group.platform === 'composite'"
          type="button"
          class="btn btn-secondary btn-sm"
          @click="$emit('composite', group)"
        >
          <Icon name="swap" size="sm" />{{ t('admin.groups.compositeRoutes.action') }}
        </button>
        <span class="route-foot-grow"></span>
        <ActionMenu :label="t('admin.routingUi.more')" :items="menuItems" trigger-class="app-icon-button route-foot-more" test-id="group-drawer-more" />
      </template>
    </template>
  </SideDrawer>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminGroup } from '@/types'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import QuotaMeter from '@/components/ui/QuotaMeter.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import Icon from '@/components/icons/Icon.vue'
import { providerName } from '@/utils/providerSeries'
import ActionMenu, { type ActionMenuItem } from './ActionMenu.vue'
import PoolBar from './PoolBar.vue'
import { memberTone, poolSplit, remainingPercent, servedModels, type Tone } from './routeSummary'

interface GroupUsage {
  today_cost: number
  yesterday_cost: number
  total_cost: number
}

interface GroupCapacity {
  concurrencyUsed: number
  concurrencyMax: number
  sessionsUsed: number
  sessionsMax: number
  rpmUsed: number
  rpmMax: number
}

const props = defineProps<{
  open: boolean
  group: AdminGroup | null
  usage?: GroupUsage | null
  capacity?: GroupCapacity | null
  duplicating?: boolean
}>()

const emit = defineEmits<{
  close: []
  edit: [group: AdminGroup]
  duplicate: [group: AdminGroup]
  composite: [group: AdminGroup]
  rateMultipliers: [group: AdminGroup]
  rpmOverrides: [group: AdminGroup]
  delete: [group: AdminGroup]
}>()

const { t } = useI18n()

const ROW = 32
const MEMBER_LIMIT = 7
const MODEL_LIMIT = 12
const toneClass: Record<Tone, string> = { ok: 'is-ok', warn: 'is-warn', bad: 'is-bad', idle: 'is-idle' }
const dotClass: Record<Tone, string> = { ok: 'is-ok', warn: 'is-warn', bad: 'is-bad', idle: '' }

interface Member {
  id: number
  name: string
  platform: string
  tone: Tone
}

const members = ref<Member[]>([])
const membersLoading = ref(false)
const showAllModels = ref(false)
let request = 0

const toneOrder: Record<Tone, number> = { bad: 0, warn: 1, ok: 2, idle: 3 }

async function loadMembers(group: AdminGroup) {
  const id = ++request
  members.value = []
  membersLoading.value = true
  try {
    const page = await adminAPI.accounts.list(1, 100, { group: String(group.id), lite: '1' })
    if (id !== request) return
    members.value = page.items
      .filter((account) => !account.group_ids || account.group_ids.includes(group.id))
      .map((account) => ({ id: account.id, name: account.name, platform: account.platform, tone: memberTone(account) }))
      .sort((a, b) => toneOrder[a.tone] - toneOrder[b.tone] || a.name.localeCompare(b.name))
  } catch (error) {
    if (id === request) members.value = []
    console.error('Error loading group members:', error)
  } finally {
    if (id === request) membersLoading.value = false
  }
}

watch(
  () => (props.open ? props.group?.id : undefined),
  (id) => {
    showAllModels.value = false
    if (id != null && props.group) void loadMembers(props.group)
  },
  { immediate: true },
)

const visibleMembers = computed(() => members.value.slice(0, MEMBER_LIMIT))
const hiddenMembers = computed(() => Math.max(0, members.value.length - MEMBER_LIMIT))
const fanHeight = computed(() => Math.max(1, membersLoading.value ? 3 : visibleMembers.value.length || 1) * ROW)

const split = computed(() => poolSplit(props.group ?? {}))
const models = computed(() => (props.group ? servedModels(props.group) : []))
const shownModels = computed(() => (showAllModels.value ? models.value : models.value.slice(0, MODEL_LIMIT)))

const subtitle = computed(() => {
  const group = props.group
  if (!group) return ''
  const platform = t(`admin.groups.platforms.${group.platform}`) || providerName(group.platform)
  const billing = group.subscription_type === 'subscription'
    ? t('admin.groups.subscription.subscription')
    : t('admin.routingUi.billing.balance')
  return `${platform} · ${billing}`
})

const formatUsd = (value: number | null | undefined) => {
  const cost = value ?? 0
  return `$${cost >= 1000 ? cost.toFixed(0) : cost >= 100 ? cost.toFixed(1) : cost.toFixed(2)}`
}
const formatMultiplier = (value: number | null | undefined) => `${Number(value ?? 1)}×`

const loadItems = computed(() => {
  const capacity = props.capacity
  if (!capacity) return []
  const item = (key: string, label: string, used: number, max: number) => ({
    key,
    label,
    used,
    max,
    left: max > 0 ? Math.max(0, Math.round((1 - used / max) * 100)) : null,
  })
  return [
    item('concurrency', t('admin.routingUi.load.concurrency'), capacity.concurrencyUsed, capacity.concurrencyMax),
    ...(capacity.sessionsMax > 0 ? [item('sessions', t('admin.routingUi.load.sessions'), capacity.sessionsUsed, capacity.sessionsMax)] : []),
    ...(capacity.rpmMax > 0 ? [item('rpm', 'RPM', capacity.rpmUsed, capacity.rpmMax)] : []),
  ]
})

const limitMeters = computed(() => {
  const group = props.group
  if (!group || group.subscription_type !== 'subscription') return []
  const today = props.usage?.today_cost ?? 0
  const rows: { key: string; label: string; left: number | null; text: string }[] = []
  if (group.daily_limit_usd) {
    rows.push({
      key: 'day',
      label: t('admin.routingUi.billing.day'),
      left: remainingPercent(today, group.daily_limit_usd),
      text: `${props.usage ? formatUsd(today) : '—'} / ${formatUsd(group.daily_limit_usd)}`,
    })
  }
  if (group.weekly_limit_usd) {
    rows.push({ key: 'week', label: t('admin.routingUi.billing.week'), left: null, text: formatUsd(group.weekly_limit_usd) })
  }
  if (group.monthly_limit_usd) {
    rows.push({ key: 'month', label: t('admin.routingUi.billing.month'), left: null, text: formatUsd(group.monthly_limit_usd) })
  }
  return rows
})

const menuItems = computed<ActionMenuItem[]>(() => {
  const group = props.group
  if (!group) return []
  return [
    { key: 'rates', label: t('admin.groups.rateMultipliers'), icon: 'dollar', action: () => emit('rateMultipliers', group) },
    { key: 'rpm', label: t('admin.groups.rpmOverrides'), icon: 'bolt', action: () => emit('rpmOverrides', group) },
    { key: 'delete', label: t('common.delete'), icon: 'trash', danger: true, action: () => emit('delete', group) },
  ]
})
</script>

<style scoped>
.route-desc {
  color: var(--operator-muted-foreground);
  font-size: 0.875rem;
}

.route-map {
  display: grid;
  grid-template-columns: auto 2rem auto 2.5rem minmax(0, 1fr);
  align-items: center;
}

.route-node {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.25rem;
  padding: 0.5rem 0.625rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-raised);
  color: var(--operator-muted-foreground);
}

.route-node small {
  font-size: 0.6875rem;
  font-variant-numeric: tabular-nums;
}

.route-node.is-group {
  border-color: rgb(var(--operator-accent-rgb) / 0.5);
  background: var(--operator-accent-soft);
  box-shadow: 0 0 0 4px rgb(var(--operator-accent-rgb) / 0.08);
}

.route-mult {
  color: var(--operator-foreground);
  font-weight: 600;
}

.route-wire {
  width: 100%;
  height: 12px;
}

.route-fan {
  width: 100%;
}

.route-path {
  fill: none;
  stroke: var(--operator-border-strong);
  stroke-width: 1.5;
  stroke-dasharray: 1;
  stroke-dashoffset: 1;
  animation: route-draw 700ms var(--operator-ease-out) forwards;
}

.route-path.is-ok { stroke: rgb(var(--operator-success-rgb) / 0.7); }
.route-path.is-warn { stroke: rgb(var(--operator-warning-rgb) / 0.75); }
.route-path.is-bad { stroke: rgb(var(--operator-destructive-rgb) / 0.7); stroke-dasharray: 0.035 0.035; stroke-dashoffset: 0; animation: none; }
.route-path.is-idle { stroke: var(--operator-border-strong); stroke-dasharray: 0.035 0.035; stroke-dashoffset: 0; animation: none; }

@keyframes route-draw {
  to { stroke-dashoffset: 0; }
}

.route-members {
  display: flex;
  min-width: 0;
  flex-direction: column;
  justify-content: center;
}

.route-member {
  display: flex;
  height: 32px;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
  padding-left: 0.375rem;
  color: var(--operator-foreground);
  font-size: 0.8125rem;
}

.route-member.is-empty {
  color: var(--operator-subtle-foreground);
}

.route-member-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.route-skel {
  width: 100%;
  height: 0.875rem;
  border-radius: 999px;
}

.route-more {
  margin-top: 0.5rem;
}

.route-load {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
  gap: 0.75rem;
}

.route-load-item {
  display: grid;
  gap: 0.25rem;
  padding: 0.625rem 0.75rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
}

.route-load-item small {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.route-load-item b {
  color: var(--operator-foreground);
  font-size: 1.125rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.route-load-item b span {
  color: var(--operator-subtle-foreground);
  font-size: 0.8125rem;
  font-weight: 400;
}

.route-models {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}

.route-models code {
  padding: 0.1875rem 0.5rem;
  border: 1px solid var(--operator-border);
  border-radius: 999px;
  background: var(--operator-raised);
  color: var(--operator-foreground);
  font-size: 0.75rem;
}

.route-models-toggle {
  height: 1.625rem;
  font-size: 0.75rem;
}

.route-muted {
  color: var(--operator-subtle-foreground);
  font-size: 0.875rem;
}

.route-limits {
  display: grid;
  gap: 0.5rem;
  margin-bottom: 0.875rem;
}

.route-limit {
  display: grid;
  grid-template-columns: 3.5rem minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.8125rem;
}

.route-limit > span {
  color: var(--operator-muted-foreground);
}

.route-limit small {
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
}

.route-foot-grow {
  flex: 1 1 auto;
}

:deep(.route-foot-more) {
  border: 1px solid var(--operator-border);
}

@media (prefers-reduced-motion: reduce) {
  .route-path {
    stroke-dashoffset: 0;
    animation: none;
  }
}
</style>
