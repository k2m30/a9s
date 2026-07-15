import { test, expect, Page } from "@playwright/test";
import { readServer } from "./server";

// Web parity of the presentation doctrine (task #53): every list has one
// "Status" column rendering finding phrases (else the humanized state), row
// colors derive from findings, and the detail shows an Attention section
// listing causes. The TUI and unit suites already pin this; the web lane
// renders the same controller ViewState (core/app) through
// core/web/templates, so this spec asserts the same doctrine against the
// REAL browser DOM in --demo mode.
//
// DOM mechanism (inspected, not assumed): the list template
// (core/web/templates/list.html) stamps each <tr> with the shared
// ListRow.Color tag as a `row-<tag>` class — row-healthy / row-warning /
// row-broken / row-dim — which page.html's CSS maps to var(--ok)/var(--warn)/
// var(--err)/var(--dim). "Issue row color" therefore means class
// row-warning or row-broken on the <tr>.
const server = readServer();

// Every assertion below names demo-fixture witnesses (a9s-demo-nopab,
// old-orphan-vol, acme-web-tg, …), so the file is meaningless against live
// AWS data.
test.skip(server.live, "demo-only spec — skipped in live mode (A9S_E2E_PROFILE set)");

const ISSUE_ROW = /row-(warning|broken)/;

// press dispatches a real keystroke and waits for the resulting POST /action
// round-trip, so #main has been swapped before the next assertion (same
// helper as web-ui.spec.ts).
async function press(page: Page, key: string): Promise<void> {
  await Promise.all([
    page.waitForResponse(
      (r) => r.url().includes("/action") && r.request().method() === "POST",
      { timeout: 10_000 },
    ),
    page.keyboard.press(key),
  ]);
}

// command drives the ':' palette to a resource list (":s3" etc.) — the ':'
// bar is client-side (no POST) until Enter dispatches the command action.
async function command(page: Page, cmd: string): Promise<void> {
  await page.keyboard.press(":");
  await page.keyboard.type(cmd);
  await press(page, "Enter");
  await expect(page.locator(".list-table")).toBeVisible();
}

// statusColIndex resolves the "Status" column position from the rendered
// header row, so the assertions don't hardcode per-type column layouts.
async function statusColIndex(page: Page): Promise<number> {
  const titles = await page.locator(".list-table thead th").allInnerTexts();
  const idx = titles.findIndex((t) => t.trim() === "Status");
  expect(idx, `every list must render a Status column, got headers: ${titles.join(", ")}`).toBeGreaterThanOrEqual(0);
  return idx;
}

// openFlaggedS3Detail filters the s3 list down to a9s-demo-nopab and opens
// its detail. Filter letters each POST (live filtering); the closing Enter
// commits the filter client-side (no POST), then 'd' opens the detail.
async function openFlaggedS3Detail(page: Page): Promise<void> {
  await command(page, "s3");
  await page.keyboard.press("/");
  await expect(page.locator("#input-bar")).toBeVisible();
  for (const ch of "nopab") {
    await press(page, ch);
  }
  await page.keyboard.press("Enter");
  await expect(page.locator("#input-bar")).toBeHidden();
  await expect(page.locator(".list-table tbody tr")).toHaveCount(1);
  await press(page, "d");
  await expect(page.locator(".detail-layout")).toBeVisible();
  await expect(page.locator("#frame-title")).toHaveText("a9s-demo-nopab");
}

// openTargetHealthChildView walks tg list → acme-web-tg detail → tg_health
// child view.
//
// KNOWN PARITY GAP (reachability): the tg_health child view is registered
// under trigger key "enter" (core/aws/catalog_networking.go), and the web
// footer advertises "enter Target Health" — but app.js maps Enter to the
// generic "select" action (which always opens the detail) and its child-view
// key set only covers e/L/s. So no keyboard key reaches an "enter"-registered
// child view in the web (same for s3 "enter S3 Objects" and lambda "enter
// Lambda Invocations"). The controller handles the action fine, so this spec
// dispatches it through window.sendAction — the exact client function every
// mapped key handler calls — keeping the server-render assertions honest.
async function openTargetHealthChildView(page: Page): Promise<void> {
  await command(page, "tg");
  await press(page, "d"); // cursor starts on row 0 = acme-web-tg
  await expect(page.locator(".detail-layout")).toBeVisible();
  await expect(page.locator("#frame-title")).toHaveText("acme-web-tg");

  await Promise.all([
    page.waitForResponse(
      (r) => r.url().includes("/action") && r.request().method() === "POST",
      { timeout: 10_000 },
    ),
    page.evaluate(() =>
      (window as unknown as { sendAction: (kind: string, arg: string) => void }).sendAction(
        "child-view",
        "enter",
      ),
    ),
  ]);
  await expect(page.locator(".list-table")).toBeVisible();
  // acme-web-tg has 3 registered targets (2 healthy + 1 unhealthy); the count
  // assertion auto-retries across the async child fetch + SSE body reload.
  await expect(page.locator(".list-table tbody tr")).toHaveCount(3, { timeout: 10_000 });
}

test.beforeEach(async ({ page }) => {
  await page.goto(server.url);
  await expect(page.locator("#main")).toBeVisible();
  await expect(page.locator(".menu-entry").first()).toBeVisible();
});

test.describe("presentation doctrine — web parity (demo fixtures)", () => {
  test("s3 list: flagged buckets show the finding phrase in Status and an issue row color", async ({ page }) => {
    await command(page, "s3");
    const statusCol = await statusColIndex(page);

    for (const bucket of ["a9s-demo-nopab", "a9s-demo-partial-pab"]) {
      const row = page.locator(".list-table tbody tr", { hasText: bucket });
      await expect(row, `flagged bucket ${bucket} must be listed`).toHaveCount(1);
      await expect(
        row.locator("td").nth(statusCol),
        `${bucket} Status cell must render the finding phrase, not a raw code or blank`,
      ).toHaveText("public access block incomplete");
      await expect(
        row,
        `${bucket} row must carry the findings-derived issue color class (row-broken — SevBroken finding)`,
      ).toHaveClass(/row-broken/);
    }

    // Doctrine contrast: a healthy bucket keeps the default color and a blank
    // Status cell — proves the issue color is finding-derived, not global.
    const healthy = page.locator(".list-table tbody tr", { hasText: "a9s-demo-healthy" });
    await expect(healthy).toHaveCount(1);
    await expect(healthy).not.toHaveClass(ISSUE_ROW);
    await expect(healthy.locator("td").nth(statusCol)).toHaveText("");
  });

  test("ebs list: unencrypted and orphan witnesses show their phrases with issue colors", async ({ page }) => {
    await command(page, "ebs");
    const statusCol = await statusColIndex(page);

    const unencrypted = page.locator(".list-table tbody tr", { hasText: "legacy-unencrypted-vol" });
    await expect(unencrypted).toHaveCount(1);
    await expect(
      unencrypted.locator("td").nth(statusCol),
      "unencrypted witness must render the wave-1 finding phrase in Status",
    ).toHaveText("unencrypted");
    await expect(unencrypted).toHaveClass(/row-warning/);

    const orphan = page.locator(".list-table tbody tr", { hasText: "old-orphan-vol" });
    await expect(orphan).toHaveCount(1);
    // The day count is computed from the fixture CreateTime at runtime, so it
    // drifts with the wall clock — pin the shape, not the number.
    await expect(
      orphan.locator("td").nth(statusCol),
      "orphan witness must render the orphan phrase in Status",
    ).toHaveText(/^orphan: unattached \d+d$/);
    await expect(orphan).toHaveClass(/row-warning/);
  });

  test("s3 flagged bucket detail: Attention section present with the cause sentence", async ({ page }) => {
    await openFlaggedS3Detail(page);

    // The shared detail body builder (core/app/detail_body.go's
    // injectAttentionSectionDetail) renders Attention as a field SECTION at
    // the top of .detail-fields — the same layout the TUI shows — not via the
    // separate .attention-block template branch.
    await expect(
      page.locator(".field-row.section", { hasText: "Attention" }),
      "flagged bucket detail must open with an Attention section",
    ).toBeVisible();

    const detail = page.locator(".detail-main");
    await expect(detail).toContainText("public access block incomplete");
    await expect(
      detail,
      "the Attention block must carry the operator cause sentence, not just the phrase",
    ).toContainText("Bucket-level public access block is missing or partial");
  });

  test("tg detail → target health child view: humanized phrase, no raw reason enum", async ({ page }) => {
    // tg list level first: the parent row already speaks the doctrine.
    await command(page, "tg");
    const statusCol = await statusColIndex(page);
    const tgRow = page.locator(".list-table tbody tr", { hasText: "acme-web-tg" });
    await expect(tgRow.locator("td").nth(statusCol)).toHaveText("unhealthy targets: 1/3");
    await expect(tgRow).toHaveClass(/row-broken/);

    await openTargetHealthChildView(page);

    const unhealthy = page.locator(".list-table tbody tr", { hasText: "i-0a1b2c3d4e5f60003" });
    await expect(unhealthy).toHaveCount(1);
    await expect(
      unhealthy,
      "the flagged target row must render the finding phrase, not the raw SDK enum",
    ).toContainText("failed health checks");

    // No raw dotted enum token anywhere in the rendered page.
    const pageText = await page.locator("body").innerText();
    expect(
      pageText.includes("Target.FailedHealthChecks"),
      "the raw 'Target.FailedHealthChecks' enum must never reach the rendered page",
    ).toBe(false);
  });

  test("unhealthy target row in the child view carries an issue row color", async ({ page }) => {
    // Web child lists share the findings-derived row color with the TUI:
    // handleActionChildView registers the child typeDef as the fallback, so
    // buildListBody resolves the same color tag both lanes render.
    await openTargetHealthChildView(page);
    const unhealthy = page.locator(".list-table tbody tr", { hasText: "i-0a1b2c3d4e5f60003" });
    await expect(unhealthy).toHaveCount(1);
    await expect(
      unhealthy,
      "unhealthy target row must carry the findings-derived issue color class, like every top-level list",
    ).toHaveClass(ISSUE_ROW, { timeout: 3_000 });
  });

  test("no raw UPPER_SNAKE token in any rendered list cell (s3/ebs/lambda/sg)", async ({ page }) => {
    // The doctrine humanizes every status enum before it reaches a list cell;
    // an UPPER_SNAKE survivor (e.g. PENDING_DELETION, ACTIVE_IMPAIRED) means
    // a raw AWS enum leaked past the humanization layer.
    const rawToken = /\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b/g;
    for (const shortName of ["s3", "ebs", "lambda", "sg"]) {
      await command(page, shortName);
      const cells = await page.locator(".list-table td").allInnerTexts();
      const leaked = cells.flatMap((c) => c.match(rawToken) ?? []);
      expect(
        leaked,
        `raw UPPER_SNAKE token(s) leaked into the ${shortName} list cells`,
      ).toEqual([]);
    }
  });
});
