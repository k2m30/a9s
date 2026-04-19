package unit_test

// aws_msk_related_extra_test.go — additional coverage for msk_related.go.
// Covers: checkMSKSG (wrong-rawstruct), checkMSKKMS, checkMSKSubnet,
// checkMSKVPC, checkMSKLogs, checkMSKS3, checkMSKSecrets.
// checkMSKAlarms, checkMSKLambda, checkMSKCFN are already covered in
// aws_msk_related_test.go and related_uncovered_struct_test.go.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// fakeMSKScram — implements MSKListScramSecretsAPI for checkMSKSecrets tests
// ---------------------------------------------------------------------------

type fakeMSKScram struct {
	secretArns []string
	err        error
}

func (f *fakeMSKScram) ListScramSecrets(_ context.Context, _ *kafka.ListScramSecretsInput, _ ...func(*kafka.Options)) (*kafka.ListScramSecretsOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &kafka.ListScramSecretsOutput{SecretArnList: f.secretArns}, nil
}

// MSKAPI stubs — fakeMSKScram must satisfy awsclient.MSKAPI to be assignable to
// ServiceClients.MSK. The production checkMSKSecrets then does a type assertion
// to MSKListScramSecretsAPI (which only needs ListScramSecrets), so these stubs
// never run in tests — they exist solely to satisfy the interface.
func (f *fakeMSKScram) ListClustersV2(_ context.Context, _ *kafka.ListClustersV2Input, _ ...func(*kafka.Options)) (*kafka.ListClustersV2Output, error) {
	return &kafka.ListClustersV2Output{}, nil
}

func (f *fakeMSKScram) DescribeClusterV2(_ context.Context, _ *kafka.DescribeClusterV2Input, _ ...func(*kafka.Options)) (*kafka.DescribeClusterV2Output, error) {
	return &kafka.DescribeClusterV2Output{}, nil
}

var _ awsclient.MSKAPI = (*fakeMSKScram)(nil)

// --- checkMSKSG: wrong RawStruct → -1 ---

func TestRelated_MSK_SG_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "msk-cluster-1", RawStruct: "not-a-kafka-cluster"}
	checker := mskCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// --- checkMSKKMS (Pattern F — reads DataVolumeKMSKeyId) ---

func TestRelated_MSK_KMS_Found(t *testing.T) {
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Provisioned: &kafkatypes.Provisioned{
				EncryptionInfo: &kafkatypes.EncryptionInfo{
					EncryptionAtRest: &kafkatypes.EncryptionAtRest{
						DataVolumeKMSKeyId: aws.String("mrk-abc1234567890123456789012345678"),
					},
				},
			},
		},
	}
	checker := mskCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "mrk-abc1234567890123456789012345678" {
		t.Errorf("ResourceIDs[0] = %q, want mrk-abc1234567890123456789012345678", result.ResourceIDs[0])
	}
}

func TestRelated_MSK_KMS_NilProvisioned(t *testing.T) {
	source := resource.Resource{
		ID:        "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{ClusterName: aws.String("analytics-kafka-cluster"), Provisioned: nil},
	}
	checker := mskCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Provisioned)", result.Count)
	}
}

func TestRelated_MSK_KMS_NilEncryptionAtRest(t *testing.T) {
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Provisioned: &kafkatypes.Provisioned{
				EncryptionInfo: &kafkatypes.EncryptionInfo{
					EncryptionAtRest: nil,
				},
			},
		},
	}
	checker := mskCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil EncryptionAtRest)", result.Count)
	}
}

func TestRelated_MSK_KMS_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "msk-cluster-kms", RawStruct: "not-a-kafka-cluster"}
	checker := mskCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct: assertStruct fails → default 0)", result.Count)
	}
}

// --- checkMSKSubnet (Pattern F — reads ClientSubnets from Provisioned) ---

func TestRelated_MSK_Subnet_Found(t *testing.T) {
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Provisioned: &kafkatypes.Provisioned{
				BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
					ClientSubnets: []string{"subnet-aaa111", "subnet-bbb222", "subnet-ccc333"},
				},
			},
		},
	}
	checker := mskCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 3 {
		t.Errorf("Count = %d, want 3", result.Count)
	}
	if result.ResourceIDs[0] != "subnet-aaa111" {
		t.Errorf("ResourceIDs[0] = %q, want subnet-aaa111", result.ResourceIDs[0])
	}
}

func TestRelated_MSK_Subnet_NilBrokerNodeGroupInfo(t *testing.T) {
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Provisioned: &kafkatypes.Provisioned{BrokerNodeGroupInfo: nil},
		},
	}
	checker := mskCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil BrokerNodeGroupInfo)", result.Count)
	}
}

func TestRelated_MSK_Subnet_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "msk-cluster-sub", RawStruct: "not-a-kafka-cluster"}
	checker := mskCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// --- checkMSKVPC (Pattern F+C — looks up first ClientSubnet in subnet cache) ---

func TestRelated_MSK_VPC_FoundViaSubnetCache(t *testing.T) {
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Provisioned: &kafkatypes.Provisioned{
				BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
					ClientSubnets: []string{"subnet-aaa111"},
				},
			},
		},
	}
	subnetRes := resource.Resource{
		ID: "subnet-aaa111",
		RawStruct: ec2types.Subnet{
			SubnetId: aws.String("subnet-aaa111"),
			VpcId:    aws.String("vpc-kafkavpc001"),
		},
	}
	cache := resource.ResourceCache{
		"subnet": resource.ResourceCacheEntry{Resources: []resource.Resource{subnetRes}},
	}
	checker := mskCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "vpc-kafkavpc001" {
		t.Errorf("ResourceIDs[0] = %q, want vpc-kafkavpc001", result.ResourceIDs[0])
	}
}

func TestRelated_MSK_VPC_NilProvisioned(t *testing.T) {
	source := resource.Resource{
		ID:        "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{ClusterName: aws.String("analytics-kafka-cluster"), Provisioned: nil},
	}
	checker := mskCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Provisioned)", result.Count)
	}
}

func TestRelated_MSK_VPC_EmptySubnets(t *testing.T) {
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Provisioned: &kafkatypes.Provisioned{
				BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
					ClientSubnets: []string{},
				},
			},
		},
	}
	checker := mskCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no subnets)", result.Count)
	}
}

func TestRelated_MSK_VPC_SubnetNotInCache(t *testing.T) {
	// Subnet is listed on cluster, but not in cache and no clients → -1
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Provisioned: &kafkatypes.Provisioned{
				BrokerNodeGroupInfo: &kafkatypes.BrokerNodeGroupInfo{
					ClientSubnets: []string{"subnet-missing"},
				},
			},
		},
	}
	checker := mskCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (empty cache, nil clients)", result.Count)
	}
}

// --- checkMSKLogs (Pattern F — reads CloudWatchLogs LogGroup) ---

func TestRelated_MSK_Logs_Found(t *testing.T) {
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Provisioned: &kafkatypes.Provisioned{
				LoggingInfo: &kafkatypes.LoggingInfo{
					BrokerLogs: &kafkatypes.BrokerLogs{
						CloudWatchLogs: &kafkatypes.CloudWatchLogs{
							Enabled:  aws.Bool(true),
							LogGroup: aws.String("/msk/analytics-kafka-cluster/broker"),
						},
					},
				},
			},
		},
	}
	checker := mskCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "/msk/analytics-kafka-cluster/broker" {
		t.Errorf("ResourceIDs[0] = %q, want /msk/analytics-kafka-cluster/broker", result.ResourceIDs[0])
	}
}

func TestRelated_MSK_Logs_DisabledCloudWatch(t *testing.T) {
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Provisioned: &kafkatypes.Provisioned{
				LoggingInfo: &kafkatypes.LoggingInfo{
					BrokerLogs: &kafkatypes.BrokerLogs{
						CloudWatchLogs: &kafkatypes.CloudWatchLogs{
							Enabled: aws.Bool(false),
						},
					},
				},
			},
		},
	}
	checker := mskCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (CloudWatch logging disabled)", result.Count)
	}
}

func TestRelated_MSK_Logs_NilLoggingInfo(t *testing.T) {
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Provisioned: &kafkatypes.Provisioned{LoggingInfo: nil},
		},
	}
	checker := mskCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil LoggingInfo)", result.Count)
	}
}

func TestRelated_MSK_Logs_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "msk-cluster-logs", RawStruct: "not-a-kafka-cluster"}
	checker := mskCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// --- checkMSKS3 (Pattern F — reads S3.Bucket from BrokerLogs) ---

func TestRelated_MSK_S3_Found(t *testing.T) {
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Provisioned: &kafkatypes.Provisioned{
				LoggingInfo: &kafkatypes.LoggingInfo{
					BrokerLogs: &kafkatypes.BrokerLogs{
						S3: &kafkatypes.S3{
							Enabled: aws.Bool(true),
							Bucket:  aws.String("my-msk-logs-bucket"),
						},
					},
				},
			},
		},
	}
	checker := mskCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if result.ResourceIDs[0] != "my-msk-logs-bucket" {
		t.Errorf("ResourceIDs[0] = %q, want my-msk-logs-bucket", result.ResourceIDs[0])
	}
}

func TestRelated_MSK_S3_Disabled(t *testing.T) {
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Provisioned: &kafkatypes.Provisioned{
				LoggingInfo: &kafkatypes.LoggingInfo{
					BrokerLogs: &kafkatypes.BrokerLogs{
						S3: &kafkatypes.S3{
							Enabled: aws.Bool(false),
							Bucket:  aws.String("my-msk-logs-bucket"),
						},
					},
				},
			},
		},
	}
	checker := mskCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (S3 logging disabled)", result.Count)
	}
}

func TestRelated_MSK_S3_NilS3Config(t *testing.T) {
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Provisioned: &kafkatypes.Provisioned{
				LoggingInfo: &kafkatypes.LoggingInfo{
					BrokerLogs: &kafkatypes.BrokerLogs{S3: nil},
				},
			},
		},
	}
	checker := mskCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil S3 config)", result.Count)
	}
}

func TestRelated_MSK_S3_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "msk-cluster-s3", RawStruct: "not-a-kafka-cluster"}
	checker := mskCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

// --- checkMSKSecrets (Pattern C — calls kafka:ListScramSecrets) ---

func TestRelated_MSK_Secrets_Found(t *testing.T) {
	const clusterARN = "arn:aws:kafka:us-east-1:123456789012:cluster/analytics-kafka-cluster/abc-123"
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			ClusterArn:  aws.String(clusterARN),
		},
	}
	clients := &awsclient.ServiceClients{
		MSK: &fakeMSKScram{
			secretArns: []string{
				"arn:aws:secretsmanager:us-east-1:123456789012:secret:AmazonMSK_kafka-scram-secret-abc123",
				"arn:aws:secretsmanager:us-east-1:123456789012:secret:AmazonMSK_kafka-user2-xyz456",
			},
		},
	}
	checker := mskCheckerByTarget(t, "secrets")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if result.ResourceIDs[0] != "AmazonMSK_kafka-scram-secret-abc123" {
		t.Errorf("ResourceIDs[0] = %q, want AmazonMSK_kafka-scram-secret-abc123", result.ResourceIDs[0])
	}
	if result.ResourceIDs[1] != "AmazonMSK_kafka-user2-xyz456" {
		t.Errorf("ResourceIDs[1] = %q, want AmazonMSK_kafka-user2-xyz456", result.ResourceIDs[1])
	}
}

func TestRelated_MSK_Secrets_EmptyList(t *testing.T) {
	const clusterARN = "arn:aws:kafka:us-east-1:123456789012:cluster/analytics-kafka-cluster/abc-123"
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			ClusterArn:  aws.String(clusterARN),
		},
	}
	clients := &awsclient.ServiceClients{
		MSK: &fakeMSKScram{secretArns: []string{}},
	}
	checker := mskCheckerByTarget(t, "secrets")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no SCRAM secrets)", result.Count)
	}
}

func TestRelated_MSK_Secrets_NilClusterARN(t *testing.T) {
	// No ClusterArn → early return Count:0 (unknown is not appropriate since identity is missing)
	source := resource.Resource{
		ID:        "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{ClusterName: aws.String("analytics-kafka-cluster"), ClusterArn: nil},
	}
	checker := mskCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no cluster ARN)", result.Count)
	}
}

func TestRelated_MSK_Secrets_NilClients(t *testing.T) {
	const clusterARN = "arn:aws:kafka:us-east-1:123456789012:cluster/analytics-kafka-cluster/abc-123"
	source := resource.Resource{
		ID: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			ClusterArn:  aws.String(clusterARN),
		},
	}
	checker := mskCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

func TestRelated_MSK_Secrets_WrongRawStruct(t *testing.T) {
	source := resource.Resource{ID: "msk-cluster-secrets", RawStruct: "not-a-kafka-cluster"}
	checker := mskCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct — assertStruct fails, empty ClusterArn)", result.Count)
	}
}
