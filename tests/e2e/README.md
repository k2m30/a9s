# Web UI end-to-end tests (Playwright)

Real-browser tests for the `--web` UI. A headless Chromium drives **actual
keystrokes** through the page's `app.js` → `POST /action` → `#main` swap, then
asserts navigation and rendering.

## Why this exists

The web UI is a Go server + vanilla `app.js`. Go/`curl` integration tests
exercise the controller and HTTP endpoints but **never execute the page's
JavaScript**, so they cannot catch:

- a static asset 404 (`/static/app.js`) → no JS loads → every key dead;
- stale chrome (frame-title not updating on navigation);
- DOM duplication (sub-fields rendered twice);
- a key that silently no-ops in the browser.

These specs press real keys in a real browser, so they catch all of the above.
Each `test` maps to a regression that previously shipped.

## Run (demo mode — default)

One-time install (downloads `@playwright/test` + the Chromium binary):

```sh
make e2e-install      # or: cd tests/e2e && npm ci && npx playwright install chromium
```

Run the suite (builds the binary, boots `a9s --demo --web`, runs the browser):

```sh
make e2e              # or: cd tests/e2e && npx playwright test
```

`global-setup.ts` builds `./a9s` and boots the demo web server on
`127.0.0.1:7799` (override with `A9S_E2E_ADDR`), parsing the per-run token from
stderr. `A9S_E2E_SKIP_BUILD=1` skips the rebuild if you already built.

## Run (live read-only AWS mode)

Point the suite at a real AWS profile to catch live-data bugs that synthetic
fixtures cannot exercise (e.g. related-panel `-1` sentinels, dead-end rows with
no resources):

```sh
A9S_E2E_PROFILE=my-readonly-profile make e2e
# with an explicit region:
A9S_E2E_PROFILE=my-readonly-profile A9S_E2E_REGION=eu-west-1 make e2e
```

Or use the dedicated target that documents the intent:

```sh
make e2e-live PROFILE=my-readonly-profile
make e2e-live PROFILE=my-readonly-profile REGION=eu-west-1
```

**Requirements:**

- Valid AWS credentials for the named profile (the profile must be in
  `~/.aws/config` / `~/.aws/credentials`).
- The profile must be read-only — a9s never makes write calls to AWS, but
  defence-in-depth is good practice.
- Node 18+ and the Chromium binary (run `make e2e-install` once).

**What runs:**

- `web-ui.spec.ts` — the demo spec **skips** in live mode (it asserts on
  specific fixture data that doesn't exist in a real account).
- `live-readonly.spec.ts` — **runs** in live mode only; skipped automatically
  when `A9S_E2E_PROFILE` is not set.

**Live checks performed:**

1. Menu renders and at least one resource type reports a non-zero count.
2. Related-panel sentinel bug: no `.related-count` element renders the raw
   value `-1` (the unknown sentinel must display as a dimmed indicator, not
   the raw integer).
3. Dead-end rows: zero/unknown related rows carry a dim/disabled CSS class.
4. Navigation bug: clicking a related row with count > 0 changes the page body
   and does not cause `#main` to disappear (the toggle-hide regression).

All assertions are data-agnostic — they do not hardcode resource names or
counts, so they work against any AWS account.

## Reports

```sh
npx playwright show-report   # after any run
```

Traces, screenshots, and video are captured on failure under `test-results/`.

## Notes

- `workers: 1` — the server keeps one per-session controller, so specs run
  serially against a shared server.
- Node 18+ required. `node_modules/`, `.runtime.json`, and reports are
  git-ignored; `package.json` + `package-lock.json` are committed for `npm ci`.
- Live fetches are slower than demo — `global-setup.ts` uses a 60 s URL-wait
  timeout in live mode (vs 20 s for demo).
