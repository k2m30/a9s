package main

import (
	"fmt"
	"reflect"

	"github.com/k2m30/a9s/internal/fieldpath"

	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
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
