<template>
  <div class="table-page-layout" :class="{ 'mobile-mode': isMobile }">
    <!-- 固定区域：操作按钮 -->
    <div v-if="$slots.actions" class="layout-section-fixed">
      <slot name="actions" />
    </div>

    <!-- 固定区域：搜索和过滤器 -->
    <div v-if="$slots.filters" class="layout-section-fixed">
      <slot name="filters" />
    </div>

    <!-- 滚动区域：表格 -->
    <div class="layout-section-scrollable">
      <div class="card table-scroll-container">
        <slot name="table" />
      </div>
    </div>

    <!-- 固定区域：分页器 -->
    <div v-if="$slots.pagination" class="layout-section-fixed">
      <slot name="pagination" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const isMobile = ref(false)

const checkMobile = () => {
  isMobile.value = window.innerWidth < 1024
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped>
.table-page-layout {
  @apply flex min-h-0 min-w-0 flex-1 flex-col;
  gap: 1rem;
}

.layout-section-fixed {
  @apply flex-shrink-0;
}

.layout-section-scrollable {
  @apply flex min-h-0 min-w-0 flex-1 flex-col;
}

/* The table body scrolls under a sticky header inside one bordered surface. */
.table-scroll-container {
  @apply flex h-full min-w-0 flex-col overflow-hidden;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-card);
  box-shadow: var(--operator-shadow-xs);
}

.table-scroll-container :deep(.table-wrapper) {
  @apply flex-1 overflow-x-auto overflow-y-auto;
  scrollbar-gutter: stable;
}

.table-scroll-container :deep(table) {
  @apply w-full;
  min-width: max-content;
  display: table;
}

.table-scroll-container :deep(thead) {
  background: var(--table-header-background);
}

.table-scroll-container :deep(th) {
  padding: 0.625rem 1rem;
  border-bottom: 1px solid var(--operator-border);
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
  font-weight: 500;
  text-align: left;
}

.table-scroll-container :deep(td) {
  padding: 0.75rem 1rem;
  color: var(--operator-foreground);
  font-size: 0.875rem;
}

.table-scroll-container :deep(tbody > tr + tr > td) {
  border-top: 1px solid var(--operator-border-subtle);
}

/* Narrow layouts: DataTable renders cards, so the frame steps aside. */
.table-page-layout.mobile-mode .table-scroll-container {
  @apply h-auto overflow-visible;
  border: 0;
  background: transparent;
  box-shadow: none;
}

.table-page-layout.mobile-mode .layout-section-scrollable {
  @apply min-h-fit flex-none;
}

.table-page-layout.mobile-mode .table-scroll-container :deep(table) {
  @apply flex-none;
  display: table;
  min-width: 100%;
}
</style>
