<template>
  <AppLayout>
    <div class="ui-page ui-stagger">
      <div class="ui-toolbar" role="toolbar" :aria-label="t('admin.plugins.title')">
        <label class="ui-search">
          <Icon name="search" size="sm" />
          <input
            v-model="query"
            type="search"
            :placeholder="t('admin.adminUi.plugins.search')"
            :aria-label="t('admin.adminUi.plugins.search')"
          />
        </label>
        <div class="viz-seg max-w-full overflow-x-auto" role="group" :aria-label="t('common.status')">
          <button
            v-for="option in stateOptions"
            :key="option.id"
            type="button"
            :aria-pressed="stateFilter === option.id"
            :data-test="`plugin-state-${option.id}`"
            @click="stateFilter = option.id"
          >
            {{ option.label }}
            <span class="plugin-seg-count">{{ stateCounts[option.id] }}</span>
          </button>
        </div>
        <HelpTooltip interactive :aria-label="t('admin.adminUi.plugins.about')" width-class="w-80">
          <template #trigger>
            <span class="app-icon-button"><Icon name="infoCircle" size="sm" /></span>
          </template>
          <ul class="space-y-1.5 pr-4">
            <li>{{ t('admin.plugins.onlyOpenAI') }}</li>
            <li>{{ t('admin.plugins.noAccountCoupling') }}</li>
            <li>{{ t('admin.plugins.uploadHint') }}</li>
            <li>{{ t('admin.plugins.runtimeNotice') }}</li>
            <li>{{ t('admin.plugins.menuNotice') }}</li>
          </ul>
        </HelpTooltip>
        <span class="ui-grow" />
        <button
          type="button"
          class="app-icon-button"
          :disabled="loading"
          :aria-label="t('common.refresh')"
          :title="t('common.refresh')"
          @click="loadPlugins"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        </button>
        <input
          ref="fileInput"
          class="hidden"
          type="file"
          accept=".s2plugin,application/zip"
          @change="handleFileSelected"
        />
        <button
          type="button"
          class="btn btn-primary"
          :disabled="uploading"
          @click="fileInput?.click()"
        >
          <Icon name="upload" size="sm" />
          {{ uploading ? t("common.processing") : t("admin.plugins.upload") }}
        </button>
      </div>

      <div v-if="loading && plugins.length === 0" class="plugin-grid" aria-busy="true">
        <div v-for="index in 3" :key="index" class="skeleton h-48" />
      </div>

      <div v-else-if="plugins.length === 0" class="ui-card">
        <div class="ui-empty">
          <span class="ui-empty-icon"><Icon name="cube" size="lg" /></span>
          <b>{{ t("admin.plugins.empty") }}</b>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="uploading" @click="fileInput?.click()">
            <Icon name="upload" size="sm" />
            {{ t("admin.plugins.upload") }}
          </button>
        </div>
      </div>

      <div v-else-if="visiblePlugins.length === 0" class="ui-card">
        <div class="ui-empty">
          <span class="ui-empty-icon"><Icon name="search" size="lg" /></span>
          <b>{{ t("admin.adminUi.noMatches") }}</b>
          <button type="button" class="btn btn-ghost btn-sm" @click="clearFilters">
            {{ t("admin.adminUi.clearFilters") }}
          </button>
        </div>
      </div>

      <TransitionGroup v-else tag="div" name="ui-list" class="plugin-grid">
        <article
          v-for="plugin in visiblePlugins"
          :key="plugin.id"
          class="ui-card plugin-card"
          :data-test="`plugin-${plugin.id}`"
          @click="openDetails(plugin)"
        >
          <header class="plugin-card-head">
            <span class="plugin-icon" :class="hasEnabledBinding(plugin) ? 'is-on' : ''" aria-hidden="true">
              <Icon name="cube" size="md" />
            </span>
            <div class="min-w-0 flex-1">
              <button type="button" class="plugin-name" @click.stop="openDetails(plugin)">
                {{ plugin.name }}
              </button>
              <p class="ui-cell-sub truncate">
                <span class="font-mono">v{{ plugin.version }}</span> · {{ plugin.plugin_key }}
              </p>
            </div>
            <span class="badge" :class="stateBadgeClass(plugin.state)">
              {{ t(`admin.plugins.${plugin.state}`) }}
            </span>
          </header>

          <p v-if="plugin.description" class="plugin-desc">{{ plugin.description }}</p>
          <p v-if="plugin.last_error" class="plugin-error" :title="plugin.last_error">
            <Icon name="exclamationCircle" size="xs" />
            <span class="truncate">{{ plugin.last_error }}</span>
          </p>

          <div class="plugin-meta">
            <ProviderMark
              v-for="platform in bindingPlatforms(plugin)"
              :key="platform"
              :platform="platform"
              :size="18"
              :label="platform"
            />
            <span class="plugin-meta-item" :title="plugin.compatibility.message || undefined">
              <span :class="compatibilityDotClass(plugin.compatibility.status)" />
              {{ t(`admin.plugins.${plugin.compatibility.status}`) }}
            </span>
            <span class="plugin-meta-item">
              <span :class="plugin.runtime_healthy ? 'ui-dot is-ok' : 'ui-dot'" />
              {{ plugin.runtime_healthy ? t("admin.plugins.healthy") : t("admin.plugins.unhealthy") }}
            </span>
            <span
              class="plugin-meta-item"
              :title="`${t('admin.plugins.signature')}: ${t(`admin.plugins.${plugin.signature_status}`)}`"
            >
              <Icon :name="plugin.signature_status === 'trusted' ? 'shield' : 'exclamationTriangle'" size="xs" />
              {{ t(`admin.plugins.${plugin.signature_status}`) }}
            </span>
            <span v-if="hasEnabledBinding(plugin)" class="plugin-meta-item">
              <Icon name="signal" size="xs" />
              {{ t("admin.adminUi.plugins.traffic", { percent: currentRollout(plugin) }) }}
            </span>
          </div>

          <footer class="plugin-card-foot" @click.stop>
            <button
              v-if="hasEnabledBinding(plugin)"
              type="button"
              class="btn btn-secondary btn-sm"
              @click="openConfiguration(plugin)"
            >
              <Icon name="cog" size="sm" />
              {{ t("admin.plugins.configure") }}
            </button>
            <button
              v-else
              type="button"
              class="btn btn-primary btn-sm"
              :disabled="!canEnable(plugin)"
              @click="enablePlugin(plugin)"
            >
              <Icon name="play" size="sm" />
              {{ t("admin.plugins.enable") }}
            </button>
            <span class="flex-1" />
            <ToolbarPopover
              :label="t('admin.adminUi.moreActions')"
              icon="more"
              icon-only
              menu
              :test-id="`plugin-more-${plugin.id}`"
            >
              <button type="button" role="menuitem" class="dropdown-item" @click="openDetails(plugin)">
                <Icon name="eye" size="sm" />{{ t("admin.adminUi.details") }}
              </button>
              <button
                v-if="!hasEnabledBinding(plugin)"
                type="button"
                role="menuitem"
                class="dropdown-item"
                @click="openConfiguration(plugin)"
              >
                <Icon name="cog" size="sm" />{{ t("admin.plugins.configure") }}
              </button>
              <button
                type="button"
                role="menuitem"
                class="dropdown-item"
                :disabled="busyID === plugin.id"
                @click="testPlugin(plugin)"
              >
                <Icon name="beaker" size="sm" />{{ t("admin.plugins.test") }}
              </button>
              <button
                v-if="hasEnabledBinding(plugin)"
                type="button"
                role="menuitem"
                class="dropdown-item"
                :disabled="busyID === plugin.id"
                @click="disablePlugin(plugin)"
              >
                <Icon name="ban" size="sm" />{{ t("admin.plugins.disable") }}
              </button>
              <button
                type="button"
                role="menuitem"
                class="dropdown-item text-red-600 dark:text-red-400"
                :disabled="busyID === plugin.id || hasEnabledBinding(plugin)"
                @click="uninstallPlugin(plugin)"
              >
                <Icon name="trash" size="sm" />{{ t("admin.plugins.uninstall") }}
              </button>
            </ToolbarPopover>
          </footer>
        </article>
      </TransitionGroup>

      <SideDrawer
        :open="detailPlugin !== null"
        :title="detailPlugin?.name || ''"
        :subtitle="detailPlugin ? `v${detailPlugin.version} · ${detailPlugin.plugin_key}` : ''"
        width="30rem"
        @close="detailId = null"
      >
        <template #leading>
          <span class="plugin-icon" :class="detailPlugin && hasEnabledBinding(detailPlugin) ? 'is-on' : ''" aria-hidden="true">
            <Icon name="cube" size="md" />
          </span>
        </template>
        <template #badge>
          <span v-if="detailPlugin" class="badge" :class="stateBadgeClass(detailPlugin.state)">
            {{ t(`admin.plugins.${detailPlugin.state}`) }}
          </span>
        </template>

        <div v-if="detailPlugin" class="space-y-6">
          <p v-if="detailPlugin.description" class="text-sm text-gray-600 dark:text-gray-300">
            {{ detailPlugin.description }}
          </p>
          <p v-if="detailPlugin.last_error" class="plugin-notice is-bad" role="alert">
            <Icon name="exclamationCircle" size="sm" />
            <span class="break-words">{{ detailPlugin.last_error }}</span>
          </p>
          <p v-else-if="detailPlugin.runtime_message" class="plugin-notice">
            <Icon name="infoCircle" size="sm" />
            <span class="break-words">{{ detailPlugin.runtime_message }}</span>
          </p>

          <section>
            <h3 class="ui-section-label">{{ t("admin.plugins.rollout") }}</h3>
            <div class="flex items-center gap-3">
              <input
                :value="rolloutValues[detailPlugin.id] ?? currentRollout(detailPlugin)"
                type="range"
                min="1"
                max="100"
                step="1"
                class="w-full accent-primary-600"
                :aria-label="t('admin.plugins.rollout')"
                :disabled="hasEnabledBinding(detailPlugin)"
                @input="setRollout(detailPlugin.id, $event)"
              />
              <span class="w-12 text-right font-mono text-sm tabular-nums">
                {{ rolloutValues[detailPlugin.id] ?? currentRollout(detailPlugin) }}%
              </span>
            </div>
          </section>

          <section>
            <h3 class="ui-section-label">{{ t("admin.plugins.compatibility") }}</h3>
            <dl class="ui-dl">
              <dt>{{ t("common.status") }}</dt>
              <dd>
                <span class="plugin-meta-item">
                  <span :class="compatibilityDotClass(detailPlugin.compatibility.status)" />
                  {{ t(`admin.plugins.${detailPlugin.compatibility.status}`) }}
                </span>
                <span v-if="detailPlugin.compatibility.message" class="ui-cell-sub block">
                  {{ detailPlugin.compatibility.message }}
                </span>
              </dd>
              <dt>{{ t("admin.plugins.currentVersion") }}</dt>
              <dd class="font-mono">{{ detailPlugin.compatibility.current_sub2api_version }}</dd>
              <dt>{{ t("admin.plugins.requiredVersion") }}</dt>
              <dd class="font-mono">{{ detailPlugin.compatibility.required_sub2api_version }}</dd>
              <dt>{{ t("admin.plugins.recommendedVersion") }}</dt>
              <dd class="font-mono">{{ detailPlugin.compatibility.recommended_sub2api_version || "-" }}</dd>
              <dt>{{ t("admin.adminUi.plugins.protocol") }}</dt>
              <dd class="font-mono">
                {{ detailPlugin.compatibility.plugin_protocol }} · {{ detailPlugin.compatibility.transport_api }} · {{ detailPlugin.compatibility.ui_bridge }}
              </dd>
            </dl>
          </section>

          <section>
            <h3 class="ui-section-label">{{ t("admin.plugins.runtime") }}</h3>
            <dl class="ui-dl">
              <dt>{{ t("admin.plugins.runtime") }}</dt>
              <dd class="plugin-meta-item">
                <span :class="detailPlugin.runtime_healthy ? 'ui-dot is-ok' : 'ui-dot'" />
                {{ detailPlugin.runtime_healthy ? t("admin.plugins.healthy") : t("admin.plugins.unhealthy") }}
              </dd>
              <dt>{{ t("admin.plugins.signature") }}</dt>
              <dd>{{ t(`admin.plugins.${detailPlugin.signature_status}`) }}</dd>
              <dt>{{ t("admin.adminUi.plugins.capabilities") }}</dt>
              <dd class="space-y-1">
                <div v-for="binding in detailPlugin.bindings" :key="binding.id" class="flex items-center gap-2">
                  <ProviderMark :platform="binding.platform" :size="16" :label="binding.platform" />
                  <span class="min-w-0 truncate font-mono text-xs" :title="binding.capability">{{ binding.capability }}</span>
                  <span class="ui-cell-sub">{{ binding.account_type }}</span>
                  <span :class="binding.enabled ? 'ui-dot is-ok' : 'ui-dot'" />
                </div>
                <span v-if="detailPlugin.bindings.length === 0">-</span>
              </dd>
            </dl>
          </section>

          <section>
            <h3 class="ui-section-label">{{ t("admin.adminUi.plugins.metadata") }}</h3>
            <dl class="ui-dl">
              <dt>{{ t("admin.adminUi.plugins.author") }}</dt>
              <dd>{{ detailPlugin.author || "-" }}</dd>
              <dt>{{ t("admin.adminUi.plugins.installedAt") }}</dt>
              <dd>{{ formatDate(detailPlugin.installed_at) }}</dd>
              <dt v-if="detailPlugin.enabled_at">{{ t("admin.adminUi.plugins.enabledAt") }}</dt>
              <dd v-if="detailPlugin.enabled_at">{{ formatDate(detailPlugin.enabled_at) }}</dd>
              <dt>{{ t("admin.adminUi.plugins.updatedAt") }}</dt>
              <dd>{{ formatDate(detailPlugin.updated_at) }}</dd>
              <dt>{{ t("admin.adminUi.plugins.sha256") }}</dt>
              <dd class="break-all font-mono text-xs">{{ detailPlugin.binary_sha256 }}</dd>
            </dl>
          </section>
        </div>

        <template v-if="detailPlugin" #footer>
          <button
            v-if="hasEnabledBinding(detailPlugin)"
            type="button"
            class="btn btn-secondary"
            :disabled="busyID === detailPlugin.id"
            @click="disablePlugin(detailPlugin)"
          >
            <Icon name="ban" size="sm" />{{ t("admin.plugins.disable") }}
          </button>
          <button
            v-else
            type="button"
            class="btn btn-primary"
            :disabled="!canEnable(detailPlugin)"
            @click="enablePlugin(detailPlugin)"
          >
            <Icon name="play" size="sm" />{{ t("admin.plugins.enable") }}
          </button>
          <button type="button" class="btn btn-secondary" @click="openConfiguration(detailPlugin)">
            <Icon name="cog" size="sm" />{{ t("admin.plugins.configure") }}
          </button>
          <button
            type="button"
            class="btn btn-ghost"
            :disabled="busyID === detailPlugin.id"
            @click="testPlugin(detailPlugin)"
          >
            <Icon name="beaker" size="sm" />{{ t("admin.plugins.test") }}
          </button>
          <span class="flex-1" />
          <button
            type="button"
            class="app-icon-button text-red-600 dark:text-red-400"
            :disabled="busyID === detailPlugin.id || hasEnabledBinding(detailPlugin)"
            :aria-label="t('admin.plugins.uninstall')"
            :title="t('admin.plugins.uninstall')"
            @click="uninstallPlugin(detailPlugin)"
          >
            <Icon name="trash" size="sm" />
          </button>
        </template>
      </SideDrawer>

      <BaseDialog
        :show="configPlugin !== null"
        :title="
          t('admin.plugins.configTitle', { name: configPlugin?.name || '' })
        "
        width="full"
        @close="closeConfiguration"
      >
        <div
          class="relative min-h-[520px] overflow-hidden bg-gray-50 dark:bg-dark-900"
          :style="{ height: `${iframeHeight}px` }"
        >
          <div
            v-if="uiLoading"
            class="absolute inset-0 z-10 flex items-center justify-center text-sm text-gray-500"
          >
            {{ t("admin.plugins.loadingUI") }}
          </div>
          <div
            v-if="uiError"
            class="absolute inset-0 z-20 flex flex-col items-center justify-center p-8 text-center"
          >
            <Icon name="exclamationTriangle" size="xl" class="text-amber-500" />
            <p class="mt-3 font-medium text-gray-800 dark:text-gray-200">
              {{ t("admin.plugins.uiUnavailable") }}
            </p>
            <p class="mt-1 max-w-xl text-sm text-gray-500">{{ uiError }}</p>
          </div>
          <iframe
            v-if="uiSession"
            ref="pluginFrame"
            :src="uiSession.url"
            sandbox="allow-scripts"
            referrerpolicy="no-referrer"
            class="h-full w-full border-0 bg-white dark:bg-dark-900"
            :title="
              t('admin.plugins.configTitle', { name: configPlugin?.name || '' })
            "
            @load="handlePluginFrameLoad"
          />
        </div>
      </BaseDialog>

      <TotpStepUpDialog :controller="pluginStepUp" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import {
  adminAPI,
  type PluginInstallation,
  type PluginUISession,
} from "@/api/admin";
import { useAppStore } from "@/stores";
import AppLayout from "@/components/layout/AppLayout.vue";
import BaseDialog from "@/components/common/BaseDialog.vue";
import HelpTooltip from "@/components/common/HelpTooltip.vue";
import ToolbarPopover from "@/components/common/ToolbarPopover.vue";
import SideDrawer from "@/components/ui/SideDrawer.vue";
import ProviderMark from "@/components/viz/ProviderMark.vue";
import Icon from "@/components/icons/Icon.vue";
import TotpStepUpDialog from "@/components/auth/TotpStepUpDialog.vue";
import {
  isStepUpBlocked,
  isStepUpCancelled,
  stepUpBlockReason,
  useStepUp,
} from "@/composables/useStepUp";

interface PluginBridgeMessage {
  source?: string;
  bridge_token?: string;
  type?: string;
  request_id?: string;
  config?: unknown;
  height?: unknown;
  level?: unknown;
  message?: unknown;
}

const { t } = useI18n();
const appStore = useAppStore();
const pluginStepUp = useStepUp();
const plugins = ref<PluginInstallation[]>([]);
const loading = ref(false);
const uploading = ref(false);
const busyID = ref<number | null>(null);
const fileInput = ref<HTMLInputElement | null>(null);
const rolloutValues = ref<Record<number, number>>({});
const configPlugin = ref<PluginInstallation | null>(null);
const uiSession = ref<PluginUISession | null>(null);
const pluginFrame = ref<HTMLIFrameElement | null>(null);
const uiLoading = ref(false);
const uiError = ref("");
const iframeHeight = ref(640);
const pluginFrameLoaded = ref(false);
const pendingBridgeRequests = new Map<string, number>();

function errorMessage(error: unknown): string {
  if (typeof error === "object" && error !== null && "message" in error) {
    return String(
      (error as { message?: unknown }).message || t("common.unknownError"),
    );
  }
  return t("common.unknownError");
}

function reportSensitiveActionError(error: unknown): void {
  if (isStepUpCancelled(error)) return;
  if (isStepUpBlocked(error)) {
    appStore.showError(
      stepUpBlockReason(error) === "STEP_UP_ADMIN_API_KEY_FORBIDDEN"
        ? t("stepUp.adminApiKeyForbidden")
        : t("stepUp.notEnabled"),
    );
    return;
  }
  appStore.showError(errorMessage(error));
}

async function loadPlugins(): Promise<void> {
  loading.value = true;
  try {
    plugins.value = await adminAPI.plugins.list();
    for (const plugin of plugins.value) {
      rolloutValues.value[plugin.id] = currentRollout(plugin);
    }
  } catch (error: unknown) {
    appStore.showError(errorMessage(error));
  } finally {
    loading.value = false;
  }
}

async function handleFileSelected(event: Event): Promise<void> {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0];
  target.value = "";
  if (!file || !file.name.toLowerCase().endsWith(".s2plugin")) {
    appStore.showError(t("admin.plugins.fileRequired"));
    return;
  }
  uploading.value = true;
  try {
    await pluginStepUp.run(() => adminAPI.plugins.upload(file));
    appStore.showSuccess(t("admin.plugins.uploadSuccess"));
    await loadPlugins();
  } catch (error: unknown) {
    reportSensitiveActionError(error);
  } finally {
    uploading.value = false;
  }
}

function currentRollout(plugin: PluginInstallation): number {
  return (
    plugin.bindings.find(
      (binding) => binding.capability === "openai.oauth.outbound_transport.v1",
    )?.rollout_percent || 100
  );
}

function hasEnabledBinding(plugin: PluginInstallation): boolean {
  return plugin.bindings.some((binding) => binding.enabled);
}

function setRollout(id: number, event: Event): void {
  const value = Number((event.target as HTMLInputElement).value);
  rolloutValues.value[id] = Math.min(100, Math.max(1, value));
}

async function enablePlugin(plugin: PluginInstallation): Promise<void> {
  let acceptUntested = false;
  if (!plugin.compatibility.tested) {
    acceptUntested = window.confirm(t("admin.plugins.confirmUntested"));
    if (!acceptUntested) return;
  }
  busyID.value = plugin.id;
  try {
    await pluginStepUp.run(() =>
      adminAPI.plugins.enable(
        plugin.id,
        rolloutValues.value[plugin.id] || 100,
        acceptUntested,
      ),
    );
    appStore.showSuccess(t("admin.plugins.enableSuccess"));
    await loadPlugins();
  } catch (error: unknown) {
    reportSensitiveActionError(error);
  } finally {
    busyID.value = null;
  }
}

async function disablePlugin(plugin: PluginInstallation): Promise<void> {
  if (!window.confirm(t("admin.plugins.confirmDisable"))) return;
  busyID.value = plugin.id;
  try {
    await pluginStepUp.run(() => adminAPI.plugins.disable(plugin.id));
    appStore.showSuccess(t("admin.plugins.disableSuccess"));
    await loadPlugins();
  } catch (error: unknown) {
    reportSensitiveActionError(error);
  } finally {
    busyID.value = null;
  }
}

async function uninstallPlugin(plugin: PluginInstallation): Promise<void> {
  if (!window.confirm(t("admin.plugins.confirmUninstall"))) return;
  busyID.value = plugin.id;
  try {
    await pluginStepUp.run(() => adminAPI.plugins.remove(plugin.id));
    appStore.showSuccess(t("admin.plugins.uninstallSuccess"));
    await loadPlugins();
  } catch (error: unknown) {
    reportSensitiveActionError(error);
  } finally {
    busyID.value = null;
  }
}

async function testPlugin(plugin: PluginInstallation): Promise<void> {
  busyID.value = plugin.id;
  try {
    const result = await pluginStepUp.run(() =>
      adminAPI.plugins.test(plugin.id),
    );
    if (result.success)
      appStore.showSuccess(result.message || t("admin.plugins.testSuccess"));
    else appStore.showError(result.message || t("common.error"));
  } catch (error: unknown) {
    reportSensitiveActionError(error);
  } finally {
    busyID.value = null;
  }
}

async function openConfiguration(plugin: PluginInstallation): Promise<void> {
  configPlugin.value = plugin;
  uiSession.value = null;
  pluginFrameLoaded.value = false;
  clearPendingBridgeRequests();
  uiLoading.value = true;
  uiError.value = "";
  iframeHeight.value = 640;
  try {
    uiSession.value = await adminAPI.plugins.createUISession(plugin.id);
  } catch (error: unknown) {
    uiLoading.value = false;
    uiError.value = errorMessage(error);
  }
}

function closeConfiguration(): void {
  clearPendingBridgeRequests();
  pluginFrameLoaded.value = false;
  configPlugin.value = null;
  uiSession.value = null;
  uiLoading.value = false;
  uiError.value = "";
}

function clearPendingBridgeRequests(): void {
  for (const timeout of pendingBridgeRequests.values()) window.clearTimeout(timeout);
  pendingBridgeRequests.clear();
}

function handlePluginFrameLoad(): void {
  // A load can also be caused by a plugin navigating its iframe. Drop all
  // outstanding responses so a late config response is never sent to the new document.
  if (pluginFrameLoaded.value) clearPendingBridgeRequests();
  pluginFrameLoaded.value = true;
  uiLoading.value = false;
}

function registerBridgeRequest(requestID: string): void {
  const timeout = window.setTimeout(() => {
    pendingBridgeRequests.delete(requestID);
  }, 30_000);
  pendingBridgeRequests.set(requestID, timeout);
}

function postBridgeResult(
  request: PluginBridgeMessage,
  payload: Record<string, unknown>,
): void {
  if (!pluginFrame.value?.contentWindow || !uiSession.value) return;
  const requestID = typeof request.request_id === "string" ? request.request_id.trim() : "";
  const timeout = pendingBridgeRequests.get(requestID);
  if (!requestID || timeout === undefined) return;
  window.clearTimeout(timeout);
  pendingBridgeRequests.delete(requestID);
  pluginFrame.value.contentWindow.postMessage(
    {
      source: "sub2api-plugin-host",
      bridge_token: uiSession.value.bridge_token,
      type: `${request.type}.result`,
      request_id: requestID,
      ...payload,
    },
    // The sandboxed iframe has an opaque origin, so no fixed target origin exists.
    // Pending request tracking plus load invalidation prevents cross-navigation leaks.
    "*",
  );
}

async function handleBridgeMessage(event: MessageEvent): Promise<void> {
  if (
    !uiSession.value ||
    !configPlugin.value ||
    event.source !== pluginFrame.value?.contentWindow ||
    event.origin !== "null"
  )
    return;
  const message = event.data as PluginBridgeMessage;
  if (
    !message ||
    message.source !== "sub2api-plugin-ui" ||
    message.bridge_token !== uiSession.value.bridge_token
  )
    return;

  const requestID = typeof message.request_id === "string" ? message.request_id.trim() : "";
  const expectsResponse =
    message.type === "config.load" ||
    message.type === "config.save" ||
    message.type === "config.test" ||
    message.type === "plugin.status";
  if (expectsResponse) {
    if (!requestID || pendingBridgeRequests.has(requestID)) return;
    registerBridgeRequest(requestID);
  }

  try {
    switch (message.type) {
      case "sub2api.plugin.ready":
        uiLoading.value = false;
        break;
      case "config.load": {
        const config = await adminAPI.plugins.getConfig(configPlugin.value.id);
        postBridgeResult(message, { ok: true, config });
        break;
      }
      case "config.save": {
        if (
          !message.config ||
          typeof message.config !== "object" ||
          Array.isArray(message.config)
        ) {
          throw new Error(t("admin.plugins.bridgeRejected"));
        }
        const config = await pluginStepUp.run(() =>
          adminAPI.plugins.saveConfig(
            configPlugin.value!.id,
            message.config as Record<string, unknown>,
          ),
        );
        postBridgeResult(message, { ok: true, config });
        appStore.showSuccess(t("common.saved"));
        break;
      }
      case "config.test": {
        const result = await pluginStepUp.run(() =>
          adminAPI.plugins.test(configPlugin.value!.id),
        );
        postBridgeResult(message, { ok: result.success, result });
        // A successful result is delivered back to the plugin UI, which owns how it
        // presents it (inline status, or an explicit ui.notify). Only force a host
        // toast on failure so genuine errors are never silently dropped — plugins
        // may call config.test for lightweight status polling, not just as an
        // explicit "test" action, and those must not spam a success toast.
        if (!result.success)
          appStore.showError(result.message || t("common.error"));
        break;
      }
      case "plugin.status": {
        // Read-only runtime status (the plugin's Health snapshot). It has no side
        // effects, so it is intentionally NOT step-up gated and never raises a host
        // toast — the plugin UI renders it however it likes. This is the generic
        // channel for any plugin to surface live state without abusing config.test.
        const result = await adminAPI.plugins.status(configPlugin.value!.id);
        postBridgeResult(message, { ok: true, result });
        break;
      }
      case "ui.resize": {
        const height = Number(message.height);
        if (Number.isFinite(height))
          iframeHeight.value = Math.min(960, Math.max(520, Math.round(height)));
        break;
      }
      case "ui.notify": {
        const text =
          typeof message.message === "string"
            ? message.message.slice(0, 500)
            : "";
        if (!text) break;
        if (message.level === "error") appStore.showError(text);
        else if (message.level === "success") appStore.showSuccess(text);
        else appStore.showInfo(text);
        break;
      }
    }
  } catch (error: unknown) {
    if (isStepUpBlocked(error)) reportSensitiveActionError(error);
    postBridgeResult(message, {
      ok: false,
      error: isStepUpCancelled(error) ? t("common.cancel") : errorMessage(error),
    });
  }
}

type StateFilter = "all" | "enabled" | "disabled" | "issues";

const query = ref("");
const stateFilter = ref<StateFilter>("all");
const detailId = ref<number | null>(null);
const detailPlugin = computed(
  () => plugins.value.find((plugin) => plugin.id === detailId.value) ?? null,
);

function stateBucket(plugin: PluginInstallation): Exclude<StateFilter, "all"> {
  if (plugin.state === "error" || plugin.state === "incompatible") return "issues";
  if (plugin.state === "enabled" || plugin.state === "starting") return "enabled";
  return "disabled";
}

const searchedPlugins = computed(() => {
  const needle = query.value.trim().toLowerCase();
  if (!needle) return plugins.value;
  return plugins.value.filter((plugin) =>
    [plugin.name, plugin.plugin_key, plugin.author, plugin.description]
      .join(" ")
      .toLowerCase()
      .includes(needle),
  );
});

const stateCounts = computed(() => {
  const counts: Record<StateFilter, number> = { all: 0, enabled: 0, disabled: 0, issues: 0 };
  for (const plugin of searchedPlugins.value) {
    counts.all += 1;
    counts[stateBucket(plugin)] += 1;
  }
  return counts;
});

const stateOptions = computed<Array<{ id: StateFilter; label: string }>>(() => [
  { id: "all", label: t("common.all") },
  { id: "enabled", label: t("admin.plugins.enabled") },
  { id: "disabled", label: t("admin.plugins.disabled") },
  { id: "issues", label: t("admin.adminUi.plugins.issues") },
]);

const visiblePlugins = computed(() =>
  stateFilter.value === "all"
    ? searchedPlugins.value
    : searchedPlugins.value.filter((plugin) => stateBucket(plugin) === stateFilter.value),
);

function clearFilters(): void {
  query.value = "";
  stateFilter.value = "all";
}

function openDetails(plugin: PluginInstallation): void {
  detailId.value = plugin.id;
}

function canEnable(plugin: PluginInstallation): boolean {
  return (
    busyID.value !== plugin.id &&
    plugin.state !== "starting" &&
    plugin.compatibility.compatible
  );
}

function bindingPlatforms(plugin: PluginInstallation): string[] {
  return [...new Set(plugin.bindings.map((binding) => binding.platform).filter(Boolean))];
}

function stateBadgeClass(state: PluginInstallation["state"]): string {
  if (state === "enabled") return "badge-success";
  if (state === "error" || state === "incompatible") return "badge-danger";
  if (state === "starting") return "badge-warning";
  return "badge-gray";
}

function compatibilityDotClass(
  status: PluginInstallation["compatibility"]["status"],
): string {
  if (status === "compatible") return "ui-dot is-ok";
  if (status === "untested") return "ui-dot is-warn";
  return "ui-dot is-bad";
}

function formatDate(value?: string): string {
  if (!value) return "-";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

onMounted(() => {
  window.addEventListener("message", handleBridgeMessage);
  void loadPlugins();
});

onBeforeUnmount(() => {
  window.removeEventListener("message", handleBridgeMessage);
  clearPendingBridgeRequests();
});
</script>

<style scoped>
.plugin-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 20rem), 1fr));
  gap: 1rem;
}

.plugin-card {
  gap: 0.875rem;
  padding: 1.125rem;
  cursor: pointer;
}

.plugin-card-head {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 0.75rem;
}

.plugin-icon {
  display: inline-flex;
  width: 2.5rem;
  height: 2.5rem;
  flex: none;
  align-items: center;
  justify-content: center;
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
  color: var(--operator-muted-foreground);
  transition:
    background-color var(--operator-duration-fast) var(--operator-ease-out),
    color var(--operator-duration-fast) var(--operator-ease-out);
}

.plugin-icon.is-on {
  background: var(--operator-accent-soft);
  color: var(--operator-accent-text);
}

.plugin-name {
  display: block;
  max-width: 100%;
  overflow: hidden;
  font-size: 0.9375rem;
  font-weight: 600;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
  border-radius: 0.25rem;
}

.plugin-name:hover {
  color: var(--operator-accent-text);
}

.plugin-name:focus-visible {
  outline: 2px solid var(--operator-accent-text);
  outline-offset: 2px;
}

.plugin-desc {
  display: -webkit-box;
  overflow: hidden;
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.plugin-error {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 0.375rem;
  color: var(--operator-destructive);
  font-size: 0.75rem;
}

.plugin-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.375rem 0.875rem;
  margin-top: auto;
  color: var(--operator-muted-foreground);
  font-size: 0.75rem;
}

.plugin-meta-item {
  display: inline-flex;
  align-items: center;
  gap: 0.375rem;
  white-space: nowrap;
}

.plugin-card-foot {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding-top: 0.875rem;
  border-top: 1px solid var(--operator-border);
  cursor: default;
}

.plugin-seg-count {
  margin-left: 0.25rem;
  color: var(--operator-subtle-foreground);
  font-variant-numeric: tabular-nums;
}

.plugin-notice {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  padding: 0.625rem 0.75rem;
  border: 1px solid var(--operator-border);
  border-radius: var(--operator-radius);
  background: var(--operator-raised);
  color: var(--operator-muted-foreground);
  font-size: 0.8125rem;
}

.plugin-notice.is-bad {
  border-color: color-mix(in srgb, var(--operator-destructive) 35%, transparent);
  color: var(--operator-destructive);
}
</style>
