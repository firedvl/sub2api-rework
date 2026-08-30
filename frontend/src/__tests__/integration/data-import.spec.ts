import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ImportDataModal from '@/components/admin/account/ImportDataModal.vue'

const { showError, previewImportData, importData } = vi.hoisted(() => ({
  showError: vi.fn(),
  previewImportData: vi.fn(),
  importData: vi.fn(),
}))

vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError }) }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { previewImportData, importData } } }))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, values?: Record<string, unknown>) =>
      values ? `${key}:${JSON.stringify(values)}` : key,
  }),
}))

const account = (index: number) => ({
  name: `account-${index}`,
  platform: 'anthropic',
  type: 'apikey',
  credentials: { api_key: `secret-${index}` },
  concurrency: 1,
  priority: 0,
})
const preview = (count: number) => ({
  total: count,
  ready: count,
  duplicate: 0,
  invalid: 0,
  items: Array.from({ length: count }, (_, index) => ({
    index: index + 1,
    status: 'ready' as const,
    name: `account-${index + 1}`,
    platform: 'anthropic',
    identity: `user-${index + 1}`,
  })),
})
const result = (data: { accounts: { source_index: number; name: string }[] }) => ({
  proxy_created: 0,
  proxy_reused: 0,
  proxy_failed: 0,
  account_created: data.accounts.length,
  account_failed: 0,
  items: data.accounts.map((account) => ({
    index: account.source_index,
    action: 'created' as const,
    name: account.name,
  })),
})

const mountModal = () =>
  mount(ImportDataModal, {
    props: { show: true, proxies: [], groups: [] },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        GroupSelector: { template: '<div />' },
        Icon: true,
      },
    },
  })
const review = async (wrapper: ReturnType<typeof mountModal>) => {
  await wrapper.get('button.btn-primary').trigger('click')
  await flushPromises()
}

describe('ImportDataModal', () => {
  beforeEach(() => {
    showError.mockReset()
    previewImportData.mockReset()
    importData.mockReset()
  })

  it('parses pasted JSONL, preserves malformed rows, and never renders credentials in preview', async () => {
    previewImportData.mockResolvedValue(preview(2))
    const wrapper = mountModal()
    await wrapper
      .get('textarea')
      .setValue(`${JSON.stringify(account(1))}\nnot-json\n${JSON.stringify(account(2))}`)
    await review(wrapper)
    expect(previewImportData).toHaveBeenCalledWith(
      expect.objectContaining({
        data: expect.objectContaining({
          accounts: expect.arrayContaining([
            expect.objectContaining({ source_index: 1 }),
            expect.objectContaining({ source_index: 3 }),
          ]),
        }),
      }),
    )
    expect(wrapper.text()).toContain('admin.accounts.bulkImportMalformed')
    expect(wrapper.text()).not.toContain('secret-1')
  })

  it('accepts an exported payload from both file picker and drop, retaining linked proxies', async () => {
    previewImportData.mockResolvedValue(preview(1))
    const body = JSON.stringify({
      type: 'sub2api-data',
      version: 1,
      exported_at: '2026-01-01T00:00:00Z',
      proxies: [
        {
          proxy_key: 'proxy-key',
          name: 'linked-proxy',
          protocol: 'http',
          host: 'localhost',
          port: 8080,
          status: 'active',
        },
      ],
      accounts: [account(1)],
    })
    const file = new File([body], 'accounts.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', { value: () => Promise.resolve(body) })
    const wrapper = mountModal()
    const input = wrapper.find('input[type=file]')
    Object.defineProperty(input.element, 'files', { value: [file] })
    await input.trigger('change')
    await review(wrapper)
    expect(previewImportData.mock.calls[0]![0].data.proxies).toHaveLength(1)

    await wrapper.get('button.btn-secondary').trigger('click')
    await wrapper.find('.border-dashed').trigger('drop', { dataTransfer: { files: [file] } })
    await review(wrapper)
    expect(previewImportData).toHaveBeenCalledTimes(2)
  })

  it('rejects a malformed uploaded file before preview', async () => {
    const wrapper = mountModal()
    const file = new File(['not-json'], 'broken.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', { value: () => Promise.resolve('not-json') })
    const input = wrapper.find('input[type=file]')
    Object.defineProperty(input.element, 'files', { value: [file] })
    await input.trigger('change')
    await review(wrapper)
    expect(previewImportData).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('admin.accounts.bulkImportNoAccounts')
  })

  it.each([1, 10, 20, 50])('imports %i ready rows sequentially in chunks of ten', async (count) => {
    previewImportData.mockResolvedValue(preview(count))
    importData.mockImplementation(
      ({ data }: { data: { accounts: { source_index: number; name: string }[] } }) =>
        Promise.resolve(result(data)),
    )
    const wrapper = mountModal()
    await wrapper
      .get('textarea')
      .setValue(JSON.stringify(Array.from({ length: count }, (_, index) => account(index + 1))))
    await review(wrapper)
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    expect(importData).toHaveBeenCalledTimes(Math.ceil(count / 10))
    for (const call of importData.mock.calls)
      expect(call[0].data.accounts.length).toBeLessThanOrEqual(10)
  })

  it('keeps invalid and duplicate rows in the final partial-success results', async () => {
    previewImportData.mockResolvedValue({
      total: 3,
      ready: 1,
      duplicate: 1,
      invalid: 1,
      items: [
        { index: 1, status: 'ready', name: 'account-1', platform: 'anthropic' },
        { index: 2, status: 'duplicate', name: 'account-2', platform: 'anthropic' },
        {
          index: 3,
          status: 'invalid',
          name: 'account-3',
          platform: 'anthropic',
          message: 'api_key is required',
        },
      ],
    })
    importData.mockResolvedValue({
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 1,
      account_skipped: 1,
      account_failed: 0,
      items: [
        { index: 1, action: 'created', name: 'account-1' },
        { index: 2, action: 'skipped', name: 'account-2', message: 'duplicate account' },
      ],
    })
    const wrapper = mountModal()
    await wrapper.get('textarea').setValue(JSON.stringify([account(1), account(2), account(3)]))
    await review(wrapper)
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    expect(importData.mock.calls[0]![0].data.accounts).toHaveLength(2)
    expect(wrapper.findAll('tbody tr')).toHaveLength(3)
    expect(wrapper.text()).toContain('api_key is required')
    expect(wrapper.text()).toContain('duplicate account')
    expect(wrapper.text()).not.toContain('secret-3')
  })

  it('reuses a chunk idempotency key after an ambiguous request failure', async () => {
    previewImportData.mockResolvedValue(preview(2))
    importData.mockRejectedValueOnce(new Error('network interrupted'))
    importData.mockResolvedValueOnce({
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 2,
      account_failed: 0,
      items: [
        { index: 1, action: 'created', name: 'account-1' },
        { index: 2, action: 'created', name: 'account-2' },
      ],
    })
    const wrapper = mountModal()
    await wrapper.get('textarea').setValue(JSON.stringify([account(1), account(2)]))
    await review(wrapper)
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('bulkImportRetryFailed'))!
      .trigger('click')
    await flushPromises()
    expect(importData).toHaveBeenCalledTimes(2)
    expect(importData.mock.calls[1]![1]).toBe(importData.mock.calls[0]![1])
    expect(importData.mock.calls[1]![0].data.accounts).toHaveLength(2)
  })

  it('uses a new idempotency key when retrying a structured row failure', async () => {
    previewImportData.mockResolvedValue(preview(1))
    importData.mockResolvedValueOnce({
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 0,
      account_failed: 1,
      items: [
        { index: 1, action: 'failed', name: 'account-1', message: 'account could not be created' },
      ],
    })
    importData.mockResolvedValueOnce({
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 1,
      account_failed: 0,
      items: [{ index: 1, action: 'created', name: 'account-1' }],
    })
    const wrapper = mountModal()
    await wrapper.get('textarea').setValue(JSON.stringify(account(1)))
    await review(wrapper)
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('bulkImportRetryFailed'))!
      .trigger('click')
    await flushPromises()
    expect(importData.mock.calls[1]![1]).not.toBe(importData.mock.calls[0]![1])
    expect(wrapper.text()).toContain('created')
    expect(wrapper.text()).not.toContain('account could not be created')
  })

  it('preserves an ambiguous chunk key when structured and transport failures are retried together', async () => {
    previewImportData.mockResolvedValue(preview(11))
    importData.mockImplementationOnce(
      ({ data }: { data: { accounts: { source_index: number; name: string }[] } }) =>
        Promise.resolve({
          ...result(data),
          account_created: 9,
          account_failed: 1,
          items: data.accounts.map((row) =>
            row.source_index === 1
              ? {
                  index: row.source_index,
                  action: 'failed',
                  name: row.name,
                  message: 'account could not be created',
                }
              : { index: row.source_index, action: 'created', name: row.name },
          ),
        }),
    )
    importData.mockRejectedValueOnce(new Error('network interrupted'))
    importData.mockImplementation(
      ({ data }: { data: { accounts: { source_index: number; name: string }[] } }) =>
        Promise.resolve(result(data)),
    )
    const wrapper = mountModal()
    await wrapper
      .get('textarea')
      .setValue(JSON.stringify(Array.from({ length: 11 }, (_, index) => account(index + 1))))
    await review(wrapper)
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    const ambiguousKey = importData.mock.calls[1]![1]
    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('bulkImportRetryFailed'))!
      .trigger('click')
    await flushPromises()
    expect(
      importData.mock.calls[2]![0].data.accounts.map(
        (row: { source_index: number }) => row.source_index,
      ),
    ).toEqual([1])
    expect(importData.mock.calls[2]![1]).not.toBe(importData.mock.calls[0]![1])
    expect(
      importData.mock.calls[3]![0].data.accounts.map(
        (row: { source_index: number }) => row.source_index,
      ),
    ).toEqual([11])
    expect(importData.mock.calls[3]![1]).toBe(ambiguousKey)
  })

  it('does not persist drafts and clears secret input when hidden', async () => {
    const storage = vi.spyOn(Storage.prototype, 'setItem')
    const wrapper = mountModal()
    await wrapper.get('textarea').setValue(JSON.stringify(account(1)))
    expect(storage).not.toHaveBeenCalled()
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
    storage.mockRestore()
  })

  it('does not restore credentials when a file read finishes after close', async () => {
    let finishRead!: (value: string) => void
    const body = JSON.stringify(account(1))
    const file = new File([body], 'accounts.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: () =>
        new Promise<string>((resolve) => {
          finishRead = resolve
        }),
    })
    const wrapper = mountModal()
    const input = wrapper.find('input[type=file]')
    Object.defineProperty(input.element, 'files', { value: [file] })
    await input.trigger('change')
    const reviewPromise = wrapper.get('button.btn-primary').trigger('click')
    await wrapper.setProps({ show: false })
    finishRead(body)
    await reviewPromise
    await flushPromises()

    expect(previewImportData).not.toHaveBeenCalled()
    await wrapper.setProps({ show: true })
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('')
  })

  it('resets shared options and result filters between imports', async () => {
    previewImportData.mockResolvedValue(preview(1))
    importData.mockImplementation(
      ({ data }: { data: { accounts: { source_index: number; name: string }[] } }) =>
        Promise.resolve(result(data)),
    )
    const wrapper = mountModal()
    await wrapper.get('textarea').setValue(JSON.stringify(account(1)))
    await review(wrapper)
    await wrapper
      .findAll('select')
      .find((select) => select.find('option[value="disabled"]').exists())!
      .setValue('disabled')
    await wrapper.get('input[type=number]').setValue('7')
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    await wrapper
      .findAll('button')
      .find((button) => button.text().includes('bulkImportFailed'))!
      .trigger('click')

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await wrapper.get('textarea').setValue(JSON.stringify(account(1)))
    await review(wrapper)
    expect(previewImportData.mock.calls[2]![0].options).toEqual({})
    await wrapper.get('button.btn-primary').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
  })
})
