import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  OperatorAssistantError,
  streamOperatorAssistant,
} from '../operatorAssistant'

function streamResponse(chunks: string[], init: ResponseInit = {}): Response {
  const encoder = new TextEncoder()
  let index = 0
  return new Response(new ReadableStream({
    pull(controller) {
      if (index === chunks.length) {
        controller.close()
        return
      }
      controller.enqueue(encoder.encode(chunks[index++]))
    },
  }), init)
}

describe('operator assistant stream client', () => {
  beforeEach(() => {
    localStorage.setItem('auth_token', 'admin-session')
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    localStorage.clear()
  })

  it('parses split Responses SSE deltas and returns gateway metadata', async () => {
    const fetchMock = vi.fn().mockResolvedValue(streamResponse([
      'event: response.output_text.delta\ndata: {"type":"response.output_',
      'text.delta","delta":"Capacity "}\n\nevent: response.output_text.delta\n',
      'data: {"type":"response.output_text.delta","delta":"is healthy."}\n\n',
      'event: response.completed\ndata: {"type":"response.completed"}\n\n',
    ], {
      status: 200,
      headers: {
        'Content-Type': 'text/event-stream',
        'X-Gateway-Model': 'gpt-5.4',
        'X-Gateway-Model-Selection': '11:gpt-5.4',
        'X-Gateway-Provider': 'openai',
      },
    }))
    vi.stubGlobal('fetch', fetchMock)
    const deltas: string[] = []

    const metadata = await streamOperatorAssistant(
      { model: 'auto', messages: [{ role: 'user', content: 'What needs attention?' }] },
      new AbortController().signal,
      { onDelta: (delta) => deltas.push(delta) },
    )

    expect(deltas.join('')).toBe('Capacity is healthy.')
    expect(metadata).toEqual({ model: 'gpt-5.4', selection: '11:gpt-5.4', provider: 'openai' })
    expect(fetchMock).toHaveBeenCalledOnce()
    const request = fetchMock.mock.calls[0][1] as RequestInit
    expect(request.headers).toMatchObject({
      Authorization: 'Bearer admin-session',
      Accept: 'text/event-stream',
      'X-Admin-UI-Request': '1',
    })
    expect(JSON.parse(String(request.body))).toMatchObject({ model: 'auto' })
  })

  it('does not expose an upstream error body from a failed stream', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(streamResponse([
      'event: response.failed\ndata: {"type":"response.failed","response":{"error":{"message":"Bearer secret-token"}}}\n\n',
    ], { status: 200 })))

    await expect(streamOperatorAssistant(
      { model: 'auto', messages: [{ role: 'user', content: 'status' }] },
      new AbortController().signal,
      { onDelta: vi.fn() },
    )).rejects.toMatchObject({
      name: 'OperatorAssistantError',
      message: 'The model could not complete this response.',
    })
  })

  it('maps session expiry without parsing the response body', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(
      '{"message":"private upstream detail"}',
      { status: 401, headers: { 'Content-Type': 'application/json' } },
    )))

    const request = streamOperatorAssistant(
      { model: 'auto', messages: [{ role: 'user', content: 'status' }] },
      new AbortController().signal,
      { onDelta: vi.fn() },
    )
    await expect(request).rejects.toBeInstanceOf(OperatorAssistantError)
    await expect(request).rejects.toMatchObject({
      status: 401,
      message: 'Your administrator session expired. Sign in again.',
    })
  })
})
