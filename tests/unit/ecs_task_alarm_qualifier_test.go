package unit_test

// ecs_task_alarm_qualifier_test.go — an ECS task and the alarms over
// the cluster it runs in.
//
// Container Insights dimensions a task's metrics by ClusterName, so every
// task in a cluster is named by that cluster's alarms. An alarm that also
// names a service is about that service's tasks only: a task of another
// service, and a task of no service at all, are not named by it.

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

func clusterAlarmRow(id string, dims ...cwtypes.Dimension) resource.Resource {
	return resource.Resource{ID: id, Name: id, RawStruct: cwtypes.MetricAlarm{
		AlarmName:          aws.String(id),
		Namespace:          aws.String("ECS/ContainerInsights"),
		Dimensions:         dims,
		StateValue:         cwtypes.StateValueOk,
		ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanThreshold,
		Threshold:          aws.Float64(80),
		EvaluationPeriods:  aws.Int32(2),
		ActionsEnabled:     aws.Bool(true),
	}}
}

func ecsTaskRow(id, clusterARN, group string) resource.Resource {
	return resource.Resource{
		ID:     id,
		Name:   id,
		Fields: map[string]string{"cluster": clusterARN},
		RawStruct: ecstypes.Task{
			TaskArn:    aws.String("arn:aws:ecs:us-east-1:123456789012:task/acme-services/" + id),
			ClusterArn: aws.String(clusterARN),
			Group:      aws.String(group),
		},
	}
}

func TestECSTaskShowsItsClustersAlarmsAndOnlyItsOwnServices(t *testing.T) {
	const clusterARN = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-services"
	cluster := clusterAlarmRow("cluster-cpu", cwtypes.Dimension{Name: aws.String("ClusterName"), Value: aws.String("acme-services")})
	service := clusterAlarmRow("api-gateway-running-count",
		cwtypes.Dimension{Name: aws.String("ClusterName"), Value: aws.String("acme-services")},
		cwtypes.Dimension{Name: aws.String("ServiceName"), Value: aws.String("api-gateway")})
	otherCluster := clusterAlarmRow("batch-cpu", cwtypes.Dimension{Name: aws.String("ClusterName"), Value: aws.String("acme-batch")})
	cache := resource.ResourceCache{"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{cluster, service, otherCluster}}}

	cases := []struct {
		name string
		task resource.Resource
		want []string
	}{
		{"a task of the service the alarm names", ecsTaskRow("t-api", clusterARN, "service:api-gateway"), []string{cluster.ID, service.ID}},
		{"a task of another service in the cluster", ecsTaskRow("t-web", clusterARN, "service:web-frontend"), []string{cluster.ID}},
		{"a task of no service", ecsTaskRow("t-oneoff", clusterARN, "family:one-off-job"), []string{cluster.ID}},
		{"a task in another cluster", ecsTaskRow("t-batch", "arn:aws:ecs:us-east-1:123456789012:cluster/acme-batch", "service:api-gateway"), []string{otherCluster.ID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rel2CheckerFor(t, "ecs-task", "alarm")(context.Background(), demo.NewServiceClients(), tc.task, cache).ResourceIDs()
			slices.Sort(got)
			want := slices.Clone(tc.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("ecs-task %s (%s) → alarms = %v, want %v", tc.task.ID, aws.ToString(tc.task.RawStruct.(ecstypes.Task).Group), got, want)
			}
		})
	}
}
