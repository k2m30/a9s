# a9s Website UX/UI Audit & Implementation Plan

**Date:** 2026-03-22
**Auditor:** Playwright-assisted automated + manual visual review
**Pages reviewed:** Home, Install, Docs, Resources, 404
**Viewports tested:** Desktop (1440x900), Tablet (768x1024), Mobile (375x812)
**Hugo version:** v0.158.0+extended (Homebrew)
**Theme:** Custom `a9s-theme` — all CSS inlined in baseof.html `<style>`, no external stylesheets

---

## Executive Summary

The website has a solid foundation — clean Tokyo Night Dark theme, good information hierarchy, and working Hugo build pipeline. However, it has **critical mobile usability issues**, **missing SEO/social meta tags**, **no 404 page**, **no favicon**, and several **accessibility gaps** that should be addressed before any promotion.

---

## Findings by Severity

### P0 — Critical (Blocks usability)

#### 1. Mobile Navigation Broken
- **Issue:** Header nav wraps to 2 lines on mobile (84px tall vs expected ~50px). Nav links are plain text with no responsive behavior.
- **Evidence:** `home-mobile-full.png` — "GitHub" wraps to second line under "a9s" logo.
- **Fix:** CSS-only hamburger menu using `<input type="checkbox">` + sibling selector (no JS needed). Add a `@media (max-width: 640px)` block in the `<style>` tag in `baseof.html`. Hugo doesn't impose constraints here — this is pure CSS/HTML in the layout template.
- **Alternative:** Simply reduce nav font-size and spacing on mobile via media query if a hamburger feels overbuilt for 4 links.

#### 2. Code Blocks Overflow on Mobile
- **Issue:** 4 of 5 code blocks on the Install page overflow their container on mobile (375px). Docker command overflows by 369px.
- **Evidence:** `install-mobile-full.png` — commands are truncated/hidden on right edge.
- **Fix:** The `pre` already has `overflow-x: auto` in baseof.html CSS, but it's not visually obvious that content is scrollable. Add `@media (max-width: 640px) { pre { font-size: 0.8rem; } }` to reduce overflow, and add a subtle right-fade gradient or scrollbar styling to signal scrollability. No Hugo involvement — pure CSS in baseof.html.
- **Note:** The code blocks come from markdown rendered by Hugo's Goldmark engine. The `include` shortcode pipes through `markdownify`, which wraps fenced code in `<pre><code>`. These are standard HTML elements fully controllable via CSS.

#### 3. 404 Page is Completely Unstyled
- **Issue:** Navigating to any invalid URL shows a plain white "Page Not Found" heading — no header, footer, navigation, or theme styling.
- **Evidence:** `404-page.png` — stark white page with black text.
- **Fix:** Create `website/themes/a9s-theme/layouts/404.html`. Hugo looks for this exact path — it's a [built-in lookup](https://gohugo.io/templates/404/). The template should extend baseof via `{{ define "main" }}...{{ end }}` to inherit the header/footer/styles. GitHub Pages serves `/404.html` automatically for missing routes.
- **Hugo note:** The 404 template has access to all Hugo functions (`.Site`, `relURL`, etc.) but NOT to page-level context like `.Title` from front matter. Use a hardcoded title.

### P1 — High (Degrades experience significantly)

#### 4. Missing Favicon
- **Issue:** Console error `404 /favicon.ico` on every page load. No `<link rel="icon">` in HTML.
- **Fix:** Create a simple favicon (terminal icon or "a9s" monogram) and add to `website/static/`. Add `<link rel="icon">` to baseof.html.

#### 5. Header Logo Not Clickable
- **Issue:** The "a9s" text in the header is an `<h1>` with no link. Users expect the logo/site name to link to the homepage.
- **Fix:** In baseof.html line 161, change `<h1>a9s</h1>` to `<h1><a href="{{ "/" | relURL }}" style="color: inherit; text-decoration: none;">a9s</a></h1>`. The `relURL` function correctly prepends the `baseURL` path prefix (`/a9s/`) needed for GitHub Pages.

#### 6. No Active Nav State
- **Issue:** No visual indicator for the current page in the nav bar. All 4 links look identical regardless of which page you're on.
- **Fix:** Use Hugo's `hasPrefix` for path matching. In baseof.html nav, change each link to:
  ```
  <a href="{{ "install/" | relURL }}"{{ if hasPrefix .RelPermalink "/a9s/install" }} class="active"{{ end }}>Install</a>
  ```
  Then add CSS: `header nav a.active { color: var(--accent); font-weight: 600; }`.
- **Hugo note:** `.RelPermalink` includes the baseURL path prefix (`/a9s/`). Using `hasPrefix` rather than `eq` handles both `/a9s/install/` and potential sub-pages. Alternatively, Hugo's menu system (`{{ .IsMenuCurrent }}`) could be used, but would require adding `[menu]` config to hugo.toml — overbuilt for 4 static links.

#### 7. Touch Targets Too Small
- **Issue:** All nav links are 17px tall on mobile. WCAG requires minimum 44px touch targets.
- **Fix:** Add `padding: 12px 8px` to nav links, especially on mobile.

#### 8. No Copy Button on Install Command
- **Issue:** The hero install box (`$ brew install k2m30/a9s/a9s`) has no copy button. Users must manually select text.
- **Fix:** Add a small clipboard icon/button with `navigator.clipboard.writeText()` functionality.

### P2 — Medium (SEO/Social/Polish)

#### 9. Missing Social Meta Tags
- **Issue:** No `og:image`, no `twitter:card`, no `twitter:image`. Sharing on Twitter/Slack/Discord will show a plain text link. Note: `og:title`, `og:description`, `og:type`, and `og:url` are already present in baseof.html — only `og:image` and Twitter tags are missing.
- **Fix:** Create an OG image (1200x630) in `website/static/og-image.png` and add to baseof.html `<head>`:
  ```html
  <meta property="og:image" content="{{ "og-image.png" | absURL }}">
  <meta name="twitter:card" content="summary_large_image">
  <meta name="twitter:image" content="{{ "og-image.png" | absURL }}">
  ```
- **Hugo note:** Use `absURL` (not `relURL`) for social meta tags — crawlers need absolute URLs. Hugo's `absURL` function prepends the full `baseURL` from hugo.toml.

#### 10. Missing Canonical URLs
- **Issue:** No `<link rel="canonical">` on any page. Can cause SEO duplicate content issues with trailing slashes.
- **Fix:** Add `<link rel="canonical" href="{{ .Permalink }}">` to baseof.html `<head>`.
- **Hugo note:** `.Permalink` is a built-in page variable that returns the absolute URL for the current page, built from `baseURL` + page path. It handles trailing slashes according to Hugo's `uglyURLs` setting (default: pretty URLs with `/`).

#### 11. No Skip-to-Content Link
- **Issue:** Keyboard-only users cannot skip the nav to reach main content. WCAG 2.1 Level A requirement.
- **Fix:** Add visually-hidden skip link as first element in body: `<a href="#main" class="skip-link">Skip to content</a>` and add `id="main"` to the `<main>` element.

#### 12. Demo GIF Performance
- **Issue:** GIF is 1.5MB and 1200x500 natural size. On mobile it displays at ~335x140 — serving 3.6x more pixels than needed.
- **Fix option A (simple):** Add `loading="lazy"` to the `<img>` tag in `index.html`. No Hugo involvement needed.
- **Fix option B (optimal):** Convert GIF to WebM/MP4 and use `<video autoplay muted loop playsinline>` with a `<source>` for each format. Place files in `website/static/`.
- **Hugo note:** Hugo v0.158.0 extended has built-in image processing (`resources.Get` + `.Resize`/`.Fill`) that could generate responsive image variants, but only for images stored in `assets/` (not `static/`). To use Hugo image processing, move `demo.gif` to `website/assets/` and use `{{ $img := resources.Get "demo.gif" }}` in the template. However, Hugo cannot process GIFs into video formats — that requires an external tool (ffmpeg). For a static GIF, `loading="lazy"` in the template HTML is the lowest-effort win.

### P3 — Low (Nice to have)

#### 13. No prefers-reduced-motion Support
- **Issue:** If any animations are added in the future, they won't respect user motion preferences. Good practice to add now.
- **Fix:** Add `@media (prefers-reduced-motion: reduce) { *, *::before, *::after { animation-duration: 0.01ms !important; } }`.

#### 14. Header Not Sticky
- **Issue:** On long pages (Docs, Resources), scrolling loses the navigation. Users must scroll all the way up.
- **Fix:** Consider `position: sticky; top: 0; z-index: 100` on the header.

#### 15. No Table of Contents on Long Pages
- **Issue:** The Docs page is very long (key bindings, commands, config, permissions). No way to jump to sections.
- **Fix:** Hugo has a built-in `{{ .TableOfContents }}` variable that generates a `<nav id="TableOfContents">` with nested `<ul>` from the page's headings. Add it to `list.html` (docs uses `_index.md`, which renders via the list template).
- **Hugo caveat:** `{{ .TableOfContents }}` is generated by the Goldmark markdown parser. The h2 headings written directly in `_index.md` (`## Getting Started`, `## Key Bindings`, etc.) **will** appear in the TOC. However, h3 headings from the `include` shortcode (e.g., `### Navigation`, `### Actions` from `keybindings.md`) are converted to HTML by `markdownify` inside the shortcode — Goldmark may **not** pick these up since they're shortcode-emitted HTML, not markdown headings. The top-level h2s should be sufficient for a useful TOC on this page.
- **Alternative if sub-headings needed:** Build a manual TOC with anchor links in the template, or switch the shortcode to use `{{% include %}}` (percent delimiters) so the inner content is processed as markdown by Goldmark — but this would require the shortcode to output raw markdown, not HTML.
- **Styling:** Hugo emits a bare `<nav>` with `<ul><li>` — all styling is via CSS. Add a sticky sidebar or top-of-page list.

#### 16. Resources Page Has No Search/Filter
- **Issue:** 62 resource types in 12 tables. Finding a specific resource requires scrolling through the entire page.
- **Fix:** Add a client-side JS filter. Since Hugo generates static HTML, the filtering must be client-side JavaScript in the template. Add an `<input>` and a `<script>` block to the resources page layout (either inline in `resources.md` or via a dedicated layout `layouts/resources/single.html`).
- **Hugo note:** Hugo is a static site generator — no server-side filtering is possible. All filtering must be JS in the browser. The simplest approach is a `<script>` tag at the bottom of the page that queries `document.querySelectorAll('table tr')` and toggles `display` based on input value.

#### 17. Footer Is Minimal
- **Issue:** Footer only has license + GitHub link. Missing useful links (Install, Docs, Resources) and version info.
- **Fix:** Expand footer with a simple 2-column layout: navigation links and project info.

#### 18. No Dark/Light Mode Toggle
- **Issue:** The site is dark-mode only. Some users prefer light mode, especially for documentation reading.
- **Fix:** Lower priority — the terminal aesthetic justifies dark-only, but a toggle would be a nice touch. Implementation requires: (1) a second set of CSS custom properties for light mode, (2) a JS toggle that adds/removes a `data-theme="light"` attribute on `<html>`, (3) `localStorage` to persist the preference. Since all CSS is inline in baseof.html, this is straightforward.
- **Hugo note:** Hugo has no built-in theme toggle. This is purely a CSS/JS concern in the layout template.

#### 19. Redundant Page Title
- **Issue:** Home page `<title>` is "a9s — Terminal UI for AWS | a9s" — "a9s" appears twice. The current template is `{{ .Title }} | a9s` and `.Title` for the home page is the `title` from hugo.toml ("a9s — Terminal UI for AWS").
- **Fix:** Use Hugo's `.IsHome` check: `{{ if .IsHome }}{{ .Site.Title }}{{ else }}{{ .Title }} | a9s{{ end }}`.
- **Hugo note:** `.IsHome` is a built-in page method. `.Site.Title` pulls from the `title` key in hugo.toml.

---

## Implementation Plan

All fixes are in the Hugo theme at `website/themes/a9s-theme/`. CSS is inline in baseof.html (no external stylesheets). Hugo version: v0.158.0+extended.

### Phase 1: Critical Fixes (P0)

| # | Task | File(s) | Hugo Feature |
|---|------|---------|-------------|
| 1.1 | Add responsive nav (CSS-only hamburger or reduced spacing) | `layouts/_default/baseof.html` — `@media` block in `<style>` | None (pure CSS) |
| 1.2 | Fix code block overflow on mobile | `layouts/_default/baseof.html` — `@media` block for `pre` font-size | None (pure CSS). Code blocks from `markdownify` in `include` shortcode |
| 1.3 | Create styled 404 page | `layouts/404.html` (new file, extends baseof via `{{ define "main" }}`) | [Hugo 404 template lookup](https://gohugo.io/templates/404/). GitHub Pages serves `/404.html` automatically |

### Phase 2: High Priority (P1)

| # | Task | File(s) | Hugo Feature |
|---|------|---------|-------------|
| 2.1 | Create and add favicon | `../../static/favicon.ico` + `<link>` in baseof.html `<head>` | `relURL` for path prefix |
| 2.2 | Make header logo link to home | `layouts/_default/baseof.html` line 161 | `{{ "/" | relURL }}` for baseURL-aware home link |
| 2.3 | Add active state to current nav link | `layouts/_default/baseof.html` nav section | `{{ hasPrefix .RelPermalink "/a9s/install" }}` conditional class. `.RelPermalink` includes baseURL path prefix |
| 2.4 | Increase nav link touch targets | `layouts/_default/baseof.html` — CSS padding on `header nav a` | None (pure CSS) |
| 2.5 | Add copy button to install command | `layouts/index.html` — add `<button>` + inline `<script>` | None (client-side JS). Hugo generates static HTML |

### Phase 3: SEO & Accessibility (P2)

| # | Task | File(s) | Hugo Feature |
|---|------|---------|-------------|
| 3.1 | Add og:image + twitter:card meta tags | `layouts/_default/baseof.html` `<head>` + `../../static/og-image.png` | `{{ "og-image.png" | absURL }}` — must use `absURL` (not `relURL`) for social crawlers |
| 3.2 | Add canonical URLs | `layouts/_default/baseof.html` `<head>` | `{{ .Permalink }}` — built-in absolute URL per page |
| 3.3 | Add skip-to-content link | `layouts/_default/baseof.html` — first element in `<body>` + `id="main"` on `<main>` | None (pure HTML/CSS) |
| 3.4 | Add `loading="lazy"` to demo GIF | `layouts/index.html` — `<img>` tag | None (HTML attribute). For video conversion, use external ffmpeg |

### Phase 4: Polish (P3)

| # | Task | File(s) | Hugo Feature |
|---|------|---------|-------------|
| 4.1 | Add prefers-reduced-motion | `layouts/_default/baseof.html` — CSS `@media` block | None (pure CSS) |
| 4.2 | Make header sticky | `layouts/_default/baseof.html` — CSS `position: sticky` | None (pure CSS) |
| 4.3 | Add TOC to Docs page | `layouts/_default/list.html` (docs is `_index.md` → list template) | `{{ .TableOfContents }}` — Hugo built-in. Will include h2 headings from `_index.md` but may miss h3s from `include` shortcode (they're `markdownify` HTML, not Goldmark-parsed markdown). Top-level h2 TOC is sufficient |
| 4.4 | Add search/filter to Resources page | `layouts/_default/single.html` or dedicated `layouts/resources/single.html` + inline `<script>` | None (client-side JS). Hugo is static — no server-side filtering possible |
| 4.5 | Expand footer with nav links | `layouts/_default/baseof.html` footer section | `relURL` for internal links |
| 4.6 | Fix redundant page title | `layouts/_default/baseof.html` `<title>` tag | `{{ .IsHome }}` conditional + `{{ .Site.Title }}` from hugo.toml |

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
