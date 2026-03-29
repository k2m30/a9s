package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/redshift"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("redshift", []string{"cluster_id", "status", "node_type", "num_nodes", "db_name", "endpoint"})

	resource.RegisterPaginated("redshift", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRedshiftClustersPage(ctx, c.Redshift, continuationToken)
	})
}

// FetchRedshiftClusters calls the Redshift DescribeClusters API and converts the
// response into a slice of generic Resource structs.
func FetchRedshiftClusters(ctx context.Context, api RedshiftDescribeClustersAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchRedshiftClustersPage(ctx, api, token)
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

// FetchRedshiftClustersPage fetches a single page of Redshift clusters.
func FetchRedshiftClustersPage(ctx context.Context, api RedshiftDescribeClustersAPI, continuationToken string) (resource.FetchResult, error) {
	input := &redshift.DescribeClustersInput{}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.DescribeClusters(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching Redshift clusters: %w", err)
	}

	var resources []resource.Resource

	for _, cluster := range output.Clusters {
		clusterID := ""
		if cluster.ClusterIdentifier != nil {
			clusterID = *cluster.ClusterIdentifier
		}

		status := ""
		if cluster.ClusterStatus != nil {
			status = *cluster.ClusterStatus
		}

		nodeType := ""
		if cluster.NodeType != nil {
			nodeType = *cluster.NodeType
		}

		numNodes := ""
		if cluster.NumberOfNodes != nil {
			numNodes = strconv.Itoa(int(*cluster.NumberOfNodes))
		}

		dbName := ""
		if cluster.DBName != nil {
			dbName = *cluster.DBName
		}

		endpoint := ""
		if cluster.Endpoint != nil && cluster.Endpoint.Address != nil {
			endpoint = *cluster.Endpoint.Address
		}

		masterUser := ""
		if cluster.MasterUsername != nil {
			masterUser = *cluster.MasterUsername
		}

		createTime := ""
		if cluster.ClusterCreateTime != nil {
			createTime = cluster.ClusterCreateTime.Format("2006-01-02 15:04")
		}

		r := resource.Resource{
			ID:     clusterID,
			Name:   clusterID,
			Status: status,
			Fields: map[string]string{
				"cluster_id":  clusterID,
				"status":      status,
				"node_type":   nodeType,
				"num_nodes":   numNodes,
				"db_name":     dbName,
				"endpoint":    endpoint,
				"master_user": masterUser,
				"create_time": createTime,
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
