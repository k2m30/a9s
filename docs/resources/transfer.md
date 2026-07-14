---
shortName: transfer
name: Transfer Family
awsApiRef: https://docs.aws.amazon.com/transfer/latest/userguide/API_DescribedServer.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
---

# transfer — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `transfer` (aliases: `sftp`, `as2`, `ftps`)
- **Display name**: Transfer Family
- **AWS API reference**: <https://docs.aws.amazon.com/transfer/latest/userguide/API_DescribedServer.html>
- **List API**: `ListServers` (paginated; `ListedServer` carries real Wave-1 data)
- **Describe API (if any)**: `DescribeServer` (per server, Wave 2 — the eks/mwaa in-fetcher N+1 pattern is NOT needed here since the list is informative; the describe augments)

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `acm`, `ct-events`, `eip`, `lambda`, `logs`, `role`, `subnet`, `vpc`, `vpce`.

### `eip`

- **Why related**: `EndpointDetails.AddressAllocationIds` — the static addresses of an internet-facing VPC endpoint; these are exactly what partners allowlist, so "which IPs is this server on" is one pivot away.
- **How discovered**: read field (present only when the VPC endpoint is internet-facing); allocation ids match the `eip` type's IDs directly.
- **Count shown**: yes.

### `acm`

- **Why related**: `DescribedServer.Certificate` — the FTPS server identity certificate (ACM ARN). AS2 certificates are transfer-managed, not ACM.
- **How discovered**: read field `Certificate` on the described server; pivot only when non-empty (FTPS servers).
- **Count shown**: yes.

### `lambda`

- **Why related**: `IdentityProviderDetails.Function` — the custom authorizer; "why is auth rejecting this user" jumps straight to it.
- **How discovered**: read field when `IdentityProviderType == AWS_LAMBDA`; pivot only when non-empty.
- **Count shown**: yes.

### `logs`

- **Why related**: `StructuredLogDestinations` (log-group ARNs) — where a failed-transfer investigation actually goes.
- **How discovered**: read field on the described server; convert ARNs to bare log-group names.
- **Count shown**: yes.

### `role`

- **Why related**: `LoggingRole` — first stop for "why are there no logs".
- **How discovered**: read field; extract bare role name from the ARN.
- **Count shown**: yes.

### `subnet`

- **Why related**: `EndpointDetails.SubnetIds` — endpoint ENIs live here; partner-reachability debugging.
- **How discovered**: read field (present when `EndpointType == VPC`).
- **Count shown**: yes.

### `vpc`

- **Why related**: `EndpointDetails.VpcId` — top of the reachability chain.
- **How discovered**: read field (present when `EndpointType == VPC`).
- **Count shown**: yes.

### `vpce`

- **Why related**: `EndpointDetails.VpcEndpointId` — the hop that carries the security groups (live-witnessed populated for the auto-created endpoint despite SDK doc wording).
- **How discovered**: read field (present when `EndpointType == VPC`).
- **Count shown**: yes.

### `ct-events`

- **Why related**: audit trail ("who stopped this server"). Universal pivot — applies to every registered type; see related-resources.md §Policy.
- **How discovered**: CloudTrail LookupEvents by ServerId.
- **Count shown**: yes.

Explicitly excluded (per `docs/related-resources.md` §`transfer`): `sg` (`EndpointDetails.SecurityGroupIds` documented-but-never-populated — reach via `vpce`), `apigw` (free-form IdP URL — copyable detail fact), `s3`/`efs` (`Domain` is an enum; the landing bucket lives in agreement `BaseDirectory`).

## 2.1 Child views

- **Agreements** (`e` on a server) — the only server-scoped child (`DescribedAgreement.ServerId`; `ListAgreements(ServerId)`). Columns: AgreementId, Description, Status, Local Profile, Partner Profile, Base Directory. Status `INACTIVE` → Warning row `inactive: partner traffic rejected`.
- **Profiles / certificates are deliberately NOT server children**: both are ACCOUNT-scoped (`ListedProfile`/`ListedCertificate` carry no ServerId) — hanging the same account-wide list under every server implies ownership the API doesn't model. The agreement detail resolves its `LocalProfileId`/`PartnerProfileId` to profile facts (`As2Id`) inline; certificate expiry surfaces there. NOTE: this diverges from the original product-goal wording ("agreements/profiles/certificates child views") on API-scoping grounds — flagged for the release owner; overridable.
- **Connectors** — no ServerId at all (outbound half of AS2, independent of servers). Candidate future top-level type (`transfer-connector`), not a child.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

- **Signal**: `State == ONLINE` → Healthy.
  - **State bucket**: Healthy.
  - **How obtained**: `ListedServer.State`.
- **Signal**: `State == OFFLINE` — the partner-facing endpoint is not accepting transfers (legitimate as a cost-stop, but partners can't connect).
  - **State bucket**: Warning.
  - **How obtained**: `ListedServer.State`.
- **Signal**: `State` in `STARTING` / `STOPPING` (transient).
  - **State bucket**: Warning.
  - **How obtained**: `ListedServer.State`.
- **Signal**: `State == START_FAILED` — server failed to come online; partners are down.
  - **State bucket**: Broken.
  - **How obtained**: `ListedServer.State`.
- **Signal**: `State == STOP_FAILED` — error condition, server likely still serving.
  - **State bucket**: Warning.
  - **How obtained**: `ListedServer.State`.

Deliberately not Wave-1 signals (a9s-devops 2026-07-14): `LoggingRole == nil` (structured logging makes nil legitimate — the gap check needs `StructuredLogDestinations`, a Describe field → Wave 2); `UserCount == 0` (AS2 and external-IdP servers legitimately 0 — column only). `Protocols` is absent from `ListedServer` — AS2-vs-SFTP is unknowable at Wave 1.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `SecurityPolicyName` in the known-weak legacy denylist (`TransferSecurityPolicy-2018-11`, `TransferSecurityPolicy-2020-06`) — weak ciphers / old TLS. Denylist, not latest-chasing: FIPS/PQ/restricted variants must not false-positive.
  - **State bucket**: Warning.
  - **API call**: `DescribeServer`, one per server (N+1; accounts run 1–5).
  - **Cost shape**: per-resource.
- **Signal**: `LoggingRole == nil` AND `len(StructuredLogDestinations) == 0` — no activity logging at all; an audit gap for a B2B endpoint.
  - **State bucket**: Warning.
  - **API call**: `DescribeServer` (same call).
  - **Cost shape**: per-resource.
- **Signal**: per-name `DescribeServer` denied — the row is KEPT name-only (`details denied`, shared DegradedDetailsDenied contract).
  - **State bucket**: Warning.
  - **API call**: `DescribeServer` (the denial IS the response).
  - **Cost shape**: per-resource.

Wave-2 detail facts (not signals): `Protocols`, `As2ServiceManagedEgressIpAddresses` (partners allowlist these — copyable), `HostKeyFingerprint` (SFTP clients pin it), `EndpointDetails` wiring, `IdentityProviderDetails.Url`.

Child-row signals: agreement `Status == INACTIVE` → Warning `inactive: partner traffic rejected`; certificate rows (on agreement detail): `InactiveDate` past or `Status == INACTIVE` → Broken `expired`, within 30 days → Warning `expires in <N>d`. Never bubbled account-wide to the server row.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `FilesIn`/`FilesOut`/`BytesIn`/`BytesOut`, AS2 `InboundMessage`/`OutboundMessage` metrics.
- OUT OF SCOPE: `ListExecutions` workflow run history.
- OUT OF SCOPE: per-user home directories / SSH keys (`ListUsers`/`DescribeUser`) — candidate later SFTP-focused pass.
- OUT OF SCOPE: AS2 message/MDN transfer history.

## 4. Issue Visualization

Surfaces S1–S5 per `docs/attention-signals.md` §Visualization Surfaces; wave→surface mapping as standard. Every signal is color-bearing (no glyph-on-green case exists for transfer).

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| `State == OFFLINE` | 1 | Warning | n/a | S2, S4 | `offline: not accepting transfers` | `Server is offline; partners cannot connect until it is started.` |
| `State == STARTING` | 1 | Warning | n/a | S2, S4 | `starting` | `Server is starting; not yet fully able to respond.` |
| `State == STOPPING` | 1 | Warning | n/a | S2, S4 | `stopping` | `Server is stopping; transfers are draining.` |
| `State == START_FAILED` | 1 | Broken | n/a | S2, S4 | `start failed` | `Server failed to come online; partner transfers are down.` |
| `State == STOP_FAILED` | 1 | Warning | n/a | S2, S4 | `stop failed` | `Stop failed; the server may still be serving transfers.` |
| legacy security policy | 2 | Warning | n/a | S2, S4, S5 | `legacy security policy` | `Security policy <name> allows weak ciphers / old TLS; move to a current policy.` |
| no activity logging | 2 | Warning | n/a | S2, S4, S5 | `no activity logging` | `Neither a logging role nor structured log destinations are configured.` |
| `DescribeServer` denied | 2 | Warning | n/a | S2, S4, S5 | `details denied` | `Access to server details was denied; only the listed fields are visible.` |

Notes:

- No raw AWS enum reaches a rendered surface (`START_FAILED` → `start failed`, etc.).
- Multiple findings stack with the framework `(+N)` suffix; S5 enumerates each.
- AccessDenied on `transfer:ListServers`: menu row shows the error state, never `0`.

## 4.1 UX review (two sentences)

Every problem row names its cause in the Status column (`offline: not accepting transfers`, `start failed`, `legacy security policy`), so the 3am operator triages without opening detail. The one drill-down that matters — which agreement/partner is affected — is one `e` keypress away on the agreements child view.

## 5. Out of Scope

- All §3.3 Wave 3 items.
- Profiles/certificates/connectors as server children (§2.1 — account-scoped; connectors have no server link at all).
- `sg`/`apigw`/`s3`/`efs` pivots (§2 exclusions with citations).
- Any UI element not in S1–S5; any write operation (`architecture.md` §"What is a9s?").

## 6. Citations

- Pivot set and exclusions — `docs/related-resources.md` § `transfer`; `AWS SDK Go v2 — transfer/types.DescribedServer § Certificate, § IdentityProviderDetails, § StructuredLogDestinations, § LoggingRole, § EndpointDetails`; sg exclusion per SDK doc on `EndpointDetails § SecurityGroupIds` ("not populated in DescribeServer responses").
- eip pivot + SubnetIds/AddressAllocationIds navigability — `user (2026-07-14, live acceptance testing): internet-facing server witnessed with EndpointDetails.AddressAllocationIds ×3 — the original AS2 witness had an internal endpoint, which hid this field`; `AWS SDK Go v2 — transfer/types.EndpointDetails § AddressAllocationIds`.
- Wave-1 State mapping — `docs/attention-signals.md` § Networking row `transfer`; `AWS SDK Go v2 — transfer/types.ListedServer § State`.
- LoggingRole-nil and UserCount-zero non-signals — `a9s-devops (2026-07-14): possible=yes, worth=no. Structured logging makes nil legitimate; AS2/external-IdP servers legitimately 0 users.`
- Security-policy denylist — `a9s-devops (2026-07-14): possible=yes, worth=yes. Denylist beats latest-chasing (FIPS/restricted variants).`
- Logging-gap Wave-2 signal — `a9s-devops (2026-07-14): possible=yes (both fields on DescribedServer), worth=yes. B2B audit gap.`
- Agreements-only child view — `AWS SDK Go v2 — transfer/types.DescribedAgreement § ServerId` (exists) vs `ListedProfile`/`ListedCertificate` (no ServerId) + `a9s-devops (2026-07-14)`; divergence from product-goal wording flagged in §2.1.
- Certificate expiry rules — `AWS SDK Go v2 — transfer/types.ListedCertificate § InactiveDate, § Status`.
- Degraded-row contract — shared `DegradedDetailsDenied` (fleet contract since v3.50.0).
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
