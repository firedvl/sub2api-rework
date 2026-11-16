<template>
  <AppLayout>
    <TablePageLayout class="ui-page ui-stagger">
      <template #filters>
        <div class="subs-top">
          <div class="ui-toolbar" role="toolbar" :aria-label="t('admin.adminUi.filters')">
            <!-- User filter: fuzzy search, then pick a user -->
            <div class="subs-search" data-filter-user-search>
              <label class="ui-search">
                <Icon name="search" size="sm" />
                <input
                  v-model="filterUserKeyword"
                  type="text"
                  :placeholder="t('admin.users.searchUsers')"
                  :aria-label="t('admin.users.searchUsers')"
                  @input="debounceSearchFilterUsers"
                  @focus="showFilterUserDropdown = true"
                />
                <button
                  v-if="selectedFilterUser"
                  type="button"
                  class="subs-search-clear"
                  :title="t('common.clear')"
                  :aria-label="t('common.clear')"
                  @click="clearFilterUser"
                >
                  <Icon name="x" size="sm" :stroke-width="2" />
                </button>
              </label>
              <Transition name="menu-pop">
                <div
                  v-if="showFilterUserDropdown && (filterUserResults.length > 0 || filterUserKeyword)"
                  class="subs-search-menu"
                >
                  <div v-if="filterUserLoading" class="subs-search-note">
                    {{ t('common.loading') }}
                  </div>
                  <div
                    v-else-if="filterUserResults.length === 0 && filterUserKeyword"
                    class="subs-search-note"
                  >
                    {{ t('common.noOptionsFound') }}
                  </div>
                  <button
                    v-for="user in filterUserResults"
                    :key="user.id"
                    type="button"
                    class="dropdown-item"
                    @click="selectFilterUser(user)"
                  >
                    <span class="min-w-0 flex-1 truncate">{{ user.email }}</span>
                    <span class="ui-cell-sub">#{{ user.id }}</span>
                  </button>
                </div>
              </Transition>
            </div>

            <div class="viz-seg" role="group" :aria-label="t('admin.subscriptions.columns.status')">
              <button
                v-for="option in statusOptions"
                :key="option.value"
                type="button"
                :aria-pressed="filters.status === option.value"
                @click="setStatusFilter(option.value)"
              >
                {{ option.label }}
                <span
                  v-if="filters.status === option.value && !loading && pagination.total > 0"
                  class="ui-chip-count"
                >{{ pagination.total }}</span>
              </button>
            </div>

            <ToolbarPopover
              :label="t('admin.adminUi.filters')"
              icon="filter"
              :count="activeFilterCount"
              align="left"
              width="19rem"
            >
              <div>
                <div class="ui-section-label">{{ t('admin.subscriptions.columns.group') }}</div>
                <Select
                  v-model="filters.group_id"
                  :options="groupOptions"
                  :placeholder="t('admin.subscriptions.allGroups')"
                  @change="applyFilters"
                />
              </div>
              <div>
                <div class="ui-section-label">{{ t('admin.adminUi.subscriptions.platform') }}</div>
                <div class="ui-chips" role="group" :aria-label="t('admin.adminUi.subscriptions.platform')">
                  <button
                    v-for="option in platformChipOptions"
                    :key="option.value"
                    type="button"
                    class="ui-chip is-icon"
                    :aria-pressed="filters.platform === option.value"
                    :aria-label="option.label"
                    :title="option.label"
                    @click="togglePlatformFilter(option.value)"
                  >
                    <ProviderMark :platform="option.value" :size="20" />
                  </button>
                </div>
              </div>
              <button
                v-if="activeFilterCount > 0"
                type="button"
                class="btn btn-ghost btn-sm self-start"
                @click="clearExtraFilters"
              >
                <Icon name="x" size="sm" />
                {{ t('admin.adminUi.clearFilters') }}
              </button>
            </ToolbarPopover>

            <span class="ui-grow" />

            <button
              type="button"
              class="app-icon-button"
              :disabled="loading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="loadSubscriptions"
            >
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </button>

            <ToolbarPopover
              :label="t('admin.adminUi.moreActions')"
              icon="more"
              icon-only
              menu
              width="14rem"
            >
              <div class="subs-menu-label">{{ t('admin.subscriptions.columns.user') }}</div>
              <button
                type="button"
                role="menuitemradio"
                class="dropdown-item"
                :aria-checked="userColumnMode === 'email'"
                @click="setUserColumnMode('email')"
              >
                <span class="flex-1">{{ t('admin.users.columns.email') }}</span>
                <Icon v-if="userColumnMode === 'email'" name="check" size="sm" class="subs-check" />
              </button>
              <button
                type="button"
                role="menuitemradio"
                class="dropdown-item"
                :aria-checked="userColumnMode === 'username'"
                @click="setUserColumnMode('username')"
              >
                <span class="flex-1">{{ t('admin.users.columns.username') }}</span>
                <Icon v-if="userColumnMode === 'username'" name="check" size="sm" class="subs-check" />
              </button>
              <div class="subs-menu-label">{{ t('admin.users.columnSettings') }}</div>
              <button
                v-for="col in toggleableColumns"
                :key="col.key"
                type="button"
                role="menuitemcheckbox"
                class="dropdown-item"
                :aria-checked="isColumnVisible(col.key)"
                @click="toggleColumn(col.key)"
              >
                <span class="flex-1">{{ col.label }}</span>
                <Icon v-if="isColumnVisible(col.key)" name="check" size="sm" class="subs-check" />
              </button>
              <div class="subs-menu-divider" role="separator"></div>
              <button type="button" role="menuitem" class="dropdown-item" @click="showGuideModal = true">
                <Icon name="questionCircle" size="sm" />
                {{ t('admin.subscriptions.guide.showGuide') }}
              </button>
            </ToolbarPopover>

            <button type="button" class="btn btn-primary" @click="showAssignModal = true">
              <Icon name="plus" size="sm" />
              {{ t('admin.subscriptions.assignSubscription') }}
            </button>
          </div>

          <!-- Unconfirmed bulk actions saved in this browser -->
          <section
            v-if="pendingOperations.length > 0 || pendingLoadError"
            class="subs-notice"
            :aria-label="t('admin.subscriptions.bulk.pendingTitle')"
          >
            <div class="subs-notice-head">
              <Icon name="exclamationTriangle" size="sm" />
              <h3>{{ t('admin.subscriptions.bulk.pendingTitle') }}</h3>
            </div>
            <p v-if="pendingLoadError" role="alert" class="subs-notice-error">{{ pendingLoadError }}</p>
            <div v-for="operation in pendingOperations" :key="operation.storageKey" class="subs-notice-row">
              <span class="min-w-0 flex-1 break-words">
                <b>{{ t(`admin.subscriptions.bulk.${operation.request.action}`) }}</b>
                <span class="ui-cell-sub">{{ operation.request.subscription_ids.map(id => `#${id}`).join(', ') }}</span>
              </span>
              <div class="flex shrink-0 gap-1">
                <button type="button" class="btn btn-secondary btn-sm" data-test="subscription-pending-retry" :disabled="loading" @click="resumeBulkOperation(operation)">
                  <Icon name="refresh" size="sm" />{{ t('admin.subscriptions.bulk.retry') }}
                </button>
                <button type="button" class="app-icon-button" :title="t('admin.subscriptions.bulk.dismissSaved')" :aria-label="t('admin.subscriptions.bulk.dismissSaved')" @click="pendingToDismiss = operation">
                  <Icon name="x" size="sm" />
                </button>
              </div>
            </div>
          </section>

          <!-- Bulk actions for the selected rows -->
          <Transition name="fade">
            <div v-if="selectedCount > 0" class="subs-bulkbar" data-test="subscription-bulk-actions">
              <span class="subs-bulkbar-count">
                {{ t('admin.subscriptions.bulk.selected', { count: selectedCount }) }}
              </span>
              <HelpTooltip :content="t('admin.subscriptions.bulk.selectionHint')" />
              <span class="ui-grow" />
              <button
                v-for="action in bulkActions"
                :key="action"
                type="button"
                :class="action === 'revoke' ? 'btn btn-danger btn-sm' : 'btn btn-secondary btn-sm'"
                :data-test="`bulk-${action}`"
                :disabled="loading || !!pendingLoadError || bulkTargets[action].length === 0"
                @click="openBulkAction(action)"
              >
                {{ t(`admin.subscriptions.bulk.${action}`) }} ({{ bulkTargets[action].length }})
              </button>
              <button type="button" class="btn btn-ghost btn-sm" @click="clearSelection">
                {{ t('admin.subscriptions.bulk.clearSelection') }}
              </button>
            </div>
          </Transition>
        </div>
      </template>

      <!-- Subscriptions Table -->
      <template #table>
        <DataTable
          :columns="columns"
          :data="subscriptions"
          :loading="loading"
          row-key="id"
          selectable
          clickable-rows
          :selected-keys="selectedIds"
          :selection-label="getSubscriptionSelectionLabel"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          @sort="handleSort"
          @row-click="openDetail"
          @update:selected-keys="handleSelectedKeysUpdate"
        >
          <template #cell-user="{ row }">
            <div class="flex min-w-0 items-center gap-2.5">
              <span class="subs-avatar" aria-hidden="true">
                {{ userInitial(row) }}
              </span>
              <RouterLink
                :to="{ path: '/admin/usage', query: { user_id: row.user_id } }"
                class="subs-user-link"
                @click.stop
              >
                {{ userColumnMode === 'email'
                  ? (row.user?.email || t('admin.redeem.userPrefix', { id: row.user_id }))
                  : (row.user?.username || t('admin.redeem.userPrefix', { id: row.user_id }))
                }}
              </RouterLink>
            </div>
          </template>

          <template #cell-group="{ row }">
            <div v-if="row.group" class="flex min-w-0 items-center gap-2">
              <ProviderMark :platform="row.group.platform" :size="20" />
              <span class="ui-cell-main">{{ row.group.name }}</span>
            </div>
            <span v-else class="ui-cell-sub">-</span>
          </template>

          <template #cell-usage="{ row }">
            <div class="subs-usage">
              <template v-for="(windows, slot) in [usageWindows(row)]" :key="slot">
                <div
                  v-for="window in windows"
                  :key="window.key"
                  class="subs-usage-row"
                  :title="window.reset ?? undefined"
                >
                  <span class="subs-usage-label">{{ window.label }}</span>
                  <div :class="meterClass(window.percent)">
                    <div class="ui-meter-track"><i :style="{ width: meterWidth(window.percent) }"></i></div>
                  </div>
                  <span class="subs-usage-amount">
                    {{ formatUsd(window.used) }}<span> / {{ formatUsd(window.limit) }}</span>
                  </span>
                </div>
                <span v-if="windows.length === 0" class="subs-unlimited">
                  <span aria-hidden="true">∞</span>
                  {{ t('admin.subscriptions.unlimited') }}
                </span>
              </template>
            </div>
          </template>

          <template #cell-expires_at="{ value }">
            <div v-if="value">
              <span class="subs-date" :class="isExpiringSoon(value) ? 'is-soon' : ''">
                {{ formatDateTimeToMinute(value) }}
              </span>
              <template
                v-for="remainingExpiry in [formatRemainingExpiry(value)]"
                :key="remainingExpiry ?? 'expired'"
              >
                <div v-if="remainingExpiry" class="ui-cell-sub">
                  {{ remainingExpiry }}
                </div>
              </template>
            </div>
            <span v-else class="ui-cell-sub">{{ t('admin.subscriptions.noExpiration') }}</span>
          </template>

          <template #cell-status="{ value }">
            <span class="subs-status">
              <span :class="statusDotClass(value)" aria-hidden="true"></span>
              {{ t(`admin.subscriptions.status.${value}`) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="subs-row-actions" @click.stop @keydown.enter.stop>
              <button
                v-if="row.status === 'active' || row.status === 'expired'"
                type="button"
                class="app-icon-button"
                :title="t('admin.subscriptions.adjust')"
                :aria-label="t('admin.subscriptions.adjust')"
                @click="handleExtend(row)"
              >
                <Icon name="calendar" size="sm" />
              </button>
              <button
                v-if="row.status === 'revoked'"
                type="button"
                class="app-icon-button"
                :title="t('admin.subscriptions.restore')"
                :aria-label="t('admin.subscriptions.restore')"
                @click="handleRestore(row)"
              >
                <Icon name="refresh" size="sm" />
              </button>
              <ToolbarPopover
                :label="t('admin.adminUi.moreActions')"
                icon="more"
                icon-only
                menu
              >
                <button type="button" role="menuitem" class="dropdown-item" @click="openDetail(row)">
                  <Icon name="eye" size="sm" />
                  {{ t('admin.adminUi.details') }}
                </button>
                <button
                  v-if="row.status === 'active'"
                  type="button"
                  role="menuitem"
                  class="dropdown-item"
                  :disabled="resettingQuota && resettingSubscription?.id === row.id"
                  @click="handleResetQuota(row)"
                >
                  <Icon name="refresh" size="sm" />
                  {{ t('admin.subscriptions.resetQuota') }}
                </button>
                <button
                  v-if="row.status === 'active'"
                  type="button"
                  role="menuitem"
                  class="dropdown-item is-danger"
                  @click="handleRevoke(row)"
                >
                  <Icon name="ban" size="sm" />
                  {{ t('admin.subscriptions.revoke') }}
                </button>
              </ToolbarPopover>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.subscriptions.noSubscriptionsYet')"
              :description="t('admin.subscriptions.assignFirstSubscription')"
              :action-text="t('admin.subscriptions.assignSubscription')"
              @action="showAssignModal = true"
            />
          </template>
        </DataTable>
      </template>

      <!-- Pagination -->
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

    <!-- Subscription detail -->
    <SideDrawer
      :open="detailSubscription !== null"
      :title="detailSubscription ? userDisplayName(detailSubscription) : ''"
      :subtitle="detailSubscription?.group?.name"
      width="30rem"
      @close="closeDetail"
    >
      <template #leading>
        <ProviderMark
          v-if="detailSubscription?.group"
          :platform="detailSubscription.group.platform"
          :size="36"
        />
        <span v-else class="subs-avatar is-lg" aria-hidden="true">
          {{ detailSubscription ? userInitial(detailSubscription) : '' }}
        </span>
      </template>
      <template #badge>
        <span v-if="detailSubscription" class="subs-status">
          <span :class="statusDotClass(detailSubscription.status)" aria-hidden="true"></span>
          {{ t(`admin.subscriptions.status.${detailSubscription.status}`) }}
        </span>
      </template>

      <template v-if="detailSubscription">
        <section>
          <div class="ui-section-label">{{ t('admin.subscriptions.columns.usage') }}</div>
          <template v-for="(windows, slot) in [usageWindows(detailSubscription)]" :key="slot">
            <div v-if="windows.length" class="subs-windows">
              <div v-for="window in windows" :key="window.key" class="subs-window">
                <div class="subs-window-head">
                  <span>{{ window.label }}</span>
                  <span class="subs-window-amount">
                    {{ formatUsd(window.used) }}<span> / {{ formatUsd(window.limit) }}</span>
                  </span>
                </div>
                <div :class="meterClass(window.percent)">
                  <div class="ui-meter-track"><i :style="{ width: meterWidth(window.percent) }"></i></div>
                  <b>{{ Math.round(window.percent) }}%</b>
                </div>
                <div v-if="window.reset" class="subs-window-reset">
                  <Icon name="clock" size="xs" />
                  {{ window.reset }}
                </div>
              </div>
            </div>
            <span v-else class="subs-unlimited">
              <span aria-hidden="true">∞</span>
              {{ t('admin.subscriptions.unlimited') }}
            </span>
          </template>
        </section>

        <dl class="ui-dl">
          <dt>{{ t('admin.subscriptions.columns.user') }}</dt>
          <dd>
            <RouterLink
              :to="{ path: '/admin/usage', query: { user_id: detailSubscription.user_id } }"
              class="subs-user-link"
            >
              {{ detailSubscription.user?.email || t('admin.redeem.userPrefix', { id: detailSubscription.user_id }) }}
            </RouterLink>
            <div v-if="detailSubscription.user?.username" class="ui-cell-sub">
              {{ detailSubscription.user.username }} · #{{ detailSubscription.user_id }}
            </div>
          </dd>
          <dt>{{ t('admin.subscriptions.columns.group') }}</dt>
          <dd>
            <GroupBadge
              v-if="detailSubscription.group"
              :name="detailSubscription.group.name"
              :platform="detailSubscription.group.platform"
              :subscription-type="detailSubscription.group.subscription_type"
              :rate-multiplier="detailSubscription.group.rate_multiplier"
            />
            <span v-else>-</span>
          </dd>
          <dt>{{ t('admin.adminUi.subscriptions.starts') }}</dt>
          <dd>{{ formatDateTimeToMinute(detailSubscription.starts_at) }}</dd>
          <dt>{{ t('admin.subscriptions.columns.expires') }}</dt>
          <dd>
            <template v-if="detailSubscription.expires_at">
              <span :class="isExpiringSoon(detailSubscription.expires_at) ? 'subs-date is-soon' : 'subs-date'">
                {{ formatDateTimeToMinute(detailSubscription.expires_at) }}
              </span>
              <div v-if="formatRemainingExpiry(detailSubscription.expires_at)" class="ui-cell-sub">
                {{ formatRemainingExpiry(detailSubscription.expires_at) }}
              </div>
            </template>
            <span v-else>{{ t('admin.subscriptions.noExpiration') }}</span>
          </dd>
          <template v-if="detailSubscription.revoked_at">
            <dt>{{ t('admin.adminUi.subscriptions.revokedAt') }}</dt>
            <dd>{{ formatDateTimeToMinute(detailSubscription.revoked_at) }}</dd>
          </template>
          <dt>{{ t('admin.adminUi.subscriptions.created') }}</dt>
          <dd>{{ formatDateTimeToMinute(detailSubscription.created_at) }}</dd>
          <dt>ID</dt>
          <dd class="tabular-nums">#{{ detailSubscription.id }}</dd>
        </dl>
      </template>

      <template #footer>
        <template v-if="detailSubscription">
          <button
            v-if="detailSubscription.status === 'active' || detailSubscription.status === 'expired'"
            type="button"
            class="btn btn-primary btn-sm"
            @click="fromDrawer(handleExtend, detailSubscription)"
          >
            <Icon name="calendar" size="sm" />
            {{ t('admin.subscriptions.adjust') }}
          </button>
          <button
            v-if="detailSubscription.status === 'active'"
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="resettingQuota && resettingSubscription?.id === detailSubscription.id"
            @click="fromDrawer(handleResetQuota, detailSubscription)"
          >
            <Icon name="refresh" size="sm" />
            {{ t('admin.subscriptions.resetQuota') }}
          </button>
          <button
            v-if="detailSubscription.status === 'revoked'"
            type="button"
            class="btn btn-primary btn-sm"
            @click="fromDrawer(handleRestore, detailSubscription)"
          >
            <Icon name="refresh" size="sm" />
            {{ t('admin.subscriptions.restore') }}
          </button>
          <span class="ui-grow flex-1" />
          <button
            v-if="detailSubscription.status === 'active'"
            type="button"
            class="btn btn-ghost btn-sm subs-danger-text"
            @click="fromDrawer(handleRevoke, detailSubscription)"
          >
            <Icon name="ban" size="sm" />
            {{ t('admin.subscriptions.revoke') }}
          </button>
        </template>
      </template>
    </SideDrawer>

    <BulkSubscriptionActionDialog
      v-if="bulkAction !== null"
      :show="true"
      :action="bulkAction"
      :subscriptions="bulkSubscriptions"
      :operation-to-resume="resumedOperation"
      @close="closeBulkAction"
      @completed="handleBulkCompleted"
    />

    <!-- Assign Subscription Modal -->
    <BaseDialog
      :show="showAssignModal"
      :title="t('admin.subscriptions.assignSubscription')"
      width="normal"
      :show-close-button="!submitting"
      :close-on-escape="!submitting"
      @close="closeAssignModal"
    >
      <form
        id="assign-subscription-form"
        @submit.prevent="handleAssignSubscription"
        class="space-y-5"
      >
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
          <input v-model="batchAssignEnabled" type="checkbox" :disabled="submitting" @change="resetAssignUsers" />
          {{ t('admin.subscriptions.batchAssign.enable') }}
        </label>
        <p v-if="batchAssignEnabled" class="input-hint">{{ t('admin.subscriptions.batchAssign.hint') }}</p>
        <div>
          <label class="input-label">{{ t('admin.subscriptions.form.user') }}</label>
          <div class="relative" data-assign-user-search>
            <input
              v-model="userSearchKeyword"
              type="text"
              :disabled="submitting || (batchAssignEnabled && assignUsers.length >= 100)"
              class="input pr-8"
              :placeholder="t('admin.usage.searchUserPlaceholder')"
              @input="debounceSearchUsers"
              @focus="showUserDropdown = true"
            />
            <button
              v-if="selectedUser"
              @click="clearUserSelection"
              type="button"
              class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
            >
              <Icon name="x" size="sm" :stroke-width="2" />
            </button>
            <!-- User Dropdown -->
            <div
              v-if="showUserDropdown && (userSearchResults.length > 0 || userSearchKeyword)"
              class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-700 dark:bg-dark-800"
            >
              <div
                v-if="userSearchLoading"
                class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('common.loading') }}
              </div>
              <div
                v-else-if="userSearchResults.length === 0 && userSearchKeyword"
                class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('common.noOptionsFound') }}
              </div>
              <button
                v-for="user in userSearchResults"
                :key="user.id"
                type="button"
                :disabled="submitting || (batchAssignEnabled && assignUsers.some((selected) => selected.id === user.id))"
                @click="selectUser(user)"
                class="w-full px-4 py-2 text-left text-sm hover:bg-gray-100 disabled:opacity-50 dark:hover:bg-dark-700"
              >
                <span class="font-medium text-gray-900 dark:text-white">{{ user.email }}</span>
                <span class="ml-2 text-gray-500 dark:text-gray-400">#{{ user.id }}</span>
              </button>
            </div>
          </div>
          <div v-if="batchAssignEnabled && assignUsers.length > 0" class="mt-2 space-y-2" data-test="assign-users">
            <p class="text-sm text-gray-600 dark:text-gray-400">
              {{ t('admin.subscriptions.batchAssign.selected', { count: assignUsers.length }) }}
            </p>
            <ul class="max-h-40 space-y-1 overflow-y-auto">
              <li v-for="user in assignUsers" :key="user.id" class="flex items-center justify-between gap-2 rounded-lg bg-gray-50 px-3 py-1 text-sm dark:bg-dark-700">
                <span class="truncate">{{ user.email }} <span class="text-gray-500">#{{ user.id }}</span></span>
                <button
                  type="button"
                  :disabled="submitting"
                  :aria-label="t('admin.subscriptions.batchAssign.removeUser', { email: user.email })"
                  @click="assignUsers = assignUsers.filter((selected) => selected.id !== user.id)"
                >
                  <Icon name="x" size="sm" />
                </button>
              </li>
            </ul>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('admin.subscriptions.form.group') }}</label>
          <Select
            v-model="assignForm.group_id"
            :disabled="submitting"
            :options="subscriptionGroupOptions"
            :placeholder="t('admin.subscriptions.selectGroup')"
          >
            <template #selected="{ option }">
              <GroupBadge
                v-if="option"
                :name="(option as unknown as GroupOption).label"
                :platform="(option as unknown as GroupOption).platform"
                :subscription-type="(option as unknown as GroupOption).subscriptionType"
                :rate-multiplier="(option as unknown as GroupOption).rate"
              />
              <span v-else class="text-gray-400">{{ t('admin.subscriptions.selectGroup') }}</span>
            </template>
            <template #option="{ option, selected }">
              <GroupOptionItem
                :name="(option as unknown as GroupOption).label"
                :platform="(option as unknown as GroupOption).platform"
                :subscription-type="(option as unknown as GroupOption).subscriptionType"
                :rate-multiplier="(option as unknown as GroupOption).rate"
                :description="(option as unknown as GroupOption).description"
                :selected="selected"
              />
            </template>
          </Select>
          <p class="input-hint">{{ t('admin.subscriptions.groupHint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.subscriptions.form.validityDays') }}</label>
          <input v-model.number="assignForm.validity_days" type="number" min="1" max="36500" step="1" :disabled="submitting" class="input" />
          <p class="input-hint">{{ t('admin.subscriptions.validityHint') }}</p>
        </div>
        <div v-if="batchAssignResult" class="space-y-2 text-sm" role="status" data-test="batch-assign-result">
          <p>{{ t('admin.subscriptions.batchAssign.result', { success: batchAssignResult.success_count, failed: batchAssignResult.failed_count }) }}</p>
          <ul v-if="batchAssignResult.errors.length" class="max-h-40 space-y-1 overflow-y-auto text-red-600 dark:text-red-400">
            <li v-for="(error, index) in batchAssignResult.errors" :key="index">{{ error }}</li>
          </ul>
          <p v-if="batchAssignResult.failed_count > 0" class="input-hint">{{ t('admin.subscriptions.batchAssign.retryHint') }}</p>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button @click="closeAssignModal" type="button" :disabled="submitting" class="btn btn-secondary">
            {{ batchAssignResult ? t('common.close') : t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="assign-subscription-form"
            :disabled="submitting || (batchAssignEnabled && assignUsers.length === 0)"
            class="btn btn-primary"
          >
            <svg
              v-if="submitting"
              class="-ml-1 mr-2 h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            {{ submitting ? t('admin.subscriptions.assigning') : t('admin.subscriptions.assign') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Adjust Subscription Modal -->
    <BaseDialog
      :show="showExtendModal"
      :title="t('admin.subscriptions.adjustSubscription')"
      width="narrow"
      @close="closeExtendModal"
    >
      <form
        v-if="extendingSubscription"
        id="extend-subscription-form"
        @submit.prevent="handleExtendSubscription"
        class="space-y-5"
      >
        <div class="rounded-lg bg-gray-50 p-4 dark:bg-dark-700">
          <p class="text-sm text-gray-600 dark:text-gray-400">
            {{ t('admin.subscriptions.adjustingFor') }}
            <span class="font-medium text-gray-900 dark:text-white">{{
              extendingSubscription.user?.email
            }}</span>
          </p>
          <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
            {{ t('admin.subscriptions.currentExpiration') }}:
            <span class="font-medium text-gray-900 dark:text-white">
              {{
                extendingSubscription.expires_at
                  ? formatDateTimeToMinute(extendingSubscription.expires_at)
                  : t('admin.subscriptions.noExpiration')
              }}
            </span>
          </p>
          <p v-if="extendingSubscription.expires_at" class="mt-1 text-sm text-gray-600 dark:text-gray-400">
            {{ t('admin.subscriptions.remainingDays') }}:
            <span class="font-medium text-gray-900 dark:text-white">
              {{ getDaysRemaining(extendingSubscription.expires_at) ?? 0 }}
            </span>
          </p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.subscriptions.form.adjustDays') }}</label>
          <div class="flex items-center gap-2">
            <input
              v-model.number="extendForm.days"
              type="number"
              required
              class="input text-center"
              :placeholder="t('admin.subscriptions.adjustDaysPlaceholder')"
            />
          </div>
          <p class="input-hint">{{ t('admin.subscriptions.adjustHint') }}</p>
        </div>
      </form>
      <template #footer>
        <div v-if="extendingSubscription" class="flex justify-end gap-3">
          <button @click="closeExtendModal" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            form="extend-subscription-form"
            :disabled="submitting"
            class="btn btn-primary"
          >
            {{ submitting ? t('admin.subscriptions.adjusting') : t('admin.subscriptions.adjust') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="pendingToDismiss !== null"
      :title="t('admin.subscriptions.bulk.dismissSaved')"
      :message="t('admin.subscriptions.bulk.dismissUnconfirmed')"
      :confirm-text="t('admin.subscriptions.bulk.dismissSaved')"
      :danger="true"
      @confirm="dismissPendingOperation"
      @cancel="pendingToDismiss = null"
    />

    <!-- Revoke Confirmation Dialog -->
    <ConfirmDialog
      :show="showRevokeDialog"
      :title="t('admin.subscriptions.revokeSubscription')"
      :message="t('admin.subscriptions.revokeConfirm', { user: revokingSubscription?.user?.email })"
      :confirm-text="t('admin.subscriptions.revoke')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmRevoke"
      @cancel="showRevokeDialog = false"
    />

    <!-- Restore Confirmation Dialog -->
    <ConfirmDialog
      :show="showRestoreDialog"
      :title="t('admin.subscriptions.restoreSubscription')"
      :message="t('admin.subscriptions.restoreConfirm', { user: restoringSubscription?.user?.email })"
      :confirm-text="t('admin.subscriptions.restore')"
      :cancel-text="t('common.cancel')"
      @confirm="confirmRestore"
      @cancel="showRestoreDialog = false"
    />

    <!-- Reset Quota Confirmation Dialog -->
    <ConfirmDialog
      :show="showResetQuotaConfirm"
      :title="t('admin.subscriptions.resetQuotaTitle')"
      :message="t('admin.subscriptions.resetQuotaConfirm', { user: resettingSubscription?.user?.email })"
      :confirm-text="t('admin.subscriptions.resetQuota')"
      :cancel-text="t('common.cancel')"
      @confirm="confirmResetQuota"
      @cancel="showResetQuotaConfirm = false"
    />
    <!-- Subscription Guide Modal -->
    <teleport to="body">
      <transition name="modal">
        <div v-if="showGuideModal" class="fixed inset-0 z-50 flex items-center justify-center p-4" @mousedown.self="showGuideModal = false">
          <div class="fixed inset-0 bg-black/50" @click="showGuideModal = false"></div>
          <div class="relative max-h-[85vh] w-full max-w-2xl overflow-y-auto rounded-xl bg-white p-6 shadow-2xl dark:bg-dark-800">
            <button type="button" class="absolute right-4 top-4 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200" @click="showGuideModal = false">
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>

            <h2 class="mb-4 text-lg font-bold text-gray-900 dark:text-white">{{ t('admin.subscriptions.guide.title') }}</h2>
            <p class="mb-5 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.subscriptions.guide.subtitle') }}</p>

            <!-- Step 1 -->
            <div class="mb-5">
              <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">1</span>
                {{ t('admin.subscriptions.guide.step1.title') }}
              </h3>
              <ol class="ml-8 list-decimal space-y-1 text-sm text-gray-600 dark:text-gray-300">
                <li>{{ t('admin.subscriptions.guide.step1.line1') }}</li>
                <li>{{ t('admin.subscriptions.guide.step1.line2') }}</li>
                <li>{{ t('admin.subscriptions.guide.step1.line3') }}</li>
              </ol>
              <div class="ml-8 mt-2">
                <router-link
                  to="/admin/groups"
                  @click="showGuideModal = false"
                  class="inline-flex items-center gap-1 text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                >
                  {{ t('admin.subscriptions.guide.step1.link') }}
                  <Icon name="arrowRight" size="xs" />
                </router-link>
              </div>
            </div>

            <!-- Step 2 -->
            <div class="mb-5">
              <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">2</span>
                {{ t('admin.subscriptions.guide.step2.title') }}
              </h3>
              <ol class="ml-8 list-decimal space-y-1 text-sm text-gray-600 dark:text-gray-300">
                <li>{{ t('admin.subscriptions.guide.step2.line1') }}</li>
                <li>{{ t('admin.subscriptions.guide.step2.line2') }}</li>
                <li>{{ t('admin.subscriptions.guide.step2.line3') }}</li>
              </ol>
            </div>

            <!-- Step 3 -->
            <div class="mb-5">
              <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-white">
                <span class="flex h-6 w-6 items-center justify-center rounded-full bg-primary-100 text-xs font-bold text-primary-700 dark:bg-primary-900/40 dark:text-primary-300">3</span>
                {{ t('admin.subscriptions.guide.step3.title') }}
              </h3>
              <div class="ml-8 overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
                <table class="w-full text-sm">
                  <tbody>
                    <tr v-for="(row, i) in guideActionRows" :key="i" class="border-b border-gray-100 dark:border-dark-700 last:border-0">
                      <td class="whitespace-nowrap bg-gray-50 px-3 py-2 font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-300">{{ row.action }}</td>
                      <td class="px-3 py-2 text-gray-600 dark:text-gray-400">{{ row.desc }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Tip -->
            <div class="rounded-lg bg-blue-50 p-3 text-xs text-blue-700 dark:bg-blue-900/20 dark:text-blue-300">
              {{ t('admin.subscriptions.guide.tip') }}
            </div>

            <div class="mt-4 text-right">
              <button type="button" class="btn btn-primary btn-sm" @click="showGuideModal = false">{{ t('common.close') }}</button>
            </div>
          </div>
        </div>
      </transition>
    </teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { AdminUser, UserSubscription, Group, GroupPlatform, SubscriptionType } from '@/types'
import type { SimpleUser } from '@/api/admin/usage'
import type { SubscriptionBulkAction, SubscriptionBulkActionResult, BulkAssignSubscriptionResult } from '@/api/admin/subscriptions'
import { useTableSelection } from '@/composables/useTableSelection'
import BulkSubscriptionActionDialog from '@/components/admin/subscription/BulkSubscriptionActionDialog.vue'
import type { Column } from '@/components/common/types'
import { formatDateTimeToMinute } from '@/utils/format'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import ToolbarPopover from '@/components/common/ToolbarPopover.vue'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import ProviderMark from '@/components/viz/ProviderMark.vue'
import Icon from '@/components/icons/Icon.vue'
import { completeBulkSubscriptionOperation, listPendingBulkSubscriptionOperations, type BulkSubscriptionOperation } from '@/components/admin/subscription/bulkSubscriptionOperation'
import {
  getRemainingDurationParts,
  getRemainingExpiryDuration,
  isOneTimeDailyQuota,
  type RemainingDurationParts
} from '@/utils/subscriptionQuota'
import { GROUP_PLATFORM_OPTIONS } from '@/constants/platforms'

const { t } = useI18n()
const appStore = useAppStore()

interface GroupOption {
  value: number
  label: string
  description: string | null
  platform: GroupPlatform
  subscriptionType: SubscriptionType
  rate: number
}

// Guide modal state
const showGuideModal = ref(false)

const guideActionRows = computed(() => [
  { action: t('admin.subscriptions.guide.actions.adjust'), desc: t('admin.subscriptions.guide.actions.adjustDesc') },
  { action: t('admin.subscriptions.guide.actions.resetQuota'), desc: t('admin.subscriptions.guide.actions.resetQuotaDesc') },
  { action: t('admin.subscriptions.guide.actions.revoke'), desc: t('admin.subscriptions.guide.actions.revokeDesc') }
])

// User column display mode: 'email' or 'username'
const userColumnMode = ref<'email' | 'username'>('email')
const USER_COLUMN_MODE_KEY = 'subscription-user-column-mode'

const loadUserColumnMode = () => {
  try {
    const saved = localStorage.getItem(USER_COLUMN_MODE_KEY)
    if (saved === 'email' || saved === 'username') {
      userColumnMode.value = saved
    }
  } catch (e) {
    console.error('Failed to load user column mode:', e)
  }
}

const saveUserColumnMode = () => {
  try {
    localStorage.setItem(USER_COLUMN_MODE_KEY, userColumnMode.value)
  } catch (e) {
    console.error('Failed to save user column mode:', e)
  }
}

const setUserColumnMode = (mode: 'email' | 'username') => {
  userColumnMode.value = mode
  saveUserColumnMode()
}

// All available columns
const allColumns = computed<Column[]>(() => [
  {
    key: 'user',
    label: userColumnMode.value === 'email'
      ? t('admin.subscriptions.columns.user')
      : t('admin.users.columns.username'),
    sortable: false
  },
  { key: 'group', label: t('admin.subscriptions.columns.group'), sortable: false },
  { key: 'usage', label: t('admin.subscriptions.columns.usage'), sortable: false },
  { key: 'expires_at', label: t('admin.subscriptions.columns.expires'), sortable: true },
  { key: 'status', label: t('admin.subscriptions.columns.status'), sortable: true },
  { key: 'actions', label: t('admin.subscriptions.columns.actions'), sortable: false }
])

// Columns that can be toggled (exclude user and actions which are always visible)
const toggleableColumns = computed(() =>
  allColumns.value.filter(col => col.key !== 'user' && col.key !== 'actions')
)

// Hidden columns set
const hiddenColumns = reactive<Set<string>>(new Set())

// Default hidden columns
const DEFAULT_HIDDEN_COLUMNS: string[] = []

// localStorage key
const HIDDEN_COLUMNS_KEY = 'subscription-hidden-columns'

// Load saved column settings
const loadSavedColumns = () => {
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as string[]
      parsed.forEach(key => hiddenColumns.add(key))
    } else {
      DEFAULT_HIDDEN_COLUMNS.forEach(key => hiddenColumns.add(key))
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
  } catch (e) {
    console.error('Failed to save columns:', e)
  }
}

// Toggle column visibility
const toggleColumn = (key: string) => {
  if (hiddenColumns.has(key)) {
    hiddenColumns.delete(key)
  } else {
    hiddenColumns.add(key)
  }
  saveColumnsToStorage()
}

// Check if column is visible
const isColumnVisible = (key: string) => !hiddenColumns.has(key)

// Filtered columns for display
const columns = computed<Column[]>(() =>
  allColumns.value.filter(col =>
    col.key === 'user' || col.key === 'actions' || !hiddenColumns.has(col.key)
  )
)

// Filter options
const statusOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'active', label: t('admin.subscriptions.status.active') },
  { value: 'expired', label: t('admin.subscriptions.status.expired') },
  { value: 'revoked', label: t('admin.subscriptions.status.revoked') }
])

const subscriptions = ref<UserSubscription[]>([])
const groups = ref<Group[]>([])
const loading = ref(false)
let abortController: AbortController | null = null

const { selectedIds, selectedCount, setSelectedIds, clear: clearSelection, removeMany: removeSelectedIds } =
  useTableSelection<UserSubscription>({ rows: subscriptions, getId: (subscription) => subscription.id })
const bulkActions: SubscriptionBulkAction[] = ['extend', 'reset_quota', 'revoke', 'restore']
const bulkAction = ref<SubscriptionBulkAction | null>(null)
const bulkSubscriptions = ref<UserSubscription[]>([])
const pendingOperations = ref<BulkSubscriptionOperation[]>([])
const pendingLoadError = ref('')
const resumedOperation = ref<BulkSubscriptionOperation | null>(null)
const pendingToDismiss = ref<BulkSubscriptionOperation | null>(null)
const refreshPendingOperations = () => {
  try {
    pendingOperations.value = listPendingBulkSubscriptionOperations()
    pendingLoadError.value = ''
  } catch (error) {
    pendingOperations.value = []
    pendingLoadError.value = t('admin.subscriptions.bulk.pendingLoadFailed')
    console.error('Failed to read pending subscription actions:', error)
  }
}
const resumeBulkOperation = (operation: BulkSubscriptionOperation) => {
  resumedOperation.value = operation
  bulkSubscriptions.value = subscriptions.value.filter(subscription => operation.request.subscription_ids.includes(subscription.id))
  bulkAction.value = operation.request.action
}
const closeBulkAction = () => {
  bulkAction.value = null
  resumedOperation.value = null
  refreshPendingOperations()
}
const dismissPendingOperation = () => {
  if (!pendingToDismiss.value) return
  completeBulkSubscriptionOperation(pendingToDismiss.value)
  pendingToDismiss.value = null
  refreshPendingOperations()
}
const bulkTargets = computed(() => {
  const selected = subscriptions.value.filter((subscription) => selectedIds.value.includes(subscription.id))
  return {
    extend: selected.filter((subscription) => ['active', 'expired'].includes(subscription.status)),
    reset_quota: selected.filter((subscription) => subscription.status === 'active'),
    revoke: selected.filter((subscription) => subscription.status === 'active'),
    restore: selected.filter((subscription) => subscription.status === 'revoked')
  }
})
const getSubscriptionSelectionLabel = (subscription: UserSubscription) =>
  t('admin.subscriptions.bulk.selectSubscription', { id: subscription.id })
const handleSelectedKeysUpdate = (keys: Array<string | number>) => {
  const visibleIds = new Set(subscriptions.value.map((subscription) => subscription.id))
  setSelectedIds(keys.filter((key): key is number => typeof key === 'number' && visibleIds.has(key)))
}
const openBulkAction = (action: SubscriptionBulkAction) => {
  if (loading.value || pendingLoadError.value || bulkTargets.value[action].length === 0) return
  resumedOperation.value = null
  bulkSubscriptions.value = [...bulkTargets.value[action]]
  bulkAction.value = action
}
const handleBulkCompleted = async (result: SubscriptionBulkActionResult) => {
  refreshPendingOperations()
  removeSelectedIds(result.results.filter((item) => item.success).map((item) => item.subscription_id))
  await loadSubscriptions()
}

// Toolbar user filter (fuzzy search -> select user_id)
const filterUserKeyword = ref('')
const filterUserResults = ref<SimpleUser[]>([])
const filterUserLoading = ref(false)
const showFilterUserDropdown = ref(false)
const selectedFilterUser = ref<SimpleUser | null>(null)
let filterUserSearchTimeout: ReturnType<typeof setTimeout> | null = null

// User search state
const userSearchKeyword = ref('')
const userSearchResults = ref<AdminUser[]>([])
const userSearchLoading = ref(false)
const showUserDropdown = ref(false)
const selectedUser = ref<AdminUser | null>(null)
const batchAssignEnabled = ref(false)
const assignUsers = ref<AdminUser[]>([])
const batchAssignResult = ref<BulkAssignSubscriptionResult | null>(null)
let userSearchTimeout: ReturnType<typeof setTimeout> | null = null

const filters = reactive({
  status: 'active',
  group_id: '',
  platform: '',
  user_id: null as number | null
})

// Sorting state
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})

const showAssignModal = ref(false)
const showExtendModal = ref(false)
const showRevokeDialog = ref(false)
const showRestoreDialog = ref(false)
const showResetQuotaConfirm = ref(false)
const submitting = ref(false)
const resettingSubscription = ref<UserSubscription | null>(null)
const resettingQuota = ref(false)
const extendingSubscription = ref<UserSubscription | null>(null)
const revokingSubscription = ref<UserSubscription | null>(null)
const restoringSubscription = ref<UserSubscription | null>(null)

const assignForm = reactive({
  user_id: null as number | null,
  group_id: null as number | null,
  validity_days: 30
})

const extendForm = reactive({
  days: 30
})

// Group options for filter (all groups)
const groupOptions = computed(() => [
  { value: '', label: t('admin.subscriptions.allGroups') },
  ...groups.value.map((g) => ({ value: g.id.toString(), label: g.name }))
])

const platformChipOptions = GROUP_PLATFORM_OPTIONS

const activeFilterCount = computed(() => (filters.group_id ? 1 : 0) + (filters.platform ? 1 : 0))

// Group options for assign (only subscription type groups)
const subscriptionGroupOptions = computed(() =>
  groups.value
    .filter((g) => g.subscription_type === 'subscription' && g.status === 'active')
    .map((g) => ({
      value: g.id,
      label: g.name,
      description: g.description,
      platform: g.platform,
      subscriptionType: g.subscription_type,
      rate: g.rate_multiplier
    }))
)

const applyFilters = () => {
  clearSelection()
  pagination.page = 1
  loadSubscriptions()
}

const setStatusFilter = (status: string) => {
  if (filters.status === status) return
  filters.status = status
  applyFilters()
}

const togglePlatformFilter = (platform: string) => {
  filters.platform = filters.platform === platform ? '' : platform
  applyFilters()
}

const clearExtraFilters = () => {
  filters.group_id = ''
  filters.platform = ''
  applyFilters()
}

// Detail drawer: follows the row by ID so it reflects reloaded data.
const detailId = ref<number | null>(null)
const detailSubscription = computed(() =>
  detailId.value === null ? null : (subscriptions.value.find((row) => row.id === detailId.value) ?? null)
)
const openDetail = (subscription: UserSubscription) => {
  detailId.value = subscription.id
}
const closeDetail = () => {
  detailId.value = null
}
// Dialogs sit below the drawer layer, so close the drawer before opening one.
const fromDrawer = (action: (subscription: UserSubscription) => void, subscription: UserSubscription) => {
  closeDetail()
  action(subscription)
}

const userDisplayName = (subscription: UserSubscription) =>
  subscription.user?.email || subscription.user?.username || t('admin.redeem.userPrefix', { id: subscription.user_id })

const userInitial = (subscription: UserSubscription) =>
  (userColumnMode.value === 'email'
    ? subscription.user?.email?.charAt(0).toUpperCase()
    : subscription.user?.username?.charAt(0).toUpperCase()) || '?'

const statusDotClass = (status: string) =>
  status === 'active' ? 'ui-dot is-ok' : status === 'expired' ? 'ui-dot is-warn' : 'ui-dot is-bad'

const loadSubscriptions = async () => {
  if (abortController) {
    abortController.abort()
  }
  const requestController = new AbortController()
  abortController = requestController
  const { signal } = requestController

  loading.value = true
  try {
    const response = await adminAPI.subscriptions.list(
      pagination.page,
      pagination.page_size,
      {
        status: (filters.status as any) || undefined,
        group_id: filters.group_id ? parseInt(filters.group_id) : undefined,
        platform: filters.platform || undefined,
        user_id: filters.user_id || undefined,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      },
      {
        signal
      }
    )
    if (signal.aborted || abortController !== requestController) return
    subscriptions.value = response.items
    const visibleIds = new Set(response.items.map((subscription) => subscription.id))
    setSelectedIds(selectedIds.value.filter((id) => visibleIds.has(id)))
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error: any) {
    if (signal.aborted || error?.name === 'AbortError' || error?.code === 'ERR_CANCELED') {
      return
    }
    appStore.showError(t('admin.subscriptions.failedToLoad'))
    console.error('Error loading subscriptions:', error)
  } finally {
    if (abortController === requestController) {
      loading.value = false
      abortController = null
    }
  }
}

const loadGroups = async () => {
  try {
    groups.value = await adminAPI.groups.getAll()
  } catch (error) {
    console.error('Error loading groups:', error)
  }
}

// Toolbar user filter search with debounce
const debounceSearchFilterUsers = () => {
  if (filterUserSearchTimeout) {
    clearTimeout(filterUserSearchTimeout)
  }
  filterUserSearchTimeout = setTimeout(searchFilterUsers, 300)
}

const searchFilterUsers = async () => {
  const keyword = filterUserKeyword.value.trim()

  // Clear active user filter if user modified the search keyword
  if (selectedFilterUser.value && keyword !== selectedFilterUser.value.email) {
    selectedFilterUser.value = null
    filters.user_id = null
    applyFilters()
  }

  if (!keyword) {
    filterUserResults.value = []
    return
  }

  filterUserLoading.value = true
  try {
    filterUserResults.value = await adminAPI.usage.searchUsers(keyword)
  } catch (error) {
    console.error('Failed to search users:', error)
    filterUserResults.value = []
  } finally {
    filterUserLoading.value = false
  }
}

const selectFilterUser = (user: SimpleUser) => {
  selectedFilterUser.value = user
  filterUserKeyword.value = user.email
  showFilterUserDropdown.value = false
  filters.user_id = user.id
  applyFilters()
}

const clearFilterUser = () => {
  selectedFilterUser.value = null
  filterUserKeyword.value = ''
  filterUserResults.value = []
  showFilterUserDropdown.value = false
  filters.user_id = null
  applyFilters()
}

// User search with debounce
const debounceSearchUsers = () => {
  if (selectedUser.value && userSearchKeyword.value.trim() !== selectedUser.value.email) {
    selectedUser.value = null
    assignForm.user_id = null
  }
  if (userSearchTimeout) {
    clearTimeout(userSearchTimeout)
  }
  userSearchTimeout = setTimeout(searchUsers, 300)
}

const searchUsers = async () => {
  const keyword = userSearchKeyword.value.trim()

  if (!keyword) {
    userSearchResults.value = []
    return
  }

  userSearchLoading.value = true
  try {
    const result = await adminAPI.users.list(1, 30, {
      search: keyword, sort_by: 'email', sort_order: 'asc'
    })
    userSearchResults.value = result.items
  } catch (error) {
    console.error('Failed to search users:', error)
    userSearchResults.value = []
  } finally {
    userSearchLoading.value = false
  }
}

const selectUser = (user: AdminUser) => {
  if (submitting.value) return
  if (batchAssignEnabled.value) {
    if (assignUsers.value.length < 100 && !assignUsers.value.some((selected) => selected.id === user.id)) {
      assignUsers.value = [...assignUsers.value, user]
    }
    userSearchKeyword.value = ''
    userSearchResults.value = []
    showUserDropdown.value = false
    return
  }
  selectedUser.value = user
  userSearchKeyword.value = user.email
  showUserDropdown.value = false
  assignForm.user_id = user.id
}

const clearUserSelection = () => {
  selectedUser.value = null
  userSearchKeyword.value = ''
  userSearchResults.value = []
  assignForm.user_id = null
}

const resetAssignUsers = () => {
  clearUserSelection()
  assignUsers.value = []
  batchAssignResult.value = null
}

const handlePageChange = (page: number) => {
  clearSelection()
  pagination.page = page
  loadSubscriptions()
}

const handlePageSizeChange = (pageSize: number) => {
  clearSelection()
  pagination.page_size = pageSize
  pagination.page = 1
  loadSubscriptions()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  clearSelection()
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadSubscriptions()
}

const closeAssignModal = () => {
  if (submitting.value) return
  showAssignModal.value = false
  batchAssignEnabled.value = false
  assignUsers.value = []
  batchAssignResult.value = null
  assignForm.user_id = null
  assignForm.group_id = null
  assignForm.validity_days = 30
  // Clear user search state
  selectedUser.value = null
  userSearchKeyword.value = ''
  userSearchResults.value = []
  showUserDropdown.value = false
}

const handleAssignSubscription = async () => {
  if (submitting.value) return
  if (batchAssignEnabled.value ? assignUsers.value.length === 0 : !assignForm.user_id) {
    appStore.showError(t('admin.subscriptions.pleaseSelectUser'))
    return
  }
  if (!assignForm.group_id) {
    appStore.showError(t('admin.subscriptions.pleaseSelectGroup'))
    return
  }
  if (!Number.isInteger(assignForm.validity_days) || assignForm.validity_days < 1 || assignForm.validity_days > 36500) {
    appStore.showError(t('admin.subscriptions.validityDaysRequired'))
    return
  }

  submitting.value = true
  try {
    if (batchAssignEnabled.value) {
      batchAssignResult.value = await adminAPI.subscriptions.bulkAssign({
        user_ids: assignUsers.value.map((user) => user.id),
        group_id: assignForm.group_id,
        validity_days: assignForm.validity_days
      })
      const result = batchAssignResult.value
      const successIds = new Set(result.subscriptions.map((subscription) => subscription.user_id))
      assignUsers.value = assignUsers.value.filter((user) => !successIds.has(user.id))
      if (result.success_count > 0) {
        appStore.showSuccess(t('admin.subscriptions.batchAssign.result', { success: result.success_count, failed: result.failed_count }))
        await loadSubscriptions()
      }
      return
    }
    await adminAPI.subscriptions.assign({
      user_id: assignForm.user_id!,
      group_id: assignForm.group_id,
      validity_days: assignForm.validity_days
    })
    appStore.showSuccess(t('admin.subscriptions.subscriptionAssigned'))
    submitting.value = false
    closeAssignModal()
    loadSubscriptions()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToAssign'))
    console.error('Error assigning subscription:', error)
  } finally {
    submitting.value = false
  }
}

const handleExtend = (subscription: UserSubscription) => {
  extendingSubscription.value = subscription
  extendForm.days = 30
  showExtendModal.value = true
}

const closeExtendModal = () => {
  showExtendModal.value = false
  extendingSubscription.value = null
}

const handleExtendSubscription = async () => {
  if (!extendingSubscription.value) return

  // 前端验证：调整后的过期时间必须在未来
  if (extendingSubscription.value.expires_at) {
    const expiresAt = new Date(extendingSubscription.value.expires_at)
    const newExpiresAt = new Date(expiresAt.getTime() + extendForm.days * 24 * 60 * 60 * 1000)
    if (newExpiresAt <= new Date()) {
      appStore.showError(t('admin.subscriptions.adjustWouldExpire'))
      return
    }
  }

  submitting.value = true
  try {
    await adminAPI.subscriptions.extend(extendingSubscription.value.id, {
      days: extendForm.days
    })
    appStore.showSuccess(t('admin.subscriptions.subscriptionAdjusted'))
    closeExtendModal()
    loadSubscriptions()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToAdjust'))
    console.error('Error adjusting subscription:', error)
  } finally {
    submitting.value = false
  }
}

const handleRevoke = (subscription: UserSubscription) => {
  revokingSubscription.value = subscription
  showRevokeDialog.value = true
}

const confirmRevoke = async () => {
  if (!revokingSubscription.value) return

  try {
    await adminAPI.subscriptions.revoke(revokingSubscription.value.id)
    appStore.showSuccess(t('admin.subscriptions.subscriptionRevoked'))
    showRevokeDialog.value = false
    revokingSubscription.value = null
    loadSubscriptions()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToRevoke'))
    console.error('Error revoking subscription:', error)
  }
}

const handleRestore = (subscription: UserSubscription) => {
  restoringSubscription.value = subscription
  showRestoreDialog.value = true
}

const confirmRestore = async () => {
  if (!restoringSubscription.value) return

  try {
    await adminAPI.subscriptions.restore(restoringSubscription.value.id)
    appStore.showSuccess(t('admin.subscriptions.subscriptionRestored'))
    showRestoreDialog.value = false
    restoringSubscription.value = null
    loadSubscriptions()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToRestore'))
    console.error('Error restoring subscription:', error)
  }
}

const handleResetQuota = (subscription: UserSubscription) => {
  resettingSubscription.value = subscription
  showResetQuotaConfirm.value = true
}

const confirmResetQuota = async () => {
  if (!resettingSubscription.value) return
  if (resettingQuota.value) return
  resettingQuota.value = true
  try {
    await adminAPI.subscriptions.resetQuota(resettingSubscription.value.id, { daily: true, weekly: true, monthly: true })
    appStore.showSuccess(t('admin.subscriptions.quotaResetSuccess'))
    showResetQuotaConfirm.value = false
    resettingSubscription.value = null
    await loadSubscriptions()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.subscriptions.failedToResetQuota'))
    console.error('Error resetting quota:', error)
  } finally {
    resettingQuota.value = false
  }
}

// Helper functions
const getDaysRemaining = (expiresAt: string): number | null => {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  if (diff < 0) return null
  return Math.ceil(diff / (1000 * 60 * 60 * 24))
}

const formatRemainingExpiry = (expiresAt: string): string | null => {
  const duration = getRemainingExpiryDuration(expiresAt)
  if (!duration) return null
  if (duration.unit === 'days') {
    return t('admin.subscriptions.daysRemaining', { days: duration.days })
  }
  if (duration.hours) {
    return t('admin.subscriptions.hoursMinutesRemaining', {
      hours: duration.hours,
      minutes: duration.minutes
    })
  }
  return t('admin.subscriptions.minutesRemaining', { minutes: duration.minutes })
}

const isExpiringSoon = (expiresAt: string): boolean => {
  const days = getDaysRemaining(expiresAt)
  return days !== null && days <= 7
}

interface UsageWindow {
  key: 'daily' | 'weekly' | 'monthly'
  label: string
  used: number
  limit: number
  percent: number
  reset: string | null
}

// Configured quota windows only; an empty list means the group is unlimited.
const usageWindows = (subscription: UserSubscription): UsageWindow[] => {
  const group = subscription.group
  if (!group) return []
  const windows = [
    {
      key: 'daily' as const,
      label: t('admin.subscriptions.daily'),
      used: subscription.daily_usage_usd,
      limit: group.daily_limit_usd,
      reset: subscription.daily_window_start ? formatDailyUsageWindow(subscription) : null
    },
    {
      key: 'weekly' as const,
      label: t('admin.subscriptions.weekly'),
      used: subscription.weekly_usage_usd,
      limit: group.weekly_limit_usd,
      reset: subscription.weekly_window_start ? formatResetTime(subscription.weekly_window_start, 'weekly') : null
    },
    {
      key: 'monthly' as const,
      label: t('admin.subscriptions.monthly'),
      used: subscription.monthly_usage_usd,
      limit: group.monthly_limit_usd,
      reset: subscription.monthly_window_start ? formatResetTime(subscription.monthly_window_start, 'monthly') : null
    }
  ]
  return windows
    .filter((window) => !!window.limit)
    .map((window) => {
      const used = window.used ?? 0
      const limit = window.limit as number
      return { ...window, used, limit, percent: Math.min((used / limit) * 100, 100) }
    })
}

const formatUsd = (value: number) => `$${value.toFixed(2)}`

const meterWidth = (percent: number) => `${percent > 0 ? Math.max(percent, 2) : 0}%`

// Tone follows how much of the window is used.
const meterClass = (percent: number) =>
  percent >= 90 ? 'ui-meter is-bad' : percent >= 70 ? 'ui-meter is-warn' : 'ui-meter is-ok'

const formatResetDuration = (parts: RemainingDurationParts): string => {
  if (parts.days > 0) {
    return t('admin.subscriptions.resetInDaysHours', { days: parts.days, hours: parts.hours })
  }

  if (parts.hours > 0) {
    return t('admin.subscriptions.resetInHoursMinutes', { hours: parts.hours, minutes: parts.minutes })
  }

  return t('admin.subscriptions.resetInMinutes', { minutes: parts.minutes })
}

const formatQuotaEndDuration = (parts: RemainingDurationParts): string => {
  if (parts.days > 0) {
    return t('admin.subscriptions.quotaEndsInDaysHours', { days: parts.days, hours: parts.hours })
  }

  if (parts.hours > 0) {
    return t('admin.subscriptions.quotaEndsInHoursMinutes', { hours: parts.hours, minutes: parts.minutes })
  }

  return t('admin.subscriptions.quotaEndsInMinutes', { minutes: parts.minutes })
}

const formatDailyUsageWindow = (subscription: UserSubscription): string => {
  if (isOneTimeDailyQuota(subscription) && subscription.expires_at) {
    const parts = getRemainingDurationParts(subscription.expires_at)
    return parts ? formatQuotaEndDuration(parts) : t('admin.subscriptions.windowNotActive')
  }

  return formatResetTime(subscription.daily_window_start, 'daily')
}

// Format reset time based on window start and period type
const formatResetTime = (windowStart: string | null, period: 'daily' | 'weekly' | 'monthly'): string => {
  if (!windowStart) return t('admin.subscriptions.windowNotActive')

  const start = new Date(windowStart)
  const now = new Date()

  // Calculate reset time based on period
  let resetTime: Date
  switch (period) {
    case 'daily':
      resetTime = new Date(start.getTime() + 24 * 60 * 60 * 1000)
      break
    case 'weekly':
      resetTime = new Date(start.getTime() + 7 * 24 * 60 * 60 * 1000)
      break
    case 'monthly':
      resetTime = new Date(start.getTime() + 30 * 24 * 60 * 60 * 1000)
      break
  }

  const parts = getRemainingDurationParts(resetTime, now)

  return parts ? formatResetDuration(parts) : t('admin.subscriptions.windowNotActive')
}

// Handle click outside to close dropdowns
const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (!target.closest('[data-assign-user-search]')) showUserDropdown.value = false
  if (!target.closest('[data-filter-user-search]')) showFilterUserDropdown.value = false
}

onMounted(() => {
  refreshPendingOperations()
  loadUserColumnMode()
  loadSavedColumns()
  loadSubscriptions()
  loadGroups()
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  if (filterUserSearchTimeout) {
    clearTimeout(filterUserSearchTimeout)
  }
  if (userSearchTimeout) {
    clearTimeout(userSearchTimeout)
  }
})
</script>

<style scoped>
.subs-top {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

/* Keep the segmented status control inside narrow viewports. */
.ui-toolbar > .viz-seg {
  max-width: 100%;
  overflow-x: auto;
  scrollbar-width: none;
}

/* User search wraps the shared .ui-search so the result list can anchor to it. */
.subs-search {
  position: relative;
  display: flex;
  min-width: 12rem;
  max-width: 22rem;
  flex: 1 1 14rem;
}

.subs-search > .ui-search {
  max-width: none;
}

.subs-search-clear {
  display: inline-grid;
  place-items: center;
  border-radius: 999px;
  color: var(--operator-subtle-foreground);
}

.subs-search-clear:hover {
  color: var(--operator-foreground);
}

.subs-search-menu {
  position: absolute;
  top: calc(100% + 0.375rem);
  left: 0;
  z-index: 60;
  width: 100%;
  min-width: 16rem;
  max-height: 15rem;
  overflow-y: auto;
  padding: 0.25rem;
  border: 1px solid var(--operator-popup-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-popup-surface);
  box-shadow: var(--operator-popup-shadow);
  transform-origin: top left;
}

.subs-search-note {
  padding: 0.5rem 0.625rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.subs-menu-label {
  padding: 0.5rem 0.625rem 0.25rem;
  color: var(--operator-subtle-foreground);
  font-size: 0.6875rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.subs-menu-divider {
  height: 1px;
  margin: 0.25rem 0;
  background: var(--operator-border);
}

.subs-check {
  color: var(--operator-accent-text);
}

.dropdown-item.is-danger,
.subs-danger-text {
  color: var(--operator-destructive);
}

.dropdown-item:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

/* Unconfirmed bulk actions */
.subs-notice {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.75rem 0.875rem;
  border: 1px solid rgb(var(--operator-warning-rgb) / 0.35);
  border-radius: var(--operator-radius-lg);
  background: rgb(var(--operator-warning-rgb) / 0.07);
  font-size: 0.875rem;
}

.subs-notice-head {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--operator-warning);
}

.subs-notice-head h3 {
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 600;
}

.subs-notice-error {
  color: var(--operator-destructive);
}

.subs-notice-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.subs-notice-row b {
  margin-right: 0.5rem;
  color: var(--operator-foreground);
  font-weight: 500;
}

/* Selection bar */
.subs-bulkbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.625rem 0.5rem 0.875rem;
  border: 1px solid rgb(var(--operator-accent-rgb) / 0.35);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-accent-soft);
}

.subs-bulkbar > .ui-grow {
  flex: 1 1 auto;
}

.subs-bulkbar-count {
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

/* Cells */
.subs-avatar {
  display: inline-grid;
  width: 1.75rem;
  height: 1.75rem;
  flex: none;
  place-items: center;
  border-radius: 999px;
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
  font-size: 0.75rem;
  font-weight: 600;
}

.subs-avatar.is-lg {
  width: 2.25rem;
  height: 2.25rem;
  font-size: 0.875rem;
}

.subs-user-link {
  overflow: hidden;
  border-radius: 4px;
  color: var(--operator-foreground);
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.subs-user-link:hover {
  color: var(--operator-accent-text);
  text-decoration: underline;
  text-underline-offset: 3px;
}

.subs-user-link:focus-visible {
  outline: 2px solid rgb(var(--operator-accent-rgb) / 0.6);
  outline-offset: 2px;
}

.subs-usage {
  display: flex;
  width: 15rem;
  max-width: 100%;
  flex-direction: column;
  gap: 0.375rem;
}

.subs-usage-row {
  display: grid;
  grid-template-columns: 3.5rem minmax(4rem, 1fr) auto;
  align-items: center;
  gap: 0.5rem;
}

.subs-usage-row .ui-meter,
.subs-window .ui-meter {
  min-width: 0;
}

.subs-usage-label {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.subs-usage-amount,
.subs-window-amount {
  color: var(--operator-foreground);
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.subs-usage-amount > span,
.subs-window-amount > span {
  color: var(--operator-subtle-foreground);
}

.subs-unlimited {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  color: var(--operator-success);
  font-size: 0.8125rem;
  font-weight: 500;
}

.subs-unlimited > span {
  font-size: 1.125rem;
  line-height: 1;
}

.subs-date {
  color: var(--operator-foreground);
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.subs-date.is-soon {
  color: var(--operator-warning);
}

.subs-status {
  display: inline-flex;
  align-items: center;
  gap: 0.4375rem;
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  white-space: nowrap;
}

.subs-row-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.125rem;
}

/* Drawer */
.subs-windows {
  display: flex;
  flex-direction: column;
  gap: 0.875rem;
}

.subs-window {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.subs-window-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.75rem;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.subs-window-amount {
  font-size: 0.8125rem;
}

.subs-window-reset {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}
</style>
