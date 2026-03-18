// Package aws defines narrow interfaces for each AWS service operation used by a9s.
// These interfaces enable dependency injection and testability.
package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/docdb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// EC2DescribeInstancesAPI defines the interface for the EC2 DescribeInstances operation.
type EC2DescribeInstancesAPI interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// S3ListBucketsAPI defines the interface for the S3 ListBuckets operation.
type S3ListBucketsAPI interface {
	ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
}

// S3ListObjectsV2API defines the interface for the S3 ListObjectsV2 operation.
type S3ListObjectsV2API interface {
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// S3GetBucketLocationAPI defines the interface for the S3 GetBucketLocation operation.
type S3GetBucketLocationAPI interface {
	GetBucketLocation(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
}

// RDSDescribeDBInstancesAPI defines the interface for the RDS DescribeDBInstances operation.
type RDSDescribeDBInstancesAPI interface {
	DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
}

// ElastiCacheDescribeCacheClustersAPI defines the interface for the ElastiCache DescribeCacheClusters operation.
type ElastiCacheDescribeCacheClustersAPI interface {
	DescribeCacheClusters(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error)
}

// DocDBDescribeDBClustersAPI defines the interface for the DocumentDB DescribeDBClusters operation.
type DocDBDescribeDBClustersAPI interface {
	DescribeDBClusters(ctx context.Context, params *docdb.DescribeDBClustersInput, optFns ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error)
}

// EKSListClustersAPI defines the interface for the EKS ListClusters operation.
type EKSListClustersAPI interface {
	ListClusters(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error)
}

// EKSDescribeClusterAPI defines the interface for the EKS DescribeCluster operation.
type EKSDescribeClusterAPI interface {
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
}

// SecretsManagerListSecretsAPI defines the interface for the SecretsManager ListSecrets operation.
type SecretsManagerListSecretsAPI interface {
	ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
}

// SecretsManagerGetSecretValueAPI defines the interface for the SecretsManager GetSecretValue operation.
type SecretsManagerGetSecretValueAPI interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// EC2DescribeVpcsAPI defines the interface for the EC2 DescribeVpcs operation.
type EC2DescribeVpcsAPI interface {
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
}

// EC2DescribeSecurityGroupsAPI defines the interface for the EC2 DescribeSecurityGroups operation.
type EC2DescribeSecurityGroupsAPI interface {
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
}

// EKSListNodegroupsAPI defines the interface for the EKS ListNodegroups operation.
type EKSListNodegroupsAPI interface {
	ListNodegroups(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error)
}

// EKSDescribeNodegroupAPI defines the interface for the EKS DescribeNodegroup operation.
type EKSDescribeNodegroupAPI interface {
	DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error)
}
