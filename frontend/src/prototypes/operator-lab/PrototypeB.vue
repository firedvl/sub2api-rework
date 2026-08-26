<script setup lang="ts">
import { computed, ref } from 'vue'
import { Icon } from '@/components/icons'
import {
  labAccounts,
  labActivity,
  labModels,
  labPages,
  labRequestTrend,
  labRoutes,
  labSettings,
  labSummary,
  type LabAccount,
  type LabPage,
} from './data'

const props = defineProps<{ page: LabPage }>()
const emit = defineEmits<{ navigate: [page: LabPage] }>()
const query = ref('')
const state = ref('all')

const pageCodes: Record<LabPage, string> = {
  overview: 'SYS.OVERVIEW',
  accounts: 'CAP.ACCOUNTS',
  models: 'RT.MODELS',
  settings: 'CFG.ROUTING',
}

const filteredAccounts = computed(() => {
  const normalized = query.value.trim().toLowerCase()
  return labAccounts
    .filter((account) => {
      const matchesState = state.value === 'all' || account.health === state.value
      return matchesState && (!normalized || `${account.provider} ${account.label} ${account.identity} ${account.models.join(' ')}`.toLowerCase().includes(normalized))
    })
    .sort((a, b) => (a.remaining ?? Number.POSITIVE_INFINITY) - (b.remaining ?? Number.POSITIVE_INFINITY))
})

function markerStyle(account: LabAccount) {
  return { left: `${account.remaining ?? 0}%` }
}
</script>

<template>
  <div class="prototype-b" data-testid="prototype-b">
    <header class="b-system-header">
      <div class="b-identity"><span class="b-logo">S2</span><strong>SUB2API // DATA CONSOLE</strong></div>
      <nav aria-label="Prototype B sections">
        <button
          v-for="(item, index) in labPages"
          :key="item.key"
          type="button"
          :class="{ active: props.page === item.key }"
          :aria-current="props.page === item.key ? 'page' : undefined"
          @click="emit('navigate', item.key)"
        >
          <span>0{{ index + 1 }}</span>{{ item.label }}
        </button>
      </nav>
      <div class="b-system-state"><i /> FIXTURE / READ ONLY <time>19:00:00 UTC</time></div>
    </header>

    <div class="b-context-row">
      <span>{{ pageCodes[props.page] }}</span>
      <span>SNAPSHOT=2026-08-25T19:00:00Z</span>
      <span>CONNECTION=LOCAL_FIXTURE</span>
      <span>WRITE_POLICY=BLOCK</span>
    </div>

    <main class="b-workspace">
      <div class="b-title-row">
        <div><span class="b-index">// {{ pageCodes[props.page] }}</span><h1>{{ labPages.find((item) => item.key === props.page)?.label }}</h1></div>
        <div class="b-inline-metrics">
          <span><small>REQ/{{ labSummary.period }}</small><strong>{{ labSummary.requests }}</strong></span>
          <span><small>SUCCESS</small><strong>{{ labSummary.success }}</strong></span>
          <span><small>ROUTABLE</small><strong>{{ labSummary.routableAccounts }}/{{ labSummary.totalAccounts }}</strong></span>
        </div>
      </div>

      <div v-if="props.page === 'overview'" class="b-overview">
        <section class="b-capacity-matrix" aria-labelledby="b-capacity-title">
          <div class="b-section-title">
            <span><strong id="b-capacity-title">NORMALIZED CAPACITY INDEX</strong> // percentage remaining by reported provider window</span>
            <span>KNOWN={{ labAccounts.filter((account) => account.remaining !== null).length }} UNKNOWN={{ labAccounts.filter((account) => account.remaining === null).length }}</span>
          </div>
          <div class="b-matrix-head">
            <span>ACCOUNT / PROVIDER</span><span>HEALTH</span>
            <div class="b-scale-labels"><span>0</span><span>25</span><span>50</span><span>75</span><span>100%</span></div>
            <span>RESET</span><span>WINDOW</span>
          </div>
          <div v-for="account in labAccounts" :key="account.id" class="b-matrix-row">
            <span class="b-account"><strong>{{ account.label }}</strong><small>{{ account.provider.toUpperCase() }} / {{ account.plan.toUpperCase() }}</small></span>
            <span :class="['b-health', account.health]"><i />{{ account.healthLabel }}</span>
            <div class="b-scale">
              <span v-for="line in 5" :key="line" class="b-gridline" />
              <i v-if="account.remaining !== null" :class="['b-marker', account.health]" :style="markerStyle(account)"><b>{{ account.remaining }}</b></i>
              <strong v-else class="b-no-data">NO DATA</strong>
            </div>
            <span class="b-mono">{{ account.resetLabel }}</span>
            <span class="b-window">{{ account.nativeQuota }}</span>
          </div>
        </section>

        <div class="b-dual-table">
          <section>
            <div class="b-section-title"><span><strong>ROUTING STATE</strong> // live aggregate</span><span>3 RECORDS</span></div>
            <div class="b-route-grid b-table-head"><span>ROUTE</span><span>TYPE</span><span>POOL</span><span>OK</span><span>RPM</span></div>
            <div v-for="item in labRoutes" :key="item.name" class="b-route-grid b-table-row">
              <span><i :class="['b-led', item.status]" />{{ item.name }}</span><span>{{ item.type }}</span><span>{{ item.capacity }}</span><span>{{ item.success }}</span><span>{{ item.rpm }}</span>
            </div>
          </section>

          <section>
            <div class="b-section-title"><span><strong>REQUEST TAPE</strong> // five-minute buckets</span><span>ERRORS/{{ labSummary.period }}={{ labSummary.errors }}</span></div>
            <div class="b-request-tape" aria-label="Request count trend">
              <div v-for="point in labRequestTrend" :key="point.time">
                <span><i :style="{ height: `${point.requests}px` }" /><b v-if="point.errors" :style="{ height: `${point.errors * 5}px` }" /></span>
                <strong>{{ point.requests }}</strong><small>{{ point.time }}</small>
              </div>
            </div>
          </section>
        </div>

        <section>
          <div class="b-section-title"><span><strong>LATEST REQUESTS</strong> // fixture activity</span><span>TAIL=5</span></div>
          <div class="b-activity-grid b-table-head"><span>TIME</span><span>ROUTE</span><span>RESOLVED MODEL</span><span>ACCOUNT</span><span>DURATION</span><span>HTTP</span></div>
          <div v-for="item in labActivity" :key="item.time" class="b-activity-grid b-table-row">
            <span>{{ item.time }}</span><span>{{ item.route }}</span><span>{{ item.model }}</span><span>{{ item.account }}</span><span>{{ item.duration }}</span><span :class="{ error: item.status !== '200' }">{{ item.status }}</span>
          </div>
        </section>
      </div>

      <div v-else-if="props.page === 'accounts'" class="b-accounts">
        <div class="b-filter-row">
          <label><span>QUERY</span><input v-model="query" type="search" placeholder="account / identity / model" /></label>
          <label><span>STATE</span><select v-model="state"><option value="all">ALL</option><option value="healthy">HEALTHY</option><option value="degraded">DEGRADED</option><option value="critical">CRITICAL</option></select></label>
          <span>ROWS={{ filteredAccounts.length }} / {{ labAccounts.length }}</span>
          <button type="button"><Icon name="download" size="sm" /> EXPORT VIEW</button>
        </div>

        <section class="b-account-table-shell">
          <div class="b-section-title"><span><strong>ACCOUNT CAPACITY REGISTER</strong> // common snapshot</span><span>SORT=CAPACITY_ASC</span></div>
          <div class="b-account-table">
            <div class="b-account-grid b-table-head"><span>ID</span><span>PROVIDER</span><span>ACCOUNT / IDENTITY</span><span>STATUS</span><span>SCHEDULING</span><span>CAPACITY INDEX</span><span>NATIVE QUOTA</span><span>RESET</span><span>PLAN</span><span>MODELS</span><span>GROUPS</span><span>CONC.</span><span>REQ</span><span>COST</span><span>ACTION</span></div>
            <div v-for="account in filteredAccounts" :key="account.id" class="b-account-grid b-table-row">
              <span>#{{ account.id }}</span>
              <span class="b-provider">{{ account.provider.toUpperCase() }}</span>
              <span class="b-account"><strong>{{ account.label }}</strong><small>{{ account.identity }}</small></span>
              <span :class="['b-health', account.health]"><i />{{ account.healthLabel }}</span>
              <span>{{ account.scheduleLabel }}</span>
              <div class="b-inline-scale"><span><i v-for="line in 5" :key="line" /></span><b v-if="account.remaining !== null" :style="markerStyle(account)" :class="account.health" /> <strong>{{ account.remaining === null ? 'N/A' : `${account.remaining}%` }}</strong></div>
              <span>{{ account.nativeQuota }}</span><span>{{ account.resetLabel }}</span><span>{{ account.plan }}</span><span>{{ account.models.length }} / {{ account.models[0] }}</span><span>{{ account.groups.join(' + ') }}</span><span>{{ account.concurrency }}</span><span>{{ account.requests }}</span><span>{{ account.cost }}</span>
              <span class="b-row-actions"><button type="button" :title="`Inspect ${account.label}`" :aria-label="`Inspect ${account.label}`"><Icon name="eye" size="sm" /></button><button type="button" :title="`More actions for ${account.label}`" :aria-label="`More actions for ${account.label}`"><Icon name="more" size="sm" /></button></span>
            </div>
          </div>
        </section>
      </div>

      <div v-else-if="props.page === 'models'" class="b-models">
        <section>
          <div class="b-section-title"><span><strong>ROUTE RESOLUTION TABLE</strong> // composite and direct paths</span><span>{{ labRoutes.length }} RECORDS</span></div>
          <div class="b-model-route-grid b-table-head"><span>ROUTE KEY</span><span>CLASS</span><span>DESTINATION ORDER</span><span>GROUPS</span><span>POOL</span><span>SUCCESS</span><span>LATENCY</span><span>RPM</span></div>
          <div v-for="item in labRoutes" :key="item.name" class="b-model-route-grid b-table-row"><span><i :class="['b-led', item.status]" />{{ item.name }}</span><span>{{ item.type }}</span><span>{{ item.destinations.join(' > ') }}</span><span>{{ item.groups.join(' + ') }}</span><span>{{ item.capacity }}</span><span>{{ item.success }}</span><span>{{ item.latency }}</span><span>{{ item.rpm }}</span></div>
        </section>
        <section>
          <div class="b-section-title"><span><strong>MODEL ACCESS CATALOG</strong> // observed paths</span><span>{{ labModels.length }} MODELS</span></div>
          <div class="b-model-grid b-table-head"><span>MODEL</span><span>PROVIDER PATH</span><span>ROUTE</span><span>STATUS</span><span>REQUESTS</span><span>TOKENS</span><span>LATENCY</span></div>
          <div v-for="model in labModels" :key="model.name" class="b-model-grid b-table-row"><span>{{ model.name }}</span><span>{{ model.provider }}</span><span>{{ model.route }}</span><span :class="['b-health', model.status]"><i />{{ model.status.toUpperCase() }}</span><span>{{ model.requests }}</span><span>{{ model.tokens }}</span><span>{{ model.latency }}</span></div>
        </section>
      </div>

      <div v-else class="b-settings">
        <section>
          <div class="b-section-title"><span><strong>ROUTING CONFIGURATION</strong> // representative fixture values</span><span>SCOPE=GLOBAL</span></div>
          <div class="b-setting-grid b-table-head"><span>KEY</span><span>DESCRIPTION</span><span>VALUE</span><span>SOURCE</span><span>ACTION</span></div>
          <div v-for="(setting, index) in labSettings" :key="setting.title" class="b-setting-grid b-table-row"><span>routing.{{ String(index + 1).padStart(2, '0') }}</span><span><strong>{{ setting.title }}</strong><small>{{ setting.description }}</small></span><span>{{ setting.value }}</span><span>FIXTURE</span><span><button type="button" :aria-label="`Edit ${setting.title}`"><Icon name="edit" size="sm" /></button></span></div>
        </section>
        <section>
          <div class="b-section-title"><span><strong>ENVIRONMENT POLICY</strong> // enforced by review server</span><span>STATE=SAFE</span></div>
          <pre>mode               operator-prototypes
bind               127.0.0.1
backend_proxy      disabled
data               synthetic/static
write_requests     reject:405
production_route   absent</pre>
        </section>
      </div>
    </main>
  </div>
</template>

<style scoped>
.prototype-b {
  min-height: calc(100vh - 72px);
  background: #080909;
  color: #d5d8d4;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-variant-numeric: tabular-nums;
}

.b-system-header {
  display: grid;
  min-height: 54px;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: stretch;
  border-bottom: 1px solid #3a3d39;
  background: #0d0f0e;
}

.b-identity { display: flex; align-items: center; gap: 10px; padding: 0 18px; border-right: 1px solid #2c2f2c; font-size: 11px; }
.b-logo { display: grid; width: 26px; height: 26px; place-items: center; border: 1px solid #6e766b; color: #d7e265; font-weight: 750; }
.b-system-header nav { display: flex; min-width: 0; overflow-x: auto; }
.b-system-header nav button { display: flex; min-width: max-content; min-height: 53px; align-items: center; gap: 8px; padding: 0 16px; border: 0; border-right: 1px solid #222522; background: transparent; color: #7c827c; font: 600 10px/1 ui-monospace, monospace; cursor: pointer; }
.b-system-header nav button span { color: #4e544e; font-size: 8px; }
.b-system-header nav button:hover { background: #121512; color: #c7cbc5; }
.b-system-header nav button.active { background: #dfe2dc; color: #0b0d0b; }
.b-system-header nav button.active span { color: #687063; }
.b-system-header button:focus-visible,
.b-filter-row button:focus-visible,
.b-row-actions button:focus-visible,
.b-setting-grid button:focus-visible { outline: 2px solid #d7e265; outline-offset: -3px; }
.b-system-state { display: flex; align-items: center; gap: 7px; padding: 0 16px; color: #94a08f; font-size: 9px; white-space: nowrap; }
.b-system-state i { width: 6px; height: 6px; background: #83ca8a; }
.b-system-state time { margin-left: 8px; color: #686e68; }

.b-context-row { display: flex; min-height: 25px; align-items: center; overflow-x: auto; border-bottom: 1px solid #242724; background: #0a0c0b; color: #555b55; font-size: 8px; white-space: nowrap; }
.b-context-row span { padding: 0 14px; border-right: 1px solid #222522; }
.b-context-row span:first-child { color: #c9d07a; }
.b-workspace { padding: 14px 18px 20px; }
.b-title-row { display: flex; min-height: 64px; align-items: end; justify-content: space-between; gap: 24px; padding-bottom: 12px; border-bottom: 1px solid #3a3d39; }
.b-index { color: #677065; font-size: 8px; }
.b-title-row h1 { margin: 7px 0 0; font-size: 20px; font-weight: 620; text-transform: uppercase; }
.b-inline-metrics { display: flex; border: 1px solid #2d302d; }
.b-inline-metrics > span { display: grid; min-width: 96px; gap: 4px; padding: 8px 11px; border-right: 1px solid #2d302d; }
.b-inline-metrics > span:last-child { border-right: 0; }
.b-inline-metrics small { color: #5f655f; font-size: 7px; }
.b-inline-metrics strong { font-size: 12px; font-weight: 620; }
.b-overview,
.b-models,
.b-settings { display: grid; gap: 14px; padding-top: 14px; }
.b-overview,
.b-overview > *,
.b-dual-table,
.b-dual-table > * { min-width: 0; }
.b-section-title { display: flex; min-height: 28px; align-items: center; justify-content: space-between; gap: 18px; padding: 0 8px; border: 1px solid #2f332f; background: #101210; color: #626962; font-size: 8px; }
.b-section-title strong { color: #c9cdc7; font-weight: 650; }
.b-section-title > span:last-child { color: #6e766c; white-space: nowrap; }

.b-matrix-head,
.b-matrix-row { display: grid; grid-template-columns: minmax(170px, .95fr) 112px minmax(260px, 1.5fr) 72px minmax(130px, .75fr); align-items: center; gap: 12px; padding: 0 8px; }
.b-matrix-head { min-height: 30px; border-right: 1px solid #272a27; border-left: 1px solid #272a27; color: #555b55; font-size: 8px; }
.b-matrix-row { min-height: 47px; border: 1px solid #272a27; border-top: 0; font-size: 9px; }
.b-matrix-row:hover { background: #0d100e; }
.b-account { display: grid; min-width: 0; gap: 3px; }
.b-account strong,
.b-account small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.b-account strong { color: #d5d8d3; font-size: 10px; font-weight: 600; }
.b-account small { color: #555c55; font-size: 7px; }
.b-health { display: inline-flex; min-width: 0; align-items: center; gap: 6px; overflow: hidden; color: #97a095; font-size: 8px; text-overflow: ellipsis; white-space: nowrap; }
.b-health i,
.b-led { width: 5px; height: 5px; flex: 0 0 auto; background: #78c784; }
.b-health.degraded,
.degraded { color: #d8b966; }
.b-health.degraded i,
.b-led.degraded { background: #d8b45c; }
.b-health.critical { color: #e17970; }
.b-health.critical i,
.b-led.critical { background: #dc675f; }
.b-scale-labels { display: flex; justify-content: space-between; color: #4f554f; }
.b-scale { position: relative; display: grid; height: 24px; grid-template-columns: repeat(5, 1fr); align-items: stretch; border-right: 1px solid #303430; border-left: 1px solid #303430; }
.b-gridline { border-right: 1px solid #242724; }
.b-gridline:last-child { border-right: 0; }
.b-marker { position: absolute; top: 4px; width: 2px; height: 16px; transform: translateX(-1px); background: #a7d2a9; }
.b-marker b { position: absolute; top: 4px; right: 5px; color: #d9ddd7; font-size: 8px; font-style: normal; }
.b-marker.degraded { background: #dbb85e; }
.b-marker.critical { background: #df6c63; }
.b-no-data { position: absolute; top: 7px; left: 50%; transform: translateX(-50%); color: #737972; font-size: 7px; }
.b-mono { color: #a6ada5; white-space: nowrap; }
.b-window { overflow: hidden; color: #6e756e; text-overflow: ellipsis; white-space: nowrap; }

.b-dual-table { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(310px, .8fr); gap: 14px; }
.b-route-grid { display: grid; grid-template-columns: minmax(120px, .8fr) 70px minmax(180px, 1.4fr) 60px 40px; align-items: center; gap: 10px; padding: 0 8px; }
.b-table-head { min-height: 28px; border-right: 1px solid #272a27; border-bottom: 1px solid #272a27; border-left: 1px solid #272a27; color: #535953; font-size: 7px; }
.b-table-row { min-height: 36px; border-right: 1px solid #272a27; border-bottom: 1px solid #272a27; border-left: 1px solid #272a27; color: #8f968e; font-size: 8px; }
.b-table-row:hover { background: #0e110f; }
.b-table-row > span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.b-table-row > span:first-child { color: #d0d4ce; }
.b-route-grid > span:first-child,
.b-model-route-grid > span:first-child { display: flex; align-items: center; gap: 6px; }
.b-request-tape { display: grid; min-height: 118px; grid-template-columns: repeat(7, 1fr); align-items: end; border: 1px solid #272a27; border-top: 0; }
.b-request-tape > div { display: grid; justify-items: center; gap: 5px; padding: 8px 3px; border-right: 1px solid #222522; }
.b-request-tape > div:last-child { border-right: 0; }
.b-request-tape > div > span { position: relative; display: flex; height: 52px; align-items: end; gap: 2px; }
.b-request-tape i { display: block; width: 6px; max-height: 49px; background: #788d75; }
.b-request-tape b { display: block; width: 3px; max-height: 18px; background: #d9685f; }
.b-request-tape strong { color: #b5bbb4; font-size: 8px; }
.b-request-tape small { color: #525852; font-size: 7px; }
.b-activity-grid { display: grid; grid-template-columns: 70px minmax(120px, .8fr) minmax(150px, 1fr) minmax(150px, 1fr) 70px 45px; align-items: center; gap: 12px; padding: 0 8px; }
.b-activity-grid .error { color: #e56c63; }

.b-filter-row { display: flex; min-height: 52px; align-items: center; gap: 10px; border-bottom: 1px solid #353935; }
.b-filter-row label { display: flex; height: 30px; align-items: center; border: 1px solid #343834; }
.b-filter-row label:focus-within { outline: 2px solid #d7e265; outline-offset: 2px; }
.b-filter-row label span { padding: 0 8px; color: #616861; font-size: 7px; }
.b-filter-row input,
.b-filter-row select { height: 28px; border: 0; border-left: 1px solid #343834; border-radius: 0; outline: 0; background: #0d0f0e; color: #c6cbc4; font: 500 9px/1 ui-monospace, monospace; }
.b-filter-row input { width: 230px; padding: 0 9px; }
.b-filter-row select { min-width: 120px; padding: 0 8px; }
.b-filter-row > span { margin-left: auto; color: #697069; font-size: 8px; }
.b-filter-row button { display: inline-flex; height: 30px; align-items: center; gap: 7px; border: 1px solid #485047; background: #151815; color: #b8beb6; font: 600 8px/1 ui-monospace, monospace; cursor: pointer; }
.b-account-table-shell { padding-top: 14px; overflow-x: auto; }
.b-account-table { min-width: 1480px; }
.b-account-grid { display: grid; grid-template-columns: 42px 72px minmax(180px, 1.2fr) 105px 110px minmax(160px, 1fr) minmax(150px, 1fr) 65px 70px minmax(120px, .85fr) minmax(140px, .9fr) 55px 50px 58px 58px; align-items: center; gap: 8px; padding: 0 7px; }
.b-account-grid.b-table-row { min-height: 52px; }
.b-provider { color: #98aa91; }
.b-inline-scale { position: relative; display: grid; grid-template-columns: 1fr 32px; align-items: center; gap: 7px; }
.b-inline-scale > span { display: grid; height: 14px; grid-template-columns: repeat(5, 1fr); border-right: 1px solid #303430; border-left: 1px solid #303430; }
.b-inline-scale > span i { border-right: 1px solid #242724; }
.b-inline-scale > b { position: absolute; top: 0; width: 2px; height: 14px; transform: translateX(-1px); background: #8ecb94; }
.b-inline-scale > b.degraded { background: #d8b45c; }
.b-inline-scale > b.critical { background: #dc675f; }
.b-inline-scale > strong { color: #bec4bc; font-size: 8px; font-weight: 500; }
.b-row-actions { display: flex; gap: 3px; }
.b-row-actions button,
.b-setting-grid button { display: grid; width: 23px; height: 23px; place-items: center; border: 1px solid #303430; background: #101210; color: #858d84; cursor: pointer; }

.b-models > section,
.b-settings > section { overflow-x: auto; }
.b-model-route-grid { display: grid; min-width: 1000px; grid-template-columns: 130px 70px minmax(210px, 1.5fr) minmax(220px, 1.4fr) minmax(180px, 1.1fr) 60px 60px 40px; align-items: center; gap: 10px; padding: 0 8px; }
.b-model-grid { display: grid; min-width: 800px; grid-template-columns: minmax(150px, 1fr) minmax(170px, 1.1fr) 130px 90px 70px 65px 65px; align-items: center; gap: 10px; padding: 0 8px; }
.b-setting-grid { display: grid; min-width: 760px; grid-template-columns: 110px minmax(300px, 1fr) 90px 70px 45px; align-items: center; gap: 12px; padding: 0 8px; }
.b-setting-grid > span:nth-child(2) { display: grid; gap: 3px; }
.b-setting-grid strong { color: #cbd0c9; font-size: 9px; font-weight: 550; }
.b-setting-grid small { color: #596059; font-size: 7px; }
.b-settings pre { margin: 0; padding: 15px; border: 1px solid #272a27; border-top: 0; background: #0a0c0b; color: #8d978b; font: 500 10px/1.8 ui-monospace, monospace; }

@media (max-width: 1080px) {
  .b-system-header { grid-template-columns: auto minmax(0, 1fr); }
  .b-system-state { display: none; }
  .b-dual-table { grid-template-columns: 1fr; }
  .b-matrix-head,
  .b-matrix-row { grid-template-columns: minmax(150px, .9fr) 100px minmax(220px, 1.4fr) 62px minmax(120px, .75fr); }
}

@media (max-width: 767px) {
  .b-system-header { display: block; }
  .b-identity { height: 46px; border-right: 0; border-bottom: 1px solid #2c2f2c; }
  .b-system-header nav button { min-height: 42px; }
  .b-context-row span:nth-child(n + 3) { display: none; }
  .b-workspace { padding: 10px; }
  .b-title-row { align-items: start; flex-direction: column; }
  .b-inline-metrics { width: 100%; }
  .b-inline-metrics > span { min-width: 0; flex: 1; }
  .b-capacity-matrix { overflow-x: auto; }
  .b-dual-table > section { overflow-x: auto; }
  .b-matrix-head,
  .b-matrix-row { min-width: 760px; }
  .b-activity-grid { min-width: 700px; }
  .b-overview > section:last-child { overflow-x: auto; }
  .b-filter-row { flex-wrap: wrap; padding: 8px 0; }
  .b-filter-row label:first-child { width: 100%; }
  .b-filter-row input { width: 100%; }
  .b-filter-row > span { margin-left: 0; }
}
</style>
