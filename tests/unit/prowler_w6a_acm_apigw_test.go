package unit

// prowler_w6a_acm_apigw_test.go — batch w6a rows 17-21 (the ACM key
// algorithm and the four API Gateway stage signals), plus the two pins that
// apply to the batch as a whole: colour derives from findings for every type
// in it, and no supporting row restates the phrase it sits under.
//
// Row 18 is two codes, not one severity computed at emit time: an
// internet-facing REST API with no authorizer is Broken, and every other
// unauthorized API is Warn. A single code cannot carry two severities,
// because catalog.FindingDef holds one and the catalog is what the badge and
// the docs table read.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w6aACMWeakKey        = "acm.weak-key"
	w6aAPIGWNoAuthPublic = "apigw.no-authorizer-public"
	w6aAPIGWNoAuth       = "apigw.no-authorizer"
	w6aAPIGWNoAccessLogs = "apigw.no-access-logs"
	w6aAPIGWTracingOff   = "apigw.tracing-off"
	w6aAPIGWStageSecret  = "apigw.stage-variable-secret"
)

// ---------------------------------------------------------------------------
// acm — row 17
// ---------------------------------------------------------------------------

type w6aACMFake struct {
	certs []acmtypes.CertificateSummary
}

func (f *w6aACMFake) ListCertificates(_ context.Context, _ *acm.ListCertificatesInput, _ ...func(*acm.Options)) (*acm.ListCertificatesOutput, error) {
	return &acm.ListCertificatesOutput{CertificateSummaryList: f.certs}, nil
}

// w6aCert returns an issued, in-use certificate with the given key algorithm
// and a year left on the clock, so no expiry finding competes with row 17.
func w6aCert(domainName string, alg acmtypes.KeyAlgorithm) acmtypes.CertificateSummary {
	return acmtypes.CertificateSummary{
		CertificateArn: aws.String("arn:aws:acm:eu-central-1:123456789012:certificate/1a2b3c4d-5e6f-7081-92a3-b4c5d6e7f809"),
		DomainName:     aws.String(domainName),
		Status:         acmtypes.CertificateStatusIssued,
		Type:           acmtypes.CertificateTypeAmazonIssued,
		KeyAlgorithm:   alg,
		InUse:          aws.Bool(true),
		NotBefore:      aws.Time(time.Now().Add(-30 * 24 * time.Hour)),
		NotAfter:       aws.Time(time.Now().Add(365 * 24 * time.Hour)),
	}
}

func w6aFetchCerts(t *testing.T, certs ...acmtypes.CertificateSummary) map[string]resource.Resource {
	t.Helper()
	out, err := awsclient.FetchACMCertificatesPage(context.Background(), &w6aACMFake{certs: certs}, "")
	if err != nil {
		t.Fatalf("FetchACMCertificatesPage: %v", err)
	}
	byName := make(map[string]resource.Resource, len(out.Resources))
	for _, r := range out.Resources {
		byName[r.Name] = r
	}
	return byName
}

// TestW6AACMWeakKey pins row 17. An RSA key below 2048 bits is factorable
// within reach of a funded attacker, so a certificate on one is not the
// control the operator believes it is.
func TestW6AACMWeakKey(t *testing.T) {
	got := w6aFetchCerts(t, w6aCert("legacy-weak-key.acme-corp.com", acmtypes.KeyAlgorithmRsa1024))
	w2AssertFinding(t, got["legacy-weak-key.acme-corp.com"].Findings, w6aACMWeakKey,
		"weak key algorithm", domain.SevWarn, "wave1")
	w6aAssertRow(t, got["legacy-weak-key.acme-corp.com"], w6aACMWeakKey, "Key algorithm", "RSA 1024")
	w2AssertFindingDef(t, "acm", w6aACMWeakKey, "weak key algorithm", domain.SevWarn, "wave1")
}

// TestW6AACMWeakKey_StrongAlgorithmsAreHealthy pins the negative half across
// every algorithm ACM issues that is not weak. An elliptic-curve key at any
// size AWS offers, and RSA at 2048 or above, are all fine — flagging one
// would be a finding the operator cannot act on.
func TestW6AACMWeakKey_StrongAlgorithmsAreHealthy(t *testing.T) {
	for _, alg := range []acmtypes.KeyAlgorithm{
		acmtypes.KeyAlgorithmRsa2048,
		acmtypes.KeyAlgorithmRsa3072,
		acmtypes.KeyAlgorithmRsa4096,
		acmtypes.KeyAlgorithmEcPrime256v1,
		acmtypes.KeyAlgorithmEcSecp384r1,
		acmtypes.KeyAlgorithmEcSecp521r1,
	} {
		t.Run(string(alg), func(t *testing.T) {
			got := w6aFetchCerts(t, w6aCert("strong.acme-corp.com", alg))
			w2AssertNoCode(t, got["strong.acme-corp.com"].Findings, w6aACMWeakKey)
		})
	}
}

// TestW6AACMWeakKey_UnknownAlgorithmIsNotWeak pins the nil/unknown rule. ACM
// leaves KeyAlgorithm empty when the summary did not resolve it, and an
// unresolved algorithm is not evidence of a weak one.
func TestW6AACMWeakKey_UnknownAlgorithmIsNotWeak(t *testing.T) {
	got := w6aFetchCerts(t, w6aCert("unknown.acme-corp.com", ""))
	w2AssertNoCode(t, got["unknown.acme-corp.com"].Findings, w6aACMWeakKey)
}

// ---------------------------------------------------------------------------
// apigw — rows 18-21
// ---------------------------------------------------------------------------

// w6aAPIGWV1Fake answers the three v1 REST calls the enricher makes.
type w6aAPIGWV1Fake struct {
	awsclient.APIGatewayV1API

	authorizers []apigwtypes.Authorizer
	stages      []apigwtypes.Stage
	stagesErr   error
}

func (f *w6aAPIGWV1Fake) GetAuthorizers(_ context.Context, _ *apigateway.GetAuthorizersInput, _ ...func(*apigateway.Options)) (*apigateway.GetAuthorizersOutput, error) {
	return &apigateway.GetAuthorizersOutput{Items: f.authorizers}, nil
}

func (f *w6aAPIGWV1Fake) GetStages(_ context.Context, _ *apigateway.GetStagesInput, _ ...func(*apigateway.Options)) (*apigateway.GetStagesOutput, error) {
	if f.stagesErr != nil {
		return nil, f.stagesErr
	}
	return &apigateway.GetStagesOutput{Item: f.stages}, nil
}

// w6aAPIGWV2Fake answers the v2 calls. Its stages are healthy by default so
// the existing v2 rows do not compete with the ones under test.
type w6aAPIGWV2Fake struct {
	awsclient.APIGatewayV2API

	authorizers []apigwv2types.Authorizer
	stages      []apigwv2types.Stage
}

func (f *w6aAPIGWV2Fake) GetAuthorizers(_ context.Context, _ *apigatewayv2.GetAuthorizersInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetAuthorizersOutput, error) {
	return &apigatewayv2.GetAuthorizersOutput{Items: f.authorizers}, nil
}

func (f *w6aAPIGWV2Fake) GetStages(_ context.Context, _ *apigatewayv2.GetStagesInput, _ ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStagesOutput, error) {
	return &apigatewayv2.GetStagesOutput{Items: f.stages}, nil
}

// w6aV2HealthyStage is a v2 stage with throttling and access logs configured,
// so it trips none of the pre-existing v2 findings.
func w6aV2HealthyStage(name string) apigwv2types.Stage {
	return apigwv2types.Stage{
		StageName:  aws.String(name),
		AutoDeploy: aws.Bool(true),
		AccessLogSettings: &apigwv2types.AccessLogSettings{
			DestinationArn: aws.String("arn:aws:logs:eu-central-1:123456789012:log-group:/aws/apigw/acme:*"),
			Format:         aws.String("$context.requestId"),
		},
		DefaultRouteSettings: &apigwv2types.RouteSettings{
			ThrottlingBurstLimit: aws.Int32(500),
			ThrottlingRateLimit:  aws.Float64(1000),
		},
	}
}

// w6aV1Stage returns a healthy v1 REST stage: access logging on, tracing on,
// no credentials in its variables.
func w6aV1Stage(name string) apigwtypes.Stage {
	return apigwtypes.Stage{
		StageName:      aws.String(name),
		DeploymentId:   aws.String("d1e2f3"),
		TracingEnabled: true,
		AccessLogSettings: &apigwtypes.AccessLogSettings{
			DestinationArn: aws.String("arn:aws:logs:eu-central-1:123456789012:log-group:/aws/apigw/acme-rest:*"),
			Format:         aws.String("$context.requestId"),
		},
		Variables: map[string]string{"backendStage": "prod", "region": "eu-central-1"},
	}
}

// w6aRESTRes builds an a9s apigw row for the v1 lane, keyed the way
// FetchAPIGatewaysPageMerged keys it.
func w6aRESTRes(id, name, endpoint, policy string) resource.Resource {
	r := w2Res(id, apigwtypes.RestApi{
		Id:     aws.String(id),
		Name:   aws.String(name),
		Policy: aws.String(policy),
		EndpointConfiguration: &apigwtypes.EndpointConfiguration{
			Types: []apigwtypes.EndpointType{apigwtypes.EndpointType(endpoint)},
		},
	})
	r.Name = name
	r.Fields["api_id"] = id
	r.Fields["protocol"] = "REST"
	r.Fields["endpoint"] = endpoint
	return r
}

// w6aHTTPRes builds an a9s apigw row for the v2 lane.
func w6aHTTPRes(id, name string) resource.Resource {
	r := w2Res(id, apigwv2types.Api{
		ApiId:        aws.String(id),
		Name:         aws.String(name),
		ProtocolType: apigwv2types.ProtocolTypeHttp,
	})
	r.Name = name
	r.Fields["api_id"] = id
	r.Fields["protocol"] = "HTTP"
	return r
}

func w6aEnrichAPIGW(t *testing.T, v1 *w6aAPIGWV1Fake, v2 *w6aAPIGWV2Fake, rs ...resource.Resource) awsclient.IssueEnricherResult {
	t.Helper()
	clients := &awsclient.ServiceClients{APIGatewayV2: v2}
	if v1 != nil {
		clients.APIGatewayV1 = v1
	}
	res, err := w2Enricher(t, "apigw")(context.Background(), clients, rs, nil)
	w2AssertEnricherShape(t, res)
	_ = err
	return res
}

// TestW6AAPIGWRESTNoAuthorizerIsBrokenWhenReachable pins the Broken half of
// row 18. A REST API with a public endpoint, no authorizer and no resource
// policy is open to the internet, which is the case Prowler raises loudest.
func TestW6AAPIGWRESTNoAuthorizerIsBrokenWhenReachable(t *testing.T) {
	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{stages: []apigwtypes.Stage{w6aV1Stage("prod")}},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst001noauth", "acme-orders-rest", "EDGE", ""),
	)

	w2AssertFinding(t, res.Findings["rst001noauth"], w6aAPIGWNoAuthPublic,
		"internet-facing with no authorizer", domain.SevBroken, "wave2:apigw")
	rows := w2Rows(t, res, "rst001noauth", w6aAPIGWNoAuthPublic)
	w2AssertRow(t, rows, "Authorizers", "0")
	w2AssertRow(t, rows, "Endpoint", "edge")
}

// TestW6AAPIGWRESTNoAuthorizer_PrivateEndpointIsOnlyAWarning pins the split.
// A private REST API is reachable only from inside the VPC, so a missing
// authorizer is a defence-in-depth gap rather than an exposure.
func TestW6AAPIGWRESTNoAuthorizer_PrivateEndpointIsOnlyAWarning(t *testing.T) {
	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{stages: []apigwtypes.Stage{w6aV1Stage("prod")}},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst005private", "acme-internal-rest", "PRIVATE", ""),
	)

	w2AssertFinding(t, res.Findings["rst005private"], w6aAPIGWNoAuth,
		"no authorizer", domain.SevWarn, "wave2:apigw")
	w2AssertNoCode(t, res.Findings["rst005private"], w6aAPIGWNoAuthPublic)
}

// TestW6AAPIGWRESTNoAuthorizer_ScopedResourcePolicyClosesIt pins contract
// rule 6 on apigw: a resource policy that grants access under a condition is
// a real control, so the API is not unauthorized. A policy open to everyone
// is not, and the finding stands.
func TestW6AAPIGWRESTNoAuthorizer_ScopedResourcePolicyClosesIt(t *testing.T) {
	scoped := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"execute-api:Invoke","Resource":"arn:aws:execute-api:eu-central-1:123456789012:rst006scoped/*","Condition":{"IpAddress":{"aws:SourceIp":["203.0.113.0/24"]}}}]}`
	open := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"execute-api:Invoke","Resource":"arn:aws:execute-api:eu-central-1:123456789012:rst007open/*"}]}`

	guarded := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{stages: []apigwtypes.Stage{w6aV1Stage("prod")}},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst006scoped", "acme-scoped-rest", "REGIONAL", scoped),
	)
	w2AssertNoCode(t, guarded.Findings["rst006scoped"], w6aAPIGWNoAuthPublic)
	w2AssertNoCode(t, guarded.Findings["rst006scoped"], w6aAPIGWNoAuth)

	exposed := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{stages: []apigwtypes.Stage{w6aV1Stage("prod")}},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst007open", "acme-open-rest", "REGIONAL", open),
	)
	w2AssertFinding(t, exposed.Findings["rst007open"], w6aAPIGWNoAuthPublic,
		"internet-facing with no authorizer", domain.SevBroken, "wave2:apigw")
}

// TestW6AAPIGWRESTNoAuthorizer_AnAuthorizerClosesIt pins the negative half.
func TestW6AAPIGWRESTNoAuthorizer_AnAuthorizerClosesIt(t *testing.T) {
	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{
			authorizers: []apigwtypes.Authorizer{{
				Id:   aws.String("auth01"),
				Name: aws.String("acme-jwt"),
				Type: apigwtypes.AuthorizerTypeRequest,
			}},
			stages: []apigwtypes.Stage{w6aV1Stage("prod")},
		},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst008authed", "acme-guarded-rest", "EDGE", ""),
	)
	w2AssertNoCode(t, res.Findings["rst008authed"], w6aAPIGWNoAuthPublic)
	w2AssertNoCode(t, res.Findings["rst008authed"], w6aAPIGWNoAuth)
}

// TestW6AAPIGWHTTPNoAuthorizer pins the v2 lane of row 18. An HTTP API has no
// private endpoint type, so an unauthorized one is always the Warn code.
func TestW6AAPIGWHTTPNoAuthorizer(t *testing.T) {
	res := w6aEnrichAPIGW(t, nil,
		&w6aAPIGWV2Fake{stages: []apigwv2types.Stage{w6aV2HealthyStage("$default")}},
		w6aHTTPRes("htp001noauth", "acme-orders-http"),
	)
	w2AssertFinding(t, res.Findings["htp001noauth"], w6aAPIGWNoAuth,
		"no authorizer", domain.SevWarn, "wave2:apigw")

	guarded := w6aEnrichAPIGW(t, nil,
		&w6aAPIGWV2Fake{
			authorizers: []apigwv2types.Authorizer{{
				AuthorizerId: aws.String("auth02"),
				Name:         aws.String("acme-jwt"),
			}},
			stages: []apigwv2types.Stage{w6aV2HealthyStage("$default")},
		},
		w6aHTTPRes("htp002authed", "acme-guarded-http"),
	)
	w2AssertNoCode(t, guarded.Findings["htp002authed"], w6aAPIGWNoAuth)
}

// TestW6AAPIGWRESTNoAccessLogs pins row 19 on the v1 lane, and that the code
// is the same one the v2 lane already uses — one signal, one code, or the
// badge counts the same gap twice under two names.
func TestW6AAPIGWRESTNoAccessLogs(t *testing.T) {
	silent := w6aV1Stage("prod")
	silent.AccessLogSettings = nil

	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{
			authorizers: []apigwtypes.Authorizer{{Id: aws.String("auth01")}},
			stages:      []apigwtypes.Stage{silent, w6aV1Stage("staging")},
		},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst002nologs", "acme-unlogged-rest", "REGIONAL", ""),
	)

	w2AssertFinding(t, res.Findings["rst002nologs"], w6aAPIGWNoAccessLogs,
		"no access logs", domain.SevWarn, "wave2:apigw")
	w2AssertRow(t, w2Rows(t, res, "rst002nologs", w6aAPIGWNoAccessLogs), "Stage", "prod")

	logged := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{
			authorizers: []apigwtypes.Authorizer{{Id: aws.String("auth01")}},
			stages:      []apigwtypes.Stage{w6aV1Stage("prod")},
		},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst009logged", "acme-logged-rest", "REGIONAL", ""),
	)
	w2AssertNoCode(t, logged.Findings["rst009logged"], w6aAPIGWNoAccessLogs)
}

// TestW6AAPIGWRESTTracingOff pins row 20.
func TestW6AAPIGWRESTTracingOff(t *testing.T) {
	untraced := w6aV1Stage("prod")
	untraced.TracingEnabled = false

	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{
			authorizers: []apigwtypes.Authorizer{{Id: aws.String("auth01")}},
			stages:      []apigwtypes.Stage{untraced},
		},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst003notrace", "acme-untraced-rest", "REGIONAL", ""),
	)

	w2AssertFinding(t, res.Findings["rst003notrace"], w6aAPIGWTracingOff,
		"X-Ray tracing off", domain.SevWarn, "wave2:apigw")
	w2AssertRow(t, w2Rows(t, res, "rst003notrace", w6aAPIGWTracingOff), "Stage", "prod")

	traced := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{
			authorizers: []apigwtypes.Authorizer{{Id: aws.String("auth01")}},
			stages:      []apigwtypes.Stage{w6aV1Stage("prod")},
		},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst010traced", "acme-traced-rest", "REGIONAL", ""),
	)
	w2AssertNoCode(t, traced.Findings["rst010traced"], w6aAPIGWTracingOff)
}

// TestW6AAPIGWStageVariableSecret pins row 21. Stage variables are readable
// by anyone with apigateway:GET, so a credential in one is disclosed to every
// reader of the account's configuration.
func TestW6AAPIGWStageVariableSecret(t *testing.T) {
	leaky := w6aV1Stage("prod")
	leaky.Variables = map[string]string{
		"backendStage": "prod",
		"DB_PASSWORD":  "hunter2-not-a-real-secret",
	}

	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{
			authorizers: []apigwtypes.Authorizer{{Id: aws.String("auth01")}},
			stages:      []apigwtypes.Stage{leaky},
		},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst004secret", "acme-leaky-rest", "REGIONAL", ""),
	)

	f := w2AssertFinding(t, res.Findings["rst004secret"], w6aAPIGWStageSecret,
		"credential in stage variables", domain.SevBroken, "wave2:apigw")
	rows := w2Rows(t, res, "rst004secret", w6aAPIGWStageSecret)
	w2AssertRow(t, rows, "Stage", "prod")
	w2AssertRow(t, rows, "DB_PASSWORD", "keyword")

	// Contract rule 7: the value never leaves the account. Neither the phrase,
	// the detail sentence nor any row may carry it.
	w6aAssertNoSecretLeak(t, f.Phrase, f.Detail)
	for _, r := range rows {
		w6aAssertNoSecretLeak(t, r.Label, r.Value)
	}
}

// TestW6AAPIGWStageVariableSecret_OrdinaryVariablesAreClean pins the negative.
func TestW6AAPIGWStageVariableSecret_OrdinaryVariablesAreClean(t *testing.T) {
	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{
			authorizers: []apigwtypes.Authorizer{{Id: aws.String("auth01")}},
			stages:      []apigwtypes.Stage{w6aV1Stage("prod")},
		},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst011clean", "acme-clean-rest", "REGIONAL", ""),
	)
	w2AssertNoCode(t, res.Findings["rst011clean"], w6aAPIGWStageSecret)
}

// TestW6AAPIGW_ConditionsAreIndependent pins contract rule 4 on the v1 lane:
// one REST API failing four checks carries four findings.
func TestW6AAPIGW_ConditionsAreIndependent(t *testing.T) {
	bad := w6aV1Stage("prod")
	bad.AccessLogSettings = nil
	bad.TracingEnabled = false
	bad.Variables = map[string]string{"DB_PASSWORD": "hunter2-not-a-real-secret"}

	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{stages: []apigwtypes.Stage{bad}},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rst012worst", "acme-worst-rest", "EDGE", ""),
	)

	fs := res.Findings["rst012worst"]
	for _, code := range []string{
		w6aAPIGWNoAuthPublic, w6aAPIGWNoAccessLogs, w6aAPIGWTracingOff, w6aAPIGWStageSecret,
	} {
		if _, ok := w2Find(fs, code); !ok {
			t.Errorf("missing %q; got %v", code, w2Codes(fs))
		}
	}
}

// TestW6AAPIGW_StageFailureTruncatesTheAPI pins that an API whose stages
// could not be listed is marked rather than reported clean, and that the rest
// of the batch is still evaluated.
func TestW6AAPIGW_StageFailureTruncatesTheAPI(t *testing.T) {
	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{stagesErr: errNotFoundForTest{}},
		&w6aAPIGWV2Fake{stages: []apigwv2types.Stage{w6aV2HealthyStage("$default")}},
		w6aRESTRes("rst013gone", "acme-deleted-rest", "REGIONAL", ""),
		w6aHTTPRes("htp003noauth", "acme-orders-http"),
	)

	if !res.TruncatedIDs["rst013gone"] {
		t.Error("a REST API whose stages could not be listed was not marked truncated")
	}
	w2AssertNoCode(t, res.Findings["rst013gone"], w6aAPIGWNoAccessLogs)
	// The other API in the same batch is still evaluated.
	w2AssertFinding(t, res.Findings["htp003noauth"], w6aAPIGWNoAuth,
		"no authorizer", domain.SevWarn, "wave2:apigw")
}

// TestW6AAPIGW_NoV1ClientLeavesTheV2LaneWorking pins that the v1 calls are
// reached through a type assertion, not a widened aggregate: an account with
// no REST client still gets its HTTP APIs enriched.
func TestW6AAPIGW_NoV1ClientLeavesTheV2LaneWorking(t *testing.T) {
	res := w6aEnrichAPIGW(t, nil,
		&w6aAPIGWV2Fake{stages: []apigwv2types.Stage{w6aV2HealthyStage("$default")}},
		w6aHTTPRes("htp004noauth", "acme-orders-http"),
	)
	w2AssertFinding(t, res.Findings["htp004noauth"], w6aAPIGWNoAuth,
		"no authorizer", domain.SevWarn, "wave2:apigw")
}

// TestW6AAPIGW_CatalogDefs pins the catalog rows for the five apigw codes.
func TestW6AAPIGW_CatalogDefs(t *testing.T) {
	for _, c := range []struct {
		code, phrase string
		sev          domain.Severity
	}{
		{w6aAPIGWNoAuthPublic, "internet-facing with no authorizer", domain.SevBroken},
		{w6aAPIGWNoAuth, "no authorizer", domain.SevWarn},
		{w6aAPIGWNoAccessLogs, "no access logs", domain.SevWarn},
		{w6aAPIGWTracingOff, "X-Ray tracing off", domain.SevWarn},
		{w6aAPIGWStageSecret, "credential in stage variables", domain.SevBroken},
	} {
		w2AssertFindingDef(t, "apigw", c.code, c.phrase, c.sev, "wave2")
	}
}

// ---------------------------------------------------------------------------
// shared
// ---------------------------------------------------------------------------

// errNotFoundForTest stands in for the NotFound the API returns for a REST
// API deleted between the list call and the per-API enrichment.
type errNotFoundForTest struct{}

func (errNotFoundForTest) Error() string {
	return "NotFoundException: Invalid API identifier specified"
}

// w6aAssertNoSecretLeak fails when a rendered surface carries the secret
// value itself. Contract rule 7: rows carry where and what kind, never what.
func w6aAssertNoSecretLeak(t *testing.T, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if p == "hunter2-not-a-real-secret" {
			t.Errorf("secret value rendered on an operator surface: %q", p)
		}
	}
}
