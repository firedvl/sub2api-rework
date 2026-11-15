<template>
  <svg class="auth-flow" viewBox="0 0 560 296" aria-hidden="true">
    <path v-for="wire in wires" :key="wire.path" class="auth-flow-wire" :d="wire.path" />
    <circle v-for="wire in wires" :key="`packet-${wire.path}`" class="auth-flow-packet" r="3">
      <animateMotion dur="2.6s" :begin="`${wire.delay}s`" repeatCount="indefinite" :path="wire.path" />
    </circle>

    <g v-for="client in clients" :key="client.name">
      <rect class="auth-flow-node" x="20" :y="client.y" width="118" height="40" rx="10" />
      <text x="79" :y="client.y + 25" text-anchor="middle">{{ client.name }}</text>
    </g>

    <g v-for="provider in providers" :key="provider.platform">
      <rect class="auth-flow-node" x="420" :y="provider.y" width="120" height="42" rx="10" />
      <!-- The logo is HTML inside the SVG so its own sizing classes apply. -->
      <foreignObject x="432" :y="provider.y + 12" width="18" height="18">
        <span class="auth-flow-logo"><PlatformIcon :platform="provider.platform" size="lg" /></span>
      </foreignObject>
      <text x="458" :y="provider.y + 26">{{ provider.name }}</text>
    </g>

    <circle class="auth-flow-halo" cx="280" cy="150" r="48" />
    <circle class="auth-flow-hub" cx="280" cy="150" r="34" />
    <g transform="translate(258 128) scale(1.375)" fill="none" stroke="#fff" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
      <path d="M7.5 10.5h8.25A4.25 4.25 0 0 1 20 14.75v.5" />
      <path d="m16.75 12 3.25 3.25L23.25 12" />
      <path d="M24.5 21.5h-8.25A4.25 4.25 0 0 1 12 17.25v-.5" />
      <path d="M15.25 20 12 16.75 8.75 20" />
    </g>
  </svg>
</template>

<script setup lang="ts">
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { providerName } from '@/utils/providerSeries'

/** Decorative diagram for the sign-in page: clients route through the gateway to providers. */
const clients = [
  { name: 'Claude Code', y: 46 },
  { name: 'Codex', y: 128 },
  { name: 'API', y: 210 },
]

const providers = (['openai', 'anthropic', 'gemini', 'antigravity'] as const).map((platform, index) => ({
  platform,
  name: providerName(platform),
  y: 22 + index * 70,
}))

const wires = [
  ...clients.map((client, index) => ({
    path: `M138 ${client.y + 20}C196 ${client.y + 20} 190 150 246 150`,
    delay: index * 0.7,
  })),
  ...providers.map((provider, index) => ({
    path: `M314 150C370 150 364 ${provider.y + 21} 420 ${provider.y + 21}`,
    delay: 0.4 + index * 0.55,
  })),
]
</script>

<style scoped>
.auth-flow {
  display: block;
  width: 100%;
  height: auto;
  color: var(--operator-foreground);
}

.auth-flow-wire {
  fill: none;
  stroke: var(--operator-border-strong);
  stroke-width: 1.5;
}

.auth-flow-node {
  fill: var(--operator-card);
  stroke: var(--operator-border-strong);
}

.auth-flow-hub {
  fill: var(--operator-accent);
}

.auth-flow-halo {
  fill: rgb(var(--operator-accent-rgb) / 0.14);
}

.auth-flow-packet {
  fill: var(--operator-accent-text);
}

.auth-flow-logo {
  display: grid;
  width: 18px;
  height: 18px;
  place-items: center;
  color: var(--operator-foreground);
}

.auth-flow text {
  fill: var(--operator-foreground);
  font-size: 12.5px;
  font-weight: 500;
}

@media (prefers-reduced-motion: reduce) {
  .auth-flow-packet {
    display: none;
  }
}
</style>
