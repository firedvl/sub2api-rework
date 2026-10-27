<template>
  <Transition :css="false" @enter="enter" @leave="leave">
    <slot />
  </Transition>
</template>

<script setup lang="ts">
/**
 * Height and opacity transition for content shown with v-if or v-show, such as
 * expandable rows and collapsible groups. Content renders at once when the
 * user prefers reduced motion or the browser lacks the Web Animations API.
 */
type Frame = Record<string, string | number>

const DURATION_MS = 200
const EASING = 'cubic-bezier(0.2, 0.9, 0.3, 1)'

const canAnimate = (el: Element): el is HTMLElement =>
  typeof (el as HTMLElement).animate === 'function' &&
  !window.matchMedia('(prefers-reduced-motion: reduce)').matches

function run(el: Element, done: () => void, from: Frame, to: Frame) {
  if (!canAnimate(el)) {
    done()
    return
  }
  const previousOverflow = el.style.overflow
  el.style.overflow = 'hidden'
  const animation = el.animate([from, to], { duration: DURATION_MS, easing: EASING })
  const finish = () => {
    el.style.overflow = previousOverflow
    done()
  }
  animation.onfinish = finish
  animation.oncancel = finish
}

// Padding collapses with the height so border-box elements can reach zero.
const closed: Frame = { height: '0px', paddingTop: '0px', paddingBottom: '0px', opacity: 0 }

function open(el: Element): Frame {
  const style = getComputedStyle(el)
  return {
    height: `${(el as HTMLElement).offsetHeight}px`,
    paddingTop: style.paddingTop,
    paddingBottom: style.paddingBottom,
    opacity: 1
  }
}

function enter(el: Element, done: () => void) {
  run(el, done, closed, open(el))
}

function leave(el: Element, done: () => void) {
  run(el, done, open(el), closed)
}
</script>
