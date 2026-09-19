package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfnsvc "github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type bkCBFake struct {
	awsclient.CodeBuildAPI
}

func (bkCBFake) ListBuildsForProject(_ context.Context, in *codebuild.ListBuildsForProjectInput, _ ...func(*codebuild.Options)) (*codebuild.ListBuildsForProjectOutput, error) {
	return &codebuild.ListBuildsForProjectOutput{Ids: []string{aws.ToString(in.ProjectName) + ":build-1"}}, nil
}

func (bkCBFake) BatchGetBuilds(context.Context, *codebuild.BatchGetBuildsInput, ...func(*codebuild.Options)) (*codebuild.BatchGetBuildsOutput, error) {
	return nil, bkOpErr("CodeBuild", "BatchGetBuilds", "AccessDeniedException")
}

// A refused BatchGetBuilds read no build, so every project it was asked about
// is not inspected rather than clean.
func TestCBBatchGetBuildsRefused_ProjectsAreMarked(t *testing.T) {
	projects := []resource.Resource{
		{ID: "acme-api-build", Name: "acme-api-build", Type: "cb"},
		{ID: "acme-web-build", Name: "acme-web-build", Type: "cb"},
	}
	res, err := awsclient.EnrichCodeBuildStatus(context.Background(), &awsclient.ServiceClients{CodeBuild: bkCBFake{}}, projects, nil)
	if err == nil {
		t.Error("a refused BatchGetBuilds returned no error")
	}
	for _, p := range projects {
		if got := res.TruncatedIDs[p.ID]; got != "BatchGetBuilds" {
			t.Errorf("%s: TruncatedIDs = %q, want %q", p.ID, got, "BatchGetBuilds")
		}
	}
	if !res.Truncated {
		t.Error("result.Truncated = false with every project unread")
	}
}

type bkCFNFake struct {
	awsclient.CFNAPI
}

func (bkCFNFake) DescribeStackEvents(context.Context, *cfnsvc.DescribeStackEventsInput, ...func(*cfnsvc.Options)) (*cfnsvc.DescribeStackEventsOutput, error) {
	return &cfnsvc.DescribeStackEventsOutput{StackEvents: []cfntypes.StackEvent{{
		ResourceStatus:    cfntypes.ResourceStatusUpdateFailed,
		LogicalResourceId: aws.String("OrdersQueue"),
		ResourceType:      aws.String("AWS::SQS::Queue"),
	}}}, nil
}

func (bkCFNFake) DescribeStacks(context.Context, *cfnsvc.DescribeStacksInput, ...func(*cfnsvc.Options)) (*cfnsvc.DescribeStacksOutput, error) {
	return &cfnsvc.DescribeStacksOutput{Stacks: []cfntypes.Stack{{
		StackName:        aws.String("acme-orders"),
		DriftInformation: &cfntypes.StackDriftInformation{StackDriftStatus: cfntypes.StackDriftStatusDrifted},
	}}}, nil
}

// A stack that failed and drifted keeps both findings, each with its own
// supporting rows.
func TestCFNCombined_FailedAndDriftedStackKeepsBothFindings(t *testing.T) {
	const id = "arn:aws:cloudformation:us-east-1:123456789012:stack/acme-orders/0001"
	stacks := []resource.Resource{{ID: id, Name: "acme-orders", Type: "cfn", Fields: map[string]string{"stack_name": "acme-orders"}}}
	res, err := awsclient.EnrichCFNCombined(context.Background(), &awsclient.ServiceClients{CloudFormation: bkCFNFake{}}, stacks, nil)
	if err != nil {
		t.Fatalf("EnrichCFNCombined: %v", err)
	}
	for _, code := range []domain.FindingCode{"cfn.recent-resource-failure", "cfn.stack-drifted"} {
		if !bkHasCode(res.Findings[id], code) {
			t.Errorf("stack carries %v, want %s", bkCodes(res.Findings[id]), code)
		}
		if len(res.AttentionDetails[id][code].Rows) == 0 {
			t.Errorf("%s lost its supporting rows in the merge", code)
		}
	}
}
