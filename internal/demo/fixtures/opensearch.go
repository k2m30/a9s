package fixtures

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
)

// OpenSearchFixtures holds typed fixture data for OpenSearch.
type OpenSearchFixtures struct {
	Domains []ostypes.DomainStatus
}

// NewOpenSearchFixtures constructs OpenSearchFixtures from the canonical demo data.
func NewOpenSearchFixtures() *OpenSearchFixtures {
	return &OpenSearchFixtures{
		Domains: []ostypes.DomainStatus{
			{
				ARN:           aws.String("arn:aws:es:us-east-1:123456789012:domain/acme-logs"),
				DomainId:      aws.String("123456789012/acme-logs"),
				DomainName:    aws.String("acme-logs"),
				EngineVersion: aws.String("OpenSearch_2.11"),
				Endpoint:      aws.String("search-acme-logs-abc123.us-east-1.es.amazonaws.com"),
				Created:       aws.Bool(true),
				Deleted:       aws.Bool(false),
				ClusterConfig: &ostypes.ClusterConfig{
					InstanceType:  ostypes.OpenSearchPartitionInstanceTypeR6gLargeSearch,
					InstanceCount: aws.Int32(3),
				},
				EBSOptions: &ostypes.EBSOptions{
					EBSEnabled: aws.Bool(true),
					VolumeType: ostypes.VolumeTypeGp3,
					VolumeSize: aws.Int32(100),
				},
				EncryptionAtRestOptions: &ostypes.EncryptionAtRestOptions{
					Enabled: aws.Bool(true),
				},
				DomainEndpointOptions: &ostypes.DomainEndpointOptions{
					EnforceHTTPS: aws.Bool(true),
				},
				Endpoints: map[string]string{
					"vpc": "vpc-search-acme-logs-abc123.us-east-1.es.amazonaws.com",
				},
				AdvancedSecurityOptions: &ostypes.AdvancedSecurityOptions{
					Enabled:                     aws.Bool(true),
					InternalUserDatabaseEnabled: aws.Bool(false),
				},
			},
			{
				ARN:           aws.String("arn:aws:es:us-east-1:123456789012:domain/acme-product-search"),
				DomainId:      aws.String("123456789012/acme-product-search"),
				DomainName:    aws.String("acme-product-search"),
				EngineVersion: aws.String("OpenSearch_2.11"),
				Endpoint:      aws.String("search-acme-product-search-def456.us-east-1.es.amazonaws.com"),
				Created:       aws.Bool(true),
				Deleted:       aws.Bool(false),
				ClusterConfig: &ostypes.ClusterConfig{
					InstanceType:  ostypes.OpenSearchPartitionInstanceTypeR6gXlargeSearch,
					InstanceCount: aws.Int32(2),
				},
				EBSOptions: &ostypes.EBSOptions{
					EBSEnabled: aws.Bool(true),
					VolumeType: ostypes.VolumeTypeGp3,
					VolumeSize: aws.Int32(200),
				},
				EncryptionAtRestOptions: &ostypes.EncryptionAtRestOptions{
					Enabled: aws.Bool(true),
				},
				DomainEndpointOptions: &ostypes.DomainEndpointOptions{
					EnforceHTTPS: aws.Bool(true),
				},
			},
			{
				ARN:           aws.String("arn:aws:es:us-east-1:123456789012:domain/staging-analytics"),
				DomainId:      aws.String("123456789012/staging-analytics"),
				DomainName:    aws.String("staging-analytics"),
				EngineVersion: aws.String("OpenSearch_2.9"),
				Endpoint:      aws.String("search-staging-analytics-ghi789.us-east-1.es.amazonaws.com"),
				Created:       aws.Bool(true),
				Deleted:       aws.Bool(false),
				ClusterConfig: &ostypes.ClusterConfig{
					InstanceType:  ostypes.OpenSearchPartitionInstanceTypeM6gLargeSearch,
					InstanceCount: aws.Int32(1),
				},
				EBSOptions: &ostypes.EBSOptions{
					EBSEnabled: aws.Bool(true),
					VolumeType: ostypes.VolumeTypeGp3,
					VolumeSize: aws.Int32(50),
				},
				EncryptionAtRestOptions: &ostypes.EncryptionAtRestOptions{
					Enabled: aws.Bool(false),
				},
				DomainEndpointOptions: &ostypes.DomainEndpointOptions{
					EnforceHTTPS: aws.Bool(true),
				},
			},
		},
	}
}
