# Claude warm-up .15 production acceptance

`0.2.3-rework.15` deployed successfully on October 30, 2026 at 2:18 PM Pacific through host updater 1.1.6. Schema remains 252. The gateway, PostgreSQL and Redis are healthy; the updater is healthy and idle.

[PR #313](https://github.com/firedvl/sub2api-rework/pull/313) merged at `65ac5516e29dd2b5ed5e1f3938b3b1e02731c779`. Its tree matches the qualified branch head. All five CI jobs and both security jobs passed. The release also updates Vue I18n/compiler 11.4.14, decode-uri-component 0.5.1 and golang.org/x/net 0.62.0 with required companion modules to clear newly reported dependency findings. No audit exceptions were added. Frontend jobs use Node 22 for the supported Vue I18n engine range.

Local checks passed all 2,879 frontend tests, production build, TypeScript, focused backend race and PostgreSQL tests, six Claude browser flows and five-platform merged binary integrity. All four [publisher jobs](https://github.com/firedvl/sub2api-rework/actions/runs/41127287179) passed; the canonical manifest passed the shared Go contract and matched the registry digest.

| Identity | Verified value |
| --- | --- |
| Version/revision | `0.2.3-rework.15` / `65ac5516e29dd2b5ed5e1f3938b3b1e02731c779` |
| Image digest | `sha256:728c389d0189fb1f7f3b74baca231ee1f0922e1517923a89e050a83e55712670` |
| Manifest SHA-256 | `5f4415b593939f1aaa7679634e06792b7e14cb18f4dce4fae686b26a5a35a1cd` |
| Prepare | `upd-8544478e4ebd92efb96a0065` |
| Install | `upd-5513ee8f122db9d70abae392`, 21:17:46Z–21:18:12Z |
| Source snapshot | `20261030T211754Z-upd-5513ee8f122db9d70abae392`, `.14` / schema 252 |

All nine accounts, credentials, groups/assignments, protected configuration and billing/automation controls match the fresh pre-install baseline. Comparison with the quiesced source archive confirms credit, billing and automation fields are unchanged; quota reset observations refreshed normally. All 283,871 historical usage rows through ID 284435 retain SHA-256 `7dc37769d8836091af9fa94dee1ef160a8daf8541e6f85a71453790b97b86135`. All 41 prior backup directories / 205 files retain their hashes. The new verified database/environment/Compose snapshot makes 42 backups; its archive is readable and the source image is retained. No database restore occurred.

Origin and public health, root, index and public settings return 200. Unauthenticated API-only Messages/count_tokens return JSON 401. Public/origin entry assets match, root/index use no-cache, and public responses report Cloudflare DYNAMIC without Age. The public account chunk contains Claude warm-up state, evaluation and confirmed-reset controls. The application binary reports the exact version/revision and has no Docker socket. The initial post-install log sample has 34 info records and no error, fatal or panic records.

The two Claude OAuth accounts retain their disabled warm-up settings. Deployment does not enable accounts or initiate a live inference canary. Countdown behavior was verified using synthetic upstream responses and browser fixtures, including successive reset cycles and a decreasing display. No authenticated production UI session or live provider reset cycle was exercised. Raw receipts remain under ignored `.codex/claude-warmup-release15/`.

The recorded application-only rollback source is `.14`, digest `sha256:9993b8a04aa4c37a15bb61d579241dc8c4270493cdfa4b2d26160229856b1b46`, subject to the updater’s identity, state, schema and backup checks. No rollback or recovery was performed.
