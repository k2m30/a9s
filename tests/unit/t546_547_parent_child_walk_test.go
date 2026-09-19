package unit

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ngWalkFake lists two clusters and fails ListNodegroups for one of them.
type ngWalkFake struct {
	awsclient.EKSAPI
	failFor    string
	nodegroups map[string][]string
}

func (f *ngWalkFake) ListClusters(_ context.Context, _ *eks.ListClustersInput, _ ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	names := make([]string, 0, len(f.nodegroups))
	for name := range f.nodegroups {
		names = append(names, name)
	}
	slices.Sort(names)
	return &eks.ListClustersOutput{Clusters: names}, nil
}

func (f *ngWalkFake) ListNodegroups(_ context.Context, in *eks.ListNodegroupsInput, _ ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error) {
	cluster := aws.ToString(in.ClusterName)
	if cluster == f.failFor {
		return nil, &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized to perform: eks:ListNodegroups"}
	}
	return &eks.ListNodegroupsOutput{Nodegroups: f.nodegroups[cluster]}, nil
}

func (f *ngWalkFake) DescribeNodegroup(_ context.Context, in *eks.DescribeNodegroupInput, _ ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error) {
	return &eks.DescribeNodegroupOutput{Nodegroup: &ekstypes.Nodegroup{
		NodegroupName: in.NodegroupName,
		ClusterName:   in.ClusterName,
		Status:        ekstypes.NodegroupStatusActive,
	}}, nil
}

// A cluster whose node-group listing is refused is reported as a failure, and
// the node groups of every other cluster are still listed: one cluster's
// denial is not the account's answer.
func TestNodeGroupsList_RefusedClusterIsReportedAndTheRestListed(t *testing.T) {
	fetch := resource.GetPaginatedFetcher("ng")
	if fetch == nil {
		t.Fatal("no paginated fetcher registered for ng")
	}
	f := &ngWalkFake{
		failFor:    "acme-denied",
		nodegroups: map[string][]string{"acme-denied": {"pool-a"}, "acme-prod": {"pool-b"}},
	}
	res, err := fetch(context.Background(), &awsclient.ServiceClients{EKS: f}, "")
	if err == nil {
		t.Fatal("err = nil, want the refused cluster reported")
	}
	if !strings.Contains(err.Error(), "not authorized") {
		t.Errorf("err = %v, want it to carry the refusal AWS gave", err)
	}
	var ids []string
	for _, r := range res.Resources {
		ids = append(ids, r.ID)
	}
	if want := []string{"acme-prod/pool-b"}; !slices.Equal(ids, want) {
		t.Errorf("ids = %v, want %v", ids, want)
	}
}
