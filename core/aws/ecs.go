// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

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
	var failures []Failure

	// AWS treats an empty/omitted Clusters list on DescribeClusters as "describe
	// the default cluster" rather than "describe nothing" — so this call must be
	// skipped, not made with a possibly-empty ClusterArns, when this page found
	// no cluster ARNs to describe.
	if len(listOutput.ClusterArns) > 0 {
		// Configuration and Tags are returned only when named in Include; the
		// related checkers read the execute-command key and the
		// CloudFormation stack tag from them.
		descOutput, err := describeAPI.DescribeClusters(ctx, &ecs.DescribeClustersInput{
			Clusters: listOutput.ClusterArns,
			Include:  []ecstypes.ClusterField{ecstypes.ClusterFieldConfigurations, ecstypes.ClusterFieldTags},
		})
		var ans listedAnswer
		var clusters []ecstypes.Cluster
		if err == nil {
			clusters, ans.gone = descOutput.Clusters, ecsGone(descOutput.Failures)
		}
		for _, cluster := range clusters {
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

			r := withWave1(resource.Resource{
				ID:   clusterName,
				Name: clusterName,
				Fields: map[string]string{
					"cluster_name":   clusterName,
					"status":         status,
					"running_tasks":  runningTasks,
					"pending_tasks":  pendingTasks,
					"services_count": servicesCount,
				},
				RawStruct: cluster,
			}, withState("ecs", status, nil)...)
			ans.rows = append(ans.rows, r)
		}
		resources, failures = keepListed("DescribeClusters", listOutput.ClusterArns, ecsClusterRowID, ans, err, func(arn string, err error) resource.Resource {
			name := ecsClusterRowID(arn)
			r := degradedRow(name, err)
			r.Fields["cluster_name"] = name
			return r
		})
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
	}, AggregateFailures("ecs: DescribeClusters", failures, len(listOutput.ClusterArns))
}

// ecsClusterStatuses is the Cluster.status domain the Amazon ECS API
// Reference lists ("The following are the possible states that are
// returned"); the SDK types it as a plain string.
// https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Cluster.html
var ecsClusterStatuses = []string{"ACTIVE", "PROVISIONING", "DEPROVISIONING", "FAILED", "INACTIVE"} //nolint:gochecknoglobals // static AWS status domain

// ecsGone reads the names an ECS Describe* call reports as no longer
// existing: a Failure whose Reason is MISSING.
func ecsGone(failures []ecstypes.Failure) map[string]bool {
	gone := map[string]bool{}
	for _, f := range failures {
		if aws.ToString(f.Reason) == "MISSING" {
			gone[aws.ToString(f.Arn)] = true
		}
	}
	return gone
}

func ecsClusterRowID(arn string) string {
	id, _ := ecsRefToID(arn, domain.RefContext{})
	return id
}

func ecsTaskRowID(arn string) string {
	id, _ := ecsTaskRefToID(arn, domain.RefContext{})
	return id
}
