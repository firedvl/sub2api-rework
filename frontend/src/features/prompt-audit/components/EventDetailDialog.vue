<template>
  <SideDrawer
    :open="show"
    :title="t('admin.promptAudit.events.detailTitle')"
    :subtitle="event ? formatDate(event.created_at) : undefined"
    width="44rem"
    @close="$emit('close')"
  >
    <template #badge>
      <span v-if="event" class="badge whitespace-nowrap" :class="decisionClass(event.decision)">{{ formatDecisionAction(event.decision, event.action) }}</span>
    </template>

    <div v-if="loading" class="space-y-3" aria-busy="true">
      <div class="skeleton h-9 w-64" />
      <div class="skeleton h-40" />
      <div class="skeleton h-24" />
    </div>
    <div v-else-if="event" class="flex flex-col gap-5">
      <div class="viz-seg self-start" role="group" data-test="event-detail-tabs" :aria-label="t('admin.promptAudit.events.detailTitle')">
        <button v-for="tab in tabs" :key="tab" type="button" :aria-pressed="activeTab === tab" :data-test="`detail-tab-${tab}`" @click="activeTab = tab">
          {{ t(`admin.promptAudit.events.tabs.${tab}`) }}
        </button>
      </div>

      <div data-test="event-detail-tab-panel">
        <div v-show="activeTab === 'summary'" class="space-y-5">
          <dl class="ui-dl">
            <dt>{{ t('admin.promptAudit.events.decision') }}</dt><dd class="font-medium">{{ formatDecisionAction(event.decision, event.action) }}</dd>
            <dt>{{ t('admin.promptAudit.events.user') }}</dt><dd><CopyValue :value="event.snapshot.username" :label="t('admin.promptAudit.events.user')" /></dd>
            <dt>{{ t('admin.promptAudit.events.email') }}</dt><dd><CopyValue :value="event.snapshot.user_email" :label="t('admin.promptAudit.events.email')" /></dd>
            <dt>{{ t('admin.promptAudit.events.apiKey') }}</dt><dd><CopyValue :value="event.snapshot.api_key_name" :label="t('admin.promptAudit.events.apiKey')" /></dd>
            <dt>{{ t('admin.promptAudit.events.group') }}</dt><dd>{{ event.snapshot.group_name || '—' }}</dd>
            <dt>{{ t('admin.promptAudit.events.model') }}</dt>
            <dd class="flex items-center gap-1.5">
              <ProviderMark v-if="event.snapshot.provider" :platform="event.snapshot.provider" :size="16" :label="event.snapshot.provider" />
              {{ event.snapshot.model || '—' }}
            </dd>
            <dt>{{ t('admin.promptAudit.events.categories') }}</dt><dd>{{ formatCategories(event.categories) }}</dd>
          </dl>
          <section>
            <h3 class="ui-section-label">{{ t('admin.promptAudit.events.promptFull') }}</h3>
            <pre class="pa-pre max-h-[min(46vh,26rem)]" data-test="summary-prompt-full">{{ displayPrompt(event) }}</pre>
          </section>
        </div>

        <div v-show="activeTab === 'risks'" class="space-y-5">
          <section data-test="risk-prompt-preview">
            <h3 class="ui-section-label flex items-center">
              {{ t('admin.promptAudit.events.promptFull') }}
              <HelpTooltip :content="t('admin.promptAudit.events.promptFullHint')" />
            </h3>
            <pre class="pa-pre max-h-[min(40vh,22rem)] overflow-auto" data-test="risk-prompt-full">{{ displayPrompt(event) }}</pre>
          </section>
          <section data-test="risk-guard-return">
            <h3 class="ui-section-label flex items-center">
              {{ t('admin.promptAudit.events.guardReturn') }}
              <HelpTooltip :content="t('admin.promptAudit.events.guardReturnHint')" />
            </h3>
            <pre class="pa-pre max-h-[min(40vh,22rem)] font-mono text-xs">{{ formatGuardReturn(event) }}</pre>
          </section>
          <section class="space-y-3">
            <h3 class="ui-section-label">{{ t('admin.promptAudit.events.riskSummaries') }}</h3>
            <article v-for="issue in event.issue_summaries" :key="`${issue.scanner_id}-${issue.code}`" class="pa-issue" data-test="risk-issue">
              <div class="flex flex-wrap items-center gap-2">
                <h4 class="font-medium">{{ issueTitle(issue) }}</h4>
                <span class="badge badge-danger">{{ issueSeverity(issue) }} · {{ issueAction(issue) }}</span>
              </div>
              <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">{{ issueDescription(issue) }}</p>
              <dl class="mt-2 grid gap-1 text-xs text-gray-500 dark:text-dark-400 sm:grid-cols-2">
                <div><dt class="inline">{{ t('admin.promptAudit.events.categories') }} · </dt><dd class="inline">{{ translateCategory(issue.category || issue.scanner_id) }}</dd></div>
                <div><dt class="inline">{{ t('admin.promptAudit.events.score') }} · </dt><dd class="inline tabular-nums">{{ issue.score }}</dd></div>
                <div class="sm:col-span-2"><dt class="inline">{{ t('admin.promptAudit.events.evidence') }} · </dt><dd class="inline break-words">{{ issue.evidence ? translateEvidence(issue.evidence) : '—' }}</dd></div>
              </dl>
            </article>
            <p v-if="event.issue_summaries.length === 0" class="ui-empty py-6">{{ t('admin.promptAudit.events.noRisks') }}</p>
          </section>
        </div>

        <dl v-show="activeTab === 'technical'" class="ui-dl">
          <dt>{{ t('admin.promptAudit.events.requestId') }}</dt><dd class="break-all font-mono text-xs"><CopyValue :value="event.snapshot.request_id" :label="t('admin.promptAudit.events.requestId')" /></dd>
          <dt>{{ t('admin.promptAudit.events.promptHash') }}</dt><dd class="break-all font-mono text-xs"><CopyValue :value="event.snapshot.prompt_hash" :label="t('admin.promptAudit.events.promptHash')" /></dd>
          <dt>{{ t('admin.promptAudit.events.technical.scanner') }}</dt><dd>{{ event.scanner_backend }} · {{ event.scanner_version }}</dd>
          <dt>{{ t('admin.promptAudit.events.technical.policy') }}</dt><dd>{{ event.policy_id }} · v{{ event.policy_version }}</dd>
          <dt>{{ t('admin.promptAudit.events.technical.guardEndpoint') }}</dt><dd>{{ event.guard_endpoint_id }}</dd>
          <dt>{{ t('admin.promptAudit.events.technical.config') }}</dt><dd>v{{ event.config_version }}</dd>
          <dt>{{ t('admin.promptAudit.events.technical.chunks') }}</dt><dd class="tabular-nums">{{ event.chunk_total }}</dd>
          <dt>{{ t('admin.promptAudit.events.technical.latency') }}</dt><dd class="tabular-nums">{{ event.latency_ms }} ms</dd>
          <dt>{{ t('admin.promptAudit.events.stage') }}</dt><dd>{{ event.snapshot.stage || 'http' }}</dd>
          <dt>{{ t('admin.promptAudit.events.technical.protocol') }}</dt><dd>{{ event.snapshot.protocol }} · {{ event.snapshot.endpoint }}</dd>
        </dl>
      </div>
    </div>
  </SideDrawer>
</template>

<script setup lang="ts">
import { defineComponent, h, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import type { PromptAuditEvent, PromptIssueSummary } from '../types'
import { SCANNER_CATALOG } from '../viewModel'

const props = defineProps<{ show: boolean; event: PromptAuditEvent | null; loading: boolean }>()
defineEmits<{ (event: 'close'): void }>()
const { t, locale } = useI18n()
const tabs = ['summary', 'risks', 'technical'] as const
const activeTab = ref<(typeof tabs)[number]>('summary')
watch(() => props.event?.id, () => { activeTab.value = 'summary' })

const DECISIONS = new Set(['pass', 'flag', 'critical'])
const ACTIONS = new Set(['Allow', 'Warn', 'Block'])
const RISK_LEVELS = new Set(['low', 'medium', 'high', 'critical'])

// Read-only value with a copy shortcut; the event list no longer carries copy buttons.
const CopyValue = defineComponent({
  props: { value: { type: String, default: '' }, label: { type: String, required: true } },
  setup(componentProps) {
    return () => h('span', { class: 'inline-flex max-w-full items-center gap-1' }, [
      h('span', { class: 'min-w-0 break-all' }, componentProps.value || '—'),
      componentProps.value ? h('button', {
        type: 'button', class: 'app-icon-button h-6 w-6 flex-none', title: t('common.copy'), 'aria-label': `${t('common.copy')} ${componentProps.label}`,
        onClick: () => navigator.clipboard?.writeText(componentProps.value),
      }, [h(Icon, { name: 'copy', size: 'xs' })]) : null,
    ])
  },
})

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value))
}
function decisionClass(decision: string): string {
  if (decision === 'critical') return 'badge-danger'
  if (decision === 'flag') return 'badge-warning'
  return 'badge-success'
}
function displayPrompt(event: PromptAuditEvent): string {
  return event.snapshot.full_prompt || event.snapshot.redacted_preview || '—'
}

function formatDecisionAction(decision: string, action: string): string {
  const decisionLabel = DECISIONS.has(decision) ? t(`admin.promptAudit.decisions.${decision}`) : decision
  const actionLabel = ACTIONS.has(action) ? t(`admin.promptAudit.actions.${action}`) : action
  return `${decisionLabel} · ${actionLabel}`
}
function translateCategory(category: string): string {
  return SCANNER_CATALOG.some((scanner) => scanner.id === category)
    ? t(`admin.promptAudit.scanners.${category}`)
    : category
}
function formatCategories(categories: string[]): string {
  if (!categories.length) return '—'
  return categories.map(translateCategory).join(', ')
}
function translateEvidence(value: string): string {
  const byId = SCANNER_CATALOG.find((scanner) => scanner.id === value)
  if (byId) return t(`admin.promptAudit.scanners.${byId.id}`)
  const byLabel = SCANNER_CATALOG.find((scanner) => scanner.label === value)
  if (byLabel) return t(`admin.promptAudit.scanners.${byLabel.id}`)
  return value
}
function formatGuardReturn(event: PromptAuditEvent): string {
  const evidence: Record<string, string> = {}
  for (const [key, value] of Object.entries(event.scanner_evidence || {})) {
    evidence[key] = translateEvidence(value)
  }
  return JSON.stringify({
    decision: DECISIONS.has(event.decision) ? t(`admin.promptAudit.decisions.${event.decision}`) : event.decision,
    risk_level: RISK_LEVELS.has(event.risk_level) ? t(`admin.promptAudit.riskLevels.${event.risk_level}`) : event.risk_level,
    action: ACTIONS.has(event.action) ? t(`admin.promptAudit.actions.${event.action}`) : event.action,
    categories: event.categories.map(translateCategory),
    matched_scanners: event.matched_scanners.map(translateCategory),
    scanner_scores: event.scanner_scores,
    scanner_evidence: evidence,
    scanner_backend: event.scanner_backend,
    scanner_version: event.scanner_version,
    guard_endpoint_id: event.guard_endpoint_id,
    chunk_total: event.chunk_total,
    latency_ms: event.latency_ms,
  }, null, 2)
}
function issueTitle(issue: PromptIssueSummary): string {
  return translateCategory(issue.category || issue.scanner_id) || issue.title
}
function issueDescription(issue: PromptIssueSummary): string {
  const category = issue.category || issue.scanner_id
  const key = `admin.promptAudit.scannerDescriptions.${category}`
  const label = t(key)
  return label === key ? issue.description : label
}
function issueSeverity(issue: PromptIssueSummary): string {
  return RISK_LEVELS.has(issue.severity) ? t(`admin.promptAudit.riskLevels.${issue.severity}`) : issue.severity_label || issue.severity
}
function issueAction(issue: PromptIssueSummary): string {
  return ACTIONS.has(issue.action) ? t(`admin.promptAudit.actions.${issue.action}`) : issue.action_label || issue.action
}
</script>

<style scoped>
.pa-pre {
  overflow: auto;
  padding: 0.875rem 1rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
}

.pa-issue {
  padding-left: 0.875rem;
  border-left: 2px solid var(--operator-destructive);
}
</style>
