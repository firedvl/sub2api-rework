# Sub2API Rework

Sub2API Rework is an experimental, English-first fork of
[Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api). It develops a
provider-neutral gateway that lets OpenAI-compatible clients use one downstream
API key while Sub2API owns provider credentials, account pools, routing,
protocol translation, and usage accounting.

This repository is not a claim of compatibility with every model or provider.
Provider behavior can differ by account type and upstream transport. See
[Codex compatibility](docs/CODEX_COMPATIBILITY.md) for tested boundaries and
known limitations.

## Status

| Area | Status | Notes |
| --- | --- | --- |
| Upstream baseline | Supported | Pinned to upstream `v0.1.183` (`e8cb019fabf8b55199436229044cbf9aa7a82564`) |
| One-key multi-provider routing | Experimental | Uses inherited API-key, group, account-pool, and composite-routing behavior |
| OpenAI-compatible gateway | Experimental | Preserves upstream Responses and Chat Completions paths |
| Codex client tools | Experimental | Antigravity forwarding reuses shared namespace and custom-tool adaptation |
| Standalone Codex web search | Experimental | Uses a separate, configurable `alpha_search` Composite route |
| Operator console | In review | The Vue UI ports codex-lb's visual system while preserving Sub2API workflows |

No stable rework release exists yet.

## Direction

```text
OpenAI-compatible clients
          |
          | one downstream API key
          v
       Sub2API
          |
          +-- OpenAI-compatible accounts
          +-- Antigravity to Gemini or Claude
          +-- future provider adapters
```

Sub2API should keep provider authentication and routing behind the gateway. A
client should not need separate provider credentials or provider-specific
account knowledge.

## Start Here

- [Architecture](docs/ARCHITECTURE.md)
- [Development](docs/DEVELOPMENT.md)
- [Roadmap](docs/ROADMAP.md)
- [Upstream sync policy](docs/UPSTREAM_SYNC.md)
- [Codex compatibility](docs/CODEX_COMPATIBILITY.md)
- [Codex standalone search](docs/CODEX_STANDALONE_SEARCH.md)
- [UI rework plan](docs/UI_REWORK.md)
- [Upstream deployment documentation](deploy/README.md)

The inherited deployment scripts and configuration remain available, but they
have not all been qualified as rework releases. Review generated configuration
and secrets before exposing an instance to a network.

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md). New and substantially reworked public
material must be written in English. Do not rename protocol fields, serialized
fields, provider identifiers, or compatibility names for translation alone.

## Upstream and License

This project preserves the history of
[Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) and tracks it through the
`upstream` Git remote. The baseline and sync process are recorded in
[docs/UPSTREAM_SYNC.md](docs/UPSTREAM_SYNC.md).

The project remains licensed under the
[GNU Lesser General Public License v3.0 or later](LICENSE). The upstream README
identifies Copyright (c) 2026 Wesley Liddick. No endorsement by the upstream
maintainers is implied.
