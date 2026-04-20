package unit_test

// aws_opensearch_related_extra_test.go — additional coverage for opensearch_related.go
// Covers: checkOpenSearchLogs, checkOpenSearchSG, checkOpenSearchVPC,
//         checkOpenSearchKMS, checkOpenSearchSubnet (Pattern F — no cache).

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// --- checkOpenSearchLogs (Pattern F — extracts from LogPublishingOptions) ---

func TestRelated_OpenSearch_Logs_ExtractsSingleGroup(t *testing.T) {
	source := resource.Resource{
		ID: "acme-logs",
		RawStruct: ostypes.DomainStatus{
			DomainName: aws.String("acme-logs"),
			LogPublishingOptions: map[string]ostypes.LogPublishingOption{
				string(ostypes.LogTypeIndexSlowLogs): {
					CloudWatchLogsLogGroupArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/opensearch/domains/acme-logs/index-slow-logs:*"),
				},
			},
		},
	}
	checker := opensearchCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 {
		t.Fatalf("ResourceIDs = %v, want 1 entry", result.ResourceIDs)
	}
	want := "/aws/opensearch/domains/acme-logs/index-slow-logs"
	if result.ResourceIDs[0] != want {
		t.Errorf("ResourceIDs[0] = %q, want %q", result.ResourceIDs[0], want)
	}
}

func TestRelated_OpenSearch_Logs_DeduplicatesGroups(t *testing.T) {
	// Same log group ARN referenced by two log types → deduplicated to 1 entry.
	arn := "arn:aws:logs:us-east-1:123456789012:log-group:/aws/opensearch/domains/acme-logs/shared:*"
	source := resource.Resource{
		ID: "acme-logs",
		RawStruct: ostypes.DomainStatus{
			DomainName: aws.String("acme-logs"),
			LogPublishingOptions: map[string]ostypes.LogPublishingOption{
				string(ostypes.LogTypeIndexSlowLogs):  {CloudWatchLogsLogGroupArn: aws.String(arn)},
				string(ostypes.LogTypeSearchSlowLogs): {CloudWatchLogsLogGroupArn: aws.String(arn)},
			},
		},
	}
	checker := opensearchCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (deduplicated)", result.Count)
	}
}

func TestRelated_OpenSearch_Logs_EmptyLogPublishingOptions(t *testing.T) {
	source := resource.Resource{
		ID: "acme-logs",
		RawStruct: ostypes.DomainStatus{
			DomainName:           aws.String("acme-logs"),
			LogPublishingOptions: map[string]ostypes.LogPublishingOption{},
		},
	}
	checker := opensearchCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no log options configured)", result.Count)
	}
}

func TestRelated_OpenSearch_Logs_NilARN(t *testing.T) {
	source := resource.Resource{
		ID: "acme-logs",
		RawStruct: ostypes.DomainStatus{
			DomainName: aws.String("acme-logs"),
			LogPublishingOptions: map[string]ostypes.LogPublishingOption{
				string(ostypes.LogTypeIndexSlowLogs): {CloudWatchLogsLogGroupArn: nil},
			},
		},
	}
	checker := opensearchCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil ARN)", result.Count)
	}
}

func TestRelated_OpenSearch_Logs_WrongRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "acme-logs",
		RawStruct: "not-a-domain-status",
	}
	checker := opensearchCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// --- checkOpenSearchSG (Pattern F) ---

func TestRelated_OpenSearch_SG_Found(t *testing.T) {
	source := resource.Resource{
		ID: "acme-logs",
		RawStruct: ostypes.DomainStatus{
			DomainName: aws.String("acme-logs"),
			VPCOptions: &ostypes.VPCDerivedInfo{
				SecurityGroupIds: []string{"sg-0abc1234", "sg-0def5678"},
			},
		},
	}
	checker := opensearchCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if result.ResourceIDs[0] != "sg-0abc1234" {
		t.Errorf("ResourceIDs[0] = %q, want sg-0abc1234", result.ResourceIDs[0])
	}
}

func TestRelated_OpenSearch_SG_NoVPCOptions(t *testing.T) {
	source := resource.Resource{
		ID:        "acme-logs",
		RawStruct: ostypes.DomainStatus{DomainName: aws.String("acme-logs"), VPCOptions: nil},
	}
	checker := opensearchCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (public domain — no VPC)", result.Count)
	}
}

func TestRelated_OpenSearch_SG_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "acme-logs", RawStruct: "not-a-domain"}
	checker := opensearchCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1", result.Count)
	}
}

// --- checkOpenSearchVPC (Pattern F) ---

func TestRelated_OpenSearch_VPC_Found(t *testing.T) {
	source := resource.Resource{
		ID: "acme-logs",
		RawStruct: ostypes.DomainStatus{
			DomainName: aws.String("acme-logs"),
			VPCOptions: &ostypes.VPCDerivedInfo{
				VPCId: aws.String("vpc-0a1b2c3d4e5f60001"),
			},
		},
	}
	checker := opensearchCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "vpc-0a1b2c3d4e5f60001" {
		t.Errorf("ResourceIDs[0] = %q, want vpc-0a1b2c3d4e5f60001", result.ResourceIDs[0])
	}
}

func TestRelated_OpenSearch_VPC_PublicDomain(t *testing.T) {
	source := resource.Resource{
		ID:        "acme-logs",
		RawStruct: ostypes.DomainStatus{DomainName: aws.String("acme-logs"), VPCOptions: nil},
	}
	checker := opensearchCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (public domain)", result.Count)
	}
}

func TestRelated_OpenSearch_VPC_EmptyVPCId(t *testing.T) {
	source := resource.Resource{
		ID: "acme-logs",
		RawStruct: ostypes.DomainStatus{
			DomainName: aws.String("acme-logs"),
			VPCOptions: &ostypes.VPCDerivedInfo{VPCId: aws.String("")},
		},
	}
	checker := opensearchCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty VPCId)", result.Count)
	}
}

// --- checkOpenSearchKMS (Pattern F) ---

func TestRelated_OpenSearch_KMS_ARNExtractedToID(t *testing.T) {
	source := resource.Resource{
		ID: "acme-logs",
		RawStruct: ostypes.DomainStatus{
			DomainName: aws.String("acme-logs"),
			EncryptionAtRestOptions: &ostypes.EncryptionAtRestOptions{
				KmsKeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/mrk-acme001"),
			},
		},
	}
	checker := opensearchCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "mrk-acme001" {
		t.Errorf("ResourceIDs[0] = %q, want mrk-acme001 (last ARN segment)", result.ResourceIDs[0])
	}
}

func TestRelated_OpenSearch_KMS_NotEncrypted(t *testing.T) {
	source := resource.Resource{
		ID:        "acme-logs",
		RawStruct: ostypes.DomainStatus{DomainName: aws.String("acme-logs"), EncryptionAtRestOptions: nil},
	}
	checker := opensearchCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no encryption)", result.Count)
	}
}

func TestRelated_OpenSearch_KMS_EmptyKeyID(t *testing.T) {
	source := resource.Resource{
		ID: "acme-logs",
		RawStruct: ostypes.DomainStatus{
			DomainName:             aws.String("acme-logs"),
			EncryptionAtRestOptions: &ostypes.EncryptionAtRestOptions{KmsKeyId: aws.String("")},
		},
	}
	checker := opensearchCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty KmsKeyId)", result.Count)
	}
}

// --- checkOpenSearchSubnet (Pattern F) ---

func TestRelated_OpenSearch_Subnet_Found(t *testing.T) {
	source := resource.Resource{
		ID: "acme-logs",
		RawStruct: ostypes.DomainStatus{
			DomainName: aws.String("acme-logs"),
			VPCOptions: &ostypes.VPCDerivedInfo{
				SubnetIds: []string{"subnet-001", "subnet-002"},
			},
		},
	}
	checker := opensearchCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
}

func TestRelated_OpenSearch_Subnet_NoVPCOptions(t *testing.T) {
	source := resource.Resource{
		ID:        "acme-logs",
		RawStruct: ostypes.DomainStatus{DomainName: aws.String("acme-logs"), VPCOptions: nil},
	}
	checker := opensearchCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (public domain)", result.Count)
	}
}

func TestRelated_OpenSearch_Subnet_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "acme-logs", RawStruct: "wrong"}
	checker := opensearchCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1", result.Count)
	}
}

// --- checkOpenSearchAlarms — empty ID early exit ---

func TestRelated_OpenSearch_Alarms_EmptyID(t *testing.T) {
	source := resource.Resource{
		ID:        "",
		RawStruct: ostypes.DomainStatus{DomainName: aws.String("")},
	}
	checker := opensearchCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty domain ID)", result.Count)
	}
}
