package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// checkerCache captures related checkers at init time, before any test can
// overwrite the global registry. This makes field-extraction tests immune
// to test-order pollution from tests that call RegisterRelated("ec2", ...).
var checkerCache = func() map[string]map[string]resource.RelatedChecker {
	cache := make(map[string]map[string]resource.RelatedChecker)
	for _, td := range resource.AllResourceTypes() {
		sn := td.ShortName
		defs := resource.GetRelated(sn)
		if len(defs) == 0 {
			continue
		}
		m := make(map[string]resource.RelatedChecker, len(defs))
		for _, def := range defs {
			if def.Checker != nil {
				m[def.TargetType] = def.Checker
			}
		}
		cache[sn] = m
	}
	return cache
}()

// fieldExtractionChecker retrieves the RelatedChecker for the given source type
// and target type from the init-time cache. Immune to test pollution.
func fieldExtractionChecker(t *testing.T, shortName, targetType string) resource.RelatedChecker {
	t.Helper()
	m, ok := checkerCache[shortName]
	if !ok {
		t.Fatalf("%s: no related checkers cached at init time", shortName)
	}
	checker, ok := m[targetType]
	if !ok {
		t.Fatalf("%s→%s: related checker not found in init-time cache", shortName, targetType)
	}
	return checker
}

// =============================================================================
// VPC checkers
// =============================================================================

// --- checkEC2VPC ---
// checkEC2VPC reads from res.Fields["vpc_id"], not from RawStruct.VpcId.

func TestRelatedFieldExtraction_EC2_VPC_ReturnsVpcID(t *testing.T) {
	res := resource.Resource{
		ID:     "i-12345abcde",
		Fields: map[string]string{"vpc_id": "vpc-abc123"},
	}
	checker := fieldExtractionChecker(t, "ec2", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.TargetType != "vpc" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "vpc")
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpc-abc123" {
		t.Errorf("ResourceIDs = %v, want [vpc-abc123]", result.ResourceIDs)
	}
}

func TestRelatedFieldExtraction_EC2_VPC_ReturnsZeroWhenFieldMissing(t *testing.T) {
	res := resource.Resource{
		ID:     "i-12345abcde",
		Fields: map[string]string{},
	}
	checker := fieldExtractionChecker(t, "ec2", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (missing vpc_id field)", result.Count)
	}
	if result.TargetType != "vpc" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "vpc")
	}
}

func TestRelatedFieldExtraction_EC2_VPC_ReturnsZeroWhenNilFields(t *testing.T) {
	res := resource.Resource{
		ID:     "i-12345abcde",
		Fields: nil,
	}
	checker := fieldExtractionChecker(t, "ec2", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Fields map)", result.Count)
	}
}

// --- checkELBVPC ---
// checkELBVPC reads from res.Fields["vpc_id"], not from RawStruct.VpcId.

func TestRelatedFieldExtraction_ELB_VPC_ReturnsVpcID(t *testing.T) {
	res := resource.Resource{
		ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc123",
		Fields: map[string]string{"vpc_id": "vpc-abc123"},
	}
	checker := fieldExtractionChecker(t, "elb", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.TargetType != "vpc" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "vpc")
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpc-abc123" {
		t.Errorf("ResourceIDs = %v, want [vpc-abc123]", result.ResourceIDs)
	}
}

func TestRelatedFieldExtraction_ELB_VPC_ReturnsZeroWhenFieldMissing(t *testing.T) {
	res := resource.Resource{
		ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc123",
		Fields: map[string]string{},
	}
	checker := fieldExtractionChecker(t, "elb", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (missing vpc_id field)", result.Count)
	}
}

// --- checkDbiVPC ---
// checkDbiVPC reads from inst.DBSubnetGroup.VpcId in RawStruct.

func TestRelatedFieldExtraction_DBI_VPC_ReturnsVpcID(t *testing.T) {
	res := resource.Resource{
		ID:     "my-db-instance",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBInstance{
			DBSubnetGroup: &rdstypes.DBSubnetGroup{
				VpcId: aws.String("vpc-abc123"),
			},
		},
	}
	checker := fieldExtractionChecker(t, "dbi", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.TargetType != "vpc" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "vpc")
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpc-abc123" {
		t.Errorf("ResourceIDs = %v, want [vpc-abc123]", result.ResourceIDs)
	}
}

func TestRelatedFieldExtraction_DBI_VPC_ReturnsZeroWhenNilSubnetGroup(t *testing.T) {
	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{DBSubnetGroup: nil},
	}
	checker := fieldExtractionChecker(t, "dbi", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil DBSubnetGroup)", result.Count)
	}
}

func TestRelatedFieldExtraction_DBI_VPC_ReturnsZeroWhenNilVpcID(t *testing.T) {
	res := resource.Resource{
		ID:     "my-db-instance",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBInstance{
			DBSubnetGroup: &rdstypes.DBSubnetGroup{VpcId: nil},
		},
	}
	checker := fieldExtractionChecker(t, "dbi", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil VpcId in DBSubnetGroup)", result.Count)
	}
}

func TestRelatedFieldExtraction_DBI_VPC_ReturnsZeroWhenNilRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:     "my-db-instance",
		Fields: map[string]string{},
	}
	checker := fieldExtractionChecker(t, "dbi", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil RawStruct yields type assertion failure)", result.Count)
	}
}

// --- checkDocdbSnapVPC ---
// checkDocdbSnapVPC reads from snap.VpcId in RawStruct.

func TestRelatedFieldExtraction_DocdbSnap_VPC_ReturnsVpcID(t *testing.T) {
	res := resource.Resource{
		ID:     "rds:cluster-snapshot:my-snap",
		Fields: map[string]string{},
		RawStruct: docdbtypes.DBClusterSnapshot{
			VpcId: aws.String("vpc-abc123"),
		},
	}
	checker := fieldExtractionChecker(t, "docdb-snap", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.TargetType != "vpc" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "vpc")
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpc-abc123" {
		t.Errorf("ResourceIDs = %v, want [vpc-abc123]", result.ResourceIDs)
	}
}

func TestRelatedFieldExtraction_DocdbSnap_VPC_ReturnsZeroWhenNilVpcID(t *testing.T) {
	res := resource.Resource{
		ID:        "rds:cluster-snapshot:my-snap",
		Fields:    map[string]string{},
		RawStruct: docdbtypes.DBClusterSnapshot{VpcId: nil},
	}
	checker := fieldExtractionChecker(t, "docdb-snap", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil VpcId)", result.Count)
	}
}

func TestRelatedFieldExtraction_DocdbSnap_VPC_ReturnsZeroWhenNilRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:     "rds:cluster-snapshot:my-snap",
		Fields: map[string]string{},
	}
	checker := fieldExtractionChecker(t, "docdb-snap", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil RawStruct)", result.Count)
	}
}

// =============================================================================
// SG checkers
// =============================================================================

// --- checkEC2SG ---

func TestRelatedFieldExtraction_EC2_SG_ExtractsGroupIDs(t *testing.T) {
	res := resource.Resource{
		ID:     "i-12345abcde",
		Fields: map[string]string{},
		RawStruct: ec2types.Instance{
			SecurityGroups: []ec2types.GroupIdentifier{
				{GroupId: aws.String("sg-111aaa")},
				{GroupId: aws.String("sg-222bbb")},
			},
		},
	}
	checker := fieldExtractionChecker(t, "ec2", "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if result.TargetType != "sg" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "sg")
	}
	wantIDs := map[string]bool{"sg-111aaa": false, "sg-222bbb": false}
	for _, id := range result.ResourceIDs {
		wantIDs[id] = true
	}
	for id, found := range wantIDs {
		if !found {
			t.Errorf("ResourceIDs missing %q; got %v", id, result.ResourceIDs)
		}
	}
}

func TestRelatedFieldExtraction_EC2_SG_ReturnsZeroWhenEmpty(t *testing.T) {
	res := resource.Resource{
		ID:     "i-12345abcde",
		Fields: map[string]string{},
		RawStruct: ec2types.Instance{
			SecurityGroups: []ec2types.GroupIdentifier{},
		},
	}
	checker := fieldExtractionChecker(t, "ec2", "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty SecurityGroups)", result.Count)
	}
}

func TestRelatedFieldExtraction_EC2_SG_SkipsNilGroupID(t *testing.T) {
	res := resource.Resource{
		ID:     "i-12345abcde",
		Fields: map[string]string{},
		RawStruct: ec2types.Instance{
			SecurityGroups: []ec2types.GroupIdentifier{
				{GroupId: nil},
				{GroupId: aws.String("")},
				{GroupId: aws.String("sg-valid111")},
			},
		},
	}
	checker := fieldExtractionChecker(t, "ec2", "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (only non-empty group IDs)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "sg-valid111" {
		t.Errorf("ResourceIDs = %v, want [sg-valid111]", result.ResourceIDs)
	}
}

func TestRelatedFieldExtraction_EC2_SG_ReturnsNegOneOnBadRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "i-12345abcde",
		Fields:    map[string]string{},
		RawStruct: "not-an-ec2-instance",
	}
	checker := fieldExtractionChecker(t, "ec2", "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (bad RawStruct type)", result.Count)
	}
}

// --- checkELBSG ---

func TestRelatedFieldExtraction_ELB_SG_ExtractsSGIDs(t *testing.T) {
	res := resource.Resource{
		ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc123",
		Fields: map[string]string{},
		RawStruct: elbv2types.LoadBalancer{
			SecurityGroups: []string{"sg-111aaa", "sg-222bbb"},
		},
	}
	checker := fieldExtractionChecker(t, "elb", "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if result.TargetType != "sg" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "sg")
	}
	wantIDs := map[string]bool{"sg-111aaa": false, "sg-222bbb": false}
	for _, id := range result.ResourceIDs {
		wantIDs[id] = true
	}
	for id, found := range wantIDs {
		if !found {
			t.Errorf("ResourceIDs missing %q; got %v", id, result.ResourceIDs)
		}
	}
}

func TestRelatedFieldExtraction_ELB_SG_ReturnsZeroWhenEmpty(t *testing.T) {
	res := resource.Resource{
		ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc123",
		Fields: map[string]string{},
		RawStruct: elbv2types.LoadBalancer{
			SecurityGroups: []string{},
		},
	}
	checker := fieldExtractionChecker(t, "elb", "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty SecurityGroups)", result.Count)
	}
}

func TestRelatedFieldExtraction_ELB_SG_SkipsEmptyStringIDs(t *testing.T) {
	res := resource.Resource{
		ID:     "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc123",
		Fields: map[string]string{},
		RawStruct: elbv2types.LoadBalancer{
			SecurityGroups: []string{"", "sg-valid222", ""},
		},
	}
	checker := fieldExtractionChecker(t, "elb", "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (only non-empty IDs)", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "sg-valid222" {
		t.Errorf("ResourceIDs = %v, want [sg-valid222]", result.ResourceIDs)
	}
}

func TestRelatedFieldExtraction_ELB_SG_ReturnsNegOneOnBadRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-alb/abc123",
		Fields:    map[string]string{},
		RawStruct: "not-a-load-balancer",
	}
	checker := fieldExtractionChecker(t, "elb", "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (bad RawStruct type)", result.Count)
	}
}

// --- checkEKSSG ---

func TestRelatedFieldExtraction_EKS_SG_ExtractsClusterSGID(t *testing.T) {
	res := resource.Resource{
		ID:     "my-eks-cluster",
		Fields: map[string]string{},
		RawStruct: ekstypes.Cluster{
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				ClusterSecurityGroupId: aws.String("sg-cluster111"),
			},
		},
	}
	checker := fieldExtractionChecker(t, "eks", "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.TargetType != "sg" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "sg")
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "sg-cluster111" {
		t.Errorf("ResourceIDs = %v, want [sg-cluster111]", result.ResourceIDs)
	}
}

func TestRelatedFieldExtraction_EKS_SG_ExtractsCombinedSGs(t *testing.T) {
	res := resource.Resource{
		ID:     "my-eks-cluster",
		Fields: map[string]string{},
		RawStruct: ekstypes.Cluster{
			ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
				ClusterSecurityGroupId: aws.String("sg-cluster111"),
				SecurityGroupIds:       []string{"sg-extra222", "sg-extra333"},
			},
		},
	}
	checker := fieldExtractionChecker(t, "eks", "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 3 {
		t.Errorf("Count = %d, want 3 (cluster SG + 2 additional)", result.Count)
	}
	wantIDs := map[string]bool{"sg-cluster111": false, "sg-extra222": false, "sg-extra333": false}
	for _, id := range result.ResourceIDs {
		wantIDs[id] = true
	}
	for id, found := range wantIDs {
		if !found {
			t.Errorf("ResourceIDs missing %q; got %v", id, result.ResourceIDs)
		}
	}
}

func TestRelatedFieldExtraction_EKS_SG_ReturnsZeroWhenNilVpcConfig(t *testing.T) {
	res := resource.Resource{
		ID:     "my-eks-cluster",
		Fields: map[string]string{},
		RawStruct: ekstypes.Cluster{
			ResourcesVpcConfig: nil,
		},
	}
	checker := fieldExtractionChecker(t, "eks", "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil ResourcesVpcConfig)", result.Count)
	}
}

func TestRelatedFieldExtraction_EKS_SG_ReturnsNegOneOnBadRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-eks-cluster",
		Fields:    map[string]string{},
		RawStruct: "not-an-eks-cluster",
	}
	checker := fieldExtractionChecker(t, "eks", "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (bad RawStruct type)", result.Count)
	}
}

// =============================================================================
// KMS checkers
// =============================================================================

// --- checkDbiKMS ---

func TestRelatedFieldExtraction_DBI_KMS_ExtractsKeyIDFromARN(t *testing.T) {
	res := resource.Resource{
		ID:     "my-db-instance",
		Fields: map[string]string{},
		RawStruct: rdstypes.DBInstance{
			KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/abc-123"),
		},
	}
	checker := fieldExtractionChecker(t, "dbi", "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.TargetType != "kms" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "kms")
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "abc-123" {
		t.Errorf("ResourceIDs = %v, want [abc-123]", result.ResourceIDs)
	}
}

func TestRelatedFieldExtraction_DBI_KMS_ReturnsZeroWhenNilKey(t *testing.T) {
	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: rdstypes.DBInstance{KmsKeyId: nil},
	}
	checker := fieldExtractionChecker(t, "dbi", "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil KmsKeyId)", result.Count)
	}
}

func TestRelatedFieldExtraction_DBI_KMS_ReturnsNegOneOnBadRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "my-db-instance",
		Fields:    map[string]string{},
		RawStruct: "not-a-db-instance",
	}
	checker := fieldExtractionChecker(t, "dbi", "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (bad RawStruct type)", result.Count)
	}
}

// --- checkDocdbSnapKMS ---

func TestRelatedFieldExtraction_DocdbSnap_KMS_ExtractsKeyIDFromARN(t *testing.T) {
	res := resource.Resource{
		ID:     "rds:cluster-snapshot:my-snap",
		Fields: map[string]string{},
		RawStruct: docdbtypes.DBClusterSnapshot{
			KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/abc-123"),
		},
	}
	checker := fieldExtractionChecker(t, "docdb-snap", "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.TargetType != "kms" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "kms")
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "abc-123" {
		t.Errorf("ResourceIDs = %v, want [abc-123]", result.ResourceIDs)
	}
}

func TestRelatedFieldExtraction_DocdbSnap_KMS_ReturnsZeroWhenNilKey(t *testing.T) {
	res := resource.Resource{
		ID:        "rds:cluster-snapshot:my-snap",
		Fields:    map[string]string{},
		RawStruct: docdbtypes.DBClusterSnapshot{KmsKeyId: nil},
	}
	checker := fieldExtractionChecker(t, "docdb-snap", "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil KmsKeyId)", result.Count)
	}
}

func TestRelatedFieldExtraction_DocdbSnap_KMS_ReturnsNegOneOnBadRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "rds:cluster-snapshot:my-snap",
		Fields:    map[string]string{},
		RawStruct: "not-a-docdb-snapshot",
	}
	checker := fieldExtractionChecker(t, "docdb-snap", "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (bad RawStruct type)", result.Count)
	}
}

// --- checkEBSKMS ---

func TestRelatedFieldExtraction_EBS_KMS_ExtractsKeyIDFromARN(t *testing.T) {
	res := resource.Resource{
		ID:     "vol-abc123456789",
		Fields: map[string]string{},
		RawStruct: ec2types.Volume{
			KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/abc-123"),
		},
	}
	checker := fieldExtractionChecker(t, "ebs", "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.TargetType != "kms" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "kms")
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "abc-123" {
		t.Errorf("ResourceIDs = %v, want [abc-123]", result.ResourceIDs)
	}
}

func TestRelatedFieldExtraction_EBS_KMS_ReturnsZeroWhenNilKey(t *testing.T) {
	res := resource.Resource{
		ID:        "vol-abc123456789",
		Fields:    map[string]string{},
		RawStruct: ec2types.Volume{KmsKeyId: nil},
	}
	checker := fieldExtractionChecker(t, "ebs", "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil KmsKeyId)", result.Count)
	}
}

func TestRelatedFieldExtraction_EBS_KMS_ReturnsZeroWhenARNHasNoSlash(t *testing.T) {
	res := resource.Resource{
		ID:        "vol-abc123456789",
		Fields:    map[string]string{},
		RawStruct: ec2types.Volume{KmsKeyId: aws.String("not-an-arn")},
	}
	checker := fieldExtractionChecker(t, "ebs", "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (ARN without slash)", result.Count)
	}
}

func TestRelatedFieldExtraction_EBS_KMS_ReturnsNegOneOnBadRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "vol-abc123456789",
		Fields:    map[string]string{},
		RawStruct: "not-an-ebs-volume",
	}
	checker := fieldExtractionChecker(t, "ebs", "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (bad RawStruct type)", result.Count)
	}
}

// --- checkLambdaKMS ---

func TestRelatedFieldExtraction_Lambda_KMS_ExtractsKeyIDFromARN(t *testing.T) {
	res := resource.Resource{
		ID:     "my-lambda-function",
		Fields: map[string]string{},
		RawStruct: lambdatypes.FunctionConfiguration{
			KMSKeyArn: aws.String("arn:aws:kms:us-east-1:123456789012:key/abc-123"),
		},
	}
	checker := fieldExtractionChecker(t, "lambda", "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.TargetType != "kms" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "kms")
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "abc-123" {
		t.Errorf("ResourceIDs = %v, want [abc-123]", result.ResourceIDs)
	}
}

func TestRelatedFieldExtraction_Lambda_KMS_ReturnsZeroWhenNilKey(t *testing.T) {
	res := resource.Resource{
		ID:        "my-lambda-function",
		Fields:    map[string]string{},
		RawStruct: lambdatypes.FunctionConfiguration{KMSKeyArn: nil},
	}
	checker := fieldExtractionChecker(t, "lambda", "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil KMSKeyArn)", result.Count)
	}
}

func TestRelatedFieldExtraction_Lambda_KMS_ReturnsZeroWhenNilRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:     "my-lambda-function",
		Fields: map[string]string{},
	}
	checker := fieldExtractionChecker(t, "lambda", "kms")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil RawStruct — type assertion fails, returns 0)", result.Count)
	}
}

// =============================================================================
// Role checkers
// =============================================================================

// --- checkECSSvcRole ---

func TestRelatedFieldExtraction_ECSSvc_Role_ExtractsRoleNameFromARN(t *testing.T) {
	res := resource.Resource{
		ID:     "arn:aws:ecs:us-east-1:123456789012:service/my-cluster/my-svc",
		Fields: map[string]string{},
		RawStruct: ecstypes.Service{
			RoleArn: aws.String("arn:aws:iam::123456789012:role/my-role"),
		},
	}
	checker := fieldExtractionChecker(t, "ecs-svc", "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.TargetType != "role" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "role")
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-role" {
		t.Errorf("ResourceIDs = %v, want [my-role]", result.ResourceIDs)
	}
}

func TestRelatedFieldExtraction_ECSSvc_Role_ReturnsZeroWhenNilRoleArn(t *testing.T) {
	res := resource.Resource{
		ID:     "arn:aws:ecs:us-east-1:123456789012:service/my-cluster/my-svc",
		Fields: map[string]string{},
		RawStruct: ecstypes.Service{
			RoleArn: nil,
		},
	}
	checker := fieldExtractionChecker(t, "ecs-svc", "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil RoleArn)", result.Count)
	}
}

func TestRelatedFieldExtraction_ECSSvc_Role_ReturnsZeroWhenNilRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:     "arn:aws:ecs:us-east-1:123456789012:service/my-cluster/my-svc",
		Fields: map[string]string{},
	}
	checker := fieldExtractionChecker(t, "ecs-svc", "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil RawStruct)", result.Count)
	}
}

// --- checkECSTaskRole ---
// The DescribeTasks response does NOT include TaskRoleArn/ExecutionRoleArn
// (they live on the TaskDefinition). When the fetcher resolves them it
// populates Fields["task_role"] and Fields["execution_role"] on the task
// Resource; the checker extracts role names from those ARNs. With no role
// fields populated the checker reports Count:0 (no link to surface).

func TestRelatedFieldExtraction_ECSTask_Role_ReturnsZeroWhenNoRoleFields(t *testing.T) {
	res := resource.Resource{
		ID:     "arn:aws:ecs:us-east-1:123456789012:task/my-cluster/abc123",
		Fields: map[string]string{},
		RawStruct: ecstypes.Task{
			TaskArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task/my-cluster/abc123"),
		},
	}
	checker := fieldExtractionChecker(t, "ecs-task", "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no task_role/execution_role fields)", result.Count)
	}
	if result.TargetType != "role" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "role")
	}
}

func TestRelatedFieldExtraction_ECSTask_Role_ReturnsZeroForNilRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:     "arn:aws:ecs:us-east-1:123456789012:task/my-cluster/abc123",
		Fields: map[string]string{},
	}
	checker := fieldExtractionChecker(t, "ecs-task", "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no role fields, regardless of RawStruct)", result.Count)
	}
}

func TestRelatedFieldExtraction_ECSTask_Role_ExtractsTaskRole(t *testing.T) {
	res := resource.Resource{
		ID: "arn:aws:ecs:us-east-1:123456789012:task/my-cluster/abc123",
		Fields: map[string]string{
			"task_role": "arn:aws:iam::123456789012:role/app-task-role",
		},
	}
	checker := fieldExtractionChecker(t, "ecs-task", "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Fatalf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "app-task-role" {
		t.Errorf("ResourceIDs = %v, want [app-task-role]", result.ResourceIDs)
	}
}

func TestRelatedFieldExtraction_ECSTask_Role_ExtractsBothRoles(t *testing.T) {
	res := resource.Resource{
		ID: "arn:aws:ecs:us-east-1:123456789012:task/my-cluster/abc123",
		Fields: map[string]string{
			"task_role":      "arn:aws:iam::123456789012:role/app-task-role",
			"execution_role": "arn:aws:iam::123456789012:role/ecs-task-exec-role",
		},
	}
	checker := fieldExtractionChecker(t, "ecs-task", "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 2 {
		t.Fatalf("Count = %d, want 2", result.Count)
	}
	got := map[string]bool{}
	for _, id := range result.ResourceIDs {
		got[id] = true
	}
	if !got["app-task-role"] || !got["ecs-task-exec-role"] {
		t.Errorf("ResourceIDs = %v, want both app-task-role and ecs-task-exec-role", result.ResourceIDs)
	}
}

// --- checkTrailRole ---

func TestRelatedFieldExtraction_Trail_Role_ExtractsRoleNameFromARN(t *testing.T) {
	res := resource.Resource{
		ID:     "my-trail",
		Fields: map[string]string{},
		RawStruct: cloudtrailtypes.Trail{
			CloudWatchLogsRoleArn: aws.String("arn:aws:iam::123456789012:role/trail-role"),
		},
	}
	checker := fieldExtractionChecker(t, "trail", "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.TargetType != "role" {
		t.Errorf("TargetType = %q, want %q", result.TargetType, "role")
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "trail-role" {
		t.Errorf("ResourceIDs = %v, want [trail-role]", result.ResourceIDs)
	}
}

func TestRelatedFieldExtraction_Trail_Role_ReturnsZeroWhenNilRoleArn(t *testing.T) {
	res := resource.Resource{
		ID:     "my-trail",
		Fields: map[string]string{},
		RawStruct: cloudtrailtypes.Trail{
			CloudWatchLogsRoleArn: nil,
		},
	}
	checker := fieldExtractionChecker(t, "trail", "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil CloudWatchLogsRoleArn)", result.Count)
	}
}

func TestRelatedFieldExtraction_Trail_Role_ReturnsZeroWhenNilRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:     "my-trail",
		Fields: map[string]string{},
	}
	checker := fieldExtractionChecker(t, "trail", "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil RawStruct)", result.Count)
	}
}

func TestRelatedFieldExtraction_Trail_Role_ReturnsZeroWhenARNHasNoSlash(t *testing.T) {
	res := resource.Resource{
		ID:     "my-trail",
		Fields: map[string]string{},
		RawStruct: cloudtrailtypes.Trail{
			CloudWatchLogsRoleArn: aws.String("arn-without-slash"),
		},
	}
	checker := fieldExtractionChecker(t, "trail", "role")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (ARN without slash cannot extract role name)", result.Count)
	}
}
