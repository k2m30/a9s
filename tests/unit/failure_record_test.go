package unit

// failure_record_test.go — a failed call is recorded once, as a class and a
// cause, never as a string.
//
//   - the recorder stores the id, the class and the cause read off the
//     error's own fields; the aggregate groups on them. A denial carrying a
//     request id reaches the log as the action, the count and one example id,
//     and the class survives the aggregate instead of reclassifying as
//     Unknown downstream.
//   - one classifier. Every site that reads a code asks the named predicate
//     rather than smithy.APIError, and the predicate answers for a modeled
//     SDK error, a generic one, and a wrapped chain alike.

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// deniedRequestID is the per-call token AWS stamps on every response. It is
// noise in a failure line: it differs on every call, so it would put each
// failure in a group of its own and push the actionable text off the line.
const deniedRequestID = "11111111-2222-3333-4444-555555555555"

// deniedAs builds the error shape the SDK hands a fetcher for a denied call:
// the operation preamble and the request id in the wrapper's own words, the
// code and the message in the API error's fields.
func deniedAs(service, operation, action string) error {
	return fmt.Errorf("operation error %s: %s, https response error StatusCode: 400, RequestID: %s, api error AccessDeniedException: %w",
		service, operation, deniedRequestID,
		&MockAPIError{
			Code: "AccessDeniedException",
			Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/a9s is not authorized to perform: " +
				action + " on resource: arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders",
		})
}

// assertOneFailureLine asserts the composite error reads as one sentence an
// operator can act on: the action the role lacks, how many resources it
// covered, one example id, and none of the per-call noise.
func assertOneFailureLine(t *testing.T, err error, action, count, example string) {
	t.Helper()
	if err == nil {
		t.Fatal("a denied call returned no error")
	}
	got := err.Error()
	for _, want := range []string{"not authorized to perform " + action, count, example} {
		if !strings.Contains(got, want) {
			t.Errorf("failure line %q does not carry %q", got, want)
		}
	}
	for _, noise := range []string{deniedRequestID, "RequestID", "operation error", "api error", "assumed-role"} {
		if strings.Contains(got, noise) {
			t.Errorf("failure line %q carries per-call noise %q", got, noise)
		}
	}
	if class := awsclient.ErrClass(err); class != "access-denied" {
		t.Errorf("ErrClass(aggregate) = %q, want %q — the recorded class must survive the aggregate", class, "access-denied")
	}
}

// TestFetcher_DeniedPerItemCall_OneLineWithActionCountAndExample drives the
// real DynamoDB fetcher with a denied DescribeTable and reads the composite
// error every surface phrases from.
func TestFetcher_DeniedPerItemCall_OneLineWithActionCountAndExample(t *testing.T) {
	listAPI := &mockDDBListTablesClient{
		output: &dynamodb.ListTablesOutput{TableNames: []string{"acme-orders", "acme-sessions"}},
	}
	describeAPI := &mockDDBDescribeTableClient{err: deniedAs("DynamoDB", "DescribeTable", "dynamodb:DescribeTable")}

	res, err := awsclient.FetchDynamoDBTablesPage(context.Background(), listAPI, describeAPI, "")
	if len(res.Resources) != 2 {
		t.Fatalf("denied describe dropped rows: got %d, want 2 degraded rows", len(res.Resources))
	}
	assertOneFailureLine(t, err, "dynamodb:DescribeTable", "2 of 2", "acme-orders")
}

// ddbDeniedPolicyFake answers GetResourcePolicy with a denial and every other
// call with an empty response, so the enricher's only failure is the denied
// per-item call.
type ddbDeniedPolicyFake struct {
	awsclient.DynamoDBAPI
}

func (f *ddbDeniedPolicyFake) DescribeContinuousBackups(
	_ context.Context, in *dynamodb.DescribeContinuousBackupsInput, _ ...func(*dynamodb.Options),
) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	if in == nil || in.TableName == nil {
		return nil, fmt.Errorf("nil input")
	}
	return &dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
			PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusEnabled,
			},
		},
	}, nil
}

func (f *ddbDeniedPolicyFake) GetResourcePolicy(
	_ context.Context, _ *dynamodb.GetResourcePolicyInput, _ ...func(*dynamodb.Options),
) (*dynamodb.GetResourcePolicyOutput, error) {
	return nil, deniedAs("DynamoDB", "GetResourcePolicy", "dynamodb:GetResourcePolicy")
}

// TestEnricher_DeniedPerItemCall_OneLineWithActionCountAndExample drives the
// real Wave 2 DynamoDB enricher through MarkSkipped and Finish.
func TestEnricher_DeniedPerItemCall_OneLineWithActionCountAndExample(t *testing.T) {
	clients := &awsclient.ServiceClients{DynamoDB: &ddbDeniedPolicyFake{}}
	resources := []resource.Resource{
		{ID: "acme-orders", Name: "acme-orders", Type: "ddb", Fields: map[string]string{"arn": "arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders"}},
		{ID: "acme-sessions", Name: "acme-sessions", Type: "ddb", Fields: map[string]string{"arn": "arn:aws:dynamodb:us-east-1:123456789012:table/acme-sessions"}},
	}

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources, nil)
	for _, r := range resources {
		if _, marked := result.TruncatedIDs[r.ID]; !marked {
			t.Errorf("%s: a denied check must leave the row uninspected, not clean", r.ID)
		}
	}
	assertOneFailureLine(t, err, "dynamodb:GetResourcePolicy", "2 of 2", "acme-")
}

// TestErrCodeIs_AnswersForEveryShapeTheSitesRead pins the one classifier
// against the four families the converted sites read: an absence AWS models
// as an error, a denial, a throttle, and an invalid parameter. Each is fed
// both as the modeled SDK type and as the generic shape, wrapped the way a
// fetcher wraps it.
func TestErrCodeIs_AnswersForEveryShapeTheSitesRead(t *testing.T) {
	for _, tc := range []struct {
		name  string
		err   error
		code  string
		match bool
		class string
	}{
		// IAM's modeled exception answers a code that is NOT its type name; a
		// site that matched only the type name would miss the wire shape and
		// one that matched only the code would miss the modeled one.
		{"modeled not-found type", &iamtypes.NoSuchEntityException{}, "NoSuchEntity", true, "NoSuchEntity"},
		{"modeled not-found type, exception spelling", &iamtypes.NoSuchEntityException{}, "NoSuchEntityException", false, "NoSuchEntity"},
		{"modeled efs absence type", &efstypes.PolicyNotFound{}, "PolicyNotFound", true, "PolicyNotFound"},
		{"generic not-found code", &MockAPIError{Code: "PolicyNotFoundException"}, "PolicyNotFoundException", true, "PolicyNotFoundException"},
		{"wrapped absence code", fmt.Errorf("reading policy: %w", &MockAPIError{Code: "NoSuchBucketPolicy"}), "NoSuchBucketPolicy", true, "NoSuchBucketPolicy"},
		{"invalid parameter", &MockAPIError{Code: "InvalidLaunchTemplateId.NotFound"}, "InvalidLaunchTemplateId.NotFound", true, "InvalidLaunchTemplateId.NotFound"},
		{"denied is not an absence", deniedAs("DynamoDB", "DescribeTable", "dynamodb:DescribeTable"), "PolicyNotFoundException", false, "access-denied"},
		{"throttle is not an absence", &MockAPIError{Code: "ThrottlingException"}, "PolicyNotFoundException", false, "throttled"},
		{"plain error carries no code", fmt.Errorf("connection reset"), "PolicyNotFoundException", false, "Unknown"},
		{"no error", nil, "PolicyNotFoundException", false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := awsclient.ErrCodeIs(tc.err, tc.code); got != tc.match {
				t.Errorf("ErrCodeIs(%v, %q) = %v, want %v", tc.err, tc.code, got, tc.match)
			}
			if got := awsclient.ErrClass(tc.err); got != tc.class {
				t.Errorf("ErrClass(%v) = %q, want %q", tc.err, got, tc.class)
			}
		})
	}
}

// TestErrCodeIs_MatchesAnyOfTheNamedCodes pins the multi-code form the
// cross-region pair and the S3 absence set need: one call, several codes.
func TestErrCodeIs_MatchesAnyOfTheNamedCodes(t *testing.T) {
	pair := []string{"PermanentRedirect", "IllegalLocationConstraintException"}
	for _, code := range pair {
		if !awsclient.ErrCodeIs(&MockAPIError{Code: code}, pair...) {
			t.Errorf("ErrCodeIs(%q, cross-region pair) = false, want true", code)
		}
	}
	if awsclient.ErrCodeIs(&MockAPIError{Code: "NoSuchBucket"}, pair...) {
		t.Error("ErrCodeIs(NoSuchBucket, cross-region pair) = true, want false")
	}
}

// TestFailedCall_RecordsIDClassAndCause pins that the recorder stores the
// three fields the aggregate groups and phrases on, read off the error rather
// than off a rendered string.
func TestFailedCall_RecordsIDClassAndCause(t *testing.T) {
	f := awsclient.FailedCall("acme-orders", deniedAs("DynamoDB", "DescribeTable", "dynamodb:DescribeTable"))
	if f.ID != "acme-orders" {
		t.Errorf("ID = %q, want %q", f.ID, "acme-orders")
	}
	if f.Class != "access-denied" {
		t.Errorf("Class = %q, want %q", f.Class, "access-denied")
	}
	if f.Cause != "not authorized to perform dynamodb:DescribeTable" {
		t.Errorf("Cause = %q, want the action the role lacks", f.Cause)
	}

	// An answer with no error to classify still records a cause: a resource
	// the service returned nothing usable for was not inspected either.
	u := awsclient.UnusableAnswer("acme-sessions", "nil table in response")
	if u.ID != "acme-sessions" || u.Cause != "nil table in response" {
		t.Errorf("UnusableAnswer = %+v, want the id and the stated cause", u)
	}
}

// TestAggregateFailures_MixedClasses_KeepsEachCause pins that two different
// denied actions stay two causes: grouping on the recorded cause never lets
// one action speak for another. A mixed-class aggregate carries no class of
// its own — there is no single answer to give.
func TestAggregateFailures_MixedClasses_KeepsEachCause(t *testing.T) {
	err := awsclient.AggregateFailures("ddb FetchByIDs", []awsclient.Failure{
		awsclient.FailedCall("acme-orders", deniedAs("DynamoDB", "DescribeTable", "dynamodb:DescribeTable")),
		awsclient.FailedCall("acme-sessions", deniedAs("DynamoDB", "ListTagsOfResource", "dynamodb:ListTagsOfResource")),
		awsclient.UnusableAnswer("acme-events", "nil table in response"),
	}, 4)
	if err == nil {
		t.Fatal("AggregateFailures returned nil for three failures")
	}
	got := err.Error()
	for _, want := range []string{
		"not authorized to perform dynamodb:DescribeTable",
		"not authorized to perform dynamodb:ListTagsOfResource",
		"nil table in response",
		"3 of 4",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("aggregate %q does not carry %q", got, want)
		}
	}
	if class := awsclient.ErrClass(err); class == "access-denied" {
		t.Errorf("ErrClass(mixed aggregate) = %q — a mixed aggregate must not claim one class", class)
	}
}

// TestNamedPredicates_ReadWhatTheSitesUsedToMatch pins the two predicates the
// converted sites needed by name: a refusal, whichever of the several codes
// AWS spells it with, and the deleted-between-list-and-describe race.
func TestNamedPredicates_ReadWhatTheSitesUsedToMatch(t *testing.T) {
	for _, tc := range []struct {
		name             string
		err              error
		denied, notFound bool
	}{
		{"bare denial code", &MockAPIError{Code: "AccessDenied"}, true, false},
		{"exception denial code", &MockAPIError{Code: "AccessDeniedException"}, true, false},
		{"denial wrapped by the fetcher", deniedAs("DynamoDB", "DescribeTable", "dynamodb:DescribeTable"), true, false},
		{"throttle is not a denial", &MockAPIError{Code: "ThrottlingException"}, false, false},
		{"modeled gone-resource type", &r53types.NoSuchHostedZone{}, false, true},
		{"generic gone-resource code", &MockAPIError{Code: "ResourceNotFoundException"}, false, true},
		{"deleted instance", &MockAPIError{Code: "InvalidInstanceID.NotFound"}, false, true},
		{"no error", nil, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := awsclient.IsAccessDenied(tc.err); got != tc.denied {
				t.Errorf("IsAccessDenied = %v, want %v", got, tc.denied)
			}
			if got := awsclient.IsNotFoundErr(tc.err); got != tc.notFound {
				t.Errorf("IsNotFoundErr = %v, want %v", got, tc.notFound)
			}
		})
	}
}

// TestAggregateFailures_ClassWithAPhrase_KeepsTheWholeSentence pins that
// carrying the class out of the aggregate does not let the class speak in the
// composite's place: "timeout" alone drops the count and the example the
// operator needs. Found by probing the classes whose table entry supplies a
// phrase (timeout, tls, dns, transport, region-unavailable) rather than the
// denial class, which supplies none.
func TestAggregateFailures_ClassWithAPhrase_KeepsTheWholeSentence(t *testing.T) {
	agg := awsclient.AggregateFailures("ebs-snap-enrich", []awsclient.Failure{
		awsclient.FailedCall("snap-0001", context.DeadlineExceeded),
		awsclient.FailedCall("snap-0002", context.DeadlineExceeded),
	}, 9)
	for _, want := range []string{"2 of 9", "timeout", "snap-0001"} {
		if !strings.Contains(awsclient.CauseOf(agg), want) {
			t.Errorf("CauseOf(aggregate) = %q, want it to carry %q", awsclient.CauseOf(agg), want)
		}
	}
	if class := awsclient.ErrClass(agg); class != "timeout" {
		t.Errorf("ErrClass(aggregate) = %q, want %q", class, "timeout")
	}
}

// TestAggregateFailures_RegionGap_NamesTheRegionOnce pins that a composite
// whose records already name the region is not decorated with it a second
// time. Found by probing the region-unavailable class against the carried
// class, which is the one class CauseInRegion acts on.
func TestAggregateFailures_RegionGap_NamesTheRegionOnce(t *testing.T) {
	gap := &smithy.OperationError{
		ServiceID: "CodeArtifact", OperationName: "ListRepositories",
		Err: &url.Error{Op: "Post", URL: "https://codeartifact.eu-central-2.amazonaws.com/",
			Err: &net.DNSError{Err: "no such host", Name: "codeartifact.eu-central-2.amazonaws.com", IsNotFound: true}},
	}
	agg := awsclient.AggregateFailures("availability-prefetch",
		[]awsclient.Failure{awsclient.FailedCallInRegion("codeartifact", gap, "eu-central-2")}, 1)
	line := awsclient.CauseInRegion(agg, "eu-central-2")
	if n := strings.Count(line, "eu-central-2"); n != 1 {
		t.Errorf("failure line %q names the region %d times, want 1", line, n)
	}
}

// iamDeniedKeysFake answers every user's access-key listing with a denial and
// leaves the other calls healthy, so the enricher's only failure is the one
// call it could not make.
type iamDeniedKeysFake struct {
	awsclient.IAMAPI
}

func (f *iamDeniedKeysFake) GetLoginProfile(
	_ context.Context, _ *iam.GetLoginProfileInput, _ ...func(*iam.Options),
) (*iam.GetLoginProfileOutput, error) {
	return nil, &iamtypes.NoSuchEntityException{Message: aws.String("Login Profile for user cannot be found")}
}

func (f *iamDeniedKeysFake) ListMFADevices(
	_ context.Context, _ *iam.ListMFADevicesInput, _ ...func(*iam.Options),
) (*iam.ListMFADevicesOutput, error) {
	return &iam.ListMFADevicesOutput{}, nil
}

func (f *iamDeniedKeysFake) ListAccessKeys(
	_ context.Context, _ *iam.ListAccessKeysInput, _ ...func(*iam.Options),
) (*iam.ListAccessKeysOutput, error) {
	return nil, deniedAs("IAM", "ListAccessKeys", "iam:ListAccessKeys")
}

func (f *iamDeniedKeysFake) ListAttachedUserPolicies(
	_ context.Context, _ *iam.ListAttachedUserPoliciesInput, _ ...func(*iam.Options),
) (*iam.ListAttachedUserPoliciesOutput, error) {
	return &iam.ListAttachedUserPoliciesOutput{}, nil
}

// TestEnricher_UninspectedUser_SaysWhy: a user whose access keys could not
// be read renders "?" AND says what refused. Marking the row without
// recording the error is the silent-skip shape.
func TestEnricher_UninspectedUser_SaysWhy(t *testing.T) {
	clients := &awsclient.ServiceClients{IAM: &iamDeniedKeysFake{}, Region: "us-east-1"}
	resources := []resource.Resource{
		{ID: "acme-deploy", Name: "acme-deploy", Type: "iam-user",
			Fields: map[string]string{"user_name": "acme-deploy", "create_date": "2020-01-01 00:00", "password_last_used": "Never"}},
	}

	result, err := awsclient.EnrichIAMUserMFA(context.Background(), clients, resources, nil)
	if _, marked := result.TruncatedIDs["acme-deploy"]; !marked {
		t.Error("a user whose keys could not be read must render \"?\"")
	}
	if err == nil {
		t.Fatal("a denied ListAccessKeys returned no error — the reason never reaches the log")
	}
	for _, want := range []string{"not authorized to perform iam:ListAccessKeys", "acme-deploy"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("failure line %q does not carry %q", err, want)
		}
	}
}

// TestEnricher_TableWithNoPolicy_IsNotAFailure pins that an empty policy
// response is an answer, not a refusal: DynamoDB reports "no resource policy"
// as PolicyNotFoundException, but a response carrying no document at all must
// not be recorded as a parse failure — the row would say a check failed when
// nothing did. Found by driving the status-bar pin's fake.
func TestEnricher_TableWithNoPolicy_IsNotAFailure(t *testing.T) {
	clients := &awsclient.ServiceClients{DynamoDB: &ddbEmptyPolicyFake{}}
	resources := []resource.Resource{
		{ID: "acme-orders", Name: "acme-orders", Type: "ddb", Fields: map[string]string{"arn": "arn:aws:dynamodb:us-east-1:123456789012:table/acme-orders"}},
	}

	result, err := awsclient.EnrichDynamoDBPITR(context.Background(), clients, resources, nil)
	if err != nil {
		t.Errorf("a table with no resource policy reported a failure: %v", err)
	}
	if _, marked := result.TruncatedIDs["acme-orders"]; marked {
		t.Error("a table with no resource policy was marked uninspected")
	}
}

// ddbEmptyPolicyFake answers every call, with no resource policy document.
type ddbEmptyPolicyFake struct {
	awsclient.DynamoDBAPI
}

func (f *ddbEmptyPolicyFake) DescribeContinuousBackups(
	_ context.Context, _ *dynamodb.DescribeContinuousBackupsInput, _ ...func(*dynamodb.Options),
) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	return &dynamodb.DescribeContinuousBackupsOutput{
		ContinuousBackupsDescription: &ddbtypes.ContinuousBackupsDescription{
			PointInTimeRecoveryDescription: &ddbtypes.PointInTimeRecoveryDescription{
				PointInTimeRecoveryStatus: ddbtypes.PointInTimeRecoveryStatusEnabled,
			},
		},
	}, nil
}

func (f *ddbEmptyPolicyFake) GetResourcePolicy(
	_ context.Context, _ *dynamodb.GetResourcePolicyInput, _ ...func(*dynamodb.Options),
) (*dynamodb.GetResourcePolicyOutput, error) {
	return &dynamodb.GetResourcePolicyOutput{}, nil
}
