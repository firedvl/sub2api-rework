<template>
  <div class="psr">
    <div class="psr-head">
      <span class="psr-icon" :class="tone === 'ok' ? 'is-ok' : ''" aria-hidden="true">
        <Icon :name="icon" size="sm" />
      </span>
      <div class="psr-text">
        <h3 :id="titleId" class="psr-title">
          <span>{{ title }}</span>
          <slot name="badge" />
        </h3>
        <p v-if="$slots.default" class="psr-sub"><slot /></p>
      </div>
      <div v-if="$slots.actions" class="psr-actions"><slot name="actions" /></div>
    </div>
    <slot name="panel" />
  </div>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'

/** One settings row inside a profile section card: icon, title, one-line status, actions, optional panel. */
defineProps<{
  icon: InstanceType<typeof Icon>['$props']['name']
  title: string
  titleId?: string
  tone?: 'ok' | 'neutral'
}>()
</script>

<style scoped>
.psr {
  padding: 0.875rem 1.125rem;
}

.psr + .psr {
  border-top: 1px solid var(--operator-border-subtle);
}

.psr-head {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.75rem;
}

.psr-icon {
  display: inline-grid;
  width: 2.125rem;
  height: 2.125rem;
  flex: none;
  place-items: center;
  border-radius: var(--operator-radius-sm);
  background: var(--operator-raised);
  color: var(--operator-subtle-foreground);
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out),
    color var(--operator-duration-fast) var(--operator-ease-out);
}

.psr-icon.is-ok {
  background: var(--operator-success-soft);
  color: var(--operator-success);
}

.psr-text {
  min-width: 0;
  flex: 1;
}

.psr-title {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 600;
}

.psr-sub {
  overflow: hidden;
  margin-top: 0.0625rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  text-overflow: ellipsis;
}

.psr-actions {
  display: flex;
  flex: none;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 0.5rem;
}

@media (max-width: 480px) {
  .psr-head {
    flex-wrap: wrap;
  }

  .psr-actions {
    width: 100%;
    justify-content: flex-start;
    padding-left: 2.875rem;
  }
}
</style>
