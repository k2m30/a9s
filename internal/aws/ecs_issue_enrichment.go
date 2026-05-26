// ecs_issue_enrichment.go — Wave 2 issue enrichment for the ecs resource type.
package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ecs canonical FindingCodes.
const (
	ecsCodeClusterIssue domain.FindingCode = "ecs.cluster-issue"
)

// EnrichECSClusters is a Wave 2 enricher for ECS clusters.
// It calls DescribeClusters with Include=STATISTICS and raises findings for:
//   - pendingTasksCount > 0 → "~" finding (pending tasks indicate scheduling pressure)
//   - runningTasksCount == 0 && registeredContainerInstancesCount > 0 → "~" finding
//     (instances registered but nothing running — likely stuck deployment or misconfiguration)
//
// Note: IssueCount is 0 for this enricher because all findings are severity "~"
// (informational) and do not contribute to the attention menu badge per the
// IssueEnricherResult contract.
func EnrichECSClusters(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string]domain.Finding),
		TruncatedIDs: make(map[string]bool),
	}
	if clients.ECS == nil || len(resources) == 0 {
		return result, nil
	}

	clusterNames := make([]string, 0, len(resources))
	for _, r := range resources {
		if name := r.Fields["cluster_name"]; name != "" {
			clusterNames = append(clusterNames, name)
		}
	}

	truncated := len(resources) > EnrichmentCap
	checked := 0

	// DescribeClusters accepts up to 100 cluster names per call.
	const descBatch = 100
	for i := 0; i < len(clusterNames); i += descBatch {
		if checked >= EnrichmentCap {
			truncated = true
			break
		}
		end := min(i+descBatch, len(clusterNames))
		batch := clusterNames[i:end]
		checked += len(batch)

		out, err := clients.ECS.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: batch,
			Include:  []ecstypes.ClusterField{ecstypes.ClusterFieldStatistics},
		})
		if err != nil {
			truncated = true
			continue
		}

		for _, cluster := range out.Clusters {
			name := ""
			if cluster.ClusterName != nil {
				name = *cluster.ClusterName
			}
			if name == "" {
				continue
			}

			pending := cluster.PendingTasksCount
			running := cluster.RunningTasksCount
			registered := cluster.RegisteredContainerInstancesCount

			var rows []domain.DetailRow
			var summaries []string

			if pending > 0 {
				rows = append(rows, domain.DetailRow{
					Label: "Pending Tasks",
					Value: fmt.Sprintf("%d tasks pending", pending),
					Tier:  "~",
				})
				summaries = append(summaries, fmt.Sprintf("%d pending tasks", pending))
			}

			if running == 0 && registered > 0 {
				rows = append(rows, domain.DetailRow{
					Label: "Tasks",
					Value: fmt.Sprintf("no running tasks (%d container instances registered)", registered),
					Tier:  "~",
				})
				summaries = append(summaries, "no running tasks but instances registered")
			}

			if len(rows) == 0 {
				continue
			}

			summary := strings.Join(summaries, "; ")
			setWave2Finding(&result, name, ecsCodeClusterIssue, summary, "~", "ecs", rows)
		}
	}

	// IssueCount is 0: all ECS cluster findings are "~" (informational) and
	// do not contribute to the attention menu badge.
	result.IssueCount = 0
	result.Truncated = truncated
	return result, nil
}
