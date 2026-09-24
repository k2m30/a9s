package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	t570Account   = "123456789012"
	t570LambdaFn  = "t570-orders-dlq-consumer"
	t570LambdaARN = "arn:aws:lambda:us-east-1:123456789012:function:t570-orders-dlq-consumer"
	t570DLQQueue  = "data-pipeline-dlq"
	t570DLQARN    = "arn:aws:sqs:us-east-1:123456789012:data-pipeline-dlq"
	t570FnLogs    = "/app/custom/no-retention"
)

// t570Lambda serves the demo account's functions plus extra ones, answering
// the per-function calls for the extras the way Lambda does.
type t570Lambda struct {
	awsclient.LambdaAPI
	extra []lambdatypes.FunctionConfiguration
}

func (f *t570Lambda) ListFunctions(ctx context.Context, in *lambda.ListFunctionsInput, opt ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	out, err := f.LambdaAPI.ListFunctions(ctx, in, opt...)
	if err != nil || in.Marker != nil {
		return out, err
	}
	out.Functions = append(out.Functions, f.extra...)
	return out, nil
}

func (f *t570Lambda) find(nameOrARN string) (lambdatypes.FunctionConfiguration, bool) {
	for _, fn := range f.extra {
		if aws.ToString(fn.FunctionName) == nameOrARN || aws.ToString(fn.FunctionArn) == nameOrARN {
			return fn, true
		}
	}
	return lambdatypes.FunctionConfiguration{}, false
}

func (f *t570Lambda) GetFunction(ctx context.Context, in *lambda.GetFunctionInput, opt ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error) {
	if fn, ok := f.find(aws.ToString(in.FunctionName)); ok {
		return &lambda.GetFunctionOutput{Configuration: &fn}, nil
	}
	return f.LambdaAPI.GetFunction(ctx, in, opt...)
}

func (f *t570Lambda) ListTags(ctx context.Context, in *lambda.ListTagsInput, opt ...func(*lambda.Options)) (*lambda.ListTagsOutput, error) {
	if _, ok := f.find(aws.ToString(in.Resource)); ok {
		return &lambda.ListTagsOutput{Tags: map[string]string{}}, nil
	}
	return f.LambdaAPI.ListTags(ctx, in, opt...)
}

// A function whose asynchronous-invocation failures go to an SQS queue and
// whose logs go to a group it names itself (LoggingConfig.LogGroup).
func t570DLQFunction() lambdatypes.FunctionConfiguration {
	return lambdatypes.FunctionConfiguration{
		FunctionName:     aws.String(t570LambdaFn),
		FunctionArn:      aws.String(t570LambdaARN),
		Runtime:          lambdatypes.RuntimePython312,
		Handler:          aws.String("app.handler"),
		Role:             aws.String("arn:aws:iam::123456789012:role/service-role/acme-lambda-execution"),
		MemorySize:       aws.Int32(256),
		Timeout:          aws.Int32(30),
		PackageType:      lambdatypes.PackageTypeZip,
		LastModified:     aws.String("2026-08-01T10:00:00.000+0000"),
		DeadLetterConfig: &lambdatypes.DeadLetterConfig{TargetArn: aws.String(t570DLQARN)},
		LoggingConfig: &lambdatypes.LoggingConfig{
			LogGroup:  aws.String(t570FnLogs),
			LogFormat: lambdatypes.LogFormatText,
		},
	}
}

func t570ClientsWithDLQFunction() *awsclient.ServiceClients {
	c := t570Demo()
	c.Lambda = &t570Lambda{LambdaAPI: c.Lambda, extra: []lambdatypes.FunctionConfiguration{t570DLQFunction()}}
	return c
}

// The lambda -> sqs contract: queues invoking the function or used as its
// DLQ (FunctionConfiguration.DeadLetterConfig.TargetArn).
func TestT570_LambdaSQS_CountsTheDeadLetterQueue(t *testing.T) {
	c := t570ClientsWithDLQFunction()
	fn := t570Row(t, c, "lambda", t570LambdaFn)
	t570Exact(t, t570Pivot(t, c, fn, "lambda", "sqs"), t570DLQQueue)
}

// A function whose DeadLetterConfig names an SNS topic lists that topic on
// its SNS row.
func TestT570_LambdaSNS_CountsTheDeadLetterTopic(t *testing.T) {
	c := t570Demo()
	fn := t570Row(t, c, "lambda", "rotate-docdb-credentials")
	if got := fn.Fields["dlq_target_arn"]; got != "arn:aws:sns:us-east-1:123456789012:ops-alerts" {
		t.Fatalf("fixture drifted: dlq_target_arn = %q", got)
	}
	t570Contains(t, t570Pivot(t, c, fn, "lambda", "sns"), []string{"arn:aws:sns:us-east-1:123456789012:ops-alerts"}, nil)
}

// The sqs -> lambda contract counts functions using the queue as their DLQ
// alongside event-source consumers.
func TestT570_SQSLambda_CountsFunctionsUsingTheQueueAsDLQ(t *testing.T) {
	c := t570ClientsWithDLQFunction()
	q := t570Row(t, c, "sqs", t570DLQQueue)
	t570Exact(t, t570Pivot(t, c, q, "sqs", "lambda"), t570LambdaFn)
}

// A function that sends its logs to a group of its own choosing
// (LoggingConfig.LogGroup) is that group's Lambda.
func TestT570_LogsLambda_CountsTheFunctionLoggingThere(t *testing.T) {
	c := t570ClientsWithDLQFunction()
	g := t570Row(t, c, "logs", t570FnLogs)
	t570Exact(t, t570Pivot(t, c, g, "logs", "lambda"), t570LambdaFn)
}

// The /aws/lambda/<name> default still names its function.
func TestT570_LogsLambda_DefaultGroupStillNamesItsFunction(t *testing.T) {
	c := t570Demo()
	g := t570Row(t, c, "logs", "/aws/lambda/rotate-docdb-credentials")
	t570Contains(t, t570Pivot(t, c, g, "logs", "lambda"), []string{"rotate-docdb-credentials"}, nil)
}

type t570CWLogs struct {
	awsclient.CWLogsAPI
	subscriptions map[string][]cwltypes.SubscriptionFilter
	exports       []cwltypes.ExportTask
}

func (f *t570CWLogs) DescribeSubscriptionFilters(ctx context.Context, in *cloudwatchlogs.DescribeSubscriptionFiltersInput, opt ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeSubscriptionFiltersOutput, error) {
	if subs, ok := f.subscriptions[aws.ToString(in.LogGroupName)]; ok {
		return &cloudwatchlogs.DescribeSubscriptionFiltersOutput{SubscriptionFilters: subs}, nil
	}
	return f.CWLogsAPI.(interface {
		DescribeSubscriptionFilters(context.Context, *cloudwatchlogs.DescribeSubscriptionFiltersInput, ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeSubscriptionFiltersOutput, error)
	}).DescribeSubscriptionFilters(ctx, in, opt...)
}

// DescribeExportTasks takes no log-group filter; every task names its group
// and destination bucket.
func (f *t570CWLogs) DescribeExportTasks(_ context.Context, _ *cloudwatchlogs.DescribeExportTasksInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeExportTasksOutput, error) {
	return &cloudwatchlogs.DescribeExportTasksOutput{ExportTasks: f.exports}, nil
}

// A subscription filter delivering the group's events to a function makes
// that function one of the group's Lambdas.
func TestT570_LogsLambda_CountsSubscriptionFilterConsumers(t *testing.T) {
	c := t570Demo()
	const group = "/app/legacy/orphan-old"
	c.CloudWatchLogs = &t570CWLogs{CWLogsAPI: c.CloudWatchLogs, subscriptions: map[string][]cwltypes.SubscriptionFilter{
		group: {{
			FilterName:     aws.String("errors-to-audit"),
			LogGroupName:   aws.String(group),
			FilterPattern:  aws.String("ERROR"),
			DestinationArn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:audit-logger"),
			Distribution:   cwltypes.DistributionByLogStream,
		}},
	}}
	g := t570Row(t, c, "logs", group)
	t570Exact(t, t570Pivot(t, c, g, "logs", "lambda"), "audit-logger")
}

type t570EKS struct {
	awsclient.EKSAPI
	cluster, nodegroup string
	remote             *ekstypes.RemoteAccessConfig
}

func (f *t570EKS) DescribeNodegroup(ctx context.Context, in *eks.DescribeNodegroupInput, opt ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
	out, err := f.EKSAPI.DescribeNodegroup(ctx, in, opt...)
	if err != nil || aws.ToString(in.ClusterName) != f.cluster || aws.ToString(in.NodegroupName) != f.nodegroup || out.Nodegroup == nil {
		return out, err
	}
	ng := *out.Nodegroup
	ng.RemoteAccess = f.remote
	ng.Resources = &ekstypes.NodegroupResources{AutoScalingGroups: ng.Resources.AutoScalingGroups}
	ng.LaunchTemplate = nil
	return &eks.DescribeNodegroupOutput{Nodegroup: &ng}, nil
}

// A node group whose SSH access is limited to a bastion's security group
// (RemoteAccess.SourceSecurityGroups) lists that group.
func TestT570_NGSG_CountsRemoteAccessSourceSecurityGroups(t *testing.T) {
	c := t570Demo()
	c.EKS = &t570EKS{EKSAPI: c.EKS, cluster: "acme-prod", nodegroup: "general-pool", remote: &ekstypes.RemoteAccessConfig{
		Ec2SshKey:            aws.String("ops-bastion"),
		SourceSecurityGroups: []string{"sg-0ddd444444444444d"},
	}}
	ng := t570Row(t, c, "ng", "acme-prod/general-pool")
	t570Exact(t, t570Pivot(t, c, ng, "ng", "sg"), "sg-0ddd444444444444d")
}

type t570Athena struct {
	awsclient.AthenaAPI
	workgroup string
	config    athenatypes.WorkGroupConfiguration
	calls     int
}

func (f *t570Athena) GetWorkGroup(ctx context.Context, in *athena.GetWorkGroupInput, opt ...func(*athena.Options)) (*athena.GetWorkGroupOutput, error) {
	f.calls++
	out, err := f.AthenaAPI.GetWorkGroup(ctx, in, opt...)
	if err != nil || aws.ToString(in.WorkGroup) != f.workgroup || out.WorkGroup == nil {
		return out, err
	}
	wg := *out.WorkGroup
	cfg := f.config
	wg.Configuration = &cfg
	return &athena.GetWorkGroupOutput{WorkGroup: &wg}, nil
}

func t570KMSKeys(t *testing.T, c *awsclient.ServiceClients, n int) (ids, arns []string) {
	t.Helper()
	for _, k := range t570List(t, c, "kms") {
		if k.Fields["arn"] == "" {
			continue
		}
		ids = append(ids, k.ID)
		arns = append(arns, k.Fields["arn"])
		if len(ids) == n {
			return ids, arns
		}
	}
	t.Fatalf("demo account holds fewer than %d KMS keys", n)
	return nil, nil
}

// A workgroup encrypts with up to three keys: the S3 query results
// (ResultConfiguration), Athena-managed results (ManagedQueryResults) and
// customer content such as notebooks (CustomerContentEncryptionConfiguration).
func TestT570_AthenaKMS_CountsEveryEncryptionKey(t *testing.T) {
	c := t570Demo()
	ids, arns := t570KMSKeys(t, c, 3)
	c.Athena = &t570Athena{AthenaAPI: c.Athena, workgroup: "primary", config: athenatypes.WorkGroupConfiguration{
		ResultConfiguration: &athenatypes.ResultConfiguration{
			OutputLocation: aws.String("s3://a9s-demo-healthy/athena/"),
			EncryptionConfiguration: &athenatypes.EncryptionConfiguration{
				EncryptionOption: athenatypes.EncryptionOptionSseKms,
				KmsKey:           aws.String(arns[0]),
			},
		},
		CustomerContentEncryptionConfiguration: &athenatypes.CustomerContentEncryptionConfiguration{KmsKey: aws.String(arns[1])},
		ManagedQueryResultsConfiguration: &athenatypes.ManagedQueryResultsConfiguration{
			Enabled:                 true,
			EncryptionConfiguration: &athenatypes.ManagedQueryResultsEncryptionConfiguration{KmsKey: aws.String(arns[2])},
		},
	}}
	wg := t570Row(t, c, "athena", "primary")
	t570Exact(t, t570Pivot(t, c, wg, "athena", "kms"), ids...)
}

type t570CodeBuild struct {
	awsclient.CodeBuildAPI
	project string
	edit    func(*cbtypes.Project)
}

func (f *t570CodeBuild) BatchGetProjects(ctx context.Context, in *codebuild.BatchGetProjectsInput, opt ...func(*codebuild.Options)) (*codebuild.BatchGetProjectsOutput, error) {
	out, err := f.CodeBuildAPI.BatchGetProjects(ctx, in, opt...)
	if err != nil {
		return out, err
	}
	for i := range out.Projects {
		if aws.ToString(out.Projects[i].Name) == f.project {
			f.edit(&out.Projects[i])
		}
	}
	return out, nil
}

func t570Buckets(t *testing.T, c *awsclient.ServiceClients, n int) []string {
	t.Helper()
	var ids []string
	for _, b := range t570List(t, c, "s3") {
		ids = append(ids, b.ID)
		if len(ids) == n {
			return ids
		}
	}
	t.Fatalf("demo account holds fewer than %d buckets", n)
	return nil
}

// A project's S3 buckets include its secondary sources
// (SecondarySources[].Location) and its build-log bucket
// (LogsConfig.S3Logs.Location), per docs/resources/cb.md.
func TestT570_CbS3_CountsSecondarySourcesAndS3Logs(t *testing.T) {
	c := t570Demo()
	b := t570Buckets(t, c, 2)
	c.CodeBuild = &t570CodeBuild{CodeBuildAPI: c.CodeBuild, project: "acme-api-build", edit: func(p *cbtypes.Project) {
		p.Artifacts = &cbtypes.ProjectArtifacts{Type: cbtypes.ArtifactsTypeNoArtifacts}
		p.SecondaryArtifacts = nil
		p.Source = &cbtypes.ProjectSource{Type: cbtypes.SourceTypeCodepipeline}
		p.SecondarySources = []cbtypes.ProjectSource{{Type: cbtypes.SourceTypeS3, Location: aws.String(b[0] + "/sources/app.zip"), SourceIdentifier: aws.String("assets")}}
		p.LogsConfig = &cbtypes.LogsConfig{
			CloudWatchLogs: &cbtypes.CloudWatchLogsConfig{Status: cbtypes.LogsConfigStatusTypeDisabled},
			S3Logs:         &cbtypes.S3LogsConfig{Status: cbtypes.LogsConfigStatusTypeEnabled, Location: aws.String(b[1] + "/build-logs")},
		}
	}}
	p := t570Row(t, c, "cb", "acme-api-build")
	t570Exact(t, t570Pivot(t, c, p, "cb", "s3"), b...)
}

type t570EventBridge struct {
	awsclient.EventBridgeAPI
	rule    string
	targets []eventbridgetypes.Target
}

func (f *t570EventBridge) ListTargetsByRule(ctx context.Context, in *eventbridge.ListTargetsByRuleInput, opt ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error) {
	if aws.ToString(in.Rule) == f.rule {
		return &eventbridge.ListTargetsByRuleOutput{Targets: f.targets}, nil
	}
	return f.EventBridgeAPI.ListTargetsByRule(ctx, in, opt...)
}

// A rule target's DeadLetterConfig.Arn is an SQS queue the rule sends to
// (docs/resources/eb-rule.md), even when the target itself is a function.
func TestT570_EbRuleSQS_CountsTargetDeadLetterQueues(t *testing.T) {
	c := t570Demo()
	c.EventBridge = &t570EventBridge{EventBridgeAPI: c.EventBridge, rule: "nightly-db-backup", targets: []eventbridgetypes.Target{{
		Id:               aws.String("audit"),
		Arn:              aws.String("arn:aws:lambda:us-east-1:123456789012:function:audit-logger"),
		DeadLetterConfig: &eventbridgetypes.DeadLetterConfig{Arn: aws.String(t570DLQARN)},
	}}}
	r := t570Row(t, c, "eb-rule", "default/nightly-db-backup")
	t570Exact(t, t570Pivot(t, c, r, "eb-rule", "sqs"), t570DLQQueue)
}

// A service's roles include its task definition's TaskRoleArn and
// ExecutionRoleArn (docs/resources/ecs-svc.md), not only Service.RoleArn.
func TestT570_ECSSvcRole_CountsTaskAndExecutionRoles(t *testing.T) {
	c := t570Demo()
	svc := t570Row(t, c, "ecs-svc", "acme-services/api-gateway")
	t570Contains(t, t570Pivot(t, c, svc, "ecs-svc", "role"), []string{"acme-lambda-execution", "acme-ci-deploy-role"}, nil)
}

type t570Glue struct {
	awsclient.GlueAPI
	job         string
	edit        func(*gluetypes.Job)
	connections map[string]gluetypes.Connection
}

func (f *t570Glue) GetJobs(ctx context.Context, in *glue.GetJobsInput, opt ...func(*glue.Options)) (*glue.GetJobsOutput, error) {
	out, err := f.GlueAPI.GetJobs(ctx, in, opt...)
	if err != nil {
		return out, err
	}
	for i := range out.Jobs {
		if aws.ToString(out.Jobs[i].Name) == f.job {
			f.edit(&out.Jobs[i])
		}
	}
	return out, nil
}

func (f *t570Glue) GetConnection(_ context.Context, in *glue.GetConnectionInput, _ ...func(*glue.Options)) (*glue.GetConnectionOutput, error) {
	if conn, ok := f.connections[aws.ToString(in.Name)]; ok {
		return &glue.GetConnectionOutput{Connection: &conn}, nil
	}
	return nil, &gluetypes.EntityNotFoundException{Message: aws.String("Connection not found")}
}

// A job that renames its continuous-logging group
// (DefaultArguments["--continuous-log-logGroup"]) logs there.
func TestT570_GlueLogs_CountsTheContinuousLoggingGroup(t *testing.T) {
	c := t570Demo()
	c.Glue = &t570Glue{GlueAPI: c.Glue, job: "acme-etl-orders", edit: func(j *gluetypes.Job) {
		args := map[string]string{}
		for k, v := range j.DefaultArguments {
			args[k] = v
		}
		args["--enable-continuous-cloudwatch-log"] = "true"
		args["--continuous-log-logGroup"] = t570FnLogs
		j.DefaultArguments = args
	}}
	job := t570Row(t, c, "glue", "acme-etl-orders")
	t570Contains(t, t570Pivot(t, c, job, "glue", "logs"), []string{t570FnLogs}, nil)

	for _, other := range t570List(t, c, "glue") {
		if other.ID == "acme-etl-orders" {
			continue
		}
		t570Contains(t, t570Pivot(t, c, other, "glue", "logs"), nil, []string{t570FnLogs})
	}
}

// A job's connections hold its database secret under the SECRET_ID
// connection property (docs/resources/glue.md).
func TestT570_GlueSecrets_CountsConnectionSecretID(t *testing.T) {
	c := t570Demo()
	secret := t570List(t, c, "secrets")[0]
	c.Glue = &t570Glue{GlueAPI: c.Glue, job: "acme-etl-orders",
		edit: func(j *gluetypes.Job) {
			j.Connections = &gluetypes.ConnectionsList{Connections: []string{"orders-jdbc"}}
		},
		connections: map[string]gluetypes.Connection{"orders-jdbc": {
			Name:           aws.String("orders-jdbc"),
			ConnectionType: gluetypes.ConnectionTypeJdbc,
			ConnectionProperties: map[string]string{
				"JDBC_CONNECTION_URL": "jdbc:postgresql://orders.cluster-example.us-east-1.rds.amazonaws.com:5432/orders",
				"SECRET_ID":           secret.Fields["arn"],
			},
		}},
	}
	job := t570Row(t, c, "glue", "acme-etl-orders")
	t570Contains(t, t570Pivot(t, c, job, "glue", "secrets"), []string{secret.ID}, nil)
}

// The ct-events -> role contract names the caller:
// userIdentity.sessionContext.sessionIssuer.arn of an AssumedRole identity.
// An AssumeRole call made from that session names the role it assumes in
// requestParameters.roleArn, which is not the caller.
func TestT570_CtEventsRole_IsTheSessionIssuer(t *testing.T) {
	c := t570Demo()
	event := cloudtrailtypes.Event{
		EventId:     aws.String("evt-t570-assume-role"),
		EventName:   aws.String("AssumeRole"),
		EventSource: aws.String("sts.amazonaws.com"),
		Username:    aws.String("deploy"),
		Resources: []cloudtrailtypes.Resource{{
			ResourceType: aws.String("AWS::IAM::Role"),
			ResourceName: aws.String("arn:aws:iam::123456789012:role/acme-eks-node-role"),
		}},
		CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","principalId":"AROAEXAMPLE333333333:deploy","arn":"arn:aws:sts::123456789012:assumed-role/acme-ci-deploy-role/deploy","accountId":"123456789012","sessionContext":{"sessionIssuer":{"type":"Role","principalId":"AROAEXAMPLE333333333","arn":"arn:aws:iam::123456789012:role/acme-ci-deploy-role","accountId":"123456789012","userName":"acme-ci-deploy-role"}}},"eventSource":"sts.amazonaws.com","eventName":"AssumeRole","awsRegion":"us-east-1","requestParameters":{"roleArn":"arn:aws:iam::123456789012:role/acme-eks-node-role","roleSessionName":"node-bootstrap"},"recipientAccountId":"123456789012"}`),
	}
	ev := resource.Resource{ID: "evt-t570-assume-role", Name: "AssumeRole", RawStruct: event, Fields: map[string]string{
		"event_name": "AssumeRole", "_ct.username": "deploy", "_ct.recipient_account": t570Account,
	}}
	cache := resource.ResourceCache{"role": resource.ResourceCacheEntry{Resources: t570List(t, c, "role")}}
	t570Exact(t, checkerByTarget(t, "ct-events", "role")(context.Background(), c, ev, cache), "acme-ci-deploy-role")
}

type t570EC2 struct {
	awsclient.EC2API
	images    func([]ec2types.Image) []ec2types.Image
	snapshots func([]ec2types.Snapshot) []ec2types.Snapshot
	flowLogs  []ec2types.FlowLog
	tgwVPC    []ec2types.TransitGatewayVpcAttachment
	tgwAtt    []ec2types.TransitGatewayAttachment
	sgs       func([]ec2types.SecurityGroup) []ec2types.SecurityGroup
	enis      func([]ec2types.NetworkInterface) []ec2types.NetworkInterface
	addresses func([]ec2types.Address) []ec2types.Address
}

func (f *t570EC2) DescribeImages(ctx context.Context, in *ec2.DescribeImagesInput, opt ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	out, err := f.EC2API.DescribeImages(ctx, in, opt...)
	if err == nil && f.images != nil {
		out.Images = f.images(out.Images)
	}
	return out, err
}

func (f *t570EC2) DescribeSnapshots(ctx context.Context, in *ec2.DescribeSnapshotsInput, opt ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	out, err := f.EC2API.DescribeSnapshots(ctx, in, opt...)
	if err == nil && f.snapshots != nil {
		out.Snapshots = f.snapshots(out.Snapshots)
	}
	return out, err
}

func (f *t570EC2) DescribeSecurityGroups(ctx context.Context, in *ec2.DescribeSecurityGroupsInput, opt ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	out, err := f.EC2API.DescribeSecurityGroups(ctx, in, opt...)
	if err == nil && f.sgs != nil {
		out.SecurityGroups = f.sgs(out.SecurityGroups)
	}
	return out, err
}

func (f *t570EC2) DescribeNetworkInterfaces(ctx context.Context, in *ec2.DescribeNetworkInterfacesInput, opt ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	out, err := f.EC2API.DescribeNetworkInterfaces(ctx, in, opt...)
	if err == nil && f.enis != nil {
		out.NetworkInterfaces = f.enis(out.NetworkInterfaces)
	}
	return out, err
}

func (f *t570EC2) DescribeAddresses(ctx context.Context, in *ec2.DescribeAddressesInput, opt ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	out, err := f.EC2API.DescribeAddresses(ctx, in, opt...)
	if err == nil && f.addresses != nil {
		out.Addresses = f.addresses(out.Addresses)
	}
	return out, err
}

// t570FilterValues is the value list of the named filter, and whether the
// request carried it.
func t570FilterValues(filters []ec2types.Filter, name string) ([]string, bool) {
	for _, f := range filters {
		if aws.ToString(f.Name) == name {
			return f.Values, true
		}
	}
	return nil, false
}

func t570In(v string, vals []string) bool {
	for _, x := range vals {
		if x == v {
			return true
		}
	}
	return false
}

// DescribeFlowLogs honours the resource-id filter as EC2 does: a flow log is
// returned when its ResourceId is one of the filter's values.
func (f *t570EC2) DescribeFlowLogs(ctx context.Context, in *ec2.DescribeFlowLogsInput, opt ...func(*ec2.Options)) (*ec2.DescribeFlowLogsOutput, error) {
	if f.flowLogs == nil {
		return f.EC2API.DescribeFlowLogs(ctx, in, opt...)
	}
	ids, filtered := t570FilterValues(in.Filter, "resource-id")
	var out []ec2types.FlowLog
	for _, fl := range f.flowLogs {
		if !filtered || t570In(aws.ToString(fl.ResourceId), ids) {
			out = append(out, fl)
		}
	}
	return &ec2.DescribeFlowLogsOutput{FlowLogs: out}, nil
}

// DescribeTransitGatewayVpcAttachments honours transit-gateway-id and state
// filters the way EC2 does.
func (f *t570EC2) DescribeTransitGatewayVpcAttachments(ctx context.Context, in *ec2.DescribeTransitGatewayVpcAttachmentsInput, opt ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayVpcAttachmentsOutput, error) {
	if f.tgwVPC == nil {
		return f.EC2API.DescribeTransitGatewayVpcAttachments(ctx, in, opt...)
	}
	tgws, byTGW := t570FilterValues(in.Filters, "transit-gateway-id")
	states, byState := t570FilterValues(in.Filters, "state")
	var out []ec2types.TransitGatewayVpcAttachment
	for _, a := range f.tgwVPC {
		if byTGW && !t570In(aws.ToString(a.TransitGatewayId), tgws) {
			continue
		}
		if byState && !t570In(string(a.State), states) {
			continue
		}
		if len(in.TransitGatewayAttachmentIds) > 0 && !t570In(aws.ToString(a.TransitGatewayAttachmentId), in.TransitGatewayAttachmentIds) {
			continue
		}
		out = append(out, a)
	}
	return &ec2.DescribeTransitGatewayVpcAttachmentsOutput{TransitGatewayVpcAttachments: out}, nil
}

// DescribeTransitGatewayAttachments honours resource-id, resource-type,
// transit-gateway-id and state filters the way EC2 does.
func (f *t570EC2) DescribeTransitGatewayAttachments(ctx context.Context, in *ec2.DescribeTransitGatewayAttachmentsInput, opt ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error) {
	if f.tgwAtt == nil {
		return f.EC2API.DescribeTransitGatewayAttachments(ctx, in, opt...)
	}
	var out []ec2types.TransitGatewayAttachment
	for _, a := range f.tgwAtt {
		keep := true
		for name, got := range map[string]string{
			"resource-id":        aws.ToString(a.ResourceId),
			"resource-type":      string(a.ResourceType),
			"transit-gateway-id": aws.ToString(a.TransitGatewayId),
			"state":              string(a.State),
		} {
			if vals, ok := t570FilterValues(in.Filters, name); ok && !t570In(got, vals) {
				keep = false
			}
		}
		if keep {
			out = append(out, a)
		}
	}
	return &ec2.DescribeTransitGatewayAttachmentsOutput{TransitGatewayAttachments: out}, nil
}

// An instance's snapshots include the ones its AMI was registered from
// (Image.BlockDeviceMappings[].Ebs.SnapshotId), per docs/resources/ec2.md.
func TestT570_EC2EBSSnap_CountsTheAMIRootSnapshot(t *testing.T) {
	c := t570Demo()
	c.EC2 = &t570EC2{EC2API: c.EC2, images: func(imgs []ec2types.Image) []ec2types.Image {
		for i := range imgs {
			if aws.ToString(imgs[i].ImageId) == "ami-0a1b2c3d4e5f60002" {
				imgs[i].RootDeviceName = aws.String("/dev/xvda")
				imgs[i].BlockDeviceMappings = []ec2types.BlockDeviceMapping{{
					DeviceName: aws.String("/dev/xvda"),
					Ebs: &ec2types.EbsBlockDevice{
						SnapshotId:          aws.String("snap-0orphan00000000c"),
						VolumeSize:          aws.Int32(20),
						VolumeType:          ec2types.VolumeTypeGp3,
						DeleteOnTermination: aws.Bool(true),
						Encrypted:           aws.Bool(true),
					},
				}}
			}
		}
		return imgs
	}}
	inst := t570Row(t, c, "ec2", "i-0a1b2c3d4e5f60003")
	t570Contains(t, t570Pivot(t, c, inst, "ec2", "ebs-snap"), []string{"snap-0orphan00000000c"}, nil)
}

type t570ELB struct {
	awsclient.ELBv2API
	health map[string][]string
}

func (f *t570ELB) DescribeTargetHealth(ctx context.Context, in *elbv2.DescribeTargetHealthInput, opt ...func(*elbv2.Options)) (*elbv2.DescribeTargetHealthOutput, error) {
	ids, ok := f.health[aws.ToString(in.TargetGroupArn)]
	if !ok {
		return f.ELBv2API.DescribeTargetHealth(ctx, in, opt...)
	}
	out := &elbv2.DescribeTargetHealthOutput{}
	for _, id := range ids {
		out.TargetHealthDescriptions = append(out.TargetHealthDescriptions, elbv2types.TargetHealthDescription{
			Target:       &elbv2types.TargetDescription{Id: aws.String(id), Port: aws.Int32(8080)},
			TargetHealth: &elbv2types.TargetHealth{State: elbv2types.TargetHealthStateEnumHealthy},
		})
	}
	return out, nil
}

// Target-group membership is DescribeTargetHealth's registered targets
// (docs/resources/ec2.md); sharing the target group's VPC is not membership.
func TestT570_EC2TG_IsRegistrationNotVPC(t *testing.T) {
	c := t570Demo()
	tg := t570Row(t, c, "tg", "acme-web-tg")
	c.ELBv2 = &t570ELB{ELBv2API: c.ELBv2, health: map[string][]string{
		tg.Fields["target_group_arn"]: {"i-0a1b2c3d4e5f60001", "i-0a1b2c3d4e5f60002"},
	}}
	registered := t570Row(t, c, "ec2", "i-0a1b2c3d4e5f60001")
	t570Exact(t, t570Pivot(t, c, registered, "ec2", "tg"), "acme-web-tg")

	sameVPC := t570Row(t, c, "ec2", "i-0a1b2c3d4e5f60007")
	t570Exact(t, t570Pivot(t, c, sameVPC, "ec2", "tg"))
}

// The sg -> sg contract is this group's own rules: the groups its
// IpPermissions and IpPermissionsEgress name in UserIdGroupPairs[].GroupId.
func TestT570_SGSG_ListsTheGroupsThisGroupsRulesName(t *testing.T) {
	c := t570Demo()
	src := t570Row(t, c, "sg", "sg-0ddd444444444444d")
	t570Exact(t, t570Pivot(t, c, src, "sg", "sg"), "sg-0bbb222222222222b")

	referenced := t570Row(t, c, "sg", "sg-0bbb222222222222b")
	own := referenced.RawStruct.(ec2types.SecurityGroup)
	var want []string
	for _, p := range append(own.IpPermissions, own.IpPermissionsEgress...) {
		for _, pair := range p.UserIdGroupPairs {
			if id := aws.ToString(pair.GroupId); id != referenced.ID {
				want = append(want, id)
			}
		}
	}
	t570Exact(t, t570Pivot(t, c, referenced, "sg", "sg"), want...)
}

type t570ACM struct {
	awsclient.ACMAPI
	cert  string
	inUse []string
}

func (f *t570ACM) DescribeCertificate(ctx context.Context, in *acm.DescribeCertificateInput, opt ...func(*acm.Options)) (*acm.DescribeCertificateOutput, error) {
	out, err := f.ACMAPI.DescribeCertificate(ctx, in, opt...)
	if err != nil || aws.ToString(in.CertificateArn) != f.cert || out.Certificate == nil {
		return out, err
	}
	cert := *out.Certificate
	cert.InUseBy = f.inUse
	return &acm.DescribeCertificateOutput{Certificate: &cert}, nil
}

// The elb list holds ELBv2 load balancers only, so a certificate served by a
// Classic Load Balancer (loadbalancer/<name>) is not one of its rows.
func TestT570_ACMELB_CountsOnlyELBv2LoadBalancers(t *testing.T) {
	c := t570Demo()
	cert := t570List(t, c, "acm")[0]
	alb := t570Row(t, c, "elb", "acme-prod-web")
	c.ACM = &t570ACM{ACMAPI: c.ACM, cert: cert.ID, inUse: []string{
		alb.Fields["load_balancer_arn"],
		"arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/legacy-web-clb",
	}}
	t570Exact(t, t570Pivot(t, c, cert, "acm", "elb"), alb.ID)
}
