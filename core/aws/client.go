// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
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
	"github.com/aws/aws-sdk-go-v2/service/mwaa"
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
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ServiceClients holds AWS service clients for all supported services.
//
// Per-Session capability stores (IAMPolicies, IdentityStore, RuleSets,
// RegionLists) are stored as unexported fields guarded by sync.RWMutex and
// accessed via thread-safe getter/setter methods. Direct field access would race under
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
	storesMu      sync.RWMutex
	iamPolicies   iamPolicyStore
	identityStore identityStore
	ruleSets      ruleSetStore
	regionLists   *RegionListStore

	// cfg is the session config CreateServiceClients was called with, kept so
	// a client for another Region can be built from it on demand. Zero value
	// when the struct is assembled field by field.
	cfg aws.Config
	// pinned marks a set whose clients a caller has replaced with its own, so
	// every Region is served by those replacements.
	pinned bool
	// parent is the set this one was derived from by InRegion, and which
	// holds the session's capability stores. Nil on the session's own set.
	parent *ServiceClients

	regionMu sync.Mutex
	byRegion map[string]*ServiceClients

	bucketRegionMu sync.Mutex
	bucketRegions  map[string]string

	// Region is the region resolved from the session's AWS config at
	// construction time (profile config, AWS_REGION/AWS_DEFAULT_REGION env,
	// or the SDK default). Immutable after CreateServiceClients returns — safe
	// to read from fetcher/related-checker goroutines without locking. Used to
	// construct ARNs (e.g. pipeline, glue) and to route a read of data that
	// lives in another Region through InRegion.
	Region string

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
	MWAA             MWAAAPI
	Transfer         TransferAPI
	// CostExplorer is account-scoped, not region-scoped —
	// callers never partition it by the session's selected region.
	CostExplorer CostsAPI
	STS          *sts.Client
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
		cfg:    cfg,
		Region: cfg.Region,
		EC2:    ec2.NewFromConfig(cfg),
		// S3/SNS/SFN/Lambda are wrapped with in-flight call coalescing
		// (coalesce.go) — a detail open fires the same read (GetBucketPolicy/
		// GetTopicAttributes/DescribeStateMachine/GetFunction) from a related
		// checker and an on-demand enricher concurrently; singleflight shares
		// one in-flight call's result instead of firing it twice. Once a call
		// completes under a detail operation, its result is also memoized for
		// the rest of that operation (completedResultMemo) so a later,
		// sequential call under the SAME operation still doesn't re-fetch;
		// calls with no active operation, and any call under a later
		// operation, always re-execute.
		S3:               NewCoalescingS3(s3.NewFromConfig(cfg)),
		RDS:              rds.NewFromConfig(cfg),
		ElastiCache:      elasticache.NewFromConfig(cfg),
		DocDB:            docdb.NewFromConfig(cfg),
		EKS:              eks.NewFromConfig(cfg),
		SecretsManager:   secretsmanager.NewFromConfig(cfg),
		Lambda:           NewCoalescingLambda(lambda.NewFromConfig(cfg)),
		CloudWatch:       cloudwatch.NewFromConfig(cfg),
		SNS:              NewCoalescingSNS(sns.NewFromConfig(cfg)),
		SQS:              sqs.NewFromConfig(cfg),
		ELBv2:            elbv2.NewFromConfig(cfg),
		ECS:              NewCoalescingECS(ecs.NewFromConfig(cfg)),
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
		SFN:              NewCoalescingSFN(sfn.NewFromConfig(cfg)),
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
		MWAA:             mwaa.NewFromConfig(cfg),
		Transfer:         transfer.NewFromConfig(cfg),
		CostExplorer:     costexplorer.NewFromConfig(cfg),
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
// thread-safe per the session-side concrete store implementations'
// contracts.

// InRegion returns the client set that reads region. The receiver answers for
// its own Region, for an empty region, for a nil receiver, and for every
// region when it cannot reach another one; any other region is the session's
// own set or one built from the session's config, once per region per
// session, reading the session's one set of capability stores.
// Concurrency-safe.
func (c *ServiceClients) InRegion(region string) *ServiceClients {
	if c == nil {
		return nil
	}
	if region == "" || region == c.Region || c.oneRegion() {
		return c
	}
	// One map of the session's sets, on the session's own, which answers
	// for its Region: a set read from another Region asked for the
	// session's gets the session's.
	o := c.storeOwner()
	if region == o.Region {
		return o
	}
	o.regionMu.Lock()
	defer o.regionMu.Unlock()
	if other, ok := o.byRegion[region]; ok {
		return other
	}
	cfg := o.cfg.Copy()
	cfg.Region = region
	other := CreateServiceClients(cfg)
	other.parent = o
	if o.byRegion == nil {
		o.byRegion = map[string]*ServiceClients{}
	}
	o.byRegion[region] = other
	return other
}

// oneRegion reports whether this set is the whole world it can see: a set
// assembled field by field carries no config to build another region's
// clients from, and a pinned one is serving a caller's own clients, which are
// what the session is meant to talk to wherever a read is routed.
func (c *ServiceClients) oneRegion() bool {
	return c.pinned || c.cfg.Region == ""
}

// storeOwner is the set that holds the session's capability stores. A session
// has one store of each kind however many Regions it reads, so a derived set
// keeps none of its own and a rewire of the session's stores reaches every
// Region's clients.
func (c *ServiceClients) storeOwner() *ServiceClients {
	if c.parent != nil {
		return c.parent
	}
	return c
}

// PinToSessionRegion makes every InRegion lookup answer with this set, for a
// caller that has replaced its clients with substitutes of its own.
func (c *ServiceClients) PinToSessionRegion() {
	c.pinned = true
}

// CloudTrailIn returns the CloudTrail client that reads region's event
// history.
func (c *ServiceClients) CloudTrailIn(region string) CloudTrailAPI {
	return c.InRegion(region).CloudTrail
}

// IAMPolicies returns the session-scoped IAM policy store, or nil if not
// yet wired. Concurrency-safe.
func (c *ServiceClients) IAMPolicies() iamPolicyStore {
	if c == nil {
		return nil
	}
	o := c.storeOwner()
	o.storesMu.RLock()
	defer o.storesMu.RUnlock()
	return o.iamPolicies
}

// SetIAMPolicies replaces the session-scoped IAM policy store. Concurrency-
// safe. Callers passing the new store from the TUI Update loop must also
// keep `Session.IAMPolicies` consistent so a subsequent ClientsReadyMsg or
// Rotate sees the same store reference.
func (c *ServiceClients) SetIAMPolicies(s iamPolicyStore) {
	if c == nil {
		return
	}
	o := c.storeOwner()
	o.storesMu.Lock()
	defer o.storesMu.Unlock()
	o.iamPolicies = s
}

// IdentityStore returns the session-scoped caller-identity store, or nil if
// not yet wired. Concurrency-safe.
func (c *ServiceClients) IdentityStore() identityStore {
	if c == nil {
		return nil
	}
	o := c.storeOwner()
	o.storesMu.RLock()
	defer o.storesMu.RUnlock()
	return o.identityStore
}

// SetIdentityStore replaces the session-scoped caller-identity store.
// Concurrency-safe.
func (c *ServiceClients) SetIdentityStore(s identityStore) {
	if c == nil {
		return
	}
	o := c.storeOwner()
	o.storesMu.Lock()
	defer o.storesMu.Unlock()
	o.identityStore = s
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
	o := c.storeOwner()
	o.storesMu.RLock()
	defer o.storesMu.RUnlock()
	return o.ruleSets
}

// SetRuleSets replaces the session-scoped SES rule set store. The previous
// store is NOT cleared — any in-flight fetcher that captured a reference
// before this swap continues to write to the orphaned old store, and its
// late writes do not affect the new active store.
func (c *ServiceClients) SetRuleSets(s ruleSetStore) {
	if c == nil {
		return
	}
	o := c.storeOwner()
	o.storesMu.Lock()
	defer o.storesMu.Unlock()
	o.ruleSets = s
}

// RegionLists returns the session-scoped store of first pages read in other
// Regions. A set no session has wired keeps one of its own. Concurrency-safe.
func (c *ServiceClients) RegionLists() *RegionListStore {
	if c == nil {
		return nil
	}
	o := c.storeOwner()
	o.storesMu.Lock()
	defer o.storesMu.Unlock()
	if o.regionLists == nil {
		o.regionLists = NewRegionListStore()
	}
	return o.regionLists
}

// SetRegionLists replaces the session-scoped per-Region list store; a read in
// flight keeps writing to the store it captured.
func (c *ServiceClients) SetRegionLists(s *RegionListStore) {
	if c == nil {
		return
	}
	o := c.storeOwner()
	o.storesMu.Lock()
	defer o.storesMu.Unlock()
	o.regionLists = s
}

// errClientMissing is domain.ErrClientMissing, the one signal that a read's
// client is absent; resource.ErrorRelated reads it as unknown.
var errClientMissing = domain.ErrClientMissing

// svcClients asserts clients holds an initialized *ServiceClients, the
// prelude every catalog fetcher/reveal closure needs before it can dereference
// a specific service client. A value that is none is errClientMissing.
func svcClients(clients any) (*ServiceClients, error) {
	c, ok := clients.(*ServiceClients)
	if !ok || c == nil {
		return nil, errClientMissing
	}
	return c, nil
}

// fetcherWithClients adapts a *ServiceClients-typed page fetcher into the
// domain.PaginatedFetcher shape a catalog ResourceTypeDef's Fetcher/
// AvailabilityFetcher field expects, running the svcClients assertion once
// instead of duplicating it in every registration closure.
func fetcherWithClients(fn func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error)) func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
	return func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, err := svcClients(clients)
		if err != nil {
			return resource.FetchResult{}, err
		}
		return fn(ctx, c, continuationToken)
	}
}

// clientsOf adapts a *ServiceClients-typed client selector into the shape
// of a catalog ResourceTypeDef's FetcherClients field. A value that is no
// client set holds none of the clients.
func clientsOf(fn func(c *ServiceClients) []any) func(clients any) []any {
	return func(clients any) []any {
		c, err := svcClients(clients)
		if err != nil {
			return []any{nil}
		}
		return fn(c)
	}
}

// filteredFetcherWithClients is fetcherWithClients for a ResourceTypeDef's
// FilteredFetcher field (domain.FilteredPaginatedFetcher).
func filteredFetcherWithClients(fn func(ctx context.Context, c *ServiceClients, filter map[string]string, continuationToken string) (resource.FetchResult, error)) func(ctx context.Context, clients any, filter map[string]string, continuationToken string) (resource.FetchResult, error) {
	return func(ctx context.Context, clients any, filter map[string]string, continuationToken string) (resource.FetchResult, error) {
		c, err := svcClients(clients)
		if err != nil {
			return resource.FetchResult{}, err
		}
		return fn(ctx, c, filter, continuationToken)
	}
}

// childFetcherWithClients is fetcherWithClients for a ResourceTypeDef's
// ChildFetcher field (domain.PaginatedChildFetcher).
func childFetcherWithClients(fn func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error)) func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
	return func(ctx context.Context, clients any, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
		c, err := svcClients(clients)
		if err != nil {
			return resource.FetchResult{}, err
		}
		return fn(ctx, c, parentCtx, continuationToken)
	}
}

// fetchByIDsWithClients is fetcherWithClients for a ResourceTypeDef's
// FetchByIDs field (domain.FetchByIDsFunc).
func fetchByIDsWithClients(fn func(ctx context.Context, c *ServiceClients, ids []string) ([]resource.Resource, error)) func(ctx context.Context, clients any, ids []string) ([]resource.Resource, error) {
	return func(ctx context.Context, clients any, ids []string) ([]resource.Resource, error) {
		c, err := svcClients(clients)
		if err != nil {
			return nil, err
		}
		return fn(ctx, c, ids)
	}
}

// revealWithClients is fetcherWithClients for a ResourceTypeDef's Reveal
// field (domain.RevealFetcher).
func revealWithClients(fn func(ctx context.Context, c *ServiceClients, resourceID string) (string, error)) func(ctx context.Context, clients any, resourceID string) (string, error) {
	return func(ctx context.Context, clients any, resourceID string) (string, error) {
		c, err := svcClients(clients)
		if err != nil {
			return "", err
		}
		return fn(ctx, c, resourceID)
	}
}
