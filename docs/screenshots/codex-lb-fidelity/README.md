# codex-lb fidelity comparison

Reference source: [`Soju06/codex-lb`](https://github.com/Soju06/codex-lb) at `cedc05f95a46fe062eaf8ff35eceea1a91735a2a`.

The reference images were generated from codex-lb's own synthetic Playwright fixtures. The Sub2API images were generated from `pnpm review:ui`, which serves localhost-only synthetic data and blocks normal writes. Desktop captures use a `1440x900` viewport. Settings uses a full-page capture. The Activity reference scrolls codex-lb's fixture-backed Request Logs section into the viewport.

`frontend/e2e/operator-navigation.spec.ts` repeats the six Sub2API operator areas at `1440x900`, `1280x800`, `1024x768`, and `768x900` and checks document overflow. The same suite covers the narrow navigation drawer, Settings scrolling, and dialog focus behavior.

## Login

| codex-lb reference | Sub2API fidelity port |
| --- | --- |
| ![codex-lb login](reference/login.jpg) | ![Sub2API login](sub2api/login.png) |

## Overview

| codex-lb reference | Sub2API fidelity port |
| --- | --- |
| ![codex-lb dashboard](reference/overview.jpg) | ![Sub2API overview](sub2api/overview.png) |

## Accounts

| codex-lb reference | Sub2API fidelity port |
| --- | --- |
| ![codex-lb accounts](reference/accounts.jpg) | ![Sub2API accounts](sub2api/accounts.png) |

## Activity

| codex-lb reference | Sub2API fidelity port |
| --- | --- |
| ![codex-lb request logs](reference/activity.jpg) | ![Sub2API activity](sub2api/activity.png) |

## Settings

| codex-lb reference | Sub2API fidelity port |
| --- | --- |
| ![codex-lb settings](reference/settings.jpg) | ![Sub2API settings](sub2api/settings.png) |

## Stats and operator surface review

The review pass uses the same localhost-only synthetic data. Full Stats captures cover the requested widths; focused captures show capacity charts and the secondary operator states changed in this pass.

### Stats

| 1440 | 1280 | 1024 | 768 |
| --- | --- | --- | --- |
| ![Stats at 1440px](sub2api/review-pass/stats-1440.png) | ![Stats at 1280px](sub2api/review-pass/stats-1280.png) | ![Stats at 1024px](sub2api/review-pass/stats-1024.png) | ![Stats at 768px](sub2api/review-pass/stats-768.png) |

| Global pool | OpenAI provider | Single-account provider | Unknown-capacity provider |
| --- | --- | --- | --- |
| ![Global capacity pool](sub2api/review-pass/stats-global-pool.png) | ![OpenAI provider capacity](sub2api/review-pass/stats-openai-provider.png) | ![Single-account provider capacity](sub2api/review-pass/stats-single-account-provider.png) | ![Unknown-capacity provider](sub2api/review-pass/stats-unknown-provider.png) |

### Dialogs and menus

| Edit account | Create account | Bulk edit | Account menu |
| --- | --- | --- | --- |
| ![Edit account dialog](sub2api/review-pass/account-edit-dialog.png) | ![Create account dialog](sub2api/review-pass/account-create-dialog.png) | ![Bulk edit dialog](sub2api/review-pass/account-bulk-edit-dialog.png) | ![Account More menu](sub2api/review-pass/account-more-menu.png) |

| More Actions | Settings navigation | Activity Select | Models and Routing Select |
| --- | --- | --- | --- |
| ![Accounts More Actions menu](sub2api/review-pass/accounts-more-actions-menu.png) | ![Settings navigation](sub2api/review-pass/settings-navigation.png) | ![Activity open Select](sub2api/review-pass/activity-open-dropdown.png) | ![Models and Routing open Select](sub2api/review-pass/models-routing-open-dropdown.png) |
