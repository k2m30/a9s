package unit_test

// A related row's count state says what was read: exact when every place the
// relation lives was read over a whole target list, a lower bound when the
// list is a subset, unknown when a place could not be read, and a proven 0
// only when everything was read and held nothing.

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// t567Badge is the count badge the related panel renders for r.
func t567Badge(r resource.RelatedCheckResult) string {
	return resource.FormatRelatedCount(r.State(), r.Count(), r.Truncated())
}

func t567ProvenZero(r resource.RelatedCheckResult) bool {
	return r.State() == domain.RelatedResolved && r.Count() == 0 && !r.Truncated()
}

// t567FieldsOnly is rows as the disk cache restores them: ID, Name and Fields
// survive, the SDK struct does not.
func t567FieldsOnly(rows []resource.Resource) resource.ResourceCacheEntry {
	out := make([]resource.Resource, len(rows))
	for i, r := range rows {
		r.RawStruct = nil
		out[i] = r
	}
	return resource.ResourceCacheEntry{Resources: out, FieldsOnly: true}
}

func t567CacheWith(base resource.ResourceCache, typ string, entry resource.ResourceCacheEntry) resource.ResourceCache {
	c := maps.Clone(base)
	c[typ] = entry
	return c
}

func t567Denied(op string) error {
	return &smithy.GenericAPIError{
		Code:    "AccessDeniedException",
		Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/session is not authorized to perform: " + op,
	}
}

// t567EKSFake answers ListClusters with two clusters and denies DescribeCluster
// on the second, the shape a role scoped to some clusters' ARNs gets.
type t567EKSFake struct {
	awsclient.EKSAPI
	denied string
}

func (f *t567EKSFake) ListClusters(context.Context, *eks.ListClustersInput, ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	return &eks.ListClustersOutput{Clusters: []string{"acme-prod", f.denied}}, nil
}

func (f *t567EKSFake) DescribeCluster(_ context.Context, in *eks.DescribeClusterInput, _ ...func(*eks.Options)) (*eks.DescribeClusterOutput, error) {
	name := aws.ToString(in.Name)
	if name == f.denied {
		return nil, t567Denied("eks:DescribeCluster on resource: arn:aws:eks:us-east-1:123456789012:cluster/" + name)
	}
	return &eks.DescribeClusterOutput{Cluster: &ekstypes.Cluster{
		Name:     aws.String(name),
		Arn:      aws.String("arn:aws:eks:us-east-1:123456789012:cluster/" + name),
		Status:   ekstypes.ClusterStatusActive,
		Version:  aws.String("1.31"),
		RoleArn:  aws.String("arn:aws:iam::123456789012:role/acme-eks-cluster-role"),
		Endpoint: aws.String("https://ABCDEF0123456789ABCDEF0123456789.gr7.us-east-1.eks.amazonaws.com"),
		ResourcesVpcConfig: &ekstypes.VpcConfigResponse{
			SubnetIds: []string{"subnet-0ccc333333333333c"},
			VpcId:     aws.String("vpc-0abc123def456789a"),
		},
	}}, nil
}

// TestT567AlarmEKS_DegradedClusterLeavesAnIDMatchExact: an alarm names a
// cluster by its ClusterName dimension, which is the cluster's ID. A cluster
// whose DescribeCluster was refused still has its ID on the row, so it cannot
// hide a match, and every alarm's EKS row is exact.
func TestT567AlarmEKS_DegradedClusterLeavesAnIDMatchExact(t *testing.T) {
	b := newRefBench(t)
	if !slices.ContainsFunc(b.byType["eks"], func(r resource.Resource) bool { return r.Fields[awsclient.DegradedFindingField] != "" }) {
		t.Fatal("demo eks list carries no degraded row; the witness needs one")
	}
	def := refRelatedDef(t, "alarm", "eks")
	t567EveryAlarmEKSExact(t, b, func(alarm resource.Resource) resource.RelatedCheckResult {
		return def.Checker(context.Background(), refClients(), alarm, b.cache)
	}, def.DisplayName)
}

// t567EveryAlarmEKSExact checks every demo alarm's EKS row: the one alarm on
// acme-prod counts it, every other alarm counts none.
func t567EveryAlarmEKSExact(t *testing.T, b refBench, check func(resource.Resource) resource.RelatedCheckResult, label string) {
	t.Helper()
	var wrong []string
	for _, alarm := range b.byType["alarm"] {
		want := "(0)"
		if alarm.ID == "eks-acme-prod-control-plane-errors" {
			want = "(1)"
		}
		if badge := t567Badge(check(alarm)); badge != want {
			wrong = append(wrong, alarm.ID+": "+label+" "+badge+", want "+want)
		}
	}
	if len(wrong) > 0 {
		t.Errorf("%d of %d alarms render a wrong EKS row, first: %s", len(wrong), len(b.byType["alarm"]), wrong[0])
	}
}

// TestT567AlarmEKS_PerItemDescribeFailureIsNotASubset: a fetch that listed
// every cluster and could not describe one read the whole list; the refused
// describe withholds details, not rows.
func TestT567AlarmEKS_PerItemDescribeFailureIsNotASubset(t *testing.T) {
	b := newRefBench(t)
	clients := refClients()
	clients.EKS = &t567EKSFake{EKSAPI: clients.EKS, denied: "acme-analytics"}
	cache := resource.ResourceCache{}
	check := refChecker(t, "alarm", "eks")
	t567EveryAlarmEKSExact(t, b, func(alarm resource.Resource) resource.RelatedCheckResult {
		return check(context.Background(), clients, alarm, cache)
	}, "EKS Clusters")
}

// TestT567EKSPivots_DegradedClusterKeepsADetailMatchALowerBound: a cluster's
// subnets and role come from DescribeCluster, so the cluster nobody could
// describe may be the one that uses this subnet or role.
func TestT567EKSPivots_DegradedClusterKeepsADetailMatchALowerBound(t *testing.T) {
	clients := refClients()
	clients.EKS = &t567EKSFake{EKSAPI: clients.EKS, denied: "acme-analytics"}
	cache := resource.ResourceCache{}
	cases := []struct {
		source string
		row    resource.Resource
		want   string
	}{
		{"subnet", resource.Resource{ID: "subnet-0ccc333333333333c", Type: "subnet"}, "(1+)"},
		{"subnet", resource.Resource{ID: "subnet-0ddd444444444444d", Type: "subnet"}, "(0+)"},
	}
	for _, tc := range cases {
		got := refChecker(t, tc.source, "eks")(context.Background(), clients, tc.row, cache)
		if badge := t567Badge(got); badge != tc.want {
			t.Errorf("%s %s → EKS Clusters %s, want %s", tc.source, tc.row.ID, badge, tc.want)
		}
	}

	b := newRefBench(t)
	for _, subnet := range b.byType["subnet"] {
		got := refChecker(t, "subnet", "eks")(context.Background(), refClients(), subnet, b.cache)
		if !got.Truncated() {
			t.Errorf("demo subnet %s → EKS Clusters %s over a list with an undescribed cluster, want a lower bound", subnet.ID, t567Badge(got))
		}
	}
}

// TestT567AlarmSFN_DegradedRowHidesAnARNMatch: an alarm names a state machine
// by its ARN, which a row without its details does not carry, so that row may
// be the one the alarm names.
func TestT567AlarmSFN_DegradedRowHidesAnARNMatch(t *testing.T) {
	b := newRefBench(t)
	const sm = "order-fulfillment-workflow"
	rows := slices.Clone(b.byType["sfn"])
	i := slices.IndexFunc(rows, func(r resource.Resource) bool { return r.ID == sm })
	if i < 0 {
		t.Fatalf("demo sfn list has no %s", sm)
	}
	rows[i] = awsclient.DegradedDetails("sfn", sm, t567Denied("states:DescribeStateMachine"))
	cache := t567CacheWith(b.cache, "sfn", resource.ResourceCacheEntry{Resources: rows})
	alarm := resource.Resource{ID: "order-fulfillment-failures", Name: "order-fulfillment-failures", Type: "alarm", RawStruct: cwtypes.MetricAlarm{
		AlarmName:  aws.String("order-fulfillment-failures"),
		AlarmArn:   aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:order-fulfillment-failures"),
		Namespace:  aws.String("AWS/States"),
		MetricName: aws.String("ExecutionsFailed"),
		Dimensions: []cwtypes.Dimension{{Name: aws.String("StateMachineArn"), Value: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:" + sm)}},
		StateValue: cwtypes.StateValueOk,
	}}
	got := refChecker(t, "alarm", "sfn")(context.Background(), refClients(), alarm, cache)
	if got.State() != domain.RelatedUnknown && !got.Truncated() {
		t.Errorf("alarm → State Machines %s with the named machine undescribed, want a lower bound or unknown", t567Badge(got))
	}
}

// TestT567AlarmMatchSpec_RawStructFlagFollowsWhatTheSpecReads: an alarm names
// a row by its ID or Name unless the spec reads more of it; only a spec that
// reads more can miss a row whose details were not read.
func TestT567AlarmMatchSpec_RawStructFlagFollowsWhatTheSpecReads(t *testing.T) {
	for _, typ := range resource.AllShortNames() {
		spec, ok := awsclient.AlarmMatchSpecFor(typ)
		if !ok {
			continue
		}
		readsBeyond := spec.Values != nil || spec.QualifierValue != nil || spec.MetricsRegion != nil
		if spec.ValuesFromRawStruct != readsBeyond {
			t.Errorf("%s: ValuesFromRawStruct=%v, but the spec reads beyond ID and Name: %v", typ, spec.ValuesFromRawStruct, readsBeyond)
		}
	}
}

// TestT567OneToOnePivots_DegradedRowNeverMakesOnePlus: a node group has one
// cluster and an Auto Scaling group one node group. Once the parent is found
// no other row can add to the answer.
func TestT567OneToOnePivots_DegradedRowNeverMakesOnePlus(t *testing.T) {
	b := newRefBench(t)
	if !slices.ContainsFunc(b.byType["ng"], func(r resource.Resource) bool { return r.Fields[awsclient.DegradedFindingField] != "" }) {
		t.Fatal("demo ng list carries no degraded row; the witness needs one")
	}
	for _, ng := range b.byType["ng"] {
		got := refChecker(t, "ng", "eks")(context.Background(), refClients(), ng, b.cache)
		if badge := t567Badge(got); badge != "(1)" {
			t.Errorf("ng %s → EKS Clusters %s, want (1)", ng.ID, badge)
		}
	}
	for _, asg := range b.byType["asg"] {
		got := refChecker(t, "asg", "ng")(context.Background(), refClients(), asg, b.cache)
		switch {
		case asg.ID == "eks-acme-prod-ng-general":
			if badge := t567Badge(got); badge != "(1)" || !slices.Equal(got.ResourceIDs(), []string{"acme-prod/general-pool"}) {
				t.Errorf("asg %s → Node Groups %s %v, want (1) [acme-prod/general-pool]", asg.ID, badge, got.ResourceIDs())
			}
		case got.Count() > 0 && got.Truncated():
			t.Errorf("asg %s → Node Groups %s, a one-to-one pivot is never N+", asg.ID, t567Badge(got))
		}
	}
}

// TestT567ReverseScans_FieldsOnlyTargetListIsReadNotSkipped: a list restored
// from disk carries its rows without the SDK struct. A pivot reads it through
// a fetch or off the rows' fields, and reaches the answer the struct-backed
// list gives.
func TestT567ReverseScans_FieldsOnlyTargetListIsReadNotSkipped(t *testing.T) {
	b := newRefBench(t)
	pairs := []struct{ source, target string }{
		{"secrets", "eb"},
		{"ecs-svc", "eb-rule"},
		{"secrets", "ecs-task"},
		{"ecr", "eb-rule"},
		{"ecs-svc", "sfn"},
	}
	for _, p := range pairs {
		t.Run(p.source+"→"+p.target, func(t *testing.T) {
			check := refChecker(t, p.source, p.target)
			restored := t567CacheWith(b.cache, p.target, t567FieldsOnly(b.byType[p.target]))
			var witnesses int
			var wrong []string
			for _, src := range b.byType[p.source] {
				full := check(context.Background(), refClients(), src, b.cache)
				if full.Count() == 0 {
					continue
				}
				witnesses++
				got := check(context.Background(), refClients(), src, restored)
				if t567Badge(got) != t567Badge(full) || !slices.Equal(sortedIDs(got), sortedIDs(full)) {
					wrong = append(wrong, fmt.Sprintf("%s = %s %v, want %s %v", src.ID, t567Badge(got), sortedIDs(got), t567Badge(full), sortedIDs(full)))
				}
			}
			if witnesses == 0 {
				t.Fatalf("no demo %s row relates to a %s; the pivot has no witness", p.source, p.target)
			}
			if len(wrong) > 0 {
				t.Errorf("%d of %d %s rows differ over a disk-restored %s list, first: %s", len(wrong), witnesses, p.source, p.target, wrong[0])
			}
		})
	}
}

// TestT567LogsKMS_CacheSeededRowCountsItsKeyFromFields: a log group's key is
// on its row's fields, which the disk cache keeps.
func TestT567LogsKMS_CacheSeededRowCountsItsKeyFromFields(t *testing.T) {
	b := newRefBench(t)
	check := refChecker(t, "logs", "kms")
	var witnesses int
	var wrong []string
	for _, lg := range b.byType["logs"] {
		if lg.Fields["kms_key_id"] == "" {
			continue
		}
		full := check(context.Background(), refClients(), lg, b.cache)
		if full.Count() == 0 {
			continue
		}
		witnesses++
		seeded := lg
		seeded.RawStruct = nil
		got := check(context.Background(), refClients(), seeded, b.cache)
		if t567Badge(got) != t567Badge(full) || !slices.Equal(sortedIDs(got), sortedIDs(full)) {
			wrong = append(wrong, fmt.Sprintf("%s = %q %v, want %s %v", lg.ID, t567Badge(got), sortedIDs(got), t567Badge(full), sortedIDs(full)))
		}
	}
	if witnesses == 0 {
		t.Fatal("no demo log group carries kms_key_id and a key the kms list holds")
	}
	if len(wrong) > 0 {
		t.Errorf("%d of %d cache-seeded log groups read their KMS row differently from the loaded row, first: %s", len(wrong), witnesses, wrong[0])
	}
}

// TestT567TGELB_ClosedARNSetIsExactOverAPartialList: a target group names its
// load balancers by ARN, a closed set; a page of the elb list nobody read can
// hold no load balancer this target group did not name.
func TestT567TGELB_ClosedARNSetIsExactOverAPartialList(t *testing.T) {
	b := newRefBench(t)
	const webARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-prod-web/1234567890abcdef"
	tg := func(arns ...string) resource.Resource {
		return resource.Resource{ID: "acme-web-tg", Name: "acme-web-tg", Type: "tg", RawStruct: elbv2types.TargetGroup{
			TargetGroupName:  aws.String("acme-web-tg"),
			TargetGroupArn:   aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web-tg/73e2d6bc24d8a067"),
			Protocol:         elbv2types.ProtocolEnumHttp,
			Port:             aws.Int32(80),
			VpcId:            aws.String("vpc-0abc123def456789a"),
			TargetType:       elbv2types.TargetTypeEnumInstance,
			LoadBalancerArns: arns,
		}}
	}
	partial := t567CacheWith(b.cache, "elb", resource.ResourceCacheEntry{Resources: b.byType["elb"], IsTruncated: true})
	check := refChecker(t, "tg", "elb")

	got := check(context.Background(), refClients(), tg(webARN), partial)
	if badge := t567Badge(got); badge != "(1)" || !slices.Equal(got.ResourceIDs(), []string{"acme-prod-web"}) {
		t.Errorf("tg → Load Balancers over a partial elb list = %s %v, want (1) [acme-prod-web]", badge, got.ResourceIDs())
	}

	got = check(context.Background(), refClients(), tg(webARN), b.cache)
	if badge := t567Badge(got); badge != "(1)" {
		t.Errorf("tg → Load Balancers over the whole elb list = %s, want (1)", badge)
	}

	const unloaded = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-batch-alb/9f8e7d6c5b4a3921"
	got = check(context.Background(), refClients(), tg(unloaded), partial)
	if t567ProvenZero(got) || got.Count() != 0 {
		t.Errorf("tg → Load Balancers naming an ARN no loaded row holds = %s %v, want no proven count", t567Badge(got), got.ResourceIDs())
	}
}

// t567NGEKSFake lists one cluster with two launch-template node groups.
type t567NGEKSFake struct {
	awsclient.EKSAPI
}

func (f *t567NGEKSFake) ListClusters(context.Context, *eks.ListClustersInput, ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	return &eks.ListClustersOutput{Clusters: []string{"acme-prod"}}, nil
}

func (f *t567NGEKSFake) ListNodegroups(context.Context, *eks.ListNodegroupsInput, ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error) {
	return &eks.ListNodegroupsOutput{Nodegroups: []string{"general-pool", "gpu-pool"}}, nil
}

func (f *t567NGEKSFake) DescribeNodegroup(_ context.Context, in *eks.DescribeNodegroupInput, _ ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
	name := aws.ToString(in.NodegroupName)
	ltID := map[string]string{"general-pool": "lt-0a1b2c3d4e5f60001", "gpu-pool": "lt-0a1b2c3d4e5f60002"}[name]
	return &eks.DescribeNodegroupOutput{Nodegroup: &ekstypes.Nodegroup{
		NodegroupName:  aws.String(name),
		ClusterName:    aws.String("acme-prod"),
		NodegroupArn:   aws.String("arn:aws:eks:us-east-1:123456789012:nodegroup/acme-prod/" + name + "/5ec8b6a2-1111-2222-3333-444455556666"),
		Status:         ekstypes.NodegroupStatusActive,
		AmiType:        ekstypes.AMITypesCustom,
		ReleaseVersion: aws.String("ami-0eks111111111111a"),
		NodeRole:       aws.String("arn:aws:iam::123456789012:role/acme-eks-node-role"),
		Subnets:        []string{"subnet-0ccc333333333333c"},
		LaunchTemplate: &ekstypes.LaunchTemplateSpecification{Id: aws.String(ltID), Name: aws.String("acme-" + name), Version: aws.String("3")},
		Resources: &ekstypes.NodegroupResources{AutoScalingGroups: []ekstypes.AutoScalingGroup{
			{Name: aws.String("eks-" + name + "-5ec8b6a2-1111-2222-3333-444455556666")},
		}},
	}}, nil
}

// t567LTEC2Fake answers DescribeLaunchTemplateVersions per template ID, and
// refuses the templates in denied the way EC2 refuses a call.
type t567LTEC2Fake struct {
	awsclient.EC2API
	data   map[string]ec2types.ResponseLaunchTemplateData
	denied map[string]bool
}

func (f *t567LTEC2Fake) DescribeLaunchTemplateVersions(_ context.Context, in *ec2.DescribeLaunchTemplateVersionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	id := aws.ToString(in.LaunchTemplateId)
	if f.denied[id] {
		return nil, &smithy.GenericAPIError{Code: "UnauthorizedOperation", Message: "You are not authorized to perform this operation."}
	}
	d, ok := f.data[id]
	if !ok {
		return nil, &smithy.GenericAPIError{Code: "InvalidLaunchTemplateId.NotFound", Message: "The specified launch template, with template ID " + id + ", does not exist."}
	}
	return &ec2.DescribeLaunchTemplateVersionsOutput{LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{{
		LaunchTemplateId:   aws.String(id),
		VersionNumber:      aws.Int64(3),
		DefaultVersion:     aws.Bool(true),
		LaunchTemplateData: &d,
	}}}, nil
}

// t567FetchNG runs the registered node-group fetcher and returns the cache
// entry the app keeps for what it read.
func t567FetchNG(t *testing.T, lt *t567LTEC2Fake) resource.ResourceCacheEntry {
	t.Helper()
	clients := refClients()
	clients.EKS = &t567NGEKSFake{EKSAPI: clients.EKS}
	lt.EC2API = clients.EC2
	clients.EC2 = lt
	result, _ := resource.GetPaginatedFetcher("ng")(context.Background(), clients, "")
	if len(result.Resources) != 2 {
		t.Fatalf("ng fetcher returned %d rows, want 2", len(result.Resources))
	}
	truncated := result.Pagination != nil && result.Pagination.IsTruncated
	return resource.ResourceCacheEntry{Resources: result.Resources, IsTruncated: truncated}
}

// TestT567AMING_UnresolvedImageIsALowerBound: a node group whose launch
// template could not be read runs an image nobody saw, which may be this AMI.
func TestT567AMING_UnresolvedImageIsALowerBound(t *testing.T) {
	const used, other = "ami-0eks111111111111a", "ami-0a1b2c3d4e5f60002"
	check := refChecker(t, "ami", "ng")
	ami := func(id string) resource.Resource {
		return resource.Resource{ID: id, Name: id, Type: "ami", RawStruct: ec2types.Image{ImageId: aws.String(id), State: ec2types.ImageStateAvailable}}
	}

	whole := t567FetchNG(t, &t567LTEC2Fake{data: map[string]ec2types.ResponseLaunchTemplateData{
		"lt-0a1b2c3d4e5f60001": {ImageId: aws.String(used), InstanceType: ec2types.InstanceTypeM5Large},
		"lt-0a1b2c3d4e5f60002": {ImageId: aws.String(used), InstanceType: ec2types.InstanceTypeG4dnXlarge},
	}})
	cache := resource.ResourceCache{"ng": whole}
	if badge := t567Badge(check(context.Background(), refClients(), ami(used), cache)); badge != "(2)" {
		t.Errorf("ami %s → Node Groups with every template read = %s, want (2)", used, badge)
	}
	if badge := t567Badge(check(context.Background(), refClients(), ami(other), cache)); badge != "(0)" {
		t.Errorf("ami %s → Node Groups with every template read = %s, want (0)", other, badge)
	}

	partial := t567FetchNG(t, &t567LTEC2Fake{
		data:   map[string]ec2types.ResponseLaunchTemplateData{"lt-0a1b2c3d4e5f60001": {ImageId: aws.String(used), InstanceType: ec2types.InstanceTypeM5Large}},
		denied: map[string]bool{"lt-0a1b2c3d4e5f60002": true},
	})
	cache = resource.ResourceCache{"ng": partial}
	if badge := t567Badge(check(context.Background(), refClients(), ami(used), cache)); badge != "(1+)" {
		t.Errorf("ami %s → Node Groups with gpu-pool's template unread = %s, want (1+)", used, badge)
	}
	if badge := t567Badge(check(context.Background(), refClients(), ami(other), cache)); badge != "(0+)" {
		t.Errorf("ami %s → Node Groups with gpu-pool's template unread = %s, want (0+)", other, badge)
	}
}

// t567ECSFake is one cluster running two tasks of one task definition whose
// DescribeTaskDefinition is refused.
type t567ECSFake struct {
	awsclient.ECSAPI
}

const (
	t567Cluster = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-services"
	t567TaskDef = "arn:aws:ecs:us-east-1:123456789012:task-definition/acme-billing-worker:7"
)

func (f *t567ECSFake) ListClusters(context.Context, *ecs.ListClustersInput, ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return &ecs.ListClustersOutput{ClusterArns: []string{t567Cluster}}, nil
}

func (f *t567ECSFake) ListTasks(context.Context, *ecs.ListTasksInput, ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	return &ecs.ListTasksOutput{TaskArns: []string{
		"arn:aws:ecs:us-east-1:123456789012:task/acme-services/0f1e2d3c4b5a69788796a5b4c3d2e1f0",
		"arn:aws:ecs:us-east-1:123456789012:task/acme-services/1a2b3c4d5e6f70819293a4b5c6d7e8f9",
	}}, nil
}

func (f *t567ECSFake) DescribeTasks(_ context.Context, in *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	out := &ecs.DescribeTasksOutput{}
	for _, arn := range in.Tasks {
		out.Tasks = append(out.Tasks, ecstypes.Task{
			TaskArn:           aws.String(arn),
			ClusterArn:        aws.String(t567Cluster),
			TaskDefinitionArn: aws.String(t567TaskDef),
			LastStatus:        aws.String("RUNNING"),
			DesiredStatus:     aws.String("RUNNING"),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("256"),
			Memory:            aws.String("512"),
			Group:             aws.String("service:billing-worker"),
		})
	}
	return out, nil
}

func (f *t567ECSFake) DescribeTaskDefinition(context.Context, *ecs.DescribeTaskDefinitionInput, ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	return nil, t567Denied("ecs:DescribeTaskDefinition")
}

// TestT567ECSTask_EveryTaskOfAnUnreadDefinitionIsUnknown: a task's roles,
// secrets and parameters are on its task definition. Every task of a
// definition nobody could read has them unread, not only the first.
func TestT567ECSTask_EveryTaskOfAnUnreadDefinitionIsUnknown(t *testing.T) {
	clients := refClients()
	clients.ECS = &t567ECSFake{ECSAPI: clients.ECS}
	result, _ := resource.GetPaginatedFetcher("ecs-task")(context.Background(), clients, "")
	if len(result.Resources) != 2 {
		t.Fatalf("ecs-task fetcher returned %d rows, want 2", len(result.Resources))
	}
	b := newRefBench(t)
	for _, task := range result.Resources {
		for _, target := range []string{"role", "secrets", "ssm"} {
			got := refChecker(t, "ecs-task", target)(context.Background(), refClients(), task, b.cache)
			if got.State() != domain.RelatedUnknown {
				t.Errorf("task %s → %s = %s (state %s), want unknown: its task definition was not read", task.ID, target, t567Badge(got), got.State())
			}
		}
	}
}
