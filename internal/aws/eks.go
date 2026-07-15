package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

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
			resources = append(resources, DegradedDetails("eks", name, &ekstypes.Cluster{Name: aws.String(name)}, descErr))
			continue
		}
		if descOutput.Cluster == nil {
			failures = append(failures, fmt.Sprintf("%s: nil cluster in response", name))
			resources = append(resources, DegradedDetails("eks", name, &ekstypes.Cluster{Name: aws.String(name)}, nil))
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
			issueCodes = append(issueCodes, domain.HumanizeStatusPhrase(string(issue.Code)))
		}
	}

	subnetIDs := ""
	if cluster.ResourcesVpcConfig != nil {
		subnetIDs = strings.Join(cluster.ResourcesVpcConfig.SubnetIds, ",")
	}

	r := resource.Resource{
		ID:   name,
		Name: clusterName,
		// Status intentionally unset — lifecycle state is emitted as a Finding.
		Fields: map[string]string{
			"cluster_name":        clusterName,
			"version":             version,
			"status":              status,
			"endpoint":            endpoint,
			"platform_version":    platformVersion,
			"arn":                 aws.ToString(cluster.Arn),
			"health_issues_count": strconv.Itoa(healthIssuesCount),
			"health_issues":       strings.Join(issueCodes, ", "),
			"subnet_ids":          subnetIDs,
		},
		RawStruct: cluster,
	}

	// emit canonical Findings for non-healthy lifecycle states, mirroring
	// colorEKSCluster's own precedence (catalog_containers.go): FAILED wins
	// outright (SevBroken, folding in Health.Issues detail if present),
	// then CREATING/UPDATING (SevWarn), then a bare Health.Issues[] on an
	// otherwise-healthy cluster (SevWarn — health is tracked independently
	// of lifecycle state, docs/resources/eks.md §3.2).
	switch {
	case cluster.Status == ekstypes.ClusterStatusFailed:
		r.Findings = []domain.Finding{healthIssueFinding(CodeEKSStateFailed, "failed", issueCodes)}
	case cluster.Status == ekstypes.ClusterStatusCreating:
		r.Findings = []domain.Finding{{
			Code: CodeEKSStateCreating, Phrase: "creating",
			Severity: domain.SevWarn, Source: "wave1",
		}}
	case cluster.Status == ekstypes.ClusterStatusUpdating:
		r.Findings = []domain.Finding{{
			Code: CodeEKSStateUpdating, Phrase: "updating",
			Severity: domain.SevWarn, Source: "wave1",
		}}
	case healthIssuesCount > 0:
		r.Findings = []domain.Finding{healthIssueWarnFinding(CodeEKSHealthIssue, issueCodes)}
	}

	return r
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
