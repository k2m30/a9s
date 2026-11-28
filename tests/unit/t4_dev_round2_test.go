package unit

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

// The policy AWS writes for a NONE function URL since October 2025: the URL
// action, and InvokeFunction limited to calls made through the URL.
func t4r2DefaultURLPolicy(name string) string {
	arn := "arn:aws:lambda:us-east-1:123456789012:function:" + name
	return `{"Version":"2012-10-17","Statement":[` +
		`{"Sid":"FunctionURLAllowPublicAccess","Effect":"Allow","Principal":"*","Action":"lambda:InvokeFunctionUrl","Resource":"` + arn + `","Condition":{"StringEquals":{"lambda:FunctionUrlAuthType":"NONE"}}},` +
		`{"Sid":"FunctionURLInvokeAllowPublicAccess","Effect":"Allow","Principal":"*","Action":"lambda:InvokeFunction","Resource":"` + arn + `","Condition":{"Bool":{"lambda:InvokedViaFunctionUrl":"true"}}}]}`
}

func TestT4R2_Lambda_TwoStatementURLDefaultIsTheURLGrant(t *testing.T) {
	const name = "acme-url-default"
	withURL := pw1EnrichLambda(t, &pw1LambdaPostureFake{
		policies:   map[string]string{name: t4r2DefaultURLPolicy(name)},
		urlConfigs: map[string][]lambdatypes.FunctionUrlConfig{name: t4OpenURL("r2aaaa1111")},
	}, name)
	if got := t4LambdaExposures(withURL, name); len(got) != 1 || got[0].Code != pw1LambdaCodeURLPublic {
		t.Errorf("with a NONE URL: exposure findings = %+v, want exactly %s", got, pw1LambdaCodeURLPublic)
	}
	deleted := pw1EnrichLambda(t, &pw1LambdaPostureFake{policies: map[string]string{name: t4r2DefaultURLPolicy(name)}}, name)
	if got := t4LambdaExposures(deleted, name); len(got) != 0 {
		t.Errorf("URL deleted, both statements left: exposure findings = %+v, want none", got)
	}
}

// t4r2QualifiedLambda answers GetPolicy per qualifier, as Lambda does.
type t4r2QualifiedLambda struct {
	*pw1LambdaPostureFake
	aliasPolicies map[string]string
}

func (f *t4r2QualifiedLambda) GetPolicy(ctx context.Context, in *lambda.GetPolicyInput, o ...func(*lambda.Options)) (*lambda.GetPolicyOutput, error) {
	if q := aws.ToString(in.Qualifier); q != "" {
		if doc, ok := f.aliasPolicies[q]; ok {
			return &lambda.GetPolicyOutput{Policy: aws.String(doc)}, nil
		}
		return nil, &lambdatypes.ResourceNotFoundException{Message: aws.String("The resource you requested does not exist.")}
	}
	return f.pw1LambdaPostureFake.GetPolicy(ctx, in, o...)
}

func TestT4R2_Lambda_AliasURLIsJudgedByTheAliasPolicy(t *testing.T) {
	const name = "acme-aliased"
	aliasURL := []lambdatypes.FunctionUrlConfig{{
		FunctionUrl: aws.String("https://r2bbbb2222.lambda-url.us-east-1.on.aws/"),
		FunctionArn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + name + ":live"),
		AuthType:    lambdatypes.FunctionUrlAuthTypeNone,
	}}
	run := func(unqualified, alias map[string]string) awsclient.IssueEnricherResult {
		fake := &t4r2QualifiedLambda{pw1LambdaPostureFake: &pw1LambdaPostureFake{
			policies:   unqualified,
			urlConfigs: map[string][]lambdatypes.FunctionUrlConfig{name: aliasURL},
		}, aliasPolicies: alias}
		res, err := awsclient.EnrichLambdaPosture(context.Background(), &awsclient.ServiceClients{Lambda: fake}, []resource.Resource{pw1LambdaResource(name)}, nil)
		if err != nil {
			t.Fatalf("EnrichLambdaPosture: %v", err)
		}
		return res
	}
	open := run(nil, map[string]string{"live": t4r2DefaultURLPolicy(name + ":live")})
	if got := t4LambdaExposures(open, name); len(got) != 1 || got[0].Code != pw1LambdaCodeURLPublic {
		t.Errorf("alias URL with its grant on the alias: exposure findings = %+v, want %s", got, pw1LambdaCodeURLPublic)
	}
	w2AssertRow(t, open.AttentionDetails[name][pw1LambdaCodeURLPublic].Rows, "Alias", "live")

	elsewhere := run(map[string]string{name: pw1FunctionURLPolicy(name, "NONE")}, nil)
	if got := t4LambdaExposures(elsewhere, name); len(got) != 0 {
		t.Errorf("grant on the function, URL on the alias: exposure findings = %+v, want none", got)
	}
}

func TestT4R2_APIGW_RowCountsTheOpenMethodsNotTheAuthorizers(t *testing.T) {
	rest := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{
			authorizers: []apigwtypes.Authorizer{{Id: aws.String("cog01"), Type: apigwtypes.AuthorizerTypeCognitoUserPools}},
			stages:      []apigwtypes.Stage{w6aV1Stage("prod")},
		},
		&w6aAPIGWV2Fake{},
		w6aRESTRes("rstr2open", "acme-open-rest", "REGIONAL", ""),
	)
	w2AssertRow(t, w2Rows(t, rest, "rstr2open", w6aAPIGWNoAuthPublic), "Open methods", "3")

	http := w6aEnrichAPIGW(t, nil, &w6aAPIGWV2Fake{stages: []apigwv2types.Stage{w6aV2HealthyStage("prod")}}, w6aHTTPRes("htpr2open", "acme-open-http"))
	w2AssertRow(t, w2Rows(t, http, "htpr2open", w6aAPIGWNoAuth), "Open routes", "2")
}

func TestT4R2_APIGW_UndeployedAPIRaisesNoExposure(t *testing.T) {
	draft := w6aV1Stage("draft")
	draft.DeploymentId = nil
	rest := w6aEnrichAPIGW(t, &w6aAPIGWV1Fake{stages: []apigwtypes.Stage{draft}}, &w6aAPIGWV2Fake{},
		w6aRESTRes("rstr2draft", "acme-draft-rest", "EDGE", ""))
	if !hasCode(rest.Findings["rstr2draft"], t4APIGWNoDeployedStage) || hasCode(rest.Findings["rstr2draft"], w6aAPIGWNoAuthPublic) {
		t.Errorf("REST undeployed, methods NONE: findings = %v, want %s alone", w2Codes(rest.Findings["rstr2draft"]), t4APIGWNoDeployedStage)
	}
	pending := w6aV2HealthyStage("$default")
	pending.DeploymentId = nil
	http := w6aEnrichAPIGW(t, nil, &w6aAPIGWV2Fake{stages: []apigwv2types.Stage{pending}}, w6aHTTPRes("htpr2draft", "acme-draft-http"))
	if !hasCode(http.Findings["htpr2draft"], t4APIGWNoDeployedStage) || hasCode(http.Findings["htpr2draft"], w6aAPIGWNoAuth) {
		t.Errorf("HTTP undeployed, routes NONE: findings = %v, want %s alone", w2Codes(http.Findings["htpr2draft"]), t4APIGWNoDeployedStage)
	}
}

type t4r2RESTTransport struct {
	mu     sync.Mutex
	sent   []time.Time
	limits []string
}

func (h *t4r2RESTTransport) Do(req *http.Request) (*http.Response, error) {
	body := `{"item":[{"stageName":"prod","deploymentId":"d1e2f3","tracingEnabled":true,"accessLogSettings":{"destinationArn":"arn:aws:logs:us-east-1:123456789012:log-group:acme","format":"$context.requestId"}}]}`
	if strings.Contains(req.URL.Path, "/resources") {
		h.mu.Lock()
		h.sent = append(h.sent, time.Now())
		h.limits = append(h.limits, req.URL.Query().Get("limit"))
		h.mu.Unlock()
		body = `{"item":[{"id":"r00t01","path":"/","resourceMethods":{"GET":{"httpMethod":"GET","authorizationType":"AWS_IAM"}}}]}`
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
}

// "GetResources | 5 requests every 2 seconds per account | can't be
// increased": a pass over more REST APIs than that sends no more than five in
// its first two seconds, each asking for the largest page.
func TestT4R2_APIGW_GetResourcesStaysInsideTheAccountQuota(t *testing.T) {
	transport := &t4r2RESTTransport{}
	v1 := apigateway.NewFromConfig(aws.Config{
		Region:      "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider("AKIAFAKE00000EXAMPLE", "fakeSecretAccessKeyFakeSecretAccessKey00", ""),
		HTTPClient:  transport,
		Retryer:     func() aws.Retryer { return aws.NopRetryer{} },
	})
	var rows []resource.Resource
	for i := range 8 {
		rows = append(rows, w6aRESTRes(fmt.Sprintf("rstr2q%04d", i), fmt.Sprintf("acme-q%d", i), "REGIONAL", ""))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	clients := &awsclient.ServiceClients{APIGatewayV1: v1, APIGatewayV2: &w6aAPIGWV2Fake{}}
	_, _ = w2Enricher(t, "apigw")(ctx, clients, rows, nil) //nolint:errcheck // the deadline cuts the pass short on purpose
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if len(transport.sent) != 5 {
		t.Errorf("GetResources sent %d times inside 1.5s, want 5", len(transport.sent))
	}
	for _, l := range transport.limits {
		if l != "500" {
			t.Errorf("GetResources limit = %q, want 500", l)
		}
	}
}

func TestT4R2_Trail_AForeignLogBucketIsJudgedOnItsOwnBlock(t *testing.T) {
	trails, err := awsclient.FetchCloudTrailTrails(context.Background(), t562TrailListFake{})
	if err != nil {
		t.Fatalf("FetchCloudTrailTrails: %v", err)
	}
	bucket := trails[0].Fields["s3_bucket"]
	account := &t4S3Account{block: t4AccountBlock(false, true, false, false)}
	run := func(listed ...string) awsclient.IssueEnricherResult {
		cache := resource.ResourceCache{"s3": {Resources: t4Buckets(listed...)}}
		res, err := awsclient.EnrichTrailLogBucket(context.Background(), t4S3Clients(&t562S3Fake{grants: t4AllUsersRead()}, account), trails, cache)
		if err != nil {
			t.Fatalf("EnrichTrailLogBucket: %v", err)
		}
		return res
	}
	if foreign := run("acme-other-bucket"); !hasCode(foreign.Findings[trails[0].ID], awsclient.CodeTrailLogBucketPublic) {
		t.Errorf("bucket absent from the account's complete bucket list: findings = %v, want %s", foreign.Findings[trails[0].ID], awsclient.CodeTrailLogBucketPublic)
	}
	if own := run(bucket); hasCode(own.Findings[trails[0].ID], awsclient.CodeTrailLogBucketPublic) {
		t.Errorf("bucket on the account's list, the account ignores public ACLs: findings = %v, want none", own.Findings[trails[0].ID])
	}
}

func TestT4R2_Identity_ACancelledReadIsNotKept(t *testing.T) {
	store := session.NewIdentityStore()
	c := &awsclient.ServiceClients{
		S3:        &t562S3Fake{grants: t4AllUsersRead()},
		S3Control: &t4S3Account{},
		STS: sts.NewFromConfig(aws.Config{
			Region:      "us-east-1",
			Credentials: credentials.NewStaticCredentialsProvider("AKIAFAKE00000EXAMPLE", "fakeSecretAccessKeyFakeSecretAccessKey00", ""),
			HTTPClient:  &ctxCaptureHTTPClient{},
			Retryer:     func() aws.Retryer { return aws.NopRetryer{} },
		}),
	}
	c.SetIdentityStore(store)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _ = awsclient.EnrichS3Posture(ctx, c, t4Buckets(t4Bucket), nil) //nolint:errcheck // the cancelled pass is the input
	if store.Err() != nil || store.AccountID() != "" {
		t.Errorf("after a cancelled pass: identity = (%q, %v), want nothing kept", store.AccountID(), store.Err())
	}
	_, _ = awsclient.EnrichS3Posture(context.Background(), c, t4Buckets(t4Bucket), nil) //nolint:errcheck // the refused STS read is the input
	if store.Err() == nil {
		t.Error("after a refused STS read: identity error = nil, want it kept for the session")
	}
}

type t4r2HeldAccount struct {
	release chan struct{}
	mu      sync.Mutex
	calls   int
}

func (f *t4r2HeldAccount) GetPublicAccessBlock(_ context.Context, _ *s3control.GetPublicAccessBlockInput, _ ...func(*s3control.Options)) (*s3control.GetPublicAccessBlockOutput, error) {
	f.mu.Lock()
	f.calls++
	first := f.calls == 1
	f.mu.Unlock()
	if first {
		<-f.release
		return &s3control.GetPublicAccessBlockOutput{PublicAccessBlockConfiguration: t4AccountBlock(true, true, true, true)}, nil
	}
	return (&t4S3Account{}).GetPublicAccessBlock(context.Background(), &s3control.GetPublicAccessBlockInput{AccountId: aws.String("123456789012")})
}

func TestT4R2_Rotation_ALateReadDoesNotFillTheNewStore(t *testing.T) {
	account := &t4r2HeldAccount{release: make(chan struct{})}
	clients := t4S3Clients(&t562S3Fake{grants: t4AllUsersRead()}, nil)
	clients.S3Control = account
	sess := session.New()
	sess.WireStores(clients)
	identityResolved(clients)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = awsclient.EnrichS3Posture(context.Background(), clients, t4Buckets(t4Bucket), nil) //nolint:errcheck // the pre-rotation pass
	}()
	for {
		account.mu.Lock()
		started := account.calls == 1
		account.mu.Unlock()
		if started {
			break
		}
		time.Sleep(time.Millisecond)
	}
	sess.Rotate()
	sess.WireStores(clients)
	identityResolved(clients)
	close(account.release)
	<-done

	res := t4EnrichS3(t, clients, t4Bucket)
	if !hasCode(res.Findings[t4Bucket], t4S3Public) {
		t.Errorf("after rotation: findings = %v, want %s from the new account's read", res.Findings[t4Bucket], t4S3Public)
	}
	if account.calls != 2 {
		t.Errorf("account block read %d times, want 2: the read in flight at rotation filled only the old store", account.calls)
	}
}
