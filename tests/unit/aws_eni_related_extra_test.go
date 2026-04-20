package unit_test

// aws_eni_related_extra_test.go — additional coverage for eni_related.go
// Covers: checkENIVPC, checkENISubnet, checkENIELB, checkENILambda,
//         checkENINAT, checkENIVPCE, lambdaFunctionNameFromENIDescription.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- checkENIVPC (Pattern F — reads Fields["vpc_id"]) ---

func TestRelated_ENI_VPC_Found(t *testing.T) {
	source := resource.Resource{
		ID:     "eni-0abc1234567890001",
		Fields: map[string]string{"vpc_id": "vpc-0a1b2c3d4e5f60001"},
	}
	checker := eniCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "vpc-0a1b2c3d4e5f60001" {
		t.Errorf("ResourceIDs[0] = %q, want vpc-0a1b2c3d4e5f60001", result.ResourceIDs[0])
	}
}

func TestRelated_ENI_VPC_EmptyVPCId(t *testing.T) {
	source := resource.Resource{
		ID:     "eni-0abc1234567890001",
		Fields: map[string]string{"vpc_id": ""},
	}
	checker := eniCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty vpc_id field)", result.Count)
	}
}

func TestRelated_ENI_VPC_NoFields(t *testing.T) {
	source := resource.Resource{ID: "eni-0abc1234567890001"}
	checker := eniCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no vpc_id field)", result.Count)
	}
}

// --- checkENISubnet (Pattern F — reads SubnetId from RawStruct) ---

func TestRelated_ENI_Subnet_Found(t *testing.T) {
	source := resource.Resource{
		ID: "eni-0abc1234567890001",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-0abc1234567890001"),
			SubnetId:           aws.String("subnet-0abc001"),
		},
	}
	checker := eniCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "subnet-0abc001" {
		t.Errorf("ResourceIDs[0] = %q, want subnet-0abc001", result.ResourceIDs[0])
	}
}

func TestRelated_ENI_Subnet_NilSubnetId(t *testing.T) {
	source := resource.Resource{
		ID: "eni-0abc1234567890001",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-0abc1234567890001"),
			SubnetId:           nil,
		},
	}
	checker := eniCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil SubnetId)", result.Count)
	}
}

func TestRelated_ENI_Subnet_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "eni-0abc1234567890001", RawStruct: "not-a-eni"}
	checker := eniCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// --- checkENIELB (Pattern F — Description = "ELB app/NAME/HASH", RequesterId = "amazon-elb") ---

func TestRelated_ENI_ELB_ALBDescription(t *testing.T) {
	source := resource.Resource{
		ID: "eni-0abc1234567890001",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-0abc1234567890001"),
			RequesterId:        aws.String("amazon-elb"),
			Description:        aws.String("ELB app/acme-prod-web/abcdef1234567890"),
		},
	}
	checker := eniCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "acme-prod-web" {
		t.Errorf("ResourceIDs[0] = %q, want acme-prod-web", result.ResourceIDs[0])
	}
}

func TestRelated_ENI_ELB_NotELBOwned(t *testing.T) {
	source := resource.Resource{
		ID: "eni-0abc1234567890001",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-0abc1234567890001"),
			RequesterId:        aws.String("amazon-ec2"),
			Description:        aws.String("some other description"),
		},
	}
	checker := eniCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (not ELB-owned)", result.Count)
	}
}

func TestRelated_ENI_ELB_ELBOwnedNoDescription(t *testing.T) {
	source := resource.Resource{
		ID: "eni-0abc1234567890001",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-0abc1234567890001"),
			RequesterId:        aws.String("amazon-elb"),
			Description:        nil,
		},
	}
	checker := eniCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (ELB-owned but no description)", result.Count)
	}
}

func TestRelated_ENI_ELB_DescriptionNotELBPrefix(t *testing.T) {
	source := resource.Resource{
		ID: "eni-0abc1234567890001",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-0abc1234567890001"),
			RequesterId:        aws.String("amazon-elb"),
			Description:        aws.String("internal network interface"),
		},
	}
	checker := eniCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (description lacks 'ELB ' prefix)", result.Count)
	}
}

func TestRelated_ENI_ELB_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "eni-0abc1234567890001", RawStruct: "not-a-eni"}
	checker := eniCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// --- checkENILambda (Pattern F — RequesterId / Description heuristic) ---

func TestRelated_ENI_Lambda_ExtractsFunctionName(t *testing.T) {
	// Standard Lambda ENI description: "AWS Lambda VPC ENI-<name>-<uuid>"
	source := resource.Resource{
		ID: "eni-lambda-001",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-lambda-001"),
			RequesterId:        aws.String("123456789012:awslambda_us-east-1"),
			Description:        aws.String("AWS Lambda VPC ENI-process-orders-1a2b3c4d-5e6f-7a8b-9c0d-1e2f3a4b5c6d"),
		},
	}
	checker := eniCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "process-orders" {
		t.Errorf("ResourceIDs[0] = %q, want process-orders", result.ResourceIDs[0])
	}
}

func TestRelated_ENI_Lambda_NotLambdaOwnedReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID: "eni-other-001",
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-other-001"),
			RequesterId:        aws.String("amazon-ec2"),
			Description:        aws.String("primary network interface"),
		},
	}
	checker := eniCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (not Lambda-owned)", result.Count)
	}
}

func TestRelated_ENI_Lambda_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "eni-bad", RawStruct: 42}
	checker := eniCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// --- checkENINAT (Pattern C — scan nat cache for NatGatewayAddresses[].NetworkInterfaceId) ---

func TestRelated_ENI_NAT_Found(t *testing.T) {
	const eniID = "eni-0abc1234567890001"
	source := resource.Resource{
		ID: eniID,
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String(eniID),
		},
	}
	natRes := resource.Resource{
		ID: "nat-0a1b2c3d4e5f60001",
		RawStruct: ec2types.NatGateway{
			NatGatewayId: aws.String("nat-0a1b2c3d4e5f60001"),
			NatGatewayAddresses: []ec2types.NatGatewayAddress{
				{NetworkInterfaceId: aws.String(eniID)},
			},
		},
	}
	cache := resource.ResourceCache{
		"nat": resource.ResourceCacheEntry{Resources: []resource.Resource{natRes}},
	}

	checker := eniCheckerByTarget(t, "nat")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "nat-0a1b2c3d4e5f60001" {
		t.Errorf("ResourceIDs[0] = %q, want nat-0a1b2c3d4e5f60001", result.ResourceIDs[0])
	}
}

func TestRelated_ENI_NAT_NoMatch(t *testing.T) {
	source := resource.Resource{
		ID:        "eni-0abc1234567890001",
		RawStruct: ec2types.NetworkInterface{NetworkInterfaceId: aws.String("eni-0abc1234567890001")},
	}
	natRes := resource.Resource{
		ID: "nat-other",
		RawStruct: ec2types.NatGateway{
			NatGatewayId: aws.String("nat-other"),
			NatGatewayAddresses: []ec2types.NatGatewayAddress{
				{NetworkInterfaceId: aws.String("eni-other")},
			},
		},
	}
	cache := resource.ResourceCache{
		"nat": resource.ResourceCacheEntry{Resources: []resource.Resource{natRes}},
	}

	checker := eniCheckerByTarget(t, "nat")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ENI_NAT_EmptyIDReturnsZero(t *testing.T) {
	source := resource.Resource{
		ID:        "",
		RawStruct: ec2types.NetworkInterface{},
	}
	checker := eniCheckerByTarget(t, "nat")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ENI ID)", result.Count)
	}
}

func TestRelated_ENI_NAT_NilCacheNoClients(t *testing.T) {
	source := resource.Resource{
		ID:        "eni-0abc1234567890001",
		RawStruct: ec2types.NetworkInterface{NetworkInterfaceId: aws.String("eni-0abc1234567890001")},
	}
	checker := eniCheckerByTarget(t, "nat")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, nil clients)", result.Count)
	}
}

// --- checkENIVPCE (Pattern C — scan vpce cache for NetworkInterfaceIds) ---

func TestRelated_ENI_VPCE_Found(t *testing.T) {
	const eniID = "eni-0abc1234567890001"
	source := resource.Resource{
		ID:        eniID,
		RawStruct: ec2types.NetworkInterface{NetworkInterfaceId: aws.String(eniID)},
	}
	vpceRes := resource.Resource{
		ID: "vpce-0a1b2c3d4e5f60001",
		RawStruct: ec2types.VpcEndpoint{
			VpcEndpointId:       aws.String("vpce-0a1b2c3d4e5f60001"),
			NetworkInterfaceIds: []string{eniID},
		},
	}
	cache := resource.ResourceCache{
		"vpce": resource.ResourceCacheEntry{Resources: []resource.Resource{vpceRes}},
	}

	checker := eniCheckerByTarget(t, "vpce")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "vpce-0a1b2c3d4e5f60001" {
		t.Errorf("ResourceIDs[0] = %q, want vpce-0a1b2c3d4e5f60001", result.ResourceIDs[0])
	}
}

func TestRelated_ENI_VPCE_NoMatch(t *testing.T) {
	source := resource.Resource{
		ID:        "eni-0abc1234567890001",
		RawStruct: ec2types.NetworkInterface{NetworkInterfaceId: aws.String("eni-0abc1234567890001")},
	}
	vpceRes := resource.Resource{
		ID: "vpce-other",
		RawStruct: ec2types.VpcEndpoint{
			VpcEndpointId:       aws.String("vpce-other"),
			NetworkInterfaceIds: []string{"eni-other"},
		},
	}
	cache := resource.ResourceCache{
		"vpce": resource.ResourceCacheEntry{Resources: []resource.Resource{vpceRes}},
	}

	checker := eniCheckerByTarget(t, "vpce")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ENI_VPCE_EmptyIDReturnsZero(t *testing.T) {
	source := resource.Resource{ID: "", RawStruct: ec2types.NetworkInterface{}}
	checker := eniCheckerByTarget(t, "vpce")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty ENI ID)", result.Count)
	}
}

func TestRelated_ENI_VPCE_NilCacheNoClients(t *testing.T) {
	source := resource.Resource{
		ID:        "eni-0abc1234567890001",
		RawStruct: ec2types.NetworkInterface{NetworkInterfaceId: aws.String("eni-0abc1234567890001")},
	}
	checker := eniCheckerByTarget(t, "vpce")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, nil clients)", result.Count)
	}
}
