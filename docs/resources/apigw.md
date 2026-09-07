---
shortName: apigw
name: API Gateways
awsApiRef: https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis.html
generatedFrom:
  - docs/architecture.md
  - docs/related-resources.md
  - docs/attention-signals.md
  - docs/historical/analysis/enrichment-visibility.md
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
- **How discovered**: call `GetVpcLinks` (one account-wide call) and read each VpcLink's `TargetArns` — the NLB ARNs — then match them against the already-loaded `elb` cache — a9s-devops: `TargetArns` is the only field that names the NLBs behind the links; the per-API integration intersection would cost an extra `GetIntegrations` beyond the checker budget.
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
- **How discovered**: not resolvable within the checker budget. Record sets are not cached as joinable structures — the r53 fetcher summarizes each zone's alias targets into one Fields string — so the alias-target reverse-scan this row originally described is not available; resolving the DNS hop would need `apigatewayv2:GetDomainNames` plus per-zone `ListResourceRecordSets` walks. (budget-excluded per related-resources.md Policy rule 7: custom-domain alias resolution requires per-zone record-set scans beyond the one-call budget.)
- **Count shown**: unknown.

### `role`

- **Why related**: Invocation/authorizer role — the IAM role APIGW assumes to call the integration target or to run a request authorizer.
- **How discovered**: `apigatewayv2:GetIntegrations` per API → read `CredentialsArn`; `apigatewayv2:GetAuthorizers` per API → read `AuthorizerCredentialsArn`; match to already-loaded `role` list — a9s-devops: these are the two places APIGW records an assumed role; anything else (e.g. Lambda execution role) belongs under the Lambda pivot, not here.
- **Count shown**: yes.

### `sfn`

- **Why related**: Step Functions integration target — APIGW can start a state-machine execution directly.
- **How discovered**: `apigatewayv2:GetIntegrations` per API detects `IntegrationUri` of the form `arn:aws:apigateway:<region>:states:action/StartExecution` — but that URI only says "this API talks to Step Functions". The target state-machine ARN lives in the route REQUEST TEMPLATE, not the IntegrationUri, so naming the specific state machine requires per-route template parsing. (budget-excluded per related-resources.md Policy rule 7: per-route request-template parsing exceeds the one-call budget; the checker detects the integration but cannot count targets.)
- **Count shown**: unknown.

### `sns`

- **Why related**: APIGW → SNS integration — publish a notification directly from an API request.
- **How discovered**: `apigatewayv2:GetIntegrations` per API detects `IntegrationUri` of the form `arn:aws:apigateway:<region>:sns:action/Publish` — but the topic ARN lives in the route REQUEST TEMPLATE, not the IntegrationUri, so naming the specific topic requires per-route template parsing — a9s-devops: identical pattern to sfn, different AWS-service slug. (budget-excluded per related-resources.md Policy rule 7: per-route request-template parsing exceeds the one-call budget; the checker detects the integration but cannot count targets.)
- **Count shown**: unknown.

### `vpce`

- **Why related**: Private APIs expose via VPC endpoint (interface type, `com.amazonaws.<region>.execute-api`).
- **How discovered**: for REST v1 APIs the IDs sit on `RestApi.EndpointConfiguration.VpcEndpointIds`, but the v2 `GetApis` items this fetcher lists carry no endpoint configuration, and the HTTP v2 path is a brittle resource-policy parse for `aws:SourceVpce` condition keys — a9s-devops: possible=yes for v1 via a first-class field, v2 only via policy parse. (budget-excluded per related-resources.md Policy rule 7: endpoint IDs are absent from the v2 list response and the v2 policy-parse gap has no in-budget resolution.)
- **Count shown**: unknown.

### `waf`

- **Why related**: WebACL attached to the API stage — the ingress filter that blocks bots, SQLi, rate abuse.
- **How discovered**: not resolvable within the checker budget — v2 APIs carry no Web ACL binding on `GetApis` (only REST v1 stages associate ACLs via `apigateway:GetWebACL`), and resolving from the WAF side requires `wafv2:ListResourcesForWebACL` per Web ACL, an O(N) fan-out over the target population. (budget-excluded per related-resources.md Policy rule 7: no in-budget lookup exists for v2 APIs.)
- **Count shown**: unknown.

### `ct-events`

- **Why related**: Audit trail for API changes — who created/updated/deleted the API, stages, routes, integrations.
- **How discovered**: universal pivot — applies to every registered type; see related-resources.md §Policy.
- **Count shown**: yes.

## 3. Attention / Issues Algorithm

**Source API**: [GetStages](https://docs.aws.amazon.com/apigateway/latest/api/API_GetStages.html)

Transcribed from `docs/attention-signals.md § Signals § DNS & CDN` row `apigw`.

### 3.1 Wave 1 — zero extra API calls

No Wave 1 signals — the list API does not return fields usable for attention.

`apigatewayv2:GetApis` returns `Api` structs whose visible fields are `Name`, `ApiId`, `ApiEndpoint`, `ProtocolType`, `CreatedDate`, `Description`, `Version`, `Tags` — none of which carry a deployment/health state (confirmed from `AWS SDK Go v2 — apigatewayv2/types.Api`). Every row is Healthy at Wave 1; attention must come from Wave 2.

### 3.2 Wave 2 — bounded extra API calls

- **Signal**: `apigatewayv2:GetStages` per HTTP/WebSocket API returns zero deployed stages (no stage with a `DeploymentId`).
  - **State bucket**: Warning.
  - **API call**: `apigatewayv2:GetStages` — one per v2 resource.
  - **Cost shape**: per-resource.

- **Signal**: a deployed stage carries a configuration gap with no code of its own.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: internet-facing REST API with no authorizer and no scoped resource policy.
  - **State bucket**: Broken.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: private REST API, or HTTP API, with no authorizer.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: a stage with no access log settings.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: a REST stage with `TracingEnabled == false`.
  - **State bucket**: Warning.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

- **Signal**: a REST stage variable whose value scans as a credential.
  - **State bucket**: Broken.
  - **How obtained**: read on the type's bounded Wave 2 pass, which the catalog registers for this type.

### 3.3 Wave 3 — OUT OF SCOPE

- OUT OF SCOPE: CloudWatch `5XXError`/`4XXError`.
- OUT OF SCOPE: `GetUsagePlans` quota-breach detection.

## 4. Issue Visualization

Every signal from §3 lands on the surfaces S1–S5 that `docs/attention-signals.md § Visualization Surfaces` defines; that section is where the wave→surface mapping lives.

<!-- BEGIN GENERATED: badge -->
Badge aggregation for `apigw`: Wave 1 issue-colored rows plus Wave 2 `!`-severity findings — this type registers a Wave 2 enricher.
<!-- END GENERATED: badge -->

One row per signal from §3:

| Signal (short) | Wave | State bucket | Severity | Surfaces reached | List text (S4) |
|---|---|---|---|---|---|
| No deployed stage (v2) | 2 | Warning | `~` | S3, S4, S5 | `no deployed stages` |
| a deployed stage carries a configuration gap with no code of its own | 2 | Warning | `~` | S3, S4, S5 | `stage configuration issues` |
| internet-facing REST API with no authorizer and no scoped resource policy | 2 | Broken | `!` | S1, S3, S4, S5 | `internet-facing with no authorizer` |
| private REST API, or HTTP API, with no authorizer | 2 | Warning | `~` | S3, S4, S5 | `no authorizer` |
| a stage with no access log settings | 2 | Warning | `~` | S3, S4, S5 | `no access logs` |
| a REST stage with `TracingEnabled == false` | 2 | Warning | `~` | S3, S4, S5 | `X-Ray tracing off` |
| a REST stage variable whose value scans as a credential | 2 | Broken | `!` | S1, S3, S4, S5 | `credential in stage variables` |

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
- Wave 1 = None; Wave 2 = `GetStages` per v2 API, no deployed stage → Warning — `docs/attention-signals.md § Signals § DNS & CDN` row `apigw`. The CloudWatch error rates and the `GetUsagePlans` quota-breach check are deferred — `docs/attention-signals.md § Not yet implemented`.
- `Api` struct fields (no `Status` field on list response; `ProtocolType`, `ApiId`, `Name`, `CreatedDate`) — `AWS SDK Go v2 — apigatewayv2/types.Api`.
- `Stage.AccessLogSettings`, `Stage.DeploymentId` (used to detect "no deployed stage") — `AWS SDK Go v2 — apigatewayv2/types.Stage § AccessLogSettings, DeploymentId`.
- `acm` discovery via `GetDomainNames` + `GetApiMappings` with `DomainNameConfigurations[].CertificateArn` — a9s-devops (2026-04-20): possible=yes, worth=yes. Cert expiry is a known outage vector for custom-domain APIs and operators want a direct pivot from the API to the cert.
- `alarm` discovery via reverse scan on `Namespace=AWS/ApiGateway` dimensions — a9s-devops (2026-04-20): possible=yes, worth=yes. Standard CloudWatch dimension convention, no extra API call when alarm list is already loaded.
- `cf` discovery via reverse scan on `Origins[].DomainName` matching `execute-api` — a9s-devops (2026-04-20): possible=yes, worth=yes. CloudFront origin hostname is the only AWS-exposed link.
- `elb` discovery via `GetVpcLinks` `TargetArns` matched against the elb cache — a9s-devops (2026-04-20): possible=yes, worth=yes. `TargetArns` is the only field naming the NLBs behind the links; one account-wide call fits the checker budget.
- `kms` discovery is transitive via Lambda integration — a9s-devops (2026-04-20): possible=yes, worth=marginal. Keep per golden-doc contract; low-value but cheap since Lambda panel already resolves KMS.
- `lambda` discovery via `GetIntegrations` parsing Lambda function ARN in `IntegrationUri` — a9s-devops (2026-04-20): possible=yes, worth=yes. Highest-traffic pivot for this resource type.
- `logs` discovery via `Stage.AccessLogSettings.DestinationArn` — a9s-devops (2026-04-20): possible=yes, worth=yes. Stage access logs are the first log surface an operator wants when an API misbehaves.
- `r53` budget exclusion — record sets are not cached as joinable structures (the r53 fetcher summarizes alias targets into one Fields string); custom-domain alias resolution needs per-zone `ListResourceRecordSets` walks — `docs/related-resources.md` § Policy rule 7.
- `role` discovery via `GetIntegrations.CredentialsArn` + `GetAuthorizers.AuthorizerCredentialsArn` — a9s-devops (2026-04-20): possible=yes, worth=yes. These are the only two APIGW-assumed-role fields.
- `sfn` detection via `GetIntegrations` integration URI `arn:aws:apigateway:...:states:action/`; the state-machine ARN itself lives in the route request template, so the target count is budget-excluded — `docs/related-resources.md` § Policy rule 7.
- `sns` detection via `GetIntegrations` integration URI `arn:aws:apigateway:...:sns:action/Publish`; the topic ARN lives in the route request template, so the target count is budget-excluded — `docs/related-resources.md` § Policy rule 7.
- `vpce` budget exclusion: v1 uses `RestApi.EndpointConfiguration.VpcEndpointIds` (first-class field) but the v2 list response carries none; v2 resource-policy parse is brittle — a9s-devops (2026-04-20): possible=yes for v1, brittle for v2. `docs/related-resources.md` § Policy rule 7; v2 gap also in §5 Out of Scope.
- `waf` budget exclusion: v2 APIs carry no Web ACL binding; WAF-side resolution requires `wafv2:ListResourcesForWebACL` per ACL (O(N)) — `docs/related-resources.md` § Policy rule 7.
- `ct-events` universal-pivot policy — `docs/related-resources.md` § Policy.
- Allowed surfaces S1–S5, Wave→surface mapping, banned-words list, list-text ≤40 chars / detail ≤100 chars — `.claude/skills/a9s-resource-spec/SKILL.md` § "Allowed visualization surfaces" and § "UX rules the spec must enforce" (skill governance, not golden docs).
- Read-only invariant — `docs/architecture.md` § "What is a9s?".
- Severity choice `~` (not `!`) for "no deployed stage" — user decision deferred; current call is a9s-devops (2026-04-20): undeployed API is a cleanup/audit concern, not a page-the-on-call concern; promote to `!` only if governance escalation is adopted.

<!-- BEGIN GENERATED: header -->
apigw — DNS & CDN. Lifecycle key: none (the list API returns no lifecycle field).
<!-- END GENERATED: header -->

<!-- BEGIN GENERATED: findings -->
| Code | Phrase | Severity | Source | Detail |
| --- | --- | --- | --- | --- |
| apigw.no-deployed-stages | no deployed stages | warn | wave2 | — |
| apigw.stage-config-issues | stage configuration issues | warn | wave2 | — |
| apigw.no-authorizer-public | internet-facing with no authorizer | broken | wave2 | Anyone on the internet can call every route this gateway exposes, because nothing checks the caller's identity. Attach an authorizer, or scope the resource policy to the callers that should reach it. |
| apigw.no-authorizer | no authorizer | warn | wave2 | Nothing checks the caller's identity, so any client that can reach the network this gateway sits on can call every route. Attach an authorizer. |
| apigw.no-access-logs | no access logs | warn | wave2 | The stage records no access logs, so a burst of abusive or failing requests leaves nothing to investigate. Point the stage's access logging at a log group. |
| apigw.tracing-off | X-Ray tracing off | warn | wave2 | Requests through this stage are not traced, so a slow or failing integration cannot be followed to its cause. Turn on X-Ray tracing for the stage. |
| apigw.stage-variable-secret | credential in stage variables | broken | wave2 | A stage variable holds what looks like a credential, and stage variables are readable by anyone who can read the gateway's configuration. Move the value into Secrets Manager and reference it from the integration. |
<!-- END GENERATED: findings -->

<!-- BEGIN GENERATED: related -->
| Target Type | Display Name | Truncated? |
| --- | --- | --- |
| logs | Log Groups | yes |
| lambda | Lambda Functions | no |
| acm | ACM Certificates | no |
| alarm | CloudWatch Alarms | yes |
| cf | CloudFront | yes |
| elb | Load Balancers | yes |
| kms | KMS Keys | no |
| role | IAM Role | no |
| ct-events | CloudTrail Events | no |
<!-- END GENERATED: related -->
