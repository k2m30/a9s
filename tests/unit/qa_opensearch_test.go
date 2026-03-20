package unit

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/resource"
)

// ---------------------------------------------------------------------------
// T-OS01 - Test OpenSearch two-step fetch (ListDomainNames -> DescribeDomains)
// ---------------------------------------------------------------------------

func TestFetchOpenSearchDomains_ParsesMultipleDomains(t *testing.T) {
	instanceCount := int32(3)
	listMock := &mockOpenSearchListDomainNamesClient{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{
				{DomainName: aws.String("search-logs"), EngineType: ostypes.EngineTypeOpenSearch},
				{DomainName: aws.String("search-products"), EngineType: ostypes.EngineTypeOpenSearch},
			},
		},
	}

	describeMock := &mockOpenSearchDescribeDomainsClient{
		output: &opensearch.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{
				{
					DomainName:    aws.String("search-logs"),
					DomainId:      aws.String("123456789012/search-logs"),
					ARN:           aws.String("arn:aws:es:us-east-1:123456789012:domain/search-logs"),
					EngineVersion: aws.String("OpenSearch_2.11"),
					Endpoint:      aws.String("search-logs-abc123def.us-east-1.es.amazonaws.com"),
					ClusterConfig: &ostypes.ClusterConfig{
						InstanceType:  ostypes.OpenSearchPartitionInstanceTypeM5LargeSearch,
						InstanceCount: &instanceCount,
					},
				},
				{
					DomainName:    aws.String("search-products"),
					DomainId:      aws.String("123456789012/search-products"),
					ARN:           aws.String("arn:aws:es:us-east-1:123456789012:domain/search-products"),
					EngineVersion: aws.String("OpenSearch_2.7"),
					Endpoint:      aws.String("search-products-xyz789.us-east-1.es.amazonaws.com"),
					ClusterConfig: &ostypes.ClusterConfig{
						InstanceType:  ostypes.OpenSearchPartitionInstanceTypeR5LargeSearch,
						InstanceCount: &instanceCount,
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}

	// Verify required fields
	requiredFields := []string{"domain_name", "engine_version", "instance_type", "instance_count", "endpoint"}
	for i, r := range resources {
		for _, key := range requiredFields {
			if _, ok := r.Fields[key]; !ok {
				t.Errorf("resource[%d].Fields missing key %q", i, key)
			}
		}
	}

	// Verify first domain
	r0 := resources[0]
	if r0.ID != "search-logs" {
		t.Errorf("resource[0].ID: expected %q, got %q", "search-logs", r0.ID)
	}
	if r0.Name != "search-logs" {
		t.Errorf("resource[0].Name: expected %q, got %q", "search-logs", r0.Name)
	}
	if r0.Fields["engine_version"] != "OpenSearch_2.11" {
		t.Errorf("resource[0].Fields[\"engine_version\"]: expected %q, got %q", "OpenSearch_2.11", r0.Fields["engine_version"])
	}
	if r0.Fields["instance_count"] != "3" {
		t.Errorf("resource[0].Fields[\"instance_count\"]: expected %q, got %q", "3", r0.Fields["instance_count"])
	}

	// Verify RawStruct is set
	if r0.RawStruct == nil {
		t.Error("resource[0].RawStruct should not be nil")
	}

	// Verify RawJSON is non-empty
	if r0.RawJSON == "" {
		t.Error("resource[0].RawJSON should not be empty")
	}
}

func TestFetchOpenSearchDomains_ListError(t *testing.T) {
	listMock := &mockOpenSearchListDomainNamesClient{
		err: fmt.Errorf("AWS API error: access denied"),
	}
	describeMock := &mockOpenSearchDescribeDomainsClient{}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

func TestFetchOpenSearchDomains_EmptyResponse(t *testing.T) {
	listMock := &mockOpenSearchListDomainNamesClient{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{},
		},
	}
	describeMock := &mockOpenSearchDescribeDomainsClient{}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

func TestFetchOpenSearchDomains_DescribeError(t *testing.T) {
	listMock := &mockOpenSearchListDomainNamesClient{
		output: &opensearch.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{
				{DomainName: aws.String("domain-1")},
			},
		},
	}
	describeMock := &mockOpenSearchDescribeDomainsClient{
		err: fmt.Errorf("describe domains failed"),
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if resources != nil {
		t.Errorf("expected nil resources on error, got %d resources", len(resources))
	}
}

// ---------------------------------------------------------------------------
// T-OS02 - Resource type definition
// ---------------------------------------------------------------------------

func TestOpenSearch_ResourceTypeDef(t *testing.T) {
	rt := resource.FindResourceType("opensearch")
	if rt == nil {
		t.Fatal("resource type 'opensearch' not found")
	}

	if rt.Name != "OpenSearch Domains" {
		t.Errorf("expected name %q, got %q", "OpenSearch Domains", rt.Name)
	}

	expected := []struct {
		title string
		key   string
		width int
	}{
		{"Domain Name", "domain_name", 28},
		{"Engine Version", "engine_version", 16},
		{"Instance Type", "instance_type", 22},
		{"Instances", "instance_count", 10},
		{"Endpoint", "endpoint", 48},
	}

	if len(rt.Columns) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(rt.Columns))
	}

	for i, want := range expected {
		col := rt.Columns[i]
		if col.Title != want.title {
			t.Errorf("column %d: expected title %q, got %q", i, want.title, col.Title)
		}
		if col.Key != want.key {
			t.Errorf("column %d (%s): expected key %q, got %q", i, want.title, want.key, col.Key)
		}
		if col.Width != want.width {
			t.Errorf("column %d (%s): expected width %d, got %d", i, want.title, want.width, col.Width)
		}
	}
}

func TestOpenSearch_Aliases(t *testing.T) {
	aliases := []string{"opensearch", "os", "elasticsearch"}
	for _, alias := range aliases {
		rt := resource.FindResourceType(alias)
		if rt == nil {
			t.Errorf("expected resource type for alias %q, got nil", alias)
		}
	}
}
