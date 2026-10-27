import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { applyTheme, installRouteTransitions } from '../viewTransition'

const Page = { template: '<div />' }

function makeRouter() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/a', component: Page },
      { path: '/b', component: Page }
    ]
  })
  installRouteTransitions(router)
  return router
}

function fakeTransition(finished: Promise<void> = Promise.resolve()) {
  return {
    finished,
    ready: Promise.resolve(),
    updateCallbackDone: Promise.resolve(),
    skipTransition: vi.fn()
  }
}

describe('view transitions', () => {
  beforeEach(() => {
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false })))
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    delete (document as Partial<Document>).startViewTransition
    document.documentElement.className = ''
  })

  it('renders directly when the browser has no view transitions', async () => {
    const router = makeRouter()
    await router.push('/a')
    await router.push('/b')
    expect(router.currentRoute.value.path).toBe('/b')
  })

  it('wraps a path change in a route transition and finishes the update after render', async () => {
    let update: (() => Promise<void> | void) | undefined
    let finish: () => void = () => {}
    const transition = fakeTransition(new Promise<void>((resolve) => { finish = resolve }))
    document.startViewTransition = vi.fn((cb) => {
      update = cb as typeof update
      return transition as unknown as ViewTransition
    }) as typeof document.startViewTransition

    const router = makeRouter()
    await router.push('/a')
    expect(document.startViewTransition).not.toHaveBeenCalled()

    const navigation = router.push('/b')
    await vi.waitFor(() => expect(update).toBeDefined())
    expect(document.documentElement.classList.contains('route-transition')).toBe(true)
    const updateDone = update?.()
    await navigation
    await updateDone
    expect(router.currentRoute.value.path).toBe('/b')
    expect(transition.skipTransition).not.toHaveBeenCalled()
    finish()
    await transition.finished
    await Promise.resolve()
    expect(document.documentElement.classList.contains('route-transition')).toBe(false)
  })

  it('skips the animation and still navigates when the old page is never captured', async () => {
    vi.useFakeTimers()
    const transition = fakeTransition()
    document.startViewTransition = vi.fn(() => transition as unknown as ViewTransition) as typeof document.startViewTransition

    const router = makeRouter()
    await router.push('/a')
    const navigation = router.push('/b')
    await vi.advanceTimersByTimeAsync(200)
    await navigation
    expect(router.currentRoute.value.path).toBe('/b')
    expect(transition.skipTransition).toHaveBeenCalled()
  })

  it('does not animate query-only changes', async () => {
    document.startViewTransition = vi.fn(() => fakeTransition() as unknown as ViewTransition) as typeof document.startViewTransition
    const router = makeRouter()
    await router.push('/a')
    await router.push('/a?tab=2')
    expect(document.startViewTransition).not.toHaveBeenCalled()
  })

  it('stores the theme and toggles the dark class', () => {
    applyTheme(true)
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')
    applyTheme(false)
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(localStorage.getItem('theme')).toBe('light')
  })
})
