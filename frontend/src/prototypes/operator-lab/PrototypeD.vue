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
  type ProviderId,
} from './data'

const props = defineProps<{ page: LabPage }>()
const emit = defineEmits<{ navigate: [page: LabPage] }>()
const selectedAccountId = ref(101)
const providerFilter = ref<ProviderId | 'all'>('all')
const actionMessage = ref('')
const resetBuckets = ['<1h', '1-4h', '>4h', 'unknown'] as const

const navIcons: Record<LabPage, 'home' | 'users' | 'swap' | 'cog'> = {
  overview: 'home',
  accounts: 'users',
  models: 'swap',
  settings: 'cog',
}

const filteredAccounts = computed(() => providerFilter.value === 'all'
  ? labAccounts
  : labAccounts.filter((account) => account.provider === providerFilter.value))

const selectedAccount = computed(() => filteredAccounts.value.find((account) => account.id === selectedAccountId.value) || filteredAccounts.value[0] || labAccounts[0])

function capacityStyle(account: LabAccount) {
  return { width: `${account.remaining ?? 0}%` }
}

function runFixtureAction(label: string) {
  actionMessage.value = `${label} preview completed against fixture data.`
}
</script>

<template>
  <div class="prototype-d" data-testid="prototype-d">
    <div class="d-dev-banner"><span>Prototype D · Sub2API Hybrid</span><span>Shared synthetic fixture · local only · no production connection</span></div>
    <div class="d-shell">
      <aside class="d-sidebar">
        <div class="d-brand"><span>S2</span><div><strong>Sub2API</strong><small>Operator Console</small></div></div>
        <nav aria-label="Prototype D sections">
          <button v-for="item in labPages" :key="item.key" type="button" :class="{ active: props.page === item.key }" :aria-current="props.page === item.key ? 'page' : undefined" @click="emit('navigate', item.key)"><Icon :name="navIcons[item.key]" size="sm" /><span>{{ item.label }}</span><i v-if="item.key === 'accounts'">3</i></button>
        </nav>
        <div class="d-sidebar-pools">
          <span class="d-sidebar-label">PROVIDER POOLS</span>
          <div v-for="provider in labProviders" :key="provider.id"><span><i :class="provider.health" />{{ provider.label }}</span><strong>{{ provider.averageRemaining }}%</strong></div>
        </div>
        <footer><span><i /> Fixture gateway</span><small>Updated 12 seconds ago</small></footer>
      </aside>

      <section class="d-workspace">
        <header class="d-topbar">
          <div class="d-breadcrumb"><span>Operator</span><Icon name="chevronRight" size="xs" /><strong>{{ labPages.find((item) => item.key === props.page)?.label }}</strong></div>
          <div class="d-topbar-actions"><span>{{ labSummary.rpm }} RPM</span><button type="button" title="Refresh fixture snapshot" aria-label="Refresh fixture snapshot"><Icon name="refresh" size="sm" /></button><button type="button" title="Prototype settings" aria-label="Prototype settings"><Icon name="cog" size="sm" /></button><span class="d-avatar">OP</span></div>
        </header>

        <main class="d-content">
          <div v-if="props.page === 'overview'" class="d-overview">
            <div class="d-page-title"><div><h1>System overview</h1><p>Capacity, routing health, and current request pressure</p></div><span class="d-status"><i /> Operational with 3 account alerts</span></div>

            <section class="d-summary-row">
              <div><span>REQUESTS TODAY</span><strong>{{ labSummary.requests }}</strong><small>{{ labSummary.rpm }} RPM current</small></div>
              <div><span>SUCCESS RATE</span><strong>{{ labSummary.success }}</strong><small>{{ labSummary.errors }} exceptions / {{ labSummary.period }}</small></div>
              <div><span>ROUTABLE ACCOUNTS</span><strong>{{ labSummary.routableAccounts }} <b>/ {{ labSummary.totalAccounts }}</b></strong><small>3 require attention</small></div>
              <div><span>ACTUAL COST</span><strong>{{ labSummary.cost }}</strong><small>{{ labSummary.accountCost }} upstream</small></div>
            </section>

            <div class="d-overview-main">
              <section class="d-panel d-horizon-panel">
                <div class="d-panel-title"><div><h2>Capacity reset horizon</h2><p>Where account capacity sits relative to its next reported reset</p></div><span>Normalized % remaining</span></div>
                <div class="d-horizon-table">
                  <div class="d-horizon-head"><span>Provider</span><span>Reset &lt;1h</span><span>1–4h</span><span>Beyond 4h</span><span>Unknown</span></div>
                  <div v-for="provider in labProviders" :key="provider.id" class="d-horizon-row">
                    <span class="d-horizon-provider"><i :class="provider.health" /><strong>{{ provider.label }}</strong><small>{{ provider.averageRemaining }}% known avg</small></span>
                    <div v-for="bucket in resetBuckets" :key="bucket" class="d-horizon-cell">
                      <span v-for="account in provider.accounts.filter((item) => item.resetBucket === bucket)" :key="account.id" :class="['d-account-chip', account.health]">
                        <span><strong>{{ account.label }}</strong><small>{{ account.healthLabel }}</small></span><b>{{ account.remaining === null ? '?' : `${account.remaining}%` }}</b>
                      </span>
                      <small v-if="!provider.accounts.some((item) => item.resetBucket === bucket)">—</small>
                    </div>
                  </div>
                </div>
              </section>

              <section class="d-panel d-routing-panel">
                <div class="d-panel-title"><div><h2>Routing health</h2><p>Composite route readiness</p></div></div>
                <div v-for="route in labRoutes" :key="route.name" class="d-routing-row">
                  <span><i :class="route.status" /><span><strong>{{ route.name }}</strong><small>{{ route.capacity }}</small></span></span><span><strong>{{ route.success }}</strong><small>{{ route.rpm }} RPM · {{ route.latency }}</small></span>
                </div>
                <button type="button" @click="emit('navigate', 'models')">Open routing workspace <Icon name="arrowRight" size="sm" /></button>
              </section>
            </div>

            <div class="d-lower-grid">
              <section class="d-panel d-account-watch">
                <div class="d-panel-title"><div><h2>Account watchlist</h2><p>Lowest or unknown capacity first</p></div><button type="button" @click="emit('navigate', 'accounts')">View all</button></div>
                <div v-for="account in [...labAccounts].sort((a, b) => (a.remaining ?? -1) - (b.remaining ?? -1)).slice(0, 4)" :key="account.id" class="d-watch-row">
                  <span><i :class="account.health" /><span><strong>{{ account.label }}</strong><small>{{ account.provider }} · {{ account.scheduleLabel }}</small></span></span>
                  <span class="d-mini-capacity"><i :class="account.health" :style="capacityStyle(account)" /></span><b>{{ account.remaining === null ? 'Unknown' : `${account.remaining}%` }}</b><small>{{ account.resetLabel }}</small>
                </div>
              </section>
              <section class="d-panel d-activity-panel">
                <div class="d-panel-title"><div><h2>Recent activity</h2><p>Resolved provider requests</p></div></div>
                <div v-for="event in labActivity" :key="event.time" class="d-activity-row"><time>{{ event.time }}</time><span><strong>{{ event.model }}</strong><small>{{ event.account }} via {{ event.route }}</small></span><code :class="{ error: event.status !== '200' }">{{ event.status }}</code></div>
              </section>
            </div>
          </div>

          <div v-else-if="props.page === 'accounts'" class="d-accounts">
            <div class="d-page-title"><div><h1>Account capacity</h1><p>Compare the pool, then inspect one account without leaving context</p></div><button class="d-primary-button" type="button"><Icon name="plus" size="sm" /> Add account</button></div>
            <div class="d-account-filter" role="group" aria-label="Filter accounts by provider"><button type="button" :class="{ active: providerFilter === 'all' }" @click="providerFilter = 'all'">All <span>{{ labAccounts.length }}</span></button><button v-for="provider in labProviders" :key="provider.id" type="button" :class="{ active: providerFilter === provider.id }" @click="providerFilter = provider.id">{{ provider.label }} <span>{{ provider.accounts.length }}</span></button></div>

            <div class="d-account-split">
              <section class="d-account-list" aria-label="Accounts">
                <header><span>{{ filteredAccounts.length }} accounts</span><span>Capacity remaining</span></header>
                <button v-for="account in filteredAccounts" :key="account.id" type="button" :class="['d-account-list-row', { selected: selectedAccount.id === account.id }]" :aria-pressed="selectedAccount.id === account.id" @click="selectedAccountId = account.id; actionMessage = ''">
                  <span class="d-account-list-name"><i :class="account.health" /><span><strong>{{ account.label }}</strong><small>{{ account.identity }} · {{ account.plan }}</small></span></span>
                  <span class="d-account-list-capacity"><strong>{{ account.remaining === null ? 'Unknown' : `${account.remaining}%` }}</strong><span><i :class="account.health" :style="capacityStyle(account)" /></span><small>{{ account.resetLabel }} reset</small></span>
                  <Icon name="chevronRight" size="sm" />
                </button>
              </section>

              <section class="d-account-detail">
                <header><span class="d-detail-provider"><i :class="selectedAccount.provider">{{ selectedAccount.provider.slice(0, 2).toUpperCase() }}</i><span><small>{{ selectedAccount.provider.toUpperCase() }} ACCOUNT #{{ selectedAccount.id }}</small><h2>{{ selectedAccount.label }}</h2><p>{{ selectedAccount.identity }} · {{ selectedAccount.plan }}</p></span></span><span :class="['d-detail-health', selectedAccount.health]"><i />{{ selectedAccount.healthLabel }}</span></header>
                <div class="d-detail-capacity">
                  <div><span>CAPACITY REMAINING</span><strong>{{ selectedAccount.remaining === null ? 'Unknown' : `${selectedAccount.remaining}%` }}</strong><small>{{ selectedAccount.nativeQuota }}</small></div>
                  <span class="d-detail-meter"><i :class="selectedAccount.health" :style="capacityStyle(selectedAccount)" /></span>
                  <dl><div><dt>Next reset</dt><dd>{{ selectedAccount.resetLabel }}</dd></div><div><dt>Scheduling</dt><dd>{{ selectedAccount.scheduleLabel }}</dd></div><div><dt>Concurrency</dt><dd>{{ selectedAccount.concurrency }}</dd></div></dl>
                </div>
                <div class="d-detail-section"><h3>Models available</h3><div class="d-tags"><span v-for="model in selectedAccount.models" :key="model">{{ model }}</span></div></div>
                <div class="d-detail-section"><h3>Routing groups</h3><div v-for="group in selectedAccount.groups" :key="group" class="d-detail-group"><span><Icon name="swap" size="sm" />{{ group }}</span><small>Active</small></div></div>
                <div class="d-detail-stats"><div><span>REQUESTS TODAY</span><strong>{{ selectedAccount.requests.toLocaleString() }}</strong></div><div><span>TOKENS</span><strong>{{ selectedAccount.tokens }}</strong></div><div><span>ACTUAL COST</span><strong>{{ selectedAccount.cost }}</strong></div></div>
                <div v-if="selectedAccount.recentFailure" class="d-detail-alert"><Icon name="exclamationTriangle" size="sm" /><span><strong>Recent failure</strong><small>{{ selectedAccount.recentFailure }}</small></span></div>
                <div class="d-detail-actions"><button type="button" @click="runFixtureAction('Connection test')"><Icon name="play" size="sm" /> Test</button><button type="button" @click="runFixtureAction('Usage refresh')"><Icon name="refresh" size="sm" /> Refresh usage</button><button type="button" @click="runFixtureAction('Edit')"><Icon name="edit" size="sm" /> Edit</button><button type="button" title="More fixture actions" aria-label="More fixture actions"><Icon name="more" size="sm" /></button></div>
                <p v-if="actionMessage" class="d-action-message" role="status">{{ actionMessage }}</p>
              </section>
            </div>
          </div>

          <div v-else-if="props.page === 'models'" class="d-models">
            <div class="d-page-title"><div><h1>Models & routing</h1><p>Model access, provider resolution, and composite route behavior</p></div><button class="d-primary-button" type="button"><Icon name="plus" size="sm" /> New route</button></div>
            <div class="d-routing-layout">
              <section class="d-panel d-route-list"><div class="d-panel-title"><div><h2>Routes</h2><p>Current external model aliases</p></div></div><button v-for="route in labRoutes" :key="route.name" type="button"><span><i :class="route.status" /><span><strong>{{ route.name }}</strong><small>{{ route.type }} · {{ route.destinations.length }} destinations</small></span></span><span><strong>{{ route.success }}</strong><Icon name="chevronRight" size="sm" /></span></button></section>
              <section class="d-panel d-route-detail"><div class="d-panel-title"><div><h2>coding-default</h2><p>Composite coding route</p></div><span class="d-status"><i /> Healthy</span></div><div class="d-route-path"><span v-for="(destination, index) in labRoutes[0].destinations" :key="destination"><i>{{ index + 1 }}</i><strong>{{ destination }}</strong><small>{{ labRoutes[0].groups[index] }}</small></span></div><dl><div><dt>Success</dt><dd>{{ labRoutes[0].success }}</dd></div><div><dt>Latency</dt><dd>{{ labRoutes[0].latency }}</dd></div><div><dt>Current load</dt><dd>{{ labRoutes[0].rpm }} RPM</dd></div><div><dt>Account pool</dt><dd>{{ labSummary.routableAccounts }} routable</dd></div></dl></section>
            </div>
            <section class="d-panel d-model-table"><div class="d-panel-title"><div><h2>Observed models</h2><p>Resolved traffic in the shared fixture window</p></div></div><header><span>Model</span><span>Provider path</span><span>Route</span><span>Health</span><span>Requests</span><span>Tokens</span><span>Latency</span></header><div v-for="model in labModels" :key="model.name"><span><strong>{{ model.name }}</strong></span><span>{{ model.provider }}</span><span>{{ model.route }}</span><span class="d-status"><i :class="model.status" />{{ model.status }}</span><span>{{ model.requests }}</span><span>{{ model.tokens }}</span><span>{{ model.latency }}</span></div></section>
          </div>

          <div v-else class="d-settings">
            <div class="d-page-title"><div><h1>Operator settings</h1><p>Capacity-aware routing and notification defaults</p></div></div>
            <div class="d-settings-layout"><nav aria-label="Settings categories"><button class="active" type="button"><Icon name="swap" size="sm" /> Routing & capacity</button><button type="button"><Icon name="bell" size="sm" /> Alerts</button><button type="button"><Icon name="server" size="sm" /> Gateway</button><button type="button"><Icon name="shield" size="sm" /> Review safety</button></nav><section class="d-panel d-settings-panel"><div class="d-panel-title"><div><h2>Routing & capacity</h2><p>Representative values from the fixture snapshot</p></div></div><div v-for="setting in labSettings" :key="setting.title" class="d-setting-row"><span><strong>{{ setting.title }}</strong><small>{{ setting.description }}</small></span><button type="button">{{ setting.value }} <Icon name="chevronRight" size="sm" /></button></div></section></div>
          </div>
        </main>
      </section>
    </div>
  </div>
</template>

<style scoped>
.prototype-d { min-height: calc(100vh - 72px); background: #0a0a0a; color: #ededeb; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; font-variant-numeric: tabular-nums; }
.d-dev-banner { display: flex; min-height: 27px; align-items: center; justify-content: space-between; padding: 0 16px; border-bottom: 1px solid #2e2e2b; background: #121211; color: #777770; font: 600 8px/1 ui-monospace, monospace; text-transform: uppercase; }
.d-dev-banner span:first-child { color: #d3db7c; }
.d-shell { display: grid; min-height: calc(100vh - 99px); grid-template-columns: 210px minmax(0, 1fr); }
.d-sidebar { position: sticky; top: 0; display: flex; height: calc(100vh - 99px); min-height: 640px; flex-direction: column; border-right: 1px solid #2f2f2c; background: #0f0f0e; }
.d-brand { display: flex; min-height: 66px; align-items: center; gap: 10px; padding: 0 15px; border-bottom: 1px solid #292927; }
.d-brand > span { display: grid; width: 30px; height: 30px; place-items: center; border: 1px solid #53534d; border-radius: 4px; background: #191918; color: #d5dd7c; font: 700 10px/1 ui-monospace, monospace; }
.d-brand > div { display: grid; gap: 2px; }
.d-brand strong { font-size: 13px; }
.d-brand small { color: #6d6d67; font-size: 9px; }
.d-sidebar nav { display: grid; gap: 3px; padding: 12px 9px; }
.d-sidebar nav button { display: grid; min-height: 38px; grid-template-columns: 18px 1fr auto; align-items: center; gap: 9px; padding: 0 10px; border: 1px solid transparent; border-radius: 4px; background: transparent; color: #85857f; font-size: 11px; text-align: left; cursor: pointer; }
.d-sidebar nav button:hover { background: #171716; color: #cecec9; }
.d-sidebar nav button.active { border-color: #393935; background: #20201e; color: #fff; }
.d-sidebar nav button.active :deep(svg) { color: #d3dc7b; }
.d-sidebar nav button > i { display: grid; min-width: 19px; height: 17px; place-items: center; border-radius: 3px; background: #3e3520; color: #e0bb67; font: 650 8px/1 ui-monospace, monospace; font-style: normal; }
.d-sidebar button:focus-visible,
.d-topbar button:focus-visible,
.d-primary-button:focus-visible,
.d-account-filter button:focus-visible,
.d-account-list-row:focus-visible,
.d-detail-actions button:focus-visible,
.d-settings button:focus-visible { outline: 2px solid #d3dc7b; outline-offset: 2px; }
.d-sidebar-pools { display: grid; gap: 2px; padding: 14px 12px; border-top: 1px solid #292927; }
.d-sidebar-label { margin-bottom: 7px; color: #5f5f59; font: 650 8px/1 ui-monospace, monospace; }
.d-sidebar-pools > div { display: flex; min-height: 29px; align-items: center; justify-content: space-between; gap: 8px; color: #7c7c76; font-size: 9px; }
.d-sidebar-pools > div span { display: flex; align-items: center; gap: 7px; }
.d-sidebar-pools i,
.d-sidebar footer i,
.d-status i,
.d-routing-row i,
.d-horizon-provider i,
.d-watch-row i,
.d-account-list-name > i,
.d-detail-health i,
.d-route-list i { width: 6px; height: 6px; flex: 0 0 auto; border-radius: 50%; background: #79c684; }
i.degraded { background: #d2aa54; }
i.critical { background: #dd685f; }
.d-sidebar-pools strong { color: #b5b5ae; font: 550 9px/1 ui-monospace, monospace; }
.d-sidebar footer { display: grid; gap: 5px; margin-top: auto; padding: 13px 15px; border-top: 1px solid #292927; }
.d-sidebar footer span { display: flex; align-items: center; gap: 7px; color: #a6a69f; font-size: 9px; }
.d-sidebar footer small { color: #62625d; font-size: 8px; }
.d-workspace { min-width: 0; }
.d-topbar { display: flex; min-height: 52px; align-items: center; justify-content: space-between; gap: 18px; padding: 0 16px; border-bottom: 1px solid #2f2f2c; background: #111110; }
.d-breadcrumb { display: flex; align-items: center; gap: 7px; color: #686862; font-size: 10px; }
.d-breadcrumb strong { color: #c9c9c3; font-weight: 580; }
.d-topbar-actions { display: flex; align-items: center; gap: 6px; }
.d-topbar-actions > span:first-child { margin-right: 5px; color: #74746e; font: 550 9px/1 ui-monospace, monospace; }
.d-topbar-actions button { display: grid; width: 30px; height: 30px; place-items: center; border: 1px solid #363633; border-radius: 3px; background: #171716; color: #9c9c96; cursor: pointer; }
.d-avatar { display: grid; width: 29px; height: 29px; margin-left: 3px; place-items: center; border: 1px solid #474741; border-radius: 50%; background: #20201e; color: #bfc2a4; font: 650 8px/1 ui-monospace, monospace; }
.d-content { padding: 16px; }
.d-overview,
.d-accounts,
.d-models,
.d-settings { display: grid; gap: 13px; }
.d-page-title { display: flex; min-height: 53px; align-items: center; justify-content: space-between; gap: 18px; }
.d-page-title h1 { margin: 0; font-size: 19px; font-weight: 650; }
.d-page-title p { margin: 5px 0 0; color: #71716b; font-size: 10px; }
.d-status { display: inline-flex; align-items: center; gap: 7px; color: #94c99a; font-size: 9px; white-space: nowrap; }
.d-summary-row { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); border: 1px solid #333330; background: #111110; }
.d-summary-row > div { display: grid; min-width: 0; gap: 6px; padding: 13px 15px; border-right: 1px solid #2d2d2a; }
.d-summary-row > div:last-child { border-right: 0; }
.d-summary-row span { color: #666660; font: 600 8px/1 ui-monospace, monospace; }
.d-summary-row strong { font: 640 20px/1 ui-monospace, monospace; }
.d-summary-row strong b { color: #666660; font-size: 10px; font-weight: 500; }
.d-summary-row small { color: #777771; font-size: 8px; }
.d-overview-main { display: grid; grid-template-columns: minmax(0, 1.65fr) minmax(260px, .65fr); gap: 13px; }
.d-lower-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 13px; }
.d-panel { min-width: 0; border: 1px solid #343431; border-radius: 4px; background: #111110; }
.d-panel-title { display: flex; min-height: 47px; align-items: center; justify-content: space-between; gap: 14px; padding: 8px 12px; border-bottom: 1px solid #2d2d2a; }
.d-panel-title h2 { margin: 0; font-size: 11px; font-weight: 630; }
.d-panel-title p { margin: 3px 0 0; color: #65655f; font-size: 8px; }
.d-panel-title > span { color: #6b6b65; font: 500 8px/1 ui-monospace, monospace; }
.d-panel-title > button { border: 0; background: transparent; color: #b9bd96; font-size: 9px; cursor: pointer; }
.d-horizon-table { overflow-x: auto; }
.d-horizon-head,
.d-horizon-row { display: grid; min-width: 740px; grid-template-columns: 135px repeat(4, minmax(135px, 1fr)); }
.d-horizon-head { min-height: 29px; align-items: center; color: #62625d; font: 600 7px/1 ui-monospace, monospace; text-transform: uppercase; }
.d-horizon-head > span { padding: 0 9px; }
.d-horizon-row { min-height: 70px; border-top: 1px solid #292927; }
.d-horizon-provider { display: grid; grid-template-columns: 7px 1fr; align-content: center; gap: 3px 7px; padding: 8px 10px; border-right: 1px solid #292927; }
.d-horizon-provider strong { font-size: 9px; }
.d-horizon-provider small { grid-column: 2; color: #62625d; font-size: 7px; }
.d-horizon-cell { display: grid; align-content: center; gap: 5px; padding: 7px; border-right: 1px solid #292927; }
.d-horizon-cell:last-child { border-right: 0; }
.d-horizon-cell > small { color: #4d4d49; text-align: center; }
.d-account-chip { display: flex; min-width: 0; min-height: 45px; align-items: center; justify-content: space-between; gap: 7px; padding: 6px 7px; border-left: 2px solid #71be7b; background: #181a17; }
.d-account-chip.degraded { border-left-color: #d0a54e; }
.d-account-chip.critical { border-left-color: #d75e56; }
.d-account-chip > span { display: grid; min-width: 0; gap: 3px; }
.d-account-chip strong,
.d-account-chip small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.d-account-chip strong { font-size: 8px; font-weight: 600; }
.d-account-chip small { color: #65655f; font-size: 7px; }
.d-account-chip b { color: #d8dbd5; font: 600 10px/1 ui-monospace, monospace; }
.d-routing-row { display: flex; min-height: 58px; align-items: center; justify-content: space-between; gap: 10px; padding: 8px 11px; border-bottom: 1px solid #292927; }
.d-routing-row > span:first-child { display: flex; min-width: 0; align-items: center; gap: 8px; }
.d-routing-row > span > span { display: grid; min-width: 0; gap: 3px; }
.d-routing-row > span:last-child { display: grid; justify-items: end; gap: 3px; text-align: right; }
.d-routing-row strong { overflow: hidden; font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }
.d-routing-row small { overflow: hidden; color: #62625d; font-size: 7px; text-overflow: ellipsis; white-space: nowrap; }
.d-routing-panel > button { display: flex; width: 100%; min-height: 36px; align-items: center; justify-content: space-between; padding: 0 11px; border: 0; background: #171716; color: #b9bd96; font-size: 9px; cursor: pointer; }
.d-watch-row { display: grid; min-height: 46px; grid-template-columns: minmax(150px, 1fr) minmax(80px, .7fr) 50px 60px; align-items: center; gap: 9px; padding: 0 11px; border-bottom: 1px solid #292927; }
.d-watch-row:last-child { border-bottom: 0; }
.d-watch-row > span:first-child { display: flex; min-width: 0; align-items: center; gap: 8px; }
.d-watch-row > span:first-child > span { display: grid; min-width: 0; gap: 2px; }
.d-watch-row strong,
.d-watch-row small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.d-watch-row strong { font-size: 8px; }
.d-watch-row small { color: #60605b; font-size: 7px; }
.d-watch-row b { font: 600 9px/1 ui-monospace, monospace; }
.d-mini-capacity { height: 5px; overflow: hidden; background: #292927; }
.d-mini-capacity i { display: block; height: 100%; background: #75bd7e; }
.d-mini-capacity i.degraded { background: #c9a24d; }
.d-mini-capacity i.critical { background: #d16058; }
.d-activity-row { display: grid; min-height: 37px; grid-template-columns: 55px minmax(0, 1fr) 35px; align-items: center; gap: 9px; padding: 0 11px; border-bottom: 1px solid #292927; }
.d-activity-row:last-child { border-bottom: 0; }
.d-activity-row time { color: #65655f; font: 500 7px/1 ui-monospace, monospace; }
.d-activity-row span { display: grid; min-width: 0; gap: 2px; }
.d-activity-row strong,
.d-activity-row small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.d-activity-row strong { font-size: 8px; }
.d-activity-row small { color: #60605b; font-size: 7px; }
.d-activity-row code { color: #78c383; font-size: 8px; }
.d-activity-row code.error { color: #dc675e; }

.d-primary-button { display: inline-flex; min-height: 34px; align-items: center; gap: 7px; padding: 0 11px; border: 1px solid #55564a; border-radius: 4px; background: #e2e4d7; color: #161714; font-size: 10px; font-weight: 630; cursor: pointer; }
.d-account-filter { display: flex; gap: 4px; overflow-x: auto; border-bottom: 1px solid #343431; }
.d-account-filter button { display: flex; min-width: max-content; min-height: 36px; align-items: center; gap: 7px; padding: 0 11px; border: 0; border-bottom: 2px solid transparent; background: transparent; color: #74746e; font-size: 9px; cursor: pointer; }
.d-account-filter button span { display: grid; min-width: 17px; height: 17px; place-items: center; border-radius: 3px; background: #232321; color: #8d8d86; font: 600 7px/1 ui-monospace, monospace; }
.d-account-filter button.active { border-bottom-color: #d3dc7b; color: #f1f1ed; }
.d-account-split { display: grid; grid-template-columns: minmax(330px, .8fr) minmax(460px, 1.2fr); min-height: 620px; border: 1px solid #343431; border-radius: 4px; background: #111110; }
.d-account-list { min-width: 0; border-right: 1px solid #343431; }
.d-account-list > header { display: flex; min-height: 35px; align-items: center; justify-content: space-between; padding: 0 11px; border-bottom: 1px solid #2d2d2a; color: #65655f; font: 600 7px/1 ui-monospace, monospace; text-transform: uppercase; }
.d-account-list-row { display: grid; width: 100%; min-height: 72px; grid-template-columns: minmax(0, 1fr) 115px 18px; align-items: center; gap: 10px; padding: 8px 10px; border: 0; border-bottom: 1px solid #292927; background: transparent; color: #a9a9a2; text-align: left; cursor: pointer; }
.d-account-list-row:hover { background: #171716; }
.d-account-list-row.selected { box-shadow: inset 2px 0 #d3dc7b; background: #1b1b19; }
.d-account-list-name { display: flex; min-width: 0; align-items: center; gap: 8px; }
.d-account-list-name > span { display: grid; min-width: 0; gap: 4px; }
.d-account-list-name strong,
.d-account-list-name small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.d-account-list-name strong { color: #e1e1dc; font-size: 9px; }
.d-account-list-name small { color: #62625d; font-size: 7px; }
.d-account-list-capacity { display: grid; grid-template-columns: 1fr 40px; align-items: center; gap: 4px 7px; }
.d-account-list-capacity > span { height: 5px; overflow: hidden; background: #2b2b28; }
.d-account-list-capacity > span i { display: block; height: 100%; background: #75bd7e; }
.d-account-list-capacity > span i.degraded { background: #caa24d; }
.d-account-list-capacity > span i.critical { background: #d16058; }
.d-account-list-capacity strong { grid-column: 2; grid-row: 1; font: 600 9px/1 ui-monospace, monospace; text-align: right; }
.d-account-list-capacity small { grid-column: 1 / -1; color: #61615c; font-size: 7px; }
.d-account-detail { min-width: 0; padding-bottom: 12px; }
.d-account-detail > header { display: flex; min-height: 86px; align-items: center; justify-content: space-between; gap: 18px; padding: 12px 15px; border-bottom: 1px solid #2d2d2a; }
.d-detail-provider { display: flex; min-width: 0; align-items: center; gap: 11px; }
.d-detail-provider > i { display: grid; width: 36px; height: 36px; flex: 0 0 auto; place-items: center; border: 1px solid #4b4b45; border-radius: 4px; background: #1d1d1b; color: #d6d9c2; font: 700 9px/1 ui-monospace, monospace; font-style: normal; }
.d-detail-provider > span { min-width: 0; }
.d-detail-provider small { color: #65655f; font: 600 7px/1 ui-monospace, monospace; }
.d-detail-provider h2 { overflow: hidden; margin: 5px 0 0; font-size: 15px; text-overflow: ellipsis; white-space: nowrap; }
.d-detail-provider p { overflow: hidden; margin: 4px 0 0; color: #777771; font-size: 8px; text-overflow: ellipsis; white-space: nowrap; }
.d-detail-health { display: inline-flex; align-items: center; gap: 6px; color: #8ac992; font-size: 8px; white-space: nowrap; }
.d-detail-health.degraded { color: #d6af58; }
.d-detail-health.critical { color: #e17168; }
.d-detail-capacity { display: grid; grid-template-columns: 180px minmax(120px, 1fr); gap: 12px 18px; padding: 14px 15px; border-bottom: 1px solid #2d2d2a; }
.d-detail-capacity > div:first-child { display: grid; gap: 5px; }
.d-detail-capacity > div:first-child span { color: #64645e; font: 600 7px/1 ui-monospace, monospace; }
.d-detail-capacity > div:first-child strong { font: 640 24px/1 ui-monospace, monospace; }
.d-detail-capacity > div:first-child small { color: #71716b; font-size: 8px; }
.d-detail-meter { align-self: center; height: 10px; overflow: hidden; background: #292927; }
.d-detail-meter i { display: block; height: 100%; background: #76bd7f; }
.d-detail-meter i.degraded { background: #cba34e; }
.d-detail-meter i.critical { background: #d16058; }
.d-detail-capacity dl { display: grid; grid-column: 1 / -1; grid-template-columns: repeat(3, 1fr); margin: 0; border-top: 1px solid #292927; }
.d-detail-capacity dl div { display: grid; gap: 4px; padding: 10px 8px 0; border-right: 1px solid #292927; }
.d-detail-capacity dl div:last-child { border-right: 0; }
.d-detail-capacity dt { color: #60605a; font-size: 7px; }
.d-detail-capacity dd { margin: 0; color: #c6c6c0; font-size: 9px; }
.d-detail-section { padding: 12px 15px; border-bottom: 1px solid #2d2d2a; }
.d-detail-section h3 { margin: 0 0 9px; color: #70706a; font: 600 7px/1 ui-monospace, monospace; text-transform: uppercase; }
.d-tags { display: flex; flex-wrap: wrap; gap: 5px; }
.d-tags span { padding: 5px 7px; border: 1px solid #363633; border-radius: 3px; background: #181817; color: #b4b4ae; font: 500 8px/1 ui-monospace, monospace; }
.d-detail-group { display: flex; min-height: 29px; align-items: center; justify-content: space-between; gap: 10px; }
.d-detail-group span { display: flex; align-items: center; gap: 7px; color: #b9b9b2; font-size: 8px; }
.d-detail-group small { color: #7ec488; font-size: 7px; }
.d-detail-stats { display: grid; grid-template-columns: repeat(3, 1fr); border-bottom: 1px solid #2d2d2a; }
.d-detail-stats > div { display: grid; gap: 5px; padding: 11px 15px; border-right: 1px solid #2d2d2a; }
.d-detail-stats > div:last-child { border-right: 0; }
.d-detail-stats span { color: #61615c; font: 600 7px/1 ui-monospace, monospace; }
.d-detail-stats strong { font: 610 12px/1 ui-monospace, monospace; }
.d-detail-alert { display: flex; gap: 9px; margin: 10px 15px 0; padding: 9px 10px; border: 1px solid #55482c; background: #201c12; color: #d7af59; }
.d-detail-alert span { display: grid; gap: 3px; }
.d-detail-alert strong { color: #d7c18f; font-size: 8px; }
.d-detail-alert small { color: #917e50; font-size: 7px; }
.d-detail-actions { display: flex; gap: 6px; padding: 12px 15px 0; }
.d-detail-actions button { display: inline-flex; min-height: 30px; align-items: center; gap: 6px; padding: 0 9px; border: 1px solid #3b3b37; border-radius: 3px; background: #181817; color: #b9b9b2; font-size: 8px; cursor: pointer; }
.d-detail-actions button:last-child { width: 30px; justify-content: center; padding: 0; }
.d-action-message { margin: 9px 15px 0; color: #83c78b; font-size: 8px; }

.d-routing-layout { display: grid; grid-template-columns: minmax(280px, .7fr) minmax(430px, 1.3fr); gap: 13px; }
.d-route-list > button { display: flex; width: 100%; min-height: 62px; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 11px; border: 0; border-bottom: 1px solid #292927; background: transparent; color: #a9a9a2; text-align: left; cursor: pointer; }
.d-route-list > button:hover { background: #171716; }
.d-route-list > button > span { display: flex; min-width: 0; align-items: center; gap: 8px; }
.d-route-list > button > span > span { display: grid; min-width: 0; gap: 3px; }
.d-route-list strong { color: #d9d9d3; font-size: 9px; }
.d-route-list small { color: #62625d; font-size: 7px; }
.d-route-detail .d-panel-title > span { color: #86c78d; }
.d-route-path { display: grid; grid-template-columns: repeat(3, 1fr); padding: 15px; }
.d-route-path > span { display: grid; gap: 5px; padding: 10px; border: 1px solid #30302d; border-right: 0; }
.d-route-path > span:last-child { border-right: 1px solid #30302d; }
.d-route-path i { display: grid; width: 17px; height: 17px; place-items: center; border-radius: 50%; background: #2a2a27; color: #9b9b94; font: 600 7px/1 ui-monospace, monospace; font-style: normal; }
.d-route-path strong { font-size: 8px; }
.d-route-path small { overflow: hidden; color: #62625d; font-size: 7px; text-overflow: ellipsis; white-space: nowrap; }
.d-route-detail dl { display: grid; grid-template-columns: repeat(4, 1fr); margin: 0; border-top: 1px solid #292927; }
.d-route-detail dl div { display: grid; gap: 5px; padding: 12px; border-right: 1px solid #292927; }
.d-route-detail dl div:last-child { border-right: 0; }
.d-route-detail dt { color: #61615c; font-size: 7px; }
.d-route-detail dd { margin: 0; font: 600 10px/1 ui-monospace, monospace; }
.d-model-table { overflow-x: auto; }
.d-model-table > header,
.d-model-table > div { display: grid; min-width: 790px; grid-template-columns: minmax(150px, 1fr) minmax(170px, 1fr) 130px 90px 70px 65px 65px; align-items: center; gap: 10px; padding: 0 11px; }
.d-model-table > header { min-height: 30px; color: #61615c; font: 600 7px/1 ui-monospace, monospace; }
.d-model-table > div { min-height: 45px; border-top: 1px solid #292927; color: #969690; font-size: 8px; }
.d-model-table > div strong { color: #d5d5cf; font-size: 8px; }
.d-model-table .d-status { font-size: 8px; }
.d-settings-layout { display: grid; grid-template-columns: 190px minmax(0, 1fr); gap: 13px; align-items: start; }
.d-settings-layout > nav { display: grid; gap: 3px; }
.d-settings-layout > nav button { display: flex; min-height: 38px; align-items: center; gap: 8px; padding: 0 10px; border: 1px solid transparent; border-radius: 3px; background: transparent; color: #777771; font-size: 9px; text-align: left; cursor: pointer; }
.d-settings-layout > nav button.active { border-color: #393935; background: #1b1b19; color: #e1e1db; }
.d-setting-row { display: flex; min-height: 66px; align-items: center; justify-content: space-between; gap: 18px; padding: 10px 13px; border-bottom: 1px solid #292927; }
.d-setting-row:last-child { border-bottom: 0; }
.d-setting-row > span { display: grid; gap: 4px; }
.d-setting-row strong { font-size: 9px; }
.d-setting-row small { color: #666660; font-size: 8px; }
.d-setting-row button { display: inline-flex; min-height: 30px; align-items: center; gap: 7px; border: 1px solid #3b3b37; border-radius: 3px; background: #181817; color: #bebeb7; font-size: 8px; cursor: pointer; }

@media (max-width: 1160px) {
  .d-shell { grid-template-columns: 178px minmax(0, 1fr); }
  .d-overview-main,
  .d-routing-layout { grid-template-columns: 1fr; }
  .d-routing-panel { display: grid; grid-template-columns: repeat(3, 1fr); }
  .d-routing-panel .d-panel-title,
  .d-routing-panel > button { grid-column: 1 / -1; }
  .d-routing-row { border-right: 1px solid #292927; }
  .d-account-split { grid-template-columns: minmax(300px, .85fr) minmax(390px, 1.15fr); }
}

@media (max-width: 900px) {
  .d-summary-row { grid-template-columns: 1fr 1fr; }
  .d-summary-row > div:nth-child(2) { border-right: 0; }
  .d-summary-row > div:nth-child(-n + 2) { border-bottom: 1px solid #2d2d2a; }
  .d-lower-grid { grid-template-columns: 1fr; }
  .d-account-split { grid-template-columns: 1fr; }
  .d-account-list { max-height: 360px; overflow-y: auto; border-right: 0; border-bottom: 1px solid #343431; }
  .d-settings-layout { grid-template-columns: 1fr; }
  .d-settings-layout > nav { display: flex; overflow-x: auto; }
  .d-settings-layout > nav button { min-width: max-content; }
}

@media (max-width: 767px) {
  .d-dev-banner { justify-content: center; text-align: center; }
  .d-dev-banner span:last-child { display: none; }
  .d-shell { display: block; }
  .d-sidebar { position: static; height: auto; min-height: 0; border-right: 0; border-bottom: 1px solid #2f2f2c; }
  .d-brand { min-height: 50px; }
  .d-sidebar nav { display: flex; overflow-x: auto; padding: 6px 8px; }
  .d-sidebar nav button { min-width: max-content; grid-template-columns: 18px auto auto; }
  .d-sidebar-pools,
  .d-sidebar footer { display: none; }
  .d-topbar { min-height: 45px; }
  .d-topbar-actions > span:first-child,
  .d-topbar-actions button:nth-of-type(2),
  .d-avatar { display: none; }
  .d-content { padding: 10px; }
  .d-page-title { align-items: start; flex-direction: column; padding: 5px 0; }
  .d-status { white-space: normal; }
  .d-summary-row { grid-template-columns: 1fr 1fr; }
  .d-routing-panel { display: block; }
  .d-detail-capacity { grid-template-columns: 1fr; }
  .d-detail-meter { height: 9px; }
  .d-detail-capacity dl { grid-column: auto; }
  .d-detail-actions { flex-wrap: wrap; }
  .d-route-path,
  .d-route-detail dl { grid-template-columns: 1fr; }
  .d-route-path > span { border-right: 1px solid #30302d; border-bottom: 0; }
  .d-route-path > span:last-child { border-bottom: 1px solid #30302d; }
}
</style>
