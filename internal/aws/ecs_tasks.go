package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("ecs-task", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchECSTasks(ctx, c.ECS, c.ECS, c.ECS)
	})
	resource.RegisterFieldKeys("ecs-task", []string{"task_id", "cluster", "status", "task_definition", "launch_type", "cpu", "memory"})
}

// FetchECSTasks performs a three-step fetch:
// 1. ListClusters to get cluster ARNs
// 2. ListTasks per cluster to get task ARNs
// 3. DescribeTasks per cluster to get full details
func FetchECSTasks(
	ctx context.Context,
	listClustersAPI ECSListClustersAPI,
	listTasksAPI ECSListTasksAPI,
	describeTasksAPI ECSDescribeTasksAPI,
) ([]resource.Resource, error) {
	listOutput, err := listClustersAPI.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return nil, err
	}

	var resources []resource.Resource

	for _, clusterArn := range listOutput.ClusterArns {
		taskListOutput, err := listTasksAPI.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster: aws.String(clusterArn),
		})
		if err != nil {
			return nil, err
		}

		if len(taskListOutput.TaskArns) == 0 {
			continue
		}

		descOutput, err := describeTasksAPI.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(clusterArn),
			Tasks:   taskListOutput.TaskArns,
		})
		if err != nil {
			return nil, err
		}

		for _, task := range descOutput.Tasks {
			// Extract task UUID from ARN (last segment after /)
			taskID := ""
			if task.TaskArn != nil {
				parts := strings.Split(*task.TaskArn, "/")
				taskID = parts[len(parts)-1]
			}

			clusterName := ""
			if task.ClusterArn != nil {
				clusterName = *task.ClusterArn
			}

			status := ""
			if task.LastStatus != nil {
				status = *task.LastStatus
			}

			taskDefinition := ""
			if task.TaskDefinitionArn != nil {
				taskDefinition = *task.TaskDefinitionArn
			}

			launchType := string(task.LaunchType)

			cpu := ""
			if task.Cpu != nil {
				cpu = *task.Cpu
			}

			memory := ""
			if task.Memory != nil {
				memory = *task.Memory
			}

			detail := map[string]string{
				"Task ID":         taskID,
				"Cluster":         clusterName,
				"Status":          status,
				"Task Definition": taskDefinition,
				"Launch Type":     launchType,
				"CPU":             cpu,
				"Memory":          memory,
			}

			if task.TaskArn != nil {
				detail["ARN"] = *task.TaskArn
			}
			if task.DesiredStatus != nil {
				detail["Desired Status"] = *task.DesiredStatus
			}
			if task.Group != nil {
				detail["Group"] = *task.Group
			}
			if task.StartedAt != nil {
				detail["Started At"] = task.StartedAt.Format("2006-01-02T15:04:05Z07:00")
			}
			if task.StoppedAt != nil {
				detail["Stopped At"] = task.StoppedAt.Format("2006-01-02T15:04:05Z07:00")
			}
			if task.StoppedReason != nil {
				detail["Stopped Reason"] = *task.StoppedReason
			}

			rawJSON := ""
			if jsonBytes, err := json.MarshalIndent(task, "", "  "); err == nil {
				rawJSON = string(jsonBytes)
			}

			r := resource.Resource{
				ID:     taskID,
				Name:   taskID,
				Status: status,
				Fields: map[string]string{
					"task_id":         taskID,
					"cluster":         clusterName,
					"status":          status,
					"task_definition": taskDefinition,
					"launch_type":     launchType,
					"cpu":             cpu,
					"memory":          memory,
				},
				DetailData: detail,
				RawJSON:    rawJSON,
				RawStruct:  task,
			}

			resources = append(resources, r)
		}
	}

	return resources, nil
}
