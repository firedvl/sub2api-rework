/**
 * Feeds the pointer position to the card under it so style.css can draw a soft
 * spotlight (`--mx`/`--my`). One delegated listener covers every card.
 */
export function installCardSpotlight(): void {
  if (typeof window === 'undefined') return
  if (!window.matchMedia('(hover: hover) and (pointer: fine)').matches) return
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
  let frame = 0
  let last: PointerEvent | null = null
  window.addEventListener(
    'pointermove',
    (event) => {
      last = event
      if (frame) return
      frame = requestAnimationFrame(() => {
        frame = 0
        const target = last?.target
        if (!(target instanceof Element)) return
        const card = target.closest<HTMLElement>('.viz-card, .ui-card')
        if (!card || !last) return
        const rect = card.getBoundingClientRect()
        card.style.setProperty('--mx', `${last.clientX - rect.left}px`)
        card.style.setProperty('--my', `${last.clientY - rect.top}px`)
      })
    },
    { passive: true },
  )
}
