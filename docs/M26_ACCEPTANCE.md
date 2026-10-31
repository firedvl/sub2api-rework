# Claude warm-up model fix .16 acceptance

`0.2.3-rework.16` deployed through updater 1.1.6 on October 30, 2026 at 11:45 PM Pacific (October 31, 06:45Z). Application, PostgreSQL and Redis are healthy, updater is idle and successful, and schema remains 252. [PR #315](https://github.com/firedvl/sub2api-rework/pull/315) merged at `95754616d0ce40ef03e51c0e87ab169b3848ccca`; its tree matches the qualified branch head. All five CI jobs and both security jobs passed, followed by all four [publisher jobs](https://github.com/firedvl/sub2api-rework/actions/runs/41176662841).

The upstream 404 was `not_found_error` for `claude-sonnet-4-5-20250929` on one OAuth account. The shared connection-test default now uses Sonnet 4.6. A real warm-up-path regression reproduced the 404 before the change and passed after it. Two bounded live requests, each capped at 16 output tokens, verified Sonnet 4.6 on both configured accounts. The affected account completed its stream and returned a new five-hour reset header; the other retained its active window. The gateway subsequently observed confirmed active windows for both. No credentials, group membership or opt-ins were changed.

Normal Messages routing already fails over between eligible Claude OAuth accounts. New handler tests verify quota-429 handoff with the same conversation, streaming only the successful response, and excluding an account already in cooldown. Focused race tests passed. The API-key-only endpoint stays isolated. The Claude Code guide documents the root gateway URL and a downstream key bound to `anthropic-default`; an answer already streaming cannot be invisibly transferred.

| Identity | Verified value |
| --- | --- |
| Revision | `95754616d0ce40ef03e51c0e87ab169b3848ccca` |
| Image digest | `sha256:47fd2c0ed6a43688278abaa8632d9a55409abe8a3ec9f9bc4c067393a63ca17f` |
| Manifest SHA-256 | `6328163a86610552c79c67fd3313d5e101ef2bb14f406def91231c6b242e29ca` |
| Prepare | `upd-01adc572905722b51f4fb037` |
| Install | `upd-822b9aa18460ecf2388336f5`, 06:45:16Z–06:45:41Z |
| Source snapshot | `20261031T064524Z-upd-822b9aa18460ecf2388336f5`, `.15` / schema 252 |

The canonical manifest passed the shared Go validator and matched the registry digest; host image labels, platform and actual binary version/revision match. All nine accounts, credentials, groups, protected configuration, billing and automation controls are preserved. Comparison with the quiesced source archive confirms credit controls unchanged. All 285,047 historical usage rows through ID 285611 retain SHA-256 `795147a25e5195626aa32c7225d9d09ef3156ce5246de8b7740c6dedc6557d57`. All 42 prior backups / 210 files retain their hashes. The new verified readable database/environment/Compose snapshot makes 43 backups; the source image remains available. No rollback or restore occurred.

Origin/public health, root, index and settings return 200; API-only unauthenticated Messages/count_tokens return JSON 401. Origin/public assets match and the account bundle contains the Claude controls. The initial post-install sample has 46 info records and no error, fatal or panic records. Both account opt-ins remain enabled and their active countdowns remain confirmed. Production was not deliberately exhausted to force a live account switch; that behavior was verified with the real handler/service/scheduler against synthetic upstream responses. No authenticated production UI interaction was performed. Raw receipts remain under ignored `.codex/claude-warmup-failover-fix/`.
