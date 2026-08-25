# Operator UI Rework

## Status

Planned. The inherited UI remains active. No production route is replaced by this
document. See [UI_AUDIT.md](UI_AUDIT.md) for the current architecture, findings,
and route-preservation matrix.

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
- Before broader reuse, update `BaseDialog` to trap focus, restore focus, close only
  the topmost eligible dialog with Escape, and use a stack-aware body scroll lock.

## Data and State Contracts

- Use `src/api/client.ts` for all HTTP work so auth, refresh, locale, timezone,
  cancellation, response unwrapping, and structured errors stay consistent.
- Treat URL query parameters as the source of truth for shareable Activity filters
  and any existing filtered route that already supports refresh and back/forward
  navigation.
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

## Migration Strategy

1. Protect the boundary. Test the production router guards directly and add a
   browser smoke path for login, admin access, navigation, and a visible page
   heading at narrow and wide widths.
2. Change the shell. Introduce the five-area navigation while every link still
   opens its current route and view. Keep secondary legacy destinations reachable.
3. Recompose Overview. Use existing read APIs and link exceptions into current
   detail routes. Do not change write behavior.
4. Refine Accounts. Preserve the current page owner and dialogs while improving
   hierarchy, scanning, filters, and responsive behavior.
5. Join Models & Routing through navigation and shared terminology. Keep Groups,
   Channels, and Monitor state and payload builders separate.
6. Join Activity through navigation and filter language. Retain each route's query
   and cancellation lifecycle.
7. Reorganize Settings last. Keep the single save contract and security behavior
   until the backend provides smaller independent contracts.

Each step is a separate reviewable change. Do not move to the next workflow until
the current one passes its parity checks.

## Legacy Coexistence

- Before migration, the new shell links to the current view. It does not create a
  duplicate implementation.
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

## First Implementation Slice

Start with real-router tests and shell/browser coverage. Then add the five-area
navigation while it still points to the inherited views. This is the smallest
change that improves orientation without duplicating page-owned behavior.
