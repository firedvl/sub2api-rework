import { describe, expect, it } from 'vitest'
import {
  CC_SWITCH_USAGE_SCRIPT,
  GROK_CC_SWITCH_MODEL,
  OPENAI_CC_SWITCH_CODEX_MODEL,
  buildCcSwitchImportDeeplink
} from '@/utils/ccswitchImport'
import type { GroupPlatform } from '@/types'

function paramsFromDeeplink(deeplink: string): URLSearchParams {
  const query = deeplink.split('?')[1] || ''
  return new URLSearchParams(query)
}

describe('ccswitchImport utils', () => {
  it.each(['https://api.example.com/', 'https://api.example.com///'])('trims trailing slashes before the Antigravity path for %s', (baseUrl) => {
    for (const clientType of ['claude', 'gemini'] as const) {
      const params = paramsFromDeeplink(buildCcSwitchImportDeeplink({
        ...baseInput,
        baseUrl,
        platform: 'antigravity',
        clientType
      }))
      expect(params.get('endpoint')).toBe('https://api.example.com/antigravity')
      expect(params.get('app')).toBe(clientType)
    }
  })
  it('defaults OpenAI CC Switch imports to the current Codex model', () => {
    expect(OPENAI_CC_SWITCH_CODEX_MODEL).toBe('gpt-5.5')
  })

  it('defaults Grok Build imports to the current Grok model', () => {
    expect(GROK_CC_SWITCH_MODEL).toBe('grok-4.5')
  })

  const baseInput = {
    baseUrl: 'https://api.example.com',
    providerName: 'Sub2API',
    apiKey: 'sk-test',
    usageScript: 'return true'
  }

  it('adds the Codex model parameter for OpenAI imports', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'openai',
        clientType: 'claude'
      })
    )

    expect(params.get('resource')).toBe('provider')
    expect(params.get('app')).toBe('codex')
    expect(params.get('endpoint')).toBe(baseInput.baseUrl)
    expect(params.get('model')).toBe(OPENAI_CC_SWITCH_CODEX_MODEL)
    expect(atob(params.get('usageScript') || '')).toBe(baseInput.usageScript)
  })

  it.each([
    ['https://api.example.com', 'https://api.example.com'],
    ['https://api.example.com/', 'https://api.example.com'],
    ['https://api.example.com/v1', 'https://api.example.com/v1'],
    ['https://api.example.com/v1/', 'https://api.example.com/v1']
  ])('preserves the configured Codex endpoint for %s', (baseUrl, endpoint) => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        baseUrl,
        platform: 'openai',
        clientType: 'claude'
      })
    )

    expect(params.get('endpoint')).toBe(endpoint)
  })

  it.each([
    'https://api.example.com',
    'https://api.example.com/',
    'https://api.example.com/v1',
    'https://api.example.com/v1/'
  ])('imports Grok Build with one /v1 suffix for base URL %s', (baseUrl) => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        baseUrl,
        platform: 'grok',
        clientType: 'claude'
      })
    )

    expect(params.get('app')).toBe('grokbuild')
    expect(params.get('endpoint')).toBe('https://api.example.com/v1')
    expect(params.get('model')).toBe(GROK_CC_SWITCH_MODEL)
  })

  it.each([
    { platform: 'anthropic' as GroupPlatform, clientType: 'claude' as const, app: 'claude' },
    { platform: 'gemini' as GroupPlatform, clientType: 'gemini' as const, app: 'gemini' }
  ])('does not add a model parameter for $platform imports', ({ platform, clientType, app }) => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform,
        clientType
      })
    )

    expect(params.get('app')).toBe(app)
    expect(params.get('endpoint')).toBe(baseInput.baseUrl)
    expect(params.has('model')).toBe(false)
  })

  it('keeps Antigravity imports on the selected client endpoint without a model parameter', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'antigravity',
        clientType: 'gemini'
      })
    )

    expect(params.get('app')).toBe('gemini')
    expect(params.get('endpoint')).toBe(`${baseInput.baseUrl}/antigravity`)
    expect(params.has('model')).toBe(false)
  })
})

describe('CC Switch usage script', () => {
  function usageUrlFor(baseUrl: string): string {
    const script = CC_SWITCH_USAGE_SCRIPT.split('{{baseUrl}}').join(baseUrl).split('{{apiKey}}').join('sk-test')
    // eslint-disable-next-line no-new-func
    const config = new Function(`return ${script}`)() as { request: { url: string } }
    return config.request.url
  }

  it.each([
    'https://api.example.com',
    'https://api.example.com/',
    'https://api.example.com/v1',
    'https://api.example.com/v1/'
  ])('queries exactly one /v1/usage for %s', (baseUrl) => {
    expect(usageUrlFor(baseUrl)).toBe('https://api.example.com/v1/usage')
  })

  it.each(['anthropic', 'openai', 'grok', 'gemini', 'antigravity'] as GroupPlatform[])(
    'queries the matching usage route for imported %s providers', (platform) => {
      for (const clientType of ['claude', 'gemini'] as const) {
        const params = paramsFromDeeplink(buildCcSwitchImportDeeplink({
          baseUrl: 'https://api.example.com/', platform, clientType,
          providerName: 'Sub2API', apiKey: 'sk-test', usageScript: CC_SWITCH_USAGE_SCRIPT
        }))
        expect(atob(params.get('usageScript') || '')).toBe(CC_SWITCH_USAGE_SCRIPT)
        expect(usageUrlFor(params.get('endpoint') || '')).toBe(
          platform === 'antigravity'
            ? 'https://api.example.com/antigravity/v1/usage'
            : 'https://api.example.com/v1/usage'
        )
      }
    }
  )
})
