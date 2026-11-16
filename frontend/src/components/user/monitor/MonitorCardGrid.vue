<template>
  <div>
    <div
      v-if="loading && items.length === 0"
      class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
      aria-busy="true"
    >
      <div v-for="i in 6" :key="i" class="ui-card gap-3 p-4">
        <div class="flex items-center gap-3">
          <div class="skeleton h-8 w-8 rounded-lg"></div>
          <div class="skeleton h-4 flex-1 rounded"></div>
        </div>
        <div class="skeleton h-7 w-1/3 rounded"></div>
        <div class="skeleton h-5 w-full rounded"></div>
      </div>
    </div>

    <div v-else-if="items.length === 0" class="ui-card">
      <div class="ui-empty">
        <span class="ui-empty-icon"><Icon name="signal" size="lg" /></span>
        <b>{{ t('channelStatus.empty.title') }}</b>
        <span class="text-xs">{{ t('channelStatus.empty.description') }}</span>
      </div>
    </div>

    <div
      v-else
      class="ui-stagger grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
    >
      <MonitorCard
        v-for="item in items"
        :key="item.id"
        :item="item"
        :window="window"
        :availability-value="resolveAvailability(item)"
        :countdown-seconds="countdownSeconds"
        @click="emit('cardClick', item)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { UserMonitorView, UserMonitorDetail } from '@/api/channelMonitor'
import Icon from '@/components/icons/Icon.vue'
import MonitorCard from './MonitorCard.vue'

const props = defineProps<{
  items: UserMonitorView[]
  window: '7d' | '15d' | '30d'
  countdownSeconds: number
  loading: boolean
  detailCache: Record<number, UserMonitorDetail>
}>()

const emit = defineEmits<{
  (e: 'cardClick', item: UserMonitorView): void
}>()

const { t } = useI18n()

function resolveAvailability(item: UserMonitorView): number | null {
  if (props.window === '7d') {
    return item.availability_7d ?? null
  }
  const detail = props.detailCache[item.id]
  if (!detail) return null
  const primary = detail.models.find(m => m.model === item.primary_model)
  if (!primary) return null
  return props.window === '15d' ? primary.availability_15d ?? null : primary.availability_30d ?? null
}
</script>
