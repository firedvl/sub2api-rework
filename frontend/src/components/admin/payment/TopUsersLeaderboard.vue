<template>
  <section class="viz-card" aria-labelledby="payment-top-users-title" data-testid="payment-top-users">
    <header class="viz-card-head">
      <h2 id="payment-top-users-title" class="viz-card-title">{{ t('payment.admin.topUsers') }}</h2>
      <div v-if="groups.length > 1" class="viz-card-tools">
        <div class="viz-seg" role="group" :aria-label="t('payment.admin.currency')">
          <button
            v-for="[currency] in groups"
            :key="currency"
            type="button"
            :aria-pressed="activeCurrency === currency"
            @click="currencyChoice = currency"
          >
            {{ currency }}
          </button>
        </div>
      </div>
    </header>
    <div class="viz-card-body">
      <div v-if="!activeUsers.length" class="ui-empty">
        <span class="ui-empty-icon"><Icon name="trophy" size="md" /></span>
        <b>{{ t('payment.admin.noData') }}</b>
      </div>
      <ol v-else class="top-users">
        <li v-for="(user, idx) in activeUsers" :key="user.user_id" class="top-user">
          <span :class="idx < 3 ? 'top-user-rank is-top' : 'top-user-rank'">{{ idx + 1 }}</span>
          <span class="min-w-0 flex-1 truncate text-sm text-gray-900 dark:text-white" :title="user.email">{{ user.email }}</span>
          <span class="top-user-bar" aria-hidden="true"><i :style="{ width: share(user.amount) + '%' }"></i></span>
          <span class="viz-row-value">{{ formatMoney(activeCurrency, user.amount) }}</span>
        </li>
      </ol>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { TopUserPaymentStats } from '@/types/payment'
import { formatMoney } from './orderDisplay'

const { t } = useI18n()

const props = defineProps<{
  users: Record<string, TopUserPaymentStats[]>
}>()

const groups = computed(() =>
  Object.entries(props.users || {})
    .filter(([, users]) => users.length > 0)
    .sort(([left], [right]) => left.localeCompare(right)),
)

const currencyChoice = ref('')
const activeCurrency = computed(() =>
  groups.value.some(([currency]) => currency === currencyChoice.value) ? currencyChoice.value : (groups.value[0]?.[0] ?? ''),
)
const activeUsers = computed(() => groups.value.find(([currency]) => currency === activeCurrency.value)?.[1] ?? [])
const maxAmount = computed(() => Math.max(1, ...activeUsers.value.map((user) => user.amount)))

function share(amount: number): number {
  return Math.max(2, Math.min(100, (amount / maxAmount.value) * 100))
}
</script>

<style scoped>
.top-users {
  display: grid;
  gap: 2px;
}

.top-user {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.75rem;
  margin: 0 -0.5rem;
  padding: 0.4375rem 0.5rem;
  border-radius: var(--operator-radius);
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.top-user:hover {
  background: var(--operator-muted);
}

.top-user-rank {
  display: inline-grid;
  width: 1.375rem;
  height: 1.375rem;
  flex: none;
  place-items: center;
  border-radius: 999px;
  background: var(--operator-raised);
  color: var(--operator-subtle-foreground);
  font-size: 0.6875rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.top-user-rank.is-top {
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
}

.top-user-bar {
  display: block;
  width: 5rem;
  height: 6px;
  flex: none;
  overflow: hidden;
  border-radius: 0 4px 4px 0;
  background: var(--operator-raised);
}

.top-user-bar i {
  display: block;
  height: 100%;
  border-radius: 0 4px 4px 0;
  background: var(--operator-accent);
  transform-origin: left;
  animation: viz-grow-x 700ms var(--operator-ease-out) both;
}

@media (max-width: 480px) {
  .top-user-bar {
    display: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .top-user-bar i {
    animation: none;
  }
}
</style>
