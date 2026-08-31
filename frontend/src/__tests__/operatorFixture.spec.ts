import { describe, expect, it } from 'vitest'
import {
  getOperatorFixtureData,
  isOperatorFixtureReadRequest,
  operatorFixtureAccounts,
  operatorFixtureProxies,
} from '../../e2e/fixtures/operatorData.ts'

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

  it('provides records for selector and open-menu fixture review', () => {
    const proxies = getOperatorFixtureData('/api/v1/admin/proxies') as {
      items: typeof operatorFixtureProxies
    }
    const users = getOperatorFixtureData('/api/v1/admin/usage/search-users') as Array<{
      email: string
    }>

    expect(proxies.items).toEqual(operatorFixtureProxies)
    expect(users).toEqual([{ id: 2, email: 'member@example.test', deleted: false }])
  })
})
