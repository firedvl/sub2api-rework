# Operator UI Rework

## Status

Planned. The inherited UI remains the active interface. No production route is
replaced by this document.

## Purpose

The new interface should help an operator inspect provider accounts, model
routes, quota and health, and recent gateway activity. It should be compact,
English-first, and easy to scan during routine operation and incidents.

## Information Architecture

1. Overview
2. Accounts
3. Models & Routing
4. Activity
5. Settings

## Design Rules

- Lead with current state, exceptions, and actions an operator can take.
- Reuse backend APIs, authentication, stores, and domain terms where they remain
  correct.
- Keep provider, account, group, route, model, quota, and health as distinct
  concepts.
- Use tables and compact controls for repeated operational data.
- Cover loading, empty, partial, error, disabled, and recovery states.
- Meet WCAG 2.2 AA for changed workflows, including keyboard access, visible
  focus, contrast, reflow, and reduced motion.
- Keep narrow and wide layouts usable without hiding critical status.
- Do not copy another product's visual identity or add decorative dashboard
  clutter.

## Migration

1. Audit routes, stores, API clients, components, localization, and design tokens.
2. Add a development-only shell or route that reuses existing data adapters.
3. Move one operator workflow at a time behind an explicit route boundary.
4. Keep the legacy UI available until the replacement covers the same real data,
   permissions, errors, and recovery paths.
5. Qualify each migrated workflow in a browser before changing production routes.

Substantially reworked copy must be English-first. Existing protocol and provider
identifiers remain unchanged.
