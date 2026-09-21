package unit_test

// s3_0916_row5_cfn_page_truncation_test.go pins cfn→s3 and cfn→eb-rule
// reporting a lower bound when ListStackResources answered with more pages to
// come.
//
// The pivot walks ListStackResources up to the page cap. A NextToken still
// pending at the cap means resources of the wanted type may sit on pages
// nobody read, so an exact count is a claim the walk did not make: a stack
// whose buckets all live past the cap renders a dead-end "(0)" the operator
// cannot drill.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

const row5StackName = "acme-web"

// row5CFNFake answers every ListStackResources call with the same page and
// records how many times it was called: with a token set the list never ends,
// so the walk runs to the page cap.
type row5CFNFake struct {
	awsclient.CFNAPI
	summaries []cfntypes.StackResourceSummary
	nextToken string
	calls     int
}

func (f *row5CFNFake) ListStackResources(
	_ context.Context, _ *cloudformation.ListStackResourcesInput, _ ...func(*cloudformation.Options),
) (*cloudformation.ListStackResourcesOutput, error) {
	f.calls++
	out := &cloudformation.ListStackResourcesOutput{StackResourceSummaries: f.summaries}
	if f.nextToken != "" {
		out.NextToken = aws.String(f.nextToken)
	}
	return out, nil
}

func row5Summary(resourceType, physicalID string) cfntypes.StackResourceSummary {
	return cfntypes.StackResourceSummary{
		LogicalResourceId:    aws.String("Resource1"),
		PhysicalResourceId:   aws.String(physicalID),
		ResourceType:         aws.String(resourceType),
		ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
		LastUpdatedTimestamp: aws.Time(time.Date(2025, 11, 4, 12, 0, 0, 0, time.UTC)),
	}
}

func TestS3_0916_Row5_StackResourcePivotsReportALowerBound(t *testing.T) {
	cases := []struct {
		name          string
		target        string
		resourceType  string
		physicalID    string
		presentOnPage bool
		nextToken     string
		wantCount     int
		wantTruncated bool
	}{
		{"s3 wanted type absent, more pages", "s3", "AWS::SQS::Queue", "acme-web-queue", false, "p2", 0, true},
		{"s3 wanted type present, more pages", "s3", "AWS::S3::Bucket", "acme-web-assets", true, "p2", 1, true},
		{"s3 wanted type present, last page", "s3", "AWS::S3::Bucket", "acme-web-assets", true, "", 1, false},
		{"eb-rule wanted type absent, more pages", "eb-rule", "AWS::SQS::Queue", "acme-web-queue", false, "p2", 0, true},
		{"eb-rule wanted type present, more pages", "eb-rule", "AWS::Events::Rule", "acme-web-nightly", true, "p2", 1, true},
		{"eb-rule wanted type present, last page", "eb-rule", "AWS::Events::Rule", "acme-web-nightly", true, "", 1, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := &row5CFNFake{
				summaries: []cfntypes.StackResourceSummary{row5Summary(c.resourceType, c.physicalID)},
				nextToken: c.nextToken,
			}
			clients := &awsclient.ServiceClients{CloudFormation: fake}
			stack := resource.Resource{ID: row5StackName, Name: row5StackName}

			got := checkerByTarget(t, "cfn", c.target)(context.Background(), clients, stack, resource.ResourceCache{})

			if got.Count() != c.wantCount {
				t.Errorf("Count = %d, want %d", got.Count(), c.wantCount)
			}
			if got.Truncated() != c.wantTruncated {
				t.Errorf("Truncated = %v, want %v", got.Truncated(), c.wantTruncated)
			}
			if c.presentOnPage {
				// A rule name is unique on its event bus, and a rule created
				// without one sits on the default bus.
				wantID := c.physicalID
				if c.target == "eb-rule" {
					wantID = "default/" + c.physicalID
				}
				if ids := got.ResourceIDs(); len(ids) != 1 || ids[0] != wantID {
					t.Errorf("ResourceIDs = %v, want [%s]", ids, wantID)
				}
			}
			wantCalls := 1
			if c.nextToken != "" {
				wantCalls = awsclient.PerParentPageCap
			}
			if fake.calls != wantCalls {
				t.Errorf("ListStackResources calls = %d, want %d — a pending token is walked to the page cap", fake.calls, wantCalls)
			}
		})
	}
}
