<script setup lang="ts">
import { Icon } from '@/components/icons'
import {
  labActivity,
  labModels,
  labPages,
  labProviders,
  labRequestTrend,
  labRoutes,
  labSettings,
  labSummary,
  type LabAccount,
  type LabPage,
} from './data'

const props = defineProps<{ page: LabPage }>()
const emit = defineEmits<{ navigate: [page: LabPage] }>()

function barStyle(account: LabAccount) {
  return { height: `${Math.max(account.remaining ?? 6, 6)}%` }
}

function horizontalStyle(account: LabAccount) {
  return { width: `${account.remaining ?? 0}%` }
}
</script>

<template>
  <div class="prototype-c" data-testid="prototype-c">
    <header class="c-header">
      <div class="c-brand"><span>S2</span><div><strong>SUB2API</strong><small>OPERATIONS COCKPIT</small></div></div>
      <nav aria-label="Prototype C sections">
        <button v-for="item in labPages" :key="item.key" type="button" :class="{ active: props.page === item.key }" :aria-current="props.page === item.key ? 'page' : undefined" @click="emit('navigate', item.key)">{{ item.label }}</button>
      </nav>
      <div class="c-clock"><i /> LIVE FIXTURE <time>19:00:00</time></div>
    </header>

    <div class="c-safety"><Icon name="beaker" size="xs" /> DEVELOPMENT PROTOTYPE · SYNTHETIC SNAPSHOT · PRODUCTION DISCONNECTED</div>

    <main class="c-main">
      <div v-if="props.page === 'overview'" class="c-overview">
        <section class="c-status-deck">
          <div class="c-health-score">
            <span class="c-eyebrow">GATEWAY HEALTH</span>
            <div><strong>96</strong><span>/ 100<small>Operational</small></span></div>
          </div>
          <div class="c-vital"><span>SUCCESS / {{ labSummary.period }}</span><strong>{{ labSummary.success }}</strong><small>{{ labSummary.successfulRequests }} successful</small></div>
          <div class="c-vital warning"><span>CAPACITY ALERTS</span><strong>3</strong><small>Across 3 providers</small></div>
          <div class="c-vital"><span>CURRENT LOAD</span><strong>{{ labSummary.rpm }} <b>RPM</b></strong><small>642 tokens/s</small></div>
          <div class="c-vital critical"><span>EXCEPTIONS / {{ labSummary.period }}</span><strong>{{ labSummary.errors }}</strong><small>{{ labSummary.upstreamErrors }} upstream of {{ labSummary.errors }} total</small></div>
          <div class="c-vital"><span>ACTUAL COST</span><strong>{{ labSummary.cost }}</strong><small>Today</small></div>
        </section>

        <section class="c-capacity-section">
          <div class="c-section-heading">
            <div><span class="c-eyebrow">CAPACITY MAP</span><h1>Provider capacity remaining</h1><p>Each column is one account, normalized within its reported quota window.</p></div>
            <div class="c-legend"><span><i class="healthy" />healthy</span><span><i class="degraded" />attention</span><span><i class="critical" />critical</span><span><i class="unknown" />unknown</span></div>
          </div>
          <div class="c-provider-grid">
            <article v-for="provider in labProviders" :key="provider.id" :class="['c-provider', provider.health]">
              <header><span><i :class="provider.health" /><strong>{{ provider.label }}</strong></span><span><strong>{{ provider.averageRemaining }}%</strong><small>known avg</small></span></header>
              <div class="c-capacity-columns">
                <div v-for="account in provider.accounts" :key="account.id" class="c-capacity-column">
                  <div class="c-column-frame">
                    <i v-if="account.remaining !== null" :class="account.health" :style="barStyle(account)" />
                    <span v-else class="c-column-unknown">?</span>
                    <b>{{ account.remaining === null ? '—' : account.remaining }}</b>
                  </div>
                  <span><strong>{{ account.label }}</strong><small>{{ account.resetLabel }} reset</small></span>
                </div>
              </div>
              <footer><span>{{ provider.accounts.filter((account) => account.schedulable).length }}/{{ provider.accounts.length }} schedulable</span><span>{{ provider.knownAccounts }}/{{ provider.accounts.length }} reporting</span></footer>
            </article>
          </div>
        </section>

        <div class="c-monitor-grid">
          <section class="c-monitor c-traffic-monitor">
            <div class="c-monitor-title"><span><strong>Request pressure</strong><small>5-minute request volume</small></span><span class="c-live"><i /> LIVE</span></div>
            <div class="c-traffic-chart" aria-label="Request pressure chart">
              <div v-for="point in labRequestTrend" :key="point.time">
                <span class="c-chart-bar" :style="{ height: `${point.requests * 1.5}px` }"><i v-if="point.errors">{{ point.errors }}</i></span>
                <b>{{ point.requests }}</b><small>{{ point.time }}</small>
              </div>
            </div>
          </section>

          <section class="c-monitor c-route-monitor">
            <div class="c-monitor-title"><span><strong>Route condition</strong><small>Current routing state</small></span></div>
            <div v-for="route in labRoutes" :key="route.name" class="c-route-line">
              <span><i :class="route.status" /><strong>{{ route.name }}</strong><small>{{ route.type }} · {{ route.capacity }}</small></span><span><strong>{{ route.success }}</strong><small>{{ route.latency }} · {{ route.rpm }} RPM</small></span>
            </div>
          </section>

          <section class="c-monitor c-events-monitor">
            <div class="c-monitor-title"><span><strong>Request tail</strong><small>Latest resolved traffic</small></span></div>
            <div v-for="event in labActivity.slice(0, 4)" :key="event.time" class="c-event-line"><time>{{ event.time }}</time><span><strong>{{ event.model }}</strong><small>{{ event.account }}</small></span><code :class="{ error: event.status !== '200' }">{{ event.status }}</code></div>
          </section>
        </div>
      </div>

      <div v-else-if="props.page === 'accounts'" class="c-accounts">
        <div class="c-page-heading"><div><span class="c-eyebrow">ACCOUNT OPERATIONS</span><h1>Capacity by provider</h1><p>Provider context first, then individual account readiness.</p></div><div class="c-account-summary"><span><strong>5</strong> routing</span><span><strong>2</strong> paused</span><span><strong>1</strong> excluded</span></div></div>

        <section v-for="provider in labProviders" :key="provider.id" class="c-provider-group">
          <header>
            <span class="c-provider-title"><i :class="provider.health" /><span><strong>{{ provider.label }}</strong><small>{{ provider.accounts.length }} accounts · {{ provider.knownAccounts }} reporting quota</small></span></span>
            <span class="c-provider-capacity"><small>KNOWN-ACCOUNT AVG</small><strong>{{ provider.averageRemaining }}%</strong></span>
            <span class="c-provider-ready"><small>SCHEDULABLE</small><strong>{{ provider.accounts.filter((account) => account.schedulable).length }}/{{ provider.accounts.length }}</strong></span>
          </header>
          <div class="c-provider-account-head"><span>Account</span><span>Capacity remaining</span><span>Window / reset</span><span>Scheduling</span><span>Models</span><span>Today</span><span>Action</span></div>
          <div v-for="account in provider.accounts" :key="account.id" class="c-provider-account-row">
            <span class="c-account-name"><i :class="account.health" /><span><strong>{{ account.label }}</strong><small>{{ account.identity }} · {{ account.plan }}</small></span></span>
            <span class="c-horizontal-capacity"><span><i :class="account.health" :style="horizontalStyle(account)" /></span><strong>{{ account.remaining === null ? 'Unknown' : `${account.remaining}%` }}</strong></span>
            <span><strong>{{ account.nativeQuota }}</strong><small>{{ account.resetLabel }}</small></span>
            <span :class="['c-schedule', { off: !account.schedulable }]"><strong>{{ account.scheduleLabel }}</strong><small>{{ account.concurrency }} concurrent</small></span>
            <span><strong>{{ account.models.length }} models</strong><small>{{ account.models.join(' · ') }}</small></span>
            <span><strong>{{ account.requests.toLocaleString() }} req</strong><small>{{ account.tokens }} · {{ account.cost }}</small></span>
            <span class="c-account-actions"><button type="button" :title="`Inspect ${account.label}`" :aria-label="`Inspect ${account.label}`"><Icon name="eye" size="sm" /></button><button type="button" :title="`More actions for ${account.label}`" :aria-label="`More actions for ${account.label}`"><Icon name="more" size="sm" /></button></span>
          </div>
        </section>
      </div>

      <div v-else-if="props.page === 'models'" class="c-models">
        <div class="c-page-heading"><div><span class="c-eyebrow">ROUTING HEALTH</span><h1>Models and destinations</h1><p>Operational health by externally addressable route.</p></div></div>
        <div class="c-route-cards">
          <article v-for="route in labRoutes" :key="route.name" :class="route.status">
            <header><span><i :class="route.status" /><strong>{{ route.name }}</strong></span><small>{{ route.type }}</small></header>
            <dl><div><dt>Success</dt><dd>{{ route.success }}</dd></div><div><dt>Latency</dt><dd>{{ route.latency }}</dd></div><div><dt>Load</dt><dd>{{ route.rpm }} RPM</dd></div></dl>
            <p>{{ route.destinations.join(' → ') }}</p><footer>{{ route.capacity }}</footer>
          </article>
        </div>
        <section class="c-model-table">
          <header><span>Model</span><span>Provider path</span><span>Route</span><span>Condition</span><span>Requests</span><span>Latency</span></header>
          <div v-for="model in labModels" :key="model.name"><span><strong>{{ model.name }}</strong><small>{{ model.tokens }} tokens</small></span><span>{{ model.provider }}</span><span>{{ model.route }}</span><span class="c-account-name"><i :class="model.status" />{{ model.status }}</span><span>{{ model.requests }}</span><span>{{ model.latency }}</span></div>
        </section>
      </div>

      <div v-else class="c-settings">
        <div class="c-page-heading"><div><span class="c-eyebrow">OPERATIONS POLICY</span><h1>Capacity safeguards</h1><p>Representative settings for monitoring and failover.</p></div></div>
        <section class="c-settings-list">
          <div v-for="setting in labSettings" :key="setting.title"><span><strong>{{ setting.title }}</strong><small>{{ setting.description }}</small></span><button type="button">{{ setting.value }} <Icon name="chevronRight" size="sm" /></button></div>
        </section>
        <aside class="c-fixture-policy"><Icon name="shield" size="lg" /><span><strong>Fixture safety active</strong><small>Loopback only · static data · writes blocked · no production route</small></span></aside>
      </div>
    </main>
  </div>
</template>

<style scoped>
.prototype-c {
  min-height: calc(100vh - 72px);
  background: #0b0c0c;
  color: #edeeeb;
  font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  font-variant-numeric: tabular-nums;
}

.c-header { display: grid; min-height: 62px; grid-template-columns: 240px minmax(0, 1fr) auto; align-items: center; border-bottom: 1px solid #343735; background: #111313; }
.c-brand { display: flex; align-items: center; gap: 11px; padding: 0 18px; }
.c-brand > span { display: grid; width: 32px; height: 32px; place-items: center; border: 1px solid #59605c; border-radius: 2px; color: #91d49a; font: 750 11px/1 ui-monospace, monospace; }
.c-brand > div { display: grid; gap: 2px; }
.c-brand strong { font-size: 12px; }
.c-brand small { color: #68706b; font: 600 8px/1 ui-monospace, monospace; }
.c-header nav { display: flex; height: 100%; justify-content: center; }
.c-header nav button { position: relative; min-width: max-content; padding: 0 15px; border: 0; background: transparent; color: #7f8581; font-size: 11px; cursor: pointer; }
.c-header nav button:hover { color: #d4d8d4; }
.c-header nav button.active { color: #fff; }
.c-header nav button.active::after { position: absolute; right: 15px; bottom: 0; left: 15px; height: 2px; background: #8dce94; content: ''; }
.c-header button:focus-visible,
.c-account-actions button:focus-visible,
.c-settings-list button:focus-visible { outline: 2px solid #a4d9aa; outline-offset: -3px; }
.c-clock { display: flex; align-items: center; gap: 7px; padding: 0 18px; color: #6f7772; font: 600 8px/1 ui-monospace, monospace; white-space: nowrap; }
.c-clock i,
.c-live i { width: 6px; height: 6px; border-radius: 50%; background: #76c783; }
.c-clock time { margin-left: 6px; color: #afb5b0; }
.c-safety { display: flex; min-height: 25px; align-items: center; justify-content: center; gap: 7px; border-bottom: 1px solid #292c2a; background: #0e1010; color: #677069; font: 600 8px/1 ui-monospace, monospace; }
.c-main { padding: 15px 18px 22px; }
.c-overview { display: grid; gap: 14px; }
.c-status-deck { display: grid; min-height: 90px; grid-template-columns: 1.25fr repeat(5, minmax(0, 1fr)); border: 1px solid #383b39; background: #111313; }
.c-health-score,
.c-vital { display: grid; align-content: center; gap: 7px; padding: 12px 16px; border-right: 1px solid #303331; }
.c-status-deck > div:last-child { border-right: 0; }
.c-eyebrow { color: #707771; font: 650 8px/1 ui-monospace, monospace; text-transform: uppercase; }
.c-health-score > div { display: flex; align-items: end; gap: 7px; }
.c-health-score > div > strong { color: #8fd099; font: 650 32px/.9 ui-monospace, monospace; }
.c-health-score > div > span { display: grid; gap: 3px; color: #717772; font: 500 10px/1 ui-monospace, monospace; }
.c-health-score small { color: #a5aca6; font-size: 8px; }
.c-vital > span { color: #646b66; font: 600 7px/1 ui-monospace, monospace; }
.c-vital > strong { font: 630 17px/1 ui-monospace, monospace; }
.c-vital > strong b { color: #727873; font-size: 8px; font-weight: 550; }
.c-vital > small { color: #666d67; font-size: 8px; }
.c-vital.warning > strong { color: #e0bb68; }
.c-vital.critical > strong { color: #e57a70; }

.c-capacity-section,
.c-monitor,
.c-provider-group,
.c-model-table,
.c-settings-list { border: 1px solid #383b39; background: #111313; }
.c-section-heading { display: flex; min-height: 66px; align-items: center; justify-content: space-between; gap: 24px; padding: 10px 14px; border-bottom: 1px solid #303331; }
.c-section-heading h1,
.c-page-heading h1 { margin: 5px 0 0; font-size: 16px; font-weight: 630; }
.c-section-heading p,
.c-page-heading p { margin: 4px 0 0; color: #6a716c; font-size: 9px; }
.c-legend { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 12px; color: #737a75; font: 500 8px/1 ui-monospace, monospace; }
.c-legend span { display: inline-flex; align-items: center; gap: 5px; }
.c-legend i,
.c-provider header > span:first-child i,
.c-provider-title > i,
.c-account-name > i,
.c-route-line i,
.c-route-cards header i { width: 7px; height: 7px; flex: 0 0 auto; border-radius: 50%; background: #7bc987; }
.c-legend i.degraded,
i.degraded { background: #d9b25b; }
.c-legend i.critical,
i.critical { background: #df6c63; }
.c-legend i.unknown { border: 1px solid #777d77; background: transparent; }
.c-provider-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); }
.c-provider { min-width: 0; border-right: 1px solid #303331; }
.c-provider:last-child { border-right: 0; }
.c-provider > header { display: flex; min-height: 48px; align-items: center; justify-content: space-between; gap: 8px; padding: 8px 12px; border-bottom: 1px solid #292c2a; }
.c-provider > header > span:first-child { display: flex; align-items: center; gap: 7px; }
.c-provider > header > span:first-child strong { font-size: 10px; font-weight: 620; }
.c-provider > header > span:last-child { display: grid; justify-items: end; gap: 2px; }
.c-provider > header > span:last-child strong { font: 620 13px/1 ui-monospace, monospace; }
.c-provider > header > span:last-child small { color: #626963; font-size: 7px; }
.c-capacity-columns { display: grid; min-height: 170px; grid-template-columns: repeat(2, 1fr); gap: 8px; align-items: end; padding: 14px 12px 10px; }
.c-capacity-column { display: grid; min-width: 0; gap: 8px; }
.c-column-frame { position: relative; height: 100px; overflow: hidden; border: 1px solid #303431; background: #0b0d0c; }
.c-column-frame > i { position: absolute; right: 0; bottom: 0; left: 0; min-height: 2px; background: #70bd7b; }
.c-column-frame > i.degraded { background: #c69e4e; }
.c-column-frame > i.critical { background: #cf5c55; }
.c-column-frame > b { position: absolute; top: 7px; left: 7px; color: #e5e8e3; font: 620 11px/1 ui-monospace, monospace; }
.c-column-unknown { position: absolute; inset: 0; display: grid; place-items: center; border: 1px dashed #555b56; color: #6e756f; font: 600 14px/1 ui-monospace, monospace; }
.c-capacity-column > span { display: grid; min-width: 0; gap: 3px; }
.c-capacity-column > span strong,
.c-capacity-column > span small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.c-capacity-column > span strong { font-size: 8px; font-weight: 600; }
.c-capacity-column > span small { color: #5f6660; font-size: 7px; }
.c-provider > footer { display: flex; min-height: 30px; align-items: center; justify-content: space-between; gap: 8px; padding: 0 10px; border-top: 1px solid #292c2a; color: #656c66; font: 500 7px/1 ui-monospace, monospace; }

.c-monitor-grid { display: grid; grid-template-columns: 1.1fr 1fr .9fr; gap: 14px; }
.c-monitor-title { display: flex; min-height: 44px; align-items: center; justify-content: space-between; padding: 7px 12px; border-bottom: 1px solid #2c2f2d; }
.c-monitor-title > span:first-child { display: grid; gap: 3px; }
.c-monitor-title strong { font-size: 10px; font-weight: 620; }
.c-monitor-title small { color: #636a64; font-size: 7px; }
.c-live { display: inline-flex; align-items: center; gap: 5px; color: #7fc48a; font: 600 7px/1 ui-monospace, monospace; }
.c-traffic-chart { display: grid; height: 145px; grid-template-columns: repeat(7, 1fr); align-items: end; padding: 15px 12px 10px; }
.c-traffic-chart > div { display: grid; justify-items: center; gap: 5px; }
.c-chart-bar { position: relative; display: block; width: 14px; max-height: 76px; background: #5b8060; }
.c-chart-bar i { position: absolute; top: -6px; right: -6px; display: grid; width: 14px; height: 14px; place-items: center; border-radius: 50%; background: #ce5e56; color: #fff; font: 650 7px/1 ui-monospace, monospace; }
.c-traffic-chart b { color: #aeb4af; font: 550 8px/1 ui-monospace, monospace; }
.c-traffic-chart small { color: #5c635d; font: 500 7px/1 ui-monospace, monospace; }
.c-route-line { display: flex; min-height: 48px; align-items: center; justify-content: space-between; gap: 10px; padding: 7px 11px; border-bottom: 1px solid #292c2a; }
.c-route-line:last-child { border-bottom: 0; }
.c-route-line > span { display: grid; min-width: 0; gap: 3px; }
.c-route-line > span:first-child { position: relative; padding-left: 13px; }
.c-route-line > span:first-child > i { position: absolute; top: 3px; left: 0; }
.c-route-line > span:last-child { justify-items: end; text-align: right; }
.c-route-line strong { overflow: hidden; font-size: 9px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.c-route-line small { overflow: hidden; color: #5f6660; font-size: 7px; text-overflow: ellipsis; white-space: nowrap; }
.c-event-line { display: grid; min-height: 36px; grid-template-columns: 54px minmax(0, 1fr) 34px; align-items: center; gap: 8px; padding: 0 10px; border-bottom: 1px solid #292c2a; }
.c-event-line:last-child { border-bottom: 0; }
.c-event-line time { color: #656c66; font: 500 7px/1 ui-monospace, monospace; }
.c-event-line span { display: grid; min-width: 0; gap: 2px; }
.c-event-line strong,
.c-event-line small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.c-event-line strong { font-size: 8px; font-weight: 580; }
.c-event-line small { color: #5e655f; font-size: 7px; }
.c-event-line code { color: #79c786; font-size: 8px; }
.c-event-line code.error { color: #e26d64; }

.c-page-heading { display: flex; min-height: 80px; align-items: center; justify-content: space-between; gap: 20px; }
.c-accounts,
.c-models,
.c-settings { display: grid; gap: 14px; }
.c-account-summary { display: flex; border: 1px solid #343735; background: #111313; }
.c-account-summary span { padding: 10px 13px; border-right: 1px solid #303331; color: #6d746f; font-size: 8px; }
.c-account-summary span:last-child { border-right: 0; }
.c-account-summary strong { margin-right: 4px; color: #e1e5df; font: 620 13px/1 ui-monospace, monospace; }
.c-provider-group > header { display: grid; min-height: 58px; grid-template-columns: minmax(220px, 1fr) 150px 120px; align-items: center; gap: 12px; padding: 8px 14px; border-bottom: 1px solid #303331; }
.c-provider-title { display: flex; align-items: center; gap: 10px; }
.c-provider-title > span { display: grid; gap: 3px; }
.c-provider-title strong { font-size: 12px; }
.c-provider-title small { color: #626963; font-size: 8px; }
.c-provider-capacity,
.c-provider-ready { display: grid; justify-items: end; gap: 4px; }
.c-provider-capacity small,
.c-provider-ready small { color: #606761; font: 550 7px/1 ui-monospace, monospace; }
.c-provider-capacity strong,
.c-provider-ready strong { font: 620 16px/1 ui-monospace, monospace; }
.c-provider-account-head,
.c-provider-account-row { display: grid; grid-template-columns: minmax(190px, 1.1fr) minmax(180px, 1fr) minmax(150px, .9fr) minmax(130px, .8fr) minmax(160px, 1fr) minmax(120px, .75fr) 65px; align-items: center; gap: 12px; padding: 0 13px; }
.c-provider-account-head { min-height: 29px; color: #606761; font: 600 7px/1 ui-monospace, monospace; text-transform: uppercase; }
.c-provider-account-row { min-height: 65px; border-top: 1px solid #292c2a; color: #9da49e; font-size: 9px; }
.c-provider-account-row > span { min-width: 0; }
.c-provider-account-row > span:not(.c-account-name, .c-horizontal-capacity, .c-account-actions) { display: grid; gap: 4px; }
.c-provider-account-row strong { overflow: hidden; color: #daddd8; font-size: 9px; font-weight: 590; text-overflow: ellipsis; white-space: nowrap; }
.c-provider-account-row small { overflow: hidden; color: #5e655f; font-size: 7px; text-overflow: ellipsis; white-space: nowrap; }
.c-account-name { display: flex; min-width: 0; align-items: center; gap: 9px; }
.c-account-name > span { display: grid; min-width: 0; gap: 3px; }
.c-horizontal-capacity { display: grid; grid-template-columns: 1fr 42px; align-items: center; gap: 8px; }
.c-horizontal-capacity > span { display: block; height: 9px; overflow: hidden; background: #292c2a; }
.c-horizontal-capacity > span i { display: block; height: 100%; background: #75bd7e; }
.c-horizontal-capacity > span i.degraded { background: #c9a34f; }
.c-horizontal-capacity > span i.critical { background: #d36058; }
.c-horizontal-capacity > strong { font: 600 9px/1 ui-monospace, monospace; }
.c-schedule.off strong { color: #db776e; }
.c-account-actions { display: flex; gap: 4px; }
.c-account-actions button { display: grid; width: 27px; height: 27px; place-items: center; border: 1px solid #363a37; border-radius: 2px; background: #151817; color: #8d958e; cursor: pointer; }

.c-route-cards { display: grid; grid-template-columns: repeat(3, 1fr); gap: 14px; }
.c-route-cards article { border: 1px solid #383b39; background: #111313; }
.c-route-cards article > header { display: flex; min-height: 42px; align-items: center; justify-content: space-between; padding: 0 11px; border-bottom: 1px solid #2d302e; }
.c-route-cards article > header span { display: flex; align-items: center; gap: 7px; }
.c-route-cards article > header strong { font-size: 10px; }
.c-route-cards article > header small { color: #646b65; font-size: 8px; }
.c-route-cards dl { display: grid; grid-template-columns: repeat(3, 1fr); margin: 0; }
.c-route-cards dl div { display: grid; gap: 5px; padding: 12px 10px; border-right: 1px solid #2b2e2c; }
.c-route-cards dl div:last-child { border-right: 0; }
.c-route-cards dt { color: #5e655f; font-size: 7px; }
.c-route-cards dd { margin: 0; font: 610 12px/1 ui-monospace, monospace; }
.c-route-cards p { margin: 0; padding: 10px; border-top: 1px solid #2b2e2c; color: #878f88; font: 500 8px/1.5 ui-monospace, monospace; }
.c-route-cards footer { padding: 8px 10px; border-top: 1px solid #2b2e2c; color: #616862; font-size: 7px; }
.c-model-table { overflow-x: auto; }
.c-model-table > header,
.c-model-table > div { display: grid; min-width: 800px; grid-template-columns: minmax(170px, 1fr) minmax(180px, 1fr) 140px 100px 80px 75px; align-items: center; gap: 12px; padding: 0 12px; }
.c-model-table > header { min-height: 30px; color: #606761; font: 600 7px/1 ui-monospace, monospace; }
.c-model-table > div { min-height: 48px; border-top: 1px solid #2b2e2c; color: #9ca39d; font-size: 9px; }
.c-model-table > div > span:first-child { display: grid; gap: 3px; }
.c-model-table strong { font-size: 9px; }
.c-model-table small { color: #5f6660; font-size: 7px; }

.c-settings { grid-template-columns: minmax(0, 1fr) 300px; align-items: start; }
.c-settings .c-page-heading { grid-column: 1 / -1; }
.c-settings-list > div { display: flex; min-height: 66px; align-items: center; justify-content: space-between; gap: 18px; padding: 10px 13px; border-bottom: 1px solid #2b2e2c; }
.c-settings-list > div:last-child { border-bottom: 0; }
.c-settings-list span { display: grid; gap: 4px; }
.c-settings-list strong { font-size: 10px; }
.c-settings-list small { color: #636a64; font-size: 8px; }
.c-settings-list button { display: inline-flex; min-height: 30px; align-items: center; gap: 8px; border: 1px solid #3b403c; background: #171a18; color: #c3c8c1; font-size: 9px; cursor: pointer; }
.c-fixture-policy { display: flex; gap: 12px; padding: 15px; border: 1px solid #344137; background: #111813; color: #82c78b; }
.c-fixture-policy span { display: grid; gap: 5px; }
.c-fixture-policy strong { color: #cfe1d1; font-size: 10px; }
.c-fixture-policy small { color: #6f8873; font-size: 8px; line-height: 1.5; }

@media (max-width: 1180px) {
  .c-status-deck { grid-template-columns: repeat(3, 1fr); }
  .c-health-score,
  .c-vital { border-bottom: 1px solid #303331; }
  .c-provider-grid { grid-template-columns: 1fr 1fr; }
  .c-provider:nth-child(2) { border-right: 0; }
  .c-provider:nth-child(-n + 2) { border-bottom: 1px solid #303331; }
  .c-monitor-grid { grid-template-columns: 1fr 1fr; }
  .c-events-monitor { grid-column: 1 / -1; }
  .c-provider-group { overflow-x: auto; }
  .c-provider-account-head,
  .c-provider-account-row { min-width: 1040px; }
}

@media (max-width: 900px) {
  .c-header { grid-template-columns: auto minmax(0, 1fr); }
  .c-clock { display: none; }
  .c-brand { padding-inline: 12px; }
  .c-header nav { justify-content: flex-start; overflow-x: auto; }
  .c-header nav button { padding-inline: 10px; }
  .c-route-cards { grid-template-columns: 1fr; }
  .c-settings { grid-template-columns: 1fr; }
  .c-settings .c-page-heading { grid-column: auto; }
}

@media (max-width: 767px) {
  .c-header { display: block; }
  .c-brand { min-height: 48px; border-bottom: 1px solid #2d302e; }
  .c-header nav { min-height: 42px; }
  .c-safety { padding: 0 10px; text-align: center; }
  .c-main { padding: 10px; }
  .c-status-deck { grid-template-columns: 1fr 1fr; }
  .c-provider-grid { grid-template-columns: 1fr; }
  .c-provider { border-right: 0; border-bottom: 1px solid #303331; }
  .c-section-heading { align-items: start; flex-direction: column; }
  .c-legend { justify-content: flex-start; }
  .c-monitor-grid { grid-template-columns: 1fr; }
  .c-events-monitor { grid-column: auto; }
  .c-page-heading { align-items: start; flex-direction: column; padding: 8px 0; }
  .c-account-summary { width: 100%; }
  .c-provider-group > header { grid-template-columns: 1fr 100px 90px; min-width: 600px; }
  .c-provider-group > header { overflow-x: auto; }
}
</style>
