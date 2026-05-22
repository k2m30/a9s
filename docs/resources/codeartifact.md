---
shortName: codeartifact
name: CodeArtifact Repos
awsApiRef: https://docs.aws.amazon.com/codeartifact/latest/APIReference/API_Repository.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# codeartifact — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `codeartifact`
- **Display name**: CodeArtifact Repos
- **AWS API reference**: <https://docs.aws.amazon.com/codeartifact/latest/APIReference/API_Repository.html>
- **List API**: `ListRepositories` (returns `RepositorySummary[]`).
- **Describe API (if any)**: `ListPackages(maxResults=1)` per repo for the Wave 2 "unused-repo" signal; `DescribeDomain` for the `kms` pivot. `DescribeRepository` is Wave 3 (out of scope) per `docs/attention-signals.md`.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `ct-events`, `kms`.

### `kms`

- **Why related**: CodeArtifact encrypts stored package assets with a KMS CMK configured at the *domain* level; operators doing key rotation or incident review need to see that this repository's domain depends on a given CMK. Source: `docs/related-resources.md § codeartifact` (amended).
- **How discovered**: Resolve the repo's `DomainName` + `DomainOwner` (from `RepositorySummary`), call `DescribeDomain`, read `DomainDescription.EncryptionKey` (a KMS key ARN). The Repository shape itself carries no encryption key — that field lives on the Domain. Citation: `AWS SDK Go v2 — codeartifact/types.DomainDescription § EncryptionKey` and `codeartifact/types.RepositorySummary § DomainName, DomainOwner`.
- **Count shown**: yes — one KMS key per repo (via its domain).

### `ct-events`

- **Why related**: Universal pivot — audit trail for repo policy/package events (who published, who changed permissions). Source: `docs/related-resources.md § codeartifact`.
- **How discovered**: Universal — CloudTrail `LookupEvents` filtered by the repo's ARN (`RepositorySummary.Arn`). No per-type discovery rule needed; the ct-events pivot is applied uniformly to every registered type via the policy in `docs/related-resources.md § Policy #4`.
- **Count shown**: yes (LookupEvents returns up to 50 events per call; the panel may display "50+" when paginated — general ct-events convention).

> Universal pivot — applies to every registered type; see `related-resources.md § Policy #4`.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

- No Wave 1 signals — the list API does not return fields usable for attention. `ListRepositories` returns only configuration (`Name`, `Arn`, `DomainName`, `DomainOwner`, `AdministratorAccount`, `CreatedTime`, `Description`); nothing indicates health, staleness, or package contents. Source: `docs/attention-signals.md § CI/CD` (codeartifact row, Wave 1 cell = `None — ListRepositories is config-only`); confirmed by `AWS SDK Go v2 — codeartifact/types.RepositorySummary`.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

- **Signal**: empty repository (no packages) with age >30 days → **Warning** (unused registry).
  - **State bucket**: Warning.
  - **API call**: `ListPackages(maxResults=1)` — one call per repository.
  - **Cost shape**: per-resource.
  - **How obtained**: `ListPackages(repository=Name, domain=DomainName, domainOwner=DomainOwner, maxResults=1)`; if the returned `packages[]` is empty AND `now - RepositorySummary.CreatedTime > 30d`, the repo is classified unused. Citations: `docs/attention-signals.md § CI/CD` (codeartifact row, Wave 2 cell); `AWS SDK Go v2 — codeartifact/types.PackageSummary` and `codeartifact/types.RepositorySummary § CreatedTime`.

### 3.3 Wave 3 — OUT OF SCOPE

Copied verbatim from `docs/attention-signals.md § CI/CD` (codeartifact row, Wave 3 cell).

- OUT OF SCOPE: `DescribeRepository` encryption check.
- OUT OF SCOPE: `GetRepositoryPermissionsPolicy` analysis.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. maintenance scheduled, unused registry. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `empty, created 47d ago`). **Healthy rows render blank** — no `OK`, no `available`. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank. Silence is the UX.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4 (short cause), S5 (full sentence).
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4 (short cause), S5 (full sentence). No S1.
- **Wave 2 finding on an already yellow/red/dim row** → redundant with color; S3 suppressed, S4 deduplicates with existing cause, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| empty repo, age >30d (unused registry) | 2 | Warning | `~` | S3, S4, S5 | `empty, created 47d ago` | `No packages published since repository was created 47 days ago — consider removing if unused.` |

Rationale for severity: an empty-but-configured registry is a housekeeping concern, not an outage — nothing is broken, the operator may simply have provisioned it ahead of an upcoming workload. `~` (informational) matches the "worth knowing, no immediate action" rule and keeps it out of the menu `issues:N` count so the count stays focused on real breakage. Classified per the attention-signals.md "Warning (unused)" label combined with S3/S4/S5 mapping for Healthy-row informational findings. — a9s-devops: possible=yes, worth=yes; an unused private registry is the kind of thing ops notices on a quarterly clean-up pass, not at 3am — informational severity is correct.

Note: `~` attaches only to Healthy (green) rows. `codeartifact` has no Wave 1 signals, so every repo's row starts green; the `~` glyph is therefore always applicable when the Wave 2 unused-repo finding fires.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — a `~ my-npm-repo` row with `empty, created 47d ago` in the Status column fully conveys the finding in place; the operator can ignore it during incident triage and revisit during the next clean-up, without ever opening detail. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): `DescribeRepository` encryption check, `GetRepositoryPermissionsPolicy` analysis.
- CodeArtifact-to-ACM, CodeArtifact-to-Kinesis, CodeArtifact-to-Lambda, CodeArtifact-to-Logs, CodeArtifact-to-R53, CodeArtifact-to-WAF pivots — deliberately excluded in `docs/related-resources.md § Deliberate exclusions` (no direct AWS API integration exists for any of these paths).
- CodeArtifact-to-CodeBuild and CodeArtifact-to-IAM-Role pivots — excluded as "heuristic-only / indirect" in `docs/related-resources.md § Deliberate exclusions`.
- `~` glyph on yellow/red/dim rows (not applicable here because `codeartifact` has no Wave 1 signals, but noted for completeness).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md § What is a9s?`).

## 6. Citations

- Display name `CodeArtifact Repos` — `docs/attention-signals.md § CI/CD` (codeartifact row, Name cell).
- AWS API reference URL — `docs/related-resources.md § Per-type contract` (codeartifact row).
- List API `ListRepositories` is config-only (no Wave 1 signals) — `docs/attention-signals.md § CI/CD` (codeartifact row, Wave 1 cell).
- `RepositorySummary` shape (fields returned by `ListRepositories`) — `AWS SDK Go v2 — codeartifact/types.RepositorySummary § Name, Arn, DomainName, DomainOwner, AdministratorAccount, CreatedTime, Description`.
- Wave 2 signal `empty repo with age >30d → Warning (unused)` — `docs/attention-signals.md § CI/CD` (codeartifact row, Wave 2 cell).
- `ListPackages(maxResults=1)` as the per-repo call — `docs/attention-signals.md § CI/CD` (codeartifact row, Wave 2 cell and Source cell: [ListPackages](https://docs.aws.amazon.com/codeartifact/latest/APIReference/API_ListPackages.html)).
- `CreatedTime` field used for age computation — `AWS SDK Go v2 — codeartifact/types.RepositorySummary § CreatedTime`.
- `PackageSummary` shape (emptiness check via `ListPackages` response) — `AWS SDK Go v2 — codeartifact/types.PackageSummary`.
- Wave 3 items (`DescribeRepository` encryption check; `GetRepositoryPermissionsPolicy` analysis) — `docs/attention-signals.md § CI/CD` (codeartifact row, Wave 3 cell).
- Expected related targets (`ct-events`, `kms`) — `docs/related-resources.md § Per-type contract` and `docs/related-resources.md § codeartifact`.
- `kms` pivot field citation (domain-level, not repo-level) — `AWS SDK Go v2 — codeartifact/types.DomainDescription § EncryptionKey`; `codeartifact/types.RepositorySummary § DomainName, DomainOwner` provides the lookup keys for `DescribeDomain`. The earlier wording "Repo EncryptionKey" in `docs/related-resources.md § codeartifact` was factually wrong (no such field exists on the Repository shape) and was amended during this spec generation — a9s-devops (2026-04-20): possible=yes, worth=yes; rationale — CodeArtifact encryption is domain-scoped; pivoting from repo to KMS requires a one-hop `DescribeDomain` call, which is cheap (cacheable per domain) and directly serves the "who depends on this CMK?" workflow during key rotation / access-audit reviews.
- `DescribeRepository` noted as Wave 3 — `docs/attention-signals.md § CI/CD` (codeartifact row, Wave 3 cell).
- `ct-events` as universal pivot — `docs/related-resources.md § Policy #4`.
- `ct-events` discovery via `RepositorySummary.Arn` — universal convention in `docs/related-resources.md § Policy #4` (ct-events `resources[].ARN` match).
- Deliberate exclusions (`codeartifact` → `acm`, `kinesis`, `lambda`, `logs`, `r53`, `waf`, `cb`, `role`) — `docs/related-resources.md § Deliberate exclusions`.
- Read-only invariant — `docs/architecture.md § What is a9s?`.
- Severity choice `~` for unused-repo finding (informational, not urgent) — a9s-devops (2026-04-20): possible=yes, worth=yes; an empty registry is a housekeeping concern discovered during quarterly clean-up, not incident-time breakage. `~` keeps it out of `issues:N` while still glyphing it on the list row. Aligns with analogous informational-background-check findings (e.g. RDS maintenance scheduled, EBS snapshot aging) in other specs.
- List text (S4) wording `empty, created 47d ago` — a9s-devops (2026-04-20): possible=yes, worth=yes; pairs the condition (`empty`) with the cause (`47d ago`) per the "state keywords are not explanations" rule; ≤40 chars. The `47d` digits are illustrative — production implementation computes the actual age from `CreatedTime`.
- Detail text (S5) wording `No packages published since repository was created 47 days ago — consider removing if unused.` — a9s-devops (2026-04-20): possible=yes, worth=yes; plain-English operator sentence with a next-step hint; ≤100 chars (exactly 99 with illustrative `47`). Avoids all banned jargon (no `Wave`, no `enrichment`, no `finding`, no `bucket`).

<!-- BEGIN GENERATED: header -->
codeartifact — CI/CD. Lifecycle key: `state`.
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Approximate? |
| --- | --- | --- |
| kms | KMS Key | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
