---
name: semver-version-bump
description: Bump app version on every code change following semver rules
type: feedback
---

Bump the app version in `cmd/a9s/main.go` (the `version` const) on every commit that changes code.

**Why:** User wants the binary version to reflect every change, not stay stuck at 0.1.0.

**How to apply:**
- PATCH (0.1.X): Bug fixes, test additions, cosmetic changes
- MINOR (0.X.0): New features, new resource types, new keybindings
- MAJOR (X.0.0): Breaking changes to CLI flags, config format, or behavior
- Update `const version = "X.Y.Z"` in `cmd/a9s/main.go` before committing
