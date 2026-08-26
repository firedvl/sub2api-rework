import { describe, expect, it } from 'vitest'
import {
  getOperatorFixtureData,
  isOperatorFixtureReadRequest,
  operatorFixtureAccounts,
} from '../../e2e/fixtures/operatorData'

describe('operator fixture data', () => {
  it('serves representative account states and rejects normal writes', () => {
    const response = getOperatorFixtureData('/api/v1/admin/accounts') as {
      items: typeof operatorFixtureAccounts
    }

    expect(new Set(response.items.map((account) => account.status))).toEqual(
      new Set(['active', 'error', 'inactive']),
    )
    expect(isOperatorFixtureReadRequest('POST', '/api/v1/admin/accounts/101/test')).toBe(false)
    expect(isOperatorFixtureReadRequest('POST', '/api/v1/admin/accounts/usage/batch')).toBe(true)
  })

  it('keeps dashboard account totals aligned with the account fixture', () => {
    const response = getOperatorFixtureData('/api/v1/admin/dashboard/snapshot-v2') as {
      stats: { total_accounts: number }
    }

    expect(response.stats.total_accounts).toBe(operatorFixtureAccounts.length)
  })
})
