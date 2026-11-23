// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kafka"

	"github.com/k2m30/a9s/v3/core/resource"
)

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

		// CurrentVersion is the opaque revision token an update call carries;
		// the version an operator reads on a Kafka cluster is the broker
		// software's. A serverless cluster reports none.
		version := ""
		if cluster.Provisioned != nil && cluster.Provisioned.CurrentBrokerSoftwareInfo != nil {
			version = aws.ToString(cluster.Provisioned.CurrentBrokerSoftwareInfo.KafkaVersion)
		}

		clusterARN := ""
		if cluster.ClusterArn != nil {
			clusterARN = *cluster.ClusterArn
		}

		findings := withState("msk", state, nil)

		r := resource.Resource{
			ID:       clusterName,
			Name:     clusterName,
			Findings: findings,
			Fields: map[string]string{
				"cluster_name": clusterName,
				"cluster_arn":  clusterARN,
				"cluster_type": clusterType,
				"state":        state,
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
