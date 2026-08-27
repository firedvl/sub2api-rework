# Operator UI Audit

## Status

Complete for the foundation phase and updated for the operator-console
implementation. The implementation changes the shell and presentation on existing
routes; it does not replace backend APIs, page-owned request lifecycles, stores, or
production behavior.

## Scope and Evidence

The audit covered the frontend application shell, route table, authentication
guards, shared HTTP client, stores, admin views, common components, localization,
styles, and existing tests.

Rendered checks used a local Vite build with mocked read responses. No production
credentials or production data were used.

| Route | Wide viewport | Narrow viewport | States inspected |
| --- | --- | --- | --- |
| `/login` | 1280 x 720 | 390 x 844 | Default sign-in form |
| `/admin/dashboard` | 1440 x 1247 | 390 x 1835 | Automatic onboarding overlay, dismissed dashboard, populated summary metrics, empty charts |

A one-off local implementation pass used fixture-only Playwright coverage for all
five areas at 1440, 1280, 1024, 768, and 390 pixels in light and dark mode. It also
checked document overflow, a 640-pixel reflow proxy with reduced motion, and
Overview failure and retry. The retained CI suite covers navigation history and
deep links, the narrow navigation drawer, table and long-page scroll geometry,
compact Activity geometry, simple-mode visibility, guard authority, and
account-dialog focus trapping and restoration. The pass did not exercise live
services, write operations, every inherited dialog, actual browser zoom, or
translated long text.

## Implementation Update

- The sidebar now uses Overview, Accounts, Models & Routing, Activity, and Settings
  as its operator top level. Existing secondary routes remain under `More`, the
  personal section, or area-local navigation.
- Models & Routing and Activity use route-aware local navigation. Feature and
  simple-mode filtering still affect visibility, while production router guards
  remain authoritative.
- Overview now leads with gateway traffic state, account exceptions, direct area
  links, and compact traffic and cost context. Empty chart regions no longer
  dominate the page, and snapshot failure has an inline Retry action.
- Accounts keeps its existing data and modal ownership while adding a labelled
  schedulability switch, clearer column sizing, a task-specific empty state, and a
  wrapping bulk-action bar.
- Settings keeps its bulk save, secret, step-up, web-search, and store-refresh
  contracts while using a flatter task-tab treatment with explicit tab and panel
  relationships.
- `BaseDialog` now traps focus in the top dialog, restores focus, handles Escape
  only at the top of the stack, and keeps body scroll locked until the last dialog
  closes.
- The production router has direct Vitest coverage, and the operator shell has a
  maintained Chromium Playwright suite in CI.

## System Inventory

| Layer | Current implementation | Migration consequence |
| --- | --- | --- |
| Framework | Vue 3, TypeScript, Vite, Pinia, Vue Router, Tailwind CSS, Axios, vue-i18n, Vitest | Keep the current stack. No new UI framework or state library is needed. |
| Entry and shell | `src/main.ts`, `src/App.vue`, `AppLayout.vue`, `AppHeader.vue`, and `AppSidebar.vue` | The shell is the smallest place to introduce the five-area information architecture, but it is shared by admin and personal routes. |
| Routing | `src/router/index.ts` owns public, personal, and admin routes plus authentication, admin, mode, compliance, and feature checks | Preserve every current path and deep link until its workflow reaches parity. Navigation visibility must not replace route authorization. |
| Shared state | Pinia stores cover auth, public settings, admin settings, compliance, announcements, onboarding, payments, and subscriptions | Operator-domain lists and forms are page-owned. Do not add a global operator store merely to support a new shell. |
| API boundary | `src/api/client.ts` adds the bearer token, locale, timezone, and UI-origin headers; unwraps responses; retries one token refresh; and normalizes errors | Reuse the client and existing API modules. A new console must not bypass refresh, cancellation, or structured error behavior. |
| Admin data | `src/api/admin/` contains domain modules for accounts, groups, channels, dashboard, operations, usage, audit, settings, and supporting features | Recompose existing calls before proposing backend changes. |
| Common components | Existing primitives include `DataTable`, `Pagination`, `SearchInput`, `Select`, `StatusBadge`, `EmptyState`, `LoadingSpinner`, `BaseDialog`, and icon components | Reuse these where their behavior fits. Correct shared accessibility defects before expanding their use. |
| Styling | Tailwind utilities, `src/style.css`, semantic primary/accent/dark palettes, dark mode, and shared component classes | Retain tokens and dark mode. Reduce mesh decoration, gradients, heavy shadows, and excess card framing in changed operator views. |
| Localization | English and Chinese message bundles; English is the configured default and fallback; a Chinese browser selects Chinese when no preference is saved | Treat English as the canonical source for new copy while retaining explicit locale choice and existing Chinese compatibility. |

## Current Admin Information Architecture

The sidebar exposes more than 20 possible admin destinations after feature flags,
mode restrictions, nested groups, and custom menu items are applied. It also keeps
personal account destinations in the same shell. The target five areas therefore
describe top-level navigation, not a deletion of current capabilities.

| Target area | Current routes that supply it |
| --- | --- |
| Overview | `/admin/dashboard`, with operational detail in `/admin/ops` |
| Accounts | `/admin/accounts`; related proxy and provider controls remain separate |
| Models & Routing | `/admin/groups`, `/admin/channels/pricing`, `/admin/channels/monitor` |
| Activity | `/admin/ops`, `/admin/usage`, `/admin/audit-logs`, and monitoring/error detail reached from those views |
| Settings | `/admin/settings`; supporting access, feature, payment, plugin, notification, security, and system routes remain available during migration |

Routes outside this first mapping, including user, subscription, order, affiliate,
announcement, redemption, promotion, risk-control, and plugin administration, must
remain reachable. They can move under contextual secondary navigation only after
their permissions and workflows are mapped.

## Ranked Findings

### 1. Navigation follows backend modules instead of operator tasks

Evidence: `AppSidebar.vue` builds a long list of independent destinations for
dashboard, operations, users, groups, channels, accounts, usage, audit, settings,
and optional business features. The rendered wide dashboard requires a long scan,
and Models & Routing and Activity each span several routes.

Impact: an operator must know which backend module owns a model, route, request,
or health signal before acting. The five-area top level should group existing
routes without changing their contracts.

Implementation outcome: resolved in the operator shell. The five areas own the
primary navigation, while local navigation and `More` preserve secondary routes.

### 2. Overview does not lead with exceptions or a clear narrow-page identity

Evidence: the dashboard starts with eight equally weighted metric cards, followed
by quick actions and three large chart panels. In the audited empty state, the
charts consume most of the page without a next action. `AppHeader.vue` hides the
route title below the `lg` breakpoint, and the page body has no replacement `h1`.

Impact: active account errors compete with totals, costs, and user metrics. On a
narrow viewport the operator sees no visible page title. A reworked Overview
should lead with service state and actionable exceptions, then traffic and cost
context.

Implementation outcome: resolved. The header keeps a visible page title at narrow
widths, and Overview now leads with gateway state, account exceptions, direct area
links, and compact operational context.

### 3. Page-owned workflow logic makes route replacement high risk

Evidence: `AccountsView.vue`, `GroupsView.vue`, and `SettingsView.vue` are large
single-file views. Their behavior is not limited to presentation:

- Accounts uses ETag-based incremental refresh and preserves active row and modal
  references.
- Groups cancels superseded loads and serializes pricing, limits, model routing,
  model lists, media controls, policy settings, and composite routes.
- Usage cancels list and export work and restores filters from the URL.
- Ops cancels stale snapshots, sequences deferred panels, and synchronizes filters
  and fullscreen state with the URL.
- Settings loads and normalizes one large payload, clears returned secret fields,
  performs step-up-authenticated saves, saves web-search settings separately, and
  refreshes both public and admin settings stores.

Impact: a shallow replacement can appear correct while breaking cancellation,
refresh, serialization, or security behavior. Initial work must recompose the
current page owners rather than duplicate their API logic.

### 4. Feature visibility and authorization have different jobs

Evidence: sidebar feature filtering treats unknown feature state as visible to
avoid flicker. The router loads settings when needed and denies access only when a
successfully loaded value explicitly disables a route. Router guards also enforce
authentication, admin access, simple mode, backend mode, compliance, payment, and
risk-control conditions.

Impact: a new navigation model that treats a hidden link as access control can
either expose an unusable destination or block a valid one after a transient
settings error. Existing route guards remain authoritative.

### 5. Shared dialog behavior is not ready for broader operator workflows

Evidence: `BaseDialog.vue` moves focus to the first focusable element and restores
focus on close, but it does not trap focus. Its body scroll lock is one global CSS
class removed by any closing or unmounting dialog, so overlapping dialogs are not
reference-counted.

Impact: keyboard focus can escape a modal, and one dialog can unlock background
scroll while another remains open. This is a pre-existing issue, not a regression
from this audit. Fix it before moving more sensitive account or settings actions
onto the shared primitive.

Implementation outcome: resolved in `BaseDialog` with stack-aware focus, Escape,
focus restoration, listener cleanup, and body scroll locking.

### 6. Existing tests do not protect a navigation migration end to end

Evidence: the repository has Vitest coverage for API modules and components, but
`router/__tests__/guards.spec.ts` tests a copied `simulateGuard` implementation
instead of the production guard. No maintained end-to-end operator workflow was
found.

Impact: route behavior can drift while tests remain green. The first
implementation slice should test the real router and add browser coverage for the
primary operator path before changing navigation.

Implementation outcome: resolved for the migration boundary. Tests now import the
production router, and the maintained Playwright suite covers the operator shell,
deep links, history, narrow navigation, guard authority, and dialog focus.

### 7. English-first needs a precise compatibility rule

Evidence: English is the default and fallback locale, while saved preferences and
Chinese browser detection can select Chinese. The source also contains inherited
Chinese comments and message content.

Impact: forcing English for every existing user would be a compatibility change,
while adding new features only in inherited non-English copy would violate the
project direction. English should be canonical for new and substantially reworked
copy; other locales may translate it or fall back to English.

## Route-Preservation Matrix

| Target area | Current owners | Behavior that must survive | First safe boundary |
| --- | --- | --- | --- |
| Overview | `/admin/dashboard`; `api/admin/dashboard.ts`; `DashboardView.vue`; operational summaries from `/admin/ops` | Dashboard snapshot and chart loading, partial and empty data, date range and granularity, links into detail views, feature-aware quick actions | Recompose existing read APIs into health, exceptions, traffic, and cost sections. Keep `/admin/dashboard` and its detail links. |
| Accounts | `/admin/accounts`; `api/admin/accounts.ts`; `AccountsView.vue`; `components/account/` | Filters, paging, sorting, ETag refresh, quota and billing refresh, OAuth and reauthorization, create/edit/bulk actions, testing, temporary unschedulable state, destructive confirmation | Keep the existing page owner and dialogs. First change navigation and table hierarchy, then extract only proven repeated presentation units. |
| Models & Routing | `/admin/groups`, `/admin/channels/pricing`, `/admin/channels/monitor`; group, channel, and monitor API modules | Superseded-request cancellation, pricing and limit normalization, platform-specific controls, model and reasoning mappings, composite-route priority/preview CRUD, monitor feature state | Present the three existing routes as one area with local subnavigation. Do not merge their state or payload builders in the first pass. |
| Activity | `/admin/ops`, `/admin/usage`, `/admin/audit-logs`; ops, usage, and audit API modules | Ops URL synchronization, stale-request cancellation, auto refresh, deferred panels, error drill-down; Usage initial query hydration plus list and export cancellation; audit pagination and step-up-gated actions | Share navigation and filter language first. Keep each route's query keys and request lifecycle until browser tests cover equivalence. |
| Settings | `/admin/settings`; `api/admin/settings.ts`; `SettingsView.vue`; `adminSettings` and `app` stores; step-up dialog | Bulk load/save normalization, configured-secret flags and secret clearing, step-up authentication and cancellation, separate web-search save, store refresh, feature toggles that affect routing/navigation | Reorganize the existing form into operator task sections without splitting the save contract. Split ownership only after backend contracts support independent saves. |

## Migration Constraints

- Do not rewrite backend APIs for a navigation change.
- Do not create a second implementation of page-owned request or serialization
  logic.
- Preserve current URLs, query parameters, permissions, feature behavior, and
  direct links.
- Keep legacy routes active until the replacement for that workflow passes parity
  checks.
- Correct shared shell and dialog accessibility before relying on them for more
  workflows.
- Introduce no new dependency unless repository-native code cannot meet a proven
  requirement.

## Prototype Decision

No prototype was added in this phase. A new shell with placeholder pages would
duplicate navigation without proving account, routing, activity, or settings
behavior. The audit and migration contract are the smallest durable groundwork;
the next code change should begin with real-router and browser regression coverage.
