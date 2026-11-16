<template>
  <AppLayout>
    <div class="ui-page ui-stagger">
      <div class="ui-toolbar">
        <label class="ui-search">
          <Icon name="search" size="sm" />
          <span class="sr-only">{{ t('availableChannels.searchPlaceholder') }}</span>
          <input
            v-model="searchQuery"
            type="search"
            :placeholder="t('availableChannels.searchPlaceholder')"
          />
        </label>
        <div
          v-if="platforms.length > 1"
          class="ui-chips"
          role="group"
          :aria-label="t('availableChannels.columns.platform')"
        >
          <button
            type="button"
            class="ui-chip"
            :aria-pressed="platformFilter === ''"
            @click="platformFilter = ''"
          >
            {{ t('common.all') }}
          </button>
          <button
            v-for="item in platforms"
            :key="item.platform"
            type="button"
            class="ui-chip"
            :aria-pressed="platformFilter === item.platform"
            :aria-label="platformLabel(item.platform)"
            :title="platformLabel(item.platform)"
            :data-testid="`channel-platform-${item.platform}`"
            @click="platformFilter = platformFilter === item.platform ? '' : item.platform"
          >
            <ProviderMark :platform="item.platform" :size="18" />
            <span class="ui-chip-count">{{ item.count }}</span>
          </button>
        </div>
        <span class="ui-grow"></span>
        <button
          type="button"
          class="app-icon-button"
          :disabled="loading"
          :title="t('common.refresh', 'Refresh')"
          :aria-label="t('common.refresh', 'Refresh')"
          @click="loadChannels"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>

      <section class="ui-card overflow-hidden">
        <AvailableChannelsTable
          :columns="columnLabels"
          :rows="filteredChannels"
          :loading="loading"
          :user-group-rates="userGroupRates"
          pricing-key-prefix="availableChannels.pricing"
          :no-pricing-label="t('availableChannels.noPricing')"
          :no-models-label="t('availableChannels.noModels')"
          :empty-label="t('availableChannels.empty')"
        />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import AvailableChannelsTable from '@/components/channels/AvailableChannelsTable.vue'
import userChannelsAPI, { type UserAvailableChannel } from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { platformLabel } from '@/utils/platformColors'

const { t } = useI18n()
const appStore = useAppStore()

const channels = ref<UserAvailableChannel[]>([])
const userGroupRates = ref<Record<number, number>>({})
const loading = ref(false)
const searchQuery = ref('')
const platformFilter = ref('')

/** Platforms present across channels, with how many channels serve each. */
const platforms = computed(() => {
  const counts = new Map<string, number>()
  for (const channel of channels.value) {
    for (const platform of new Set(channel.platforms.map((section) => section.platform))) {
      counts.set(platform, (counts.get(platform) ?? 0) + 1)
    }
  }
  return [...counts.entries()].map(([platform, count]) => ({ platform, count }))
})

watch(platforms, (list) => {
  if (platformFilter.value && !list.some((item) => item.platform === platformFilter.value)) platformFilter.value = ''
})

const platformChannels = computed(() => {
  const platform = platformFilter.value
  if (!platform) return channels.value
  return channels.value
    .map((ch) => ({ ...ch, platforms: ch.platforms.filter((section) => section.platform === platform) }))
    .filter((ch) => ch.platforms.length > 0)
})

const columnLabels = computed(() => ({
  name: t('availableChannels.columns.name'),
  description: t('availableChannels.columns.description'),
  platform: t('availableChannels.columns.platform'),
  groups: t('availableChannels.columns.groups'),
  supportedModels: t('availableChannels.columns.supportedModels'),
}))

/**
 * 搜索过滤：
 * - 命中渠道名/描述 → 整个渠道（所有 platforms）都保留
 * - 否则按 platform/group/model 维度在 sections 里过滤，保留有匹配的 section
 * - 所有 sections 都不匹配时，渠道本身被过滤掉
 */
const filteredChannels = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return platformChannels.value
  return platformChannels.value
    .map((ch) => {
      const nameHit = ch.name.toLowerCase().includes(q)
      const descHit = (ch.description || '').toLowerCase().includes(q)
      if (nameHit || descHit) return ch
      const matchingSections = ch.platforms.filter(
        (p) =>
          p.platform.toLowerCase().includes(q) ||
          p.groups.some((g) => g.name.toLowerCase().includes(q)) ||
          p.supported_models.some((m) => m.name.toLowerCase().includes(q)),
      )
      if (matchingSections.length === 0) return null
      return { ...ch, platforms: matchingSections }
    })
    .filter((ch): ch is UserAvailableChannel => ch !== null)
})

async function loadChannels() {
  loading.value = true
  try {
    // 渠道列表和用户专属倍率并发拉取。专属倍率失败不阻塞渠道展示——
    // 失败时只是无法渲染专属倍率角标，降级为仅显示默认倍率。
    const [list, rates] = await Promise.all([
      userChannelsAPI.getAvailable(),
      userGroupsAPI.getUserGroupRates().catch((err: unknown) => {
        console.error('Failed to load user group rates:', err)
        return {} as Record<number, number>
      }),
    ])
    channels.value = list
    userGroupRates.value = rates
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(loadChannels)
</script>
