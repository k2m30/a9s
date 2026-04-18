package unit

// qa_opensearch_status_field_test.go — P2.4 verification test.
//
// The reviewer claimed defaults_databases.go uses Key: "status" for OpenSearch
// but the fetcher doesn't write Fields["status"]. This test verifies whether
// that claim is correct.
//
// If this test PASSES: the reviewer was wrong — Fields["status"] is populated.
// If this test FAILS:  the reviewer was right — a fix is needed in opensearch.go.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// TestFetchOpenSearchDomains_PopulatesFieldsStatus verifies that every resource
// returned by FetchOpenSearchDomains has a non-empty Fields["status"].
//
// The Status column in defaults_databases.go uses Key: "status", so a blank
// Fields["status"] would render an empty Status column for every domain.
func TestFetchOpenSearchDomains_PopulatesFieldsStatus(t *testing.T) {
	instanceCount := int32(2)
	listMock := &mockOpenSearchListDomainNamesClient{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{
				{DomainName: aws.String("search-available"), EngineType: ostypes.EngineTypeOpenSearch},
				{DomainName: aws.String("search-processing"), EngineType: ostypes.EngineTypeOpenSearch},
				{DomainName: aws.String("search-deleted"), EngineType: ostypes.EngineTypeOpenSearch},
			},
		},
	}

	processing := true
	deleted := true

	describeMock := &mockOpenSearchDescribeDomainsClient{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{
				{
					// Available domain: Processing=nil, Deleted=nil
					DomainName:    aws.String("search-available"),
					DomainId:      aws.String("123456789012/search-available"),
					ARN:           aws.String("arn:aws:es:us-east-1:123456789012:domain/search-available"),
					EngineVersion: aws.String("OpenSearch_2.11"),
					Endpoint:      aws.String("search-available.us-east-1.es.amazonaws.com"),
					ClusterConfig: &ostypes.ClusterConfig{
						InstanceType:  ostypes.OpenSearchPartitionInstanceTypeM5LargeSearch,
						InstanceCount: &instanceCount,
					},
				},
				{
					// Processing domain: Processing=true
					DomainName:    aws.String("search-processing"),
					DomainId:      aws.String("123456789012/search-processing"),
					ARN:           aws.String("arn:aws:es:us-east-1:123456789012:domain/search-processing"),
					EngineVersion: aws.String("OpenSearch_2.7"),
					Endpoint:      aws.String("search-processing.us-east-1.es.amazonaws.com"),
					Processing:    &processing,
					ClusterConfig: &ostypes.ClusterConfig{
						InstanceType:  ostypes.OpenSearchPartitionInstanceTypeR5LargeSearch,
						InstanceCount: &instanceCount,
					},
				},
				{
					// Deleted domain: Deleted=true
					DomainName:    aws.String("search-deleted"),
					DomainId:      aws.String("123456789012/search-deleted"),
					ARN:           aws.String("arn:aws:es:us-east-1:123456789012:domain/search-deleted"),
					EngineVersion: aws.String("OpenSearch_1.3"),
					Deleted:       &deleted,
					ClusterConfig: &ostypes.ClusterConfig{
						InstanceType:  ostypes.OpenSearchPartitionInstanceTypeT3MediumSearch,
						InstanceCount: &instanceCount,
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 3 {
		t.Fatalf("expected 3 resources, got %d", len(resources))
	}

	for _, r := range resources {
		if r.Fields["status"] == "" {
			t.Errorf("domain %q has empty Fields[\"status\"] — Status column would render blank (P2.4 bug)", r.Name)
		}
	}

	// Verify the specific status values are what the fetcher derives.
	statusByName := make(map[string]string, len(resources))
	for _, r := range resources {
		statusByName[r.Name] = r.Fields["status"]
	}

	if statusByName["search-available"] != "available" {
		t.Errorf("search-available: expected Fields[\"status\"]=%q, got %q", "available", statusByName["search-available"])
	}
	if statusByName["search-processing"] != "processing" {
		t.Errorf("search-processing: expected Fields[\"status\"]=%q, got %q", "processing", statusByName["search-processing"])
	}
	if statusByName["search-deleted"] != "deleted" {
		t.Errorf("search-deleted: expected Fields[\"status\"]=%q, got %q", "deleted", statusByName["search-deleted"])
	}
}
