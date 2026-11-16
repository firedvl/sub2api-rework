<template>
  <AppLayout>
    <div v-if="loading && !stats" class="viz-grid" role="status" :aria-label="t('common.loading')">
      <div class="skeleton viz-span-12 h-48 rounded-xl"></div>
      <div class="skeleton viz-span-8 h-72 rounded-xl"></div>
      <div class="skeleton viz-span-4 h-72 rounded-xl"></div>
    </div>
    <div v-else-if="stats" class="viz-grid ui-stagger">
      <UserDashboardStats class="viz-span-12" :stats="stats" :balance="user?.balance || 0" :is-simple="authStore.isSimpleMode">
        <template #actions><UserDashboardQuickActions /></template>
      </UserDashboardStats>
      <UserDashboardCharts v-model:startDate="startDate" v-model:endDate="endDate" v-model:granularity="granularity" :loading="loadingCharts" :trend="trendData" :models="modelStats" @dateRangeChange="loadCharts" @granularityChange="loadCharts" @refresh="refreshAll" />
      <UserDashboardRecentUsage :class="showPlatforms ? 'viz-span-8' : 'viz-span-12'" :data="recentUsage" :loading="loadingUsage" />
      <UserDashboardPlatforms v-if="showPlatforms" class="viz-span-4" :stats="stats" :platform-quotas="platformQuotas" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'; import { useAuthStore } from '@/stores/auth'; import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import AppLayout from '@/components/layout/AppLayout.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'; import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'; import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import UserDashboardPlatforms from '@/components/user/dashboard/UserDashboardPlatforms.vue'
import { useI18n } from 'vue-i18n'
import type { UsageLog, TrendDataPoint, ModelStat, PlatformQuotaItem } from '@/types'
import { getMyPlatformQuotas } from '@/api/user'
import { formatDateLocalInput } from '@/utils/format'

const { t } = useI18n()
const authStore = useAuthStore(); const user = computed(() => authStore.user)
const stats = ref<UserStatsType | null>(null); const loading = ref(false); const loadingUsage = ref(false); const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([]); const modelStats = ref<ModelStat[]>([]); const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)
// Platform rows need usage or a configured quota; simple mode hides spend entirely.
const showPlatforms = computed(() => !authStore.isSimpleMode && (
  (stats.value?.by_platform?.length ?? 0) > 0 ||
  (platformQuotas.value ?? []).some((q) => q.daily_limit_usd != null || q.weekly_limit_usd != null || q.monthly_limit_usd != null)
))

const startDate = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000))); const endDate = ref(formatDateLocalInput(new Date())); const granularity = ref('day')

const loadStats = async () => { loading.value = true; try { await authStore.refreshUser(); stats.value = await usageAPI.getDashboardStats() } catch (error) { console.error('Failed to load dashboard stats:', error) } finally { loading.value = false } }
const loadCharts = async () => { loadingCharts.value = true; try { const res = await Promise.all([usageAPI.getDashboardTrend({ start_date: startDate.value, end_date: endDate.value, granularity: granularity.value as any }), usageAPI.getDashboardModels({ start_date: startDate.value, end_date: endDate.value })]); trendData.value = res[0].trend || []; modelStats.value = res[1].models || [] } catch (error) { console.error('Failed to load charts:', error) } finally { loadingCharts.value = false } }
const loadRecent = async () => { loadingUsage.value = true; try { const res = await usageAPI.getByDateRange(startDate.value, endDate.value); recentUsage.value = res.items.slice(0, 5) } catch (error) { console.error('Failed to load recent usage:', error) } finally { loadingUsage.value = false } }
const loadPlatformQuotas = async () => { try { const data = await getMyPlatformQuotas(); platformQuotas.value = data.platform_quotas ?? [] } catch (error) { console.warn('Failed to load platform quotas:', error); platformQuotas.value = [] } }
const refreshAll = () => { loadStats(); loadCharts(); loadRecent(); loadPlatformQuotas() }

onMounted(() => { refreshAll() })
</script>
