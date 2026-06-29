import { test, expect, Page } from "@playwright/test";
import { readServer } from "./server";

// Booted + tokenized by global-setup. The tokenized URL establishes the session.
const server = readServer();

// This spec asserts demo-fixture specifics (web-prod-01, acme-web-tg, ec2(27)…),
// so it is meaningless against live AWS data. Skip the whole file in live mode;
// the data-agnostic live checks live in live-readonly.spec.ts.
test.skip(server.live, "demo-only spec — skipped in live mode (A9S_E2E_PROFILE set)");

// press dispatches a real keystroke to the page and waits for the resulting
// POST /action to return, so the #main fragment has been swapped before the
// next assertion. Every mapped key in app.js calls sendAction -> POST /action.
async function press(page: Page, key: string): Promise<void> {
  await Promise.all([
    page.waitForResponse(
      (r) => r.url().includes("/action") && r.request().method() === "POST",
      { timeout: 10_000 },
    ),
    page.keyboard.press(key),
  ]);
}

test.beforeEach(async ({ page }) => {
  await page.goto(server.url);
  await expect(page.locator("#main")).toBeVisible();
  await expect(page.locator(".menu-entry").first()).toBeVisible();
});

test.describe("a9s web UI — real-browser key navigation", () => {
  // The headline regression: a wrong embed fs.Sub made /static/app.js 404, so
  // the browser loaded NO JavaScript and every key was dead. curl/API tests
  // passed because they never execute the page <script>. This catches it.
  test("static app.js loads (no 404) and wires the keydown handler", async ({ page, request }) => {
    const res = await request.get(`${server.baseURL}/static/app.js`);
    expect(
      res.status(),
      "GET /static/app.js — a 404 means no JS loads and every key is dead",
    ).toBe(200);
    expect(await res.text()).toContain("sendAction");
    const wired = await page.evaluate(
      () => typeof (window as unknown as { sendAction?: unknown }).sendAction === "function",
    );
    expect(wired, "window.sendAction must be defined — proves app.js executed in the browser").toBe(true);
  });

  test("Enter on the menu navigates into a resource list", async ({ page }) => {
    await press(page, "Enter");
    await expect(page.locator(".list-table")).toBeVisible();
    await expect(page.locator(".list-table tbody tr").first()).toBeVisible();
  });

  test("pressing d opens a detail and the frame title updates (not stale)", async ({ page }) => {
    await press(page, "Enter");
    await expect(page.locator(".list-table")).toBeVisible();
    const listTitle = (await page.locator("#frame-title").textContent())?.trim();

    await press(page, "d");
    await expect(page.locator(".detail-layout")).toBeVisible();
    const detailTitle = (await page.locator("#frame-title").textContent())?.trim();

    expect(
      detailTitle,
      "frame-title must update on navigation, not stay on the prior screen",
    ).not.toBe(listTitle);
    expect(detailTitle).toBe("web-prod-01");
  });

  test("detail sub-fields render once, not duplicated", async ({ page }) => {
    await press(page, "Enter");
    await expect(page.locator(".list-table")).toBeVisible();
    await press(page, "d");
    await expect(page.locator(".detail-layout")).toBeVisible();

    const duplicated = await page.locator(".field-row.sub").evaluateAll((rows) =>
      rows.filter((r) => {
        const k = r.querySelector(".field-key");
        const v = r.querySelector(".field-value");
        return (
          !!k &&
          !!v &&
          k.textContent!.trim() === v.textContent!.trim() &&
          k.textContent!.trim() !== ""
        );
      }).length,
    );
    expect(
      duplicated,
      "label-less sub-fields (Key==Value) must render once, not in both key and value spans",
    ).toBe(0);
  });

  test("related-navigate into a single-target opens the cached detail", async ({ page }) => {
    await press(page, "Enter");
    await expect(page.locator(".list-table")).toBeVisible();
    await press(page, "d");
    await expect(page.locator(".detail-layout")).toBeVisible();
    await expect(page.locator(".related-panel")).toBeVisible();
    // The first related row is "Target Groups" (count 1) at cursor 0.
    await press(page, "Tab"); // focus the related panel
    await press(page, "Enter"); // navigate into the single-target row

    await expect(page.locator(".detail-layout")).toBeVisible();
    await expect(
      page.locator("#frame-title"),
      "single-target related-navigate must seed the detail from cache, not land on an empty placeholder titled 'detail'",
    ).toHaveText("acme-web-tg");
    expect(await page.locator(".detail-fields .field-row").count()).toBeGreaterThan(0);
  });

  test("Escape pops back through the stack to the menu", async ({ page }) => {
    await press(page, "Enter");
    await expect(page.locator(".list-table")).toBeVisible();
    await press(page, "d");
    await expect(page.locator(".detail-layout")).toBeVisible();

    await press(page, "Escape");
    await expect(page.locator(".list-table")).toBeVisible();
    await press(page, "Escape");
    await expect(page.locator(".menu-entry").first()).toBeVisible();
  });

  // Regression: ActionSelect on a resource list was a no-op, so Enter on a row
  // (and a row click) did nothing instead of opening the detail like the TUI.
  test("Enter on a list row opens the detail (was a no-op)", async ({ page }) => {
    await press(page, "Enter"); // menu -> list
    await expect(page.locator(".list-table")).toBeVisible();
    await press(page, "Enter"); // list ROW -> detail
    await expect(page.locator(".detail-layout")).toBeVisible();
    await expect(page.locator("#frame-title")).toHaveText("web-prod-01");
  });

  test("clicking a list row opens the detail", async ({ page }) => {
    await press(page, "Enter"); // menu -> list
    await expect(page.locator(".list-table tbody tr").first()).toBeVisible();
    await page.locator(".list-table tbody tr").first().click(); // clickSelect -> select -> detail
    await expect(page.locator(".detail-layout")).toBeVisible();
  });

  // Regression: snapshot() set Body.Kind for help/identity but never the body
  // data, so ? and i swapped to a blank pane.
  test("? opens the help screen with real keybindings", async ({ page }) => {
    await press(page, "?");
    await expect(page.locator(".help-hint").first()).toBeVisible();
    await expect(page.getByText("up/down", { exact: false }).first()).toBeVisible();
  });

  test("i opens the identity screen populated with the caller identity", async ({ page }) => {
    await press(page, "i");
    await expect(page.locator(".identity-table")).toBeVisible();
    await expect(page.getByText("Account ID", { exact: false })).toBeVisible();
  });

  // Regression: navigable detail fields (the underlined links — ImageId, VpcId,
  // SubnetId, …) had no onclick, so clicking them did nothing ("navigation in
  // the detail view doesn't work").
  test("clicking a navigable detail field navigates to that resource", async ({ page }) => {
    await press(page, "Enter"); // menu -> ec2 list
    await expect(page.locator(".list-table")).toBeVisible();
    await press(page, "d"); // -> web-prod-01 detail
    await expect(page.locator(".detail-layout")).toBeVisible();
    await expect(page.locator("#frame-title")).toHaveText("web-prod-01");

    const navField = page.locator(".field-navigable").first();
    await expect(navField).toBeVisible();
    await navField.click(); // clickField -> navigate to the field's target

    await expect(
      page.locator("#frame-title"),
      "clicking a navigable field must navigate away from the web-prod-01 detail",
    ).not.toHaveText("web-prod-01");
    await expect(page.locator(".detail-layout, .list-table")).toBeVisible();
  });
});

test.describe("a9s web UI — menu fidelity + interaction (TUI parity)", () => {
  test("menu renders category section headers (COMPUTE, NETWORKING, …)", async ({ page }) => {
    // Bug: the web menu was a flat list with no category grouping, while the TUI
    // groups resource types under COMPUTE / NETWORKING / DATABASES & STORAGE / ….
    // MenuEntry.Category was already in the ViewState; the template never used it.
    const sections = page.locator(".menu-section");
    await expect(sections.first()).toBeVisible();
    const labels = (await sections.allTextContents()).map((s) => s.trim());
    expect(labels, "menu must group under COMPUTE").toContain("COMPUTE");
    expect(labels, "menu must group under NETWORKING").toContain("NETWORKING");
    expect(labels.length, "expected the full TUI category set").toBeGreaterThanOrEqual(8);
  });

  test("a keypress does not synchronously flash the loading indicator", async ({ page }) => {
    // Bug: sendAction() flipped #loading-indicator to display:block on every
    // keydown, so every key visibly blinked "loading". Fixed by delaying the
    // reveal ~180ms so sub-frame round-trips never show it.
    const disp = await page.evaluate(() => {
      document.dispatchEvent(new KeyboardEvent("keydown", { key: "j", bubbles: true }));
      return getComputedStyle(document.getElementById("loading-indicator")!).display;
    });
    expect(disp, "loading indicator must not flip visible synchronously on a keypress").toBe("none");
  });

  test("vim j/k move the menu selection", async ({ page }) => {
    const selName = async () => (await page.locator(".menu-entry.selected .name").textContent())?.trim();
    const first = await selName();
    await press(page, "j");
    expect(await selName(), "j must move selection down").not.toBe(first);
    await press(page, "k");
    expect(await selName(), "k must move selection back up").toBe(first);
  });

  test("vim h/l are mapped (horizontal scroll) inside a list", async ({ page }) => {
    // Bug: only ArrowLeft/ArrowRight scrolled; h/l were unmapped so they did
    // nothing. press() waits for POST /action — pre-fix it would time out
    // because no action was ever sent for h/l.
    await press(page, "Enter");
    await expect(page.locator(".list-table")).toBeVisible();
    await press(page, "l");
    await press(page, "h");
  });

  test("double-clicking a menu row navigates exactly one level (not two)", async ({ page }) => {
    // Bug: clickSelect chained move-top → N×move-down → select; a double-click
    // fired it twice and the 2nd run executed on the screen the 1st had already
    // opened, drilling a level deeper ("double click leads to two screens away").
    await page.locator(".menu-entry").first().dblclick();
    await expect(page.locator(".list-table")).toBeVisible({ timeout: 10_000 });
    await page.waitForTimeout(700); // allow any (buggy) 2nd-click chain to land
    await expect(
      page.locator(".detail-layout"),
      "a double-click must not drill past the list into a detail",
    ).toHaveCount(0);
    await expect(page.locator(".list-table")).toBeVisible();
  });

  test("the menu shows footer key hints (ctrl+z / ctrl+r)", async ({ page }) => {
    // Bug: ViewState.Footer was never populated by the controller, so the web
    // footer was empty while the TUI shows 'ctrl+z Issues only' / 'ctrl+r Refresh'.
    const footer = page.locator("#footer");
    await expect(footer).toContainText("Issues only");
    await expect(footer).toContainText("Refresh");
    await expect(footer.locator(".key-hint")).toHaveCount(2);
  });

  test("ctrl+z toggles the issues-only filter from the menu", async ({ page }) => {
    // Bug: ctrl+z (the TUI's issues-only toggle) was unmapped in the web.
    const before = await page.locator(".menu-entry").count();
    await press(page, "Control+z");
    const after = await page.locator(".menu-entry").count();
    expect(after, "ctrl+z must filter to issues-only (fewer rows)").toBeLessThan(before);
    const allBadged = await page
      .locator(".menu-entry")
      .evaluateAll((rows) => rows.every((r) => !!r.querySelector(".badge")));
    expect(allBadged, "every remaining row must carry an issue badge").toBe(true);
  });

  test("detail field cursor moves with up/down and Enter navigates the focused field", async ({ page }) => {
    // Bug 6: the web detail had no visible left-column cursor (up/down did
    // nothing) and Enter on a focused navigable field was a no-op — only the
    // mouse could navigate a field.
    await press(page, "Enter"); // menu -> ec2 list
    await expect(page.locator(".list-table")).toBeVisible();
    await press(page, "d"); // -> web-prod-01 detail
    await expect(page.locator("#frame-title")).toHaveText("web-prod-01");

    await press(page, "ArrowDown");
    await expect(page.locator(".field-row.field-cursor")).toHaveCount(1);
    const firstCursor = await page.locator(".field-row.field-cursor").textContent();
    await press(page, "ArrowDown");
    expect(
      await page.locator(".field-row.field-cursor").textContent(),
      "ArrowDown must move the field cursor to a different field",
    ).not.toBe(firstCursor);

    // Walk down onto a navigable field, then Enter must navigate.
    for (let i = 0; i < 12; i++) {
      if ((await page.locator(".field-row.field-cursor.field-navigable").count()) > 0) break;
      await press(page, "ArrowDown");
    }
    await expect(page.locator(".field-row.field-cursor.field-navigable")).toHaveCount(1);
    await press(page, "Enter");
    await expect(
      page.locator("#frame-title"),
      "Enter on a focused navigable field must navigate away from web-prod-01",
    ).not.toHaveText("web-prod-01");
    await expect(page.locator(".detail-layout")).toBeVisible();
  });

  test("'!' shows the error log, not the attention filter", async ({ page }) => {
    // Bug: '!' was wrongly mapped to toggle-attention; it is the TUI's error-log
    // key. Demo has no errors -> an info flash, and the menu must NOT be filtered.
    const before = await page.locator(".menu-entry").count();
    await press(page, "!");
    await expect(page.locator("#flash")).toHaveText("No errors this session");
    expect(
      await page.locator(".menu-entry").count(),
      "'!' must not filter the menu (that is ctrl+z)",
    ).toBe(before);
  });
});
