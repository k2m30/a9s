---
name: release
description: Automate the a9s release process — run checks, update changelog, tag, and push to trigger GoReleaser
---

## Pre-Release Checks

Run all of these and stop if any fail:

1. `make test` — all tests must pass
2. `make lint` — no lint errors (skip if golangci-lint not installed locally)
3. `make security` — no known vulnerabilities (skip if govulncheck not installed)
4. `make verify-readonly` — confirm no write API calls
5. `goreleaser check -f .goreleaser.yaml` — validate release config

## Determine Version

Ask the user: "What type of release is this?"
- **Major** (breaking changes): bump X in vX.Y.Z
- **Minor** (new features/resources): bump Y in vX.Y.Z
- **Patch** (bug fixes): bump Z in vX.Y.Z

Get the latest tag: `git describe --tags --abbrev=0`
Calculate the next version based on user's choice.

## Update CHANGELOG

1. Read CHANGELOG.md
2. Get commits since last tag: `git log $(git describe --tags --abbrev=0)..HEAD --oneline`
3. Categorize commits into Added/Changed/Fixed/Removed based on conventional commit prefixes
4. Replace the [Unreleased] section with the new version and today's date
5. Add a new empty [Unreleased] section at the top
6. Update comparison links at the bottom

## Create Release

1. Stage and commit CHANGELOG.md: `git commit -m "chore: release vX.Y.Z"`
2. Create a feature branch, push, and create a PR
3. After PR is merged, pull main and create annotated tag: `git tag -a vX.Y.Z -m "vX.Y.Z"`
4. Push the tag: `git push origin vX.Y.Z`
5. Monitor the release workflow: `gh run watch` on the Release workflow
6. Verify release artifacts: `gh release view vX.Y.Z`
7. Verify Homebrew formula updated: `gh api repos/k2m30/homebrew-a9s/contents/Formula`

## Post-Release

1. Report: version, number of artifacts, platforms, package formats
2. Suggest: announce in GitHub Discussions
