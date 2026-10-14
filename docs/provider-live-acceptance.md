# Native provider live acceptance

This procedure is opt-in. It can consume API credits and does not deploy
production, enroll accounts, change billing, or qualify a subscription proxy.
Use a disposable staging Gateway with the exact reviewed source/image, database
and Redis isolation, and an approved API-key account. Keep production's OpenAI
key, groups, routes, credentials, and services out of this fixture.

## Before inference

Record the source SHA, OCI digest, Gateway version/schema, Codex CLI and desktop
versions, approved route/group, public alias, dispatched upstream model, and
official provider inventory from the authorized account. Do not infer inventory
or entitlement from alias publication. Anthropic's model-list and Gemini's
model-list endpoints are separate from the Gateway catalog.

Confirm API access and spending permission for this route, including any free
tier or included plan credits. Set an explicit call/output-token allowance;
disable paid overages where the provider supports it. A token allowance is not
a dollar billing cap. Keep credentials in the supported store/environment and
keep reports outside Git. Do not log request headers, tokens, credential stores,
raw provider errors, account identities, or private task material.

Run capability discovery with schema 2 and preflight for the intended request.
`ROUTE_DEPENDENT` is advisory; unsupported requests must fail before provider
transport. Never enable an account, change a mapping, or substitute another
provider to make preflight green.

## Bounded Gateway primitive probe

Agent-Setup provides `scripts/probe_provider_responses.py`. Its default is a dry
run with no credential read or network request:

```sh
python3 scripts/probe_provider_responses.py --base-url https://isolated-gateway.example/v1 --model APPROVED_ALIAS
```

Only in the approved staging environment, use `--execute --authorized-api
--spending-approved --gateway-revision REVIEWED_SHA --route-scope APPROVED_GROUP`.
Supply the isolated downstream key through `PROVIDER_QUALIFICATION_KEY`; the
probe never chooses the normal `SUB2API_API_KEY`. Use `--max-output-tokens` for
the approved per-call allowance (default128, maximum2048). Windows uses `py -3`.

The probe sends at most four requests: text, streaming text, a harmless function
declaration with a unique token, and its matched result continuation. It executes
no model tool. It replays every output item, including provider thought envelopes,
and stops on a failure without retries. HTTP redirects are refused; time and
response-size limits apply. Body reads use a monotonic deadline; DNS, connect,
TLS and header parsing retain standard-library/OS timeout limits. A failed or incomplete response is a failed test,
even if the HTTP status is 200 or a tool call was partially emitted.

The report records primitive acceptance and usage counters. It cannot establish
API authorization, account inventory, actual upstream identity, billed cost,
Codex compatibility, or routing qualification. Do not turn it into a qualification
receipt. Match its request IDs, time window, model mapping and usage against
staging Gateway/provider receipts; distinguish public alias, dispatched ID, and
provider `modelVersion`/response-model evidence. Any missing or conflicting
identity blocks promotion.

## Extended provider cases

Use the existing mocked tests as request/response fixtures for the same isolated
account. Execute each live case once, with a separate token and receipt:

| Case | Required evidence |
| --- | --- |
| Multi-turn | User/assistant history retained; signed parts unaltered; result pairing and order retained |
| Stream lifecycle | Ordered created/item/content/argument/terminal events; continuous sequence; terminal output matches emitted identities |
| Function tools | Multiple calls remain distinct; IDs and namespaces survive results and subsequent turns; no arbitrary text execution |
| Error | Invalid request rejected before transport; authorized provider auth/entitlement error normalized without secrets; no blind retry |
| Cancellation/timeout | Cancel a started stream once; no completed status fabricated; observed paid usage accounted; no completed tool replay |
| Accounting | Input, cached input, output and thinking counters agree with provider evidence, including failure/disconnect paths |
| Unsupported shapes | Custom grammar, deferred discovery, disabled parallel calls, reasoning/verbosity controls and modalities reject where unsupported |

Do not repeatedly probe an unavailable alias. For an upstream404, inspect the
approved model inventory, mapping, provider dispatch and entitlement. A correct
public name does not prove that an account or provider offers it.

## Actual Codex acceptance

First run Agent-Setup's synthetic `probe_codex_harness.py` on the exact installed
client to obtain its structural request contract. Compare it with the adapter
subset. Current Gemini gaps include grammar-based custom `exec`, explicit
reasoning/verbosity, cache/client metadata, and encrypted V2 tasks. Native
Anthropic explicitly rejects V2 `agent_message`; use a supported V1 profile only
after exact-client acceptance. Do not patch bundled model metadata, drop encrypted
payloads, or translate tool operations into text prompts.

Use an isolated profile/home and public disposable fixture files. Preserve the
normal profiles. Exercise direct shell/file tool creation, read, and result
continuation; V1 task delivery; each supported V2 task/result path; child tools;
parent continuation; one failed child before work; one failure after a partial
edit; and unknown execution status. Verify the parent retains completed results,
reconciles partial changes and never blindly replays work. A version the provider
cannot support remains disqualified.

Then check the intended configured integrations: global/role instructions,
trusted hooks, skills, plugins, MCP, browser tools, sandbox write denial and
approval handling. Disabled integrations in the synthetic fixture are not live
acceptance. An official Claude SDK worker needs its own permission/usage tests
and receipt; it is not a native Responses child or a way around V2 encryption.

Only after all required cases pass may Agent-Setup's existing route-, revision-
and client-scoped receipt mark a native model qualified. Missing evidence stays
pending. Provider/model substitution must be visible and must never bypass
access, cost, sandbox, or approval boundaries.

## Deployment gate

Keep the immutable `.8`/schema251/updater1.1.6 baseline and a checked deployment
configuration, PostgreSQL backup and application/Redis backup records. Verify
the exact candidate's Linux updater staging job, OpenAI regressions and schema
contract before recommending deployment. Native Gemini tests do not qualify
any production account. Publication and production deployment require separate
authorization for the exact source SHA and image digest.

After an authorized deployment, check HTTP/container health, Gateway version,
database/schema251, Redis, assets, existing OpenAI streaming/tool behavior and
capabilities v2. Optional external routes stay disabled until live qualification.
For an ordinary same-schema application rollback, preserve acknowledged database
writes and independently recheck health/version/schema. After possible failed
candidate exposure, obey updater `critical`/`EXPOSURE_POSSIBLE` recovery rules;
do not restore a database merely to remove a provider adapter.
