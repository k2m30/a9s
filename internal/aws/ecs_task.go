package aws

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchECSTasksPage fetches one page of ECS clusters using the continuationToken,
// then for each cluster in that page fetches all tasks via ListTasks+DescribeTasks.
// IsTruncated reflects whether ListClusters has more pages beyond this one.
// Fields["efs_file_system_ids"] is always "" (no task-definition join).
// Use the SetPaginatedForTest path for the full join via DescribeTaskDefinition.
func FetchECSTasksPage(
	ctx context.Context,
	listClustersAPI ECSListClustersAPI,
	listTasksAPI ECSListTasksAPI,
	describeTasksAPI ECSDescribeTasksAPI,
	continuationToken string,
) (resource.FetchResult, error) {
	return fetchECSTasksPageWithJoin(ctx, listClustersAPI, listTasksAPI, describeTasksAPI, nil, continuationToken)
}

// fetchECSTasksPageWithJoin is the full implementation used by the SetPaginatedForTest
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
			// Skipped gracefully when describeTaskDefAPI is nil. A join failure
			// is recorded as a per-task Fields["task_def_join_error"]="true" so
			// reverse-scan checkers (e.g. checkEFSECSTask) can report
			// Approximate without the fetcher lying about pagination
			// truncation (which would misleadingly surface "m: load more").
			efsFileSystemIDs, joinErr := ecsJoinEFSVolumes(ctx, task, seenTaskDefs, describeTaskDefAPI)

			fields := map[string]string{
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
			}
			if joinErr != nil {
				fields["task_def_join_error"] = "true"
			}

			// PR-03c: emit wave1 Findings for non-healthy transitional states.
			// RUNNING and STOPPED → no Finding (lifecycle; stop_code handled structurally).
			findings := ecsTaskWave1Findings(status)

			r := resource.Resource{
				ID:        taskID,
				Name:      taskID,
				Fields:    fields,
				Findings:  findings,
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
// Returns a sorted, comma-separated string of file-system IDs (or "" if none)
// and an error when DescribeTaskDefinition failed. The caller surfaces the
// error as pagination truncation so downstream reverse-scan checkers
// (e.g. checkEFSECSTask) report Approximate rather than a silently-wrong
// definite zero.
func ecsJoinEFSVolumes(
	ctx context.Context,
	task ecstypes.Task,
	seenTaskDefs map[string]*ecstypes.TaskDefinition,
	api ECSDescribeTaskDefinitionAPI,
) (string, error) {
	if api == nil {
		return "", nil
	}
	if task.TaskDefinitionArn == nil || *task.TaskDefinitionArn == "" {
		return "", nil
	}
	arn := *task.TaskDefinitionArn

	td, cached := seenTaskDefs[arn]
	if !cached {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecs.DescribeTaskDefinitionOutput, error) {
			return api.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: &arn,
			})
		})
		if err != nil {
			seenTaskDefs[arn] = nil
			// "Task definition does not exist" (ClientException) is a
			// definitive absence, not an error worth surfacing as
			// truncation — no volumes = no EFS IDs. Every other error
			// (access denied, throttled, transient) is propagated so the
			// fetcher marks Pagination.IsTruncated and reverse-scan
			// checkers report Approximate.
			var apiErr smithy.APIError
			if errors.As(err, &apiErr) && apiErr.ErrorCode() == "ClientException" {
				return "", nil
			}
			return "", fmt.Errorf("describing task definition %s: %w", arn, err)
		}
		if out == nil || out.TaskDefinition == nil {
			seenTaskDefs[arn] = nil
			return "", nil
		}
		seenTaskDefs[arn] = out.TaskDefinition
		td = out.TaskDefinition
	}
	if td == nil {
		return "", nil
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
		return "", nil
	}

	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return strings.Join(ids, ","), nil
}

// FetchECSTasks performs a three-step fetch:
// 1. ListClusters to get cluster ARNs
// 2. ListTasks per cluster to get task ARNs
// 3. DescribeTasks per cluster to get full details
//
// Fields["efs_file_system_ids"] is always "" (no task-definition join).
// Use the SetPaginatedForTest path (init) for the full join via DescribeTaskDefinition.
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
