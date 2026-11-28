package unit_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
)

const (
	t9cClusterARN     = "arn:aws:ecs:us-east-1:123456789012:cluster/acme-prod"
	t9cRunningTaskID  = "3f9c2a7e5b1d4c8a9e6f0b2d4a6c8e1f"
	t9cStoppedTaskID  = "8b2d4f6a1c3e5a7b9d0f2e4c6a8b0d1e"
	t9cTaskDefinition = "arn:aws:ecs:us-east-1:123456789012:task-definition/orders-api:42"
)

// t9cECS answers as ECS does: DescribeClusters returns a cluster's settings,
// configuration and tags only when Include names them, and ListTasks returns
// the tasks whose desired status is the one asked for, RUNNING when none is.
type t9cECS struct {
	awsclient.ECSAPI
	describeClusters []*ecs.DescribeClustersInput
	listTasks        []*ecs.ListTasksInput
}

func (f *t9cECS) ListClusters(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return &ecs.ListClustersOutput{ClusterArns: []string{t9cClusterARN}}, nil
}

func (f *t9cECS) DescribeClusters(_ context.Context, in *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	f.describeClusters = append(f.describeClusters, in)
	if !slices.Contains(in.Clusters, t9cClusterARN) && !slices.Contains(in.Clusters, "acme-prod") {
		return &ecs.DescribeClustersOutput{}, nil
	}
	c := ecstypes.Cluster{
		ClusterArn:                        aws.String(t9cClusterARN),
		ClusterName:                       aws.String("acme-prod"),
		Status:                            aws.String("ACTIVE"),
		RunningTasksCount:                 1,
		ActiveServicesCount:               1,
		RegisteredContainerInstancesCount: 0,
	}
	if slices.Contains(in.Include, ecstypes.ClusterFieldSettings) {
		c.Settings = []ecstypes.ClusterSetting{{Name: ecstypes.ClusterSettingNameContainerInsights, Value: aws.String("enhanced")}}
	}
	if slices.Contains(in.Include, ecstypes.ClusterFieldTags) {
		c.Tags = []ecstypes.Tag{{Key: aws.String("team"), Value: aws.String("orders")}}
	}
	return &ecs.DescribeClustersOutput{Clusters: []ecstypes.Cluster{c}}, nil
}

func (f *t9cECS) ListTasks(_ context.Context, in *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	f.listTasks = append(f.listTasks, in)
	switch in.DesiredStatus {
	case "", ecstypes.DesiredStatusRunning:
		return &ecs.ListTasksOutput{TaskArns: []string{t9cTaskARN(t9cRunningTaskID)}}, nil
	case ecstypes.DesiredStatusStopped:
		return &ecs.ListTasksOutput{TaskArns: []string{t9cTaskARN(t9cStoppedTaskID)}}, nil
	}
	return &ecs.ListTasksOutput{}, nil
}

func t9cTaskARN(id string) string {
	return "arn:aws:ecs:us-east-1:123456789012:task/acme-prod/" + id
}

func (f *t9cECS) DescribeTasks(_ context.Context, in *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	started := time.Now().Add(-2 * time.Hour)
	var out []ecstypes.Task
	for _, arn := range in.Tasks {
		task := ecstypes.Task{
			TaskArn:           aws.String(arn),
			ClusterArn:        aws.String(t9cClusterARN),
			TaskDefinitionArn: aws.String(t9cTaskDefinition),
			LaunchType:        ecstypes.LaunchTypeFargate,
			Cpu:               aws.String("512"),
			Memory:            aws.String("1024"),
			CreatedAt:         aws.Time(started),
			Containers:        []ecstypes.Container{{Name: aws.String("api"), Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/orders-api:1.4.2")}},
		}
		switch {
		case strings.HasSuffix(arn, t9cRunningTaskID):
			task.LastStatus, task.DesiredStatus = aws.String("RUNNING"), aws.String("RUNNING")
		case strings.HasSuffix(arn, t9cStoppedTaskID):
			task.LastStatus, task.DesiredStatus = aws.String("STOPPED"), aws.String("STOPPED")
			task.StopCode = ecstypes.TaskStopCodeEssentialContainerExited
			task.StoppedReason = aws.String("Essential container in task exited")
			task.StoppedAt = aws.Time(started.Add(time.Hour))
		default:
			continue
		}
		out = append(out, task)
	}
	return &ecs.DescribeTasksOutput{Tasks: out}, nil
}

func (f *t9cECS) DescribeTaskDefinition(_ context.Context, _ *ecs.DescribeTaskDefinitionInput, _ ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error) {
	return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: &ecstypes.TaskDefinition{
		TaskDefinitionArn: aws.String(t9cTaskDefinition),
		Family:            aws.String("orders-api"),
		ContainerDefinitions: []ecstypes.ContainerDefinition{{
			Name:  aws.String("api"),
			Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/orders-api:1.4.2"),
		}},
	}}, nil
}

func t9cECSClients() (*awsclient.ServiceClients, *t9cECS) {
	clients := demo.NewServiceClients()
	fake := &t9cECS{ECSAPI: clients.ECS}
	clients.ECS = fake
	return clients, fake
}

// The ecs detail lists Settings, which DescribeClusters returns only when
// Include names it: "If SETTINGS is specified, the settings for the cluster
// are included" (ecs DescribeClustersInput.Include). Container Insights is a
// setting.
func TestT9c_Row10_ECSClusterDetailShowsSettings(t *testing.T) {
	clients, fake := t9cECSClients()
	rows := t9cFetch(t, "ecs", clients)
	row, ok := t9cRowByID(rows, "acme-prod")
	if !ok {
		t.Fatalf("no ecs row acme-prod in %d rows", len(rows))
	}
	if td := catalog.FindAny("ecs"); td.DetailEnrich != nil {
		enriched, err := td.DetailEnrich(context.Background(), t9cDetailCtx(clients), row)
		if err != nil && !errors.As(err, new(domain.PartialDetailErr)) {
			t.Fatalf("ecs detail enricher: %v", err)
		}
		row = enriched
	}
	shown := t9bPathRows(t9bDetailRows(t, row, "ecs"), "Settings")
	if !strings.Contains(strings.Join(shown, " "), "enhanced") {
		t.Errorf("ecs detail Settings rows %v do not show the containerInsights setting", shown)
	}
	asked := false
	for _, in := range fake.describeClusters {
		if slices.Contains(in.Include, ecstypes.ClusterFieldSettings) {
			asked = true
		}
	}
	if !asked {
		t.Errorf("no DescribeClusters call names SETTINGS in Include: %d calls", len(fake.describeClusters))
	}
}

// ListTasks returns RUNNING-desired tasks unless asked otherwise: "The default
// status filter is RUNNING" (ecs ListTasksInput.DesiredStatus). A stopped task
// and its stop reason are part of the ecs-task list, so the list asks for
// STOPPED too, and still lists the running ones.
func TestT9c_Row10_ECSTaskListIncludesStoppedTasks(t *testing.T) {
	clients, fake := t9cECSClients()
	rows := t9cFetch(t, "ecs-task", clients)
	for _, id := range []string{t9cRunningTaskID, t9cStoppedTaskID} {
		if _, ok := t9cRowByID(rows, id); !ok {
			var ids []string
			for _, r := range rows {
				ids = append(ids, r.ID)
			}
			t.Errorf("ecs-task list lacks task %s; rows %v", id, ids)
		}
	}
	var statuses []string
	for _, in := range fake.listTasks {
		statuses = append(statuses, string(in.DesiredStatus))
	}
	if !slices.Contains(statuses, string(ecstypes.DesiredStatusStopped)) {
		t.Errorf("no ListTasks call asks for DesiredStatus STOPPED; desired statuses sent %q", statuses)
	}
}

// t9cEB answers DescribeEnvironments with the terminated environment only
// when IncludeDeleted is true: "true : Environments that have been deleted
// after IncludedDeletedBackTo are displayed" (elasticbeanstalk
// DescribeEnvironmentsInput.IncludeDeleted). The list sends the value
// explicitly, so its answer does not rest on a server default.
type t9cEB struct {
	awsclient.ElasticBeanstalkAPI
	calls []*elasticbeanstalk.DescribeEnvironmentsInput
}

func (f *t9cEB) DescribeEnvironments(_ context.Context, in *elasticbeanstalk.DescribeEnvironmentsInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentsOutput, error) {
	f.calls = append(f.calls, in)
	created := time.Now().Add(-30 * 24 * time.Hour)
	envs := []ebtypes.EnvironmentDescription{{
		EnvironmentName:   aws.String("orders-prod"),
		EnvironmentId:     aws.String("e-a1b2c3d4e5"),
		ApplicationName:   aws.String("orders"),
		EnvironmentArn:    aws.String("arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/orders/orders-prod"),
		Status:            ebtypes.EnvironmentStatusReady,
		Health:            ebtypes.EnvironmentHealthGreen,
		SolutionStackName: aws.String("64bit Amazon Linux 2023 v4.3.0 running Python 3.11"),
		VersionLabel:      aws.String("v42"),
		DateCreated:       aws.Time(created),
		DateUpdated:       aws.Time(created.Add(time.Hour)),
	}}
	if aws.ToBool(in.IncludeDeleted) {
		envs = append(envs, ebtypes.EnvironmentDescription{
			EnvironmentName:   aws.String("orders-old"),
			EnvironmentId:     aws.String("e-f6g7h8i9j0"),
			ApplicationName:   aws.String("orders"),
			EnvironmentArn:    aws.String("arn:aws:elasticbeanstalk:us-east-1:123456789012:environment/orders/orders-old"),
			Status:            ebtypes.EnvironmentStatusTerminated,
			Health:            ebtypes.EnvironmentHealthGrey,
			SolutionStackName: aws.String("64bit Amazon Linux 2 v3.5.0 running Python 3.8"),
			VersionLabel:      aws.String("v17"),
			DateCreated:       aws.Time(created.Add(-60 * 24 * time.Hour)),
			DateUpdated:       aws.Time(time.Now().Add(-12 * time.Hour)),
		})
	}
	return &elasticbeanstalk.DescribeEnvironmentsOutput{Environments: envs}, nil
}

// The eb list shows a terminated environment, the row its Terminated state
// describes, so DescribeEnvironments is asked with IncludeDeleted true.
func TestT9c_Row10_EBListIncludesDeletedEnvironments(t *testing.T) {
	clients := demo.NewServiceClients()
	fake := &t9cEB{ElasticBeanstalkAPI: clients.ElasticBeanstalk}
	clients.ElasticBeanstalk = fake
	rows := t9cFetch(t, "eb", clients)
	names := map[string]bool{}
	for _, r := range rows {
		names[r.Fields["environment_name"]] = true
	}
	for _, want := range []string{"orders-prod", "orders-old"} {
		if !names[want] {
			t.Errorf("eb list lacks environment %s; listed %v", want, names)
		}
	}
	for _, in := range fake.calls {
		if in.IncludeDeleted == nil || !*in.IncludeDeleted {
			t.Errorf("DescribeEnvironments sent IncludeDeleted %v, want true", in.IncludeDeleted)
		}
	}
}
