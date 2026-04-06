package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ecs-task", []string{"task_id", "cluster", "status", "task_definition", "launch_type", "cpu", "memory"})

	resource.RegisterPaginated("ecs-task", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchECSTasksPage(ctx, c.ECS, c.ECS, c.ECS, continuationToken)
	})
}

// FetchECSTasksPage fetches one page of ECS clusters using the continuationToken,
// then for each cluster in that page fetches all tasks via ListTasks+DescribeTasks.
// IsTruncated reflects whether ListClusters has more pages beyond this one.
func FetchECSTasksPage(
	ctx context.Context,
	listClustersAPI ECSListClustersAPI,
	listTasksAPI ECSListTasksAPI,
	describeTasksAPI ECSDescribeTasksAPI,
	continuationToken string,
) (resource.FetchResult, error) {
	input := &ecs.ListClustersInput{}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	listOutput, err := listClustersAPI.ListClusters(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing ECS clusters: %w", err)
	}

	var resources []resource.Resource

	for _, clusterArn := range listOutput.ClusterArns {
		taskListOutput, err := listTasksAPI.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster: aws.String(clusterArn),
		})
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("listing ECS tasks: %w", err)
		}

		if len(taskListOutput.TaskArns) == 0 {
			continue
		}

		descOutput, err := describeTasksAPI.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(clusterArn),
			Tasks:   taskListOutput.TaskArns,
		})
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("describing ECS tasks: %w", err)
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
				RawStruct: task,
			}

			resources = append(resources, r)
		}
	}

	nextToken := ""
	isTruncated := false
	if listOutput.NextToken != nil {
		nextToken = *listOutput.NextToken
		isTruncated = true
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, nil
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
	var allResources []resource.Resource
	continuationToken := ""

	for {
		result, err := FetchECSTasksPage(ctx, listClustersAPI, listTasksAPI, describeTasksAPI, continuationToken)
		if err != nil {
			return nil, err
		}

		allResources = append(allResources, result.Resources...)

		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		continuationToken = result.Pagination.NextToken
	}

	return allResources, nil
}
