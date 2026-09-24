// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/domain"
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
	seenTaskDefs := make(map[string]taskDefRead)
	var joinFailures []Failure
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
			// Tags are returned only when named in Include.
			descOutput, err := describeTasksAPI.DescribeTasks(ctx, &ecs.DescribeTasksInput{
				Cluster: aws.String(clusterArn),
				Tasks:   taskArns,
				Include: []ecstypes.TaskField{ecstypes.TaskFieldTags},
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
					taskID, _ = ecsTaskRefToID(taskArn, domain.RefContext{})
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
					"log_groups":          taskDefJoin.logGroups,
					"container_images":    containerImages,
					"arn":                 taskArn,
				}
				if taskDefJoin.logGroupsUnread {
					fields["log_groups_unread"] = "true"
				}
				if joinErr != nil {
					fields["task_def_join_error"] = "true"
					joinFailures = append(joinFailures, FailedCall(taskID, joinErr))
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
	// A task whose definition could not be read keeps its row: the failure
	// is the row's, reported once with the page, and leaves the list whole.
	result, err := walk.page(ctx, continuationToken)
	return result, JoinAggregates(err, AggregateFailures("ecs-task: DescribeTaskDefinition", joinFailures, len(result.Resources)))
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
	// ssmParamNames is a sorted, comma-joined list of the SSM parameter
	// references (ARN or name) ecsSecretRefs classifies, as the definition
	// writes them: the ecs-task:ssm pivot resolves them against the loaded
	// list, which knows the session's Region and account.
	ssmParamNames string
	// logGroups is a sorted, comma-joined list of the awslogs-group of each
	// container — required for the logs:ecs-task related-panel pivot.
	logGroups string
	// logGroupsUnread is set when a container's destination is not readable
	// from the definition (ecsContainerLogGroups).
	logGroupsUnread bool
}

// taskDefRead is one DescribeTaskDefinition answer, kept for every task of
// the page that runs the definition: a failure is theirs as much as the
// first task's.
type taskDefRead struct {
	def *ecstypes.TaskDefinition
	err error
}

// readTaskDefinition reads one task definition. "Task definition does not
// exist" (ClientException) is a definitive absence, not an error worth
// surfacing as truncation — no volumes = no EFS IDs. Every other error
// (access denied, throttled, transient) is kept so the task is marked
// unjoined and reverse-scan checkers report Truncated.
func readTaskDefinition(ctx context.Context, api ECSDescribeTaskDefinitionAPI, arn string) taskDefRead {
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecs.DescribeTaskDefinitionOutput, error) {
		return api.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
			TaskDefinition: &arn,
		})
	})
	switch {
	case ErrCodeIs(err, "ClientException"):
		return taskDefRead{}
	case err != nil:
		return taskDefRead{err: fmt.Errorf("describing task definition %s: %w", arn, err)}
	case out == nil:
		return taskDefRead{}
	}
	return taskDefRead{def: out.TaskDefinition}
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
	seenTaskDefs map[string]taskDefRead,
	api ECSDescribeTaskDefinitionAPI,
) (taskDefJoinFields, error) {
	if api == nil {
		return taskDefJoinFields{}, nil
	}
	if task.TaskDefinitionArn == nil || *task.TaskDefinitionArn == "" {
		return taskDefJoinFields{}, nil
	}
	arn := *task.TaskDefinitionArn

	read, cached := seenTaskDefs[arn]
	if !cached {
		read = readTaskDefinition(ctx, api, arn)
		seenTaskDefs[arn] = read
	}
	if read.err != nil || read.def == nil {
		return taskDefJoinFields{}, read.err
	}
	td := read.def

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
	secrets, params := ecsSecretRefs(td)
	for _, v := range secrets {
		secretsSeen[v] = struct{}{}
	}
	for _, v := range params {
		ssmSeen[v] = struct{}{}
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
	byRegion, whole := ecsContainerLogGroups(td, arnRegionOf(aws.ToString(task.TaskArn), "ecs"))
	groups := byRegion[""]
	sort.Strings(groups)
	out.logGroups = strings.Join(slices.Compact(groups), ",")
	out.logGroupsUnread = !whole

	return out, nil
}

// taskDefJoined reports whether the list fetcher read this task's task
// definition. The EFS volumes, the roles, and the Secrets Manager and SSM
// references of a task live on its definition rather than on the task, so a
// task whose join failed carries none of them: a reader that takes the empty
// field for "none" states a zero about a definition nobody read.
func taskDefJoined(res resource.Resource) bool {
	return res.Fields["task_def_join_error"] != "true"
}

// ecsTaskJoinedLogGroups is ecsTaskLogGroups for a row the list's join
// answers (ecsTaskAnsweredByJoin).
func ecsTaskJoinedLogGroups(row resource.Resource) (groups []string, read bool) {
	field := row.Fields["log_groups"]
	if !taskDefJoined(row) {
		return nil, false
	}
	whole := row.Fields["log_groups_unread"] != "true"
	if field == "" {
		return nil, whole
	}
	return strings.Split(field, ","), whole
}

// ecsTaskAnsweredByJoin reports whether the list's definition join answers
// field for row without a call: the join wrote it, or failed and left the row
// unread.
func ecsTaskAnsweredByJoin(row resource.Resource, field string) bool {
	_, ok := row.Fields[field]
	return ok || !taskDefJoined(row)
}

// splitECSTaskJoin parts ecs-task rows into the ones the join answers for
// field and the ones that need their definition read.
func splitECSTaskJoin(rows []resource.Resource, field string) (joined, unjoined []resource.Resource) {
	for _, r := range rows {
		if ecsTaskAnsweredByJoin(r, field) {
			joined = append(joined, r)
		} else {
			unjoined = append(unjoined, r)
		}
	}
	return joined, unjoined
}

// ecsTaskLogGroups is the log groups in the task's own Region that the task's
// containers write to, and whether every container's destination was read:
// the list fetcher's join when the row carries it, else one read of the
// definition. A task whose join failed was not read; the fetcher already met
// the failure. A group in another Region is left out: its one reader,
// logs → ecs-task, asks about a group listed in the Region the tasks are.
func ecsTaskLogGroups(ctx context.Context, clients any, row resource.Resource) (groups []string, read bool, err error) {
	if ecsTaskAnsweredByJoin(row, "log_groups") {
		groups, read := ecsTaskJoinedLogGroups(row)
		return groups, read, nil
	}
	taskDef, taskARN := row.Fields["task_definition"], row.Fields["arn"]
	if task, ok := assertStruct[ecstypes.Task](row.RawStruct); ok {
		taskDef, taskARN = cmp.Or(aws.ToString(task.TaskDefinitionArn), taskDef), cmp.Or(aws.ToString(task.TaskArn), taskARN)
	}
	if taskDef == "" {
		return nil, false, nil
	}
	def, err := ecsTaskDefinition(ctx, clients, taskDef)
	if err == nil && def == nil {
		err = errClientMissing
	}
	if err != nil {
		return nil, false, err
	}
	byRegion, whole := ecsContainerLogGroups(def, cmp.Or(arnRegionOf(taskARN, "ecs"), sessionRegion(clients)))
	return byRegion[""], whole, nil
}
