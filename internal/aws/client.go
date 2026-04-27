package aws

import (
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
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
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
)

// ServiceClients holds AWS service clients for all supported services.
//
// Per-Session capability stores (IAMPolicies, IdentityStore, RuleSets) are
// stored as unexported fields guarded by sync.RWMutex and accessed via
// thread-safe getter/setter methods. Direct field access would race under
// `-race` because:
//
//   - Writes happen on the Bubble Tea Update goroutine (handleClientsReady,
//     handleRefresh swap on Ctrl+R).
//   - Reads happen on fetcher goroutines spawned via tea.Cmd
//     (FetchIAMPoliciesByIDsFull, accountIDFromClients,
//     sesActiveReceiptRuleSet).
//
// Interface fields are 2-word values; concurrent unsynchronized read/write
// is undefined behavior per the Go memory model. Methods serialize the
// access.
type ServiceClients struct {
	storesMu       sync.RWMutex
	iamPolicies    iamPolicyStore
	identityStore  identityStore
	ruleSets       ruleSetStore

	EC2              EC2API
	S3               S3API
	RDS              RDSAPI
	ElastiCache      ElastiCacheAPI
	DocDB            DocDBAPI
	EKS              EKSAPI
	SecretsManager   SecretsManagerAPI
	Lambda           LambdaAPI
	CloudWatch       CloudWatchAPI
	SNS              SNSAPI
	SQS              SQSAPI
	ELBv2            ELBv2API
	ECS              ECSAPI
	CloudFormation   CFNAPI
	IAM              IAMAPI
	CloudWatchLogs   CWLogsAPI
	SSM              SSMAPI
	DynamoDB         DynamoDBAPI
	ACM              ACMAPI
	AutoScaling      ASGAPI
	CloudFront       CloudFrontAPI
	Route53          Route53API
	APIGatewayV1     APIGatewayV1API
	APIGatewayV2     APIGatewayV2API
	ECR              ECRAPI
	EFS              EFSAPI
	EventBridge      EventBridgeAPI
	SFN              SFNAPI
	CodePipeline     CodePipelineAPI
	Kinesis          KinesisAPI
	WAFv2            WAFv2API
	Glue             GlueAPI
	ElasticBeanstalk ElasticBeanstalkAPI
	SES              SESV1API
	SESv2            SESv2API
	Redshift         RedshiftAPI
	CloudTrail       CloudTrailAPI
	Athena           AthenaAPI
	CodeArtifact     CodeArtifactAPI
	CodeBuild        CodeBuildAPI
	OpenSearch       OpenSearchAPI
	KMS              KMSAPI
	MSK              MSKAPI
	Backup           BackupAPI
	STS              *sts.Client
}

// NewAWSSessionContext creates a new AWS config using the given context, profile,
// and region. If profile is empty, the default profile is used. If region is
// empty, the default region from the config/environment is used.
// The provided context is forwarded to the AWS SDK config loader.
func NewAWSSessionContext(ctx context.Context, profile, region string) (aws.Config, error) {
	var opts []func(*config.LoadOptions) error

	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	return config.LoadDefaultConfig(ctx, opts...)
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
		APIGatewayV1:     apigateway.NewFromConfig(cfg),
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
		SES:              ses.NewFromConfig(cfg),
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
		STS:              sts.NewFromConfig(cfg),
	}
}

// ─── Per-Session capability store accessors ───────────────────────────────
//
// Each accessor pair (getter / setter) serializes access via storesMu so the
// Bubble Tea Update goroutine (which swaps stores on profile/region switch
// or Ctrl+R refresh) and fetcher goroutines (which read the stores during
// resource fetch / related-checker dispatch) don't race on the underlying
// interface field.
//
// Read paths use a read-lock; write paths use a write-lock. The lock guards
// only the field itself — methods on the returned store are independently
// thread-safe per session.{PolicyStore,IdentityStore,RuleSetStore} contracts.

// IAMPolicies returns the session-scoped IAM policy store, or nil if not
// yet wired. Concurrency-safe.
func (c *ServiceClients) IAMPolicies() iamPolicyStore {
	if c == nil {
		return nil
	}
	c.storesMu.RLock()
	defer c.storesMu.RUnlock()
	return c.iamPolicies
}

// SetIAMPolicies replaces the session-scoped IAM policy store. Concurrency-
// safe. Callers passing the new store from the TUI Update loop must also
// keep `Session.IAMPolicies` consistent so a subsequent ClientsReadyMsg or
// Rotate sees the same store reference.
func (c *ServiceClients) SetIAMPolicies(s iamPolicyStore) {
	if c == nil {
		return
	}
	c.storesMu.Lock()
	defer c.storesMu.Unlock()
	c.iamPolicies = s
}

// IdentityStore returns the session-scoped caller-identity store, or nil if
// not yet wired. Concurrency-safe.
func (c *ServiceClients) IdentityStore() identityStore {
	if c == nil {
		return nil
	}
	c.storesMu.RLock()
	defer c.storesMu.RUnlock()
	return c.identityStore
}

// SetIdentityStore replaces the session-scoped caller-identity store.
// Concurrency-safe.
func (c *ServiceClients) SetIdentityStore(s identityStore) {
	if c == nil {
		return
	}
	c.storesMu.Lock()
	defer c.storesMu.Unlock()
	c.identityStore = s
}

// RuleSets returns the session-scoped SES rule set store, or nil if not yet
// wired. Concurrency-safe. Callers should capture the returned reference at
// the start of a fetch operation so a concurrent SetRuleSets (e.g. Ctrl+R
// refresh) can't repoison the new active store with a late writer — the
// captured reference will be the now-orphaned old store. See
// `sesActiveReceiptRuleSet` for the canonical capture pattern.
func (c *ServiceClients) RuleSets() ruleSetStore {
	if c == nil {
		return nil
	}
	c.storesMu.RLock()
	defer c.storesMu.RUnlock()
	return c.ruleSets
}

// SetRuleSets replaces the session-scoped SES rule set store. The previous
// store is NOT cleared — any in-flight fetcher that captured a reference
// before this swap continues to write to the orphaned old store, and its
// late writes do not affect the new active store.
func (c *ServiceClients) SetRuleSets(s ruleSetStore) {
	if c == nil {
		return
	}
	c.storesMu.Lock()
	defer c.storesMu.Unlock()
	c.ruleSets = s
}
