import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { beforeEach, describe, expect, it } from 'vitest'
import {
  OPERATOR_PRODUCT_DESCRIPTOR,
  OPERATOR_PRODUCT_NAME,
  TECHNICAL_PRODUCT_NAME,
  resolveOperatorProductDescriptor,
  resolveOperatorProductName,
  updateFavicon,
} from '@/utils/branding'

const frontendRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../../..')

describe('operator identity', () => {
  it('normalizes only the legacy defaults', () => {
    expect(resolveOperatorProductName()).toBe(OPERATOR_PRODUCT_NAME)
    expect(resolveOperatorProductName(' Sub2API ')).toBe(OPERATOR_PRODUCT_NAME)
    expect(resolveOperatorProductName('Private Gateway')).toBe('Private Gateway')
    expect(resolveOperatorProductDescriptor('Subscription to API Conversion Platform'))
      .toBe(OPERATOR_PRODUCT_DESCRIPTOR)
    expect(resolveOperatorProductDescriptor('Internal routing plane')).toBe('Internal routing plane')
    expect(TECHNICAL_PRODUCT_NAME).toBe('Sub2API Rework')
  })

  it('ships an English Gateway document and monochrome default mark', () => {
    const html = readFileSync(resolve(frontendRoot, 'index.html'), 'utf8')
    const logo = readFileSync(resolve(frontendRoot, 'public/logo.svg'), 'utf8')
    const setup = readFileSync(resolve(frontendRoot, 'src/views/setup/SetupWizardView.vue'), 'utf8')

    expect(html).toContain('<html lang="en">')
    expect(html).toContain('<title data-operator-brand>Gateway</title>')
    expect(html).toContain('href="/logo.svg"')
    expect(logo).toContain('<title id="title">Gateway</title>')
    expect(logo.match(/#[\da-f]{6}/gi)).toEqual(['#111111', '#ffffff'])
    expect(logo).not.toMatch(/gradient/i)
    expect(setup).toContain('class="auth-codex-shell"')
    expect(setup).toContain('src="/logo.svg" alt="" aria-hidden="true"')
    expect(setup).not.toContain('name="cog"')
  })
})

describe('updateFavicon', () => {
  beforeEach(() => {
    document.head.innerHTML = '<link rel="icon" href="/logo.svg">'
  })

  it('replaces the default favicon with the configured logo', () => {
    updateFavicon('https://example.com/custom-logo.png')

    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.href).toBe('https://example.com/custom-logo.png')
  })

  it('ignores unsafe logo URLs', () => {
    updateFavicon('javascript:alert(1)')

    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.getAttribute('href')).toBe('/logo.svg')
  })
})
