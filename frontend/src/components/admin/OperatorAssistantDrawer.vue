<template>
  <Teleport to="body">
    <Transition name="operator-assistant">
      <div
        v-if="show"
        class="operator-assistant-overlay"
        role="dialog"
        aria-modal="true"
        aria-labelledby="operator-assistant-title"
        @mousedown.self="close"
      >
        <aside ref="panelRef" class="operator-assistant-panel" tabindex="-1">
          <header class="operator-assistant-header">
            <div class="operator-assistant-heading">
              <span class="operator-assistant-mark" aria-hidden="true">
                <Icon name="terminal" size="sm" />
              </span>
              <div>
                <h2 id="operator-assistant-title">{{ t('admin.operatorAssistant.title') }}</h2>
                <span class="operator-assistant-state">
                  <span class="operator-assistant-state-dot" />
                  {{ t('admin.operatorAssistant.ready') }}
                </span>
              </div>
            </div>
            <div class="operator-assistant-header-actions">
              <button
                type="button"
                class="operator-assistant-icon-button"
                :disabled="messages.length === 0"
                :aria-label="t('admin.operatorAssistant.clear')"
                :title="t('admin.operatorAssistant.clear')"
                @click="clearConversation"
              >
                <Icon name="trash" size="sm" />
              </button>
              <button
                type="button"
                class="operator-assistant-icon-button"
                :aria-label="t('common.close')"
                :title="t('common.close')"
                @click="close"
              >
                <Icon name="x" size="sm" />
              </button>
            </div>
          </header>

          <div class="operator-assistant-toolbar">
            <label for="operator-assistant-model">{{ t('admin.operatorAssistant.model') }}</label>
            <Select
              id="operator-assistant-model"
              v-model="selectedModel"
              :options="modelOptions"
              :disabled="modelsLoading"
              :searchable="modelOptions.length > 6"
              :aria-label="t('admin.operatorAssistant.model')"
            />
          </div>

          <div
            ref="conversationRef"
            class="operator-assistant-conversation"
            role="log"
            aria-live="polite"
            aria-relevant="additions text"
            :aria-busy="generating"
          >
            <div v-if="modelsError" class="operator-assistant-notice" role="alert">
              <span>{{ modelsError }}</span>
              <button type="button" @click="loadModels">{{ t('admin.operatorAssistant.retry') }}</button>
            </div>
            <div v-else-if="!modelsLoading && modelOptions.length === 1" class="operator-assistant-notice" role="status">
              {{ t('admin.operatorAssistant.noModels') }}
            </div>

            <div v-if="messages.length === 0" class="operator-assistant-empty">
              <div class="operator-assistant-prompts">
                <button
                  v-for="prompt in starterPrompts"
                  :key="prompt"
                  type="button"
                  :disabled="!modelsReady"
                  @click="sendPrompt(prompt)"
                >
                  <span>{{ prompt }}</span>
                  <Icon name="arrowRight" size="sm" aria-hidden="true" />
                </button>
              </div>
            </div>

            <article
              v-for="(message, index) in messages"
              :key="message.id"
              class="operator-assistant-message"
              :class="`operator-assistant-message-${message.role}`"
            >
              <div class="operator-assistant-message-heading">
                <span>{{ message.role === 'user' ? t('admin.operatorAssistant.you') : 'Gateway' }}</span>
                <button
                  v-if="message.role === 'assistant' && message.content"
                  type="button"
                  class="operator-assistant-copy"
                  :aria-label="t('common.copy')"
                  :title="t('common.copy')"
                  @click="copyToClipboard(message.content)"
                >
                  <Icon name="copy" size="xs" />
                </button>
              </div>
              <div
                v-if="message.role === 'assistant'"
                class="operator-assistant-markdown"
                v-html="renderMarkdown(message.content || (message.status === 'streaming' ? t('admin.operatorAssistant.thinking') : ''))"
              />
              <p v-else class="operator-assistant-user-text">{{ message.content }}</p>

              <div v-if="message.status === 'error'" class="operator-assistant-message-error" role="alert">
                <span>{{ message.error }}</span>
                <button type="button" :disabled="generating" @click="retry(index)">
                  <Icon name="refresh" size="xs" />
                  {{ t('admin.operatorAssistant.retry') }}
                </button>
              </div>
              <div v-else-if="message.status === 'cancelled'" class="operator-assistant-message-status">
                {{ t('admin.operatorAssistant.stopped') }}
              </div>
              <div v-else-if="message.role === 'assistant' && message.metadata" class="operator-assistant-metadata">
                {{ metadataLabel(message) }}
                <span>{{ t('admin.operatorAssistant.contextRefreshed') }}</span>
              </div>
            </article>
          </div>

          <form class="operator-assistant-composer" @submit.prevent="sendPrompt()">
            <textarea
              ref="composerRef"
              v-model="composer"
              :placeholder="t('admin.operatorAssistant.placeholder')"
              :aria-label="t('admin.operatorAssistant.placeholder')"
              maxlength="4000"
              rows="3"
              @keydown.enter.exact.prevent="sendPrompt()"
            />
            <div class="operator-assistant-composer-footer">
              <span>{{ composer.length }} / 4000</span>
              <button
                v-if="generating"
                type="button"
                class="operator-assistant-submit"
                :aria-label="t('admin.operatorAssistant.stop')"
                :title="t('admin.operatorAssistant.stop')"
                @click="stop"
              >
                <Icon name="stop" size="sm" />
              </button>
              <button
                v-else
                type="submit"
                class="operator-assistant-submit"
                :disabled="!canSend"
                :aria-label="t('admin.operatorAssistant.send')"
                :title="t('admin.operatorAssistant.send')"
              >
                <Icon name="send" size="sm" />
              </button>
            </div>
          </form>
        </aside>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import DOMPurify from 'dompurify'
import { Marked } from 'marked'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import {
  getOperatorAssistantModels,
  OperatorAssistantError,
  streamOperatorAssistant,
  type OperatorAssistantMessage as RequestMessage,
  type OperatorAssistantMetadata,
  type OperatorAssistantModel,
} from '@/api/operatorAssistant'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (event: 'close'): void }>()
const { t } = useI18n()
const route = useRoute()
const { copyToClipboard } = useClipboard()

type MessageStatus = 'complete' | 'streaming' | 'error' | 'cancelled'
interface AssistantMessage {
  id: number
  role: 'user' | 'assistant'
  content: string
  status: MessageStatus
  error?: string
  metadata?: OperatorAssistantMetadata
  requestedModel?: string
  elapsedSeconds?: number
}

const MODEL_STORAGE_KEY = 'operator_assistant_model'
const panelRef = ref<HTMLElement | null>(null)
const conversationRef = ref<HTMLElement | null>(null)
const composerRef = ref<HTMLTextAreaElement | null>(null)
const composer = ref('')
const messages = ref<AssistantMessage[]>([])
const models = ref<OperatorAssistantModel[]>([])
const modelsLoading = ref(false)
const modelsError = ref('')
const selectedModel = ref(localStorage.getItem(MODEL_STORAGE_KEY) || 'auto')
const generating = ref(false)
let abortController: AbortController | null = null
let restoreTarget: HTMLElement | null = null
let messageID = 0
let scrollFrame = 0

const starterPrompts = computed(() => [
  t('admin.operatorAssistant.prompts.attention'),
  t('admin.operatorAssistant.prompts.capacity'),
  t('admin.operatorAssistant.prompts.errors'),
  t('admin.operatorAssistant.prompts.models'),
])
const modelOptions = computed<SelectOption[]>(() => [
  { value: 'auto', label: t('admin.operatorAssistant.auto') },
  ...models.value
    .filter((model) => model.available)
    .map((model) => ({
      value: model.id,
      label: `${model.model} / ${model.group_name}`,
    })),
])
const modelsReady = computed(() => !modelsLoading.value && !modelsError.value && modelOptions.value.length > 1)
const canSend = computed(() => modelsReady.value && composer.value.trim().length > 0)

const escapeHTML = (value: string) => value.replace(/[&<>"']/g, (character) => ({
  '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
}[character] || character))

const markdown = new Marked({
  breaks: true,
  gfm: true,
  renderer: {
    html: ({ text }) => escapeHTML(text),
    image: ({ text }) => escapeHTML(text),
  },
})

function renderMarkdown(content: string): string {
  const parsed = markdown.parse(content, { async: false })
  const sanitized = DOMPurify.sanitize(parsed, {
    ALLOWED_TAGS: ['p', 'br', 'strong', 'em', 'del', 'code', 'pre', 'blockquote', 'ul', 'ol', 'li', 'a', 'h1', 'h2', 'h3', 'h4', 'table', 'thead', 'tbody', 'tr', 'th', 'td'],
    ALLOWED_ATTR: ['href', 'title', 'target', 'rel'],
  })
  const template = document.createElement('template')
  template.innerHTML = sanitized
  template.content.querySelectorAll('a').forEach((link) => {
    const href = link.getAttribute('href') || ''
    try {
      if (!href) throw new Error('empty link')
      const url = new URL(href, window.location.href)
      if (url.protocol !== 'http:' && url.protocol !== 'https:') throw new Error('unsafe protocol')
      if (url.origin !== window.location.origin) {
        link.target = '_blank'
        link.rel = 'noopener noreferrer'
      }
    } catch {
      link.replaceWith(document.createTextNode(link.textContent || href))
    }
  })
  return template.innerHTML
}

async function loadModels() {
  modelsLoading.value = true
  modelsError.value = ''
  try {
    const result = await getOperatorAssistantModels()
    models.value = result.models || []
    const validSelections = new Set(['auto', ...models.value.filter((model) => model.available).map((model) => model.id)])
    if (!validSelections.has(selectedModel.value)) selectedModel.value = result.default || 'auto'
  } catch {
    models.value = []
    modelsError.value = t('admin.operatorAssistant.modelsError')
  } finally {
    modelsLoading.value = false
  }
}

function queueScroll() {
  cancelAnimationFrame(scrollFrame)
  scrollFrame = requestAnimationFrame(() => {
    if (conversationRef.value) conversationRef.value.scrollTop = conversationRef.value.scrollHeight
  })
}

function historyForRequest(): RequestMessage[] {
  return messages.value
    .filter((message) => message.content.trim() && (message.role === 'user' || message.status === 'complete'))
    .slice(-12)
    .map(({ role, content }) => ({ role, content }))
}

async function sendPrompt(value = composer.value) {
  const prompt = value.trim()
  if (!prompt || generating.value || !modelsReady.value) return

  composer.value = ''
  const requestedModel = selectedModel.value
  messages.value.push({ id: ++messageID, role: 'user', content: prompt, status: 'complete' })
  const requestMessages = historyForRequest()
  const answer: AssistantMessage = {
    id: ++messageID,
    role: 'assistant',
    content: '',
    status: 'streaming',
    requestedModel,
  }
  messages.value.push(answer)
  generating.value = true
  abortController = new AbortController()
  const startedAt = performance.now()
  queueScroll()

  try {
    await streamOperatorAssistant(
      { model: requestedModel, messages: requestMessages, route: route.path },
      abortController.signal,
      {
        onMetadata: (metadata) => { answer.metadata = metadata },
        onDelta: (delta) => {
          answer.content += delta
          queueScroll()
        },
      },
    )
    if (!answer.content.trim()) throw new OperatorAssistantError(t('admin.operatorAssistant.emptyResponse'))
    answer.status = 'complete'
    answer.elapsedSeconds = (performance.now() - startedAt) / 1000
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      answer.status = 'cancelled'
    } else {
      answer.status = 'error'
      answer.error = error instanceof OperatorAssistantError
        ? error.message
        : t('admin.operatorAssistant.requestError')
    }
  } finally {
    generating.value = false
    abortController = null
    queueScroll()
    await nextTick()
    composerRef.value?.focus()
  }
}

function retry(index: number) {
  const priorUser = [...messages.value.slice(0, index)].reverse().find((message) => message.role === 'user')
  if (priorUser) sendPrompt(priorUser.content)
}

function stop() {
  abortController?.abort()
}

function clearConversation() {
  stop()
  messages.value = []
  composer.value = ''
  nextTick(() => composerRef.value?.focus())
}

function metadataLabel(message: AssistantMessage): string {
  const actualModel = message.metadata?.model || ''
  const requestedModel = message.requestedModel || 'auto'
  const model = requestedModel === 'auto' && actualModel
    ? `Auto -> ${actualModel}`
    : actualModel || requestedModel
  return [model, message.metadata?.provider, message.elapsedSeconds ? `${message.elapsedSeconds.toFixed(1)}s` : '']
    .filter(Boolean)
    .join(' / ')
}

function close() {
  stop()
  emit('close')
}

function focusableElements(): HTMLElement[] {
  return Array.from(panelRef.value?.querySelectorAll<HTMLElement>(
    'button:not([disabled]), input:not([disabled]), textarea:not([disabled]), [href], [tabindex]:not([tabindex="-1"])',
  ) || []).filter((element) => element.tabIndex >= 0)
}

function handleKeydown(event: KeyboardEvent) {
  if (!props.show || event.defaultPrevented) return
  if (event.key === 'Escape') {
    event.preventDefault()
    close()
    return
  }
  if (event.key !== 'Tab' || (document.activeElement as HTMLElement | null)?.closest('.operator-select-menu')) return
  const focusable = focusableElements()
  if (!focusable.length) {
    event.preventDefault()
    panelRef.value?.focus()
    return
  }
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

watch(selectedModel, (value) => localStorage.setItem(MODEL_STORAGE_KEY, value))
watch(
  () => props.show,
  async (show) => {
    if (show) {
      restoreTarget = document.activeElement as HTMLElement
      document.body.classList.add('operator-assistant-open')
      document.addEventListener('keydown', handleKeydown)
      await loadModels()
      await nextTick()
      composerRef.value?.focus()
    } else {
      stop()
      document.body.classList.remove('operator-assistant-open')
      document.removeEventListener('keydown', handleKeydown)
      if (restoreTarget?.isConnected) restoreTarget.focus()
      restoreTarget = null
    }
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  stop()
  cancelAnimationFrame(scrollFrame)
  document.body.classList.remove('operator-assistant-open')
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<style scoped>
:global(body.operator-assistant-open) { overflow: hidden; }

.operator-assistant-overlay {
  position: fixed;
  inset: 0;
  z-index: 100000010;
  display: flex;
  justify-content: flex-end;
  background: oklch(0 0 0 / 0.48);
}

.operator-assistant-panel {
  display: grid;
  width: min(30rem, 100vw);
  height: 100dvh;
  grid-template-rows: auto auto minmax(0, 1fr) auto;
  border-left: 1px solid var(--operator-border);
  background: var(--operator-background);
  box-shadow: -12px 0 32px oklch(0 0 0 / 0.22);
  color: var(--operator-foreground);
  outline: none;
}

.operator-assistant-header,
.operator-assistant-toolbar,
.operator-assistant-composer {
  border-color: var(--operator-border);
  background: var(--operator-card);
}

.operator-assistant-header {
  display: flex;
  min-height: 4.25rem;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.75rem 1rem;
  border-bottom-width: 1px;
}

.operator-assistant-heading,
.operator-assistant-header-actions,
.operator-assistant-message-heading,
.operator-assistant-metadata,
.operator-assistant-composer-footer,
.operator-assistant-message-error {
  display: flex;
  align-items: center;
}

.operator-assistant-heading { gap: 0.625rem; }
.operator-assistant-heading h2 { margin: 0; font-size: 0.9375rem; font-weight: 650; line-height: 1.25rem; }
.operator-assistant-mark {
  display: grid;
  width: 2rem;
  height: 2rem;
  place-items: center;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
}
.operator-assistant-state { display: flex; align-items: center; gap: 0.375rem; color: var(--operator-muted-foreground); font-size: 0.75rem; }
.operator-assistant-state-dot { width: 0.375rem; height: 0.375rem; border-radius: 999px; background: var(--operator-success-fill); }
.operator-assistant-header-actions { gap: 0.25rem; }

.operator-assistant-icon-button,
.operator-assistant-copy,
.operator-assistant-submit {
  display: inline-grid;
  place-items: center;
  border-radius: var(--operator-radius);
  transition: background-color 150ms ease, color 150ms ease, border-color 150ms ease;
}
.operator-assistant-icon-button { width: 2.25rem; height: 2.25rem; color: var(--operator-muted-foreground); }
.operator-assistant-icon-button:hover:not(:disabled), .operator-assistant-copy:hover { background: var(--operator-muted); color: var(--operator-foreground); }
.operator-assistant-icon-button:disabled { cursor: not-allowed; opacity: 0.35; }

.operator-assistant-toolbar {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: 0.75rem;
  padding: 0.625rem 1rem;
  border-bottom-width: 1px;
}
.operator-assistant-toolbar label { color: var(--operator-muted-foreground); font-size: 0.75rem; font-weight: 600; }
.operator-assistant-toolbar :deep(.select-trigger) { min-height: 2.25rem; padding: 0.375rem 0.625rem; border-radius: var(--operator-radius); }

.operator-assistant-conversation { min-height: 0; overflow-y: auto; overscroll-behavior: contain; }
.operator-assistant-empty { display: grid; min-height: 100%; place-items: center; padding: 1.25rem; }
.operator-assistant-prompts { width: 100%; border-top: 1px solid var(--operator-border); }
.operator-assistant-prompts button {
  display: flex;
  width: 100%;
  min-height: 3rem;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.625rem 0.25rem;
  border-bottom: 1px solid var(--operator-border-subtle);
  color: var(--operator-foreground);
  text-align: left;
}
.operator-assistant-prompts button:hover:not(:disabled) { padding-left: 0.5rem; background: var(--operator-muted); }
.operator-assistant-prompts button:disabled { cursor: not-allowed; opacity: 0.45; }

.operator-assistant-notice {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin: 0.75rem 1rem 0;
  padding: 0.625rem 0.75rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}
.operator-assistant-notice button { color: var(--operator-foreground); font-weight: 600; }

.operator-assistant-message { padding: 1rem; border-bottom: 1px solid var(--operator-border-subtle); }
.operator-assistant-message-user { background: color-mix(in oklch, var(--operator-muted) 55%, transparent); }
.operator-assistant-message-heading { min-height: 1.5rem; justify-content: space-between; color: var(--operator-muted-foreground); font-size: 0.6875rem; font-weight: 650; text-transform: uppercase; }
.operator-assistant-copy { width: 1.75rem; height: 1.75rem; color: var(--operator-muted-foreground); }
.operator-assistant-user-text { margin: 0.375rem 0 0; white-space: pre-wrap; overflow-wrap: anywhere; font-size: 0.875rem; line-height: 1.5rem; }

.operator-assistant-markdown { margin-top: 0.375rem; overflow-wrap: anywhere; font-size: 0.875rem; line-height: 1.55; }
.operator-assistant-markdown :deep(p) { margin: 0 0 0.625rem; }
.operator-assistant-markdown :deep(p:last-child) { margin-bottom: 0; }
.operator-assistant-markdown :deep(ul), .operator-assistant-markdown :deep(ol) { margin: 0.5rem 0; padding-left: 1.25rem; }
.operator-assistant-markdown :deep(ul) { list-style: disc; }
.operator-assistant-markdown :deep(ol) { list-style: decimal; }
.operator-assistant-markdown :deep(code) { padding: 0.125rem 0.25rem; border-radius: 0.25rem; background: var(--operator-muted); font-size: 0.8125rem; }
.operator-assistant-markdown :deep(pre) { max-width: 100%; margin: 0.625rem 0; padding: 0.75rem; overflow-x: auto; border: 1px solid var(--operator-border); border-radius: var(--operator-radius); background: var(--operator-card); }
.operator-assistant-markdown :deep(pre code) { padding: 0; background: transparent; }
.operator-assistant-markdown :deep(a) { color: var(--operator-foreground); text-decoration: underline; text-underline-offset: 2px; }
.operator-assistant-markdown :deep(blockquote) { margin: 0.625rem 0; padding-left: 0.75rem; border-left: 2px solid var(--operator-border); color: var(--operator-muted-foreground); }
.operator-assistant-markdown :deep(table) { display: block; max-width: 100%; margin: 0.625rem 0; overflow-x: auto; border-collapse: collapse; }
.operator-assistant-markdown :deep(th), .operator-assistant-markdown :deep(td) { padding: 0.375rem 0.5rem; border: 1px solid var(--operator-border); text-align: left; }

.operator-assistant-message-error { margin-top: 0.625rem; justify-content: space-between; gap: 0.75rem; color: var(--operator-destructive); font-size: 0.75rem; }
.operator-assistant-message-error button { display: inline-flex; flex: 0 0 auto; align-items: center; gap: 0.25rem; font-weight: 650; }
.operator-assistant-message-status { margin-top: 0.5rem; color: var(--operator-muted-foreground); font-size: 0.75rem; }
.operator-assistant-metadata { margin-top: 0.625rem; flex-wrap: wrap; gap: 0.375rem 0.75rem; color: var(--operator-muted-foreground); font-size: 0.6875rem; }

.operator-assistant-composer { padding: 0.75rem 1rem max(0.75rem, env(safe-area-inset-bottom)); border-top-width: 1px; }
.operator-assistant-composer textarea {
  display: block;
  width: 100%;
  height: 5rem;
  resize: none;
  padding: 0.625rem 0.75rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-background);
  color: var(--operator-foreground);
  font-size: 0.875rem;
  line-height: 1.35rem;
  outline: none;
}
.operator-assistant-composer textarea:focus { border-color: var(--operator-focus); box-shadow: 0 0 0 3px color-mix(in oklch, var(--operator-focus) 20%, transparent); }
.operator-assistant-composer textarea::placeholder { color: var(--operator-muted-foreground); }
.operator-assistant-composer-footer { min-height: 2.25rem; justify-content: space-between; padding-top: 0.5rem; color: var(--operator-muted-foreground); font-size: 0.6875rem; }
.operator-assistant-submit { width: 2.25rem; height: 2.25rem; border: 1px solid var(--operator-primary); background: var(--operator-primary); color: var(--operator-primary-foreground); }
.operator-assistant-submit:hover:not(:disabled) { opacity: 0.85; }
.operator-assistant-submit:disabled { cursor: not-allowed; opacity: 0.35; }

.operator-assistant-enter-active, .operator-assistant-leave-active { transition: background-color 200ms ease; }
.operator-assistant-enter-active .operator-assistant-panel, .operator-assistant-leave-active .operator-assistant-panel { transition: transform 220ms ease; }
.operator-assistant-enter-from, .operator-assistant-leave-to { background: transparent; }
.operator-assistant-enter-from .operator-assistant-panel, .operator-assistant-leave-to .operator-assistant-panel { transform: translateX(100%); }

@media (max-width: 639px) {
  .operator-assistant-panel { width: 100vw; border-left: 0; box-shadow: none; }
  .operator-assistant-header { padding-right: 0.75rem; padding-left: 0.75rem; }
  .operator-assistant-toolbar, .operator-assistant-message, .operator-assistant-composer { padding-right: 0.75rem; padding-left: 0.75rem; }
}

@media (prefers-reduced-motion: reduce) {
  .operator-assistant-enter-active,
  .operator-assistant-leave-active,
  .operator-assistant-enter-active .operator-assistant-panel,
  .operator-assistant-leave-active .operator-assistant-panel { transition: none; }
}
</style>
