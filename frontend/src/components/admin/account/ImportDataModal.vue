<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.bulkImportTitle')"
    width="extra-wide"
    close-on-click-outside
    @close="handleClose"
  >
    <div class="space-y-4">
      <ol class="grid grid-cols-4 gap-2 text-xs" :aria-label="t('admin.accounts.bulkImportSteps')">
        <li
          v-for="(label, index) in steps"
          :key="label"
          :aria-current="index + 1 === step ? 'step' : undefined"
          :class="
            index + 1 === step
              ? 'font-semibold text-primary-600 dark:text-primary-400'
              : 'text-gray-500 dark:text-dark-400'
          "
        >
          {{ index + 1 }}. {{ label }}
        </li>
      </ol>

      <section v-if="step === 1" class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-dark-300">
          {{ t('admin.accounts.bulkImportAddHint') }}
        </p>
        <textarea
          v-model="pasteText"
          class="input min-h-44 w-full font-mono text-sm"
          :placeholder="t('admin.accounts.bulkImportPastePlaceholder')"
          :aria-label="t('admin.accounts.bulkImportPaste')"
        />
        <div class="flex flex-wrap gap-3">
          <button type="button" class="btn btn-secondary" @click="fileInput?.click()">
            <Icon name="upload" size="sm" class="mr-1" />
            {{ t('common.chooseFile') }}
          </button>
          <span class="text-sm text-gray-500 dark:text-dark-400">
            {{
              files.length
                ? files.map((file) => file.name).join(', ')
                : t('admin.accounts.bulkImportDropHint')
            }}
          </span>
        </div>
        <div
          class="rounded-lg border border-dashed border-gray-300 p-5 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-dark-400"
          @dragover.prevent
          @drop.prevent="handleDrop"
        >
          {{ t('admin.accounts.bulkImportDropHint') }}
        </div>
        <input
          ref="fileInput"
          class="hidden"
          type="file"
          accept="application/json,.json,.jsonl,application/x-ndjson"
          multiple
          @change="handleFileChange"
        />
      </section>

      <section v-else-if="step === 2" class="space-y-4">
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-4" aria-live="polite">
          <div
            v-for="item in counts"
            :key="item.label"
            class="rounded border border-gray-200 p-3 text-sm dark:border-dark-700"
          >
            <div class="text-gray-500 dark:text-dark-400">{{ item.label }}</div>
            <strong>{{ item.value }}</strong>
          </div>
        </div>
        <div class="max-h-72 overflow-auto rounded border border-gray-200 dark:border-dark-700">
          <table class="min-w-full text-left text-sm">
            <thead class="sticky top-0 bg-gray-50 text-xs dark:bg-dark-800">
              <tr>
                <th class="p-2">{{ t('admin.accounts.bulkImportStatus') }}</th>
                <th class="p-2">{{ t('admin.accounts.columns.platform') }}</th>
                <th class="p-2">{{ t('admin.accounts.columns.name') }}</th>
                <th class="p-2">{{ t('admin.accounts.bulkImportIdentity') }}</th>
                <th class="p-2">{{ t('admin.accounts.bulkImportMessage') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in previewRows"
                :key="row.index"
                class="border-t border-gray-100 dark:border-dark-700"
              >
                <td class="p-2 capitalize">{{ row.status }}</td>
                <td class="p-2">
                  <select
                    v-if="editableAccount(row.index)"
                    v-model="editableAccount(row.index)!.platform"
                    class="input py-1"
                    @change="previewDirty = true"
                  >
                    <option
                      v-if="row.platform && !isKnownPlatform(row.platform)"
                      :value="row.platform"
                      disabled
                    >
                      {{ row.platform }}
                    </option>
                    <option v-for="platform in platforms" :key="platform" :value="platform">
                      {{ platform }}
                    </option>
                  </select>
                  <span v-else>{{ row.platform || '-' }}</span>
                </td>
                <td class="p-2">
                  <input
                    v-if="editableAccount(row.index)"
                    v-model="editableAccount(row.index)!.name"
                    class="input w-full py-1"
                    @change="previewDirty = true"
                  />
                  <span v-else>{{ row.name || '-' }}</span>
                </td>
                <td class="p-2">{{ row.identity || row.credential_hint || '-' }}</td>
                <td class="p-2">{{ row.message || '-' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="grid gap-3 md:grid-cols-2">
          <label class="input-label">
            {{ t('admin.accounts.columns.status') }}
            <select
              v-model="optionState.status"
              class="input mt-1 w-full"
              @change="previewDirty = true"
            >
              <option value="">{{ t('admin.accounts.bulkImportKeepSource') }}</option>
              <option value="active">{{ t('admin.accounts.status.active') }}</option>
              <option value="disabled">{{ t('admin.accounts.bulkImportDisabled') }}</option>
            </select>
          </label>
          <label class="input-label">
            {{ t('admin.accounts.columns.proxy') }}
            <select
              v-model="optionState.proxy_id"
              class="input mt-1 w-full"
              @change="previewDirty = true"
            >
              <option value="">{{ t('admin.accounts.bulkImportKeepSource') }}</option>
              <option v-for="proxy in proxies" :key="proxy.id" :value="String(proxy.id)">
                {{ proxy.name }}
              </option>
            </select>
          </label>
          <label class="input-label">
            {{ t('admin.accounts.columns.priority') }}
            <input
              v-model="optionState.priority"
              class="input mt-1 w-full"
              type="number"
              @change="previewDirty = true"
            />
          </label>
          <label class="flex items-center gap-2 pt-6 text-sm">
            <input
              v-model="optionState.applySchedulable"
              type="checkbox"
              @change="previewDirty = true"
            />
            {{ t('admin.accounts.bulkImportOverrideSchedulable') }}
            <input
              v-if="optionState.applySchedulable"
              v-model="optionState.schedulable"
              type="checkbox"
              @change="previewDirty = true"
            />
            {{ optionState.applySchedulable ? t('admin.accounts.schedulable') : '' }}
          </label>
        </div>
        <GroupSelector
          v-model="optionState.group_ids"
          :groups="groups"
          @update:model-value="previewDirty = true"
        />
      </section>

      <section v-else-if="step === 3" class="space-y-3" aria-live="polite">
        <p class="text-sm text-gray-600 dark:text-dark-300">
          {{ t('admin.accounts.bulkImportProgress', { completed, total: importQueue.length }) }}
        </p>
        <progress class="w-full" :value="completed" :max="Math.max(importQueue.length, 1)">
          {{ completed }}/{{ importQueue.length }}
        </progress>
      </section>

      <section v-else class="space-y-4">
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
          <div
            v-for="item in resultCounts"
            :key="item.label"
            class="rounded border border-gray-200 p-3 text-sm dark:border-dark-700"
          >
            <div class="text-gray-500 dark:text-dark-400">{{ item.label }}</div>
            <strong>{{ item.value }}</strong>
          </div>
        </div>
        <div
          class="flex flex-wrap gap-2"
          role="group"
          :aria-label="t('admin.accounts.bulkImportResults')"
        >
          <button
            v-for="filter in resultFilters"
            :key="filter.value"
            type="button"
            class="btn btn-secondary"
            :aria-pressed="resultFilter === filter.value"
            @click="resultFilter = filter.value"
          >
            {{ filter.label }}
          </button>
          <button class="btn btn-secondary" type="button" @click="copySummary">
            {{ t('common.copy') }}
          </button>
        </div>
        <div class="max-h-72 overflow-auto rounded border border-gray-200 dark:border-dark-700">
          <table class="min-w-full text-left text-sm">
            <thead class="sticky top-0 bg-gray-50 text-xs dark:bg-dark-800">
              <tr>
                <th class="p-2">{{ t('admin.accounts.bulkImportStatus') }}</th>
                <th class="p-2">{{ t('admin.accounts.columns.name') }}</th>
                <th class="p-2">{{ t('admin.accounts.bulkImportMessage') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="item in filteredResults"
                :key="item.index"
                class="border-t border-gray-100 dark:border-dark-700"
              >
                <td class="p-2 capitalize">{{ item.action }}</td>
                <td class="p-2">{{ item.name || '-' }}</td>
                <td class="p-2">{{ item.message || '-' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
    <template #footer>
      <div class="flex flex-wrap justify-end gap-3">
        <button
          v-if="step === 2"
          class="btn btn-secondary"
          type="button"
          :disabled="previewing"
          @click="step = 1"
        >
          {{ t('common.back') }}
        </button>
        <button
          v-if="step === 4 && failedAccounts.length"
          class="btn btn-secondary"
          type="button"
          @click="retryFailed"
        >
          {{ t('admin.accounts.bulkImportRetryFailed') }}
        </button>
        <button v-if="step === 4" class="btn btn-primary" type="button" @click="handleClose">
          {{ t('common.close') }}
        </button>
        <button
          v-else-if="step === 1"
          class="btn btn-primary"
          type="button"
          :disabled="previewing"
          @click="preview"
        >
          {{
            previewing
              ? t('admin.accounts.bulkImportPreviewing')
              : t('admin.accounts.bulkImportReview')
          }}
        </button>
        <button
          v-else-if="step === 2"
          class="btn btn-primary"
          type="button"
          :disabled="previewing || !importableAccounts.length"
          @click="previewDirty ? preview() : startImport()"
        >
          {{
            previewDirty
              ? t('admin.accounts.bulkImportRevalidate')
              : t('admin.accounts.dataImportButton')
          }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type {
  AccountPlatform,
  AdminDataAccount,
  AdminDataImportItem,
  AdminDataImportOptions,
  AdminDataPayload,
  AdminDataPreviewItem,
  AdminDataProxy,
  AdminGroup,
  Proxy,
} from '@/types'

interface Props {
  show: boolean
  proxies: Proxy[]
  groups: AdminGroup[]
}
interface ParsedInput {
  accounts: AdminDataAccount[]
  proxies: AdminDataProxy[]
  issues: number[]
}
interface RetryBatch {
  rows: AdminDataAccount[]
  idempotencyKey?: string
}
const props = defineProps<Props>()
const emit = defineEmits<{ close: []; imported: [] }>()
const { t } = useI18n()
const appStore = useAppStore()
const step = ref(1)
const pasteText = ref('')
const files = ref<File[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
const accounts = ref<AdminDataAccount[]>([])
const dataProxies = ref<AdminDataProxy[]>([])
const parseIssues = ref<number[]>([])
const previewItems = ref<AdminDataPreviewItem[]>([])
const previewing = ref(false)
const previewDirty = ref(false)
const completed = ref(0)
const importQueue = ref<AdminDataAccount[]>([])
const results = ref<AdminDataImportItem[]>([])
const failedAccounts = ref<AdminDataAccount[]>([])
const retryBatches = ref<RetryBatch[]>([])
const imported = ref(false)
const resultFilter = ref<'all' | 'successful' | 'skipped' | 'failed'>('all')
const optionState = ref({
  status: '',
  proxy_id: '',
  priority: '',
  applySchedulable: false,
  schedulable: true,
  group_ids: [] as number[],
})
const platforms: AccountPlatform[] = [
  'anthropic',
  'openai',
  'gemini',
  'antigravity',
  'grok',
  'kimi',
  'zhipu',
  'deepseek',
]
const isKnownPlatform = (platform: string) => platforms.includes(platform as AccountPlatform)
const steps = computed(() => [
  t('admin.accounts.bulkImportAdd'),
  t('admin.accounts.bulkImportReview'),
  t('admin.accounts.bulkImportImport'),
  t('admin.accounts.bulkImportResults'),
])
const importOptions = computed<AdminDataImportOptions>(() => {
  const state = optionState.value
  return {
    ...(state.status ? { status: state.status as 'active' | 'disabled' } : {}),
    ...(state.proxy_id ? { proxy_id: Number(state.proxy_id) } : {}),
    ...(state.priority !== '' ? { priority: Number(state.priority) } : {}),
    ...(state.applySchedulable ? { schedulable: state.schedulable } : {}),
    ...(state.group_ids.length ? { group_ids: state.group_ids } : {}),
  }
})
const payload = (rows: AdminDataAccount[]): AdminDataPayload => ({
  exported_at: new Date().toISOString(),
  proxies: dataProxies.value,
  accounts: rows,
})
const counts = computed(() => [
  {
    label: t('admin.accounts.bulkImportTotal'),
    value: previewItems.value.length + parseIssues.value.length,
  },
  {
    label: t('admin.accounts.bulkImportReady'),
    value: previewItems.value.filter((row) => row.status === 'ready').length,
  },
  {
    label: t('admin.accounts.bulkImportDuplicates'),
    value: previewItems.value.filter((row) => row.status === 'duplicate').length,
  },
  {
    label: t('admin.accounts.bulkImportInvalid'),
    value:
      previewItems.value.filter((row) =>
        ['invalid', 'unsupported', 'conflict'].includes(row.status),
      ).length + parseIssues.value.length,
  },
])
const previewRows = computed<AdminDataPreviewItem[]>(() => [
  ...previewItems.value,
  ...parseIssues.value.map((index) => ({
    index,
    status: 'invalid' as const,
    message: t('admin.accounts.bulkImportMalformed'),
  })),
])
const importableAccounts = computed(() =>
  accounts.value.filter((account) =>
    ['ready', 'duplicate'].includes(
      previewItems.value.find((row) => row.index === account.source_index)?.status || '',
    ),
  ),
)
const editableAccount = (index: number) =>
  accounts.value.find((account) => account.source_index === index)
const resultCounts = computed(() => [
  {
    label: t('admin.accounts.bulkImportCreated'),
    value: results.value.filter((item) => item.action === 'created').length,
  },
  {
    label: t('admin.accounts.bulkImportUpdated'),
    value: results.value.filter((item) => item.action === 'updated').length,
  },
  {
    label: t('admin.accounts.bulkImportSkipped'),
    value: results.value.filter((item) => item.action === 'skipped').length,
  },
  {
    label: t('admin.accounts.bulkImportFailed'),
    value: results.value.filter((item) => item.action === 'failed').length,
  },
])
const resultFilters = computed(() => [
  { value: 'all' as const, label: t('common.all') },
  { value: 'successful' as const, label: t('admin.accounts.bulkImportSuccessful') },
  { value: 'skipped' as const, label: t('admin.accounts.bulkImportSkipped') },
  { value: 'failed' as const, label: t('admin.accounts.bulkImportFailed') },
])
const filteredResults = computed(() =>
  results.value.filter(
    (item) =>
      resultFilter.value === 'all' ||
      (resultFilter.value === 'successful'
        ? ['created', 'updated'].includes(item.action)
        : item.action === resultFilter.value),
  ),
)
const isPayload = (value: unknown): value is AdminDataPayload =>
  !!value && typeof value === 'object' && !Array.isArray(value) && 'accounts' in value
const parseText = (text: string, offset = 0): ParsedInput => {
  const result: ParsedInput = { accounts: [], proxies: [], issues: [] }
  const add = (value: unknown, index: number) => {
    if (value && typeof value === 'object' && !Array.isArray(value) && 'name' in value)
      result.accounts.push({ ...(value as AdminDataAccount), source_index: offset + index })
    else result.issues.push(offset + index)
  }
  try {
    const value = JSON.parse(text)
    if (isPayload(value)) {
      if (
        (value.type && !['sub2api-data', 'sub2api-bundle'].includes(value.type)) ||
        (value.version && value.version !== 1)
      ) {
        result.issues.push(offset + 1)
        return result
      }
      result.proxies.push(...(value.proxies || []))
      value.accounts.forEach((row, index) => add(row, index + 1))
    } else (Array.isArray(value) ? value : [value]).forEach((row, index) => add(row, index + 1))
  } catch {
    text
      .split(/\r?\n/)
      .filter(Boolean)
      .forEach((line, index) => {
        try {
          add(JSON.parse(line), index + 1)
        } catch {
          result.issues.push(offset + index + 1)
        }
      })
  }
  return result
}
const readFile = async (file: File) =>
  typeof file.text === 'function' ? file.text() : new TextDecoder().decode(await file.arrayBuffer())
const collectInput = async () => {
  const collected = parseText(pasteText.value)
  let offset = collected.accounts.length + collected.issues.length
  for (const file of files.value) {
    const next = parseText(await readFile(file), offset)
    collected.accounts.push(...next.accounts)
    collected.proxies.push(...next.proxies)
    collected.issues.push(...next.issues)
    offset += next.accounts.length + next.issues.length
  }
  if (!props.show) return false
  accounts.value = collected.accounts
  dataProxies.value = collected.proxies
  parseIssues.value = collected.issues
  return true
}
const preview = async () => {
  if (step.value === 1 && !(await collectInput())) return
  if (!accounts.value.length) {
    appStore.showError(t('admin.accounts.bulkImportNoAccounts'))
    return
  }
  previewing.value = true
  try {
    const response = await adminAPI.accounts.previewImportData({
      data: payload(accounts.value),
      options: importOptions.value,
    })
    previewItems.value = response.items
    previewDirty.value = false
    step.value = 2
  } catch {
    appStore.showError(t('admin.accounts.bulkImportPreviewFailed'))
  } finally {
    previewing.value = false
  }
}
const operationKey = () =>
  globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}`
const chunkRows = (rows: AdminDataAccount[]) => {
  const batches: RetryBatch[] = []
  for (let start = 0; start < rows.length; start += 10)
    batches.push({ rows: rows.slice(start, start + 10) })
  return batches
}
const clearSuccessful = (chunk: AdminDataAccount[], items: AdminDataImportItem[]) => {
  const failed = new Set(items.filter((item) => item.action === 'failed').map((item) => item.index))
  accounts.value = accounts.value.filter(
    (account) =>
      !chunk.some((row) => row.source_index === account.source_index) ||
      failed.has(account.source_index!),
  )
  return chunk.filter((row) => failed.has(row.source_index!))
}
const preflightFailures = () => [
  ...previewItems.value
    .filter((row) => ['invalid', 'unsupported', 'conflict'].includes(row.status))
    .map((row) => ({
      index: row.index,
      action: 'failed' as const,
      name: row.name,
      message: row.message || t('admin.accounts.bulkImportMalformed'),
    })),
  ...parseIssues.value.map((index) => ({
    index,
    action: 'failed' as const,
    message: t('admin.accounts.bulkImportMalformed'),
  })),
]
const startImport = async (retry?: RetryBatch[]) => {
  const batches = retry || chunkRows(importableAccounts.value)
  const retrying = !!retry
  importQueue.value = batches.flatMap((batch) => batch.rows)
  completed.value = 0
  if (retrying) {
    const retried = new Set(importQueue.value.map((account) => account.source_index))
    results.value = results.value.filter((item) => !retried.has(item.index))
  } else {
    results.value = preflightFailures()
    accounts.value = [...importQueue.value]
  }
  failedAccounts.value = []
  retryBatches.value = []
  step.value = 3
  for (const batch of batches) {
    const chunk = batch.rows
    const key = batch.idempotencyKey || operationKey()
    try {
      const response = await adminAPI.accounts.importData(
        { data: payload(chunk), skip_default_group_bind: true, options: importOptions.value },
        key,
      )
      const items = response.items || []
      results.value.push(...items)
      const failed = clearSuccessful(chunk, items)
      if (failed.length) {
        failedAccounts.value.push(...failed)
        retryBatches.value.push({ rows: failed })
      }
    } catch {
      results.value.push(
        ...chunk.map((account) => ({
          index: account.source_index!,
          action: 'failed' as const,
          name: account.name,
          message: t('admin.accounts.bulkImportTransportFailed'),
        })),
      )
      failedAccounts.value.push(...chunk)
      retryBatches.value.push({ rows: chunk, idempotencyKey: key })
    } finally {
      completed.value += chunk.length
    }
  }
  pasteText.value = ''
  files.value = []
  if (!failedAccounts.value.length) dataProxies.value = []
  if (fileInput.value) fileInput.value.value = ''
  step.value = 4
  imported.value ||= results.value.some(
    (item) => item.action === 'created' || item.action === 'updated',
  )
}
const retryFailed = async () => {
  const batches = retryBatches.value.map((batch) => ({
    rows: [...batch.rows],
    idempotencyKey: batch.idempotencyKey,
  }))
  accounts.value = batches.flatMap((batch) => batch.rows)
  await startImport(batches)
}
const handleFileChange = (event: Event) => {
  files.value = Array.from((event.target as HTMLInputElement).files || []).filter((file) =>
    /\.(json|jsonl)$/i.test(file.name),
  )
}
const handleDrop = (event: DragEvent) => {
  files.value = Array.from(event.dataTransfer?.files || []).filter((file) =>
    /\.(json|jsonl)$/i.test(file.name),
  )
}
const copySummary = async () => {
  const text = filteredResults.value
    .map((item) => `${item.action}: ${item.name || '-'}${item.message ? ` - ${item.message}` : ''}`)
    .join('\n')
  await navigator.clipboard?.writeText(text)
}
const clear = () => {
  step.value = 1
  pasteText.value = ''
  files.value = []
  accounts.value = []
  dataProxies.value = []
  parseIssues.value = []
  previewItems.value = []
  importQueue.value = []
  results.value = []
  failedAccounts.value = []
  retryBatches.value = []
  imported.value = false
  previewDirty.value = false
  resultFilter.value = 'all'
  optionState.value = {
    status: '',
    proxy_id: '',
    priority: '',
    applySchedulable: false,
    schedulable: true,
    group_ids: [],
  }
  if (fileInput.value) fileInput.value.value = ''
}
const handleClose = () => {
  if (step.value === 3) return
  const didImport = imported.value
  clear()
  if (didImport) emit('imported')
  else emit('close')
}
watch(
  () => props.show,
  () => clear(),
)
</script>
