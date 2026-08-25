# Roadmap

Status terms: **Supported** means maintained in this fork, **Experimental** means
implemented but still being qualified, **Known limitation** records a confirmed
boundary, and **Planned** means no compatibility claim is made.

## Milestone 1: Gateway Foundation

- Experimental: one downstream API key across authorized provider routes
- Experimental: composite model routing
- Experimental: OpenAI, Gemini, and Claude paths inherited from Sub2API
- Experimental: Codex Responses and client function tools
- Planned: standalone Codex web search
- Known limitation: some Antigravity transports reject native search and custom
  functions in the same inference request

## Milestone 2: Operator UI

- Planned: Overview
- Planned: Accounts
- Planned: Models & Routing
- Planned: Activity
- Planned: Settings
- Planned: English-first operator workflows

## Milestone 3: Integration Hardening

- Planned: additional models and providers
- Planned: cancellation and failure-semantics qualification
- Planned: quota and account-health visibility
- Planned: explicit routing controls

## Milestone 4: External Client Integrations

- Planned: generic OpenAI-compatible clients
- Planned: agent and orchestration clients

Milestone status changes require tests for the stated path. A result from one
provider transport does not establish support for every transport or account
type.
