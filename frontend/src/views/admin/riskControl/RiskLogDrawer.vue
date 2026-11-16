<template>
  <SideDrawer
    :open="row !== null"
    :title="row?.user_email || t('admin.riskControl.inputDetailTitle')"
    :subtitle="row ? formatDateTime(row.created_at) : ''"
    width="34rem"
    @close="emit('close')"
  >
    <template v-if="row?.provider" #leading>
      <ProviderMark :platform="row.provider" :size="36" :label="row.provider" />
    </template>
    <template v-if="row" #badge>
      <span :class="resultBadgeClass(row)">{{ resultLabel(row, t) }}</span>
    </template>

    <div v-if="row" class="rc-drawer">
      <p v-if="row.matched_keyword" class="rc-keyword" :title="row.matched_keyword">
        <Icon name="exclamationTriangle" size="sm" />
        <span>{{ t('admin.riskControl.matchedKeyword') }}: <b>{{ row.matched_keyword }}</b></span>
      </p>

      <section>
        <div class="ui-section-label">{{ t('admin.riskControl.inputDetailContent') }}</div>
        <pre class="rc-input">{{ inputText }}</pre>
      </section>

      <section v-if="scoreRows.length > 0">
        <div class="ui-section-label">{{ t('admin.adminUi.riskControl.categoryScores') }}</div>
        <ul class="rc-scores">
          <li v-for="score in scoreRows" :key="score.category">
            <span class="truncate">{{ score.category }}</span>
            <div :class="score.hit ? 'ui-meter is-bad' : 'ui-meter'">
              <div class="ui-meter-track"><i :style="{ width: percentWidth(score.score) }"></i></div>
            </div>
            <span class="rc-score-value">{{ percent(score.score) }}<template v-if="score.threshold !== null"> / {{ percent(score.threshold) }}</template></span>
          </li>
        </ul>
      </section>

      <section>
        <div class="ui-section-label">{{ t('admin.adminUi.details') }}</div>
        <dl class="ui-dl">
          <dt>{{ t('admin.riskControl.table.user') }}</dt>
          <dd>{{ row.user_email || '-' }}<span v-if="row.user_id" class="rc-muted"> · UID {{ row.user_id }}</span></dd>
          <dt>{{ t('admin.riskControl.table.group') }}</dt>
          <dd>{{ row.group_name || '-' }}</dd>
          <dt>{{ t('admin.riskControl.table.apiKey') }}</dt>
          <dd>{{ row.api_key_name || '-' }}</dd>
          <dt>{{ t('admin.riskControl.table.endpoint') }}</dt>
          <dd class="font-mono text-xs">{{ row.endpoint || '-' }}</dd>
          <dt>{{ t('admin.riskControl.model') }}</dt>
          <dd>{{ row.provider || '-' }} / {{ row.model || '-' }}</dd>
          <dt>{{ t('admin.riskControl.table.highest') }}</dt>
          <dd>{{ row.highest_category || '-' }} / {{ percent(row.highest_score) }}</dd>
          <dt>{{ t('admin.riskControl.table.actionMeta') }}</dt>
          <dd>
            {{ violationCountText(row, t) }}
            <span class="rc-muted"> · {{ row.email_sent ? t('admin.riskControl.emailSent') : t('admin.riskControl.emailNotSent') }}</span>
            <span v-if="row.auto_banned" class="rc-banned"> · {{ t('admin.riskControl.autoBanned') }}</span>
          </dd>
          <dt>{{ t('admin.riskControl.table.latency') }}</dt>
          <dd>
            {{ latencyText(row.upstream_latency_ms) }}
            <span v-if="row.queue_delay_ms !== null && row.queue_delay_ms !== undefined" class="rc-muted"> · {{ t('admin.riskControl.queueDelay', { ms: row.queue_delay_ms }) }}</span>
          </dd>
          <dt>{{ t('admin.riskControl.auditSource') }}</dt>
          <dd data-test="audit-engine-meta">
            <template v-if="row.engine_meta">
              {{ engineLabel(row.engine_meta.engine) }} · {{ row.engine_meta.model || '-' }} · {{ row.engine_meta.rules_version || '-' }}
              <span v-if="row.engine_meta.skipped_images"> · {{ t('admin.riskControl.skippedImages', { count: row.engine_meta.skipped_images }) }}</span>
            </template>
            <template v-else>{{ ['cyber_policy', 'keyword_block', 'hash_block'].includes(row.action) ? '-' : t('admin.riskControl.legacyAuditSource') }}</template>
          </dd>
          <template v-if="row.request_id">
            <dt>{{ t('admin.adminUi.riskControl.requestId') }}</dt>
            <dd class="font-mono text-xs">{{ row.request_id }}</dd>
          </template>
        </dl>
      </section>
    </div>

    <template #footer>
      <button
        v-if="row && canUnbanRow(row)"
        type="button"
        class="btn btn-primary"
        :disabled="unbanning"
        @click="emit('unban', row)"
      >
        <Icon name="checkCircle" size="sm" :class="unbanning ? 'animate-spin' : ''" />
        {{ unbanning ? t('common.processing') : t('admin.riskControl.unbanUser') }}
      </button>
      <span class="flex-1"></span>
      <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.close') }}</button>
    </template>
  </SideDrawer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import Icon from '@/components/icons/Icon.vue'
import type { ContentModerationLog } from '@/api/admin/riskControl'
import {
  canUnbanRow,
  engineLabel,
  formatDateTime,
  latencyText,
  percent,
  percentWidth,
  resultBadgeClass,
  resultLabel,
  violationCountText,
} from './riskControlFormat'

/** Read-only detail for one audit record; unban reuses the parent's handler. */
const props = defineProps<{
  row: ContentModerationLog | null
  unbanning: boolean
}>()
const emit = defineEmits<{ close: []; unban: [row: ContentModerationLog] }>()
const { t } = useI18n()

const inputText = computed(() => props.row?.input_excerpt || props.row?.error || '-')

const scoreRows = computed(() => {
  const row = props.row
  if (!row?.category_scores) return []
  return Object.entries(row.category_scores)
    .map(([category, score]) => {
      const threshold = row.threshold_snapshot?.[category]
      return {
        category,
        score,
        threshold: Number.isFinite(threshold) ? Number(threshold) : null,
        hit: Number.isFinite(threshold) && score >= Number(threshold),
      }
    })
    .sort((a, b) => b.score - a.score)
    .slice(0, 8)
})
</script>

<style scoped>
.rc-drawer {
  display: grid;
  gap: 1.25rem;
}

.rc-keyword {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.625rem 0.75rem;
  border-radius: var(--operator-radius);
  background: var(--operator-destructive-soft);
  color: var(--operator-destructive);
  font-size: 0.8125rem;
  overflow-wrap: anywhere;
}

.rc-input {
  max-height: 22rem;
  overflow: auto;
  padding: 0.75rem 0.875rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.rc-scores {
  display: grid;
  gap: 0.375rem;
}

.rc-scores li {
  display: grid;
  grid-template-columns: minmax(0, 9rem) minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
  color: var(--operator-muted-foreground);
  font-size: 0.78rem;
}

.rc-score-value {
  color: var(--operator-foreground);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.75rem;
}

.rc-muted {
  color: var(--operator-subtle-foreground);
}

.rc-banned {
  color: var(--operator-destructive);
}
</style>
