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
- **Describe API (if any)**: `ListPackages(maxResults=1)` per repo for the Wave 2 "unused-repo" signal; `GetRepositoryPermissionsPolicy` per repo for the Wave 2 public-policy signal; `DescribeDomain` for the `kms` pivot. `DescribeRepository` is Wave 3 (out of scope) per `docs/attention-signals.md § Not yet implemented`.

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

**Source API**: [ListPackages](https://docs.aws.amazon.com/codeartifact/latest/APIReference/API_ListPackages.html)

Transcribed from `docs/attention-signals.md § Signals § CI/CD` row `codeartifact`.

### 3.1 Wave 1 — zero extra API calls

- No Wave 1 signals — the list API does not return fields usable for attention. `ListRepositories` returns only configuration (`Name`, `Arn`, `DomainName`, `DomainOwner`, `AdministratorAccount`, `CreatedTime`, `Description`); nothing indicates health, staleness, or package contents — `AWS SDK Go v2 — codeartifact/types.RepositorySummary`. Every finding on `docs/attention-signals.md § Signals § CI/CD` row `codeartifact` comes from a later read.

### 3.2 Wave 2 — bounded extra API calls

One bullet per distinct signal.

`ListPackages(repository=Name, domain=DomainName, domainOwner=DomainOwner,
maxResults=1)` runs once per repository and fills the package count the detail
view shows. It raises no finding: an empty registry is not reported (see §5).

- **Signal**: the repository permissions policy has an Allow statement that grants to any principal with no restrictive condition → **`public access policy`**. The policy is parsed and evaluated, not string-matched, so a wildcard principal written any legal way is caught and one scoped by a condition is not — `core/iampolicy/evaluate.go`.
  - **State bucket**: Broken.
  - **API call**: `GetRepositoryPermissionsPolicy` — one call per repository. Implemented: `core/aws/codeartifact_issue_enrichment.go:100-132`.
  - **Cost shape**: per-resource.
  - **Why**: a publicly readable/writable CodeArtifact repository is a real supply-chain exposure (dependency-confusion and package-poisoning surface) that operators must see — a9s-devops (2026-07-05): possible=yes, worth=yes.

- **Signal**: repository has no permissions policy at all → **`no permissions policy`**.
  - **State bucket**: Warning.
  - **API call**: same `GetRepositoryPermissionsPolicy` call as above (a policy-not-found response yields this finding); no added cost.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

From `docs/attention-signals.md § Not yet implemented`; the `GetRepositoryPermissionsPolicy` analysis originally listed there is now an implemented Wave 2 signal (see §3.2).

- OUT OF SCOPE: `DescribeRepository` encryption check.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `codeartifact`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| policy grants to any principal | 2 | Broken | `!` | S1, S2, S3, S4, S5 | `public access policy` |
| no permissions policy | 2 | Warning | `~` | S2, S3, S4, S5 | `no permissions policy` |

Rationale for severity: a repository anyone can reach is a live supply-chain exposure, so it reads Broken and bumps the menu count. A repository with no policy at all is open to its whole domain but no further, which is worth knowing on a review pass rather than at 3am, so it reads Warning.

Note: both findings colour the row, and each carries its tier in the detail view's Attention section — `!` for the public policy, `~` for the missing one.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — a red `my-npm-repo` row reading `public access policy` says the repository is reachable by anyone, and a yellow one reading `no permissions policy` says it is open to the domain; both name the exposure in the Status column. All problem rows are self-explanatory in the list — operator can triage without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above): `DescribeRepository` encryption check.
- Empty repository older than 30 days (unused registry) — a9s reads the package count but reports nothing for it; the condition is recorded on `docs/attention-signals.md § Not yet implemented`.
- CodeArtifact-to-ACM, CodeArtifact-to-Kinesis, CodeArtifact-to-Lambda, CodeArtifact-to-Logs, CodeArtifact-to-R53, CodeArtifact-to-WAF pivots — deliberately excluded in `docs/related-resources.md § Deliberate exclusions` (no direct AWS API integration exists for any of these paths).
- CodeArtifact-to-CodeBuild and CodeArtifact-to-IAM-Role pivots — excluded as "heuristic-only / indirect" in `docs/related-resources.md § Deliberate exclusions`.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md § What is a9s?`).

## 6. Citations

- Display name `CodeArtifact Repos` — `docs/attention-signals.md § Signals § CI/CD` row `codeartifact`.
- AWS API reference URL — `docs/related-resources.md § Per-type contract` (codeartifact row).
- List API `ListRepositories` is config-only — `core/aws/codeartifact.go`.
- `RepositorySummary` shape (fields returned by `ListRepositories`) — `AWS SDK Go v2 — codeartifact/types.RepositorySummary § Name, Arn, DomainName, DomainOwner, AdministratorAccount, CreatedTime, Description`.
- Wave 2 signal `empty repo with age >30d → Warning (unused)` — `docs/attention-signals.md § Signals § CI/CD` row `codeartifact`.
- `ListPackages(maxResults=1)` as the per-repo call — `core/aws/codeartifact_issue_enrichment.go`; [ListPackages](https://docs.aws.amazon.com/codeartifact/latest/APIReference/API_ListPackages.html).
- `CreatedTime` field used for age computation — `AWS SDK Go v2 — codeartifact/types.RepositorySummary § CreatedTime`.
- `PackageSummary` shape (emptiness check via `ListPackages` response) — `AWS SDK Go v2 — codeartifact/types.PackageSummary`.
- Deferred item (`DescribeRepository` encryption check) — `docs/attention-signals.md § Not yet implemented`. The `GetRepositoryPermissionsPolicy` analysis originally listed there shipped as a Wave 2 signal and was moved to §3.2 during this amendment.
- Wave 2 public-policy signal — the repository permissions policy is parsed and its Allow statements evaluated for a wildcard principal without a restrictive condition (`core/iampolicy/evaluate.go`); a public verdict raises `!` "public access policy" and a missing policy raises `~` "no permissions policy" — `core/aws/codeartifact_issue_enrichment.go`, one `GetRepositoryPermissionsPolicy` call per repo; phrase and severity on `core/aws/catalog_cicd.go`. a9s-devops (2026-07-05): possible=yes, worth=yes. A public CodeArtifact repository is a live supply-chain exposure (dependency confusion, package poisoning); operators doing an access review must see it without leaving the list.
- Expected related targets (`ct-events`, `kms`) — `docs/related-resources.md § Per-type contract` and `docs/related-resources.md § codeartifact`.
- `kms` pivot field citation (domain-level, not repo-level) — `AWS SDK Go v2 — codeartifact/types.DomainDescription § EncryptionKey`; `codeartifact/types.RepositorySummary § DomainName, DomainOwner` provides the lookup keys for `DescribeDomain`. The earlier wording "Repo EncryptionKey" in `docs/related-resources.md § codeartifact` was factually wrong (no such field exists on the Repository shape) and was amended during this spec generation — a9s-devops (2026-04-20): possible=yes, worth=yes; rationale — CodeArtifact encryption is domain-scoped; pivoting from repo to KMS requires a one-hop `DescribeDomain` call, which is cheap (cacheable per domain) and directly serves the "who depends on this CMK?" workflow during key rotation / access-audit reviews.
- `DescribeRepository` deferred — `docs/attention-signals.md § Not yet implemented`.
- `ct-events` as universal pivot — `docs/related-resources.md § Policy #4`.
- `ct-events` discovery via `RepositorySummary.Arn` — universal convention in `docs/related-resources.md § Policy #4` (ct-events `resources[].ARN` match).
- Deliberate exclusions (`codeartifact` → `acm`, `kinesis`, `lambda`, `logs`, `r53`, `waf`, `cb`, `role`) — `docs/related-resources.md § Deliberate exclusions`.
- Read-only invariant — `docs/architecture.md § What is a9s?`.
- Severity choice `~` for unused-repo finding (informational, not urgent) — a9s-devops (2026-04-20): possible=yes, worth=yes; an empty registry is a housekeeping concern discovered during quarterly clean-up, not incident-time breakage. `~` keeps it out of `issues:N` while still carrying its tier in the detail view. Aligns with analogous informational-background-check findings (e.g. RDS maintenance scheduled, EBS snapshot aging) in other specs.
- List text (S4) wording `empty, created 47d ago` — a9s-devops (2026-04-20): possible=yes, worth=yes; pairs the condition (`empty`) with the cause (`47d ago`) per the "state keywords are not explanations" rule; ≤40 chars. The `47d` digits are illustrative — production implementation computes the actual age from `CreatedTime`.
- S5 sentence wording (declared on the finding definition, rendered in the Findings table below) `No packages published since repository was created 47 days ago — consider removing if unused.` — a9s-devops (2026-04-20): possible=yes, worth=yes; plain-English operator sentence with a next-step hint. Avoids all banned jargon (no `Wave`, no `enrichment`, no `finding`, no `bucket`).

<!-- BEGIN GENERATED: header -->
codeartifact — CI/CD. Lifecycle key: none (the list API returns no lifecycle field).
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| codeartifact.no-permissions-policy | no permissions policy | warn | wave2 | — |
| codeartifact.public-access-policy | public access policy | broken | wave2 | The repository's resource policy grants a wildcard principal, so any AWS account can read the packages it holds and, depending on the actions allowed, publish into it. Replace the "*" principal with the accounts or roles that need the repository, or add a condition that requires the caller's account or ARN to equal one you expect; a condition that only says whether a key is set scopes nothing. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| kms | KMS Key | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
