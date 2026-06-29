import { test, expect, Page } from "@playwright/test";
import { readServer } from "./server";

// Booted + tokenized by global-setup. The tokenized URL establishes the session.
const server = readServer();

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
});
