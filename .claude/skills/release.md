---
name: release
description: Automate the a9s release process — run checks, write release notes, tag, and push to trigger GoReleaser
---

## Pre-Release Checks

Run all of these and stop if any fail:

1. `go test ./tests/unit/ -count=1 -timeout 120s` — all tests must pass
2. `golangci-lint run ./...` — no lint errors
3. `govulncheck ./...` — no known vulnerabilities
4. Run `a9s-consistency-checker` agent — verify code/docs/website alignment
5. Run `test-coverage-analyzer` agent — check for coverage gaps
6. Run `a9s-architect` agent — verify architecture (target: 8.5+/10)
7. `goreleaser check -f .goreleaser.yaml` — validate release config
8. `go build -o a9s ./cmd/a9s/` — rebuild binary

**Exception**: Docs-only changes (*.md, docs/, website/, specs/, .claude/, LICENSE) do NOT require steps 4-6.

## Determine Version

Ask the user: "What type of release is this?"
- **Major** (breaking changes): bump X in vX.Y.Z
- **Minor** (new features/resources): bump Y in vX.Y.Z
- **Patch** (bug fixes): bump Z in vX.Y.Z

Get the latest tag: `git describe --tags --abbrev=0`
Calculate the next version based on user's choice.

## Write Release Notes

Create `releases/vX.Y.Z.md` with a curated changelog:

```markdown
## a9s vX.Y.Z

### Highlights (for minor/major) or ### Fixes (for patch)

- **Feature/fix name** -- description
- ...

### Install

\```sh
brew install k2m30/a9s/a9s
# or
go install github.com/k2m30/a9s/v3/cmd/a9s@vX.Y.Z
# or
docker run --rm -it ghcr.io/k2m30/a9s:vX.Y.Z --demo
\```
```

Use `git log $(git describe --tags --abbrev=0)..HEAD --oneline` as input but write human-readable descriptions, not raw commits.

The release workflow reads this file and passes it to GoReleaser as the `RELEASE_NOTES` env var.

## Create Release

1. Commit all changes including the release notes file
2. Push to main: `git push origin main`
3. Create annotated tag: `git tag -a vX.Y.Z -m "vX.Y.Z: short description"`
4. Push the tag: `git push origin vX.Y.Z`
5. Monitor the release workflow: `gh run watch` on the Release workflow
6. If GoReleaser fails with shutdown signal: root cause is OOM during cross-compilation. The `--parallelism 1` flag should prevent this. If it still happens, re-run the failed job.

## Post-Release Verification

1. `gh release view vX.Y.Z` — verify release exists with assets and notes
2. `brew uninstall a9s && brew untap k2m30/a9s && brew install k2m30/a9s/a9s` — verify Homebrew
3. `a9s --version` — verify installed version
4. `docker pull ghcr.io/k2m30/a9s:vX.Y.Z` — verify Docker image
5. `docker pull ghcr.io/k2m30/a9s:vX` — verify semver major tag
6. `docker pull ghcr.io/k2m30/a9s:vX.Y` — verify semver minor tag
7. `GOBIN=/tmp/gobin go install github.com/k2m30/a9s/v3/cmd/a9s@vX.Y.Z && /tmp/gobin/a9s --version` — verify go install
