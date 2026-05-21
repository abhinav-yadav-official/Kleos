# Progress + Health Dashboard Design

## Goal

Make the current Kleos backend progress visible at `https://abhiyadav.in/kleos/` with a read-only dashboard that shows project phase status, live API health, verification evidence, and the remaining Gmail SMTP blocker.

## Scope

This slice builds a static, unauthenticated dashboard. It does not add login UI, onboarding forms, campaign workflows, or editable settings. The dashboard is an operator/status surface for the work already completed.

## User Experience

The first screen shows:
- Phase cards for Foundation, Auth, SMTP, Resume, and Preferences.
- Live health checks for `/kleos/api/healthz` and `/kleos/api/readyz`.
- A concise verification timeline from recent completed checkpoints.
- A blocker panel that says Gmail app-password SMTP verification is still required and lists the needed credential fields.

The page should feel like a compact operations dashboard: quiet, scannable, and work-focused. It should not look like a marketing landing page.

## Architecture

Add a static `web/` app built with plain HTML, CSS, and JavaScript. The deployed nginx config already serves `/kleos/` as static files and proxies `/kleos/api/` to the Go API, so this slice should fit the existing deployment shape without adding a Node runtime.

Dashboard data is split into:
- Static project status and verification summaries embedded in `web/assets/status.json`.
- Live API health fetched in the browser from `/kleos/api/healthz` and `/kleos/api/readyz`.

## Plan Cleanup

Reorganize the top of `plan.md` so future agents see the current state first:
- Current Status
- Verified Checkpoints
- Active Blockers
- Next Phase
- Existing detailed plan

Keep `checkpoints.txt` append-only. The dashboard may summarize checkpoint evidence, but `checkpoints.txt` remains the source of history.

## Deployment

Update CI/deploy only if needed to copy `web/` into `/opt/kleos/web/out` or equivalent static path consumed by the existing nginx snippet. Verify:
- `https://abhiyadav.in/kleos/` returns the dashboard.
- Browser health checks show both API endpoints healthy.
- `https://abhiyadav.in/kleos/api/healthz` still proxies to Go.

## Testing

Local checks:
- Static file syntax sanity where practical.
- No secrets in `web/`.
- `make test`, `make build`, and `make lint` still pass for Go.

Live checks:
- Fetch `/kleos/` and confirm dashboard HTML is served.
- Fetch `/kleos/api/healthz` and `/kleos/api/readyz`.
- Confirm the dashboard JS points to `/kleos/api/*`, not localhost.

## Deferred Scope

The dashboard is read-only in this slice. Authenticated onboarding UI can come next after this status surface is deployed.
