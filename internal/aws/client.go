package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
)

// ServiceClients holds AWS service clients for all supported services.
type ServiceClients struct {
	EC2              *ec2.Client
	S3               *s3.Client
	RDS              *rds.Client
	ElastiCache      *elasticache.Client
	DocDB            *docdb.Client
	EKS              *eks.Client
	SecretsManager   *secretsmanager.Client
	Lambda           *lambda.Client
	CloudWatch       *cloudwatch.Client
	SNS              *sns.Client
	SQS              *sqs.Client
	ELBv2            *elbv2.Client
	ECS              *ecs.Client
	CloudFormation   *cloudformation.Client
	IAM              *iam.Client
	CloudWatchLogs   *cloudwatchlogs.Client
	SSM              *ssm.Client
	DynamoDB         *dynamodb.Client
	ACM              *acm.Client
	AutoScaling      *autoscaling.Client
	CloudFront       *cloudfront.Client
	Route53          *route53.Client
	APIGatewayV2     *apigatewayv2.Client
	ECR              *ecr.Client
	EFS              *efs.Client
	EventBridge      *eventbridge.Client
	SFN              *sfn.Client
	CodePipeline     *codepipeline.Client
	Kinesis          *kinesis.Client
	WAFv2            *wafv2.Client
	Glue             *glue.Client
	ElasticBeanstalk *elasticbeanstalk.Client
	SESv2            *sesv2.Client
	Redshift         *redshift.Client
	CloudTrail       *cloudtrail.Client
	Athena           *athena.Client
	CodeArtifact     *codeartifact.Client
	CodeBuild        *codebuild.Client
	OpenSearch       *opensearch.Client
	KMS              *kms.Client
	MSK              *kafka.Client
	Backup           *backup.Client
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
		EC2:              ec2.NewFromConfig(cfg),
		S3:               s3.NewFromConfig(cfg),
		RDS:              rds.NewFromConfig(cfg),
		ElastiCache:      elasticache.NewFromConfig(cfg),
		DocDB:            docdb.NewFromConfig(cfg),
		EKS:              eks.NewFromConfig(cfg),
		SecretsManager:   secretsmanager.NewFromConfig(cfg),
		Lambda:           lambda.NewFromConfig(cfg),
		CloudWatch:       cloudwatch.NewFromConfig(cfg),
		SNS:              sns.NewFromConfig(cfg),
		SQS:              sqs.NewFromConfig(cfg),
		ELBv2:            elbv2.NewFromConfig(cfg),
		ECS:              ecs.NewFromConfig(cfg),
		CloudFormation:   cloudformation.NewFromConfig(cfg),
		IAM:              iam.NewFromConfig(cfg),
		CloudWatchLogs:   cloudwatchlogs.NewFromConfig(cfg),
		SSM:              ssm.NewFromConfig(cfg),
		DynamoDB:         dynamodb.NewFromConfig(cfg),
		ACM:              acm.NewFromConfig(cfg),
		AutoScaling:      autoscaling.NewFromConfig(cfg),
		CloudFront:       cloudfront.NewFromConfig(cfg),
		Route53:          route53.NewFromConfig(cfg),
		APIGatewayV2:     apigatewayv2.NewFromConfig(cfg),
		ECR:              ecr.NewFromConfig(cfg),
		EFS:              efs.NewFromConfig(cfg),
		EventBridge:      eventbridge.NewFromConfig(cfg),
		SFN:              sfn.NewFromConfig(cfg),
		CodePipeline:     codepipeline.NewFromConfig(cfg),
		Kinesis:          kinesis.NewFromConfig(cfg),
		WAFv2:            wafv2.NewFromConfig(cfg),
		Glue:             glue.NewFromConfig(cfg),
		ElasticBeanstalk: elasticbeanstalk.NewFromConfig(cfg),
		SESv2:            sesv2.NewFromConfig(cfg),
		Redshift:         redshift.NewFromConfig(cfg),
		CloudTrail:       cloudtrail.NewFromConfig(cfg),
		Athena:           athena.NewFromConfig(cfg),
		CodeArtifact:     codeartifact.NewFromConfig(cfg),
		CodeBuild:        codebuild.NewFromConfig(cfg),
		OpenSearch:       opensearch.NewFromConfig(cfg),
		KMS:              kms.NewFromConfig(cfg),
		MSK:              kafka.NewFromConfig(cfg),
		Backup:           backup.NewFromConfig(cfg),
	}
}
