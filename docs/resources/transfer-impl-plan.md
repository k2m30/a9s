# transfer — Implementation Plan

Derived from [`docs/resources/transfer.md`](transfer.md). Spec has zero TBDs; the §2.1 child-view divergence from the product-goal wording is flagged there (agreements-only, overridable by the release owner).

## 0. Architecture decision — in-fetcher N+1 (the mwaa/eks pattern), rich degraded rows

`ListServers` is informative (State/Domain/EndpointType/IdentityProviderType/LoggingRole/UserCount), but every §2 pivot field (`EndpointDetails`, `StructuredLogDestinations`, `Certificate`, `IdentityProviderDetails`) and both §3.2 config signals live only on `DescribedServer` — and pivots must work on the FIRST detail open, before any enrichment sweep. Accounts run 1–5 servers. Therefore the fetcher does `ListServers` + `DescribeServer` per id (both `RetryOnThrottle`, E3/E5 aggregation), `RawStruct = DescribedServer`, all findings fetcher-written (`Source: "wave1"`). NO `transfer_issue_enrichment.go`, NO `Wave2` catalog field.

**Degraded rows are RICH here** (unlike mwaa): a denied `DescribeServer` still has the full `ListedServer` — build the row from list fields (state finding included), then append the `transfer.warn.details_denied` finding (shared `DetailsDeniedFindingDef("transfer")` declared in the catalog) and aggregate the failure. A listed server never vanishes AND never loses the data the list already gave us. `RawStruct` falls back to the `ListedServer`.

`Resource.ID` = ServerId (`s-…`). `Fields["arn"]` from `Arn`.

Findings (codes `transfer.*`):
- `transfer.warn.offline` "offline: not accepting transfers" SevWarn
- `transfer.warn.starting` "starting" / `transfer.warn.stopping` "stopping" SevWarn
- `transfer.broken.start_failed` "start failed" SevBroken
- `transfer.warn.stop_failed` "stop failed" SevWarn
- `transfer.warn.legacy_policy` "legacy security policy" SevWarn (denylist: TransferSecurityPolicy-2018-11, -2020-06; Detail names the policy)
- `transfer.warn.no_logging` "no activity logging" SevWarn (LoggingRole nil AND StructuredLogDestinations empty)
- `DetailsDeniedFindingDef("transfer")`

§4 precedence: state finding first, then legacy-policy, then no-logging, then details-denied.

## 1. Behavioral test spec (pseudocode) — one case per §4 row

```text
TEST: online_silence            GIVEN ONLINE, modern policy, structured logs  THEN green, S4 blank, 0 findings
TEST: state_phrases             one subtest per OFFLINE/STARTING/STOPPING/START_FAILED/STOP_FAILED
                                THEN exact §4 phrase + severity; no raw enum anywhere
TEST: legacy_policy             GIVEN ONLINE + TransferSecurityPolicy-2018-11
                                THEN finding "legacy security policy"; Detail contains the policy name
TEST: no_logging                GIVEN ONLINE + LoggingRole nil + no StructuredLogDestinations
                                THEN finding "no activity logging"
TEST: multi_stack               GIVEN OFFLINE + legacy policy + no logging
                                THEN Findings ordered [offline…, legacy…, no activity…]; S4 "offline: not accepting transfers (+2)"
TEST: details_denied_rich       GIVEN DescribeServer AccessDenied for one id
                                THEN row KEPT with list fields (state finding present) + details-denied finding appended;
                                     composite error names the id
TEST: list_denied_is_error      GIVEN ListServers AccessDenied THEN (FetchResult{}, err), never empty success
TEST: partial_describe          GIVEN 5 listed, 2 describes fail THEN 5 rows (2 rich-degraded) + composite error (U12/E5)
TEST: related_targets           graph root resolves: role 1, vpc 1, subnet 3, vpce 1, logs ≥1, ct-events drillable;
                                acm 1 on the FTPS fixture; lambda 1 on the AWS_LAMBDA-IdP fixture; sg NOT registered
TEST: agreements_child          `e` on graph root lists agreements; INACTIVE row → "inactive: partner traffic rejected";
                                columns per spec §2.1
TEST: cert_expiry_on_agreement  agreement detail resolves profiles + certs; expired cert Broken "expired",
                                <30d Warning "expires in <N>d"
TEST: wave3_anti                no CloudWatch metric strings; UserCount==0 produces no finding; LoggingRole-nil alone
                                (with structured logs present) produces no finding
```

## 2. Fixture list (`internal/demo/fixtures/transfer.go`; synthetic account 123456789012)

```text
prod-as2-gateway     (GRAPH ROOT) ONLINE, Protocols [AS2], Domain S3, EndpointType VPC
                     (VpcEndpointId+VpcId+3 SubnetIds from vpc-graph fixtures), SERVICE_MANAGED,
                     LoggingRole → existing role fixture, StructuredLogDestinations → 1 logs fixture,
                     modern policy (TransferSecurityPolicy-2024-01), egress IPs ×3, UserCount 0.
                     Pivot counts: role 1, vpc 1, subnet 3, vpce 1, logs 1 → ≥2 on 1/6… BOOST: subnet 3
                     satisfies ≥2; add 2nd log destination → logs 2 → ≥2 ratio 2/6 = 33% < 50%!
                     → give graph root TWO log destinations AND keep subnet 3 → ratio 2/6; add acm?
                     FTPS on the SAME server (Protocols [AS2, FTPS] + Certificate → acm fixture) → acm 1;
                     lambda stays on its own fixture. Countable pivots on root: role1 vpc1 subnet3 vpce1
                     logs2 acm1 = 6 pivots, ≥2 on 2 → 33%. Per 9.3 need ≥50% → subnet 3 ✓, logs 2 ✓,
                     egress? not a pivot. Root gets 2 log groups + 3 subnets → 2/6. NEED one more ≥2:
                     ct-events exempt; sg absent. → Solution: agreements 2 on root (child view rows are
                     not pivots). ACCEPTED DEVIATION: document ratio 2/6 with reason (single-valued
                     fields — VpcId/VpcEndpointId/Certificate/LoggingRole are 1:1 by API shape; only
                     subnet+logs are list-valued). Phase 9.3 reports the structural ceiling.
sftp-users-prod      ONLINE, Protocols [SFTP], SERVICE_MANAGED, UserCount 12, modern policy,
                     PUBLIC endpoint (no EndpointDetails) — proves conditional pivots absent cleanly.
sftp-lambda-auth     ONLINE, IdentityProviderType AWS_LAMBDA, Function → lambda fixture (lambda pivot 1).
warn-transfer-offline        OFFLINE
warn-transfer-starting       STARTING
warn-transfer-stopping       STOPPING
broken-transfer-start-failed START_FAILED
warn-transfer-stop-failed    STOP_FAILED
warn-transfer-legacy-policy  ONLINE + TransferSecurityPolicy-2018-11
warn-transfer-no-logging     ONLINE + LoggingRole nil + no StructuredLogDestinations
warn-transfer-multi          OFFLINE + legacy policy + no logging → "offline: not accepting transfers (+2)"
warn-transfer-details-denied listed, DescribeServer denied (fake) → rich degraded row
Agreements (server-scoped, on graph root): agr-prod-partner ACTIVE (LocalProfile lp-1, PartnerProfile pp-1,
                     BaseDirectory /a9s-demo-healthy/inbound, AccessRole → role fixture);
                     agr-old-partner INACTIVE.
Profiles: lp-1 LOCAL As2Id "ACME-LOCAL"; pp-1 PARTNER As2Id "PARTNER-CO".
Certificates: cert-fresh ACTIVE (InactiveDate +2y), cert-expiring ACTIVE (+20d), cert-expired
                     (InactiveDate past) — attached to lp-1/pp-1 so agreement detail witnesses all three.
```

Menu badge: OFFLINE, STARTING, STOPPING, STOP_FAILED, legacy, no-logging, multi, details-denied = 8 Warning + 1 Broken = **issues:9**; rows 12. Counts: type 68 (menu 69); docs 67→68.

## 3. File scope union

| File | Owner |
|---|---|
| internal/demo/fixtures/transfer.go (+sibling touches: logs/role/acm/lambda/vpc-graph refs) | 6a coder |
| internal/demo/fakes/transfer.go (ListServers/DescribeServer/ListAgreements/DescribeAgreement/ListProfiles?/DescribeProfile/ListCertificates/DescribeCertificate — denial for the denied id) | 6a coder |
| internal/demo/client.go (`clients.Transfer = fakes.NewTransfer()`) | 6a coder |
| internal/aws/transfer.go — fetcher (in-fetcher N+1, rich degraded) | 7 coder |
| internal/aws/transfer_interfaces.go — TransferAPI narrow | 6a stub / 7 extend |
| internal/aws/transfer_related.go — 8 checkers (field-driven; display names: "ACM Certificates", "Lambda Functions", "Log Groups", "IAM Roles", "Subnets", "VPC", "VPC Endpoints", ct-events) | 7 coder |
| internal/aws/transfer_children.go or in catalog — agreements ChildFetcher (ListAgreements+DescribeAgreement N+1; inline profile/cert resolution on agreement detail via DetailEnrich) | 7 coder |
| internal/aws/client.go — Transfer field + NewFromConfig | 6a stub / 7 verify |
| internal/aws/catalog_networking.go — ResourceTypeDef (Category NETWORKING, Aliases [transfer sftp as2 ftps] — uniqueness gate), Children (Agreements), Findings incl. DetailsDeniedFindingDef("transfer") | 7 coder |
| internal/config/defaults_networking.go — columns: Server Id, Status, Domain, Endpoint, Identity Provider, Users, Created? (per §3.1: ServerId State Domain EndpointType IdentityProviderType UserCount) | 7 coder |
| .a9s/views/transfer.yaml (viewsgen) | 7 coder |
| tests/unit/aws_transfer_test.go + aws_transfer_related_test.go (+child/agreement tests) | 6b QA |
| tests/integration/scenario_transfer_visual_test.go + drillThroughFixtures row + counts pins (menu 68→69, filter tests, smoke scripts resource-types(69)) | runner |
| docs counts 67→68 (README.tmpl ×3, website resources.md +row), README regen | runner |

## 4. Coverage matrix deltas vs mwaa

U3/U4/U7d N/A (no glyph-on-green — all signals color-bearing). U7a covered by warn-transfer-multi (3 findings → (+2)). U9 graph-root ≥2 ratio: 2/6 — STRUCTURAL CEILING (single-valued API fields), documented deviation for 9.3. U12 partial_describe. U14: ARN params absent (ServerId-addressed) except cert/agreement ids — E7 fields populated. U16 scenario asserts AssertNoEnrichmentErrors.
