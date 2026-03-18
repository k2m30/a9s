# Quickstart: Align Documentation With Implementation

## What This Feature Does

Updates all project documentation (specs, CLAUDE.md, design spec, agent definitions, QA stories) to accurately reflect the current implementation state. No code changes.

## How to Execute

1. **Update CLAUDE.md** — Fix project structure (`src/` → actual dirs), normalize Go version references
2. **Annotate spec 001 FRs** — Add inline status to each of 19 FRs based on research.md findings
3. **Annotate spec 002 FRs** — Add inline status to each of 16 FRs
4. **Annotate spec 003 FRs** — Add inline status to each of 19 FRs
5. **Update spec Status fields** — Draft → Complete or Partial
6. **Add Future Work sections** — For unimplemented FRs (breadcrumbs, history nav, etc.)
7. **Fix agent definitions** — tui-ux-auditor.md "src/" reference, a9s-integrator.md stale cleanup steps
8. **Spot-check QA stories** — Key bindings, S3 nav, detail view, copy, profile switching areas
9. **Verify design spec** — Key binding tables match actual implementation

## How to Verify

- Run `go build -o a9s ./cmd/a9s/` — still builds
- Run `go test ./tests/unit/ -count=1 -timeout 120s` — still passes
- Search for "src/" in CLAUDE.md — zero hits
- Search for uncontextualized "internal/app/" in all docs — zero hits
- Read each spec Status field — none say "Draft"
