package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("msk", []string{"cluster_name", "cluster_type", "state", "version"})

	resource.RegisterPaginated("msk", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchMSKClustersPage(ctx, c.MSK, continuationToken)
	})
}

// computeMSKFindings returns the Wave-1 findings for an MSK cluster state.
// Returns nil for ACTIVE (healthy) clusters.
func computeMSKFindings(state kafkatypes.ClusterState) []domain.Finding {
	switch state {
	case kafkatypes.ClusterStateActive:
		return nil
	case kafkatypes.ClusterStateCreating:
		return []domain.Finding{{Code: CodeMSKCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"}}
	case kafkatypes.ClusterStateUpdating:
		return []domain.Finding{{Code: CodeMSKUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1"}}
	case kafkatypes.ClusterStateMaintenance:
		return []domain.Finding{{Code: CodeMSKMaintenance, Phrase: "maintenance", Severity: domain.SevWarn, Source: "wave1"}}
	case kafkatypes.ClusterStateRebootingBroker:
		return []domain.Finding{{Code: CodeMSKRebootingBroker, Phrase: "rebooting broker", Severity: domain.SevWarn, Source: "wave1"}}
	case kafkatypes.ClusterStateHealing:
		return []domain.Finding{{Code: CodeMSKHealing, Phrase: "healing", Severity: domain.SevWarn, Source: "wave1"}}
	case kafkatypes.ClusterStateDeleting:
		return []domain.Finding{{Code: CodeMSKDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"}}
	case kafkatypes.ClusterStateFailed:
		return []domain.Finding{{Code: CodeMSKFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"}}
	default:
		return nil
	}
}

// FetchMSKClusters calls the MSK ListClustersV2 API and returns a slice of
// generic Resource structs.
func FetchMSKClusters(ctx context.Context, api MSKListClustersV2API) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchMSKClustersPage(ctx, api, token)
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
		statusPhrase := ""
		if len(findings) > 0 {
			statusPhrase = findings[0].Phrase
			if len(findings) > 1 {
				statusPhrase = fmt.Sprintf("%s (+%d)", statusPhrase, len(findings)-1)
			}
		}

		r := resource.Resource{
			ID:   clusterName,
			Name: clusterName,
			Fields: map[string]string{
				"cluster_name": clusterName,
				"cluster_arn":  clusterARN,
				"cluster_type": clusterType,
				"state":        state,
				"status":       statusPhrase,
				"version":      version,
			},
			Findings:  findings,
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
