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
