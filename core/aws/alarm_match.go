// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// alarm_match.go holds the table both directions of every alarm pivot read:
// which CloudWatch namespaces and dimension names name a resource of a type,
// and which alarm actions do. A dimension name alone is ambiguous —
// ClusterName names an ECS cluster in one namespace and an EKS cluster in
// another — so a match is a (namespace, dimension name, value) triple.
package aws

import (
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// AlarmDimension is one dimension of one metric an alarm watches, carrying
// the namespace of the metric it belongs to.
type AlarmDimension struct {
	Namespace string
	Name      string
	Value     string
}

// AlarmDimensions returns every metric dimension the alarm watches: those of
// a single-metric alarm, whose namespace and dimensions are top-level, and
// those of every query of a metric-math or anomaly-detection alarm, whose
// top-level namespace and dimensions AWS leaves empty and whose queries each
// carry a namespace of their own.
func AlarmDimensions(a cwtypes.MetricAlarm) []AlarmDimension {
	var out []AlarmDimension
	collect := func(namespace string, dims []cwtypes.Dimension) {
		for _, d := range dims {
			if d.Name != nil && d.Value != nil {
				out = append(out, AlarmDimension{Namespace: namespace, Name: *d.Name, Value: *d.Value})
			}
		}
	}
	collect(aws.ToString(a.Namespace), a.Dimensions)
	for _, q := range a.Metrics {
		if q.MetricStat != nil && q.MetricStat.Metric != nil {
			collect(aws.ToString(q.MetricStat.Metric.Namespace), q.MetricStat.Metric.Dimensions)
		}
	}
	return out
}

// AlarmMetric is a metric an alarm watches, as DescribeMetricFilters names
// one: the namespace a metric filter emits into and the metric's name.
type AlarmMetric struct {
	Namespace string
	Name      string
}

// AlarmMetricWatched returns the metric a single-metric alarm watches. An
// alarm over a metric-math expression watches several and names none of them
// at the top level, which is no one metric a filter can be looked up by.
func AlarmMetricWatched(a cwtypes.MetricAlarm) (AlarmMetric, bool) {
	m := AlarmMetric{Namespace: aws.ToString(a.Namespace), Name: aws.ToString(a.MetricName)}
	return m, m.Namespace != "" && m.Name != ""
}

// AlarmMatchSpec is how one resource type is named by an alarm: the metric
// namespaces the service publishes its per-resource metrics under, the
// dimension names that carry the resource's identity in them, and the
// service whose action ARNs name the resource too.
type AlarmMatchSpec struct {
	Namespaces     []string
	DimensionNames []string
	// ActionService is the AWS service of an alarm action ARN that names the
	// resource: an alarm links to the group whose scaling policy it runs and
	// to the topic it notifies without carrying either as a dimension.
	ActionService string
	// QualifierDimension and QualifierValue pin a dimension the resource
	// shares with something more specific than itself: a cluster's name is
	// carried by the alarms of every service in it, so an alarm that names a
	// service must name this row's. known is false for a row that cannot say
	// which one it belongs to; a row that belongs to none answers "", true.
	QualifierDimension string
	QualifierValue     func(resource.Resource) (name string, known bool)
	// Values are what a dimension or action must carry to name the row, and
	// whether the row said who it is at all; with no Values, the row's ID and
	// Name are its identity.
	Values func(resource.Resource) (values []string, read bool)
	// ValuesFromRawStruct marks a type whose identity is read from the row's
	// RawStruct, so a row restored without one may be named by alarms this
	// scan cannot recognise.
	ValuesFromRawStruct bool
	// MetricsRegion returns the region a row's metrics are published in, and
	// so the only region an alarm over them can live in. Nil for a type whose
	// metrics are in the region its rows are, and a nil answer of "" says the
	// same of one row.
	MetricsRegion func(resource.Resource) string
}

// metricsRegionOf is MetricsRegion for a spec that declares none.
func (s AlarmMatchSpec) metricsRegionOf(row resource.Resource) string {
	if s.MetricsRegion == nil {
		return ""
	}
	return s.MetricsRegion(row)
}

// metricsRegionUSEast1 is the MetricsRegion of a global service: CloudFront
// publishes its per-distribution metrics in us-east-1 whatever region the
// operator is browsing.
func metricsRegionUSEast1(resource.Resource) string { return "us-east-1" }

// metricsRegionOfWebACL is the MetricsRegion of a web ACL: AWS publishes a
// CloudFront-scope ACL's metrics in us-east-1, and documents the Region
// dimension as required for every protected resource type except CloudFront
// distributions
// (docs.aws.amazon.com/waf/latest/developerguide/waf-metrics.html).
func metricsRegionOfWebACL(row resource.Resource) string { return wafRegionOf(row.Fields["scope"]) }

// alarmValuesOf returns what a dimension or an action must carry to name the
// row, and whether the row's identity could be read.
func (s AlarmMatchSpec) alarmValuesOf(row resource.Resource) (values []string, read bool) {
	if s.Values != nil {
		return s.Values(row)
	}
	return nonEmpty(row.ID, row.Name), true
}

// names reports whether the alarm names the row, by a dimension of a metric
// it watches or by one of its actions.
func (s AlarmMatchSpec) names(a cwtypes.MetricAlarm, row resource.Resource) bool {
	values, _ := s.alarmValuesOf(row)
	if len(values) == 0 {
		return false
	}
	dims := AlarmDimensions(a)
	for _, d := range dims {
		if slices.Contains(s.Namespaces, d.Namespace) && slices.Contains(s.DimensionNames, d.Name) &&
			slices.Contains(values, d.Value) && s.qualifies(dims, row) {
			return true
		}
	}
	return s.actionNames(a, values)
}

// qualifies reports whether the alarm's qualifier dimension names what this
// row belongs to. An alarm that carries none of it, and a row that cannot
// say, leave the question unanswered and the dimension match stands; a row
// that belongs to nothing is not named by an alarm that names something.
func (s AlarmMatchSpec) qualifies(dims []AlarmDimension, row resource.Resource) bool {
	if s.QualifierDimension == "" {
		return true
	}
	want, known := s.QualifierValue(row)
	if !known {
		return true
	}
	carried := false
	for _, d := range dims {
		if d.Name == s.QualifierDimension {
			carried = true
			if d.Value == want {
				return true
			}
		}
	}
	return !carried
}

// actionNames reports whether one of the alarm's actions names the row.
func (s AlarmMatchSpec) actionNames(a cwtypes.MetricAlarm, values []string) bool {
	if s.ActionService == "" {
		return false
	}
	for _, actions := range [][]string{a.AlarmActions, a.OKActions, a.InsufficientDataActions} {
		for _, action := range actions {
			if slices.Contains(values, alarmActionTarget(action, s.ActionService)) {
				return true
			}
		}
	}
	return false
}

// alarmActionTarget returns what an alarm action ARN of service names: the
// Auto Scaling group whose policy the action runs, which the policy ARN
// carries after "autoScalingGroupName/", or the ARN itself for a service
// whose actions name the resource directly.
func alarmActionTarget(action, service string) string {
	a, ok := ARNForService(action, service)
	if !ok {
		return ""
	}
	if _, after, found := strings.Cut(a.Resource, "autoScalingGroupName/"); found {
		group, _, _ := strings.Cut(after, ":")
		return group
	}
	return action
}

// nonEmpty returns the distinct non-empty values, in order.
func nonEmpty(values ...string) []string {
	var out []string
	for _, v := range values {
		if v != "" && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// alarmMatchSpecs is the one table of how each type is named by an alarm,
// from each service's CloudWatch metrics documentation.
//
//nolint:gochecknoglobals // the catalog's alarm-match table, read-only after init
var alarmMatchSpecs = map[string]AlarmMatchSpec{
	"apigw":      {Namespaces: []string{"AWS/ApiGateway"}, DimensionNames: []string{"ApiId", "ApiName"}},
	"asg":        {Namespaces: []string{"AWS/AutoScaling", "AWS/EC2"}, DimensionNames: []string{"AutoScalingGroupName"}, ActionService: "autoscaling"},
	"cb":         {Namespaces: []string{"AWS/CodeBuild"}, DimensionNames: []string{"ProjectName"}},
	"cf":         {Namespaces: []string{"AWS/CloudFront"}, DimensionNames: []string{"DistributionId"}, MetricsRegion: metricsRegionUSEast1},
	"dbc":        {Namespaces: []string{"AWS/RDS", "AWS/DocDB"}, DimensionNames: []string{"DBClusterIdentifier"}},
	"dbi":        {Namespaces: []string{"AWS/RDS"}, DimensionNames: []string{"DBInstanceIdentifier"}},
	"ddb":        {Namespaces: []string{"AWS/DynamoDB"}, DimensionNames: []string{"TableName"}},
	"eb":         {Namespaces: []string{"AWS/ElasticBeanstalk", "ElasticBeanstalk/SQSD"}, DimensionNames: []string{"EnvironmentName"}},
	"ebs":        {Namespaces: []string{"AWS/EBS"}, DimensionNames: []string{"VolumeId"}},
	"ec2":        {Namespaces: []string{"AWS/EC2"}, DimensionNames: []string{"InstanceId"}, Values: ec2AlarmValues},
	"ecs":        {Namespaces: []string{"AWS/ECS", "ECS/ContainerInsights"}, DimensionNames: []string{"ClusterName"}},
	"ecs-svc":    {Namespaces: []string{"AWS/ECS", "ECS/ContainerInsights"}, DimensionNames: []string{"ServiceName"}, QualifierDimension: "ClusterName", QualifierValue: ecsClusterField},
	"ecs-task":   {Namespaces: []string{"ECS/ContainerInsights"}, DimensionNames: []string{"ClusterName", "TaskId"}, QualifierDimension: "ServiceName", QualifierValue: ecsTaskServiceName, Values: ecsTaskAlarmValues},
	"efs":        {Namespaces: []string{"AWS/EFS"}, DimensionNames: []string{"FileSystemId"}},
	"eip":        {Namespaces: []string{"AWS/EC2"}, DimensionNames: []string{"NetworkInterfaceId"}, Values: eipAlarmValues, ValuesFromRawStruct: true},
	"eks":        {Namespaces: []string{"AWS/EKS", "ContainerInsights"}, DimensionNames: []string{"ClusterName"}},
	"elb":        {Namespaces: []string{"AWS/ApplicationELB", "AWS/NetworkELB", "AWS/GatewayELB", "AWS/ELB"}, DimensionNames: []string{"LoadBalancer", "LoadBalancerName"}, Values: elbAlarmValues},
	"glue":       {Namespaces: []string{"Glue"}, DimensionNames: []string{"JobName"}},
	"kinesis":    {Namespaces: []string{"AWS/Kinesis"}, DimensionNames: []string{"StreamName"}},
	"kms":        {Namespaces: []string{"AWS/KMS"}, DimensionNames: []string{"KeyId"}},
	"lambda":     {Namespaces: []string{"AWS/Lambda"}, DimensionNames: []string{"FunctionName"}},
	"logs":       {Namespaces: []string{"AWS/Logs"}, DimensionNames: []string{"LogGroupName"}},
	"msk":        {Namespaces: []string{"AWS/Kafka"}, DimensionNames: []string{"Cluster Name"}},
	"mwaa":       {Namespaces: []string{"AmazonMWAA", "AWS/MWAA"}, DimensionNames: []string{"Environment", "EnvironmentName"}},
	"nat":        {Namespaces: []string{"AWS/NATGateway"}, DimensionNames: []string{"NatGatewayId"}},
	"opensearch": {Namespaces: []string{"AWS/ES"}, DimensionNames: []string{"DomainName"}},
	"redis":      {Namespaces: []string{"AWS/ElastiCache"}, DimensionNames: []string{"CacheClusterId", "ReplicationGroupId"}, Values: redisAlarmValues, ValuesFromRawStruct: true},
	"redshift":   {Namespaces: []string{"AWS/Redshift"}, DimensionNames: []string{"ClusterIdentifier"}},
	"s3":         {Namespaces: []string{"AWS/S3"}, DimensionNames: []string{"BucketName"}},
	"sfn":        {Namespaces: []string{"AWS/States"}, DimensionNames: []string{"StateMachineArn"}, Values: sfnAlarmValues},
	"sns":        {Namespaces: []string{"AWS/SNS"}, DimensionNames: []string{"TopicName"}, ActionService: "sns", Values: snsAlarmValues},
	"sqs":        {Namespaces: []string{"AWS/SQS"}, DimensionNames: []string{"QueueName"}},
	"tg":         {Namespaces: []string{"AWS/ApplicationELB", "AWS/NetworkELB", "AWS/GatewayELB"}, DimensionNames: []string{"TargetGroup"}, Values: tgAlarmValues},
	"vpce":       {Namespaces: []string{"AWS/PrivateLinkEndpoints"}, DimensionNames: []string{"VPC Endpoint Id", "VpcEndpointId"}},
	"waf":        {Namespaces: []string{"AWS/WAFV2"}, DimensionNames: []string{"WebACL"}, MetricsRegion: metricsRegionOfWebACL},
}

// AlarmMatchSpecFor returns how an alarm names a resource of the type, and
// false for a type no alarm pivots to.
func AlarmMatchSpecFor(targetType string) (AlarmMatchSpec, bool) {
	spec, ok := alarmMatchSpecs[targetType]
	return spec, ok
}

func ec2AlarmValues(res resource.Resource) ([]string, bool) {
	instanceID, _, _ := ec2Identity(res)
	return nonEmpty(instanceID), true
}

// elbAlarmValues: ELBv2 metrics carry the load balancer as the part of its
// ARN after "loadbalancer/", classic ELB metrics as its name.
func elbAlarmValues(res resource.Resource) ([]string, bool) {
	elbARN := res.Fields["load_balancer_arn"]
	if raw, ok := assertStruct[elbv2types.LoadBalancer](res.RawStruct); elbARN == "" && ok {
		elbARN = aws.ToString(raw.LoadBalancerArn)
	}
	return nonEmpty(elbv2Dimension(elbARN), res.ID, res.Name), true
}

// tgAlarmValues: ELBv2 metrics carry the target group as the part of its ARN
// from "targetgroup/" on.
func tgAlarmValues(res resource.Resource) ([]string, bool) {
	return nonEmpty(elbv2Dimension(tgARN(res)), res.ID, res.Name), true
}

// eipAlarmValues: an Elastic IP publishes no metric of its own, and the
// traffic through it is metered on the network interface it is associated
// with.
func eipAlarmValues(res resource.Resource) ([]string, bool) {
	raw, ok := assertStruct[ec2types.Address](res.RawStruct)
	if !ok {
		return nil, false
	}
	return nonEmpty(aws.ToString(raw.NetworkInterfaceId)), true
}

func ecsClusterField(res resource.Resource) (string, bool) {
	cluster := res.Fields["cluster"]
	return cluster, cluster != ""
}

// ecsTaskAlarmValues: Container Insights dimensions a task's metrics by the
// cluster it runs in, and by the task's own id in the enhanced set. A task
// carries its cluster as an ARN, while the dimension carries the name.
func ecsTaskAlarmValues(res resource.Resource) ([]string, bool) {
	cluster, _ := ecsClusterField(res)
	return nonEmpty(lastSegment(cluster, "/"), res.ID), true
}

// ecsTaskServiceName is the service a task belongs to, which ECS writes into
// the task's group as "service:<name>"; a task started any other way belongs
// to none.
func ecsTaskServiceName(res resource.Resource) (string, bool) {
	task, ok := assertStruct[ecstypes.Task](res.RawStruct)
	if !ok {
		return "", false
	}
	name, ofService := strings.CutPrefix(aws.ToString(task.Group), "service:")
	if !ofService {
		return "", true
	}
	return name, true
}

// redisAlarmValues: ElastiCache publishes per-node metrics under
// CacheClusterId, so a replication group is named by each of its members as
// well as by its own id.
func redisAlarmValues(res resource.Resource) ([]string, bool) {
	rg, ok := assertStruct[elasticachetypes.ReplicationGroup](res.RawStruct)
	if !ok {
		return nonEmpty(res.ID), true
	}
	return nonEmpty(append([]string{aws.ToString(rg.ReplicationGroupId)}, rg.MemberClusters...)...), true
}

// sfnAlarmValues: Step Functions metrics carry the state machine's ARN,
// while the row is keyed by its name. A row that has not said what its ARN
// is cannot tell whether an ARN dimension names it.
func sfnAlarmValues(res resource.Resource) ([]string, bool) {
	arn := res.Fields["arn"]
	return nonEmpty(arn, res.ID, res.Name), arn != ""
}

// snsAlarmValues: SNS metrics carry the topic's name, while an alarm action
// carries its ARN. A row that has not said what its ARN is cannot tell
// whether an action names it.
func snsAlarmValues(res resource.Resource) ([]string, bool) {
	arn := res.Fields["topic_arn"]
	return nonEmpty(arn, res.ID, res.Name), arn != ""
}
