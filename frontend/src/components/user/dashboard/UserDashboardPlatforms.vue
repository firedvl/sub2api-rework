<template>
  <section v-if="platformCards.length > 0" class="viz-card" aria-labelledby="pd-platforms">
    <header class="viz-card-head">
      <h2 id="pd-platforms" class="viz-card-title">{{ t('dashboard.platformBreakdown') }}</h2>
      <span class="viz-card-tools text-xs text-gray-500">{{ t('dashboard.platformCount', { count: platformCount }) }}</span>
    </header>
    <ul class="viz-card-body pd-platforms">
      <li
        v-for="item in platformCards"
        :key="item.platform"
        data-testid="platform-card"
        :data-platform="item.platform"
        class="pd-platform"
        :class="{ 'is-other': item.isOther }"
      >
        <div class="pd-platform-row">
          <ProviderMark v-if="!item.isOther" :platform="item.platform" :size="28" />
          <span v-else class="pd-platform-other" aria-hidden="true"><Icon name="layers" size="xs" /></span>
          <span class="min-w-0 flex-1">
            <span class="ui-cell-main">{{ item.isOther ? t('dashboard.platformOther') : providerName(item.platform) }}</span>
            <span class="ui-cell-sub">
              {{ t('dashboard.requests') }} {{ item.total_requests > 0 ? formatInt(item.total_requests) : '–' }}
              · {{ t('dashboard.tokens') }} {{ item.total_tokens > 0 ? formatCompact(item.total_tokens) : '–' }}
            </span>
          </span>
          <span class="text-right">
            <b class="block font-semibold tabular-nums" :title="t('dashboard.actual')">${{ item.total_actual_cost.toFixed(2) }}</b>
            <small class="text-xs text-gray-500 tabular-nums" :title="t('dashboard.todayCost')">+${{ item.today_actual_cost.toFixed(2) }}</small>
          </span>
        </div>
        <div v-if="hasAnyLimit(item.quota) && !item.isOther" class="pd-quota">
          <template v-for="w in (['daily', 'weekly', 'monthly'] as const)" :key="w">
            <div
              v-if="quotaVal(item.quota, `${w}_limit_usd`) != null"
              class="pd-quota-row"
              :title="quotaVal(item.quota, `${w}_window_resets_at`) ? t('dashboard.platformQuota.resetsAt', { time: formatResetTime(quotaVal(item.quota, `${w}_window_resets_at`) as string) }) : undefined"
            >
              <span class="pd-quota-label">{{ t(`dashboard.platformQuota.${w}`) }}</span>
              <template v-if="(quotaVal(item.quota, `${w}_limit_usd`) as number) === 0">
                <span class="badge badge-danger">{{ t('dashboard.platformQuota.disabled') }}</span>
              </template>
              <template v-else>
                <QuotaMeter
                  :value="100 - calcPercent((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0, quotaVal(item.quota, `${w}_limit_usd`) as number)"
                  :label="t(`dashboard.platformQuota.${w}`)"
                />
                <span class="pd-quota-amount">${{ formatUsd((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0) }} / ${{ formatUsd(quotaVal(item.quota, `${w}_limit_usd`) as number) }}</span>
              </template>
            </div>
          </template>
        </div>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import QuotaMeter from '@/components/ui/QuotaMeter.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import { providerName } from '@/utils/providerSeries'
import { formatCompact, formatInt } from '@/utils/vizFormat'
import type { PlatformDashboardStats, UserDashboardStats as UserStatsType } from '@/api/usage'
import type { PlatformQuotaItem } from '@/types'

/** Per-platform spend with any configured platform quota windows. */
const props = defineProps<{
  stats: UserStatsType
  platformQuotas?: PlatformQuotaItem[] | null
}>()
const { t } = useI18n()

interface FusedPlatformCard {
  platform: string
  total_actual_cost: number
  today_actual_cost: number
  total_requests: number
  total_tokens: number
  isOther?: boolean
  quota?: PlatformQuotaItem
}



// 处理"各平台之和 < 总值"的差值：后端按平台聚合时过滤了无法归属平台的行
// （group 与 account 都缺 platform）。这里把差值作为"其他"卡片显式展示，
// 避免 Row 1 总值与 Row 3 平台拆分加总对不上、用户困惑。
const OTHER_THRESHOLD = 0.0001
const platformCards = computed<FusedPlatformCard[]>(() => {
  // 建立 by_platform Map
  const byPlat = new Map<string, PlatformDashboardStats>()
  for (const item of props.stats?.by_platform ?? []) byPlat.set(item.platform, item)

  // 建立 quota Map
  const byQuota = new Map<string, PlatformQuotaItem>()
  for (const q of props.platformQuotas ?? []) byQuota.set(q.platform, q)

  // union 平台集合。后端 by_platform / quota 接口均不会返回 platform='__other__'，
  // 无需显式排除；__other__ 由下方差值补差逻辑单独追加。
  const platforms = new Set<string>(byPlat.keys())
  for (const [platform, quota] of byQuota) {
    if (hasAnyLimit(quota)) platforms.add(platform)
  }

  const PLATFORM_ORDER = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go', 'typesafe']
  const cards: FusedPlatformCard[] = []

  for (const p of platforms) {
    const stat = byPlat.get(p)
    cards.push({
      platform: p,
      total_actual_cost: stat?.total_actual_cost ?? 0,
      today_actual_cost: stat?.today_actual_cost ?? 0,
      total_requests: stat?.total_requests ?? 0,
      total_tokens: stat?.total_tokens ?? 0,
      quota: byQuota.get(p),
    })
  }

  // 排序：按 PLATFORM_ORDER，未知平台按名称排序
  cards.sort((a, b) => {
    const ai = PLATFORM_ORDER.indexOf(a.platform)
    const bi = PLATFORM_ORDER.indexOf(b.platform)
    if (ai === -1 && bi === -1) return a.platform.localeCompare(b.platform)
    if (ai === -1) return 1
    if (bi === -1) return -1
    return ai - bi
  })

  // __other__ 补差逻辑：只对 by_platform 有 usage 数据的总和计算
  const total = props.stats?.total_actual_cost ?? 0
  const today = props.stats?.today_actual_cost ?? 0
  const sumTotal = cards.reduce((s, c) => s + c.total_actual_cost, 0)
  const sumToday = cards.reduce((s, c) => s + c.today_actual_cost, 0)
  const diffTotal = Math.max(0, total - sumTotal)
  const diffToday = Math.max(0, today - sumToday)

  if (diffTotal > OTHER_THRESHOLD || diffToday > OTHER_THRESHOLD) {
    cards.push({
      platform: '__other__',
      total_actual_cost: diffTotal,
      today_actual_cost: diffToday,
      total_requests: 0,
      total_tokens: 0,
      isOther: true,
    })
  }

  return cards
})

const platformCount = computed(() => platformCards.value.filter(card => !card.isOther).length)

// Quota helpers

type QuotaWindow = 'daily' | 'weekly' | 'monthly'
type QuotaField = `${QuotaWindow}_limit_usd` | `${QuotaWindow}_usage_usd` | `${QuotaWindow}_window_resets_at`

function quotaVal(q: PlatformQuotaItem | undefined, key: QuotaField): PlatformQuotaItem[QuotaField] {
  return q?.[key]
}

function hasAnyLimit(q: PlatformQuotaItem | undefined): boolean {
  if (!q) return false
  return q.daily_limit_usd != null || q.weekly_limit_usd != null || q.monthly_limit_usd != null
}

function calcPercent(usage: number, limit: number): number {
  if (!limit || limit <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((usage / limit) * 100)))
}

// 与 formatBalance 一致使用 Intl.NumberFormat 做半偶舍入，避免 toFixed 在不同 JS 引擎
// 下偶发截断而非四舍五入（与后端展示精度不一致）。
const usdFormatter = new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
})
function formatUsd(n: number): string {
  if (!Number.isFinite(n)) return '0.00'
  return usdFormatter.format(n)
}

function formatResetTime(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString(undefined, {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

</script>

<style scoped>
.pd-platforms {
  display: flex;
  flex-direction: column;
}

.pd-platform {
  padding: 0.625rem 0;
}

.pd-platform + .pd-platform {
  border-top: 1px solid var(--operator-border-subtle);
}

.pd-platform:first-child {
  padding-top: 0;
}

.pd-platform-row {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.625rem;
}

.pd-platform-other {
  display: inline-grid;
  width: 1.75rem;
  height: 1.75rem;
  flex: none;
  place-items: center;
  border: 1px dashed var(--operator-border-strong);
  border-radius: 0.5rem;
  color: var(--operator-subtle-foreground);
}

.pd-quota {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  margin: 0.5rem 0 0 2.375rem;
}

.pd-quota-row {
  display: grid;
  grid-template-columns: 4.5rem minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.75rem;
}

.pd-quota-label {
  overflow: hidden;
  color: var(--operator-muted-foreground);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pd-quota-amount {
  color: var(--operator-subtle-foreground);
  font-variant-numeric: tabular-nums;
}

@media (max-width: 480px) {
  .pd-quota {
    margin-left: 0;
  }

  .pd-quota-row {
    grid-template-columns: 3.75rem minmax(0, 1fr);
  }

  .pd-quota-amount {
    grid-column: 2;
  }
}
</style>
