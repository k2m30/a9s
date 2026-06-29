import { test, expect, Page } from "@playwright/test";
import { readServer } from "./server";

// Booted + tokenized by global-setup. The tokenized URL establishes the session.
const server = readServer();

// This entire suite is skipped when not running in live mode.
// Set A9S_E2E_PROFILE to a read-only AWS profile name to enable it.
test.skip(!server.live, "set A9S_E2E_PROFILE to a read-only AWS profile to run live tests");

// Live operations (connect, per-type availability fetches, list + related-check
// fetches) are far slower than demo, and span multiple round-trips — so the
// default 30s per-test timeout (playwright.config.ts) is too tight. Give the
// live suite a generous per-test budget; the internal poll/waitFor timeouts
// still bound each individual step.
test.describe.configure({ timeout: 240_000 });

// press dispatches a real keystroke and waits for the resulting POST /action to
// return, so the #main fragment has been swapped before the next assertion.
async function press(page: Page, key: string, timeout = 15_000): Promise<void> {
  await Promise.all([
    page.waitForResponse(
      (r) => r.url().includes("/action") && r.request().method() === "POST",
      { timeout },
    ),
    page.keyboard.press(key),
  ]);
}

// waitForMenuEntries waits up to `timeoutMs` for at least one .menu-entry to
// appear, then polls for availability counts on the entries.  Live fetches are
// slow, so the timeout is generous.
async function waitForMenuEntries(page: Page, timeoutMs = 90_000): Promise<void> {
  await expect(page.locator(".menu-entry").first()).toBeVisible({ timeout: timeoutMs });
}

// findNavigableMenuEntry scans the visible .menu-entry elements and returns the
// index of the first one whose text includes a non-zero parenthesised count
// (e.g. "EC2 Instances (12)"), or -1 if none found.  We prefer a row that has
// real resources so the related panel has something to show.
async function findNavigableMenuEntry(page: Page): Promise<number> {
  const entries = page.locator(".menu-entry");
  const count = await entries.count();
  for (let i = 0; i < count; i++) {
    const text = await entries.nth(i).textContent();
    // Match "(N)" where N > 0 — availability count present and non-zero.
    if (text && /\(\s*[1-9]\d*\s*\)/.test(text)) {
      return i;
    }
  }
  return -1;
}

// preferredTypes lists shortNames we'd like to try first, in order of
// likelihood to have both rows AND a populated related panel.
const preferredTypes = ["ec2", "vol", "rds", "lambda", "ecs"];

test.describe("a9s web UI — live read-only AWS data structural checks", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto(server.url);
    await expect(page.locator("#main")).toBeVisible();
    // Wait generously — the first render after a live boot may be slow.
    await waitForMenuEntries(page, 90_000);
  });

  test("menu renders and at least one resource type reports a non-zero count", async ({ page }) => {
    // By the time beforeEach finishes we already have menu entries visible.
    // Now wait for at least one entry to carry an availability count badge.
    // Live fetches may still be in-flight, so poll for up to 2 minutes.
    await expect
      .poll(
        async () => {
          const entries = page.locator(".menu-entry");
          const count = await entries.count();
          if (count === 0) return false;
          for (let i = 0; i < count; i++) {
            const text = await entries.nth(i).textContent();
            if (text && /\(\s*[1-9]\d*\s*\)/.test(text)) return true;
          }
          return false;
        },
        {
          message: "expected at least one menu entry with a non-zero resource count",
          timeout: 120_000,
          intervals: [2_000, 5_000, 10_000],
        },
      )
      .toBe(true);
  });

  test("related-panel sentinel and dead-end bugs are not present on live data", async ({ page }) => {
    // The menu is visible immediately, but availability counts arrive
    // asynchronously over the SSE stream. Wait until at least one type has a
    // non-zero count before scanning — otherwise we'd skip before live data
    // loads (this is a fresh session per test, so it re-fetches availability).
    await expect
      .poll(async () => (await findNavigableMenuEntry(page)) >= 0, {
        message: "expected at least one navigable (count>0) menu entry before navigating",
        timeout: 120_000,
        intervals: [2_000, 5_000, 10_000],
      })
      .toBe(true);

    // Step 1: Pick a resource type that has rows.  Try preferred types first by
    // matching menu text, then fall back to the first entry with any count.
    const entries = page.locator(".menu-entry");
    const entryCount = await entries.count();
    const entryTexts: string[] = [];
    for (let i = 0; i < entryCount; i++) {
      entryTexts.push((await entries.nth(i).textContent()) ?? "");
    }

    let targetIdx = -1;
    for (const preferred of preferredTypes) {
      const idx = entryTexts.findIndex((t) =>
        t.toLowerCase().includes(preferred),
      );
      if (idx >= 0 && /\(\s*[1-9]\d*\s*\)/.test(entryTexts[idx])) {
        targetIdx = idx;
        break;
      }
    }
    if (targetIdx < 0) {
      targetIdx = await findNavigableMenuEntry(page);
    }
    if (targetIdx < 0) {
      test.skip(true, "no menu entry with a non-zero count found — skipping related-panel checks");
      return;
    }

    console.log(`live test: navigating to menu entry ${targetIdx}: "${entryTexts[targetIdx]}"`);

    // Step 2: Move cursor to the chosen entry and open the list.
    for (let i = 0; i < targetIdx; i++) {
      await press(page, "ArrowDown");
    }
    await press(page, "Enter");
    await expect(page.locator(".list-table")).toBeVisible({ timeout: 30_000 });

    // Step 3: Open the first row's detail view.
    await press(page, "d", 15_000);
    await expect(page.locator(".detail-layout")).toBeVisible({ timeout: 15_000 });

    // Wait for the related panel to appear — it loads asynchronously on live.
    const relatedPanel = page.locator(".related-panel");
    const panelVisible = await relatedPanel
      .waitFor({ state: "visible", timeout: 30_000 })
      .then(() => true)
      .catch(() => false);

    if (!panelVisible) {
      console.log("live test: no related-panel on this resource type — skipping related checks");
      return;
    }

    // -----------------------------------------------------------------------
    // Bug 1: No related-count must render the raw sentinel value "-1".
    // The unknown sentinel must be displayed as a dimmed/empty indicator.
    // -----------------------------------------------------------------------
    const counts = page.locator(".related-count");
    const countTexts = await counts.allTextContents();
    for (const txt of countTexts) {
      // toContain, not toBe: catches both the bare "-1" and the "(-1)" the
      // actionable branch used to leak before the count was pre-formatted by
      // resource.FormatRelatedCount (which renders -1 as an empty badge).
      expect(
        txt.trim(),
        `related-count must not render the raw "-1" sentinel — got "${txt.trim()}"`,
      ).not.toContain("-1");
    }
    console.log(`live test: related-count values: ${JSON.stringify(countTexts)}`);

    // -----------------------------------------------------------------------
    // Bug 2: every resolved row is EITHER actionable-and-clickable OR a marked
    // dead-end — never both, never neither. (A zero/-1 row is NOT automatically a
    // dead-end: a FetchFilter or Approximate row resolves the real count on
    // drill-in and stays clickable, per resource.IsRelatedActionable. A true
    // dead-end — zero with no FetchFilter, not approximate — must be dimmed and
    // non-navigable.)
    // -----------------------------------------------------------------------
    const allRows = page.locator(".related-row");
    const rowCount = await allRows.count();

    for (let i = 0; i < rowCount; i++) {
      const row = allRows.nth(i);
      const rowText = (await row.textContent()) ?? "";
      if (rowText.includes("…")) continue; // skip still-loading rows

      const cls = (await row.getAttribute("class")) ?? "";
      const onclick = (await row.getAttribute("onclick")) ?? "";
      const isDeadEnd = cls.includes("dead-end");
      const isClickable = onclick.includes("clickRelated");

      expect(
        isClickable !== isDeadEnd,
        `related row "${rowText.trim()}" must be either clickable (actionable) or a marked dead-end, not both/neither (class="${cls}", onclick="${onclick}")`,
      ).toBe(true);
      if (isDeadEnd) {
        expect(isClickable, `dead-end row "${rowText.trim()}" must not be clickable`).toBe(false);
      }
    }

    // -----------------------------------------------------------------------
    // Bug 3: Clicking a related row with count > 0 must navigate (body
    // changes) and the related panel must NOT simply disappear (toggle-hide).
    // -----------------------------------------------------------------------
    let navigableRowIdx = -1;
    for (let i = 0; i < rowCount; i++) {
      const countText = await allRows.nth(i).locator(".related-count").textContent();
      if (countText && /\(\s*[1-9]\d*\s*\)/.test(countText)) {
        navigableRowIdx = i;
        break;
      }
    }

    if (navigableRowIdx < 0) {
      console.log(
        "live test: no related row with count > 0 on this resource — skipping navigation check",
      );
      return;
    }

    const rowLabel = await allRows.nth(navigableRowIdx).textContent();
    console.log(`live test: clicking related row ${navigableRowIdx}: "${rowLabel?.trim()}"`);

    // Capture the body content before navigation.
    const bodyBefore = await page.locator("#body").innerHTML();

    // CLICK the related row directly to exercise the clickRelated() /
    // related-select onclick path. A keyboard Tab+Enter would pass through the
    // key handler even if the click path regressed, so it would not protect the
    // click regression this test describes.
    await Promise.all([
      page.waitForResponse(
        (r) => r.url().includes("/action") && r.request().method() === "POST",
        { timeout: 20_000 },
      ),
      allRows.nth(navigableRowIdx).click(),
    ]);

    // The body must have changed — we navigated somewhere.
    const bodyAfter = await page.locator("#body").innerHTML();
    expect(
      bodyAfter,
      "clicking a related row with count > 0 must change the page body (navigation must occur)",
    ).not.toBe(bodyBefore);

    // The main container must still be present — it must not have disappeared
    // (the toggle-hide bug caused #main to vanish entirely).
    await expect(page.locator("#main")).toBeVisible({
      timeout: 5_000,
    });

    console.log(
      `live test: navigation from related row succeeded; body changed from ${bodyBefore.length} to ${bodyAfter.length} bytes`,
    );
  });
});
