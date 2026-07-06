import { test, expect, type Page } from "@playwright/test";
import { readServer } from "./server";
import * as fs from "fs";
import * as path from "path";

// Live-mode spec: the server runs against the REAL read-only AWS profile
// (A9S_E2E_PROFILE) and this spec asserts the rendered web UI matches
// checklists/s3.json — the checklist cmd/checklist derived independently from
// raw AWS facts (cmd/snapshot) + the docs/resources/s3.md rules, never from
// a9s code. Workflow: re-run the snapshot + checklist tools to refresh the baseline, then
// run this against the same account.
//
//   A9S_E2E_PROFILE=<profile> A9S_E2E_CHECKLIST=tests/e2e/testdata/snapshot/<profile>--<region> \
//     npx playwright test live-s3.spec.ts
//
// The baseline and the live account can drift between collection and the test
// run — a count mismatch first warrants a collector re-run, not a bug report.
const server = readServer();
const checklistDir = process.env.A9S_E2E_CHECKLIST || "";
test.skip(!server.live, "live-only spec — set A9S_E2E_PROFILE");
test.skip(!checklistDir, "set A9S_E2E_CHECKLIST=<dir containing checklists/s3.json>");

type Flagged = { name: string; decorator: string; status: string };
type Checklist = {
  menu: { display: string; availability: number; avail_truncated: boolean; issues: number; issues_truncated: boolean };
  list: {
    columns: string[];
    jargon: string[];
    shown: number;
    truncated: boolean;
    colors: Record<string, number>;
    decorators: Record<string, number>;
    flagged: Flagged[];
  };
};

const REPO_ROOT = path.resolve(__dirname, "..", "..");
const expected: Checklist = checklistDir
  ? JSON.parse(fs.readFileSync(path.join(path.resolve(REPO_ROOT, checklistDir), "checklists", "s3.json"), "utf-8"))
  : (undefined as never);

// Live Wave-2 enrichment makes one AWS call per bucket after the list opens —
// tens of seconds end-to-end. One long test keeps the session (and its
// enrichment state) instead of paying that wait per test.
test.setTimeout(300_000);
const ENRICH_WAIT_MS = 180_000;

async function press(page: Page, key: string): Promise<void> {
  await Promise.all([
    page.waitForResponse((r) => r.url().includes("/action") && r.request().method() === "POST", { timeout: 10_000 }),
    page.keyboard.press(key),
  ]);
}

test("live s3: root menu + list match the checklist", async ({ page }) => {
  await page.goto(server.url);
  await expect(page.locator(".menu-entry").first()).toBeVisible();

  // Live boot: the menu renders immediately while the AWS connect runs in the
  // background. Clicking before clients are ready silently no-ops, so wait for
  // the s3 entry's availability count — proof the session is connected and s3
  // is selectable.
  const s3Entry = page.locator(".menu-entry", { has: page.locator(".alias", { hasText: ":s3" }) });
  await expect(s3Entry.locator(".avail"), "s3 availability must load before navigating").toContainText("(", {
    timeout: 90_000,
  });

  // Navigate via the `:s3` command — index-independent, unlike clickSelect,
  // whose move-down chain skips dimmed entries and can land on the wrong row
  // in a live menu that still has unavailable types.
  await page.keyboard.press(":");
  await page.keyboard.type("s3");
  await expect(page.locator("#input-bar")).toHaveText(":s3█");
  await press(page, "Enter");
  await expect(page.locator(".list-table")).toBeVisible({ timeout: 30_000 });
  await expect(page.locator(".list-table tbody tr").first()).toBeVisible();

  // --- U10: exact columns, no jargon headers -------------------------------
  const headers = (await page.locator(".list-table thead th").allTextContents()).map((h) => h.trim());
  expect(headers).toEqual(expected.list.columns);
  for (const j of expected.list.jargon) {
    expect(headers.some((h) => h.includes(j)), `no header may include "${j}"`).toBe(false);
  }

  // --- wait for live Wave-2 enrichment to land (glyphs appear) -------------
  if (expected.list.decorators["!"] > 0) {
    await expect
      .poll(async () => page.locator(".list-table tbody tr.dec-error").count(), {
        timeout: ENRICH_WAIT_MS,
        message: "Wave-2 enrichment (dec-error rows) never arrived in the live web UI",
      })
      .toBeGreaterThan(0);
  }

  // --- list: row count, colors, decorators, truncation ---------------------
  await expect(page.locator(".list-table tbody tr")).toHaveCount(expected.list.shown);
  await expect(page.locator(".list-table tbody tr.row-healthy")).toHaveCount(expected.list.colors.healthy);
  await expect(page.locator(".list-table tbody tr.row-broken")).toHaveCount(expected.list.colors.broken);
  await expect(page.locator(".list-table tbody tr.row-warning")).toHaveCount(expected.list.colors.warning);
  await expect(page.locator(".list-table tbody tr.row-dim")).toHaveCount(expected.list.colors.dim);
  await expect(page.locator(".list-table tbody tr.dec-error")).toHaveCount(expected.list.decorators["!"]);
  await expect(page.locator(".list-table tbody tr.dec-warn")).toHaveCount(expected.list.decorators["~"]);
  if (expected.list.truncated) {
    await expect(page.locator(".list-truncated")).toBeVisible();
  } else {
    await expect(page.locator(".list-truncated")).toHaveCount(0);
  }

  // --- flagged rows: VISIBLE glyph before the name + status + green color --
  await expect(page.locator(".list-table tbody .row-glyph", { hasText: "!" })).toHaveCount(
    expected.list.decorators["!"],
  );
  for (const f of expected.list.flagged) {
    const row = page.locator(".list-table tbody tr", { has: page.locator("td", { hasText: f.name }) });
    await expect(row, `row for ${f.name}`).toHaveCount(1);
    const nameCell = row.locator("td", { hasText: f.name });
    await expect(nameCell.locator(".row-glyph"), `${f.name} glyph`).toHaveText(f.decorator);
    await expect(nameCell, `${f.name} name cell layout`).toHaveText(`${f.decorator} ${f.name}`);
    await expect(row, `${f.name} status text`).toContainText(f.status);
    // S2/S3: finding rides a HEALTHY (green) row — assert the rendered color.
    await expect(nameCell, `${f.name} renders green`).toHaveCSS("color", "rgb(158, 206, 106)");
  }

  // --- S4: only findings populate Status; healthy rows render blank --------
  const statusIdx = headers.findIndex((h) => h === "Status");
  expect(statusIdx, "a Status column must exist").toBeGreaterThanOrEqual(0);
  const nonBlankStatus = await page
    .locator(".list-table tbody tr")
    .evaluateAll(
      (rows, idx) => rows.filter((r) => (r.children[idx]?.textContent ?? "").trim() !== "").length,
      statusIdx,
    );
  expect(nonBlankStatus, "only findings may populate the Status column").toBe(expected.list.flagged.length);

  // --- back to menu: S1 issue badge (session kept the enrichment state) ----
  // The list's row flags can arrive instantly from the persisted type cache
  // (C6b), so reaching this point does NOT mean the in-session Wave-2 pass
  // has finished — on live data it makes one call per bucket and the badge
  // lands via SSE when the background drain completes. Give the badge the
  // same enrichment budget the in-list wait has, not the default 5s.
  await press(page, "Escape");
  await expect(s3Entry).toBeVisible();
  if (expected.menu.issues > 0) {
    const badgeText = `! ${expected.menu.issues}${expected.menu.issues_truncated ? "+" : ""}`;
    await expect(s3Entry.locator(".badge")).toHaveText(badgeText, { timeout: ENRICH_WAIT_MS });
  } else {
    await expect(s3Entry.locator(".badge")).toHaveCount(0);
  }
  const availText = `(${expected.menu.availability}${expected.menu.avail_truncated ? "+" : ""})`;
  await expect(s3Entry.locator(".avail")).toHaveText(availText);
});
