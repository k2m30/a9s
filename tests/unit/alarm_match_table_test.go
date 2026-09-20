package unit_test

// alarm_match_table_test.go — alarm↔resource matching from one table.
//
// A CloudWatch alarm names what it watches in three places: the top-level
// Namespace/Dimensions of a single-metric alarm, the MetricStat of every
// query in Metrics[] of a metric-math or anomaly-detection alarm (whose
// top-level Namespace and Dimensions are empty), and — for Auto Scaling and
// SNS — its action ARNs. A dimension name alone is ambiguous: ClusterName is
// both an ECS and an EKS dimension, LoadBalancer is both an ALB and an NLB
// one, so a match is (namespace, dimension name, value). Both directions of
// every alarm pivot read that one table, so a resource and an alarm always
// agree on whether they belong together.

import (
	"context"
	"maps"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

const alarmMatchUnrelatedNamespace = "Custom/Unrelated"

func alarmMatchResource(id string, a cwtypes.MetricAlarm) resource.Resource {
	a.AlarmName = aws.String(id)
	a.AlarmArn = aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:" + id)
	return resource.Resource{
		ID:        id,
		Name:      id,
		Fields:    map[string]string{"alarm_name": id},
		RawStruct: a,
	}
}

func alarmMatchStat(id, namespace, metric string, dims ...cwtypes.Dimension) cwtypes.MetricDataQuery {
	return cwtypes.MetricDataQuery{
		Id: aws.String(id),
		MetricStat: &cwtypes.MetricStat{
			Metric: &cwtypes.Metric{
				Namespace:  aws.String(namespace),
				MetricName: aws.String(metric),
				Dimensions: dims,
			},
			Period: aws.Int32(60),
			Stat:   aws.String("Sum"),
		},
		ReturnData: aws.Bool(false),
	}
}

func alarmMatchExpr(id, expr string) cwtypes.MetricDataQuery {
	return cwtypes.MetricDataQuery{Id: aws.String(id), Expression: aws.String(expr), ReturnData: aws.Bool(true)}
}

func alarmMatchDim(name, value string) cwtypes.Dimension {
	return cwtypes.Dimension{Name: aws.String(name), Value: aws.String(value)}
}

// alarmMatchMetricMath is the shape DescribeAlarms returns for a metric-math
// alarm: no top-level Namespace, MetricName or Dimensions, only Metrics[].
func alarmMatchMetricMath(namespace, dimName, dimValue string) cwtypes.MetricAlarm {
	return cwtypes.MetricAlarm{
		StateValue:         cwtypes.StateValueOk,
		ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanThreshold,
		Threshold:          aws.Float64(5),
		EvaluationPeriods:  aws.Int32(3),
		ActionsEnabled:     aws.Bool(true),
		Metrics: []cwtypes.MetricDataQuery{
			alarmMatchStat("m1", namespace, "HTTPCode_Target_5XX_Count", alarmMatchDim(dimName, dimValue)),
			alarmMatchStat("m2", namespace, "RequestCount", alarmMatchDim(dimName, dimValue)),
			alarmMatchExpr("e1", "m1/m2*100"),
		},
	}
}

// alarmMatchAnomaly is the shape of an anomaly-detection alarm: one metric
// query, one ANOMALY_DETECTION_BAND expression, ThresholdMetricId set.
func alarmMatchAnomaly(namespace, metric, dimName, dimValue string) cwtypes.MetricAlarm {
	stat := alarmMatchStat("m1", namespace, metric, alarmMatchDim(dimName, dimValue))
	stat.ReturnData = aws.Bool(true)
	return cwtypes.MetricAlarm{
		StateValue:         cwtypes.StateValueOk,
		ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanUpperThreshold,
		EvaluationPeriods:  aws.Int32(2),
		ThresholdMetricId:  aws.String("ad1"),
		ActionsEnabled:     aws.Bool(true),
		Metrics:            []cwtypes.MetricDataQuery{stat, alarmMatchExpr("ad1", "ANOMALY_DETECTION_BAND(m1, 2)")},
	}
}

func alarmMatchSingle(namespace, metric string, dims ...cwtypes.Dimension) cwtypes.MetricAlarm {
	return cwtypes.MetricAlarm{
		Namespace:          aws.String(namespace),
		MetricName:         aws.String(metric),
		Dimensions:         dims,
		Statistic:          cwtypes.StatisticAverage,
		Period:             aws.Int32(300),
		StateValue:         cwtypes.StateValueOk,
		ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanThreshold,
		Threshold:          aws.Float64(80),
		EvaluationPeriods:  aws.Int32(2),
		ActionsEnabled:     aws.Bool(true),
	}
}

func alarmMatchDemoRow(t *testing.T, shortName string, match func(resource.Resource) bool) resource.Resource {
	t.Helper()
	for _, r := range rel2DemoList(t, shortName) {
		if match(r) {
			return r
		}
	}
	t.Fatalf("no demo %s row matches", shortName)
	return resource.Resource{}
}

func alarmMatchByName(name string) func(resource.Resource) bool {
	return func(r resource.Resource) bool { return r.ID == name || r.Name == name }
}

// alarmMatchCheck runs the registered source→target checker and returns the
// IDs it resolved.
func alarmMatchCheck(t *testing.T, source, target string, row resource.Resource, cache resource.ResourceCache) []string {
	t.Helper()
	return rel2CheckerFor(t, source, target)(context.Background(), demo.NewServiceClients(), row, cache).ResourceIDs()
}

func alarmMatchCacheWith(t *testing.T, alarms []resource.Resource, shortNames ...string) resource.ResourceCache {
	t.Helper()
	cache := rel2DemoCacheFor(t, shortNames...)
	cache["alarm"] = resource.ResourceCacheEntry{Resources: alarms}
	return cache
}

// alarmMatchRelocated moves every namespace the alarm names — top-level and
// per metric query — to one no pivot declares, and drops its actions, so the
// only thing it still shares with a resource is a dimension name and value.
func alarmMatchRelocated(a cwtypes.MetricAlarm) cwtypes.MetricAlarm {
	if a.Namespace != nil {
		a.Namespace = aws.String(alarmMatchUnrelatedNamespace)
	}
	metrics := make([]cwtypes.MetricDataQuery, len(a.Metrics))
	for i, q := range a.Metrics {
		if q.MetricStat != nil && q.MetricStat.Metric != nil {
			stat := *q.MetricStat
			metric := *stat.Metric
			metric.Namespace = aws.String(alarmMatchUnrelatedNamespace)
			stat.Metric = &metric
			q.MetricStat = &stat
		}
		metrics[i] = q
	}
	a.Metrics = metrics
	a.AlarmActions, a.OKActions, a.InsufficientDataActions = nil, nil, nil
	return a
}

// alarmMatchForwardTypes lists every type with a resource→alarm pivot.
func alarmMatchForwardTypes() []string {
	var out []string
	for _, name := range resource.AllShortNames() {
		if alarmMatchRegistered(name, "alarm") {
			out = append(out, name)
		}
	}
	return out
}

func alarmMatchRegistered(source, target string) bool {
	for _, def := range resource.GetRelated(source) {
		if def.TargetType == target && def.Checker != nil {
			return true
		}
	}
	return false
}

// alarmMatchReverseTypes lists every type an alarm pivots to by what it
// watches; ct-events is the alarm's own API history, not a watched resource.
func alarmMatchReverseTypes() []string {
	var out []string
	for _, def := range resource.GetRelated("alarm") {
		if def.TargetType != "ct-events" && def.Checker != nil {
			out = append(out, def.TargetType)
		}
	}
	return out
}

// --- AlarmDimensions reads every place an alarm names a metric ------------

func TestAlarmDimensionsReadsEveryMetricTheAlarmWatches(t *testing.T) {
	const lb = "app/acme-prod-web/1234567890abcdef"
	crossNamespace := cwtypes.MetricAlarm{
		Metrics: []cwtypes.MetricDataQuery{
			alarmMatchStat("m1", "AWS/SQS", "ApproximateNumberOfMessagesVisible", alarmMatchDim("QueueName", "order-processing-queue")),
			alarmMatchStat("m2", "AWS/Lambda", "ConcurrentExecutions", alarmMatchDim("FunctionName", "order-worker")),
			alarmMatchExpr("e1", "m1/m2"),
		},
	}
	nilParts := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/EC2"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("InstanceId")},
			{Value: aws.String("i-0a1b2c3d4e5f60001")},
			alarmMatchDim("InstanceId", "i-0a1b2c3d4e5f60002"),
		},
		Metrics: []cwtypes.MetricDataQuery{
			{Id: aws.String("m1"), MetricStat: &cwtypes.MetricStat{}},
			{Id: aws.String("m2")},
			alarmMatchExpr("e1", "SEARCH('{AWS/EC2,InstanceId} CPUUtilization', 'Average', 300)"),
		},
	}

	cases := []struct {
		name  string
		alarm cwtypes.MetricAlarm
		want  []awsclient.AlarmDimension
	}{
		{
			"single-metric alarm keeps its top-level namespace",
			alarmMatchSingle("AWS/SQS", "ApproximateAgeOfOldestMessage", alarmMatchDim("QueueName", "order-processing-queue")),
			[]awsclient.AlarmDimension{{Namespace: "AWS/SQS", Name: "QueueName", Value: "order-processing-queue"}},
		},
		{
			"metric-math alarm names its load balancer only inside Metrics",
			alarmMatchMetricMath("AWS/ApplicationELB", "LoadBalancer", lb),
			[]awsclient.AlarmDimension{{Namespace: "AWS/ApplicationELB", Name: "LoadBalancer", Value: lb}},
		},
		{
			"anomaly-detection alarm names its instance only inside Metrics",
			alarmMatchAnomaly("AWS/RDS", "CPUUtilization", "DBInstanceIdentifier", "prod-api-primary"),
			[]awsclient.AlarmDimension{{Namespace: "AWS/RDS", Name: "DBInstanceIdentifier", Value: "prod-api-primary"}},
		},
		{
			"each metric query keeps its own namespace",
			crossNamespace,
			[]awsclient.AlarmDimension{
				{Namespace: "AWS/SQS", Name: "QueueName", Value: "order-processing-queue"},
				{Namespace: "AWS/Lambda", Name: "FunctionName", Value: "order-worker"},
			},
		},
		{
			"a dimension missing its name or value, and a query with no metric, name nothing",
			nilParts,
			[]awsclient.AlarmDimension{{Namespace: "AWS/EC2", Name: "InstanceId", Value: "i-0a1b2c3d4e5f60002"}},
		},
		{"an empty alarm names nothing", cwtypes.MetricAlarm{}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := map[awsclient.AlarmDimension]bool{}
			for _, d := range awsclient.AlarmDimensions(tc.alarm) {
				got[d] = true
			}
			want := map[awsclient.AlarmDimension]bool{}
			for _, d := range tc.want {
				want[d] = true
			}
			if !maps.Equal(got, want) {
				t.Errorf("AlarmDimensions = %v, want %v", got, want)
			}
		})
	}
}

// The witness: a 5xx-ratio alarm over an ALB is a metric-math alarm, and it
// is the alarm an operator most wants to see on the load balancer.
func TestMetricMathAlarmAppearsOnItsLoadBalancer(t *testing.T) {
	elb := alarmMatchDemoRow(t, "elb", func(r resource.Resource) bool {
		return strings.HasSuffix(r.Fields["load_balancer_arn"], "loadbalancer/app/acme-prod-web/1234567890abcdef")
	})
	const lb = "app/acme-prod-web/1234567890abcdef"
	ratio := alarmMatchResource("acme-prod-web-5xx-ratio", alarmMatchMetricMath("AWS/ApplicationELB", "LoadBalancer", lb))
	other := alarmMatchResource("acme-other-web-5xx-ratio", alarmMatchMetricMath("AWS/ApplicationELB", "LoadBalancer", "app/acme-other-web/fedcba0987654321"))
	cache := alarmMatchCacheWith(t, []resource.Resource{ratio, other}, "elb")

	if got := alarmMatchCheck(t, "elb", "alarm", elb, cache); !slices.Equal(got, []string{ratio.ID}) {
		t.Errorf("elb %s → alarms = %v, want [%s]: the ratio alarm watches this load balancer through m1/m2", elb.ID, got, ratio.ID)
	}
	// The reverse half is a claim only where an alarm→elb pivot is registered.
	if !alarmMatchRegistered("alarm", "elb") {
		return
	}
	if got := alarmMatchCheck(t, "alarm", "elb", ratio, cache); !slices.Equal(got, []string{elb.ID}) {
		t.Errorf("alarm %s → elb = %v, want [%s]", ratio.ID, got, elb.ID)
	}
	if got := alarmMatchCheck(t, "alarm", "elb", other, cache); len(got) != 0 {
		t.Errorf("alarm %s → elb = %v, want none: it watches a load balancer that is not in the list", other.ID, got)
	}
}

func TestAnomalyDetectionAlarmAppearsOnItsInstanceBothWays(t *testing.T) {
	dbi := alarmMatchDemoRow(t, "dbi", alarmMatchByName("prod-api-primary"))
	band := alarmMatchResource("prod-api-primary-cpu-anomaly", alarmMatchAnomaly("AWS/RDS", "CPUUtilization", "DBInstanceIdentifier", "prod-api-primary"))
	cache := alarmMatchCacheWith(t, []resource.Resource{band}, "dbi")

	if got := alarmMatchCheck(t, "dbi", "alarm", dbi, cache); !slices.Equal(got, []string{band.ID}) {
		t.Errorf("dbi %s → alarms = %v, want [%s]", dbi.ID, got, band.ID)
	}
	if got := alarmMatchCheck(t, "alarm", "dbi", band, cache); !slices.Equal(got, []string{dbi.ID}) {
		t.Errorf("alarm %s → dbi = %v, want [%s]", band.ID, got, dbi.ID)
	}
}

// --- every resource→alarm pivot reads AlarmMatchSpecFor -------------------

func TestAlarmMatchSpecCoversEveryAlarmPivot(t *testing.T) {
	types := append(alarmMatchForwardTypes(), alarmMatchReverseTypes()...)
	for _, name := range types {
		spec, ok := awsclient.AlarmMatchSpecFor(name)
		if !ok {
			t.Errorf("%s has an alarm pivot but no AlarmMatchSpecFor entry", name)
			continue
		}
		if len(spec.Namespaces) == 0 || len(spec.DimensionNames) == 0 {
			t.Errorf("%s: spec %+v — a match needs a namespace and a dimension name", name, spec)
		}
	}
	if spec, ok := awsclient.AlarmMatchSpecFor("vpc"); ok {
		t.Errorf("vpc has no alarm pivot, yet AlarmMatchSpecFor returned %+v", spec)
	}
}

// The namespace and dimension each AWS service publishes its per-resource
// metrics under, from the service's CloudWatch metrics documentation.
func TestAlarmMatchSpecNamesTheServicesOwnMetrics(t *testing.T) {
	cases := []struct{ shortName, namespace, dimension string }{
		{"ec2", "AWS/EC2", "InstanceId"},
		{"ebs", "AWS/EBS", "VolumeId"},
		{"asg", "AWS/AutoScaling", "AutoScalingGroupName"},
		{"lambda", "AWS/Lambda", "FunctionName"},
		{"ecs", "AWS/ECS", "ClusterName"},
		{"ecs-svc", "AWS/ECS", "ServiceName"},
		{"eks", "AWS/EKS", "ClusterName"},
		{"dbi", "AWS/RDS", "DBInstanceIdentifier"},
		{"redis", "AWS/ElastiCache", "CacheClusterId"},
		{"ddb", "AWS/DynamoDB", "TableName"},
		{"opensearch", "AWS/ES", "DomainName"},
		{"redshift", "AWS/Redshift", "ClusterIdentifier"},
		{"efs", "AWS/EFS", "FileSystemId"},
		{"cb", "AWS/CodeBuild", "ProjectName"},
		{"logs", "AWS/Logs", "LogGroupName"},
		{"waf", "AWS/WAFV2", "WebACL"},
		{"elb", "AWS/ApplicationELB", "LoadBalancer"},
		{"tg", "AWS/ApplicationELB", "TargetGroup"},
		{"nat", "AWS/NATGateway", "NatGatewayId"},
		// PrivateLink publishes the endpoint dimension with spaces; the
		// unspaced spelling is the legacy one for the same endpoint id.
		{"vpce", "AWS/PrivateLinkEndpoints", "VPC Endpoint Id"},
		{"vpce", "AWS/PrivateLinkEndpoints", "VpcEndpointId"},
		{"cf", "AWS/CloudFront", "DistributionId"},
		// HTTP and WebSocket APIs publish ApiId; REST APIs publish ApiName.
		{"apigw", "AWS/ApiGateway", "ApiId"},
		{"apigw", "AWS/ApiGateway", "ApiName"},
		{"glue", "Glue", "JobName"},
		{"sqs", "AWS/SQS", "QueueName"},
		{"sns", "AWS/SNS", "TopicName"},
		{"eb", "AWS/ElasticBeanstalk", "EnvironmentName"},
		// The worker daemon publishes its Health metric in its own namespace.
		{"eb", "ElasticBeanstalk/SQSD", "EnvironmentName"},
		// Airflow's own metrics carry Environment under AmazonMWAA;
		// AWS/MWAA documents only Cluster, Queue and Database.
		{"mwaa", "AmazonMWAA", "Environment"},
		{"kinesis", "AWS/Kinesis", "StreamName"},
		{"msk", "AWS/Kafka", "Cluster Name"},
		{"sfn", "AWS/States", "StateMachineArn"},
		{"kms", "AWS/KMS", "KeyId"},
		{"s3", "AWS/S3", "BucketName"},
	}
	for _, tc := range cases {
		spec, ok := awsclient.AlarmMatchSpecFor(tc.shortName)
		if !ok {
			t.Errorf("%s: no AlarmMatchSpecFor entry", tc.shortName)
			continue
		}
		if !slices.Contains(spec.Namespaces, tc.namespace) {
			t.Errorf("%s: Namespaces %v lack %s", tc.shortName, spec.Namespaces, tc.namespace)
		}
		if !slices.Contains(spec.DimensionNames, tc.dimension) {
			t.Errorf("%s: DimensionNames %v lack %q", tc.shortName, spec.DimensionNames, tc.dimension)
		}
	}

	// ClusterName is shared by ECS and EKS; only the namespace tells them apart.
	ecs, _ := awsclient.AlarmMatchSpecFor("ecs")
	for _, ns := range []string{"AWS/EKS", "ContainerInsights"} {
		if slices.Contains(ecs.Namespaces, ns) {
			t.Errorf("ecs Namespaces %v include the EKS namespace %s", ecs.Namespaces, ns)
		}
	}
	eks, _ := awsclient.AlarmMatchSpecFor("eks")
	for _, ns := range []string{"AWS/ECS", "ECS/ContainerInsights"} {
		if slices.Contains(eks.Namespaces, ns) {
			t.Errorf("eks Namespaces %v include the ECS namespace %s", eks.Namespaces, ns)
		}
	}
}

// A type whose entry carries more than one namespace or more than one
// spelling of its dimension is reachable through each of them: Airflow's own
// metrics carry Environment under AmazonMWAA while AWS/MWAA documents only
// Cluster, Queue and Database; PrivateLink's endpoint dimension has a legacy
// unspaced spelling of the same endpoint id; the Elastic Beanstalk worker
// daemon publishes Health under ElasticBeanstalk/SQSD.
func TestAlarmMatchesOnEachDeclaredNamespaceAndSpelling(t *testing.T) {
	cases := []struct {
		shortName, namespace, dimension, metric, value, other string
	}{
		{"mwaa", "AmazonMWAA", "Environment", "TaskInstanceFailures", "prod-airflow-etl", "prod-airflow-reporting"},
		{"vpce", "AWS/PrivateLinkEndpoints", "VpcEndpointId", "PacketsDropped", "vpce-0aaa111111111111a", "vpce-0bbb222222222222b"},
		{"eb", "ElasticBeanstalk/SQSD", "EnvironmentName", "Health", "acme-prod-api", "acme-prod-web"},
	}
	for _, tc := range cases {
		t.Run(tc.shortName+"/"+tc.namespace+"/"+tc.dimension, func(t *testing.T) {
			row := alarmMatchDemoRow(t, tc.shortName, alarmMatchByName(tc.value))
			other := alarmMatchDemoRow(t, tc.shortName, alarmMatchByName(tc.other))
			mine := alarmMatchResource(tc.value+"-"+tc.metric, alarmMatchSingle(tc.namespace, tc.metric, alarmMatchDim(tc.dimension, tc.value)))
			theirs := alarmMatchResource(tc.other+"-"+tc.metric, alarmMatchSingle(tc.namespace, tc.metric, alarmMatchDim(tc.dimension, tc.other)))
			cache := alarmMatchCacheWith(t, []resource.Resource{mine, theirs}, tc.shortName)

			if got := alarmMatchCheck(t, tc.shortName, "alarm", row, cache); !slices.Equal(got, []string{mine.ID}) {
				t.Errorf("%s %s → alarms = %v, want [%s]", tc.shortName, row.ID, got, mine.ID)
			}
			if got := alarmMatchCheck(t, tc.shortName, "alarm", other, cache); !slices.Equal(got, []string{theirs.ID}) {
				t.Errorf("%s %s → alarms = %v, want [%s]", tc.shortName, other.ID, got, theirs.ID)
			}
			if !alarmMatchRegistered("alarm", tc.shortName) {
				return
			}
			if got := alarmMatchCheck(t, "alarm", tc.shortName, mine, cache); !slices.Equal(got, []string{row.ID}) {
				t.Errorf("alarm %s → %s = %v, want [%s]", mine.ID, tc.shortName, got, row.ID)
			}
		})
	}
}

// The witness: an EKS cluster and an ECS cluster may share a name, and an
// alarm on the EKS control plane is not an alarm on the ECS cluster.
func TestECSClusterDoesNotShowAnEKSAlarmOnTheSameName(t *testing.T) {
	ecs := alarmMatchDemoRow(t, "ecs", alarmMatchByName("acme-services"))
	eks := alarmMatchDemoRow(t, "eks", alarmMatchByName("acme-prod"))
	onECS := alarmMatchResource("acme-services-cpu", alarmMatchSingle("AWS/ECS", "CPUUtilization", alarmMatchDim("ClusterName", ecs.Name)))
	eksNamedLikeECS := alarmMatchResource("acme-services-eks-apiserver", alarmMatchSingle("AWS/EKS", "apiserver_request_total", alarmMatchDim("ClusterName", ecs.Name)))
	onEKS := alarmMatchResource("acme-prod-eks-apiserver", alarmMatchSingle("AWS/EKS", "apiserver_request_total", alarmMatchDim("ClusterName", eks.Name)))
	ecsNamedLikeEKS := alarmMatchResource("acme-prod-ecs-cpu", alarmMatchSingle("AWS/ECS", "CPUUtilization", alarmMatchDim("ClusterName", eks.Name)))
	cache := alarmMatchCacheWith(t, []resource.Resource{onECS, eksNamedLikeECS, onEKS, ecsNamedLikeEKS}, "ecs", "eks")

	if got := alarmMatchCheck(t, "ecs", "alarm", ecs, cache); !slices.Equal(got, []string{onECS.ID}) {
		t.Errorf("ecs %s → alarms = %v, want [%s]", ecs.ID, got, onECS.ID)
	}
	if got := alarmMatchCheck(t, "eks", "alarm", eks, cache); !slices.Equal(got, []string{onEKS.ID}) {
		t.Errorf("eks %s → alarms = %v, want [%s]", eks.ID, got, onEKS.ID)
	}
	if got := alarmMatchCheck(t, "alarm", "ecs", eksNamedLikeECS, cache); len(got) != 0 {
		t.Errorf("alarm %s → ecs = %v, want none", eksNamedLikeECS.ID, got)
	}
	if got := alarmMatchCheck(t, "alarm", "eks", eksNamedLikeECS, cache); len(got) != 0 {
		t.Errorf("alarm %s → eks = %v, want none: no EKS cluster has that name", eksNamedLikeECS.ID, got)
	}
	if got := alarmMatchCheck(t, "alarm", "eks", onEKS, cache); !slices.Equal(got, []string{eks.ID}) {
		t.Errorf("alarm %s → eks = %v, want [%s]", onEKS.ID, got, eks.ID)
	}
	if got := alarmMatchCheck(t, "alarm", "ecs", ecsNamedLikeEKS, cache); len(got) != 0 {
		t.Errorf("alarm %s → ecs = %v, want none: no ECS cluster has that name", ecsNamedLikeEKS.ID, got)
	}
}

// Every demo alarm, moved to a namespace no pivot declares and stripped of
// its actions, still carries the dimension its resource is found by. No
// resource may claim it: a dimension name without its namespace is a guess.
func TestAlarmOutsideTheSpecNamespacesMatchesNoResource(t *testing.T) {
	var relocated []resource.Resource
	for _, a := range rel2DemoList(t, "alarm") {
		raw, ok := a.RawStruct.(cwtypes.MetricAlarm)
		if !ok {
			t.Fatalf("demo alarm %s RawStruct is %T", a.ID, a.RawStruct)
		}
		relocated = append(relocated, alarmMatchResource(a.ID, alarmMatchRelocated(raw)))
	}
	forward := alarmMatchForwardTypes()
	reverse := alarmMatchReverseTypes()
	cache := alarmMatchCacheWith(t, relocated, append(slices.Clone(forward), reverse...)...)

	for _, source := range forward {
		t.Run(source+"→alarm", func(t *testing.T) {
			for _, row := range cache[source].Resources {
				if got := alarmMatchCheck(t, source, "alarm", row, cache); len(got) != 0 {
					t.Errorf("%s %s → alarms %v", source, row.ID, got)
				}
			}
		})
	}
	for _, target := range reverse {
		t.Run("alarm→"+target, func(t *testing.T) {
			for _, a := range relocated {
				if got := alarmMatchCheck(t, "alarm", target, a, cache); len(got) != 0 {
					t.Errorf("alarm %s → %s %v", a.ID, target, got)
				}
			}
		})
	}
}

// --- alarm→resource reads the same table -----------------------------------

// A REST API publishes ApiName, an HTTP API publishes ApiId; the list row
// is the API's id in both cases, so both directions resolve to it.
func TestAPIGatewayAlarmMatchesByApiIdAndApiNameBothWays(t *testing.T) {
	httpAPI := alarmMatchDemoRow(t, "apigw", alarmMatchByName("acme-public-api"))
	restAPI := alarmMatchDemoRow(t, "apigw", alarmMatchByName("acme-orders-rest"))
	byID := alarmMatchResource("acme-public-api-5xx", alarmMatchSingle("AWS/ApiGateway", "5xx", alarmMatchDim("ApiId", httpAPI.ID)))
	byName := alarmMatchResource("acme-orders-rest-5xx", alarmMatchSingle("AWS/ApiGateway", "5XXError", alarmMatchDim("ApiName", restAPI.Name), alarmMatchDim("Stage", "prod")))
	wrongNS := alarmMatchResource("acme-orders-rest-custom", alarmMatchSingle(alarmMatchUnrelatedNamespace, "5XXError", alarmMatchDim("ApiName", restAPI.Name)))
	cache := alarmMatchCacheWith(t, []resource.Resource{byID, byName, wrongNS}, "apigw")

	if restAPI.ID == restAPI.Name {
		t.Fatalf("REST row %s: ID equals Name, so the case cannot tell a name from an id", restAPI.ID)
	}
	if got := alarmMatchCheck(t, "apigw", "alarm", httpAPI, cache); !slices.Equal(got, []string{byID.ID}) {
		t.Errorf("apigw %s → alarms = %v, want [%s]", httpAPI.ID, got, byID.ID)
	}
	if got := alarmMatchCheck(t, "apigw", "alarm", restAPI, cache); !slices.Equal(got, []string{byName.ID}) {
		t.Errorf("apigw %s (%s) → alarms = %v, want [%s]", restAPI.ID, restAPI.Name, got, byName.ID)
	}
	if got := alarmMatchCheck(t, "alarm", "apigw", byID, cache); !slices.Equal(got, []string{httpAPI.ID}) {
		t.Errorf("alarm %s → apigw = %v, want [%s]", byID.ID, got, httpAPI.ID)
	}
	if got := alarmMatchCheck(t, "alarm", "apigw", byName, cache); !slices.Equal(got, []string{restAPI.ID}) {
		t.Errorf("alarm %s → apigw = %v, want the row id [%s], not the API name", byName.ID, got, restAPI.ID)
	}
	if got := alarmMatchCheck(t, "alarm", "apigw", wrongNS, cache); len(got) != 0 {
		t.Errorf("alarm %s → apigw = %v, want none", wrongNS.ID, got)
	}
}

// --- asg and sns also match through alarm actions --------------------------

// A step-scaling policy is usually driven by a metric of the workload — here
// queue depth — so the alarm carries no AutoScalingGroupName dimension; its
// only link to the group is the scaling-policy ARN in AlarmActions.
func TestScalingAlarmWithoutASGDimensionAppearsOnTheGroup(t *testing.T) {
	asg := alarmMatchDemoRow(t, "asg", alarmMatchByName("acme-web-prod-asg"))
	other := alarmMatchDemoRow(t, "asg", alarmMatchByName("acme-worker-batch-asg"))
	policyARN := func(group string) string {
		return "arn:aws:autoscaling:us-east-1:123456789012:scalingPolicy:1a2b3c4d-5678-90ab-cdef-111111111111:autoScalingGroupName/" + group + ":policyName/" + group + "-scale-out"
	}
	scaling := alarmMatchSingle("AWS/SQS", "ApproximateNumberOfMessagesVisible", alarmMatchDim("QueueName", "order-processing-queue"))
	scaling.AlarmActions = []string{policyARN(asg.Name)}
	scaleOut := alarmMatchResource("acme-web-prod-queue-depth-scale-out", scaling)
	cache := alarmMatchCacheWith(t, []resource.Resource{scaleOut}, "asg")

	if got := alarmMatchCheck(t, "asg", "alarm", asg, cache); !slices.Equal(got, []string{scaleOut.ID}) {
		t.Errorf("asg %s → alarms = %v, want [%s]: the alarm drives this group's scaling policy", asg.ID, got, scaleOut.ID)
	}
	if got := alarmMatchCheck(t, "asg", "alarm", other, cache); len(got) != 0 {
		t.Errorf("asg %s → alarms = %v, want none: the policy belongs to another group", other.ID, got)
	}
	if got := alarmMatchCheck(t, "alarm", "asg", scaleOut, cache); !slices.Equal(got, []string{asg.ID}) {
		t.Errorf("alarm %s → asg = %v, want [%s]", scaleOut.ID, got, asg.ID)
	}
}

// SNS publishes per-topic metrics under TopicName, and an alarm on a topic's
// failed deliveries often notifies a different topic. The watched topic is
// found through the dimension, the notified one through the action.
func TestSNSTopicMatchesByDimensionAndByAction(t *testing.T) {
	topics := rel2DemoList(t, "sns")
	if len(topics) < 2 {
		t.Fatalf("need two demo topics, have %d", len(topics))
	}
	watched, notified := topics[0], topics[1]
	failures := alarmMatchSingle("AWS/SNS", "NumberOfNotificationsFailed", alarmMatchDim("TopicName", watched.Name))
	failures.AlarmActions = []string{notified.Fields["topic_arn"]}
	alarm := alarmMatchResource("sns-delivery-failures", failures)
	customNS := alarmMatchResource("sns-custom-topic-metric", alarmMatchSingle(alarmMatchUnrelatedNamespace, "Published", alarmMatchDim("TopicName", watched.Name)))
	cache := alarmMatchCacheWith(t, []resource.Resource{alarm, customNS}, "sns")

	for _, topic := range []resource.Resource{watched, notified} {
		if got := alarmMatchCheck(t, "sns", "alarm", topic, cache); !slices.Equal(got, []string{alarm.ID}) {
			t.Errorf("sns %s → alarms = %v, want [%s]", topic.ID, got, alarm.ID)
		}
	}
	got := alarmMatchCheck(t, "alarm", "sns", alarm, cache)
	sort.Strings(got)
	want := []string{watched.ID, notified.ID}
	sort.Strings(want)
	if !slices.Equal(got, want) {
		t.Errorf("alarm %s → sns = %v, want the row ids %v", alarm.ID, got, want)
	}
	if got := alarmMatchCheck(t, "alarm", "sns", customNS, cache); len(got) != 0 {
		t.Errorf("alarm %s → sns = %v, want none", customNS.ID, got)
	}
}

// --- both directions give the same pairs on the demo -----------------------

// Only types with both a resource→alarm and an alarm→resource pivot are
// compared: a type with one direction has nothing to be symmetric with.
func TestAlarmPivotsAreSymmetricOnTheDemo(t *testing.T) {
	var names []string
	for _, name := range alarmMatchReverseTypes() {
		if alarmMatchRegistered(name, "alarm") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatal("no type has an alarm pivot in both directions; the comparison proves nothing")
	}
	cache := rel2DemoCacheFor(t, append(names, "alarm")...)
	clients := demo.NewServiceClients()

	pairs := func(direction string) map[string]bool {
		out := map[string]bool{}
		for _, name := range names {
			if direction == "forward" {
				for _, def := range resource.GetRelated(name) {
					if def.TargetType != "alarm" || def.Checker == nil {
						continue
					}
					for _, row := range cache[name].Resources {
						for _, id := range def.Checker(context.Background(), clients, row, cache).ResourceIDs() {
							out[name+" "+row.ID+" ⇄ alarm "+id] = true
						}
					}
				}
				continue
			}
			for _, def := range resource.GetRelated("alarm") {
				if def.TargetType != name || def.Checker == nil {
					continue
				}
				for _, a := range cache["alarm"].Resources {
					for _, id := range def.Checker(context.Background(), clients, a, cache).ResourceIDs() {
						out[name+" "+id+" ⇄ alarm "+a.ID] = true
					}
				}
			}
		}
		return out
	}
	fwd, rev := pairs("forward"), pairs("reverse")
	if len(fwd) == 0 {
		t.Fatal("no resource→alarm pair on the demo; the comparison proves nothing")
	}
	var onlyFwd, onlyRev []string
	for p := range fwd {
		if !rev[p] {
			onlyFwd = append(onlyFwd, p)
		}
	}
	for p := range rev {
		if !fwd[p] {
			onlyRev = append(onlyRev, p)
		}
	}
	sort.Strings(onlyFwd)
	sort.Strings(onlyRev)
	if len(onlyFwd) > 0 {
		t.Errorf("%d pairs the resource shows and the alarm does not:\n  %s", len(onlyFwd), strings.Join(onlyFwd, "\n  "))
	}
	if len(onlyRev) > 0 {
		t.Errorf("%d pairs the alarm shows and the resource does not:\n  %s", len(onlyRev), strings.Join(onlyRev, "\n  "))
	}
}
