package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
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
	Lambda         *lambda.Client
	CloudWatch     *cloudwatch.Client
	SNS            *sns.Client
	SQS            *sqs.Client
	ELBv2          *elbv2.Client
	ECS            *ecs.Client
	CloudFormation *cloudformation.Client
	IAM            *iam.Client
	CloudWatchLogs *cloudwatchlogs.Client
	SSM            *ssm.Client
	DynamoDB       *dynamodb.Client
	ACM            *acm.Client
	AutoScaling    *autoscaling.Client
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
		Lambda:         lambda.NewFromConfig(cfg),
		CloudWatch:     cloudwatch.NewFromConfig(cfg),
		SNS:            sns.NewFromConfig(cfg),
		SQS:            sqs.NewFromConfig(cfg),
		ELBv2:          elbv2.NewFromConfig(cfg),
		ECS:            ecs.NewFromConfig(cfg),
		CloudFormation: cloudformation.NewFromConfig(cfg),
		IAM:            iam.NewFromConfig(cfg),
		CloudWatchLogs: cloudwatchlogs.NewFromConfig(cfg),
		SSM:            ssm.NewFromConfig(cfg),
		DynamoDB:       dynamodb.NewFromConfig(cfg),
		ACM:            acm.NewFromConfig(cfg),
		AutoScaling:    autoscaling.NewFromConfig(cfg),
	}
}
