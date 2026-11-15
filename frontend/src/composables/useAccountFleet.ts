import { onBeforeUnmount, ref, shallowRef } from 'vue'
import { adminAPI } from '@/api/admin'
import { supportsBatchAccountUsage } from '@/utils/operatorCapacity'
import type { Account, AccountUsageInfo } from '@/types'

/**
 * Loads every account and its passive capacity snapshot. Overview and Stats
 * share this loader. The lists are shallow refs: the fleet can hold thousands
 * of rows that are replaced as a whole, never mutated in place.
 */
export function useAccountFleet(snapshotFailedMessage: () => string) {
  const accounts = shallowRef<Account[]>([])
  const usageByAccountId = shallowRef<Record<string, AccountUsageInfo | null>>({})
  const errorsByAccountId = shallowRef<Record<string, string | null>>({})
  const loading = ref(false)
  const error = ref(false)
  let sequence = 0

  async function load() {
    const current = ++sequence
    loading.value = true
    error.value = false
    try {
      const pageSize = 1000
      const firstPage = await adminAPI.accounts.list(1, pageSize, { include_scheduler_score: '0' })
      const rows = [...firstPage.items]
      for (let page = 2; page <= firstPage.pages; page += 1) {
        const response = await adminAPI.accounts.list(page, pageSize, { include_scheduler_score: '0' })
        if (current !== sequence) return
        rows.push(...response.items)
      }
      if (current !== sequence) return

      const usageAccountIDs = rows.filter(supportsBatchAccountUsage).map((account) => account.id)
      let usage: Record<string, AccountUsageInfo | null> = {}
      let errors: Record<string, string | null> = {}
      if (usageAccountIDs.length) {
        try {
          const response = await adminAPI.accounts.getBatchUsage(usageAccountIDs, false)
          usage = response.usage ?? {}
          errors = response.errors ?? {}
        } catch (cause) {
          // The fleet still renders; each account shows that its snapshot failed.
          errors = Object.fromEntries(usageAccountIDs.map((id) => [String(id), snapshotFailedMessage()]))
          console.error('Failed to load passive capacity snapshots:', cause)
        }
      }
      if (current !== sequence) return

      accounts.value = rows
      usageByAccountId.value = usage
      errorsByAccountId.value = errors
    } catch (cause) {
      if (current !== sequence) return
      error.value = true
      console.error('Failed to load the account fleet:', cause)
    } finally {
      if (current === sequence) loading.value = false
    }
  }

  onBeforeUnmount(() => {
    sequence += 1
  })

  return { accounts, usageByAccountId, errorsByAccountId, loading, error, load }
}
