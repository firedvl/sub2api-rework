import { describe, expect, it } from 'vitest'
import { getOperatorArea, operatorAreas } from '../operatorNavigation'

describe('operator navigation', () => {
  it('places Stats between Overview and Accounts', () => {
    expect(operatorAreas.map((area) => area.id)).toEqual([
      'overview',
      'stats',
      'accounts',
      'models-routing',
      'activity',
      'settings',
    ])
    expect(getOperatorArea('/admin/stats')?.primaryPath).toBe('/admin/stats')
  })
})
