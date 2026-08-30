import { apiClient, buildApiUrl } from './client'
import { ADMIN_UI_REQUEST_HEADER } from './adminUIRequest'

export interface OperatorAssistantMessage {
  role: 'user' | 'assistant'
  content: string
}

export interface OperatorAssistantModel {
  id: string
  model: string
  group_id: number
  group_name: string
  platform: string
  available: boolean
}

export interface OperatorAssistantModels {
  default: string
  models: OperatorAssistantModel[]
}

export interface OperatorAssistantRequest {
  model: string
  messages: OperatorAssistantMessage[]
  route?: string
}

export interface OperatorAssistantMetadata {
  model: string
  selection: string
  provider: string
}

interface StreamCallbacks {
  onDelta: (delta: string) => void
  onMetadata?: (metadata: OperatorAssistantMetadata) => void
}

export class OperatorAssistantError extends Error {
  constructor(message: string, readonly status = 0) {
    super(message)
    this.name = 'OperatorAssistantError'
  }
}

export async function getOperatorAssistantModels(): Promise<OperatorAssistantModels> {
  const response = await apiClient.get<OperatorAssistantModels>('/admin/operator-assistant/models')
  return response.data
}

function requestError(status: number): OperatorAssistantError {
  const messages: Record<number, string> = {
    401: 'Your administrator session expired. Sign in again.',
    403: 'Administrator access is required.',
    409: 'The selected model is no longer available.',
    429: 'Ask Gateway is busy. Try again shortly.',
    503: 'No eligible model capacity is currently available.',
    504: 'Ask Gateway timed out. Try again.',
  }
  return new OperatorAssistantError(messages[status] || 'Ask Gateway could not complete the request.', status)
}

function streamError(type: string): OperatorAssistantError {
  if (type === 'response.incomplete') {
    return new OperatorAssistantError('The model stopped before completing its answer.')
  }
  if (type === 'response.cancelled' || type === 'response.canceled') {
    return new OperatorAssistantError('The model cancelled the response.')
  }
  return new OperatorAssistantError('The model could not complete this response.')
}

async function consumeSSE(
  body: ReadableStream<Uint8Array>,
  callbacks: StreamCallbacks,
): Promise<void> {
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let terminal = false

  const handleBlock = (block: string) => {
    const data = block
      .split(/\r?\n/)
      .filter((line) => line.startsWith('data:'))
      .map((line) => line.slice(5).trimStart())
      .join('\n')
    if (!data) return
    if (data === '[DONE]') {
      terminal = true
      return
    }

    let event: Record<string, unknown>
    try {
      event = JSON.parse(data) as Record<string, unknown>
    } catch {
      throw new OperatorAssistantError('Ask Gateway returned an invalid stream.')
    }

    const type = typeof event.type === 'string' ? event.type : ''
    if (type === 'response.output_text.delta' && typeof event.delta === 'string') {
      callbacks.onDelta(event.delta)
      return
    }
    if (type === 'response.completed' || type === 'response.done') {
      terminal = true
      return
    }
    if (
      type === 'error' ||
      type === 'response.failed' ||
      type === 'response.incomplete' ||
      type === 'response.cancelled' ||
      type === 'response.canceled'
    ) {
      throw streamError(type)
    }
  }

  while (true) {
    const { done, value } = await reader.read()
    buffer += decoder.decode(value, { stream: !done })
    const blocks = buffer.split(/\r?\n\r?\n/)
    buffer = blocks.pop() || ''
    for (const block of blocks) handleBlock(block)
    if (done) break
  }
  if (buffer.trim()) handleBlock(buffer)
  if (!terminal) throw new OperatorAssistantError('The response stream ended before completion.')
}

export async function streamOperatorAssistant(
  request: OperatorAssistantRequest,
  signal: AbortSignal,
  callbacks: StreamCallbacks,
): Promise<OperatorAssistantMetadata> {
  const token = localStorage.getItem('auth_token')
  const response = await fetch(buildApiUrl('/admin/operator-assistant'), {
    method: 'POST',
    headers: {
      Accept: 'text/event-stream',
      'Content-Type': 'application/json',
      [ADMIN_UI_REQUEST_HEADER]: '1',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(request),
    credentials: 'include',
    signal,
  })

  if (!response.ok) throw requestError(response.status)
  if (!response.body) throw new OperatorAssistantError('Ask Gateway returned no response stream.')

  const metadata = {
    model: response.headers.get('X-Gateway-Model') || '',
    selection: response.headers.get('X-Gateway-Model-Selection') || '',
    provider: response.headers.get('X-Gateway-Provider') || '',
  }
  callbacks.onMetadata?.(metadata)
  await consumeSSE(response.body, callbacks)
  return metadata
}
