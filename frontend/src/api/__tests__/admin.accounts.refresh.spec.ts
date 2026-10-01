import { beforeEach, describe, expect, it, vi } from 'vitest'
import { refreshCredentials } from '../admin/accounts'

const post = vi.hoisted(() => vi.fn())
vi.mock('../client', () => ({ apiClient: { post } }))
beforeEach(() => { post.mockReset() })

describe('account credential refresh response', () => {
  it('normalizes the legacy account response', async () => {
    const account = { id: 42, name: 'refreshed' }
    post.mockResolvedValue({ data: account })
    expect(await refreshCredentials(42)).toEqual({ account })
    expect(post).toHaveBeenCalledWith('/admin/accounts/42/refresh')
  })
  it('retains partial success with its updated account and warning', async () => {
    const result = { account: { id: 42, name: 'refreshed' }, warning: 'missing_project_id_temporary', message: 'Project ID pending' }
    post.mockResolvedValue({ data: result })
    expect(await refreshCredentials(42)).toEqual(result)
  })
  it.each([null, {}, { warning: 'missing_project_id_temporary' }, { account: { id: 43 } }])('rejects missing or wrong account data', async data => {
    post.mockResolvedValue({ data })
    await expect(refreshCredentials(42)).rejects.toThrow('Invalid account refresh response')
  })
  it('keeps request failures intact', async () => {
    const error = new Error('Upstream refresh failed')
    post.mockRejectedValue(error)
    await expect(refreshCredentials(42)).rejects.toBe(error)
  })
})
