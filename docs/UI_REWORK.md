# Operator UI Rework

## Status

Implemented on `ui/operator-console-implementation` for human review. The shell
recomposes existing routes and page owners; no backend contract was replaced and
no production deployment was performed. See [UI_AUDIT.md](UI_AUDIT.md) for the
architecture, evidence, and route-preservation matrix.

## Purpose

The operator console should make the gateway's current state and next useful
action clear. It should help an operator inspect provider accounts, model routes,
quota and health, and recent gateway activity without requiring knowledge of the
backend module layout.

The interface should be compact, English-first, and suited to routine operation
and incident response. It must keep existing authentication, API, store, route,
provider, and account behavior unless a separately tested change replaces it.

## Design Direction

- Lead with service state, actionable exceptions, and the controls needed to
  resolve them.
- Use a quiet work surface with dense tables, restrained borders, and semantic
  status color. Do not use decorative mesh backgrounds, broad gradients, or
  equal-weight card grids as the main hierarchy.
- Keep provider, account, group, route, model, quota, usage, and health as distinct
  terms.
- Prefer one page heading, one clear primary action, and contextual secondary
  actions.
- Preserve recognition: keep current values, active filters, applied routes, and
  consequences visible near each action.
- Use motion only for state change and feedback, and respect reduced motion.

## Information Architecture

### Overview

Answer four questions in order:

1. Is the gateway serving traffic?
2. Which accounts, routes, or providers need attention?
3. What changed in recent traffic, latency, errors, quota, or cost?
4. Where should the operator go next?

Use existing dashboard and operations data. Show a compact health summary,
actionable exceptions, current traffic, and recent trends. Empty panels must say
why they are empty and link to the next useful action when one exists.

### Accounts

Treat provider accounts and pools as the primary objects. Expose provider, auth
type and auth health, schedulability, status, quota or cooldown, group membership,
priority, recent usage, and available actions. Preserve existing OAuth,
reauthorization, testing, bulk edit, refresh, and destructive flows.

### Models & Routing

Group the existing Groups, Channel Pricing, and Channel Monitor routes under one
area with local subnavigation. Make the path from public model to target platform,
upstream model, group, account pool, priority, and fallback legible. Keep current
pricing, policy, model mapping, composite-route, and monitoring contracts
separate until their owners can be changed safely.

### Activity

Bring operational monitoring, usage records, errors, and audit history under one
area with consistent time, model, provider, group, account, status, and request
filters. Preserve each route's current URL query keys, request cancellation,
auto-refresh, export, and drill-down behavior during the first pass.

### Settings

Organize existing controls by operator task, such as gateway behavior, access and
authentication, feature availability, notifications, security, and system
maintenance. Do not split the current bulk settings save until independent backend
contracts exist. Preserve step-up authentication, configured-secret handling,
web-search save behavior, and settings-store refresh.

### Secondary Capabilities

Existing user, subscription, order, affiliate, announcement, redemption,
promotion, risk-control, plugin, proxy, and other feature routes remain available
during migration. They should use contextual secondary navigation rather than
expanding the five-area top level. No route disappears merely because it is not in
the first operator map.

## Navigation

- Use Overview, Accounts, Models & Routing, Activity, and Settings as the stable
  top level.
- Keep current paths and deep links. The new navigation initially points to the
  existing views.
- Use local tabs or a compact secondary list for routes within an area. Tabs must
  represent views, not one-off commands.
- Keep feature-controlled destinations visible while feature state is unknown,
  matching current tolerant behavior. The router remains the source of truth for
  authentication, authorization, modes, compliance, and feature access.
- Keep personal account actions separate from operator navigation, using the user
  menu or an explicit personal section.
- On narrow layouts, use a dismissible navigation drawer and keep the current page
  title visible outside the drawer.
- Retain direct access to legacy routes until the mapped replacement passes parity
  checks.

## Component Strategy

- Reuse `AppLayout`, `AppHeader`, `AppSidebar`, `DataTable`, `Pagination`,
  `SearchInput`, `Select`, `StatusBadge`, `EmptyState`, `LoadingSpinner`, and the
  existing icon set where they meet the required behavior.
- Do not add a second component library, design-token layer, query library, or
  operator-wide Pinia store for the first migration.
- Keep remote data, request cancellation, forms, and serialization with their
  current page owners. Extract a unit only after two real consumers share the same
  behavior.
- Use tables for repeated operational records and compact unframed sections for
  page composition. Avoid cards nested inside cards.
- Use text plus icons or shape for health and severity. Color alone must not carry
  status.
- Keep control dimensions stable so loading labels, counts, status text, and long
  model names do not shift the layout.
- `BaseDialog` traps and restores focus, lets only the top dialog handle Escape,
  and uses a stack-aware body scroll lock.

## Data and State Contracts

- Use `src/api/client.ts` for all HTTP work so auth, refresh, locale, timezone,
  cancellation, response unwrapping, and structured errors stay consistent.
- Treat URL query parameters as the source of truth only where the existing route
  already supports refresh and back/forward navigation. Ops does; Usage currently
  reads `start_date`, `end_date`, and `user_id` on mount but does not write or watch
  query state.
- Preserve account ETag refresh and stable object references.
- Preserve Groups request cancellation and every existing payload transform,
  including model routing and composite routes.
- Preserve Usage list and export cancellation.
- Preserve Ops request sequencing, cancellation, deferred loading, auto refresh,
  and URL synchronization.
- Preserve Settings bulk normalization, secret clearing, step-up behavior,
  separate web-search save, and public/admin settings refresh.
- Do not infer access from navigation visibility. Route guards and backend
  authorization remain authoritative.

## Implemented Migration

1. Production-router tests cover setup, authentication, admin access, simple and
   backend modes, compliance, feature-disabled routes, unknown feature state, and
   protected deep links.
2. The shell presents five operator areas while preserving secondary and personal
   routes. Area-local links share active state with their primary sidebar owner.
3. Overview uses existing dashboard calls to show traffic state, account
   exceptions, direct area links, compact usage and cost context, optional charts,
   and inline snapshot recovery.
4. Accounts keeps its ETag refresh, stable row references, paging, filters, sorting,
   quota and billing work, OAuth, reauthorization, tests, bulk actions, and dialogs.
   Presentation changes improve schedulability, scan order, bulk-action wrapping,
   and the empty state.
5. Models & Routing joins Groups, Channel Pricing, and Channel Monitor through
   local navigation. Each route still owns its state, cancellation, and payloads.
6. Activity joins Ops, Usage, and Audit Logs through local navigation. Ops keeps
   query and history synchronization; Usage retains initial query hydration and its
   existing cancellation behavior.
7. Settings keeps one normalized save lifecycle, configured-secret handling,
   step-up authentication, separate web-search saving, and store refresh. Its task
   tabs now expose explicit tab and panel semantics with a flatter hierarchy.

## Legacy Coexistence

- The shell links to the current views. It does not create duplicate workflow
  implementations.
- A migrated view takes over its existing route only after it covers the same data,
  permissions, actions, validation, pending states, failures, cancellation,
  recovery, and direct links.
- If a preview is needed, keep it development-only, use the real API modules and
  stores, and remove it when the production migration starts.
- Keep unmapped and optional routes in secondary navigation. Preserve their direct
  URLs and router metadata.
- Do not use a permanent feature flag or second legacy shell unless a measured
  rollout risk requires one.

## English-First Copy

- English is the canonical source, configured default, and fallback for all new or
  substantially reworked copy.
- Add the English message before or with the UI change. Other locales may translate
  it or use the English fallback; never expose a raw translation key.
- Retain the locale selector, saved user preference, and current Chinese-browser
  compatibility unless a separate product decision changes them.
- Use one plain term for each domain object and action. Prefer labels such as
  `Disable account`, `Test account`, and `Save routing changes` over generic
  `Continue` or `Submit`.
- Do not rename API fields, provider names, protocol names, model identifiers,
  serialized values, or compatibility terms for language consistency.
- Do not perform a blind translation of inherited source or untouched screens.

## Responsive Behavior

- Choose layout changes where content and controls stop fitting, not from a device
  name list.
- Keep the page title, current system state, active filters, and primary action
  visible at narrow widths.
- Let secondary navigation move into a drawer or local overflow menu without
  hiding critical status or actions.
- Give operational tables explicit column priority. Preserve identity, status, and
  primary action; let secondary detail move into an expandable row or detail view.
- Allow long provider, model, route, and account names to wrap or truncate with an
  accessible full value. Do not let them overlap controls.
- Verify narrow and wide content, 200% zoom, reflow, long English text, translated
  text, and data-heavy tables.

## Accessibility

- Meet WCAG 2.2 AA for every changed workflow.
- Use landmarks and one visible `h1` per page. Keep heading order logical.
- Use native buttons, links, inputs, checkboxes, and tables before custom ARIA
  patterns.
- Provide persistent labels, concise constraints, and errors that identify both
  the problem and the correction.
- Keep every action keyboard operable with visible, unobscured focus and logical
  focus order.
- Trap and restore focus for modal dialogs, support Escape where dismissal is safe,
  and confirm destructive actions in proportion to consequence.
- Meet contrast and target-size requirements, and never use color alone for
  status.
- Provide text summaries or tables for chart information needed to make a decision.
- Respect reduced motion. Onboarding must remain dismissible and replayable and
  must not block routine operation after completion.

## States and Recovery

Every migrated workflow must cover the states it can actually enter:

- initial loading and background refresh;
- empty, filtered-empty, and partial data;
- success, stale data, warning, and error;
- disabled or unavailable features;
- validation and step-up authentication;
- cancellation, retry, and back navigation;
- destructive confirmation and post-action result.

Keep feedback near the affected region. A whole-page spinner should not block
usable partial data. Prevent duplicate submissions when repeating an action could
change state twice.

## Testing Strategy

Use the smallest layer that proves each contract:

1. Vitest for payload transforms, state helpers, route metadata, and the real
   router guard behavior. Do not test copied guard logic.
2. Existing API tests for request methods, parameters, headers, cancellation, and
   response/error translation.
3. Component tests for shared interaction behavior, especially dialog focus,
   navigation state, forms, and destructive confirmation.
4. Playwright for login-to-operator navigation, one primary flow per migrated
   area, narrow and wide layouts, keyboard operation, focus, long content,
   loading, empty, partial, error, disabled, success, cancellation, and recovery.
5. `pnpm lint:check`, `pnpm typecheck`, `pnpm test:run`, and `pnpm build` for each
   code-bearing UI change.

Browser evidence must name the routes, widths, states, and interactions actually
checked. A build, screenshot, click path, and accessibility pass prove different
things and must not be reported as interchangeable.

A one-off local browser qualification covered all five areas at 1440, 1280, 1024,
768, and 390 pixels in light and dark mode, plus document overflow, a 640-pixel
reduced-motion reflow proxy, and Overview failure and retry. The retained CI suite
covers navigation, active state, deep links, history, the narrow drawer, table and
long-page scroll geometry, compact Activity geometry, simple-mode visibility,
route-guard authority, and account-dialog focus. Live services, writes, actual
browser zoom, translated long text, and every inherited dialog remain outside
this fixture-only work.

## Workflow Definition of Done

A workflow can replace its inherited view only when it:

- uses the same auth and API boundary;
- preserves current routes, query state, permissions, and feature behavior;
- covers real data and every current write action in scope;
- preserves cancellation, refresh, serialization, validation, and security
  invariants from the audit matrix;
- exposes loading, empty, partial, error, disabled, success, and recovery states;
- passes relevant unit, API, component, browser, and production-build checks;
- remains usable with keyboard, narrow and wide layouts, zoom, reflow, long text,
  dark mode, and reduced motion.

## Review Boundary

This branch is the first reviewable operator-console implementation. It changes
information architecture and presentation while retaining existing route and
workflow owners. Production adoption still requires human UI review and any
deployment-specific qualification; this branch does not deploy itself.
