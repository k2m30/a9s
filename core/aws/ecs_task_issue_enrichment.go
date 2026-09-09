// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecs_task_issue_enrichment.go — Wave 2 issue enrichment for the ecs-task resource type.
package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ecs-task canonical FindingCodes.
const (
	ecsTaskCodeTaskFailed    domain.FindingCode = "ecs-task.task-failed"
	ecsTaskCodePrivileged    domain.FindingCode = "ecs-task.privileged"
	ecsTaskCodeHostNamespace domain.FindingCode = "ecs-task.host-namespace"
	ecsTaskCodeWritableRoot  domain.FindingCode = "ecs-task.writable-root"
	ecsTaskCodeNoLogging     domain.FindingCode = "ecs-task.no-logging"
	//nolint:gosec // G101 false positive: a finding code, not a credential
	ecsTaskCodeEnvSecret domain.FindingCode = "ecs-task.env-secret"
)

// ecsTaskGone reports a task that is stopped or on its way there. The row is
// a task, not a definition, so once teardown starts its definition's posture
// is no longer an open item — nobody is going to reconfigure a task that is
// already draining. The single place that fact is spelled: the fetcher's
// lifecycle findings and the Wave-2 posture pass both call it. The lifecycle
// finding for these states still fires; a state is not a posture.
func ecsTaskGone(lastStatus string) bool {
	switch lastStatus {
	case "STOPPED", "STOPPING", "DEPROVISIONING", "DEACTIVATING":
		return true
	}
	return false
}

// EnrichECSTasks is a Wave 2 enricher for ECS tasks.
// It groups tasks by cluster ARN and calls DescribeTasks (up to 100 per call)
// to surface failures that Wave 1 status coloring cannot detect.
//
// Findings raised (severity "!"):
//   - StopCode == TaskFailedToStart → task never launched
//   - StopCode == EssentialContainerExited → essential container died
//   - Any container with a non-zero ExitCode → container crash detected
func EnrichECSTasks(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
		TruncatedIDs: make(map[string]string),
	}
	if clients.ECS == nil || len(resources) == 0 {
		return result, nil
	}

	// Cap the work list BEFORE grouping: the cap limits how many tasks a9s
	// looked at, and capping the input keeps which tasks those are
	// deterministic (grouping first made it depend on map order) while
	// recording every dropped row as uninspected. Resource.ID IS the task ID
	// (core/aws/ecs_task.go), which is also how TruncatedIDs is keyed below.
	resources = capAtEnrichmentCap(&result, resources, resourceIDsOf)

	// Group task ARNs by cluster ARN.
	clusterTasks := make(map[string][]string)
	taskIDToResource := make(map[string]string) // taskID → resource key (task_id field)
	for _, r := range resources {
		cluster := r.Fields["cluster"]
		taskID := r.Fields["task_id"]
		if cluster == "" || taskID == "" {
			continue
		}
		// Reconstruct task ARN from cluster and task ID (task_id is the last segment).
		// We need to find the full ARN — use the cluster ARN stored in the field.
		// The cluster field stores the full cluster ARN from the fetcher.
		clusterTasks[cluster] = append(clusterTasks[cluster], taskID)
		taskIDToResource[taskID] = taskID
	}

	taskDefByTaskID := make(map[string]string, len(resources))
	truncated := false
	var failures []Failure
	total := 0
	const op = "DescribeTasks"

	// DescribeTasks accepts up to 100 task ARNs per call.
	const descBatch = 100
	for clusterARN, taskIDs := range clusterTasks {
		for i := 0; i < len(taskIDs); i += descBatch {
			end := min(i+descBatch, len(taskIDs))
			batch := taskIDs[i:end]
			total += len(batch)

			out, err := clients.ECS.DescribeTasks(ctx, &ecs.DescribeTasksInput{
				Cluster: aws.String(clusterARN),
				Tasks:   batch,
			})
			if err != nil {
				truncated = true
				for _, taskID := range batch {
					MarkSkipped(&result, taskID, &failures, err)
				}
				continue
			}

			for _, task := range out.Tasks {
				// Identify the resource by task ID (last segment of ARN).
				taskID := ""
				if task.TaskArn != nil {
					parts := strings.Split(*task.TaskArn, "/")
					taskID = parts[len(parts)-1]
				}
				if taskID == "" {
					continue
				}
				if arn := aws.ToString(task.TaskDefinitionArn); arn != "" && !ecsTaskGone(aws.ToString(task.LastStatus)) {
					taskDefByTaskID[taskID] = arn
				}

				var rows []domain.DetailRow

				// Check stop code for known failure modes.
				switch task.StopCode {
				case ecstypes.TaskStopCodeTaskFailedToStart, ecstypes.TaskStopCodeEssentialContainerExited:
					// The plain-English reading of the stop code is the
					// finding's phrase; the row is where the identifier
					// itself is stated, once.
					rows = append(rows, domain.DetailRow{
						Label: "Stop Code",
						Value: string(task.StopCode),
						Tier:  "!",
					})
				}

				// Check containers for non-zero exit codes.
				for _, container := range task.Containers {
					if container.ExitCode != nil && *container.ExitCode != 0 {
						name := ""
						if container.Name != nil {
							name = *container.Name
						}
						rows = append(rows, domain.DetailRow{
							Label: "Container",
							Value: fmt.Sprintf("%s exited with code %d", name, *container.ExitCode),
							Tier:  "!",
						})
					}
				}

				if len(rows) == 0 {
					continue
				}

				setWave2Finding(&result, taskID, ecsTaskCodeTaskFailed, rows)
			}
		}
	}

	if err := ecsTaskDefinitionPosture(ctx, clients, &result, taskDefByTaskID); err != nil {
		failures = append(failures, FailedCall("", err))
	}

	SetTruncated(&result, truncated)
	err := Finish(&result, failures, total, op)
	return result, err
}

// ecsTaskDefinitionPosture describes each DISTINCT task definition once and
// folds its posture findings onto every task running it. Definitions are
// shared by design — a service's tasks all run the same revision — so the
// describe is keyed by definition ARN, never by task.
func ecsTaskDefinitionPosture(ctx context.Context, clients *ServiceClients, result *IssueEnricherResult, taskDefByTaskID map[string]string) error {
	if len(taskDefByTaskID) == 0 {
		return nil
	}
	tasksByDef := make(map[string][]string)
	for taskID, defARN := range taskDefByTaskID {
		tasksByDef[defARN] = append(tasksByDef[defARN], taskID)
	}
	defARNs := make([]string, 0, len(tasksByDef))
	for arn := range tasksByDef {
		defARNs = append(defARNs, arn)
	}
	sort.Strings(defARNs)
	defARNs = capAtEnrichmentCap(result, defARNs, func(arn string) []string { return tasksByDef[arn] })

	const op = "DescribeTaskDefinition"
	var mu sync.Mutex
	var failures []Failure
	_ = ForEachParallel(ctx, len(defARNs), EnrichmentParallelism, func(i int) {
		defARN := defARNs[i]
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ecs.DescribeTaskDefinitionOutput, error) {
			return clients.ECS.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: aws.String(defARN),
			})
		})
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			for _, taskID := range tasksByDef[defARN] {
				MarkSkipped(result, taskID, &failures, err)
			}
			return
		}
		// The service answered and the answer carries no definition. There is
		// no error to read a cause off, so a9s states one: handing this arm a
		// nil error left every task on the definition marked with no reason
		// and the aggregate saying "no reason given" about a call a9s can name.
		if out == nil || out.TaskDefinition == nil {
			for _, taskID := range tasksByDef[defARN] {
				MarkUnusable(result, taskID, &failures, "DescribeTaskDefinition returned no task definition")
			}
			return
		}
		for _, taskID := range tasksByDef[defARN] {
			applyTaskDefinitionFindings(result, taskID, *out.TaskDefinition)
		}
	})

	return Finish(result, failures, len(defARNs), op)
}

// applyTaskDefinitionFindings evaluates every task-definition posture rule
// independently: one definition can trip all five, and each keeps its own
// code, phrase and supporting rows.
func applyTaskDefinitionFindings(result *IssueEnricherResult, taskID string, td ecstypes.TaskDefinition) {
	var privileged, writableRoot, noLogging []domain.DetailRow
	var secretRows []domain.DetailRow
	for _, c := range td.ContainerDefinitions {
		name := aws.ToString(c.Name)
		if c.Privileged != nil && *c.Privileged {
			privileged = append(privileged, domain.DetailRow{Label: "Container", Value: name, Tier: "!"})
		}
		if c.ReadonlyRootFilesystem == nil || !*c.ReadonlyRootFilesystem {
			writableRoot = append(writableRoot, domain.DetailRow{Label: "Container", Value: name, Tier: "~"})
		}
		if c.LogConfiguration == nil {
			noLogging = append(noLogging, domain.DetailRow{Label: "Container", Value: name, Tier: "~"})
		}
		env := make(map[string]string, len(c.Environment))
		for _, kv := range c.Environment {
			if kv.Name != nil {
				env[*kv.Name] = aws.ToString(kv.Value)
			}
		}
		if rows := secretScanRows(env); len(rows) > 0 {
			secretRows = append(secretRows, domain.DetailRow{Label: "Container", Value: name, Tier: "!"})
			secretRows = append(secretRows, rows...)
		}
	}

	if len(privileged) > 0 {
		setWave2Finding(result, taskID, ecsTaskCodePrivileged, privileged)

	}
	var nsRows []domain.DetailRow
	if td.NetworkMode == ecstypes.NetworkModeHost {
		nsRows = append(nsRows, domain.DetailRow{Label: "Network mode", Value: "host", Tier: "~"})
	}
	if td.PidMode == ecstypes.PidModeHost {
		nsRows = append(nsRows, domain.DetailRow{Label: "Process namespace", Value: "host", Tier: "~"})
	}
	if len(nsRows) > 0 {
		setWave2Finding(result, taskID, ecsTaskCodeHostNamespace, nsRows)

	}
	if len(writableRoot) > 0 {
		setWave2Finding(result, taskID, ecsTaskCodeWritableRoot, writableRoot)

	}
	if len(noLogging) > 0 {
		setWave2Finding(result, taskID, ecsTaskCodeNoLogging, noLogging)

	}
	if len(secretRows) > 0 {
		setWave2Finding(result, taskID, ecsTaskCodeEnvSecret, secretRows)

	}
}
