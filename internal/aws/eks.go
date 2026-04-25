package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("eks", []string{"cluster_name", "version", "status", "endpoint", "platform_version", "arn", "health_issues_count", "health_issues"})

	resource.RegisterPaginated("eks", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEKSClustersPage(ctx, c, continuationToken)
	})
}

// FetchEKSClustersPage fetches a single page of EKS clusters using the registered
// paginated fetcher pattern. For each cluster name returned by ListClusters,
// DescribeCluster is called. Per-item describe failures are aggregated into a
// composite error returned alongside partial results (E2, E3, E5).
func FetchEKSClustersPage(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
	input := &eks.ListClustersInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = aws.String(continuationToken)
	}

	listOutput, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.ListClustersOutput, error) {
		return c.EKS.ListClusters(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("listing EKS clusters: %w", err)
	}

	total := len(listOutput.Clusters)
	var resources []resource.Resource
	var failures []string
	for _, name := range listOutput.Clusters {
		descOutput, descErr := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*eks.DescribeClusterOutput, error) {
			return c.EKS.DescribeCluster(ctx, &eks.DescribeClusterInput{
				Name: aws.String(name),
			})
		})
		if descErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", name, descErr.Error()))
			continue
		}
		if descOutput.Cluster == nil {
			failures = append(failures, fmt.Sprintf("%s: nil cluster in response", name))
			continue
		}
		resources = append(resources, buildEKSResource(name, descOutput.Cluster))
	}

	isTruncated := listOutput.NextToken != nil
	var nextToken string
	if listOutput.NextToken != nil {
		nextToken = *listOutput.NextToken
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   -1,
		},
	}, AggregateFailures("eks: DescribeCluster", failures, total)
}

// buildEKSResource constructs a Resource from a cluster name and EKS Cluster struct.
func buildEKSResource(name string, cluster *ekstypes.Cluster) resource.Resource {
	clusterName := ""
	if cluster.Name != nil {
		clusterName = *cluster.Name
	}

	version := ""
	if cluster.Version != nil {
		version = *cluster.Version
	}

	status := string(cluster.Status)

	endpoint := ""
	if cluster.Endpoint != nil {
		endpoint = *cluster.Endpoint
	}

	platformVersion := ""
	if cluster.PlatformVersion != nil {
		platformVersion = *cluster.PlatformVersion
	}

	// Wave 2: health.issues[] — populated by DescribeCluster (called per cluster in fetcher).
	healthIssuesCount := 0
	var issueCodes []string
	if cluster.Health != nil {
		for _, issue := range cluster.Health.Issues {
			healthIssuesCount++
			issueCodes = append(issueCodes, string(issue.Code))
		}
	}

	return resource.Resource{
		ID:     name,
		Name:   clusterName,
		Status: status,
		Fields: map[string]string{
			"cluster_name":        clusterName,
			"version":             version,
			"status":              status,
			"endpoint":            endpoint,
			"platform_version":    platformVersion,
			"arn":                 aws.ToString(cluster.Arn),
			"health_issues_count": strconv.Itoa(healthIssuesCount),
			"health_issues":       strings.Join(issueCodes, ","),
		},
		RawStruct: cluster,
	}
}

// FetchEKSClusters performs a two-step fetch: ListClusters to get cluster names
// (paginated via NextToken), then DescribeCluster for each name to get full details.
func FetchEKSClusters(ctx context.Context, listAPI EKSListClustersAPI, describeAPI EKSDescribeClusterAPI) ([]resource.Resource, error) {
	// Step 1: Collect all cluster names across pages
	var allClusters []string
	var nextToken *string

	for {
		listOutput, err := listAPI.ListClusters(ctx, &eks.ListClustersInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing EKS clusters: %w", err)
		}

		allClusters = append(allClusters, listOutput.Clusters...)

		if listOutput.NextToken == nil {
			break
		}
		nextToken = listOutput.NextToken
	}

	// Step 2: Describe each cluster
	var resources []resource.Resource

	for _, clusterName := range allClusters {
		descOutput, err := describeAPI.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: aws.String(clusterName),
		})
		if err != nil {
			return nil, fmt.Errorf("describing EKS cluster %s: %w", clusterName, err)
		}

		if descOutput.Cluster == nil {
			continue
		}

		resources = append(resources, buildEKSResource(clusterName, descOutput.Cluster))
	}

	return resources, nil
}
