import { sanitizeUrl } from './url'

export const OPERATOR_PRODUCT_NAME = 'Gateway'
export const OPERATOR_PRODUCT_DESCRIPTOR = 'AI Gateway'
export const TECHNICAL_PRODUCT_NAME = 'Sub2API Rework'

const LEGACY_PRODUCT_NAME = 'sub2api'
const LEGACY_PRODUCT_DESCRIPTORS = new Set([
  'subscription to api conversion platform',
  'ai api gateway platform',
])

export function resolveOperatorProductName(name?: string | null): string {
  const normalized = name?.trim()
  return !normalized || normalized.toLowerCase() === LEGACY_PRODUCT_NAME
    ? OPERATOR_PRODUCT_NAME
    : normalized
}

export function resolveOperatorProductDescriptor(descriptor?: string | null): string {
  const normalized = descriptor?.trim()
  return !normalized || LEGACY_PRODUCT_DESCRIPTORS.has(normalized.toLowerCase())
    ? OPERATOR_PRODUCT_DESCRIPTOR
    : normalized
}

export function updateFavicon(logoUrl: string): void {
  const sanitizedLogoUrl = sanitizeUrl(logoUrl, {
    allowRelative: true,
    allowDataUrl: true,
  })
  if (!sanitizedLogoUrl) {
    return
  }

  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }

  link.type = sanitizedLogoUrl.endsWith('.svg') ? 'image/svg+xml' : 'image/x-icon'
  link.href = sanitizedLogoUrl
}
