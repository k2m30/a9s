package unit_test

// aws6_fake_refuses_unregistered_key_test.go — every demo fake is honest about
// a key it does not hold.
//
// A fake stands in for a whole account. A key the fixtures never registered is
// therefore a key the account does not have, and the only truthful answer is
// the service's not-found error. Answering an empty result instead turns a
// fixture gap into a confident zero: a pivot to a resource nobody modelled
// renders "none", which is the exact class of lie demo mode exists to disprove
// and the reason the IAM fake grew the rule first.
//
// Each pin drives ONE by-id lookup per fake with a key no fixture registers,
// and asserts two things: the service's own error code, and that
// awsclient.IsNotFoundErr recognizes it. The second is what production reads —
// a fake that refuses with a spelling the predicate does not know is refused in
// a way no caller can act on.
//
// The registered half of the contract is pinned by
// TestDemoFakesStillAnswerEveryRegisteredKey below, which drains every type's
// real fetcher over these same fakes: if a refusal is too eager, a bench that
// used to produce rows stops producing them.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
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
	"github.com/aws/aws-sdk-go-v2/service/mwaa"
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
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"
)

// unregistered is the key no fixture in any service registers. It is shaped
// like a plausible name so a fake that pattern-matches on shape rather than on
// its own registry cannot pass by accident.
const unregisteredKey = "acme-not-in-any-fixture"

// fakeNotFoundPin names one demo fake, the by-id lookup that stands for it, and
// the error code that service answers for a key it does not hold.
//
// The code is the SERVICE's, not a uniform one: AWS spells this differently per
// API, and a fake answering a code its own SDK never emits would be a shape
// production code has no reason to handle.
type fakeNotFoundPin struct {
	name     string
	wantCode string
	// missingKey overrides unregisteredKey for a lookup whose input must be
	// well-formed before the fake can even ask whether it holds it (an ARN, a
	// queue URL). The key is still one no fixture registers — the shape is
	// valid so the refusal is about registration, not about parsing.
	missingKey string
	lookup     func(ctx context.Context, id string) error
}

// key returns the key this pin probes with.
func (p fakeNotFoundPin) key() string {
	if p.missingKey != "" {
		return p.missingKey
	}
	return unregisteredKey
}

func fakeNotFoundPins() []fakeNotFoundPin {
	return []fakeNotFoundPin{
		{"acm", "ResourceNotFoundException", "arn:aws:acm:us-east-1:123456789012:certificate/00000000-0000-0000-0000-000000000000", func(ctx context.Context, id string) error {
			_, err := fakes.NewACM().DescribeCertificate(ctx, &acm.DescribeCertificateInput{CertificateArn: aws.String(id)})
			return err
		}},
		{"apigw", "NotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewAPIGW().GetStages(ctx, &apigatewayv2.GetStagesInput{ApiId: aws.String(id)})
			return err
		}},
		{"apigw_v1", "NotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewAPIGWV1().GetStages(ctx, &apigateway.GetStagesInput{RestApiId: aws.String(id)})
			return err
		}},
		{"asg", "ValidationError", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewASG().DescribeScalingActivities(ctx, &autoscaling.DescribeScalingActivitiesInput{AutoScalingGroupName: aws.String(id)})
			return err
		}},
		{"athena", "InvalidRequestException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewAthena().GetWorkGroup(ctx, &athena.GetWorkGroupInput{WorkGroup: aws.String(id)})
			return err
		}},
		{"backup", "ResourceNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewBackup().DescribeBackupVault(ctx, &backup.DescribeBackupVaultInput{BackupVaultName: aws.String(id)})
			return err
		}},
		{"cfn", "ValidationError", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewCFN().DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: aws.String(id)})
			return err
		}},
		{"cloudfront", "NoSuchDistribution", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewCloudFront().GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{Id: aws.String(id)})
			return err
		}},
		{"cloudtrail", "TrailNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewCloudTrail().GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{Name: aws.String(id)})
			return err
		}},
		{"cloudwatch", "ResourceNotFound", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewCloudWatch().DescribeAlarmHistory(ctx, &cloudwatch.DescribeAlarmHistoryInput{AlarmName: aws.String(id)})
			return err
		}},
		{"codeartifact", "ResourceNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewCodeArtifact().DescribeDomain(ctx, &codeartifact.DescribeDomainInput{Domain: aws.String(id)})
			return err
		}},
		{"codebuild", "ResourceNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewCodeBuild().ListBuildsForProject(ctx, &codebuild.ListBuildsForProjectInput{ProjectName: aws.String(id)})
			return err
		}},
		{"codepipeline", "PipelineNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewCodePipeline().GetPipeline(ctx, &codepipeline.GetPipelineInput{Name: aws.String(id)})
			return err
		}},
		{"cwlogs", "ResourceNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewCWLogs().DescribeLogStreams(ctx, &cloudwatchlogs.DescribeLogStreamsInput{LogGroupName: aws.String(id)})
			return err
		}},
		{"docdb", "DBSubnetGroupNotFoundFault", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewDocDB().DescribeDBSubnetGroups(ctx, &docdb.DescribeDBSubnetGroupsInput{DBSubnetGroupName: aws.String(id)})
			return err
		}},
		{"dynamodb", "ResourceNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewDynamoDB().DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(id)})
			return err
		}},
		{"eb", "ResourceNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewEB().DescribeEnvironmentHealth(ctx, &elasticbeanstalk.DescribeEnvironmentHealthInput{EnvironmentName: aws.String(id)})
			return err
		}},
		{"ec2", "InvalidInstanceID.NotFound", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewEC2().DescribeInstanceAttribute(ctx, &ec2.DescribeInstanceAttributeInput{InstanceId: aws.String(id)})
			return err
		}},
		{"ecr", "RepositoryNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewECR().ListImages(ctx, &ecr.ListImagesInput{RepositoryName: aws.String(id)})
			return err
		}},
		{"ecs", "ClusterNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewECS().ListServices(ctx, &ecs.ListServicesInput{Cluster: aws.String(id)})
			return err
		}},
		{"efs", "FileSystemNotFound", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewEFS().DescribeMountTargets(ctx, &efs.DescribeMountTargetsInput{FileSystemId: aws.String(id)})
			return err
		}},
		{"eks", "ResourceNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewEKS().DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(id)})
			return err
		}},
		{"elasticache", "CacheSubnetGroupNotFoundFault", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewElastiCache().DescribeCacheSubnetGroups(ctx, &elasticache.DescribeCacheSubnetGroupsInput{CacheSubnetGroupName: aws.String(id)})
			return err
		}},
		{"elb", "LoadBalancerNotFoundException", "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-not-in-any-fixture/0000000000000000", func(ctx context.Context, id string) error {
			_, err := fakes.NewELB().DescribeLoadBalancerAttributes(ctx, &elbv2.DescribeLoadBalancerAttributesInput{LoadBalancerArn: aws.String(id)})
			return err
		}},
		{"eventbridge", "ResourceNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewEventBridge().ListTargetsByRule(ctx, &eventbridge.ListTargetsByRuleInput{Rule: aws.String(id)})
			return err
		}},
		{"glue", "EntityNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewGlue().GetJobRuns(ctx, &glue.GetJobRunsInput{JobName: aws.String(id)})
			return err
		}},
		{"iam", "NoSuchEntity", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewIAM().GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String(id)})
			return err
		}},
		{"kinesis", "ResourceNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewKinesis().DescribeStreamSummary(ctx, &kinesis.DescribeStreamSummaryInput{StreamName: aws.String(id)})
			return err
		}},
		{"kms", "NotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewKMS().DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: aws.String(id)})
			return err
		}},
		{"lambda", "ResourceNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewLambda().GetFunction(ctx, &lambda.GetFunctionInput{FunctionName: aws.String(id)})
			return err
		}},
		{"msk", "NotFoundException", "arn:aws:kafka:us-east-1:123456789012:cluster/acme-not-in-any-fixture/00000000-0000-0000-0000-000000000000-1", func(ctx context.Context, id string) error {
			_, err := fakes.NewMSK().DescribeClusterV2(ctx, &kafka.DescribeClusterV2Input{ClusterArn: aws.String(id)})
			return err
		}},
		{"mwaa", "ResourceNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewMWAA().GetEnvironment(ctx, &mwaa.GetEnvironmentInput{Name: aws.String(id)})
			return err
		}},
		{"opensearch", "ResourceNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewOpenSearch().DescribeDomainConfig(ctx, &opensearch.DescribeDomainConfigInput{DomainName: aws.String(id)})
			return err
		}},
		{"r53", "NoSuchHostedZone", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewR53().GetHostedZone(ctx, &route53.GetHostedZoneInput{Id: aws.String(id)})
			return err
		}},
		{"rds", "DBSubnetGroupNotFoundFault", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewRDS().DescribeDBSubnetGroups(ctx, &rds.DescribeDBSubnetGroupsInput{DBSubnetGroupName: aws.String(id)})
			return err
		}},
		{"redshift", "ClusterNotFoundFault", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewRedshift().DescribeLoggingStatus(ctx, &redshift.DescribeLoggingStatusInput{ClusterIdentifier: aws.String(id)})
			return err
		}},
		{"s3", "NoSuchBucket", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewS3().GetBucketVersioning(ctx, &s3.GetBucketVersioningInput{Bucket: aws.String(id)})
			return err
		}},
		{"secrets", "ResourceNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewSecrets().GetResourcePolicy(ctx, &secretsmanager.GetResourcePolicyInput{SecretId: aws.String(id)})
			return err
		}},
		{"ses", "NotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewSES().GetEmailIdentity(ctx, &sesv2.GetEmailIdentityInput{EmailIdentity: aws.String(id)})
			return err
		}},
		{"sfn", "StateMachineDoesNotExist", "arn:aws:states:us-east-1:123456789012:stateMachine:acme-not-in-any-fixture", func(ctx context.Context, id string) error {
			_, err := fakes.NewSFN().DescribeStateMachine(ctx, &sfn.DescribeStateMachineInput{StateMachineArn: aws.String(id)})
			return err
		}},
		{"sns", "NotFound", "arn:aws:sns:us-east-1:123456789012:acme-not-in-any-fixture", func(ctx context.Context, id string) error {
			_, err := fakes.NewSNS().GetTopicAttributes(ctx, &sns.GetTopicAttributesInput{TopicArn: aws.String(id)})
			return err
		}},
		{"sqs", "QueueDoesNotExist", "https://sqs.us-east-1.amazonaws.com/123456789012/acme-not-in-any-fixture", func(ctx context.Context, id string) error {
			_, err := fakes.NewSQS().GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{QueueUrl: aws.String(id)})
			return err
		}},
		{"ssm", "ParameterNotFound", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewSSM().GetParameter(ctx, &ssm.GetParameterInput{Name: aws.String(id)})
			return err
		}},
		{"transfer", "ResourceNotFoundException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewTransfer().DescribeServer(ctx, &transfer.DescribeServerInput{ServerId: aws.String(id)})
			return err
		}},
		{"waf", "WAFNonexistentItemException", "", func(ctx context.Context, id string) error {
			_, err := fakes.NewWAF().GetWebACL(ctx, &wafv2.GetWebACLInput{Id: aws.String(id), Name: aws.String(id), Scope: "REGIONAL"})
			return err
		}},
	}
}

// TestDemoFakeRefusesAnUnregisteredKey is the pin per fake.
func TestDemoFakeRefusesAnUnregisteredKey(t *testing.T) {
	for _, pin := range fakeNotFoundPins() {
		t.Run(pin.name, func(t *testing.T) {
			key := pin.key()
			err := pin.lookup(context.Background(), key)
			if err == nil {
				t.Fatalf("%s: a lookup of %q the fixtures never registered succeeded; "+
					"an empty answer for a key the account does not hold reads as \"this resource has none\", "+
					"want the service's %s", pin.name, key, pin.wantCode)
			}
			if !awsclient.ErrCodeIs(err, pin.wantCode) {
				t.Errorf("%s: refused %q with %v, want error code %q — the fake answers its own service's spelling",
					pin.name, key, err, pin.wantCode)
			}
			if !awsclient.IsNotFoundErr(err) {
				t.Errorf("%s: refused %q with %v, which awsclient.IsNotFoundErr does not recognize — "+
					"a refusal production cannot classify is a refusal no caller can act on",
					pin.name, key, err)
			}
		})
	}
}

// TestDemoFakesStillAnswerEveryRegisteredKey is the other half: refusing an
// unregistered key must not start refusing a registered one. Every type's real
// fetcher is drained over the same fakes, and a type that produced rows before
// still does.
//
// This is the guard against an over-eager refusal — the failure mode a
// too-clever "unknown key" check introduces, where the demo related panels go
// empty for keys the fixtures DO register.
func TestDemoFakesStillAnswerEveryRegisteredKey(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)

	populated := 0
	for _, td := range resource.AllResourceTypes() {
		if len(byType[td.ShortName]) > 0 {
			populated++
		}
	}
	// The demo account models most of the catalog. A refusal that swallowed
	// whole types would show up here long before anyone opened the app.
	if populated < 60 {
		t.Errorf("only %d resource types produced demo rows; the fixtures used to populate far more — "+
			"an unregistered-key refusal is rejecting keys the fixtures DO register", populated)
	}
}
