// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// fetchECSTasksPageWithJoin fetches one page of ECS tasks: ListClusters,
// every ListTasks page of each cluster, and DescribeTasks for at most 100
// tasks a call, through parentChildWalk. describeTaskDefAPI may be nil; in
// that case the EFS volume join is skipped and Fields["efs_file_system_ids"]
// is always "". This is the full implementation registered as the ecs-task
// catalog Fetcher (see catalog_compute.go).
func fetchECSTasksPageWithJoin(
	ctx context.Context,
	listClustersAPI ECSListClustersAPI,
	listTasksAPI ECSListTasksAPI,
	describeTasksAPI ECSDescribeTasksAPI,
	describeTaskDefAPI ECSDescribeTaskDefinitionAPI,
	continuationToken string,
) (resource.FetchResult, error) {
	// Memoize DescribeTaskDefinition results across every cluster of the page.
	seenTaskDefs := make(map[string]*ecstypes.TaskDefinition)
	walk := parentChildWalk{
		listParents: func(ctx context.Context, token *string) ([]string, *string, error) {
			out, err := listClustersAPI.ListClusters(ctx, &ecs.ListClustersInput{NextToken: token})
			if err != nil {
				return nil, nil, fmt.Errorf("listing ECS clusters: %w", err)
			}
			return out.ClusterArns, out.NextToken, nil
		},
		listChildren: func(ctx context.Context, clusterArn string, token *string) ([]string, *string, error) {
			out, err := listTasksAPI.ListTasks(ctx, &ecs.ListTasksInput{
				Cluster:    aws.String(clusterArn),
				MaxResults: aws.Int32(100),
				NextToken:  token,
			})
			if err != nil {
				return nil, nil, fmt.Errorf("listing ECS tasks: %w", err)
			}
			return out.TaskArns, out.NextToken, nil
		},
		describe: func(ctx context.Context, clusterArn string, taskArns []string) ([]resource.Resource, error) {
			descOutput, err := describeTasksAPI.DescribeTasks(ctx, &ecs.DescribeTasksInput{
				Cluster: aws.String(clusterArn),
				Tasks:   taskArns,
			})
			if err != nil {
				return nil, fmt.Errorf("describing ECS tasks: %w", err)
			}
			resources := make([]resource.Resource, 0, len(descOutput.Tasks))
			for _, task := range descOutput.Tasks {
				taskID := ""
				taskArn := ""
				if task.TaskArn != nil {
					taskArn = *task.TaskArn
					parts := strings.Split(taskArn, "/")
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
				healthStatus := ecsTaskHealthWords(task.HealthStatus)

				var images []string
				for _, container := range task.Containers {
					if container.Image != nil && *container.Image != "" {
						images = append(images, *container.Image)
					}
				}
				containerImages := strings.Join(images, ",")

				// Join task definition to extract EFS file-system IDs, IAM
				// roles, and Secrets Manager / SSM ValueFrom references.
				// Skipped gracefully when describeTaskDefAPI is nil. A join
				// failure is recorded as a per-task
				// Fields["task_def_join_error"]="true" so reverse-scan checkers
				// (e.g. checkEFSECSTask) can report Truncated without the
				// fetcher lying about pagination truncation (which would
				// misleadingly surface "m: load more").
				taskDefJoin, joinErr := ecsJoinTaskDefinition(ctx, task, seenTaskDefs, describeTaskDefAPI)

				fields := map[string]string{
					"task_id":             taskID,
					"cluster":             clusterName,
					"status":              status,
					"stop_code":           stopCode,
					"health_status":       healthStatus,
					"task_definition":     taskDefinition,
					"launch_type":         launchType,
					"cpu":                 cpu,
					"memory":              memory,
					"efs_file_system_ids": taskDefJoin.efsFileSystemIDs,
					"task_role":           taskDefJoin.taskRoleARN,
					"execution_role":      taskDefJoin.executionRoleARN,
					"secret_arns":         taskDefJoin.secretARNs,
					"ssm_param_names":     taskDefJoin.ssmParamNames,
					"container_images":    containerImages,
					"arn":                 taskArn,
				}
				if joinErr != nil {
					fields["task_def_join_error"] = "true"
				}

				findings := ecsTaskStructuralFindings(status, stopCode, healthStatus)

				r := resource.Resource{
					ID:        taskID,
					Name:      taskID,
					Fields:    fields,
					Findings:  findings,
					RawStruct: task,
				}

				resources = append(resources, r)
			}
			return resources, nil
		},
		batch: 100,
	}
	return walk.page(ctx, continuationToken)
}

// taskDefJoinFields holds the per-task fields resolved by ecsJoinTaskDefinition
// from a single DescribeTaskDefinition call.
type taskDefJoinFields struct {
	efsFileSystemIDs string
	taskRoleARN      string
	executionRoleARN string
	// secretARNs is a sorted, comma-joined list of Secrets Manager ARNs from
	// ContainerDefinitions[].Secrets[].ValueFrom and
	// ContainerDefinitions[].RepositoryCredentials.CredentialsParameter —
	// required for the ecs-task:secrets related-panel pivot.
	secretARNs string
	// ssmParamNames is a sorted, comma-joined list of SSM parameter names
	// resolved from ContainerDefinitions[].Secrets[].ValueFrom — required for
	// the ecs-task:ssm related-panel pivot.
	ssmParamNames string
}

// ecsJoinTaskDefinition resolves the task definition for a task (using the
// memoized seenTaskDefs map) and extracts the fields required by the
// ecs-task related-panel pivots: EFS file-system IDs, the task/execution IAM
// role ARNs, and the Secrets Manager / SSM references injected via
// ContainerDefinitions[].Secrets[]. Returns a zero-value taskDefJoinFields and
// an error when DescribeTaskDefinition failed. The caller surfaces the error
// as pagination truncation so downstream reverse-scan checkers (e.g.
// checkEFSECSTask) report Truncated rather than a silently-wrong definite
// zero.
func ecsJoinTaskDefinition(
	ctx context.Context,
	task ecstypes.Task,
	seenTaskDefs map[string]*ecstypes.TaskDefinition,
	api ECSDescribeTaskDefinitionAPI,
) (taskDefJoinFields, error) {
	if api == nil {
		return taskDefJoinFields{}, nil
	}
	if task.TaskDefinitionArn == nil || *task.TaskDefinitionArn == "" {
		return taskDefJoinFields{}, nil
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
			// checkers report Truncated.
			if ErrCodeIs(err, "ClientException") {
				return taskDefJoinFields{}, nil
			}
			return taskDefJoinFields{}, fmt.Errorf("describing task definition %s: %w", arn, err)
		}
		if out == nil || out.TaskDefinition == nil {
			seenTaskDefs[arn] = nil
			return taskDefJoinFields{}, nil
		}
		seenTaskDefs[arn] = out.TaskDefinition
		td = out.TaskDefinition
	}
	if td == nil {
		return taskDefJoinFields{}, nil
	}

	var out taskDefJoinFields

	efsSeen := make(map[string]struct{})
	for _, v := range td.Volumes {
		if v.EfsVolumeConfiguration != nil &&
			v.EfsVolumeConfiguration.FileSystemId != nil &&
			*v.EfsVolumeConfiguration.FileSystemId != "" {
			efsSeen[*v.EfsVolumeConfiguration.FileSystemId] = struct{}{}
		}
	}
	if len(efsSeen) > 0 {
		ids := make([]string, 0, len(efsSeen))
		for id := range efsSeen {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		out.efsFileSystemIDs = strings.Join(ids, ",")
	}

	if td.TaskRoleArn != nil {
		out.taskRoleARN = *td.TaskRoleArn
	}
	if td.ExecutionRoleArn != nil {
		out.executionRoleARN = *td.ExecutionRoleArn
	}

	secretsSeen := make(map[string]struct{})
	ssmSeen := make(map[string]struct{})
	const ssmParamPrefix = "parameter/"
	for _, c := range td.ContainerDefinitions {
		for _, s := range c.Secrets {
			if s.ValueFrom == nil || *s.ValueFrom == "" {
				continue
			}
			v := *s.ValueFrom
			ssmARN, isSSM := ARNForService(v, "ssm")
			switch {
			case isSecret(v):
				secretsSeen[v] = struct{}{}
			case isSSM:
				// The parameter's real Name (as returned by
				// DescribeParameters) keeps its own leading "/", which the
				// "parameter/" resource prefix absorbs; re-add it so the
				// extracted name matches the ssm cache's canonical
				// Resource.ID.
				if after, found := strings.CutPrefix(ssmARN.Resource, ssmParamPrefix); found && after != "" {
					ssmSeen["/"+after] = struct{}{}
				}
			case strings.HasPrefix(v, "/"):
				// Bare SSM parameter name (no ARN) resolved to the
				// account/region namespace by the SSM client at read time.
				ssmSeen[v] = struct{}{}
			}
		}
		if c.RepositoryCredentials != nil && c.RepositoryCredentials.CredentialsParameter != nil &&
			*c.RepositoryCredentials.CredentialsParameter != "" {
			secretsSeen[*c.RepositoryCredentials.CredentialsParameter] = struct{}{}
		}
	}
	if len(secretsSeen) > 0 {
		ids := make([]string, 0, len(secretsSeen))
		for id := range secretsSeen {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		out.secretARNs = strings.Join(ids, ",")
	}
	if len(ssmSeen) > 0 {
		ids := make([]string, 0, len(ssmSeen))
		for id := range ssmSeen {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		out.ssmParamNames = strings.Join(ids, ",")
	}

	return out, nil
}
