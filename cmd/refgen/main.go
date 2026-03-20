package main

import (
	"fmt"
	"reflect"

	"github.com/k2m30/a9s/internal/fieldpath"

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

func main() {
	resources := []resourceDef{
		{"s3", "s3types.Bucket", reflect.TypeOf(s3types.Bucket{})},
		{"s3_objects", "s3types.Object", reflect.TypeOf(s3types.Object{})},
		{"ec2", "ec2types.Instance", reflect.TypeOf(ec2types.Instance{})},
		{"dbi", "rdstypes.DBInstance", reflect.TypeOf(rdstypes.DBInstance{})},
		{"redis", "elasticachetypes.CacheCluster", reflect.TypeOf(elasticachetypes.CacheCluster{})},
		{"dbc", "docdbtypes.DBCluster", reflect.TypeOf(docdbtypes.DBCluster{})},
		{"eks", "ekstypes.Cluster", reflect.TypeOf(ekstypes.Cluster{})},
		{"secrets", "smtypes.SecretListEntry", reflect.TypeOf(smtypes.SecretListEntry{})},
		{"vpc", "ec2types.Vpc", reflect.TypeOf(ec2types.Vpc{})},
		{"sg", "ec2types.SecurityGroup", reflect.TypeOf(ec2types.SecurityGroup{})},
		{"ng", "ekstypes.Nodegroup", reflect.TypeOf(ekstypes.Nodegroup{})},
		{"subnet", "ec2types.Subnet", reflect.TypeOf(ec2types.Subnet{})},
		{"rtb", "ec2types.RouteTable", reflect.TypeOf(ec2types.RouteTable{})},
		{"nat", "ec2types.NatGateway", reflect.TypeOf(ec2types.NatGateway{})},
		{"igw", "ec2types.InternetGateway", reflect.TypeOf(ec2types.InternetGateway{})},
		{"lambda", "lambdatypes.FunctionConfiguration", reflect.TypeOf(lambdatypes.FunctionConfiguration{})},
		{"alarm", "cwtypes.MetricAlarm", reflect.TypeOf(cwtypes.MetricAlarm{})},
		{"sns", "snstypes.Topic", reflect.TypeOf(snstypes.Topic{})},
		{"elb", "elbv2types.LoadBalancer", reflect.TypeOf(elbv2types.LoadBalancer{})},
		{"tg", "elbv2types.TargetGroup", reflect.TypeOf(elbv2types.TargetGroup{})},
		{"ecs", "ecstypes.Cluster", reflect.TypeOf(ecstypes.Cluster{})},
		{"ecs-svc", "ecstypes.Service", reflect.TypeOf(ecstypes.Service{})},
		{"cfn", "cfntypes.Stack", reflect.TypeOf(cfntypes.Stack{})},
		{"role", "iamtypes.Role", reflect.TypeOf(iamtypes.Role{})},
		{"logs", "cwlogstypes.LogGroup", reflect.TypeOf(cwlogstypes.LogGroup{})},
		{"ssm", "ssmtypes.ParameterMetadata", reflect.TypeOf(ssmtypes.ParameterMetadata{})},
		{"ddb", "ddbtypes.TableDescription", reflect.TypeOf(ddbtypes.TableDescription{})},
		{"eip", "ec2types.Address", reflect.TypeOf(ec2types.Address{})},
		{"acm", "acmtypes.CertificateSummary", reflect.TypeOf(acmtypes.CertificateSummary{})},
		{"asg", "asgtypes.AutoScalingGroup", reflect.TypeOf(asgtypes.AutoScalingGroup{})},
		{"ecs-task", "ecstypes.Task", reflect.TypeOf(ecstypes.Task{})},
		{"policy", "iamtypes.Policy", reflect.TypeOf(iamtypes.Policy{})},
		{"rds-snap", "rdstypes.DBSnapshot", reflect.TypeOf(rdstypes.DBSnapshot{})},
		{"tgw", "ec2types.TransitGateway", reflect.TypeOf(ec2types.TransitGateway{})},
		{"vpce", "ec2types.VpcEndpoint", reflect.TypeOf(ec2types.VpcEndpoint{})},
		{"eni", "ec2types.NetworkInterface", reflect.TypeOf(ec2types.NetworkInterface{})},
		{"sns-sub", "snstypes.Subscription", reflect.TypeOf(snstypes.Subscription{})},
		{"iam-user", "iamtypes.User", reflect.TypeOf(iamtypes.User{})},
		{"iam-group", "iamtypes.Group", reflect.TypeOf(iamtypes.Group{})},
		{"docdb-snap", "docdbtypes.DBClusterSnapshot", reflect.TypeOf(docdbtypes.DBClusterSnapshot{})},
		{"cf", "cftypes.DistributionSummary", reflect.TypeOf(cftypes.DistributionSummary{})},
		{"r53", "r53types.HostedZone", reflect.TypeOf(r53types.HostedZone{})},
		{"r53_records", "r53types.ResourceRecordSet", reflect.TypeOf(r53types.ResourceRecordSet{})},
		{"apigw", "apigwtypes.Api", reflect.TypeOf(apigwtypes.Api{})},
		{"ecr", "ecrtypes.Repository", reflect.TypeOf(ecrtypes.Repository{})},
		{"efs", "efstypes.FileSystemDescription", reflect.TypeOf(efstypes.FileSystemDescription{})},
		{"eb-rule", "eventbridgetypes.Rule", reflect.TypeOf(eventbridgetypes.Rule{})},
		{"sfn", "sfntypes.StateMachineListItem", reflect.TypeOf(sfntypes.StateMachineListItem{})},
		{"pipeline", "cptypes.PipelineSummary", reflect.TypeOf(cptypes.PipelineSummary{})},
		{"kinesis", "kinesistypes.StreamSummary", reflect.TypeOf(kinesistypes.StreamSummary{})},
		{"waf", "wafv2types.WebACLSummary", reflect.TypeOf(wafv2types.WebACLSummary{})},
		{"glue", "gluetypes.Job", reflect.TypeOf(gluetypes.Job{})},
		{"eb", "ebtypes.EnvironmentDescription", reflect.TypeOf(ebtypes.EnvironmentDescription{})},
		{"ses", "sesv2types.IdentityInfo", reflect.TypeOf(sesv2types.IdentityInfo{})},
		{"redshift", "redshifttypes.Cluster", reflect.TypeOf(redshifttypes.Cluster{})},
		{"trail", "cloudtrailtypes.Trail", reflect.TypeOf(cloudtrailtypes.Trail{})},
		{"athena", "athenatypes.WorkGroupSummary", reflect.TypeOf(athenatypes.WorkGroupSummary{})},
		{"codeartifact", "codeartifacttypes.RepositorySummary", reflect.TypeOf(codeartifacttypes.RepositorySummary{})},
		{"cb", "cbtypes.Project", reflect.TypeOf(cbtypes.Project{})},
		{"opensearch", "ostypes.DomainStatus", reflect.TypeOf(ostypes.DomainStatus{})},
		{"kms", "kmstypes.KeyMetadata", reflect.TypeOf(kmstypes.KeyMetadata{})},
		{"msk", "kafkatypes.Cluster", reflect.TypeOf(kafkatypes.Cluster{})},
		{"backup", "backuptypes.BackupPlansListMember", reflect.TypeOf(backuptypes.BackupPlansListMember{})},
	}

	fmt.Println("# views_reference.yaml")
	fmt.Println("# Generated from AWS SDK Go v2 struct reflection")
	fmt.Println("# Use these paths in your views.yaml configuration")
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
