package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_TGW_NilClients(t *testing.T) {
	res := tgwSrcResource()
	emptyCache := resource.ResourceCache{}

	checker := tgwCheckerByTarget(t, "rtb")
	result := checker(context.Background(), nil, res, emptyCache)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients, empty cache)", result.Count())
	}
}

func TestRelated_TGW_VPC_NilClients(t *testing.T) {
	res := tgwSrcResource()
	checker := tgwCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, nil)
	if result.State() != domain.RelatedUnknown {
		t.Errorf("State = %v, want RelatedUnknown (nil clients)", result.State())
	}
}

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

var _ awsclient.EC2API = (*tgwVpcAttachmentsFake)(nil)

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

	if result.Count() != 2 {
		t.Fatalf("Count = %d, want 2", result.Count())
	}
	if len(result.ResourceIDs()) != 2 {
		t.Fatalf("ResourceIDs length = %d, want 2: %v", len(result.ResourceIDs()), result.ResourceIDs())
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs() {
		seen[id] = true
	}
	for _, want := range []string{"vpc-aaa111", "vpc-bbb222"} {
		if !seen[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs())
		}
	}
	if result.Err() != nil {
		t.Errorf("unexpected Err: %v", result.Err())
	}
}

func TestRelated_TGW_VPC_Empty(t *testing.T) {
	fake := &tgwVpcAttachmentsFake{
		results: map[string][]ec2types.TransitGatewayVpcAttachment{
			tgwTestID: {},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	res := tgwSrcResource()

	checker := tgwCheckerByTarget(t, "vpc")
	result := checker(context.Background(), clients, res, nil)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no attachments)", result.Count())
	}
	if len(result.ResourceIDs()) != 0 {
		t.Errorf("ResourceIDs = %v, want empty", result.ResourceIDs())
	}
}

func TestRelated_TGW_VPC_WrongRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        tgwTestID,
		RawStruct: ec2types.Vpc{VpcId: aws.String("vpc-wrong")},
	}

	checker := tgwCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, res, nil)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count())
	}
}

const tgwSLRName = "AWSServiceRoleForVPCTransitGateway"
const tgwSLRARN = "arn:aws:iam::123456789012:role/aws-service-role/transitgateway.amazonaws.com/AWSServiceRoleForVPCTransitGateway"

// Role rows are keyed by role name, and the ARN is read through the role
// resolver.
func TestRelated_TGW_Role_Match(t *testing.T) {
	fake := newFakeIAMWithRole(tgwSLRARN, tgwSLRName)
	clients := &awsclient.ServiceClients{IAM: fake}
	res := tgwSrcResource()

	checker := tgwCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, res, nil)

	if result.Count() != 1 {
		t.Fatalf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != tgwSLRName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), tgwSLRName)
	}
	if result.Err() != nil {
		t.Errorf("unexpected Err: %v", result.Err())
	}
}

// GetRole's NoSuchEntity means the service-linked role does not exist in
// this account.
func TestRelated_TGW_Role_Empty(t *testing.T) {
	fake := newFakeIAMWithNoSuchEntityRole()
	clients := &awsclient.ServiceClients{IAM: fake}
	res := tgwSrcResource()

	checker := tgwCheckerByTarget(t, "role")
	result := checker(context.Background(), clients, res, nil)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (NoSuchEntity — SLR not present)", result.Count())
	}
	if result.Err() != nil {
		t.Errorf("unexpected Err: %v", result.Err())
	}
}

func TestRelated_TGW_Role_NilClients(t *testing.T) {
	res := tgwSrcResource()

	checker := tgwCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, res, nil)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count())
	}
}

func TestRelated_TGW_Subnet_Match(t *testing.T) {
	const sub1 = "subnet-0a1b2c3d4e5f60001"
	const sub2 = "subnet-0a1b2c3d4e5f60002"

	fake := &tgwVpcAttachmentsFake{
		results: map[string][]ec2types.TransitGatewayVpcAttachment{
			tgwTestID: {
				{VpcId: aws.String("vpc-aaa111"), SubnetIds: []string{sub1}},
				{VpcId: aws.String("vpc-bbb222"), SubnetIds: []string{sub2}},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	res := tgwSrcResource()

	checker := tgwCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, res, nil)

	if result.Count() != 2 {
		t.Fatalf("Count = %d, want 2", result.Count())
	}
	seen := map[string]bool{}
	for _, id := range result.ResourceIDs() {
		seen[id] = true
	}
	for _, want := range []string{sub1, sub2} {
		if !seen[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs())
		}
	}
	if result.Err() != nil {
		t.Errorf("unexpected Err: %v", result.Err())
	}
}

func TestRelated_TGW_Subnet_Empty(t *testing.T) {
	fake := &tgwVpcAttachmentsFake{
		results: map[string][]ec2types.TransitGatewayVpcAttachment{
			tgwTestID: {
				{VpcId: aws.String("vpc-aaa111"), SubnetIds: []string{}},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fake}
	res := tgwSrcResource()

	checker := tgwCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, res, nil)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no subnet IDs in attachment)", result.Count())
	}
	if len(result.ResourceIDs()) != 0 {
		t.Errorf("ResourceIDs = %v, want empty", result.ResourceIDs())
	}
}

func TestRelated_TGW_Subnet_NilClients(t *testing.T) {
	res := tgwSrcResource()

	checker := tgwCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, res, nil)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count())
	}
}
