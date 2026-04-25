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

	"github.com/aws/aws-sdk-go-v2/aws"
	kafkasvc "github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	smithy "github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// strictMSKFake mirrors AWS: rejects DescribeClusterV2 when ClusterArn is not
// a valid ARN.
type strictMSKFake struct {
	awsclient.MSKAPI
	calledWith string
}

func (f *strictMSKFake) DescribeClusterV2(
	_ context.Context,
	input *kafkasvc.DescribeClusterV2Input,
	_ ...func(*kafkasvc.Options),
) (*kafkasvc.DescribeClusterV2Output, error) {
	got := aws.ToString(input.ClusterArn)
	f.calledWith = got
	if !strings.HasPrefix(got, "arn:aws:") {
		return nil, &smithy.GenericAPIError{
			Code:    "ValidationError",
			Message: "'" + got + "' is not a valid ARN",
		}
	}
	return &kafkasvc.DescribeClusterV2Output{
		ClusterInfo: &kafkatypes.Cluster{},
	}, nil
}

// TestEnrichMSKCluster_UsesARNFromFields verifies the enricher passes
// r.Fields["cluster_arn"] to DescribeClusterV2, not r.ID.
func TestEnrichMSKCluster_UsesARNFromFields(t *testing.T) {
	const clusterName = "prod-kafka"
	const clusterARN = "arn:aws:kafka:us-east-1:123456789012:cluster/prod-kafka/abc123-def456"

	fake := &strictMSKFake{}
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
	if fake.calledWith != clusterARN {
		t.Errorf("DescribeClusterV2 was called with %q, want %q (ARN from Fields[\"cluster_arn\"])",
			fake.calledWith, clusterARN)
	}
}
