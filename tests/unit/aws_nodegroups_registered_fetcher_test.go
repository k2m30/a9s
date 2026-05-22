package unit

// aws_nodegroups_registered_fetcher_test.go — Tests for the registered paginated fetcher
// for "ng" (resource.GetPaginatedFetcher("ng")), which is the path used by the live app
// via internal/tui/app_fetchers.go.
//
// These tests are DISTINCT from aws_nodegroups_image_id_test.go, which only exercises
// the standalone FetchNodeGroups helper. This file targets the closure registered in
// internal/aws/ng.go via resource.SetPaginatedForTest("ng", ...).

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/resource"

	// Side-effect import: triggers init() in internal/aws which calls
	// resource.SetPaginatedForTest("ng", ...) — required so GetPaginatedFetcher("ng") is non-nil.
	_ "github.com/k2m30/a9s/v3/internal/aws"
)

// ---------------------------------------------------------------------------
// Minimal EKSAPI fake for registered-fetcher tests
// ---------------------------------------------------------------------------

// ngTestEKSFake implements awsclient.EKSAPI with programmable per-cluster, per-nodegroup
// responses for the three operations the registered "ng" fetcher calls.
type ngTestEKSFake struct {
	// clusters returned by ListClusters
	clusters []string
	// nodegroups keyed by cluster name
	nodegroups map[string][]string
	// nodegroup details keyed by "cluster/nodegroup"
	nodegroupDetails map[string]*ekstypes.Nodegroup
}

func (f *ngTestEKSFake) ListClusters(
	_ context.Context,
	_ *eks.ListClustersInput,
	_ ...func(*eks.Options),
) (*eks.ListClustersOutput, error) {
	return &eks.ListClustersOutput{Clusters: f.clusters}, nil
}

func (f *ngTestEKSFake) DescribeCluster(
	_ context.Context,
	input *eks.DescribeClusterInput,
	_ ...func(*eks.Options),
) (*eks.DescribeClusterOutput, error) {
	// Not called by the registered "ng" fetcher; return a stub.
	name := aws.ToString(input.Name)
	return &eks.DescribeClusterOutput{
		Cluster: &ekstypes.Cluster{Name: aws.String(name)},
	}, nil
}

func (f *ngTestEKSFake) ListNodegroups(
	_ context.Context,
	input *eks.ListNodegroupsInput,
	_ ...func(*eks.Options),
) (*eks.ListNodegroupsOutput, error) {
	cluster := aws.ToString(input.ClusterName)
	ngs := f.nodegroups[cluster]
	return &eks.ListNodegroupsOutput{Nodegroups: ngs}, nil
}

func (f *ngTestEKSFake) DescribeNodegroup(
	_ context.Context,
	input *eks.DescribeNodegroupInput,
	_ ...func(*eks.Options),
) (*eks.DescribeNodegroupOutput, error) {
	key := aws.ToString(input.ClusterName) + "/" + aws.ToString(input.NodegroupName)
	ng := f.nodegroupDetails[key]
	return &eks.DescribeNodegroupOutput{Nodegroup: ng}, nil
}

// ---------------------------------------------------------------------------
// Minimal EC2API fake for registered-fetcher tests
//
// We embed *fakes.EC2Fake (which satisfies the full EC2API) and override
// DescribeLaunchTemplateVersions to return test-controlled data.
// ---------------------------------------------------------------------------

// ngTestEC2Fake wraps fakes.EC2Fake and overrides DescribeLaunchTemplateVersions.
type ngTestEC2Fake struct {
	*fakes.EC2Fake
	// ltOutputs keyed by "<launchTemplateId>:<version>"
	ltOutputs map[string]*ec2.DescribeLaunchTemplateVersionsOutput
	// ltErr, if non-nil, is returned for every DescribeLaunchTemplateVersions call.
	ltErr error
}

// DescribeLaunchTemplateVersions overrides the embedded EC2Fake method.
func (f *ngTestEC2Fake) DescribeLaunchTemplateVersions(
	_ context.Context,
	input *ec2.DescribeLaunchTemplateVersionsInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	if f.ltErr != nil {
		return nil, f.ltErr
	}
	if input.LaunchTemplateId == nil {
		return &ec2.DescribeLaunchTemplateVersionsOutput{}, nil
	}
	ltID := *input.LaunchTemplateId
	version := "$Default"
	if len(input.Versions) > 0 {
		version = input.Versions[0]
	}
	key := ltID + ":" + version
	if out, ok := f.ltOutputs[key]; ok {
		return out, nil
	}
	return &ec2.DescribeLaunchTemplateVersionsOutput{}, nil
}

// ---------------------------------------------------------------------------
// Helper: build ServiceClients for registered-fetcher tests
// ---------------------------------------------------------------------------

func newNGTestClients(eksFake *ngTestEKSFake, ec2Fake *ngTestEC2Fake) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{
		EKS: eksFake,
		EC2: ec2Fake,
	}
}

// ---------------------------------------------------------------------------
// TestRegisteredNGFetcher_ResolvesImageIDFromCustomLaunchTemplate
//
// Verifies that the registered "ng" paginated fetcher (the path used by the
// live app) populates Fields["image_id"] for a nodegroup with a custom
// LaunchTemplate, and leaves Fields["image_id"] == "" for one without.
// ---------------------------------------------------------------------------

func TestRegisteredNGFetcher_ResolvesImageIDFromCustomLaunchTemplate(t *testing.T) {
	pf := resource.GetPaginatedFetcher("ng")
	if pf == nil {
		t.Fatal("paginated fetcher for 'ng' not registered — ensure internal/aws package is imported")
	}

	desiredCustom := int32(3)
	desiredDefault := int32(2)

	eksFake := &ngTestEKSFake{
		clusters: []string{"prod"},
		nodegroups: map[string][]string{
			"prod": {"ng-custom", "ng-default"},
		},
		nodegroupDetails: map[string]*ekstypes.Nodegroup{
			"prod/ng-custom": {
				NodegroupName: aws.String("ng-custom"),
				ClusterName:   aws.String("prod"),
				Status:        ekstypes.NodegroupStatusActive,
				InstanceTypes: []string{"m5.large"},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					DesiredSize: &desiredCustom,
				},
				// Custom LaunchTemplate — registered fetcher must call resolveNGImageID.
				LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
					Id:      aws.String("lt-100"),
					Version: aws.String("3"),
				},
			},
			"prod/ng-default": {
				NodegroupName: aws.String("ng-default"),
				ClusterName:   aws.String("prod"),
				Status:        ekstypes.NodegroupStatusActive,
				InstanceTypes: []string{"t3.medium"},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					DesiredSize: &desiredDefault,
				},
				// No custom LaunchTemplate — image_id must remain "".
				LaunchTemplate: nil,
			},
		},
	}

	ec2Fake := &ngTestEC2Fake{
		EC2Fake: fakes.NewEC2(),
		ltOutputs: map[string]*ec2.DescribeLaunchTemplateVersionsOutput{
			"lt-100:3": {
				LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{
					{
						LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
							ImageId: aws.String("ami-xyz"),
						},
					},
				},
			},
		},
	}

	sc := newNGTestClients(eksFake, ec2Fake)

	result, err := pf(context.Background(), sc, "")
	if err != nil {
		t.Fatalf("registered fetcher returned unexpected error: %v", err)
	}
	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}

	// Locate resources by name — order is not guaranteed by the fetcher.
	byName := make(map[string]resource.Resource, len(result.Resources))
	for _, r := range result.Resources {
		byName[r.Name] = r
	}

	// ng-custom: must have image_id resolved from the custom launch template.
	custom, ok := byName["ng-custom"]
	if !ok {
		t.Fatal("resource 'ng-custom' not found in fetcher output")
	}
	if got := custom.Fields["image_id"]; got != "ami-xyz" {
		t.Errorf("ng-custom Fields[\"image_id\"]: expected \"ami-xyz\", got %q", got)
	}

	// ng-default: nil LaunchTemplate → image_id must be "".
	defaultNG, ok := byName["ng-default"]
	if !ok {
		t.Fatal("resource 'ng-default' not found in fetcher output")
	}
	if got := defaultNG.Fields["image_id"]; got != "" {
		t.Errorf("ng-default Fields[\"image_id\"]: expected \"\", got %q", got)
	}

	// resource.GetFieldKeys("ng") must include "image_id".
	keys := resource.GetFieldKeys("ng")
	found := false
	for _, k := range keys {
		if k == "image_id" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("resource.GetFieldKeys(\"ng\") does not contain \"image_id\"; got: %v", keys)
	}
}

// ---------------------------------------------------------------------------
// TestRegisteredNGFetcher_ImageIDEmptyWhenLaunchTemplateResolveFails
//
// Verifies that the registered "ng" fetcher emits the nodegroup even when
// DescribeLaunchTemplateVersions returns an error, leaving Fields["image_id"] == "".
// ---------------------------------------------------------------------------

func TestRegisteredNGFetcher_ImageIDEmptyWhenLaunchTemplateResolveFails(t *testing.T) {
	pf := resource.GetPaginatedFetcher("ng")
	if pf == nil {
		t.Fatal("paginated fetcher for 'ng' not registered")
	}

	desired := int32(1)

	eksFake := &ngTestEKSFake{
		clusters: []string{"staging"},
		nodegroups: map[string][]string{
			"staging": {"ng-lt-fail"},
		},
		nodegroupDetails: map[string]*ekstypes.Nodegroup{
			"staging/ng-lt-fail": {
				NodegroupName: aws.String("ng-lt-fail"),
				ClusterName:   aws.String("staging"),
				Status:        ekstypes.NodegroupStatusActive,
				InstanceTypes: []string{"c5.xlarge"},
				ScalingConfig: &ekstypes.NodegroupScalingConfig{
					DesiredSize: &desired,
				},
				LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
					Id:      aws.String("lt-bad"),
					Version: aws.String("1"),
				},
			},
		},
	}

	// EC2 fake returns an error for every DescribeLaunchTemplateVersions call.
	ec2Fake := &ngTestEC2Fake{
		EC2Fake: fakes.NewEC2(),
		ltErr:   errNGTestLTNotFound,
	}

	sc := newNGTestClients(eksFake, ec2Fake)

	result, err := pf(context.Background(), sc, "")
	if err != nil {
		t.Fatalf("registered fetcher must not propagate DescribeLaunchTemplateVersions error; got: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource (nodegroup emitted despite LT error), got %d", len(result.Resources))
	}

	r := result.Resources[0]
	if r.Name != "ng-lt-fail" {
		t.Errorf("expected resource name \"ng-lt-fail\", got %q", r.Name)
	}
	if got := r.Fields["image_id"]; got != "" {
		t.Errorf("Fields[\"image_id\"]: expected \"\" when LT resolve fails, got %q", got)
	}
}

// errNGTestLTNotFound is a sentinel error used in TestRegisteredNGFetcher_ImageIDEmptyWhenLaunchTemplateResolveFails.
// Defined at package level to avoid repetition.
var errNGTestLTNotFound = fmt.Errorf("EC2 API error: launch template not found")
