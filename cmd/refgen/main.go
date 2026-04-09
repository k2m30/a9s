package main

import (
	"fmt"
	"reflect"

	"github.com/k2m30/a9s/v3/internal/fieldpath"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"

	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
)

type resourceDef struct {
	name    string
	comment string
	typ     reflect.Type
}

// SQSQueueAttributes is a synthetic struct representing the fields returned by
// SQS GetQueueAttributes (QueueAttributeNameAll). SQS returns a map[string]string
// so there is no SDK struct to reflect on.
type SQSQueueAttributes struct {
	QueueArn                              string
	ApproximateNumberOfMessages           string
	ApproximateNumberOfMessagesNotVisible string
	ApproximateNumberOfMessagesDelayed    string
	CreatedTimestamp                      string
	LastModifiedTimestamp                 string
	VisibilityTimeout                     string
	MaximumMessageSize                    string
	MessageRetentionPeriod                string
	DelaySeconds                          string
	Policy                                string
	RedrivePolicy                         string
	RedriveAllowPolicy                    string
	DeadLetterTargetArn                   string
	FifoQueue                             string
	ContentBasedDeduplication             string
	DeduplicationScope                    string
	FifoThroughputLimit                   string
	KmsMasterKeyId                        string
	KmsDataKeyReusePeriodSeconds          string
	SqsManagedSseEnabled                  string
}

func main() {
	resources := []resourceDef{
		{"s3", "s3types.Bucket", reflect.TypeFor[s3types.Bucket]()},
		{"s3_objects", "s3types.Object", reflect.TypeFor[s3types.Object]()},
		{"ec2", "ec2types.Instance", reflect.TypeFor[ec2types.Instance]()},
		{"dbi", "rdstypes.DBInstance", reflect.TypeFor[rdstypes.DBInstance]()},
		{"dbi_events", "rdstypes.Event", reflect.TypeFor[rdstypes.Event]()},
		{"redis", "elasticachetypes.CacheCluster", reflect.TypeFor[elasticachetypes.CacheCluster]()},
		{"dbc", "docdbtypes.DBCluster", reflect.TypeFor[docdbtypes.DBCluster]()},
		{"eks", "ekstypes.Cluster", reflect.TypeFor[ekstypes.Cluster]()},
		{"secrets", "smtypes.SecretListEntry", reflect.TypeFor[smtypes.SecretListEntry]()},
		{"vpc", "ec2types.Vpc", reflect.TypeFor[ec2types.Vpc]()},
		{"sg", "ec2types.SecurityGroup", reflect.TypeFor[ec2types.SecurityGroup]()},
		{"ng", "ekstypes.Nodegroup", reflect.TypeFor[ekstypes.Nodegroup]()},
		{"subnet", "ec2types.Subnet", reflect.TypeFor[ec2types.Subnet]()},
		{"rtb", "ec2types.RouteTable", reflect.TypeFor[ec2types.RouteTable]()},
		{"nat", "ec2types.NatGateway", reflect.TypeFor[ec2types.NatGateway]()},
		{"igw", "ec2types.InternetGateway", reflect.TypeFor[ec2types.InternetGateway]()},
		{"lambda", "lambdatypes.FunctionConfiguration", reflect.TypeFor[lambdatypes.FunctionConfiguration]()},
		{"alarm", "cwtypes.MetricAlarm", reflect.TypeFor[cwtypes.MetricAlarm]()},
		{"sns", "snstypes.Topic", reflect.TypeFor[snstypes.Topic]()},
		{"elb", "elbv2types.LoadBalancer", reflect.TypeFor[elbv2types.LoadBalancer]()},
		{"tg", "elbv2types.TargetGroup", reflect.TypeFor[elbv2types.TargetGroup]()},
		{"ecs", "ecstypes.Cluster", reflect.TypeFor[ecstypes.Cluster]()},
		{"ecs-svc", "ecstypes.Service", reflect.TypeFor[ecstypes.Service]()},
		{"cfn", "cfntypes.Stack", reflect.TypeFor[cfntypes.Stack]()},
		{"role", "iamtypes.Role", reflect.TypeFor[iamtypes.Role]()},
		{"logs", "cwlogstypes.LogGroup", reflect.TypeFor[cwlogstypes.LogGroup]()},
		{"ssm", "ssmtypes.ParameterMetadata", reflect.TypeFor[ssmtypes.ParameterMetadata]()},
		{"ddb", "ddbtypes.TableDescription", reflect.TypeFor[ddbtypes.TableDescription]()},
		{"eip", "ec2types.Address", reflect.TypeFor[ec2types.Address]()},
		{"acm", "acmtypes.CertificateSummary", reflect.TypeFor[acmtypes.CertificateSummary]()},
		{"asg", "asgtypes.AutoScalingGroup", reflect.TypeFor[asgtypes.AutoScalingGroup]()},
		{"ecs-task", "ecstypes.Task", reflect.TypeFor[ecstypes.Task]()},
		{"policy", "iamtypes.Policy", reflect.TypeFor[iamtypes.Policy]()},
		{"rds-snap", "rdstypes.DBSnapshot", reflect.TypeFor[rdstypes.DBSnapshot]()},
		{"tgw", "ec2types.TransitGateway", reflect.TypeFor[ec2types.TransitGateway]()},
		{"vpce", "ec2types.VpcEndpoint", reflect.TypeFor[ec2types.VpcEndpoint]()},
		{"eni", "ec2types.NetworkInterface", reflect.TypeFor[ec2types.NetworkInterface]()},
		{"sns-sub", "snstypes.Subscription", reflect.TypeFor[snstypes.Subscription]()},
		{"sns_subscriptions", "snstypes.Subscription", reflect.TypeFor[snstypes.Subscription]()},
		{"sqs", "SQSQueueAttributes (synthetic)", reflect.TypeFor[SQSQueueAttributes]()},
		{"iam-user", "iamtypes.User", reflect.TypeFor[iamtypes.User]()},
		{"iam-group", "iamtypes.Group", reflect.TypeFor[iamtypes.Group]()},
		{"docdb-snap", "docdbtypes.DBClusterSnapshot", reflect.TypeFor[docdbtypes.DBClusterSnapshot]()},
		{"cf", "cftypes.DistributionSummary", reflect.TypeFor[cftypes.DistributionSummary]()},
		{"r53", "r53types.HostedZone", reflect.TypeFor[r53types.HostedZone]()},
		{"r53_records", "r53types.ResourceRecordSet", reflect.TypeFor[r53types.ResourceRecordSet]()},
		{"apigw", "apigwtypes.Api", reflect.TypeFor[apigwtypes.Api]()},
		{"ecr", "ecrtypes.Repository", reflect.TypeFor[ecrtypes.Repository]()},
		{"efs", "efstypes.FileSystemDescription", reflect.TypeFor[efstypes.FileSystemDescription]()},
		{"eb-rule", "eventbridgetypes.Rule", reflect.TypeFor[eventbridgetypes.Rule]()},
		{"eb_rule_targets", "eventbridgetypes.Target", reflect.TypeFor[eventbridgetypes.Target]()},
		{"sfn", "sfntypes.StateMachineListItem", reflect.TypeFor[sfntypes.StateMachineListItem]()},
		{"pipeline", "cptypes.PipelineSummary", reflect.TypeFor[cptypes.PipelineSummary]()},
		{"kinesis", "kinesistypes.StreamSummary", reflect.TypeFor[kinesistypes.StreamSummary]()},
		{"waf", "wafv2types.WebACLSummary", reflect.TypeFor[wafv2types.WebACLSummary]()},
		{"glue", "gluetypes.Job", reflect.TypeFor[gluetypes.Job]()},
		{"glue_runs", "gluetypes.JobRun", reflect.TypeFor[gluetypes.JobRun]()},
		{"eb", "ebtypes.EnvironmentDescription", reflect.TypeFor[ebtypes.EnvironmentDescription]()},
		{"ses", "sesv2types.IdentityInfo", reflect.TypeFor[sesv2types.IdentityInfo]()},
		{"redshift", "redshifttypes.Cluster", reflect.TypeFor[redshifttypes.Cluster]()},
		{"trail", "cloudtrailtypes.Trail", reflect.TypeFor[cloudtrailtypes.Trail]()},
		{"athena", "athenatypes.WorkGroupSummary", reflect.TypeFor[athenatypes.WorkGroupSummary]()},
		{"codeartifact", "codeartifacttypes.RepositorySummary", reflect.TypeFor[codeartifacttypes.RepositorySummary]()},
		{"cb", "cbtypes.Project", reflect.TypeFor[cbtypes.Project]()},
		{"opensearch", "ostypes.DomainStatus", reflect.TypeFor[ostypes.DomainStatus]()},
		{"kms", "kmstypes.KeyMetadata", reflect.TypeFor[kmstypes.KeyMetadata]()},
		{"msk", "kafkatypes.Cluster", reflect.TypeFor[kafkatypes.Cluster]()},
		{"backup", "backuptypes.BackupPlansListMember", reflect.TypeFor[backuptypes.BackupPlansListMember]()},
		{"log_streams", "cwlogstypes.LogStream", reflect.TypeFor[cwlogstypes.LogStream]()},
		{"log_events", "cwlogstypes.OutputLogEvent", reflect.TypeFor[cwlogstypes.OutputLogEvent]()},
		{"tg_health", "elbv2types.TargetHealthDescription", reflect.TypeFor[elbv2types.TargetHealthDescription]()},
		{"ecs_svc_events", "ecstypes.ServiceEvent", reflect.TypeFor[ecstypes.ServiceEvent]()},
		{"ecs_tasks", "ecstypes.Task", reflect.TypeFor[ecstypes.Task]()},
		{"ecs_svc_logs", "cwlogstypes.FilteredLogEvent", reflect.TypeFor[cwlogstypes.FilteredLogEvent]()},
		{"cfn_events", "cfntypes.StackEvent", reflect.TypeFor[cfntypes.StackEvent]()},
		{"cfn_resources", "cfntypes.StackResourceSummary", reflect.TypeFor[cfntypes.StackResourceSummary]()},
		{"asg_activities", "asgtypes.Activity", reflect.TypeFor[asgtypes.Activity]()},
		{"alarm_history", "cwtypes.AlarmHistoryItem", reflect.TypeFor[cwtypes.AlarmHistoryItem]()},
		{"elb_listeners", "elbv2types.Listener", reflect.TypeFor[elbv2types.Listener]()},
		{"sfn_executions", "sfntypes.ExecutionListItem", reflect.TypeFor[sfntypes.ExecutionListItem]()},
		{"sfn_execution_history", "sfntypes.HistoryEvent", reflect.TypeFor[sfntypes.HistoryEvent]()},
		{"cb_builds", "cbtypes.Build", reflect.TypeFor[cbtypes.Build]()},
		{"cb_build_logs", "cwlogstypes.OutputLogEvent", reflect.TypeFor[cwlogstypes.OutputLogEvent]()},
		{"ecr_images", "ecrtypes.ImageDetail", reflect.TypeFor[ecrtypes.ImageDetail]()},
		{"role_policies", "awsclient.RolePolicyRow", reflect.TypeFor[awsclient.RolePolicyRow]()},
		{"iam_group_members", "iamtypes.User", reflect.TypeFor[iamtypes.User]()},
		{"elb_listener_rules", "elbv2types.Rule", reflect.TypeFor[elbv2types.Rule]()},
		{"ebs", "ec2types.Volume", reflect.TypeFor[ec2types.Volume]()},
		{"ebs-snap", "ec2types.Snapshot", reflect.TypeFor[ec2types.Snapshot]()},
		{"ami", "ec2types.Image", reflect.TypeFor[ec2types.Image]()},
		{"ct-events", "cloudtrailtypes.Event", reflect.TypeFor[cloudtrailtypes.Event]()},
	}

	fmt.Println("# views_reference.yaml")
	fmt.Println("# Generated from AWS SDK Go v2 struct reflection")
	fmt.Println("# Use these paths in your ~/.a9s/views/ configuration files")
	fmt.Println()

	for _, r := range resources {
		paths := fieldpath.EnumeratePaths(r.typ, "")
		fmt.Printf("%s:  # %s\n", r.name, r.comment)
		for _, p := range paths {
			fmt.Printf("  - %s\n", p)
		}
		fmt.Println()
	}
}
