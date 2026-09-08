package unit

// wipfix_cap_independence_test.go — a page cap on ONE walk must never suppress
// a check that did not depend on that walk.
//
// Rows 5, 6 and the apigw site of row 6's sweep. The shared shape: an
// enricher counts something by walking pages, the walk hits PerParentPageCap,
// and the enricher then records the whole resource in TruncatedIDs.
// FoldWave2Rows (core/runtime/helpers.go) skips every id in that map, so the
// findings the enricher DID determine — from a separate API call that has
// nothing to do with the capped walk — never reach the row and a resolved one
// can never clear. The count itself already says it is a lower bound: the "+"
// resource.FormatTruncated puts on it.

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	apigwv2types "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// findingCodes returns the codes emitted for id, for readable failures.
func findingCodes(findings []domain.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, string(f.Code))
	}
	return out
}

func hasFindingCode(findings []domain.Finding, code string) bool {
	for _, f := range findings {
		if string(f.Code) == code {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Row 6 — sns: a capped subscription walk must not stop the topic posture read
// ---------------------------------------------------------------------------

// snsCappedSubsFake always answers ListSubscriptionsByTopic with a NextToken,
// so the walk runs into PerParentPageCap, and reports the topic as
// unencrypted (no KmsMasterKeyId) — a fact GetTopicAttributes alone decides.
type snsCappedSubsFake struct {
	awsclient.SNSAPI
	mu        sync.Mutex
	listCalls int
	attrCalls int
}

func (f *snsCappedSubsFake) ListSubscriptionsByTopic(
	_ context.Context,
	in *sns.ListSubscriptionsByTopicInput,
	_ ...func(*sns.Options),
) (*sns.ListSubscriptionsByTopicOutput, error) {
	f.mu.Lock()
	f.listCalls++
	n := f.listCalls
	f.mu.Unlock()
	arn := ""
	if in != nil && in.TopicArn != nil {
		arn = *in.TopicArn
	}
	return &sns.ListSubscriptionsByTopicOutput{
		Subscriptions: []snstypes.Subscription{{
			TopicArn:        aws.String(arn),
			SubscriptionArn: aws.String(fmt.Sprintf("%s:sub-%d", arn, n)),
			Protocol:        aws.String("sqs"),
		}},
		NextToken: aws.String(fmt.Sprintf("page-%d", n)),
	}, nil
}

func (f *snsCappedSubsFake) GetTopicAttributes(
	_ context.Context,
	in *sns.GetTopicAttributesInput,
	_ ...func(*sns.Options),
) (*sns.GetTopicAttributesOutput, error) {
	f.mu.Lock()
	f.attrCalls++
	f.mu.Unlock()
	arn := ""
	if in != nil && in.TopicArn != nil {
		arn = *in.TopicArn
	}
	// No KmsMasterKeyId at all: AWS omits the attribute for an unencrypted
	// topic, which is what sns.no-kms is read from.
	return &sns.GetTopicAttributesOutput{Attributes: map[string]string{
		"TopicArn": arn,
		"Policy":   `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":"arn:aws:iam::123456789012:root"},"Action":"SNS:Publish","Resource":"` + arn + `"}]}`,
	}}, nil
}

// TestEnrichSNS_CappedSubscriptionWalkStillReadsPosture pins row 6: a topic
// with more subscription pages than PerParentPageCap is still read for its own
// posture — encryption and access policy come from GetTopicAttributes, which
// the subscription walk's completeness has no bearing on.
func TestEnrichSNS_CappedSubscriptionWalkStillReadsPosture(t *testing.T) {
	const name = "acme-busy-topic"
	arn := "arn:aws:sns:us-east-1:123456789012:" + name
	fake := &snsCappedSubsFake{}
	rows := []resource.Resource{{
		ID:     arn,
		Name:   name,
		Fields: map[string]string{"topic_name": name, "topic_arn": arn},
	}}

	result, err := awsclient.EnrichSNSSubscriptions(context.Background(),
		&awsclient.ServiceClients{SNS: fake}, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.listCalls != awsclient.PerParentPageCap {
		t.Fatalf("ListSubscriptionsByTopic called %d times, want %d (the walk must still be capped)",
			fake.listCalls, awsclient.PerParentPageCap)
	}
	if fake.attrCalls != 1 {
		t.Errorf("GetTopicAttributes called %d times, want 1 — posture must be read for a topic whose subscription walk was capped", fake.attrCalls)
	}
	if !hasFindingCode(result.Findings[arn], "sns.no-kms") {
		t.Errorf("findings for the capped topic = %v, want sns.no-kms — an unencrypted topic stays unencrypted however many subscribers it has",
			findingCodes(result.Findings[arn]))
	}
	if result.TruncatedIDs[arn] {
		t.Errorf("TruncatedIDs[%q] = true — a subscriber-count cap must not mark the row uninspected: FoldWave2Rows skips every id in this map, so the posture findings above would never reach the row", arn)
	}
	if got := result.FieldUpdates[arn]["subs_count"]; !strings.HasSuffix(got, "+") {
		t.Errorf("subs_count = %q, want a %q-suffixed lower bound — the count is where the cap is reported", got, "+")
	}
}

// ---------------------------------------------------------------------------
// Row 5 — codeartifact: a package-count cap must not suppress the policy verdict
// ---------------------------------------------------------------------------

// caCappedPackagesFake always answers ListPackages with a NextToken (the count
// walk runs into the cap) and returns a repository policy open to everyone.
type caCappedPackagesFake struct {
	awsclient.CodeArtifactAPI
	mu       sync.Mutex
	pkgCalls int
}

func (f *caCappedPackagesFake) ListPackages(
	_ context.Context,
	_ *codeartifact.ListPackagesInput,
	_ ...func(*codeartifact.Options),
) (*codeartifact.ListPackagesOutput, error) {
	f.mu.Lock()
	f.pkgCalls++
	n := f.pkgCalls
	f.mu.Unlock()
	return &codeartifact.ListPackagesOutput{
		Packages:  []codeartifacttypes.PackageSummary{{Package: aws.String(fmt.Sprintf("pkg-%d", n))}},
		NextToken: aws.String(fmt.Sprintf("page-%d", n)),
	}, nil
}

func (f *caCappedPackagesFake) GetRepositoryPermissionsPolicy(
	_ context.Context,
	_ *codeartifact.GetRepositoryPermissionsPolicyInput,
	_ ...func(*codeartifact.Options),
) (*codeartifact.GetRepositoryPermissionsPolicyOutput, error) {
	return &codeartifact.GetRepositoryPermissionsPolicyOutput{
		Policy: &codeartifacttypes.ResourcePolicy{
			Document: aws.String(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"codeartifact:ReadFromRepository","Resource":"*"}]}`),
		},
	}, nil
}

var _ awsclient.CodeArtifactListPackagesAPI = (*caCappedPackagesFake)(nil)

// TestEnrichCodeArtifact_CappedPackageCountKeepsPolicyVerdict pins row 5: the
// package count is informational and its cap is reported by the "+" on the
// count; the permissions-policy verdict is a separate call that completed and
// must reach the row.
func TestEnrichCodeArtifact_CappedPackageCountKeepsPolicyVerdict(t *testing.T) {
	const repoID = "acme-domain/acme-repo"
	fake := &caCappedPackagesFake{}
	rows := []resource.Resource{{
		ID:   repoID,
		Name: "acme-repo",
		Fields: map[string]string{
			"repository_name": "acme-repo",
			"domain_name":     "acme-domain",
			"domain_owner":    "123456789012",
		},
	}}

	result, err := awsclient.EnrichCodeArtifactRepository(context.Background(),
		&awsclient.ServiceClients{CodeArtifact: fake}, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.pkgCalls != awsclient.PerParentPageCap {
		t.Fatalf("ListPackages called %d times, want %d (the walk must still be capped)",
			fake.pkgCalls, awsclient.PerParentPageCap)
	}
	if len(result.Findings[repoID]) == 0 {
		t.Errorf("no findings for %q — the public-policy verdict came from GetRepositoryPermissionsPolicy, which the package-count cap says nothing about", repoID)
	}
	if result.TruncatedIDs[repoID] {
		t.Errorf("TruncatedIDs[%q] = true — an informational count cap must not mark the row uninspected: FoldWave2Rows would drop the policy finding above", repoID)
	}
	if got := result.FieldUpdates[repoID]["package_count"]; !strings.HasSuffix(got, "+") {
		t.Errorf("package_count = %q, want a %q-suffixed lower bound — the count is where the cap is reported", got, "+")
	}
}

// ---------------------------------------------------------------------------
// Row 6's sweep — apigw: a capped stage walk must not suppress the authorizer check
// ---------------------------------------------------------------------------

// apigwCappedStagesFake always answers GetStages with a NextToken and reports
// an HTTP API with no authorizers at all — a fact GetAuthorizers alone decides.
type apigwCappedStagesFake struct {
	awsclient.APIGatewayV2API
	mu          sync.Mutex
	stageCalls  int
	authzCalls  int
	stageOutput []apigwv2types.Stage
}

func (f *apigwCappedStagesFake) GetApis(
	_ context.Context,
	_ *apigatewayv2.GetApisInput,
	_ ...func(*apigatewayv2.Options),
) (*apigatewayv2.GetApisOutput, error) {
	return &apigatewayv2.GetApisOutput{}, nil
}

func (f *apigwCappedStagesFake) GetStages(
	_ context.Context,
	_ *apigatewayv2.GetStagesInput,
	_ ...func(*apigatewayv2.Options),
) (*apigatewayv2.GetStagesOutput, error) {
	f.mu.Lock()
	f.stageCalls++
	n := f.stageCalls
	f.mu.Unlock()
	return &apigatewayv2.GetStagesOutput{
		Items:     append([]apigwv2types.Stage(nil), f.stageOutput...),
		NextToken: aws.String(fmt.Sprintf("page-%d", n)),
	}, nil
}

func (f *apigwCappedStagesFake) GetAuthorizers(
	_ context.Context,
	_ *apigatewayv2.GetAuthorizersInput,
	_ ...func(*apigatewayv2.Options),
) (*apigatewayv2.GetAuthorizersOutput, error) {
	f.mu.Lock()
	f.authzCalls++
	f.mu.Unlock()
	return &apigatewayv2.GetAuthorizersOutput{}, nil
}

// TestEnrichAPIGateway_CappedStageWalkKeepsAuthorizerFinding pins row 6's
// sweep at the apigw site: the stage count cap is informational and reported
// by the "+" on stages_count; the no-authorizer verdict came from
// GetAuthorizers and must reach the row.
func TestEnrichAPIGateway_CappedStageWalkKeepsAuthorizerFinding(t *testing.T) {
	const apiID = "acme-http-api"
	fake := &apigwCappedStagesFake{stageOutput: []apigwv2types.Stage{{StageName: aws.String("prod")}}}
	rows := []resource.Resource{{
		ID:     apiID,
		Name:   "acme-http",
		Fields: map[string]string{"protocol": "HTTP", "name": "acme-http"},
	}}

	result, err := awsclient.EnrichAPIGatewayStage(context.Background(),
		&awsclient.ServiceClients{APIGatewayV2: fake}, rows, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.stageCalls != awsclient.PerParentPageCap {
		t.Fatalf("GetStages called %d times, want %d (the walk must still be capped)",
			fake.stageCalls, awsclient.PerParentPageCap)
	}
	if fake.authzCalls != 1 {
		t.Errorf("GetAuthorizers called %d times, want 1", fake.authzCalls)
	}
	if len(result.Findings[apiID]) == 0 {
		t.Errorf("no findings for %q — an API with zero authorizers is unauthenticated whatever its stage count", apiID)
	}
	if result.TruncatedIDs[apiID] {
		t.Errorf("TruncatedIDs[%q] = true — an informational stage-count cap must not mark the row uninspected: FoldWave2Rows would drop the authorizer finding above", apiID)
	}
	if got := result.FieldUpdates[apiID]["stages_count"]; !strings.HasSuffix(got, "+") {
		t.Errorf("stages_count = %q, want a %q-suffixed lower bound — the count is where the cap is reported", got, "+")
	}
}
