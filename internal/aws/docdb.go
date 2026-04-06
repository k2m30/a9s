package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("dbc", []string{"cluster_id", "engine_version", "status", "instances", "endpoint"})

	resource.RegisterPaginated("dbc", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchDocDBClustersPage(ctx, c.DocDB, continuationToken)
	})

	resource.RegisterRelated("dbc", []resource.RelatedDef{
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkDbcSG},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkDbcAlarm, NeedsTargetCache: true},
		{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkDbcSecrets},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkDbcLogs, NeedsTargetCache: true},
	})
}

// FetchDocDBClusters calls the DescribeDBClusters API and converts
// the response into a slice of generic Resource structs.
// Returns all DB clusters (Aurora, DocumentDB, Neptune) — no engine filter.
func FetchDocDBClusters(ctx context.Context, api DocDBDescribeDBClustersAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchDocDBClustersPage(ctx, api, token)
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

// FetchDocDBClustersPage fetches a single page of DocumentDB clusters.
func FetchDocDBClustersPage(ctx context.Context, api DocDBDescribeDBClustersAPI, continuationToken string) (resource.FetchResult, error) {
	input := &docdb.DescribeDBClustersInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.DescribeDBClusters(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching DocumentDB clusters: %w", err)
	}

	var resources []resource.Resource

	for _, cluster := range output.DBClusters {
		clusterID := ""
		if cluster.DBClusterIdentifier != nil {
			clusterID = *cluster.DBClusterIdentifier
		}

		engineVersion := ""
		if cluster.EngineVersion != nil {
			engineVersion = *cluster.EngineVersion
		}

		status := ""
		if cluster.Status != nil {
			status = *cluster.Status
		}

		instances := fmt.Sprintf("%d", len(cluster.DBClusterMembers))

		endpoint := ""
		if cluster.Endpoint != nil {
			endpoint = *cluster.Endpoint
		}

		r := resource.Resource{
			ID:     clusterID,
			Name:   clusterID,
			Status: status,
			Fields: map[string]string{
				"cluster_id":     clusterID,
				"engine_version": engineVersion,
				"status":         status,
				"instances":      instances,
				"endpoint":       endpoint,
			},
			RawStruct: cluster,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.Marker != nil {
		nextToken = *output.Marker
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
