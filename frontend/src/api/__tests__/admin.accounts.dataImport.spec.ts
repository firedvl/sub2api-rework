import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))

vi.mock('@/api/client', () => ({
  apiClient: { post }
}))

import { importData, previewImportData } from '@/api/admin/accounts'
import type { AdminDataPayload } from '@/types'

const payload: AdminDataPayload = {
  type: 'sub2api-data',
  version: 1,
  exported_at: '2026-08-29T00:00:00Z',
  proxies: [],
  accounts: [{
    name: 'primary',
    platform: 'openai',
    type: 'oauth',
    credentials: { access_token: 'secret' },
    concurrency: 1,
    priority: 0
  }]
}

describe('admin account data import API', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    post.mockReset()
  })

  it('uses the preview endpoint without an idempotency header', async () => {
    post.mockResolvedValue({ data: { total: 1, ready: 1, duplicate: 0, invalid: 0, items: [] } })

    await previewImportData({ data: payload })

    expect(post).toHaveBeenCalledWith('/admin/accounts/data/preview', { data: payload }, {
      timeout: 120000
    })
  })

  it('forwards the stable import operation key', async () => {
    post.mockResolvedValue({ data: { account_created: 1, account_failed: 0 } })

    await importData({ data: payload, skip_default_group_bind: true }, 'bulk-account-import-run-0')

    expect(post).toHaveBeenCalledWith('/admin/accounts/data', {
      data: payload,
      skip_default_group_bind: true
    }, {
      timeout: 120000,
      headers: { 'Idempotency-Key': 'bulk-account-import-run-0' }
    })
    expect(localStorage.length).toBe(0)
    expect(sessionStorage.length).toBe(0)
  })
})
