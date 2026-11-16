import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

function readSource(path: string): string {
  return readFileSync(resolve(path), 'utf8')
}

describe('admin platform filters', () => {
  it('uses the group platform catalog on the subscriptions page', () => {
    const source = readSource('src/views/admin/SubscriptionsView.vue')
    expect(source).toContain("import { GROUP_PLATFORM_OPTIONS } from '@/constants/platforms'")
    // The Filters popover renders the catalog as logo chips.
    expect(source).toMatch(/const platformChipOptions = GROUP_PLATFORM_OPTIONS/)
  })

  it('uses the shared catalogs on the groups page', () => {
    const source = readSource('src/views/admin/GroupsView.vue')
    expect(source).toContain('...GROUP_PLATFORM_OPTIONS')
    expect(source).toContain('...CONCRETE_PLATFORM_OPTIONS')
  })

  it('uses the concrete platform catalog wherever concrete platforms are selected', () => {
    for (const path of [
      'src/components/admin/ErrorPassthroughRulesModal.vue',
      'src/views/admin/ops/components/OpsDashboardHeader.vue'
    ]) {
      const source = readSource(path)
      expect(source).toContain("import { CONCRETE_PLATFORM_OPTIONS } from '@/constants/platforms'")
      expect(source).toMatch(/platformOptions\s*=.*CONCRETE_PLATFORM_OPTIONS|pOpts.*\.\.\.CONCRETE_PLATFORM_OPTIONS/s)
    }
  })
  it('offers account provider chips only for platforms present in the fleet', () => {
    const source = readSource('src/views/admin/AccountsView.vue')
    expect(source).toMatch(/const providerChips = computed\(\(\) => \{\n {2}const counts = \{ \.\.\.knownPlatformCounts\.value \}/)
    expect(readSource('src/components/admin/account/AccountTableFilters.vue')).toContain(':data-testid="`provider-chip-${platform.value}`"')
  })
})
