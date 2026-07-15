# Licensing Notes (maintainer)

How the dual-licensing setup works and the rules that keep it workable.

## Structure

- **Whole repository**: GPL-3.0-or-later ([LICENSE](../LICENSE)).
- **`core/`**: additionally offered under a commercial license by the
  copyright holder ([COMMERCIAL-LICENSE.md](../COMMERCIAL-LICENSE.md)).
  SPDX headers: `GPL-3.0-or-later OR LicenseRef-Commercial`.
- **`internal/tui/`**: GPL-3.0-or-later only. SPDX headers:
  `GPL-3.0-or-later`.
- Derived paid applications (desktop/mobile) live in a separate private
  module that imports `github.com/k2m30/a9s/v3/core/...` and ship under the
  commercial license.

## Hard rules

1. **Never ship a GPL binary to the Apple App Store or Google Play Store.**
   App-store terms impose restrictions incompatible with the GPL (the
   FSF's long-standing position since the 2010 GNU Go takedown). The paid
   apps ship under the commercial license ONLY — no GPL code that the
   copyright holder cannot relicense may be linked into them.
2. **Dual licensing requires 100% relicensable copyright over `core/`.**
   Every line under `core/` is either authored by the copyright holder or
   contributed under the CLA ([CONTRIBUTING.md](../CONTRIBUTING.md)).
   Un-CLA'd PRs touching `core/` are never merged.
3. **Dependency hygiene.** All `core/` dependencies must stay permissive
   (Apache-2.0 / BSD / MIT). A GPL or AGPL dependency would infect the
   commercial binary. Re-audit with
   `go-licenses report ./core/...` after any dependency change.
   Audit status at extraction time (2026-07-15): all third-party deps are
   Apache-2.0, BSD-3-Clause, or MIT — clean.
4. **The TUI stays GPL.** `internal/tui/` is not offered commercially, so
   outside contributions to the TUI need no CLA and the k9s-style community
   contract is preserved.

## Release-consumption loop for the private apps repo

```text
require github.com/k2m30/a9s/v3 vX.Y.Z   // pin a published tag
replace github.com/k2m30/a9s/v3 => ../a9s // local dev only — drop for release
```

Bump the pinned tag to pick up new core features shipped with TUI releases.
