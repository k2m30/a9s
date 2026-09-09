package unit

// prowler_w1_lambda_test.go — behavioural pins for the lambda posture signals
// of batch w1: public-policy, function-url-public and env-secret.
//
// public-policy and function-url-public are the lambda type's first Wave 2
// enricher (EnrichLambdaPosture); env-secret is computed in the fetcher from
// the ListFunctions response, which already carries the environment.
//
// The policy assertions are written against iampolicy's verdict, not against
// the text of a document: a conditioned wildcard grant, a single-object
// Statement and a URL-encoded document all have to be handled, and a
// substring match over the raw JSON gets each of them wrong.

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	pw1LambdaCodePublicPolicy = domain.FindingCode("lambda.public-policy")
	pw1LambdaCodeURLPublic    = domain.FindingCode("lambda.function-url-public")
	pw1LambdaCodeEnvSecret    = domain.FindingCode("lambda.env-secret")
)

// ─── fakes ──────────────────────────────────────────────────────────────────

// pw1LambdaListFake serves ListFunctions for the wave-1 fetcher path.
type pw1LambdaListFake struct {
	functions []lambdatypes.FunctionConfiguration
}

func (f *pw1LambdaListFake) ListFunctions(_ context.Context, _ *lambda.ListFunctionsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	return &lambda.ListFunctionsOutput{Functions: f.functions}, nil
}

// pw1LambdaPostureFake serves GetPolicy and ListFunctionUrlConfigs for
// EnrichLambdaPosture.
type pw1LambdaPostureFake struct {
	awsclient.LambdaAPI
	policies   map[string]string                          // function name → policy document
	policyErr  map[string]error                           // function name → GetPolicy error
	urlConfigs map[string][]lambdatypes.FunctionUrlConfig // function name → URL configs
	urlErr     map[string]error

	// EnrichLambdaPosture fans this fake out through ForEachParallel.
	mu        sync.Mutex
	getPolicy []string
}

func (f *pw1LambdaPostureFake) GetPolicy(_ context.Context, in *lambda.GetPolicyInput, _ ...func(*lambda.Options)) (*lambda.GetPolicyOutput, error) {
	name := aws.ToString(in.FunctionName)
	f.mu.Lock()
	f.getPolicy = append(f.getPolicy, name)
	f.mu.Unlock()
	if err, ok := f.policyErr[name]; ok {
		return nil, err
	}
	doc, ok := f.policies[name]
	if !ok {
		return nil, &lambdatypes.ResourceNotFoundException{
			Message: aws.String("The resource you requested does not exist."),
		}
	}
	return &lambda.GetPolicyOutput{Policy: aws.String(doc)}, nil
}

func (f *pw1LambdaPostureFake) ListFunctionUrlConfigs(_ context.Context, in *lambda.ListFunctionUrlConfigsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionUrlConfigsOutput, error) {
	name := aws.ToString(in.FunctionName)
	if err, ok := f.urlErr[name]; ok {
		return nil, err
	}
	return &lambda.ListFunctionUrlConfigsOutput{FunctionUrlConfigs: f.urlConfigs[name]}, nil
}

// GetFunction is the stub half of a partial test double: this fake embeds
// LambdaAPI as a nil interface and implements only the two posture reads. The
// enricher now asks this one to tell a function with no resource policy from a
// function that is gone, which GetPolicy answers with the same code. Every
// function in these scenarios exists.
func (f *pw1LambdaPostureFake) GetFunction(_ context.Context, in *lambda.GetFunctionInput, _ ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	return &lambda.GetFunctionOutput{
		Configuration: &lambdatypes.FunctionConfiguration{FunctionName: in.FunctionName},
	}, nil
}

// pw1LambdaFn builds a realistic healthy ListFunctions entry: active, current
// runtime, a dead-letter queue, and an environment that references its
// credential rather than carrying it.
func pw1LambdaFn(name string, env map[string]string) lambdatypes.FunctionConfiguration {
	fn := lambdatypes.FunctionConfiguration{
		FunctionName:     aws.String(name),
		FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + name),
		Runtime:          lambdatypes.RuntimePython312,
		Handler:          aws.String("app.handler"),
		MemorySize:       aws.Int32(512),
		Timeout:          aws.Int32(30),
		CodeSize:         2_400_000,
		State:            lambdatypes.StateActive,
		LastUpdateStatus: lambdatypes.LastUpdateStatusSuccessful,
		LastModified:     aws.String("2026-07-01T12:00:00.000+0000"),
		PackageType:      lambdatypes.PackageTypeZip,
		DeadLetterConfig: &lambdatypes.DeadLetterConfig{
			TargetArn: aws.String("arn:aws:sqs:us-east-1:123456789012:acme-dlq"),
		},
	}
	if env != nil {
		fn.Environment = &lambdatypes.EnvironmentResponse{Variables: env}
	}
	return fn
}

func pw1FetchLambdas(t *testing.T, fns ...lambdatypes.FunctionConfiguration) []resource.Resource {
	t.Helper()
	out, err := awsclient.FetchLambdaFunctionsPage(context.Background(), &pw1LambdaListFake{functions: fns}, "")
	if err != nil {
		t.Fatalf("FetchLambdaFunctionsPage: %v", err)
	}
	return out.Resources
}

func pw1LambdaResource(name string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"function_name": name,
			"arn":           "arn:aws:lambda:us-east-1:123456789012:function:" + name,
			"state":         "Active",
		},
	}
}

func pw1EnrichLambda(t *testing.T, fake *pw1LambdaPostureFake, names ...string) awsclient.IssueEnricherResult {
	t.Helper()
	rs := make([]resource.Resource, 0, len(names))
	for _, n := range names {
		rs = append(rs, pw1LambdaResource(n))
	}
	res, err := awsclient.EnrichLambdaPosture(context.Background(), &awsclient.ServiceClients{Lambda: fake}, rs, nil)
	if err != nil && len(fake.policyErr) == 0 && len(fake.urlErr) == 0 {
		t.Fatalf("EnrichLambdaPosture: %v", err)
	}
	return res
}

// ─── row 12: lambda.public-policy ───────────────────────────────────────────

const pw1PublicInvokePolicy = `{
  "Version": "2012-10-17",
  "Id": "default",
  "Statement": [
    {
      "Sid": "AllowPublicInvoke",
      "Effect": "Allow",
      "Principal": "*",
      "Action": "lambda:InvokeFunction",
      "Resource": "arn:aws:lambda:us-east-1:123456789012:function:acme-open"
    }
  ]
}`

// pw1ConditionedPolicy scopes the same wildcard principal to one source
// account — the shape AWS itself generates when an S3 bucket or an API
// Gateway stage is wired to a function.
const pw1ConditionedPolicy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "AllowFromApiGateway",
      "Effect": "Allow",
      "Principal": "*",
      "Action": "lambda:InvokeFunction",
      "Resource": "arn:aws:lambda:us-east-1:123456789012:function:acme-scoped",
      "Condition": {"StringEquals": {"aws:SourceAccount": "123456789012"}}
    }
  ]
}`

// TestLambda_PublicPolicy_WildcardPrincipal pins the Broken finding and the
// two rows an operator needs: who is allowed, and to do what.
func TestLambda_PublicPolicy_WildcardPrincipal(t *testing.T) {
	const name = "acme-open"
	fake := &pw1LambdaPostureFake{policies: map[string]string{name: pw1PublicInvokePolicy}}
	res := pw1EnrichLambda(t, fake, name)

	pw1RequireFinding(t, res.Findings[name], pw1LambdaCodePublicPolicy,
		"invokable by anyone", domain.SevBroken, "wave2")
	rows := pw1Rows(res, name, pw1LambdaCodePublicPolicy)
	pw1RequireRow(t, rows, "Principal", "*")
	pw1RequireRow(t, rows, "Actions", "lambda:InvokeFunction")
}

// TestLambda_PublicPolicy_ConditionedGrantIsHealthy pins the verdict the
// shared policy engine defines: a wildcard principal scoped by a restrictive
// condition is a scoped grant, not a public function. A substring match for
// "Principal":"*" over the document reports this one as public.
func TestLambda_PublicPolicy_ConditionedGrantIsHealthy(t *testing.T) {
	const name = "acme-scoped"
	fake := &pw1LambdaPostureFake{policies: map[string]string{name: pw1ConditionedPolicy}}
	res := pw1EnrichLambda(t, fake, name)
	pw1RequireNoFinding(t, res.Findings[name], pw1LambdaCodePublicPolicy)
}

// TestLambda_PublicPolicy_SingleStatementObject pins that a policy whose
// Statement is a bare object rather than an array is evaluated. AWS accepts
// both shapes and the IAM console emits the object form.
func TestLambda_PublicPolicy_SingleStatementObject(t *testing.T) {
	const name = "acme-object-stmt"
	doc := `{"Version":"2012-10-17","Statement":{"Effect":"Allow","Principal":"*","Action":"lambda:InvokeFunctionUrl","Resource":"*"}}`
	fake := &pw1LambdaPostureFake{policies: map[string]string{name: doc}}
	res := pw1EnrichLambda(t, fake, name)
	pw1RequireFinding(t, res.Findings[name], pw1LambdaCodePublicPolicy,
		"invokable by anyone", domain.SevBroken, "wave2")
}

// TestLambda_PublicPolicy_URLEncodedDocument pins that a URL-encoded policy
// document is decoded before evaluation. Several IAM read APIs return the
// document in that form, and a raw json.Unmarshal fails on it.
func TestLambda_PublicPolicy_URLEncodedDocument(t *testing.T) {
	const name = "acme-encoded"
	// Path-style escaping, which is what AWS emits: a document is percent-encoded
	// per RFC 3986 and a literal '+' stays a '+'. Inverted from url.QueryEscape,
	// whose form-style '+' for space leaves the JSON unparsable — do not restore it.
	fake := &pw1LambdaPostureFake{policies: map[string]string{
		name: url.PathEscape(pw1PublicInvokePolicy),
	}}
	res := pw1EnrichLambda(t, fake, name)
	pw1RequireFinding(t, res.Findings[name], pw1LambdaCodePublicPolicy,
		"invokable by anyone", domain.SevBroken, "wave2")
}

// TestLambda_PublicPolicy_CrossAccountGrantIsNotPublic pins that naming
// another account is not the same as naming everyone: this batch defines no
// cross-account row for lambda, so a named foreign principal is silent.
func TestLambda_PublicPolicy_CrossAccountGrantIsNotPublic(t *testing.T) {
	const name = "acme-partner"
	doc := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::210987654321:root"},"Action":"lambda:InvokeFunction","Resource":"*"}]}`
	fake := &pw1LambdaPostureFake{policies: map[string]string{name: doc}}
	res := pw1EnrichLambda(t, fake, name)
	pw1RequireNoFinding(t, res.Findings[name], pw1LambdaCodePublicPolicy)
}

// TestLambda_PublicPolicy_NoPolicyIsHealthyNotAFailure pins the common case:
// most functions have no resource policy at all, and the
// ResourceNotFoundException that reports it is neither a finding nor a
// truncation.
func TestLambda_PublicPolicy_NoPolicyIsHealthyNotAFailure(t *testing.T) {
	const name = "acme-nopolicy"
	fake := &pw1LambdaPostureFake{}
	res, err := awsclient.EnrichLambdaPosture(context.Background(),
		&awsclient.ServiceClients{Lambda: fake}, []resource.Resource{pw1LambdaResource(name)}, nil)
	if err != nil {
		t.Fatalf("a function without a resource policy must not fail the enricher: %v", err)
	}
	pw1RequireNoFinding(t, res.Findings[name], pw1LambdaCodePublicPolicy)
	if _, marked := res.TruncatedIDs[name]; marked {
		t.Errorf("%s marked truncated for having no resource policy", name)
	}
	if res.Truncated {
		t.Errorf("Truncated set for a function with no resource policy")
	}
}

// TestLambda_PublicPolicy_ErrorMarksOnlyThatFunction pins the partial-failure
// contract across a batch.
func TestLambda_PublicPolicy_ErrorMarksOnlyThatFunction(t *testing.T) {
	const bad = "acme-denied"
	const good = "acme-open"
	fake := &pw1LambdaPostureFake{
		policies:  map[string]string{good: pw1PublicInvokePolicy},
		policyErr: map[string]error{bad: &lambdatypes.ServiceException{Message: aws.String("internal error")}},
	}
	res := pw1EnrichLambda(t, fake, bad, good)

	if _, marked := res.TruncatedIDs[bad]; !marked {
		t.Errorf("TruncatedIDs missing %s after a failed GetPolicy", bad)
	}
	pw1RequireFinding(t, res.Findings[good], pw1LambdaCodePublicPolicy,
		"invokable by anyone", domain.SevBroken, "wave2")
	if _, marked := res.TruncatedIDs[good]; marked {
		t.Errorf("healthy neighbour %s wrongly marked truncated", good)
	}
}

// TestLambda_Posture_NilClientReturnsEmptyResult pins that the enricher is a
// no-op before the Lambda client exists, with every reference map non-nil.
func TestLambda_Posture_NilClientReturnsEmptyResult(t *testing.T) {
	res, err := awsclient.EnrichLambdaPosture(context.Background(),
		&awsclient.ServiceClients{}, []resource.Resource{pw1LambdaResource("acme-open")}, nil)
	if err != nil {
		t.Fatalf("nil Lambda client must not error: %v", err)
	}
	if res.Findings == nil || res.TruncatedIDs == nil {
		t.Errorf("reference maps must be non-nil on success: %+v", res)
	}
	if len(res.Findings) != 0 {
		t.Errorf("findings emitted without a client: %+v", res.Findings)
	}
}

// TestLambda_Posture_CapBoundsTheWalk pins the cap boundary for an enricher
// that emits "!" findings: past the cap the issue count is a lower bound.
func TestLambda_Posture_CapBoundsTheWalk(t *testing.T) {
	fake := &pw1LambdaPostureFake{}
	names := make([]string, 0, awsclient.EnrichmentCap+1)
	for i := 0; i <= awsclient.EnrichmentCap; i++ {
		names = append(names, "acme-fn-"+strings.Repeat("x", i%3)+pw1Digits(i))
	}
	res := pw1EnrichLambda(t, fake, names...)
	if !res.Truncated {
		t.Errorf("Truncated = false for %d functions; EnrichmentCap is %d", len(names), awsclient.EnrichmentCap)
	}
	if len(fake.getPolicy) > awsclient.EnrichmentCap {
		t.Errorf("GetPolicy called %d times, cap is %d", len(fake.getPolicy), awsclient.EnrichmentCap)
	}
}

// pw1Digits renders n as a zero-padded three-digit suffix.
func pw1Digits(n int) string {
	const d = "0123456789"
	return string([]byte{d[(n/100)%10], d[(n/10)%10], d[n%10]})
}

// ─── row 13: lambda.function-url-public ─────────────────────────────────────

// TestLambda_FunctionURLPublic_AuthTypeNone pins the Broken finding: a
// function URL with AuthType NONE is an unauthenticated HTTPS endpoint.
func TestLambda_FunctionURLPublic_AuthTypeNone(t *testing.T) {
	const name = "acme-url-open"
	fake := &pw1LambdaPostureFake{urlConfigs: map[string][]lambdatypes.FunctionUrlConfig{
		name: {{
			FunctionUrl: aws.String("https://abc123.lambda-url.us-east-1.on.aws/"),
			AuthType:    lambdatypes.FunctionUrlAuthTypeNone,
		}},
	}}
	res := pw1EnrichLambda(t, fake, name)

	pw1RequireFinding(t, res.Findings[name], pw1LambdaCodeURLPublic,
		"function endpoint open without authentication", domain.SevBroken, "wave2")
	pw1RequireRow(t, pw1Rows(res, name, pw1LambdaCodeURLPublic), "Endpoint auth", "none")
}

// TestLambda_FunctionURLPublic_WildcardCORSIsCited pins the extra row: an open
// endpoint that also allows any origin is callable from any page.
func TestLambda_FunctionURLPublic_WildcardCORSIsCited(t *testing.T) {
	const name = "acme-url-cors"
	fake := &pw1LambdaPostureFake{urlConfigs: map[string][]lambdatypes.FunctionUrlConfig{
		name: {{
			FunctionUrl: aws.String("https://def456.lambda-url.us-east-1.on.aws/"),
			AuthType:    lambdatypes.FunctionUrlAuthTypeNone,
			Cors:        &lambdatypes.Cors{AllowOrigins: []string{"*"}},
		}},
	}}
	res := pw1EnrichLambda(t, fake, name)
	pw1RequireRow(t, pw1Rows(res, name, pw1LambdaCodeURLPublic), "Allowed origins", "any")
}

// TestLambda_FunctionURLPublic_ScopedCORSIsNotCited pins that a named origin
// list does not produce the CORS row.
func TestLambda_FunctionURLPublic_ScopedCORSIsNotCited(t *testing.T) {
	const name = "acme-url-scoped-cors"
	fake := &pw1LambdaPostureFake{urlConfigs: map[string][]lambdatypes.FunctionUrlConfig{
		name: {{
			FunctionUrl: aws.String("https://ghi789.lambda-url.us-east-1.on.aws/"),
			AuthType:    lambdatypes.FunctionUrlAuthTypeNone,
			Cors:        &lambdatypes.Cors{AllowOrigins: []string{"https://app.example.com"}},
		}},
	}}
	res := pw1EnrichLambda(t, fake, name)
	for _, r := range pw1Rows(res, name, pw1LambdaCodeURLPublic) {
		if r.Label == "Allowed origins" {
			t.Errorf("CORS row emitted for a scoped origin list: %+v", r)
		}
	}
}

// TestLambda_FunctionURLPublic_IAMAuthIsHealthy pins the negative case.
func TestLambda_FunctionURLPublic_IAMAuthIsHealthy(t *testing.T) {
	const name = "acme-url-iam"
	fake := &pw1LambdaPostureFake{urlConfigs: map[string][]lambdatypes.FunctionUrlConfig{
		name: {{
			FunctionUrl: aws.String("https://jkl012.lambda-url.us-east-1.on.aws/"),
			AuthType:    lambdatypes.FunctionUrlAuthTypeAwsIam,
			Cors:        &lambdatypes.Cors{AllowOrigins: []string{"*"}},
		}},
	}}
	res := pw1EnrichLambda(t, fake, name)
	pw1RequireNoFinding(t, res.Findings[name], pw1LambdaCodeURLPublic)
}

// TestLambda_FunctionURLPublic_NoURLConfigIsHealthy pins that a function with
// no URL at all emits nothing.
func TestLambda_FunctionURLPublic_NoURLConfigIsHealthy(t *testing.T) {
	const name = "acme-no-url"
	res := pw1EnrichLambda(t, &pw1LambdaPostureFake{}, name)
	pw1RequireNoFinding(t, res.Findings[name], pw1LambdaCodeURLPublic)
}

// TestLambda_PolicyAndURLAreTwoFindings pins independence: a function that is
// both publicly invokable and fronted by an unauthenticated URL carries two
// findings, each with its own code and its own rows.
func TestLambda_PolicyAndURLAreTwoFindings(t *testing.T) {
	const name = "acme-open"
	fake := &pw1LambdaPostureFake{
		policies: map[string]string{name: pw1PublicInvokePolicy},
		urlConfigs: map[string][]lambdatypes.FunctionUrlConfig{
			name: {{
				FunctionUrl: aws.String("https://mno345.lambda-url.us-east-1.on.aws/"),
				AuthType:    lambdatypes.FunctionUrlAuthTypeNone,
			}},
		},
	}
	res := pw1EnrichLambda(t, fake, name)
	pw1RequireFinding(t, res.Findings[name], pw1LambdaCodePublicPolicy,
		"invokable by anyone", domain.SevBroken, "wave2")
	pw1RequireFinding(t, res.Findings[name], pw1LambdaCodeURLPublic,
		"function endpoint open without authentication", domain.SevBroken, "wave2")
	if len(pw1Rows(res, name, pw1LambdaCodePublicPolicy)) == 0 {
		t.Errorf("policy rows crowded out by the URL finding's rows")
	}
	if len(pw1Rows(res, name, pw1LambdaCodeURLPublic)) == 0 {
		t.Errorf("URL rows crowded out by the policy finding's rows")
	}
}

// ─── row 14: lambda.env-secret ──────────────────────────────────────────────

// TestLambda_EnvSecret_PlaintextVariable pins the Broken finding and the
// Where:Kind row, and that the value never reaches the finding text.
func TestLambda_EnvSecret_PlaintextVariable(t *testing.T) {
	const name = "acme-leaky"
	rs := pw1FetchLambdas(t, pw1LambdaFn(name, map[string]string{
		"APP_ENV":     "production",
		"DB_PASSWORD": "hunter2hunter2",
	}))
	r := pw1ResourceByID(t, rs, name)

	f := pw1RequireFinding(t, r.Findings, pw1LambdaCodeEnvSecret,
		"credential in environment variables", domain.SevBroken, "wave1")
	pw1RequireRow(t, r.AttentionDetails[pw1LambdaCodeEnvSecret].Rows, "DB_PASSWORD", "keyword")
	if strings.Contains(f.Phrase+f.Detail, "hunter2hunter2") {
		t.Errorf("credential value leaked into the finding text")
	}
	for _, row := range r.AttentionDetails[pw1LambdaCodeEnvSecret].Rows {
		if strings.Contains(row.Label+row.Value, "hunter2hunter2") {
			t.Errorf("credential value leaked into an AttentionDetail row: %+v", row)
		}
	}
}

// TestLambda_EnvSecret_ReferenceIsHealthy pins the negative case: an
// environment that points at Secrets Manager is the recommended shape.
func TestLambda_EnvSecret_ReferenceIsHealthy(t *testing.T) {
	const name = "acme-clean"
	rs := pw1FetchLambdas(t, pw1LambdaFn(name, map[string]string{
		"APP_ENV":         "production",
		"DB_PASSWORD_ARN": "arn:aws:secretsmanager:us-east-1:123456789012:secret:acme/db-AbCdEf",
		"TABLE_NAME":      "acme-orders",
	}))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, pw1LambdaCodeEnvSecret)
}

// TestLambda_EnvSecret_NoEnvironmentIsHealthy pins that an absent Environment
// block emits nothing.
func TestLambda_EnvSecret_NoEnvironmentIsHealthy(t *testing.T) {
	const name = "acme-noenv"
	rs := pw1FetchLambdas(t, pw1LambdaFn(name, nil))
	pw1RequireNoFinding(t, pw1ResourceByID(t, rs, name).Findings, pw1LambdaCodeEnvSecret)
}

// TestLambda_EnvSecret_AccessKeyIsReportedByItsOwnKind pins that a structured
// credential is classified as such rather than as a keyword match.
func TestLambda_EnvSecret_AccessKeyIsReportedByItsOwnKind(t *testing.T) {
	const name = "acme-akid"
	rs := pw1FetchLambdas(t, pw1LambdaFn(name, map[string]string{
		"BACKUP_KEY": "AKIAIOSFODNN7EXAMPLE",
	}))
	r := pw1ResourceByID(t, rs, name)
	pw1RequireFinding(t, r.Findings, pw1LambdaCodeEnvSecret,
		"credential in environment variables", domain.SevBroken, "wave1")
	pw1RequireRow(t, r.AttentionDetails[pw1LambdaCodeEnvSecret].Rows, "BACKUP_KEY", "aws-access-key")
}

// TestLambda_EnvSecret_CoexistsWithLifecycleFinding pins independence against
// the fetcher's exclusive lifecycle switch: a function with no dead-letter
// queue that also leaks a credential must carry both findings.
func TestLambda_EnvSecret_CoexistsWithLifecycleFinding(t *testing.T) {
	const name = "acme-nodlq-leaky"
	fn := pw1LambdaFn(name, map[string]string{"DB_PASSWORD": "hunter2hunter2"})
	fn.DeadLetterConfig = nil
	rs := pw1FetchLambdas(t, fn)
	r := pw1ResourceByID(t, rs, name)

	pw1RequireFinding(t, r.Findings, pw1LambdaCodeEnvSecret,
		"credential in environment variables", domain.SevBroken, "wave1")
	if _, ok := pw1FindFinding(r.Findings, domain.FindingCode("lambda.dlq.missing")); !ok {
		t.Errorf("lambda.dlq.missing lost when the env-secret finding was added: %+v", r.Findings)
	}
}

// ─── demo bench ─────────────────────────────────────────────────────────────

// TestLambda_DemoBench_EachSignalHasExactlyOneWitness pins the demo fixture
// contract for the three lambda signals.
func TestLambda_DemoBench_EachSignalHasExactlyOneWitness(t *testing.T) {
	out, err := awsclient.FetchLambdaFunctionsPage(context.Background(), fakes.NewLambda(), "")
	if err != nil {
		t.Fatalf("FetchLambdaFunctionsPage(demo): %v", err)
	}
	var envCarriers []string
	for _, r := range out.Resources {
		if _, ok := pw1FindFinding(r.Findings, pw1LambdaCodeEnvSecret); ok {
			envCarriers = append(envCarriers, r.ID)
		}
	}
	pw1RequireOnlyWitness(t, pw1LambdaCodeEnvSecret, fixtures.LambdaEnvSecret, envCarriers)

	res, eerr := awsclient.EnrichLambdaPosture(context.Background(),
		&awsclient.ServiceClients{Lambda: fakes.NewLambda()}, out.Resources, nil)
	if eerr != nil {
		t.Fatalf("EnrichLambdaPosture(demo): %v", eerr)
	}
	for code, witness := range map[domain.FindingCode]string{
		pw1LambdaCodePublicPolicy: fixtures.LambdaPublicPolicy,
		pw1LambdaCodeURLPublic:    fixtures.LambdaFunctionURLPublic,
	} {
		var carriers []string
		for id, fs := range res.Findings {
			if _, ok := pw1FindFinding(fs, code); ok {
				carriers = append(carriers, id)
			}
		}
		pw1RequireOnlyWitness(t, code, witness, carriers)
	}
}

// TestLambda_EnvSecret_StillReportedOnFailedOrInactiveFunction is deliberately
// the inverse of the deleted-resource rule, and stays that way.
//
// Common contract rule 4 silences posture findings on deleted, terminated and
// deleting resources. Lambda has no such state: a deleted function is simply
// absent from ListFunctions. "Failed" means the last create or update did not
// apply and "Inactive" means the function was evicted from memory after idle
// time — both still exist, and both still hand their environment to anyone
// who can call lambda:GetFunctionConfiguration. Silencing the credential leak
// there would hide a live exposure behind a lifecycle state, so if this test
// is ever inverted, the reason must be that lambda gained a real deleted
// state.
func TestLambda_EnvSecret_StillReportedOnFailedOrInactiveFunction(t *testing.T) {
	for _, state := range []lambdatypes.State{lambdatypes.StateFailed, lambdatypes.StateInactive} {
		name := "acme-" + strings.ToLower(string(state))
		fn := pw1LambdaFn(name, map[string]string{"DB_PASSWORD": "hunter2hunter2"})
		fn.State = state
		rs := pw1FetchLambdas(t, fn)
		r := pw1ResourceByID(t, rs, name)
		if _, ok := pw1FindFinding(r.Findings, pw1LambdaCodeEnvSecret); !ok {
			t.Errorf("state %q: the credential is still readable, so lambda.env-secret must still fire: %+v",
				state, r.Findings)
		}
	}
}
