import { nextTick } from 'vue'
import type { Router } from 'vue-router'

type TransitionKind = 'route' | 'theme'

const kindClass = (kind: TransitionKind) => `${kind}-transition`

/** View transitions run only when the browser supports them and the user allows motion. */
export function canUseViewTransition(): boolean {
  return typeof document !== 'undefined'
    && typeof document.startViewTransition === 'function'
    && document.visibilityState === 'visible'
    && !window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/**
 * Runs a DOM update inside a view transition. The kind is exposed as a class on
 * <html> so style.css can give route changes and theme changes their own motion.
 * Returns null when the update ran without a transition.
 */
export function runViewTransition(
  kind: TransitionKind,
  update: () => void | Promise<void>
): ViewTransition | null {
  if (!canUseViewTransition()) {
    void update()
    return null
  }
  const root = document.documentElement
  root.classList.add(kindClass(kind))
  const transition = document.startViewTransition(update)
  transition.finished.finally(() => root.classList.remove(kindClass(kind)))
  return transition
}

/** Switches the color theme with a short cross-fade and stores the choice. */
export function applyTheme(dark: boolean): void {
  runViewTransition('theme', () => {
    document.documentElement.classList.toggle('dark', dark)
  })
  localStorage.setItem('theme', dark ? 'dark' : 'light')
}

// The browser captures the old page on its next rendered frame. If no frame
// arrives in time (a busy main thread, a page that is not being painted), the
// navigation goes ahead without the animation instead of waiting.
const CAPTURE_DEADLINE_MS = 150

/**
 * Cross-fades page content between routes. Query-only changes (filters, tabs
 * synced to the URL) update in place, and the first navigation renders directly.
 */
export function installRouteTransitions(router: Router): void {
  let finishUpdate: (() => void) | null = null
  // Rendering stays paused until the update callback settles, so never leave one open.
  const finishPending = () => {
    const done = finishUpdate
    finishUpdate = null
    if (done) void nextTick(done)
  }

  router.beforeResolve((to, from) => {
    finishPending()
    if (!from.matched.length || to.path === from.path || !canUseViewTransition()) return
    return new Promise<void>((resolve) => {
      let proceeded = false
      const proceed = () => {
        if (proceeded) return
        proceeded = true
        resolve()
      }
      const transition = runViewTransition('route', () => new Promise<void>((done) => {
        if (proceeded) {
          done()
          return
        }
        finishUpdate = done
        proceed()
      }))
      window.setTimeout(() => {
        if (proceeded) return
        transition?.skipTransition()
        proceed()
      }, CAPTURE_DEADLINE_MS)
    })
  })

  // Resolve after Vue renders the new route so the transition captures it.
  router.afterEach(finishPending)
}
