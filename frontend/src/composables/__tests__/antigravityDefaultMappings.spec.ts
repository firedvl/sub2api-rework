import { beforeEach, describe, expect, it, vi } from 'vitest'

const getMapping = vi.hoisted(() => vi.fn())
vi.mock('@/api/admin/accounts', () => ({ getAntigravityDefaultModelMapping: getMapping }))

describe('Antigravity default mapping cache', () => {
  beforeEach(() => {
    vi.resetModules()
    getMapping.mockReset()
  })

  it('surfaces a failed fetch and retries without caching a false empty success', async () => {
    const failure = new Error('mapping service unavailable')
    getMapping.mockRejectedValueOnce(failure).mockResolvedValueOnce({ 'gemini-custom': 'gemini-provider' })
    const { fetchAntigravityDefaultMappings } = await import('../useModelWhitelist')
    await expect(fetchAntigravityDefaultMappings()).rejects.toBe(failure)
    await expect(fetchAntigravityDefaultMappings()).resolves.toEqual([{ from: 'gemini-custom', to: 'gemini-provider' }])
    await expect(fetchAntigravityDefaultMappings()).resolves.toEqual([{ from: 'gemini-custom', to: 'gemini-provider' }])
    expect(getMapping).toHaveBeenCalledTimes(2)
  })

  it('caches a genuinely successful empty mapping', async () => {
    getMapping.mockResolvedValue({})
    const { fetchAntigravityDefaultMappings } = await import('../useModelWhitelist')
    await expect(fetchAntigravityDefaultMappings()).resolves.toEqual([])
    await expect(fetchAntigravityDefaultMappings()).resolves.toEqual([])
    expect(getMapping).toHaveBeenCalledTimes(1)
  })
})
