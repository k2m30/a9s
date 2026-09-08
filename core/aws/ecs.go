// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchECSClustersPage fetches a single page of ECS clusters.
// It paginates ListClusters using continuationToken, then calls DescribeClusters
// for the batch of ARNs returned on that page.
func FetchECSClustersPage(ctx context.Context, listAPI ECSListClustersAPI, describeAPI ECSDescribeClustersAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ecs.ListClustersInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	listOutput, err := listAPI.ListClusters(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing ECS clusters: %w", err)
	}

	var resources []resource.Resource

	// AWS treats an empty/omitted Clusters list on DescribeClusters as "describe
	// the default cluster" rather than "describe nothing" — so this call must be
	// skipped, not made with a possibly-empty ClusterArns, when this page found
	// no cluster ARNs to describe.
	if len(listOutput.ClusterArns) > 0 {
		descOutput, err := describeAPI.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: listOutput.ClusterArns,
		})
		if err != nil {
			return resource.FetchResult{}, fmt.Errorf("describing ECS clusters: %w", err)
		}

		for _, cluster := range descOutput.Clusters {
			clusterName := ""
			if cluster.ClusterName != nil {
				clusterName = *cluster.ClusterName
			}

			status := ""
			if cluster.Status != nil {
				status = *cluster.Status
			}

			runningTasks := fmt.Sprintf("%d", cluster.RunningTasksCount)
			pendingTasks := fmt.Sprintf("%d", cluster.PendingTasksCount)
			servicesCount := fmt.Sprintf("%d", cluster.ActiveServicesCount)

			findings := ecsClusterFindings(status)

			r := resource.Resource{
				ID:   clusterName,
				Name: clusterName,
				Fields: map[string]string{
					"cluster_name":   clusterName,
					"status":         status,
					"running_tasks":  runningTasks,
					"pending_tasks":  pendingTasks,
					"services_count": servicesCount,
				},
				Findings:  findings,
				RawStruct: cluster,
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

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}

// ecsClusterFindings is the one predicate for a cluster's lifecycle state.
// ACTIVE reports nothing. colorECSCluster runs it over Fields for rows built
// outside the fetcher.
func ecsClusterFindings(status string) []domain.Finding {
	switch status {
	case "PROVISIONING":
		return []domain.Finding{wave1Finding(CodeECSStateProvisioning)}
	case "DEPROVISIONING":
		return []domain.Finding{wave1Finding(CodeECSStateDeprovisioning)}
	case "FAILED":
		return []domain.Finding{wave1Finding(CodeECSStateFailed)}
	case "INACTIVE":
		return []domain.Finding{wave1Finding(CodeECSStateInactive)}
	}
	return nil
}
