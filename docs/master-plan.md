# a9s Open-Source Master Plan

Reference model: [k9s](https://github.com/derailed/k9s) (33k stars, Apache-2.0, sole maintainer)

## Current State Audit

### What Exists

| Area | Status | Notes |
|------|--------|-------|
| CI workflow (lint, test, build, security, verify-readonly, install-test) | DONE | All green |
| CodeQL workflow | DONE | |
| Release workflow (GoReleaser on tag push) | DONE | Triggers on `v*` tags |
| Pages workflow | DONE | Hugo site stub |
| Stale issues workflow | DONE | 30d stale, 7d close |
| `.goreleaser.yaml` | DONE | linux/darwin/windows x amd64/arm64, deb/rpm/apk, Homebrew tap, cosign, SBOM, Docker |
| Dockerfile | DONE | Scratch-based, minimal |
| `.golangci.yml` | DONE | v2, 9 linters, diagnostic gocritic |
| Dependabot | DONE | gomod + github-actions weekly |
| Issue templates (bug + feature) | DONE | YAML form-based |
| PR template | DONE | |
| CODEOWNERS | DONE | |
| FUNDING.yml | DONE | GitHub Sponsors |
| CODE_OF_CONDUCT.md | DONE | Contributor Covenant |
| CONTRIBUTING.md | DONE | |
| SECURITY.md | DONE | |
| CHANGELOG.md | DONE | |
| ROADMAP.md | DONE | |
| LICENSE (GPL-3.0) | DONE | |
| README.md with badges | DONE | CI, Report Card, Release, License, Downloads, Codecov |
| go.mod at Go 1.26.1 | DONE | |
| Branch protection on main | DONE | Requires status checks |
| Wiki disabled | DONE | |
| GitHub Pages enabled | DONE | Actions source |
| Topics set | DONE | 10 topics |
| Discussions enabled | DONE | |
| 1,045+ unit tests | DONE | |
| golangci-lint + govulncheck local prereqs | DONE | Documented in CLAUDE.md |

### What's Missing or Incomplete

| # | Area | Priority | Effort | Notes |
|---|------|----------|--------|-------|
| 1 | **Animated demo GIF/recording** | HIGH | Medium | README says `<!-- TODO -->`. Need VHS tape or asciinema + live AWS. k9s uses YouTube + asciinema + PNGs. |
| 2 | **Website (k2m30.github.io/a9s)** | HIGH | Large | Pages workflow exists but site is a stub. k9s has full Hugo site at k9scli.io with features, docs, install guide. |
| 3 | **HOMEBREW_TAP_TOKEN secret** | HIGH | Small | GoReleaser needs this to push formula. Create PAT, add as repo secret. |
| 4 | **First release (v3.0.0)** | HIGH | Small | Tag + push triggers GoReleaser. Blocked on #1, #3. |
| 5 | **README restructure** | MEDIUM | Medium | See detailed plan below. |
| 6 | **Docker multi-arch build** | MEDIUM | Medium | Current Dockerfile works but GoReleaser config uses `dockers_v2` (not a real key). Needs `dockers` with buildx for amd64+arm64. |
| 7 | **Release notes template** | MEDIUM | Small | k9s has hand-written per-release notes in `change_logs/`. We should have a template. |
| 8 | **Social preview image** | MEDIUM | Small | 1280x640 PNG for GitHub social card. |
| 9 | **Cosign key setup** | MEDIUM | Small | GoReleaser config references cosign but needs keyless signing setup (OIDC via GitHub Actions). |
| 10 | **Codecov token** | LOW | Small | Badge exists but `CODECOV_TOKEN` secret may not be configured. |
| 11 | **Go Report Card** | LOW | Small | Badge links to goreportcard.com but may not be indexed yet. Visit the URL to trigger first scan. |
| 12 | **Homepage field** | LOW | Small | Currently points to GitHub repo. Should point to Pages site once ready. |
| 13 | **Additional distribution** | LOW | Large | Snap, AUR, Scoop, Winget, MacPorts. k9s supports 15+ methods. Start with what GoReleaser gives us for free. |
| 14 | **Per-release changelog files** | LOW | Small | k9s keeps `change_logs/release_v0.X.Y.md`. Optional but nice for detailed notes. |
| 15 | **Community growth** | LOW | Ongoing | Slack/Discord, blog posts, conference talks, YouTube demos. |

---

## Detailed Plans

### 1. Animated Demo

**Options (pick one):**

| Method | Pros | Cons |
|--------|------|------|
| **VHS** (charmbracelet/vhs) | Reproducible `.tape` files, GIF/MP4 output, CI-able | Needs live AWS or mock server |
| **Asciinema** | Terminal-native, lightweight, embeddable | Requires real terminal session, not a GIF |
| **Screen recording** (manual) | Highest quality, shows real usage | Not reproducible, manual effort |

**Recommendation:** VHS with a `.tape` file checked into `docs/demos/demo.tape`. Use the `gobubble-dev` AWS profile for real data. Generate both GIF (for README) and MP4 (for website).

**Demo script should show:**
1. Launch a9s, main menu visible (resource categories)
2. Navigate to EC2, show instance list with status colors
3. Filter instances by name
4. Open detail view, scroll through fields
5. Press `y` for YAML view
6. Press `Esc` back to list
7. Switch to S3, drill into bucket, show objects
8. Press `?` for help screen

**Files to create:**
- `docs/demos/demo.tape` -- VHS script
- `assets/demo.gif` -- Generated output (checked in or generated in CI)

### 2. Website

**Tech stack:** Hugo (same as k9s) via GitHub Pages workflow (already exists).

**Pages to create:**

| Page | Content |
|------|---------|
| **Home** | Hero with terminal screenshot/GIF, tagline, feature highlights, install snippet |
| **Features** | Grid of features with screenshots: resource browsing, YAML view, multi-profile, filtering, etc. |
| **Install** | All installation methods with copy-paste commands |
| **Docs** | Key bindings reference, configuration (views.yaml), AWS profile setup |
| **Resource Types** | Full list of 62 supported resource types grouped by category |

**Directory structure:**
```
website/
  config.toml
  content/
    _index.md
    features.md
    install.md
    docs/
      keybindings.md
      configuration.md
      aws-setup.md
    resources.md
  static/
    img/
  themes/
```

### 3. HOMEBREW_TAP_TOKEN

1. Create a GitHub PAT (classic) with `repo` scope
2. Add as repository secret: `Settings > Secrets > Actions > HOMEBREW_TAP_TOKEN`
3. Create the tap repo `k2m30/homebrew-a9s` if it doesn't exist
4. GoReleaser will auto-push the formula on release

### 4. First Release (v3.0.0)

**Pre-release checklist:**
- [ ] Demo GIF generated and added to README (#1)
- [ ] HOMEBREW_TAP_TOKEN secret configured (#3)
- [ ] Cosign keyless signing tested (#9)
- [ ] GoReleaser config validated: `goreleaser check`
- [ ] GoReleaser dry run: `goreleaser release --snapshot --clean`
- [ ] CODECOV_TOKEN configured (#10)
- [ ] Go Report Card first scan triggered (#11)
- [ ] README finalized (#5)
- [ ] Version in `cmd/a9s/main.go` set to `3.0.0`

**Release process:**
```
git tag -a v3.0.0 -m "v3.0.0: initial open-source release"
git push origin v3.0.0
```
GoReleaser workflow triggers automatically, producing:
- GitHub Release with binaries (6 archives: linux/darwin/windows x amd64/arm64)
- Linux packages (.deb, .rpm, .apk)
- Docker image at ghcr.io/k2m30/a9s
- Homebrew formula updated
- Checksums + cosign signature
- SBOM for each archive

### 5. README Restructure

**Target structure (modeled on k9s):**

```
# a9s - Terminal UI for AWS

[badges row]

[demo GIF]

[one-paragraph description]

## Features
[bullet list with screenshots]

## Installation
### Homebrew
### Go install
### Docker
### Download binary
### Build from source

## Quick Start
[3-step: install, configure AWS profile, run]

## Key Bindings
[table of essential keys]

## Configuration
[views.yaml customization]

## Supported Resource Types
[table: 62 types grouped by category]

## Screenshots
[detail view, YAML view, help screen, filtering]

## Contributing
[link to CONTRIBUTING.md]

## License
[GPL-3.0]
```

### 6. Docker Multi-Arch

Current `.goreleaser.yaml` uses `dockers_v2` which is not a valid GoReleaser key. Fix:

```yaml
dockers:
  - image_templates:
      - "ghcr.io/k2m30/a9s:v{{ .Version }}-amd64"
    use: buildx
    build_flag_templates:
      - "--platform=linux/amd64"
    goarch: amd64
  - image_templates:
      - "ghcr.io/k2m30/a9s:v{{ .Version }}-arm64"
    use: buildx
    build_flag_templates:
      - "--platform=linux/arm64"
    goarch: arm64

docker_manifests:
  - name_template: "ghcr.io/k2m30/a9s:v{{ .Version }}"
    image_templates:
      - "ghcr.io/k2m30/a9s:v{{ .Version }}-amd64"
      - "ghcr.io/k2m30/a9s:v{{ .Version }}-arm64"
  - name_template: "ghcr.io/k2m30/a9s:latest"
    image_templates:
      - "ghcr.io/k2m30/a9s:v{{ .Version }}-amd64"
      - "ghcr.io/k2m30/a9s:v{{ .Version }}-arm64"
```

### 9. Cosign Keyless Signing

GoReleaser config already has `signs:` with cosign. For keyless signing in GitHub Actions:

1. Ensure `id-token: write` permission in release workflow
2. Cosign uses OIDC (Sigstore Fulcio) automatically in GitHub Actions
3. No keys to manage — signatures are tied to the GitHub Actions identity
4. Verify locally: `cosign verify-blob --signature checksums.txt.sig checksums.txt --certificate-identity-regexp='github.com/k2m30/a9s' --certificate-oidc-issuer='https://token.actions.githubusercontent.com'`

---

## Implementation Order

### Phase 1: Release-Ready (do first)
1. Fix GoReleaser config (docker multi-arch, validate with `goreleaser check`)
2. Create HOMEBREW_TAP_TOKEN secret (manual)
3. Create `k2m30/homebrew-a9s` repo (manual)
4. Ensure cosign works (add `id-token: write` to release workflow)
5. Configure CODECOV_TOKEN secret (manual)
6. Run `goreleaser release --snapshot --clean` locally to validate

### Phase 2: Demo & README
7. Record demo with VHS (needs `gobubble-dev` AWS profile)
8. Restructure README with demo GIF and screenshots
9. Create social preview image
10. Trigger Go Report Card first scan

### Phase 3: Website
11. Build Hugo site in `website/` directory
12. Update homepage field to point to Pages URL
13. Add features page with screenshots
14. Add docs pages (keybindings, config, AWS setup)

### Phase 4: First Release
15. Set version to 3.0.0
16. Tag v3.0.0, push — GoReleaser produces release
17. Verify: GitHub Release, Homebrew formula, Docker image, cosign signatures, SBOM
18. Announce: README updated, website live

### Phase 5: Community & Polish (ongoing)
19. Per-release changelog files
20. Additional distribution channels (Snap, AUR, Scoop)
21. Blog post / announcement
22. YouTube demo video

---

## Agents & Verification

### Existing Agents

| Agent | Role in Release Process |
|-------|------------------------|
| `a9s-tui-reviewer` | Review code for BT v2 correctness before release |
| `a9s-security-auditor` | Verify read-only API usage, no hardcoded secrets |
| `a9s-release-validator` | Validate GoReleaser config, verify builds |
| `a9s-docs-reviewer` | Verify README matches code, services list matches fetchers |
| `test-coverage-analyzer` | Analyze test coverage gaps before release |

### Suggested New Agents

| Agent | Purpose |
|-------|---------|
| `a9s-license-checker` | Verify all dependencies have compatible licenses (GPL-3.0 compatibility) |

### Verification Checklist (pre-release)

Run all of these before tagging:
```
go build ./...
go test ./tests/unit/ -count=1 -timeout 120s
golangci-lint run ./...
govulncheck ./...
goreleaser check
goreleaser release --snapshot --clean
make verify-readonly
```

---

## k9s Comparison

| Aspect | k9s | a9s (current) | a9s (target) |
|--------|-----|---------------|--------------|
| Stars | 33k | 0 | N/A |
| CI linters | 25+ | 9 | 9 (sufficient) |
| Architectures | 5 (linux) + 2 (darwin) + 2 (windows) + 2 (freebsd) | 2+2+2 | Same (add arm/v7 later) |
| Distribution | 15+ methods | Homebrew + Go + Docker + binary | Add Snap, Scoop over time |
| Signing | None | Cosign (configured) | Cosign keyless |
| SBOM | Yes | Yes (configured) | Yes |
| Website | Hugo at k9scli.io | Stub at GH Pages | Hugo site |
| Demo | YouTube + asciinema + PNGs | Nothing | VHS GIF |
| Coverage reporting | None | Codecov (configured) | Codecov |
| Security scanning | None | CodeQL + govulncheck | Same (ahead of k9s) |
| Branch protection | None (!) | Yes | Yes |
| Code of Conduct | None | Yes | Yes |
| SECURITY.md | None | Yes | Yes |

**Key takeaway:** k9s has minimal CI/infrastructure for its size. a9s is already ahead on security scanning, branch protection, and community files. Main gaps are demo content and website.
