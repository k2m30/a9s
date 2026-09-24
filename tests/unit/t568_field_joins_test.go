package unit_test

// A relationship AWS records in a field is read from that field. A pivot that
// joins by a shared subnet, a tag nobody set, a naming convention or a name
// that also names something else counts resources the relationship does not
// hold; where no field records it, the answer is candidates, never a count.

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigatewayv2/types"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ─── apigw → elb: the integration's IntegrationUri ────────────────────────

func t568LB(name, kind, id string, subnets ...string) resource.Resource {
	arn := "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/" + kind + "/" + name + "/" + id
	var azs []elbv2types.AvailabilityZone
	for i, s := range subnets {
		azs = append(azs, elbv2types.AvailabilityZone{SubnetId: aws.String(s), ZoneName: aws.String([]string{"us-east-1a", "us-east-1b"}[i%2])})
	}
	return resource.Resource{ID: name, Name: name, Type: "elb",
		Fields: map[string]string{"load_balancer_arn": arn, "name": name, "scheme": "internal"},
		RawStruct: elbv2types.LoadBalancer{LoadBalancerArn: aws.String(arn), LoadBalancerName: aws.String(name),
			Scheme: elbv2types.LoadBalancerSchemeEnumInternal, AvailabilityZones: azs, VpcId: aws.String("vpc-0a1b2c3d4e5f60001"),
			SecurityGroups: []string{"sg-0aaa111111111111a"}},
	}
}

// A private integration names the listener it sends to in IntegrationUri.
// Another load balancer in the VPC link's subnets and security group is not
// behind the API.
// https://docs.aws.amazon.com/apigatewayv2/latest/api-reference/apis-apiid-integrations.html
func TestFieldJoin_APIGWPrivateIntegrationNamesItsListener(t *testing.T) {
	const listener = "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-int-a/50dc6c495c0c9188/f2f7dc8efc522ab2"
	fake := &t568APIGW{
		integrations: map[string][]apigwtypes.Integration{"a1b2c3d4e5": {{
			IntegrationId: aws.String("int001"), IntegrationType: apigwtypes.IntegrationTypeHttpProxy,
			ConnectionType: apigwtypes.ConnectionTypeVpcLink, ConnectionId: aws.String("vl0a1b2c"),
			IntegrationMethod: aws.String("ANY"), IntegrationUri: aws.String(listener), PayloadFormatVersion: aws.String("1.0"),
		}}},
		links: []apigwtypes.VpcLink{{VpcLinkId: aws.String("vl0a1b2c"), Name: aws.String("acme-internal"),
			SubnetIds: []string{"subnet-0a1b2c3d4e5f60001", "subnet-0a1b2c3d4e5f60002"}, SecurityGroupIds: []string{"sg-0aaa111111111111a"},
			VpcLinkStatus: apigwtypes.VpcLinkStatusAvailable}},
	}
	lbs := resource.ResourceCache{"elb": {Resources: []resource.Resource{
		t568LB("acme-int-a", "app", "50dc6c495c0c9188", "subnet-0a1b2c3d4e5f60001", "subnet-0a1b2c3d4e5f60002"),
		t568LB("acme-int-b", "app", "7a8b9c0d1e2f3a4b", "subnet-0a1b2c3d4e5f60001", "subnet-0a1b2c3d4e5f60002"),
	}}}
	got := refChecker(t, "apigw", "elb")(context.Background(), &awsclient.ServiceClients{APIGatewayV2: fake, Region: "us-east-1"}, t568HTTPAPI("a1b2c3d4e5", "acme-internal-api"), lbs)
	t568RequireExact(t, "apigw → elb", got, "acme-int-a")
}

// ─── ecs → asg, ecs → ec2 ─────────────────────────────────────────────────

// t568ECS answers the ECS reads the cluster pivots make, as ECS does.
type t568ECS struct {
	awsclient.ECSAPI
	providers  map[string]ecstypes.CapacityProvider
	instances  map[string][]ecstypes.ContainerInstance // cluster ARN → registered container instances
	taskDefs   map[string]ecstypes.TaskDefinition
	clusterARN map[string]string // cluster name → ARN
}

func (f *t568ECS) DescribeCapacityProviders(_ context.Context, in *ecs.DescribeCapacityProvidersInput, _ ...func(*ecs.Options)) (*ecs.DescribeCapacityProvidersOutput, error) {
	out := &ecs.DescribeCapacityProvidersOutput{}
	for _, name := range in.CapacityProviders {
		if p, ok := f.providers[name]; ok {
			out.CapacityProviders = append(out.CapacityProviders, p)
		} else {
			out.Failures = append(out.Failures, ecstypes.Failure{Arn: aws.String(name), Reason: aws.String("MISSING")})
		}
	}
	return out, nil
}

func (f *t568ECS) cluster(ref string) string {
	if arn, ok := f.clusterARN[ref]; ok {
		return arn
	}
	return ref
}

func (f *t568ECS) ListContainerInstances(_ context.Context, in *ecs.ListContainerInstancesInput, _ ...func(*ecs.Options)) (*ecs.ListContainerInstancesOutput, error) {
	out := &ecs.ListContainerInstancesOutput{ContainerInstanceArns: []string{}}
	for _, ci := range f.instances[f.cluster(aws.ToString(in.Cluster))] {
		out.ContainerInstanceArns = append(out.ContainerInstanceArns, aws.ToString(ci.ContainerInstanceArn))
	}
	return out, nil
}

func (f *t568ECS) DescribeContainerInstances(_ context.Context, in *ecs.DescribeContainerInstancesInput, _ ...func(*ecs.Options)) (*ecs.DescribeContainerInstancesOutput, error) {
	out := &ecs.DescribeContainerInstancesOutput{}
	for _, ci := range f.instances[f.cluster(aws.ToString(in.Cluster))] {
		if slices.Contains(in.ContainerInstances, aws.ToString(ci.ContainerInstanceArn)) {
			out.ContainerInstances = append(out.ContainerInstances, ci)
		}
	}
	return out, nil
}

func (f *t568ECS) DescribeTaskDefinition(_ context.Context, in *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	td, ok := f.taskDefs[aws.ToString(in.TaskDefinition)]
	if !ok {
		return nil, &smithy.GenericAPIError{Code: "ClientException", Message: "Unable to describe task definition."}
	}
	return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &td}, nil
}

func t568ClusterRow(name string, providers ...string) resource.Resource {
	arn := "arn:aws:ecs:us-east-1:123456789012:cluster/" + name
	return resource.Resource{ID: name, Name: name, Type: "ecs", Fields: map[string]string{"cluster_name": name, "arn": arn, "status": "ACTIVE"},
		RawStruct: ecstypes.Cluster{ClusterName: aws.String(name), ClusterArn: aws.String(arn), Status: aws.String("ACTIVE"), CapacityProviders: providers}}
}

func t568ASG(name string) resource.Resource {
	arn := "arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:6d9b1c3a-2f4e-4a5b-8c7d-0e1f2a3b4c5d:autoScalingGroupName/" + name
	return resource.Resource{ID: name, Name: name, Type: "asg", Fields: map[string]string{"name": name, "arn": arn},
		RawStruct: asgtypes.AutoScalingGroup{AutoScalingGroupName: aws.String(name), AutoScalingGroupARN: aws.String(arn),
			MinSize: aws.Int32(1), MaxSize: aws.Int32(4), DesiredCapacity: aws.Int32(2),
			Tags: []asgtypes.TagDescription{{Key: aws.String("AmazonECSManaged"), Value: aws.String(""), PropagateAtLaunch: aws.Bool(true),
				ResourceId: aws.String(name), ResourceType: aws.String("auto-scaling-group")}}}}
}

// A cluster's Auto Scaling groups are the ones its capacity providers name
// (DescribeCapacityProviders → autoScalingGroupProvider.autoScalingGroupArn).
// The AmazonECSManaged tag is on every ECS-managed group of every cluster.
// https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_AutoScalingGroupProvider.html
func TestFieldJoin_ClusterASGsAreItsCapacityProvidersGroups(t *testing.T) {
	prodASG, stagingASG := t568ASG("acme-prod-nodes"), t568ASG("acme-staging-nodes")
	fake := &t568ECS{providers: map[string]ecstypes.CapacityProvider{
		"acme-prod-cp": {Name: aws.String("acme-prod-cp"), Status: ecstypes.CapacityProviderStatusActive,
			CapacityProviderArn:      aws.String("arn:aws:ecs:us-east-1:123456789012:capacity-provider/acme-prod-cp"),
			AutoScalingGroupProvider: &ecstypes.AutoScalingGroupProvider{AutoScalingGroupArn: aws.String(prodASG.Fields["arn"])}},
		"acme-staging-cp": {Name: aws.String("acme-staging-cp"), Status: ecstypes.CapacityProviderStatusActive,
			CapacityProviderArn:      aws.String("arn:aws:ecs:us-east-1:123456789012:capacity-provider/acme-staging-cp"),
			AutoScalingGroupProvider: &ecstypes.AutoScalingGroupProvider{AutoScalingGroupArn: aws.String(stagingASG.Fields["arn"])}},
		"FARGATE": {Name: aws.String("FARGATE"), Status: ecstypes.CapacityProviderStatusActive},
	}}
	asgs := resource.ResourceCache{"asg": {Resources: []resource.Resource{prodASG, stagingASG}}}
	got := refChecker(t, "ecs", "asg")(context.Background(), &awsclient.ServiceClients{ECS: fake, Region: "us-east-1"}, t568ClusterRow("prod", "acme-prod-cp", "FARGATE"), asgs)
	t568RequireExact(t, "ecs prod → asg", got, "acme-prod-nodes")
}

func t568Instance(id string, tags map[string]string) resource.Resource {
	var ts []ec2types.Tag
	for k, v := range tags {
		ts = append(ts, ec2types.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return resource.Resource{ID: id, Name: id, Type: "ec2", Fields: map[string]string{"instance_id": id, "state": "running"},
		RawStruct: ec2types.Instance{InstanceId: aws.String(id), Tags: ts, State: &ec2types.InstanceState{Name: ec2types.InstanceStateNameRunning}}}
}

// A cluster's EC2 instances are its registered container instances
// (ListContainerInstances + DescribeContainerInstances → ec2InstanceId).
// ECS sets no cluster tag on an instance; a ClusterName tag is free text.
// https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_ContainerInstance.html
func TestFieldJoin_ClusterInstancesAreItsContainerInstances(t *testing.T) {
	const prodARN = "arn:aws:ecs:us-east-1:123456789012:cluster/prod"
	fake := &t568ECS{
		clusterARN: map[string]string{"prod": prodARN},
		instances: map[string][]ecstypes.ContainerInstance{prodARN: {{
			ContainerInstanceArn: aws.String("arn:aws:ecs:us-east-1:123456789012:container-instance/prod/2dd1b186f39845a584488d2ef155c131"),
			Ec2InstanceId:        aws.String("i-0a1b2c3d4e5f60001"), Status: aws.String("ACTIVE"), AgentConnected: true,
		}}},
	}
	ec2s := resource.ResourceCache{"ec2": {Resources: []resource.Resource{
		t568Instance("i-0a1b2c3d4e5f60001", map[string]string{"Name": "acme-prod-node"}),
		t568Instance("i-0a1b2c3d4e5f60002", map[string]string{"ClusterName": "prod"}),
	}}}
	got := refChecker(t, "ecs", "ec2")(context.Background(), &awsclient.ServiceClients{ECS: fake, Region: "us-east-1"}, t568ClusterRow("prod"), ec2s)
	t568RequireExact(t, "ecs prod → ec2", got, "i-0a1b2c3d4e5f60001")
}

// ─── dbi → eni: RDS records no owning instance on its interfaces ──────────

// t568RDSENIs answers DescribeNetworkInterfaces with the filters applied as
// EC2 applies them: every filter must hold, any of a filter's values does.
type t568RDSENIs struct {
	awsclient.EC2API
	enis []ec2types.NetworkInterface
}

func (f *t568RDSENIs) DescribeNetworkInterfaces(_ context.Context, in *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	out := &ec2.DescribeNetworkInterfacesOutput{}
	for _, eni := range f.enis {
		keep := true
		for _, flt := range in.Filters {
			switch aws.ToString(flt.Name) {
			case "description":
				keep = keep && slices.Contains(flt.Values, aws.ToString(eni.Description))
			case "group-id":
				keep = keep && slices.ContainsFunc(eni.Groups, func(g ec2types.GroupIdentifier) bool { return slices.Contains(flt.Values, aws.ToString(g.GroupId)) })
			case "requester-id":
				keep = keep && slices.Contains(flt.Values, aws.ToString(eni.RequesterId))
			case "attachment.instance-owner-id":
				keep = keep && eni.Attachment != nil && slices.Contains(flt.Values, aws.ToString(eni.Attachment.InstanceOwnerId))
			}
		}
		if keep && len(in.NetworkInterfaceIds) > 0 {
			keep = slices.Contains(in.NetworkInterfaceIds, aws.ToString(eni.NetworkInterfaceId))
		}
		if keep {
			out.NetworkInterfaces = append(out.NetworkInterfaces, eni)
		}
	}
	return out, nil
}

func t568RDSENI(id, sg string) ec2types.NetworkInterface {
	return ec2types.NetworkInterface{
		NetworkInterfaceId: aws.String(id), Description: aws.String("RDSNetworkInterface"), RequesterId: aws.String("amazon-rds"),
		RequesterManaged: aws.Bool(true), InterfaceType: ec2types.NetworkInterfaceTypeInterface, SubnetId: aws.String("subnet-0a1b2c3d4e5f60001"),
		VpcId: aws.String("vpc-0a1b2c3d4e5f60001"), Groups: []ec2types.GroupIdentifier{{GroupId: aws.String(sg), GroupName: aws.String("acme-db")}},
		Attachment: &ec2types.NetworkInterfaceAttachment{InstanceOwnerId: aws.String("amazon-rds"), Status: ec2types.AttachmentStatusAttached},
		Status:     ec2types.NetworkInterfaceStatusInUse,
	}
}

// An RDS interface carries the description "RDSNetworkInterface", the
// requester amazon-rds and the security groups of whatever uses it — nothing
// that says which instance it serves. Two databases on one security group
// share every field, so the pivot offers candidates, never a count.
func TestFieldJoin_DBInstanceInterfacesAreCandidates(t *testing.T) {
	fake := &t568RDSENIs{enis: []ec2types.NetworkInterface{
		t568RDSENI("eni-0db1111111111111a", "sg-0db0000000000000a"),
		t568RDSENI("eni-0db2222222222222b", "sg-0db0000000000000a"),
	}}
	db := resource.Resource{ID: "acme-orders-prod", Name: "acme-orders-prod", Type: "dbi", RawStruct: rdstypes.DBInstance{
		DBInstanceIdentifier: aws.String("acme-orders-prod"), Engine: aws.String("postgres"), DBInstanceStatus: aws.String("available"),
		VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{{VpcSecurityGroupId: aws.String("sg-0db0000000000000a"), Status: aws.String("active")}},
	}}
	got := refChecker(t, "dbi", "eni")(context.Background(), &awsclient.ServiceClients{EC2: fake, Region: "us-east-1"}, db, resource.ResourceCache{})
	if got.EffectiveState() == domain.RelatedResolved && got.Coverage() == domain.CoverageComplete {
		t.Errorf("dbi → eni = %v as an exact count: no field of an RDS interface names its instance", got.ResourceIDs())
	}
}

// ─── lambda ↔ eni: one Hyperplane ENI per subnet and security-group set ───

// "The first time you attach a function to a VPC using a particular subnet
// and security group combination, Lambda creates a Hyperplane ENI. Other
// functions in your account that use the same subnet and security group
// combination can also use this ENI."
// https://docs.aws.amazon.com/lambda/latest/dg/configuration-vpc.html
func TestFieldJoin_HyperplaneENIIsSharedByItsSubnetAndSecurityGroups(t *testing.T) {
	fn := func(name string, subnets, sgs []string) resource.Resource {
		return resource.Resource{ID: name, Name: name, Type: "lambda", RawStruct: lambdatypes.FunctionConfiguration{
			FunctionName: aws.String(name), FunctionArn: aws.String("arn:aws:lambda:us-east-1:123456789012:function:" + name),
			VpcConfig: &lambdatypes.VpcConfigResponse{SubnetIds: subnets, SecurityGroupIds: sgs, VpcId: aws.String("vpc-0a1b2c3d4e5f60001")},
		}}
	}
	api := fn("orders-api", []string{"subnet-0a1b2c3d4e5f60001", "subnet-0a1b2c3d4e5f60002"}, []string{"sg-0aaa111111111111a"})
	worker := fn("orders-worker", []string{"subnet-0a1b2c3d4e5f60001"}, []string{"sg-0aaa111111111111a"})
	billing := fn("billing", []string{"subnet-0a1b2c3d4e5f60001"}, []string{"sg-0aaa111111111111a", "sg-0bbb222222222222b"})
	eni := resource.Resource{ID: "eni-0f1e2d3c4b5a69788", Name: "eni-0f1e2d3c4b5a69788", Type: "eni", RawStruct: ec2types.NetworkInterface{
		NetworkInterfaceId: aws.String("eni-0f1e2d3c4b5a69788"), InterfaceType: ec2types.NetworkInterfaceTypeLambda,
		Description: aws.String("AWS Lambda VPC ENI-orders-api-0a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"),
		RequesterId: aws.String("AROAEXAMPLEROLEID1234:orders-api"), RequesterManaged: aws.Bool(true),
		SubnetId: aws.String("subnet-0a1b2c3d4e5f60001"), VpcId: aws.String("vpc-0a1b2c3d4e5f60001"),
		Groups: []ec2types.GroupIdentifier{{GroupId: aws.String("sg-0aaa111111111111a"), GroupName: aws.String("acme-orders")}},
		Status: ec2types.NetworkInterfaceStatusInUse,
	}}
	cache := resource.ResourceCache{
		"lambda": {Resources: []resource.Resource{api, worker, billing}},
		"eni":    {Resources: []resource.Resource{eni}},
	}

	fns := refChecker(t, "eni", "lambda")(context.Background(), nil, eni, cache)
	if ids := fns.ResourceIDs(); !slices.Contains(ids, "orders-api") || !slices.Contains(ids, "orders-worker") || slices.Contains(ids, "billing") {
		t.Errorf("eni → lambda = %v (coverage %v), want orders-api and orders-worker (same subnet, same security groups) and not billing", ids, fns.Coverage())
	}
	for _, tc := range []struct {
		fn   resource.Resource
		uses bool
	}{{worker, true}, {billing, false}} {
		got := refChecker(t, "lambda", "eni")(context.Background(), nil, tc.fn, cache)
		if slices.Contains(got.ResourceIDs(), eni.ID) != tc.uses {
			t.Errorf("lambda %s → eni = %v, want the Hyperplane ENI counted: %v", tc.fn.ID, got.ResourceIDs(), tc.uses)
		}
	}
}

// ─── logs → ecs-task: the task definition's awslogs-group ─────────────────

// A task writes to the log group its container definitions name in the
// awslogs driver's awslogs-group option — the field ecs-task → logs reads. A
// log group named /ecs/<family> belongs to whichever task names it there.
// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/using_awslogs.html
func TestFieldJoin_LogGroupTasksAreTheTasksWritingToIt(t *testing.T) {
	const (
		apiTD     = "arn:aws:ecs:us-east-1:123456789012:task-definition/api:42"
		billingTD = "arn:aws:ecs:us-east-1:123456789012:task-definition/billing:3"
	)
	logTo := func(family, group string) ecstypes.TaskDefinition {
		return ecstypes.TaskDefinition{Family: aws.String(family), Revision: 1, ContainerDefinitions: []ecstypes.ContainerDefinition{{
			Name: aws.String(family), Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/" + family + ":v1"),
			LogConfiguration: &ecstypes.LogConfiguration{LogDriver: ecstypes.LogDriverAwslogs, Options: map[string]string{
				"awslogs-group": group, "awslogs-region": "us-east-1", "awslogs-stream-prefix": family,
			}},
		}}}
	}
	fake := &t568ECS{taskDefs: map[string]ecstypes.TaskDefinition{
		apiTD:     logTo("api", "/aws/ecs/acme-api"),
		billingTD: logTo("billing", "/ecs/api"),
	}}
	task := func(id, td string) resource.Resource {
		return resource.Resource{ID: id, Name: id, Type: "ecs-task",
			Fields: map[string]string{"cluster": "prod", "task_definition": td, "last_status": "RUNNING"},
			RawStruct: ecstypes.Task{TaskArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod/" + id),
				TaskDefinitionArn: aws.String(td), ClusterArn: aws.String(t568Cluster), LastStatus: aws.String("RUNNING")}}
	}
	tasks := resource.ResourceCache{"ecs-task": {Resources: []resource.Resource{
		task("0f1e2d3c4b5a49788a9b0c1d2e3f4a5b", apiTD),
		task("1a2b3c4d5e6f4a7b8c9d0e1f2a3b4c5d", billingTD),
	}}}
	check := refChecker(t, "logs", "ecs-task")
	clients := &awsclient.ServiceClients{ECS: fake, Region: "us-east-1"}
	for group, want := range map[string]string{"/ecs/api": "1a2b3c4d5e6f4a7b8c9d0e1f2a3b4c5d", "/aws/ecs/acme-api": "0f1e2d3c4b5a49788a9b0c1d2e3f4a5b"} {
		lg := resource.Resource{ID: group, Name: group, Type: "logs", Fields: map[string]string{"log_group_name": group,
			"arn": "arn:aws:logs:us-east-1:123456789012:log-group:" + group}}
		t568RequireExact(t, "logs "+group+" → ecs-task", check(context.Background(), clients, lg, tasks), want)
	}
}

// ─── alarms: the dimension each service documents ─────────────────────────

func t568Alarm(name, namespace, metric string, dims ...string) resource.Resource {
	var ds []cwtypes.Dimension
	for i := 0; i+1 < len(dims); i += 2 {
		ds = append(ds, cwtypes.Dimension{Name: aws.String(dims[i]), Value: aws.String(dims[i+1])})
	}
	arn := "arn:aws:cloudwatch:us-east-1:123456789012:alarm:" + name
	return resource.Resource{ID: name, Name: name, Type: "alarm", Fields: map[string]string{"alarm_name": name, "arn": arn, "state": "OK", "namespace": namespace},
		RawStruct: cwtypes.MetricAlarm{AlarmName: aws.String(name), AlarmArn: aws.String(arn), Namespace: aws.String(namespace),
			MetricName: aws.String(metric), Dimensions: ds, StateValue: cwtypes.StateValueOk, Statistic: cwtypes.StatisticSum,
			Period: aws.Int32(300), EvaluationPeriods: aws.Int32(1), Threshold: aws.Float64(100),
			ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanThreshold}}
}

// t568WAF answers GetWebACL with the ACL's full configuration.
type t568WAF struct {
	awsclient.WAFv2API
	acl wafv2types.WebACL
}

func (f *t568WAF) GetWebACL(_ context.Context, in *wafv2.GetWebACLInput, _ ...func(*wafv2.Options)) (*wafv2.GetWebACLOutput, error) {
	if aws.ToString(in.Id) != aws.ToString(f.acl.Id) || aws.ToString(in.Name) != aws.ToString(f.acl.Name) {
		return nil, &wafv2types.WAFNonexistentItemException{Message: aws.String("AWS WAF couldn't perform the operation because your resource doesn't exist.")}
	}
	acl := f.acl
	return &wafv2.GetWebACLOutput{WebACL: &acl, LockToken: aws.String("0a1b2c3d-0000-4000-8000-000000000001")}, nil
}

// AWS/WAFV2's WebACL dimension is "The metric name of the WebACL" — the
// ACL's VisibilityConfig.MetricName, which need not be its name.
// https://docs.aws.amazon.com/waf/latest/developerguide/waf-metrics.html
func TestFieldJoin_WebACLAlarmsNameItsMetricName(t *testing.T) {
	const (
		aclID  = "3f8e1c2a-7b4d-4e6f-9a1b-2c3d4e5f6a7b"
		aclARN = "arn:aws:wafv2:us-east-1:123456789012:regional/webacl/acme-edge-acl/" + aclID
	)
	fake := &t568WAF{acl: wafv2types.WebACL{
		Name: aws.String("acme-edge-acl"), Id: aws.String(aclID), ARN: aws.String(aclARN),
		DefaultAction:    &wafv2types.DefaultAction{Allow: &wafv2types.AllowAction{}},
		VisibilityConfig: &wafv2types.VisibilityConfig{MetricName: aws.String("acmeEdgeAclMetric"), CloudWatchMetricsEnabled: true, SampledRequestsEnabled: true},
	}}
	acl := resource.Resource{ID: aclID, Name: "acme-edge-acl", Type: "waf",
		Fields:    map[string]string{"name": "acme-edge-acl", "id": aclID, "arn": aclARN, "scope": "REGIONAL", "lock_token": "0a1b2c3d-0000-4000-8000-000000000001"},
		RawStruct: wafv2types.WebACLSummary{Name: aws.String("acme-edge-acl"), Id: aws.String(aclID), ARN: aws.String(aclARN), LockToken: aws.String("0a1b2c3d-0000-4000-8000-000000000001")}}
	byMetric := t568Alarm("acme-edge-blocked-spike", "AWS/WAFV2", "BlockedRequests", "WebACL", "acmeEdgeAclMetric", "Region", "us-east-1", "Rule", "ALL")
	byName := t568Alarm("acme-edge-by-name", "AWS/WAFV2", "BlockedRequests", "WebACL", "acme-edge-acl", "Region", "us-east-1", "Rule", "ALL")
	cache := resource.ResourceCache{
		"alarm": {Resources: []resource.Resource{byMetric, byName}},
		"waf":   {Resources: []resource.Resource{acl}},
	}
	clients := &awsclient.ServiceClients{WAFv2: fake, Region: "us-east-1"}

	t568RequireExact(t, "waf → alarm", refChecker(t, "waf", "alarm")(context.Background(), clients, acl, cache), byMetric.ID)
	if got := refChecker(t, "alarm", "waf")(context.Background(), clients, byMetric, cache); !slices.Contains(got.ResourceIDs(), aclID) {
		t.Errorf("alarm %s → waf = %v (state %v), want the ACL whose metric name the alarm watches", byMetric.ID, got.ResourceIDs(), got.EffectiveState())
	}
	if got := refChecker(t, "alarm", "waf")(context.Background(), clients, byName, cache); slices.Contains(got.ResourceIDs(), aclID) {
		t.Errorf("alarm %s → waf = %v: its WebACL dimension is the ACL's name, which is not the metric name the ACL publishes", byName.ID, got.ResourceIDs())
	}
}

// AWS/ECS publishes CPUUtilization and MemoryUtilization per ClusterName and
// ServiceName; a task of a service is named by its cluster and its service.
// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/available-metrics.html
func TestFieldJoin_TaskAlarmsIncludeItsServicesECSMetrics(t *testing.T) {
	task := resource.Resource{ID: "0f1e2d3c4b5a49788a9b0c1d2e3f4a5b", Name: "0f1e2d3c4b5a49788a9b0c1d2e3f4a5b", Type: "ecs-task",
		Fields: map[string]string{"cluster": "prod", "task_definition": t568APITaskDef, "group": "service:api", "last_status": "RUNNING"},
		RawStruct: ecstypes.Task{TaskArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod/0f1e2d3c4b5a49788a9b0c1d2e3f4a5b"),
			ClusterArn: aws.String(t568Cluster), Group: aws.String("service:api"), TaskDefinitionArn: aws.String(t568APITaskDef), LastStatus: aws.String("RUNNING")}}
	own := t568Alarm("acme-api-cpu-high", "AWS/ECS", "CPUUtilization", "ClusterName", "prod", "ServiceName", "api")
	other := t568Alarm("acme-worker-cpu-high", "AWS/ECS", "CPUUtilization", "ClusterName", "prod", "ServiceName", "api-worker")
	cache := resource.ResourceCache{"alarm": {Resources: []resource.Resource{own, other}}}
	t568RequireExact(t, "ecs-task → alarm", refChecker(t, "ecs-task", "alarm")(context.Background(), nil, task, cache), own.ID)
}

// ─── CloudTrail drills: the fields the events carry ───────────────────────

// t568CT answers LookupEvents as CloudTrail does: one lookup attribute, and
// every event whose recorded field equals its value.
type t568CT struct {
	awsclient.CloudTrailAPI
	events []cloudtrailtypes.Event
}

func (f *t568CT) LookupEvents(_ context.Context, in *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	out := &cloudtrail.LookupEventsOutput{Events: []cloudtrailtypes.Event{}}
	for _, ev := range f.events {
		keep := true
		for _, a := range in.LookupAttributes {
			v := aws.ToString(a.AttributeValue)
			switch a.AttributeKey {
			case cloudtrailtypes.LookupAttributeKeyUsername:
				keep = keep && aws.ToString(ev.Username) == v
			case cloudtrailtypes.LookupAttributeKeyEventSource:
				keep = keep && aws.ToString(ev.EventSource) == v
			case cloudtrailtypes.LookupAttributeKeyEventName:
				keep = keep && aws.ToString(ev.EventName) == v
			case cloudtrailtypes.LookupAttributeKeyResourceName:
				keep = keep && slices.ContainsFunc(ev.Resources, func(r cloudtrailtypes.Resource) bool { return aws.ToString(r.ResourceName) == v })
			case cloudtrailtypes.LookupAttributeKeyResourceType:
				keep = keep && slices.ContainsFunc(ev.Resources, func(r cloudtrailtypes.Resource) bool { return aws.ToString(r.ResourceType) == v })
			default:
				keep = false
			}
		}
		if keep {
			out.Events = append(out.Events, ev)
		}
	}
	return out, nil
}

func t568CTEvent(t *testing.T, id, name, source, username string, identity, params map[string]any, resources ...cloudtrailtypes.Resource) cloudtrailtypes.Event {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"eventVersion": "1.09", "userIdentity": identity, "eventTime": "2026-09-20T10:15:00Z",
		"eventSource": source, "eventName": name, "awsRegion": "us-east-1", "sourceIPAddress": "198.51.100.23",
		"userAgent": "aws-cli/2.17.0", "requestParameters": params, "requestID": id + "-req", "eventID": id,
		"readOnly": true, "eventType": "AwsApiCall", "managementEvent": true, "recipientAccountId": "123456789012", "eventCategory": "Management",
	})
	if err != nil {
		t.Fatal(err)
	}
	return cloudtrailtypes.Event{EventId: aws.String(id), EventName: aws.String(name), EventSource: aws.String(source),
		Username: aws.String(username), EventTime: aws.Time(time.Date(2026, 9, 20, 10, 15, 0, 0, time.UTC)),
		ReadOnly: aws.String("true"), Resources: resources, CloudTrailEvent: aws.String(string(body))}
}

// t568Drill runs the pivot on src and opens its drill: the ct-events list
// fetched with the filter the pivot hands on, against events as CloudTrail
// holds them.
func t568Drill(t *testing.T, srcType string, src resource.Resource, events ...cloudtrailtypes.Event) []string {
	t.Helper()
	clients := &awsclient.ServiceClients{CloudTrail: &t568CT{events: events}, Region: "us-east-1"}
	pivot := refChecker(t, srcType, "ct-events")(context.Background(), clients, src, resource.ResourceCache{})
	if pivot.FetchFilter() == nil {
		t.Fatalf("%s → ct-events handed on no filter (state %v)", srcType, pivot.EffectiveState())
	}
	page, err := resource.GetFilteredPaginatedFetcher("ct-events")(context.Background(), clients, pivot.FetchFilter(), "")
	if err != nil {
		t.Fatalf("%s → ct-events drill: %v", srcType, err)
	}
	var ids []string
	for _, r := range page.Resources {
		ids = append(ids, r.ID)
	}
	slices.Sort(ids)
	return ids
}

// LookupEvents' Username attribute also returns the calls of an assumed-role
// session named like the user; the event's userIdentity.type tells an IAM
// user's own call ("IAMUser") from a role session's ("AssumedRole").
// https://docs.aws.amazon.com/awscloudtrail/latest/userguide/cloudtrail-event-reference-user-identity.html
func TestFieldJoin_UserEventsAreTheUsersOwnCalls(t *testing.T) {
	user := resource.Resource{ID: "alice", Name: "alice", Type: "iam-user", Fields: map[string]string{
		"user_name": "alice", "arn": "arn:aws:iam::123456789012:user/alice", "user_id": "AIDAEXAMPLEALICE12345"}}
	own := t568CTEvent(t, "0a1b2c3d-1111-4a5b-8c7d-0e1f2a3b4c5d", "ListBuckets", "s3.amazonaws.com", "alice",
		map[string]any{"type": "IAMUser", "principalId": "AIDAEXAMPLEALICE12345", "arn": "arn:aws:iam::123456789012:user/alice", "accountId": "123456789012", "userName": "alice"}, nil)
	session := t568CTEvent(t, "0a1b2c3d-2222-4a5b-8c7d-0e1f2a3b4c5d", "ListBuckets", "s3.amazonaws.com", "alice",
		map[string]any{"type": "AssumedRole", "principalId": "AROAEXAMPLEDEPLOY1234:alice", "arn": "arn:aws:sts::123456789012:assumed-role/acme-deploy/alice", "accountId": "123456789012",
			"sessionContext": map[string]any{"sessionIssuer": map[string]any{"type": "Role", "arn": "arn:aws:iam::123456789012:role/acme-deploy", "userName": "acme-deploy"}}}, nil)
	if got := t568Drill(t, "iam-user", user, own, session); !slices.Equal(got, []string{aws.ToString(own.EventId)}) {
		t.Errorf("iam-user alice → ct-events drill = %v, want only the user's own call %s", got, aws.ToString(own.EventId))
	}
}

// A package-manager request is recorded with the repository only in
// requestParameters (domainName, repositoryName, packageName) and no
// resources entry; a control-plane call names the repository as the API's
// domain and repository parameters.
// https://docs.aws.amazon.com/codeartifact/latest/ug/codeartifact-information-in-cloudtrail.html
func TestFieldJoin_RepositoryEventsIncludePackageRequests(t *testing.T) {
	repo := resource.Resource{ID: "acme/acme-npm", Name: "acme-npm", Type: "codeartifact", Fields: map[string]string{
		"repo_name": "acme-npm", "domain_name": "acme", "domain_owner": "123456789012",
		"arn": "arn:aws:codeartifact:us-east-1:123456789012:repository/acme/acme-npm"}}
	caller := map[string]any{"type": "AssumedRole", "principalId": "AROAEXAMPLECIBUILD123:ci", "arn": "arn:aws:sts::123456789012:assumed-role/acme-ci/ci", "accountId": "123456789012"}
	read := func(id, domain, repoName string) cloudtrailtypes.Event {
		return t568CTEvent(t, id, "ReadFromRepository", "codeartifact.amazonaws.com", "ci", caller, map[string]any{
			"domainName": domain, "domainOwner": "123456789012", "repositoryName": repoName,
			"packageName": "lodash", "packageFormat": "npm", "packageVersion": "4.17.21"})
	}
	pkg := read("1b2c3d4e-0001-4a5b-8c7d-0e1f2a3b4c5d", "acme", "acme-npm")
	otherDomain := read("1b2c3d4e-0002-4a5b-8c7d-0e1f2a3b4c5d", "partner", "acme-npm")
	otherRepo := read("1b2c3d4e-0003-4a5b-8c7d-0e1f2a3b4c5d", "acme", "acme-npm-cache")
	describe := t568CTEvent(t, "1b2c3d4e-0004-4a5b-8c7d-0e1f2a3b4c5d", "DescribeRepository", "codeartifact.amazonaws.com", "ci", caller,
		map[string]any{"domain": "acme", "domainOwner": "123456789012", "repository": "acme-npm"},
		cloudtrailtypes.Resource{ResourceType: aws.String("AWS::CodeArtifact::Repository"), ResourceName: aws.String(repo.Fields["arn"])})

	got := t568Drill(t, "codeartifact", repo, pkg, otherDomain, otherRepo, describe)
	want := []string{aws.ToString(pkg.EventId), aws.ToString(describe.EventId)}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("codeartifact acme/acme-npm → ct-events drill = %v, want the package request and the control-plane call %v", got, want)
	}
}
