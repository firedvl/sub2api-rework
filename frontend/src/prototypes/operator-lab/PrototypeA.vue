<script setup lang="ts">
import { computed, ref } from 'vue'
import { Icon } from '@/components/icons'
import {
  labAccounts,
  labActivity,
  labModels,
  labPages,
  labProviders,
  labRoutes,
  labSettings,
  labSummary,
  type LabAccount,
  type LabPage,
} from './data'

const props = defineProps<{ page: LabPage }>()
const emit = defineEmits<{ navigate: [page: LabPage] }>()
const search = ref('')
const provider = ref('all')
const expandedAccount = ref<number | null>(null)

const pageTitles: Record<LabPage, { title: string; description: string }> = {
  overview: { title: 'Overview', description: 'Gateway health and normalized account capacity' },
  accounts: { title: 'Accounts', description: 'Provider credentials, quota windows, and scheduling state' },
  models: { title: 'Models & Routing', description: 'Model access and composite route health' },
  settings: { title: 'Settings', description: 'Operator defaults and capacity controls' },
}

const navIcons: Record<LabPage, 'grid' | 'users' | 'swap' | 'cog'> = {
  overview: 'grid',
  accounts: 'users',
  models: 'swap',
  settings: 'cog',
}

const filteredAccounts = computed(() => {
  const query = search.value.trim().toLowerCase()
  return labAccounts.filter((account) => {
    const matchesProvider = provider.value === 'all' || account.provider === provider.value
    const matchesQuery = !query || `${account.label} ${account.identity} ${account.models.join(' ')}`.toLowerCase().includes(query)
    return matchesProvider && matchesQuery
  })
})

function capacityStyle(account: LabAccount) {
  return { width: `${account.remaining ?? 0}%` }
}

function capacityTone(account: LabAccount) {
  if (account.remaining === null) return 'unknown'
  if (account.remaining <= 15) return 'critical'
  if (account.remaining <= 30) return 'warning'
  return 'healthy'
}
</script>

<template>
  <div class="prototype-a" data-testid="prototype-a">
    <div class="fixture-band">
      <span>PROTOTYPE A</span>
      <span>Fixture snapshot · 25 Aug 2026 19:00 UTC</span>
      <span>No production connection</span>
    </div>

    <div class="a-shell">
      <aside class="a-sidebar">
        <div class="a-brand">
          <span class="a-brand-mark">S2</span>
          <span><strong>Sub2API</strong><small>operator</small></span>
        </div>

        <nav aria-label="Prototype A sections">
          <button
            v-for="item in labPages"
            :key="item.key"
            type="button"
            :class="{ active: props.page === item.key }"
            :aria-current="props.page === item.key ? 'page' : undefined"
            @click="emit('navigate', item.key)"
          >
            <Icon :name="navIcons[item.key]" size="sm" />
            <span>{{ item.label }}</span>
          </button>
        </nav>

        <div class="a-pool-status">
          <span><i class="status-dot healthy" />Gateway online</span>
          <dl>
            <div><dt>Routable</dt><dd>{{ labSummary.routableAccounts }}/{{ labSummary.totalAccounts }}</dd></div>
            <div><dt>RPM</dt><dd>{{ labSummary.rpm }}</dd></div>
          </dl>
        </div>
      </aside>

      <section class="a-workspace">
        <header class="a-page-header">
          <div>
            <h1>{{ pageTitles[props.page].title }}</h1>
            <p>{{ pageTitles[props.page].description }}</p>
          </div>
          <div class="a-header-actions">
            <span>Updated 12s ago</span>
            <button type="button" title="Refresh fixture snapshot" aria-label="Refresh fixture snapshot">
              <Icon name="refresh" size="sm" />
            </button>
          </div>
        </header>

        <div v-if="props.page === 'overview'" class="a-page a-overview">
          <section class="a-metric-strip" aria-label="Current gateway metrics">
            <div><span>Requests today</span><strong>{{ labSummary.requests }}</strong><small>{{ labSummary.rpm }} RPM</small></div>
            <div><span>Success</span><strong>{{ labSummary.success }}</strong><small>{{ labSummary.errors }} exceptions / {{ labSummary.period }}</small></div>
            <div><span>Tokens</span><strong>{{ labSummary.tokens }}</strong><small>5.48M total</small></div>
            <div><span>Actual cost</span><strong>{{ labSummary.cost }}</strong><small>{{ labSummary.accountCost }} upstream</small></div>
            <div><span>Median response</span><strong>{{ labSummary.latency }}</strong><small>328ms TTFT</small></div>
          </section>

          <div class="a-overview-grid">
            <section class="a-panel a-capacity-panel">
              <div class="a-panel-heading">
                <div><h2>Account capacity remaining</h2><p>Normalized by each provider's reported window</p></div>
                <span>{{ labAccounts.length }} accounts</span>
              </div>
              <div class="a-capacity-table" role="table" aria-label="Account capacity remaining">
                <div class="a-capacity-head" role="row">
                  <span>Account</span><span>Remaining</span><span>Reset</span><span>State</span>
                </div>
                <div v-for="account in labAccounts" :key="account.id" class="a-capacity-row" role="row">
                  <span class="a-account-cell">
                    <i :class="['provider-mark', account.provider]">{{ account.provider.slice(0, 2).toUpperCase() }}</i>
                    <span><strong>{{ account.label }}</strong><small>{{ account.provider }} · {{ account.plan }}</small></span>
                  </span>
                  <span class="a-quota-cell">
                    <span class="a-quota-label"><strong>{{ account.remaining === null ? 'Unknown' : `${account.remaining}%` }}</strong><small>{{ account.nativeQuota }}</small></span>
                    <span :class="['a-quota-track', capacityTone(account)]"><i :style="capacityStyle(account)" /></span>
                  </span>
                  <span class="a-reset">{{ account.resetLabel }}</span>
                  <span :class="['a-state', account.health]"><i class="status-dot" />{{ account.healthLabel }}</span>
                </div>
              </div>
            </section>

            <section class="a-panel a-provider-panel">
              <div class="a-panel-heading"><div><h2>Provider pools</h2><p>Known-account average</p></div></div>
              <div v-for="item in labProviders" :key="item.id" class="a-provider-row">
                <span><i :class="['status-dot', item.health]" /><strong>{{ item.label }}</strong></span>
                <span>{{ item.averageRemaining }}%</span>
                <small>{{ item.knownAccounts }}/{{ item.accounts.length }} reporting</small>
              </div>
              <div class="a-attention">
                <Icon name="exclamationTriangle" size="sm" />
                <span><strong>3 accounts need attention</strong><small>1 exhausted · 1 auth error · 1 stale probe</small></span>
              </div>
            </section>
          </div>

          <div class="a-bottom-grid">
            <section class="a-panel">
              <div class="a-panel-heading"><div><h2>Route health</h2><p>Composite routing destinations</p></div></div>
              <div class="a-route-head"><span>Route</span><span>Pool</span><span>Success</span><span>RPM</span></div>
              <div v-for="item in labRoutes" :key="item.name" class="a-route-row">
                <span><i :class="['status-dot', item.status]" /><strong>{{ item.name }}</strong></span>
                <span>{{ item.capacity }}</span><span>{{ item.success }}</span><span>{{ item.rpm }}</span>
              </div>
            </section>
            <section class="a-panel">
              <div class="a-panel-heading"><div><h2>Recent requests</h2><p>Latest fixture activity</p></div></div>
              <div v-for="item in labActivity.slice(0, 4)" :key="item.time" class="a-activity-row">
                <time>{{ item.time }}</time><span><strong>{{ item.model }}</strong><small>{{ item.account }}</small></span>
                <code :class="{ error: item.status !== '200' }">{{ item.status }}</code>
              </div>
            </section>
          </div>
        </div>

        <div v-else-if="props.page === 'accounts'" class="a-page a-accounts">
          <div class="a-toolbar">
            <label class="a-search">
              <Icon name="search" size="sm" />
              <span class="sr-only">Search accounts</span>
              <input v-model="search" type="search" placeholder="Search account or model" />
            </label>
            <label>
              <span class="sr-only">Provider</span>
              <select v-model="provider">
                <option value="all">All providers</option>
                <option v-for="item in labProviders" :key="item.id" :value="item.id">{{ item.label }}</option>
              </select>
            </label>
            <span class="a-result-count">{{ filteredAccounts.length }} accounts</span>
            <button class="a-text-button" type="button"><Icon name="plus" size="sm" /> Add account</button>
          </div>

          <section class="a-panel a-account-table-shell">
            <div class="a-account-table" role="table" aria-label="Accounts">
              <div class="a-account-head" role="row">
                <span>Provider / Account</span><span>Health</span><span>Capacity remaining</span><span>Reset</span><span>Scheduling</span><span>Models</span><span>Today</span><span>Actions</span>
              </div>
              <template v-for="account in filteredAccounts" :key="account.id">
                <div class="a-account-row" role="row">
                  <span class="a-account-cell">
                    <i :class="['provider-mark', account.provider]">{{ account.provider.slice(0, 2).toUpperCase() }}</i>
                    <span><strong>{{ account.label }}</strong><small>{{ account.identity }}</small></span>
                  </span>
                  <span :class="['a-state', account.health]"><i class="status-dot" />{{ account.healthLabel }}</span>
                  <span class="a-quota-cell compact">
                    <span class="a-quota-label"><strong>{{ account.remaining === null ? 'Unknown' : `${account.remaining}%` }}</strong><small>{{ account.nativeQuota }}</small></span>
                    <span :class="['a-quota-track', capacityTone(account)]"><i :style="capacityStyle(account)" /></span>
                  </span>
                  <span class="a-reset">{{ account.resetLabel }}</span>
                  <span><strong>{{ account.scheduleLabel }}</strong><small>{{ account.concurrency }} concurrent</small></span>
                  <span class="a-model-count"><strong>{{ account.models.length }}</strong><small>{{ account.models[0] }}</small></span>
                  <span><strong>{{ account.requests.toLocaleString() }} req</strong><small>{{ account.tokens }} · {{ account.cost }}</small></span>
                  <span class="a-actions">
                    <button type="button" :title="`Inspect ${account.label}`" :aria-label="`Inspect ${account.label}`" @click="expandedAccount = expandedAccount === account.id ? null : account.id">
                      <Icon :name="expandedAccount === account.id ? 'chevronUp' : 'chevronDown'" size="sm" />
                    </button>
                    <button type="button" :title="`More actions for ${account.label}`" :aria-label="`More actions for ${account.label}`"><Icon name="more" size="sm" /></button>
                  </span>
                </div>
                <div v-if="expandedAccount === account.id" class="a-account-detail" role="row">
                  <div><span>Models available</span><strong>{{ account.models.join(' · ') }}</strong></div>
                  <div><span>Routing groups</span><strong>{{ account.groups.join(' · ') }}</strong></div>
                  <div><span>Recent failure</span><strong>{{ account.recentFailure || 'None in the last hour' }}</strong></div>
                  <button type="button"><Icon name="edit" size="sm" /> Edit account</button>
                </div>
              </template>
            </div>
          </section>
        </div>

        <div v-else-if="props.page === 'models'" class="a-page a-models">
          <section class="a-panel">
            <div class="a-panel-heading"><div><h2>Composite routes</h2><p>Resolution order and current pool health</p></div><button class="a-text-button" type="button"><Icon name="plus" size="sm" /> New route</button></div>
            <div class="a-model-route-grid a-route-head"><span>Route</span><span>Destinations</span><span>Provider groups</span><span>Success</span><span>Latency</span></div>
            <div v-for="item in labRoutes" :key="item.name" class="a-model-route-grid a-route-row">
              <span><i :class="['status-dot', item.status]" /><strong>{{ item.name }}</strong><small>{{ item.type }}</small></span>
              <span>{{ item.destinations.join(' → ') }}</span><span>{{ item.groups.join(' · ') }}</span><span>{{ item.success }}</span><span>{{ item.latency }}</span>
            </div>
          </section>
          <section class="a-panel">
            <div class="a-panel-heading"><div><h2>Model access</h2><p>Observed traffic and resolved provider path</p></div></div>
            <div class="a-model-grid a-route-head"><span>Model</span><span>Provider path</span><span>Route</span><span>Requests</span><span>Tokens</span><span>Latency</span></div>
            <div v-for="model in labModels" :key="model.name" class="a-model-grid a-route-row">
              <span><i :class="['status-dot', model.status]" /><strong>{{ model.name }}</strong></span><span>{{ model.provider }}</span><span>{{ model.route }}</span><span>{{ model.requests }}</span><span>{{ model.tokens }}</span><span>{{ model.latency }}</span>
            </div>
          </section>
        </div>

        <div v-else class="a-page a-settings">
          <section class="a-panel a-settings-panel">
            <div class="a-panel-heading"><div><h2>Capacity & routing</h2><p>Shared operator defaults</p></div><span>Fixture values</span></div>
            <div v-for="setting in labSettings" :key="setting.title" class="a-setting-row">
              <span><strong>{{ setting.title }}</strong><small>{{ setting.description }}</small></span>
              <button type="button">{{ setting.value }} <Icon name="chevronRight" size="sm" /></button>
            </div>
          </section>
          <section class="a-panel a-settings-panel">
            <div class="a-panel-heading"><div><h2>Review environment</h2><p>Prototype safety boundary</p></div></div>
            <dl class="a-definition-list">
              <div><dt>Data source</dt><dd>Shared synthetic fixture</dd></div>
              <div><dt>Backend proxy</dt><dd>Disabled</dd></div>
              <div><dt>Writes</dt><dd>Rejected by fixture server</dd></div>
              <div><dt>Production navigation</dt><dd>Prototype route excluded</dd></div>
            </dl>
          </section>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.prototype-a {
  min-height: calc(100vh - 72px);
  background: #090909;
  color: #e9e9e6;
  font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}

.fixture-band {
  display: flex;
  height: 28px;
  align-items: center;
  justify-content: space-between;
  padding: 0 18px;
  border-bottom: 1px solid #292927;
  background: #111110;
  color: #8e8e87;
  font: 600 10px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
  text-transform: uppercase;
}

.fixture-band span:first-child { color: #d7e265; }

.a-shell {
  display: grid;
  min-height: calc(100vh - 100px);
  grid-template-columns: 184px minmax(0, 1fr);
}

.a-sidebar {
  position: sticky;
  top: 0;
  display: flex;
  height: calc(100vh - 100px);
  min-height: 620px;
  flex-direction: column;
  border-right: 1px solid #2a2a28;
  background: #0d0d0c;
}

.a-brand {
  display: flex;
  height: 68px;
  align-items: center;
  gap: 10px;
  padding: 0 16px;
  border-bottom: 1px solid #262624;
}

.a-brand-mark {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 1px solid #4d4d48;
  border-radius: 4px;
  background: #1a1a18;
  color: #d7e265;
  font: 700 11px/1 ui-monospace, monospace;
}

.a-brand > span:last-child { display: grid; }
.a-brand strong { font-size: 13px; font-weight: 680; }
.a-brand small { margin-top: 2px; color: #73736d; font-size: 10px; text-transform: uppercase; }

.a-sidebar nav {
  display: grid;
  gap: 3px;
  padding: 14px 9px;
}

.a-sidebar nav button {
  display: flex;
  min-height: 38px;
  align-items: center;
  gap: 10px;
  padding: 0 10px;
  border: 1px solid transparent;
  border-radius: 3px;
  background: transparent;
  color: #92928d;
  font-size: 12px;
  text-align: left;
  cursor: pointer;
}

.a-sidebar nav button:hover { background: #171716; color: #d7d7d2; }
.a-sidebar nav button.active { border-color: #373733; background: #20201e; color: #fff; }
.a-sidebar nav button.active :deep(svg) { color: #d7e265; }
.a-sidebar nav button:focus-visible,
.a-header-actions button:focus-visible,
.a-actions button:focus-visible,
.a-text-button:focus-visible,
.a-setting-row button:focus-visible { outline: 2px solid #d7e265; outline-offset: 2px; }

.a-pool-status {
  margin-top: auto;
  padding: 14px;
  border-top: 1px solid #262624;
  color: #9d9d97;
  font-size: 11px;
}

.a-pool-status > span { display: flex; align-items: center; gap: 7px; }
.a-pool-status dl { display: grid; grid-template-columns: 1fr 1fr; margin: 12px 0 0; }
.a-pool-status dl div { display: grid; gap: 4px; }
.a-pool-status dt { color: #65655f; }
.a-pool-status dd { margin: 0; color: #deded9; font: 650 12px/1 ui-monospace, monospace; }

.a-workspace { min-width: 0; }

.a-page-header {
  display: flex;
  min-height: 68px;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 10px 22px;
  border-bottom: 1px solid #2a2a28;
  background: #10100f;
}

.a-page-header h1 { margin: 0; font-size: 17px; font-weight: 670; line-height: 1.2; }
.a-page-header p { margin: 4px 0 0; color: #7f7f79; font-size: 11px; }
.a-header-actions { display: flex; align-items: center; gap: 10px; color: #71716b; font-size: 10px; }
.a-header-actions button,
.a-actions button {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 1px solid #353532;
  border-radius: 3px;
  background: #171716;
  color: #b9b9b3;
  cursor: pointer;
}

.a-page { display: grid; gap: 12px; padding: 14px; }
.a-metric-strip { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); border: 1px solid #30302d; background: #111110; }
.a-metric-strip > div { display: grid; min-width: 0; gap: 5px; padding: 13px 15px; border-right: 1px solid #292927; }
.a-metric-strip > div:last-child { border-right: 0; }
.a-metric-strip span { overflow: hidden; color: #777771; font-size: 9px; text-overflow: ellipsis; text-transform: uppercase; white-space: nowrap; }
.a-metric-strip strong { font: 650 20px/1.1 ui-monospace, SFMono-Regular, Menlo, monospace; }
.a-metric-strip small { color: #7f7f79; font-size: 10px; }

.a-overview-grid { display: grid; grid-template-columns: minmax(0, 1.7fr) minmax(230px, .65fr); gap: 12px; }
.a-bottom-grid { display: grid; grid-template-columns: minmax(0, 1.25fr) minmax(300px, .75fr); gap: 12px; }
.a-panel { min-width: 0; border: 1px solid #30302d; border-radius: 2px; background: #111110; }
.a-panel-heading { display: flex; min-height: 48px; align-items: center; justify-content: space-between; gap: 16px; padding: 9px 13px; border-bottom: 1px solid #2a2a27; }
.a-panel-heading h2 { margin: 0; color: #eeeeea; font-size: 12px; font-weight: 650; }
.a-panel-heading p { margin: 3px 0 0; color: #696964; font-size: 10px; }
.a-panel-heading > span { color: #73736e; font: 500 10px/1 ui-monospace, monospace; }

.a-capacity-head,
.a-capacity-row { display: grid; grid-template-columns: minmax(170px, 1.05fr) minmax(180px, 1fr) 70px minmax(105px, .65fr); align-items: center; gap: 10px; padding: 0 12px; }
.a-capacity-head { min-height: 28px; border-bottom: 1px solid #252523; color: #61615d; font-size: 9px; text-transform: uppercase; }
.a-capacity-row { min-height: 52px; border-bottom: 1px solid #252523; font-size: 11px; }
.a-capacity-row:last-child { border-bottom: 0; }
.a-account-cell { display: flex; min-width: 0; align-items: center; gap: 9px; }
.a-account-cell > span { display: grid; min-width: 0; gap: 3px; }
.a-account-cell strong,
.a-account-cell small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.a-account-cell strong { color: #e4e4df; font-size: 11px; font-weight: 610; }
.a-account-cell small { color: #686863; font-size: 9px; }
.provider-mark { display: grid; width: 25px; height: 25px; flex: 0 0 auto; place-items: center; border: 1px solid #44443f; border-radius: 3px; background: #1d1d1b; color: #bbb; font: 700 8px/1 ui-monospace, monospace; font-style: normal; }
.provider-mark.openai { color: #77dca2; }
.provider-mark.claude { color: #e8ae77; }
.provider-mark.gemini { color: #9aa9ee; }
.provider-mark.antigravity { color: #d8a0e8; }

.a-quota-cell { display: grid; min-width: 0; gap: 5px; }
.a-quota-label { display: flex; min-width: 0; justify-content: space-between; gap: 8px; }
.a-quota-label strong { font: 650 10px/1 ui-monospace, monospace; }
.a-quota-label small { overflow: hidden; color: #666660; font-size: 8px; text-overflow: ellipsis; white-space: nowrap; }
.a-quota-track { display: block; height: 4px; overflow: hidden; background: #2a2a27; }
.a-quota-track i { display: block; height: 100%; background: #8ccf91; }
.a-quota-track.warning i { background: #d9ae57; }
.a-quota-track.critical i { background: #df6e63; }
.a-quota-track.unknown { border: 1px dashed #4b4b46; background: transparent; }
.a-reset { color: #aeaea7; font: 500 10px/1 ui-monospace, monospace; }
.a-state { display: inline-flex; min-width: 0; align-items: center; gap: 6px; color: #a5a59f; font-size: 10px; }
.a-state.critical { color: #ef8b82; }
.a-state.degraded { color: #d9b668; }
.status-dot { display: inline-block; width: 6px; height: 6px; flex: 0 0 auto; border-radius: 50%; background: #7fc98a; }
.status-dot.degraded,
.degraded .status-dot { background: #d9ae57; }
.status-dot.critical,
.critical .status-dot { background: #dc655c; }
.status-dot.healthy,
.healthy .status-dot { background: #7fc98a; }

.a-provider-panel { align-self: start; }
.a-provider-row { display: grid; min-height: 46px; grid-template-columns: 1fr auto; align-items: center; gap: 4px 10px; padding: 7px 12px; border-bottom: 1px solid #252523; }
.a-provider-row > span:first-child { display: flex; align-items: center; gap: 7px; font-size: 11px; }
.a-provider-row > span:nth-child(2) { font: 650 13px/1 ui-monospace, monospace; }
.a-provider-row small { grid-column: 1 / -1; padding-left: 13px; color: #6c6c66; font-size: 9px; }
.a-attention { display: flex; gap: 9px; margin: 10px; padding: 10px; border: 1px solid #55492a; background: #211d12; color: #e2bb65; }
.a-attention span { display: grid; gap: 3px; }
.a-attention strong { color: #e3d6b6; font-size: 10px; }
.a-attention small { color: #917e50; font-size: 9px; }

.a-route-head,
.a-route-row { display: grid; min-height: 34px; grid-template-columns: minmax(120px, .8fr) minmax(160px, 1.5fr) 70px 45px; align-items: center; gap: 10px; padding: 0 12px; }
.a-route-head { min-height: 28px; color: #61615d; font-size: 9px; text-transform: uppercase; }
.a-route-row { border-top: 1px solid #252523; color: #a8a8a1; font-size: 10px; }
.a-route-row > span:first-child { display: flex; min-width: 0; align-items: center; gap: 7px; }
.a-route-row strong { color: #e0e0db; font-weight: 600; }
.a-route-row small { display: block; margin-top: 2px; color: #666660; }
.a-activity-row { display: grid; min-height: 45px; grid-template-columns: 62px minmax(0, 1fr) 38px; align-items: center; gap: 8px; padding: 0 12px; border-bottom: 1px solid #252523; }
.a-activity-row:last-child { border-bottom: 0; }
.a-activity-row time { color: #65655f; font: 500 9px/1 ui-monospace, monospace; }
.a-activity-row span { display: grid; min-width: 0; gap: 3px; }
.a-activity-row strong { overflow: hidden; font-size: 10px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.a-activity-row small { overflow: hidden; color: #676761; font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }
.a-activity-row code { color: #7fd091; font-size: 9px; }
.a-activity-row code.error { color: #eb766d; }

.a-toolbar { display: flex; align-items: center; gap: 8px; }
.a-search { display: flex; width: min(320px, 35vw); min-width: 180px; height: 34px; align-items: center; gap: 8px; padding: 0 10px; border: 1px solid #343431; background: #121211; color: #666; }
.a-search:focus-within,
.a-toolbar select:focus-visible { outline: 2px solid #d7e265; outline-offset: 2px; }
.a-search input { min-width: 0; flex: 1; border: 0; outline: 0; background: transparent; color: #eee; font-size: 11px; }
.a-toolbar select { height: 34px; padding: 0 30px 0 10px; border: 1px solid #343431; border-radius: 0; background: #121211; color: #b8b8b1; font-size: 11px; }
.a-result-count { margin-left: auto; color: #6f6f69; font: 500 10px/1 ui-monospace, monospace; }
.a-text-button { display: inline-flex; min-height: 32px; align-items: center; gap: 7px; padding: 0 10px; border: 1px solid #44443f; border-radius: 3px; background: #1d1d1b; color: #d6d6d0; font-size: 10px; cursor: pointer; }
.a-account-table-shell { overflow-x: auto; }
.a-account-table { min-width: 1040px; }
.a-account-head,
.a-account-row { display: grid; grid-template-columns: minmax(190px, 1.25fr) 108px minmax(180px, 1.15fr) 68px minmax(115px, .8fr) minmax(105px, .7fr) minmax(105px, .75fr) 68px; align-items: center; gap: 12px; padding: 0 12px; }
.a-account-head { min-height: 34px; border-bottom: 1px solid #30302d; background: #161615; color: #686862; font-size: 9px; text-transform: uppercase; }
.a-account-row { min-height: 62px; border-bottom: 1px solid #272725; color: #a3a39d; font-size: 10px; }
.a-account-row:hover { background: #151514; }
.a-account-row > span { min-width: 0; }
.a-account-row > span > strong { display: block; overflow: hidden; color: #d5d5d0; font-size: 10px; font-weight: 580; text-overflow: ellipsis; white-space: nowrap; }
.a-account-row > span > small { display: block; overflow: hidden; margin-top: 3px; color: #64645f; font-size: 8px; text-overflow: ellipsis; white-space: nowrap; }
.a-actions { display: flex; gap: 4px; }
.a-actions button { width: 26px; height: 26px; }
.a-account-detail { display: grid; grid-template-columns: 1fr 1fr 1fr auto; align-items: center; gap: 18px; padding: 12px 16px 12px 48px; border-bottom: 1px solid #34342f; background: #191916; }
.a-account-detail div { display: grid; min-width: 0; gap: 4px; }
.a-account-detail div span { color: #6f6f68; font-size: 8px; text-transform: uppercase; }
.a-account-detail div strong { overflow: hidden; font-size: 9px; font-weight: 550; text-overflow: ellipsis; white-space: nowrap; }
.a-account-detail button { display: inline-flex; min-height: 30px; align-items: center; gap: 6px; border: 1px solid #45453f; background: #20201d; color: #ddd; font-size: 9px; }

.a-models { grid-template-columns: 1fr; }
.a-model-route-grid { grid-template-columns: minmax(140px, .8fr) minmax(220px, 1.5fr) minmax(220px, 1.4fr) 70px 70px; }
.a-model-grid { grid-template-columns: minmax(160px, 1.1fr) minmax(180px, 1.2fr) minmax(130px, .9fr) 70px 70px 70px; }
.a-settings { grid-template-columns: minmax(0, 1.5fr) minmax(280px, .75fr); align-items: start; }
.a-setting-row { display: flex; min-height: 64px; align-items: center; justify-content: space-between; gap: 20px; padding: 10px 14px; border-bottom: 1px solid #272725; }
.a-setting-row:last-child { border-bottom: 0; }
.a-setting-row > span { display: grid; gap: 4px; }
.a-setting-row strong { font-size: 11px; font-weight: 600; }
.a-setting-row small { color: #6d6d67; font-size: 9px; }
.a-setting-row button { display: inline-flex; min-height: 30px; align-items: center; gap: 8px; border: 0; background: transparent; color: #bdbdb6; font: 550 10px/1 ui-monospace, monospace; cursor: pointer; }
.a-definition-list { margin: 0; }
.a-definition-list div { display: flex; min-height: 48px; align-items: center; justify-content: space-between; gap: 16px; padding: 8px 13px; border-bottom: 1px solid #272725; }
.a-definition-list div:last-child { border-bottom: 0; }
.a-definition-list dt { color: #777771; font-size: 10px; }
.a-definition-list dd { margin: 0; color: #d1d1cb; font: 550 9px/1 ui-monospace, monospace; text-align: right; }
.sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; }

@media (max-width: 1180px) {
  .a-shell { grid-template-columns: 164px minmax(0, 1fr); }
  .a-sidebar nav button { padding-inline: 8px; }
  .a-metric-strip { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .a-metric-strip > div:nth-child(3) { border-right: 0; }
  .a-metric-strip > div:nth-child(n + 4) { border-top: 1px solid #292927; }
  .a-overview-grid,
  .a-bottom-grid { grid-template-columns: 1fr; }
  .a-provider-panel { display: grid; grid-template-columns: repeat(4, 1fr); }
  .a-provider-panel .a-panel-heading,
  .a-provider-panel .a-attention { grid-column: 1 / -1; }
  .a-provider-row { border-right: 1px solid #252523; }
  .a-settings { grid-template-columns: 1fr; }
}

@media (max-width: 768px) {
  .fixture-band { justify-content: center; }
  .fixture-band span:not(:first-child) { display: none; }
  .a-shell { display: block; }
  .a-sidebar { position: static; height: auto; min-height: 0; border-right: 0; border-bottom: 1px solid #2a2a28; }
  .a-brand { height: 50px; }
  .a-sidebar nav { display: flex; overflow-x: auto; padding: 6px 8px; }
  .a-sidebar nav button { min-width: max-content; min-height: 34px; }
  .a-pool-status { display: none; }
  .a-page-header { min-height: 60px; padding: 9px 12px; }
  .a-header-actions > span { display: none; }
  .a-page { padding: 10px; }
  .a-metric-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .a-metric-strip > div { border-top: 1px solid #292927; }
  .a-metric-strip > div:nth-child(-n + 2) { border-top: 0; }
  .a-metric-strip > div:nth-child(even) { border-right: 0; }
  .a-capacity-table { overflow-x: auto; }
  .a-bottom-grid > .a-panel:first-child { overflow-x: auto; }
  .a-capacity-head,
  .a-capacity-row { min-width: 650px; }
  .a-provider-panel { grid-template-columns: 1fr 1fr; }
  .a-toolbar { flex-wrap: wrap; }
  .a-search { width: 100%; }
  .a-result-count { margin-left: 0; }
  .a-account-table-shell { overflow-x: auto; }
  .a-account-detail { min-width: 1040px; }
  .a-models .a-panel { overflow-x: auto; }
  .a-model-route-grid,
  .a-model-grid { min-width: 820px; }
}
</style>
