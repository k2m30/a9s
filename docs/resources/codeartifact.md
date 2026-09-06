---
shortName: codeartifact
name: CodeArtifact Repos
awsApiRef: https://docs.aws.amazon.com/codeartifact/latest/APIReference/API_Repository.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# codeartifact — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `codeartifact`
- **Display name**: CodeArtifact Repos
- **AWS API reference**: <https://docs.aws.amazon.com/codeartifact/latest/APIReference/API_Repository.html>
- **List API**: `ListRepositories` (returns `RepositorySummary[]`).
- **Describe API (if any)**: `ListPackages(maxResults=1)` per repo for the Wave 2 "unused-repo" signal; `GetRepositoryPermissionsPolicy` per repo for the Wave 2 public-policy signal; `DescribeDomain` for the `kms` pivot. `DescribeRepository` is Wave 3 (out of scope) per `docs/attention-signals.md`.

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

- **Signal**: repository permissions policy grants public access (`"Principal":"*"` in the policy document) → **`!` background concern** ("public access policy").
  - **State bucket**: Healthy + `!` background concern.
  - **API call**: `GetRepositoryPermissionsPolicy` — one call per repository. Implemented: `core/aws/codeartifact_issue_enrichment.go:100-132`.
  - **Cost shape**: per-resource.
  - **Why**: a publicly readable/writable CodeArtifact repository is a real supply-chain exposure (dependency-confusion and package-poisoning surface) that operators must see — a9s-devops (2026-07-05): possible=yes, worth=yes.

- **Signal**: repository has no permissions policy at all → informational (`~`, "no permissions policy").
  - **State bucket**: Healthy + `~` informational.
  - **API call**: same `GetRepositoryPermissionsPolicy` call as above (a policy-not-found response yields this finding); no added cost.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

From `docs/attention-signals.md § CI/CD` (codeartifact row, Wave 3 cell); the `GetRepositoryPermissionsPolicy` analysis originally listed there is now an implemented Wave 2 signal (see §3.2).

- OUT OF SCOPE: `DescribeRepository` encryption check.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count + list frame title `!N` suffix | Aggregated count of `!`-severity findings. `~` findings do not bump. The list frame title appends a space-separated `!N` after the count parentheses when the current list has N > 0 issues (`s3(50+) !5`, `ec2(17) !1`), or `!N+` when N is a truncated lower bound; N uses the same aggregation as the menu badge; the generated note under this table says which waves feed it for this type. No suffix when N = 0, and omitted in attention-only mode (`ctrl+z`) — the filtered count already is the issue count, so `name(5 of 50+) [!]` stays as-is. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. maintenance scheduled, unused registry. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `empty, created 47d ago`). **Healthy rows render blank** — no `OK`, no `available`. |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `codeartifact`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank. Silence is the UX.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4 (short cause), S5 (full sentence).
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4 (short cause), S5 (full sentence). No S1.
- **Wave 2 finding on an already yellow/red/dim row** → redundant with color; S3 suppressed, S4 deduplicates with existing cause, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| empty repo, age >30d (unused registry) | 2 | Warning | `~` | S3, S4, S5 | `empty, created 47d ago` |
| policy grants `"Principal":"*"` | 2 | Healthy | `!` | S1, S3, S4, S5 | `public access policy` |
| no permissions policy | 2 | Healthy | `~` | S3, S4, S5 | `no permissions policy` |

Rationale for severity: an empty-but-configured registry is a housekeeping concern, not an outage — nothing is broken, the operator may simply have provisioned it ahead of an upcoming workload. `~` (informational) matches the "worth knowing, no immediate action" rule and keeps it out of the menu `issues:N` count so the count stays focused on real breakage. Classified per the attention-signals.md "Warning (unused)" label combined with S3/S4/S5 mapping for Healthy-row informational findings. — a9s-devops: possible=yes, worth=yes; an unused private registry is the kind of thing ops notices on a quarterly clean-up pass, not at 3am — informational severity is correct.

Note: `~` attaches only to Healthy (green) rows. `codeartifact` has no Wave 1 signals, so every repo's row starts green; the `~` glyph is therefore always applicable when the Wave 2 unused-repo finding fires.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — a `~ my-npm-repo` row with `empty, created 47d ago` in the Status column fully conveys the finding in place; the operator can ignore it during incident triage and revisit during the next clean-up, without ever opening detail. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): `DescribeRepository` encryption check.
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
- Wave 3 item (`DescribeRepository` encryption check) — `docs/attention-signals.md § CI/CD` (codeartifact row, Wave 3 cell). The `GetRepositoryPermissionsPolicy` analysis originally listed there shipped as a Wave 2 signal and was moved to §3.2 during this amendment.
- Wave 2 public-policy signal (`"Principal":"*"` in the repository permissions policy → `!` "public access policy"; policy absent → `~` "no permissions policy") — implemented `core/aws/codeartifact_issue_enrichment.go:100-132`, one `GetRepositoryPermissionsPolicy` call per repo — a9s-devops (2026-07-05): possible=yes, worth=yes. A public CodeArtifact repository is a live supply-chain exposure (dependency confusion, package poisoning); operators doing an access review must see it without leaving the list.
- Expected related targets (`ct-events`, `kms`) — `docs/related-resources.md § Per-type contract` and `docs/related-resources.md § codeartifact`.
- `kms` pivot field citation (domain-level, not repo-level) — `AWS SDK Go v2 — codeartifact/types.DomainDescription § EncryptionKey`; `codeartifact/types.RepositorySummary § DomainName, DomainOwner` provides the lookup keys for `DescribeDomain`. The earlier wording "Repo EncryptionKey" in `docs/related-resources.md § codeartifact` was factually wrong (no such field exists on the Repository shape) and was amended during this spec generation — a9s-devops (2026-04-20): possible=yes, worth=yes; rationale — CodeArtifact encryption is domain-scoped; pivoting from repo to KMS requires a one-hop `DescribeDomain` call, which is cheap (cacheable per domain) and directly serves the "who depends on this CMK?" workflow during key rotation / access-audit reviews.
- `DescribeRepository` noted as Wave 3 — `docs/attention-signals.md § CI/CD` (codeartifact row, Wave 3 cell).
- `ct-events` as universal pivot — `docs/related-resources.md § Policy #4`.
- `ct-events` discovery via `RepositorySummary.Arn` — universal convention in `docs/related-resources.md § Policy #4` (ct-events `resources[].ARN` match).
- Deliberate exclusions (`codeartifact` → `acm`, `kinesis`, `lambda`, `logs`, `r53`, `waf`, `cb`, `role`) — `docs/related-resources.md § Deliberate exclusions`.
- Read-only invariant — `docs/architecture.md § What is a9s?`.
- Severity choice `~` for unused-repo finding (informational, not urgent) — a9s-devops (2026-04-20): possible=yes, worth=yes; an empty registry is a housekeeping concern discovered during quarterly clean-up, not incident-time breakage. `~` keeps it out of `issues:N` while still glyphing it on the list row. Aligns with analogous informational-background-check findings (e.g. RDS maintenance scheduled, EBS snapshot aging) in other specs.
- List text (S4) wording `empty, created 47d ago` — a9s-devops (2026-04-20): possible=yes, worth=yes; pairs the condition (`empty`) with the cause (`47d ago`) per the "state keywords are not explanations" rule; ≤40 chars. The `47d` digits are illustrative — production implementation computes the actual age from `CreatedTime`.
- Detail text (S5) wording `No packages published since repository was created 47 days ago — consider removing if unused.` — a9s-devops (2026-04-20): possible=yes, worth=yes; plain-English operator sentence with a next-step hint. Avoids all banned jargon (no `Wave`, no `enrichment`, no `finding`, no `bucket`).

<!-- BEGIN GENERATED: header -->
codeartifact — CI/CD. Lifecycle key: none (the list API returns no lifecycle field).
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| codeartifact.no-permissions-policy | no permissions policy | warn | wave2 | — |
| codeartifact.public-access-policy | public access policy | broken | wave2 | The repository's resource policy grants a wildcard principal, so any AWS account can read the packages it holds and, depending on the actions allowed, publish into it. Replace the "\*" principal with the accounts or roles that need the repository, or scope the grant with a condition. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| kms | KMS Key | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
