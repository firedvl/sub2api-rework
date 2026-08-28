import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))

vi.mock('../client', () => ({ apiClient: { get, post } }))

import { checkUpdates, installUpdate, prepareUpdate, rollbackUpdate } from '@/api/admin/system'

describe('admin system update API', () => {
  beforeEach(() => { get.mockReset(); post.mockReset() })

  it('checks updates with an explicit force parameter', async () => {
    get.mockResolvedValue({ data: { state: 'up_to_date' } })
    await checkUpdates(true)
    expect(get).toHaveBeenCalledWith('/admin/system/check-updates', { params: { force: 'true' } })
  })

  it('sends only the approved version to prepare', async () => {
    post.mockResolvedValue({ data: { operation_id: 'op-1' } })
    await prepareUpdate('1.2.3-rework.4')
    expect(post).toHaveBeenCalledWith('/admin/system/prepare', { version: '1.2.3-rework.4' })
  })

  it('sends exact confirmation payloads for install and rollback', async () => {
    post.mockResolvedValue({ data: { operation_id: 'op-1' } })
    await installUpdate('1.2.3-rework.4', 'INSTALL 1.2.3-rework.4')
    await rollbackUpdate('1.2.3-rework.3', 'ROLLBACK 1.2.3-rework.3')
    expect(post).toHaveBeenNthCalledWith(1, '/admin/system/install', { version: '1.2.3-rework.4', confirmation: 'INSTALL 1.2.3-rework.4' })
    expect(post).toHaveBeenNthCalledWith(2, '/admin/system/rollback', { version: '1.2.3-rework.3', confirmation: 'ROLLBACK 1.2.3-rework.3' })
  })
})
