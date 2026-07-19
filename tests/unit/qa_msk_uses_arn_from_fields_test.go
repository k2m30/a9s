package unit

// qa_msk_uses_arn_from_fields_test.go — Regression: EnrichMSKCluster must call
// DescribeClusterV2 with the cluster ARN from r.Fields["cluster_arn"], NOT the
// bare cluster name in r.ID.
//
// Same shape as tg/sfn/elb/acm. msk fetcher (msk.go) sets `ID: clusterName`
// and stores the ARN in Fields["cluster_arn"].

import (
	"context"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestEnrichMSKCluster_UsesARNFromFields verifies the enricher passes
// r.Fields["cluster_arn"] to DescribeClusterV2, not r.ID.
func TestEnrichMSKCluster_UsesARNFromFields(t *testing.T) {
	const clusterName = "prod-kafka"
	const clusterARN = "arn:aws:kafka:us-east-1:123456789012:cluster/prod-kafka/abc123-def456"

	fake := &fakeMSKDescribeClusterV2{RejectNonARN: true}
	clients := &awsclient.ServiceClients{MSK: fake}
	resources := []resource.Resource{{
		ID:     clusterName,
		Name:   clusterName,
		Fields: map[string]string{"cluster_arn": clusterARN},
	}}

	_, err := awsclient.EnrichMSKCluster(context.Background(), clients, resources, nil)
	if err != nil && strings.Contains(err.Error(), "ValidationError") {
		t.Fatalf("enricher passed bare cluster name to AWS instead of ARN; got: %v", err)
	}
	if got := fake.CalledWith(); got != clusterARN {
		t.Errorf("DescribeClusterV2 was called with %q, want %q (ARN from Fields[\"cluster_arn\"])",
			got, clusterARN)
	}
}
