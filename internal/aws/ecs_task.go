package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ecs-task", []string{"task_id", "cluster", "last_status", "stop_code", "health_status", "task_definition", "launch_type", "cpu", "memory", "status", "efs_file_system_ids"})

	resource.RegisterPaginated("ecs-task", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return fetchECSTasksPageWithJoin(ctx, c.ECS, c.ECS, c.ECS, c.ECS, continuationToken)
	})
}

// FetchECSTasksPage fetches one page of ECS clusters using the continuationToken,
// then for each cluster in that page fetches all tasks via ListTasks+DescribeTasks.
// IsTruncated reflects whether ListClusters has more pages beyond this one.
// Fields["efs_file_system_ids"] is always "" (no task-definition join).
// Use the RegisterPaginated path for the full join via DescribeTaskDefinition.
func FetchECSTasksPage(
	ctx context.Context,
	listClustersAPI ECSListClustersAPI,
	listTasksAPI ECSListTasksAPI,
	describeTasksAPI ECSDescribeTasksAPI,
	continuationToken string,
) (resource.FetchResult, error) {
	return fetchECSTasksPageWithJoin(ctx, listClustersAPI, listTasksAPI, describeTasksAPI, nil, continuationToken)
}

// fetchECSTasksPageWithJoin is the full implementation used by the RegisterPaginated
// closure in init(). describeTaskDefAPI may be nil; in that case the EFS volume
// join is skipped and Fields["efs_file_system_ids"] is always "".
func fetchECSTasksPageWithJoin(
	ctx context.Context,
	listClustersAPI ECSListClustersAPI,
	listTasksAPI ECSListTasksAPI,
	describeTasksAPI ECSDescribeTasksAPI,
	describeTaskDefAPI ECSDescribeTaskDefinitionAPI,
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

	// Memoize DescribeTaskDefinition results across all clusters in this page.
	seenTaskDefs := make(map[string]*ecstypes.TaskDefinition)

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

			stopCode := string(task.StopCode)
			healthStatus := string(task.HealthStatus)

			// Join task definition to extract EFS file-system IDs.
			// Skipped gracefully when describeTaskDefAPI is nil.
			efsFileSystemIDs := ecsJoinEFSVolumes(ctx, task, seenTaskDefs, describeTaskDefAPI)

			r := resource.Resource{
				ID:     taskID,
				Name:   taskID,
				Status: status,
				Fields: map[string]string{
					"task_id":             taskID,
					"cluster":             clusterName,
					"status":              status,
					"last_status":         status,
					"stop_code":           stopCode,
					"health_status":       healthStatus,
					"task_definition":     taskDefinition,
					"launch_type":         launchType,
					"cpu":                 cpu,
					"memory":              memory,
					"efs_file_system_ids": efsFileSystemIDs,
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

// ecsJoinEFSVolumes resolves the task definition for a task (using the memoized
// seenTaskDefs map) and extracts the unique EFS file-system IDs from its Volumes.
// Returns a sorted, comma-separated string of file-system IDs, or "" if none.
// When api is nil, returns "" immediately. Errors from DescribeTaskDefinition
// are swallowed; they produce "" for that task.
func ecsJoinEFSVolumes(
	ctx context.Context,
	task ecstypes.Task,
	seenTaskDefs map[string]*ecstypes.TaskDefinition,
	api ECSDescribeTaskDefinitionAPI,
) string {
	if api == nil {
		return ""
	}
	if task.TaskDefinitionArn == nil || *task.TaskDefinitionArn == "" {
		return ""
	}
	arn := *task.TaskDefinitionArn

	td, cached := seenTaskDefs[arn]
	if !cached {
		out, err := api.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: &arn,
		})
		if err != nil || out == nil || out.TaskDefinition == nil {
			// Gracefully tolerate errors — skip join for this task.
			seenTaskDefs[arn] = nil
			return ""
		}
		seenTaskDefs[arn] = out.TaskDefinition
		td = out.TaskDefinition
	}
	if td == nil {
		return ""
	}

	// Collect unique EFS file-system IDs from Volumes.
	seen := make(map[string]struct{})
	for _, v := range td.Volumes {
		if v.EfsVolumeConfiguration != nil &&
			v.EfsVolumeConfiguration.FileSystemId != nil &&
			*v.EfsVolumeConfiguration.FileSystemId != "" {
			seen[*v.EfsVolumeConfiguration.FileSystemId] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return ""
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

// FetchECSTasks performs a three-step fetch:
// 1. ListClusters to get cluster ARNs
// 2. ListTasks per cluster to get task ARNs
// 3. DescribeTasks per cluster to get full details
//
// Fields["efs_file_system_ids"] is always "" (no task-definition join).
// Use the RegisterPaginated path (init) for the full join via DescribeTaskDefinition.
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
