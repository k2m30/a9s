// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// ecs_issue_enrichment.go — Wave 2 issue enrichment for the ecs resource type.
package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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
func EnrichECSClusters(ctx context.Context, clients *ServiceClients, resources []resource.Resource, _ resource.ResourceCache) (IssueEnricherResult, error) {
	result := IssueEnricherResult{
		Findings:     make(map[string][]domain.Finding),
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

	clusterNames = capAtEnrichmentCap(&result, clusterNames, func(n string) []string { return []string{n} })

	// DescribeClusters accepts up to 100 cluster names per call.
	const descBatch = 100
	var failures []Failure
	for i := 0; i < len(clusterNames); i += descBatch {
		end := min(i+descBatch, len(clusterNames))
		batch := clusterNames[i:end]

		out, err := clients.ECS.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: batch,
			Include:  []ecstypes.ClusterField{ecstypes.ClusterFieldStatistics},
		})
		if err != nil {
			// The whole batch was skipped — mark each cluster so its row shows a
			// "?" coverage gap. A "~"-only enricher never truncates the issue
			// badge, but a failed batch must not vanish silently (no finding, no
			// badge, no "?").
			for _, name := range batch {
				MarkSkipped(&result, name, &failures, err)
			}
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

			if pending > 0 {
				rows = append(rows, domain.DetailRow{
					Label: "Pending Tasks",
					Value: fmt.Sprintf("%d tasks pending", pending),
					Tier:  "~",
				})
			}

			if running == 0 && registered > 0 {
				rows = append(rows, domain.DetailRow{
					Label: "Tasks",
					Value: fmt.Sprintf("no running tasks (%d container instances registered)", registered),
					Tier:  "~",
				})
			}

			if len(rows) == 0 {
				continue
			}

			setWave2Finding(&result, name, ecsCodeClusterIssue,
				catalog.Phrase(ecsCodeClusterIssue), "~", "ecs", rows)
		}
	}

	MarkInformationalOnly(&result)
	return result, AggregateFailures("ecs-enrich: DescribeClusters", failures, len(clusterNames))
}
