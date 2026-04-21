---
shortName: waf
name: WAF Web ACLs
awsApiRef: https://docs.aws.amazon.com/waf/latest/APIReference/API_WebACL.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# waf — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `waf`
- **Display name**: WAF Web ACLs
- **AWS API reference**: https://docs.aws.amazon.com/waf/latest/APIReference/API_WebACL.html
- **List API**: `ListWebACLs` (returns `WebACLSummary` — config-only; no health fields)
- **Describe API (if any)**: `GetWebACL` (per ACL; returns `WebACL` with `Rules`, `DefaultAction`, etc.)

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `alarm`, `apigw`, `cf`, `ct-events`, `elb`, `logs`.

### `alarm`

- **Why related**: Blocked-request alarms — CloudWatch alarms tracking WAF `BlockedRequests` / `AllowedRequests` on this Web ACL are the primary runtime observability signal for a WAF deployment (docs/related-resources.md §`waf`).
- **How discovered**: cross-reference the already-loaded `alarm` list by `Dimensions` — CloudWatch alarms with `Namespace=AWS/WAFV2` and a `WebACL` dimension matching this ACL's `Name` (and optionally `Region` dimension) — a9s-devops: reverse-scan of the alarms list is the operator's established pattern for binding alarms to their protected resource, with no extra API call required.
- **Count shown**: yes.

### `apigw`

- **Why related**: API Gateways with this Web ACL attached (docs/related-resources.md §`waf`).
- **How discovered**: call `ListResourcesForWebACL` with `ResourceType=API_GATEWAY` per Web ACL (Regional scope only) — a9s-devops: this is the only first-class AWS API that enumerates protected resources from the WAF side; list is bounded and cheap per ACL.
- **Count shown**: yes.

### `cf`

- **Why related**: CloudFront distributions with this Web ACL attached (docs/related-resources.md §`waf`).
- **How discovered**: call `ListResourcesForWebACL` with `ResourceType=CLOUDFRONT` per Web ACL (only for ACLs with `Scope=CLOUDFRONT`, i.e. us-east-1 global) — a9s-devops: CloudFront WAF binding is surfaced here and equivalently on `Distribution.WebACLId`; the WAF-side call keeps the panel deterministic per ACL.
- **Count shown**: yes.

### `elb`

- **Why related**: Application Load Balancers with this Web ACL attached (docs/related-resources.md §`waf`).
- **How discovered**: call `ListResourcesForWebACL` with `ResourceType=APPLICATION_LOAD_BALANCER` per Web ACL (Regional scope) — a9s-devops: the only direct WAF→ALB enumeration; returns ALB ARNs that match entries in the already-loaded `elb` list.
- **Count shown**: yes.

### `logs`

- **Why related**: Logging configuration → CloudWatch Logs destination (docs/related-resources.md §`waf`).
- **How discovered**: call `GetLoggingConfiguration` per Web ACL — `LoggingConfiguration.LogDestinationConfigs[]` contains ARNs; filter for ARNs beginning with `arn:aws:logs:` (CloudWatch Logs destinations) and match against the already-loaded `logs` list — a9s-devops: WAF also supports Kinesis Firehose and S3 destinations, but only the CW Logs variants bind to the `logs` panel; others are not the target's scope.
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for ACL rule changes (docs/related-resources.md §`waf`).
- **How discovered**: universal pivot — applies to every registered type; see related-resources.md §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention. `ListWebACLs` returns `WebACLSummary` (ARN, Name, Id, Description, LockToken) which is config-only and exposes no health, state, or rule data.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `Rules==[]` (no-op ACL).
  - **State bucket**: Healthy (background finding; row stays green — no runtime health signal available).
  - **API call**: `GetWebACL` — one call per resource.
  - **Cost shape**: per-resource.
- **Signal**: `DefaultAction==Allow` with zero rules (allow-all, no protection).
  - **State bucket**: Healthy (background finding; row stays green — no runtime health signal available).
  - **API call**: `GetWebACL` — one call per resource.
  - **Cost shape**: per-resource.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: `ListResourcesForWebACL` per ACL (as a Wave 3 coverage check — distinct from its Wave-independent use as the related-panel discovery path in §2).
- OUT OF SCOPE: CloudWatch `BlockedRequests` spike detection.
- OUT OF SCOPE: managed-rule-group version drift check.

## 4. Issue Visualization

Every signal from §3.1 and §3.2 must land on one or more of these five existing surfaces. No other UI is allowed.

| # | Surface | Mechanism |
|---|---|---|
| S1 | Menu `issues:N` count | Aggregated count of `!`-severity findings. `~` findings do not bump. |
| S2 | Row color (list view) | Row colored by state bucket — Healthy=green, Warning=yellow, Broken=red, Dim=gray. Yellow/red/dim are themselves the attention signal. |
| S3 | `!` / `~` glyph before the name | Annotates a Healthy (green) row with "no immediate action, but worth knowing" — e.g. maintenance scheduled, certificate expiring soon. `!` = important background concern, `~` = informational. **Never appears on yellow/red/dim rows.** |
| S4 | Status / description column text | Short human-readable cause (e.g. `stopping: Server.SpotInstanceShutdown`, `expires in 7d`). **Healthy rows render blank** — no `OK` / `available` / `ACTIVE` / `running`. Empty means "nothing to see." |
| S5 | Detail view enrichment line | Short operator-readable sentence rendered inline in the detail view. No ceremonial header. |

Wave → surface mapping:

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank.
- **Wave 2 background finding on a Healthy row, important** (`DefaultAction==Allow` + zero rules) → `!` on green row → S1, S3, S4, S5.
- **Wave 2 background finding on a Healthy row, informational** (`Rules==[]`) → `~` on green row → S3, S4, S5. No S1.

One row per signal from §3 that has operator-readable surface text:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `Rules==[]` (no-op ACL) | 2 | Healthy | `~` | S3, S4, S5 | `no rules — ACL inert` | `Web ACL has no rules; requests pass through unfiltered.` |
| `DefaultAction==Allow` + zero rules | 2 | Healthy | `!` | S1, S3, S4, S5 | `allow-all: no rules, default Allow` | `Default action is Allow and no rules are configured — no protection in effect.` |

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes — both WAF findings carry the cause in the Status column (`no rules — ACL inert`, `allow-all: no rules, default Allow`), so a red/yellow-less green row with `!` or `~` prefix tells the operator exactly which configuration gap exists without opening detail.

## 5. Out of Scope

- All §3.3 Wave 3 signals (copied above).
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").

## 6. Citations

- Contract row and related targets — `docs/related-resources.md` § `waf` (targets: `alarm`, `apigw`, `cf`, `ct-events`, `elb`, `logs`).
- Wave 1 = None, Wave 2 signals — `docs/attention-signals.md` § Security & IAM row `waf`.
- Wave 3 signals — `docs/attention-signals.md` § Security & IAM row `waf` (`ListResourcesForWebACL`, `BlockedRequests` spike, managed-rule-group version drift).
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- `WebACL` shape (`Rules`, `DefaultAction`, `ARN`, `Id`, `Name`, `VisibilityConfig`) — `AWS SDK Go v2 — service/wafv2/types.WebACL`.
- `WebACLSummary` shape (list-response fields: `ARN`, `Description`, `Id`, `LockToken`, `Name`) — `AWS SDK Go v2 — service/wafv2/types.WebACLSummary`.
- `DefaultAction.Allow` / `DefaultAction.Block` discriminator — `AWS SDK Go v2 — service/wafv2/types.DefaultAction § Allow, Block`.
- `Rule` shape (`Name`, `Priority`, `Statement`, `Action`, `VisibilityConfig`) — `AWS SDK Go v2 — service/wafv2/types.Rule`.
- `alarm` discovery via CloudWatch alarm `Dimensions` with `Namespace=AWS/WAFV2` and `WebACL` dimension — a9s-devops (2026-04-20): possible=yes, worth=yes. CloudWatch's WAFV2 namespace emits `BlockedRequests`/`AllowedRequests` with a `WebACL` dimension on the ACL name; reverse-scan of the already-loaded alarm list matches the operator's pattern of binding alarms to their protected resource.
- `apigw` / `cf` / `elb` discovery via `ListResourcesForWebACL` per ACL (`ResourceType=API_GATEWAY` | `CLOUDFRONT` | `APPLICATION_LOAD_BALANCER`) — a9s-devops (2026-04-20): possible=yes, worth=yes. This is the only WAF-side API that enumerates protected resources; scope is Regional for APIGW/ALB and CLOUDFRONT for CF; cost is one call per ACL per resource type, bounded and cheap.
- `logs` discovery via `GetLoggingConfiguration` per ACL, filtering `LogDestinationConfigs[]` ARNs that begin with `arn:aws:logs:` — a9s-devops (2026-04-20): possible=yes, worth=yes. WAF logging also supports Kinesis Firehose and S3 sinks; only CW Logs destinations bind to the `logs` panel target.
- `~` severity for `Rules==[]` (empty ACL is a config hygiene concern but no active security regression — the ACL simply does nothing) and `!` severity for `DefaultAction==Allow` + zero rules (allow-all default with no rules is a real protection gap that warrants the menu count bump) — a9s-devops (2026-04-20): possible=yes, worth=yes. Severity split matches the attention-signals.md row (Warning vs Broken) and the S1-S5 mapping rules for Wave 2 background findings on a Healthy row.
- List text and detail text wording for both Wave 2 signals — generated per the output-template §4 rules (≤40 char S4, ≤100 char S5, no jargon, state + cause).
