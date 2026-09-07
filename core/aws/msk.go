// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// computeMSKFindings returns a []domain.Finding for the given MSK cluster state.
func computeMSKFindings(state kafkatypes.ClusterState) []domain.Finding {
	switch state {
	case kafkatypes.ClusterStateFailed:
		return []domain.Finding{wave1Finding(CodeMSKFailed, domain.SevBroken)}
	case kafkatypes.ClusterStateCreating:
		return []domain.Finding{wave1Finding(CodeMSKCreating, domain.SevWarn)}
	case kafkatypes.ClusterStateUpdating:
		return []domain.Finding{wave1Finding(CodeMSKUpdating, domain.SevWarn)}
	case kafkatypes.ClusterStateMaintenance:
		return []domain.Finding{wave1Finding(CodeMSKMaintenance, domain.SevWarn)}
	case kafkatypes.ClusterStateRebootingBroker:
		return []domain.Finding{wave1Finding(CodeMSKRebootingBroker, domain.SevWarn)}
	case kafkatypes.ClusterStateHealing:
		return []domain.Finding{wave1Finding(CodeMSKHealing, domain.SevWarn)}
	case kafkatypes.ClusterStateDeleting:
		return []domain.Finding{wave1Finding(CodeMSKDeleting, domain.SevWarn)}
	default:
		return nil
	}
}

// FetchMSKClustersPage fetches a single page of MSK clusters.
func FetchMSKClustersPage(ctx context.Context, api MSKListClustersV2API, continuationToken string) (resource.FetchResult, error) {
	input := &kafka.ListClustersV2Input{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.ListClustersV2(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching MSK clusters: %w", err)
	}

	var resources []resource.Resource

	for _, cluster := range output.ClusterInfoList {
		clusterName := ""
		if cluster.ClusterName != nil {
			clusterName = *cluster.ClusterName
		}

		clusterType := string(cluster.ClusterType)
		state := string(cluster.State)

		version := ""
		if cluster.CurrentVersion != nil {
			version = *cluster.CurrentVersion
		}

		clusterARN := ""
		if cluster.ClusterArn != nil {
			clusterARN = *cluster.ClusterArn
		}

		findings := computeMSKFindings(cluster.State)
		statusPhrase := domain.StatusPhrase(findings)

		r := resource.Resource{
			ID:       clusterName,
			Name:     clusterName,
			Findings: findings,
			Fields: map[string]string{
				"cluster_name": clusterName,
				"cluster_arn":  clusterARN,
				"cluster_type": clusterType,
				"state":        state,
				"status":       statusPhrase,
				"version":      version,
			},
			RawStruct: cluster,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
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
