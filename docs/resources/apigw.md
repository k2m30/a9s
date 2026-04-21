---
shortName: apigw
name: API Gateways
awsApiRef: https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/enrichment-visibility.md
---

# apigw — Resource Spec

Golden UX/UI doc for this resource, written from the operator's perspective. Describes what the list row, Status column, glyphs, and detail view should look like — the should-be, not the is. Implementation conforms to this doc; tests assert against it. When code and this doc disagree, the code is wrong.

## 1. Identity

- **shortName**: `apigw`
- **Display name**: API Gateways
- **AWS API reference**: <https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis.html>
- **List API**: `apigatewayv2:GetApis` (plus REST v1 `apigateway:GetRestApis` for v1 APIs — see §1 note).
- **Describe API (if any)**: `apigatewayv2:GetStages` per HTTP/WebSocket API (Wave 2). REST v1 APIs are listed but not stage-enriched today (v1 `GetStages` not wired).

The list row identifies the API by `Name` and `ApiId`; protocol (`ProtocolType` = `HTTP` or `WEBSOCKET` for v2) is a useful disambiguator since one account can hold REST, HTTP, and WebSocket APIs with similar names.

## 2. Related Resources Panel (detail view, right column)

Expected targets from `docs/related-resources.md` Per-type contract: `acm`, `alarm`, `cf`, `ct-events`, `elb`, `kms`, `lambda`, `logs`, `r53`, `role`, `sfn`, `sns`, `vpce`, `waf`.

### `acm`

- **Why related**: Custom-domain TLS certificate — the cert that terminates TLS on the API's custom domain. If it expires the custom domain stops serving.
- **How discovered**: call `apigatewayv2:GetDomainNames` (account-wide) and `apigatewayv2:GetApiMappings` per domain; read `DomainNameConfigurations[].CertificateArn` and match API mappings back to this `ApiId` — a9s-devops: cert→custom-domain→api is the only AWS-exposed chain for v2.
- **Count shown**: yes.

### `alarm`

- **Why related**: Stage latency/error alarms watching this API.
- **How discovered**: reverse-scan the already-loaded `alarm` list for `Namespace=="AWS/ApiGateway"` with `Dimensions` containing `ApiName`/`ApiId` matching this API — a9s-devops: standard CloudWatch dimension convention for APIGW alarms, no extra API call needed when the alarm list is already loaded in the sweep.
- **Count shown**: yes.

### `cf`

- **Why related**: APIGW often fronted by CloudFront for edge caching, WAF, and a friendlier hostname.
- **How discovered**: reverse-scan the already-loaded `cf` list for `Origins.Items[].DomainName` ending in `execute-api.<region>.amazonaws.com` and matching this `ApiId` — a9s-devops: CloudFront origin hostname is the only place that reveals the upstream API without walking APIGW configs.
- **Count shown**: yes.

### `elb`

- **Why related**: VpcLink NLB backend — HTTP APIs use a VpcLink backed by a Network Load Balancer to reach private VPC services.
- **How discovered**: call `apigatewayv2:GetVpcLinks`, collect each VpcLink's associated NLB ARN/subnet set, then intersect with `apigatewayv2:GetIntegrations` per API whose `IntegrationType==VPC_LINK` and `ConnectionId` matches a VpcLink — a9s-devops: a VpcLink is API-scoped only via integrations, so the account-wide VpcLink list plus integrations per API is the chain.
- **Count shown**: yes.

### `kms`

- **Why related**: KMS key referenced by Lambda integrations. Golden doc notes this is a **weak pair**: API Gateway itself exposes no direct KMS field.
- **How discovered**: for each Lambda integration URI resolved via `apigatewayv2:GetIntegrations`, follow the already-loaded `lambda` list to read `FunctionConfiguration.KMSKeyArn` — a9s-devops: yes this is a transitive pivot; keep it because operators triaging "why is this API throwing 5xx" sometimes chase a KMS key that's `PendingDeletion` on the integration target. Low-value when the Lambda panel already shows the KMS link, but acceptable for this type's audience.
- **Count shown**: yes.

### `lambda`

- **Why related**: Lambda integrations — the most common APIGW backend.
- **How discovered**: `apigatewayv2:GetIntegrations` per API, parse `IntegrationUri` for Lambda function ARNs (`arn:aws:apigateway:...functions/arn:aws:lambda:...:function:<name>/invocations`) and match against the already-loaded `lambda` list — a9s-devops: `IntegrationType==AWS_PROXY` with a Lambda ARN in `IntegrationUri` is the canonical pattern.
- **Count shown**: yes.

### `logs`

- **Why related**: API access log destination — the log group that stores per-request access entries for the stage.
- **How discovered**: read `Stage.AccessLogSettings.DestinationArn` on each stage returned by `apigatewayv2:GetStages`; match to the already-loaded `logs` list by log-group ARN — a9s-devops: stage access logs are the single operator-visible log surface; execution logs live on a different per-method path and are rarely what on-call wants first.
- **Count shown**: yes.

### `r53`

- **Why related**: R53 alias records for the API's custom domains — the DNS surface operators hit in a browser.
- **How discovered**: for each custom domain from `apigatewayv2:GetDomainNames`, collect `DomainNameConfigurations[].ApiGatewayDomainName` (the regional or CloudFront-fronted target) and reverse-scan the already-loaded `r53` hosted-zone record sets for `AliasTarget.DNSName` matching — a9s-devops: alias-target matching is the only way to surface the DNS hop without walking every zone's records.
- **Count shown**: yes.

### `role`

- **Why related**: Invocation/authorizer role — the IAM role APIGW assumes to call the integration target or to run a request authorizer.
- **How discovered**: `apigatewayv2:GetIntegrations` per API → read `CredentialsArn`; `apigatewayv2:GetAuthorizers` per API → read `AuthorizerCredentialsArn`; match to already-loaded `role` list — a9s-devops: these are the two places APIGW records an assumed role; anything else (e.g. Lambda execution role) belongs under the Lambda pivot, not here.
- **Count shown**: yes.

### `sfn`

- **Why related**: Step Functions integration target — APIGW can start a state-machine execution directly.
- **How discovered**: `apigatewayv2:GetIntegrations` per API → `IntegrationUri` of the form `arn:aws:apigateway:<region>:states:action/StartExecution` paired with a request-template referencing a state-machine ARN, or direct `arn:aws:states:<region>:<acct>:stateMachine:<name>` in the URI; match to already-loaded `sfn` list — a9s-devops: AWS-service integrations via APIGW use this `:states:action/` ARN form.
- **Count shown**: yes.

### `sns`

- **Why related**: APIGW → SNS integration — publish a notification directly from an API request.
- **How discovered**: `apigatewayv2:GetIntegrations` per API → `IntegrationUri` of the form `arn:aws:apigateway:<region>:sns:action/Publish` (with topic ARN in request templates) or direct `arn:aws:sns:<region>:<acct>:<topic>`; match to already-loaded `sns` list — a9s-devops: identical pattern to sfn, different AWS-service slug.
- **Count shown**: yes.

### `vpce`

- **Why related**: Private APIs expose via VPC endpoint (interface type, `com.amazonaws.<region>.execute-api`).
- **How discovered**: for REST v1 APIs, read `RestApi.EndpointConfiguration.VpcEndpointIds` directly; for HTTP v2 APIs, parse the API's resource policy for `aws:SourceVpce` condition keys (requires `apigatewayv2:GetApi` or policy fetch) — a9s-devops: possible=yes for v1 via a first-class field, possible=yes for v2 only via policy parse which is brittle; surface the v1 path now and cite the v2 gap honestly.
- **Count shown**: yes for v1; unknown for v2 (policy-parse path).

### `waf`

- **Why related**: WebACL attached to the API stage — the ingress filter that blocks bots, SQLi, rate abuse.
- **How discovered**: call `wafv2:GetWebACLForResource` per stage ARN (WAFv2 regional scope); match returned `WebACLArn` to already-loaded `waf` list — a9s-devops: this is the only reverse lookup WAFv2 offers per resource ARN; the alternative (enumerate all ACLs and their resources) is O(N·M).
- **Count shown**: yes.

### `ct-events`

- **Why related**: Audit trail for API changes — who created/updated/deleted the API, stages, routes, integrations.
- **How discovered**: universal pivot — applies to every registered type; see related-resources.md §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

Transcribed from `docs/attention-signals.md`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention.

`apigatewayv2:GetApis` returns `Api` structs whose visible fields are `Name`, `ApiId`, `ApiEndpoint`, `ProtocolType`, `CreatedDate`, `Description`, `Version`, `Tags` — none of which carry a deployment/health state (confirmed from `AWS SDK Go v2 — apigatewayv2/types.Api`). Every row is Healthy at Wave 1; attention must come from Wave 2.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `apigatewayv2:GetStages` per HTTP/WebSocket API returns zero deployed stages (no stage with a `DeploymentId`).
  - **State bucket**: Warning.
  - **API call**: `apigatewayv2:GetStages` — one per v2 resource.
  - **Cost shape**: per-resource.

- **Signal**: REST v1 API — stage enrichment not wired. Surfaced as informational only, not as an issue.
  - **State bucket**: Healthy (not an issue, just a caveat for the operator that a9s doesn't currently enrich v1).
  - **API call**: none (deliberately skipped).
  - **Cost shape**: n/a.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `5XXError`/`4XXError`.
- OUT OF SCOPE: `GetUsagePlans` quota-breach detection.

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

- **Wave 1 Healthy** → no §4 row (omit). S2 renders green, S4 renders blank. Silence is the UX.
- **Wave 1 Warning / Broken / Dim** → S2 (color) + S4 (cause text). No S1, S3, S5.
- **Wave 2 background finding on a Healthy row, important** → `!` glyph on green row. S1, S3, S4 (short cause), S5 (full sentence).
- **Wave 2 background finding on a Healthy row, informational** → `~` glyph on green row. S3, S4 (short cause), S5 (full sentence). No S1.
- **Wave 2 finding on an already yellow/red/dim row** → redundant with color; S3 suppressed, S4 deduplicates with existing cause, S5 still carries the full sentence, S1 still counts if `!`.

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) | Detail text (S5) |
|---|---|---|---|---|---|---|
| No deployed stage (v2) | 2 | Warning (background finding on Healthy row) | `~` | S3, S4, S5 | `no deployed stage` | `API has no deployed stage — undeployed or orphaned; deploy a stage or delete the API.` |

Notes on the single row above:

- Severity is `~` (informational), not `!` — an undeployed API costs nothing and breaks nothing external; it's a cleanup/audit concern, not a page-the-on-call concern. The operator benefits from seeing the hint without the `issues:N` count in the menu climbing. If the team later decides undeployed APIs are a real problem (e.g. governance rule), promote to `!` in a follow-up — the spec change is the contract, not a code change.
- REST v1 APIs render entirely blank in S4 because a9s does not currently enrich v1 stages. This is an intentional omission documented in §3.2. The operator will see v1 and v2 mixed in the list and — critically — **no silent red rows** appear for v1 simply because stage enrichment is off. Silence is the correct UX when the tool has nothing factual to say.

## 4.1 UX review (two sentences)

At 3am, glancing at the list, can the operator tell what's wrong with a problem row without opening detail? Yes for v2 — a `~` glyph with `no deployed stage` in the Status column is self-explanatory — and yes for v1 by design, because v1 rows carry no enrichment today and therefore never surface a false alarm; the cost is that a genuinely broken v1 API won't surface either, which is a known gap the operator learns from the tool's docs, not from a surprise.

## 5. Out of Scope

- All §3.3 Wave 3 signals (CloudWatch `5XXError`/`4XXError`; `GetUsagePlans` quota-breach detection).
- REST v1 stage enrichment — not currently wired; documented as a known caveat rather than a TBD.
- Any UI element not listed in §4 — e.g. new columns, new icons, new views, new key bindings.
- Any write operation. a9s is read-only by design (`architecture.md` §"What is a9s?").
- `vpce` discovery for HTTP v2 APIs via resource-policy parse — a9s-devops: possible=yes but brittle (string match on a JSON policy document), worth=no today. v1 `EndpointConfiguration.VpcEndpointIds` is the supported path; v2 remains a gap until a cleaner API field is added by AWS.

## 6. Citations

- `apigw` shortName + contract row — `docs/related-resources.md` § `apigw` (table row and "Per-target reasoning" subsection).
- Related targets (`acm`, `alarm`, `cf`, `ct-events`, `elb`, `kms`, `lambda`, `logs`, `r53`, `role`, `sfn`, `sns`, `vpce`, `waf`) — `docs/related-resources.md` § `apigw`.
- Wave 1 = None; Wave 2 = `GetStages` per v2 API, no deployed stage → Warning; Wave 3 = `5XXError`/`4XXError` + `GetUsagePlans` quota-breach — `docs/attention-signals.md` § DNS, CDN, Certs — `apigw` row.
- `Api` struct fields (no `Status` field on list response; `ProtocolType`, `ApiId`, `Name`, `CreatedDate`) — `AWS SDK Go v2 — apigatewayv2/types.Api`.
- `Stage.AccessLogSettings`, `Stage.DeploymentId` (used to detect "no deployed stage") — `AWS SDK Go v2 — apigatewayv2/types.Stage § AccessLogSettings, DeploymentId`.
- `acm` discovery via `GetDomainNames` + `GetApiMappings` with `DomainNameConfigurations[].CertificateArn` — a9s-devops (2026-04-20): possible=yes, worth=yes. Cert expiry is a known outage vector for custom-domain APIs and operators want a direct pivot from the API to the cert.
- `alarm` discovery via reverse scan on `Namespace=AWS/ApiGateway` dimensions — a9s-devops (2026-04-20): possible=yes, worth=yes. Standard CloudWatch dimension convention, no extra API call when alarm list is already loaded.
- `cf` discovery via reverse scan on `Origins[].DomainName` matching `execute-api` — a9s-devops (2026-04-20): possible=yes, worth=yes. CloudFront origin hostname is the only AWS-exposed link.
- `elb` discovery via `GetVpcLinks` + `GetIntegrations` (`IntegrationType==VPC_LINK`) — a9s-devops (2026-04-20): possible=yes, worth=yes. Only chain AWS exposes for VpcLink → NLB.
- `kms` discovery is transitive via Lambda integration — a9s-devops (2026-04-20): possible=yes, worth=marginal. Keep per golden-doc contract; low-value but cheap since Lambda panel already resolves KMS.
- `lambda` discovery via `GetIntegrations` parsing Lambda function ARN in `IntegrationUri` — a9s-devops (2026-04-20): possible=yes, worth=yes. Highest-traffic pivot for this resource type.
- `logs` discovery via `Stage.AccessLogSettings.DestinationArn` — a9s-devops (2026-04-20): possible=yes, worth=yes. Stage access logs are the first log surface an operator wants when an API misbehaves.
- `r53` discovery via reverse scan on hosted-zone record sets with `AliasTarget.DNSName` matching custom-domain target — a9s-devops (2026-04-20): possible=yes, worth=yes. Alias-target matching is the only available reverse link.
- `role` discovery via `GetIntegrations.CredentialsArn` + `GetAuthorizers.AuthorizerCredentialsArn` — a9s-devops (2026-04-20): possible=yes, worth=yes. These are the only two APIGW-assumed-role fields.
- `sfn` discovery via `GetIntegrations` integration URI `arn:aws:apigateway:...:states:action/` — a9s-devops (2026-04-20): possible=yes, worth=yes. Canonical AWS-service integration ARN form.
- `sns` discovery via `GetIntegrations` integration URI `arn:aws:apigateway:...:sns:action/Publish` — a9s-devops (2026-04-20): possible=yes, worth=yes. Same pattern as sfn.
- `vpce` discovery split: v1 uses `RestApi.EndpointConfiguration.VpcEndpointIds` (first-class field); v2 only via resource-policy parse — a9s-devops (2026-04-20): possible=yes for v1, brittle for v2. Deferred v2 path to §5 Out of Scope.
- `waf` discovery via `wafv2:GetWebACLForResource` per stage ARN — a9s-devops (2026-04-20): possible=yes, worth=yes. Only reverse lookup WAFv2 offers per resource.
- `ct-events` universal-pivot policy — `docs/related-resources.md` § Policy.
- Allowed surfaces S1–S5, Wave→surface mapping, banned-words list, list-text ≤40 chars / detail ≤100 chars — `.claude/skills/a9s-resource-spec/SKILL.md` § "Allowed visualization surfaces" and § "UX rules the spec must enforce" (skill governance, not golden docs).
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- Severity choice `~` (not `!`) for "no deployed stage" — user decision deferred; current call is a9s-devops (2026-04-20): undeployed API is a cleanup/audit concern, not a page-the-on-call concern; promote to `!` only if governance escalation is adopted.
