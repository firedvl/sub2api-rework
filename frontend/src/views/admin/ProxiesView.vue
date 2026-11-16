<template>
  <AppLayout>
    <div class="ui-page ui-stagger proxies-page">
      <div class="ui-toolbar" role="toolbar" :aria-label="t('admin.adminUi.filters')">
        <label class="ui-search">
          <Icon name="search" size="sm" />
          <input
            v-model="searchQuery"
            type="search"
            :placeholder="t('admin.proxies.searchProxies')"
            :aria-label="t('admin.proxies.searchProxies')"
            @input="handleSearch"
          />
        </label>

        <div class="viz-seg" role="group" :aria-label="t('admin.proxies.columns.status')">
          <button
            v-for="option in statusSegments"
            :key="option.value"
            type="button"
            :aria-pressed="filters.status === option.value"
            :data-test="`proxy-status-${option.value || 'all'}`"
            @click="setStatusFilter(option.value)"
          >
            {{ option.label }}
            <span v-if="option.count !== null" class="proxies-seg-count">{{ option.count }}</span>
          </button>
        </div>

        <ToolbarPopover
          :label="t('admin.adminUi.filters')"
          icon="filter"
          align="left"
          :count="activeFilterCount"
          test-id="proxy-filters"
        >
          <div class="proxies-filter-panel">
            <label class="input-label">{{ t('admin.proxies.filters.protocol') }}</label>
            <Select
              v-model="filters.protocol"
              :options="protocolOptions"
              :placeholder="t('admin.proxies.allProtocols')"
              @change="handleFilterChange"
            />
            <button
              v-if="activeFilterCount"
              type="button"
              class="btn btn-ghost btn-sm proxies-filter-clear"
              @click="clearFilters"
            >
              {{ t('admin.adminUi.clearFilters') }}
            </button>
          </div>
        </ToolbarPopover>

        <span v-if="selectedCount > 0" class="ui-chip is-active proxies-selection">
          {{ t('admin.adminUi.proxies.selected', { count: selectedCount }) }}
          <button
            type="button"
            class="proxies-selection-clear"
            :aria-label="t('admin.adminUi.proxies.clearSelection')"
            :title="t('admin.adminUi.proxies.clearSelection')"
            @click="clearSelectedProxies"
          >
            <Icon name="x" size="xs" />
          </button>
        </span>

        <span class="ui-grow" />

        <button
          type="button"
          class="app-icon-button"
          :disabled="loading"
          :title="t('common.refresh')"
          :aria-label="t('common.refresh')"
          @click="reloadAll"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        </button>

        <ToolbarPopover
          :label="t('admin.adminUi.moreActions')"
          icon="more"
          icon-only
          menu
          width="15rem"
          test-id="proxy-more"
        >
          <button
            type="button"
            role="menuitem"
            class="dropdown-item"
            :disabled="batchTesting || loading"
            @click="handleBatchTest"
          >
            <Icon name="play" size="sm" :class="batchTesting ? 'animate-pulse' : ''" />
            {{ selectedCount > 0 ? t('admin.adminUi.proxies.testSelected', { count: selectedCount }) : t('admin.adminUi.proxies.testAll') }}
          </button>
          <button
            type="button"
            role="menuitem"
            class="dropdown-item"
            :disabled="batchQualityChecking || loading"
            @click="handleBatchQualityCheck"
          >
            <Icon name="shield" size="sm" :class="batchQualityChecking ? 'animate-pulse' : ''" />
            {{ selectedCount > 0 ? t('admin.adminUi.proxies.qualitySelected', { count: selectedCount }) : t('admin.adminUi.proxies.qualityAll') }}
          </button>
          <div class="proxies-menu-sep" role="separator" />
          <button type="button" role="menuitem" class="dropdown-item" @click="showImportData = true">
            <Icon name="upload" size="sm" />
            {{ t('admin.proxies.dataImport') }}
          </button>
          <button type="button" role="menuitem" class="dropdown-item" @click="showExportDataDialog = true">
            <Icon name="download" size="sm" />
            {{ selectedCount > 0 ? t('admin.proxies.dataExportSelected') : t('admin.proxies.dataExport') }}
          </button>
          <div class="proxies-menu-sep" role="separator" />
          <button
            type="button"
            role="menuitem"
            class="dropdown-item proxies-menu-danger"
            :disabled="selectedCount === 0"
            @click="openBatchDelete"
          >
            <Icon name="trash" size="sm" />
            {{ t('admin.adminUi.proxies.deleteSelected', { count: selectedCount }) }}
          </button>
        </ToolbarPopover>

        <button type="button" class="btn btn-primary" @click="showCreateModal = true">
          <Icon name="plus" size="sm" />
          <span>{{ t('admin.proxies.createProxy') }}</span>
        </button>
      </div>

      <section class="ui-card proxies-card">
        <div ref="proxyTableRef" class="operator-proxy-table">
          <DataTable
            :columns="columns"
            :data="proxies"
            :loading="loading"
            :expandable-actions="false"
            :server-side-sort="true"
            :clickable-rows="true"
            row-key="id"
            default-sort-key="id"
            default-sort-order="desc"
            @sort="handleSort"
            @row-click="openDetail"
          >
            <template #header-select>
              <input
                type="checkbox"
                class="operator-checkbox h-4 w-4 cursor-pointer rounded border-gray-300 focus:ring-gray-500"
                :checked="allVisibleSelected"
                :aria-label="t('common.selectAll')"
                @click.stop
                @change="toggleSelectAllVisible($event)"
              />
            </template>

            <template #cell-select="{ row }">
              <input
                type="checkbox"
                class="operator-checkbox h-4 w-4 cursor-pointer rounded border-gray-300 focus:ring-gray-500"
                :checked="selectedProxyIds.has(row.id)"
                :aria-label="row.name"
                @click.stop
                @change="toggleSelectRow(row.id, $event)"
              />
            </template>

            <template #cell-name="{ row }">
              <button
                type="button"
                class="proxies-name"
                :title="row.name"
                :aria-label="`${t('admin.adminUi.details')}: ${row.name}`"
                @click.stop="openDetail(row)"
              >
                <img
                  v-if="row.country_code"
                  class="proxies-flag"
                  :src="flagUrl(row.country_code)"
                  :alt="row.country || row.country_code"
                />
                <span v-else class="proxies-flag is-empty" aria-hidden="true">
                  <Icon name="globe" size="xs" />
                </span>
                <span class="min-w-0">
                  <span class="ui-cell-main">{{ row.name }}</span>
                  <span class="ui-cell-sub">{{ formatLocation(row) || row.ip_address || '—' }}</span>
                </span>
              </button>
            </template>

            <template #cell-protocol="{ row }">
              <div class="proxies-endpoint">
                <span v-if="row.protocol" class="badge badge-gray">{{ row.protocol.toUpperCase() }}</span>
                <code class="proxies-host" :title="`${row.host}:${row.port}`">{{ row.host }}:{{ row.port }}</code>
                <Icon
                  v-if="row.username || row.password"
                  name="lock"
                  size="xs"
                  class="proxies-auth-icon"
                  :title="t('admin.proxies.columns.auth')"
                />
                <div class="relative">
                  <button
                    type="button"
                    class="operator-proxy-inline-action"
                    :title="t('admin.proxies.copyProxyUrl')"
                    :aria-label="`${t('admin.proxies.copyProxyUrl')} ${row.name}`"
                    @click.stop="copyProxyUrl(row)"
                    @contextmenu.prevent.stop="toggleCopyMenu(row.id)"
                  >
                    <Icon name="copy" size="sm" aria-hidden="true" />
                  </button>
                  <div
                    v-if="copyMenuProxyId === row.id"
                    class="operator-menu menu-enter absolute left-0 top-full z-50 mt-1 w-auto min-w-[180px] py-1"
                  >
                    <button
                      v-for="fmt in getCopyFormats(row)"
                      :key="fmt.label"
                      type="button"
                      class="operator-menu-item flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs"
                      @click.stop="copyFormat(fmt.value)"
                    >
                      <span class="truncate font-mono text-gray-600 dark:text-gray-300">{{ fmt.label }}</span>
                    </button>
                  </div>
                </div>
              </div>
            </template>

            <template #cell-status="{ row }">
              <div class="proxies-health" :title="row.latency_message || undefined">
                <span :class="latencyDotClass(row)" />
                <span :class="latencyTextClass(row)">{{ latencyText(row) }}</span>
                <span
                  v-if="row.quality_grade"
                  class="proxies-grade"
                  :class="qualityGradeClass(row.quality_status)"
                  :title="row.quality_summary || qualityOverallLabel(row.quality_status)"
                >{{ row.quality_grade }}</span>
                <span v-if="row.status !== 'active'" :class="['badge', row.status === 'expired' ? 'badge-danger' : 'badge-gray']">
                  {{ proxyStatusLabel(row.status) }}
                </span>
              </div>
            </template>

            <template #cell-account_count="{ row, value }">
              <button
                v-if="(value || 0) > 0"
                type="button"
                class="proxies-count"
                :aria-label="t('admin.proxies.accountsTitle', { name: row.name })"
                :title="t('admin.proxies.accountsTitle', { name: row.name })"
                @click.stop="openAccountsModal(row)"
              >
                <Icon name="users" size="xs" />
                {{ value }}
              </button>
              <span v-else class="proxies-count is-zero">0</span>
            </template>

            <template #cell-expiry="{ row }">
              <span v-if="!row.expires_at && row.status !== 'expired'" class="proxies-muted">{{ t('admin.proxies.neverExpires') }}</span>
              <span
                v-else
                :class="expiryBadgeClass(row)"
                :title="row.expires_at ? formatDateTime(row.expires_at) : undefined"
              >{{ expiryLabel(row) }}</span>
            </template>

            <template #cell-actions="{ row }">
              <div class="operator-proxy-row-actions" @click.stop>
                <ProxyTestButton
                  :state="testState(row.id)"
                  :label="t('admin.proxies.testConnection')"
                  :aria-label="`${t('admin.proxies.testConnection')} ${row.name}`"
                  @test="handleTestConnection(row)"
                />
                <button
                  type="button"
                  class="operator-proxy-row-action"
                  :title="t('common.edit')"
                  :aria-label="`${t('common.edit')} ${row.name}`"
                  @click.stop="handleEdit(row)"
                >
                  <Icon name="edit" size="sm" aria-hidden="true" />
                </button>
                <ToolbarPopover
                  :label="`${t('admin.adminUi.moreActions')} ${row.name}`"
                  icon="more"
                  icon-only
                  menu
                  width="12rem"
                >
                  <button
                    type="button"
                    role="menuitem"
                    class="dropdown-item"
                    :disabled="qualityCheckingProxyIds.has(row.id)"
                    @click="handleQualityCheck(row)"
                  >
                    <Icon name="shield" size="sm" :class="qualityCheckingProxyIds.has(row.id) ? 'animate-pulse' : ''" />
                    {{ t('admin.proxies.qualityCheck') }}
                  </button>
                  <button type="button" role="menuitem" class="dropdown-item" @click="copyProxyUrl(row)">
                    <Icon name="copy" size="sm" />
                    {{ t('admin.proxies.copyProxyUrl') }}
                  </button>
                  <button
                    type="button"
                    role="menuitem"
                    class="dropdown-item proxies-menu-danger"
                    @click="handleDelete(row)"
                  >
                    <Icon name="trash" size="sm" />
                    {{ t('common.delete') }}
                  </button>
                </ToolbarPopover>
              </div>
            </template>

            <template #empty>
              <div class="ui-empty">
                <span class="ui-empty-icon"><Icon name="globe" size="md" /></span>
                <b>{{ hasActiveQuery ? t('admin.adminUi.noMatches') : t('admin.proxies.noProxiesYet') }}</b>
                <button v-if="hasActiveQuery" type="button" class="btn btn-secondary btn-sm" @click="clearAllQuery">
                  {{ t('admin.adminUi.clearFilters') }}
                </button>
                <button v-else type="button" class="btn btn-primary btn-sm" @click="showCreateModal = true">
                  <Icon name="plus" size="sm" />
                  {{ t('admin.proxies.createProxy') }}
                </button>
              </div>
            </template>
          </DataTable>
        </div>

        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </section>
    </div>

    <!-- Create Proxy Modal -->
    <BaseDialog
      :show="showCreateModal"
      :title="t('admin.proxies.createProxy')"
      width="normal"
      @close="closeCreateModal"
    >
      <!-- Tab Switch -->
      <div
        class="mb-6 flex items-center justify-between gap-3 border-b border-gray-200 dark:border-dark-600"
      >
        <div class="flex min-w-0 shrink-0">
          <button
            type="button"
            @click="createMode = 'standard'"
            :class="[
              '-mb-px border-b-2 px-4 py-2 text-sm font-medium transition-colors',
              createMode === 'standard'
                ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
            ]"
          >
            <Icon name="plus" size="sm" class="mr-1.5 inline" />
            {{ t('admin.proxies.standardAdd') }}
          </button>
          <button
            type="button"
            @click="createMode = 'batch'"
            :class="[
              '-mb-px border-b-2 px-4 py-2 text-sm font-medium transition-colors',
              createMode === 'batch'
                ? 'border-primary-500 text-primary-600 dark:text-primary-400'
                : 'border-transparent text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300'
            ]"
          >
            <svg
              class="mr-1.5 inline h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="1.5"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 010 3.75H5.625a1.875 1.875 0 010-3.75z"
              />
            </svg>
            {{ t('admin.proxies.batchAdd') }}
          </button>
        </div>
        <ProxyAdBanner />
      </div>

      <!-- Standard Add Form -->
      <form
        v-if="createMode === 'standard'"
        id="create-proxy-form"
        @submit.prevent="handleCreateProxy"
        class="space-y-5"
      >
        <div>
          <label class="input-label">{{ t('admin.proxies.name') }}</label>
          <input
            v-model="createForm.name"
            type="text"
            required
            class="input"
            :placeholder="t('admin.proxies.enterProxyName')"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.protocol') }}</label>
          <Select v-model="createForm.protocol" :options="protocolSelectOptions" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="input-label">{{ t('admin.proxies.host') }}</label>
            <input
              v-model="createForm.host"
              type="text"
              required
              :placeholder="t('admin.proxies.form.hostPlaceholder')"
              class="input"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.proxies.port') }}</label>
            <input
              v-model.number="createForm.port"
              type="number"
              required
              min="1"
              max="65535"
              :placeholder="t('admin.proxies.form.portPlaceholder')"
              class="input"
            />
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.username') }}</label>
          <input
            v-model="createForm.username"
            type="text"
            class="input"
            :placeholder="t('admin.proxies.optionalAuth')"
          />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.password') }}</label>
          <div class="relative">
            <input
              v-model="createForm.password"
              :type="createPasswordVisible ? 'text' : 'password'"
              class="input pr-10"
              :placeholder="t('admin.proxies.optionalAuth')"
            />
            <button
              type="button"
              class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              @click="createPasswordVisible = !createPasswordVisible"
            >
              <Icon :name="createPasswordVisible ? 'eyeOff' : 'eye'" size="md" />
            </button>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.expiresAt') }}</label>
          <div class="mb-2 flex flex-wrap gap-2">
            <button
              v-for="d in EXPIRY_PRESETS"
              :key="d"
              type="button"
              class="btn btn-sm"
              :class="createForm.expires_at === addDaysToBase('', d) ? 'btn-primary' : 'btn-secondary'"
              @click="createExpiresDays = d"
            >
              {{ t('admin.proxies.nDays', { days: d }) }}
            </button>
          </div>
          <input
            v-model.number="createExpiresDays"
            type="number"
            min="0"
            class="input mb-2"
            :placeholder="t('admin.proxies.expiryDaysPlaceholder')"
          />
          <input v-model="createForm.expires_at" type="date" max="9999-12-31" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.fallbackMode') }}</label>
          <Select v-model="createForm.fallback_mode" :options="[
            { label: t('admin.proxies.fallbackNone'), value: 'none' },
            { label: t('admin.proxies.fallbackProxy'), value: 'proxy' },
            { label: t('admin.proxies.fallbackDirect'), value: 'direct' },
          ]" />
        </div>
        <div v-if="createForm.fallback_mode === 'proxy'">
          <label class="input-label">{{ t('admin.proxies.backupProxy') }}</label>
          <Select v-model="createForm.backup_proxy_id" :options="backupProxyOptions()" />
        </div>

      </form>

      <!-- Batch Add Form -->
      <div v-else class="space-y-5">
        <div>
          <label class="input-label">{{ t('admin.proxies.batchInput') }}</label>
          <textarea
            v-model="batchInput"
            rows="10"
            class="input font-mono text-sm"
            :placeholder="t('admin.proxies.batchInputPlaceholder')"
            @input="parseBatchInput"
          ></textarea>
          <p class="input-hint mt-2">
            {{ t('admin.proxies.batchInputHint') }}
          </p>
        </div>

        <!-- Parse Result -->
        <div v-if="batchParseResult.total > 0" class="rounded-lg bg-gray-50 p-4 dark:bg-dark-700">
            <div class="flex items-center gap-4 text-sm">
              <div class="flex items-center gap-1.5">
              <Icon name="checkCircle" size="sm" :stroke-width="2" class="text-primary-500" />
              <span class="text-gray-700 dark:text-gray-300">
                {{ t('admin.proxies.parsedCount', { count: batchParseResult.valid }) }}
              </span>
            </div>
            <div v-if="batchParseResult.invalid > 0" class="flex items-center gap-1.5">
              <Icon
                name="exclamationCircle"
                size="sm"
                :stroke-width="2"
                class="text-amber-500"
              />
              <span class="text-amber-600 dark:text-amber-400">
                {{ t('admin.proxies.invalidCount', { count: batchParseResult.invalid }) }}
              </span>
            </div>
            <div v-if="batchParseResult.duplicate > 0" class="flex items-center gap-1.5">
              <svg
                class="h-4 w-4 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                stroke-width="2"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  d="M15.75 17.25v3.375c0 .621-.504 1.125-1.125 1.125h-9.75a1.125 1.125 0 01-1.125-1.125V7.875c0-.621.504-1.125 1.125-1.125H6.75a9.06 9.06 0 011.5.124m7.5 10.376h3.375c.621 0 1.125-.504 1.125-1.125V11.25c0-4.46-3.243-8.161-7.5-8.876a9.06 9.06 0 00-1.5-.124H9.375c-.621 0-1.125.504-1.125 1.125v3.5m7.5 10.375H9.375a1.125 1.125 0 01-1.125-1.125v-9.25m12 6.625v-1.875a3.375 3.375 0 00-3.375-3.375h-1.5a1.125 1.125 0 01-1.125-1.125v-1.5a3.375 3.375 0 00-3.375-3.375H9.75"
                />
              </svg>
              <span class="text-gray-500 dark:text-gray-400">
                {{ t('admin.proxies.duplicateCount', { count: batchParseResult.duplicate }) }}
              </span>
            </div>
          </div>
        </div>

      </div>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button @click="closeCreateModal" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            v-if="createMode === 'standard'"
            type="submit"
            form="create-proxy-form"
            :disabled="submitting"
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
            {{ submitting ? t('admin.proxies.creating') : t('common.create') }}
          </button>
          <button
            v-else
            @click="handleBatchCreate"
            type="button"
            :disabled="submitting || batchParseResult.valid === 0"
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
            {{
              submitting
                ? t('admin.proxies.importing')
                : t('admin.proxies.importProxies', { count: batchParseResult.valid })
            }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Edit Proxy Modal -->
    <BaseDialog
      :show="showEditModal"
      :title="t('admin.proxies.editProxy')"
      width="normal"
      @close="closeEditModal"
    >
      <form
        v-if="editingProxy"
        id="edit-proxy-form"
        @submit.prevent="handleUpdateProxy"
        class="space-y-5"
      >
        <div>
          <label class="input-label">{{ t('admin.proxies.name') }}</label>
          <input v-model="editForm.name" type="text" required class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.protocol') }}</label>
          <Select v-model="editForm.protocol" :options="protocolSelectOptions" />
        </div>
        <div class="grid grid-cols-2 gap-4">
          <div>
            <label class="input-label">{{ t('admin.proxies.host') }}</label>
            <input v-model="editForm.host" type="text" required class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.proxies.port') }}</label>
            <input
              v-model.number="editForm.port"
              type="number"
              required
              min="1"
              max="65535"
              class="input"
            />
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.username') }}</label>
          <input v-model="editForm.username" type="text" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.password') }}</label>
          <div class="relative">
            <input
              v-model="editForm.password"
              :type="editPasswordVisible ? 'text' : 'password'"
              :placeholder="t('admin.proxies.leaveEmptyToKeep')"
              class="input pr-10"
              @input="editPasswordDirty = true"
            />
            <button
              type="button"
              class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              @click="editPasswordVisible = !editPasswordVisible"
            >
              <Icon :name="editPasswordVisible ? 'eyeOff' : 'eye'" size="md" />
            </button>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.status') }}</label>
          <Select v-model="editForm.status" :options="editStatusOptions" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.expiresAt') }}</label>
          <div class="mb-2 flex flex-wrap gap-2">
            <button
              v-for="d in EXPIRY_PRESETS"
              :key="d"
              type="button"
              class="btn btn-sm"
              :class="editForm.expires_at === addDaysToBase(editBaseDate, d) ? 'btn-primary' : 'btn-secondary'"
              @click="editExpiresDays = d"
            >
              {{ t('admin.proxies.nDays', { days: d }) }}
            </button>
          </div>
          <input
            v-model.number="editExpiresDays"
            type="number"
            min="0"
            class="input mb-2"
            :placeholder="t('admin.proxies.expiryDaysPlaceholder')"
          />
          <input v-model="editForm.expires_at" type="date" max="9999-12-31" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.proxies.fallbackMode') }}</label>
          <Select v-model="editForm.fallback_mode" :options="[
            { label: t('admin.proxies.fallbackNone'), value: 'none' },
            { label: t('admin.proxies.fallbackProxy'), value: 'proxy' },
            { label: t('admin.proxies.fallbackDirect'), value: 'direct' },
          ]" />
        </div>
        <div v-if="editForm.fallback_mode === 'proxy'">
          <label class="input-label">{{ t('admin.proxies.backupProxy') }}</label>
          <Select v-model="editForm.backup_proxy_id" :options="backupProxyOptions(editingProxy?.id)" />
        </div>

      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button @click="closeEditModal" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            v-if="editingProxy"
            type="submit"
            form="edit-proxy-form"
            :disabled="submitting"
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
            {{ submitting ? t('admin.proxies.updating') : t('common.update') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.proxies.deleteProxy')"
      :message="t('admin.proxies.deleteConfirm', { name: deletingProxy?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- Batch Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showBatchDeleteDialog"
      :title="t('admin.proxies.batchDelete')"
      :message="t('admin.proxies.batchDeleteConfirm', { count: selectedCount })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmBatchDelete"
      @cancel="showBatchDeleteDialog = false"
    />
    <ConfirmDialog
      :show="showExportDataDialog"
      :title="t('admin.proxies.dataExport')"
      :message="t('admin.proxies.dataExportConfirmMessage')"
      :confirm-text="t('admin.proxies.dataExportConfirm')"
      :cancel-text="t('common.cancel')"
      @confirm="handleExportData"
      @cancel="showExportDataDialog = false"
    />

    <ImportDataModal
      :show="showImportData"
      @close="showImportData = false"
      @imported="handleDataImported"
    />

    <BaseDialog
      :show="showQualityReportDialog"
      :title="t('admin.proxies.qualityReportTitle')"
      width="normal"
      @close="closeQualityReportDialog"
    >
      <div v-if="qualityReport" class="space-y-4">
        <div class="rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-700">
          <div class="flex items-center justify-between gap-4">
            <div>
              <div class="text-sm text-gray-500 dark:text-gray-400">
                {{ qualityReportProxy?.name || '-' }}
              </div>
              <div class="mt-1 text-sm text-gray-700 dark:text-gray-200">
                {{ qualityReport.summary }}
              </div>
            </div>
            <div class="text-right">
              <div class="text-2xl font-semibold text-gray-900 dark:text-white">
                {{ qualityReport.score }}
              </div>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.proxies.qualityGrade', { grade: qualityReport.grade }) }}
              </div>
            </div>
          </div>
          <div class="mt-3 grid grid-cols-2 gap-2 text-xs text-gray-600 dark:text-gray-300">
            <div>{{ t('admin.proxies.qualityExitIP') }}: {{ qualityReport.exit_ip || '-' }}</div>
            <div>{{ t('admin.proxies.qualityCountry') }}: {{ qualityReport.country || '-' }}</div>
            <div>
              {{ t('admin.proxies.qualityBaseLatency') }}:
              {{ typeof qualityReport.base_latency_ms === 'number' ? `${qualityReport.base_latency_ms}ms` : '-' }}
            </div>
            <div>{{ t('admin.proxies.qualityCheckedAt') }}: {{ new Date(qualityReport.checked_at * 1000).toLocaleString() }}</div>
          </div>
        </div>

        <div class="max-h-80 overflow-auto rounded-lg border border-gray-200 dark:border-dark-600">
          <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
              <tr>
                <th class="whitespace-nowrap px-3 py-2 text-left">{{ t('admin.proxies.qualityTableTarget') }}</th>
                <th class="whitespace-nowrap px-3 py-2 text-left">{{ t('admin.proxies.qualityTableStatus') }}</th>
                <th class="whitespace-nowrap px-3 py-2 text-left">HTTP</th>
                <th class="whitespace-nowrap px-3 py-2 text-left">{{ t('admin.proxies.qualityTableLatency') }}</th>
                <th class="px-3 py-2 text-left">{{ t('admin.proxies.qualityTableMessage') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-for="item in qualityReport.items" :key="item.target">
                <td class="whitespace-nowrap px-3 py-2 text-gray-900 dark:text-white">{{ qualityTargetLabel(item.target) }}</td>
                <td class="whitespace-nowrap px-3 py-2">
                  <span class="badge whitespace-nowrap" :class="qualityStatusClass(item.status)">{{ qualityStatusLabel(item.status) }}</span>
                </td>
                <td class="whitespace-nowrap px-3 py-2 text-gray-600 dark:text-gray-300">{{ item.http_status ?? '-' }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-gray-600 dark:text-gray-300">
                  {{ typeof item.latency_ms === 'number' ? `${item.latency_ms}ms` : '-' }}
                </td>
                <td class="px-3 py-2 text-gray-600 dark:text-gray-300">
                  <span>{{ item.message || '-' }}</span>
                  <span v-if="item.cf_ray" class="ml-1 text-xs text-gray-400">(cf-ray: {{ item.cf_ray }})</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <button @click="closeQualityReportDialog" class="btn btn-secondary">
            {{ t('common.close') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Proxy Accounts Dialog -->
    <BaseDialog
      :show="showAccountsModal"
      :title="t('admin.proxies.accountsTitle', { name: accountsProxy?.name || '' })"
      width="normal"
      @close="closeAccountsModal"
    >
      <div v-if="accountsLoading" class="flex items-center justify-center py-8 text-sm text-gray-500">
        <Icon name="refresh" size="md" class="mr-2 animate-spin" />
        {{ t('common.loading') }}
      </div>
      <div v-else-if="proxyAccounts.length === 0" class="py-6 text-center text-sm text-gray-500">
        {{ t('admin.proxies.accountsEmpty') }}
      </div>
      <div v-else class="max-h-80 overflow-auto">
        <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
          <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
            <tr>
              <th class="px-4 py-2 text-left">{{ t('admin.proxies.accountName') }}</th>
              <th class="px-4 py-2 text-left">{{ t('admin.accounts.columns.platformType') }}</th>
              <th class="px-4 py-2 text-left">{{ t('admin.proxies.accountNotes') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
            <tr v-for="account in proxyAccounts" :key="account.id">
              <td class="px-4 py-2 font-medium text-gray-900 dark:text-white">{{ account.name }}</td>
              <td class="px-4 py-2">
                <PlatformTypeBadge :platform="account.platform" :type="account.type" />
              </td>
              <td class="px-4 py-2 text-gray-600 dark:text-gray-300">
                {{ account.notes || '-' }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <template #footer>
        <div class="flex justify-end">
          <button @click="closeAccountsModal" class="btn btn-secondary">
            {{ t('common.close') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <SideDrawer
      :open="!!detailProxy"
      :title="detailProxy?.name || ''"
      :subtitle="detailProxy ? `${detailProxy.protocol}://${detailProxy.host}:${detailProxy.port}` : ''"
      width="32rem"
      @close="closeDetail"
    >
      <template #leading>
        <img
          v-if="detailProxy?.country_code"
          class="proxies-drawer-flag"
          :src="flagUrl(detailProxy.country_code)"
          :alt="detailProxy.country || detailProxy.country_code"
        />
        <span v-else class="proxies-drawer-flag is-empty" aria-hidden="true"><Icon name="globe" size="md" /></span>
      </template>
      <template #badge>
        <span v-if="detailProxy" :class="['badge', detailProxy.status === 'active' ? 'badge-success' : detailProxy.status === 'expired' ? 'badge-danger' : 'badge-gray']">
          {{ proxyStatusLabel(detailProxy.status) }}
        </span>
      </template>

      <template v-if="detailProxy">
        <div class="proxies-stats">
          <div class="proxies-stat">
            <span class="proxies-stat-label">{{ t('admin.proxies.columns.latency') }}</span>
            <span class="proxies-stat-value">
              <span :class="latencyDotClass(detailProxy)" />
              <span :class="latencyTextClass(detailProxy)">{{ latencyText(detailProxy) }}</span>
            </span>
          </div>
          <div class="proxies-stat">
            <span class="proxies-stat-label">{{ t('admin.adminUi.proxies.quality') }}</span>
            <span class="proxies-stat-value">
              <template v-if="typeof detailProxy.quality_checked === 'number'">
                <span class="proxies-grade" :class="qualityGradeClass(detailProxy.quality_status)">{{ detailProxy.quality_grade || '-' }}</span>
                <span>{{ detailProxy.quality_score ?? '-' }}</span>
              </template>
              <span v-else class="proxies-muted">—</span>
            </span>
          </div>
          <div class="proxies-stat">
            <span class="proxies-stat-label">{{ t('admin.proxies.columns.accounts') }}</span>
            <span class="proxies-stat-value">{{ detailProxy.account_count || 0 }}</span>
          </div>
        </div>

        <p v-if="detailProxy.latency_status === 'failed' && detailProxy.latency_message" class="proxies-drawer-error">
          <Icon name="exclamationTriangle" size="sm" />
          <span>{{ detailProxy.latency_message }}</span>
        </p>

        <section>
          <h3 class="ui-section-label">{{ t('admin.adminUi.proxies.connection') }}</h3>
          <dl class="ui-dl">
            <dt>{{ t('admin.proxies.protocol') }}</dt>
            <dd>{{ detailProxy.protocol.toUpperCase() }}</dd>
            <dt>{{ t('admin.proxies.host') }}</dt>
            <dd class="font-mono text-xs">{{ detailProxy.host }}</dd>
            <dt>{{ t('admin.proxies.port') }}</dt>
            <dd class="font-mono text-xs">{{ detailProxy.port }}</dd>
            <dt>{{ t('admin.proxies.columns.auth') }}</dt>
            <dd>
              <span v-if="!detailProxy.username && !detailProxy.password" class="proxies-muted">—</span>
              <span v-else class="proxies-auth">
                <span v-if="detailProxy.username">{{ detailProxy.username }}</span>
                <span v-if="detailProxy.password" class="font-mono">
                  {{ visiblePasswordIds.has(detailProxy.id) ? detailProxy.password : '••••••' }}
                </span>
                <button
                  v-if="detailProxy.password"
                  type="button"
                  class="operator-proxy-inline-action"
                  :title="visiblePasswordIds.has(detailProxy.id) ? t('admin.adminUi.proxies.hidePassword') : t('admin.adminUi.proxies.showPassword')"
                  :aria-label="visiblePasswordIds.has(detailProxy.id) ? t('admin.adminUi.proxies.hidePassword') : t('admin.adminUi.proxies.showPassword')"
                  @click="togglePasswordVisible(detailProxy.id)"
                >
                  <Icon :name="visiblePasswordIds.has(detailProxy.id) ? 'eyeOff' : 'eye'" size="sm" aria-hidden="true" />
                </button>
              </span>
            </dd>
            <dt>URL</dt>
            <dd class="proxies-url">
              <div class="relative">
                <button
                  type="button"
                  class="btn btn-ghost btn-sm"
                  @click.stop="copyProxyUrl(detailProxy)"
                  @contextmenu.prevent.stop="toggleCopyMenu(detailProxy.id)"
                >
                  <Icon name="copy" size="sm" />
                  {{ t('admin.proxies.copyProxyUrl') }}
                </button>
                <div
                  v-if="copyMenuProxyId === detailProxy.id"
                  class="operator-menu menu-enter absolute left-0 top-full z-50 mt-1 w-auto min-w-[180px] py-1"
                >
                  <button
                    v-for="fmt in getCopyFormats(detailProxy)"
                    :key="fmt.label"
                    type="button"
                    class="operator-menu-item flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs"
                    @click.stop="copyFormat(fmt.value)"
                  >
                    <span class="truncate font-mono text-gray-600 dark:text-gray-300">{{ fmt.label }}</span>
                  </button>
                </div>
              </div>
            </dd>
          </dl>
        </section>

        <section>
          <h3 class="ui-section-label">{{ t('admin.adminUi.proxies.exit') }}</h3>
          <dl class="ui-dl">
            <dt>{{ t('admin.proxies.qualityExitIP') }}</dt>
            <dd class="font-mono text-xs">{{ detailProxy.ip_address || '—' }}</dd>
            <dt>{{ t('admin.proxies.columns.location') }}</dt>
            <dd>{{ [detailProxy.country, detailProxy.region, detailProxy.city].filter(Boolean).join(' · ') || '—' }}</dd>
            <template v-if="detailProxy.latency_status !== 'failed' && detailProxy.latency_message">
              <dt>{{ t('admin.adminUi.proxies.lastResult') }}</dt>
              <dd>{{ detailProxy.latency_message }}</dd>
            </template>
          </dl>
        </section>

        <section v-if="typeof detailProxy.quality_checked === 'number'">
          <h3 class="ui-section-label">{{ t('admin.adminUi.proxies.quality') }}</h3>
          <dl class="ui-dl">
            <dt>{{ t('admin.proxies.status') }}</dt>
            <dd><span class="badge" :class="qualityOverallClass(detailProxy.quality_status)">{{ qualityOverallLabel(detailProxy.quality_status) }}</span></dd>
            <dt>{{ t('admin.proxies.qualityCheckedAt') }}</dt>
            <dd>{{ new Date(detailProxy.quality_checked * 1000).toLocaleString() }}</dd>
            <template v-if="detailProxy.quality_summary">
              <dt>{{ t('admin.adminUi.proxies.summary') }}</dt>
              <dd>{{ detailProxy.quality_summary }}</dd>
            </template>
          </dl>
        </section>

        <section>
          <h3 class="ui-section-label">{{ t('admin.adminUi.proxies.lifecycle') }}</h3>
          <dl class="ui-dl">
            <dt>{{ t('admin.proxies.expiresAt') }}</dt>
            <dd>
              <template v-if="detailProxy.expires_at">
                {{ formatDateTime(detailProxy.expires_at) }}
                <span :class="expiryBadgeClass(detailProxy)">{{ expiryLabel(detailProxy) }}</span>
              </template>
              <span v-else>{{ t('admin.proxies.neverExpires') }}</span>
            </dd>
            <dt>{{ t('admin.proxies.fallbackMode') }}</dt>
            <dd>{{ fallbackLabel(detailProxy) }}</dd>
            <dt>{{ t('admin.proxies.columns.createdAt') }}</dt>
            <dd>{{ formatDateTime(detailProxy.created_at) }}</dd>
            <dt>{{ t('admin.adminUi.proxies.updated') }}</dt>
            <dd>{{ formatDateTime(detailProxy.updated_at) }}</dd>
          </dl>
        </section>
      </template>

      <template #footer>
        <template v-if="detailProxy">
          <ProxyTestButton
            :state="testState(detailProxy.id)"
            :icon-only="false"
            :label="t('admin.proxies.testConnection')"
            @test="handleTestConnection(detailProxy)"
          />
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="qualityCheckingProxyIds.has(detailProxy.id)"
            @click="runFromDrawer(handleQualityCheck)"
          >
            <Icon name="shield" size="sm" />
            {{ t('admin.proxies.qualityCheck') }}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" @click="runFromDrawer(handleEdit)">
            <Icon name="edit" size="sm" />
            {{ t('common.edit') }}
          </button>
          <button
            v-if="(detailProxy.account_count || 0) > 0"
            type="button"
            class="btn btn-secondary btn-sm"
            @click="runFromDrawer(openAccountsModal)"
          >
            <Icon name="users" size="sm" />
            {{ t('admin.proxies.columns.accounts') }}
          </button>
          <span class="ui-grow flex-1" />
          <button
            type="button"
            class="btn btn-ghost btn-sm proxies-menu-danger"
            :disabled="(detailProxy.account_count || 0) > 0"
            :title="(detailProxy.account_count || 0) > 0 ? t('admin.proxies.deleteBlockedInUse') : undefined"
            @click="runFromDrawer(handleDelete)"
          >
            <Icon name="trash" size="sm" />
            {{ t('common.delete') }}
          </button>
        </template>
      </template>
    </SideDrawer>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Proxy, ProxyAccountSummary, ProxyProtocol, ProxyQualityCheckResult } from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import ToolbarPopover from '@/components/common/ToolbarPopover.vue'
import SideDrawer from '@/components/ui/SideDrawer.vue'
import ImportDataModal from '@/components/admin/proxy/ImportDataModal.vue'
import ProxyTestButton from '@/components/admin/proxy/ProxyTestButton.vue'
import Select from '@/components/common/Select.vue'
import ProxyAdBanner from '@/components/common/ProxyAdBanner.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useSwipeSelect } from '@/composables/useSwipeSelect'
import { useTableSelection } from '@/composables/useTableSelection'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatDateTime } from '@/utils/format'
import { proxyExpiryBadgeClass, proxyExpiryLabelKey } from '@/utils/proxyExpiry'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

// Created/updated time, auth details and exit geo live in the detail drawer.
const columns = computed<Column[]>(() => [
  { key: 'select', label: '', sortable: false, class: 'operator-proxy-select-column' },
  { key: 'name', label: t('admin.proxies.columns.name'), sortable: true, class: 'operator-proxy-name-column' },
  { key: 'protocol', label: t('admin.proxies.columns.address'), sortable: true, class: 'operator-proxy-endpoint-column' },
  { key: 'status', label: t('admin.adminUi.proxies.health'), sortable: true, class: 'operator-proxy-health-column' },
  { key: 'account_count', label: t('admin.proxies.columns.accounts'), sortable: true, class: 'operator-proxy-account-column' },
  { key: 'expiry', label: t('admin.proxies.columns.expiry'), sortable: true, class: 'operator-proxy-lifecycle-column' },
  { key: 'actions', label: t('admin.proxies.columns.actions'), sortable: false, class: 'operator-proxy-actions-column' }
])

// Filter options
const protocolOptions = computed(() => [
  { value: '', label: t('admin.proxies.allProtocols') },
  { value: 'http', label: 'HTTP' },
  { value: 'https', label: 'HTTPS' },
  { value: 'socks5', label: 'SOCKS5' },
  { value: 'socks5h', label: 'SOCKS5H' }
])

type ProxyStatusFilter = '' | Proxy['status']
const STATUS_FILTERS: Exclude<ProxyStatusFilter, ''>[] = ['active', 'inactive', 'expired']
const proxyStatusLabel = (status: string) => {
  if (status === 'active') return t('admin.accounts.status.active')
  if (status === 'inactive') return t('admin.accounts.status.inactive')
  if (status === 'expired') return t('admin.proxies.expired')
  return status
}

// Form options
const protocolSelectOptions = computed(() => [
  { value: 'http', label: t('admin.proxies.protocols.http') },
  { value: 'https', label: t('admin.proxies.protocols.https') },
  { value: 'socks5', label: t('admin.proxies.protocols.socks5') },
  { value: 'socks5h', label: t('admin.proxies.protocols.socks5h') }
])

const editStatusOptions = computed(() => [
  { value: 'active', label: t('admin.accounts.status.active') },
  { value: 'inactive', label: t('admin.accounts.status.inactive') }
])

const proxies = ref<Proxy[]>([])
const visiblePasswordIds = reactive(new Set<number>())
const copyMenuProxyId = ref<number | null>(null)
const loading = ref(false)
const searchQuery = ref('')
const filters = reactive({
  protocol: '',
  status: '' as ProxyStatusFilter
})
// Per-status totals for the status segment (same search/protocol scope as the list).
const statusCounts = reactive<Record<Exclude<ProxyStatusFilter, ''>, number | null>>({
  active: null,
  inactive: null,
  expired: null
})
const statusSegments = computed(() => {
  const known = STATUS_FILTERS.every((status) => statusCounts[status] !== null)
  return [
    {
      value: '' as ProxyStatusFilter,
      label: t('common.all'),
      count: known ? STATUS_FILTERS.reduce((sum, status) => sum + (statusCounts[status] ?? 0), 0) : null
    },
    ...STATUS_FILTERS.map((status) => ({ value: status, label: proxyStatusLabel(status), count: statusCounts[status] }))
  ]
})
const activeFilterCount = computed(() => (filters.protocol ? 1 : 0))
const hasActiveQuery = computed(() => Boolean(searchQuery.value || filters.protocol || filters.status))
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})
const sortState = reactive({
  sort_by: 'id',
  sort_order: 'desc' as 'asc' | 'desc'
})

const showCreateModal = ref(false)
const createPasswordVisible = ref(false)
const showEditModal = ref(false)
const editPasswordVisible = ref(false)
const editPasswordDirty = ref(false)
const showImportData = ref(false)
const showDeleteDialog = ref(false)
const showBatchDeleteDialog = ref(false)
const showExportDataDialog = ref(false)
const showAccountsModal = ref(false)
const submitting = ref(false)
const exportingData = ref(false)
const testingProxyIds = ref<Set<number>>(new Set())
const qualityCheckingProxyIds = ref<Set<number>>(new Set())
const batchTesting = ref(false)
const batchQualityChecking = ref(false)
const proxyTableRef = ref<HTMLElement | null>(null)
const {
  selectedSet: selectedProxyIds,
  selectedCount,
  allVisibleSelected,
  isSelected,
  select,
  deselect,
  clear: clearSelectedProxies,
  removeMany: removeSelectedProxies,
  toggleVisible,
  batchUpdate
} = useTableSelection<Proxy>({
  rows: proxies,
  getId: (proxy) => proxy.id
})
useSwipeSelect(proxyTableRef, {
  isSelected,
  select,
  deselect,
  batchUpdate
})
const accountsProxy = ref<Proxy | null>(null)
const proxyAccounts = ref<ProxyAccountSummary[]>([])
const accountsLoading = ref(false)
const editingProxy = ref<Proxy | null>(null)
const deletingProxy = ref<Proxy | null>(null)
const showQualityReportDialog = ref(false)
const qualityReportProxy = ref<Proxy | null>(null)
const qualityReport = ref<ProxyQualityCheckResult | null>(null)

// Batch import state
const createMode = ref<'standard' | 'batch'>('standard')
const batchInput = ref('')
const batchParseResult = reactive({
  total: 0,
  valid: 0,
  invalid: 0,
  duplicate: 0,
  proxies: [] as Array<{
    protocol: ProxyProtocol
    host: string
    port: number
    username: string
    password: string
  }>
})

const createForm = reactive({
  name: '',
  protocol: 'http' as ProxyProtocol,
  host: '',
  port: 8080,
  username: '',
  password: '',
  expires_at: '' as string,
  fallback_mode: 'none' as 'none' | 'proxy' | 'direct',
  backup_proxy_id: null as number | null,
  expiry_warn_days: 7 as number,
})

const editForm = reactive({
  name: '',
  protocol: 'http' as ProxyProtocol,
  host: '',
  port: 8080,
  username: '',
  password: '',
  status: 'active' as 'active' | 'inactive' | 'expired',
  expires_at: '' as string,
  fallback_mode: 'none' as 'none' | 'proxy' | 'direct',
  backup_proxy_id: null as number | null,
  expiry_warn_days: 7 as number,
})

const allProxiesForBackup = ref<Proxy[]>([])
const loadBackupProxyOptions = async () => {
  allProxiesForBackup.value = await adminAPI.proxies.getAllWithCount()
}
const backupProxyOptions = (excludeId?: number) =>
  allProxiesForBackup.value
    .filter(p => p.id !== excludeId)
    .map(p => ({ label: `${p.name} (${p.host}:${p.port})`, value: p.id }))

let abortController: AbortController | null = null

const isAbortError = (error: unknown) => {
  if (!error || typeof error !== 'object') return false
  const maybeError = error as { name?: string; code?: string }
  return maybeError.name === 'AbortError' || maybeError.code === 'ERR_CANCELED'
}

const toggleSelectRow = (id: number, event: Event) => {
  const target = event.target as HTMLInputElement
  if (target.checked) {
    select(id)
    return
  }
  deselect(id)
}

const toggleSelectAllVisible = (event: Event) => {
  const target = event.target as HTMLInputElement
  toggleVisible(target.checked)
}

const buildProxyQueryFilters = () => ({
  protocol: filters.protocol || undefined,
  status: (filters.status || undefined) as 'active' | 'inactive' | 'expired' | undefined,
  search: searchQuery.value || undefined,
  sort_by: sortState.sort_by,
  sort_order: sortState.sort_order
})

const loadProxies = async () => {
  if (abortController) {
    abortController.abort()
  }
  const currentAbortController = new AbortController()
  abortController = currentAbortController
  loading.value = true
  try {
    const response = await adminAPI.proxies.list(
      pagination.page,
      pagination.page_size,
      buildProxyQueryFilters(),
      { signal: currentAbortController.signal }
    )
    if (currentAbortController.signal.aborted || abortController !== currentAbortController) {
      return
    }
    proxies.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error) {
    if (isAbortError(error)) {
      return
    }
    appStore.showError(t('admin.proxies.failedToLoad'))
    console.error('Error loading proxies:', error)
  } finally {
    if (abortController === currentAbortController) {
      loading.value = false
      abortController = null
    }
  }
}

let countsRequestId = 0
const loadStatusCounts = async () => {
  const requestId = ++countsRequestId
  try {
    const totals = await Promise.all(
      STATUS_FILTERS.map((status) =>
        adminAPI.proxies.list(1, 1, {
          protocol: filters.protocol || undefined,
          status,
          search: searchQuery.value || undefined
        })
      )
    )
    if (requestId !== countsRequestId) return
    STATUS_FILTERS.forEach((status, index) => {
      statusCounts[status] = totals[index]?.total ?? null
    })
  } catch (error) {
    if (requestId === countsRequestId) {
      STATUS_FILTERS.forEach((status) => { statusCounts[status] = null })
    }
    console.error('Error loading proxy status counts:', error)
  }
}

// Reload the list and the status counts (scope or data changed).
const reloadAll = () => {
  loadStatusCounts()
  loadProxies()
}

const handleFilterChange = () => {
  pagination.page = 1
  reloadAll()
}

const setStatusFilter = (status: ProxyStatusFilter) => {
  if (filters.status === status) return
  filters.status = status
  pagination.page = 1
  loadProxies()
}

const clearFilters = () => {
  filters.protocol = ''
  handleFilterChange()
}

const clearAllQuery = () => {
  clearTimeout(searchTimeout)
  searchQuery.value = ''
  filters.protocol = ''
  filters.status = ''
  handleFilterChange()
}

let searchTimeout: ReturnType<typeof setTimeout>
const handleSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    reloadAll()
  }, 300)
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadProxies()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadProxies()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadProxies()
}

const closeCreateModal = () => {
  showCreateModal.value = false
  createMode.value = 'standard'
  createForm.name = ''
  createForm.protocol = 'http'
  createForm.host = ''
  createForm.port = 8080
  createForm.username = ''
  createForm.password = ''
  createForm.expires_at = ''
  createForm.fallback_mode = 'none'
  createForm.backup_proxy_id = null
  createForm.expiry_warn_days = 7
  createPasswordVisible.value = false
  batchInput.value = ''
  batchParseResult.total = 0
  batchParseResult.valid = 0
  batchParseResult.invalid = 0
  batchParseResult.duplicate = 0
  batchParseResult.proxies = []
}

const handleDataImported = () => {
  showImportData.value = false
  reloadAll()
}

// Parse proxy URL: protocol://user:pass@host:port or protocol://host:port
// Host may be a domain, IPv4, or bracketed IPv6 ([2001:db8::1]).
const parseProxyUrl = (
  line: string
): {
  protocol: ProxyProtocol
  host: string
  port: number
  username: string
  password: string
} | null => {
  const trimmed = line.trim()
  if (!trimmed) return null

  // Regex to parse proxy URL (supports http, https, socks5, socks5h).
  // Host alternatives: [bracketed-IPv6] | hostname/IPv4 (colon-free, so the
  // match stops before the final :port).
  const regex =
    /^(https?|socks5h?):\/\/(?:([^:@\[\]]+):([^@\[\]]+)@)?(\[[0-9a-f:.]+\]|[^:\[\]]+):(\d+)$/i
  const match = trimmed.match(regex)

  if (!match) return null

  const [, protocol, username, password, rawHost, port] = match
  const portNum = parseInt(port, 10)

  if (portNum < 1 || portNum > 65535) return null

  // Strip brackets from IPv6 literals; the backend re-brackets via net.JoinHostPort.
  const host = rawHost.replace(/^\[|\]$/g, '').trim()

  return {
    protocol: protocol.toLowerCase() as ProxyProtocol,
    host,
    port: portNum,
    username: username?.trim() || '',
    password: password?.trim() || ''
  }
}

const parseBatchInput = () => {
  const lines = batchInput.value.split('\n').filter((l) => l.trim())
  const seen = new Set<string>()
  const proxies: typeof batchParseResult.proxies = []
  let invalid = 0
  let duplicate = 0

  for (const line of lines) {
    const parsed = parseProxyUrl(line)
    if (!parsed) {
      invalid++
      continue
    }

    // Check for duplicates (same host:port:username:password)
    const key = `${parsed.host}:${parsed.port}:${parsed.username}:${parsed.password}`
    if (seen.has(key)) {
      duplicate++
      continue
    }
    seen.add(key)
    proxies.push(parsed)
  }

  batchParseResult.total = lines.length
  batchParseResult.valid = proxies.length
  batchParseResult.invalid = invalid
  batchParseResult.duplicate = duplicate
  batchParseResult.proxies = proxies
}

const handleBatchCreate = async () => {
  if (batchParseResult.valid === 0) return

  submitting.value = true
  try {
    const result = await adminAPI.proxies.batchCreate(batchParseResult.proxies)
    const created = result.created || 0
    const skipped = result.skipped || 0

    if (created > 0) {
      appStore.showSuccess(t('admin.proxies.batchImportSuccess', { created, skipped }))
    } else {
      appStore.showInfo(t('admin.proxies.batchImportAllSkipped', { skipped }))
    }

    closeCreateModal()
    reloadAll()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failedToImport'))
    console.error('Error batch creating proxies:', error)
  } finally {
    submitting.value = false
  }
}

const handleCreateProxy = async () => {
  if (!createForm.name.trim()) {
    appStore.showError(t('admin.proxies.nameRequired'))
    return
  }
  if (!createForm.host.trim()) {
    appStore.showError(t('admin.proxies.hostRequired'))
    return
  }
  if (createForm.port < 1 || createForm.port > 65535) {
    appStore.showError(t('admin.proxies.portInvalid'))
    return
  }
  submitting.value = true
  try {
    await adminAPI.proxies.create({
      name: createForm.name.trim(),
      protocol: createForm.protocol,
      host: createForm.host.trim(),
      port: createForm.port,
      username: createForm.username.trim() || null,
      password: createForm.password.trim() || null,
      expires_at: createForm.expires_at ? Math.floor(new Date(createForm.expires_at).getTime() / 1000) : null,
      fallback_mode: createForm.fallback_mode,
      backup_proxy_id: createForm.fallback_mode === 'proxy' ? createForm.backup_proxy_id : null,
      expiry_warn_days: createForm.expiry_warn_days,
    })
    appStore.showSuccess(t('admin.proxies.proxyCreated'))
    closeCreateModal()
    reloadAll()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failedToCreate'))
    console.error('Error creating proxy:', error)
  } finally {
    submitting.value = false
  }
}

const handleEdit = (proxy: Proxy) => {
  editingProxy.value = proxy
  editForm.name = proxy.name
  editForm.protocol = proxy.protocol
  editForm.host = proxy.host
  editForm.port = proxy.port
  editForm.username = proxy.username || ''
  editForm.password = proxy.password || ''
  editForm.status = proxy.status === 'expired' ? 'inactive' : proxy.status
  editForm.expires_at = proxy.expires_at ? proxy.expires_at.slice(0, 10) : ''
  editForm.fallback_mode = proxy.fallback_mode || 'none'
  editForm.backup_proxy_id = proxy.backup_proxy_id ?? null
  editForm.expiry_warn_days = proxy.expiry_warn_days ?? 7
  editPasswordVisible.value = false
  editPasswordDirty.value = false
  showEditModal.value = true
}

const closeEditModal = () => {
  showEditModal.value = false
  editingProxy.value = null
  editPasswordVisible.value = false
  editPasswordDirty.value = false
}

const handleUpdateProxy = async () => {
  if (!editingProxy.value) return
  if (!editForm.name.trim()) {
    appStore.showError(t('admin.proxies.nameRequired'))
    return
  }
  if (!editForm.host.trim()) {
    appStore.showError(t('admin.proxies.hostRequired'))
    return
  }
  if (editForm.port < 1 || editForm.port > 65535) {
    appStore.showError(t('admin.proxies.portInvalid'))
    return
  }

  submitting.value = true
  try {
    const updateData: any = {
      name: editForm.name.trim(),
      protocol: editForm.protocol,
      host: editForm.host.trim(),
      port: editForm.port,
      username: editForm.username.trim(),
      status: editForm.status,
      expires_at: editForm.expires_at ? Math.floor(new Date(editForm.expires_at).getTime() / 1000) : null,
      fallback_mode: editForm.fallback_mode,
      backup_proxy_id: editForm.fallback_mode === 'proxy' ? editForm.backup_proxy_id : null,
      expiry_warn_days: editForm.expiry_warn_days,
    }

    // Only include password if user actually modified the field
    if (editPasswordDirty.value) {
      updateData.password = editForm.password.trim()
    }

    await adminAPI.proxies.update(editingProxy.value.id, updateData)
    appStore.showSuccess(t('admin.proxies.proxyUpdated'))
    closeEditModal()
    reloadAll()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failedToUpdate'))
    console.error('Error updating proxy:', error)
  } finally {
    submitting.value = false
  }
}

const applyLatencyResult = (
  proxyId: number,
  result: {
    success: boolean
    latency_ms?: number
    message?: string
    ip_address?: string
    country?: string
    country_code?: string
    region?: string
    city?: string
  }
) => {
  const target = proxies.value.find((proxy) => proxy.id === proxyId)
  if (!target) return
  if (result.success) {
    target.latency_status = 'success'
    target.latency_ms = result.latency_ms
    target.ip_address = result.ip_address
    target.country = result.country
    target.country_code = result.country_code
    target.region = result.region
    target.city = result.city
  } else {
    target.latency_status = 'failed'
    target.latency_ms = undefined
    target.ip_address = undefined
    target.country = undefined
    target.country_code = undefined
    target.region = undefined
    target.city = undefined
  }
  target.latency_message = result.message
}

const summarizeQualityStatus = (result: ProxyQualityCheckResult): Proxy['quality_status'] => {
  if (result.challenge_count > 0) return 'challenge'
  if (result.failed_count > 0) return 'failed'
  if (result.warn_count > 0) return 'warn'
  return 'healthy'
}

const applyQualityResult = (proxyId: number, result: ProxyQualityCheckResult) => {
  const target = proxies.value.find((proxy) => proxy.id === proxyId)
  if (!target) return
  target.quality_status = summarizeQualityStatus(result)
  target.quality_score = result.score
  target.quality_grade = result.grade
  target.quality_summary = result.summary
  target.quality_checked = result.checked_at
}

const formatLocation = (proxy: Proxy) => {
  const parts = [proxy.country, proxy.city].filter(Boolean) as string[]
  return parts.join(' · ')
}

const flagUrl = (code: string) =>
  `https://unpkg.com/flag-icons/flags/4x3/${code.toLowerCase()}.svg`

type LatencyTone = 'ok' | 'warn' | 'bad' | 'none'
const latencyTone = (proxy: Proxy): LatencyTone => {
  if (proxy.latency_status === 'failed') return 'bad'
  if (typeof proxy.latency_ms !== 'number') return 'none'
  return proxy.latency_ms < 200 ? 'ok' : 'warn'
}
const LATENCY_DOT: Record<LatencyTone, string> = {
  ok: 'ui-dot is-ok',
  warn: 'ui-dot is-warn',
  bad: 'ui-dot is-bad',
  none: 'ui-dot'
}
const LATENCY_TEXT: Record<LatencyTone, string> = {
  ok: 'proxies-latency is-ok',
  warn: 'proxies-latency is-warn',
  bad: 'proxies-latency is-bad',
  none: 'proxies-latency'
}
const latencyDotClass = (proxy: Proxy) => LATENCY_DOT[latencyTone(proxy)]
const latencyTextClass = (proxy: Proxy) => LATENCY_TEXT[latencyTone(proxy)]
const latencyText = (proxy: Proxy) => {
  if (proxy.latency_status === 'failed') return t('admin.proxies.testFailed')
  if (typeof proxy.latency_ms === 'number') return `${proxy.latency_ms} ms`
  return t('admin.adminUi.proxies.untested')
}
const qualityGradeClass = (status?: string) => {
  if (status === 'healthy') return 'is-ok'
  if (status === 'warn') return 'is-warn'
  return 'is-bad'
}
const fallbackLabel = (proxy: Proxy) => {
  if (proxy.fallback_mode === 'direct') return t('admin.proxies.fallbackDirect')
  if (proxy.fallback_mode === 'proxy') {
    const backup = allProxiesForBackup.value.find((p) => p.id === proxy.backup_proxy_id)
    return `${t('admin.proxies.fallbackProxy')}${backup ? ` · ${backup.name}` : ''}`
  }
  return t('admin.proxies.fallbackNone')
}
const togglePasswordVisible = (id: number) => {
  if (visiblePasswordIds.has(id)) visiblePasswordIds.delete(id)
  else visiblePasswordIds.add(id)
}

// Detail drawer follows the live row so test/quality results update in place.
const detailProxyId = ref<number | null>(null)
const detailProxy = computed(() =>
  detailProxyId.value === null ? null : proxies.value.find((proxy) => proxy.id === detailProxyId.value) ?? null
)
const openDetail = (proxy: Proxy) => {
  copyMenuProxyId.value = null
  detailProxyId.value = proxy.id
}
const closeDetail = () => {
  detailProxyId.value = null
}
// Dialogs sit below the drawer, so close it before handing off.
const runFromDrawer = (action: (proxy: Proxy) => unknown) => {
  const proxy = detailProxy.value
  if (!proxy) return
  closeDetail()
  action(proxy)
}

const startTestingProxy = (proxyId: number) => {
  testingProxyIds.value = new Set([...testingProxyIds.value, proxyId])
}

const stopTestingProxy = (proxyId: number) => {
  const next = new Set(testingProxyIds.value)
  next.delete(proxyId)
  testingProxyIds.value = next
}

const startQualityCheckingProxy = (proxyId: number) => {
  qualityCheckingProxyIds.value = new Set([...qualityCheckingProxyIds.value, proxyId])
}

const stopQualityCheckingProxy = (proxyId: number) => {
  const next = new Set(qualityCheckingProxyIds.value)
  next.delete(proxyId)
  qualityCheckingProxyIds.value = next
}

// Brief check/x pop on the Test button after each result.
const testFeedback = reactive(new Map<number, 'ok' | 'fail'>())
const feedbackTimers = new Map<number, ReturnType<typeof setTimeout>>()
const flashTestFeedback = (proxyId: number, ok: boolean) => {
  clearTimeout(feedbackTimers.get(proxyId))
  testFeedback.set(proxyId, ok ? 'ok' : 'fail')
  feedbackTimers.set(proxyId, setTimeout(() => {
    testFeedback.delete(proxyId)
    feedbackTimers.delete(proxyId)
  }, 1600))
}
const testState = (proxyId: number) =>
  testingProxyIds.value.has(proxyId) ? 'testing' : testFeedback.get(proxyId) ?? 'idle'

const runProxyTest = async (proxyId: number, notify: boolean) => {
  startTestingProxy(proxyId)
  try {
    const result = await adminAPI.proxies.testProxy(proxyId)
    applyLatencyResult(proxyId, result)
    flashTestFeedback(proxyId, result.success)
    if (notify) {
      if (result.success) {
        const message = result.latency_ms
          ? t('admin.proxies.proxyWorkingWithLatency', { latency: result.latency_ms })
          : t('admin.proxies.proxyWorking')
        appStore.showSuccess(message)
      } else {
        appStore.showError(result.message || t('admin.proxies.proxyTestFailed'))
      }
    }
    return result
  } catch (error: any) {
    const message = error.response?.data?.detail || t('admin.proxies.failedToTest')
    applyLatencyResult(proxyId, { success: false, message })
    flashTestFeedback(proxyId, false)
    if (notify) {
      appStore.showError(message)
    }
    console.error('Error testing proxy:', error)
    return null
  } finally {
    stopTestingProxy(proxyId)
  }
}

const handleTestConnection = async (proxy: Proxy) => {
  await runProxyTest(proxy.id, true)
}

const handleQualityCheck = async (proxy: Proxy) => {
  startQualityCheckingProxy(proxy.id)
  try {
    const result = await adminAPI.proxies.checkProxyQuality(proxy.id)
    qualityReportProxy.value = proxy
    qualityReport.value = result
    showQualityReportDialog.value = true

    const baseStep = result.items.find((item) => item.target === 'base_connectivity')
    if (baseStep && baseStep.status === 'pass') {
      applyLatencyResult(proxy.id, {
        success: true,
        latency_ms: result.base_latency_ms,
        message: result.summary,
        ip_address: result.exit_ip,
        country: result.country,
        country_code: result.country_code
      })
    }
    applyQualityResult(proxy.id, result)

    appStore.showSuccess(
      t('admin.proxies.qualityCheckDone', { score: result.score, grade: result.grade })
    )
  } catch (error: any) {
    const message = error.response?.data?.detail || t('admin.proxies.qualityCheckFailed')
    appStore.showError(message)
    console.error('Error checking proxy quality:', error)
  } finally {
    stopQualityCheckingProxy(proxy.id)
  }
}

const runBatchProxyQualityChecks = async (ids: number[]) => {
  if (ids.length === 0) return { total: 0, healthy: 0, warn: 0, challenge: 0, failed: 0 }

  const concurrency = 3
  let index = 0
  let healthy = 0
  let warn = 0
  let challenge = 0
  let failed = 0

  const worker = async () => {
    while (index < ids.length) {
      const current = ids[index]
      index++
      startQualityCheckingProxy(current)
      try {
        const result = await adminAPI.proxies.checkProxyQuality(current)
        const target = proxies.value.find((proxy) => proxy.id === current)
        if (target) {
          const baseStep = result.items.find((item) => item.target === 'base_connectivity')
          if (baseStep && baseStep.status === 'pass') {
            applyLatencyResult(current, {
              success: true,
              latency_ms: result.base_latency_ms,
              message: result.summary,
              ip_address: result.exit_ip,
              country: result.country,
              country_code: result.country_code
            })
          }
        }
        applyQualityResult(current, result)
        if (result.challenge_count > 0) {
          challenge++
        } else if (result.failed_count > 0) {
          failed++
        } else if (result.warn_count > 0) {
          warn++
        } else {
          healthy++
        }
      } catch {
        failed++
      } finally {
        stopQualityCheckingProxy(current)
      }
    }
  }

  const workers = Array.from({ length: Math.min(concurrency, ids.length) }, () => worker())
  await Promise.all(workers)
  return {
    total: ids.length,
    healthy,
    warn,
    challenge,
    failed
  }
}

const closeQualityReportDialog = () => {
  showQualityReportDialog.value = false
  qualityReportProxy.value = null
  qualityReport.value = null
}

const qualityStatusClass = (status: string) => {
  if (status === 'pass') return 'badge-success'
  if (status === 'warn') return 'badge-warning'
  if (status === 'challenge') return 'badge-danger'
  return 'badge-danger'
}

const qualityStatusLabel = (status: string) => {
  if (status === 'pass') return t('admin.proxies.qualityStatusPass')
  if (status === 'warn') return t('admin.proxies.qualityStatusWarn')
  if (status === 'challenge') return t('admin.proxies.qualityStatusChallenge')
  return t('admin.proxies.qualityStatusFail')
}

// 有效期「选天数」⇄ 日历联动:天数自 base 起算(创建=今天;编辑=代理创建日),本地日历日 round-trip 稳定;canonical 仍是 expires_at 日期串
const EXPIRY_PRESETS = [7, 30, 90, 180]
const toLocalDateStr = (dt: Date): string => {
  const y = dt.getFullYear()
  const m = String(dt.getMonth() + 1).padStart(2, '0')
  const d = String(dt.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}
// base 为空 → 今天本地 00:00;否则该日期本地 00:00
const baseDateOrToday = (baseDateStr: string): Date => {
  const base = baseDateStr ? new Date(`${baseDateStr}T00:00:00`) : new Date()
  base.setHours(0, 0, 0, 0)
  return base
}
// base + N 天 → 本地 YYYY-MM-DD;N≤0/空 → '' 表示永不过期
const addDaysToBase = (baseDateStr: string, n: number | null): string => {
  const days = Number(n)
  if (!days || days <= 0) return ''
  const dt = baseDateOrToday(baseDateStr)
  dt.setDate(dt.getDate() + days)
  return toLocalDateStr(dt)
}
// target 相对 base 的整天数(本地日历差,避免时区/时刻抖动)
const daysFromBase = (baseDateStr: string, targetDateStr: string): number | null => {
  if (!targetDateStr) return null
  const target = new Date(`${targetDateStr}T00:00:00`)
  return Math.round((target.getTime() - baseDateOrToday(baseDateStr).getTime()) / 86400000)
}
// 编辑时有效期自「代理创建日」起算;创建时无 created_at → base='' 用今天
const editBaseDate = computed(() =>
  editingProxy.value?.created_at ? editingProxy.value.created_at.slice(0, 10) : '',
)
const createExpiresDays = computed<number | null>({
  get: () => daysFromBase('', createForm.expires_at),
  set: (v) => {
    createForm.expires_at = addDaysToBase('', v)
  },
})
const editExpiresDays = computed<number | null>({
  get: () => daysFromBase(editBaseDate.value, editForm.expires_at),
  set: (v) => {
    editForm.expires_at = addDaysToBase(editBaseDate.value, v)
  },
})

const expiryLabel = (row: Proxy): string => {
  const { key, params } = proxyExpiryLabelKey(row.expires_at, row.status)
  return params ? t(key, params) : t(key)
}

const expiryBadgeClass = (row: Proxy): string =>
  proxyExpiryBadgeClass(row.expires_at, row.status)

const qualityOverallClass = (status?: string) => {
  if (status === 'healthy') return 'badge-success'
  if (status === 'warn') return 'badge-warning'
  if (status === 'challenge') return 'badge-danger'
  return 'badge-danger'
}

const qualityOverallLabel = (status?: string) => {
  if (status === 'healthy') return t('admin.proxies.qualityStatusHealthy')
  if (status === 'warn') return t('admin.proxies.qualityStatusWarn')
  if (status === 'challenge') return t('admin.proxies.qualityStatusChallenge')
  return t('admin.proxies.qualityStatusFail')
}

const qualityTargetLabel = (target: string) => {
  switch (target) {
    case 'base_connectivity':
      return t('admin.proxies.qualityTargetBase')
    case 'openai':
      return 'OpenAI'
    case 'anthropic':
      return 'Anthropic'
    case 'gemini':
      return 'Gemini'
    case 'grok':
      return 'Grok'
    case 'kimi':
      return 'Kimi'
    case 'zhipu':
      return 'Zhipu GLM'
    case 'deepseek':
      return 'DeepSeek'
    case 'minimax':
      return 'MiniMax'
    default:
      return target
  }
}

const fetchAllProxiesForBatch = async (): Promise<Proxy[]> => {
  const pageSize = 200
  const result: Proxy[] = []
  let page = 1
  let totalPages = 1

  while (page <= totalPages) {
    const response = await adminAPI.proxies.list(
      page,
      pageSize,
      {
        protocol: filters.protocol || undefined,
        status: filters.status as any,
        search: searchQuery.value || undefined,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      }
    )
    result.push(...response.items)
    totalPages = response.pages || 1
    page++
  }

  return result
}

const runBatchProxyTests = async (ids: number[]) => {
  if (ids.length === 0) return
  const concurrency = 5
  let index = 0

  const worker = async () => {
    while (index < ids.length) {
      const current = ids[index]
      index++
      await runProxyTest(current, false)
    }
  }

  const workers = Array.from({ length: Math.min(concurrency, ids.length) }, () => worker())
  await Promise.all(workers)
}

const handleBatchTest = async () => {
  if (batchTesting.value) return

  batchTesting.value = true
  try {
    let ids: number[] = []
    if (selectedCount.value > 0) {
      ids = Array.from(selectedProxyIds.value)
    } else {
      const allProxies = await fetchAllProxiesForBatch()
      ids = allProxies.map((proxy) => proxy.id)
    }

    if (ids.length === 0) {
      appStore.showInfo(t('admin.proxies.batchTestEmpty'))
      return
    }

    await runBatchProxyTests(ids)
    appStore.showSuccess(t('admin.proxies.batchTestDone', { count: ids.length }))
    loadProxies()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.batchTestFailed'))
    console.error('Error batch testing proxies:', error)
  } finally {
    batchTesting.value = false
  }
}

const handleBatchQualityCheck = async () => {
  if (batchQualityChecking.value) return

  batchQualityChecking.value = true
  try {
    let ids: number[] = []
    if (selectedCount.value > 0) {
      ids = Array.from(selectedProxyIds.value)
    } else {
      const allProxies = await fetchAllProxiesForBatch()
      ids = allProxies.map((proxy) => proxy.id)
    }

    if (ids.length === 0) {
      appStore.showInfo(t('admin.proxies.batchQualityEmpty'))
      return
    }

    const summary = await runBatchProxyQualityChecks(ids)
    appStore.showSuccess(
      t('admin.proxies.batchQualityDone', {
        count: summary.total,
        healthy: summary.healthy,
        warn: summary.warn,
        challenge: summary.challenge,
        failed: summary.failed
      })
    )
    loadProxies()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.batchQualityFailed'))
    console.error('Error batch checking quality:', error)
  } finally {
    batchQualityChecking.value = false
  }
}

const formatExportTimestamp = () => {
  const now = new Date()
  const pad2 = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}${pad2(now.getMonth() + 1)}${pad2(now.getDate())}${pad2(now.getHours())}${pad2(now.getMinutes())}${pad2(now.getSeconds())}`
}

const handleExportData = async () => {
  if (exportingData.value) return
  exportingData.value = true
  try {
    const dataPayload = await adminAPI.proxies.exportData(
      selectedCount.value > 0
        ? { ids: Array.from(selectedProxyIds.value) }
        : {
            filters: buildProxyQueryFilters()
          }
    )
    const timestamp = formatExportTimestamp()
    const filename = `sub2api-proxy-${timestamp}.json`
    const blob = new Blob([JSON.stringify(dataPayload, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    link.click()
    URL.revokeObjectURL(url)
    appStore.showSuccess(t('admin.proxies.dataExported'))
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.proxies.dataExportFailed'))
  } finally {
    exportingData.value = false
    showExportDataDialog.value = false
  }
}

const handleDelete = (proxy: Proxy) => {
  if ((proxy.account_count || 0) > 0) {
    appStore.showError(t('admin.proxies.deleteBlockedInUse'))
    return
  }
  deletingProxy.value = proxy
  showDeleteDialog.value = true
}

const openBatchDelete = () => {
  if (selectedCount.value === 0) {
    return
  }
  showBatchDeleteDialog.value = true
}

const confirmDelete = async () => {
  if (!deletingProxy.value) return

  try {
    await adminAPI.proxies.delete(deletingProxy.value.id)
    appStore.showSuccess(t('admin.proxies.proxyDeleted'))
    showDeleteDialog.value = false
    removeSelectedProxies([deletingProxy.value.id])
    deletingProxy.value = null
    reloadAll()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failedToDelete'))
    console.error('Error deleting proxy:', error)
  }
}

const confirmBatchDelete = async () => {
  const ids = Array.from(selectedProxyIds.value)
  if (ids.length === 0) {
    showBatchDeleteDialog.value = false
    return
  }

  try {
    const result = await adminAPI.proxies.batchDelete(ids)
    const deleted = result.deleted_ids?.length || 0
    const skipped = result.skipped?.length || 0

    if (deleted > 0) {
      appStore.showSuccess(t('admin.proxies.batchDeleteDone', { deleted, skipped }))
    } else if (skipped > 0) {
      appStore.showInfo(t('admin.proxies.batchDeleteSkipped', { skipped }))
    }

    clearSelectedProxies()
    showBatchDeleteDialog.value = false
    reloadAll()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.batchDeleteFailed'))
    console.error('Error batch deleting proxies:', error)
  }
}

const openAccountsModal = async (proxy: Proxy) => {
  accountsProxy.value = proxy
  proxyAccounts.value = []
  accountsLoading.value = true
  showAccountsModal.value = true

  try {
    proxyAccounts.value = await adminAPI.proxies.getProxyAccounts(proxy.id)
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.accountsFailed'))
    console.error('Error loading proxy accounts:', error)
  } finally {
    accountsLoading.value = false
  }
}

const closeAccountsModal = () => {
  showAccountsModal.value = false
  accountsProxy.value = null
  proxyAccounts.value = []
}

// ── Proxy URL copy ──
function buildAuthPart(row: any): string {
  const user = row.username ? encodeURIComponent(row.username) : ''
  const pass = row.password ? encodeURIComponent(row.password) : ''
  if (user && pass) return `${user}:${pass}@`
  if (user) return `${user}@`
  if (pass) return `:${pass}@`
  return ''
}

function buildProxyUrl(row: any): string {
  return `${row.protocol}://${buildAuthPart(row)}${row.host}:${row.port}`
}

function getCopyFormats(row: any) {
  const hasAuth = row.username || row.password
  const fullUrl = buildProxyUrl(row)
  const formats = [
    { label: fullUrl, value: fullUrl },
  ]
  if (hasAuth) {
    const withoutProtocol = fullUrl.replace(/^[^:]+:\/\//, '')
    formats.push({ label: withoutProtocol, value: withoutProtocol })
  }
  formats.push({ label: `${row.host}:${row.port}`, value: `${row.host}:${row.port}` })
  return formats
}

function copyProxyUrl(row: any) {
  copyToClipboard(buildProxyUrl(row), t('admin.proxies.urlCopied'))
  copyMenuProxyId.value = null
}

function toggleCopyMenu(id: number) {
  copyMenuProxyId.value = copyMenuProxyId.value === id ? null : id
}

function copyFormat(value: string) {
  copyToClipboard(value, t('admin.proxies.urlCopied'))
  copyMenuProxyId.value = null
}

function closeCopyMenu() {
  copyMenuProxyId.value = null
}

onMounted(() => {
  reloadAll()
  loadBackupProxyOptions()
  document.addEventListener('click', closeCopyMenu)
})

onUnmounted(() => {
  clearTimeout(searchTimeout)
  feedbackTimers.forEach((timer) => clearTimeout(timer))
  abortController?.abort()
  document.removeEventListener('click', closeCopyMenu)
})
</script>

<style scoped>
.proxies-seg-count {
  margin-left: 0.25rem;
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
  font-variant-numeric: tabular-nums;
}

.proxies-filter-panel {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
  padding: 0.5rem;
}

.proxies-filter-clear {
  align-self: flex-start;
  margin-top: 0.25rem;
}

.proxies-selection {
  padding-right: 0.25rem;
}

.proxies-selection-clear {
  display: inline-grid;
  width: 1.375rem;
  height: 1.375rem;
  place-items: center;
  border-radius: 999px;
  color: var(--operator-muted-foreground);
}

.proxies-selection-clear:hover {
  background: var(--operator-muted);
  color: var(--operator-foreground);
}

.proxies-menu-sep {
  height: 1px;
  margin: 0.25rem 0.375rem;
  background: var(--operator-border);
}

.proxies-menu-danger,
.proxies-menu-danger:hover {
  color: var(--operator-destructive);
}

.dropdown-item:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.proxies-card {
  overflow: hidden;
}

.operator-proxy-table :deep(.table-wrapper table) {
  width: 100%;
  min-width: 0;
  table-layout: fixed;
}

.operator-proxy-table :deep(.operator-proxy-select-column) { width: 2.75rem; }
.operator-proxy-table :deep(.operator-proxy-name-column) { width: 24%; }
.operator-proxy-table :deep(.operator-proxy-endpoint-column) { width: 30%; }
.operator-proxy-table :deep(.operator-proxy-health-column) { width: 18%; }
.operator-proxy-table :deep(.operator-proxy-account-column) { width: 8%; }
.operator-proxy-table :deep(.operator-proxy-lifecycle-column) { width: 11%; }
.operator-proxy-table :deep(.operator-proxy-actions-column) { width: 7.5rem; }

.proxies-name {
  display: flex;
  width: 100%;
  min-width: 0;
  align-items: center;
  gap: 0.625rem;
  border-radius: var(--operator-radius-sm);
  text-align: left;
}

.proxies-name:focus-visible,
.proxies-count:focus-visible,
.operator-proxy-inline-action:focus-visible,
.operator-proxy-row-action:focus-visible,
.proxies-selection-clear:focus-visible {
  outline: 2px solid var(--operator-focus);
  outline-offset: 2px;
}

.proxies-name:hover .ui-cell-main {
  color: var(--operator-accent-text);
}

.proxies-flag {
  display: inline-grid;
  width: 1.5rem;
  height: 1.125rem;
  flex: none;
  place-items: center;
  border-radius: 3px;
  box-shadow: 0 0 0 1px var(--operator-border);
  object-fit: cover;
}

.proxies-flag.is-empty {
  background: var(--operator-raised);
  color: var(--operator-subtle-foreground);
}

.proxies-endpoint,
.proxies-health,
.proxies-auth {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
}

.proxies-host {
  min-width: 0;
  overflow: hidden;
  color: var(--operator-foreground);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.75rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.proxies-auth-icon {
  flex: none;
  color: var(--operator-subtle-foreground);
}

.operator-proxy-inline-action {
  display: inline-flex;
  width: 1.75rem;
  height: 1.75rem;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: var(--operator-radius-sm);
  color: var(--operator-subtle-foreground);
  transition: color var(--operator-duration-fast) var(--operator-ease-out), background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.operator-proxy-inline-action:hover {
  background: var(--operator-muted);
  color: var(--operator-foreground);
}

.proxies-health {
  flex-wrap: wrap;
  row-gap: 0.25rem;
}

.proxies-latency {
  color: var(--operator-subtle-foreground);
  font-size: 0.8125rem;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.proxies-latency.is-ok { color: var(--operator-success); }
.proxies-latency.is-warn { color: var(--operator-warning); }
.proxies-latency.is-bad { color: var(--operator-destructive); }

.proxies-grade {
  display: inline-grid;
  min-width: 1.25rem;
  height: 1.25rem;
  place-items: center;
  padding: 0 0.25rem;
  border-radius: var(--operator-radius-sm);
  font-size: 0.6875rem;
  font-weight: 700;
}

.proxies-grade.is-ok { background: rgb(var(--operator-success-rgb) / 0.14); color: var(--operator-success); }
.proxies-grade.is-warn { background: rgb(var(--operator-warning-rgb) / 0.14); color: var(--operator-warning); }
.proxies-grade.is-bad { background: rgb(var(--operator-destructive-rgb) / 0.14); color: var(--operator-destructive); }

.proxies-count {
  display: inline-flex;
  align-items: center;
  gap: 0.3125rem;
  padding: 0.125rem 0.5rem;
  border-radius: 999px;
  background: var(--operator-raised);
  color: var(--operator-foreground);
  font-size: 0.8125rem;
  font-variant-numeric: tabular-nums;
  transition: background-color var(--operator-duration-fast) var(--operator-ease-out);
}

button.proxies-count:hover {
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
}

.proxies-count.is-zero {
  background: transparent;
  color: var(--operator-subtle-foreground);
}

.proxies-muted {
  color: var(--operator-subtle-foreground);
  font-size: 0.8125rem;
}

.operator-proxy-row-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.125rem;
}

.operator-proxy-row-action {
  display: inline-flex;
  width: 2rem;
  height: 2rem;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: var(--operator-radius);
  color: var(--operator-muted-foreground);
  transition: color var(--operator-duration-fast) var(--operator-ease-out), background-color var(--operator-duration-fast) var(--operator-ease-out);
}

.operator-proxy-row-action:hover {
  background: var(--operator-muted);
  color: var(--operator-foreground);
}

.operator-proxy-row-actions :deep(.toolbar-popover-icon) {
  border-color: transparent;
  background: transparent;
}

/* Drawer */
.proxies-drawer-flag {
  display: grid;
  width: 2.5rem;
  height: 1.875rem;
  flex: none;
  place-items: center;
  border-radius: 4px;
  box-shadow: 0 0 0 1px var(--operator-border);
  object-fit: cover;
}

.proxies-drawer-flag.is-empty {
  background: var(--operator-raised);
  color: var(--operator-subtle-foreground);
}

.proxies-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0.5rem;
}

.proxies-stat {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 0.375rem;
  padding: 0.75rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius-lg);
  background: var(--operator-card);
}

.proxies-stat-label {
  color: var(--operator-subtle-foreground);
  font-size: 0.75rem;
}

.proxies-stat-value {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.5rem;
  color: var(--operator-foreground);
  font-size: 1.125rem;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.proxies-stat-value .proxies-latency {
  font-size: 1.125rem;
  font-weight: 600;
}

.proxies-drawer-error {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  padding: 0.625rem 0.75rem;
  border-radius: var(--operator-radius);
  background: rgb(var(--operator-destructive-rgb) / 0.1);
  color: var(--operator-destructive);
  font-size: 0.8125rem;
  overflow-wrap: anywhere;
}

.proxies-url {
  margin: -0.25rem 0;
}

.proxies-url .btn {
  margin-left: -0.5rem;
}

@media (min-width: 768px) and (max-width: 1099px) {
  .operator-proxy-table :deep(.operator-proxy-lifecycle-column) { display: none; }
  .operator-proxy-table :deep(.operator-proxy-name-column) { width: 26%; }
  .operator-proxy-table :deep(.operator-proxy-endpoint-column) { width: 34%; }
  .operator-proxy-table :deep(.operator-proxy-health-column) { width: 22%; }
}
</style>
