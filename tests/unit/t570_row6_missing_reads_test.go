package unit_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	sfntypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

type t570EB struct {
	awsclient.ElasticBeanstalkAPI
	versions []ebtypes.ApplicationVersionDescription
	env      string
	options  []ebtypes.ConfigurationOptionSetting
}

// DescribeApplicationVersions honours VersionLabels as Elastic Beanstalk does.
func (f *t570EB) DescribeApplicationVersions(ctx context.Context, in *elasticbeanstalk.DescribeApplicationVersionsInput, opt ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeApplicationVersionsOutput, error) {
	if f.versions == nil {
		return f.ElasticBeanstalkAPI.DescribeApplicationVersions(ctx, in, opt...)
	}
	var out []ebtypes.ApplicationVersionDescription
	for _, v := range f.versions {
		if aws.ToString(v.ApplicationName) != aws.ToString(in.ApplicationName) {
			continue
		}
		if len(in.VersionLabels) > 0 && !t570In(aws.ToString(v.VersionLabel), in.VersionLabels) {
			continue
		}
		out = append(out, v)
	}
	return &elasticbeanstalk.DescribeApplicationVersionsOutput{ApplicationVersions: out}, nil
}

func (f *t570EB) DescribeConfigurationSettings(ctx context.Context, in *elasticbeanstalk.DescribeConfigurationSettingsInput, opt ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
	out, err := f.ElasticBeanstalkAPI.DescribeConfigurationSettings(ctx, in, opt...)
	if err != nil || f.options == nil || aws.ToString(in.EnvironmentName) != f.env {
		return out, err
	}
	cp := *out
	cp.ConfigurationSettings = append([]ebtypes.ConfigurationSettingsDescription(nil), out.ConfigurationSettings...)
	if len(cp.ConfigurationSettings) == 0 {
		cp.ConfigurationSettings = []ebtypes.ConfigurationSettingsDescription{{ApplicationName: in.ApplicationName, EnvironmentName: in.EnvironmentName}}
	}
	cp.ConfigurationSettings[0].OptionSettings = append(append([]ebtypes.ConfigurationOptionSetting(nil), cp.ConfigurationSettings[0].OptionSettings...), f.options...)
	return &cp, nil
}

// An environment's S3 source bundle is the one of the version it runs
// (EnvironmentDescription.VersionLabel); the application's other versions are
// not deployed there.
func TestT570_EbS3_CountsOnlyTheRunningVersionsBucket(t *testing.T) {
	c := t570Demo()
	envRow := t570Row(t, c, "eb", "e-acmeprodapi")
	env := envRow.RawStruct.(ebtypes.EnvironmentDescription)
	running := aws.ToString(env.VersionLabel)
	if running == "" {
		t.Fatal("fixture drifted: the environment names no running version")
	}
	b := t570Buckets(t, c, 2)
	app := env.ApplicationName
	c.ElasticBeanstalk = &t570EB{ElasticBeanstalkAPI: c.ElasticBeanstalk, versions: []ebtypes.ApplicationVersionDescription{
		{ApplicationName: app, VersionLabel: aws.String(running), Status: ebtypes.ApplicationVersionStatusProcessed,
			SourceBundle: &ebtypes.S3Location{S3Bucket: aws.String(b[0]), S3Key: aws.String("bundles/" + running + ".zip")}},
		{ApplicationName: app, VersionLabel: aws.String("v-previous"), Status: ebtypes.ApplicationVersionStatusProcessed,
			SourceBundle: &ebtypes.S3Location{S3Bucket: aws.String(b[1]), S3Key: aws.String("bundles/v-previous.zip")}},
	}}
	t570Exact(t, t570Pivot(t, c, envRow, "eb", "s3"), b[0])
}

// Elastic Beanstalk injects a secret as an environment variable through the
// aws:elasticbeanstalk:application:environmentsecrets namespace, whose
// option value is the secret's ARN.
func TestT570_SecretsEB_ReadsTheEnvironmentSecretsNamespace(t *testing.T) {
	c := t570Demo()
	envRow := t570Row(t, c, "eb", "e-acmeprodapi")
	env := envRow.RawStruct.(ebtypes.EnvironmentDescription)
	unreferenced := t570SecretsUnreferencedBy(t, c, env)
	used, unused := unreferenced[0], unreferenced[1]
	c.ElasticBeanstalk = &t570EB{ElasticBeanstalkAPI: c.ElasticBeanstalk, env: aws.ToString(env.EnvironmentName), options: []ebtypes.ConfigurationOptionSetting{{
		Namespace:  aws.String("aws:elasticbeanstalk:application:environmentsecrets"),
		OptionName: aws.String("DB_PASSWORD"),
		Value:      aws.String(used.Fields["arn"]),
	}}}
	t570Contains(t, t570Pivot(t, c, used, "secrets", "eb"), []string{envRow.ID}, nil)
	t570Contains(t, t570Pivot(t, c, unused, "secrets", "eb"), nil, []string{envRow.ID})
}

// t570SecretsUnreferencedBy lists the demo secrets whose ARN and name appear
// in none of the environment's configuration option values.
func t570SecretsUnreferencedBy(t *testing.T, c *awsclient.ServiceClients, env ebtypes.EnvironmentDescription) []resource.Resource {
	t.Helper()
	cfg, err := c.ElasticBeanstalk.DescribeConfigurationSettings(context.Background(), &elasticbeanstalk.DescribeConfigurationSettingsInput{
		ApplicationName: env.ApplicationName, EnvironmentName: env.EnvironmentName,
	})
	if err != nil {
		t.Fatalf("DescribeConfigurationSettings: %v", err)
	}
	var values []string
	for _, cs := range cfg.ConfigurationSettings {
		for _, o := range cs.OptionSettings {
			values = append(values, aws.ToString(o.Value))
		}
	}
	var out []resource.Resource
	for _, s := range t570List(t, c, "secrets") {
		referenced := false
		for _, v := range values {
			if strings.Contains(v, s.Fields["arn"]) || strings.Contains(v, s.ID) {
				referenced = true
			}
		}
		if !referenced {
			out = append(out, s)
		}
	}
	if len(out) < 2 {
		t.Fatalf("demo account holds %d secrets no environment references, want 2", len(out))
	}
	return out
}

type t570EC2Tags struct {
	awsclient.EC2API
	instance string
	tags     []ec2types.Tag
}

func (f *t570EC2Tags) DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput, opt ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	out, err := f.EC2API.DescribeInstances(ctx, in, opt...)
	if err != nil {
		return out, err
	}
	for i := range out.Reservations {
		for j := range out.Reservations[i].Instances {
			if aws.ToString(out.Reservations[i].Instances[j].InstanceId) == f.instance {
				out.Reservations[i].Instances[j].Tags = append(append([]ec2types.Tag(nil), out.Reservations[i].Instances[j].Tags...), f.tags...)
			}
		}
	}
	return out, nil
}

const t570DailyPlan = "11111111-1111-1111-1111-111111111111"

var t570DailyTag = map[string]string{"backup": "daily"}

// A plan that selects an EC2 instance, by ARN or by its tags, backs up the
// instance's attached volumes with it.
func TestT570_EBSBackup_ReadsTheAttachedInstance(t *testing.T) {
	c := t570Demo()
	c.EC2 = &t570EC2Tags{EC2API: c.EC2, instance: "i-0a1b2c3d4e5f60003", tags: []ec2types.Tag{{Key: aws.String("backup"), Value: aws.String("daily")}}}
	vol := t570Row(t, c, "ebs", "vol-0a1b2c3d4e5f60002")
	t570Contains(t, t570Pivot(t, c, vol, "ebs", "backup"), []string{t570DailyPlan}, nil)
}

type t570DDB struct {
	awsclient.DynamoDBAPI
	tags         map[string]map[string]string
	destinations map[string][]ddbtypes.KinesisDataStreamDestination
}

func (f *t570DDB) ListTagsOfResource(ctx context.Context, in *dynamodb.ListTagsOfResourceInput, opt ...func(*dynamodb.Options)) (*dynamodb.ListTagsOfResourceOutput, error) {
	if tags, ok := f.tags[aws.ToString(in.ResourceArn)]; ok {
		out := &dynamodb.ListTagsOfResourceOutput{Tags: []ddbtypes.Tag{}}
		for k, v := range tags {
			out.Tags = append(out.Tags, ddbtypes.Tag{Key: aws.String(k), Value: aws.String(v)})
		}
		return out, nil
	}
	return f.DynamoDBAPI.(interface {
		ListTagsOfResource(context.Context, *dynamodb.ListTagsOfResourceInput, ...func(*dynamodb.Options)) (*dynamodb.ListTagsOfResourceOutput, error)
	}).ListTagsOfResource(ctx, in, opt...)
}

func (f *t570DDB) DescribeKinesisStreamingDestination(ctx context.Context, in *dynamodb.DescribeKinesisStreamingDestinationInput, opt ...func(*dynamodb.Options)) (*dynamodb.DescribeKinesisStreamingDestinationOutput, error) {
	if dests, ok := f.destinations[aws.ToString(in.TableName)]; ok {
		return &dynamodb.DescribeKinesisStreamingDestinationOutput{TableName: in.TableName, KinesisDataStreamDestinations: dests}, nil
	}
	if f.destinations != nil {
		return &dynamodb.DescribeKinesisStreamingDestinationOutput{TableName: in.TableName}, nil
	}
	return f.DynamoDBAPI.DescribeKinesisStreamingDestination(ctx, in, opt...)
}

// A plan selecting by tag (ListOfTags aws:ResourceTag/backup = daily) covers
// a table carrying that tag; the table's tags are readable, so a table
// without it is an exact answer, not an unknown.
func TestT570_DdbBackup_ReadsTheTablesTags(t *testing.T) {
	c := t570Demo()
	tagged := t570Row(t, c, "ddb", "audit-pitr-off")
	untagged := t570Row(t, c, "ddb", "legacy-archived")
	c.DynamoDB = &t570DDB{DynamoDBAPI: c.DynamoDB, tags: map[string]map[string]string{
		tagged.Fields["arn"]:   t570DailyTag,
		untagged.Fields["arn"]: {"team": "data"},
	}}
	t570Contains(t, t570Pivot(t, c, tagged, "ddb", "backup"), []string{t570DailyPlan}, nil)
	t570Contains(t, t570Pivot(t, c, untagged, "ddb", "backup"), nil, []string{t570DailyPlan})
}

type t570S3Tags struct {
	awsclient.S3API
	tags map[string]map[string]string
}

func (f *t570S3Tags) GetBucketTagging(ctx context.Context, in *s3.GetBucketTaggingInput, opt ...func(*s3.Options)) (*s3.GetBucketTaggingOutput, error) {
	if tags, ok := f.tags[aws.ToString(in.Bucket)]; ok {
		out := &s3.GetBucketTaggingOutput{}
		for k, v := range tags {
			out.TagSet = append(out.TagSet, s3types.Tag{Key: aws.String(k), Value: aws.String(v)})
		}
		return out, nil
	}
	return f.S3API.(interface {
		GetBucketTagging(context.Context, *s3.GetBucketTaggingInput, ...func(*s3.Options)) (*s3.GetBucketTaggingOutput, error)
	}).GetBucketTagging(ctx, in, opt...)
}

func t570SessionRegionBuckets(t *testing.T, c *awsclient.ServiceClients, n int) []resource.Resource {
	t.Helper()
	var out []resource.Resource
	for _, b := range t570List(t, c, "s3") {
		raw := b.RawStruct.(s3types.Bucket)
		if aws.ToString(raw.BucketRegion) == "us-east-1" && b.ID != "a9s-demo-healthy" {
			out = append(out, b)
			if len(out) == n {
				return out
			}
		}
	}
	t.Fatalf("demo account holds fewer than %d us-east-1 buckets", n)
	return nil
}

// As for tables: a bucket's tags (GetBucketTagging) decide a tag-selecting
// plan.
func TestT570_S3Backup_ReadsTheBucketsTags(t *testing.T) {
	c := t570Demo()
	b := t570SessionRegionBuckets(t, c, 2)
	c.S3 = &t570S3Tags{S3API: c.S3, tags: map[string]map[string]string{b[0].ID: t570DailyTag, b[1].ID: {"team": "web"}}}
	t570Contains(t, t570Pivot(t, c, b[0], "s3", "backup"), []string{t570DailyPlan}, nil)
	t570Contains(t, t570Pivot(t, c, b[1], "s3", "backup"), nil, []string{t570DailyPlan})
}

type t570RDSTags struct {
	awsclient.RDSAPI
	instance string
}

func (f *t570RDSTags) DescribeDBInstances(ctx context.Context, in *rds.DescribeDBInstancesInput, opt ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error) {
	out, err := f.RDSAPI.DescribeDBInstances(ctx, in, opt...)
	if err != nil {
		return out, err
	}
	for i := range out.DBInstances {
		if aws.ToString(out.DBInstances[i].DBInstanceIdentifier) == f.instance {
			out.DBInstances[i].TagList = append(append([]rdstypes.Tag(nil), out.DBInstances[i].TagList...), rdstypes.Tag{Key: aws.String("backup"), Value: aws.String("daily")})
		}
	}
	return out, nil
}

func (f *t570RDSTags) ListTagsForResource(ctx context.Context, in *rds.ListTagsForResourceInput, opt ...func(*rds.Options)) (*rds.ListTagsForResourceOutput, error) {
	out, err := f.RDSAPI.(interface {
		ListTagsForResource(context.Context, *rds.ListTagsForResourceInput, ...func(*rds.Options)) (*rds.ListTagsForResourceOutput, error)
	}).ListTagsForResource(ctx, in, opt...)
	if err != nil || aws.ToString(in.ResourceName) != "arn:aws:rds:us-east-1:123456789012:db:"+f.instance {
		return out, err
	}
	cp := *out
	cp.TagList = append(append([]rdstypes.Tag(nil), out.TagList...), rdstypes.Tag{Key: aws.String("backup"), Value: aws.String("daily")})
	return &cp, nil
}

// AWS Backup protects a DB snapshot's parent instance; a plan selecting the
// parent by tag covers the snapshot's instance.
func TestT570_DBISnapBackup_ReadsTheParentsTags(t *testing.T) {
	c := t570Demo()
	c.RDS = &t570RDSTags{RDSAPI: c.RDS, instance: "prod-dbi-1"}
	snap := t570Row(t, c, "dbi-snap", "prod-dbi-1-manual-2026-02-11")
	t570Contains(t, t570Pivot(t, c, snap, "dbi-snap", "backup"), []string{t570DailyPlan}, nil)
}

type t570DocDBTags struct {
	awsclient.DocDBAPI
	cluster string
}

func (f *t570DocDBTags) ListTagsForResource(ctx context.Context, in *docdb.ListTagsForResourceInput, opt ...func(*docdb.Options)) (*docdb.ListTagsForResourceOutput, error) {
	out, err := f.DocDBAPI.(interface {
		ListTagsForResource(context.Context, *docdb.ListTagsForResourceInput, ...func(*docdb.Options)) (*docdb.ListTagsForResourceOutput, error)
	}).ListTagsForResource(ctx, in, opt...)
	if err != nil || aws.ToString(in.ResourceName) != "arn:aws:rds:us-east-1:123456789012:cluster:"+f.cluster {
		return out, err
	}
	cp := *out
	cp.TagList = append(append([]docdbtypes.Tag(nil), out.TagList...), docdbtypes.Tag{Key: aws.String("backup"), Value: aws.String("daily")})
	return &cp, nil
}

// As for DB snapshots: a cluster snapshot's parent cluster's tags decide a
// tag-selecting plan.
func TestT570_DbcSnapBackup_ReadsTheParentsTags(t *testing.T) {
	c := t570Demo()
	c.DocDB = &t570DocDBTags{DocDBAPI: c.DocDB, cluster: "acme-docdb-prod"}
	snap := t570Row(t, c, "dbc-snap", "rds:acme-docdb-prod-2026-03-20")
	t570Contains(t, t570Pivot(t, c, snap, "dbc-snap", "backup"), []string{t570DailyPlan}, nil)
}

type t570CodePipeline struct {
	awsclient.CodePipelineAPI
	pipeline string
	stages   []cptypes.StageDeclaration
}

func (f *t570CodePipeline) GetPipeline(ctx context.Context, in *codepipeline.GetPipelineInput, opt ...func(*codepipeline.Options)) (*codepipeline.GetPipelineOutput, error) {
	out, err := f.CodePipelineAPI.GetPipeline(ctx, in, opt...)
	if err != nil || aws.ToString(in.Name) != f.pipeline || out.Pipeline == nil {
		return out, err
	}
	p := *out.Pipeline
	p.Stages = f.stages
	return &codepipeline.GetPipelineOutput{Pipeline: &p, Metadata: out.Metadata}, nil
}

// A blue/green ECS deploy action (provider CodeDeployToECS) names a CodeDeploy
// application and deployment group, not the ECS service; the service is in
// the deployment group (codedeploy:GetDeploymentGroup ecsServices). The
// pivot either reads it there or says it did not, never a proven 0.
func TestT570_PipelineECSSvc_CodeDeployToECSIsNotAProvenZero(t *testing.T) {
	c := t570Demo()
	c.CodePipeline = &t570CodePipeline{CodePipelineAPI: c.CodePipeline, pipeline: "acme-api-deploy", stages: []cptypes.StageDeclaration{
		{Name: aws.String("Source"), Actions: []cptypes.ActionDeclaration{{
			Name:            aws.String("Image"),
			ActionTypeId:    &cptypes.ActionTypeId{Category: cptypes.ActionCategorySource, Owner: cptypes.ActionOwnerAws, Provider: aws.String("ECR"), Version: aws.String("1")},
			Configuration:   map[string]string{"RepositoryName": "acme/api-service", "ImageTag": "latest"},
			OutputArtifacts: []cptypes.OutputArtifact{{Name: aws.String("MyImage")}},
		}}},
		{Name: aws.String("Deploy"), Actions: []cptypes.ActionDeclaration{{
			Name:         aws.String("BlueGreen"),
			ActionTypeId: &cptypes.ActionTypeId{Category: cptypes.ActionCategoryDeploy, Owner: cptypes.ActionOwnerAws, Provider: aws.String("CodeDeployToECS"), Version: aws.String("1")},
			Configuration: map[string]string{
				"ApplicationName":                "AppECS-acme-services-api-gateway",
				"DeploymentGroupName":            "DgpECS-acme-services-api-gateway",
				"TaskDefinitionTemplateArtifact": "SourceArtifact",
				"AppSpecTemplateArtifact":        "SourceArtifact",
				"Image1ArtifactName":             "MyImage",
				"Image1ContainerName":            "IMAGE1_NAME",
			},
			InputArtifacts: []cptypes.InputArtifact{{Name: aws.String("SourceArtifact")}, {Name: aws.String("MyImage")}},
		}}},
	}}
	p := t570Row(t, c, "pipeline", "acme-api-deploy")
	t570NotProvenZero(t, t570Pivot(t, c, p, "pipeline", "ecs-svc"))
}

type t570Kinesis struct {
	awsclient.KinesisAPI
	consumers map[string][]kinesistypes.Consumer
}

func (f *t570Kinesis) ListStreamConsumers(_ context.Context, in *kinesis.ListStreamConsumersInput, _ ...func(*kinesis.Options)) (*kinesis.ListStreamConsumersOutput, error) {
	return &kinesis.ListStreamConsumersOutput{Consumers: f.consumers[aws.ToString(in.StreamARN)]}, nil
}

type t570LambdaESM struct {
	awsclient.LambdaAPI
	bySource map[string][]lambdatypes.EventSourceMappingConfiguration
}

func (f *t570LambdaESM) ListEventSourceMappings(ctx context.Context, in *lambda.ListEventSourceMappingsInput, opt ...func(*lambda.Options)) (*lambda.ListEventSourceMappingsOutput, error) {
	if m, ok := f.bySource[aws.ToString(in.EventSourceArn)]; ok {
		return &lambda.ListEventSourceMappingsOutput{EventSourceMappings: m}, nil
	}
	return f.LambdaAPI.ListEventSourceMappings(ctx, in, opt...)
}

// A function reading the stream through an enhanced fan-out consumer is
// mapped to the consumer's ARN (stream/<name>/consumer/<consumer>:<ts>), not
// the stream's; the stream's Lambdas include it.
func TestT570_KinesisLambda_CountsEnhancedFanOutConsumers(t *testing.T) {
	c := t570Demo()
	stream := t570Row(t, c, "kinesis", "clickstream-ingest")
	streamARN := stream.Fields["stream_arn"]
	consumerARN := streamARN + "/consumer/audit-efo:1767225600"
	c.Kinesis = &t570Kinesis{KinesisAPI: c.Kinesis, consumers: map[string][]kinesistypes.Consumer{streamARN: {{
		ConsumerName:              aws.String("audit-efo"),
		ConsumerARN:               aws.String(consumerARN),
		ConsumerStatus:            kinesistypes.ConsumerStatusActive,
		ConsumerCreationTimestamp: aws.Time(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
	}}}}
	c.Lambda = &t570LambdaESM{LambdaAPI: c.Lambda, bySource: map[string][]lambdatypes.EventSourceMappingConfiguration{consumerARN: {{
		UUID:             aws.String("0e570000-0000-4000-8000-00000000efo1"),
		EventSourceArn:   aws.String(consumerARN),
		FunctionArn:      aws.String("arn:aws:lambda:us-east-1:123456789012:function:audit-logger"),
		State:            aws.String("Enabled"),
		BatchSize:        aws.Int32(100),
		StartingPosition: lambdatypes.EventSourcePositionLatest,
	}}}}
	t570Contains(t, t570Pivot(t, c, stream, "kinesis", "lambda"), []string{"audit-logger"}, nil)
}

// A DynamoDB streaming destination whose DestinationStatus is DISABLED sends
// the table's changes nowhere, so neither side counts it.
func TestT570_KinesisDdbStreaming_CountsOnlyActiveDestinations(t *testing.T) {
	c := t570Demo()
	stream := t570Row(t, c, "kinesis", "clickstream-ingest")
	streamARN := stream.Fields["stream_arn"]
	const active, disabled = "audit-pitr-off", "legacy-archived"
	c.DynamoDB = &t570DDB{DynamoDBAPI: c.DynamoDB, destinations: map[string][]ddbtypes.KinesisDataStreamDestination{
		active:   {{StreamArn: aws.String(streamARN), DestinationStatus: ddbtypes.DestinationStatusActive}},
		disabled: {{StreamArn: aws.String(streamARN), DestinationStatus: ddbtypes.DestinationStatusDisabled, DestinationStatusDescription: aws.String("Disabled by the table owner")}},
	}}
	cache := resource.ResourceCache{"ddb": resource.ResourceCacheEntry{Resources: t570List(t, c, "ddb")}}
	t570Exact(t, checkerByTarget(t, "kinesis", "ddb")(context.Background(), c, stream, cache), active)
	t570Exact(t, t570Pivot(t, c, t570Row(t, c, "ddb", active), "ddb", "kinesis"), stream.ID)
	t570Exact(t, t570Pivot(t, c, t570Row(t, c, "ddb", disabled), "ddb", "kinesis"))
}

type t570SFN struct {
	awsclient.SFNAPI
	machine string
	logging *sfntypes.LoggingConfiguration
}

func (f *t570SFN) DescribeStateMachine(ctx context.Context, in *sfn.DescribeStateMachineInput, opt ...func(*sfn.Options)) (*sfn.DescribeStateMachineOutput, error) {
	out, err := f.SFNAPI.DescribeStateMachine(ctx, in, opt...)
	if err != nil || aws.ToString(out.Name) != f.machine {
		return out, err
	}
	cp := *out
	cp.LoggingConfiguration = f.logging
	return &cp, nil
}

// A state machine logs to the group its LoggingConfiguration.Destinations
// names, whatever that group is called.
func TestT570_SFNLogs_ReadsLoggingDestinations(t *testing.T) {
	c := t570Demo()
	c.SFN = &t570SFN{SFNAPI: c.SFN, machine: "order-fulfillment-workflow", logging: &sfntypes.LoggingConfiguration{
		Level:                sfntypes.LogLevelError,
		IncludeExecutionData: false,
		Destinations: []sfntypes.LogDestination{{CloudWatchLogsLogGroup: &sfntypes.CloudWatchLogsLogGroup{
			LogGroupArn: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + t570FnLogs + ":*"),
		}}},
	}}
	m := t570Row(t, c, "sfn", "order-fulfillment-workflow")
	t570Exact(t, t570Pivot(t, c, m, "sfn", "logs"), t570FnLogs)
}

// A CloudTrail record of an ECR repository names it in Resources[] as
// AWS::ECR::Repository, by ARN or by bare name; both reach the repository.
func TestT570_CtEventsECR_ReachesTheRepository(t *testing.T) {
	c := t570Demo()
	repo := t570Row(t, c, "ecr", "acme/api-service")
	cache := resource.ResourceCache{"ecr": resource.ResourceCacheEntry{Resources: t570List(t, c, "ecr")}}
	for _, name := range []string{repo.Fields["arn"], "acme/api-service"} {
		ev := resource.Resource{ID: "evt-t570-ecr", Name: "PutImage", Fields: map[string]string{
			"event_name": "PutImage", "_ct.username": "ci-deploy", "_ct.recipient_account": t570Account,
		}, RawStruct: cloudtrailtypes.Event{
			EventId:     aws.String("evt-t570-ecr"),
			EventName:   aws.String("PutImage"),
			EventSource: aws.String("ecr.amazonaws.com"),
			Resources:   []cloudtrailtypes.Resource{{ResourceType: aws.String("AWS::ECR::Repository"), ResourceName: aws.String(name)}},
			CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","userName":"ci-deploy","accountId":"123456789012"},` +
				`"eventSource":"ecr.amazonaws.com","eventName":"PutImage","awsRegion":"us-east-1","requestParameters":{"repositoryName":"acme/api-service","imageTag":"latest"},"recipientAccountId":"123456789012"}`),
		}}
		t570Exact(t, checkerByTarget(t, "ct-events", "ecr")(context.Background(), c, ev, cache), repo.ID)
	}
}
