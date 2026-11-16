export const ACCENTS = ['iris', 'azure', 'cyan', 'magenta'] as const
export type Accent = (typeof ACCENTS)[number]

/** RGB channels of each accent's 500 shade, for swatches. */
export const ACCENT_SWATCH: Record<Accent, string> = {
  iris: '109 94 252',
  azure: '59 130 246',
  cyan: '6 182 212',
  magenta: '236 72 153',
}

const STORAGE_KEY = 'accent'

export function readAccent(): Accent {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    return (ACCENTS as readonly string[]).includes(saved ?? '') ? (saved as Accent) : 'iris'
  } catch {
    return 'iris'
  }
}

/** Applies an accent to <html> and remembers it. Iris is the default and needs no attribute. */
export function applyAccent(accent: Accent): void {
  const root = document.documentElement
  if (accent === 'iris') delete root.dataset.accent
  else root.dataset.accent = accent
  try {
    localStorage.setItem(STORAGE_KEY, accent)
  } catch {
    // Private mode: the accent still applies for this session.
  }
}
