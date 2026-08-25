# Security Policy

## Supported Versions

Sub2API Rework has no stable release yet. Security fixes target the current
`main` branch and may not be backported to older commits.

## System and Scope

The repository contains an internet-facing API gateway, an operator and user
web application, OAuth and API-key integrations, provider account pools,
billing and usage records, and deployment assets. Reports may cover the backend,
frontend, deployment configuration, authentication and authorization, protocol
translation, provider routing, and secret handling in this repository.

## Threat Model and Trust Boundaries

Treat downstream request bodies, headers, uploaded content, callback data,
provider responses, configurable upstream URLs, and browser input as untrusted.
Provider credentials, OAuth tokens, downstream API keys, session material,
account routing, tenant data, usage records, and operator actions cross security
boundaries.

## Security Invariants

- Authenticate and authorize a request before protected gateway or management
  behavior runs.
- Do not expose upstream credentials or stored secrets to downstream clients,
  logs, errors, metrics, or public reports.
- Preserve user, group, and account-pool isolation during routing and billing.
- Validate network destinations and redirects at configurable outbound-request
  boundaries.
- Verify OAuth state and callback inputs before storing credentials.
- Keep parsing and resource use bounded for attacker-controlled input.
- Fail without committing partial billing, credential, or routing state when an
  operation cannot complete safely.

## Reporting a Vulnerability

Use GitHub's private **Report a vulnerability** flow for this repository. Do not
open a public issue with exploit details, credentials, personal data, or private
deployment information. Include affected versions, reachability, impact, and a
minimal reproduction that does not target systems you do not own.

Provider terms-of-service disputes and upstream availability problems are not
security reports by themselves. A gateway flaw that exposes credentials,
bypasses authorization, crosses tenant boundaries, or makes unsafe outbound
requests remains in scope even when it depends on an upstream provider.

## Known Limits

Provider transports and compatibility paths are still experimental. A passing
provider integration test establishes behavior only for the tested path; it does
not establish a security guarantee for every account type, model, or transport.
