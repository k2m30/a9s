// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wipfix_qa_aws_blind_test.go reproduces the reviewer's three fetch-side
// scenarios literally:
//
//   - a CodeArtifact repository whose package count hit its page cap, beside a
//     permissions-policy check that completed;
//   - an SNS topic whose subscription walk hit its page cap, beside the
//     posture read that needs only GetTopicAttributes;
//   - two continuations of the merged API Gateway cursor, where the V2 lane
//     reported exhaustion on the first.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	apigatewaytypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	snssvc "github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// --- row 5: a capped package count beside a completed policy check ---------

const wipfixOpenRepoPolicy = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":["codeartifact:ReadFromRepository"],"Resource":"*"}]}`

// caEndlessPackagesFake never stops offering another page of packages, so the
// count walk can only end at PerParentPageCap. The permissions policy is a
// different call and answers straight away.
type caEndlessPackagesFake struct {
	awsclient.CodeArtifactAPI
	pkgCalls    int
	policyCalls int
}

func (f *caEndlessPackagesFake) ListPackages(
	_ context.Context, _ *codeartifact.ListPackagesInput, _ ...func(*codeartifact.Options),
) (*codeartifact.ListPackagesOutput, error) {
	f.pkgCalls++
	return &codeartifact.ListPackagesOutput{
		Packages:  []codeartifacttypes.PackageSummary{{Package: aws.String("example-lib")}},
		NextToken: aws.String("more"),
	}, nil
}

func (f *caEndlessPackagesFake) GetRepositoryPermissionsPolicy(
	_ context.Context, _ *codeartifact.GetRepositoryPermissionsPolicyInput, _ ...func(*codeartifact.Options),
) (*codeartifact.GetRepositoryPermissionsPolicyOutput, error) {
	f.policyCalls++
	return &codeartifact.GetRepositoryPermissionsPolicyOutput{
		Policy: &codeartifacttypes.ResourcePolicy{Document: aws.String(wipfixOpenRepoPolicy)},
	}, nil
}

func wipfixCARows() []resource.Resource {
	return []resource.Resource{{
		ID:   "example-domain/example-repo",
		Name: "example-repo",
		Fields: map[string]string{
			"repo_name":   "example-repo",
			"domain_name": "example-domain",
		},
	}}
}

// TestEnrichCodeArtifact_CappedPackageCountKeepsThePolicyVerdict pins row 5:
// the package count is informational and its cap is reported by the "+" on
// the count. The permissions-policy verdict comes from a different call the
// count says nothing about, and marking the row uninspected drops it.
func TestEnrichCodeArtifact_CappedPackageCountKeepsThePolicyVerdict(t *testing.T) {
	fake := &caEndlessPackagesFake{}
	res, err := awsclient.EnrichCodeArtifactRepository(context.Background(),
		&awsclient.ServiceClients{CodeArtifact: fake}, wipfixCARows(), nil)
	if err != nil {
		t.Fatalf("EnrichCodeArtifactRepository returned an error for a page-capped count: %v", err)
	}
	if fake.pkgCalls != awsclient.PerParentPageCap {
		t.Fatalf("precondition: ListPackages called %d times, want the cap %d",
			fake.pkgCalls, awsclient.PerParentPageCap)
	}
	if fake.policyCalls != 1 {
		t.Fatalf("precondition: GetRepositoryPermissionsPolicy called %d times, want 1", fake.policyCalls)
	}

	id := "example-domain/example-repo"
	if res.TruncatedIDs[id] {
		t.Errorf("TruncatedIDs[%s] = true — an informational count cap must not mark the row "+
			"uninspected: FoldWave2Rows skips every id in this map and the policy finding "+
			"below never reaches the row", id)
	}
	if !hasFindingCode(res.Findings[id], "codeartifact.public-access-policy") {
		t.Errorf("findings for the capped repository = %v, want codeartifact.public-access-policy — "+
			"a repository open to anyone stays open however many packages it holds",
			codesOf(res.Findings[id]))
	}
	if got := res.FieldUpdates[id]["package_count"]; got != "10+" {
		t.Errorf("package_count = %q, want %q — the cap reports itself as the count's lower bound",
			got, "10+")
	}

	td := resource.FindResourceType("codeartifact")
	if td == nil {
		t.Fatal("resource type codeartifact is not registered")
	}
	folded := wipfixCARows()
	runtime.FoldWave2Rows(folded, *td, res.Findings, res.AttentionDetails, res.TruncatedIDs)
	if !hasFindingCode(folded[0].Findings, "codeartifact.public-access-policy") {
		t.Errorf("after FoldWave2Rows the row carries %v, want codeartifact.public-access-policy",
			codesOf(folded[0].Findings))
	}
}

// --- row 6: a capped subscription walk beside the posture read -------------

// snsEndlessSubsFake never stops offering another page of subscriptions. The
// topic is unencrypted, which GetTopicAttributes alone establishes.
type snsEndlessSubsFake struct {
	awsclient.SNSAPI
	subCalls  int
	attrCalls int
}

func (f *snsEndlessSubsFake) ListSubscriptionsByTopic(
	_ context.Context, _ *snssvc.ListSubscriptionsByTopicInput, _ ...func(*snssvc.Options),
) (*snssvc.ListSubscriptionsByTopicOutput, error) {
	f.subCalls++
	return &snssvc.ListSubscriptionsByTopicOutput{
		Subscriptions: []snstypes.Subscription{{
			SubscriptionArn: aws.String("arn:aws:sns:us-east-1:123456789012:example-topic:11111111-2222-3333-4444-555555555555"),
			Protocol:        aws.String("sqs"),
		}},
		NextToken: aws.String("more"),
	}, nil
}

func (f *snsEndlessSubsFake) GetTopicAttributes(
	_ context.Context, _ *snssvc.GetTopicAttributesInput, _ ...func(*snssvc.Options),
) (*snssvc.GetTopicAttributesOutput, error) {
	f.attrCalls++
	return &snssvc.GetTopicAttributesOutput{Attributes: map[string]string{
		"TopicArn": wipfixSNSTopicARN,
		"Policy":   `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"123456789012"},"Action":"SNS:Publish","Resource":"*"}]}`,
	}}, nil
}

const wipfixSNSTopicARN = "arn:aws:sns:us-east-1:123456789012:example-busy-topic"

// TestEnrichSNS_CappedSubscriptionWalkStillReadsPosture pins row 6: the access
// policy and the encryption key come from GetTopicAttributes, one call that
// the subscription walk's completeness has no bearing on. A topic with many
// subscribers is exactly the topic whose posture matters most.
func TestEnrichSNS_CappedSubscriptionWalkStillReadsPosture(t *testing.T) {
	fake := &snsEndlessSubsFake{}
	rows := []resource.Resource{{ID: wipfixSNSTopicARN, Name: "example-busy-topic", Fields: map[string]string{}}}

	res, err := awsclient.EnrichSNSSubscriptions(context.Background(),
		&awsclient.ServiceClients{SNS: fake}, rows, nil)
	if err != nil {
		t.Fatalf("EnrichSNSSubscriptions returned an error for a page-capped walk: %v", err)
	}
	if fake.subCalls != awsclient.PerParentPageCap {
		t.Fatalf("precondition: ListSubscriptionsByTopic called %d times, want the cap %d",
			fake.subCalls, awsclient.PerParentPageCap)
	}

	if fake.attrCalls != 1 {
		t.Errorf("GetTopicAttributes called %d times, want 1 — posture must be read for a topic "+
			"whose subscription walk was capped", fake.attrCalls)
	}
	if !hasFindingCode(res.Findings[wipfixSNSTopicARN], "sns.no-kms") {
		t.Errorf("findings for the capped topic = %v, want sns.no-kms — an unencrypted topic "+
			"stays unencrypted however many subscribers it has",
			codesOf(res.Findings[wipfixSNSTopicARN]))
	}
	if res.TruncatedIDs[wipfixSNSTopicARN] {
		t.Errorf("TruncatedIDs[%s] = true — a subscriber-count cap must not mark the row "+
			"uninspected: FoldWave2Rows skips every id in this map, so the posture findings "+
			"above would never reach the row", wipfixSNSTopicARN)
	}
	if got := res.FieldUpdates[wipfixSNSTopicARN]["subs_count"]; got != "10+" {
		t.Errorf("subs_count = %q, want %q", got, "10+")
	}
}

// TestEnrichSNS_CappedWalkNeverClaimsAllPending is the negative half: a
// capped walk cannot rule out a confirmed subscriber on a page it never read,
// so the subscription-derived findings must stay off.
func TestEnrichSNS_CappedWalkNeverClaimsAllPending(t *testing.T) {
	res, err := awsclient.EnrichSNSSubscriptions(context.Background(),
		&awsclient.ServiceClients{SNS: &snsEndlessSubsFake{}},
		[]resource.Resource{{ID: wipfixSNSTopicARN, Fields: map[string]string{}}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, code := range []domain.FindingCode{"sns.all-pending-confirmation", "sns.no-subscribers"} {
		if hasFindingCode(res.Findings[wipfixSNSTopicARN], code) {
			t.Errorf("a topic whose subscription walk was capped raised %s — "+
				"the pages it never read could hold a confirmed subscriber", code)
		}
	}
}

// --- row 8: the merged API Gateway cursor ----------------------------------

// apigwV1ManyPagesFake hands back one REST API per call and always another
// position, so the V1 lane truncates every time and the merged fetcher keeps
// being asked to continue.
type apigwV1ManyPagesFake struct {
	awsclient.APIGatewayV1API
	calls int
}

func (f *apigwV1ManyPagesFake) GetRestApis(
	_ context.Context, _ *apigateway.GetRestApisInput, _ ...func(*apigateway.Options),
) (*apigateway.GetRestApisOutput, error) {
	f.calls++
	return &apigateway.GetRestApisOutput{
		Items:    []apigatewaytypes.RestApi{{Id: aws.String("rest-000"), Name: aws.String("example-rest")}},
		Position: aws.String("pos-next"),
	}, nil
}

// apigwV2OnePageFake reports exhaustion on its very first call: one HTTP API,
// no NextToken. Every later call is a re-walk of a finished lane.
type apigwV2OnePageFake struct {
	awsclient.APIGatewayV2API
	calls int
}

func (f *apigwV2OnePageFake) GetApis(
	_ context.Context, _ *apigatewayv2.GetApisInput, _ ...func(*apigatewayv2.Options),
) (*apigatewayv2.GetApisOutput, error) {
	f.calls++
	return &apigatewayv2.GetApisOutput{
		Items: []apigwv2types.Api{{
			ApiId: aws.String("http-only"), Name: aws.String("example-http"),
			ProtocolType: apigwv2types.ProtocolTypeHttp,
		}},
	}, nil
}

// TestFetchAPIGatewaysPageMerged_FinishedV2LaneIsNotRewalked pins row 8: once
// the V2 lane has reported exhaustion the cursor must say so. A cursor that
// can only record a token cannot distinguish "V2 finished" from "V2 starts at
// page one", so every continuation re-walks it and its rows enter the
// accumulated result again.
func TestFetchAPIGatewaysPageMerged_FinishedV2LaneIsNotRewalked(t *testing.T) {
	v1, v2 := &apigwV1ManyPagesFake{}, &apigwV2OnePageFake{}
	clients := &awsclient.ServiceClients{APIGatewayV1: v1, APIGatewayV2: v2}

	token := ""
	httpRows := 0
	for range 3 {
		res, err := awsclient.FetchAPIGatewaysPageMerged(context.Background(), clients, token)
		if err != nil {
			t.Fatalf("FetchAPIGatewaysPageMerged: %v", err)
		}
		for _, r := range res.Resources {
			if r.ID == "http-only" {
				httpRows++
			}
		}
		if res.Pagination == nil || !res.Pagination.IsTruncated {
			t.Fatalf("precondition: the merged walk finished early; the V1 lane must stay truncated")
		}
		token = res.Pagination.NextToken
	}

	if v2.calls != 1 {
		t.Errorf("GetApis called %d times across three continuations, want 1 — the V2 lane "+
			"reported exhaustion on the first call and the cursor must say so", v2.calls)
	}
	if httpRows != 1 {
		t.Errorf("the HTTP API %q was delivered %d times across three continuations, want 1 — "+
			"a re-walked lane's rows enter the accumulated result again", "http-only", httpRows)
	}
}

// TestFetchAPIGatewaysPageMerged_UnfinishedV2LaneIsResumed is the negative
// half: a V2 lane that is still truncated must continue from its own token,
// not be skipped.
func TestFetchAPIGatewaysPageMerged_UnfinishedV2LaneIsResumed(t *testing.T) {
	v1, v2 := &apigwV1ManyPagesFake{}, &apigwV2TruncatedFake{}
	clients := &awsclient.ServiceClients{APIGatewayV1: v1, APIGatewayV2: v2}

	res, err := awsclient.FetchAPIGatewaysPageMerged(context.Background(), clients, "")
	if err != nil {
		t.Fatalf("FetchAPIGatewaysPageMerged: %v", err)
	}
	if _, err := awsclient.FetchAPIGatewaysPageMerged(context.Background(), clients,
		res.Pagination.NextToken); err != nil {
		t.Fatalf("second continuation: %v", err)
	}
	if v2.calls != 2 {
		t.Errorf("GetApis called %d times, want 2 — a truncated V2 lane must be resumed", v2.calls)
	}
	if v2.lastToken != "v2-page-2" {
		t.Errorf("the second V2 call used token %q, want %q — the cursor must carry its position",
			v2.lastToken, "v2-page-2")
	}
}

type apigwV2TruncatedFake struct {
	awsclient.APIGatewayV2API
	calls     int
	lastToken string
}

func (f *apigwV2TruncatedFake) GetApis(
	_ context.Context, in *apigatewayv2.GetApisInput, _ ...func(*apigatewayv2.Options),
) (*apigatewayv2.GetApisOutput, error) {
	f.calls++
	f.lastToken = aws.ToString(in.NextToken)
	return &apigatewayv2.GetApisOutput{
		Items: []apigwv2types.Api{{
			ApiId: aws.String("http-" + aws.ToString(in.NextToken)), Name: aws.String("example-http"),
			ProtocolType: apigwv2types.ProtocolTypeHttp,
		}},
		NextToken: aws.String("v2-page-2"),
	}, nil
}
