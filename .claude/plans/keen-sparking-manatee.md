# Local Release Scripts

## Context

CI workflows can't be easily debugged or run locally. Need a local script to run GoReleaser snapshot, Docker multi-arch build, and GitHub release — all from the local machine. Gitignored so it doesn't ship.

## Plan

### 1. Create `scripts/release-local.sh`

Single script with subcommands: `snapshot`, `docker`, `release`.

```bash
scripts/release-local.sh snapshot   # goreleaser --snapshot --clean --skip=publish
scripts/release-local.sh docker     # build + tag multi-arch Docker image locally
scripts/release-local.sh release    # full: goreleaser release --clean (publishes to GitHub + brew tap)
scripts/release-local.sh all        # snapshot + docker
```

**snapshot** — Runs `goreleaser release --snapshot --clean --skip=publish`. Validates config, builds all 4 binaries, generates checksums and Formula. Output in `dist/`.

**docker** — Builds linux/amd64 and linux/arm64 binaries, then `docker buildx build --platform linux/amd64,linux/arm64` with local tag. Optionally pushes to ghcr.io if `--push` flag given.

**release** — Runs `goreleaser release --clean` with `GITHUB_TOKEN` and `HOMEBREW_TAP_TOKEN` from environment. This is the real deal — publishes to GitHub Releases and updates the brew tap.

**all** — Runs snapshot + docker (no publish).

### 2. Update `.gitignore`

Add `scripts/` to the gitignore under "Internal tooling".

### Files to modify

| File | Change |
|------|--------|
| `scripts/release-local.sh` | New file — local release runner |
| `.gitignore` | Add `scripts/` |

### Verification

1. `scripts/release-local.sh snapshot` — builds all 4 archives + Formula in `dist/`
2. `scripts/release-local.sh docker` — builds multi-arch image tagged locally
3. Script is not tracked by git after `.gitignore` update
