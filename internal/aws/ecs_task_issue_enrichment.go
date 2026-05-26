// ecs_task_issue_enrichment.go — Wave 2 issue enrichment for the ecs-task resource type.
package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ecs-task canonical FindingCodes.
const (
	ecsTaskCodeTaskFailed domain.FindingCode = "ecs-task.task-failed"
)

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
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.ECS == nil || len(resources) == 0 {
		return result, nil
	}

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

	truncated := len(resources) > EnrichmentCap
	checked := 0

	// DescribeTasks accepts up to 100 task ARNs per call.
	const descBatch = 100
	for clusterARN, taskIDs := range clusterTasks {
		for i := 0; i < len(taskIDs); i += descBatch {
			if checked >= EnrichmentCap {
				truncated = true
				break
			}
			end := min(i+descBatch, len(taskIDs))
			batch := taskIDs[i:end]
			checked += len(batch)

			out, err := clients.ECS.DescribeTasks(ctx, &ecs.DescribeTasksInput{
				Cluster: aws.String(clusterARN),
				Tasks:   batch,
			})
			if err != nil {
				truncated = true
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

				var rows []domain.DetailRow

				// Check stop code for known failure modes.
				switch task.StopCode {
				case ecstypes.TaskStopCodeTaskFailedToStart:
					rows = append(rows, domain.DetailRow{
						Label: "Stop Code",
						Value: "TaskFailedToStart — task never launched",
						Tier:  "!",
					})
				case ecstypes.TaskStopCodeEssentialContainerExited:
					rows = append(rows, domain.DetailRow{
						Label: "Stop Code",
						Value: "EssentialContainerExited — essential container died",
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
						break // One finding per task is sufficient.
					}
				}

				if len(rows) == 0 {
					continue
				}

				summary := rows[0].Value
				setWave2Finding(&result, taskID, ecsTaskCodeTaskFailed, summary, "!", "ecs-task", rows)
			}
		}
	}

	result.IssueCount = len(result.Findings)
	result.Truncated = truncated
	return result, nil
}
