# a9s Website UX/UI Audit & Implementation Plan

**Date:** 2026-03-22
**Auditor:** Playwright-assisted automated + manual visual review
**Pages reviewed:** Home, Install, Docs, Resources, 404
**Viewports tested:** Desktop (1440x900), Tablet (768x1024), Mobile (375x812)

---

## Executive Summary

The website has a solid foundation — clean Tokyo Night Dark theme, good information hierarchy, and working Hugo build pipeline. However, it has **critical mobile usability issues**, **missing SEO/social meta tags**, **no 404 page**, **no favicon**, and several **accessibility gaps** that should be addressed before any promotion.

---

## Findings by Severity

### P0 — Critical (Blocks usability)

#### 1. Mobile Navigation Broken
- **Issue:** Header nav wraps to 2 lines on mobile (84px tall vs expected ~50px). Nav links are plain text with no responsive behavior.
- **Evidence:** `home-mobile-full.png` — "GitHub" wraps to second line under "a9s" logo.
- **Fix:** Add hamburger menu for screens < 640px, or collapse nav to icon row.

#### 2. Code Blocks Overflow on Mobile
- **Issue:** 4 of 5 code blocks on the Install page overflow their container on mobile (375px). Docker command overflows by 369px.
- **Evidence:** `install-mobile-full.png` — commands are truncated/hidden on right edge.
- **Fix:** Add `overflow-x: auto` to `pre` blocks (already present) BUT also add `word-break: break-all` for inline code, and reduce `font-size` on mobile. Consider wrapping long commands.

#### 3. 404 Page is Completely Unstyled
- **Issue:** Navigating to any invalid URL shows a plain white "Page Not Found" heading — no header, footer, navigation, or theme styling.
- **Evidence:** `404-page.png` — stark white page with black text.
- **Fix:** Create `website/themes/a9s-theme/layouts/404.html` using the baseof template.

### P1 — High (Degrades experience significantly)

#### 4. Missing Favicon
- **Issue:** Console error `404 /favicon.ico` on every page load. No `<link rel="icon">` in HTML.
- **Fix:** Create a simple favicon (terminal icon or "a9s" monogram) and add to `website/static/`. Add `<link rel="icon">` to baseof.html.

#### 5. Header Logo Not Clickable
- **Issue:** The "a9s" text in the header is an `<h1>` with no link. Users expect the logo/site name to link to the homepage.
- **Fix:** Wrap `<h1>a9s</h1>` in `<a href="{{ "/" | relURL }}">`.

#### 6. No Active Nav State
- **Issue:** No visual indicator for the current page in the nav bar. All 4 links look identical regardless of which page you're on.
- **Fix:** Add conditional class in Hugo template: `{{ if eq .RelPermalink "/a9s/install/" }}class="active"{{ end }}` and style with `color: var(--accent); font-weight: 600`.

#### 7. Touch Targets Too Small
- **Issue:** All nav links are 17px tall on mobile. WCAG requires minimum 44px touch targets.
- **Fix:** Add `padding: 12px 8px` to nav links, especially on mobile.

#### 8. No Copy Button on Install Command
- **Issue:** The hero install box (`$ brew install k2m30/a9s/a9s`) has no copy button. Users must manually select text.
- **Fix:** Add a small clipboard icon/button with `navigator.clipboard.writeText()` functionality.

### P2 — Medium (SEO/Social/Polish)

#### 9. Missing Social Meta Tags
- **Issue:** No `og:image`, no `twitter:card`, no `twitter:image`. Sharing on Twitter/Slack/Discord will show a plain text link.
- **Fix:** Create an OG image (1200x630) and add meta tags to baseof.html:
  ```html
  <meta property="og:image" content="...">
  <meta name="twitter:card" content="summary_large_image">
  <meta name="twitter:image" content="...">
  ```

#### 10. Missing Canonical URLs
- **Issue:** No `<link rel="canonical">` on any page. Can cause SEO duplicate content issues with trailing slashes.
- **Fix:** Add `<link rel="canonical" href="{{ .Permalink }}">` to baseof.html.

#### 11. No Skip-to-Content Link
- **Issue:** Keyboard-only users cannot skip the nav to reach main content. WCAG 2.1 Level A requirement.
- **Fix:** Add visually-hidden skip link as first element in body: `<a href="#main" class="skip-link">Skip to content</a>` and add `id="main"` to the `<main>` element.

#### 12. Demo GIF Performance
- **Issue:** GIF is 1.5MB and 1200x500 natural size. On mobile it displays at ~335x140 — serving 3.6x more pixels than needed.
- **Fix:** Convert to WebM/MP4 video (typically 60-80% smaller) with GIF fallback, or at minimum provide a smaller mobile variant. Consider lazy loading.

### P3 — Low (Nice to have)

#### 13. No prefers-reduced-motion Support
- **Issue:** If any animations are added in the future, they won't respect user motion preferences. Good practice to add now.
- **Fix:** Add `@media (prefers-reduced-motion: reduce) { *, *::before, *::after { animation-duration: 0.01ms !important; } }`.

#### 14. Header Not Sticky
- **Issue:** On long pages (Docs, Resources), scrolling loses the navigation. Users must scroll all the way up.
- **Fix:** Consider `position: sticky; top: 0; z-index: 100` on the header.

#### 15. No Table of Contents on Long Pages
- **Issue:** The Docs page is very long (key bindings, commands, config, permissions). No way to jump to sections.
- **Fix:** Add a TOC sidebar or sticky section nav for the Docs page.

#### 16. Resources Page Has No Search/Filter
- **Issue:** 62 resource types in 12 tables. Finding a specific resource requires scrolling through the entire page.
- **Fix:** Add a simple client-side filter input at the top that hides non-matching rows.

#### 17. Footer Is Minimal
- **Issue:** Footer only has license + GitHub link. Missing useful links (Install, Docs, Resources) and version info.
- **Fix:** Expand footer with a simple 2-column layout: navigation links and project info.

#### 18. No Dark/Light Mode Toggle
- **Issue:** The site is dark-mode only. Some users prefer light mode, especially for documentation reading.
- **Fix:** Lower priority — the terminal aesthetic justifies dark-only, but a toggle would be a nice touch.

#### 19. Redundant Page Title
- **Issue:** Home page `<title>` is "a9s — Terminal UI for AWS | a9s" — "a9s" appears twice.
- **Fix:** Use `{{ if .IsHome }}a9s — Terminal UI for AWS{{ else }}{{ .Title }} | a9s{{ end }}`.

---

## Implementation Plan

### Phase 1: Critical Fixes (P0) — Est. 1-2 hours

| # | Task | File(s) |
|---|------|---------|
| 1.1 | Add responsive nav with hamburger menu for mobile | `baseof.html` (CSS + JS) |
| 1.2 | Fix code block overflow on mobile | `baseof.html` (CSS media query) |
| 1.3 | Create styled 404 page | `layouts/404.html` |

### Phase 2: High Priority (P1) — Est. 2-3 hours

| # | Task | File(s) |
|---|------|---------|
| 2.1 | Create and add favicon | `static/favicon.ico`, `baseof.html` |
| 2.2 | Make header logo clickable to home | `baseof.html` |
| 2.3 | Add active state to current nav link | `baseof.html` (Hugo conditional + CSS) |
| 2.4 | Increase nav link touch targets | `baseof.html` (CSS) |
| 2.5 | Add copy button to install command box | `index.html` + `baseof.html` (JS) |

### Phase 3: SEO & Accessibility (P2) — Est. 1-2 hours

| # | Task | File(s) |
|---|------|---------|
| 3.1 | Add og:image, twitter:card meta tags | `baseof.html`, `static/og-image.png` |
| 3.2 | Add canonical URLs | `baseof.html` |
| 3.3 | Add skip-to-content link | `baseof.html` (HTML + CSS) |
| 3.4 | Optimize demo GIF (convert to video or compress) | `static/demo.gif` → `static/demo.webm` |

### Phase 4: Polish (P3) — Est. 2-4 hours

| # | Task | File(s) |
|---|------|---------|
| 4.1 | Add prefers-reduced-motion media query | `baseof.html` (CSS) |
| 4.2 | Make header sticky | `baseof.html` (CSS) |
| 4.3 | Add TOC to Docs page | `layouts/_default/single.html` or new partial |
| 4.4 | Add search/filter to Resources page | `resources.md` + JS |
| 4.5 | Expand footer with nav links | `baseof.html` |
| 4.6 | Fix redundant page title | `baseof.html` |

---

## Appendix: Screenshots

All screenshots saved during audit:
- `home-desktop-full.png` — Homepage at 1440px
- `home-tablet-full.png` — Homepage at 768px
- `home-mobile-full.png` — Homepage at 375px
- `install-desktop-full.png` — Install page at 1440px
- `install-mobile-full.png` — Install page at 375px (code overflow visible)
- `docs-desktop-full.png` — Docs page at 1440px
- `docs-mobile-full.png` — Docs page at 375px
- `resources-desktop-full.png` — Resources page at 1440px
- `404-page.png` — Unstyled 404 page
