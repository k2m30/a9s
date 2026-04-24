package unit_test

// aws_wave5_related_test.go — Wave 5 coverage fill for zero-hit branches in:
//   - kms_related.go:      kmsRoleNamesFromPolicyJSON
//   - opensearch_related.go: checkOpenSearchCFN, checkOpenSearchACM
//   - dbc_related.go:    checkDbcSubnet
//   - vpc_related.go:      checkVPCENI, checkVPCTGW
//   - lambda_related.go:   checkLambdaSQS, checkLambdaCFN, checkLambdaEBRule
//   - efs_related.go:      checkEFSLambda
//   - ses_related.go:      checkSESLambda

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdb_types "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ────────────────────────────────────────────────────────────────────────────
// kms_related.go — kmsRoleNamesFromPolicyJSON (0.0%)
// ────────────────────────────────────────────────────────────────────────────
// kmsRoleNamesFromPolicyJSON is tested indirectly via checkKMSRole; the role
// checker calls it after fetching the key policy. The function itself is
// package-private, but we can reach its branches via a mock ServiceClients
// that implements kms:GetKeyPolicy. Testing the pure parsing logic via the
// registered checker with a mock KMS client that returns a known policy JSON.

// TestRelated_KMS_Role_NilClients verifies checkKMSRole returns Count=-1
// when no KMS client is available. The function's policy-parsing branches
// are only reachable after a successful API call; here we verify the guard.
func TestRelated_KMS_Role_NilClients(t *testing.T) {
	const keyID = "mrk-aaa111bbb222"
	src := resource.Resource{
		ID: keyID,
		Fields: map[string]string{
			"key_id": keyID,
		},
	}
	checker := kmsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	// nil clients → guard fires: Count=-1
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil KMS client)", result.Count)
	}
}

// TestRelated_KMS_Role_EmptyKeyID verifies checkKMSRole returns Count=0
// when the resource has no extractable key ID.
func TestRelated_KMS_Role_EmptyKeyID(t *testing.T) {
	src := resource.Resource{
		ID:     "",
		Fields: map[string]string{},
	}
	checker := kmsCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	// empty key ID → Count=0 (no key → no roles)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty key ID)", result.Count)
	}
}

// opensearch_related.go coverage-fill moved to tests/unit/aws_opensearch_related_test.go
// (rewritten by the a9s-implement-resource skill phase 6b handoff).

// ────────────────────────────────────────────────────────────────────────────
// dbc_related.go — checkDbcSubnet (37.5%)
// ────────────────────────────────────────────────────────────────────────────

// TestRelated_DBC_Subnet_NilClientsW5 verifies checkDbcSubnet returns Count=-1
// when no DocDB client is available (dbcSubnetGroup returns nil).
func TestRelated_DBC_Subnet_NilClientsW5(t *testing.T) {
	src := resource.Resource{
		ID: "my-docdb-cluster",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("my-docdb-cluster"),
			DBSubnetGroup:       aws.String("my-subnet-group"),
		},
	}
	checker := dbcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	// nil clients → dbcSubnetGroup returns nil → Count=-1
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil DocDB client)", result.Count)
	}
}

// TestRelated_DBC_Subnet_WrongRawStruct verifies checkDbcSubnet returns Count=-1
// when RawStruct is not a DBCluster.
func TestRelated_DBC_Subnet_WrongRawStruct(t *testing.T) {
	src := resource.Resource{
		ID:        "my-docdb-cluster",
		RawStruct: "not-a-cluster",
	}
	checker := dbcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// TestRelated_DBC_Subnet_NoSubnetGroup verifies checkDbcSubnet returns Count=-1
// when the cluster has no DBSubnetGroup name (nil pointer in DBSubnetGroup field).
func TestRelated_DBC_Subnet_NoSubnetGroup(t *testing.T) {
	src := resource.Resource{
		ID: "my-docdb-cluster",
		RawStruct: docdb_types.DBCluster{
			DBClusterIdentifier: aws.String("my-docdb-cluster"),
			DBSubnetGroup:       nil,
		},
	}
	checker := dbcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	// no DBSubnetGroup → dbcSubnetGroup returns nil → Count=-1
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no DBSubnetGroup name)", result.Count)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// vpc_related.go — checkVPCENI (47.4%) and checkVPCTGW (0.0%)
// ────────────────────────────────────────────────────────────────────────────

// TestRelated_VPC_ENI_FieldMatch verifies checkVPCENI counts an ENI
// whose vpc_id field matches the source VPC's ID.
func TestRelated_VPC_ENI_FieldMatch(t *testing.T) {
	res := vpcSrcResource()
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:     "eni-aabbccdd",
				Fields: map[string]string{"vpc_id": vpcTestID},
			},
		}},
	}
	checker := vpcCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, res, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (eni vpc_id matches)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "eni-aabbccdd" {
		t.Errorf("ResourceIDs = %v, want [eni-aabbccdd]", result.ResourceIDs)
	}
}

// TestRelated_VPC_ENI_RawStructMatch verifies checkVPCENI counts an ENI
// whose ec2types.NetworkInterface.VpcId matches the source VPC (no vpc_id field).
func TestRelated_VPC_ENI_RawStructMatch(t *testing.T) {
	res := vpcSrcResource()
	eniRes := resource.Resource{
		ID:     "eni-rawstruct001",
		Fields: map[string]string{}, // no vpc_id field — falls through to RawStruct
		RawStruct: ec2types.NetworkInterface{
			NetworkInterfaceId: aws.String("eni-rawstruct001"),
			VpcId:              aws.String(vpcTestID),
		},
	}
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{eniRes}},
	}
	checker := vpcCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, res, cache)
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (RawStruct VpcId match)", result.Count)
	}
}

// TestRelated_VPC_ENI_NoMatch verifies checkVPCENI returns Count=0 when no ENI
// belongs to the source VPC.
func TestRelated_VPC_ENI_NoMatch(t *testing.T) {
	res := vpcSrcResource()
	cache := resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:     "eni-other",
				Fields: map[string]string{"vpc_id": "vpc-zzz999"},
			},
		}},
	}
	checker := vpcCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, res, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (different VPC)", result.Count)
	}
}

// TestRelated_VPC_ENI_EmptyVPCID verifies checkVPCENI returns Count=0 when the
// source VPC resource has an empty ID (early exit guard).
func TestRelated_VPC_ENI_EmptyVPCID(t *testing.T) {
	src := resource.Resource{
		ID:     "",
		Fields: map[string]string{},
		RawStruct: ec2types.Vpc{
			VpcId: aws.String(""),
		},
	}
	checker := vpcCheckerByTarget(t, "eni")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty VPC ID)", result.Count)
	}
}

// TestRelated_VPC_TGW_NilClients verifies checkVPCTGW returns Count=-1 when
// no EC2 client is available to call DescribeTransitGatewayAttachments.
func TestRelated_VPC_TGW_NilClients(t *testing.T) {
	res := vpcSrcResource()
	checker := vpcCheckerByTarget(t, "tgw")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	// nil clients → Count=-1
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil EC2 client)", result.Count)
	}
}

// TestRelated_VPC_TGW_EmptyVPCID verifies checkVPCTGW returns Count=0 when the
// source VPC resource has an empty ID (early exit guard).
func TestRelated_VPC_TGW_EmptyVPCID(t *testing.T) {
	src := resource.Resource{
		ID:     "",
		Fields: map[string]string{},
		RawStruct: ec2types.Vpc{
			VpcId: aws.String(""),
		},
	}
	checker := vpcCheckerByTarget(t, "tgw")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty VPC ID)", result.Count)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// lambda_related.go — checkLambdaSQS, checkLambdaCFN, checkLambdaEBRule
// All three require live API calls; we cover the branching paths reachable
// without a real client (nil-client guard + wrong-struct guard + empty-name guard)
// that the existing tests don't yet exercise at >50%.
// ────────────────────────────────────────────────────────────────────────────

// TestRelated_Lambda_SQS_NilClientWithID verifies checkLambdaSQS returns Count=-1
// when the function has a non-empty name but Lambda client is nil.
// (The existing test covers the same path; this test exercises it from
// the resource.ID path rather than the Name path, exercising the function name
// extraction logic.)
func TestRelated_Lambda_SQS_NilClientFromIDField(t *testing.T) {
	// Use ID field — lambdaSQS reads ID first
	src := resource.Resource{
		ID:        "func-from-id-field",
		Name:      "",
		RawStruct: lambdatypes.FunctionConfiguration{},
	}
	checker := lambdaCheckerByTarget(t, "sqs")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil Lambda client, ID-path name extraction)", result.Count)
	}
}

// TestRelated_Lambda_CFN_NilClientWithARN verifies checkLambdaCFN returns Count=-1
// when the function has a valid ARN but Lambda client is nil.
func TestRelated_Lambda_CFN_NilClientWithARN(t *testing.T) {
	src := resource.Resource{
		ID:   "my-tagged-function",
		Name: "my-tagged-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("my-tagged-function"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:my-tagged-function"),
		},
	}
	checker := lambdaCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	// nil Lambda client → cannot call ListTags → Count=-1
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil Lambda client, has ARN)", result.Count)
	}
}

// TestRelated_Lambda_EBRule_NilClientWithName verifies checkLambdaEBRule returns
// Count=-1 when the function has a name but EventBridge client is nil.
func TestRelated_Lambda_EBRule_NilClientWithName(t *testing.T) {
	src := resource.Resource{
		ID:   "rule-target-function",
		Name: "rule-target-function",
		RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String("rule-target-function"),
			FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:rule-target-function"),
		},
	}
	checker := lambdaCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	// nil EventBridge client → Count=-1 (targets are only accessible via live API)
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil EventBridge client)", result.Count)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// efs_related.go — checkEFSLambda (17.6%)
// ────────────────────────────────────────────────────────────────────────────

// TestRelated_EFS_Lambda_EmptyFSID verifies checkEFSLambda returns Count=0
// when the filesystem resource has an empty ID (early exit guard).
func TestRelated_EFS_Lambda_EmptyFSID(t *testing.T) {
	src := resource.Resource{
		ID: "",
		RawStruct: efstypes.FileSystemDescription{
			FileSystemId: aws.String(""),
		},
	}
	checker := efsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty filesystem ID)", result.Count)
	}
}

// TestRelated_EFS_Lambda_NilClients verifies checkEFSLambda returns Count=-1
// when the filesystem has a valid ID but no EFS client is available.
// The function requires efs:DescribeAccessPoints which is a live API call.
func TestRelated_EFS_Lambda_NilClients(t *testing.T) {
	src := resource.Resource{
		ID: "fs-abc1234def567890",
		RawStruct: efstypes.FileSystemDescription{
			FileSystemId: aws.String("fs-abc1234def567890"),
		},
	}
	checker := efsCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, src, resource.ResourceCache{})
	// nil EFS client → Count=-1 (cannot resolve access points)
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil EFS client)", result.Count)
	}
}

// SES related-checker coverage lives in aws_ses_related_test.go (phase 6b).
