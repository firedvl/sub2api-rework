<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Icon } from '@/components/icons'
import PrototypeA from './PrototypeA.vue'
import PrototypeB from './PrototypeB.vue'
import PrototypeC from './PrototypeC.vue'
import PrototypeD from './PrototypeD.vue'
import { labPages, prototypes, type LabPage, type PrototypeKey } from './data'

const route = useRoute()
const router = useRouter()
const components = { a: PrototypeA, b: PrototypeB, c: PrototypeC, d: PrototypeD }

const prototype = computed<PrototypeKey>(() => {
  const candidate = String(route.query.variant || '').toLowerCase()
  return prototypes.some((item) => item.key === candidate) ? candidate as PrototypeKey : 'a'
})

const page = computed<LabPage>(() => {
  const candidate = String(route.query.page || '').toLowerCase()
  return labPages.some((item) => item.key === candidate) ? candidate as LabPage : 'overview'
})

const currentComponent = computed(() => components[prototype.value])
const currentPrototype = computed(() => prototypes.find((item) => item.key === prototype.value) || prototypes[0])

function updateQuery(next: { variant?: PrototypeKey; page?: LabPage }) {
  void router.replace({
    query: {
      ...route.query,
      variant: next.variant || prototype.value,
      page: next.page || page.value,
    },
  })
}

function cyclePrototype(direction: -1 | 1) {
  const currentIndex = prototypes.findIndex((item) => item.key === prototype.value)
  const nextIndex = (currentIndex + direction + prototypes.length) % prototypes.length
  updateQuery({ variant: prototypes[nextIndex].key })
}

function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
  const target = event.target as HTMLElement | null
  if (target?.matches('input, textarea, select, [contenteditable="true"]')) return
  event.preventDefault()
  cyclePrototype(event.key === 'ArrowLeft' ? -1 : 1)
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <main class="operator-lab" data-testid="operator-prototype-lab">
    <component :is="currentComponent" :page="page" @navigate="updateQuery({ page: $event })" />

    <nav class="prototype-switcher" aria-label="Prototype comparison">
      <span class="prototype-switcher-kicker">DEV LAB</span>
      <button class="switcher-arrow" type="button" title="Previous prototype" aria-label="Previous prototype" @click="cyclePrototype(-1)">
        <Icon name="arrowLeft" size="sm" />
      </button>
      <div class="prototype-options" role="group" aria-label="Choose prototype">
        <button
          v-for="item in prototypes"
          :key="item.key"
          type="button"
          :aria-pressed="prototype === item.key"
          :class="{ active: prototype === item.key }"
          @click="updateQuery({ variant: item.key })"
        >
          {{ item.key.toUpperCase() }}
        </button>
      </div>
      <button class="switcher-arrow" type="button" title="Next prototype" aria-label="Next prototype" @click="cyclePrototype(1)">
        <Icon name="arrowRight" size="sm" />
      </button>
      <span class="prototype-current" aria-live="polite">
        <strong>{{ currentPrototype.name }}</strong>
        <span>{{ currentPrototype.descriptor }}</span>
      </span>
    </nav>
  </main>
</template>

<style scoped>
.operator-lab {
  min-height: 100vh;
  padding-bottom: 72px;
  background: #080808;
  color: #f4f4f3;
  letter-spacing: 0;
}

.prototype-switcher {
  position: fixed;
  z-index: 100;
  left: 50%;
  bottom: 14px;
  display: flex;
  min-height: 46px;
  max-width: calc(100vw - 28px);
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  transform: translateX(-50%);
  border: 1px solid #444441;
  border-radius: 6px;
  background: #151514;
  box-shadow: 0 16px 40px rgb(0 0 0 / 45%);
  color: #f7f7f5;
}

.prototype-switcher-kicker {
  padding: 0 7px;
  color: #a6a6a0;
  font: 700 10px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
}

.switcher-arrow,
.prototype-options button {
  display: inline-flex;
  min-width: 34px;
  min-height: 34px;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
  border-radius: 4px;
  background: transparent;
  color: #bdbdb8;
  cursor: pointer;
}

.switcher-arrow:hover,
.prototype-options button:hover {
  background: #242422;
  color: #fff;
}

.switcher-arrow:focus-visible,
.prototype-options button:focus-visible {
  outline: 2px solid #d7e265;
  outline-offset: 2px;
}

.prototype-options {
  display: flex;
  gap: 2px;
}

.prototype-options button {
  font: 700 12px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
}

.prototype-options button.active {
  border-color: #696963;
  background: #f2f2ee;
  color: #11110f;
}

.prototype-current {
  display: grid;
  min-width: 210px;
  padding: 0 8px 0 4px;
  line-height: 1.15;
}

.prototype-current strong {
  font-size: 12px;
  font-weight: 650;
}

.prototype-current span {
  margin-top: 3px;
  color: #999992;
  font-size: 10px;
}

@media (max-width: 767px) {
  .operator-lab {
    padding-bottom: 116px;
  }

  .prototype-switcher {
    width: calc(100vw - 24px);
    flex-wrap: wrap;
    justify-content: center;
  }

  .prototype-current {
    width: 100%;
    min-width: 0;
    text-align: center;
  }

  .prototype-current span {
    display: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .operator-lab * {
    scroll-behavior: auto !important;
    transition-duration: 1ms !important;
    animation-duration: 1ms !important;
  }
}
</style>
