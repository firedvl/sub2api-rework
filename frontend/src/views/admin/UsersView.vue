<template>
  <AppLayout>
    <TablePageLayout class="ui-stagger">
      <template #filters>
        <div class="ui-toolbar">
          <label class="ui-search" :title="t('admin.users.searchUsers')">
            <Icon name="search" size="sm" />
            <input
              v-model="searchQuery"
              type="search"
              :placeholder="t('common.search')"
              :aria-label="t('admin.users.searchUsers')"
              data-test="users-search"
              @input="handleSearch"
            />
          </label>

          <div class="viz-seg" role="group" :aria-label="t('admin.users.columns.status')">
            <button
              v-for="option in statusOptions"
              :key="option.value"
              type="button"
              :aria-pressed="filters.status === option.value"
              :data-test="`users-status-${option.value || 'all'}`"
              @click="setStatusFilter(option.value)"
            >
              <span v-if="option.value === 'active'" class="ui-dot is-ok" aria-hidden="true"></span>
              <span v-else-if="option.value === 'disabled'" class="ui-dot is-bad" aria-hidden="true"></span>
              {{ option.label }}
              <span v-if="filters.status === option.value" class="users-seg-count">{{ pagination.total }}</span>
            </button>
          </div>

          <ToolbarPopover
            :label="t('admin.adminUi.filters')"
            icon="filter"
            :count="activeFilterCount"
            align="left"
            test-id="users-filters"
            @open="loadFilterGroups"
          >
            <div class="users-field">
              <span class="users-field-label">{{ t('admin.users.columns.role') }}</span>
              <Select v-model="filters.role" :options="roleOptions" @change="applyFilter" />
            </div>
            <div class="users-field">
              <span class="users-field-label">{{ t('admin.users.authorizedGroupFilter') }}</span>
              <Select
                v-model="filters.group"
                :options="groupFilterOptions"
                searchable
                creatable
                :creatable-prefix="t('admin.users.fuzzySearch')"
                :search-placeholder="t('admin.users.searchAuthorizedGroups')"
                @change="applyFilter"
              />
            </div>
            <div class="users-field">
              <span class="users-field-label">{{ t('admin.users.apiKeyGroupFilter') }}</span>
              <Select
                v-model="filters.apiKeyGroup"
                :options="apiKeyGroupFilterOptions"
                searchable
                :search-placeholder="t('admin.users.searchApiKeyGroups')"
                @change="applyFilter"
              />
            </div>
            <div v-for="attr in filterableAttributes" :key="attr.id" class="users-field">
              <span class="users-field-label">{{ attr.name }}</span>
              <Select
                v-if="attr.type === 'select' || attr.type === 'multi_select'"
                :model-value="activeAttributeFilters[attr.id] ?? ''"
                :options="[{ value: '', label: t('common.all') }, ...(attr.options || [])]"
                @update:model-value="(val) => { updateAttributeFilter(attr.id, String(val ?? '')); applyFilter() }"
              />
              <input
                v-else
                :value="activeAttributeFilters[attr.id] ?? ''"
                :type="attr.type === 'number' ? 'number' : 'text'"
                :aria-label="attr.name"
                class="input"
                @input="(e) => updateAttributeFilter(attr.id, (e.target as HTMLInputElement).value)"
                @change="applyFilter"
              />
            </div>
            <button
              v-if="activeFilterCount > 0"
              type="button"
              class="btn btn-ghost btn-sm self-start"
              @click="clearFilters"
            >
              <Icon name="x" size="sm" />
              {{ t('admin.adminUi.clearFilters') }}
            </button>
          </ToolbarPopover>

          <span class="ui-grow" />

          <template v-if="selectedCount > 0">
            <button
              type="button"
              class="btn btn-secondary"
              data-test="bulk-edit-limits"
              @click="showBulkEditModal = true"
            >
              <Icon name="users" size="sm" />
              {{ t('admin.users.bulkLimits.action', { count: selectedCount }) }}
            </button>
            <button
              type="button"
              class="btn btn-danger"
              data-test="bulk-delete-users"
              :disabled="bulkDeleting"
              @click="bulkDeleteIds = [...selectedIds]"
            >
              <Icon name="trash" size="sm" />
              {{ t('admin.users.bulkDelete.action', { count: selectedCount }) }}
            </button>
          </template>

          <button
            type="button"
            class="app-icon-button"
            :aria-label="t('common.refresh')"
            :title="t('common.refresh')"
            :disabled="loading"
            @click="loadUsers"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>

          <ToolbarPopover
            v-slot="{ close }"
            :label="t('admin.users.columnSettings')"
            icon="grid"
            icon-only
            width="15rem"
            test-id="users-columns"
          >
            <div class="users-column-list">
              <label
                v-for="col in toggleableColumns"
                :key="col.key"
                class="users-column-option"
                :title="isForcedVisibleColumn(col.key) ? t('admin.users.columnAlwaysVisible') : undefined"
              >
                <input
                  type="checkbox"
                  :checked="isColumnVisible(col.key)"
                  :disabled="isForcedVisibleColumn(col.key)"
                  @change="toggleColumn(col.key)"
                />
                <span>{{ col.label }}</span>
              </label>
            </div>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              @click="close(); showAttributesModal = true"
            >
              <Icon name="cog" size="sm" />
              {{ t('admin.users.attributes.configButton') }}
            </button>
          </ToolbarPopover>

          <button type="button" class="btn btn-primary" @click="showCreateModal = true">
            <Icon name="userPlus" size="sm" />
            {{ t('admin.users.createUser') }}
          </button>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="sortedUsers"
          :loading="loading"
          row-key="id"
          selectable
          clickable-rows
          :selected-keys="selectedIds"
          :selection-label="getUserSelectionLabel"
          :expandable-actions="false"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          :sort-storage-key="USER_SORT_STORAGE_KEY"
          @sort="handleSort"
          @row-click="openUserDrawer"
          @update:selected-keys="handleSelectedKeysUpdate"
        >
          <template #cell-email="{ value, row }">
            <button
              type="button"
              class="users-identity"
              :aria-label="`${t('admin.adminUi.details')}: ${value}`"
              @click.stop="openUserDrawer(row)"
            >
              <span class="users-avatar" aria-hidden="true">{{ userInitial(value) }}</span>
              <span class="min-w-0 text-left">
                <span class="ui-cell-main">{{ value }}</span>
                <span class="ui-cell-sub">{{ row.username || `#${row.id}` }}</span>
              </span>
            </button>
          </template>

          <template #cell-id="{ value }">
            <span class="tabular-nums text-gray-500 dark:text-dark-400">{{ value }}</span>
          </template>

          <template #cell-username="{ value }">
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ value || '-' }}</span>
          </template>

          <template #cell-notes="{ value }">
            <span
              v-if="value"
              :title="value"
              class="block max-w-xs truncate text-sm text-gray-600 dark:text-gray-400"
            >{{ value }}</span>
            <span v-else class="text-sm text-gray-400">-</span>
          </template>

          <!-- Dynamic attribute columns -->
          <template
            v-for="def in attributeDefinitions.filter(d => d.enabled)"
            :key="def.id"
            #[`cell-attr_${def.id}`]="{ row }"
          >
            <span
              class="block max-w-xs truncate text-sm text-gray-700 dark:text-gray-300"
              :title="getAttributeValue(row.id, def.id)"
            >
              {{ getAttributeValue(row.id, def.id) }}
            </span>
          </template>

          <template #cell-role="{ value }">
            <span :class="value === 'admin' ? 'badge badge-primary' : 'badge badge-gray'">
              {{ t('admin.users.roles.' + value) }}
            </span>
          </template>

          <template #cell-groups="{ row }">
            <div v-if="allGroups.length > 0" class="flex flex-col gap-1">
              <span
                v-if="getUserGroups(row).exclusive.length > 0"
                class="group/ex relative inline-flex cursor-pointer items-center gap-1 whitespace-nowrap text-xs"
                @click.stop="toggleExpandedGroup(row.id)"
              >
                <Icon name="shield" size="xs" class="h-3.5 w-3.5 text-primary-500 dark:text-primary-400" />
                <span class="font-medium tabular-nums text-gray-900 dark:text-white">{{ getUserGroups(row).exclusive.length }}</span>
                <span class="text-gray-500 dark:text-dark-400">{{ t('admin.users.exclusiveLabel') }}</span>
                <div
                  v-if="expandedGroupUserId !== row.id"
                  class="tooltip-enter pointer-events-none absolute left-0 top-full z-50 mt-1.5 rounded bg-gray-900 px-2.5 py-1.5 text-xs text-white opacity-0 shadow-lg transition-opacity duration-75 group-hover/ex:opacity-100 dark:bg-dark-600"
                >
                  <div class="flex flex-col gap-0.5 whitespace-nowrap">
                    <span v-for="g in getUserGroups(row).exclusive" :key="g.id">{{ g.name }}</span>
                  </div>
                </div>
                <div
                  v-if="expandedGroupUserId === row.id"
                  class="dropdown left-0 top-full mt-1.5 min-w-[160px] text-xs"
                >
                  <div class="px-2.5 py-1.5 text-[10px] font-medium uppercase tracking-wider text-gray-400 dark:text-dark-400">
                    {{ t('admin.users.clickToReplace') }}
                  </div>
                  <button
                    v-for="g in getUserGroups(row).exclusive"
                    :key="g.id"
                    type="button"
                    class="dropdown-item"
                    @click.stop="openGroupReplace(row, g)"
                  >
                    <Icon name="swap" size="xs" class="h-3.5 w-3.5 flex-shrink-0 opacity-50" />
                    <span class="flex-1">{{ g.name }}</span>
                  </button>
                </div>
              </span>
              <span
                v-if="getUserGroups(row).publicGroups.length > 0"
                class="group/pub relative inline-flex cursor-default items-center gap-1 whitespace-nowrap text-xs"
              >
                <Icon name="globe" size="xs" class="h-3.5 w-3.5 text-gray-400 dark:text-dark-500" />
                <span class="font-medium tabular-nums text-gray-600 dark:text-dark-300">{{ getUserGroups(row).publicGroups.length }}</span>
                <span class="text-gray-400 dark:text-dark-500">{{ t('admin.users.publicLabel') }}</span>
                <div class="pointer-events-none absolute left-0 top-full z-50 mt-1.5 rounded bg-gray-900 px-2.5 py-1.5 text-xs text-white opacity-0 shadow-lg transition-opacity duration-75 group-hover/pub:opacity-100 dark:bg-dark-600">
                  <div class="flex flex-col gap-0.5 whitespace-nowrap">
                    <span v-for="g in getUserGroups(row).publicGroups" :key="g.id">{{ g.name }}</span>
                  </div>
                </div>
              </span>
              <span
                v-if="getUserGroups(row).exclusive.length === 0 && getUserGroups(row).publicGroups.length === 0"
                class="text-xs text-gray-400 dark:text-dark-500"
              >-</span>
            </div>
            <span v-else class="text-xs text-gray-400 dark:text-dark-500">-</span>
          </template>

          <template #cell-subscriptions="{ row }">
            <div
              v-if="row.subscriptions && row.subscriptions.length > 0"
              class="flex flex-wrap gap-1.5"
            >
              <GroupBadge
                v-for="sub in row.subscriptions"
                :key="sub.id"
                :name="sub.group?.name || ''"
                :platform="sub.group?.platform"
                :subscription-type="sub.group?.subscription_type"
                :rate-multiplier="sub.group?.rate_multiplier"
                :days-remaining="sub.expires_at ? getDaysRemaining(sub.expires_at) : null"
                :title="sub.expires_at ? formatDateTime(sub.expires_at) : ''"
              />
            </div>
            <span v-else class="text-xs text-gray-400 dark:text-dark-500" :title="t('admin.users.noSubscription')">-</span>
          </template>

          <template #cell-balance="{ value, row }">
            <button
              type="button"
              class="users-num-link"
              :title="t('admin.users.balanceHistoryTip')"
              @click.stop="handleBalanceHistory(row)"
            >
              ${{ value.toFixed(2) }}
            </button>
          </template>

          <template #cell-balance_platform_quota="{ row }">
            <button
              type="button"
              class="block text-left underline decoration-dashed decoration-gray-300 underline-offset-4 transition-colors hover:decoration-primary-400 dark:decoration-dark-500"
              :title="t('admin.users.platformQuota.cellColumnTooltip')"
              @click.stop="handlePlatformQuota(row)"
            >
              <UserPlatformQuotaCell :quotas="platformQuotaStats[row.id]" />
            </button>
          </template>

          <!-- Usage columns sort current-page data through a today / 30d menu (desc → asc → off). -->
          <template
            v-for="usageKey in USAGE_COLUMN_KEYS"
            :key="usageKey"
            #[`header-${usageKey}`]="{ column }"
          >
            <div class="flex items-center gap-1.5">
              <span>{{ column.label }}</span>
              <div class="usage-sort-trigger relative">
                <button
                  type="button"
                  class="flex items-center gap-1 rounded px-1 py-0.5 transition-colors hover:bg-gray-200 dark:hover:bg-dark-700"
                  :class="usageSort && usageSort.key === usageKey
                    ? 'text-primary-600 dark:text-primary-400'
                    : 'text-gray-400 dark:text-dark-500'"
                  :title="t('admin.users.sortBy')"
                  :aria-label="t('admin.users.sortBy')"
                  :data-test="`usage-sort-trigger-${usageKey}`"
                  @click.stop="toggleUsageSortMenu(usageKey)"
                >
                  <span
                    v-if="usageSort && usageSort.key === usageKey"
                    class="text-[10px] normal-case font-medium tracking-normal"
                  >{{ usageSort.metric === 'today' ? t('admin.users.today') : t('admin.users.total') }}</span>
                  <Icon
                    v-if="usageSort && usageSort.key === usageKey"
                    :name="usageSort.order === 'desc' ? 'arrowDown' : 'arrowUp'"
                    size="xs"
                  />
                  <Icon v-else name="arrowsUpDown" size="xs" />
                </button>
                <div
                  v-if="openUsageSortMenu === usageKey"
                  class="dropdown right-0 top-full mt-1 min-w-[132px]"
                >
                  <button
                    v-for="metric in (['today', 'total'] as const)"
                    :key="metric"
                    type="button"
                    class="dropdown-item justify-between text-xs normal-case tracking-normal"
                    :class="isUsageSortActive(usageKey, metric) ? 'font-medium text-primary-600 dark:text-primary-400' : ''"
                    :data-test="`usage-sort-${usageKey}-${metric}`"
                    @click.stop="toggleUsageSort(usageKey, metric)"
                  >
                    <span>{{ metric === 'today' ? t('admin.users.today') : t('admin.users.total') }}</span>
                    <Icon
                      v-if="getUsageSortOrder(usageKey, metric)"
                      :name="getUsageSortOrder(usageKey, metric) === 'desc' ? 'arrowDown' : 'arrowUp'"
                      size="xs"
                    />
                  </button>
                  <div class="px-2.5 py-1 text-[10px] normal-case tracking-normal text-gray-400 dark:text-dark-500">
                    {{ t('admin.users.sortCurrentPageOnly') }}
                  </div>
                </div>
              </div>
            </div>
          </template>

          <template #cell-usage="{ row }">
            <PlatformUsageBreakdown
              :today="usageStats[row.id]?.today_actual_cost ?? 0"
              :total="usageStats[row.id]?.total_actual_cost ?? 0"
              :by-platform="usageStats[row.id]?.by_platform"
            />
          </template>

          <template #cell-usage_anthropic="{ row }">
            <PlatformCostCell :usage="getPlatformUsage(row.id, 'anthropic')" />
          </template>

          <template #cell-usage_openai="{ row }">
            <PlatformCostCell :usage="getPlatformUsage(row.id, 'openai')" />
          </template>

          <template #cell-usage_gemini="{ row }">
            <PlatformCostCell :usage="getPlatformUsage(row.id, 'gemini')" />
          </template>

          <template #cell-usage_antigravity="{ row }">
            <PlatformCostCell :usage="getPlatformUsage(row.id, 'antigravity')" />
          </template>

          <template #cell-concurrency="{ row }">
            <div class="flex justify-end">
              <UserConcurrencyCell :current="row.current_concurrency ?? 0" :max="row.concurrency" />
            </div>
          </template>

          <template #cell-status="{ value }">
            <span class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <span :class="value === 'active' ? 'ui-dot is-ok' : 'ui-dot is-bad'" aria-hidden="true"></span>
              {{ value === 'active' ? t('common.active') : t('admin.users.disabled') }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm tabular-nums text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>

          <template #cell-last_used_at="{ value }">
            <span class="text-sm tabular-nums text-gray-500 dark:text-dark-400">
              {{ value ? formatDateTime(value) : '-' }}
            </span>
          </template>

          <template #cell-last_active_at="{ value }">
            <span class="text-sm tabular-nums text-gray-500 dark:text-dark-400">
              {{ value ? formatDateTime(value) : '-' }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center justify-end gap-0.5">
              <button
                type="button"
                class="app-icon-button"
                :title="t('common.edit')"
                @click.stop="handleEdit(row)"
              >
                <Icon name="edit" size="sm" />
                <span class="sr-only">{{ t('common.edit') }}</span>
              </button>
              <button
                v-if="row.role !== 'admin'"
                type="button"
                class="app-icon-button"
                :class="row.status === 'active' ? 'users-danger-hover' : 'users-ok-hover'"
                :title="row.status === 'active' ? t('admin.users.disable') : t('admin.users.enable')"
                @click.stop="handleToggleStatus(row)"
              >
                <Icon v-if="row.status === 'active'" name="ban" size="sm" />
                <Icon v-else name="checkCircle" size="sm" />
                <span class="sr-only">{{ row.status === 'active' ? t('admin.users.disable') : t('admin.users.enable') }}</span>
              </button>
              <button
                type="button"
                class="app-icon-button action-menu-trigger"
                :title="t('common.more')"
                aria-haspopup="menu"
                :aria-expanded="activeMenuId === row.id"
                @click.stop="openActionMenu(row, $event)"
              >
                <Icon name="more" size="sm" />
                <span class="sr-only">{{ t('common.more') }}</span>
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.users.noUsersYet')"
              :action-text="t('admin.users.createUser')"
              @action="showCreateModal = true"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Row action menu: teleported so it is never clipped by the table scroller. -->
    <Teleport to="body">
      <div
        v-if="activeMenuUser && menuPosition"
        class="dropdown action-menu-content fixed z-[9999] w-52"
        :style="{ top: menuPosition.top + 'px', left: menuPosition.left + 'px' }"
      >
        <button type="button" class="dropdown-item" @click="handleViewApiKeys(activeMenuUser); closeActionMenu()">
          <Icon name="key" size="sm" class="text-gray-400" />
          {{ t('admin.users.apiKeys') }}
        </button>
        <button type="button" class="dropdown-item" @click="handleAllowedGroups(activeMenuUser); closeActionMenu()">
          <Icon name="users" size="sm" class="text-gray-400" />
          {{ t('admin.users.groups') }}
        </button>
        <button type="button" class="dropdown-item" @click="handlePlatformQuota(activeMenuUser); closeActionMenu()">
          <Icon name="chartBar" size="sm" class="text-gray-400" />
          {{ t('admin.users.platformQuota.menuItem') }}
        </button>
        <div class="my-1 border-t border-gray-100 dark:border-dark-700"></div>
        <button type="button" class="dropdown-item" @click="handleDeposit(activeMenuUser); closeActionMenu()">
          <Icon name="plus" size="sm" class="text-emerald-500" />
          {{ t('admin.users.deposit') }}
        </button>
        <button type="button" class="dropdown-item" @click="handleWithdraw(activeMenuUser); closeActionMenu()">
          <Icon name="minus" size="sm" class="text-amber-500" />
          {{ t('admin.users.withdraw') }}
        </button>
        <button type="button" class="dropdown-item" @click="handleBalanceHistory(activeMenuUser); closeActionMenu()">
          <Icon name="clock" size="sm" class="text-gray-400" />
          {{ t('admin.users.balanceHistory') }}
        </button>
        <template v-if="activeMenuUser.role !== 'admin'">
          <div class="my-1 border-t border-gray-100 dark:border-dark-700"></div>
          <button
            type="button"
            class="dropdown-item text-red-600 dark:text-red-400"
            @click="handleDelete(activeMenuUser); closeActionMenu()"
          >
            <Icon name="trash" size="sm" />
            {{ t('common.delete') }}
          </button>
        </template>
      </div>
    </Teleport>

    <SideDrawer
      :open="drawerUser !== null"
      :title="drawerUser?.email ?? ''"
      :subtitle="drawerUser ? (drawerUser.username ? `${drawerUser.username} · #${drawerUser.id}` : `#${drawerUser.id}`) : undefined"
      width="32rem"
      @close="closeUserDrawer"
    >
      <template #leading>
        <span v-if="drawerUser" class="users-avatar is-lg" aria-hidden="true">{{ userInitial(drawerUser.email) }}</span>
      </template>
      <template #badge>
        <span v-if="drawerUser" class="flex items-center gap-2">
          <span :class="drawerUser.role === 'admin' ? 'badge badge-primary' : 'badge badge-gray'">
            {{ t('admin.users.roles.' + drawerUser.role) }}
          </span>
          <span
            :class="drawerUser.status === 'active' ? 'ui-dot is-ok' : 'ui-dot is-bad'"
            role="img"
            :aria-label="drawerUser.status === 'active' ? t('common.active') : t('admin.users.disabled')"
            :title="drawerUser.status === 'active' ? t('common.active') : t('admin.users.disabled')"
          ></span>
        </span>
      </template>

      <template v-if="drawerUser">
        <div class="users-drawer-stats" data-test="user-drawer">
          <button
            type="button"
            class="users-stat is-action"
            :title="t('admin.users.balanceHistoryTip')"
            @click="runFromDrawer(handleBalanceHistory)"
          >
            <span class="users-stat-label">{{ t('admin.users.columns.balance') }}</span>
            <span class="users-stat-value"><CountUp :value="drawerUser.balance" :format="formatUsd" /></span>
          </button>
          <div class="users-stat">
            <span class="users-stat-label">{{ t('admin.users.columns.concurrency') }}</span>
            <span class="users-stat-value">
              {{ drawerUser.current_concurrency ?? 0 }}<small>/{{ drawerUser.concurrency }}</small>
            </span>
          </div>
          <button
            type="button"
            class="users-stat is-action"
            data-test="user-drawer-api-keys"
            @click="runFromDrawer(handleViewApiKeys)"
          >
            <span class="users-stat-label">{{ t('admin.users.apiKeys') }}</span>
            <span class="users-stat-value">
              <span v-if="drawerApiKeyCount === undefined" class="skeleton inline-block h-5 w-8 align-middle"></span>
              <template v-else-if="drawerApiKeyCount === null">–</template>
              <CountUp v-else :value="drawerApiKeyCount" />
            </span>
          </button>
        </div>

        <section>
          <h3 class="ui-section-label">{{ t('admin.adminUi.details') }}</h3>
          <dl class="ui-dl">
            <dt>{{ t('admin.users.columns.id') }}</dt>
            <dd class="tabular-nums">{{ drawerUser.id }}</dd>
            <dt>{{ t('admin.users.columns.username') }}</dt>
            <dd>{{ drawerUser.username || '-' }}</dd>
            <template v-if="drawerUser.notes">
              <dt>{{ t('admin.users.columns.notes') }}</dt>
              <dd class="whitespace-pre-line">{{ drawerUser.notes }}</dd>
            </template>
            <template v-for="def in attributeDefinitions.filter(d => d.enabled)" :key="def.id">
              <template v-if="userAttributeValues[drawerUser.id]?.[def.id]">
                <dt>{{ def.name }}</dt>
                <dd>{{ getAttributeValue(drawerUser.id, def.id) }}</dd>
              </template>
            </template>
            <dt>{{ t('admin.users.columns.lastActive') }}</dt>
            <dd class="tabular-nums">{{ drawerUser.last_active_at ? formatDateTime(drawerUser.last_active_at) : '-' }}</dd>
            <dt>{{ t('admin.users.columns.lastUsed') }}</dt>
            <dd class="tabular-nums">{{ drawerUser.last_used_at ? formatDateTime(drawerUser.last_used_at) : '-' }}</dd>
            <dt>{{ t('admin.users.columns.created') }}</dt>
            <dd class="tabular-nums">{{ formatDateTime(drawerUser.created_at) }}</dd>
          </dl>
        </section>

        <section>
          <div class="users-section-head">
            <h3 class="ui-section-label">{{ t('admin.users.columns.groups') }}</h3>
            <button type="button" class="btn btn-ghost btn-sm" @click="runFromDrawer(handleAllowedGroups)">
              <Icon name="edit" size="sm" />
              {{ t('common.edit') }}
            </button>
          </div>
          <div class="flex flex-wrap items-center gap-1.5">
            <button
              v-for="g in getUserGroups(drawerUser).exclusive"
              :key="g.id"
              type="button"
              class="ui-chip"
              :title="t('admin.users.clickToReplace')"
              @click="replaceGroupFromDrawer(g)"
            >
              <Icon name="shield" size="xs" class="text-primary-500" />
              {{ g.name }}
              <Icon name="swap" size="xs" class="opacity-50" />
            </button>
            <span
              v-if="getUserGroups(drawerUser).publicGroups.length > 0"
              class="ui-chip"
              :title="getUserGroups(drawerUser).publicGroups.map(g => g.name).join(', ')"
            >
              <Icon name="globe" size="xs" />
              {{ getUserGroups(drawerUser).publicGroups.length }} {{ t('admin.users.publicLabel') }}
            </span>
            <span
              v-if="allGroups.length > 0 && getUserGroups(drawerUser).exclusive.length === 0 && getUserGroups(drawerUser).publicGroups.length === 0"
              class="text-sm text-gray-400"
            >-</span>
          </div>
          <div v-if="drawerUser.subscriptions?.length" class="mt-3 flex flex-wrap gap-1.5">
            <GroupBadge
              v-for="sub in drawerUser.subscriptions"
              :key="sub.id"
              :name="sub.group?.name || ''"
              :platform="sub.group?.platform"
              :subscription-type="sub.group?.subscription_type"
              :rate-multiplier="sub.group?.rate_multiplier"
              :days-remaining="sub.expires_at ? getDaysRemaining(sub.expires_at) : null"
              :title="sub.expires_at ? formatDateTime(sub.expires_at) : ''"
            />
          </div>
        </section>

        <section>
          <div class="users-section-head">
            <h3 class="ui-section-label">{{ t('admin.users.platformQuota.menuItem') }}</h3>
            <button type="button" class="btn btn-ghost btn-sm" @click="runFromDrawer(handlePlatformQuota)">
              <Icon name="edit" size="sm" />
              {{ t('common.edit') }}
            </button>
          </div>
          <div v-if="drawerQuotaItems === undefined" class="skeleton h-10 w-full"></div>
          <p v-else-if="drawerQuotaItems === null" class="text-sm text-gray-400">–</p>
          <p v-else-if="drawerQuotaRows.length === 0" class="text-sm text-gray-400">
            {{ t('admin.users.platformQuota.cellNotConfigured') }}
          </p>
          <ul v-else class="users-quota-list">
            <li v-for="quota in drawerQuotaRows" :key="quota.platform">
              <span class="users-quota-platform">
                <ProviderMark :platform="quota.platform" :size="20" />
                <span class="truncate">{{ quota.platform }}</span>
              </span>
              <div class="users-quota-windows">
                <div v-for="win in quota.windows" :key="win.key" :title="`${formatUsd(win.used)} / ${formatUsd(win.limit)}`">
                  <span class="users-quota-window">{{ win.label }}</span>
                  <QuotaMeter :value="win.remaining" :label="`${quota.platform} ${win.label}`" />
                </div>
              </div>
            </li>
          </ul>
        </section>

        <section>
          <h3 class="ui-section-label">{{ t('admin.users.columns.usage') }}</h3>
          <div v-if="drawerUsageStats === undefined" class="skeleton h-10 w-full"></div>
          <p v-else-if="drawerUsageStats === null" class="text-sm text-gray-400">–</p>
          <PlatformUsageBreakdown
            v-else
            :today="drawerUsageStats.today_actual_cost ?? 0"
            :total="drawerUsageStats.total_actual_cost ?? 0"
            :by-platform="drawerUsageStats.by_platform"
          />
        </section>
      </template>

      <template #footer>
        <template v-if="drawerUser">
          <button type="button" class="btn btn-primary btn-sm" data-test="user-drawer-edit" @click="runFromDrawer(handleEdit)">
            <Icon name="edit" size="sm" />
            {{ t('common.edit') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="runFromDrawer(handleDeposit)">
            <Icon name="plus" size="sm" />
            {{ t('admin.users.deposit') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="runFromDrawer(handleWithdraw)">
            <Icon name="minus" size="sm" />
            {{ t('admin.users.withdraw') }}
          </button>
          <span class="flex-1"></span>
          <template v-if="drawerUser.role !== 'admin'">
            <button type="button" class="btn btn-ghost btn-sm" @click="handleToggleStatus(drawerUser)">
              <Icon :name="drawerUser.status === 'active' ? 'ban' : 'checkCircle'" size="sm" />
              {{ drawerUser.status === 'active' ? t('admin.users.disable') : t('admin.users.enable') }}
            </button>
            <button
              type="button"
              class="app-icon-button users-danger-hover"
              :aria-label="t('common.delete')"
              :title="t('common.delete')"
              @click="runFromDrawer(handleDelete)"
            >
              <Icon name="trash" size="sm" />
            </button>
          </template>
        </template>
      </template>
    </SideDrawer>

    <ConfirmDialog :show="showDeleteDialog" :title="t('admin.users.deleteUser')" :message="t('admin.users.deleteConfirm', { email: deletingUser?.email })" :danger="true" @confirm="confirmDelete" @cancel="showDeleteDialog = false" />
    <ConfirmDialog
      :show="bulkDeleteIds.length > 0"
      :title="t('admin.users.bulkDelete.title')"
      :message="t('admin.users.bulkDelete.confirm', { count: bulkDeleteIds.length })"
      :confirm-text="t('common.delete')"
      danger
      @confirm="confirmBulkDelete"
      @cancel="bulkDeleteIds = []"
    />
    <UserCreateModal :show="showCreateModal" @close="showCreateModal = false" @success="loadUsers" />
    <UserEditModal :show="showEditModal" :user="editingUser" @close="closeEditModal" @success="loadUsers" />
    <BulkEditUserModal
      :show="showBulkEditModal"
      :selected-ids="selectedIds"
      @close="showBulkEditModal = false"
      @success="handleBulkLimitsSuccess"
    />
    <UserPlatformQuotaModal
      :show="showPlatformQuotaModal"
      :user="platformQuotaUser"
      @close="closePlatformQuotaModal"
      @success="loadUsers"
    />
    <UserApiKeysModal :show="showApiKeysModal" :user="viewingUser" @close="closeApiKeysModal" />
    <UserAllowedGroupsModal :show="showAllowedGroupsModal" :user="allowedGroupsUser" @close="closeAllowedGroupsModal" @success="loadUsers" />
    <UserBalanceModal :show="showBalanceModal" :user="balanceUser" :operation="balanceOperation" @close="closeBalanceModal" @success="loadUsers" />
    <UserBalanceHistoryModal :show="showBalanceHistoryModal" :user="balanceHistoryUser" @close="closeBalanceHistoryModal" @deposit="handleDepositFromHistory" @withdraw="handleWithdrawFromHistory" />
    <GroupReplaceModal :show="showGroupReplaceModal" :user="groupReplaceUser" :old-group="groupReplaceOldGroup" :all-groups="allGroups" @close="closeGroupReplaceModal" @success="loadUsers" />
    <UserAttributesConfigModal :show="showAttributesModal" @close="handleAttributesModalClose" />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useTableSelection } from '@/composables/useTableSelection'
import { formatDateTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'
import ToolbarPopover from '@/components/common/ToolbarPopover.vue'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import CountUp from '@/components/ui/CountUp.vue'
import QuotaMeter from '@/components/ui/QuotaMeter.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'

const { t } = useI18n()
import { adminAPI } from '@/api/admin'
import type { AdminUser, AdminGroup, UserAttributeDefinition } from '@/types'
import type { BatchUserUsageStats } from '@/api/admin/dashboard'
import type { PlatformQuotaItem } from '@/api/admin/users'
import type { Column } from '@/components/common/types'
import type { SelectOption } from '@/components/common/Select.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import Select from '@/components/common/Select.vue'
import { buildApiKeyGroupFilterOptions } from './apiKeyGroupFilterOptions'
import UserAttributesConfigModal from '@/components/user/UserAttributesConfigModal.vue'
import UserConcurrencyCell from '@/components/user/UserConcurrencyCell.vue'
import PlatformUsageBreakdown from '@/components/user/PlatformUsageBreakdown.vue'
import PlatformCostCell from '@/components/user/PlatformCostCell.vue'
import UserPlatformQuotaCell from '@/components/user/UserPlatformQuotaCell.vue'
import UserCreateModal from '@/components/admin/user/UserCreateModal.vue'
import UserEditModal from '@/components/admin/user/UserEditModal.vue'
import BulkEditUserModal from '@/components/admin/user/BulkEditUserModal.vue'
import UserPlatformQuotaModal from '@/components/admin/user/UserPlatformQuotaModal.vue'
import UserApiKeysModal from '@/components/admin/user/UserApiKeysModal.vue'
import UserAllowedGroupsModal from '@/components/admin/user/UserAllowedGroupsModal.vue'
import UserBalanceModal from '@/components/admin/user/UserBalanceModal.vue'
import UserBalanceHistoryModal from '@/components/admin/user/UserBalanceHistoryModal.vue'
import GroupReplaceModal from '@/components/admin/user/GroupReplaceModal.vue'

const appStore = useAppStore()

// Generate dynamic attribute columns from enabled definitions
const attributeColumns = computed<Column[]>(() =>
  attributeDefinitions.value
    .filter(def => def.enabled)
    .map(def => ({
      key: `attr_${def.id}`,
      label: def.name,
      sortable: false
    }))
)

// Get formatted attribute value for display in table
const getAttributeValue = (userId: number, attrId: number): string => {
  const userAttrs = userAttributeValues.value[userId]
  if (!userAttrs) return '-'
  const value = userAttrs[attrId]
  if (!value) return '-'

  // Find definition for this attribute
  const def = attributeDefinitions.value.find(d => d.id === attrId)
  if (!def) return value

  // Format based on type
  if (def.type === 'multi_select' && value) {
    try {
      const arr = JSON.parse(value)
      if (Array.isArray(arr)) {
        // Map values to labels
        return arr.map(v => {
          const opt = def.options?.find(o => o.value === v)
          return opt?.label || v
        }).join(', ')
      }
    } catch {
      return value
    }
  }

  if (def.type === 'select' && value && def.options) {
    const opt = def.options.find(o => o.value === value)
    return opt?.label || value
  }

  return value
}

// All possible columns (for column settings)
const allColumns = computed<Column[]>(() => [
  { key: 'email', label: t('admin.users.columns.user'), sortable: true },
  { key: 'id', label: t('admin.users.columns.id'), sortable: true },
  { key: 'username', label: t('admin.users.columns.username'), sortable: true },
  { key: 'notes', label: t('admin.users.columns.notes'), sortable: false },
  // Dynamic attribute columns
  ...attributeColumns.value,
  { key: 'role', label: t('admin.users.columns.role'), sortable: true },
  { key: 'groups', label: t('admin.users.columns.groups'), sortable: false },
  { key: 'subscriptions', label: t('admin.users.columns.subscriptions'), sortable: false },
  { key: 'balance', label: t('admin.users.columns.balance'), sortable: true, class: 'text-right' },
  { key: 'balance_platform_quota', label: t('admin.users.columns.balancePlatformQuota'), sortable: false },
  { key: 'usage', label: t('admin.users.columns.usage'), sortable: false },
  { key: 'usage_anthropic', label: t('admin.users.columns.usageAnthropic'), sortable: false },
  { key: 'usage_openai', label: t('admin.users.columns.usageOpenAI'), sortable: false },
  { key: 'usage_gemini', label: t('admin.users.columns.usageGemini'), sortable: false },
  { key: 'usage_antigravity', label: t('admin.users.columns.usageAntigravity'), sortable: false },
  { key: 'concurrency', label: t('admin.users.columns.concurrency'), sortable: true, class: 'text-right' },
  { key: 'status', label: t('admin.users.columns.status'), sortable: true },
  { key: 'last_active_at', label: t('admin.users.columns.lastActive'), sortable: true },
  { key: 'last_used_at', label: t('admin.users.columns.lastUsed'), sortable: true },
  { key: 'created_at', label: t('admin.users.columns.created'), sortable: true },
  { key: 'actions', label: t('admin.users.columns.actions'), sortable: false }
])

// Columns that can be toggled (exclude email and actions which are always visible)
const toggleableColumns = computed(() =>
  allColumns.value.filter(col => col.key !== 'email' && col.key !== 'actions')
)

// Hidden columns (stored in Set - columns NOT in this set are visible)
// This way, new columns are visible by default
const hiddenColumns = reactive<Set<string>>(new Set())

// Default hidden columns (columns hidden by default on first load)
const DEFAULT_HIDDEN_COLUMNS = [
  'notes', 'groups', 'subscriptions', 'usage', 'concurrency',
  'usage_anthropic', 'usage_openai', 'usage_gemini', 'usage_antigravity',
  'balance_platform_quota'
]
const REMOVED_COLUMNS = new Set(['last_login_at'])
// 强制可见列：加载时会被强制移出 hiddenColumns，并在列设置 UI 上 disabled。
// 当前没有列需要强制可见 —— last_active_at 已改为可被用户隐藏。
const FORCED_VISIBLE_COLUMNS = new Set<string>()

// localStorage keys for column settings
const HIDDEN_COLUMNS_KEY = 'user-hidden-columns'
// 列设置 schema 版本号。每次给 DEFAULT_HIDDEN_COLUMNS 新增列时 bump 一次，
// 并在 VERSION_NEW_HIDDEN_COLUMNS 中登记该版本新增的 key。
// 这样老用户升级后这些新列会被自动隐藏一次，而不会影响他们对其它老列的偏好。
const COLUMN_SETTINGS_VERSION_KEY = 'user-column-settings-version'
const COLUMN_SETTINGS_VERSION = 3
const VERSION_NEW_HIDDEN_COLUMNS: Record<number, string[]> = {
  2: ['usage_anthropic', 'usage_openai', 'usage_gemini', 'usage_antigravity'],
  3: ['balance_platform_quota']
}

// Load saved column settings
const loadSavedColumns = () => {
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as string[]
      parsed
        .filter(key => !REMOVED_COLUMNS.has(key) && !FORCED_VISIBLE_COLUMNS.has(key))
        .forEach(key => hiddenColumns.add(key))

      // 老用户升级：把每个未应用过的版本里新增的默认隐藏列自动追加到 hiddenColumns。
      const storedVersion = Number(localStorage.getItem(COLUMN_SETTINGS_VERSION_KEY) ?? '1')
      if (storedVersion < COLUMN_SETTINGS_VERSION) {
        let mutated = false
        for (let v = storedVersion + 1; v <= COLUMN_SETTINGS_VERSION; v++) {
          for (const key of VERSION_NEW_HIDDEN_COLUMNS[v] ?? []) {
            if (REMOVED_COLUMNS.has(key) || FORCED_VISIBLE_COLUMNS.has(key)) continue
            if (!hiddenColumns.has(key)) {
              hiddenColumns.add(key)
              mutated = true
            }
          }
        }
        if (mutated) saveColumnsToStorage()
        else localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
      }
    } else {
      // Use default hidden columns on first load
      DEFAULT_HIDDEN_COLUMNS.forEach(key => hiddenColumns.add(key))
      localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
    }
  } catch (e) {
    console.error('Failed to load saved columns:', e)
    DEFAULT_HIDDEN_COLUMNS.forEach(key => hiddenColumns.add(key))
  }
}

// Save column settings to localStorage
const saveColumnsToStorage = () => {
  try {
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
    localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
  } catch (e) {
    console.error('Failed to save columns:', e)
  }
}

// Toggle column visibility
const isForcedVisibleColumn = (key: string) => FORCED_VISIBLE_COLUMNS.has(key)
const toggleColumn = (key: string) => {
  // 强制可见列(如 last_active_at)在加载时会被恢复成可见，
  // 这里阻止用户在当前会话隐藏它，避免"取消勾选 → 刷新又恢复"的反直觉行为。
  if (FORCED_VISIBLE_COLUMNS.has(key)) return
  const wasHidden = hiddenColumns.has(key)
  if (hiddenColumns.has(key)) {
    hiddenColumns.delete(key)
  } else {
    hiddenColumns.add(key)
  }
  saveColumnsToStorage()
  if (wasHidden && (key === 'usage' || key.startsWith('usage_') || key.startsWith('attr_') || key === 'balance_platform_quota')) {
    refreshCurrentPageSecondaryData()
  }
  if (key === 'subscriptions') {
    loadUsers()
  }
  if (wasHidden && key === 'groups') {
    loadAllGroups()
  }
}

// Check if column is visible (not in hidden set)
const isColumnVisible = (key: string) => !hiddenColumns.has(key)
// usage 主列或任意 usage_<platform> 子列可见时都需要批量拉取用量数据
// 列 key → 平台名（'usage' 主列汇总所有平台时为 null）
// 显式数组取代 Object.keys()：保证迭代顺序（决定列头排序按钮渲染顺序）
// 不会因 JS 引擎差异或 USAGE_COLUMN_PLATFORMS 属性顺序调整而静默变化。
const USAGE_COLUMN_KEYS: readonly string[] = ['usage', 'usage_anthropic', 'usage_openai', 'usage_gemini', 'usage_antigravity']
const USAGE_COLUMN_PLATFORMS: Record<string, string | null> = {
  usage: null,
  usage_anthropic: 'anthropic',
  usage_openai: 'openai',
  usage_gemini: 'gemini',
  usage_antigravity: 'antigravity'
}
const PLATFORM_USAGE_COLUMNS = USAGE_COLUMN_KEYS.filter((k) => k !== 'usage')
const hasVisibleUsageColumn = computed(
  () => !hiddenColumns.has('usage') || PLATFORM_USAGE_COLUMNS.some((k) => !hiddenColumns.has(k))
)
const hasVisibleGroupsColumn = computed(() => !hiddenColumns.has('groups'))
const hasVisiblePlatformQuotaColumn = computed(() => !hiddenColumns.has('balance_platform_quota'))
const hasVisibleAttributeColumns = computed(() =>
  attributeDefinitions.value.some((def) => def.enabled && !hiddenColumns.has(`attr_${def.id}`))
)

// Filtered columns based on visibility
const columns = computed<Column[]>(() =>
  allColumns.value.filter(col =>
    col.key === 'email' || col.key === 'actions' || !hiddenColumns.has(col.key)
  )
)

const users = ref<AdminUser[]>([])
const loading = ref(false)
const searchQuery = ref('')
const USER_SORT_STORAGE_KEY = 'admin-users-table-sort'
const loadInitialSortState = (): { sort_by: string; sort_order: 'asc' | 'desc' } => {
  const fallback = { sort_by: 'created_at', sort_order: 'desc' as 'asc' | 'desc' }
  const sortable = new Set(['email', 'id', 'username', 'role', 'balance', 'concurrency', 'status', 'last_used_at', 'last_active_at', 'created_at'])
  try {
    const raw = localStorage.getItem(USER_SORT_STORAGE_KEY)
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as { key?: string; order?: string }
    const key = typeof parsed.key === 'string' ? parsed.key : ''
    if (!sortable.has(key)) return fallback
    return {
      sort_by: key,
      sort_order: parsed.order === 'asc' ? 'asc' : 'desc'
    }
  } catch {
    return fallback
  }
}
const sortState = reactive(loadInitialSortState())

// Groups data for the groups column and the existing "authorised group" filter (active only)
const allGroups = ref<AdminGroup[]>([])
const loadAllGroups = async () => {
  if (allGroups.value.length > 0) return
  try {
    allGroups.value = await adminAPI.groups.getAll()
  } catch (e) {
    console.error('Failed to load groups:', e)
  }
}

// Groups for the API Key group filter — includes disabled groups so admins can
// filter users whose keys are still bound to a now-disabled group.
const allGroupsForApiKeyFilter = ref<AdminGroup[]>([])
const loadAllGroupsForApiKeyFilter = async () => {
  if (allGroupsForApiKeyFilter.value.length > 0) return
  try {
    allGroupsForApiKeyFilter.value = await adminAPI.groups.getAllIncludingInactive()
  } catch (e) {
    console.error('Failed to load groups for API key filter:', e)
  }
}
// Resolve user's accessible groups: exclusive groups first, then public groups
const getUserGroups = (user: AdminUser) => {
  const exclusive: AdminGroup[] = []
  const publicGroups: AdminGroup[] = []
  for (const g of allGroups.value) {
    if (g.status !== 'active' || g.subscription_type !== 'standard') continue
    if (g.is_exclusive) {
      if (user.allowed_groups?.includes(g.id)) {
        exclusive.push(g)
      }
    } else {
      publicGroups.push(g)
    }
  }
  return { exclusive, publicGroups }
}

// Group filter options: "All Groups" + active exclusive groups (value = group name for fuzzy match)
const groupFilterOptions = computed(() => {
  const options: { value: string; label: string }[] = [
    { value: '', label: t('admin.users.allAuthorizedGroups') }
  ]
  for (const g of allGroups.value) {
    if (g.status !== 'active' || !g.is_exclusive || g.subscription_type !== 'standard') continue
    options.push({ value: g.name, label: g.name })
  }
  return options
})

// API Key group filter options: "All" + groups partitioned by type (value = group id).
// Uses allGroupsForApiKeyFilter which includes disabled groups.
const apiKeyGroupFilterOptions = computed(() =>
  buildApiKeyGroupFilterOptions(allGroupsForApiKeyFilter.value, {
    all: t('admin.users.allApiKeyGroups'),
    exclusive: t('admin.users.apiKeyGroupExclusive'),
    public: t('admin.users.apiKeyGroupPublic'),
    subscription: t('admin.users.apiKeyGroupSubscription'),
    disabled: t('admin.users.apiKeyGroupDisabled'),
  }) as SelectOption[]
)

// Filter values (role, status, and custom attributes)
const filters = reactive({
  role: '',
  status: '',
  group: '',  // group name for fuzzy match, '' = all
  apiKeyGroup: null as number | null  // group id bound to the user's API keys, null = all
})
const activeAttributeFilters = reactive<Record<number, string>>({})

// localStorage key for filter values
const FILTER_VALUES_KEY = 'user-filter-values'

// All filterable attribute definitions (enabled attributes)
const filterableAttributes = computed(() =>
  attributeDefinitions.value.filter(def => def.enabled)
)

const statusOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'active', label: t('common.active') },
  { value: 'disabled', label: t('admin.users.disabled') }
])

const roleOptions = computed(() => [
  { value: '', label: t('admin.users.allRoles') },
  { value: 'admin', label: t('admin.users.admin') },
  { value: 'user', label: t('admin.users.user') }
])

// Filters popover badge: every non-default filter except search and the status segment.
const activeFilterCount = computed(() =>
  (filters.role ? 1 : 0) +
  (filters.group ? 1 : 0) +
  (filters.apiKeyGroup !== null ? 1 : 0) +
  Object.values(activeAttributeFilters).filter(Boolean).length
)

// Load saved filter values from localStorage
const loadSavedFilters = () => {
  try {
    const savedValues = localStorage.getItem(FILTER_VALUES_KEY)
    if (savedValues) {
      const parsed = JSON.parse(savedValues)
      if (parsed.role) filters.role = parsed.role
      if (parsed.status) filters.status = parsed.status
      if (parsed.group) filters.group = parsed.group
      if (typeof parsed.apiKeyGroup === 'number') filters.apiKeyGroup = parsed.apiKeyGroup
      if (parsed.attributes) {
        Object.assign(activeAttributeFilters, parsed.attributes)
      }
    }
  } catch (e) {
    console.error('Failed to load saved filters:', e)
  }
}

// Save filter values to localStorage
const saveFiltersToStorage = () => {
  try {
    const values = {
      role: filters.role,
      status: filters.status,
      group: filters.group,
      apiKeyGroup: filters.apiKeyGroup,
      attributes: activeAttributeFilters
    }
    localStorage.setItem(FILTER_VALUES_KEY, JSON.stringify(values))
  } catch (e) {
    console.error('Failed to save filters:', e)
  }
}

const usageStats = ref<Record<string, BatchUserUsageStats>>({})
const platformQuotaStats = ref<Record<number, PlatformQuotaItem[]>>({})

const getPlatformUsage = (userId: number, platform: string) =>
  usageStats.value[userId]?.by_platform?.find((p) => p.platform === platform)

// 用量列前端排序：DataTable 工作在 server-side-sort 模式，所有 sortable
// 字段都会触发后端查询，而用量列数据是异步批量拉取后再合并到当前页，
// 因此采用独立的前端排序状态对当前页 users 做本地排序。
// 排序状态独立于后端 sortState 持久化；缺失数据按 0 处理（desc 沉底、asc 置顶）。
type UsageMetric = 'today' | 'total'
type UsageSortState = { key: string; metric: UsageMetric; order: 'asc' | 'desc' } | null
const USAGE_SORT_STORAGE_KEY = 'admin-users-usage-sort'
// 列头排序按钮点击后弹出的"今日/近30天"选择菜单，同时只允许一个列展开。
const openUsageSortMenu = ref<string | null>(null)

const loadInitialUsageSort = (): UsageSortState => {
  try {
    const raw = localStorage.getItem(USAGE_SORT_STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<{ key: string; metric: string; order: string }>
    if (!parsed.key || !USAGE_COLUMN_KEYS.includes(parsed.key)) return null
    const metric: UsageMetric = parsed.metric === 'total' ? 'total' : 'today'
    const order: 'asc' | 'desc' = parsed.order === 'asc' ? 'asc' : 'desc'
    return { key: parsed.key, metric, order }
  } catch {
    return null
  }
}
const usageSort = ref<UsageSortState>(loadInitialUsageSort())
const persistUsageSort = () => {
  try {
    if (usageSort.value) {
      localStorage.setItem(USAGE_SORT_STORAGE_KEY, JSON.stringify(usageSort.value))
    } else {
      localStorage.removeItem(USAGE_SORT_STORAGE_KEY)
    }
  } catch (e) {
    console.error('Failed to persist usage sort:', e)
  }
}
const clearUsageSort = () => {
  if (!usageSort.value) return
  usageSort.value = null
  openUsageSortMenu.value = null
  persistUsageSort()
}

const isUsageSortActive = (key: string, metric: UsageMetric) =>
  !!usageSort.value && usageSort.value.key === key && usageSort.value.metric === metric
const getUsageSortOrder = (key: string, metric: UsageMetric): 'asc' | 'desc' | null =>
  isUsageSortActive(key, metric) ? usageSort.value!.order : null

// 三态循环：desc → asc → off。选完即关闭菜单（用户大多希望"选中即应用"，
// 想再切换 order 时重新打开菜单点同一项即可）。
const toggleUsageSort = (key: string, metric: UsageMetric) => {
  const cur = usageSort.value
  if (cur && cur.key === key && cur.metric === metric) {
    usageSort.value = cur.order === 'desc' ? { key, metric, order: 'asc' } : null
  } else {
    usageSort.value = { key, metric, order: 'desc' }
  }
  persistUsageSort()
  openUsageSortMenu.value = null
}

// 点击图标本身不触发排序，仅开关菜单；首次排序由用户在菜单内选择 metric 触发（默认 desc，详见 toggleUsageSort）。
const toggleUsageSortMenu = (key: string) => {
  openUsageSortMenu.value = openUsageSortMenu.value === key ? null : key
}

const getUsageValue = (userId: number, key: string, metric: UsageMetric): number => {
  const stats = usageStats.value[userId]
  if (!stats) return 0
  const platform = USAGE_COLUMN_PLATFORMS[key]
  if (platform === null) {
    return metric === 'today' ? stats.today_actual_cost ?? 0 : stats.total_actual_cost ?? 0
  }
  const p = stats.by_platform?.find((x) => x.platform === platform)
  if (!p) return 0
  return metric === 'today' ? p.today_actual_cost ?? 0 : p.total_actual_cost ?? 0
}

// 在 server-side 排序结果之上叠加用量列的本地排序；无 usageSort 时直接透传原数组。
// 稳定排序：等值按原 index 保序，避免拉取新用量数据时表行抖动。
const sortedUsers = computed(() => {
  const s = usageSort.value
  if (!s) return users.value
  return [...users.value]
    .map((row, index) => ({ row, index }))
    .sort((a, b) => {
      const av = getUsageValue(a.row.id, s.key, s.metric)
      const bv = getUsageValue(b.row.id, s.key, s.metric)
      if (av !== bv) return s.order === 'asc' ? av - bv : bv - av
      return a.index - b.index
    })
    .map((x) => x.row)
})

const {
  selectedIds,
  selectedCount,
  setSelectedIds,
  clear: clearSelection,
  removeMany: removeSelectedIds
} = useTableSelection<AdminUser>({
  rows: sortedUsers,
  getId: (user) => user.id
})

const handleSelectedKeysUpdate = (keys: Array<string | number>) => {
  setSelectedIds(keys.filter((key): key is number => typeof key === 'number'))
}

const getUserSelectionLabel = (user: AdminUser) =>
  t('admin.users.bulkLimits.selectUser', { email: user.email })

// User attribute definitions and values
const attributeDefinitions = ref<UserAttributeDefinition[]>([])
const userAttributeValues = ref<Record<number, Record<number, string>>>({})
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const showCreateModal = ref(false)
const showEditModal = ref(false)
const showBulkEditModal = ref(false)
const showDeleteDialog = ref(false)
const bulkDeleteIds = ref<number[]>([])
const bulkDeleting = ref(false)
const showApiKeysModal = ref(false)
const showAttributesModal = ref(false)
const showPlatformQuotaModal = ref(false)
const editingUser = ref<AdminUser | null>(null)
const deletingUser = ref<AdminUser | null>(null)
const viewingUser = ref<AdminUser | null>(null)
const platformQuotaUser = ref<AdminUser | null>(null)

const handlePlatformQuota = (user: AdminUser) => {
  platformQuotaUser.value = user
  showPlatformQuotaModal.value = true
}

const closePlatformQuotaModal = () => {
  showPlatformQuotaModal.value = false
  platformQuotaUser.value = null
}
let abortController: AbortController | null = null
let secondaryDataSeq = 0

const loadUsersSecondaryData = async (
  userIds: number[],
  signal?: AbortSignal,
  expectedSeq?: number
) => {
  if (userIds.length === 0) return

  const tasks: Promise<void>[] = []

  if (hasVisibleUsageColumn.value) {
    tasks.push(
      (async () => {
        try {
          const usageResponse = await adminAPI.dashboard.getBatchUsersUsage(userIds)
          if (signal?.aborted) return
          if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
          usageStats.value = usageResponse.stats
        } catch (e) {
          if (signal?.aborted) return
          console.error('Failed to load usage stats:', e)
        }
      })()
    )
  }

  if (attributeDefinitions.value.length > 0 && hasVisibleAttributeColumns.value) {
    tasks.push(
      (async () => {
        try {
          const attrResponse = await adminAPI.userAttributes.getBatchUserAttributes(userIds)
          if (signal?.aborted) return
          if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
          userAttributeValues.value = attrResponse.attributes
        } catch (e) {
          if (signal?.aborted) return
          console.error('Failed to load user attribute values:', e)
        }
      })()
    )
  }

  if (hasVisiblePlatformQuotaColumn.value) {
    tasks.push(
      (async () => {
        try {
          // 无批量端点：对当前页用户逐个拉取，分块并发（每批 6），批间检查中止条件，避免大 pageSize 时请求洪峰
          const CHUNK = 6
          for (let i = 0; i < userIds.length; i += CHUNK) {
            if (signal?.aborted) return
            if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
            const chunk = userIds.slice(i, i + CHUNK)
            const results = await Promise.allSettled(
              chunk.map((id) => adminAPI.users.getPlatformQuotas(id))
            )
            if (signal?.aborted) return
            if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
            const merged = { ...platformQuotaStats.value }
            results.forEach((r, idx) => {
              if (r.status === 'fulfilled') {
                merged[chunk[idx]] = r.value.platform_quotas || []
              }
            })
            platformQuotaStats.value = merged
          }
        } catch (e) {
          if (signal?.aborted) return
          console.error('Failed to load platform quotas:', e)
        }
      })()
    )
  }

  if (tasks.length > 0) {
    await Promise.allSettled(tasks)
  }
}

const refreshCurrentPageSecondaryData = () => {
  const userIds = users.value.map((u) => u.id)
  if (userIds.length === 0) return
  const seq = ++secondaryDataSeq
  void loadUsersSecondaryData(userIds, undefined, seq)
}

// Action Menu State
const activeMenuId = ref<number | null>(null)
const menuPosition = ref<{ top: number; left: number } | null>(null)
const activeMenuUser = computed(() =>
  activeMenuId.value === null ? null : users.value.find((user) => user.id === activeMenuId.value) ?? null
)

const openActionMenu = (user: AdminUser, e: MouseEvent) => {
  if (activeMenuId.value === user.id) {
    closeActionMenu()
  } else {
    const target = e.currentTarget as HTMLElement
    if (!target) {
      closeActionMenu()
      return
    }

    const rect = target.getBoundingClientRect()
    const menuWidth = 208
    const menuHeight = 300
    const padding = 8
    const viewportWidth = window.innerWidth
    const viewportHeight = window.innerHeight

    let left, top

    if (viewportWidth < 768) {
      // 居中显示,水平位置
      left = Math.max(padding, Math.min(
        rect.left + rect.width / 2 - menuWidth / 2,
        viewportWidth - menuWidth - padding
      ))

      // 优先显示在按钮下方
      top = rect.bottom + 4

      // 如果下方空间不够,显示在上方
      if (top + menuHeight > viewportHeight - padding) {
        top = rect.top - menuHeight - 4
        // 如果上方也不够,就贴在视口顶部
        if (top < padding) {
          top = padding
        }
      }
    } else {
      left = Math.max(padding, Math.min(
        e.clientX - menuWidth,
        viewportWidth - menuWidth - padding
      ))
      top = e.clientY
      if (top + menuHeight > viewportHeight - padding) {
        top = viewportHeight - menuHeight - padding
      }
    }

    menuPosition.value = { top, left }
    activeMenuId.value = user.id
  }
}

const closeActionMenu = () => {
  activeMenuId.value = null
  menuPosition.value = null
}

// Close menu when clicking outside
const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (!target.closest('.action-menu-trigger') && !target.closest('.action-menu-content')) {
    closeActionMenu()
  }
  // Close usage sort dropdown when clicking outside any usage-sort-trigger
  if (openUsageSortMenu.value !== null && !target.closest('.usage-sort-trigger')) {
    openUsageSortMenu.value = null
  }
  // Close expanded group dropdown when clicking outside
  if (expandedGroupUserId.value !== null) {
    expandedGroupUserId.value = null
  }
}

// Allowed groups modal state
const showAllowedGroupsModal = ref(false)
const allowedGroupsUser = ref<AdminUser | null>(null)

// Expanded group dropdown state (click to show exclusive groups list)
const expandedGroupUserId = ref<number | null>(null)
const toggleExpandedGroup = (userId: number) => {
  expandedGroupUserId.value = expandedGroupUserId.value === userId ? null : userId
}

// Group replace modal state
const showGroupReplaceModal = ref(false)
const groupReplaceUser = ref<AdminUser | null>(null)
const groupReplaceOldGroup = ref<{ id: number; name: string } | null>(null)

// Balance (Deposit/Withdraw) modal state
const showBalanceModal = ref(false)
const balanceUser = ref<AdminUser | null>(null)
const balanceOperation = ref<'add' | 'subtract'>('add')

// Balance History modal state
const showBalanceHistoryModal = ref(false)
const balanceHistoryUser = ref<AdminUser | null>(null)

// 计算剩余天数
const getDaysRemaining = (expiresAt: string): number => {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diffMs = expires.getTime() - now.getTime()
  return Math.ceil(diffMs / (1000 * 60 * 60 * 24))
}

const loadAttributeDefinitions = async () => {
  try {
    attributeDefinitions.value = await adminAPI.userAttributes.listEnabledDefinitions()
  } catch (e) {
    console.error('Failed to load attribute definitions:', e)
  }
}

// Handle attributes modal close - reload definitions and users
const handleAttributesModalClose = async () => {
  showAttributesModal.value = false
  await loadAttributeDefinitions()
  loadUsers()
}

const loadUsers = async () => {
  abortController?.abort()
  const currentAbortController = new AbortController()
  abortController = currentAbortController
  const { signal } = currentAbortController
  loading.value = true
  try {
    // Build attribute filters from active filters
    const attrFilters: Record<number, string> = {}
    for (const [attrId, value] of Object.entries(activeAttributeFilters)) {
      if (value) {
        attrFilters[Number(attrId)] = value
      }
    }

    const response = await adminAPI.users.list(
      pagination.page,
      pagination.page_size,
      {
        role: filters.role as any,
        status: filters.status as any,
        search: searchQuery.value || undefined,
        group_name: filters.group || undefined,
        api_key_group_id: filters.apiKeyGroup ?? undefined,
        attributes: Object.keys(attrFilters).length > 0 ? attrFilters : undefined,
        // 始终请求 subscriptions：列隐藏时仍需用于 UserPlatformQuotaModal 的 active-subscription 警示 banner
        include_subscriptions: true,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      },
      { signal }
    )
    if (signal.aborted) {
      return
    }
    users.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
    usageStats.value = {}
    userAttributeValues.value = {}
    platformQuotaStats.value = {}

    // Defer heavy secondary data so table can render first.
    if (response.items.length > 0) {
      const userIds = response.items.map((u) => u.id)
      const seq = ++secondaryDataSeq
      window.setTimeout(() => {
        if (signal.aborted || seq !== secondaryDataSeq) return
        void loadUsersSecondaryData(userIds, signal, seq)
      }, 50)
    }
  } catch (error: any) {
    const errorInfo = error as { name?: string; code?: string }
    if (errorInfo?.name === 'AbortError' || errorInfo?.name === 'CanceledError' || errorInfo?.code === 'ERR_CANCELED') {
      return
    }
    const message = error.response?.data?.detail || error.message || t('admin.users.failedToLoad')
    appStore.showError(message)
    console.error('Error loading users:', error)
  } finally {
    if (abortController === currentAbortController) {
      loading.value = false
    }
  }
}

const handleBulkLimitsSuccess = async () => {
  clearSelection()
  await loadUsers()
}

let searchTimeout: ReturnType<typeof setTimeout>
const handleSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadUsers()
  }, 300)
}

const handlePageChange = (page: number) => {
  // 确保页码在有效范围内
  const validPage = Math.max(1, Math.min(page, pagination.pages || 1))
  pagination.page = validPage
  loadUsers()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadUsers()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  clearUsageSort()
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadUsers()
}

// Filter helpers
const setStatusFilter = (status: string) => {
  if (filters.status === status) return
  filters.status = status
  applyFilter()
}

const loadFilterGroups = () => {
  void loadAllGroups()
  void loadAllGroupsForApiKeyFilter()
}

const clearFilters = () => {
  filters.role = ''
  filters.group = ''
  filters.apiKeyGroup = null
  for (const key of Object.keys(activeAttributeFilters)) {
    Reflect.deleteProperty(activeAttributeFilters, key)
  }
  applyFilter()
}

const updateAttributeFilter = (attrId: number, value: string) => {
  activeAttributeFilters[attrId] = value
}

// Apply filter and save to localStorage
const applyFilter = () => {
  saveFiltersToStorage()
  pagination.page = 1
  loadUsers()
}

const handleEdit = (user: AdminUser) => {
  editingUser.value = user
  showEditModal.value = true
}

const closeEditModal = () => {
  showEditModal.value = false
  editingUser.value = null
}

const handleToggleStatus = async (user: AdminUser) => {
  const newStatus = user.status === 'active' ? 'disabled' : 'active'
  try {
    const updated = await adminAPI.users.toggleStatus(user.id, newStatus)
    appStore.showSuccess(
      newStatus === 'active' ? t('admin.users.userEnabled') : t('admin.users.userDisabled')
    )
    if (loading.value) {
      loadUsers()
      return
    }
    const row = users.value.find(current => current.id === user.id)
    if (row) {
      row.status = updated.status
      row.updated_at = updated.updated_at
    }
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.failedToToggle'))
    console.error('Error toggling user status:', error)
  }
}

const handleViewApiKeys = (user: AdminUser) => {
  viewingUser.value = user
  showApiKeysModal.value = true
}

const closeApiKeysModal = () => {
  showApiKeysModal.value = false
  viewingUser.value = null
}

const handleAllowedGroups = (user: AdminUser) => {
  allowedGroupsUser.value = user
  showAllowedGroupsModal.value = true
}

const closeAllowedGroupsModal = () => {
  showAllowedGroupsModal.value = false
  allowedGroupsUser.value = null
}

const openGroupReplace = (user: AdminUser, group: { id: number; name: string }) => {
  expandedGroupUserId.value = null
  groupReplaceUser.value = user
  groupReplaceOldGroup.value = group
  showGroupReplaceModal.value = true
}

const closeGroupReplaceModal = () => {
  showGroupReplaceModal.value = false
  groupReplaceUser.value = null
  groupReplaceOldGroup.value = null
}

const handleDelete = (user: AdminUser) => {
  deletingUser.value = user
  showDeleteDialog.value = true
}

const confirmDelete = async () => {
  if (!deletingUser.value) return
  try {
    await adminAPI.users.delete(deletingUser.value.id)
    appStore.showSuccess(t('common.success'))
    showDeleteDialog.value = false
    deletingUser.value = null
    loadUsers()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.users.failedToDelete'))
    console.error('Error deleting user:', error)
  }
}

const confirmBulkDelete = async () => {
  if (bulkDeleting.value || bulkDeleteIds.value.length === 0) return
  const ids = [...bulkDeleteIds.value]
  bulkDeleteIds.value = []
  bulkDeleting.value = true
  const deletedIds: number[] = []
  for (const id of ids) {
    try {
      await adminAPI.users.delete(id)
      deletedIds.push(id)
    } catch (error) {
      console.error('Error deleting user:', error)
    }
  }
  removeSelectedIds(deletedIds)
  if (deletedIds.length > 0) {
    appStore.showSuccess(t('admin.users.bulkDelete.success', { count: deletedIds.length }))
    pagination.page = 1
  }
  const failed = ids.length - deletedIds.length
  if (failed > 0) appStore.showError(t('admin.users.bulkDelete.failed', { count: failed }))
  await loadUsers()
  bulkDeleting.value = false
}

const handleDeposit = (user: AdminUser) => {
  balanceUser.value = user
  balanceOperation.value = 'add'
  showBalanceModal.value = true
}

const handleWithdraw = (user: AdminUser) => {
  balanceUser.value = user
  balanceOperation.value = 'subtract'
  showBalanceModal.value = true
}

const closeBalanceModal = () => {
  showBalanceModal.value = false
  balanceUser.value = null
}

const handleBalanceHistory = (user: AdminUser) => {
  balanceHistoryUser.value = user
  showBalanceHistoryModal.value = true
}

const closeBalanceHistoryModal = () => {
  showBalanceHistoryModal.value = false
  balanceHistoryUser.value = null
}

// Handle deposit from balance history modal
const handleDepositFromHistory = () => {
  if (balanceHistoryUser.value) {
    handleDeposit(balanceHistoryUser.value)
  }
}

// Handle withdraw from balance history modal
const handleWithdrawFromHistory = () => {
  if (balanceHistoryUser.value) {
    handleWithdraw(balanceHistoryUser.value)
  }
}

const userInitial = (email: string) => (email || '?').charAt(0).toUpperCase()
const formatUsd = (value: number) => `$${value.toFixed(2)}`

// User detail drawer. The list row stays the source of truth so in-place
// updates (status toggles) show immediately; extra detail loads per open.
const drawerUserId = ref<number | null>(null)
const drawerUser = computed(() =>
  drawerUserId.value === null ? null : users.value.find((user) => user.id === drawerUserId.value) ?? null
)
// undefined = loading, null = failed
const drawerApiKeyCount = ref<number | null | undefined>(undefined)
const drawerQuotas = ref<PlatformQuotaItem[] | null | undefined>(undefined)
const drawerUsage = ref<BatchUserUsageStats | null | undefined>(undefined)
let drawerSeq = 0

const drawerQuotaItems = computed(() =>
  drawerUserId.value === null ? undefined : platformQuotaStats.value[drawerUserId.value] ?? drawerQuotas.value
)
const drawerUsageStats = computed(() =>
  drawerUserId.value === null ? undefined : usageStats.value[drawerUserId.value] ?? drawerUsage.value
)

const QUOTA_WINDOWS = ['daily', 'weekly', 'monthly'] as const
const drawerQuotaRows = computed(() => {
  const labels = {
    daily: t('admin.users.platformQuota.windowDaily'),
    weekly: t('admin.users.platformQuota.windowWeekly'),
    monthly: t('admin.users.platformQuota.windowMonthly')
  }
  return (drawerQuotaItems.value ?? [])
    .map((item) => ({
      platform: item.platform,
      windows: QUOTA_WINDOWS.flatMap((key) => {
        const limit = item[`${key}_limit_usd`]
        if (limit === null || limit === undefined) return []
        const used = item[`${key}_usage_usd`] ?? 0
        // A zero limit blocks the window entirely.
        const remaining = limit > 0 ? Math.max(0, ((limit - used) / limit) * 100) : 0
        return [{ key, label: labels[key], used, limit, remaining }]
      })
    }))
    .filter((row) => row.windows.length > 0)
})

const openUserDrawer = (user: AdminUser) => {
  closeActionMenu()
  const id = user.id
  const seq = ++drawerSeq
  const isCurrent = () => seq === drawerSeq && drawerUserId.value === id
  drawerUserId.value = id
  drawerApiKeyCount.value = undefined
  drawerQuotas.value = undefined
  drawerUsage.value = undefined
  void loadAllGroups()

  adminAPI.users.getUserApiKeys(id)
    .then((response) => { if (isCurrent()) drawerApiKeyCount.value = response.total })
    .catch((e) => {
      if (isCurrent()) drawerApiKeyCount.value = null
      console.error('Failed to load user API keys:', e)
    })
  if (!platformQuotaStats.value[id]) {
    adminAPI.users.getPlatformQuotas(id)
      .then((response) => { if (isCurrent()) drawerQuotas.value = response.platform_quotas || [] })
      .catch((e) => {
        if (isCurrent()) drawerQuotas.value = null
        console.error('Failed to load platform quotas:', e)
      })
  }
  if (!usageStats.value[id]) {
    adminAPI.dashboard.getBatchUsersUsage([id])
      .then((response) => {
        if (!isCurrent()) return
        drawerUsage.value = response.stats[id] ?? { user_id: id, today_actual_cost: 0, total_actual_cost: 0 }
      })
      .catch((e) => {
        if (isCurrent()) drawerUsage.value = null
        console.error('Failed to load usage stats:', e)
      })
  }
}

const closeUserDrawer = () => {
  drawerSeq++
  drawerUserId.value = null
}

// Modals render below the drawer layer, so close the drawer before opening one.
const runFromDrawer = (action: (user: AdminUser) => void) => {
  const user = drawerUser.value
  if (!user) return
  closeUserDrawer()
  action(user)
}

const replaceGroupFromDrawer = (group: AdminGroup) => {
  const user = drawerUser.value
  if (!user) return
  closeUserDrawer()
  openGroupReplace(user, group)
}

// 滚动时关闭菜单
const handleScroll = () => {
  closeActionMenu()
}

onMounted(async () => {
  await loadAttributeDefinitions()
  loadSavedFilters()
  loadSavedColumns()
  loadUsers()
  if (hasVisibleGroupsColumn.value || filters.group) {
    loadAllGroups()
  }
  if (filters.apiKeyGroup !== null) {
    loadAllGroupsForApiKeyFilter()
  }
  document.addEventListener('click', handleClickOutside)
  window.addEventListener('scroll', handleScroll, true)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  window.removeEventListener('scroll', handleScroll, true)
  clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>

<style scoped>
.users-seg-count {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
}

.users-field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.users-field-label {
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
  font-weight: 500;
}

.users-column-list {
  display: flex;
  max-height: 18rem;
  flex-direction: column;
  overflow-y: auto;
}

.users-column-option {
  display: flex;
  min-height: 2rem;
  align-items: center;
  gap: 0.5rem;
  padding: 0 0.25rem;
  border-radius: var(--operator-radius-sm);
  color: var(--operator-popup-foreground);
  cursor: pointer;
  font-size: 0.8125rem;
}

.users-column-option:hover {
  background: var(--operator-popup-hover);
}

.users-column-option input {
  accent-color: rgb(var(--operator-accent-rgb));
}

.users-column-option:has(input:disabled) {
  cursor: not-allowed;
  opacity: 0.6;
}

.users-identity {
  display: flex;
  max-width: 18rem;
  min-width: 0;
  align-items: center;
  gap: 0.625rem;
  border-radius: var(--operator-radius-sm);
  text-align: left;
}

.users-identity:hover .ui-cell-main {
  color: var(--operator-accent-text);
}

.users-identity:focus-visible {
  outline: 2px solid rgb(var(--operator-accent-rgb) / 0.6);
  outline-offset: 2px;
}

.users-avatar {
  display: inline-grid;
  width: 2rem;
  height: 2rem;
  flex-shrink: 0;
  place-items: center;
  border-radius: 999px;
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
  font-size: 0.8125rem;
  font-weight: 600;
}

.users-avatar.is-lg {
  width: 2.5rem;
  height: 2.5rem;
  font-size: 1rem;
}

.users-num-link {
  color: var(--operator-foreground);
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  text-decoration: underline dashed var(--operator-border-strong);
  text-underline-offset: 4px;
  transition: color var(--operator-duration-fast) var(--operator-ease-out);
}

.users-num-link:hover {
  color: var(--operator-accent-text);
}

.users-danger-hover:hover {
  color: var(--operator-destructive);
}

.users-ok-hover:hover {
  color: var(--operator-success);
}

.users-drawer-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.625rem;
}

.users-stat {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.75rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-card);
  text-align: left;
}

.users-stat.is-action {
  transition:
    border-color var(--operator-duration-fast) var(--operator-ease-out),
    transform 120ms var(--operator-ease-out);
}

.users-stat.is-action:hover {
  border-color: var(--operator-border-strong);
}

.users-stat.is-action:active {
  transform: scale(0.98);
}

.users-stat-label {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.users-stat-value {
  overflow: hidden;
  color: var(--operator-foreground);
  font-size: 1.25rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.01em;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.users-stat-value small {
  color: var(--operator-subtle-foreground);
  font-size: 0.875rem;
  font-weight: 500;
}

.users-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.users-section-head .ui-section-label {
  margin-bottom: 0;
}

.users-section-head + * {
  margin-top: 0.5rem;
}

.users-quota-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.users-quota-list li {
  display: grid;
  grid-template-columns: 7.5rem minmax(0, 1fr);
  align-items: center;
  gap: 0.75rem;
}

.users-quota-platform {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
  color: var(--operator-foreground);
  font-size: 0.8125rem;
}

.users-quota-windows {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.5rem;
}

.users-quota-window {
  display: block;
  color: var(--operator-subtle-foreground);
  font-size: 0.6875rem;
}

.users-quota-windows :deep(.ui-meter) {
  min-width: 0;
}

@media (max-width: 480px) {
  .users-quota-list li {
    grid-template-columns: minmax(0, 1fr);
    gap: 0.375rem;
  }

  .users-stat-value {
    font-size: 1.0625rem;
  }
}
</style>
