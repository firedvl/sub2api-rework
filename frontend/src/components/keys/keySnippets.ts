import type { GroupPlatform } from '@/types'

export type KeySnippetId = 'curl' | 'openai' | 'claude'

export interface KeySnippet {
  id: KeySnippetId
  label: string
  code: string
}

/** Strips a trailing `/v1` and slashes so every snippet appends its own path once. */
export function gatewayRoot(baseUrl: string): string {
  return baseUrl.trim().replace(/\/+$/, '').replace(/\/v1$/, '').replace(/\/+$/, '')
}

/**
 * Ready-to-paste snippets built only from the key and the site's API base URL.
 * Claude Code points at the Antigravity prefix for Antigravity groups, as the full
 * client guide does.
 */
export function buildKeySnippets(baseUrl: string, apiKey: string, platform: GroupPlatform | null | undefined): KeySnippet[] {
  const root = gatewayRoot(baseUrl)
  const apiBase = `${root}/v1`
  const claudeBase = platform === 'antigravity' ? `${root}/antigravity` : root
  return [
    {
      id: 'curl',
      label: 'curl',
      code: `curl ${apiBase}/models \\\n  -H "Authorization: Bearer ${apiKey}"`,
    },
    {
      id: 'openai',
      label: 'OpenAI SDK',
      code: `from openai import OpenAI\n\nclient = OpenAI(\n    base_url="${apiBase}",\n    api_key="${apiKey}",\n)\nprint(client.models.list())`,
    },
    {
      id: 'claude',
      label: 'Claude Code',
      code: `export ANTHROPIC_BASE_URL="${claudeBase}"\nexport ANTHROPIC_AUTH_TOKEN="${apiKey}"`,
    },
  ]
}

/** Claude-family groups open on the Claude Code snippet; everything else on curl. */
export function defaultSnippetFor(platform: GroupPlatform | null | undefined): KeySnippetId {
  return platform === 'anthropic' || platform === 'antigravity' ? 'claude' : 'curl'
}
