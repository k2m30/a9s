package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// FetchECSClusters performs a two-step fetch: ListClusters to get ARNs,
// then DescribeClusters for full details.
func FetchECSClusters(ctx context.Context, listAPI ECSListClustersAPI, describeAPI ECSDescribeClustersAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchECSClustersPage(ctx, listAPI, describeAPI, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

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

	if len(listOutput.ClusterArns) == 0 {
		return resource.FetchResult{
			Resources: nil,
			Pagination: &resource.PaginationMeta{
				IsTruncated: false,
				TotalHint:   0,
				PageSize:    0,
			},
		}, nil
	}

	descOutput, err := describeAPI.DescribeClusters(ctx, &ecs.DescribeClustersInput{
		Clusters: listOutput.ClusterArns,
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("describing ECS clusters: %w", err)
	}

	var resources []resource.Resource

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

		// PR-03c: emit wave1 Findings for non-healthy lifecycle states.
		// ACTIVE → no Finding (healthy). Fields["status"] is still populated
		// so the existing structural Color path works as fallback.
		var findings []domain.Finding
		switch status {
		case "PROVISIONING":
			findings = []domain.Finding{{Code: CodeECSStateProvisioning, Phrase: "provisioning", Severity: domain.SevWarn, Source: "wave1"}}
		case "DEPROVISIONING":
			findings = []domain.Finding{{Code: CodeECSStateDeprovisioning, Phrase: "deprovisioning", Severity: domain.SevWarn, Source: "wave1"}}
		case "FAILED":
			findings = []domain.Finding{{Code: CodeECSStateFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"}}
		case "INACTIVE":
			findings = []domain.Finding{{Code: CodeECSStateInactive, Phrase: "inactive", Severity: domain.SevBroken, Source: "wave1"}}
		}

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
