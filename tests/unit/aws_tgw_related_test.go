package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// tgwCheckerByTarget retrieves the RelatedChecker for the given targetType
// and fails the test if the checker is nil or not found.
func tgwCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("tgw") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("tgw related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("tgw related checker for %s not found", target)
	return nil
}

const tgwTestID = "tgw-abc123"

// tgwSrcResource returns a canonical test resource for a TGW.
func tgwSrcResource() resource.Resource {
	return resource.Resource{
		ID:   tgwTestID,
		Name: "test-tgw",
		Fields: map[string]string{
			"tgw_id": tgwTestID,
		},
		RawStruct: ec2types.TransitGateway{
			TransitGatewayId: aws.String(tgwTestID),
		},
	}
}

// --- RTB checker tests (Pattern C — reverse cache lookup) ---

// TestRelated_TGW_RTB_Match verifies that an RTB whose route has a
// TransitGatewayId matching the source TGW is counted.
func TestRelated_TGW_RTB_Match(t *testing.T) {
	res := tgwSrcResource()
	cache := resource.ResourceCache{
		"rtb": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "rtb-match",
				RawStruct: ec2types.RouteTable{
					Routes: []ec2types.Route{
						{TransitGatewayId: aws.String(tgwTestID), DestinationCidrBlock: aws.String("10.1.0.0/16")},
					},
				},
			},
		}},
	}

	checker := tgwCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
}

// TestRelated_TGW_RTB_NoMatch verifies that RTBs whose routes point to a
// different TGW produce Count=0.
func TestRelated_TGW_RTB_NoMatch(t *testing.T) {
	res := tgwSrcResource()
	cache := resource.ResourceCache{
		"rtb": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID: "rtb-other",
				RawStruct: ec2types.RouteTable{
					Routes: []ec2types.Route{
						{TransitGatewayId: aws.String("tgw-different"), DestinationCidrBlock: aws.String("10.2.0.0/16")},
					},
				},
			},
		}},
	}

	checker := tgwCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

// TestRelated_TGW_NilClients verifies that the RTB checker returns Count=-1
// when the cache has no rtb entry (cache miss / nil clients).
func TestRelated_TGW_NilClients(t *testing.T) {
	res := tgwSrcResource()
	emptyCache := resource.ResourceCache{}

	checker := tgwCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, emptyCache)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients, empty cache)", result.Count)
	}
}

// --- VPC checker nil-clients test ---

// TestRelated_TGW_VPC_NilClients verifies that the vpc checker returns Count:-1
// when clients are nil (DescribeTransitGatewayVpcAttachments cannot be called).
func TestRelated_TGW_VPC_NilClients(t *testing.T) {
	res := tgwSrcResource()
	checker := tgwCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, nil)
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

// --- CFN checker tests (tag-based, no cache needed) ---

// TestRelated_TGW_CFN_HasTag verifies that a TGW with the aws:cloudformation:stack-name
// tag produces Count=1 with the stack name in ResourceIDs.
func TestRelated_TGW_CFN_HasTag(t *testing.T) {
	res := resource.Resource{
		ID: tgwTestID,
		RawStruct: ec2types.TransitGateway{
			Tags: []ec2types.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("network-stack")},
			},
		},
	}

	checker := tgwCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (TGW has CFN tag)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "network-stack" {
		t.Errorf("ResourceIDs = %v, want [\"network-stack\"]", result.ResourceIDs)
	}
}

// TestRelated_TGW_CFN_NoTag verifies that a TGW without the aws:cloudformation:stack-name
// tag produces Count=0.
func TestRelated_TGW_CFN_NoTag(t *testing.T) {
	res := resource.Resource{
		ID:        tgwTestID,
		RawStruct: ec2types.TransitGateway{},
	}

	checker := tgwCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (TGW has no CFN tag)", result.Count)
	}
}

// --- VPC checker tests (Pattern A — direct API call) ---

// tgwVpcAttachmentsFake implements awsclient.EC2API for tgw→vpc checker testing.
// It embeds the interface and overrides only DescribeTransitGatewayVpcAttachments
// so test callers can seed a per-TGW-id response.
type tgwVpcAttachmentsFake struct {
	awsclient.EC2API
	results map[string][]ec2types.TransitGatewayVpcAttachment
	err     error
}

func (f *tgwVpcAttachmentsFake) DescribeTransitGatewayVpcAttachments(
	_ context.Context,
	in *ec2.DescribeTransitGatewayVpcAttachmentsInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeTransitGatewayVpcAttachmentsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	tgwID := ""
	if in != nil {
		for _, fl := range in.Filters {
			if fl.Name != nil && *fl.Name == "transit-gateway-id" && len(fl.Values) > 0 {
				tgwID = fl.Values[0]
				break
			}
		}
	}
	return &ec2.DescribeTransitGatewayVpcAttachmentsOutput{
		TransitGatewayVpcAttachments: f.results[tgwID],
	}, nil
}

// Compile-time check: the fake satisfies the aggregate EC2API interface.
var _ awsclient.EC2API = (*tgwVpcAttachmentsFake)(nil)

// TestRelated_TGW_VPC_Match verifies that two distinct VpcIds returned by the
// fake produce Count=2 with both ids in ResourceIDs.
func TestRelated_TGW_VPC_Match(t *testing.T) {
	fake := &tgwVpcAttachmentsFake{
		results: map[string][]ec2types.TransitGatewayVpcAttachment{
			tgwTestID: {
				{VpcId: aws.String("vpc-aaa111")},
				{VpcId: aws.String("vpc-bbb222")},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	res := tgwSrcResource()

	checker := tgwCheckerByTarget(t, "vpc")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 2 {
		t.Fatalf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Fatalf("ResourceIDs length = %d, want 2: %v", len(result.ResourceIDs), result.ResourceIDs)
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs {
		seen[id] = true
	}
	for _, want := range []string{"vpc-aaa111", "vpc-bbb222"} {
		if !seen[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
	// Guard against an error slipping through while Count still looks right.
	if result.Err != nil {
		t.Errorf("unexpected Err: %v", result.Err)
	}
}

// TestRelated_TGW_VPC_Empty verifies that zero attachments produce Count=0.
func TestRelated_TGW_VPC_Empty(t *testing.T) {
	fake := &tgwVpcAttachmentsFake{
		results: map[string][]ec2types.TransitGatewayVpcAttachment{
			// Empty slice for this TGW id — valid, explicit "no attachments" response.
			tgwTestID: {},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	res := tgwSrcResource()

	checker := tgwCheckerByTarget(t, "vpc")
	result := checker(context.Background(), clients, res, nil)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no attachments)", result.Count)
	}
	if len(result.ResourceIDs) != 0 {
		t.Errorf("ResourceIDs = %v, want empty", result.ResourceIDs)
	}
}

// TestRelated_TGW_VPC_WrongRawStruct verifies the checker returns Count=-1
// when RawStruct is not ec2types.TransitGateway (defensive guard).
func TestRelated_TGW_VPC_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID: tgwTestID,
		// Intentionally wrong type — a VPC struct, not a TransitGateway.
		RawStruct: ec2types.Vpc{VpcId: aws.String("vpc-wrong")},
	}

	checker := tgwCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, nil)

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}
