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

## Run

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
stdout. `A9S_E2E_SKIP_BUILD=1` skips the rebuild if you already built.

Reports: `npx playwright show-report` after a run. Traces/screenshots/video are
captured on failure under `test-results/`.

## Notes

- `workers: 1` — the server keeps one per-session controller, so specs run
  serially against a shared server.
- Node 18+ required. `node_modules/`, `.runtime.json`, and reports are
  git-ignored; `package.json` + `package-lock.json` are committed for `npm ci`.
