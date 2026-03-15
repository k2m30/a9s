package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// ServiceClients holds AWS service clients for all supported services.
type ServiceClients struct {
	EC2            *ec2.Client
	S3             *s3.Client
	RDS            *rds.Client
	ElastiCache    *elasticache.Client
	DocDB          *docdb.Client
	EKS            *eks.Client
	SecretsManager *secretsmanager.Client
}

// NewAWSSession creates a new AWS config using the given profile and region.
// If profile is empty, the default profile is used.
// If region is empty, the default region from the config/environment is used.
func NewAWSSession(profile, region string) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error

	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	return config.LoadDefaultConfig(context.Background(), opts...)
}

// CreateServiceClients creates all service clients from the given AWS config.
func CreateServiceClients(cfg aws.Config) *ServiceClients {
	return &ServiceClients{
		EC2:            ec2.NewFromConfig(cfg),
		S3:             s3.NewFromConfig(cfg),
		RDS:            rds.NewFromConfig(cfg),
		ElastiCache:    elasticache.NewFromConfig(cfg),
		DocDB:          docdb.NewFromConfig(cfg),
		EKS:            eks.NewFromConfig(cfg),
		SecretsManager: secretsmanager.NewFromConfig(cfg),
	}
}
