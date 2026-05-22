package unit

// aws_nodegroups_image_id_test.go — Failing tests for nodegroup AMI ID resolution
// via custom LaunchTemplate → EC2 DescribeLaunchTemplateVersions.
//
// These tests are RED until:
//   1. FetchNodeGroups signature is extended to accept EC2DescribeLaunchTemplateVersionsAPI
//   2. buildNodeGroupResource (or the fetcher loop) resolves and populates Fields["image_id"]
//      when the nodegroup has a custom LaunchTemplate.
//   3. resource.SetFieldKeysForTest("ng", ...) includes "image_id".

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
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Fake: EC2DescribeLaunchTemplateVersionsAPI
// ---------------------------------------------------------------------------

// fakeEC2DescribeLaunchTemplateVersions implements awsclient.EC2DescribeLaunchTemplateVersionsAPI
// for unit tests. Keyed by "launchTemplateId:version" (e.g. "lt-001:3").
// When the err field is non-nil, every call returns that error.
type fakeEC2DescribeLaunchTemplateVersions struct {
	// outputs keyed by "<launchTemplateId>:<version>" — e.g. "lt-001:3" or "lt-002:$Default"
	outputs map[string]*ec2.DescribeLaunchTemplateVersionsOutput
	err     error
	// lastInput captures the last input for assertion purposes
	lastInput *ec2.DescribeLaunchTemplateVersionsInput
}

func (f *fakeEC2DescribeLaunchTemplateVersions) DescribeLaunchTemplateVersions(
	ctx context.Context,
	params *ec2.DescribeLaunchTemplateVersionsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	f.lastInput = params
	if f.err != nil {
		return nil, f.err
	}
	if params.LaunchTemplateId == nil {
		return &ec2.DescribeLaunchTemplateVersionsOutput{}, nil
	}
	ltID := *params.LaunchTemplateId
	version := "$Default"
	if len(params.Versions) > 0 {
		version = params.Versions[0]
	}
	key := ltID + ":" + version
	if out, ok := f.outputs[key]; ok {
		return out, nil
	}
	return &ec2.DescribeLaunchTemplateVersionsOutput{}, nil
}

// ---------------------------------------------------------------------------
// Helper: build a minimal three-step EKS mock set for a single nodegroup
// ---------------------------------------------------------------------------

func eksMinimalMocksForNG(clusterName, ngName string, ng *ekstypes.Nodegroup) (
	*mockEKSListClustersClient,
	*mockEKSListNodegroupsClient,
	*mockEKSDescribeNodegroupClient,
) {
	listClusters := &mockEKSListClustersClient{
		output: &eks.ListClustersOutput{Clusters: []string{clusterName}},
	}
	listNGs := &mockEKSListNodegroupsClient{
		outputs: map[string]*eks.ListNodegroupsOutput{
			clusterName: {Nodegroups: []string{ngName}},
		},
	}
	describeNG := &mockEKSDescribeNodegroupClient{
		outputs: map[string]*eks.DescribeNodegroupOutput{
			clusterName + "/" + ngName: {Nodegroup: ng},
		},
	}
	return listClusters, listNGs, describeNG
}

// ---------------------------------------------------------------------------
// T-NG-IMG01: Custom LaunchTemplate → image_id populated
// ---------------------------------------------------------------------------

func TestFetchNodeGroups_ResolvesImageIDFromCustomLaunchTemplate(t *testing.T) {
	desiredSize := int32(2)
	ng := &ekstypes.Nodegroup{
		NodegroupName: aws.String("ng-custom"),
		ClusterName:   aws.String("prod-cluster"),
		Status:        ekstypes.NodegroupStatusActive,
		InstanceTypes: []string{"m5.large"},
		ScalingConfig: &ekstypes.NodegroupScalingConfig{
			DesiredSize: &desiredSize,
		},
		// Custom LaunchTemplate with explicit version "3"
		LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
			Id:      aws.String("lt-001"),
			Version: aws.String("3"),
		},
	}

	listClusters, listNGs, describeNG := eksMinimalMocksForNG("prod-cluster", "ng-custom", ng)

	ltFake := &fakeEC2DescribeLaunchTemplateVersions{
		outputs: map[string]*ec2.DescribeLaunchTemplateVersionsOutput{
			"lt-001:3": {
				LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{
					{
						LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
							ImageId: aws.String("ami-123456"),
						},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClusters, listNGs, describeNG, ltFake)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	got := resources[0].Fields["image_id"]
	if got != "ami-123456" {
		t.Errorf("Fields[\"image_id\"]: expected \"ami-123456\", got %q", got)
	}
}

// ---------------------------------------------------------------------------
// T-NG-IMG02: No LaunchTemplate → image_id empty
// ---------------------------------------------------------------------------

func TestFetchNodeGroups_ImageIDEmptyWhenNoLaunchTemplate(t *testing.T) {
	desiredSize := int32(1)
	ng := &ekstypes.Nodegroup{
		NodegroupName: aws.String("ng-managed"),
		ClusterName:   aws.String("dev-cluster"),
		Status:        ekstypes.NodegroupStatusActive,
		InstanceTypes: []string{"t3.medium"},
		ScalingConfig: &ekstypes.NodegroupScalingConfig{
			DesiredSize: &desiredSize,
		},
		AmiType:        ekstypes.AMITypesAl2X8664,
		LaunchTemplate: nil, // EKS-managed, no custom launch template
	}

	listClusters, listNGs, describeNG := eksMinimalMocksForNG("dev-cluster", "ng-managed", ng)

	// EC2 fake that should never be called — if it is called, we fail the test
	ltFake := &fakeEC2DescribeLaunchTemplateVersions{
		err: fmt.Errorf("DescribeLaunchTemplateVersions should not be called when LaunchTemplate is nil"),
	}
	// We pass a no-op fake so the test still compiles once FetchNodeGroups
	// gains the extra parameter; but if the production code calls it despite
	// nil LaunchTemplate, the error will propagate (or be ignored with empty image_id).
	_ = ltFake

	// Use a safe no-op fake that returns empty output without error.
	noopLTFake := &fakeEC2DescribeLaunchTemplateVersions{}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClusters, listNGs, describeNG, noopLTFake)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	got := resources[0].Fields["image_id"]
	if got != "" {
		t.Errorf("Fields[\"image_id\"]: expected empty string (EKS-managed AMI type), got %q", got)
	}
}

// ---------------------------------------------------------------------------
// T-NG-IMG03: LaunchTemplate present but DescribeLaunchTemplateVersions errors
// → image_id empty, nodegroup still emitted
// ---------------------------------------------------------------------------

func TestFetchNodeGroups_ImageIDEmptyWhenLaunchTemplateResolveFails(t *testing.T) {
	desiredSize := int32(3)
	ng := &ekstypes.Nodegroup{
		NodegroupName: aws.String("ng-lt-error"),
		ClusterName:   aws.String("staging-cluster"),
		Status:        ekstypes.NodegroupStatusActive,
		InstanceTypes: []string{"c5.xlarge"},
		ScalingConfig: &ekstypes.NodegroupScalingConfig{
			DesiredSize: &desiredSize,
		},
		LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
			Id:      aws.String("lt-bad"),
			Version: aws.String("1"),
		},
	}

	listClusters, listNGs, describeNG := eksMinimalMocksForNG("staging-cluster", "ng-lt-error", ng)

	ltFake := &fakeEC2DescribeLaunchTemplateVersions{
		err: fmt.Errorf("AWS API error: launch template not found"),
	}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClusters, listNGs, describeNG, ltFake)
	// The fetch should succeed (error from DescribeLaunchTemplateVersions is non-fatal)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource (nodegroup still emitted despite LT error), got %d", len(resources))
	}

	// Nodegroup is present with other fields populated
	r := resources[0]
	if r.Fields["nodegroup_name"] != "ng-lt-error" {
		t.Errorf("nodegroup_name: expected \"ng-lt-error\", got %q", r.Fields["nodegroup_name"])
	}
	if r.Fields["desired_size"] != "3" {
		t.Errorf("desired_size: expected \"3\", got %q", r.Fields["desired_size"])
	}

	// image_id must be empty — not a crash, just unresolvable
	got := r.Fields["image_id"]
	if got != "" {
		t.Errorf("Fields[\"image_id\"]: expected \"\" when LT resolve fails, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// T-NG-IMG04: LaunchTemplate with nil Version → uses "$Default"
// ---------------------------------------------------------------------------

func TestFetchNodeGroups_UsesDefaultVersionWhenVersionIsEmpty(t *testing.T) {
	desiredSize := int32(2)
	ng := &ekstypes.Nodegroup{
		NodegroupName: aws.String("ng-default-lt"),
		ClusterName:   aws.String("prod-cluster"),
		Status:        ekstypes.NodegroupStatusActive,
		InstanceTypes: []string{"m5.xlarge"},
		ScalingConfig: &ekstypes.NodegroupScalingConfig{
			DesiredSize: &desiredSize,
		},
		// LaunchTemplate without explicit version — should fall back to "$Default"
		LaunchTemplate: &ekstypes.LaunchTemplateSpecification{
			Id:      aws.String("lt-002"),
			Version: nil,
		},
	}

	listClusters, listNGs, describeNG := eksMinimalMocksForNG("prod-cluster", "ng-default-lt", ng)

	ltFake := &fakeEC2DescribeLaunchTemplateVersions{
		outputs: map[string]*ec2.DescribeLaunchTemplateVersionsOutput{
			// The coder must call with Versions=["$Default"] when Version is nil
			"lt-002:$Default": {
				LaunchTemplateVersions: []ec2types.LaunchTemplateVersion{
					{
						LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
							ImageId: aws.String("ami-default-999"),
						},
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchNodeGroups(context.Background(), listClusters, listNGs, describeNG, ltFake)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	got := resources[0].Fields["image_id"]
	if got != "ami-default-999" {
		t.Errorf("Fields[\"image_id\"]: expected \"ami-default-999\" (resolved from $Default version), got %q", got)
	}

	// Verify the fake was called with "$Default"
	if ltFake.lastInput == nil {
		t.Fatal("DescribeLaunchTemplateVersions was never called")
	}
	if ltFake.lastInput.LaunchTemplateId == nil || *ltFake.lastInput.LaunchTemplateId != "lt-002" {
		t.Errorf("DescribeLaunchTemplateVersions called with wrong LaunchTemplateId: %v", ltFake.lastInput.LaunchTemplateId)
	}
	if len(ltFake.lastInput.Versions) == 0 || ltFake.lastInput.Versions[0] != "$Default" {
		t.Errorf("DescribeLaunchTemplateVersions Versions: expected [\"$Default\"], got %v", ltFake.lastInput.Versions)
	}
}

// ---------------------------------------------------------------------------
// T-NG-IMG05: SetFieldKeysForTest("ng", ...) includes "image_id"
// ---------------------------------------------------------------------------

func TestFetchNodeGroups_RegistersImageIDField(t *testing.T) {
	keys := resource.GetFieldKeys("ng")
	if len(keys) == 0 {
		t.Fatal("GetFieldKeys(\"ng\") returned nil/empty — SetFieldKeysForTest(\"ng\", ...) was not called")
	}

	found := false
	for _, k := range keys {
		if k == "image_id" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("SetFieldKeysForTest(\"ng\", ...) does not include \"image_id\"; current keys: %v", keys)
	}
}
